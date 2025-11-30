package node

import (
	"context"
	"sync"
	"time"

	"github.com/snple/beacon/consts"
	"github.com/snple/beacon/core"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/nodes"
	"github.com/snple/beacon/util/metadata"
	"github.com/snple/beacon/util/token"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type NodeService struct {
	cs *core.CoreService

	sync *SyncService
	wire *WireService
	pin  *PinService

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

	return ns, nil
}

func (ns *NodeService) Start() {

}

func (ns *NodeService) Stop() {
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
}

func defaultNodeOptions() nodeOptions {
	return nodeOptions{
		keepAlive: 10 * time.Second,
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

	if node.Status != consts.ON {
		s.Logger().Sugar().Errorf("node connect error: node is not enable, id: %v, ip: %v",
			in.Id, metadata.GetPeerAddr(ctx))
		return &output, status.Error(codes.FailedPrecondition, "The node is not enable")
	}

	if node.Secret != in.Secret {
		s.Logger().Sugar().Errorf("node connect error: node secret is not valid, id: %v, ip: %v",
			in.Id, metadata.GetPeerAddr(ctx))
		return &output, status.Error(codes.Unauthenticated, "Please supply valid secret")
	}

	token, err := token.ClaimNodeToken(node.Id)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "claim token: %v", err)
	}

	s.Logger().Sugar().Infof("node connect success, id: %v, ip: %v", node.Id, metadata.GetPeerAddr(ctx))

	node.Secret = ""

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

	reply.Secret = ""

	return reply, err
}

func (s *NodeService) Push(ctx context.Context, in *pb.MyEmpty) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

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

	_, err = s.Core().GetNode().View(ctx, request)
	if err != nil {
		return &output, err
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
