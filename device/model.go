// 简化的数据模型定义
// 配合 Cluster 注册表使用，去除冗余字段
package device

import (
	"time"

	"github.com/uptrace/bun"
)

// Node 设备节点（对应 Device）
// 简化：去掉 Desc, Tags, Config（这些信息可以从根 Wire 的 Cluster 获取）
type Node struct {
	bun.BaseModel `bun:"node"`
	ID            string    `bun:"type:TEXT,pk" json:"id"`
	Name          string    `bun:"name,type:TEXT" json:"name"`     // 设备名称
	Secret        string    `bun:"secret,type:TEXT" json:"secret"` // 认证密钥
	Status        int32     `bun:"status" json:"status"`           // 状态
	Deleted       time.Time `bun:"deleted,soft_delete" json:"-"`
	Created       time.Time `bun:"created" json:"created"`
	Updated       time.Time `bun:"updated" json:"updated"`
}

// Wire 端点（对应 Endpoint）
// 简化：去掉 Desc, Tags, Config, Source
// 新增：Clusters 字段，存储包含的 Cluster 名称列表（逗号分隔）
type Wire struct {
	bun.BaseModel `bun:"wire"`
	ID            string    `bun:"type:TEXT,pk" json:"id"`
	NodeID        string    `bun:"node_id,type:TEXT" json:"node_id"`
	Name          string    `bun:"name,type:TEXT" json:"name"`         // Wire 名称
	Clusters      string    `bun:"clusters,type:TEXT" json:"clusters"` // 包含的 Cluster 列表，如 "OnOff,LevelControl"
	Status        int32     `bun:"status" json:"status"`
	Deleted       time.Time `bun:"deleted,soft_delete" json:"-"`
	Created       time.Time `bun:"created" json:"created"`
	Updated       time.Time `bun:"updated" json:"updated"`
}

// Pin 属性（对应 Attribute）
// 简化：去掉 Desc, Tags, Config（这些从 Cluster 注册表获取）
// 保留 Type, Rw 和 Addr
type Pin struct {
	bun.BaseModel `bun:"pin"`
	ID            string    `bun:"type:TEXT,pk" json:"id"`
	NodeID        string    `bun:"node_id,type:TEXT" json:"node_id"`
	WireID        string    `bun:"wire_id,type:TEXT" json:"wire_id"`
	Name          string    `bun:"name,type:TEXT" json:"name"` // Pin 名称（对应 Cluster 中的 PinTemplate.Name）
	Addr          string    `bun:"addr,type:TEXT" json:"addr"` // 硬件地址（可选，用于 Edge 端映射）
	Type          string    `bun:"type,type:TEXT" json:"type"` // 数据类型
	Status        int32     `bun:"status" json:"status"`
	Rw            int32     `bun:"rw" json:"rw"` // 0: 只读, 1: 读写
	Deleted       time.Time `bun:"deleted,soft_delete" json:"-"`
	Created       time.Time `bun:"created" json:"created"`
	Updated       time.Time `bun:"updated" json:"updated"`
}

// PinValue Pin 的当前值
type PinValue struct {
	bun.BaseModel `bun:"pin_value"`
	ID            string    `bun:"type:TEXT,pk" json:"id"` // 同 Pin.ID
	NodeID        string    `bun:"node_id,type:TEXT" json:"node_id"`
	WireID        string    `bun:"wire_id,type:TEXT" json:"wire_id"`
	Value         string    `bun:"value,type:TEXT" json:"value"`
	Deleted       time.Time `bun:"deleted,soft_delete" json:"-"`
	Updated       time.Time `bun:"updated" json:"updated"`
}

// PinWrite Pin 的写入值（用于 Core -> Edge 的写入同步）
type PinWrite struct {
	bun.BaseModel `bun:"pin_write"`
	ID            string    `bun:"type:TEXT,pk" json:"id"` // 同 Pin.ID
	NodeID        string    `bun:"node_id,type:TEXT" json:"node_id"`
	WireID        string    `bun:"wire_id,type:TEXT" json:"wire_id"`
	Value         string    `bun:"value,type:TEXT" json:"value"`
	Deleted       time.Time `bun:"deleted,soft_delete" json:"-"`
	Updated       time.Time `bun:"updated" json:"updated"`
}

// Sync 同步状态
type Sync struct {
	bun.BaseModel `bun:"sync"`
	Key           string    `bun:"type:TEXT,pk" json:"key"`
	Value         time.Time `bun:"value" json:"value"`
}

// SyncGlobal 全局同步状态（可选）
type SyncGlobal struct {
	bun.BaseModel `bun:"sync_global"`
	Key           string    `bun:"type:TEXT,pk" json:"key"`
	Value         time.Time `bun:"value" json:"value"`
}

// 注意：Const 结构已移除
// 设备基本信息通过根 Wire（包含 BasicInformation Cluster）获取
