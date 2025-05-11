package main

import (
	"github.com/snple/beacon/cmd/core/funcs"
)

func main() {
	root := funcs.NewRoot()
	root.Execute()
}
