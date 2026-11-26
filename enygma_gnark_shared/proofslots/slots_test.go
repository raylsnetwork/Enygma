package proofslots_test

import (
	"context"
	"runtime"
	"testing"
	"time"

	"enygma_gnark_shared/proofslots"
)

func TestCapacityUsesPositiveConfiguration(t *testing.T) {
	const fallback = 2
	testCapacity := runtime.GOMAXPROCS(fallback)
	t.Cleanup(func() { runtime.GOMAXPROCS(testCapacity) })
	for _, tc := range []struct {
		name  string
		value string
		want  int
	}{
		{"absent", "", fallback},
		{"invalid", "bad", fallback},
		{"zero", "0", fallback},
		{"negative", "-1", fallback},
		{"configured", "1", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GNARK_MAX_CONCURRENT_PROOFS", tc.value)
			if got := cap(proofslots.New()); got != tc.want {
				t.Fatalf("got capacity %d, want %d", got, tc.want)
			}
		})
	}
}

func TestAcquireTimesOutBeforeRequestDeadline(t *testing.T) {
	limiter := make(proofslots.Limiter, 1)
	limiter <- struct{}{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if limiter.Acquire(ctx, 10*time.Millisecond) || ctx.Err() != nil {
		t.Fatal("the slot wait must expire while the request is still active")
	}
}
