package core

import (
	"bytes"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/snple/beacon/internal/mq"
	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// 会话元数据持久化。
//
// 消息队列本身已经持久化在 "client:<clientID>:" 前缀下，但订阅、会话超时等元数据
// 此前只存在于内存中：进程重启后 sessionPresent 变假、订阅丢失，而旧队列仍会被投递。
// 这里把会话元数据单独存储，使"会话状态"和"待投递消息"的生命周期保持一致。
const sessionKeyPrefix = "session:"

// 会话记录持久化格式：
//
//	[version(1)][ClientID][Timeout(4)][ExpiresAt(8)][SubCount(4)][订阅项...]
//
// 订阅项：[Topic][options(1)]，options: bit0=QoS, bit1=NoLocal,
// bit2=RetainAsPublished, bit3-4=RetainHandling。
const sessionRecordFormatVersion byte = 1

var (
	_ mq.BinaryMarshaler   = (*sessionRecord)(nil)
	_ mq.BinaryUnmarshaler = (*sessionRecord)(nil)
)

// sessionSubscription 可持久化的订阅项。
type sessionSubscription struct {
	Topic             string
	QoS               uint8
	NoLocal           bool
	RetainAsPublished bool
	RetainHandling    uint8
}

// sessionRecord 会话元数据。
// ExpiresAt: 0 表示会话当前在线（不设过期）；>0 表示离线后的过期时间（Unix 秒）。
// 遗嘱消息不持久化：它属于单次连接，重连时由新的 CONNECT 提供。
// 认证数据（AuthMethod/AuthData）也不持久化：认证只在建立连接时进行。
type sessionRecord struct {
	ClientID      string
	Timeout       uint32
	ExpiresAt     int64
	Subscriptions []sessionSubscription
}

// MarshalBinary 将会话记录编码为固定二进制格式（不使用反射式序列化）。
func (r *sessionRecord) MarshalBinary() ([]byte, error) {
	if r == nil {
		return nil, fmt.Errorf("nil session record")
	}

	var buf bytes.Buffer
	buf.WriteByte(sessionRecordFormatVersion)

	if err := packet.EncodeString(&buf, r.ClientID); err != nil {
		return nil, err
	}
	if err := packet.EncodeUint32(&buf, r.Timeout); err != nil {
		return nil, err
	}
	if err := packet.EncodeInt64(&buf, r.ExpiresAt); err != nil {
		return nil, err
	}
	// 订阅数量上限由数据长度决定；写 uint32 以免给合法会话设置人为上限。
	if uint64(len(r.Subscriptions)) > uint64(^uint32(0)) {
		return nil, fmt.Errorf("too many subscriptions: %d", len(r.Subscriptions))
	}
	if err := packet.EncodeUint32(&buf, uint32(len(r.Subscriptions))); err != nil {
		return nil, err
	}

	for _, sub := range r.Subscriptions {
		if err := packet.EncodeString(&buf, sub.Topic); err != nil {
			return nil, err
		}
		var opts byte
		opts |= sub.QoS & 0x01
		if sub.NoLocal {
			opts |= 0x02
		}
		if sub.RetainAsPublished {
			opts |= 0x04
		}
		opts |= (sub.RetainHandling & 0x03) << 3
		if err := packet.WriteByte(&buf, opts); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

// UnmarshalBinary 从固定二进制格式解码会话记录。
func (r *sessionRecord) UnmarshalBinary(data []byte) error {
	rd := bytes.NewReader(data)

	version, err := rd.ReadByte()
	if err != nil {
		return err
	}
	if version != sessionRecordFormatVersion {
		return fmt.Errorf("unsupported session record version %d", version)
	}

	r.ClientID, err = packet.DecodeString(rd)
	if err != nil {
		return err
	}
	r.Timeout, err = packet.DecodeUint32(rd)
	if err != nil {
		return err
	}
	r.ExpiresAt, err = packet.DecodeInt64(rd)
	if err != nil {
		return err
	}

	count, err := packet.DecodeUint32(rd)
	if err != nil {
		return err
	}
	// 每个订阅项至少 3 字节（Topic 长度前缀 2 + options 1），
	// 先按剩余字节校验，避免畸形数据触发大切片预分配。
	if uint64(count)*3 > uint64(rd.Len()) {
		return fmt.Errorf("invalid subscription count %d", count)
	}

	r.Subscriptions = make([]sessionSubscription, 0, count)
	for range count {
		topic, err := packet.DecodeString(rd)
		if err != nil {
			return err
		}
		opts, err := packet.ReadByte(rd)
		if err != nil {
			return err
		}
		r.Subscriptions = append(r.Subscriptions, sessionSubscription{
			Topic:             topic,
			QoS:               opts & 0x01,
			NoLocal:           opts&0x02 != 0,
			RetainAsPublished: opts&0x04 != 0,
			RetainHandling:    (opts >> 3) & 0x03,
		})
	}

	if rd.Len() != 0 {
		return fmt.Errorf("trailing bytes in session record")
	}

	return nil
}

func sessionKey(clientID string) []byte {
	return []byte(sessionKeyPrefix + clientID)
}

func newSessionRecord(client *Client, expiresAt int64) *sessionRecord {
	rec := &sessionRecord{
		ClientID:  client.ID,
		Timeout:   client.session.timeout.Load(),
		ExpiresAt: expiresAt,
	}

	client.session.subsMu.RLock()
	for topic, opts := range client.session.subscriptions {
		rec.Subscriptions = append(rec.Subscriptions, sessionSubscription{
			Topic:             topic,
			QoS:               uint8(opts.QoS),
			NoLocal:           opts.NoLocal,
			RetainAsPublished: opts.RetainAsPublished,
			RetainHandling:    opts.RetainHandling,
		})
	}
	client.session.subsMu.RUnlock()

	return rec
}

// saveSession 持久化会话元数据（expiresAt=0 表示在线）。
func (c *Core) saveSession(client *Client, expiresAt int64) {
	if c.store == nil || client == nil || client.session == nil {
		return
	}

	data, err := mq.EncodeMessage(newSessionRecord(client, expiresAt))
	if err != nil {
		c.logger.Warn("Failed to encode session record",
			zap.String("clientID", client.ID),
			zap.Error(err))
		return
	}

	if err := c.store.db().Update(func(txn *badger.Txn) error {
		return txn.Set(sessionKey(client.ID), data)
	}); err != nil {
		c.logger.Warn("Failed to persist session record",
			zap.String("clientID", client.ID),
			zap.Error(err))
	}
}

// saveSessionExpiring 持久化离线会话及其过期时间。
func (c *Core) saveSessionExpiring(client *Client, expiresAt time.Time) {
	c.saveSession(client, expiresAt.Unix())
}

// deleteSession 删除会话元数据。
func (c *Core) deleteSession(clientID string) {
	if c.store == nil {
		return
	}
	if err := c.store.db().Update(func(txn *badger.Txn) error {
		return txn.Delete(sessionKey(clientID))
	}); err != nil {
		c.logger.Warn("Failed to delete session record",
			zap.String("clientID", clientID),
			zap.Error(err))
	}
}

// loadSessionRecords 读取全部会话元数据。
func (c *Core) loadSessionRecords() ([]*sessionRecord, error) {
	if c.store == nil {
		return nil, nil
	}

	var records []*sessionRecord
	prefix := []byte(sessionKeyPrefix)

	err := c.store.db().View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()

			var rec sessionRecord
			if err := item.Value(func(val []byte) error {
				return mq.DecodeMessage(&rec, val)
			}); err != nil {
				c.logger.Warn("Failed to decode session record",
					zap.String("key", string(item.Key())),
					zap.Error(err))
				continue
			}
			if rec.ClientID == "" {
				continue
			}
			records = append(records, &rec)
		}
		return nil
	})

	return records, err
}

