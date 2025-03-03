package shiftime

import (
	"github.com/snple/beacon/pb"
)

func Node(item *pb.Node) {
	if item != nil {
		item.Created = item.Created / 1000
		item.Updated = item.Updated / 1000
		item.Deleted = item.Deleted / 1000
	}
}

func Nodes(items []*pb.Node) {
	for _, item := range items {
		Node(item)
	}
}

func Slot(item *pb.Slot) {
	if item != nil {
		item.Created = item.Created / 1000
		item.Updated = item.Updated / 1000
		item.Deleted = item.Deleted / 1000
	}
}

func Slots(items []*pb.Slot) {
	for _, item := range items {
		Slot(item)
	}
}

func Wire(item *pb.Wire) {
	if item != nil {
		item.Created = item.Created / 1000
		item.Updated = item.Updated / 1000
		item.Deleted = item.Deleted / 1000
	}
}

func Wires(items []*pb.Wire) {
	for _, item := range items {
		Wire(item)
	}
}

func Pin(item *pb.Pin) {
	if item != nil {
		item.Created = item.Created / 1000
		item.Updated = item.Updated / 1000
		item.Deleted = item.Deleted / 1000
	}
}

func Pins(items []*pb.Pin) {
	for _, item := range items {
		Pin(item)
	}
}

func PinValue(item *pb.PinValue) {
	if item != nil {
		item.Updated = item.Updated / 1000
	}
}

func PinNameValue(item *pb.PinNameValue) {
	if item != nil {
		item.Updated = item.Updated / 1000
	}
}

func PinNameValues(items []*pb.PinNameValue) {
	for _, item := range items {
		PinNameValue(item)
	}
}

func User(item *pb.User) {
	if item != nil {
		item.Created = item.Created / 1000
		item.Updated = item.Updated / 1000
		item.Deleted = item.Deleted / 1000
	}
}

func Users(items []*pb.User) {
	for _, item := range items {
		User(item)
	}
}

func Const(item *pb.Const) {
	if item != nil {
		item.Created = item.Created / 1000
		item.Updated = item.Updated / 1000
		item.Deleted = item.Deleted / 1000
	}
}

func Consts(items []*pb.Const) {
	for _, item := range items {
		Const(item)
	}
}

func ConstValue(item *pb.ConstValue) {
	if item != nil {
		item.Updated = item.Updated / 1000
	}
}

func ConstNameValue(item *pb.ConstNameValue) {
	if item != nil {
		item.Updated = item.Updated / 1000
	}
}

func ConstNameValues(items []*pb.ConstNameValue) {
	for _, item := range items {
		ConstNameValue(item)
	}
}
