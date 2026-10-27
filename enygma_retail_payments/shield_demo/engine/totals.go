package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"time"
)

// Totals sum up what a run has cost so far.
type Totals struct {
	// Txs and Gas count every transaction mined since the run started — gas
	// funding, registrations, deposits, tag channels, tags and transfers —
	// so they assume the run is the only one sending to the chain.
	Txs int    `json:"txs"`
	Gas uint64 `json:"gas"`
	// ProveMs is the time the gnark server spent on the transfers' proofs
	// (Payment, Payment2in and UsdrFee), RelayMs the transfers' relay
	// round-trips: the per-transfer figures in the UI's tables, added up.
	ProveMs int64 `json:"proveMs"`
	RelayMs int64 `json:"relayMs"`
}

// Totals returns the run's totals so far, reading transactions and gas from
// the blocks mined since the run started.
func (e *Engine) Totals(ctx context.Context) (Totals, error) {
	t := Totals{ProveMs: e.proving.Milliseconds(), RelayMs: e.relaying.Milliseconds()}
	if e.chain == nil {
		return t, nil
	}
	head, err := e.chain.client.BlockNumber(ctx)
	if err != nil {
		return t, err
	}
	for n := e.startBlock; n <= head; n++ {
		h, err := e.chain.client.HeaderByNumber(ctx, new(big.Int).SetUint64(n))
		if err != nil {
			return t, fmt.Errorf("block %d: %w", n, err)
		}
		count, err := e.chain.client.TransactionCount(ctx, h.Hash())
		if err != nil {
			return t, fmt.Errorf("block %d: %w", n, err)
		}
		t.Txs += int(count)
		t.Gas += h.GasUsed
	}
	return t, nil
}

// showTotals is the run's last step: it shows the totals.
func (e *Engine) showTotals(ctx context.Context, s *step) error {
	t, err := e.publishTotals(ctx, s)
	if err != nil {
		return err
	}
	s.done("%d transactions · %s gas · %s proving · %s relaying",
		t.Txs, formatNum(t.Gas), seconds(t.ProveMs), seconds(t.RelayMs))
	return nil
}

// publishTotals sends the totals to the UI's totals card.
func (e *Engine) publishTotals(ctx context.Context, s *step) (Totals, error) {
	t, err := e.Totals(ctx)
	if err != nil {
		return t, fmt.Errorf("totals: %w", err)
	}
	data, _ := json.Marshal(t)
	s.proto("totals", string(data))
	return t, nil
}

// seconds formats ms as seconds, e.g. 12345 → "12.3 s".
func seconds(ms int64) string {
	return fmt.Sprintf("%.1f s", (time.Duration(ms) * time.Millisecond).Seconds())
}
