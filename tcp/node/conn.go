package node

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/consts"
	"github.com/snple/beacon/core"
	"github.com/snple/beacon/pb"
)

type Conn struct {
	ns *NodeService

	net.Conn

	id string

	readWatchEvent  *WatchEvent
	writeWatchEvent *WatchEvent
	lock            sync.RWMutex

	ctx     context.Context
	cancel  context.CancelCauseFunc
	closeWG sync.WaitGroup
}

func NewConn(ns *NodeService, conn net.Conn) (*Conn, error) {
	ctx, cancel := context.WithCancelCause(ns.Context())

	c := &Conn{
		ns:   ns,
		Conn: conn,

		ctx:    ctx,
		cancel: cancel,
	}

	if err := c.auth(); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Conn) Close() error {
	c.cancel(errors.New("close"))
	c.closeWG.Wait()

	// send close message
	msg := nson.Map{
		"fn": nson.String("close"),
	}
	nson.WriteMap(c.Conn, msg)

	return c.Conn.Close()
}

func (c *Conn) ID() string {
	return c.id
}

func (c *Conn) auth() error {
	c.Conn.SetReadDeadline(time.Now().Add(time.Second * 10))

	req, err := nson.ReadMap(c.Conn)
	if err != nil {
		return err
	}

	c.Conn.SetReadDeadline(time.Time{})

	fn, err := req.GetString("fn")
	if err != nil {
		writeError(c.Conn, err)

		return err
	}

	if fn != "auth" {
		writeError(c.Conn, errors.New("invalid request, fn != auth"))

		return errors.New("invalid request")
	}
	var node *pb.Node

	if req.Contains("id") {
		nodeId, err := req.GetString("id")
		if err != nil {
			writeError(c.Conn, err)

			return err
		}

		node, err = c.ns.Core().GetNode().View(c.ns.Context(), &pb.Id{Id: nodeId})
		if err != nil {
			writeError(c.Conn, err)

			return err
		}
	} else if req.Contains("name") {
		nodeName, err := req.GetString("name")
		if err != nil {
			writeError(c.Conn, err)

			return err
		}

		node, err = c.ns.Core().GetNode().Name(c.ns.Context(), &pb.Name{Name: nodeName})
		if err != nil {
			writeError(c.Conn, err)

			return err
		}
	} else {
		writeError(c.Conn, errors.New("invalid request, id or name is required"))

		return errors.New("invalid request")
	}

	if node.Status != consts.ON {
		writeError(c.Conn, errors.New("invalid request, node is not enable"))

		return errors.New("invalid request")
	}

	secret, err := req.GetString("secret")
	if err != nil {
		writeError(c.Conn, err)

		return err
	}

	_ = secret

	// if node.Secret != secret {
	// 	writeError(c.Conn, errors.New("invalid request, secret is not valid"))

	// 	return errors.New("invalid request")
	// }

	c.id = node.Id

	resp := nson.Map{
		"fn":   nson.String("auth"),
		"code": nson.I32(0),
	}

	return nson.WriteMap(c.Conn, resp)
}

func (c *Conn) handle() {
	for {
		req, err := nson.ReadMap(c.Conn)
		if err != nil {
			return
		}

		fn, err := req.GetString("fn")
		if err != nil {
			writeError(c.Conn, err)

			return
		}

		switch fn {
		case "close":
			return
		case "ping":
			c.handlePing()
		case "get_value":
			c.handleGetValue(req)
		case "set_value":
			c.handleSetValue(req)
		case "get_write":
			c.handleGetWrite(req)
		case "set_write":
			c.handleSetWrite(req)
		case "watch_value":
			c.handleWatchValue(req)
		case "watch_write":
			c.handleWatchWrite(req)
		case "push":
			c.handlePush(req)
		default:
			writeError(c.Conn, errors.New("invalid request, fn not found"))

			return
		}
	}
}

func (c *Conn) handlePing() {
	msg := nson.Map{
		"fn": nson.String("pong"),
	}
	nson.WriteMap(c.Conn, msg)
}

func (c *Conn) handleGetValue(_ nson.Map) error {
	return nil
}

func (c *Conn) handleSetValue(req nson.Map) error {
	// values, err := req.GetMap("values")
	// if err != nil {
	// 	writeError(c.Conn, err)

	// 	return nil
	// }

	errors := nson.Map{}

	// for name, value := range values {
	// 	valueStr, err := dt.EncodeNsonValue(value)
	// 	if err != nil {
	// 		errors[name] = nson.String(err.Error())
	// 		continue
	// 	}

	// 	_, err = c.ns.Core().GetPin().SetValueByName(c.ctx,
	// 		&cores.PinNameValue{NodeId: c.id, Name: name, Value: valueStr})
	// 	if err != nil {
	// 		errors[name] = nson.String(err.Error())
	// 	}
	// }

	if len(errors) > 0 {
		msg := nson.Map{
			"fn":     nson.String("set_value"),
			"code":   nson.I32(-1),
			"errors": errors,
		}
		return nson.WriteMap(c.Conn, msg)
	}

	msg := nson.Map{
		"fn":   nson.String("set_value"),
		"code": nson.I32(0),
	}
	return nson.WriteMap(c.Conn, msg)
}

