package model

import (
	"time"

	"github.com/uptrace/bun"
)

type Wire struct {
	bun.BaseModel `bun:"wire"`
	ID            string    `bun:"type:TEXT,pk" json:"id"`
	Name          string    `bun:"name,type:TEXT" json:"name"`
	Type          string    `bun:"type,type:TEXT" json:"type"`
	Tags          string    `bun:"tags,type:TEXT" json:"tags"`
	Clusters      string    `bun:"clusters,type:TEXT" json:"clusters"`
	Updated       time.Time `bun:"updated" json:"updated"`
}

type Pin struct {
	bun.BaseModel `bun:"pin"`
	ID            string    `bun:"type:TEXT,pk" json:"id"`
	WireID        string    `bun:"wire_id,type:TEXT" json:"wire_id"`
	Name          string    `bun:"name,type:TEXT" json:"name"`
	Tags          string    `bun:"tags,type:TEXT" json:"tags"`
	Addr          string    `bun:"addr,type:TEXT" json:"addr"`
	Type          string    `bun:"type,type:TEXT" json:"type"`
	Rw            int32     `bun:"rw" json:"rw"`
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
