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