func (c *Conn) handleGetWrite(_ nson.Map) error {
	return nil
}

func (c *Conn) handleSetWrite(_ nson.Map) error {
	return nil
}

func (c *Conn) handleWatchValue(_ nson.Map) error {
	return nil
}

func (c *Conn) handleWatchWrite(req nson.Map) error {
	pinNames, err := req.GetArray("pins")
	if err != nil {
		writeError(c.Conn, err)

		return nil
	}

	if len(pinNames) == 0 {

	}

	c.lock.Lock()
	defer c.lock.Unlock()

	if c.writeWatchEvent == nil {
		c.writeWatchEvent = NewWatchEvent(c, core.NOTIFY_PW, pinNames)
	} else {
		c.writeWatchEvent.pinNames = make(map[string]struct{})
		for _, pinName := range pinNames {
			if pinNameStr, ok := pinName.(nson.String); ok {
				c.writeWatchEvent.pinNames[string(pinNameStr)] = struct{}{}
			}
		}
	}

	return nil
}

func (c *Conn) handlePush(_ nson.Map) error {
	return nil
}

type WatchEvent struct {
	conn       *Conn
	notifyType core.NotifyType

	pinNames map[string]struct{}
	values   nson.Map
	lock     sync.Mutex
}

func NewWatchEvent(conn *Conn, notifyType core.NotifyType, pinNames nson.Array) *WatchEvent {
	pinNamesMap := make(map[string]struct{})
	for _, pinName := range pinNames {
		if pinNameStr, ok := pinName.(nson.String); ok {
			pinNamesMap[string(pinNameStr)] = struct{}{}
		}
	}

	w := &WatchEvent{
		conn:       conn,
		notifyType: notifyType,
		pinNames:   pinNamesMap,
		values:     nson.Map{},
	}

	// go w.watch()

	return w
}

// func (w *WatchEvent) watch() {
// 	w.conn.closeWG.Add(1)
// 	defer w.conn.closeWG.Done()

// 	notify := w.conn.ns.Core().GetSync().Notify(w.conn.id, w.notifyType)
// 	defer notify.Close()

// 	after := int64(0)
// 	limit := uint32(100)

// 	type pinCacheKey struct {
// 		wireId string
// 		pinId  string
// 	}

// 	type pinCacheValue struct {
// 		wireName string
// 		pinName  string
// 		pinType  string
// 	}

// 	pinCache := cache.NewCache(func(ctx context.Context, key pinCacheKey) (pinCacheValue, time.Duration, error) {
// 		wireReply, err := w.conn.ns.Core().GetWire().View(ctx, &pb.Id{Id: key.wireId})
// 		if err != nil {
// 			return pinCacheValue{}, 0, err
// 		}

// 		pinReply, err := w.conn.ns.Core().GetPin().View(ctx, &pb.Id{Id: key.pinId})
// 		if err != nil {
// 			return pinCacheValue{}, 0, err
// 		}

// 		return pinCacheValue{
// 			wireName: wireReply.Name,
// 			pinName:  pinReply.Name,
// 			pinType:  pinReply.Type,
// 		}, time.Second * 60, nil
// 	})

// 	for {
// 		select {
// 		case <-w.conn.ctx.Done():
// 			return
// 		case <-notify.Wait():
// 			var remotes *cores.PinPullValueResponse
// 			var err error

// 			if w.notifyType == core.NOTIFY_PW {
// 				remotes, err = w.conn.ns.Core().GetPin().PullWrite(w.conn.ctx, &cores.PinPullValueRequest{After: after, Limit: limit, NodeId: w.conn.id})
// 			} else {
// 				remotes, err = w.conn.ns.Core().GetPin().PullValue(w.conn.ctx, &cores.PinPullValueRequest{After: after, Limit: limit, NodeId: w.conn.id})
// 			}

// 			if err != nil {
// 				w.conn.ns.Logger().Error("failed to pull pin write", zap.Error(err))
// 				continue
// 			}

// 			for _, pin := range remotes.Pins {
// 				after = pin.Updated

// 				option, err := pinCache.GetWithMiss(w.conn.ctx, pinCacheKey{wireId: pin.WireId, pinId: pin.Id})
// 				if err != nil {
// 					w.conn.ns.Logger().Error("failed to get pin cache", zap.Error(err))
// 					continue
// 				}

// 				if option.IsSome() {
// 					pinCacheValue := option.Unwrap()

// 					pinName := fmt.Sprintf("%s.%s", pinCacheValue.wireName, pinCacheValue.pinName)
// 					if _, ok := w.pinNames[pinName]; !ok {
// 						continue
// 					}

// 					tag, err := dt.ParseTypeTag(pinCacheValue.pinType)
// 					if err != nil {
// 						w.conn.ns.Logger().Error("failed to parse pin type", zap.Error(err))
// 						continue
// 					}

// 					value, err := dt.DecodeNsonValue(pin.Value, tag)
// 					if err != nil {
// 						w.conn.ns.Logger().Error("failed to decode pin value", zap.Error(err))
// 						continue
// 					}

// 					w.lock.Lock()
// 					w.values[pinCacheValue.pinName] = value
// 					w.lock.Unlock()
// 				}
// 			}
// 		}
// 	}
// }
