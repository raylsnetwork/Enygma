// Package vkslots ties each verifying-key slot of the shared Verifier to the
// circuit that owns it.
package vkslots

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"slices"

	"golang.org/x/crypto/sha3"
)

// Key is a Groth16 verifying key as the Verifier stores it: G2 coordinates
// in EIP-197 order (x[0], x[1], y[0], y[1]).
type Key struct {
	Alpha1 [2]*big.Int
	Beta2  [4]*big.Int
	Gamma2 [4]*big.Int
	Delta2 [4]*big.Int
	IC     [][2]*big.Int
}

// NPublic is the number of public inputs the key verifies: one fewer than
// its IC points, the first of which is the constant term.
func (k Key) NPublic() int { return len(k.IC) - 1 }

// NamedKey is a key and the circuit it was loaded for.
type NamedKey struct {
	Circuit string
	Key     Key
}

// Slot is one manifest entry.
type Slot struct {
	Slot    int    `json:"slot"`
	Circuit string `json:"circuit"`
	NPublic *int   `json:"nPublic,omitempty"`
	VKHash  string `json:"vkHash,omitempty"`
}

// Manifest lists a deployment's slots in registration order. About is
// free text for the reader of the file.
type Manifest struct {
	About []string `json:"about,omitempty"`
	Slots []Slot   `json:"slots"`
}

// Check returns an error naming the first slot whose key does not match the
// manifest: keys[i] is the key init will register in slot i.
func (m Manifest) Check(keys []NamedKey) error {
	if len(keys) != len(m.Slots) {
		return fmt.Errorf("init would register %d keys, but the manifest has %d slots", len(keys), len(m.Slots))
	}
	for i, k := range keys {
		want := m.Slots[i]
		if k.Circuit != want.Circuit {
			return fmt.Errorf("slot %d: the manifest gives it to %q, but init would register %q's key there", i, want.Circuit, k.Circuit)
		}
		if want.NPublic != nil && k.Key.NPublic() != *want.NPublic {
			return fmt.Errorf("slot %d (%s): the manifest pins %d public inputs, but the key has %d public inputs", i, want.Circuit, *want.NPublic, k.Key.NPublic())
		}
		if want.VKHash != "" && k.Key.Hash() != want.VKHash {
			return fmt.Errorf("slot %d (%s): the manifest pins vk hash %s, but the key's is %s", i, want.Circuit, want.VKHash, k.Key.Hash())
		}
	}
	return nil
}

// fixedWords returns the key's 14 fixed-size words in declaration order:
// alpha1, then beta2, gamma2 and delta2 (x[0], x[1], y[0], y[1] each).
func (k Key) fixedWords() []*big.Int {
	w := append([]*big.Int{}, k.Alpha1[:]...)
	w = append(w, k.Beta2[:]...)
	w = append(w, k.Gamma2[:]...)
	return append(w, k.Delta2[:]...)
}

// Hash returns keccak256(abi.encode(vk)) of the key as an
// IEnygmaDvp.VerifyingKey, 0x-prefixed hex.
func (k Key) Hash() string {
	fixed := k.fixedWords()
	// abi.encode of a struct with a dynamic member: the offset of the
	// struct, its fixed words, the offset of ic within the struct, then ic's
	// length and points.
	words := []*big.Int{big.NewInt(32)}
	words = append(words, fixed...)
	words = append(words, big.NewInt(int64(32*(len(fixed)+1))), big.NewInt(int64(len(k.IC))))
	for _, p := range k.IC {
		words = append(words, p[0], p[1])
	}
	h := sha3.NewLegacyKeccak256()
	var buf [32]byte
	for _, w := range words {
		w.FillBytes(buf[:])
		h.Write(buf[:])
	}
	return "0x" + hex.EncodeToString(h.Sum(nil))
}

// Load reads the manifest at path. It refuses a file it cannot read whole:
// a field it does not know (a misspelt "nPublic" would otherwise leave the
// slot unpinned), no slots, a slot numbered out of order, a slot with no
// circuit, or anything after the manifest's JSON object.
func Load(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", path, err)
	}
	if dec.More() {
		return Manifest{}, fmt.Errorf("%s: more JSON after the manifest", path)
	}
	if err := exactKeys(data); err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", path, err)
	}
	if len(m.Slots) == 0 {
		return Manifest{}, fmt.Errorf("%s: no slots", path)
	}
	for i, s := range m.Slots {
		if s.Slot != i {
			return Manifest{}, fmt.Errorf("%s: entry %d is numbered slot %d", path, i, s.Slot)
		}
		if s.Circuit == "" {
			return Manifest{}, fmt.Errorf("%s: slot %d names no circuit", path, i)
		}
	}
	return m, nil
}

