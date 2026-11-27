package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"enygma_dvp/gnark_circuits/server/config"
)

// TestExportsEveryServedCircuit: the ceremony seals keys for exactly the
// circuits export writes, so those and the ones whose keys come from retail
// must be exactly the circuits whose proving keys the server loads
// (server/config's *Pk paths, <Name>PK.key), each once.
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
	want := slices.Concat(exported, fromRetail)
	slices.Sort(served)
	slices.Sort(want)
	if !slices.Equal(served, want) {
		t.Fatalf("export writes %v and leaves %v to retail, the server loads keys for %v", exported, fromRetail, served)
	}
}

// TestExportKeepsExistingFile: export fails on a <Name>.r1cs already in the
// directory and leaves it as it was.
func TestExportKeepsExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, circuits[0].name+".r1cs")
	if err := os.WriteFile(path, []byte("kept"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := export(dir, io.Discard); !errors.Is(err, os.ErrExist) {
		t.Fatalf("export over an existing %s: err %v, want one wrapping os.ErrExist", path, err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "kept" {
		t.Fatalf("%s after export: %q, %v; want it kept", path, got, err)
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
	want := keyPair{PK: "scripts/keys/DvPInitiatorPK.key", VK: "scripts/keys/DvPInitiatorVK.key"}
	if got := manifest["DvPInitiator"]; got != want {
		t.Fatalf("DvPInitiator's keys at %+v, want %+v", got, want)
	}
}
