// Property-based regression tests for the three fee-variant circuits that
// received the same StContractAddress/StTreeNumber fix as PaymentCircuit
// (see payment_test.go): PaymentFeeCircuit, PaymentRelayerFeePublicCircuit,
// and UsdrFeeCircuit. Each gets its own valid-witness happy path plus the
// two tamper tests that lock in the fix, mirroring payment_test.go's
// pattern exactly.
package test

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
	"github.com/iden3/go-iden3-crypto/poseidon"

	"enygma_dvp/gnark_circuits/templates"
)

const feeFamilyTestDepth = 2

// --- shared input-note fixture (1 input, depth 2) used by all 3 circuits ---

type feeFamilyInputFixture struct {
	sk, pk                   *big.Int
	tokenId, valueIn, saltIn *big.Int
	treeNumber, pathIndices  *big.Int
	siblings                 []*big.Int
	root, contractAddress    *big.Int
	nullifier                *big.Int
}

func buildFeeFamilyInputFixture(t *testing.T, tokenId int64) *feeFamilyInputFixture {
	t.Helper()
	f := &feeFamilyInputFixture{}

	f.sk = big.NewInt(12345)
	pk, err := poseidon.Hash([]*big.Int{f.sk})
	if err != nil {
		t.Fatal(err)
	}
	f.pk = pk

	f.tokenId = big.NewInt(tokenId)
	f.valueIn = big.NewInt(100)
	f.saltIn = big.NewInt(111)
	inputCommitment, err := poseidon.Hash([]*big.Int{f.pk, f.saltIn, f.valueIn, f.tokenId})
	if err != nil {
		t.Fatal(err)
	}

	f.treeNumber = big.NewInt(0)
	f.pathIndices = big.NewInt(0)
	f.siblings = []*big.Int{big.NewInt(222), big.NewInt(333)}
	root := inputCommitment
	for _, sib := range f.siblings {
		root = paymentTestHashLeftRight(root, sib)
	}
	f.root = root

	f.contractAddress = big.NewInt(0xABCD)

	globalIdx := big.NewInt(0) // treeNumber*2^depth + pathIndices = 0
	f.nullifier, err = poseidon.Hash([]*big.Int{f.sk, globalIdx, f.contractAddress})
	if err != nil {
		t.Fatal(err)
	}

	return f
}

func feeFamilyConfig() templates.PaymentCircuitConfig {
	return templates.PaymentCircuitConfig{
		TmNInputs:         1,
		TmMOutputs:        2,
		TmMerkleTreeDepth: feeFamilyTestDepth,
		TmRange:           frontend.Variable("1000000000000000000000000000000000000"),
	}
}

// ============================== PaymentFeeCircuit ==============================

func buildValidPaymentFeeWitness(t *testing.T) (*templates.PaymentFeeCircuit, *feeFamilyInputFixture) {
	t.Helper()
	f := buildFeeFamilyInputFixture(t, 7)

	fee := big.NewInt(10)

	otherSk := big.NewInt(999)
	pkOut0, err := poseidon.Hash([]*big.Int{otherSk})
	if err != nil {
		t.Fatal(err)
	}
	saltOut0 := big.NewInt(444)
	valueOut0 := big.NewInt(50)
	cmt0, err := poseidon.Hash([]*big.Int{pkOut0, saltOut0, valueOut0, f.tokenId})
	if err != nil {
		t.Fatal(err)
	}

	pkOut1 := f.pk // Alice's change, must be sender-owned
	saltOut1 := big.NewInt(555)
	valueOut1 := big.NewInt(40) // 50 + 40 + 10(fee) == 100
	cmt1, err := poseidon.Hash([]*big.Int{pkOut1, saltOut1, valueOut1, f.tokenId})
	if err != nil {
		t.Fatal(err)
	}

	c := &templates.PaymentFeeCircuit{
		Config:               feeFamilyConfig(),
		StMessage:            big.NewInt(0),
		StTreeNumbers:        []frontend.Variable{f.treeNumber},
		StMerkleRoots:        []frontend.Variable{f.root},
		StNullifiers:         []frontend.Variable{f.nullifier},
		StCommitmentsOut:     []frontend.Variable{cmt0, cmt1},
		StContractAddress:    f.contractAddress,
		StFee:                fee,
		WtPrivateKeysIn:      []frontend.Variable{f.sk},
		WtValuesIn:           []frontend.Variable{f.valueIn},
		WtSaltsIn:            []frontend.Variable{f.saltIn},
		WtPathElements:       [][]frontend.Variable{{f.siblings[0], f.siblings[1]}},
		WtPathIndices:        []frontend.Variable{f.pathIndices},
		WtTokenId:            f.tokenId,
		WtSpendPublicKeysOut: []frontend.Variable{pkOut0, pkOut1},
		WtValuesOut:          []frontend.Variable{valueOut0, valueOut1},
		WtSaltsOut:           []frontend.Variable{saltOut0, saltOut1},
	}
	return c, f
}

