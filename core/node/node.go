package node

import (
	"context"
	"crypto/tls"
	"sync"
	"time"

	"github.com/snple/beacon/core"
	"github.com/snple/beacon/device"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
	"github.com/snple/beacon/pb/nodes"
	"github.com/snple/beacon/util/metadata"
	"github.com/snple/beacon/util/token"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	queen "snple.com/queen/core"
)

type NodeService struct {
	cs *core.CoreService

	sync *SyncService
	wire *WireService
	pin  *PinService

	// queen broker for edge device communication
	broker *queen.Broker
	// internal client for request/response and action handlers
	internalClient *queen.InternalClient

	ctx     context.Context
	cancel  func()
	closeWG sync.WaitGroup

	dopts nodeOptions

	nodes.UnimplementedNodeServiceServer
}

func Node(cs *core.CoreService, opts ...NodeOption) (*NodeService, error) {
	ctx, cancel := context.WithCancel(cs.Context())

	ns := &NodeService{
		cs:     cs,
		ctx:    ctx,
		cancel: cancel,
		dopts:  defaultNodeOptions(),
	}

	for _, opt := range extraNodeOptions {
		opt.apply(&ns.dopts)
	}

	for _, opt := range opts {
		opt.apply(&ns.dopts)
	}

	ns.sync = newSyncService(ns)
	ns.wire = newWireService(ns)
	ns.pin = newPinService(ns)

	// 初始化 queen broker
	if ns.dopts.brokerEnable {
		if err := ns.initBroker(); err != nil {
			cancel()
			return nil, err
		}
	}

	return ns, nil
}

func (ns *NodeService) Start() {
	// 启动 queen broker
	if ns.broker != nil {
		if err := ns.broker.Start(); err != nil {
			ns.Logger().Sugar().Errorf("Failed to start queen broker: %v", err)
		} else {
			ns.Logger().Sugar().Infof("Queen broker started on %s", ns.dopts.brokerAddr)
		}
	}

	// 启动消息处理协程
	if ns.internalClient != nil {
		ns.closeWG.Add(1)
		go ns.processMessages()
	}
}

func (ns *NodeService) Stop() {
	// 关闭内部客户端
	if ns.internalClient != nil {
		ns.internalClient.Close()
	}

	// 停止 queen broker
	if ns.broker != nil {
		if err := ns.broker.Stop(); err != nil {
			ns.Logger().Sugar().Errorf("Failed to stop queen broker: %v", err)
		}
	}

	ns.cancel()
	ns.closeWG.Wait()
}

func (ns *NodeService) Core() *core.CoreService {
	return ns.cs
}

func (ns *NodeService) Context() context.Context {
	return ns.ctx
}

func (ns *NodeService) Logger() *zap.Logger {
	return ns.cs.Logger()
}

func (ns *NodeService) RegisterGrpc(server *grpc.Server) {
	nodes.RegisterSyncServiceServer(server, ns.sync)
	nodes.RegisterNodeServiceServer(server, ns)
	nodes.RegisterWireServiceServer(server, ns.wire)
	nodes.RegisterPinServiceServer(server, ns.pin)
}

type nodeOptions struct {
	keepAlive time.Duration

	// queen broker options
	brokerEnable    bool
	brokerAddr      string
	brokerTLSConfig *tls.Config
}

func defaultNodeOptions() nodeOptions {
	return nodeOptions{
		keepAlive:    10 * time.Second,
		brokerEnable: false,
		brokerAddr:   ":3883",
	}
}

type NodeOption interface {
	apply(*nodeOptions)
}

var extraNodeOptions []NodeOption

type funcNodeOption struct {
	f func(*nodeOptions)
}

func (fdo *funcNodeOption) apply(do *nodeOptions) {
	fdo.f(do)
}

func newFuncNodeOption(f func(*nodeOptions)) *funcNodeOption {
	return &funcNodeOption{
		f: f,
	}
}

func WithKeepAlive(d time.Duration) NodeOption {
	return newFuncNodeOption(func(o *nodeOptions) {
		o.keepAlive = d
	})
}

// WithBrokerEnable 启用 queen broker
func WithBrokerEnable(enable bool) NodeOption {
	return newFuncNodeOption(func(o *nodeOptions) {
		o.brokerEnable = enable
	})
}

// WithBrokerAddr 设置 queen broker 监听地址
func WithBrokerAddr(addr string) NodeOption {
	return newFuncNodeOption(func(o *nodeOptions) {
		o.brokerAddr = addr
	})
}

// WithBrokerTLS 设置 queen broker TLS 配置
func WithBrokerTLS(tlsConfig *tls.Config) NodeOption {
	return newFuncNodeOption(func(o *nodeOptions) {
		o.brokerTLSConfig = tlsConfig
	})
}

// initBroker 初始化 queen broker
func (ns *NodeService) initBroker() error {
	brokerOpts := []queen.BrokerOption{
		queen.WithAddress(ns.dopts.brokerAddr),
	}

	if ns.dopts.brokerTLSConfig != nil {
		brokerOpts = append(brokerOpts, queen.WithTLS(ns.dopts.brokerTLSConfig))
	}

	// 设置认证器，使用 beacon 的 node 认证
	brokerOpts = append(brokerOpts, queen.WithAuthFunc(func(info *queen.AuthInfo) error {
		return ns.authenticateBrokerClient(info)
	}))

	broker, err := queen.New(brokerOpts...)
	if err != nil {
		return err
	}

	ns.broker = broker

	// 创建内部客户端并注册必要的 action 处理器（非致命）
	icCfg := queen.InternalClientConfig{ClientID: "beacon-core", BufferSize: 200}
	ic, err := ns.broker.NewInternalClient(icCfg)
	if err != nil {
		ns.Logger().Sugar().Warnf("Failed to create internal client: %v", err)
		// 仍然返回 broker 可用，但不阻塞服务启动
		return nil
	}
	ns.internalClient = ic
	// 注册内部 action 处理器由 queen.go 中实现
	if err := ns.registerInternalHandlers(); err != nil {
		ns.Logger().Sugar().Warnf("Failed to register internal handlers: %v", err)
	}

	// 订阅所有 beacon 主题
	if err := ns.subscribeTopics(); err != nil {
		ns.Logger().Sugar().Warnf("Failed to subscribe topics: %v", err)
	}

	return nil
}

