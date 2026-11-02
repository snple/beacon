package core

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"time"

	"github.com/snple/beacon/packet"
)

// listenerEntry 是已注册监听器的元信息。
type listenerEntry struct {
	address   string
	protocol  string
	startedAt time.Time
}

// ListenerInfo 是监听器的公开视图。
type ListenerInfo struct {
	Address   string
	Protocol  string
	StartedAt time.Time
}

// ServerInfo 是服务端总体信息。
type ServerInfo struct {
	ProtocolVersion uint8
	StartedAt       time.Time
	Uptime          time.Duration
	MaxClients      uint32
	Connected       int64
	Sessions        int64
	Listeners       []ListenerInfo
	Limits          packet.Limits
	Storage         string
}

func (c *Core) registerListener(address, protocol string) uint64 {
	c.listenersMu.Lock()
	defer c.listenersMu.Unlock()

	c.listenerSeq++
	id := c.listenerSeq
	if c.listeners == nil {
		c.listeners = make(map[uint64]listenerEntry)
	}
	c.listeners[id] = listenerEntry{address: address, protocol: protocol, startedAt: time.Now()}
	return id
}

func (c *Core) unregisterListener(id uint64) {
	c.listenersMu.Lock()
	delete(c.listeners, id)
	c.listenersMu.Unlock()
}

// Listeners 返回当前监听器列表。
func (c *Core) Listeners() []ListenerInfo {
	c.listenersMu.Lock()
	defer c.listenersMu.Unlock()

	out := make([]ListenerInfo, 0, len(c.listeners))
	for _, e := range c.listeners {
		out = append(out, ListenerInfo{Address: e.address, Protocol: e.protocol, StartedAt: e.startedAt})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Address < out[j].Address })
	return out
}

// Ready 报告 core 是否在运行。
func (c *Core) Ready() bool { return c.running.Load() }

// limits 返回协议层限制。
func (c *Core) limits() packet.Limits {
	return packet.Limits{
		MaxPacketSize:        c.options.MaxPacketSize,
		MaxRecords:           c.options.MaxRecords,
		MaxWaitMs:            c.options.MaxWaitMs,
		MaxBatch:             c.options.MaxBytes,
		MaxChannelsPerClient: uint16(min(c.options.MaxChannelsPerClient, 65535)),
	}
}

// Info 返回服务端总体信息。
func (c *Core) Info() ServerInfo {
	c.clientsMu.RLock()
	sessions := int64(len(c.clients))
	c.clientsMu.RUnlock()

	return ServerInfo{
		ProtocolVersion: packet.ProtocolVersion,
		StartedAt:       c.startedAt,
		Uptime:          time.Since(c.startedAt),
		MaxClients:      c.options.MaxClients,
		Connected:       c.stats.ClientsConnected.Load(),
		Sessions:        sessions,
		Listeners:       c.Listeners(),
		Limits:          c.limits(),
		Storage:         "memory",
	}
}

// ClientSummary 是客户端列表的轻量概要。
type ClientSummary struct {
	ClientID    string
	Connected   bool
	RemoteAddr  string
	ConnectedAt time.Time
	LastSeenAt  time.Time
	ExpiresAt   time.Time
	Pending     [3]uint32
	Delivered   uint32
}

// ListClients 返回全部会话概要（在线 + 离线保留），按 ClientID 字典序。
func (c *Core) ListClients() []ClientSummary {
	c.clientsMu.RLock()
	clients := make([]*Client, 0, len(c.clients))
	for _, cl := range c.clients {
		clients = append(clients, cl)
	}
	c.clientsMu.RUnlock()

	out := make([]ClientSummary, 0, len(clients))
	for _, cl := range clients {
		pending, delivered, _ := cl.mb.depth()
		summary := ClientSummary{
			ClientID:  cl.ID,
			Connected: cl.currentConn() != nil,
			Pending:   pending,
			Delivered: delivered,
			ExpiresAt: cl.expiry(),
		}
		if ms := cl.connectedAt.Load(); ms > 0 {
			summary.ConnectedAt = time.UnixMilli(ms)
		}
		if ms := cl.lastSeen.Load(); ms > 0 {
			summary.LastSeenAt = time.UnixMilli(ms)
		}
		if cn := cl.currentConn(); cn != nil {
			summary.RemoteAddr = cn.remoteAddr
		}
		out = append(out, summary)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].ClientID < out[j].ClientID })
	return out
}

// ClientInfo 是单个客户端的详情。
type ClientInfo struct {
	ClientID          string
	Connected         bool
	RemoteAddr        string
	ClientSessionID   uint64
	CreatedAt         time.Time
	ConnectedAt       time.Time
	LastSeenAt        time.Time
	ExpiresAt         time.Time
	SessionTimeout    uint32
	Pending           [3]uint32
	Delivered         uint32
	OldestUncommitted packet.PacketID
	Channels          []string
	Identity          *Identity
	MessagesSent      int64
	MessagesDropped   int64
	MessagesRejected  int64
}

