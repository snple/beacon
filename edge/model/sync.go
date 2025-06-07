package model

import "time"

type Sync struct {
	Key     string    `bun:"type:TEXT,pk" json:"key"`
	Updated time.Time `bun:"updated" json:"updated"`
}

const (
	SYNC_PREFIX    = "sync_"
	SYNC_NODE      = "sync_node"
	SYNC_WIRE      = "sync_wire"
	SYNC_PIN       = "sync_pin"
	SYNC_CONST     = "sync_const"
	SYNC_PIN_VALUE = "sync_pin_v"
	SYNC_PIN_WRITE = "sync_pin_w"

	SYNC_NODE_REMOTE_TO_LOCAL = "sync_node_rtl"
	SYNC_NODE_LOCAL_TO_REMOTE = "sync_node_ltr"

	SYNC_PIN_VALUE_REMOTE_TO_LOCAL = "sync_pv_rtl"
	SYNC_PIN_VALUE_LOCAL_TO_REMOTE = "sync_pv_ltr"

	SYNC_PIN_WRITE_REMOTE_TO_LOCAL = "sync_pw_rtl"
	SYNC_PIN_WRITE_LOCAL_TO_REMOTE = "sync_pw_ltr"
)
