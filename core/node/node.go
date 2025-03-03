package node

import (
	"context"
	"sync"
	"time"

	"github.com/snple/beacon/consts"
	"github.com/snple/beacon/core"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
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

	sync     *SyncService
	slot     *SlotService
	wire     *WireService
	pin      *PinService
	constant *ConstService

	auth *AuthService
	user *UserService

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
	ns.slot = newSlotService(ns)
	ns.wire = newWireService(ns)
	ns.pin = newPinService(ns)
	ns.constant = newConstService(ns)

	ns.auth = newAuthService(ns)
	ns.user = newUserService(ns)

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
	nodes.RegisterSlotServiceServer(server, ns.slot)
	nodes.RegisterWireServiceServer(server, ns.wire)
	nodes.RegisterPinServiceServer(server, ns.pin)
	nodes.RegisterConstServiceServer(server, ns.constant)

	nodes.RegisterAuthServiceServer(server, ns.auth)
	nodes.RegisterUserServiceServer(server, ns.user)
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

		if in.GetId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Node.ID")
		}

		if in.GetSecret() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Node.Secret")
		}
	}

	request := &pb.Id{Id: in.GetId()}

	reply, err := s.Core().GetNode().View(ctx, request)
	if err != nil {
		return &output, err
	}

	if reply.GetStatus() != consts.ON {
		s.Logger().Sugar().Errorf("node connect error: node is not enable, id: %v, ip: %v",
			in.GetId(), metadata.GetPeerAddr(ctx))
		return &output, status.Error(codes.FailedPrecondition, "The node is not enable")
	}

	if reply.GetSecret() != in.GetSecret() {
		s.Logger().Sugar().Errorf("node connect error: node secret is not valid, id: %v, ip: %v",
			in.GetId(), metadata.GetPeerAddr(ctx))
		return &output, status.Error(codes.Unauthenticated, "Please supply valid secret")
	}

	token, err := token.ClaimNodeToken(reply.Id)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "claim token: %v", err)
	}

	s.Logger().Sugar().Infof("node connect success, id: %v, ip: %v", in.GetId(), metadata.GetPeerAddr(ctx))

	reply.Secret = ""

	output.Node = reply
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

func (s *NodeService) Update(ctx context.Context, in *pb.Node) (*pb.Node, error) {
	var output pb.Node

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

	if in.GetId() != nodeID {
		return &output, status.Error(codes.NotFound, "Query: in.GetId() != nodeID")
	}

	request := &pb.Id{Id: nodeID}

	reply, err := s.Core().GetNode().View(ctx, request)
	if err != nil {
		return &output, err
	}

	in.Secret = reply.GetSecret()
	in.Status = reply.GetStatus()

	return s.Core().GetNode().Update(ctx, in)
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

func (s *NodeService) Link(ctx context.Context, in *nodes.NodeLinkRequest) (*pb.MyBool, error) {
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

	request := &cores.NodeLinkRequest{Id: nodeID, Status: in.GetStatus()}

	return s.Core().GetNode().Link(ctx, request)
}

func (s *NodeService) ViewWithDeleted(ctx context.Context, in *pb.MyEmpty) (*pb.Node, error) {
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

	reply, err := s.Core().GetNode().ViewWithDeleted(ctx, request)
	if err != nil {
		return &output, err
	}

	reply.Secret = ""

	return reply, err
}

func (s *NodeService) Sync(ctx context.Context, in *pb.Node) (*pb.MyBool, error) {
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

	if in.GetId() != nodeID {
		return &output, status.Error(codes.NotFound, "Query: in.GetId() != nodeID")
	}

	request := &pb.Id{Id: nodeID}

	reply, err := s.Core().GetNode().ViewWithDeleted(ctx, request)
	if err != nil {
		return &output, err
	}

	in.Secret = reply.GetSecret()
	in.Status = reply.GetStatus()
	in.Deleted = reply.GetDeleted()

	return s.Core().GetNode().Sync(ctx, in)
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
