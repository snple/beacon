package packet

import (
	"strings"
	"unicode/utf8"
)

// ValidateClientID 校验客户端 ID 是否符合服务端约束。
//
// 约束：非空、不超过 MaxClientIDLength、合法 UTF-8、不含控制字符。
func ValidateClientID(id string) bool {
	if len(id) == 0 || len(id) > MaxClientIDLength || !utf8.ValidString(id) {
		return false
	}
	return !strings.ContainsFunc(id, func(r rune) bool {
		return r < 0x20 || r == 0x7f
	})
}

// ValidateChannelName 校验发布用的 channel 名。
//
// 允许 '-'、'_'、'/'、'.'，不允许通配符与 ':'。
// ':' 被禁止是为了让内部索引拼接（"chan:name" 形式）不会产生前缀包含歧义。
func ValidateChannelName(name string) bool {
	if len(name) == 0 || len(name) > MaxChannelLength {
		return false
	}
	if !utf8.ValidString(name) {
		return false
	}
	for _, r := range name {
		switch {
		case r == '*' || r == '#' || r == '+':
			return false
		case r == ':':
			return false
		case r < 0x20 || r == 0x7f:
			return false
		}
	}
	return true
}

// ValidateChannelFilter 校验声明/权限用的 channel 过滤器。
//
// 语法：
//   - "*"  单层通配符，必须独占一个层级
//   - "**" 多层通配符，必须独占最后一个层级
func ValidateChannelFilter(filter string) bool {
	if len(filter) == 0 || len(filter) > MaxChannelLength {
		return false
	}
	if !utf8.ValidString(filter) {
		return false
	}
	for _, r := range filter {
		if r == '#' || r == '+' || r == ':' || r < 0x20 || r == 0x7f {
			return false
		}
	}

	parts := strings.Split(filter, "/")
	for i, part := range parts {
		if part == ChannelWildcardMulti {
			if i != len(parts)-1 {
				return false
			}
			continue
		}
		if strings.Contains(part, ChannelWildcardSingle) && part != ChannelWildcardSingle {
			return false
		}
	}
	return true
}

// MatchChannel 检查 channel 名是否匹配过滤器。
// filter 可含通配符；name 不允许含通配符。
func MatchChannel(filter, name string) bool {
	if filter == name {
		return true
	}

	filterParts := strings.Split(filter, "/")
	nameParts := strings.Split(name, "/")

	for i, fp := range filterParts {
		if fp == ChannelWildcardMulti {
			return i == len(filterParts)-1
		}
		if i >= len(nameParts) {
			return false
		}
		if fp == ChannelWildcardSingle {
			continue
		}
		if fp != nameParts[i] {
			return false
		}
	}

	return len(filterParts) == len(nameParts)
}

// FilterSubsumes 报告允许模式 allow 是否覆盖请求模式 request 所能匹配的全部 channel。
//
// 用于 ACL：声明 "device/**" 时必须确认调用方被授权覆盖 "device/**"。
// 注意 "a/**" 也会匹配 "a" 本身（** 可匹配零层）。
func FilterSubsumes(allow, request string) bool {
	a := strings.Split(allow, "/")
	r := strings.Split(request, "/")

	var subsume func(ai, ri int) bool
	subsume = func(ai, ri int) bool {
		// allow 的多层通配符覆盖剩余全部（含零层）
		if ai < len(a) && a[ai] == ChannelWildcardMulti {
			return true
		}
		if ai == len(a) && ri == len(r) {
			return true
		}
		if ai == len(a) {
			// allow 已耗尽：只有 request 剩余部分是单个 "**"（可匹配空）才成立
			return ri == len(r)-1 && r[ri] == ChannelWildcardMulti
		}
		if ri == len(r) {
			return ai == len(a)-1 && a[ai] == ChannelWildcardMulti
		}
		// request 的可变长度要求 allow 也具备
		if r[ri] == ChannelWildcardMulti {
			return false
		}
		if a[ai] == ChannelWildcardSingle {
			return subsume(ai+1, ri+1)
		}
		if r[ri] == ChannelWildcardSingle {
			return false
		}
		if a[ai] != r[ri] {
			return false
		}
		return subsume(ai+1, ri+1)
	}

	return subsume(0, 0)
}
