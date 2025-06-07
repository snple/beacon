package edge

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/snple/beacon/consts"
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

	s.es.GetSync().setNodeUpdatedLocalToRemote(s.ctx, time.Time{})

	if err := s.sync2(s.ctx); err != nil {
		return err
	}

	if err := s.syncPinValue2(s.ctx); err != nil {
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

	s.es.GetSync().setNodeUpdatedRemoteToLocal(s.ctx, time.Time{})

	if err := s.sync1(s.ctx); err != nil {
		return err
	}

	if err := s.syncPinValue1(s.ctx); err != nil {
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

	s.NodeLink(s.ctx)
	s.link(true)
	defer s.link(false)

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
		go s.waitRemoteNodeUpdated(ctx)
		go s.waitLocalNodeUpdated(ctx)
		go s.waitRemotePinValueUpdated(ctx)
		go s.waitLocalPinValueUpdated(ctx)
		go s.waitRemotePinWriteUpdated(ctx)
		go s.waitLocalPinWriteUpdated(ctx)
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

func (s *NodeUpService) IsLinked() bool {
	return s.es.GetStatus().GetNodeLink() == consts.ON
}

func (s *NodeUpService) link(value bool) {
	if value {
		s.es.GetStatus().SetNodeLink(consts.ON)
	} else {
		s.es.GetStatus().SetNodeLink(consts.OFF)
	}
}

func (s *NodeUpService) ticker(ctx context.Context) {
	s.closeWG.Add(1)
	defer s.closeWG.Done()

	tokenRefreshTicker := time.NewTicker(s.es.dopts.SyncOptions.TokenRefresh)
	defer tokenRefreshTicker.Stop()

	linkStatusTicker := time.NewTicker(s.es.dopts.SyncOptions.Link)
	defer linkStatusTicker.Stop()

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
		case <-linkStatusTicker.C:
			err := s.NodeLink(ctx)
			if err != nil {
				s.es.Logger().Sugar().Errorf("link node : %v", err)
			} else {
				s.es.Logger().Sugar().Info("link node ticker success")
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

func (s *NodeUpService) ConstServiceClient() nodes.ConstServiceClient {
	return nodes.NewConstServiceClient(s.NodeConn)
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

func (s *NodeUpService) NodeLink(ctx context.Context) error {
	ctx = metadata.SetToken(ctx, s.GetToken())

	request := &nodes.NodeLinkRequest{Status: consts.ON}
	_, err := s.NodeServiceClient().Link(ctx, request)
	if err != nil {
		return err
	}

	return nil
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

	if len(reply.GetToken()) == 0 {
		return errors.New("login: reply token is empty")
	}

	// set token
	s.lock.Lock()
	s.token = reply.GetToken()
	s.lock.Unlock()

	return nil
}

func (s *NodeUpService) waitRemoteNodeUpdated(ctx context.Context) {
	s.closeWG.Add(1)
	defer s.closeWG.Done()

	for {
		ctx := metadata.SetToken(ctx, s.GetToken())

		stream, err := s.SyncServiceClient().WaitNodeUpdated(ctx, &pb.MyEmpty{})
		if err != nil {
			if code, ok := status.FromError(err); ok {
				if code.Code() == codes.Canceled {
					return
				}
			}

			s.es.Logger().Sugar().Errorf("WaitNodeUpdated: %v", err)

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

				s.es.Logger().Sugar().Errorf("WaitNodeUpdated.Recv(): %v", err)
				return
			}

			ctx = metadata.SetToken(ctx, s.GetToken())

			err = s.sync1(ctx)
			if err != nil {
				s.es.Logger().Sugar().Errorf("sync1: %v", err)
			}
		}
	}
}

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

			err := s.sync2(ctx)
			if err != nil {
				s.es.Logger().Sugar().Errorf("sync2: %v", err)
			}
		}
	}
}

func (s *NodeUpService) waitRemotePinValueUpdated(ctx context.Context) {
	s.closeWG.Add(1)
	defer s.closeWG.Done()

	for {
		ctx := metadata.SetToken(ctx, s.GetToken())

		stream, err := s.SyncServiceClient().WaitPinValueUpdated(ctx, &pb.MyEmpty{})
		if err != nil {
			if code, ok := status.FromError(err); ok {
				if code.Code() == codes.Canceled {
					return
				}
			}

			s.es.Logger().Sugar().Errorf("WaitPinValueUpdated: %v", err)

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

				s.es.Logger().Sugar().Errorf("WaitPinValueUpdated.Recv(): %v", err)
				return
			}

			ctx = metadata.SetToken(ctx, s.GetToken())

			err = s.syncPinValue1(ctx)
			if err != nil {
				s.es.Logger().Sugar().Errorf("syncPinValue1: %v", err)
			}
		}
	}
}

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

			err := s.syncPinValue2(ctx)
			if err != nil {
				s.es.Logger().Sugar().Errorf("syncPinValue2: %v", err)
			}
		}
	}
}

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

			err = s.syncPinWrite1(ctx)
			if err != nil {
				s.es.Logger().Sugar().Errorf("syncPinWrite1: %v", err)
			}
		}
	}
}

