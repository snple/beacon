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
	Deleted       time.Time `bun:"deleted,soft_delete" json:"-"`
	Created       time.Time `bun:"created" json:"created"`
	Updated       time.Time `bun:"updated" json:"updated"`
}
