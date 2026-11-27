package vkslots

import (
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/sha3"
)

// testKey returns a key with nPublic public inputs (nPublic+1 IC points),
// every coordinate distinct and derived from seed.
func testKey(seed int64, nPublic int) Key {
	n := seed * 1000
	next := func() *big.Int { n++; return big.NewInt(n) }
	k := Key{
		Alpha1: [2]*big.Int{next(), next()},
		Beta2:  [4]*big.Int{next(), next(), next(), next()},
		Gamma2: [4]*big.Int{next(), next(), next(), next()},
		Delta2: [4]*big.Int{next(), next(), next(), next()},
	}
	for i := 0; i <= nPublic; i++ {
		k.IC = append(k.IC, [2]*big.Int{next(), next()})
	}
	return k
}

func intp(n int) *int { return &n }

func TestCheckRefusesAKeyOfAnotherCircuitInASlot(t *testing.T) {
	m := Manifest{Slots: []Slot{
		{Slot: 0, Circuit: "Payment", NPublic: intp(4)},
		{Slot: 1, Circuit: "OwnershipErc721"},
	}}
	keys := []NamedKey{
		{Circuit: "Payment", Key: testKey(1, 4)},
		{Circuit: "DvPInitiator", Key: testKey(2, 8)},
	}
	err := m.Check(keys)
	if err == nil || !strings.Contains(err.Error(), `slot 1`) || !strings.Contains(err.Error(), `"OwnershipErc721"`) {
		t.Fatalf("Check = %v, want a refusal naming slot 1 and OwnershipErc721", err)
	}
}

func TestCheckRefusesAKeyCountOtherThanTheManifests(t *testing.T) {
	m := Manifest{Slots: []Slot{
		{Slot: 0, Circuit: "Payment"},
		{Slot: 1, Circuit: "OwnershipErc721"},
	}}
	payment := NamedKey{Circuit: "Payment", Key: testKey(1, 4)}
	erc721 := NamedKey{Circuit: "OwnershipErc721", Key: testKey(2, 3)}
	extra := NamedKey{Circuit: "DvPInitiator", Key: testKey(3, 8)}
	for name, keys := range map[string][]NamedKey{
		"fewer": {payment},
		"more":  {payment, erc721, extra},
	} {
		t.Run(name, func(t *testing.T) {
			err := m.Check(keys)
			if err == nil || !strings.Contains(err.Error(), "2 slots") {
				t.Fatalf("Check = %v, want a refusal naming the manifest's 2 slots", err)
			}
		})
	}
}

func TestCheckRefusesAKeyWithAnotherNumberOfPublicInputs(t *testing.T) {
	m := Manifest{Slots: []Slot{{Slot: 0, Circuit: "Payment", NPublic: intp(4)}}}
	err := m.Check([]NamedKey{{Circuit: "Payment", Key: testKey(1, 5)}})
	if err == nil || !strings.Contains(err.Error(), "slot 0") || !strings.Contains(err.Error(), "5 public inputs") {
		t.Fatalf("Check = %v, want a refusal naming slot 0 and the key's 5 public inputs", err)
	}
}

func TestCheckAcceptsAnyKeyInASlotThatPinsOnlyItsCircuit(t *testing.T) {
	m := Manifest{Slots: []Slot{
		{Slot: 0, Circuit: "Payment", NPublic: intp(4)},
		{Slot: 1, Circuit: "OwnershipErc721"},
	}}
	if err := m.Check([]NamedKey{
		{Circuit: "Payment", Key: testKey(1, 4)},
		{Circuit: "OwnershipErc721", Key: testKey(2, 1)},
	}); err != nil {
		t.Fatalf("Check = %v, want nil", err)
	}
}

// The expected hash is keccak256(abi.encode(vk)) of testKey(1, 2) as
// ethers 5.8.0's defaultAbiCoder encodes IEnygmaDvp.VerifyingKey.
func TestHashIsKeccakOfTheAbiEncodedVerifyingKey(t *testing.T) {
	const want = "0xe8f844f28c8cabce64ce2eeba33af5962d212bf08860cccc3a980b63538a7b1f"
	if got := testKey(1, 2).Hash(); got != want {
		t.Fatalf("Hash = %s, want %s", got, want)
	}
}

func TestCheckRefusesAKeyWhoseHashIsNotTheManifests(t *testing.T) {
	m := Manifest{Slots: []Slot{{Slot: 0, Circuit: "Payment", VKHash: testKey(1, 4).Hash()}}}
	if err := m.Check([]NamedKey{{Circuit: "Payment", Key: testKey(1, 4)}}); err != nil {
		t.Fatalf("Check of the pinned key = %v, want nil", err)
	}
	err := m.Check([]NamedKey{{Circuit: "Payment", Key: testKey(2, 4)}})
	if err == nil || !strings.Contains(err.Error(), "slot 0") || !strings.Contains(err.Error(), testKey(2, 4).Hash()) {
		t.Fatalf("Check = %v, want a refusal naming slot 0 and the key's hash", err)
	}
}

