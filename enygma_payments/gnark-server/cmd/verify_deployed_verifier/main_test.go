package main

// TestExpectedConstants_* and TestMatching_* cover L-04 part 3's actual
// logic. The tool was also manually exercised against this repo's real
// data during development (not repeated here since it needs a live
// Hardhat node / compiled artifacts on disk):
//   - keys/EnygmaVk.key vs its own compiled EnygmaVerifier.sol artifact
//     (both -artifact and -rpc against a freshly deployed instance): PASS
//   - keys/EnygmaVk.key vs the unrelated EnygmaFeeVerifier.sol artifact
//     (both modes): FAIL, all constants reported missing
// That run is also what caught the bug TestMatching_HandlesNarrowerPushWidths
// pins down below: a first version of the matching logic assumed every
// constant is emitted as a full 32-byte PUSH32 immediate and reported 4
// false-positive mismatches against this repo's own genuinely-matching
// EnygmaVk.key/EnygmaVerifier.sol pair, because Solidity picks the
// narrowest PUSH opcode that fits each constant's actual value.

import (
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

type squareCircuit struct {
	X frontend.Variable
	Y frontend.Variable `gnark:",public"`
}

func (c *squareCircuit) Define(api frontend.API) error {
	api.AssertIsEqual(api.Mul(c.X, c.X), c.Y)
	return nil
}

// writeToyVK compiles a minimal circuit, runs Setup, and writes the
// resulting verifying key to a temp file, returning its path.
func writeToyVK(t *testing.T) string {
	t.Helper()
	circuit := &squareCircuit{}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	_, vk, err := groth16.Setup(ccs)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	f, err := os.CreateTemp("", "l04-tool-vk-*.key")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	if _, err := vk.WriteTo(f); err != nil {
		t.Fatalf("write vk: %v", err)
	}
	f.Close()
	return f.Name()
}

func TestExpectedConstants_ParsesToyVK(t *testing.T) {
	vkPath := writeToyVK(t)
	constants, err := expectedConstants(vkPath)
	if err != nil {
		t.Fatalf("expectedConstants: %v", err)
	}
	// A minimal one-public-input circuit still has alpha/beta/gamma/delta
	// (8 coordinates across G1+G2×3) plus at least CONSTANT + one PUB
	// point (2 coordinates each) — comfortably more than 8.
	if len(constants) < 8 {
		t.Fatalf("expected at least 8 constants for a minimal circuit's VK, got %d", len(constants))
	}
	for name, val := range constants {
		if val == nil || val.Sign() < 0 {
			t.Fatalf("constant %s has an invalid value: %v", name, val)
		}
	}
}

func TestExpectedConstants_MissingFileErrors(t *testing.T) {
	if _, err := expectedConstants("/nonexistent/path/vk.key"); err == nil {
		t.Fatal("expected an error loading a nonexistent VK file")
	}
}

// TestMatching_HandlesNarrowerPushWidths reproduces the actual bug caught
// during development: build fake "bytecode" containing every expected
// constant's MINIMAL (unpadded) big-endian encoding — exactly what
// Solidity emits for a constant whose value doesn't need the full 32
// bytes — and confirm every constant is still found. A fixed-width
// (always-32-byte) matcher fails this for any constant with a leading
// zero byte.
func TestMatching_HandlesNarrowerPushWidths(t *testing.T) {
	vkPath := writeToyVK(t)
	constants, err := expectedConstants(vkPath)
	if err != nil {
		t.Fatalf("expectedConstants: %v", err)
	}

	// Force at least one constant to need a narrower encoding than 32
	// bytes, regardless of what this run's random Setup happened to
	// produce, so the test doesn't depend on getting lucky.
	constants["FORCED_NARROW"] = big.NewInt(0x42) // fits in 1 byte

	var bytecode strings.Builder
	bytecode.WriteString("6001600201") // a few unrelated opcode bytes, for realism
	for _, val := range constants {
		bytecode.WriteString(fmtHex(val))
		bytecode.WriteString("00") // opcode separator, never part of the encoded value
	}
	fakeBytecode := bytecode.String()

	missing := findMissing(constants, fakeBytecode)
	if len(missing) != 0 {
		t.Fatalf("FAIL (tool bug regressed): %d constants falsely reported missing: %v", len(missing), missing)
	}
}

// TestMatching_DetectsGenuineMismatch confirms the tool still correctly
// flags a real mismatch — bytecode missing one of the expected constants
// entirely, not just narrower-than-32-bytes.
func TestMatching_DetectsGenuineMismatch(t *testing.T) {
	vkPath := writeToyVK(t)
	constants, err := expectedConstants(vkPath)
	if err != nil {
		t.Fatalf("expectedConstants: %v", err)
	}

	var bytecode strings.Builder
	skipped := ""
	for name, val := range constants {
		if skipped == "" {
			skipped = name
			continue // omit exactly one constant from the fake bytecode
		}
		bytecode.WriteString(fmtHex(val))
		bytecode.WriteString("00")
	}

	missing := findMissing(constants, bytecode.String())
	if len(missing) != 1 || missing[0] != skipped {
		t.Fatalf("expected exactly [%s] reported missing, got %v", skipped, missing)
	}
	t.Logf("correctly detected the omitted constant: %v", missing)
}