// authenticateBrokerClient 认证 broker 客户端
func (ns *NodeService) authenticateBrokerClient(info *queen.AuthInfo) error {
	ctx := context.Background()

	// ClientID 作为 Node ID 或 Name
	clientID := info.ClientID
	if clientID == "" {
		return status.Error(codes.InvalidArgument, "ClientID is required")
	}

	// AuthData 作为 Secret
	secret := string(info.AuthData)
	if secret == "" {
		return status.Error(codes.InvalidArgument, "Secret is required")
	}

	// 尝试通过 ID 或 Name 获取 Node
	var node *pb.Node
	var err error

	node, err = ns.Core().GetNode().View(ctx, &pb.Id{Id: clientID})
	if err != nil {
		// 尝试通过 Name 获取
		node, err = ns.Core().GetNode().Name(ctx, &pb.Name{Name: clientID})
		if err != nil {
			return status.Errorf(codes.NotFound, "Node not found: %s", clientID)
		}
	}

	if node.Status != device.ON {
		return status.Error(codes.FailedPrecondition, "Node is not enabled")
	}

	// 验证密钥
	secretReply, err := ns.Core().GetNode().GetSecret(ctx, &pb.Id{Id: node.Id})
	if err != nil {
		return status.Errorf(codes.Internal, "GetSecret failed: %v", err)
	}

	if secretReply.Message != secret {
		return status.Error(codes.Unauthenticated, "Invalid secret")
	}

	ns.Logger().Sugar().Infof("Queen broker client authenticated: %s, remote: %s", node.Id, info.RemoteAddr)

	return nil
}

// GetBroker 获取 queen broker 实例
func (ns *NodeService) GetBroker() *queen.Broker {
	return ns.broker
}

func (s *NodeService) Login(ctx context.Context, in *nodes.NodeLoginRequest) (*nodes.NodeLoginReply, error) {
	var output nodes.NodeLoginReply

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Id == "" && in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Node.ID or Node.Name")
		}

		if in.Secret == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Node.Secret")
		}
	}

	var node *pb.Node
	var err error

	if in.Id != "" {
		node, err = s.Core().GetNode().View(ctx, &pb.Id{Id: in.Id})
	} else if in.Name != "" {
		node, err = s.Core().GetNode().Name(ctx, &pb.Name{Name: in.Name})
	} else {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid Node.ID or Node.Name")
	}

	if err != nil {
		return &output, err
	}

	if node.Status != device.ON {
		s.Logger().Sugar().Errorf("node connect error: node is not enable, id: %v, ip: %v",
			in.Id, metadata.GetPeerAddr(ctx))
		return &output, status.Error(codes.FailedPrecondition, "The node is not enable")
	}

	// 通过 GetSecret API 获取密钥并验证
	secretReply, err := s.Core().GetNode().GetSecret(ctx, &pb.Id{Id: node.Id})
	if err != nil {
		return &output, status.Errorf(codes.Internal, "GetSecret failed: %v", err)
	}

	if secretReply.Message != in.Secret {
		s.Logger().Sugar().Errorf("node connect error: node secret is not valid, id: %v, ip: %v",
			in.Id, metadata.GetPeerAddr(ctx))
		return &output, status.Error(codes.Unauthenticated, "Please supply valid secret")
	}

	token, err := token.ClaimNodeToken(node.Id)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "claim token: %v", err)
	}

	s.Logger().Sugar().Infof("node connect success, id: %v, ip: %v", node.Id, metadata.GetPeerAddr(ctx))

	output.Node = node
	output.Token = token

	return &output, nil
}

func validateToken(ctx context.Context) (nodeID string, err error) {
	tks, err := metadata.GetToken(ctx)
	if err != nil {
		return "", err
	}

	ok, nodeID := token.ValidateNodeToken(tks)
	if !ok {
		return "", status.Errorf(codes.Unauthenticated, "Token validation failed")
	}

	return nodeID, nil
}

func (s *NodeService) View(ctx context.Context, in *pb.MyEmpty) (*pb.Node, error) {
	var output pb.Node
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	request := &pb.Id{Id: nodeID}

	reply, err := s.Core().GetNode().View(ctx, request)
	if err != nil {
		return &output, err
	}

	return reply, err
}

func (s *NodeService) Push(ctx context.Context, in *nodes.NodePushRequest) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	if in == nil || len(in.Nson) == 0 {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid NSON data")
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	// 转发到 Core
	request := &cores.NodePushRequest{
		Id:   nodeID,
		Nson: in.Nson,
	}

	return s.Core().GetNode().Push(ctx, request)
}

func (s *NodeService) KeepAlive(in *pb.MyEmpty, stream nodes.NodeService_KeepAliveServer) error {
	var err error

	// basic validation
	{
		if in == nil {
			return status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	_, err = validateToken(stream.Context())
	if err != nil {
		return err
	}

	for {
		err := stream.Send(&nodes.NodeKeepAliveReply{Time: int32(time.Now().Unix())})
		if err != nil {
			return err
		}

		time.Sleep(s.dopts.keepAlive)
	}
}
