package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"

	"enygma_retail_payments/gnark_circuits/server/config"
)

// manifestFile is the setup ceremony's key manifest (enygma_ceremony's
// ceremony.ManifestFile): where phase2-seal writes each circuit's keys,
// relative to its -keys directory.
const manifestFile = "keys.json"

// keyPair is a circuit's entry in the key manifest.
type keyPair struct {
	PK string `json:"pk"`
	VK string `json:"vk"`
}

// provingKeyField is a server config field naming a proving key, <Base>Pk<N>: the
// key of the circuit <Base><N>, whose verifying key is field <Base>Vk<N>.
var provingKeyField = regexp.MustCompile(`^(\w+?)Pk(\d*)$`)

// serverConfig is the config the gnark server loads its keys by.
func serverConfig() (any, error) { return *config.Load(), nil }

// keyPaths returns each circuit's key paths as the server config cfg names
// them, relative to this module's directory, which the server runs from.
func keyPaths(cfg any) (map[string]keyPair, error) {
	v := reflect.ValueOf(cfg)
	paths := map[string]keyPair{}
	for i := range v.NumField() {
		m := provingKeyField.FindStringSubmatch(v.Type().Field(i).Name)
		if m == nil {
			continue
		}
		vk := v.FieldByName(m[1] + "Vk" + m[2])
		if !vk.IsValid() {
			return nil, fmt.Errorf("the server config has %s but no %sVk%s", v.Type().Field(i).Name, m[1], m[2])
		}
		// The ceremony refuses a path outside its keys directory.
		paths[m[1]+m[2]] = keyPair{PK: filepath.Clean(v.Field(i).String()), VK: filepath.Clean(vk.String())}
	}
	return paths, nil
}

// writeManifest writes the key manifest into dir: each exported circuit's
// key paths, so the ceremony seals the keys where the server loads them.
func writeManifest(dir string) error {
	cfg, err := serverConfig()
	if err != nil {
		return err
	}
	paths, err := keyPaths(cfg)
	if err != nil {
		return err
	}
	manifest := map[string]keyPair{}
	for _, c := range circuits {
		p, ok := paths[c.name]
		if !ok {
			return fmt.Errorf("the server loads no keys for %s", c.name)
		}
		manifest[c.name] = p
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, manifestFile), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
