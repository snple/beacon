# Beacon

Beacon 是一个轻量级消息总线协议，专为物联网和实时通信场景设计。

## 功能特性

- **发布/订阅模式**：支持主题订阅和消息发布
- **定向投递**：支持通过目标客户端 ID 将消息发送给指定客户端
- **PUBLISH 请求-响应元数据**：PUBLISH 可携带响应主题和关联数据，用于应用层实现请求-响应、异步回调和任务结果匹配

## 使用方法

导入包：

```go
import "github.com/snple/beacon/client"
import "github.com/snple/beacon/core"
```

更多使用示例请参考 `examples/` 目录。

## License

MIT
