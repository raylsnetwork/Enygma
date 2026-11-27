package coordinator

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"path/filepath"
	"testing"

	"enygma_ceremony/ceremony"
	"enygma_ceremony/internal/testcircuits"
)

// TestStatusNamesPhase1 (audit H1): a Phase 2's status says which SRS it
// starts from, what that SRS says it was sealed from, and each circuit's
// constraint system, for participants to compare with the announcement.
func TestStatusNamesPhase1(t *testing.T) {
	work := t.TempDir()
	p := func(name string) string { return filepath.Join(work, name) }
	if _, err := ceremony.NewPhase1(p("p1-0"), 8); err != nil {
		t.Fatal(err)
	}
	if _, err := ceremony.Contribute(p("p1-0"), p("p1-1")); err != nil {
		t.Fatal(err)
	}
	r, err := ceremony.SealPhase1(p("srs"), ceremony.Beacon{Value: bytes.Repeat([]byte{7}, 32)}, ceremony.Chain{Paths: []string{p("p1-1")}, Head: sha256.Sum256(readFile(t, p("p1-1")))})
	if err != nil {
		t.Fatal(err)
	}
	r1cs := t.TempDir()
	if err := testcircuits.WriteCircuit(r1cs, "Cubic", &testcircuits.Cubic{}); err != nil {
		t.Fatal(err)
	}
	circuits, err := ceremony.CircuitsIn(r1cs)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if _, err := ceremony.NewPhase2(filepath.Join(dir, "state-0"), "enygma_retail_payments", ceremony.SRS{Path: p("srs"), SHA256: r.Hash, Rehearsal: true}, circuits); err != nil {
		t.Fatal(err)
	}
	srv, err := New(Config{Dir: dir, Auth: AuthOpen, Phase1: r.Phase1, Log: log.New(new(bytes.Buffer), "", 0)})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Release()
	st := srv.status()
	head := sha256.Sum256(readFile(t, p("p1-1")))
	cubicHash := sha256.Sum256(readFile(t, filepath.Join(r1cs, "Cubic.r1cs")))
	if st.SRS != hex.EncodeToString(r.Hash[:]) || len(st.R1CS) != 1 || st.R1CS[0] != hex.EncodeToString(cubicHash[:]) ||
		st.Phase1 == nil || st.Phase1.Head != hex.EncodeToString(head[:]) || st.Phase1.Contributions != 1 || st.Phase1.DrandRound != 0 {
		t.Fatalf("status %+v, Phase 1 %+v", st, st.Phase1)
	}
}
