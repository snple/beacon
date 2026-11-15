package core

import (
	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// mergeProperties 以 base 为底合并 extra；extra 中的同名键覆盖 base。
// 两者都为空时返回 nil。
func mergeProperties(base, extra map[string]string) map[string]string {
	if len(extra) == 0 {
		return base
	}
	if len(base) == 0 {
		return extra
	}
	out := make(map[string]string, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// fitProperties 裁剪掉放不进对端接收上限的响应属性。
//
// fixed 是该响应除属性之外的固定编码开销；属性整组丢弃而不是截断，
// 避免只回传一半键值造成语义歧义。label 只用于日志。
func (cn *conn) fitProperties(label string, props map[string]string, fixed int64) map[string]string {
	if len(props) == 0 {
		return nil
	}
	limit := cn.responseBudget()
	if limit <= 0 || fixed+int64(packet.StringMapSize(props)) <= int64(limit) {
		return props
	}
	cn.core.logger.Warn("response properties dropped: exceeds client packet limit",
		zap.String("clientID", cn.client.ID),
		zap.String("response", label),
		zap.Uint32("maxPacketSize", cn.maxPacketSize))
	return nil
}

// pullProperties 为本条 PULL 组装并裁剪 PULL_OK 属性，返回属性与编码大小。
//
// 每个 PULL 请求只计算一次（长轮询期间复用），避免重复触发 OnPull。
func (cn *conn) pullProperties(p *packet.PullPacket) (map[string]string, int64) {
	if cn.core.hooks.Pull == nil {
		return nil, 0
	}
	ctx := &PullContext{
		ClientID:   cn.client.ID,
		Identity:   cn.client.getIdentity(),
		Properties: p.Properties,
		WaitMs:     p.WaitMs,
		MaxCount:   p.MaxCount,
		MaxBytes:   p.MaxBytes,
	}
	cn.core.hooks.callOnPull(ctx)
	props := cn.fitProperties("PULL_OK", ctx.ResponseProperties, pullResponseOverhead)
	if len(props) == 0 {
		return nil, 0
	}
	return props, int64(packet.StringMapSize(props))
}
