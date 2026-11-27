package config

import (
	"math/big"
	"strings"
	"testing"
)

func TestWarningsFlagAMinimumFeeOfZero(t *testing.T) {
	for _, tc := range []struct {
		name   string
		minFee int64
		warn   bool
	}{
		{"zero", 0, true},
		{"negative", -1, true},
		{"one", 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			warnings := (&Config{MinFee: big.NewInt(tc.minFee)}).Warnings()
			got := false
			for _, w := range warnings {
				if strings.Contains(w, "RELAYER_MIN_FEE") {
					got = true
				}
			}
			if got != tc.warn {
				t.Fatalf("RELAYER_MIN_FEE=%d: warning %v, want %v (warnings %q)", tc.minFee, got, tc.warn, warnings)
			}
		})
	}
}
