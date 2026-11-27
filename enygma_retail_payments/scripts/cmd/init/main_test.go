package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// vkJSON returns a circom-format VK JSON with nPublic public inputs, its
// coordinates derived from seed (no real curve points: init only checks
// and forwards them).
func vkJSON(seed, nPublic int) VerificationKeyJSON {
	n := seed * 1000
	next := func() string { n++; return fmt.Sprint(n) }
	g2 := func() [][]string { return [][]string{{next(), next()}, {next(), next()}, {"0", "1"}} }
	vk := VerificationKeyJSON{
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
// a vk_slots.json pinning both, the repo's EnygmaDvp and Verifier ABIs,
// and a config pointing at a port nothing
// listens on, so init fails at its first RPC once its checks pass. edit
// changes the manifest before it is written.
func projectRoot(t *testing.T, edit func(slots []map[string]any)) string {
	t.Helper()
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
	}{{"Payment", 1, 6}, {"PaymentFee", 2, 7}}
	var cfgCircuits []map[string]any
	var slots []map[string]any
	for i, c := range circuits {
		vk := vkJSON(c.seed, c.nPublic)
		write(filepath.Join("build", c.name+".json"), vk)
		cfgCircuits = append(cfgCircuits, map[string]any{"id": i + 1, "filename": c.name, "tags": []string{}})
		slots = append(slots, map[string]any{
			"slot": i, "circuit": c.name, "nPublic": c.nPublic,
			"vkHash": formatVKey(vk).slotKey().Hash(),
		})
	}
	edit(slots)
	write("vk_slots.json", map[string]any{"slots": slots})
	write("enygmapayment.config.json", map[string]any{
		"owner": map[string]any{"relayer-fixed-fee": "0", "usdr-fixed-fee": "0"},
		"network": map[string]any{
			"host": "127.0.0.1", "port": "1", "chain-id": "1337",
			"accounts": []map[string]string{{"private": "0x34d091c661db4c814d65c8ae9277b7055c0dde5a752ce5a3fdfd4ea11a8f7154"}},
		},
		"circom": map[string]any{"circuits": cfgCircuits},
	})
	write(filepath.Join("build", "receipts.json"), map[string]any{})
	for _, name := range []string{"EnygmaDvp", "Verifier"} {
		rel := filepath.Join("contracts", "abis", name+".json")
		data, err := os.ReadFile(filepath.Join("..", "..", "..", rel))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, rel), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestInitRefusesKeysTheManifestDoesNotPin(t *testing.T) {
	for name, tc := range map[string]struct {
		edit func(slots []map[string]any)
		want string
	}{
		"circuit": {func(s []map[string]any) { s[1]["circuit"] = "UsdrFee" }, `slot 1: the manifest gives it to "UsdrFee"`},
		"nPublic": {func(s []map[string]any) { s[0]["nPublic"] = 9 }, "slot 0 (Payment): the manifest pins 9 public inputs"},
		"vkHash":  {func(s []map[string]any) { s[1]["vkHash"] = "0x00" }, "slot 1 (PaymentFee): the manifest pins vk hash 0x00"},
	} {
		t.Run(name, func(t *testing.T) {
			initProjectRoot = projectRoot(t, tc.edit)
			err := initializePayment()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("initializePayment = %v, want a refusal containing %q", err, tc.want)
			}
		})
	}
}

// With the manifest as written, init gets past its checks to its first
// RPC, so each refusal above comes from the one entry its case changed.
func TestInitGetsPastAManifestThatPinsItsKeys(t *testing.T) {
	initProjectRoot = projectRoot(t, func([]map[string]any) {})
	err := initializePayment()
	if err == nil || strings.Contains(err.Error(), "manifest") || !strings.Contains(err.Error(), "127.0.0.1:1") {
		t.Fatalf("initializePayment = %v, want it to fail at its first RPC to 127.0.0.1:1", err)
	}
}
