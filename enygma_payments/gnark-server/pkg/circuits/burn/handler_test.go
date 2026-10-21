package burn

import (
	"bytes"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	utils "enygma_payments/gnark-server/utils"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark/backend/groth16"
	groth16_bn254 "github.com/consensys/gnark/backend/groth16/bn254"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/gin-gonic/gin"
)

// TestHandler_ReturnsTheProofsPublicWitness drives the burn prove handler
// end to end with throwaway keys from a fresh setup: the public signals it
// returns must be the proof's own public witness, and the returned proof
// plus those signals must verify against the throwaway verifying key.
func TestHandler_ReturnsTheProofsPublicWitness(t *testing.T) {
	solver.RegisterHint(utils.ModHint)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &BurnCircuit{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	dir := t.TempDir()
	pkPath, vkPath := filepath.Join(dir, "burn.pk"), filepath.Join(dir, "burn.vk")
	if err := utils.SavingFiles(pkPath, vkPath, pk, vk); err != nil {
		t.Fatalf("save keys: %v", err)
	}

	w := burnWitness(t,
		big.NewInt(424242), // sk
		big.NewInt(67890),  // prevR
		big.NewInt(500),    // previousBalance
		big.NewInt(100),    // amount
		big.NewInt(12345),  // blockNumber
	)
	dec := func(v frontend.Variable) string { return v.(*big.Int).String() }
	body, err := json.Marshal(BurnRequest{
		PublicKey:           dec(w.PublicKey),
		PreviousCommit:      [2]string{dec(w.PreviousCommit[0]), dec(w.PreviousCommit[1])},
		NewCommit:           [2]string{dec(w.NewCommit[0]), dec(w.NewCommit[1])},
		Amount:              dec(w.Amount),
		BlockNumber:         dec(w.BlockNumber),
		Nullifier:           dec(w.Nullifier),
		SecretKey:           dec(w.SecretKey),
		PreviousBalance:     dec(w.PreviousBalance),
		PreviousRandomValue: dec(w.PreviousRandomValue),
		DomainId:            dec(w.DomainId),
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/proof/burn", NewHandler(pkPath, vkPath))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/proof/burn", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var out BurnOutput
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	full, err := frontend.NewWitness(w, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("witness: %v", err)
	}
	pub, err := full.Public()
	if err != nil {
		t.Fatalf("public witness: %v", err)
	}
	want := pub.Vector().(fr.Vector)
	if len(out.PublicSignal) != len(want) {
		t.Fatalf("got %d public signals, want %d", len(out.PublicSignal), len(want))
	}
	for i := range want {
		if out.PublicSignal[i].Cmp(want[i].BigInt(new(big.Int))) != 0 {
			t.Errorf("public signal %d = %s, want %s", i, out.PublicSignal[i], want[i].String())
		}
	}

	if err := groth16.Verify(proofFromRemix(t, out.Proof), vk, publicWitness(t, out.PublicSignal)); err != nil {
		t.Fatalf("returned proof and signals do not verify: %v", err)
	}
}

// proofFromRemix rebuilds a proof from the handler's 8-element Remix order:
// A.x, A.y, B.x.A1, B.x.A0, B.y.A1, B.y.A0, C.x, C.y.
func proofFromRemix(t *testing.T, p []*big.Int) groth16.Proof {
	t.Helper()
	if len(p) != 8 {
		t.Fatalf("proof has %d elements, want 8", len(p))
	}
	var proof groth16_bn254.Proof
	proof.Ar.X.SetBigInt(p[0])
	proof.Ar.Y.SetBigInt(p[1])
	proof.Bs.X.A1.SetBigInt(p[2])
	proof.Bs.X.A0.SetBigInt(p[3])
	proof.Bs.Y.A1.SetBigInt(p[4])
	proof.Bs.Y.A0.SetBigInt(p[5])
	proof.Krs.X.SetBigInt(p[6])
	proof.Krs.Y.SetBigInt(p[7])
	return &proof
}

func publicWitness(t *testing.T, signals []*big.Int) witness.Witness {
	t.Helper()
	w, err := witness.New(ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("new witness: %v", err)
	}
	values := make(chan any, len(signals))
	for _, s := range signals {
		values <- s
	}
	close(values)
	if err := w.Fill(len(signals), 0, values); err != nil {
		t.Fatalf("fill public witness: %v", err)
	}
	return w
}
