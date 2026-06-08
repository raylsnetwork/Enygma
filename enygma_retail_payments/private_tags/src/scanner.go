package tags

// ScanCursor tracks incremental scan progress for both registries.
// Persist it to disk between wallet polls so the wallet never rescans old data.
//
// Typical usage:
//
//	cursor, _ := tags.LoadScanCursor("wallet.cursor")
//	tip, _    := client.BlockNumber(ctx)
//
//	tagMatches, cursor, _ = tags.ScanBlocksFromCursor(client, tagReg, channels, cursor, tip)
//	newChannels, cursor, _ = tags.ScanChannelsFromCursor(client, chReg, dk, cursor, 100)
//
//	cursor.Save("wallet.cursor")
//
// Deleting the cursor file is safe — the wallet rescans from genesis at the
// cost of extra RPC calls.

import (
	"context"
	"crypto/mlkem"
	"encoding/json"
	"fmt"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// tagPublishedSig is the keccak256 topic of TagPublished(uint256,bytes32,address,bytes).
var tagPublishedSig = crypto.Keccak256Hash([]byte("TagPublished(uint256,bytes32,address,bytes)"))

// ── Cursor ────────────────────────────────────────────────────────────────────

// ScanCursor holds the watermarks for a single wallet's scan progress.
type ScanCursor struct {
	LastTagBlock   uint64 `json:"lastTagBlock"`   // next tag scan starts at +1
	LastChannelIdx uint64 `json:"lastChannelIdx"` // next channel scan starts here
}

// NewScanCursor returns a cursor starting from genesis (block 0, channel 0).
func NewScanCursor() *ScanCursor { return &ScanCursor{} }

// Save writes the cursor to a JSON file at path.
func (sc *ScanCursor) Save(path string) error {
	data, err := json.Marshal(sc)
	if err != nil {
		return fmt.Errorf("marshal cursor: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// LoadScanCursor reads a cursor from path.
// If the file does not exist, returns a zero cursor (start from genesis).
func LoadScanCursor(path string) (*ScanCursor, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NewScanCursor(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read cursor: %w", err)
	}
	var sc ScanCursor
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, fmt.Errorf("parse cursor: %w", err)
	}
	return &sc, nil
}

// ── Tag scanning ──────────────────────────────────────────────────────────────

// ScanBlocksFromCursor fetches all TagPublished events from
// [cursor.LastTagBlock+1, toBlock] in a single eth_getLogs call and returns
// every entry that matches one of the caller's channels.
//
// One RPC call covers any block range — no per-block loop.
func ScanBlocksFromCursor(
	client *ethclient.Client,
	registryAddr common.Address,
	channels []Channel,
	cursor *ScanCursor,
	toBlock uint64,
) (matches []ScannedTag, next *ScanCursor, err error) {
	next = &ScanCursor{LastTagBlock: cursor.LastTagBlock, LastChannelIdx: cursor.LastChannelIdx}
	if cursor.LastTagBlock >= toBlock {
		return nil, next, nil
	}

	logs, err := client.FilterLogs(context.Background(), ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(cursor.LastTagBlock + 1),
		ToBlock:   new(big.Int).SetUint64(toBlock),
		Addresses: []common.Address{registryAddr},
		Topics:    [][]common.Hash{{tagPublishedSig}},
	})
	if err != nil {
		return nil, next, fmt.Errorf("FilterLogs: %w", err)
	}

	// ctxtArg decodes the non-indexed `ctxt bytes` field from log.Data.
	bytesType, _ := abi.NewType("bytes", "", nil)
	ctxtArg := abi.Arguments{{Type: bytesType}}

	for _, log := range logs {
		// Topics layout: [0]=sig [1]=blockNumber [2]=tag [3]=publisher
		if len(log.Topics) < 4 {
			continue
		}
		var tag [32]byte
		copy(tag[:], log.Topics[2].Bytes())
		publisher := common.BytesToAddress(log.Topics[3].Bytes())

		decoded, err := ctxtArg.Unpack(log.Data)
		if err != nil {
			continue
		}
		ctxt, _ := decoded[0].([]byte)

		for _, ch := range channels {
			expected, err := DeriveTag(log.BlockNumber, ch.PkSpend, ch.SharedSecret)
			if err != nil {
				continue
			}
			if tag == expected {
				matches = append(matches, ScannedTag{
					BlockNumber:  log.BlockNumber,
					Entry:        TagEntry{Publisher: publisher, Tag: tag, Ctxt: ctxt},
					SharedSecret: ch.SharedSecret,
				})
			}
		}
	}

	next.LastTagBlock = toBlock
	return matches, next, nil
}

// ── Channel scanning ──────────────────────────────────────────────────────────

// ScanChannelsFromCursor checks up to maxNew new channel records starting from
// cursor.LastChannelIdx and returns any channels addressed to dk.
//
// The cursor advances past all examined records so subsequent calls resume
// from the right position even when no channels matched.
func ScanChannelsFromCursor(
	client *ethclient.Client,
	channelRegistryAddr common.Address,
	dk *mlkem.DecapsulationKey768,
	cursor *ScanCursor,
	maxNew uint64,
) (found []FoundChannel, next *ScanCursor, err error) {
	next = &ScanCursor{LastTagBlock: cursor.LastTagBlock, LastChannelIdx: cursor.LastChannelIdx}

	a, err := loadChannelRegistryABI()
	if err != nil {
		return nil, next, err
	}
	c := bind.NewBoundContract(channelRegistryAddr, a, client, client, client)
	var countOut []interface{}
	if err := c.Call(&bind.CallOpts{}, &countOut, "getChannelCount"); err != nil {
		return nil, next, fmt.Errorf("getChannelCount: %w", err)
	}
	total := countOut[0].(*big.Int).Uint64()

	fromIdx := cursor.LastChannelIdx
	endIdx := fromIdx + maxNew
	if endIdx > total {
		endIdx = total
	}

	found, err = ScanChannels(client, channelRegistryAddr, dk, fromIdx, endIdx-fromIdx)
	if err != nil {
		return nil, next, err
	}
	next.LastChannelIdx = endIdx
	return found, next, nil
}
