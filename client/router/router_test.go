package router

import (
	"testing"
)

// ============================================================================
// 主题匹配器测试
// ============================================================================

func TestTopicMatcher_ExactMatch(t *testing.T) {
	matcher := newTopicMatcher("sensor/room1/temperature")

	tests := []struct {
		name    string
		topic   string
		matched bool
	}{
		{"exact match", "sensor/room1/temperature", true},
		{"different topic", "sensor/room2/temperature", false},
		{"shorter topic", "sensor/room1", false},
		{"longer topic", "sensor/room1/temperature/extra", false},
		{"completely different", "device/control", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, matched := matcher.match(tt.topic)
			if matched != tt.matched {
				t.Errorf("match(%q) = %v, want %v", tt.topic, matched, tt.matched)
			}
		})
	}
}

func TestTopicMatcher_SingleWildcard(t *testing.T) {
	matcher := newTopicMatcher("sensor/*/temperature")

	tests := []struct {
		name       string
		topic      string
		matched    bool
		paramKey   string
		paramValue string
	}{
		{"match room1", "sensor/room1/temperature", true, "*1", "room1"},
		{"match room2", "sensor/room2/temperature", true, "*1", "room2"},
		{"match kitchen", "sensor/kitchen/temperature", true, "*1", "kitchen"},
		{"too short", "sensor/temperature", false, "", ""},
		{"too long", "sensor/room1/floor1/temperature", false, "", ""},
		{"wrong prefix", "device/room1/temperature", false, "", ""},
		{"wrong suffix", "sensor/room1/humidity", false, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, matched := matcher.match(tt.topic)
			if matched != tt.matched {
				t.Errorf("match(%q) = %v, want %v", tt.topic, matched, tt.matched)
			}
			if matched && tt.paramKey != "" {
				if params[tt.paramKey] != tt.paramValue {
					t.Errorf("params[%q] = %q, want %q", tt.paramKey, params[tt.paramKey], tt.paramValue)
				}
			}
		})
	}
}

func TestTopicMatcher_MultipleSingleWildcards(t *testing.T) {
	matcher := newTopicMatcher("sensor/*/data/*")

	tests := []struct {
		name        string
		topic       string
		matched     bool
		param1Key   string
		param1Value string
		param2Key   string
		param2Value string
	}{
		{"match both", "sensor/room1/data/temp", true, "*1", "room1", "*2", "temp"},
		{"match both 2", "sensor/kitchen/data/humidity", true, "*1", "kitchen", "*2", "humidity"},
		{"too short", "sensor/room1/data", false, "", "", "", ""},
		{"wrong middle", "sensor/room1/info/temp", false, "", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, matched := matcher.match(tt.topic)
			if matched != tt.matched {
				t.Errorf("match(%q) = %v, want %v", tt.topic, matched, tt.matched)
			}
			if matched {
				if params[tt.param1Key] != tt.param1Value {
					t.Errorf("params[%q] = %q, want %q", tt.param1Key, params[tt.param1Key], tt.param1Value)
				}
				if params[tt.param2Key] != tt.param2Value {
					t.Errorf("params[%q] = %q, want %q", tt.param2Key, params[tt.param2Key], tt.param2Value)
				}
			}
		})
	}
}

func TestTopicMatcher_MultiLevelWildcard(t *testing.T) {
	matcher := newTopicMatcher("sensor/**")

	tests := []struct {
		name       string
		topic      string
		matched    bool
		paramValue string
	}{
		{"single level", "sensor/room1", true, "room1"},
		{"two levels", "sensor/room1/temp", true, "room1/temp"},
		{"three levels", "sensor/room1/floor1/temp", true, "room1/floor1/temp"},
		{"only prefix", "sensor", true, ""}, // ** 可以匹配空
		{"wrong prefix", "device/room1", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, matched := matcher.match(tt.topic)
			if matched != tt.matched {
				t.Errorf("match(%q) = %v, want %v", tt.topic, matched, tt.matched)
			}
			if matched && tt.paramValue != "" {
				if params["**"] != tt.paramValue {
					t.Errorf("params[\"**\"] = %q, want %q", params["**"], tt.paramValue)
				}
			}
		})
	}
}

func TestTopicMatcher_CombinedWildcards(t *testing.T) {
	matcher := newTopicMatcher("sensor/*/data/**")

	tests := []struct {
		name        string
		topic       string
		matched     bool
		singleParam string
		multiParam  string
	}{
		{"match both", "sensor/room1/data/temp/raw", true, "room1", "temp/raw"},
		{"single after data", "sensor/kitchen/data/humidity", true, "kitchen", "humidity"},
		{"empty multi", "sensor/room1/data", true, "room1", ""},
		{"wrong middle", "sensor/room1/info/temp", false, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, matched := matcher.match(tt.topic)
			if matched != tt.matched {
				t.Errorf("match(%q) = %v, want %v", tt.topic, matched, tt.matched)
			}
			if matched {
				if params["*1"] != tt.singleParam {
					t.Errorf("params[\"*1\"] = %q, want %q", params["*1"], tt.singleParam)
				}
			}
		})
	}
}

// ============================================================================
// Action 匹配器测试
// ============================================================================

func TestActionMatcher_ExactMatch(t *testing.T) {
	matcher := newActionMatcher("user.get.profile")

	tests := []struct {
		name    string
		action  string
		matched bool
	}{
		{"exact match", "user.get.profile", true},
		{"different action", "user.set.profile", false},
		{"shorter action", "user.get", false},
		{"longer action", "user.get.profile.extra", false},
		{"completely different", "device.control", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, matched := matcher.match(tt.action)
			if matched != tt.matched {
				t.Errorf("match(%q) = %v, want %v", tt.action, matched, tt.matched)
			}
		})
	}
}

