// Package device 提供设备初始化和配置管理功能
package device

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/snple/beacon/edge/model"
	"github.com/snple/beacon/util"
	"github.com/uptrace/bun"
)

// Initializer 设备初始化器
// 用于在设备首次启动或升级时创建/更新 Wire 和 Pin
// 这些操作在设备生命周期中只执行一次，或在版本升级时执行合并操作
type Initializer struct {
	db *bun.DB
}

// NewInitializer 创建设备初始化器
func NewInitializer(db *bun.DB) *Initializer {
	return &Initializer{
		db: db,
	}
}

// CreateWireFromTemplate 从 Cluster 模板创建 Wire 和对应的 Pin
// 如果 Wire 已存在，则跳过创建
func (init *Initializer) CreateWireFromTemplate(ctx context.Context, wireName string, clusterNames []string) error {
	tx, err := init.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 检查 Wire 是否已存在
	exists, err := tx.NewSelect().
		Model((*model.Wire)(nil)).
		Where("name = ?", wireName).
		Exists(ctx)
	if err != nil {
		return err
	}
	if exists {
		// Wire 已存在，跳过
		return nil
	}

	// 使用 WireBuilder 构建 Wire 和 Pin
	builder := NewWireBuilder(wireName)
	for _, clusterName := range clusterNames {
		builder.WithCluster(clusterName)
	}
	result := builder.Build()

	// 生成 ID
	result.Wire.ID = util.RandomID()
	result.Wire.Updated = time.Now()

	// 插入 Wire
	_, err = tx.NewInsert().Model(result.Wire).Exec(ctx)
	if err != nil {
		return err
	}

	// 插入 Pin
	for _, pin := range result.Pins {
		pin.ID = util.RandomID()
		pin.WireID = result.Wire.ID
		pin.Updated = time.Now()

		// 从 Cluster 获取 Pin 的完整定义
		if pin.Type == "" {
			for _, clusterName := range clusterNames {
				cluster := GetCluster(clusterName)
				if cluster != nil {
					if tpl := cluster.GetPinTemplate(pin.Name); tpl != nil {
						pin.Type = tpl.Type
						break
					}
				}
			}
		}

		_, err = tx.NewInsert().Model(pin).Exec(ctx)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// CreateWireFromBuilder 从 WireBuilder 创建 Wire 和 Pin
// 如果 Wire 已存在，则跳过创建
func (init *Initializer) CreateWireFromBuilder(ctx context.Context, result *SimpleBuildResult) error {
	tx, err := init.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 检查 Wire 是否已存在
	exists, err := tx.NewSelect().
		Model((*model.Wire)(nil)).
		Where("name = ?", result.Wire.Name).
		Exists(ctx)
	if err != nil {
		return err
	}
	if exists {
		// Wire 已存在，跳过
		return nil
	}

	// 生成 ID
	result.Wire.ID = util.RandomID()
	result.Wire.Updated = time.Now()

	// 插入 Wire
	_, err = tx.NewInsert().Model(result.Wire).Exec(ctx)
	if err != nil {
		return err
	}

	// 插入 Pin
	clusterNames := parseClusterNames(result.Wire.Clusters)
	for _, pin := range result.Pins {
		pin.ID = util.RandomID()
		pin.WireID = result.Wire.ID
		pin.Updated = time.Now()

		// 从 Cluster 获取 Pin 的完整定义
		if pin.Type == "" {
			for _, clusterName := range clusterNames {
				cluster := GetCluster(clusterName)
				if cluster != nil {
					if tpl := cluster.GetPinTemplate(pin.Name); tpl != nil {
						pin.Type = tpl.Type
						break
					}
				}
			}
		}

		_, err = tx.NewInsert().Model(pin).Exec(ctx)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// MergeWireFromTemplate 合并 Wire 配置（用于设备升级）
// 这是一个智能合并操作：
// - 添加新的 Pin（来自新的 Cluster）
// - 保留现有的 Pin（不删除）
// - 更新 Wire 的 Clusters 字段
func (init *Initializer) MergeWireFromTemplate(ctx context.Context, wireName string, clusterNames []string) error {
	tx, err := init.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 查找 Wire
	wire := model.Wire{}
	err = tx.NewSelect().Model(&wire).Where("name = ?", wireName).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			// Wire 不存在，创建新的
			return init.CreateWireFromTemplate(ctx, wireName, clusterNames)
		}
		return err
	}

	// 获取现有的 Pin
	var existingPins []model.Pin
	err = tx.NewSelect().Model(&existingPins).Where("wire_id = ?", wire.ID).Scan(ctx)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// 创建已存在 Pin 的名称集合
	existingPinNames := make(map[string]bool)
	for _, pin := range existingPins {
		existingPinNames[pin.Name] = true
	}

	// 从 Cluster 获取应有的 Pin，只添加不存在的
	for _, clusterName := range clusterNames {
		cluster := GetCluster(clusterName)
		if cluster == nil {
			continue
		}

		for _, tpl := range cluster.Pins {
			// 如果 Pin 不存在，则创建
			if !existingPinNames[tpl.Name] {
				pin := model.Pin{
					ID:      util.RandomID(),
					WireID:  wire.ID,
					Name:    tpl.Name,
					Type:    tpl.Type,
					Rw:      tpl.Rw,
					Updated: time.Now(),
				}

				_, err = tx.NewInsert().Model(&pin).Exec(ctx)
				if err != nil {
					return err
				}
			}
		}
	}

	// 更新 Wire 的 Clusters 字段
	wire.Clusters = strings.Join(clusterNames, ",")
	wire.Updated = time.Now()
	_, err = tx.NewUpdate().Model(&wire).WherePK().Exec(ctx)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// DeleteWire 删除 Wire 及其所有 Pin
func (init *Initializer) DeleteWire(ctx context.Context, wireName string) error {
	tx, err := init.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 查找 Wire
	wire := model.Wire{}
	err = tx.NewSelect().Model(&wire).Where("name = ?", wireName).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil // Wire 不存在，视为成功
		}
		return err
	}

	// 删除所有关联的 Pin
	_, err = tx.NewDelete().Model((*model.Pin)(nil)).Where("wire_id = ?", wire.ID).Exec(ctx)
	if err != nil {
		return err
	}

	// 删除 Wire
	_, err = tx.NewDelete().Model(&wire).WherePK().Exec(ctx)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// UpdatePinAddress 更新 Pin 的物理地址
func (init *Initializer) UpdatePinAddress(ctx context.Context, wireID, pinName, address string) error {
	_, err := init.db.NewUpdate().
		Model((*model.Pin)(nil)).
		Set("addr = ?", address).
		Where("wire_id = ?", wireID).
		Where("name = ?", pinName).
		Exec(ctx)
	return err
}

// Helper functions

func parseClusterNames(clusters string) []string {
	if clusters == "" {
		return []string{}
	}
	result := []string{}
	for _, name := range strings.Split(clusters, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			result = append(result, name)
		}
	}
	return result
}
