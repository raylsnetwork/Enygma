package main

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/consensys/gnark/frontend"

	"enygma_dvp_auctions/gnark_circuits/server/circuits/auctionBatch"
	"enygma_dvp_auctions/gnark_circuits/server/circuits/auctionBid"
	"enygma_dvp_auctions/gnark_circuits/server/circuits/auctionFinal"
	"enygma_dvp_auctions/gnark_circuits/server/circuits/auctionLock"
	"enygma_dvp_auctions/gnark_circuits/server/circuits/auctionRevert"
	"enygma_dvp_auctions/gnark_circuits/server/circuits/auctionWithdraw"
	"enygma_dvp_auctions/gnark_circuits/server/config"
)

// TestExportsEveryServedCircuit: the ceremony seals keys for exactly the
// circuits export writes, so those must be exactly the circuits whose
// proving keys the server loads (server/config's *Pk paths, <Name>.pk).
func TestExportsEveryServedCircuit(t *testing.T) {
	c, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	cfg := reflect.ValueOf(*c)
	var served []string
	for i := range cfg.NumField() {
		if strings.HasSuffix(cfg.Type().Field(i).Name, "Pk") {
			served = append(served, strings.TrimSuffix(filepath.Base(cfg.Field(i).String()), ".pk"))
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

// requests are the prove handlers' request bodies, by circuit: a handler
// copies each field into the circuit input of the same name.
var requests = map[string]any{
	"AuctionLock":     auctionLock.AuctionLockRequest{},
	"AuctionBid":      auctionBid.AuctionBidRequest{},
	"AuctionWithdraw": auctionWithdraw.AuctionWithdrawRequest{},
	"AuctionBatch":    auctionBatch.AuctionBatchRequest{},
	"AuctionFinal":    auctionFinal.AuctionFinalRequest{},
	"AuctionRevert":   auctionRevert.AuctionRevertRequest{},
}

// TestExportsTheShapesTheServerProves: a circuit's slice lengths (the Merkle
// depth, the batch sizes) are part of the circuit, and the ceremony's keys
// prove only the one export compiles. Each must be the shape the server's
// prove handler fills: every input, by name and length, a field of its
// request body.
func TestExportsTheShapesTheServerProves(t *testing.T) {
	for _, c := range circuits {
		req, ok := requests[c.name]
		if !ok {
			t.Errorf("%s: no prove handler request", c.name)
			continue
		}
		got := inputs(t, reflect.ValueOf(c.circuit()).Elem())
		want := inputs(t, reflect.ValueOf(req))
		if !maps.Equal(got, want) {
			t.Errorf("%s: export compiles inputs %v, the server's request fills %v", c.name, got, want)
		}
	}
}

var variable = reflect.TypeFor[frontend.Variable]()

// inputs counts a circuit's or a request's inputs by field name: one for a
// frontend.Variable or a string, the length of a slice or array of them. A
// circuit's Config is no input; any other field fails the test.
func inputs(t *testing.T, v reflect.Value) map[string]int {
	t.Helper()
	isInput := func(ty reflect.Type) bool { return ty == variable || ty.Kind() == reflect.String }
	n := map[string]int{}
	for i := range v.NumField() {
		name, f := v.Type().Field(i).Name, v.Field(i)
		switch {
		case isInput(f.Type()):
			n[name] = 1
		case (f.Kind() == reflect.Slice || f.Kind() == reflect.Array) && isInput(f.Type().Elem()):
			n[name] = f.Len()
		case name != "Config":
			t.Fatalf("%s.%s: a %s, neither an input nor the config", v.Type(), name, f.Type())
		}
	}
	return n
}

// TestExportNeverOverwrites: an export into a directory that already holds a
// circuit's file fails on it and leaves it as it was.
func TestExportNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, circuits[0].name+".r1cs")
	if err := os.WriteFile(path, []byte("kept"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := export(dir, io.Discard); !errors.Is(err, fs.ErrExist) {
		t.Errorf("export over %s: got error %v, want one wrapping fs.ErrExist", path, err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "kept" {
		t.Errorf("%s after export: %d bytes, %v; want it kept, %q", path, len(got), err, "kept")
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
	want := keyPair{PK: "scripts/keys/AuctionBatch.pk", VK: "scripts/keys/AuctionBatch.vk"}
	if got := manifest["AuctionBatch"]; got != want {
		t.Fatalf("AuctionBatch's keys at %+v, want %+v", got, want)
	}
}
