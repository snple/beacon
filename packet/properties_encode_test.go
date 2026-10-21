package packet

import (
	"bytes"
	"strings"
	"testing"
)

// TestUserPropertiesEncodeDeterministic 验证用户属性编码与 map 遍历顺序无关。
//
// 回归：旧实现直接遍历 map 编码，相同内容会产出不同字节序列，
// 破坏签名、去重与测试复现。
func TestUserPropertiesEncodeDeterministic(t *testing.T) {
	props := map[string]string{
		"z_last":  "1",
		"a_first": "2",
		"m_mid":   "3",
		"b":       "4",
		"y":       "5",
	}

	var first []byte
	for i := 0; i < 32; i++ {
		p := NewConnectProperties()
		for k, v := range props {
			p.UserProperties[k] = v
		}
		var buf bytes.Buffer
		if err := p.Encode(&buf); err != nil {
			t.Fatalf("Encode: %v", err)
		}
		if i == 0 {
			first = buf.Bytes()
			continue
		}
		if !bytes.Equal(first, buf.Bytes()) {
			t.Fatalf("编码结果不确定（第 %d 次）:\n%x\n%x", i, first, buf.Bytes())
		}
	}

	// 键必须按字典序出现
	got := string(first)
	last := -1
	for _, k := range []string{"a_first", "b", "m_mid", "y", "z_last"} {
		idx := strings.Index(got, k)
		if idx < 0 {
			t.Fatalf("缺少键 %q", k)
		}
		if idx < last {
			t.Fatalf("键 %q 未按字典序编码", k)
		}
		last = idx
	}

	// 往返一致
	var decoded ConnectProperties
	if err := decoded.Decode(bytes.NewReader(first)); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	for k, v := range props {
		if decoded.UserProperties[k] != v {
			t.Fatalf("往返后 %q = %q, 期望 %q", k, decoded.UserProperties[k], v)
		}
	}
}

// TestPropertiesEncodeRejectsOversizedValue 验证超长属性值不再静默产出畸形包。
//
// 回归：旧实现丢弃 EncodeString 的返回值，超长字符串会被截断成长度前缀
// 与实际内容不匹配的畸形数据。
func TestPropertiesEncodeRejectsOversizedValue(t *testing.T) {
	huge := strings.Repeat("x", MaxStringLength+1)

	t.Run("connect", func(t *testing.T) {
		p := NewConnectProperties()
		p.AuthMethod = huge
		var buf bytes.Buffer
		if err := p.Encode(&buf); err == nil {
			t.Fatal("期望返回错误")
		}
		if buf.Len() != 0 {
			t.Fatalf("出错时不应写入任何字节，实际写入 %d 字节", buf.Len())
		}
	})

	t.Run("connect-user-property", func(t *testing.T) {
		p := NewConnectProperties()
		p.UserProperties["k"] = huge
		var buf bytes.Buffer
		if err := p.Encode(&buf); err == nil {
			t.Fatal("期望返回错误")
		}
		if buf.Len() != 0 {
			t.Fatalf("出错时不应写入任何字节，实际写入 %d 字节", buf.Len())
		}
	})

	t.Run("publish", func(t *testing.T) {
		p := NewPublishProperties()
		p.ContentType = huge
		var buf bytes.Buffer
		if err := p.Encode(&buf); err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("publish-correlation-data", func(t *testing.T) {
		p := NewPublishProperties()
		p.CorrelationData = make([]byte, MaxStringLength+1)
		var buf bytes.Buffer
		if err := p.Encode(&buf); err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("connack", func(t *testing.T) {
		p := NewConnackProperties()
		p.ClientID = huge
		var buf bytes.Buffer
		if err := p.Encode(&buf); err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("auth", func(t *testing.T) {
		p := NewAuthProperties()
		p.AuthMethod = huge
		var buf bytes.Buffer
		if err := p.Encode(&buf); err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("reason", func(t *testing.T) {
		p := NewReasonProperties()
		p.ReasonString = huge
		var buf bytes.Buffer
		if err := p.Encode(&buf); err == nil {
			t.Fatal("期望返回错误")
		}
	})
}
