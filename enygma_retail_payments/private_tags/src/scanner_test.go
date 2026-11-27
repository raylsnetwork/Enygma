package tags

import (
	"bytes"
	"crypto/mlkem"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// A node that reports two channel records but fails every getChannel call,
// as a dropped connection would: the scan returns the error and its cursor
// stays put, rather than moving past records it never read.
func TestScanChannelsKeepsCursorOnRPCError(t *testing.T) {
	getChannelCount := crypto.Keccak256([]byte("getChannelCount()"))[:4]
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Params []struct {
				Data  hexutil.Bytes `json:"data"`
				Input hexutil.Bytes `json:"input"`
			} `json:"params"`
		}
		json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck
		resp := map[string]any{"jsonrpc": "2.0", "id": req.ID}
		call := append(req.Params[0].Input, req.Params[0].Data...)
		if bytes.HasPrefix(call, getChannelCount) {
			resp["result"] = hexutil.Bytes(common.LeftPadBytes([]byte{2}, 32))
		} else {
			resp["error"] = map[string]any{"code": -32000, "message": "node unavailable"}
		}
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer srv.Close()
	client, err := ethclient.Dial(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	dk, err := mlkem.GenerateKey768()
	if err != nil {
		t.Fatal(err)
	}

	_, next, err := ScanChannelsFromCursor(client, common.HexToAddress("0xc4"), dk, 2, &ScanCursor{}, 10)
	if err == nil {
		t.Fatal("scan succeeded with every channel record unreadable")
	}
	if next.LastChannelIdx != 0 {
		t.Fatalf("cursor moved to channel %d on an RPC error, want 0", next.LastChannelIdx)
	}
}

// N tag logs across two blocks cost one Poseidon tag per channel per block,
// not one per channel per log: the scanner derives a block's tags once and
// matches every log in that block against them. The first block is 0, the
// cache's zero value, and one channel's spend key is past the field, so its
// tag cannot be derived and must match nothing, not a zero tag.
func TestMatchTagLogsDerivesEachTagOncePerBlock(t *testing.T) {
	calls := 0
	orig := deriveTag
	deriveTag = func(b uint64, pk *big.Int, ss []byte) ([32]byte, error) {
		calls++
		return orig(b, pk, ss)
	}
	defer func() { deriveTag = orig }()

	channels := []Channel{
		{SharedSecret: bytes.Repeat([]byte{1}, 32), PkSpend: big.NewInt(11)},
		{SharedSecret: bytes.Repeat([]byte{2}, 32), PkSpend: big.NewInt(22)},
		{SharedSecret: bytes.Repeat([]byte{3}, 32), PkSpend: big.NewInt(33)},
		{SharedSecret: bytes.Repeat([]byte{4}, 32), PkSpend: new(big.Int).Lsh(big.NewInt(1), 255)},
	}
	if _, err := DeriveTag(8, channels[3].PkSpend, channels[3].SharedSecret); err == nil {
		t.Fatal("channel 3's tag derives; it must not, to stand for a channel the scanner cannot tag")
	}
	wanted, err := DeriveTag(8, channels[1].PkSpend, channels[1].SharedSecret)
	if err != nil {
		t.Fatal(err)
	}
	ctxt, err := abi.Arguments{{Type: mustBytesType(t)}}.Pack([]byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	tagLog := func(block uint64, tag common.Hash) types.Log {
		return types.Log{
			BlockNumber: block,
			Topics:      []common.Hash{tagPublishedSig, {}, tag, {}},
			Data:        ctxt,
		}
	}
	logs := []types.Log{
		tagLog(0, common.Hash{0xa}), tagLog(0, common.Hash{0xb}),
		tagLog(0, common.Hash{0xc}), tagLog(0, common.Hash{0xd}),
		tagLog(8, common.Hash{}), tagLog(8, common.Hash(wanted)),
	}

	calls = 0
	matches := matchTagLogs(logs, channels)
	if want := 2 * len(channels); calls != want {
		t.Fatalf("%d logs in 2 blocks took %d DeriveTag calls, want %d (once per channel per block)", len(logs), calls, want)
	}
	if len(matches) != 1 || matches[0].BlockNumber != 8 || matches[0].PkSpend != channels[1].PkSpend {
		t.Fatalf("matches = %+v, want channel 1's tag in block 8", matches)
	}
}

// A channel record whose bitmap sets bits only at or above the user count
// names no registered user, so the scanner skips it without decapsulating,
// even when it is in fact addressed to the scanning wallet. The same record
// with a bit below the count is found, so the skip is the bitmap's doing.
// A bitmap shorter than the user count names no user past its end.
func TestScanChannelsSkipsBitmapAboveUserCount(t *testing.T) {
	dk, err := mlkem.GenerateKey768()
	if err != nil {
		t.Fatal(err)
	}
	const userCount = 4
	_, c1, c2, _, err := PrepareChannelSetup(dk.EncapsulationKey().Bytes(), []byte("hi"), nil, PrivacyNone, userCount, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		bitmap []byte
		want   int
	}{
		{"bits 4 and 15 only", []byte{0x10, 0x80}, 0},
		{"no bitmap bytes", []byte{}, 0},
		{"bit 3", []byte{0x08}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := channelRecordNode(t, c1, c2, tc.bitmap)
			found, err := ScanChannels(client, common.HexToAddress("0xc4"), dk, userCount, 0, 10)
			if err != nil {
				t.Fatal(err)
			}
			if len(found) != tc.want {
				t.Fatalf("bitmap %08b with %d users: found %d channels, want %d", tc.bitmap, userCount, len(found), tc.want)
			}
		})
	}
}

func mustBytesType(t *testing.T) abi.Type {
	t.Helper()
	ty, err := abi.NewType("bytes", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	return ty
}

// channelRecordNode serves a TagChannelRegistry holding one record.
func channelRecordNode(t *testing.T, c1, c2, bitmap []byte) *ethclient.Client {
	t.Helper()
	a, err := loadChannelRegistryABI()
	if err != nil {
		t.Fatal(err)
	}
	record, err := a.Methods["getChannel"].Outputs.Pack(common.HexToAddress("0x5e"), c1, c2, bitmap)
	if err != nil {
		t.Fatal(err)
	}
	getChannelCount := a.Methods["getChannelCount"].ID
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Params []struct {
				Data  hexutil.Bytes `json:"data"`
				Input hexutil.Bytes `json:"input"`
			} `json:"params"`
		}
		json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck
		call := append(req.Params[0].Input, req.Params[0].Data...)
		result := hexutil.Bytes(record)
		if bytes.HasPrefix(call, getChannelCount) {
			result = common.LeftPadBytes([]byte{1}, 32)
		}
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result}) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)
	client, err := ethclient.Dial(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return client
}