func (s *NodeUpService) waitLocalPinWriteUpdated(ctx context.Context) {
	s.closeWG.Add(1)
	defer s.closeWG.Done()

	notify := s.es.GetSync().Notify(NOTIFY_PW)
	defer notify.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case <-notify.Wait():
			ctx = metadata.SetToken(ctx, s.GetToken())

			err := s.syncPinWrite2(ctx)
			if err != nil {
				s.es.Logger().Sugar().Errorf("syncPinWrite2: %v", err)
			}
		}
	}
}

func (s *NodeUpService) sync(ctx context.Context) error {
	ctx = metadata.SetToken(ctx, s.GetToken())

	if err := s.sync1(ctx); err != nil {
		return err
	}

	if err := s.sync2(ctx); err != nil {
		return err
	}

	if err := s.syncPinValue1(ctx); err != nil {
		return err
	}

	if err := s.syncPinValue2(ctx); err != nil {
		return err
	}

	if err := s.syncPinWrite1(ctx); err != nil {
		return err
	}

	return s.syncPinWrite2(ctx)
}

func (s *NodeUpService) sync1(ctx context.Context) error {
	return s.syncRemoteToLocal(ctx)
}

func (s *NodeUpService) sync2(ctx context.Context) error {
	return s.syncLocalToRemote(ctx)
}

func (s *NodeUpService) syncPinValue1(ctx context.Context) error {
	return s.syncPinValueRemoteToLocal(ctx)
}

func (s *NodeUpService) syncPinValue2(ctx context.Context) error {
	return s.syncPinValueLocalToRemote(ctx)
}

func (s *NodeUpService) syncPinWrite1(ctx context.Context) error {
	return s.syncPinWriteRemoteToLocal(ctx)
}

func (s *NodeUpService) syncPinWrite2(ctx context.Context) error {
	return s.syncPinWriteLocalToRemote(ctx)
}

func (s *NodeUpService) syncRemoteToLocal(ctx context.Context) error {
	nodeUpdated, err := s.SyncServiceClient().GetNodeUpdated(ctx, &pb.MyEmpty{})
	if err != nil {
		return err
	}

	nodeUpdated2, err := s.es.GetSync().getNodeUpdatedRemoteToLocal(ctx)
	if err != nil {
		return err
	}

	if nodeUpdated.GetUpdated() <= nodeUpdated2.UnixMicro() {
		return nil
	}

	// node
	{
		remote, err := s.NodeServiceClient().ViewWithDeleted(ctx, &pb.MyEmpty{})
		if err != nil {
			return err
		}

		_, err = s.es.GetNode().Sync(ctx, remote)
		if err != nil {
			return err
		}
	}

	// wire
	{
		after := nodeUpdated2.UnixMicro()
		limit := uint32(10)

		for {
			remotes, err := s.WireServiceClient().Pull(ctx, &nodes.WirePullRequest{After: after, Limit: limit})
			if err != nil {
				return err
			}

			for _, remote := range remotes.GetWire() {
				_, err = s.es.GetWire().Sync(ctx, remote)
				if err != nil {
					return err
				}

				after = remote.GetUpdated()
			}

			if len(remotes.GetWire()) < int(limit) {
				break
			}
		}
	}

	// pin
	{
		after := nodeUpdated2.UnixMicro()
		limit := uint32(10)

		for {
			remotes, err := s.PinServiceClient().Pull(ctx, &nodes.PinPullRequest{After: after, Limit: limit})
			if err != nil {
				return err
			}

			for _, remote := range remotes.GetPin() {
				_, err := s.es.GetPin().Sync(ctx, remote)
				if err != nil {
					return err
				}

				after = remote.GetUpdated()
			}

			if len(remotes.GetPin()) < int(limit) {
				break
			}
		}
	}

	// const
	{
		after := nodeUpdated2.UnixMicro()
		limit := uint32(10)

		for {
			remotes, err := s.ConstServiceClient().Pull(ctx, &nodes.ConstPullRequest{After: after, Limit: limit})
			if err != nil {
				return err
			}

			for _, remote := range remotes.GetConst() {
				_, err := s.es.GetConst().Sync(ctx, remote)
				if err != nil {
					return err
				}

				after = remote.GetUpdated()
			}

			if len(remotes.GetConst()) < int(limit) {
				break
			}
		}
	}

	return s.es.GetSync().setNodeUpdatedRemoteToLocal(ctx, time.UnixMicro(nodeUpdated.GetUpdated()))
}

