package model

import "time"

type Sync struct {
	Key     string    `bun:"type:TEXT,pk" json:"key"`
	Updated time.Time `bun:"updated" json:"updated"`
}

const (
	SYNC_PREFIX    = "sync_"
	SYNC_NODE      = "sync_node"  // 本地配置数据最新时间戳
	SYNC_PIN_VALUE = "sync_pin_v" // 本地 PinValue 最新时间戳
	SYNC_PIN_WRITE = "sync_pin_w" // 本地 PinWrite 最新时间戳（从 Core 拉取的）

	SYNC_NODE_TO_REMOTE        = "sync_node_ltr" // 配置数据已同步到 Core 的时间戳
	SYNC_PIN_VALUE_TO_REMOTE   = "sync_pv_ltr"   // PinValue 已同步到 Core 的时间戳
	SYNC_PIN_WRITE_FROM_REMOTE = "sync_pw_rtl"   // PinWrite 已从 Core 拉取的时间戳
)