// recoverSessions 在启动时恢复会话（订阅 + 离线过期时间），并清理过期会话与孤儿队列。
func (c *Core) recoverSessions() {
	if c.store == nil {
		return
	}

	now := time.Now()
	records, err := c.loadSessionRecords()
	if err != nil {
		c.logger.Error("Failed to load sessions", zap.Error(err))
		return
	}

	restored := 0
	for _, rec := range records {
		// 旧版本可能持久化了含 ':'/控制字符的 ID；这类 ID 会让队列 key 前缀
		// 互相包含。直接丢弃会话记录，其队列随后由孤儿清理删除。
		if !packet.ValidateClientID(rec.ClientID) {
			c.logger.Warn("Dropping persisted session with invalid client ID",
				zap.String("clientID", rec.ClientID))
			c.deleteSession(rec.ClientID)
			continue
		}

		expiresAt := rec.ExpiresAt

		// ExpiresAt=0 说明上次进程退出时连接仍在线：按"现在断开"重新计算过期时间。
		if expiresAt == 0 {
			if rec.Timeout == 0 {
				c.deleteSession(rec.ClientID)
				continue
			}
			expiresAt = now.Add(time.Duration(rec.Timeout) * time.Second).Unix()
			rec.ExpiresAt = expiresAt
			c.saveRecord(rec)
		}

		if expiresAt <= now.Unix() {
			// 停机期间会话已过期：删除记录，队列由孤儿清理删除
			c.deleteSession(rec.ClientID)
			continue
		}

		client := &Client{
			ID:   rec.ClientID,
			core: c,
			session: &session{
				id:            rec.ClientID,
				subscriptions: make(map[string]packet.SubscribeOptions, len(rec.Subscriptions)),
				pendingAck:    make(map[packet.PacketID]pendingMessage),
				core:          c,
			},
		}
		client.session.timeout.Store(rec.Timeout)
		// 恢复的会话不是通过 newClient 构造的，必须补上限流器，
		// 否则重连后 MaxPublishRate 对该客户端失效。
		client.initPublishLimiter()

		client.initQueue()

		for _, sub := range rec.Subscriptions {
			opts := packet.SubscribeOptions{
				QoS:               packet.QoS(sub.QoS),
				NoLocal:           sub.NoLocal,
				RetainAsPublished: sub.RetainAsPublished,
				RetainHandling:    sub.RetainHandling,
			}
			client.session.subscriptions[sub.Topic] = opts
			c.addSubscription(client.ID, packet.Subscription{Topic: sub.Topic, Options: opts})
		}

		c.clientsMu.Lock()
		c.clients[rec.ClientID] = client
		c.clientsMu.Unlock()

		c.offlineSessionsMu.Lock()
		c.offlineSessions[rec.ClientID] = time.Unix(expiresAt, 0)
		c.offlineSessionsMu.Unlock()

		restored++
	}

	// 没有对应会话的客户端队列（崩溃残留、CleanStart、旧版本遗留）一并清理
	c.cleanupOrphanClientQueues()

	// 会话数超过 MaxOfflineSessions 时收敛到上限内
	if evicted := c.trimOfflineSessions(); evicted > 0 {
		c.logger.Warn("Trimmed restored offline sessions to MaxOfflineSessions",
			zap.Int("evicted", evicted),
			zap.Int("limit", c.options.MaxOfflineSessions))
	}

	if restored > 0 {
		c.logger.Info("Recovered sessions", zap.Int("count", restored))
	}
}

