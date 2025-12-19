package core

import (
	"github.com/danclive/nson-go"
)

// 定义 Core 内部使用的类型
// 替代原先的 protobuf 中简单的请求/响应类型
// 注意：Node, Wire, Pin, NsonValue 等数据类型继续使用 pb 包中的定义

// 基础类型
type Id struct {
	Id string
}

type Name struct {
	Name string
}

type Message struct {
	Message string
}

type MyBool struct {
	Bool bool
}

type MyEmpty struct{}

// Node 节点
type Node struct {
	Id      string
	Name    string
	Device  string
	Tags    []string
	Status  int32
	Updated int64
	Wires   []*Wire
}

// Wire 线路
type Wire struct {
	Id   string
	Name string
	Type string
	Tags []string
	Pins []*Pin
}

// Pin 引脚
type Pin struct {
	Id   string
	Name string
	Addr string
	Type uint32
	Rw   int32
	Tags []string
}

// PinValue Pin 值
type PinValue struct {
	Id      string
	Value   nson.Value
	Updated int64
}

// PinNameValue Pin 名称值
type PinNameValue struct {
	Name    string
	Value   nson.Value
	Updated int64
}

// Node 相关
type NodeListResponse struct {
	Nodes []*Node
}

type NodePushRequest struct {
	Id   string
	Nson []byte
}

type NodeSecretRequest struct {
	Id     string
	Secret string
}

// Wire 相关
type WireViewRequest struct {
	NodeId string
	WireId string
}

type WireNameRequest struct {
	NodeId string
	Name   string
}

type WireListRequest struct {
	NodeId string
}

type WireListResponse struct {
	Wires []*Wire
}

// Pin 相关
type PinViewRequest struct {
	NodeId string
	PinId  string
}

type PinNameRequest struct {
	NodeId string
	Name   string
}

type PinListRequest struct {
	NodeId string
	WireId string // optional: filter by wire
}

type PinListResponse struct {
	Pins []*Pin
}

// PinValue 相关
type PinNameValueRequest struct {
	NodeId string
	Name   string
	Value  nson.Value
}

// PinWrite 相关
type PinPullWriteRequest struct {
	NodeId string
}

// Sync 相关
type SyncUpdated struct {
	NodeId  string
	Updated int64
}
