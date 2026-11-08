package core

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/snple/beacon/packet"
)

func decls(filters ...string) []packet.ChannelDecl {
	out := make([]packet.ChannelDecl, len(filters))
	for i, f := range filters {
		out[i] = packet.ChannelDecl{Channel: f}
	}
	return out
}

func matchIDs(m map[string]struct{}) map[string]bool {
	out := make(map[string]bool, len(m))
	for id := range m {
		out[id] = true
	}
	return out
}

func TestMatcherExactAndWildcard(t *testing.T) {
	m := newMatcher()
	m.declare("exact", decls("a/b"))
	m.declare("single", decls("a/*"))
	m.declare("multi", decls("a/**"))
	m.declare("all", decls("**"))
	m.declare("star", decls("*"))

	cases := []struct {
		name string
		want []string
	}{
		{"a", []string{"all", "multi", "star"}},
		{"a/b", []string{"all", "exact", "single", "multi"}},
		{"a/b/c", []string{"all", "multi"}},
		{"x", []string{"all", "star"}},
	}
	for _, c := range cases {
		got := matchIDs(m.match(c.name))
		if len(got) != len(c.want) {
			t.Fatalf("match(%q) = %v, want %v", c.name, got, c.want)
		}
		for _, id := range c.want {
			if !got[id] {
				t.Fatalf("match(%q) missing %q (got %v)", c.name, id, got)
			}
		}
	}
}

// TestMatcherMatchesReference 用 packet.MatchChannel 作为参照实现，
// 随机生成声明/重声明/注销序列，验证 trie 与协议语义完全一致。
func TestMatcherMatchesReference(t *testing.T) {
	filters := genTestFilters(3)
	names := genTestNames(4)

	m := newMatcher()
	declared := make(map[string][]string) // clientID -> filters

	r := rand.New(rand.NewPCG(1, 2))
	for step := 0; step < 200; step++ {
		clientID := fmt.Sprintf("c%d", r.IntN(12))

		if r.IntN(5) == 0 {
			m.remove(clientID)
			delete(declared, clientID)
		} else {
			n := r.IntN(8)
			chosen := make([]string, 0, n)
			for i := 0; i < n; i++ {
				chosen = append(chosen, filters[r.IntN(len(filters))])
			}
			m.declare(clientID, decls(chosen...))
			if len(chosen) == 0 {
				delete(declared, clientID)
			} else {
				declared[clientID] = chosen
			}
		}

		for _, name := range names {
			got := m.match(name)
			for clientID, fs := range declared {
				want := false
				for _, f := range fs {
					if packet.MatchChannel(f, name) {
						want = true
						break
					}
				}
				_, ok := got[clientID]
				if ok != want {
					t.Fatalf("step=%d name=%q client=%q filters=%v: got=%v want=%v",
						step, name, clientID, fs, ok, want)
				}
			}
			// 反向检查：没有多余命中
			for clientID := range got {
				if _, ok := declared[clientID]; !ok {
					t.Fatalf("step=%d name=%q: unexpected client %q", step, name, clientID)
				}
			}
		}
	}
}

func TestMatcherRemovePrunesTrie(t *testing.T) {
	m := newMatcher()
	m.declare("c1", decls("a/*/c", "a/**", "**"))
	if got := m.wildcardCount(); got != 3 {
		t.Fatalf("wildcardCount=%d, want 3", got)
	}

	m.remove("c1")
	if got := m.wildcardCount(); got != 0 {
		t.Fatalf("wildcardCount=%d after remove, want 0", got)
	}
	if got := m.match("a/b/c"); len(got) != 0 {
		t.Fatalf("match after remove = %v", got)
	}
	if !m.wild.root.empty() {
		t.Fatal("trie should be pruned back to an empty root")
	}
}

func TestMatcherSharedFilterBookkeeping(t *testing.T) {
	m := newMatcher()
	m.declare("c1", decls("a/**", "a/**"))
	m.declare("c2", decls("a/**"))

	if got := m.wildcardCount(); got != 1 {
		t.Fatalf("wildcardCount=%d, want 1 distinct filter", got)
	}
	if got := matchIDs(m.match("a/x")); len(got) != 2 || !got["c1"] || !got["c2"] {
		t.Fatalf("match = %v", got)
	}

	m.remove("c1")
	if got := m.wildcardCount(); got != 1 {
		t.Fatalf("wildcardCount=%d after removing one client, want 1", got)
	}
	m.remove("c2")
	if got := m.wildcardCount(); got != 0 {
		t.Fatalf("wildcardCount=%d after removing all, want 0", got)
	}
}

func TestMatcherRedeclareReplaces(t *testing.T) {
	m := newMatcher()
	m.declare("c1", decls("a/**"))
	m.declare("c1", decls("b/**"))

	if got := m.match("a/x"); len(got) != 0 {
		t.Fatalf("old declaration still matches: %v", got)
	}
	if got := m.match("b/x"); len(got) != 1 || !matchIDs(got)["c1"] {
		t.Fatalf("new declaration does not match: %v", got)
	}
}

func genTestNames(depth int) []string {
	segs := []string{"a", "b", "c"}
	out := []string{}
	var build func(prefix string, left int)
	build = func(prefix string, left int) {
		if prefix != "" {
			out = append(out, prefix)
		}
		if left == 0 {
			return
		}
		for _, s := range segs {
			if prefix == "" {
				build(s, left-1)
			} else {
				build(prefix+"/"+s, left-1)
			}
		}
	}
	build("", depth)
	return out
}

func genTestFilters(depth int) []string {
	names := genTestNames(depth)
	seen := map[string]struct{}{}
	out := make([]string, 0, len(names)*4)
	appendFilter := func(f string) {
		if !packet.ValidateChannelFilter(f) {
			return
		}
		if _, ok := seen[f]; ok {
			return
		}
		seen[f] = struct{}{}
		out = append(out, f)
	}
	for _, n := range names {
		appendFilter(n)
		appendFilter(n + "/**")
		parts := splitSlash(n)
		for i := range parts {
			clone := append([]string(nil), parts...)
			clone[i] = "*"
			appendFilter(joinSlash(clone))
		}
	}
	appendFilter("**")
	appendFilter("*")
	return out
}

func splitSlash(s string) []string {
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == '/' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	return append(out, cur)
}

func joinSlash(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "/"
		}
		out += p
	}
	return out
}

func BenchmarkMatcherExactOnly(b *testing.B) {
	m := newMatcher()
	for i := 0; i < 10000; i++ {
		m.declare(fmt.Sprintf("dev/%d", i), decls(fmt.Sprintf("dev/%d", i)))
	}
	b.ReportAllocs()

	for b.Loop() {
		m.match("dev/5000")
	}
}

func BenchmarkMatcherWithWildcards(b *testing.B) {
	m := newMatcher()
	for i := 0; i < 10000; i++ {
		id := fmt.Sprintf("exact-%d", i)
		m.declare(id, decls(fmt.Sprintf("dev/%d/temp", i)))
	}
	for i := 0; i < 1000; i++ {
		id := fmt.Sprintf("wild-%d", i)
		m.declare(id, decls(fmt.Sprintf("dev/%d/**", i)))
	}
	m.declare("global", decls("**"))
	m.declare("single", decls("dev/*/temp"))

	b.ReportAllocs()

	for b.Loop() {
		m.match("dev/5000/temp")
	}
}
