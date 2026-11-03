package engine

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// tagEventsABI is the private-tag registries' events, which have no ABI
// file in contracts/abis.
const tagEventsABI = `[
 {"type":"event","name":"TagPublished","inputs":[
  {"name":"blockNumber","type":"uint256","indexed":true},
  {"name":"tag","type":"bytes32","indexed":true},
  {"name":"publisher","type":"address","indexed":true},
  {"name":"ctxt","type":"bytes","indexed":false}]},
 {"type":"event","name":"ChannelOpened","inputs":[
  {"name":"channelIdx","type":"uint256","indexed":true},
  {"name":"sender","type":"address","indexed":true}]}
]`

// eventsByTopic indexes the events of abis by their topic.
func eventsByTopic(abis ...abi.ABI) map[common.Hash]abi.Event {
	m := map[common.Hash]abi.Event{}
	for _, a := range abis {
		for _, ev := range a.Events {
			m[ev.ID] = ev
		}
	}
	return m
}

// contractName is the diagram node, or token, deployed at addr, or its address.
func (c *chain) contractName(addr common.Address) string {
	switch addr {
	case c.dvpAddr:
		return NodeDvp
	case c.vaultAddr:
		return NodeVault
	case c.usdrVaultAddr:
		return NodeUsdrVault
	case c.erc20Addr:
		return "RaylsERC20"
	case c.usdrErc20Addr:
		return "UsdrERC20"
	case c.registryAddr:
		return NodeUserRegistry
	case c.tagRegistryAddr:
		return NodeTagRegistry
	case c.channelRegistryAddr:
		return NodeTagChannelRegistry
	}
	return addr.Hex()
}

// receiptOf is the receipt hop detail of the mined transaction tx.
func (c *chain) receiptOf(ctx context.Context, tx common.Hash) (HopDetail, error) {
	r, err := c.client.TransactionReceipt(ctx, tx)
	if err != nil {
		return HopDetail{}, fmt.Errorf("receipt for %s: %w", tx.Hex(), err)
	}
	return c.receipt(r), nil
}

// receipt is the receipt hop detail of r: its transaction, block, gas and
// every event it logged, decoded where the event is one the demo's
// contracts declare.
func (c *chain) receipt(r *ethtypes.Receipt) HopDetail {
	d := HopDetail{Kind: DetailReceipt, TxHash: r.TxHash.Hex(), Block: r.BlockNumber.Uint64(), GasUsed: r.GasUsed, Events: []LoggedEvent{}}
	for _, l := range r.Logs {
		d.Events = append(d.Events, c.decodeLog(l))
	}
	return d
}

// decodeLog is l as an event: its contract, name and arguments; a log of an
// event the demo doesn't know keeps its raw topics and data.
func (c *chain) decodeLog(l *ethtypes.Log) LoggedEvent {
	out := LoggedEvent{Contract: c.contractName(l.Address), Address: l.Address.Hex()}
	var ev abi.Event
	ok := len(l.Topics) > 0
	if ok {
		ev, ok = c.events[l.Topics[0]]
	}
	if ok {
		values, err := ev.Inputs.NonIndexed().UnpackValues(l.Data)
		ok = err == nil && len(ev.Inputs)-len(ev.Inputs.NonIndexed()) == len(l.Topics)-1
		if ok {
			ti, vi := 1, 0
			for _, in := range ev.Inputs {
				var v string
				if in.Indexed {
					v = topicValue(in.Type, l.Topics[ti])
					ti++
				} else {
					v = argValue(values[vi])
					vi++
				}
				out.Args = append(out.Args, EventArg{Name: in.Name, Value: v})
			}
			out.Name = ev.Name
			return out
		}
	}
	out.Name = "unknown event"
	for i, t := range l.Topics {
		out.Args = append(out.Args, EventArg{Name: fmt.Sprintf("topic%d", i), Value: t.Hex()})
	}
	out.Args = append(out.Args, EventArg{Name: "data", Value: "0x" + hex.EncodeToString(l.Data)})
	return out
}

// topicValue is an indexed argument of type t as its topic carries it.
func topicValue(t abi.Type, topic common.Hash) string {
	switch t.T {
	case abi.UintTy, abi.IntTy:
		return argValue(topic.Big())
	case abi.AddressTy:
		return common.BytesToAddress(topic.Bytes()).Hex()
	}
	return topic.Hex()
}

// argValue is an event argument as the page shows it: a number that fits in
// 64 bits in decimal, a larger one as a field element, bytes as 0x-hex.
func argValue(v any) string {
	switch x := v.(type) {
	case *big.Int:
		if x.IsUint64() {
			return x.String()
		}
		return fieldHex(x)
	case []byte:
		return "0x" + hex.EncodeToString(x)
	case [32]byte:
		return "0x" + hex.EncodeToString(x[:])
	case common.Address:
		return x.Hex()
	case string:
		return x
	}
	return strings.TrimSpace(fmt.Sprint(v))
}
