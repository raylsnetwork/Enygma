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

	"enygma_dvp/gnark_circuits/server/circuits/dvpDestination"
	"enygma_dvp/gnark_circuits/server/circuits/dvpInit"
	"enygma_dvp/gnark_circuits/server/config"
	"enygma_dvp/gnark_circuits/templates"
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

// TestExportsTheDvPLegsAtTheDepthTheServerProves: the dvpInit and
// dvpDestination prove handlers take one Merkle path element per level of
// the vaults' trees (their requests' WtPathElements) and prove at that
// depth, the one cmd/keygen compiles them at. The keys a ceremony seals
// from export's constraint systems fit the server only if export compiles
// each DvP leg at that depth too.
func TestExportsTheDvPLegsAtTheDepthTheServerProves(t *testing.T) {
	served := map[string]int{
		"DvPInitiator":   len(dvpInit.DvPInitiatorRequest{}.WtPathElements),
		"DvPDestination": len(dvpDestination.DvPDestinationRequest{}.WtPathElements),
	}
	for _, c := range circuits {
		want, ok := served[c.name]
		if !ok {
			continue
		}
		delete(served, c.name)
		var depth, path int
		switch circuit := c.circuit().(type) {
		case *templates.DvPInitiatorCircuit:
			depth, path = circuit.Config.TmMerkleTreeDepth, len(circuit.WtPathElements)
		case *templates.DvPDestinationCircuit:
			depth, path = circuit.Config.TmMerkleTreeDepth, len(circuit.WtPathElements)
		default:
			t.Fatalf("%s: export compiles a %T, not a DvP leg", c.name, circuit)
		}
		if depth != want || path != want {
			t.Errorf("%s: export compiles it at depth %d with %d path elements; the server proves it at depth %d", c.name, depth, path, want)
		}
	}
	if len(served) != 0 {
		t.Fatalf("export writes no %v", served)
	}
}
