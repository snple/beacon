package device

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// ============================================================================
// 使用示例
// ============================================================================

// Example1_FirstInitialization 示例1：首次初始化设备（使用配置文件）
func Example1_FirstInitialization(db *bun.DB, configFile string) error {
	ctx := context.Background()

	// 创建配置管理器
	configMgr := NewConfigManager(db)

	// 从配置文件加载设备定义
	config, err := configMgr.LoadFromFile(configFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 应用配置（create 模式：只创建不存在的 Wire）
	err = configMgr.ApplyConfig(ctx, config, "create")
	if err != nil {
		return fmt.Errorf("apply config: %w", err)
	}

	fmt.Printf("Device initialized successfully with version %s\n", config.Version)
	return nil
}

// Example2_UpgradeDevice 示例2：设备升级（合并新配置）
func Example2_UpgradeDevice(db *bun.DB, newConfigFile string) error {
	ctx := context.Background()

	configMgr := NewConfigManager(db)

	// 加载新版本配置
	newConfig, err := configMgr.LoadFromFile(newConfigFile)
	if err != nil {
		return fmt.Errorf("load new config: %w", err)
	}

	// 应用配置（merge 模式：添加新 Pin，保留现有 Pin）
	err = configMgr.ApplyConfig(ctx, newConfig, "merge")
	if err != nil {
		return fmt.Errorf("merge config: %w", err)
	}

	fmt.Printf("Device upgraded to version %s\n", newConfig.Version)
	return nil
}

// Example3_ProgrammaticInitialization 示例3：编程方式初始化设备
func Example3_ProgrammaticInitialization(db *bun.DB) error {
	ctx := context.Background()

	// 创建初始化器
	init := NewInitializer(db)

	// 方式1：使用模板创建
	err := init.CreateWireFromTemplate(ctx, "light1", []string{
		"OnOff",
		"LevelControl",
		"ColorControl",
	})
	if err != nil {
		return err
	}

	// 方式2：使用 Builder 创建
	colorLight := BuildColorLightWire("light2")
	err = init.CreateWireFromBuilder(ctx, colorLight)
	if err != nil {
		return err
	}

	// 方式3：使用预构建模板
	err = init.CreateWireFromBuilder(ctx, BuildRootWire())
	if err != nil {
		return err
	}

	err = init.CreateWireFromBuilder(ctx, BuildTempHumiSensorWire("sensor1"))
	if err != nil {
		return err
	}

	return nil
}

// Example4_ExportConfig 示例4：导出当前配置到文件
func Example4_ExportConfig(db *bun.DB, outputFile string) error {
	ctx := context.Background()

	configMgr := NewConfigManager(db)

	// 从数据库导出当前配置
	config, err := configMgr.ExportConfig(ctx, "1.0.0")
	if err != nil {
		return fmt.Errorf("export config: %w", err)
	}

	// 保存到文件
	err = configMgr.SaveToFile(config, outputFile)
	if err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("Config exported to %s\n", outputFile)
	return nil
}

// Example5_ManualOperations 示例5：手动操作（不使用配置文件）
func Example5_ManualOperations(db *bun.DB) error {
	ctx := context.Background()

	init := NewInitializer(db)

	// 创建设备
	err := init.CreateWireFromTemplate(ctx, "custom_device", []string{
		"OnOff",
		"TemperatureMeasurement",
	})
	if err != nil {
		return err
	}

	// 更新 Pin 地址
	// 首先需要获取 Wire ID
	// 注意：这里需要先查询 Wire，然后更新 Pin
	// 实际使用中建议通过 ConfigManager 统一管理

	// 删除设备
	err = init.DeleteWire(ctx, "custom_device")
	if err != nil {
		return err
	}

	return nil
}

// Example6_CreateConfigProgrammatically 示例6：编程方式创建配置
func Example6_CreateConfigProgrammatically(db *bun.DB) error {
	ctx := context.Background()

	// 定义设备配置
	config := &DeviceConfig{
		Version: "1.0.0",
		Wires: []WireConfig{
			{
				Name:     "root",
				Clusters: []string{"BasicInformation"},
			},
			{
				Name:     "light1",
				Clusters: []string{"OnOff", "LevelControl", "ColorControl"},
				Pins: []PinConfig{
					{Name: "onoff", Addr: "GPIO_1"},
					{Name: "level", Addr: "PWM_1"},
					{Name: "hue", Addr: "PWM_2"},
					{Name: "saturation", Addr: "PWM_3"},
				},
			},
			{
				Name:     "sensor1",
				Clusters: []string{"TemperatureMeasurement", "HumidityMeasurement"},
				Pins: []PinConfig{
					{Name: "temperature", Addr: "I2C_0x48_TEMP"},
					{Name: "humidity", Addr: "I2C_0x48_HUMI"},
				},
			},
		},
	}

	// 应用配置
	configMgr := NewConfigManager(db)
	return configMgr.ApplyConfig(ctx, config, "create")
}

// Example7_MergeOnUpgrade 示例7：版本升级时的合并操作
func Example7_MergeOnUpgrade(db *bun.DB) error {
	ctx := context.Background()

	init := NewInitializer(db)

	// 假设原来的设备只有 OnOff 功能
	// 现在要升级，添加 LevelControl 功能
	err := init.MergeWireFromTemplate(ctx, "light1", []string{
		"OnOff",
		"LevelControl", // 新增
	})
	if err != nil {
		return err
	}

	// 合并操作会：
	// 1. 保留现有的所有 Pin
	// 2. 添加新 Cluster 的 Pin（如果不存在）
	// 3. 更新 Wire 的 Clusters 字段

	return nil
}