func TestLoadReadsAManifest(t *testing.T) {
	path := writeFile(t, `{"slots": [
		{"slot": 0, "circuit": "Payment", "nPublic": 4, "vkHash": "0xab"},
		{"slot": 1, "circuit": "OwnershipErc721"}
	]}`)
	m, err := Load(path)
	if err != nil {
		t.Fatalf("Load = %v", err)
	}
	if len(m.Slots) != 2 || m.Slots[0].Circuit != "Payment" || *m.Slots[0].NPublic != 4 ||
		m.Slots[0].VKHash != "0xab" || m.Slots[1].Circuit != "OwnershipErc721" || m.Slots[1].NPublic != nil {
		t.Fatalf("Load = %+v", m)
	}
}

func TestLoadRefusesAManifestItCannotReadWhole(t *testing.T) {
	for name, tc := range map[string]struct{ body, want string }{
		"unknown field": {`{"slots": [{"slot": 0, "circuit": "Payment", "npublic": 4}]}`, `unknown field "npublic"`},
		"no slots":      {`{"slots": []}`, "no slots"},
		"misnumbered":   {`{"slots": [{"slot": 0, "circuit": "A"}, {"slot": 2, "circuit": "B"}]}`, "entry 1 is numbered slot 2"},
		"no circuit":    {`{"slots": [{"slot": 0}]}`, "slot 0 names no circuit"},
		"trailing":      {`{"slots": [{"slot": 0, "circuit": "A"}]} {}`, "after the manifest"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Load(writeFile(t, tc.body))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Load = %v, want an error containing %q", err, tc.want)
			}
		})
	}
}

func writeFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vk_slots.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// verifierStorage lays keys out as solc 0.8.24 lays out Verifier.sol's
// storage (solc --storage-layout): _verificationKeys at slot 1, 15 slots a
// key (alpha1 0-1, beta2 2-5, gamma2 6-9, delta2 10-13, ic's length 14 and
// its points from keccak256 of that slot), _vkLength at slot 15001.
func verifierStorage(keys []Key) map[string]*big.Int {
	st := map[string]*big.Int{}
	put := func(slot, v *big.Int) { st[slot.String()] = v }
	for i, k := range keys {
		base := int64(1 + 15*i)
		words := [][]*big.Int{k.Alpha1[:], k.Beta2[:], k.Gamma2[:], k.Delta2[:]}
		off := int64(0)
		for _, ws := range words {
			for _, w := range ws {
				put(big.NewInt(base+off), w)
				off++
			}
		}
		icSlot := big.NewInt(base + 14)
		put(icSlot, big.NewInt(int64(len(k.IC))))
		h := sha3.NewLegacyKeccak256()
		h.Write(icSlot.FillBytes(make([]byte, 32)))
		data := new(big.Int).SetBytes(h.Sum(nil))
		for j, p := range k.IC {
			put(new(big.Int).Add(data, big.NewInt(int64(2*j))), p[0])
			put(new(big.Int).Add(data, big.NewInt(int64(2*j+1))), p[1])
		}
	}
	put(big.NewInt(15001), big.NewInt(int64(len(keys))))
	return st
}

// reader reads st as eth_getStorageAt would: a slot never written is 0.
func reader(st map[string]*big.Int) func(*big.Int) (*big.Int, error) {
	return func(slot *big.Int) (*big.Int, error) {
		if v, ok := st[slot.String()]; ok {
			return v, nil
		}
		return new(big.Int), nil
	}
}

func TestReadBackAcceptsTheKeysInitRegistered(t *testing.T) {
	keys := []Key{testKey(1, 4), testKey(2, 1), testKey(3, 8)}
	if err := ReadBack(reader(verifierStorage(keys)), keys); err != nil {
		t.Fatalf("ReadBack = %v, want nil", err)
	}
}

func TestReadBackRefusesASlotHoldingAnotherKey(t *testing.T) {
	keys := []Key{testKey(1, 4), testKey(2, 1), testKey(3, 8)}
	onChain := []Key{keys[0], testKey(9, 1), keys[2]}
	err := ReadBack(reader(verifierStorage(onChain)), keys)
	if err == nil || !strings.Contains(err.Error(), "slot 1") || !strings.Contains(err.Error(), testKey(9, 1).Hash()) {
		t.Fatalf("ReadBack = %v, want a refusal naming slot 1 and the hash it holds", err)
	}
}

func TestReadBackRefusesAVerifierHoldingMoreKeys(t *testing.T) {
	keys := []Key{testKey(1, 4), testKey(2, 1)}
	onChain := append(append([]Key{}, keys...), testKey(3, 8))
	err := ReadBack(reader(verifierStorage(onChain)), keys)
	if err == nil || !strings.Contains(err.Error(), "holds 3 keys") {
		t.Fatalf("ReadBack = %v, want a refusal naming the Verifier's 3 keys", err)
	}
}

func TestReadBackRefusesASlotWhoseKeyHasAnotherNumberOfICPoints(t *testing.T) {
	keys := []Key{testKey(1, 4), testKey(2, 1)}
	onChain := []Key{keys[0], testKey(2, 3)}
	err := ReadBack(reader(verifierStorage(onChain)), keys)
	if err == nil || !strings.Contains(err.Error(), "slot 1") || !strings.Contains(err.Error(), "4 IC points") {
		t.Fatalf("ReadBack = %v, want a refusal naming slot 1 and its 4 IC points", err)
	}
}
