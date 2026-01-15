package core

import (
	"container/heap"
	"sync"

	"github.com/snple/beacon/packet"
)

// Message 内部消息表示
type Message struct {
	// 基础消息属性
	Topic    string
	Payload  []byte
	QoS      packet.QoS
	Retain   bool
	PacketID uint16

	// 消息元数据
	Priority       *packet.Priority
	TraceID        string
	ContentType    string
	UserProperties map[string]string // 用户属性

	// 时间相关
	Timestamp  int64
	ExpiryTime int64

	// 请求-响应模式属性
	TargetClientID  string // 目标客户端ID，用于点对点消息
	SourceClientID  string // 来源客户端ID，标识发送者
	ResponseTopic   string // 响应主题，接收方可往此主题发回响应
	CorrelationData []byte // 关联数据，用于匹配请求和响应
}

// messageNode 链表节点
type messageNode struct {
	msg  *Message
	next *messageNode
}

// MessageQueue 基于链表的高效消息队列
type MessageQueue struct {
	priority packet.Priority
	head     *messageNode
	tail     *messageNode
	size     int
	maxSize  int
	mu       sync.Mutex
}

// NewMessageQueue 创建新的消息队列
func NewMessageQueue(priority packet.Priority, maxSize int) *MessageQueue {
	return &MessageQueue{
		priority: priority,
		maxSize:  maxSize,
	}
}

// Push 添加消息到队列 O(1)
func (q *MessageQueue) Push(msg *Message) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.maxSize > 0 && q.size >= q.maxSize {
		return false // 队列已满
	}

	node := &messageNode{msg: msg}
	if q.tail == nil {
		q.head = node
		q.tail = node
	} else {
		q.tail.next = node
		q.tail = node
	}
	q.size++
	return true
}

// Pop 从队列取出消息 O(1)
func (q *MessageQueue) Pop() *Message {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.head == nil {
		return nil
	}

	node := q.head
	q.head = node.next
	if q.head == nil {
		q.tail = nil
	}
	q.size--
	return node.msg
}

// Len 返回队列长度 O(1)
func (q *MessageQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.size
}

// PriorityQueue 带优先级的消息队列 (堆实现)
type PriorityQueue struct {
	items []*priorityItem
	mu    sync.Mutex
}

type priorityItem struct {
	message  *Message
	priority int
	index    int
}

func (pq *PriorityQueue) Len() int { return len(pq.items) }

func (pq *PriorityQueue) Less(i, j int) bool {
	// 高优先级在前
	return pq.items[i].priority > pq.items[j].priority
}

func (pq *PriorityQueue) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
	pq.items[i].index = i
	pq.items[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(pq.items)
	item := x.(*priorityItem)
	item.index = n
	pq.items = append(pq.items, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := pq.items
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	pq.items = old[0 : n-1]
	return item
}

// NewPriorityQueue 创建新的优先级队列
func NewPriorityQueue() *PriorityQueue {
	return &PriorityQueue{
		items: make([]*priorityItem, 0, 64), // 预分配容量减少扩容
	}
}

// Enqueue 入队
func (pq *PriorityQueue) Enqueue(msg *Message) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	priority := int(packet.PriorityNormal)
	if msg.Priority != nil {
		priority = int(*msg.Priority)
	}

	item := &priorityItem{
		message:  msg,
		priority: priority,
	}
	heap.Push(pq, item)
}

// Dequeue 出队
func (pq *PriorityQueue) Dequeue() *Message {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if len(pq.items) == 0 {
		return nil
	}

	item := heap.Pop(pq).(*priorityItem)
	return item.message
}

// Size 返回队列大小
func (pq *PriorityQueue) Size() int {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	return len(pq.items)
}

// RingBuffer 环形缓冲区实现的消息队列
// 适用于固定容量、高吞吐场景
type RingBuffer struct {
	buffer []*Message
	head   int // 读指针
	tail   int // 写指针
	size   int // 当前元素数量
	cap    int // 容量
	mu     sync.Mutex
}

// NewRingBuffer 创建环形缓冲区
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 1024
	}
	return &RingBuffer{
		buffer: make([]*Message, capacity),
		cap:    capacity,
	}
}

// Push 添加消息 O(1)
func (r *RingBuffer) Push(msg *Message) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.size >= r.cap {
		return false // 缓冲区已满
	}

	r.buffer[r.tail] = msg
	r.tail = (r.tail + 1) % r.cap
	r.size++
	return true
}

// Pop 取出消息 O(1)
func (r *RingBuffer) Pop() *Message {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.size == 0 {
		return nil
	}

	msg := r.buffer[r.head]
	r.buffer[r.head] = nil // 帮助 GC
	r.head = (r.head + 1) % r.cap
	r.size--
	return msg
}

// PopN 批量取出消息 O(n)
func (r *RingBuffer) PopN(n int) []*Message {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.size == 0 {
		return nil
	}

	if n > r.size {
		n = r.size
	}

	result := make([]*Message, n)
	for i := 0; i < n; i++ {
		result[i] = r.buffer[r.head]
		r.buffer[r.head] = nil
		r.head = (r.head + 1) % r.cap
	}
	r.size -= n
	return result
}

// Len 返回当前元素数量 O(1)
func (r *RingBuffer) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.size
}

// Cap 返回容量 O(1)
func (r *RingBuffer) Cap() int {
	return r.cap
}

// IsFull 检查是否已满
func (r *RingBuffer) IsFull() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.size >= r.cap
}

// IsEmpty 检查是否为空
func (r *RingBuffer) IsEmpty() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.size == 0
}

// Clear 清空缓冲区
func (r *RingBuffer) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.buffer {
		r.buffer[i] = nil
	}
	r.head = 0
	r.tail = 0
	r.size = 0
}