// saveRecord 持久化已解码的记录（用于更新恢复后的过期时间）。
func (c *Core) saveRecord(rec *sessionRecord) {
	data, err := mq.EncodeMessage(rec)
	if err != nil {
		c.logger.Warn("Failed to encode session record",
			zap.String("clientID", rec.ClientID),
			zap.Error(err))
		return
	}
	if err := c.store.db().Update(func(txn *badger.Txn) error {
		return txn.Set(sessionKey(rec.ClientID), data)
	}); err != nil {
		c.logger.Warn("Failed to persist session record",
			zap.String("clientID", rec.ClientID),
			zap.Error(err))
	}
}

// cleanupOrphanClientQueues 删除没有对应会话的客户端持久化队列。
//
// 队列 key 形如 "client:<clientID>:<12 字节 PacketID>"，因此可以从 key 末尾
// 精确切出 clientID（即使 clientID 本身包含 ':'）。
func (c *Core) cleanupOrphanClientQueues() {
	if c.store == nil {
		return
	}

	const (
		queuePrefix = "client:"
		idLen       = packet.PacketIDSize
		batchSize   = 1000
	)

	live := make(map[string]struct{}, len(c.clients))
	c.clientsMu.RLock()
	for id := range c.clients {
		live[id] = struct{}{}
	}
	c.clientsMu.RUnlock()

	totalDeleted := 0
	for {
		deleted := 0
		err := c.store.db().Update(func(txn *badger.Txn) error {
			opts := badger.DefaultIteratorOptions
			opts.PrefetchValues = false
			opts.Prefix = []byte(queuePrefix)
			it := txn.NewIterator(opts)
			defer it.Close()

			for it.Seek([]byte(queuePrefix)); it.ValidForPrefix([]byte(queuePrefix)); it.Next() {
				key := it.Item().KeyCopy(nil)
				clientID, ok := clientIDFromQueueKey(key)
				if !ok {
					continue
				}
				if _, exists := live[clientID]; exists {
					continue
				}
				if err := txn.Delete(key); err != nil {
					return err
				}
				deleted++
				if deleted >= batchSize {
					break
				}
			}
			return nil
		})
		if err != nil {
			c.logger.Warn("Failed to clean up orphan client queues", zap.Error(err))
			return
		}

		totalDeleted += deleted
		if deleted < batchSize {
			break
		}
	}

	if totalDeleted > 0 {
		c.logger.Info("Cleaned up orphan client queue messages", zap.Int("count", totalDeleted))
	}
}

// clientIDFromQueueKey 从 "client:<clientID>:<12 字节 PacketID>" 中解析 clientID。
func clientIDFromQueueKey(key []byte) (string, bool) {
	const prefix = "client:"
	if !bytes.HasPrefix(key, []byte(prefix)) {
		return "", false
	}

	// 至少要有 "client:" + clientID(>=1) + ":" + PacketID
	if len(key) < len(prefix)+1+1+packet.PacketIDSize {
		return "", false
	}

	end := len(key) - packet.PacketIDSize - 1
	if key[end] != ':' {
		return "", false
	}

	return string(key[len(prefix):end]), true
}
