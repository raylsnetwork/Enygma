// Package testcircuits holds the small circuits the ceremony's tests run a
// whole setup for, and checks that the keys a ceremony seals for them prove.
package testcircuits

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

// Cubic asserts x³ + x + 5 = y: a few constraints, no commitment.
type Cubic struct {
	X frontend.Variable
	Y frontend.Variable `gnark:",public"`
}

// Define builds Cubic's constraints.
func (c *Cubic) Define(api frontend.API) error {
	api.AssertIsEqual(c.Y, api.Add(api.Mul(c.X, c.X, c.X), c.X, 5))
	return nil
}

// Committed asserts x·y²⁰ = z and commits to x and y in-circuit, so its keys
// carry a Pedersen commitment basis: the σ that Phase 2 updates beside δ.
type Committed struct {
	X, Y frontend.Variable
	Z    frontend.Variable `gnark:",public"`
}

// Define builds Committed's constraints.
func (c *Committed) Define(api frontend.API) error {
	committer, ok := api.(frontend.Committer)
	if !ok {
		return errors.New("builder cannot commit")
	}
	cmt, err := committer.Commit(c.X, c.Y)
	if err != nil {
		return err
	}
	api.AssertIsDifferent(cmt, 0)
	acc := c.X
	for range 20 {
		acc = api.Mul(acc, c.Y)
	}
	api.AssertIsEqual(acc, c.Z)
	return nil
}

// Commitments commits to each of its variables apart: a circuit of
// len(X) in-circuit commitments, as many as gnark's Phase 2 then holds σs.
type Commitments struct {
	X []frontend.Variable
}

// Define builds Commitments' constraints.
func (c *Commitments) Define(api frontend.API) error {
	committer, ok := api.(frontend.Committer)
	if !ok {
		return errors.New("builder cannot commit")
	}
	for _, x := range c.X {
		cmt, err := committer.Commit(x)
		if err != nil {
			return err
		}
		api.AssertIsDifferent(cmt, 0)
	}
	return nil
}

// Circuits are the test circuits by the name their keys are sealed under,
// each with a satisfying assignment and one that differs only in its public
// input. Cubic needs a domain of 4, Committed one of 32.
var Circuits = []struct {
	Name         string
	Circuit      frontend.Circuit
	Valid, Wrong frontend.Circuit
}{
	{"Cubic", &Cubic{}, &Cubic{X: 3, Y: 35}, &Cubic{X: 3, Y: 36}},
	{"Committed", &Committed{}, &Committed{X: 3, Y: 2, Z: 3 << 20}, &Committed{X: 3, Y: 2, Z: 3<<20 + 1}},
}

// WriteR1CS compiles every test circuit into dir as <Name>.r1cs.
func WriteR1CS(dir string) error {
	for _, c := range Circuits {
		if err := WriteCircuit(dir, c.Name, c.Circuit); err != nil {
			return err
		}
	}
	return nil
}

// WriteCircuit compiles circuit into dir as <name>.r1cs.
func WriteCircuit(dir, name string, circuit frontend.Circuit) error {
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, name+".r1cs"))
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	if _, err := ccs.WriteTo(w); err != nil {
		f.Close()
		return err
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// CheckKeys loads each test circuit's constraint system from r1csDir and its
// sealed keys from keysDir, as the gnark servers load them (ReadFrom), and
// checks that a proof of the valid assignment verifies and that the proof
// does not verify against the wrong public input.
//
// The keys are at <Name>PK.key and <Name>VK.key, or where r1csDir's key
// manifest, keys.json, puts them.
func CheckKeys(keysDir, r1csDir string) error {
	paths := map[string]struct{ PK, VK string }{}
	if data, err := os.ReadFile(filepath.Join(r1csDir, "keys.json")); err == nil {
		if err := json.Unmarshal(data, &paths); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, c := range Circuits {
		ccs := groth16.NewCS(ecc.BN254)
		pk := groth16.NewProvingKey(ecc.BN254)
		vk := groth16.NewVerifyingKey(ecc.BN254)
		if err := readInto(filepath.Join(r1csDir, c.Name+".r1cs"), ccs.ReadFrom); err != nil {
			return err
		}
		p, ok := paths[c.Name]
		if !ok {
			p.PK, p.VK = c.Name+"PK.key", c.Name+"VK.key"
		}
		if err := readInto(filepath.Join(keysDir, p.PK), pk.ReadFrom); err != nil {
			return err
		}
		if err := readInto(filepath.Join(keysDir, p.VK), vk.ReadFrom); err != nil {
			return err
		}
		valid, err := frontend.NewWitness(c.Valid, ecc.BN254.ScalarField())
		if err != nil {
			return err
		}
		proof, err := groth16.Prove(ccs, pk, valid)
		if err != nil {
			return fmt.Errorf("%s: prove: %w", c.Name, err)
		}
		public, err := valid.Public()
		if err != nil {
			return err
		}
		if err := groth16.Verify(proof, vk, public); err != nil {
			return fmt.Errorf("%s: the proof does not verify: %w", c.Name, err)
		}
		wrong, err := frontend.NewWitness(c.Wrong, ecc.BN254.ScalarField(), frontend.PublicOnly())
		if err != nil {
			return err
		}
		if groth16.Verify(proof, vk, wrong) == nil {
			return fmt.Errorf("%s: the proof verifies against a wrong public input", c.Name)
		}
	}
	return nil
}

// readInto opens the file at path and hands it to a gnark ReadFrom.
func readInto(path string, readFrom func(io.Reader) (int64, error)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := readFrom(bufio.NewReader(f)); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