func TestActionMatcher_SingleWildcard(t *testing.T) {
	matcher := newActionMatcher("user.*.get")

	tests := []struct {
		name       string
		action     string
		matched    bool
		paramKey   string
		paramValue string
	}{
		{"match 123", "user.123.get", true, "*1", "123"},
		{"match abc", "user.abc.get", true, "*1", "abc"},
		{"match profile", "user.profile.get", true, "*1", "profile"},
		{"too short", "user.get", false, "", ""},
		{"too long", "user.123.profile.get", false, "", ""},
		{"wrong prefix", "device.123.get", false, "", ""},
		{"wrong suffix", "user.123.set", false, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, matched := matcher.match(tt.action)
			if matched != tt.matched {
				t.Errorf("match(%q) = %v, want %v", tt.action, matched, tt.matched)
			}
			if matched && tt.paramKey != "" {
				if params[tt.paramKey] != tt.paramValue {
					t.Errorf("params[%q] = %q, want %q", tt.paramKey, params[tt.paramKey], tt.paramValue)
				}
			}
		})
	}
}

func TestActionMatcher_MultiLevelWildcard(t *testing.T) {
	matcher := newActionMatcher("api.**")

	tests := []struct {
		name       string
		action     string
		matched    bool
		paramValue string
	}{
		{"single level", "api.v1", true, "v1"},
		{"two levels", "api.v1.user", true, "v1.user"},
		{"three levels", "api.v1.user.list", true, "v1.user.list"},
		{"only prefix", "api", true, ""}, // ** 可以匹配空
		{"wrong prefix", "internal.v1", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, matched := matcher.match(tt.action)
			if matched != tt.matched {
				t.Errorf("match(%q) = %v, want %v", tt.action, matched, tt.matched)
			}
			if matched && tt.paramValue != "" {
				if params["**"] != tt.paramValue {
					t.Errorf("params[\"**\"] = %q, want %q", params["**"], tt.paramValue)
				}
			}
		})
	}
}

func TestActionMatcher_CombinedWildcards(t *testing.T) {
	matcher := newActionMatcher("service.*.method.**")

	tests := []struct {
		name        string
		action      string
		matched     bool
		singleParam string
		multiParam  string
	}{
		{"match both", "service.user.method.get.profile", true, "user", "get.profile"},
		{"single after method", "service.order.method.create", true, "order", "create"},
		{"empty multi", "service.user.method", true, "user", ""},
		{"wrong middle", "service.user.handler.get", false, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, matched := matcher.match(tt.action)
			if matched != tt.matched {
				t.Errorf("match(%q) = %v, want %v", tt.action, matched, tt.matched)
			}
			if matched {
				if params["*1"] != tt.singleParam {
					t.Errorf("params[\"*1\"] = %q, want %q", params["*1"], tt.singleParam)
				}
			}
		})
	}
}

// ============================================================================
// 边缘情况测试
// ============================================================================

func TestTopicMatcher_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		topic   string
		matched bool
	}{
		{"empty pattern vs empty topic", "", "", true},
		{"single segment", "sensor", "sensor", true},
		{"single * pattern", "*", "anything", true},
		{"** only", "**", "a/b/c", true},
		{"leading slash", "/sensor/temp", "/sensor/temp", true},
		{"trailing slash", "sensor/temp/", "sensor/temp/", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := newTopicMatcher(tt.pattern)
			_, matched := matcher.match(tt.topic)
			if matched != tt.matched {
				t.Errorf("pattern=%q topic=%q: got %v, want %v", tt.pattern, tt.topic, matched, tt.matched)
			}
		})
	}
}

func TestActionMatcher_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		action  string
		matched bool
	}{
		{"empty pattern vs empty action", "", "", true},
		{"single segment", "user", "user", true},
		{"single * pattern", "*", "anything", true},
		{"** only", "**", "a.b.c", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := newActionMatcher(tt.pattern)
			_, matched := matcher.match(tt.action)
			if matched != tt.matched {
				t.Errorf("pattern=%q action=%q: got %v, want %v", tt.pattern, tt.action, matched, tt.matched)
			}
		})
	}
}

// ============================================================================
// 基准测试
// ============================================================================

func BenchmarkTopicMatcher_Exact(b *testing.B) {
	matcher := newTopicMatcher("sensor/room1/temperature")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.match("sensor/room1/temperature")
	}
}

func BenchmarkTopicMatcher_SingleWildcard(b *testing.B) {
	matcher := newTopicMatcher("sensor/*/temperature")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.match("sensor/room1/temperature")
	}
}

func BenchmarkTopicMatcher_MultiWildcard(b *testing.B) {
	matcher := newTopicMatcher("sensor/**")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.match("sensor/room1/floor1/temperature")
	}
}

func BenchmarkActionMatcher_Exact(b *testing.B) {
	matcher := newActionMatcher("user.profile.get")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.match("user.profile.get")
	}
}

func BenchmarkActionMatcher_SingleWildcard(b *testing.B) {
	matcher := newActionMatcher("user.*.get")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.match("user.123.get")
	}
}

func BenchmarkActionMatcher_MultiWildcard(b *testing.B) {
	matcher := newActionMatcher("api.**")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.match("api.v1.user.profile.get")
	}
}