// GetClientInfo 返回单个客户端详情。
func (c *Core) GetClientInfo(clientID string) (*ClientInfo, error) {
	c.clientsMu.RLock()
	cl := c.clients[clientID]
	c.clientsMu.RUnlock()
	if cl == nil {
		return nil, &ClientNotFoundError{ClientID: clientID}
	}

	pending, delivered, oldest := cl.mb.depth()

	info := &ClientInfo{
		ClientID:          cl.ID,
		Connected:         cl.currentConn() != nil,
		ClientSessionID:   cl.sessionID.Load(),
		CreatedAt:         cl.createdAt,
		ExpiresAt:         cl.expiry(),
		SessionTimeout:    cl.sessionTimeout(),
		Pending:           pending,
		Delivered:         delivered,
		OldestUncommitted: oldest,
		Channels:          c.matcher.filters(cl.ID),
		Identity:          cl.getIdentity(),
		MessagesSent:      cl.sent.Load(),
		MessagesDropped:   cl.dropped.Load(),
		MessagesRejected:  cl.rejected.Load(),
	}
	if ms := cl.connectedAt.Load(); ms > 0 {
		info.ConnectedAt = time.UnixMilli(ms)
	}
	if ms := cl.lastSeen.Load(); ms > 0 {
		info.LastSeenAt = time.UnixMilli(ms)
	}
	if cn := cl.currentConn(); cn != nil {
		info.RemoteAddr = cn.remoteAddr
	}
	return info, nil
}

// DisconnectClient 断开指定客户端的当前连接。
//
// reason 会通过 DISCONNECT 发给客户端；reason=ReasonKicked 时客户端不应自动重连。
func (c *Core) DisconnectClient(clientID string, reason packet.ReasonCode) error {
	c.clientsMu.RLock()
	cl := c.clients[clientID]
	c.clientsMu.RUnlock()
	if cl == nil {
		return &ClientNotFoundError{ClientID: clientID}
	}
	cn := cl.currentConn()
	if cn == nil {
		return ErrClientNotConnected
	}
	cn.terminate(reason, true)
	return nil
}

// DisconnectClientAndWait 断开并等待连接彻底关闭。
func (c *Core) DisconnectClientAndWait(ctx context.Context, clientID string, reason packet.ReasonCode) error {
	c.clientsMu.RLock()
	cl := c.clients[clientID]
	c.clientsMu.RUnlock()
	if cl == nil {
		return &ClientNotFoundError{ClientID: clientID}
	}
	cn := cl.currentConn()
	if cn == nil {
		return ErrClientNotConnected
	}
	cn.terminate(reason, true)

	done := make(chan struct{})
	go func() {
		cn.close()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// PushOptions 服务端推送选项。
//
// Priority 的零值为 Low（尽力而为）。不显式设置时按 Medium 处理；
// 需要 Low 时请使用 WithPriority(packet.PriorityLow)。
type PushOptions struct {
	Priority packet.Priority
	Headers  map[string]string

	prioritySet bool
}

// WithPriority 显式设置优先级（允许显式指定 Low）。
func (o PushOptions) WithPriority(p packet.Priority) PushOptions {
	o.Priority = p
	o.prioritySet = true
	return o
}

// WithHeaders 设置自定义元数据（会被拷贝，调用方可复用 map）。
func (o PushOptions) WithHeaders(h map[string]string) PushOptions {
	o.Headers = h
	return o
}

func (o PushOptions) effectivePriority() packet.Priority {
	if o.prioritySet {
		return o.Priority
	}
	if o.Priority != packet.PriorityLow {
		return o.Priority
	}
	return packet.PriorityMedium
}

// PushResult 服务端推送结果。
type PushResult struct {
	ID      packet.PacketID
	Reason  packet.ReasonCode
	Matched uint32
	Dropped uint32
}

// Push 以系统身份向 channel 推送一条消息（不做写权限校验）。
func (c *Core) Push(channel string, payload []byte, opts PushOptions) PushResult {
	res, _ := c.pushTo("", channel, payload, opts)
	return res
}

// PushToClient 向指定客户端定向投递一条消息。
func (c *Core) PushToClient(clientID, channel string, payload []byte, opts PushOptions) (PushResult, error) {
	return c.pushTo(clientID, channel, payload, opts)
}

// pushTo 是服务端推送的内部实现。target 为空表示普通 channel 发布。
//
// 因为消息会保留在队列中被多个会话共享，payload 与 headers 都做一次拷贝，
// 避免调用方后续修改导致队列内容变化。
func (c *Core) pushTo(target, channel string, payload []byte, opts PushOptions) (PushResult, error) {
	rec := &packet.Record{
		Channel: channel,
		Target:  target,
		Headers: maps.Clone(opts.Headers),
		Payload: append([]byte(nil), payload...),
	}
	rec.SetPriority(opts.effectivePriority())

	res := c.pushRecord(nil, rec, true)
	out := PushResult{ID: res.ID, Reason: res.Reason, Matched: res.Matched, Dropped: res.Dropped}

	switch res.Reason {
	case packet.ReasonTargetNotFound:
		return out, &ClientNotFoundError{ClientID: target}
	case packet.ReasonChannelInvalid, packet.ReasonPacketTooLarge, packet.ReasonNotAuthorized:
		return out, fmt.Errorf("push rejected: %s", res.Reason)
	default:
		return out, nil
	}
}
