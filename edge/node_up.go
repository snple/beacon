package edge

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/snple/beacon/dt"
	"github.com/snple/beacon/pb"
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
// 新的同步流程:
// 1. 调用 Core NodeService.Push 清除 Core 端的配置数据（Wire/Pin 等）
// 2. 使用 WireService.Push 流式发送所有 Wire 数据
// 3. 使用 PinService.Push 流式发送所有 Pin 数据
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

	// 获取节点配置并导出为 NSON
	_, err = s.es.GetStorage().GetNode()
	if err != nil {
		return err
	}

	// 导出配置为 NSON 字节
	data, err := s.es.GetStorage().ExportConfig()
	if err != nil {
		return err
	}

	// 推送节点配置到 Core
	_, err = s.NodeServiceClient().Push(ctx, &nodes.NodePushRequest{
		Nson: data,
	})
	if err != nil {
		return err
	}

	return s.es.GetSync().setNodeToRemote(ctx, nodeUpdated)
}

// syncPinValueToRemote: Edge PinValue 同步到 Core
// 使用 PushValue 流式方法发送 PinValue 数据
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

	// 创建 PushValue 流
	stream, err := s.PinServiceClient().PushValue(ctx)
	if err != nil {
		return err
	}

	after := pinValueUpdated2
	limit := 100

	// 从本地存储读取 PinValue 并发送
	for {
		items, err := s.es.GetStorage().ListPinValues(after, limit)
		if err != nil {
			return err
		}

		for _, item := range items {
			if item.Updated.After(pinValueUpdated) {
				goto DONE
			}

			// 使用 dt.DecodeNsonValue 反序列化
			var value *pb.NsonValue
			if len(item.Value) > 0 {
				var err error
				value, err = dt.DecodeNsonValue(item.Value)
				if err != nil {
					s.es.Logger().Sugar().Errorf("DecodeNsonValue: %v", err)
					continue
				}
			}

			err = stream.Send(&pb.PinValue{
				Id:      item.ID,
				Value:   value,
				Updated: item.Updated.UnixMicro(),
			})
			if err != nil {
				s.es.Logger().Sugar().Errorf("PushValue Send: %v", err)
				return err
			}

			after = item.Updated
		}

		if len(items) < limit {
			break
		}
	}

DONE:
	if _, err := stream.CloseAndRecv(); err != nil {
		return err
	}

	return s.es.GetSync().setPinValueToRemote(ctx, pinValueUpdated)
}

// syncPinWriteFromRemote: 从 Core 拉取 PinWrite 写命令
// 使用 PullWrite 服务端流式方法接收 PinWrite 数据
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

	// 从 Core 拉取 PinWrite 数据
	stream, err := s.PinServiceClient().PullWrite(ctx, &nodes.PinPullWriteRequest{
		After: after,
		Limit: limit,
	})
	if err != nil {
		return err
	}

	for {
		remote, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if remote.Updated > pinWriteUpdated.Updated {
			break
		}

		// 保存到本地存储
		pin, err := s.es.GetStorage().GetPinByID(remote.Id)
		if err != nil {
			s.es.Logger().Sugar().Errorf("GetPinByID: %v", err)
			continue
		}

		err = s.es.GetStorage().SetPinWrite(ctx, remote.Id, remote.Value, time.UnixMicro(remote.Updated))
		if err != nil {
			s.es.Logger().Sugar().Errorf("SetPinWrite: %v", err)
			return err
		}

		err = s.es.GetPin().afterUpdateWrite(ctx, pin, remote.Value)
		if err != nil {
			s.es.Logger().Sugar().Errorf("afterUpdateWrite: %v", err)
			return err
		}

		after = remote.Updated
	}

	return s.es.GetSync().setPinWriteFromRemote(ctx, time.UnixMicro(pinWriteUpdated.GetUpdated()))
}
