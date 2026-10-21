package burn

// The burn circuit's statement layout must agree with the public witness
// gnark actually produces from BurnCircuit: same length, and every named
// field's offset/size slice holding exactly that field's values. Sentinels
// are assigned through the circuit struct by hand (X before Y), so these
// tests also pin the flattening order.
//
// Run:
//
//	CC=/usr/bin/clang go test ./pkg/circuits/burn/... -run TestStatementLayout -v

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark/frontend"
)

// sentinel is a value unique to (field, element): field*1000 + element.
func sentinel(field, element int) *big.Int {
	return big.NewInt(int64(field*1000 + element))
}

// Field numbers for sentinels; independent of the layout under test.
const (
	sPublicKey = iota + 1
	sPreviousCommitment
	sNewCommitment
	sAmount
	sBlockNumber
	sNullifier
	sDomainId
)

func sentinelWitness() BurnCircuit {
	return BurnCircuit{
		PublicKey:      sentinel(sPublicKey, 0),
		PreviousCommit: [2]frontend.Variable{sentinel(sPreviousCommitment, 0), sentinel(sPreviousCommitment, 1)}, // X, Y
		NewCommit:      [2]frontend.Variable{sentinel(sNewCommitment, 0), sentinel(sNewCommitment, 1)},
		Amount:         sentinel(sAmount, 0),
		BlockNumber:    sentinel(sBlockNumber, 0),
		Nullifier:      sentinel(sNullifier, 0),
		DomainId:       sentinel(sDomainId, 0),
	}
}

func sentinelPublicWitness(t *testing.T) fr.Vector {
	t.Helper()
	w := sentinelWitness()
	pub, err := frontend.NewWitness(&w, ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		t.Fatalf("NewWitness: %v", err)
	}
	return pub.Vector().(fr.Vector)
}

func TestStatementLayout_LengthMatchesPublicWitness(t *testing.T) {
	got := StatementLayout().Len
	if want := len(sentinelPublicWitness(t)); got != want {
		t.Fatalf("layout length %d, public witness length %d", got, want)
	}
}

func TestStatementLayout_FieldsSliceTheirOwnSentinels(t *testing.T) {
	pub := sentinelPublicWitness(t)
	cases := []struct {
		name  string
		field int
	}{
		{"PublicKey", sPublicKey},
		{"PreviousCommitment", sPreviousCommitment},
		{"NewCommitment", sNewCommitment},
		{"Amount", sAmount},
		{"BlockNumber", sBlockNumber},
		{"Nullifier", sNullifier},
		{"DomainId", sDomainId},
	}
	layout := StatementLayout()
	if len(layout.Fields) != len(cases) {
		t.Fatalf("layout has %d fields, want %d", len(layout.Fields), len(cases))
	}
	for _, tc := range cases {
		f, ok := layout.Field(tc.name)
		if !ok {
			t.Errorf("layout has no field %q", tc.name)
			continue
		}
		if f.Offset+f.Size > len(pub) {
			t.Errorf("%s: [%d,%d) out of range of %d signals", tc.name, f.Offset, f.Offset+f.Size, len(pub))
			continue
		}
		for e := 0; e < f.Size; e++ {
			var got big.Int
			pub[f.Offset+e].BigInt(&got)
			if want := sentinel(tc.field, e); got.Cmp(want) != 0 {
				t.Errorf("%s[%d] (signal %d) = %s, want %s", tc.name, e, f.Offset+e, &got, want)
			}
		}
	}
}
