package packet

import (
	"math/rand/v2"
	"strings"
	"testing"
)

func TestValidateChannelName(t *testing.T) {
	valid := []string{"a", "a/b", "sensors/1/temp", "$sys/x"}
	invalid := []string{"", "a/*", "a/**", "a/#", "a/+", "a:b", "a\x00b"}

	for _, s := range valid {
		if !ValidateChannelName(s) {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range invalid {
		if ValidateChannelName(s) {
			t.Errorf("%q should be invalid", s)
		}
	}
}

func TestValidateChannelFilter(t *testing.T) {
	valid := []string{"a", "a/*", "a/**", "*", "**", "a/b/*/c", "*/b"}
	invalid := []string{"", "a/**/b", "a/b**", "a/#", "a:b", "a*"}

	for _, s := range valid {
		if !ValidateChannelFilter(s) {
			t.Errorf("%q should be a valid filter", s)
		}
	}
	for _, s := range invalid {
		if ValidateChannelFilter(s) {
			t.Errorf("%q should be an invalid filter", s)
		}
	}
}

func TestMatchChannel(t *testing.T) {
	cases := []struct {
		filter, name string
		want         bool
	}{
		{"a/b", "a/b", true},
		{"a/b", "a/c", false},
		{"a/*", "a/b", true},
		{"a/*", "a/b/c", false},
		{"a/**", "a", true},
		{"a/**", "a/b/c", true},
		{"**", "anything/at/all", true},
		{"*", "a", true},
		{"*", "a/b", false},
		{"a/*/c", "a/b/c", true},
	}
	for _, c := range cases {
		if got := MatchChannel(c.filter, c.name); got != c.want {
			t.Errorf("MatchChannel(%q,%q)=%v want %v", c.filter, c.name, got, c.want)
		}
	}
}

// 随机生成小规模 channel 空间，验证 subsumes 与 MatchChannel 的一致性。
//
// names 必须比 filters 深一层：像 "a/b/**" 这样的请求模式能匹配到
// 第 4 层，只有在名字空间包含该层时才能发现「allow 是精确前缀」的误判。
func TestFilterSubsumesProperty(t *testing.T) {
	segments := []string{"a", "b"}
	names := genNames(segments, 4)
	filters := genFilters(segments, 3)

	for _, allow := range filters {
		for _, req := range filters {
			got := FilterSubsumes(allow, req)
			want := true
			for _, n := range names {
				if MatchChannel(req, n) && !MatchChannel(allow, n) {
					want = false
					break
				}
			}
			if got != want {
				t.Fatalf("subsumes(%q,%q)=%v want %v", allow, req, got, want)
			}
		}
	}
}

// TestFilterSubsumesExactCannotCoverWildcard 验证 ACL 的关键边界：
// 只被授权精确 channel 时，不能声明 "name/**" 来获取该前缀下的所有子 channel。
func TestFilterSubsumesExactCannotCoverWildcard(t *testing.T) {
	cases := []struct {
		allow, request string
		want           bool
	}{
		{"a", "a/**", false},
		{"a", "a/*", false},
		{"a/b", "a/**", false},
		{"a/b", "a/b/**", false},
		{"a/**", "a", true},
		{"a/**", "a/*", true},
		{"a/**", "a/**", true},
		{"*", "a", true},
		{"a/*", "a/b", true},
		{"a/b", "a/*", false},
	}
	for _, c := range cases {
		if got := FilterSubsumes(c.allow, c.request); got != c.want {
			t.Errorf("FilterSubsumes(%q, %q) = %v, want %v", c.allow, c.request, got, c.want)
		}
	}
}

func genNames(segs []string, depth int) []string {
	out := []string{}
	var build func(prefix string, left int)
	build = func(prefix string, left int) {
		// 记录每一层的前缀：** 能匹配零层，只有包含「浅层」名称时
		// 才能真正验证 FilterSubsumes 的边界语义。
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

func genFilters(segs []string, depth int) []string {
	base := genNames(segs, depth)
	out := append([]string{}, base...)
	for _, n := range base {
		out = append(out, n+"/**")
		parts := splitSlash(n)
		for i := range parts {
			clone := append([]string{}, parts...)
			clone[i] = "*"
			out = append(out, joinSlash(clone))
		}
	}
	uniq := make([]string, 0, len(out))
	seen := map[string]struct{}{}
	for _, f := range out {
		if !ValidateChannelFilter(f) {
			continue
		}
		if _, ok := seen[f]; ok {
			continue
		}
		seen[f] = struct{}{}
		uniq = append(uniq, f)
	}
	return uniq
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

func TestPacketIDOrderingApprox(t *testing.T) {
	// PacketID 以毫秒时间戳开头，连续生成的 ID 应大致有序。
	var prev PacketID
	for i := 0; i < 1000; i++ {
		id := NewPacketID()
		if i > 0 && string(id[:6]) < string(prev[:6]) {
			t.Fatalf("timestamp went backwards at %d", i)
		}
		prev = id
	}
	_ = rand.IntN
}

func TestChannelLengthLimit(t *testing.T) {
	if MinChannelLength != 256 || MaxChannelLength != 1024 {
		t.Fatalf("unexpected channel length bounds: min=%d max=%d", MinChannelLength, MaxChannelLength)
	}

	nameMin := strings.Repeat("a", MinChannelLength)
	nameMax := strings.Repeat("a", MaxChannelLength)
	nameOver := strings.Repeat("a", MaxChannelLength+1)

	if !ValidateChannelNameLimit(nameMin, MinChannelLength) {
		t.Fatal("name at configured limit should be valid")
	}
	if ValidateChannelNameLimit(strings.Repeat("a", MinChannelLength+1), MinChannelLength) {
		t.Fatal("name over configured limit should be invalid")
	}
	if !ValidateChannelNameLimit(nameMax, MaxChannelLength) {
		t.Fatal("name at hard cap should be valid")
	}
	if ValidateChannelName(nameOver) {
		t.Fatal("name over hard cap should be invalid")
	}

	// maxLen <= 0 或超过硬上限时回落到协议硬上限。
	if !ValidateChannelNameLimit(nameMax, 0) || ValidateChannelNameLimit(nameOver, 0) {
		t.Fatal("non-positive limit should clamp to the protocol hard cap")
	}
	if !ValidateChannelNameLimit(nameMax, 1<<20) || ValidateChannelNameLimit(nameOver, 1<<20) {
		t.Fatal("limit above the hard cap should clamp to the protocol hard cap")
	}

	filterMin := strings.Repeat("a", MinChannelLength-3) + "/**"
	if len(filterMin) != MinChannelLength {
		t.Fatalf("bad fixture length: %d", len(filterMin))
	}
	if !ValidateChannelFilterLimit(filterMin, MinChannelLength) {
		t.Fatal("filter at configured limit should be valid")
	}
	if ValidateChannelFilterLimit(filterMin, MinChannelLength-1) {
		t.Fatal("filter over configured limit should be invalid")
	}
	if ValidateChannelFilter(nameOver) {
		t.Fatal("filter over hard cap should be invalid")
	}
}

// "$" 前缀保留给 broker：通配符过滤器不得覆盖它，只有显式 "$" 过滤器命中。
func TestReservedChannelNamespace(t *testing.T) {
	cases := []struct {
		filter, name string
		want         bool
	}{
		{"**", "$sys/event", false},
		{"*", "$sys", false},
		{"*/*", "$sys/event", false},
		{"**", "normal/x", true},
		{"$sys/**", "$sys/event", true},
		{"$sys/*", "$sys/event", true},
		{"$sys/event", "$sys/event", true},
		{"a/**", "$sys/event", false},
	}
	for _, c := range cases {
		if got := MatchChannel(c.filter, c.name); got != c.want {
			t.Errorf("MatchChannel(%q, %q) = %v, want %v", c.filter, c.name, got, c.want)
		}
	}

	// ACL：通配符授权不覆盖保留命名空间。
	if FilterSubsumes("**", "$sys/event") {
		t.Error(`FilterSubsumes("**", "$sys/event") must be false`)
	}
	if !FilterSubsumes("$sys/**", "$sys/event") {
		t.Error(`FilterSubsumes("$sys/**", "$sys/event") must be true`)
	}

	if !IsReservedChannel("$sys/x") || IsReservedChannel("sys/x") {
		t.Error("IsReservedChannel misclassified a channel")
	}
	if ValidateClientChannelName("$sys/x", MaxChannelLength) {
		t.Error("client must not publish to reserved channels")
	}
	if !ValidateClientChannelName("sys/x", MaxChannelLength) {
		t.Error("normal channel should be accepted")
	}
}
