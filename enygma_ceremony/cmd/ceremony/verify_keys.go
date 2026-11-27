package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"enygma_ceremony/ceremony"
)

const phase2SealFile = "ceremony-seal.json"

// verifyKeys is the release check after independently replaying both phases.
// The approved digests must come from that review, outside the installed bundle.
func verifyKeys(fs *flag.FlagSet) func([]string, io.Writer) error {
	name := fs.String("ceremony", "", "approved protocol name")
	srs := fs.String("srs", "", "Phase 1 SRS, with its seal record beside it")
	r1cs := fs.String("r1cs", "", "fresh export of the release circuits, with keys.json")
	keys := fs.String("keys", "", "installed key directory, with its Phase 2 seal record")
	phase1Hash := fs.String("phase1-seal-sha256", "", "approved SHA-256 of the Phase 1 seal record")
	phase2Hash := fs.String("phase2-seal-sha256", "", "approved SHA-256 of the Phase 2 seal record")
	return func(args []string, out io.Writer) error {
		if len(args) != 0 || *name == "" || *srs == "" || *r1cs == "" || *keys == "" {
			return fmt.Errorf("%w: give -ceremony, -srs, -r1cs, -keys and both approved seal digests", errUsage)
		}
		if err := verifyInstalledKeys(*name, *srs, *r1cs, *keys, *phase1Hash, *phase2Hash); err != nil {
			return err
		}
		_, err := fmt.Fprintf(out, "Verified %s's installed circuits and keys against both approved ceremony seals.\n", *name)
		return err
	}
}

func verifyInstalledKeys(name, srs, r1cs, keys, phase1Hash, phase2Hash string) error {
	phase1, err := readApprovedSeal(srs+".json", phase1Hash, "srs")
	if err != nil {
		return err
	}
	phase2, err := readApprovedSeal(filepath.Join(keys, phase2SealFile), phase2Hash, "keys")
	if err != nil {
		return err
	}
	if phase2.Ceremony != name {
		return fmt.Errorf("Phase 2 sealed %q, expected %q", phase2.Ceremony, name)
	}
	if phase1.SRS != phase2.SRS {
		return fmt.Errorf("Phase 2 used a different SRS from the approved Phase 1")
	}
	if err := requireFileHash(srs, phase1.SRS); err != nil {
		return err
	}

	if _, err := os.Stat(filepath.Join(r1cs, ceremony.ManifestFile)); err != nil {
		return fmt.Errorf("release circuits require the protocol's %s: %w", ceremony.ManifestFile, err)
	}
	circuits, err := ceremony.CircuitsIn(r1cs)
	if err != nil {
		return err
	}
	if len(circuits) != len(phase2.Circuits) {
		return fmt.Errorf("release exports %d circuits, the approved seal has %d", len(circuits), len(phase2.Circuits))
	}
	for i, circuit := range circuits {
		sealed := phase2.Circuits[i]
		if sealed.Name != circuit.Name || sealed.PK != circuit.PK || sealed.VK != circuit.VK {
			return fmt.Errorf("%s: circuit or installed key paths differ from the approved seal", circuit.Name)
		}
		for _, file := range []struct{ path, want string }{
			{circuit.R1CS, sealed.R1CS},
			{filepath.Join(keys, circuit.PK), sealed.PKHash},
			{filepath.Join(keys, circuit.VK), sealed.VKHash},
		} {
			if err := requireFileHash(file.path, file.want); err != nil {
				return err
			}
		}
	}
	return nil
}

func readApprovedSeal(path, pin, kind string) (sealRecord, error) {
	want, err := hex.DecodeString(pin)
	if err != nil || len(want) != sha256.Size {
		return sealRecord{}, fmt.Errorf("%w: %s needs an approved seal SHA-256", errUsage, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return sealRecord{}, err
	}
	if got := sha256.Sum256(data); got != [sha256.Size]byte(want) {
		return sealRecord{}, fmt.Errorf("%s differs from the approved seal digest", path)
	}
	var record sealRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return sealRecord{}, fmt.Errorf("read %s: %w", path, err)
	}
	if record.Kind != kind || record.Contributions < 1 || record.Beacon.Unverified || record.Beacon.DrandRound == 0 {
		return sealRecord{}, fmt.Errorf("%s is not a contributed %s seal with a verified drand round", path, kind)
	}
	return record, nil
}

func requireFileHash(path, want string) error {
	got, err := hashHex(path)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("%s differs from the file in the approved ceremony seal", path)
	}
	return nil
}
