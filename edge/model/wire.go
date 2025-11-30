package model

import (
	"time"

	"github.com/uptrace/bun"
)

// Wire 端点
type Wire struct {
	bun.BaseModel `bun:"wire"`
	ID            string    `bun:"type:TEXT,pk" json:"id"`
	Name          string    `bun:"name,type:TEXT" json:"name"`
	Clusters      string    `bun:"clusters,type:TEXT" json:"clusters"` // Cluster 列表，如 "OnOff,LevelControl"
	Status        int32     `bun:"status" json:"status"`
	Deleted       time.Time `bun:"deleted,soft_delete" json:"-"`
	Created       time.Time `bun:"created" json:"created"`
	Updated       time.Time `bun:"updated" json:"updated"`
}

// Pin 属性
type Pin struct {
	bun.BaseModel `bun:"pin"`
	ID            string    `bun:"type:TEXT,pk" json:"id"`
	WireID        string    `bun:"wire_id,type:TEXT" json:"wire_id"`
	Name          string    `bun:"name,type:TEXT" json:"name"`
	Addr          string    `bun:"addr,type:TEXT" json:"addr"`
	Type          string    `bun:"type,type:TEXT" json:"type"`
	Status        int32     `bun:"status" json:"status"`
	Rw            int32     `bun:"rw" json:"rw"`
	Deleted       time.Time `bun:"deleted,soft_delete" json:"-"`
	Created       time.Time `bun:"created" json:"created"`
	Updated       time.Time `bun:"updated" json:"updated"`
}

type PinValue struct {
	ID      string    `bun:"type:TEXT,pk" json:"id"`
	WireID  string    `bun:"wire_id,type:TEXT" json:"wire_id"`
	Value   string    `bun:"value,type:TEXT" json:"value"`
	Updated time.Time `bun:"updated" json:"updated"`
}

const (
	PIN_VALUE_PREFIX = "pv_"
	PIN_WRITE_PREFIX = "pw_"
)
