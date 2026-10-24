package core

import (
	"fmt"
	"sort"
	"testing"

	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// 通配符语义以 packet.MatchTopic 为唯一参照实现。
// core 里保留两棵树遍历（订阅树按"过滤器→具体主题"匹配，保留消息树按"过滤器→主题集合"
// 枚举），方向相反不能合并；本文件用同一份语料对三者做差分校验，防止语义漂移。

var matchCorpusFilters = []string{
	"a",
	"a/b",
	"a/b/c",
	"a/*",
	"*",
	"**",
	"a/**",
	"a/*/c",
	"*/b",
	"设备/温度",
	"设备/*",
	// 非法过滤器（** 不在末尾）：任何主题都不应匹配
	"a/**/b",
	"**/b",
}

var matchCorpusTopics = []string{
	"a",
	"a/b",
	"a/b/c",
	"a/b/d",
	"a/x",
	"a/x/c",
	"b",
	"b/b",
	"b/c",
	"设备",
	"设备/温度",
	"设备/湿度",
}

// TestSubTreeMatchAgreesWithPacketOracle 验证订阅树匹配与 packet.MatchTopic 一致。
func TestSubTreeMatchAgreesWithPacketOracle(t *testing.T) {
	tree := newSubTree()
	for i, filter := range matchCorpusFilters {
		tree.subscribe(fmt.Sprintf("c%d", i), filter, packet.SubscribeOptions{QoS: packet.QoS0})
	}

	for _, topic := range matchCorpusTopics {
		t.Run(topic, func(t *testing.T) {
			got := tree.matchTopic(topic)

			want := map[string]struct{}{}
			for i, filter := range matchCorpusFilters {
				if packet.MatchTopic(filter, topic) {
					want[fmt.Sprintf("c%d", i)] = struct{}{}
				}
			}

			require.Len(t, got, len(want), "subscriber set mismatch for topic %q", topic)
			for clientID := range want {
				require.Contains(t, got, clientID, "topic %q should match %s", topic, clientID)
			}
		})
	}
}

// TestRetainTreeMatchAgreesWithPacketOracle 验证保留消息树匹配与 packet.MatchTopic 一致。
func TestRetainTreeMatchAgreesWithPacketOracle(t *testing.T) {
	store := newRetainStore()
	for _, topic := range matchCorpusTopics {
		require.NoError(t, store.set(topic, 0))
	}

	for _, filter := range matchCorpusFilters {
		t.Run(filter, func(t *testing.T) {
			got := store.matchTopics(filter)
			sort.Strings(got)

			var want []string
			for _, topic := range matchCorpusTopics {
				if packet.MatchTopic(filter, topic) {
					want = append(want, topic)
				}
			}
			sort.Strings(want)

			require.Equal(t, want, got, "retained topics for filter %q", filter)
		})
	}
}
