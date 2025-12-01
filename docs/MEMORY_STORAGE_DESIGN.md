# 内存存储架构设计

## 概述

使用内存存储配置数据，Badger 仅用于持久化，简化架构并提升性能。

## 数据分类

### 1. 配置数据（内存 + NSON 持久化）
- **Node**: 节点元数据
- **Wire**: 通信总线配置
- **Pin**: 数据点配置

存储位置：
- **Core**: 内存 Map + Badger（key: node_id, value: nson 序列化）
- **Edge**: 内存 Map + 本地 nson 文件

### 2. 实时数据（保持现有方案）
- **PinValue**: Pin 的当前值（读）
- **PinWrite**: Pin 的写入值（写）
- **Sync**: 同步时间戳

存储位置：
- **Core**: Badger 键值存储
- **Edge**: Badger 键值存储

## Core 端数据结构

```go
type CoreStorage struct {
    // 内存缓存
    nodes map[string]*NodeData  // key: node_id
    
    // 索引（用于快速查询）
    nodesByName map[string]string  // name -> node_id
    wires map[string]*Wire        // key: wire_id
    wiresByNodeAndName map[string]string  // "node_id:name" -> wire_id
    pins map[string]*Pin          // key: pin_id
    pinsByWireAndName map[string]string   // "wire_id:name" -> pin_id
    
    // 持久化
    badger *badger.DB
    
    // Node Secret 单独存储（可修改）
    secrets map[string]string  // node_id -> secret
}

type NodeData struct {
    Node   Node
    Wires  []Wire
    Pins   []Pin  // 所有 Wire 下的 Pin
}
```

## Edge 端数据结构

```go
type EdgeStorage struct {
    // 内存数据
    node   Node
    wires  map[string]*Wire  // key: wire_id
    pins   map[string]*Pin   // key: pin_id
    
    // 索引
    wiresByName map[string]string  // name -> wire_id
    pinsByName map[string]string   // "wire_name.pin_name" -> pin_id
    
    // 配置文件路径
    configFile string  // nson 文件路径
    
    // Secret 配置
    secret string  // 从配置或文件读取
}
```

## NSON 文件格式

```nson
{
    "version": "1.0",
    "node": {
        "id": "node_001",
        "name": "EdgeDevice01",
        "status": 1
    },
    "wires": [
        {
            "id": "wire_001",
            "name": "modbus",
            "type": "modbus_rtu",
            "tags": "temperature,pressure",
            "clusters": "cluster1"
        }
    ],
    "pins": [
        {
            "id": "pin_001",
            "wire_id": "wire_001",
            "name": "temp_sensor_1",
            "addr": "40001",
            "type": "float32",
            "rw": 1,
            "tags": "sensor,critical"
        }
    ],
    // 签名（可选）
    "signature": "sha256:abc123..."
}
```

## API 变化

### 1. 简化的同步 API

```protobuf
// Edge → Core: 推送完整配置
rpc PushConfig(ConfigData) returns (MyBool);

// Core → Edge: 拉取完整配置
rpc PullConfig(Id) returns (ConfigData);

message ConfigData {
    string node_id = 1;
    bytes nson_data = 2;  // NSON 序列化的完整配置
    string signature = 3;  // 可选签名
}
```

### 2. 批量查询 API

```protobuf
// 返回整个 Node 的配置（一次性）
rpc GetNodeConfig(Id) returns (NodeConfigResponse);

message NodeConfigResponse {
    Node node = 1;
    repeated Wire wires = 2;
    repeated Pin pins = 3;
}
```

## 持久化策略

### Core 端

