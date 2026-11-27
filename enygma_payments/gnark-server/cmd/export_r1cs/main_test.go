package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"enygma_payments/gnark-server/config"
)

var (
	// keyField is a config field naming a key: EnygmaPk, WithdrawVk6.
	keyField = regexp.MustCompile(`[PV]k\d*$`)
	// pkFile is a proving key's file, <Base>Pk<N>.key, sealed as <Base><N>.
	pkFile = regexp.MustCompile(`^(\w+)Pk(\d*)\.key$`)
	vkFile = regexp.MustCompile(`^\w+Vk\d*\.key$`)
)

// TestExportsEveryServedCircuit: the ceremony seals keys for exactly the
// circuits export writes, so those must be exactly the circuits whose
// proving keys the server loads (config.Load's key paths, each proving key
// <Base>Pk<N>.key for the circuit named <Base><N>).
func TestExportsEveryServedCircuit(t *testing.T) {
	cfg := reflect.ValueOf(*config.Load())
	var served []string
	for i := range cfg.NumField() {
		field, path := cfg.Type().Field(i).Name, cfg.Field(i).String()
		if !keyField.MatchString(field) && !strings.HasSuffix(path, ".key") {
			continue // BindHost, Port
		}
		base := filepath.Base(path)
		if m := pkFile.FindStringSubmatch(base); m != nil {
			served = append(served, m[1]+m[2])
		} else if !vkFile.MatchString(base) {
			t.Fatalf("config.Load's %s is %q, neither a <Base>Pk<N>.key nor a <Base>Vk<N>.key", field, path)
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
	want := keyPair{PK: "keys/zkdvp/WithdrawPk6.key", VK: "keys/zkdvp/WithdrawVk6.key"}
	if got := manifest["Withdraw6"]; got != want {
		t.Fatalf("Withdraw6's keys at %+v, want %+v", got, want)
	}
}
