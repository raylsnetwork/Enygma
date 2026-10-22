package templates

// Each Payment-family statement layout must agree with the public witness
// gnark actually produces from its circuit at its deployed config: same
// length, and every named field's offset/size slice holding exactly that
// field's values. Sentinels are assigned through the circuit structs by hand,
// so these tests also pin the order gnark flattens the slices in.
//
// Run:
//
//	CC=/usr/bin/clang go test ./templates/... -run TestStatementLayout -v

import (
	"math/big"
	"testing"

	"enygma_gnark_shared/statement"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark/frontend"
)

// sentinel is a value unique to (field, element): field*1000 + element.
func sentinel(field, element int) *big.Int {
	return big.NewInt(int64(field*1000 + element))
}

// Field numbers for sentinels; independent of the layouts under test.
const (
	sMessage = iota + 1
	sTreeNumbers
	sMerkleRoots
	sNullifiers
	sCommitmentsOut
	sContractAddress
	sFee
	sTokenId
)

func fill(vs []frontend.Variable, field int) {
	for i := range vs {
		vs[i] = sentinel(field, i)
	}
}

// fillPayment assigns sentinels to the public fields every Payment-family
// circuit has.
func fillPayment(message *frontend.Variable, trees, roots, nullifiers, commitments []frontend.Variable, contractAddress *frontend.Variable) {
	*message = sentinel(sMessage, 0)
	fill(trees, sTreeNumbers)
	fill(roots, sMerkleRoots)
	fill(nullifiers, sNullifiers)
	fill(commitments, sCommitmentsOut)
	*contractAddress = sentinel(sContractAddress, 0)
}

var paymentFields = []sentinelField{
	{"Message", sMessage},
	{"TreeNumbers", sTreeNumbers},
	{"MerkleRoots", sMerkleRoots},
	{"Nullifiers", sNullifiers},
	{"CommitmentsOut", sCommitmentsOut},
	{"ContractAddress", sContractAddress},
}

var feeFields = append(append([]sentinelField{}, paymentFields...), sentinelField{"Fee", sFee})

var usdrFeeFields = append(append([]sentinelField{}, feeFields...), sentinelField{"TokenId", sTokenId})

type sentinelField struct {
	name  string
	field int
}

type sentinelCase struct {
	layout  statement.Layout
	witness frontend.Circuit
	fields  []sentinelField
}

func sentinelCases() []sentinelCase {
	payment := func(cfg PaymentCircuitConfig) *PaymentCircuit {
		c := NewPaymentCircuit(cfg)
		fillPayment(&c.StMessage, c.StTreeNumbers, c.StMerkleRoots, c.StNullifiers, c.StCommitmentsOut, &c.StContractAddress)
		return &c
	}
	fee := NewPaymentFeeCircuit(PaymentFeeConfig)
	fillPayment(&fee.StMessage, fee.StTreeNumbers, fee.StMerkleRoots, fee.StNullifiers, fee.StCommitmentsOut, &fee.StContractAddress)
	fee.StFee = sentinel(sFee, 0)
	relayer := NewPaymentRelayerFeePublicCircuit(PaymentRelayerFeePublicConfig)
	fillPayment(&relayer.StMessage, relayer.StTreeNumbers, relayer.StMerkleRoots, relayer.StNullifiers, relayer.StCommitmentsOut, &relayer.StContractAddress)
	relayer.StFee = sentinel(sFee, 0)
	usdr := NewUsdrFeeCircuit(UsdrFeeConfig)
	fillPayment(&usdr.StMessage, usdr.StTreeNumbers, usdr.StMerkleRoots, usdr.StNullifiers, usdr.StCommitmentsOut, &usdr.StContractAddress)
	usdr.StFee = sentinel(sFee, 0)
	usdr.StTokenId = sentinel(sTokenId, 0)

	return []sentinelCase{
		{PaymentStatementLayout(), payment(PaymentConfig), paymentFields},
		{Payment2inStatementLayout(), payment(Payment2inConfig), paymentFields},
		{PaymentFeeStatementLayout(), &fee, feeFields},
		{PaymentRelayerFeeStatementLayout(), &relayer, feeFields},
		{UsdrFeeStatementLayout(), &usdr, usdrFeeFields},
	}
}

func publicVector(t *testing.T, w frontend.Circuit) fr.Vector {
	t.Helper()
	pub, err := frontend.NewWitness(w, ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		t.Fatalf("NewWitness: %v", err)
	}
	return pub.Vector().(fr.Vector)
}

func TestStatementLayout_LengthMatchesPublicWitness(t *testing.T) {
	for _, tc := range sentinelCases() {
		if got, want := tc.layout.Len, len(publicVector(t, tc.witness)); got != want {
			t.Errorf("%s: layout length %d, public witness length %d", tc.layout.Circuit, got, want)
		}
	}
}

func TestStatementLayout_FieldsSliceTheirOwnSentinels(t *testing.T) {
	for _, tc := range sentinelCases() {
		pub := publicVector(t, tc.witness)
		l := tc.layout
		if len(l.Fields) != len(tc.fields) {
			t.Errorf("%s: layout has %d fields, want %d", l.Circuit, len(l.Fields), len(tc.fields))
		}
		for _, sf := range tc.fields {
			f, ok := l.Field(sf.name)
			if !ok {
				t.Errorf("%s: layout has no field %q", l.Circuit, sf.name)
				continue
			}
			if f.Offset+f.Size > len(pub) {
				t.Errorf("%s.%s: [%d,%d) out of range of %d signals", l.Circuit, sf.name, f.Offset, f.Offset+f.Size, len(pub))
				continue
			}
			for e := 0; e < f.Size; e++ {
				var got big.Int
				pub[f.Offset+e].BigInt(&got)
				if want := sentinel(sf.field, e); got.Cmp(want) != 0 {
					t.Errorf("%s.%s[%d] (signal %d) = %s, want %s", l.Circuit, sf.name, e, f.Offset+e, &got, want)
				}
			}
		}
	}
}

// The contracts read a receipt's message, and a 1-input receipt's output
// commitments, at Payment's offsets without knowing which Payment-family
// circuit proved it, so every layout must put those fields where Payment
// does (Payment2in: the message only).
func TestStatementLayout_FamilySharesPaymentsPrefix(t *testing.T) {
	payment := PaymentStatementLayout()
	for _, l := range StatementLayouts() {
		shared := []string{"Message"}
		if l.Circuit != "Payment2in" {
			shared = append(shared, "CommitmentsOut")
		}
		for _, name := range shared {
			f, _ := l.Field(name)
			p, _ := payment.Field(name)
			if f.Offset != p.Offset {
				t.Errorf("%s.%s at offset %d, Payment's at %d", l.Circuit, name, f.Offset, p.Offset)
			}
		}
	}
}
