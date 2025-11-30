package model

import (
	"time"

	"github.com/uptrace/bun"
)

type Sync struct {
	bun.BaseModel `bun:"sync"`
	ID            string    `bun:"type:TEXT,pk" json:"id"`
	Updated       time.Time `bun:"updated" json:"updated"`
}

const (
	SYNC_NODE_SUFFIX      = ""
	SYNC_PIN_VALUE_SUFFIX = "_pv"
	SYNC_PIN_WRITE_SUFFIX = "_pw"
)

type SyncGlobal struct {
	bun.BaseModel `bun:"sync_global"`
	Name          string    `bun:"type:TEXT,pk" json:"name"`
	Updated       time.Time `bun:"updated" json:"updated"`
}

const (
	SYNC_GLOBAL_NODE = "node"
	SYNC_GLOBAL_WIRE = "wire"
	SYNC_GLOBAL_PIN  = "pin"
)
