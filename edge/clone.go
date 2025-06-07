package edge

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"
	"time"

	"github.com/snple/beacon/edge/model"
	"github.com/snple/beacon/util"
	"github.com/uptrace/bun"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type cloneService struct {
	es *EdgeService
}

func newCloneService(es *EdgeService) *cloneService {
	return &cloneService{
		es: es,
	}
}

func (s *cloneService) wire(ctx context.Context, db bun.IDB, wireID string) error {
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

		err = db.NewSelect().Model(&pins).Where("wire_id = ?", wireID).Order("id ASC").Scan(ctx)
		if err != nil {
			return status.Errorf(codes.Internal, "Query: %v", err)
		}

		for _, pin := range pins {
			pin.ID = util.RandomID()
			pin.WireID = item.ID

			pin.Created = time.Now()
			pin.Updated = time.Now()

			_, err = db.NewInsert().Model(&pin).Exec(ctx)
			if err != nil {
				return status.Errorf(codes.Internal, "Insert: %v", err)
			}
		}
	}

	{
		err = s.es.GetSync().setNodeUpdated(ctx, time.Now())
		if err != nil {
			return status.Errorf(codes.Internal, "Insert: %v", err)
		}

		err = s.es.GetSync().setWireUpdated(ctx, time.Now())
		if err != nil {
			return status.Errorf(codes.Internal, "Insert: %v", err)
		}
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
	}

	item.ID = util.RandomID()
	item.Name = fmt.Sprintf("%v_clone_%v", item.Name, randNameSuffix())

	item.Created = time.Now()
	item.Updated = time.Now()

	_, err = db.NewInsert().Model(&item).Exec(ctx)
	if err != nil {
		return status.Errorf(codes.Internal, "Insert: %v", err)
	}

	{
		err = s.es.GetSync().setNodeUpdated(ctx, time.Now())
		if err != nil {
			return status.Errorf(codes.Internal, "Insert: %v", err)
		}

		err = s.es.GetSync().setPinUpdated(ctx, time.Now())
		if err != nil {
			return status.Errorf(codes.Internal, "Insert: %v", err)
		}
	}

	return nil
}

func (s *cloneService) const_(ctx context.Context, db bun.IDB, constID string) error {
	var err error

	item := model.Const{
		ID: constID,
	}

	err = db.NewSelect().Model(&item).WherePK().Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return status.Errorf(codes.NotFound, "Query: %v", err)
		}

		return status.Errorf(codes.Internal, "Query: %v", err)
	}

	item.ID = util.RandomID()
	item.Name = fmt.Sprintf("%v_clone_%v", item.Name, randNameSuffix())

	item.Created = time.Now()
	item.Updated = time.Now()

	_, err = db.NewInsert().Model(&item).Exec(ctx)
	if err != nil {
		return status.Errorf(codes.Internal, "Insert: %v", err)
	}

	{
		err = s.es.GetSync().setNodeUpdated(ctx, time.Now())
		if err != nil {
			return status.Errorf(codes.Internal, "Insert: %v", err)
		}

		err = s.es.GetSync().setConstUpdated(ctx, time.Now())
		if err != nil {
			return status.Errorf(codes.Internal, "Insert: %v", err)
		}
	}

	return nil
}

func randNameSuffix() string {
	buf := new(bytes.Buffer)

	random := rand.Uint32()
	binary.Write(buf, binary.BigEndian, random)

	return hex.EncodeToString(buf.Bytes())
}