func emptyPaymentFeeCircuit() *templates.PaymentFeeCircuit {
	cfg := feeFamilyConfig()
	c := &templates.PaymentFeeCircuit{
		Config:               cfg,
		StTreeNumbers:        make([]frontend.Variable, cfg.TmNInputs),
		StMerkleRoots:        make([]frontend.Variable, cfg.TmNInputs),
		StNullifiers:         make([]frontend.Variable, cfg.TmNInputs),
		StCommitmentsOut:     make([]frontend.Variable, cfg.TmMOutputs),
		WtPrivateKeysIn:      make([]frontend.Variable, cfg.TmNInputs),
		WtValuesIn:           make([]frontend.Variable, cfg.TmNInputs),
		WtSaltsIn:            make([]frontend.Variable, cfg.TmNInputs),
		WtPathElements:       make([][]frontend.Variable, cfg.TmNInputs),
		WtPathIndices:        make([]frontend.Variable, cfg.TmNInputs),
		WtSpendPublicKeysOut: make([]frontend.Variable, cfg.TmMOutputs),
		WtValuesOut:          make([]frontend.Variable, cfg.TmMOutputs),
		WtSaltsOut:           make([]frontend.Variable, cfg.TmMOutputs),
	}
	for i := range c.WtPathElements {
		c.WtPathElements[i] = make([]frontend.Variable, cfg.TmMerkleTreeDepth)
	}
	return c
}

func TestPaymentFeeCircuit_ValidWitness_Succeeds(t *testing.T) {
	assert := test.NewAssert(t)
	wc, _ := buildValidPaymentFeeWitness(t)
	assert.ProverSucceeded(emptyPaymentFeeCircuit(), wc, test.WithCurves(ecc.BN254))
}

func TestPaymentFeeCircuit_TamperedContractAddress_Fails(t *testing.T) {
	assert := test.NewAssert(t)
	wc, _ := buildValidPaymentFeeWitness(t)
	wc.StContractAddress = big.NewInt(0xDEAD)
	assert.ProverFailed(emptyPaymentFeeCircuit(), wc, test.WithCurves(ecc.BN254))
}

func TestPaymentFeeCircuit_TamperedTreeNumber_Fails(t *testing.T) {
	assert := test.NewAssert(t)
	wc, _ := buildValidPaymentFeeWitness(t)
	wc.StTreeNumbers = []frontend.Variable{big.NewInt(1)}
	assert.ProverFailed(emptyPaymentFeeCircuit(), wc, test.WithCurves(ecc.BN254))
}

// ======================= PaymentRelayerFeePublicCircuit =======================

