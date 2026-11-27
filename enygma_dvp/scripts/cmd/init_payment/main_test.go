package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"enygma_dvp/layout"
)

// vkJSON returns a circom-format VK JSON with nPublic public inputs, its
// coordinates derived from seed (no real curve points: init only checks
// and forwards them).
func vkJSON(seed, nPublic int) initPaymentVKJSON {
	n := seed * 1000
	next := func() string { n++; return fmt.Sprint(n) }
	g2 := func() [][]string { return [][]string{{next(), next()}, {next(), next()}, {"0", "1"}} }
	vk := initPaymentVKJSON{
		Protocol: "groth16", Curve: "bn128",
		VkAlpha1: []string{next(), next(), "1"},
		VkBeta2:  g2(), VkGamma2: g2(), VkDelta2: g2(),
	}
	for i := 0; i <= nPublic; i++ {
		vk.IC = append(vk.IC, []string{next(), next(), "1"})
	}
	return vk
}

// projectRoot writes a project root for init: two circuits, their VK JSONs,
// a vk_slots_payment.json pinning both and a config at the circuits' tree
// depth, but no contract artifacts, so init fails loading the EnygmaDvp ABI
// once its checks pass. edit changes the manifest before it is written.
func projectRoot(t *testing.T, edit func(slots []map[string]any)) string {
	t.Helper()
	// The caller points initPaymentProjectRoot at the root this returns; put the
	// committed root back when the test ends, for the tests that read it.
	saved := initPaymentProjectRoot
	t.Cleanup(func() { initPaymentProjectRoot = saved })
	root := t.TempDir()
	write := func(rel string, v any) {
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	circuits := []struct {
		name          string
		seed, nPublic int
	}{{"Payment", 1, 8}, {"PaymentFee", 2, 9}}
	var cfgCircuits []map[string]any
	var slots []map[string]any
	for i, c := range circuits {
		vk := vkJSON(c.seed, c.nPublic)
		write(filepath.Join("build", c.name+".json"), vk)
		cfgCircuits = append(cfgCircuits, map[string]any{"id": i + 1, "filename": c.name, "tags": []string{}})
		slots = append(slots, map[string]any{
			"slot": i, "circuit": c.name, "nPublic": c.nPublic,
			"vkHash": formatPaymentVKey(vk).slotKey().Hash(),
		})
	}
	edit(slots)
	write("vk_slots_payment.json", map[string]any{"slots": slots})
	write("enygmadvp_payment.config.json", map[string]any{
		"network": map[string]any{
			"host": "127.0.0.1", "port": "1", "chain-id": "1337",
			"accounts": []map[string]string{{"private": "0x34d091c661db4c814d65c8ae9277b7055c0dde5a752ce5a3fdfd4ea11a8f7154"}},
		},
		"circom": map[string]any{
			"meta-parameters": map[string]any{"tree-depth": layout.MerkleTreeDepth},
			"circuits":        cfgCircuits,
		},
	})
	write(filepath.Join("build", "payment_receipts.json"), map[string]any{})
	return root
}

func TestInitRefusesKeysTheManifestDoesNotPin(t *testing.T) {
	for name, tc := range map[string]struct {
		edit func(slots []map[string]any)
		want string
	}{
		"circuit": {func(s []map[string]any) { s[1]["circuit"] = "UsdrFee" }, `slot 1: the manifest gives it to "UsdrFee"`},
		"nPublic": {func(s []map[string]any) { s[0]["nPublic"] = 9 }, "the manifest pins 9 public inputs"},
		"vkHash":  {func(s []map[string]any) { s[1]["vkHash"] = "0x00" }, "the manifest pins vk hash 0x00"},
	} {
		t.Run(name, func(t *testing.T) {
			initPaymentProjectRoot = projectRoot(t, tc.edit)
			err := initializePayment()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("initializePayment = %v, want a refusal containing %q", err, tc.want)
			}
		})
	}
}

// With the manifest as written, init gets past its checks to loading the
// EnygmaDvp ABI, so each refusal above comes from the one entry its case
// changed.
func TestInitGetsPastAManifestThatPinsItsKeys(t *testing.T) {
	initPaymentProjectRoot = projectRoot(t, func([]map[string]any) {})
	err := initializePayment()
	if err == nil || !strings.Contains(err.Error(), "failed to load EnygmaDvp ABI") {
		t.Fatalf("initializePayment = %v, want it to fail loading the EnygmaDvp ABI", err)
	}
}

// The committed config is registered as it stands, so its tree-depth must be
// the circuits' (layout.MerkleTreeDepth), and init must refuse any other
// before it registers a vault.
func TestLoadInitPaymentConfig_RefusesATreeDepthOtherThanTheCircuits(t *testing.T) {
	committed, err := loadInitPaymentConfig()
	if err != nil {
		t.Fatalf("the committed enygmadvp_payment.config.json is refused: %v", err)
	}
	if got := committed.Circom.MetaParameters.TreeDepth; got != layout.MerkleTreeDepth {
		t.Fatalf("the committed config loads with tree-depth %d, want the circuits' %d", got, layout.MerkleTreeDepth)
	}
	for _, depth := range []int{0, 8, layout.MerkleTreeDepth - 1, layout.MerkleTreeDepth + 1} {
		withTreeDepth(t, &initPaymentProjectRoot, "enygmadvp_payment.config.json", depth)
		want := fmt.Sprintf("sets tree-depth %d, but the circuits are compiled at depth %d", depth, layout.MerkleTreeDepth)
		if _, err := loadInitPaymentConfig(); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("tree-depth %d: loadInitPaymentConfig returned err=%v, want it refused with %q", depth, err, want)
		}
	}
}

// withTreeDepth points *root, until the test ends, at a temporary copy of the
// committed config named name whose tree-depth is depth.
func withTreeDepth(t *testing.T, root *string, name string, depth int) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(*root, name))
	if err != nil {
		t.Fatal(err)
	}
	// UseNumber keeps the config's other numbers digit for digit.
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var config map[string]any
	if err := dec.Decode(&config); err != nil {
		t.Fatal(err)
	}
	meta := config["circom"].(map[string]any)["meta-parameters"].(map[string]any)
	if _, ok := meta["tree-depth"]; !ok {
		t.Fatalf("%s has no circom.meta-parameters.tree-depth to change", name)
	}
	meta["tree-depth"] = depth
	if data, err = json.Marshal(config); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
	saved := *root
	*root = dir
	t.Cleanup(func() { *root = saved })
}