func (s *NodeUpService) syncLocalToRemote(ctx context.Context) error {
	nodeUpdated, err := s.es.GetSync().getNodeUpdated(ctx)
	if err != nil {
		return err
	}

	nodeUpdated2, err := s.es.GetSync().getNodeUpdatedLocalToRemote(ctx)
	if err != nil {
		return err
	}

	if nodeUpdated.UnixMicro() <= nodeUpdated2.UnixMicro() {
		return nil
	}

	// node
	{
		local, err := s.es.GetNode().ViewWithDeleted(ctx, &pb.MyEmpty{})
		if err != nil {
			return err
		}

		_, err = s.NodeServiceClient().Sync(ctx, local)
		if err != nil {
			return err
		}
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

			for _, local := range locals.GetWire() {
				_, err = s.WireServiceClient().Sync(ctx, local)
				if err != nil {
					return err
				}

				after = local.GetUpdated()
			}

			if len(locals.GetWire()) < int(limit) {
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

			for _, local := range locals.GetPin() {
				_, err = s.PinServiceClient().Sync(ctx, local)
				if err != nil {
					return err
				}

				after = local.GetUpdated()
			}

			if len(locals.GetPin()) < int(limit) {
				break
			}
		}
	}

	// const
	{
		after := nodeUpdated2.UnixMicro()
		limit := uint32(10)

		for {
			locals, err := s.es.GetConst().Pull(ctx, &edges.ConstPullRequest{After: after, Limit: limit})
			if err != nil {
				return err
			}

			for _, local := range locals.GetConst() {
				_, err = s.ConstServiceClient().Sync(ctx, local)
				if err != nil {
					return err
				}

				after = local.GetUpdated()
			}

			if len(locals.GetConst()) < int(limit) {
				break
			}
		}
	}

	return s.es.GetSync().setNodeUpdatedLocalToRemote(ctx, nodeUpdated)
}

func (s *NodeUpService) syncPinValueRemoteToLocal(ctx context.Context) error {
	pinValueUpdated, err := s.SyncServiceClient().GetPinValueUpdated(ctx, &pb.MyEmpty{})
	if err != nil {
		return err
	}

	pinValueUpdated2, err := s.es.GetSync().getPinValueUpdatedRemoteToLocal(ctx)
	if err != nil {
		return err
	}

	if pinValueUpdated.GetUpdated() <= pinValueUpdated2.UnixMicro() {
		return nil
	}

	after := pinValueUpdated2.UnixMicro()
	limit := uint32(100)

PULL:
	for {
		remotes, err := s.PinServiceClient().PullValue(ctx, &nodes.PinPullValueRequest{After: after, Limit: limit})
		if err != nil {
			return err
		}

		for _, remote := range remotes.GetPin() {
			if remote.GetUpdated() > pinValueUpdated.GetUpdated() {
				break PULL
			}

			_, err = s.es.GetPin().SyncValue(ctx,
				&pb.PinValue{Id: remote.GetId(), Value: remote.GetValue(), Updated: remote.GetUpdated()})
			if err != nil {
				s.es.Logger().Sugar().Errorf("SyncValue: %v", err)
				return err
			}

			after = remote.GetUpdated()
		}

		if len(remotes.GetPin()) < int(limit) {
			break
		}
	}

	return s.es.GetSync().setPinValueUpdatedRemoteToLocal(ctx, time.UnixMicro(pinValueUpdated.GetUpdated()))
}