func buildValidPaymentRelayerFeePublicWitness(t *testing.T) *templates.PaymentRelayerFeePublicCircuit {
	t.Helper()
	f := buildFeeFamilyInputFixture(t, 7)

	fee := big.NewInt(20)

	bobSk := big.NewInt(999)
	pkOut0, err := poseidon.Hash([]*big.Int{bobSk})
	if err != nil {
		t.Fatal(err)
	}
	saltOut0 := big.NewInt(444)
	valueOut0 := big.NewInt(50)
	cmt0, err := poseidon.Hash([]*big.Int{pkOut0, saltOut0, valueOut0, f.tokenId})
	if err != nil {
		t.Fatal(err)
	}

	pkOut1 := f.pk // Alice's change
	saltOut1 := big.NewInt(555)
	valueOut1 := big.NewInt(30)
	cmt1, err := poseidon.Hash([]*big.Int{pkOut1, saltOut1, valueOut1, f.tokenId})
	if err != nil {
		t.Fatal(err)
	}

	relayerSk := big.NewInt(31337)
	pkOut2, err := poseidon.Hash([]*big.Int{relayerSk})
	if err != nil {
		t.Fatal(err)
	}
	saltOut2 := big.NewInt(666)
	valueOut2 := fee // 50 + 30 + 20 == 100, and WtValuesOut[2] == StFee
	cmt2, err := poseidon.Hash([]*big.Int{pkOut2, saltOut2, valueOut2, f.tokenId})
	if err != nil {
		t.Fatal(err)
	}

	cfg := feeFamilyConfig()
	cfg.TmMOutputs = 3

	return &templates.PaymentRelayerFeePublicCircuit{
		Config:               cfg,
		StMessage:            big.NewInt(0),
		StTreeNumbers:        []frontend.Variable{f.treeNumber},
		StMerkleRoots:        []frontend.Variable{f.root},
		StNullifiers:         []frontend.Variable{f.nullifier},
		StCommitmentsOut:     []frontend.Variable{cmt0, cmt1, cmt2},
		StContractAddress:    f.contractAddress,
		StFee:                fee,
		WtPrivateKeysIn:      []frontend.Variable{f.sk},
		WtValuesIn:           []frontend.Variable{f.valueIn},
		WtSaltsIn:            []frontend.Variable{f.saltIn},
		WtPathElements:       [][]frontend.Variable{{f.siblings[0], f.siblings[1]}},
		WtPathIndices:        []frontend.Variable{f.pathIndices},
		WtTokenId:            f.tokenId,
		WtSpendPublicKeysOut: []frontend.Variable{pkOut0, pkOut1, pkOut2},
		WtValuesOut:          []frontend.Variable{valueOut0, valueOut1, valueOut2},
		WtSaltsOut:           []frontend.Variable{saltOut0, saltOut1, saltOut2},
	}
}

func emptyPaymentRelayerFeePublicCircuit() *templates.PaymentRelayerFeePublicCircuit {
	cfg := feeFamilyConfig()
	cfg.TmMOutputs = 3
	c := &templates.PaymentRelayerFeePublicCircuit{
		Config:               cfg,
		StTreeNumbers:        make([]frontend.Variable, cfg.TmNInputs),
		StMerkleRoots:        make([]frontend.Variable, cfg.TmNInputs),
		StNullifiers:         make([]frontend.Variable, cfg.TmNInputs),
		StCommitmentsOut:     make([]frontend.Variable, cfg.TmMOutputs),
		WtPrivateKeysIn:      make([]frontend.Variable, cfg.TmNInputs),
		WtValuesIn:           make([]frontend.Variable, cfg.TmNInputs),
		WtSaltsIn:            make([]frontend.Variable, cfg.TmNInputs),
		WtPathElements:       make([][]frontend.Variable, cfg.TmNInputs),
		WtPathIndices:        make([]frontend.Variable, cfg.TmNInputs),
		WtSpendPublicKeysOut: make([]frontend.Variable, cfg.TmMOutputs),
		WtValuesOut:          make([]frontend.Variable, cfg.TmMOutputs),
		WtSaltsOut:           make([]frontend.Variable, cfg.TmMOutputs),
	}
	for i := range c.WtPathElements {
		c.WtPathElements[i] = make([]frontend.Variable, cfg.TmMerkleTreeDepth)
	}
	return c
}

func TestPaymentRelayerFeePublicCircuit_ValidWitness_Succeeds(t *testing.T) {
	assert := test.NewAssert(t)
	wc := buildValidPaymentRelayerFeePublicWitness(t)
	assert.ProverSucceeded(emptyPaymentRelayerFeePublicCircuit(), wc, test.WithCurves(ecc.BN254))
}

func TestPaymentRelayerFeePublicCircuit_TamperedContractAddress_Fails(t *testing.T) {
	assert := test.NewAssert(t)
	wc := buildValidPaymentRelayerFeePublicWitness(t)
	wc.StContractAddress = big.NewInt(0xDEAD)
	assert.ProverFailed(emptyPaymentRelayerFeePublicCircuit(), wc, test.WithCurves(ecc.BN254))
}

func TestPaymentRelayerFeePublicCircuit_TamperedTreeNumber_Fails(t *testing.T) {
	assert := test.NewAssert(t)
	wc := buildValidPaymentRelayerFeePublicWitness(t)
	wc.StTreeNumbers = []frontend.Variable{big.NewInt(1)}
	assert.ProverFailed(emptyPaymentRelayerFeePublicCircuit(), wc, test.WithCurves(ecc.BN254))
}

// ============================== UsdrFeeCircuit ==============================

