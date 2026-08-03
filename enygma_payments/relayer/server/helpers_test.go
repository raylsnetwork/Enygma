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
