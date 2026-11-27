package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"

	"enygma_dvp/gnark_circuits/retailkeys"
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
	out, err := exportAll(circuits, mapping, nil, build, false)
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
	if !slices.Equal(out.Exported, []string{"A"}) || !exists(t, filepath.Join(build, "A.json")) {
		t.Errorf("exported %v, want A written", out.Exported)
	}
}

func TestExportAll_PlaceholderGivesEveryConfigCircuitAVK(t *testing.T) {
	circuits, mapping, build := scenario(t)
	out, err := exportAll(circuits, mapping, nil, build, true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(out.Exported, []string{"A"}) || !slices.Equal(out.Placeheld, []string{"B", "C"}) {
		t.Fatalf("exported %v, placeholders %v; want [A], [B C]", out.Exported, out.Placeheld)
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

// A build dir from an earlier setup already holds the JSONs of circuits
// with no key file here (scripts/init.sh runs the export without
// -placeholder): they are kept as they are, and the export succeeds.
func TestExportAll_KeepsAnEarlierSetupsJSONForACircuitWithNoKeyFile(t *testing.T) {
	circuits, mapping, build := scenario(t)
	earlier := map[string][]byte{"B": []byte(`{"protocol":"groth16","from":"B"}`), "C": []byte(`{"protocol":"groth16","from":"C"}`)}
	for name, data := range earlier {
		if err := os.WriteFile(filepath.Join(build, name+".json"), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, placeholder := range []bool{false, true} {
		out, err := exportAll(circuits, mapping, nil, build, placeholder)
		if err != nil {
			t.Fatalf("placeholder=%v: %v", placeholder, err)
		}
		if !slices.Equal(out.Kept, []string{"B", "C"}) || len(out.Placeheld) != 0 {
			t.Errorf("placeholder=%v: kept %v, placeholders %v; want [B C], none", placeholder, out.Kept, out.Placeheld)
		}
		for name, data := range earlier {
			if got, _ := os.ReadFile(filepath.Join(build, name+".json")); !bytes.Equal(got, data) {
				t.Errorf("placeholder=%v: %s.json was overwritten", placeholder, name)
			}
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

// A retired circuit (retiredCircuits) gets the placeholder whatever is there:
// its key file (A) or an earlier setup's JSON (B) would register a real VK
// in a slot another vault's constants also read, and no -placeholder is
// needed for it.
func TestExportAll_RetiredCircuitAlwaysGetsThePlaceholder(t *testing.T) {
	circuits, mapping, build := scenario(t)
	earlierB := []byte(`{"protocol":"groth16","from":"B"}`)
	if err := os.WriteFile(filepath.Join(build, "B.json"), earlierB, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(build, "C.json"), []byte(`{"from":"C"}`), 0644); err != nil {
		t.Fatal(err)
	}
	retired := map[string]bool{"A": true, "B": true}
	out, err := exportAll(circuits, mapping, retired, build, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Exported) != 0 || !slices.Equal(out.Placeheld, []string{"A", "B"}) || !slices.Equal(out.Kept, []string{"C"}) {
		t.Fatalf("exported %v, placeholders %v, kept %v; want none, [A B], [C]", out.Exported, out.Placeheld, out.Kept)
	}
	real, err := readVK(mapping["A"])
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"A", "B"} {
		data, _ := os.ReadFile(filepath.Join(build, name+".json"))
		if bytes.Equal(data, earlierB) {
			t.Errorf("%s.json is the earlier setup's", name)
		}
		if strings.Contains(string(data), real.G1.Alpha.X.String()) {
			t.Errorf("%s.json carries A's real VK", name)
		}
	}
}

// retailPayment writes the Payment circuit's verifying key (a real one) and a
// stand-in proving key to a fresh dir, as enygma_retail_payments' key dir
// holds them, and copies both to another, as make dvp-keygen does; it returns
// the copies' dir and retail's.
func retailPayment(t *testing.T) (keys, retail string) {
	t.Helper()
	keys, retail = t.TempDir(), t.TempDir()
	keyFile(t, retail, "PaymentVK.key")
	if err := os.WriteFile(filepath.Join(retail, "PaymentPK.key"), []byte("Payment proving key"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"PaymentPK.key", "PaymentVK.key"} {
		b, err := os.ReadFile(filepath.Join(retail, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(keys, name), b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	return keys, retail
}

// The Payment circuit, VK slot 0, is exported from keys that are retail's,
// beside the config's other circuits.
func TestRun_ExportsRetailsPaymentKey(t *testing.T) {
	keys, retail := retailPayment(t)
	circuits, mapping, build := scenario(t)
	circuits = append(circuits, "Payment")
	mapping["Payment"] = filepath.Join(keys, "PaymentVK.key")
	out, err := run(circuits, mapping, nil, keys, retail, build, true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(out.Exported, []string{"A", "Payment"}) || !exists(t, filepath.Join(build, "Payment.json")) {
		t.Errorf("exported %v, want [A Payment] with Payment.json written", out.Exported)
	}
}

// A Payment proving or verifying key that is missing, or not retail's byte for
// byte, is refused, naming make dvp-keygen, before anything is written (the
// build dir included), with or without -placeholder: neither a placeholder
// nor an earlier JSON stands in for slot 0.
func TestRun_RefusesAPaymentKeyThatIsNotRetails(t *testing.T) {
	for _, name := range []string{"PaymentPK.key", "PaymentVK.key"} {
		for _, how := range []string{"changed", "missing"} {
			t.Run(how+" "+name, func(t *testing.T) {
				keys, retail := retailPayment(t)
				path := filepath.Join(keys, name)
				if how == "missing" {
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				} else {
					b, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					b[len(b)-1] ^= 1
					if err := os.WriteFile(path, b, 0644); err != nil {
						t.Fatal(err)
					}
				}
				for _, placeholder := range []bool{false, true} {
					build := filepath.Join(t.TempDir(), "build")
					_, err := run([]string{"A", "Payment"}, map[string]string{
						"A":       keyFile(t, t.TempDir(), "AVK.key"),
						"Payment": filepath.Join(keys, "PaymentVK.key"),
					}, nil, keys, retail, build, placeholder)
					if err == nil || !strings.Contains(err.Error(), "make dvp-keygen") {
						t.Fatalf("placeholder=%v: got %v, want the %s %s refused, naming make dvp-keygen", placeholder, err, how, name)
					}
					if how == "changed" && !strings.Contains(err.Error(), path+" is not enygma_retail_payments'") {
						t.Errorf("placeholder=%v: error %q does not name %s", placeholder, err, path)
					}
					if _, err := os.Stat(build); !errors.Is(err, os.ErrNotExist) {
						t.Errorf("placeholder=%v: the build dir was written before the keys were checked: %v", placeholder, err)
					}
				}
			})
		}
	}
}

// A build dir from an earlier setup may hold a Payment.json exported from
// other keys: with the Payment keys missing it is refused, not kept, and the
// JSON is left as it was.
func TestRun_RefusesAMissingPaymentKeyOverAnEarlierJSON(t *testing.T) {
	keys, retail := retailPayment(t)
	if err := os.Remove(filepath.Join(keys, "PaymentVK.key")); err != nil {
		t.Fatal(err)
	}
	build := t.TempDir()
	earlier := []byte(`{"protocol":"groth16","from":"an earlier setup"}`)
	if err := os.WriteFile(filepath.Join(build, "Payment.json"), earlier, 0644); err != nil {
		t.Fatal(err)
	}
	out, err := run([]string{"Payment"}, map[string]string{"Payment": filepath.Join(keys, "PaymentVK.key")}, nil, keys, retail, build, false)
	if err == nil || len(out.Kept) != 0 {
		t.Fatalf("got kept %v, error %v; want the missing Payment key refused", out.Kept, err)
	}
	if got, _ := os.ReadFile(filepath.Join(build, "Payment.json")); !bytes.Equal(got, earlier) {
		t.Error("the earlier Payment.json was overwritten")
	}
}

// The VK file export_vk_init exports for a Payment-family circuit is the one
// run checks: <circuit>VK.key in retailkeys.KeysDir.
func TestCircuitMapping_ExportsThePaymentKeysRunChecks(t *testing.T) {
	n := 0
	for _, name := range retailkeys.PaymentFamily {
		path, ok := circuitMapping[name]
		if !ok {
			continue
		}
		n++
		if want := filepath.Join(retailkeys.KeysDir, name+"VK.key"); path != want {
			t.Errorf("circuitMapping[%q] is %s, but run checks %s", name, path, want)
		}
	}
	if n == 0 {
		t.Fatal("circuitMapping maps no Payment-family circuit, but VK slot 0 is Payment's")
	}
}