func buildValidUsdrFeeWitness(t *testing.T) *templates.UsdrFeeCircuit {
	t.Helper()
	usdrTokenId := int64(0)
	f := buildFeeFamilyInputFixture(t, usdrTokenId)

	fee := big.NewInt(20)

	relayerSk := big.NewInt(31337)
	pkOut0, err := poseidon.Hash([]*big.Int{relayerSk})
	if err != nil {
		t.Fatal(err)
	}
	saltOut0 := big.NewInt(444)
	valueOut0 := fee // WtValuesOut[0] == StFee
	cmt0, err := poseidon.Hash([]*big.Int{pkOut0, saltOut0, valueOut0, f.tokenId})
	if err != nil {
		t.Fatal(err)
	}

	pkOut1 := f.pk // sender's change
	saltOut1 := big.NewInt(555)
	valueOut1 := big.NewInt(80) // 20 + 80 == 100
	cmt1, err := poseidon.Hash([]*big.Int{pkOut1, saltOut1, valueOut1, f.tokenId})
	if err != nil {
		t.Fatal(err)
	}

	cfg := feeFamilyConfig()

	return &templates.UsdrFeeCircuit{
		Config:               cfg,
		StMessage:            big.NewInt(0),
		StTreeNumbers:        []frontend.Variable{f.treeNumber},
		StMerkleRoots:        []frontend.Variable{f.root},
		StNullifiers:         []frontend.Variable{f.nullifier},
		StCommitmentsOut:     []frontend.Variable{cmt0, cmt1},
		StContractAddress:    f.contractAddress,
		StFee:                fee,
		StTokenId:            f.tokenId,
		WtPrivateKeysIn:      []frontend.Variable{f.sk},
		WtValuesIn:           []frontend.Variable{f.valueIn},
		WtSaltsIn:            []frontend.Variable{f.saltIn},
		WtPathElements:       [][]frontend.Variable{{f.siblings[0], f.siblings[1]}},
		WtPathIndices:        []frontend.Variable{f.pathIndices},
		WtSpendPublicKeysOut: []frontend.Variable{pkOut0, pkOut1},
		WtValuesOut:          []frontend.Variable{valueOut0, valueOut1},
		WtSaltsOut:           []frontend.Variable{saltOut0, saltOut1},
	}
}

func emptyUsdrFeeCircuit() *templates.UsdrFeeCircuit {
	cfg := feeFamilyConfig()
	c := &templates.UsdrFeeCircuit{
		Config:               cfg,
		StTreeNumbers:        make([]frontend.Variable, cfg.TmNInputs),
		StMerkleRoots:        make([]frontend.Variable, cfg.TmNInputs),
		StNullifiers:         make([]frontend.Variable, cfg.TmNInputs),
		StCommitmentsOut:     make([]frontend.Variable, cfg.TmMOutputs),
		WtPrivateKeysIn:      make([]frontend.Variable, cfg.TmNInputs),
		WtValuesIn:           make([]frontend.Variable, cfg.TmNInputs),
		WtSaltsIn:            make([]frontend.Variable, cfg.TmNInputs),
		WtPathElements:       make([][]frontend.Variable, cfg.TmNInputs),
		WtPathIndices:        make([]frontend.Variable, cfg.TmNInputs),
		WtSpendPublicKeysOut: make([]frontend.Variable, cfg.TmMOutputs),
		WtValuesOut:          make([]frontend.Variable, cfg.TmMOutputs),
		WtSaltsOut:           make([]frontend.Variable, cfg.TmMOutputs),
	}
	for i := range c.WtPathElements {
		c.WtPathElements[i] = make([]frontend.Variable, cfg.TmMerkleTreeDepth)
	}
	return c
}

func TestUsdrFeeCircuit_ValidWitness_Succeeds(t *testing.T) {
	assert := test.NewAssert(t)
	wc := buildValidUsdrFeeWitness(t)
	assert.ProverSucceeded(emptyUsdrFeeCircuit(), wc, test.WithCurves(ecc.BN254))
}

func TestUsdrFeeCircuit_TamperedContractAddress_Fails(t *testing.T) {
	assert := test.NewAssert(t)
	wc := buildValidUsdrFeeWitness(t)
	wc.StContractAddress = big.NewInt(0xDEAD)
	assert.ProverFailed(emptyUsdrFeeCircuit(), wc, test.WithCurves(ecc.BN254))
}

func TestUsdrFeeCircuit_TamperedTreeNumber_Fails(t *testing.T) {
	assert := test.NewAssert(t)
	wc := buildValidUsdrFeeWitness(t)
	wc.StTreeNumbers = []frontend.Variable{big.NewInt(1)}
	assert.ProverFailed(emptyUsdrFeeCircuit(), wc, test.WithCurves(ecc.BN254))
}
