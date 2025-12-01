package model

import (
	"time"

	"github.com/uptrace/bun"
)

// Wire 端点
type Wire struct {
	bun.BaseModel `bun:"wire"`
	ID            string    `bun:"type:TEXT,pk" json:"id"`
	NodeID        string    `bun:"node_id,type:TEXT" json:"node_id"`
	Name          string    `bun:"name,type:TEXT" json:"name"`
	Clusters      string    `bun:"clusters,type:TEXT" json:"clusters"`
	Updated       time.Time `bun:"updated" json:"updated"`
}

// Pin 属性
type Pin struct {
	bun.BaseModel `bun:"pin"`
	ID            string    `bun:"type:TEXT,pk" json:"id"`
	NodeID        string    `bun:"node_id,type:TEXT" json:"node_id"`
	WireID        string    `bun:"wire_id,type:TEXT" json:"wire_id"`
	Name          string    `bun:"name,type:TEXT" json:"name"`
	Tags          string    `bun:"tags,type:TEXT" json:"tags"`
	Addr          string    `bun:"addr,type:TEXT" json:"addr"`
	Type          string    `bun:"type,type:TEXT" json:"type"`
	Rw            int32     `bun:"rw" json:"rw"`
	Updated       time.Time `bun:"updated" json:"updated"`
}

type PinValue struct {
	bun.BaseModel `bun:"pin_value"`
	ID            string    `bun:"type:TEXT,pk" json:"id"`
	NodeID        string    `bun:"node_id,type:TEXT" json:"node_id"`
	WireID        string    `bun:"wire_id,type:TEXT" json:"wire_id"`
	Value         string    `bun:"value,type:TEXT" json:"value"`
	Updated       time.Time `bun:"updated" json:"updated"`
}

type PinWrite struct {
	bun.BaseModel `bun:"pin_write"`
	ID            string    `bun:"type:TEXT,pk" json:"id"`
	NodeID        string    `bun:"node_id,type:TEXT" json:"node_id"`
	WireID        string    `bun:"wire_id,type:TEXT" json:"wire_id"`
	Value         string    `bun:"value,type:TEXT" json:"value"`
	Updated       time.Time `bun:"updated" json:"updated"`
}
