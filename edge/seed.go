package edge

import (
	"context"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/snple/beacon/device"
	"github.com/snple/beacon/edge/storage"
	"github.com/snple/beacon/util"
)

func Seed(db *badger.DB, nodeName string) error {
	ctx := context.Background()
	store := storage.New(db)

	// 初始化 Node
	{
		// check if node already exists
		_, err := store.GetNode()
		if err != nil {
			// Node not exists, create it
			// 使用 device.BuildRootWire() 创建 root Wire
			result := device.BuildRootWire()

			// 构建 Pins
			pins := make([]storage.Pin, 0, len(result.Pins))
			for _, builderPin := range result.Pins {
				// 从 Cluster 获取 Pin 的 Type
				pinType := builderPin.Type
				cluster := device.GetCluster("BasicInformation")
				if cluster != nil {
					if tpl := cluster.GetPinTemplate(builderPin.Name); tpl != nil {
						pinType = tpl.Type
					}
				}

				pin := storage.Pin{
					ID:   util.RandomID(),
					Name: builderPin.Name,
					Addr: builderPin.Addr,
					Type: pinType,
					Rw:   builderPin.Rw,
				}
				pins = append(pins, pin)
			}

			// 构建 Wire
			wire := storage.Wire{
				ID:       util.RandomID(),
				Name:     result.Wire.Name,
				Type:     result.Wire.Type,
				Clusters: parseClusterString(result.Wire.Clusters),
				Pins:     pins,
			}

			// 构建 Node
			node := &storage.Node{
				ID:      util.RandomID(),
				Name:    nodeName,
				Updated: time.Now(),
				Wires:   []storage.Wire{wire},
			}

			if err := store.SetNode(ctx, node); err != nil {
				return err
			}

			fmt.Printf("seed: the initial node created: %v with %d wires and %d pins\n",
				node.ID, len(node.Wires), len(pins))
		}
	}

	return nil
}

// parseClusterString 将逗号分隔的字符串解析为数组
func parseClusterString(s string) []string {
	if s == "" {
		return nil
	}
	result := make([]string, 0)
	for _, part := range splitString(s, ",") {
		part = trimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func splitString(s, sep string) []string {
	if s == "" {
		return nil
	}
	result := make([]string, 0)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep[0] {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
