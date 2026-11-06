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

// ReservedChannelPrefix 是 broker 内部 channel 的保留前缀。
const ReservedChannelPrefix = "$"

// IsReservedChannel 报告 name 是否落在 broker 内部保留的 "$" 命名空间。
//
// 客户端不得发布到或声明该前缀的 channel：它们由 broker 内部产生
// （例如系统事件），允许客户端参与会破坏其可信性。
func IsReservedChannel(name string) bool {
	return strings.HasPrefix(name, ReservedChannelPrefix)
}

// ValidateClientChannelName 校验客户端可发布/可声明的 channel 名：
// 在 ValidateChannelNameLimit 基础上额外禁止保留前缀。
func ValidateClientChannelName(name string, maxLen int) bool {
	return !IsReservedChannel(name) && ValidateChannelNameLimit(name, maxLen)
}

// ValidateClientChannelFilter 校验客户端可声明的过滤器：
// 在 ValidateChannelFilterLimit 基础上额外禁止保留前缀。
//
// "*" / "**" 这类通配符本身不落在保留命名空间，但仍可用于声明全部
// 公开 channel；是否覆盖 "$" 由 ACL 决定。
func ValidateClientChannelFilter(filter string, maxLen int) bool {
	return !IsReservedChannel(filter) && ValidateChannelFilterLimit(filter, maxLen)
}

// ValidateChannelName 校验发布用的 channel 名。
//
// 长度按协议硬上限 MaxChannelLength 校验；服务端应使用
// ValidateChannelNameLimit 施加自己配置的更严格上限。
//
// 允许 '-'、'_'、'/'、'.'，不允许通配符与 ':'。
// ':' 被禁止是为了让内部索引拼接（"chan:name" 形式）不会产生前缀包含歧义。
func ValidateChannelName(name string) bool {
	return ValidateChannelNameLimit(name, MaxChannelLength)
}

// ValidateChannelNameLimit 按给定长度上限校验发布用的 channel 名。
// maxLen <= 0 或超过协议硬上限时按 MaxChannelLength 处理。
func ValidateChannelNameLimit(name string, maxLen int) bool {
	if maxLen <= 0 || maxLen > MaxChannelLength {
		maxLen = MaxChannelLength
	}
	if len(name) == 0 || len(name) > maxLen {
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
// 长度按协议硬上限 MaxChannelLength 校验；服务端应使用
// ValidateChannelFilterLimit 施加自己配置的更严格上限。
//
// 语法：
//   - "*"  单层通配符，必须独占一个层级
//   - "**" 多层通配符，必须独占最后一个层级
func ValidateChannelFilter(filter string) bool {
	return ValidateChannelFilterLimit(filter, MaxChannelLength)
}

// ValidateChannelFilterLimit 按给定长度上限校验 channel 过滤器。
// maxLen <= 0 或超过协议硬上限时按 MaxChannelLength 处理。
func ValidateChannelFilterLimit(filter string, maxLen int) bool {
	if maxLen <= 0 || maxLen > MaxChannelLength {
		maxLen = MaxChannelLength
	}
	if len(filter) == 0 || len(filter) > maxLen {
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
//
// 与 MQTT 一致：首层是通配符（"*" 或 "**"）的过滤器不匹配 "$" 开头的
// 保留 channel，只有显式以 "$" 开头的过滤器才能命中它们。
func MatchChannel(filter, name string) bool {
	if filter == name {
		return true
	}

	filterParts := strings.Split(filter, "/")
	nameParts := strings.Split(name, "/")

	if len(filterParts) > 0 && isFirstLevelWildcard(filterParts[0]) && isReservedName(nameParts) {
		return false
	}

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

func isFirstLevelWildcard(level string) bool {
	return level == ChannelWildcardSingle || level == ChannelWildcardMulti
}

func isReservedName(parts []string) bool {
	return len(parts) > 0 && strings.HasPrefix(parts[0], ReservedChannelPrefix)
}

// FilterSubsumes 报告允许模式 allow 是否覆盖请求模式 request 所能匹配的全部 channel。
//
// 用于 ACL：声明 "device/**" 时必须确认调用方被授权覆盖 "device/**"。
// 注意 "a/**" 也会匹配 "a" 本身（** 可匹配零层）。
//
// 与 MatchChannel 一致：以通配符开头的 allow 不覆盖 "$" 保留命名空间。
func FilterSubsumes(allow, request string) bool {
	a := strings.Split(allow, "/")
	r := strings.Split(request, "/")

	if isFirstLevelWildcard(a[0]) && isReservedName(r) {
		return false
	}

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
			// allow 已耗尽：request 还有剩余层级（即使只是 "**"）就无法被完全覆盖。
			// "**" 虽然能匹配零层，但也能匹配更多层，不能视为已被 allow 覆盖。
			return false
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
