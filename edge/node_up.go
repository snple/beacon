package edge

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/edges"
	"github.com/snple/beacon/pb/nodes"
	"github.com/snple/beacon/util/metadata"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type NodeUpService struct {
	es *EdgeService

	NodeConn *grpc.ClientConn

	token string
	lock  sync.RWMutex

	ctx     context.Context
	cancel  func()
	closeWG sync.WaitGroup
}

func newNodeUpService(es *EdgeService) (*NodeUpService, error) {
	var err error

	es.Logger().Sugar().Infof("link node service: %v", es.dopts.NodeOptions.Addr)

	nodeConn, err := grpc.NewClient(es.dopts.NodeOptions.Addr, es.dopts.NodeOptions.GRPCOptions...)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(es.Context())

	s := &NodeUpService{
		es:       es,
		NodeConn: nodeConn,
		ctx:      ctx,
		cancel:   cancel,
	}

	return s, nil
}

func (s *NodeUpService) start() {
	s.closeWG.Add(1)
	defer s.closeWG.Done()

	s.es.Logger().Sugar().Info("node service started")

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			s.try()
		}
	}
}

func (s *NodeUpService) stop() {
	s.cancel()
	s.closeWG.Wait()
}

func (s *NodeUpService) push() error {
	// login
	operation := func() error {
		err := s.login(s.ctx)
		if err != nil {
			s.es.Logger().Sugar().Errorf("node login: %v", err)
		}

		return err
	}

	err := backoff.Retry(operation, backoff.WithContext(backoff.NewExponentialBackOff(), s.ctx))
	if err != nil {
		s.es.Logger().Sugar().Errorf("backoff.Retry: %v", err)
		return err
	}

	s.es.Logger().Sugar().Info("node login success")

	s.es.GetSync().setNodeToRemote(s.ctx, time.Time{})

	// 同步配置数据 Edge → Core
	if err := s.syncLocalToRemote(s.ctx); err != nil {
		return err
	}

	// 同步运行数据 Edge → Core
	if err := s.syncPinValueToRemote(s.ctx); err != nil {
		return err
	}

	s.es.Logger().Sugar().Info("push success")

	return nil
}

func (s *NodeUpService) pull() error {
	// login
	operation := func() error {
		err := s.login(s.ctx)
		if err != nil {
			s.es.Logger().Sugar().Errorf("node login: %v", err)
		}

		return err
	}

	err := backoff.Retry(operation, backoff.WithContext(backoff.NewExponentialBackOff(), s.ctx))
	if err != nil {
		s.es.Logger().Sugar().Errorf("backoff.Retry: %v", err)
		return err
	}

	s.es.Logger().Sugar().Info("node login success")

	s.es.GetSync().setPinWriteFromRemote(s.ctx, time.Time{})

	// 拉取写命令 Core → Edge
	if err := s.syncPinWriteFromRemote(s.ctx); err != nil {
		return err
	}

	s.es.Logger().Sugar().Info("pull success")

	return nil
}

func (s *NodeUpService) try() {
	// login
	operation := func() error {
		err := s.login(s.ctx)
		if err != nil {
			s.es.Logger().Sugar().Infof("node login: %v", err)
		}

		return err
	}

	err := backoff.Retry(operation, backoff.WithContext(backoff.NewExponentialBackOff(), s.ctx))
	if err != nil {
		s.es.Logger().Sugar().Errorf("backoff.Retry: %v", err)
		return
	}

	s.es.Logger().Sugar().Info("node login success")

	if err := s.sync(s.ctx); err != nil {
		s.es.Logger().Sugar().Errorf("sync: %v", err)
		time.Sleep(time.Second * 3)
		return
	}

	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	// start ticker
	go s.ticker(ctx)

	// start realtime sync
	if s.es.dopts.SyncOptions.Realtime {
		// Edge → Core: 配置数据同步
		go s.waitLocalNodeUpdated(ctx)
		// Edge → Core: PinValue 同步
		go s.waitLocalPinValueUpdated(ctx)
		// Core → Edge: PinWrite 命令拉取
		go s.waitRemotePinWriteUpdated(ctx)
	}

	// keep alive
	{
		ctx = metadata.SetToken(ctx, s.GetToken())

		stream, err := s.NodeServiceClient().KeepAlive(ctx, &pb.MyEmpty{})
		if err != nil {
			s.es.Logger().Sugar().Errorf("KeepAlive: %v", err)
			return
		}

		for {
			reply, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}

				if code, ok := status.FromError(err); ok {
					if code.Code() == codes.Canceled {
						return
					}
				}

				s.es.Logger().Sugar().Errorf("KeepAlive.Recv(): %v", err)
				return
			}

			s.es.Logger().Sugar().Infof("keep alive reply: %+v", reply)
		}
	}
}