func (s *NodeUpService) syncPinValueLocalToRemote(ctx context.Context) error {
	pinValueUpdated, err := s.es.GetSync().getPinValueUpdated(ctx)
	if err != nil {
		return err
	}

	pinValueUpdated2, err := s.es.GetSync().getPinValueUpdatedLocalToRemote(ctx)
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

		for _, local := range locals.GetPin() {
			if local.GetUpdated() > pinValueUpdated.UnixMicro() {
				break PULL
			}

			_, err = s.PinServiceClient().SyncValue(ctx,
				&pb.PinValue{Id: local.GetId(), Value: local.GetValue(), Updated: local.GetUpdated()})
			if err != nil {
				s.es.Logger().Sugar().Errorf("SyncValue: %v", err)
				return err
			}

			after = local.GetUpdated()
		}

		if len(locals.GetPin()) < int(limit) {
			break
		}
	}

	return s.es.GetSync().setPinValueUpdatedLocalToRemote(ctx, pinValueUpdated)
}

func (s *NodeUpService) syncPinWriteRemoteToLocal(ctx context.Context) error {
	pinWriteUpdated, err := s.SyncServiceClient().GetPinWriteUpdated(ctx, &pb.MyEmpty{})
	if err != nil {
		return err
	}

	pinWriteUpdated2, err := s.es.GetSync().getPinWriteUpdatedRemoteToLocal(ctx)
	if err != nil {
		return err
	}

	if pinWriteUpdated.GetUpdated() <= pinWriteUpdated2.UnixMicro() {
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

		for _, remote := range remotes.GetPin() {
			if remote.GetUpdated() > pinWriteUpdated.GetUpdated() {
				break PULL
			}

			_, err = s.es.GetPin().SyncWrite(ctx,
				&pb.PinValue{Id: remote.GetId(), Value: remote.GetValue(), Updated: remote.GetUpdated()})
			if err != nil {
				s.es.Logger().Sugar().Errorf("SyncWrite: %v", err)
				return err
			}

			after = remote.GetUpdated()
		}

		if len(remotes.GetPin()) < int(limit) {
			break
		}
	}

	return s.es.GetSync().setPinWriteUpdatedRemoteToLocal(ctx, time.UnixMicro(pinWriteUpdated.GetUpdated()))
}

func (s *NodeUpService) syncPinWriteLocalToRemote(ctx context.Context) error {
	pinWriteUpdated, err := s.es.GetSync().getPinWriteUpdated(ctx)
	if err != nil {
		return err
	}

	pinWriteUpdated2, err := s.es.GetSync().getPinWriteUpdatedLocalToRemote(ctx)
	if err != nil {
		return err
	}

	if pinWriteUpdated.UnixMicro() <= pinWriteUpdated2.UnixMicro() {
		return nil
	}

	after := pinWriteUpdated2.UnixMicro()
	limit := uint32(100)

PULL:
	for {
		locals, err := s.es.GetPin().PullWrite(ctx, &edges.PinPullValueRequest{After: after, Limit: limit})
		if err != nil {
			return err
		}

		for _, local := range locals.GetPin() {
			if local.GetUpdated() > pinWriteUpdated.UnixMicro() {
				break PULL
			}

			_, err = s.PinServiceClient().SyncWrite(ctx,
				&pb.PinValue{Id: local.GetId(), Value: local.GetValue(), Updated: local.GetUpdated()})
			if err != nil {
				s.es.Logger().Sugar().Errorf("SyncWrite: %v", err)
				return err
			}

			after = local.GetUpdated()
		}

		if len(locals.GetPin()) < int(limit) {
			break
		}
	}

	return s.es.GetSync().setPinWriteUpdatedLocalToRemote(ctx, pinWriteUpdated)
}