// exactKeys refuses a key spelt other than the manifest's own: encoding/json
// matches field names case-insensitively, so it reads "npublic" as nPublic.
func exactKeys(data []byte) error {
	var raw struct {
		Top   map[string]json.RawMessage
		Slots []map[string]json.RawMessage
	}
	if err := json.Unmarshal(data, &raw.Top); err != nil {
		return err
	}
	if err := json.Unmarshal(raw.Top["slots"], &raw.Slots); err != nil {
		return err
	}
	allowed := func(keys map[string]json.RawMessage, names ...string) error {
		for k := range keys {
			if !slices.Contains(names, k) {
				return fmt.Errorf("unknown field %q", k)
			}
		}
		return nil
	}
	if err := allowed(raw.Top, "about", "slots"); err != nil {
		return err
	}
	for _, slot := range raw.Slots {
		if err := allowed(slot, "slot", "circuit", "nPublic", "vkHash"); err != nil {
			return err
		}
	}
	return nil
}

// The Verifier's storage layout (solc --storage-layout of Verifier.sol):
// _verificationKeys starts at slot 1, each key takes 15 slots (alpha1,
// beta2, gamma2 and delta2's 14 words, then ic's length, whose points start
// at keccak256 of that slot, two slots a point), and _vkLength follows the
// 1000 keys.
const (
	keysSlot     = 1
	slotsPerKey  = 15
	icOffset     = 14
	vkLengthSlot = keysSlot + slotsPerKey*1000
)

// StorageReader returns the word at a storage slot of the Verifier, as
// eth_getStorageAt does.
type StorageReader func(slot *big.Int) (*big.Int, error)

// ReadBack reads the Verifier's keys back from its storage and returns an
// error unless it holds exactly keys, in order: keys[i] is what init
// registered in slot i.
func ReadBack(read StorageReader, keys []Key) error {
	n, err := read(big.NewInt(vkLengthSlot))
	if err != nil {
		return fmt.Errorf("read the Verifier's key count: %w", err)
	}
	if !n.IsInt64() || n.Int64() != int64(len(keys)) {
		return fmt.Errorf("the Verifier holds %s keys, but init registered %d", n, len(keys))
	}
	for i, want := range keys {
		got, err := readKey(read, i, len(want.IC))
		if err != nil {
			return fmt.Errorf("slot %d: %w", i, err)
		}
		if got.Hash() != want.Hash() {
			return fmt.Errorf("slot %d: the Verifier holds vk hash %s, but init registered %s", i, got.Hash(), want.Hash())
		}
	}
	return nil
}

// Resume returns how many of keys the Verifier already holds, keys[i] in
// slot i, so that a run of init that stopped part-way registers only the
// rest, keys[n:]. Slots only ever get appended, so it refuses a Verifier
// holding more keys than keys, or a slot holding another key: registering
// on top of either would leave keys in the wrong slots for good.
func Resume(read StorageReader, keys []Key) (int, error) {
	n, err := read(big.NewInt(vkLengthSlot))
	if err != nil {
		return 0, fmt.Errorf("read the Verifier's key count: %w", err)
	}
	if !n.IsInt64() || n.Int64() > int64(len(keys)) {
		return 0, fmt.Errorf("the Verifier holds %s keys, more than the %d init registers", n, len(keys))
	}
	held := int(n.Int64())
	if err := ReadBack(read, keys[:held]); err != nil {
		return 0, err
	}
	return held, nil
}

// readKey reads slot i's key, refusing one whose ic does not have nIC points.
func readKey(read StorageReader, i, nIC int) (Key, error) {
	base := int64(keysSlot + slotsPerKey*i)
	word := func(off int64) (*big.Int, error) { return read(big.NewInt(base + off)) }
	var k Key
	fixed := make([]*big.Int, icOffset)
	for off := range fixed {
		w, err := word(int64(off))
		if err != nil {
			return Key{}, err
		}
		fixed[off] = w
	}
	copy(k.Alpha1[:], fixed[0:2])
	copy(k.Beta2[:], fixed[2:6])
	copy(k.Gamma2[:], fixed[6:10])
	copy(k.Delta2[:], fixed[10:14])
	icSlot := big.NewInt(base + icOffset)
	n, err := read(icSlot)
	if err != nil {
		return Key{}, err
	}
	if !n.IsInt64() || n.Int64() != int64(nIC) {
		return Key{}, fmt.Errorf("the Verifier holds a key with %s IC points, but init registered one with %d", n, nIC)
	}
	h := sha3.NewLegacyKeccak256()
	h.Write(icSlot.FillBytes(make([]byte, 32)))
	data := new(big.Int).SetBytes(h.Sum(nil))
	for j := 0; j < nIC; j++ {
		x, err := read(new(big.Int).Add(data, big.NewInt(int64(2*j))))
		if err != nil {
			return Key{}, err
		}
		y, err := read(new(big.Int).Add(data, big.NewInt(int64(2*j+1))))
		if err != nil {
			return Key{}, err
		}
		k.IC = append(k.IC, [2]*big.Int{x, y})
	}
	return k, nil
}

// CheckFile loads the manifest at path and checks keys against it, naming
// path in any error.
func CheckFile(path string, keys []NamedKey) error {
	m, err := Load(path)
	if err != nil {
		return err
	}
	if err := m.Check(keys); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
