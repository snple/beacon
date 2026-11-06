package core

import (
	"strings"
	"sync"

	"github.com/snple/beacon/packet"
)

// matcher 维护 channel 过滤器 → 客户端 的映射。
//
// 与 MQTT 的订阅树同构（"*" 等价于 "+"，"**" 等价于 "#"）：
//   - 精确过滤器放在 map 中，发布命中时 O(1) 复制；
//   - 含通配符的过滤器放进按层级组织的 trie，匹配复杂度只与 channel 深度
//     和实际命中的分支有关，不再随通配符订阅总数线性增长。
//
// 每个过滤器节点保存该过滤器下的客户端集合，因此同一过滤器被多个客户端
// 声明时只会占一条 trie 路径。
type matcher struct {
	mu          sync.RWMutex
	exact       map[string]map[string]struct{}
	wild        *filterTrie
	byClient    map[string][]string // clientID -> filters
	wildFilters int                 // 当前非空的不同的通配符过滤器数量
}

type filterTrie struct {
	root *trieNode
}

// trieNode 是订阅树的一个节点。
//
// terminal 保存「通配符过滤器恰好结束于此」的客户端集合；multi 保存
// "<prefix>/**" 的客户端集合（"**" 可匹配零层或多层，因此在匹配路径上
// 每经过一个节点都要收集 multi）。
type trieNode struct {
	children map[string]*trieNode // 字面量层级
	single   *trieNode            // "*" 层级
	terminal map[string]struct{}  // 以 "*" 或字面量结尾的通配符过滤器
	multi    map[string]struct{}  // "<prefix>/**"
}

func newMatcher() *matcher {
	return &matcher{
		exact:    make(map[string]map[string]struct{}),
		wild:     &filterTrie{root: &trieNode{}},
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
			if m.wild.add(d.Channel, clientID) {
				m.wildFilters++
			}
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
			if m.wild.remove(filter, clientID) {
				m.wildFilters--
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

	if m.wildFilters > 0 {
		out = m.wild.root.collect(channel, 0, out)
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
	return m.wildFilters
}

// add 登记一个通配符过滤器，返回该过滤器是否是新建的（用于统计）。
func (t *filterTrie) add(filter, clientID string) (newFilter bool) {
	parts, multi := splitFilter(filter)

	n := t.root
	for _, seg := range parts {
		if seg == packet.ChannelWildcardSingle {
			if n.single == nil {
				n.single = &trieNode{}
			}
			n = n.single
			continue
		}
		if n.children == nil {
			n.children = make(map[string]*trieNode)
		}
		child := n.children[seg]
		if child == nil {
			child = &trieNode{}
			n.children[seg] = child
		}
		n = child
	}

	if multi {
		if n.multi == nil {
			n.multi = make(map[string]struct{})
			newFilter = true
		}
		n.multi[clientID] = struct{}{}
		return newFilter
	}
	if n.terminal == nil {
		n.terminal = make(map[string]struct{})
		newFilter = true
	}
	n.terminal[clientID] = struct{}{}
	return newFilter
}

// remove 注销一个通配符过滤器，返回该过滤器是否已无任何客户端。
func (t *filterTrie) remove(filter, clientID string) (filterGone bool) {
	parts, multi := splitFilter(filter)
	_, filterGone = t.root.remove(parts, multi, clientID)
	return filterGone
}

func (n *trieNode) remove(parts []string, multi bool, clientID string) (empty, filterGone bool) {
	if len(parts) == 0 {
		if multi {
			if n.multi != nil {
				delete(n.multi, clientID)
				if len(n.multi) == 0 {
					n.multi = nil
					filterGone = true
				}
			}
		} else if n.terminal != nil {
			delete(n.terminal, clientID)
			if len(n.terminal) == 0 {
				n.terminal = nil
				filterGone = true
			}
		}
		return n.empty(), filterGone
	}

	seg := parts[0]
	var child *trieNode
	if seg == packet.ChannelWildcardSingle {
		child = n.single
	} else if n.children != nil {
		child = n.children[seg]
	}
	if child == nil {
		return false, false
	}

	childEmpty, filterGone := child.remove(parts[1:], multi, clientID)
	if childEmpty {
		if seg == packet.ChannelWildcardSingle {
			n.single = nil
		} else {
			delete(n.children, seg)
			if len(n.children) == 0 {
				n.children = nil
			}
		}
	}
	return n.empty(), filterGone
}

func (n *trieNode) empty() bool {
	return n.single == nil && len(n.children) == 0 && len(n.terminal) == 0 && len(n.multi) == 0
}

// collect 把匹配 name[pos:] 的客户端并入 out。
//
// pos > len(name) 表示所有层级都已消费完，此时该节点的 terminal 也命中。
//
// 与 packet.MatchChannel 一致：根节点的 "*" / "**" 分支不匹配 "$" 开头的
// 保留 channel，只有显式以 "$" 开头的过滤器才命中。
func (n *trieNode) collect(name string, pos int, out map[string]struct{}) map[string]struct{} {
	root := pos == 0
	reserved := root && name != "" && name[0] == packet.ReservedChannelPrefix[0]

	// "**" 匹配零层或多层：匹配路径上的每个节点都可能挂着 multi 订阅。
	if !reserved {
		out = mergeClients(out, n.multi)
	}

	if pos > len(name) {
		return mergeClients(out, n.terminal)
	}

	seg := name[pos:]
	next := len(name) + 1
	if i := strings.IndexByte(seg, '/'); i >= 0 {
		seg, next = seg[:i], pos+i+1
	}

	if child := n.children[seg]; child != nil {
		out = child.collect(name, next, out)
	}
	if n.single != nil && !reserved {
		out = n.single.collect(name, next, out)
	}
	return out
}

func mergeClients(out, set map[string]struct{}) map[string]struct{} {
	if len(set) == 0 {
		return out
	}
	if out == nil {
		out = make(map[string]struct{}, len(set))
	}
	for id := range set {
		out[id] = struct{}{}
	}
	return out
}

// splitFilter 拆分过滤器；末尾的 "**" 单独用 multi 表示，不进入路径层级。
func splitFilter(filter string) (parts []string, multi bool) {
	parts = strings.Split(filter, "/")
	if last := parts[len(parts)-1]; last == packet.ChannelWildcardMulti {
		multi = true
		parts = parts[:len(parts)-1]
	}
	return parts, multi
}
