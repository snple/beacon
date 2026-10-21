package packet

import "unicode/utf8"

// ValidateClientID 校验客户端 ID 是否符合服务端约束。
//
// 约束：
//   - 非空且不超过 MaxClientIDLength 字节；
//   - 合法 UTF-8；
//   - 不包含 ':'：core 用 "client:<clientID>:" 作为持久化队列的 key 前缀，
//     允许 ':' 会让 "a" 与 "a:b" 的队列前缀互相包含；
//   - 不包含控制字符：避免日志注入与不可见 ID。
func ValidateClientID(id string) bool {
	if len(id) == 0 || len(id) > MaxClientIDLength || !utf8.ValidString(id) {
		return false
	}

	for _, r := range id {
		if r == ':' || r < 0x20 || r == 0x7f {
			return false
		}
	}

	return true
}
