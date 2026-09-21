package main

import "github.com/ethereum/go-ethereum/common"

var enygmaAddress = common.HexToAddress("0x32B025d7f97fC9A9BDB4B57BB233Daf1D8bd6b82")

func EnygmaAddress() common.Address {
	return enygmaAddress
}
