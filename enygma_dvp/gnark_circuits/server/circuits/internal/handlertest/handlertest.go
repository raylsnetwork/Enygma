// Package handlertest drives a prove handler end to end in tests: throwaway
// keys from a fresh setup, a request over httptest, and checks that the
// public signals the handler returns are the public witness it proved.
package handlertest

import (
	"bytes"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark/backend/groth16"
	groth16_bn254 "github.com/consensys/gnark/backend/groth16/bn254"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/gin-gonic/gin"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

// Keys compiles circuit, runs a throwaway setup and saves the keys where a
// handler can load them.
func Keys(t *testing.T, circuit frontend.Circuit) (pkPath, vkPath string, vk groth16.VerifyingKey) {
	t.Helper()
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	dir := t.TempDir()
	pkPath, vkPath = filepath.Join(dir, "pk.key"), filepath.Join(dir, "vk.key")
	writeKey(t, pkPath, pk)
	writeKey(t, vkPath, vk)
	return pkPath, vkPath, vk
}

func writeKey(t *testing.T, path string, key io.WriterTo) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := key.WriteTo(f); err != nil {
		t.Fatal(err)
	}
}

// Post sends request to handler and returns the proof and public signals it
// responds with. Both are accepted as JSON numbers or decimal strings.
func Post(t *testing.T, handler gin.HandlerFunc, request any) (proof, signals []*big.Int) {
	t.Helper()
	rec := serve(t, handler, request)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var out struct {
		Proof        []json.Number `json:"proof"`
		PublicSignal []json.Number `json:"publicSignal"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response %s: %v", rec.Body, err)
	}
	return bigInts(t, out.Proof), bigInts(t, out.PublicSignal)
}

// PostRejected sends request to handler and checks that it answers with a
// proving error and no public signals.
func PostRejected(t *testing.T, handler gin.HandlerFunc, request any) {
	t.Helper()
	rec := serve(t, handler, request)
	if rec.Code == http.StatusOK {
		t.Fatalf("status 200, want an error: %s", rec.Body)
	}
	var out struct {
		Error        string          `json:"error"`
		PublicSignal json.RawMessage `json:"publicSignal"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response %s: %v", rec.Body, err)
	}
	if !strings.HasPrefix(out.Error, "prove: ") {
		t.Errorf("response %s is not a proving error", rec.Body)
	}
	if out.PublicSignal != nil {
		t.Errorf("response %s carries public signals", rec.Body)
	}
}

// serve posts request to handler. A handler that panics answers 500 rather
// than ending the test binary.
func serve(t *testing.T, handler gin.HandlerFunc, request any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.POST("/proof", handler)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/proof", bytes.NewReader(body)))
	return rec
}

func bigInts(t *testing.T, ns []json.Number) []*big.Int {
	t.Helper()
	out := make([]*big.Int, len(ns))
	for i, n := range ns {
		v, ok := new(big.Int).SetString(n.String(), 10)
		if !ok {
			t.Fatalf("element %d: %q is not a decimal integer", i, n)
		}
		out[i] = v
	}
	return out
}

// AssertProvedWitness checks that signals are exactly assignment's public
// witness, and that proof verifies against them under vk.
func AssertProvedWitness(t *testing.T, assignment frontend.Circuit, vk groth16.VerifyingKey, proof, signals []*big.Int) {
	t.Helper()
	pub, err := frontend.NewWitness(assignment, ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		t.Fatalf("public witness: %v", err)
	}
	want := pub.Vector().(fr.Vector)
	if len(signals) != len(want) {
		t.Fatalf("got %d public signals, want %d", len(signals), len(want))
	}
	for i := range want {
		if signals[i].Cmp(want[i].BigInt(new(big.Int))) != 0 {
			t.Errorf("public signal %d = %s, want %s", i, signals[i], want[i].String())
		}
	}
	if err := groth16.Verify(proofFromRemix(t, proof), vk, publicWitness(t, signals)); err != nil {
		t.Fatalf("returned proof and signals do not verify: %v", err)
	}
}

// AssertStatementLen checks that a handler returned a statement of its
// circuit's statement length, as the generated enygma_dvp/layout package
// gives it.
func AssertStatementLen(t *testing.T, signals []*big.Int, statementLen int) {
	t.Helper()
	if len(signals) != statementLen {
		t.Errorf("got %d public signals, the statement length is %d", len(signals), statementLen)
	}
}

// proofFromRemix rebuilds a proof from the handlers' 8-element Remix order:
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

// Hash is Poseidon over inputs, the hash every enygma_dvp circuit uses.
func Hash(t *testing.T, inputs ...*big.Int) *big.Int {
	t.Helper()
	h, err := poseidon.Hash(inputs)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// Note is a spendable input note: leaf 0 of tree 0 of a depth-Depth Merkle
// tree, with its root and tree-bound nullifier.
type Note struct {
	Sk, Pk, Salt, Value, TokenId *big.Int
	Siblings                     []*big.Int
	Root, Nullifier              *big.Int
}

// NewNote builds the note sk owns holding value of tokenId at depth.
func NewNote(t *testing.T, sk, value, tokenId int64, depth int) Note {
	t.Helper()
	n := Note{
		Sk:      big.NewInt(sk),
		Salt:    big.NewInt(111),
		Value:   big.NewInt(value),
		TokenId: big.NewInt(tokenId),
	}
	n.Pk = Hash(t, n.Sk)
	n.Root = Commitment(t, n.Pk, n.Salt, n.Value, n.TokenId)
	for i := 0; i < depth; i++ {
		sibling := big.NewInt(int64(200 + i))
		n.Siblings = append(n.Siblings, sibling)
		n.Root = Hash(t, n.Root, sibling) // path index 0: the leaf is always left
	}
	// Poseidon(sk, treeNumber*2^depth + pathIndex) with tree 0, index 0.
	n.Nullifier = Hash(t, n.Sk, big.NewInt(0))
	return n
}

// Commitment is a note commitment, Poseidon(pk, salt, value, tokenId).
func Commitment(t *testing.T, pk, salt, value, tokenId *big.Int) *big.Int {
	t.Helper()
	return Hash(t, pk, salt, value, tokenId)
}

// Strings renders values as the decimal strings requests carry.
func Strings(values ...*big.Int) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = v.String()
	}
	return out
}

// Vault is the vault address the tests bind proofs to. Requests carry it in
// hex, which the witness accepts, so a handler that echoed the request
// field instead of reading its witness would return something else.
var Vault = big.NewInt(0xABCD)

// VaultHex is Vault as a request carries it.
const VaultHex = "0xabcd"
