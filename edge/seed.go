package edge

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/snple/beacon/device"
	"github.com/snple/beacon/edge/model"
	"github.com/snple/beacon/util"
	"github.com/uptrace/bun"
)

func Seed(db *bun.DB, nodeName string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	if err = seed(tx, nodeName); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func seed(db bun.Tx, nodeName string) error {
	var err error
	ctx := context.Background()

	// 初始化 Node
	{
		seedNode := func() error {
			node := model.Node{
				ID:      util.RandomID(),
				Name:    nodeName,
				Updated: time.Now(),
			}

			_, err = db.NewInsert().Model(&node).Exec(ctx)
			if err != nil {
				return err
			}

			fmt.Printf("seed: the initial node created: %v\n", node.ID)

			return nil
		}

		// check if node already exists
		// if not, create it
		err = db.NewSelect().Model(&model.Node{}).Scan(ctx)
		if err != nil {
			if err == sql.ErrNoRows {
				if err = seedNode(); err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}

	// 初始化默认的 root Wire（使用 device.Initializer）
	// 注意：实际项目中应该使用配置文件来定义设备
	{
		seedRootWire := func() error {
			// 使用 device.BuildRootWire() 创建 root Wire
			result := device.BuildRootWire()

			result.Wire.ID = util.RandomID()
			result.Wire.Updated = time.Now()

			// 插入 Wire
			_, err = db.NewInsert().Model(result.Wire).Exec(ctx)
			if err != nil {
				return err
			}

			// 插入 Pin
			for _, pin := range result.Pins {
				pin.ID = util.RandomID()
				pin.WireID = result.Wire.ID
				pin.Updated = time.Now()

				// 从 Cluster 获取 Pin 的 Type
				cluster := device.GetCluster("BasicInformation")
				if cluster != nil {
					if tpl := cluster.GetPinTemplate(pin.Name); tpl != nil {
						pin.Type = tpl.Type
					}
				}

				_, err = db.NewInsert().Model(pin).Exec(ctx)
				if err != nil {
					return err
				}
			}

			fmt.Printf("seed: the root wire created with %d pins\n", len(result.Pins))

			return nil
		}

		// 检查 root Wire 是否已存在
		err = db.NewSelect().Model(&model.Wire{}).Where("name = ?", "root").Scan(ctx)
		if err != nil {
			if err == sql.ErrNoRows {
				if err = seedRootWire(); err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}

	return nil
}
