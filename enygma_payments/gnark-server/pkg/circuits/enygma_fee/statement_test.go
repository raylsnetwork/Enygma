package enygma_fee

// The transfer-with-fee circuit's statement layout must agree with the public
// witness gnark actually produces from EnygmaFeeCircuit: same length, and
// every named field's offset/size slice holding exactly that field's values.
// Sentinels are assigned through the circuit struct by hand (X before Y), so
// these tests also pin the flattening order.
//
// Run:
//
//	CC=/usr/bin/clang go test ./pkg/circuits/enygma_fee/... -run TestStatementLayout -v

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark/frontend"
)

var k = DefaultConfig.NCommitment

// sentinel is a value unique to (field, element): field*1000 + element.
func sentinel(field, element int) *big.Int {
	return big.NewInt(int64(field*1000 + element))
}

// Field numbers for sentinels; independent of the layout under test.
const (
	sHashedSharedSecrets = iota + 1
	sPublicKeys
	sPreviousCommitments
	sTxCommitments
	sBlockNumber
	sAnonymitySet
	sMessageTags
	sNullifier
	sFeeAmount
	sSumTxCommitments
	sSumTxValuesWithFee
	sDomainId
)

func sentinelWitness() EnygmaFeeCircuit {
	w := createCircuitTemplate(DefaultConfig)
	for i := 0; i < k; i++ {
		w.HashedSharedSecrets[i] = sentinel(sHashedSharedSecrets, i)
		w.PublicKey[i] = sentinel(sPublicKeys, i)
		w.PreviousCommit[i] = [2]frontend.Variable{sentinel(sPreviousCommitments, 2*i), sentinel(sPreviousCommitments, 2*i+1)} // X, Y
		w.TxCommit[i] = [2]frontend.Variable{sentinel(sTxCommitments, 2*i), sentinel(sTxCommitments, 2*i+1)}
		w.AnonymitySet[i] = sentinel(sAnonymitySet, i)
		w.MessageTags[i] = sentinel(sMessageTags, i)
	}
	w.BlockNumber = sentinel(sBlockNumber, 0)
	w.Nullifier = sentinel(sNullifier, 0)
	w.Fee = sentinel(sFeeAmount, 0)
	w.SumTxCommit = [2]frontend.Variable{sentinel(sSumTxCommitments, 0), sentinel(sSumTxCommitments, 1)} // X, Y
	w.SumTxValuesWithFee = sentinel(sSumTxValuesWithFee, 0)
	w.DomainId = sentinel(sDomainId, 0)
	return w
}

func publicWitness(t *testing.T) fr.Vector {
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
	if want := len(publicWitness(t)); got != want {
		t.Fatalf("layout length %d, public witness length %d", got, want)
	}
}

func TestStatementLayout_FieldsSliceTheirOwnSentinels(t *testing.T) {
	pub := publicWitness(t)
	cases := []struct {
		name  string
		field int
	}{
		{"HashedSharedSecrets", sHashedSharedSecrets},
		{"PublicKeys", sPublicKeys},
		{"PreviousCommitments", sPreviousCommitments},
		{"TxCommitments", sTxCommitments},
		{"BlockNumber", sBlockNumber},
		{"AnonymitySet", sAnonymitySet},
		{"MessageTags", sMessageTags},
		{"Nullifier", sNullifier},
		{"FeeAmount", sFeeAmount},
		{"SumTxCommitments", sSumTxCommitments},
		{"SumTxValuesWithFee", sSumTxValuesWithFee},
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