```go
// 保存节点配置
func (s *CoreStorage) SaveNode(nodeID string) error {
    data := s.nodes[nodeID]
    
    // 序列化为 NSON
    nsonData, err := nson.Marshal(data)
    if err != nil {
        return err
    }
    
    // 保存到 Badger
    key := []byte("node:" + nodeID)
    return s.badger.Update(func(txn *badger.Txn) error {
        return txn.Set(key, nsonData)
    })
}

// 启动时加载所有节点
func (s *CoreStorage) LoadAll() error {
    return s.badger.View(func(txn *badger.Txn) error {
        it := txn.NewIterator(badger.DefaultIteratorOptions)
        defer it.Close()
        
        prefix := []byte("node:")
        for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
            item := it.Item()
            err := item.Value(func(val []byte) error {
                var data NodeData
                err := nson.Unmarshal(val, &data)
                if err != nil {
                    return err
                }
                
                // 加载到内存并建立索引
                s.loadNodeData(&data)
                return nil
            })
            if err != nil {
                return err
            }
        }
        return nil
    })
}
```

### Edge 端

```go
// 启动时加载配置
func (s *EdgeStorage) LoadConfig(filename string) error {
    data, err := os.ReadFile(filename)
    if err != nil {
        return err
    }
    
    // 反序列化 NSON
    var config EdgeConfig
    err = nson.Unmarshal(data, &config)
    if err != nil {
        return err
    }
    
    // 验证签名（可选）
    if config.Signature != "" {
        if !s.verifySignature(&config) {
            return errors.New("signature verification failed")
        }
    }
    
    // 加载到内存
    s.loadConfig(&config)
    return nil
}

// 生成配置文件
func GenerateConfigFile(node *Node, wires []*Wire, pins []*Pin, filename string) error {
    config := EdgeConfig{
        Version: "1.0",
        Node:    node,
        Wires:   wires,
        Pins:    pins,
    }
    
    // 序列化为 NSON
    data, err := nson.Marshal(config)
    if err != nil {
        return err
    }
    
    // 写入文件
    return os.WriteFile(filename, data, 0644)
}
```

## Secret 管理

### Core 端

```go
// Secret 单独存储在 Badger
func (s *CoreStorage) SaveSecret(nodeID, secret string) error {
    key := []byte("secret:" + nodeID)
    return s.badger.Update(func(txn *badger.Txn) error {
        return txn.Set(key, []byte(secret))
    })
}

func (s *CoreStorage) GetSecret(nodeID string) (string, error) {
    // 先查内存
    if secret, ok := s.secrets[nodeID]; ok {
        return secret, nil
    }
    
    // 查 Badger
    var secret string
    err := s.badger.View(func(txn *badger.Txn) error {
        key := []byte("secret:" + nodeID)
        item, err := txn.Get(key)
        if err != nil {
            return err
        }
        return item.Value(func(val []byte) error {
            secret = string(val)
            return nil
        })
    })
    
    if err == nil {
        s.secrets[nodeID] = secret
    }
    return secret, err
}
```

### Edge 端

```go
// 从配置文件读取
type EdgeConfig struct {
    Secret string `json:"secret" nson:"secret"`
    // 或者从环境变量
    // SecretEnv string
}

// 或使用签名验证
func GenerateSecretFromKey(nodeID string, privateKey []byte) string {
    // 使用私钥生成签名作为 secret
    hash := sha256.Sum256(append([]byte(nodeID), privateKey...))
    return base64.StdEncoding.EncodeToString(hash[:])
}
```

## 优势总结

1. **性能**: 所有配置查询都在内存，微秒级响应
2. **简化**: 去掉复杂的 ORM 和 SQL
3. **同步**: 原子性更新，一次性同步整个配置
4. **部署**: Edge 端无需数据库，一个文件搞定
5. **可维护**: NSON 文件可读、可编辑、可版本控制
6. **扩展性**: 可以轻松添加签名、加密、压缩

## 迁移路径

1. Phase 1: 实现内存存储层
2. Phase 2: 保留 SQL 查询作为 fallback
3. Phase 3: 完全切换到内存存储
4. Phase 4: 提供迁移工具（SQL → NSON）
