package packet

import (
	"math/rand/v2"
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
func TestFilterSubsumesProperty(t *testing.T) {
	segments := []string{"a", "b", "c"}
	names := genNames(segments, 3)
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

func genNames(segs []string, depth int) []string {
	out := []string{}
	var build func(prefix string, d int)
	build = func(prefix string, d int) {
		if d == 0 {
			if prefix != "" {
				out = append(out, prefix)
			}
			return
		}
		for _, s := range segs {
			if prefix == "" {
				build(s, d-1)
			} else {
				build(prefix+"/"+s, d-1)
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
