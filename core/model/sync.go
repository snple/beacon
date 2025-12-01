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
