package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"enygma_ceremony/ceremony"
)

func TestVerifyProductionKeys(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(dir string, phase1, phase2 *sealRecord) error
		want   string
	}{
		{name: "approved"},
		{
			name: "rehearsal phase 1",
			change: func(_ string, phase1, _ *sealRecord) error {
				phase1.Beacon.Unverified = true
				return nil
			},
			want: "verified drand round",
		},
		{
			name: "rehearsal phase 2",
			change: func(_ string, _, phase2 *sealRecord) error {
				phase2.Beacon.Unverified = true
				return nil
			},
			want: "verified drand round",
		},
		{
			name: "wrong phase 1",
			change: func(_ string, _, phase2 *sealRecord) error {
				phase2.SRS = strings.Repeat("42", 32)
				return nil
			},
			want: "different SRS",
		},
		{
			name: "wrong protocol",
			change: func(_ string, _, phase2 *sealRecord) error {
				phase2.Ceremony = "another protocol"
				return nil
			},
			want: "Phase 2 sealed",
		},
		{
			name: "missing circuit",
			change: func(_ string, _, phase2 *sealRecord) error {
				phase2.Circuits = nil
				return nil
			},
			want: "release exports 1 circuits",
		},
		{
			name: "wrong key path",
			change: func(_ string, _, phase2 *sealRecord) error {
				phase2.Circuits[0].VK = "different.vk"
				return nil
			},
			want: "installed key paths differ",
		},
		{
			name: "different proving key",
			change: func(dir string, _, _ *sealRecord) error {
				return os.WriteFile(filepath.Join(dir, "keys", "Cubic.pk"), []byte("development key"), 0o600)
			},
			want: "Cubic.pk differs",
		},
		{
			name: "different verifying key",
			change: func(dir string, _, _ *sealRecord) error {
				return os.WriteFile(filepath.Join(dir, "keys", "Cubic.vk"), []byte("development key"), 0o600)
			},
			want: "Cubic.vk differs",
		},
		{
			name: "different circuit",
			change: func(dir string, _, _ *sealRecord) error {
				return os.WriteFile(filepath.Join(dir, "r1cs", "Cubic.r1cs"), []byte("different circuit"), 0o600)
			},
			want: "Cubic.r1cs differs",
		},
		{
			name: "no export manifest",
			change: func(dir string, _, _ *sealRecord) error {
				return os.Remove(filepath.Join(dir, "r1cs", ceremony.ManifestFile))
			},
			want: "release circuits require",
		},
		{name: "unapproved seal digest", want: "differs from the approved seal digest"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			srs := filepath.Join(dir, "phase1.srs")
			r1cs := filepath.Join(dir, "r1cs")
			keys := filepath.Join(dir, "keys")
			for _, path := range []string{r1cs, keys} {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			write := func(path, content string) string {
				t.Helper()
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
				return fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
			}
			srsHash := write(srs, "sealed SRS")
			write(filepath.Join(r1cs, ceremony.ManifestFile), `{"Cubic":{"pk":"Cubic.pk","vk":"Cubic.vk"}}`)
			phase1 := sealRecord{Kind: "srs", SRS: srsHash, Contributions: 1, Beacon: beaconRecord{DrandRound: 12}}
			phase2 := sealRecord{
				Kind: "keys", Ceremony: "example", SRS: srsHash, Contributions: 1, Beacon: beaconRecord{DrandRound: 14},
				Circuits: []circuitRecord{{
					Name: "Cubic", R1CS: write(filepath.Join(r1cs, "Cubic.r1cs"), "compiled circuit"),
					PK: "Cubic.pk", PKHash: write(filepath.Join(keys, "Cubic.pk"), "sealed proving key"),
					VK: "Cubic.vk", VKHash: write(filepath.Join(keys, "Cubic.vk"), "sealed verifying key"),
				}},
			}
			if tc.change != nil {
				if err := tc.change(dir, &phase1, &phase2); err != nil {
					t.Fatal(err)
				}
			}
			seal := func(path string, record sealRecord) string {
				t.Helper()
				if err := writeRecord(path, record); err != nil {
					t.Fatal(err)
				}
				return fmt.Sprintf("%x", sha256.Sum256(readFile(t, path)))
			}
			phase1Pin := seal(srs+".json", phase1)
			phase2Pin := seal(filepath.Join(keys, phase2SealFile), phase2)
			if tc.name == "unapproved seal digest" {
				phase2Pin = strings.Repeat("42", 32)
			}
			code, out, errOut := runCommand("verify-keys", "-ceremony", "example", "-srs", srs,
				"-r1cs", r1cs, "-keys", keys, "-phase1-seal-sha256", phase1Pin, "-phase2-seal-sha256", phase2Pin)
			if tc.want == "" {
				if code != 0 || !strings.Contains(out, "against both approved ceremony seals") {
					t.Fatalf("verify-keys returned %d: %s%s", code, out, errOut)
				}
			} else if code != 1 || !strings.Contains(errOut, tc.want) {
				t.Fatalf("verify-keys returned %d, want refusal containing %q: %s%s", code, tc.want, out, errOut)
			}
		})
	}
}
