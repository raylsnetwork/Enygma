package utils

// TestMustLoadKeys_* and TestSelfVerify_* confirm the L-04 fix (cited
// inside H-12 as the control gap that would make a malicious or stale
// verifier swap invisible):
//
//  1. every handler used to do `pk, _ := LoadProvingKey(...)`, discarding
//     the error — a missing/unreadable key left the server looking
//     healthy until the first real proof request panicked. MustLoadKeys
//     must fail the process immediately instead.
//  2. no handler ever called LoadVerifyingKey or groth16.Verify — a
//     mismatched or corrupted pk/vk pair would only be discovered by
//     whoever received the bad proof next. SelfVerify must catch that
//     before a handler returns the proof.

import (
	"os"
	"os/exec"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

// squareCircuit is a minimal circuit (Y == X*X) used only to exercise
// MustLoadKeys/SelfVerify end to end without depending on any of this
// package's other circuit-specific test fixtures.
type squareCircuit struct {
	X frontend.Variable
	Y frontend.Variable `gnark:",public"`
}

func (c *squareCircuit) Define(api frontend.API) error {
	api.AssertIsEqual(api.Mul(c.X, c.X), c.Y)
	return nil
}

// TestMustLoadKeys_FailsFastOnMissingProvingKey confirms L-04 part 1: a
// missing key file must crash the process immediately (log.Fatal calls
// os.Exit, so this test re-execs itself as a subprocess to observe that
// exit — the standard Go idiom for testing log.Fatal/os.Exit paths, since
// they cannot be caught in-process).
func TestMustLoadKeys_FailsFastOnMissingProvingKey(t *testing.T) {
	if os.Getenv("L04_SUBPROCESS_PK") == "1" {
		MustLoadKeys(ecc.BN254, "/nonexistent/path/pk.key", "/nonexistent/path/vk.key")
		return // unreachable if MustLoadKeys behaves correctly
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestMustLoadKeys_FailsFastOnMissingProvingKey")
	cmd.Env = append(os.Environ(), "L04_SUBPROCESS_PK=1")
	err := cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); ok && !exitErr.Success() {
		t.Logf("MustLoadKeys correctly exited the process on a missing proving key: %v", err)
		return
	}
	t.Fatalf("FAIL (L-04 regressed): MustLoadKeys did not exit the process on a missing proving key (err=%v)", err)
}

// TestMustLoadKeys_FailsFastOnMissingVerifyingKey is the same check for a
// present-but-wrong vkPath — L-04's finding was specifically that vkPath
// was accepted by every handler and never used at all.
func TestMustLoadKeys_FailsFastOnMissingVerifyingKey(t *testing.T) {
	if os.Getenv("L04_SUBPROCESS_VK") == "1" {
		// Build a real proving key so only the verifying key load fails.
		circuit := &squareCircuit{}
		ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
		if err != nil {
			panic(err)
		}
		pk, _, err := groth16.Setup(ccs)
		if err != nil {
			panic(err)
		}
		pkFile, err := os.CreateTemp("", "l04-pk-*.key")
		if err != nil {
			panic(err)
		}
		defer os.Remove(pkFile.Name())
		if _, err := pk.WriteTo(pkFile); err != nil {
			panic(err)
		}
		pkFile.Close()

		MustLoadKeys(ecc.BN254, pkFile.Name(), "/nonexistent/path/vk.key")
		return // unreachable if MustLoadKeys behaves correctly
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestMustLoadKeys_FailsFastOnMissingVerifyingKey")
	cmd.Env = append(os.Environ(), "L04_SUBPROCESS_VK=1")
	err := cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); ok && !exitErr.Success() {
		t.Logf("MustLoadKeys correctly exited the process on a missing verifying key: %v", err)
		return
	}
	t.Fatalf("FAIL (L-04 regressed): MustLoadKeys did not exit the process on a missing verifying key (err=%v)", err)
}

// TestMustLoadKeys_SucceedsOnValidKeys is the honest-path control: real
// keys load without exiting.
func TestMustLoadKeys_SucceedsOnValidKeys(t *testing.T) {
	circuit := &squareCircuit{}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	pkFile, err := os.CreateTemp("", "l04-pk-*.key")
	if err != nil {
		t.Fatalf("create pk file: %v", err)
	}
	defer os.Remove(pkFile.Name())
	if _, err := pk.WriteTo(pkFile); err != nil {
		t.Fatalf("write pk: %v", err)
	}
	pkFile.Close()

	vkFile, err := os.CreateTemp("", "l04-vk-*.key")
	if err != nil {
		t.Fatalf("create vk file: %v", err)
	}
	defer os.Remove(vkFile.Name())
	if _, err := vk.WriteTo(vkFile); err != nil {
		t.Fatalf("write vk: %v", err)
	}
	vkFile.Close()

	// Must not exit the process.
	loadedPk, loadedVk := MustLoadKeys(ecc.BN254, pkFile.Name(), vkFile.Name())
	if loadedPk == nil || loadedVk == nil {
		t.Fatal("MustLoadKeys returned a nil key on a valid path")
	}
}

// TestSelfVerify_AcceptsValidProof is the honest-path control for L-04
// part 2: a genuine proof against its matching vk must pass.
func TestSelfVerify_AcceptsValidProof(t *testing.T) {
	circuit := &squareCircuit{}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	w, err := frontend.NewWitness(&squareCircuit{X: 6, Y: 36}, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("witness: %v", err)
	}
	proof, err := groth16.Prove(ccs, pk, w)
	if err != nil {
		t.Fatalf("prove: %v", err)
	}

	if err := SelfVerify(proof, vk, w); err != nil {
		t.Fatalf("SelfVerify rejected a genuine proof against its own vk: %v", err)
	}
	t.Log("SelfVerify accepts a genuine proof against its matching verifying key")
}

// TestSelfVerify_RejectsMismatchedVerifyingKey reproduces L-04's actual
// concern directly: a proof generated under one key, checked against a
// DIFFERENT circuit's verifying key (standing in for "the pk on disk and
// the vk on disk no longer correspond to the same ceremony output" —
// exactly the silent mismatch nothing in gnark-server checked for before
// this fix).
func TestSelfVerify_RejectsMismatchedVerifyingKey(t *testing.T) {
	circuit := &squareCircuit{}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	pk, _, err := groth16.Setup(ccs)
	if err != nil {
		t.Fatalf("setup (first): %v", err)
	}
	// A second, independent Setup on the SAME circuit still produces a
	// different, non-corresponding (pk, vk) pair — Setup's randomness
	// means no two runs share a trapdoor, which is the entire point.
	_, wrongVk, err := groth16.Setup(ccs)
	if err != nil {
		t.Fatalf("setup (second): %v", err)
	}

	w, err := frontend.NewWitness(&squareCircuit{X: 6, Y: 36}, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("witness: %v", err)
	}
	proof, err := groth16.Prove(ccs, pk, w)
	if err != nil {
		t.Fatalf("prove: %v", err)
	}

	if err := SelfVerify(proof, wrongVk, w); err == nil {
		t.Fatal("FAIL (L-04 regressed): SelfVerify accepted a proof against a non-matching verifying key")
	} else {
		t.Logf("SelfVerify correctly rejected a proof against a mismatched verifying key: %v", err)
	}
}
