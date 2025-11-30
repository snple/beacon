package core

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"
	"time"

	"github.com/snple/beacon/core/model"
	"github.com/snple/beacon/util"
	"github.com/uptrace/bun"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type cloneService struct {
	cs *CoreService
}

func newCloneService(cs *CoreService) *cloneService {
	return &cloneService{
		cs: cs,
	}
}

func (s *cloneService) node(ctx context.Context, db bun.IDB, nodeID string) error {
	var err error

	node := model.Node{
		ID: nodeID,
	}

	err = db.NewSelect().Model(&node).WherePK().Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return status.Errorf(codes.NotFound, "Query: %v", err)
		}

		return status.Errorf(codes.Internal, "Query: %v", err)
	}

	node.ID = util.RandomID()
	node.Name = fmt.Sprintf("%v_clone_%v", node.Name, randNameSuffix())

	node.Created = time.Now()
	node.Updated = time.Now()

	_, err = db.NewInsert().Model(&node).Exec(ctx)
	if err != nil {
		return status.Errorf(codes.Internal, "Insert: %v", err)
	}

	// wire
	{
		var wires []model.Wire

		err = db.NewSelect().Model(&wires).Where("node_id = ?", nodeID).Order("id ASC").Scan(ctx)
		if err != nil {
			return status.Errorf(codes.Internal, "Query: %v", err)
		}

		for _, wire := range wires {
			oldWireId := wire.ID

			wire.ID = util.RandomID()
			wire.NodeID = node.ID

			wire.Created = time.Now()
			wire.Updated = time.Now()

			_, err = db.NewInsert().Model(&wire).Exec(ctx)
			if err != nil {
				return status.Errorf(codes.Internal, "Insert: %v", err)
			}

			// pin
			{
				var pins []model.Pin

				err = db.NewSelect().Model(&pins).Where("wire_id = ?", oldWireId).Order("id ASC").Scan(ctx)
				if err != nil {
					return status.Errorf(codes.Internal, "Query: %v", err)
				}

				for _, pin := range pins {
					pin.ID = util.RandomID()
					pin.WireID = wire.ID
					pin.NodeID = wire.NodeID

					pin.Created = time.Now()
					pin.Updated = time.Now()

					_, err = db.NewInsert().Model(&pin).Exec(ctx)
					if err != nil {
						return status.Errorf(codes.Internal, "Insert: %v", err)
					}
				}
			}
		}
	}

	err = s.cs.GetSync().setNodeUpdated(ctx, db, node.ID, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Insert: %v", err)
	}

	return nil
}

func (s *cloneService) wire(ctx context.Context, db bun.IDB, wireID, nodeID string) error {
	var err error

	item := model.Wire{
		ID: wireID,
	}

	err = db.NewSelect().Model(&item).WherePK().Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return status.Errorf(codes.NotFound, "Query: %v", err)
		}

		return status.Errorf(codes.Internal, "Query: %v", err)
	}

	// node validation
	if nodeID != "" {
		node := model.Node{
			ID: nodeID,
		}

		err = db.NewSelect().Model(&node).WherePK().Scan(ctx)
		if err != nil {
			if err == sql.ErrNoRows {
				return status.Error(codes.InvalidArgument, "Please supply valid NodeId")
			}

			return status.Errorf(codes.Internal, "Query: %v", err)
		}

		item.NodeID = node.ID
	}

	item.ID = util.RandomID()
	item.Name = fmt.Sprintf("%v_clone_%v", item.Name, randNameSuffix())

	item.Created = time.Now()
	item.Updated = time.Now()

	_, err = db.NewInsert().Model(&item).Exec(ctx)
	if err != nil {
		return status.Errorf(codes.Internal, "Insert: %v", err)
	}

	// clone pins
	{
		var pins []model.Pin

		err = db.NewSelect().Model(&pins).Where("wire_id = ?", item.ID).Order("id ASC").Scan(ctx)
		if err != nil {
			return status.Errorf(codes.Internal, "Query: %v", err)
		}

		for _, pin := range pins {
			pin.ID = util.RandomID()
			pin.WireID = item.ID
			pin.NodeID = item.NodeID

			pin.Created = time.Now()
			pin.Updated = time.Now()

			_, err = db.NewInsert().Model(&pin).Exec(ctx)
			if err != nil {
				return status.Errorf(codes.Internal, "Insert: %v", err)
			}
		}
	}

	err = s.cs.GetSync().setNodeUpdated(ctx, db, item.NodeID, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Insert: %v", err)
	}

	return nil
}

func (s *cloneService) pin(ctx context.Context, db bun.IDB, pinID, wireID string) error {
	var err error

	item := model.Pin{
		ID: pinID,
	}

	err = db.NewSelect().Model(&item).WherePK().Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return status.Errorf(codes.NotFound, "Query: %v", err)
		}

		return status.Errorf(codes.Internal, "Query: %v", err)
	}

	// wire validation
	if wireID != "" {
		wire := model.Wire{
			ID: wireID,
		}

		err = db.NewSelect().Model(&wire).WherePK().Scan(ctx)
		if err != nil {
			if err == sql.ErrNoRows {
				return status.Error(codes.InvalidArgument, "Please supply valid Wire.ID")
			}

			return status.Errorf(codes.Internal, "Query: %v", err)
		}

		item.WireID = wire.ID
		item.NodeID = wire.NodeID
	}

	item.ID = util.RandomID()
	item.Name = fmt.Sprintf("%v_clone_%v", item.Name, randNameSuffix())

	item.Created = time.Now()
	item.Updated = time.Now()

	_, err = db.NewInsert().Model(&item).Exec(ctx)
	if err != nil {
		return status.Errorf(codes.Internal, "Insert: %v", err)
	}

	err = s.cs.GetSync().setNodeUpdated(ctx, db, item.NodeID, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Insert: %v", err)
	}

	return nil
}

func randNameSuffix() string {
	buf := new(bytes.Buffer)

	random := rand.Uint32()
	binary.Write(buf, binary.BigEndian, random)

	return hex.EncodeToString(buf.Bytes())
}
