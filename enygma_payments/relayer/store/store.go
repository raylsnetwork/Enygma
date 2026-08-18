// Package store persists the relayer's request-idempotency state to disk,
// so a crash between broadcasting a transaction and returning the response
// doesn't leave a client unable to tell what happened to their request.
//
// Before this package existed, dedup lived entirely in an in-memory
// sync.Map: fine for rejecting a concurrent duplicate, but gone on restart.
// A crash mid-flight meant the next identical request would look brand new
// — either double-submitting (wasted gas, though the contract's nullifier
// check makes it harmless beyond cost) or, worse, a client with no way to
// learn whether their original request actually landed.
//
// This does NOT reconcile with chain state on startup (e.g. by querying
// whether a nullifier was already consumed) — Enygma.sol has no public
// getter for that, and adding one is out of scope here. Instead, a stuck
// "pending" record simply expires after a configurable staleness window,
// after which the key is free to retry. That's a real limitation: a crash
// during the window means a legitimate retry is blocked for up to that long.
// It is not a limitation once a request reaches a terminal state — a mined
// success is remembered indefinitely (until deliberately pruned) and replayed
// idempotently; a failure clears immediately so retries aren't blocked at all.
package store

import (
	"encoding/json"
	"fmt"
	"time"

	"go.etcd.io/bbolt"
)

// Status is a request's lifecycle state.
type Status string

const (
	StatusPending Status = "pending" // claimed, broadcast not yet confirmed
	StatusMined   Status = "mined"   // terminal success — replay this result
)

// Record is what's persisted per dedup key.
type Record struct {
	Status      Status    `json:"status"`
	TxHash      string    `json:"txHash,omitempty"`
	BlockNumber uint64    `json:"blockNumber,omitempty"`
	GasUsed     uint64    `json:"gasUsed,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

var bucketName = []byte("requests")

// Store wraps a bbolt (embedded, pure-Go, no CGo) key-value file.
type Store struct {
	db *bbolt.DB
}

// Open creates or opens the store file at path, creating the bucket if needed.
func Open(path string) (*Store, error) {
	db, err := bbolt.Open(path, 0600, &bbolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketName)
		return err
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("init bucket: %w", err)
	}
	return &Store{db: db}, nil
}

// Close releases the underlying file. Safe to call once at shutdown.
func (s *Store) Close() error {
	return s.db.Close()
}

// TryBeginPending attempts to claim key for a new in-flight submission.
//
//   - blocked=false: key is free (never seen, previously failed, or a
//     pending claim older than staleness) — a Pending record was just
//     written for it, and the caller should proceed to submit.
//   - blocked=true, cached!=nil: a prior submission for this key already
//     succeeded — the caller should return the cached result rather than
//     resubmitting (idempotent replay).
//   - blocked=true, cached==nil: another submission for this key is
//     genuinely still in flight (a Pending record fresher than staleness) —
//     the caller should reject as a duplicate.
//
// All of this happens inside one bbolt write transaction, so concurrent
// callers can't both observe "free" for the same key.
func (s *Store) TryBeginPending(key string, staleness time.Duration) (blocked bool, cached *Record, err error) {
	err = s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketName)
		raw := b.Get([]byte(key))
		if raw != nil {
			var rec Record
			if jsonErr := json.Unmarshal(raw, &rec); jsonErr == nil {
				switch rec.Status {
				case StatusMined:
					blocked = true
					cached = &rec
					return nil
				case StatusPending:
					if time.Since(rec.UpdatedAt) < staleness {
						blocked = true
						return nil
					}
					// stale — fall through and reclaim the key
				}
			}
		}

		newRec := Record{Status: StatusPending, UpdatedAt: time.Now()}
		buf, jsonErr := json.Marshal(newRec)
		if jsonErr != nil {
			return jsonErr
		}
		return b.Put([]byte(key), buf)
	})
	return blocked, cached, err
}

// MarkMined records a terminal success. Future TryBeginPending calls for
// this key return it as a cached replay instead of allowing resubmission.
func (s *Store) MarkMined(key, txHash string, blockNumber, gasUsed uint64) error {
	rec := Record{
		Status:      StatusMined,
		TxHash:      txHash,
		BlockNumber: blockNumber,
		GasUsed:     gasUsed,
		UpdatedAt:   time.Now(),
	}
	buf, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketName).Put([]byte(key), buf)
	})
}

// MarkFailed clears key's record entirely, so the next request for it is
// treated as free to retry immediately rather than waiting out staleness.
func (s *Store) MarkFailed(key string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketName).Delete([]byte(key))
	})
}
