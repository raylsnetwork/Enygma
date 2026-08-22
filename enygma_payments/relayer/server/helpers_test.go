package server

import (
	"math/big"
	"os"
	"path/filepath"
	"testing"
)

// ── parseProof8 ───────────────────────────────────────────────────────────────

func TestParseProof8_Valid(t *testing.T) {
	var raw [8]string
	for i := range raw {
		raw[i] = big.NewInt(int64(i + 1)).String()
	}
	got, err := parseProof8(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, v := range got {
		if v.Int64() != int64(i+1) {
			t.Errorf("[%d]: got %v, want %d", i, v, i+1)
		}
	}
}

func TestParseProof8_InvalidDecimal(t *testing.T) {
	var raw [8]string
	for i := range raw {
		raw[i] = "1"
	}
	raw[3] = "not-a-number"
	if _, err := parseProof8(raw); err == nil {
		t.Fatal("expected error for invalid decimal, got nil")
	}
}

// ── parseCommitments ──────────────────────────────────────────────────────────

func TestParseCommitments_Valid(t *testing.T) {
	raw := [][]string{
		{"100", "200"},
		{"300", "400"},
	}
	got, err := parseCommitments(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0].C1.Int64() != 100 || got[0].C2.Int64() != 200 {
		t.Errorf("commitment[0]: got {%v %v}, want {100 200}", got[0].C1, got[0].C2)
	}
	if got[1].C1.Int64() != 300 || got[1].C2.Int64() != 400 {
		t.Errorf("commitment[1]: got {%v %v}, want {300 400}", got[1].C1, got[1].C2)
	}
}

func TestParseCommitments_WrongPairSize(t *testing.T) {
	if _, err := parseCommitments([][]string{{"1", "2", "3"}}); err == nil {
		t.Fatal("expected error for 3-element pair")
	}
	if _, err := parseCommitments([][]string{{"1"}}); err == nil {
		t.Fatal("expected error for 1-element pair")
	}
}

func TestParseCommitments_InvalidDecimal(t *testing.T) {
	if _, err := parseCommitments([][]string{{"1", "bad"}}); err == nil {
		t.Fatal("expected error for invalid C2")
	}
	if _, err := parseCommitments([][]string{{"bad", "1"}}); err == nil {
		t.Fatal("expected error for invalid C1")
	}
}

// ── int64sToBI ────────────────────────────────────────────────────────────────

func TestInt64sToBI(t *testing.T) {
	got := int64sToBI([]int64{1, 2, 3})
	if len(got) != 3 || got[0].Int64() != 1 || got[1].Int64() != 2 || got[2].Int64() != 3 {
		t.Errorf("unexpected result: %v", got)
	}
	if len(int64sToBI(nil)) != 0 {
		t.Error("expected empty slice for nil input")
	}
}

// ── bumpGasPrice ──────────────────────────────────────────────────────────────

func TestBumpGasPrice_AlwaysStrictlyGreater(t *testing.T) {
	// Covers the full range where the 112/100 integer-division bump alone
	// would be a no-op (1..8 wei), the boundary where it starts working on
	// its own (9 wei), and realistic higher values.
	for _, gasPrice := range []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 100, 1_000_000_000} {
		gp := big.NewInt(gasPrice)
		bumped := bumpGasPrice(gp)
		if bumped.Cmp(gp) <= 0 {
			t.Errorf("bumpGasPrice(%d) = %v, want strictly greater than %d", gasPrice, bumped, gasPrice)
		}
	}
}

func TestBumpGasPrice_LowPriceFallsBackToPlusOne(t *testing.T) {
	// At gasPrice=1, 1*112/100 truncates to 1 (no-op) — must fall back to
	// exactly gasPrice+1, not silently stay at gasPrice.
	got := bumpGasPrice(big.NewInt(1))
	if got.Int64() != 2 {
		t.Errorf("bumpGasPrice(1) = %v, want 2 (gasPrice+1 fallback)", got)
	}
}

func TestBumpGasPrice_HighPriceUsesPercentageBump(t *testing.T) {
	// At realistic gas prices the 12% bump dominates the +1 floor.
	got := bumpGasPrice(big.NewInt(1_000_000_000))
	want := big.NewInt(1_120_000_000)
	if got.Cmp(want) != 0 {
		t.Errorf("bumpGasPrice(1e9) = %v, want %v", got, want)
	}
}

// ── readAddressJSON ───────────────────────────────────────────────────────────

func TestReadAddressJSON_Valid(t *testing.T) {
	f := filepath.Join(t.TempDir(), "address.json")
	if err := os.WriteFile(f, []byte(`{"address":"0xDeAdBeEf"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readAddressJSON(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "0xDeAdBeEf" {
		t.Errorf("got %q, want 0xDeAdBeEf", got)
	}
}

func TestReadAddressJSON_Missing(t *testing.T) {
	if _, err := readAddressJSON("/nonexistent/path/address.json"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadAddressJSON_EmptyAddress(t *testing.T) {
	f := filepath.Join(t.TempDir(), "address.json")
	if err := os.WriteFile(f, []byte(`{"address":""}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readAddressJSON(f); err == nil {
		t.Fatal("expected error for empty address field")
	}
}

func TestReadAddressJSON_InvalidJSON(t *testing.T) {
	f := filepath.Join(t.TempDir(), "address.json")
	if err := os.WriteFile(f, []byte(`not json`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readAddressJSON(f); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