func (s *NodeUpService) ticker(ctx context.Context) {
	s.closeWG.Add(1)
	defer s.closeWG.Done()

	tokenRefreshTicker := time.NewTicker(s.es.dopts.SyncOptions.TokenRefresh)
	defer tokenRefreshTicker.Stop()

	syncTicker := time.NewTicker(s.es.dopts.SyncOptions.Interval)
	defer syncTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.es.Logger().Sugar().Info("ticker: ctx.Done()")
			return
		case <-tokenRefreshTicker.C:
			err := s.login(ctx)
			if err != nil {
				s.es.Logger().Sugar().Errorf("node login: %v", err)
			}
		case <-syncTicker.C:
			if err := s.sync(ctx); err != nil {
				s.es.Logger().Sugar().Errorf("sync: %v", err)
			}
		}
	}
}

func (s *NodeUpService) SyncServiceClient() nodes.SyncServiceClient {
	return nodes.NewSyncServiceClient(s.NodeConn)
}

func (s *NodeUpService) NodeServiceClient() nodes.NodeServiceClient {
	return nodes.NewNodeServiceClient(s.NodeConn)
}

func (s *NodeUpService) WireServiceClient() nodes.WireServiceClient {
	return nodes.NewWireServiceClient(s.NodeConn)
}

func (s *NodeUpService) PinServiceClient() nodes.PinServiceClient {
	return nodes.NewPinServiceClient(s.NodeConn)
}

func (s *NodeUpService) GetToken() string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.token
}

func (s *NodeUpService) SetToken(ctx context.Context) context.Context {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return metadata.SetToken(ctx, s.token)
}

func (s *NodeUpService) login(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var err error

	request := &nodes.NodeLoginRequest{
		Id:     s.es.dopts.nodeID,
		Secret: s.es.dopts.secret,
	}

	// try login
	reply, err := s.NodeServiceClient().Login(ctx, request)
	if err != nil {
		return err
	}

	if reply.Token == "" {
		return errors.New("login: reply token is empty")
	}

	// set token
	s.lock.Lock()
	s.token = reply.Token
	s.lock.Unlock()

	return nil
}

// waitLocalNodeUpdated: Edge 配置数据变化时同步到 Core
func (s *NodeUpService) waitLocalNodeUpdated(ctx context.Context) {
	s.closeWG.Add(1)
	defer s.closeWG.Done()

	notify := s.es.GetSync().Notify(NOTIFY)
	defer notify.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case <-notify.Wait():
			ctx = metadata.SetToken(ctx, s.GetToken())

			err := s.syncLocalToRemote(ctx)
			if err != nil {
				s.es.Logger().Sugar().Errorf("syncLocalToRemote: %v", err)
			}
		}
	}
}

// waitLocalPinValueUpdated: Edge PinValue 变化时同步到 Core
func (s *NodeUpService) waitLocalPinValueUpdated(ctx context.Context) {
	s.closeWG.Add(1)
	defer s.closeWG.Done()

	notify := s.es.GetSync().Notify(NOTIFY_PV)
	defer notify.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case <-notify.Wait():
			ctx = metadata.SetToken(ctx, s.GetToken())

			err := s.syncPinValueToRemote(ctx)
			if err != nil {
				s.es.Logger().Sugar().Errorf("syncPinValueToRemote: %v", err)
			}
		}
	}
}

// waitRemotePinWriteUpdated: 监听 Core 的 PinWrite 更新，拉取写命令
func (s *NodeUpService) waitRemotePinWriteUpdated(ctx context.Context) {
	s.closeWG.Add(1)
	defer s.closeWG.Done()

	for {
		ctx := metadata.SetToken(ctx, s.GetToken())

		stream, err := s.SyncServiceClient().WaitPinWriteUpdated(ctx, &pb.MyEmpty{})
		if err != nil {
			if code, ok := status.FromError(err); ok {
				if code.Code() == codes.Canceled {
					return
				}
			}

			s.es.Logger().Sugar().Errorf("WaitPinWriteUpdated: %v", err)

			// retry
			time.Sleep(s.es.dopts.SyncOptions.Retry)
			continue
		}

		for {
			_, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				if code, ok := status.FromError(err); ok {
					if code.Code() == codes.Canceled {
						return
					}
				}

				s.es.Logger().Sugar().Errorf("WaitPinWriteUpdated.Recv(): %v", err)
				return
			}

			ctx = metadata.SetToken(ctx, s.GetToken())

			err = s.syncPinWriteFromRemote(ctx)
			if err != nil {
				s.es.Logger().Sugar().Errorf("syncPinWriteFromRemote: %v", err)
			}
		}
	}
}

