package core

import (
	"sync"

	"github.com/snple/beacon/packet"
)

// matcher 维护 channel 过滤器 → 客户端 的映射。
//
// 精确过滤器走 map，含通配符的过滤器放在线性列表中逐个匹配。
// 单客户端声明的过滤器数量由 MaxChannelsPerClient 限制，因此线性匹配的
// 上界是「通配符过滤器总数」，对典型 IoT 部署（每台设备一个精确 channel）
// 来说精确匹配是 O(1)。
type matcher struct {
	mu       sync.RWMutex
	exact    map[string]map[string]struct{}
	wild     map[string]map[string]struct{} // filter -> clientIDs
	byClient map[string][]string            // clientID -> filters
}

func newMatcher() *matcher {
	return &matcher{
		exact:    make(map[string]map[string]struct{}),
		wild:     make(map[string]map[string]struct{}),
		byClient: make(map[string][]string),
	}
}

func isWildcardFilter(filter string) bool {
	for i := 0; i < len(filter); i++ {
		if filter[i] == '*' {
			return true
		}
	}
	return false
}

// declare 用新声明全量替换某客户端的过滤器集合。
func (m *matcher) declare(clientID string, decls []packet.ChannelDecl) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.removeClientLocked(clientID)

	if len(decls) == 0 {
		return
	}

	seen := make(map[string]struct{}, len(decls))
	for _, d := range decls {
		if _, dup := seen[d.Channel]; dup {
			continue
		}
		seen[d.Channel] = struct{}{}

		if isWildcardFilter(d.Channel) {
			set := m.wild[d.Channel]
			if set == nil {
				set = make(map[string]struct{})
				m.wild[d.Channel] = set
			}
			set[clientID] = struct{}{}
		} else {
			set := m.exact[d.Channel]
			if set == nil {
				set = make(map[string]struct{})
				m.exact[d.Channel] = set
			}
			set[clientID] = struct{}{}
		}
		m.byClient[clientID] = append(m.byClient[clientID], d.Channel)
	}
}

// remove 移除某客户端的全部过滤器。
func (m *matcher) remove(clientID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeClientLocked(clientID)
}

func (m *matcher) removeClientLocked(clientID string) {
	for _, filter := range m.byClient[clientID] {
		if isWildcardFilter(filter) {
			if set := m.wild[filter]; set != nil {
				delete(set, clientID)
				if len(set) == 0 {
					delete(m.wild, filter)
				}
			}
			continue
		}
		if set := m.exact[filter]; set != nil {
			delete(set, clientID)
			if len(set) == 0 {
				delete(m.exact, filter)
			}
		}
	}
	delete(m.byClient, clientID)
}

// match 返回匹配该 channel 的客户端集合。
func (m *matcher) match(channel string) map[string]struct{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var out map[string]struct{}

	if set, ok := m.exact[channel]; ok && len(set) > 0 {
		out = make(map[string]struct{}, len(set))
		for id := range set {
			out[id] = struct{}{}
		}
	}

	for filter, set := range m.wild {
		if !packet.MatchChannel(filter, channel) {
			continue
		}
		if out == nil {
			out = make(map[string]struct{}, len(set))
		}
		for id := range set {
			out[id] = struct{}{}
		}
	}

	return out
}

// filters 返回某客户端当前声明的过滤器。
func (m *matcher) filters(clientID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	src := m.byClient[clientID]
	out := make([]string, len(src))
	copy(out, src)
	return out
}

// wildcardCount 返回通配符过滤器数量（用于容量观测）。
func (m *matcher) wildcardCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.wild)
}
