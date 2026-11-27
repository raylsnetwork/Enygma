package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"enygma_retail_payments/gnark_circuits/server/config"
)

// TestExportsEveryServedCircuit: the ceremony seals keys for exactly the
// circuits export writes, so those must be exactly the circuits whose
// proving keys the server loads (server/config's *Pk paths, <Name>PK.key).
func TestExportsEveryServedCircuit(t *testing.T) {
	cfg := reflect.ValueOf(*config.Load())
	var served []string
	for i := range cfg.NumField() {
		if strings.HasSuffix(cfg.Type().Field(i).Name, "Pk") {
			served = append(served, strings.TrimSuffix(filepath.Base(cfg.Field(i).String()), "PK.key"))
		}
	}
	var exported []string
	for _, c := range circuits {
		exported = append(exported, c.name)
	}
	slices.Sort(served)
	slices.Sort(exported)
	if !slices.Equal(served, exported) {
		t.Fatalf("export writes %v, the server loads keys for %v", exported, served)
	}
}

// TestManifestPutsKeysWhereTheServerLoadsThem: the key manifest export
// writes names every exported circuit, each at the paths the server loads
// its keys from, so the ceremony seals them there.
func TestManifestPutsKeysWhereTheServerLoadsThem(t *testing.T) {
	dir := t.TempDir()
	if err := writeManifest(dir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, manifestFile))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]keyPair
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest) != len(circuits) {
		t.Fatalf("the manifest names %d circuits, export writes %d", len(manifest), len(circuits))
	}
	want := keyPair{PK: "scripts/keys/PaymentPK.key", VK: "scripts/keys/PaymentVK.key"}
	if got := manifest["Payment"]; got != want {
		t.Fatalf("Payment's keys at %+v, want %+v", got, want)
	}
}