func (s *NodeUpService) sync(ctx context.Context) error {
	ctx = metadata.SetToken(ctx, s.GetToken())

	// Edge → Core: 配置数据同步
	if err := s.syncLocalToRemote(ctx); err != nil {
		return err
	}

	// Edge → Core: PinValue 同步
	if err := s.syncPinValueToRemote(ctx); err != nil {
		return err
	}

	// Core → Edge: PinWrite 命令拉取
	return s.syncPinWriteFromRemote(ctx)
}

// syncLocalToRemote: Edge 配置数据同步到 Core (Node/Wire/Pin)
func (s *NodeUpService) syncLocalToRemote(ctx context.Context) error {
	nodeUpdated, err := s.es.GetSync().getNodeUpdated(ctx)
	if err != nil {
		return err
	}

	nodeUpdated2, err := s.es.GetSync().getNodeToRemote(ctx)
	if err != nil {
		return err
	}

	if nodeUpdated.UnixMicro() <= nodeUpdated2.UnixMicro() {
		return nil
	}

	// wire
	{
		after := nodeUpdated2.UnixMicro()
		limit := uint32(10)

		for {
			locals, err := s.es.GetWire().Pull(ctx, &edges.WirePullRequest{After: after, Limit: limit})
			if err != nil {
				return err
			}

			for _, local := range locals.Wires {
				_, err = s.WireServiceClient().Sync(ctx, local)
				if err != nil {
					return err
				}

				after = local.Updated
			}

			if len(locals.Wires) < int(limit) {
				break
			}
		}
	}

	// pin
	{
		after := nodeUpdated2.UnixMicro()
		limit := uint32(10)

		for {
			locals, err := s.es.GetPin().Pull(ctx, &edges.PinPullRequest{After: after, Limit: limit})
			if err != nil {
				return err
			}

			for _, local := range locals.Pins {
				_, err = s.PinServiceClient().Sync(ctx, local)
				if err != nil {
					return err
				}

				after = local.Updated
			}

			if len(locals.Pins) < int(limit) {
				break
			}
		}
	}

	return s.es.GetSync().setNodeToRemote(ctx, nodeUpdated)
}

// syncPinValueToRemote: Edge PinValue 同步到 Core
func (s *NodeUpService) syncPinValueToRemote(ctx context.Context) error {
	pinValueUpdated, err := s.es.GetSync().getPinValueUpdated(ctx)
	if err != nil {
		return err
	}

	pinValueUpdated2, err := s.es.GetSync().getPinValueToRemote(ctx)
	if err != nil {
		return err
	}

	if pinValueUpdated.UnixMicro() <= pinValueUpdated2.UnixMicro() {
		return nil
	}

	after := pinValueUpdated2.UnixMicro()
	limit := uint32(100)

PULL:
	for {
		locals, err := s.es.GetPin().PullValue(ctx, &edges.PinPullValueRequest{After: after, Limit: limit})
		if err != nil {
			return err
		}

		for _, local := range locals.Pins {
			if local.Updated > pinValueUpdated.UnixMicro() {
				break PULL
			}

			_, err = s.PinServiceClient().SyncValue(ctx,
				&pb.PinValue{Id: local.Id, Value: local.Value, Updated: local.Updated})
			if err != nil {
				s.es.Logger().Sugar().Errorf("SyncValue: %v", err)
				return err
			}

			after = local.Updated
		}

		if len(locals.Pins) < int(limit) {
			break
		}
	}

	return s.es.GetSync().setPinValueToRemote(ctx, pinValueUpdated)
}

// syncPinWriteFromRemote: 从 Core 拉取 PinWrite 写命令
func (s *NodeUpService) syncPinWriteFromRemote(ctx context.Context) error {
	pinWriteUpdated, err := s.SyncServiceClient().GetPinWriteUpdated(ctx, &pb.MyEmpty{})
	if err != nil {
		return err
	}

	pinWriteUpdated2, err := s.es.GetSync().getPinWriteFromRemote(ctx)
	if err != nil {
		return err
	}

	if pinWriteUpdated.Updated <= pinWriteUpdated2.UnixMicro() {
		return nil
	}

	after := pinWriteUpdated2.UnixMicro()
	limit := uint32(100)

PULL:
	for {
		remotes, err := s.PinServiceClient().PullWrite(ctx, &nodes.PinPullValueRequest{After: after, Limit: limit})
		if err != nil {
			return err
		}

		for _, remote := range remotes.Pins {
			if remote.Updated > pinWriteUpdated.Updated {
				break PULL
			}

			_, err = s.es.GetPin().SyncWrite(ctx,
				&pb.PinValue{Id: remote.Id, Value: remote.Value, Updated: remote.Updated})
			if err != nil {
				s.es.Logger().Sugar().Errorf("SyncWrite: %v", err)
				return err
			}

			after = remote.Updated
		}

		if len(remotes.Pins) < int(limit) {
			break
		}
	}

	return s.es.GetSync().setPinWriteFromRemote(ctx, time.UnixMicro(pinWriteUpdated.GetUpdated()))
}
