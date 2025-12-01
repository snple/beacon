package device

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/snple/beacon/edge/model"
	"github.com/uptrace/bun"
)

// DeviceConfig 设备配置结构（用于从配置文件加载）
type DeviceConfig struct {
	Version string       `json:"version" yaml:"version"` // 设备模型版本（写死在配置中）
	Wires   []WireConfig `json:"wires" yaml:"wires"`     // Wire 配置列表
}

// WireConfig Wire 配置
type WireConfig struct {
	Name     string      `json:"name" yaml:"name"`                     // Wire 名称
	Clusters []string    `json:"clusters" yaml:"clusters"`             // Cluster 列表
	Pins     []PinConfig `json:"pins,omitempty" yaml:"pins,omitempty"` // 可选：Pin 地址配置
}

// PinConfig Pin 配置（主要用于配置地址）
type PinConfig struct {
	Name string `json:"name" yaml:"name"` // Pin 名称
	Addr string `json:"addr" yaml:"addr"` // 物理地址（如 GPIO 引脚、Modbus 地址等）
}

// ConfigManager 配置管理器
type ConfigManager struct {
	db   *bun.DB
	init *Initializer
}

// NewConfigManager 创建配置管理器
func NewConfigManager(db *bun.DB) *ConfigManager {
	return &ConfigManager{
		db:   db,
		init: NewInitializer(db),
	}
}

// LoadFromFile 从文件加载设备配置
func (m *ConfigManager) LoadFromFile(filename string) (*DeviceConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var config DeviceConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	return &config, nil
}

// ApplyConfig 应用设备配置
// mode: "create" - 只创建不存在的 Wire（首次初始化）
//
//	"merge"  - 合并配置，添加新的 Pin，保留现有 Pin（版本升级）
func (m *ConfigManager) ApplyConfig(ctx context.Context, config *DeviceConfig, mode string) error {
	for _, wireConfig := range config.Wires {
		var err error

		switch mode {
		case "create":
			// 只创建，如果存在则跳过
			err = m.init.CreateWireFromTemplate(ctx, wireConfig.Name, wireConfig.Clusters)
		case "merge":
			// 合并模式，添加新 Pin，保留现有 Pin
			err = m.init.MergeWireFromTemplate(ctx, wireConfig.Name, wireConfig.Clusters)
		default:
			return fmt.Errorf("unknown mode: %s", mode)
		}

		if err != nil {
			return fmt.Errorf("process wire %s: %w", wireConfig.Name, err)
		}

		// 更新 Pin 的地址配置
		if len(wireConfig.Pins) > 0 {
			err = m.updatePinAddresses(ctx, wireConfig.Name, wireConfig.Pins)
			if err != nil {
				return fmt.Errorf("update pin addresses for wire %s: %w", wireConfig.Name, err)
			}
		}
	}

	return nil
}

// updatePinAddresses 更新 Pin 的地址配置
func (m *ConfigManager) updatePinAddresses(ctx context.Context, wireName string, pins []PinConfig) error {
	// 获取 Wire
	wire := model.Wire{}
	err := m.db.NewSelect().Model(&wire).Where("name = ?", wireName).Scan(ctx)
	if err != nil {
		return err
	}

	// 更新每个 Pin 的地址
	for _, pinConfig := range pins {
		_, err = m.db.NewUpdate().
			Model((*model.Pin)(nil)).
			Set("addr = ?", pinConfig.Addr).
			Where("wire_id = ?", wire.ID).
			Where("name = ?", pinConfig.Name).
			Exec(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

// SaveToFile 保存配置到文件
func (m *ConfigManager) SaveToFile(config *DeviceConfig, filename string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

// ExportConfig 从数据库导出当前配置
func (m *ConfigManager) ExportConfig(ctx context.Context, version string) (*DeviceConfig, error) {
	config := &DeviceConfig{
		Version: version,
		Wires:   make([]WireConfig, 0),
	}

	// 查询所有 Wire
	var wires []model.Wire
	err := m.db.NewSelect().Model(&wires).Scan(ctx)
	if err != nil {
		return nil, err
	}

	// 为每个 Wire 导出配置
	for _, wire := range wires {
		wireConfig := WireConfig{
			Name:     wire.Name,
			Clusters: parseClusterNames(wire.Clusters),
			Pins:     make([]PinConfig, 0),
		}

		// 查询 Wire 的所有 Pin
		var pins []model.Pin
		err = m.db.NewSelect().Model(&pins).Where("wire_id = ?", wire.ID).Scan(ctx)
		if err != nil {
			return nil, err
		}

		// 导出 Pin 配置
		for _, pin := range pins {
			if pin.Addr != "" {
				wireConfig.Pins = append(wireConfig.Pins, PinConfig{
					Name: pin.Name,
					Addr: pin.Addr,
				})
			}
		}

		config.Wires = append(config.Wires, wireConfig)
	}

	return config, nil
}
