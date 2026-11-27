package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

// tinyCircuit is the smallest circuit with a public input, for a real
// verifying key to export.
type tinyCircuit struct {
	X frontend.Variable `gnark:",public"`
	Y frontend.Variable
}

func (c *tinyCircuit) Define(api frontend.API) error {
	api.AssertIsEqual(c.X, c.Y)
	return nil
}

// retailDir writes a verifying key and a stand-in proving key for every
// circuit to a fresh dir, as enygma_retail_payments' key dir holds them.
func retailDir(t *testing.T) string {
	t.Helper()
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &tinyCircuit{})
	if err != nil {
		t.Fatal(err)
	}
	_, vk, err := groth16.Setup(ccs)
	if err != nil {
		t.Fatal(err)
	}
	var vkBytes bytes.Buffer
	if _, err := vk.WriteTo(&vkBytes); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, c := range circuits {
		write(t, filepath.Join(dir, c+"VK.key"), vkBytes.Bytes())
		write(t, filepath.Join(dir, c+"PK.key"), []byte(c+" proving key"))
	}
	return dir
}

// copyOf copies every key in dir to a fresh dir, as make dvp-keygen does.
func copyOf(t *testing.T, dir string) string {
	t.Helper()
	out := t.TempDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(out, e.Name()), b)
	}
	return out
}

func write(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// Keys copied from retail export: one JSON per circuit.
func TestRun_ExportsRetailsKeys(t *testing.T) {
	retail := retailDir(t)
	build := filepath.Join(t.TempDir(), "build")
	if err := run(copyOf(t, retail), retail, build); err != nil {
		t.Fatal(err)
	}
	for _, c := range circuits {
		if _, err := os.Stat(filepath.Join(build, c+".json")); err != nil {
			t.Errorf("%s: %v", c, err)
		}
	}
}

// A proving or verifying key that is not retail's, byte for byte, is
// refused before anything is exported, naming the key and the fix.
func TestRun_RefusesKeysThatAreNotRetails(t *testing.T) {
	retail := retailDir(t)
	for _, c := range circuits {
		for _, kind := range []string{"PK", "VK"} {
			name := c + kind + ".key"
			t.Run(name, func(t *testing.T) {
				ours := copyOf(t, retail)
				b, err := os.ReadFile(filepath.Join(ours, name))
				if err != nil {
					t.Fatal(err)
				}
				b[len(b)-1] ^= 1
				write(t, filepath.Join(ours, name), b)
				build := filepath.Join(t.TempDir(), "build")
				err = run(ours, retail, build)
				if err == nil {
					t.Fatalf("exported with a %s that is not retail's", name)
				}
				if !strings.Contains(err.Error(), filepath.Join(ours, name)+" is not enygma_retail_payments'") ||
					!strings.Contains(err.Error(), "make dvp-keygen") {
					t.Errorf("error %q does not name %s and make dvp-keygen", err, name)
				}
				if _, err := os.Stat(build); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("the build dir was written before the keys were checked: %v", err)
				}
			})
		}
	}
}

// A key make dvp-keygen has not copied yet is refused as missing, naming
// the fix.
func TestRun_RefusesAMissingKey(t *testing.T) {
	retail := retailDir(t)
	ours := copyOf(t, retail)
	if err := os.Remove(filepath.Join(ours, "UsdrFeePK.key")); err != nil {
		t.Fatal(err)
	}
	err := run(ours, retail, filepath.Join(t.TempDir(), "build"))
	if !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "make dvp-keygen") {
		t.Fatalf("got %v, want a missing-key error naming make dvp-keygen", err)
	}
}
