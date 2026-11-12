package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

// keyFile writes a real verifying key (a setup of placeholderCircuit) to
// dir/name and returns its path.
func keyFile(t *testing.T, dir, name string) string {
	t.Helper()
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &placeholderCircuit{})
	if err != nil {
		t.Fatal(err)
	}
	_, vk, err := groth16.Setup(ccs)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := vk.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// A: key file present. B: mapped to a key file that is not there. C: in the
// config but not in the mapping (as BrokerRegistration is).
func scenario(t *testing.T) (circuits []string, mapping map[string]string, buildDir string) {
	keys, build := t.TempDir(), t.TempDir()
	return []string{"A", "B", "C"}, map[string]string{
		"A": keyFile(t, keys, "AVK.key"),
		"B": filepath.Join(keys, "BVK.key"),
	}, build
}

func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	return err == nil
}

func TestExportAll_NamesEveryCircuitWithNoKeyFile(t *testing.T) {
	circuits, mapping, build := scenario(t)
	exported, _, err := exportAll(circuits, mapping, build, false)
	if err == nil {
		t.Fatal("exportAll succeeded with two circuits missing their key file")
	}
	for _, name := range []string{"B", "C"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name %s", err, name)
		}
		if exists(t, filepath.Join(build, name+".json")) {
			t.Errorf("%s.json written with no key file and no -placeholder", name)
		}
	}
	if !slices.Equal(exported, []string{"A"}) || !exists(t, filepath.Join(build, "A.json")) {
		t.Errorf("exported %v, want A written", exported)
	}
}

func TestExportAll_PlaceholderGivesEveryConfigCircuitAVK(t *testing.T) {
	circuits, mapping, build := scenario(t)
	exported, placeheld, err := exportAll(circuits, mapping, build, true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(exported, []string{"A"}) || !slices.Equal(placeheld, []string{"B", "C"}) {
		t.Fatalf("exported %v, placeholders %v; want [A], [B C]", exported, placeheld)
	}
	a, _ := os.ReadFile(filepath.Join(build, "A.json"))
	for _, name := range circuits {
		data, err := os.ReadFile(filepath.Join(build, name+".json"))
		if err != nil {
			t.Fatalf("%s.json: %v", name, err)
		}
		// A placeholder is never a copy of a real circuit's key: a proof
		// for that circuit would then verify in the placeholder's slot.
		if name != "A" && bytes.Equal(data, a) {
			t.Errorf("%s.json is a copy of A's real VK", name)
		}
	}
}

// No proof verifies against the placeholder: not even one for the same
// circuit, made with a proving key from another setup of it.
func TestPlaceholderVK_VerifiesNoProof(t *testing.T) {
	vk, err := placeholderVK()
	if err != nil {
		t.Fatal(err)
	}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &placeholderCircuit{})
	if err != nil {
		t.Fatal(err)
	}
	pk, ownVK, err := groth16.Setup(ccs)
	if err != nil {
		t.Fatal(err)
	}
	w, err := frontend.NewWitness(&placeholderCircuit{X: 1}, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatal(err)
	}
	proof, err := groth16.Prove(ccs, pk, w)
	if err != nil {
		t.Fatal(err)
	}
	public, err := w.Public()
	if err != nil {
		t.Fatal(err)
	}
	if err := groth16.Verify(proof, ownVK, public); err != nil {
		t.Fatalf("the proof does not verify against its own setup's VK: %v", err)
	}
	if err := groth16.Verify(proof, vk, public); err == nil {
		t.Error("a proof from another setup verifies against the placeholder VK")
	}
}
