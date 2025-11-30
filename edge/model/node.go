package model

import (
	"time"

	"github.com/uptrace/bun"
)

// Node 设备节点
type Node struct {
	bun.BaseModel `bun:"node"`
	ID            string    `bun:"type:TEXT,pk" json:"id"`
	Name          string    `bun:"name,type:TEXT" json:"name"`
	Secret        string    `bun:"secret,type:TEXT" json:"secret"`
	Updated       time.Time `bun:"updated" json:"updated"`
}
