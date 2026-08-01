package mq

import "maps"

// CloneBytes 复制字节切片（nil 保持 nil）
func CloneBytes(data []byte) []byte {
	if data == nil {
		return nil
	}
	cloned := make([]byte, len(data))
	copy(cloned, data)
	return cloned
}

// CloneStringMap 复制字符串映射（nil 保持 nil）
func CloneStringMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	cloned := make(map[string]string, len(src))
	maps.Copy(cloned, src)
	return cloned
}
