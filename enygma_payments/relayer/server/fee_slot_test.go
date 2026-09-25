package server

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"strings"
	"testing"

	"enygma_payments/relayer/config"
	enygma "enygma_payments/relayer/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// feeSlotChain is a chain double answering only what verifyFeeSlot reads.
type feeSlotChain struct {
	fee       *big.Int
	accountID map[common.Address]*big.Int
	feeErr    error
}

func (m *feeSlotChain) Transfer(*bind.TransactOpts, []enygma.IEnygmaPoint, enygma.IEnygmaProof, []enygma.IEnygmaPoint, enygma.IEnygmaUsdrProof, []*big.Int, string) (*types.Transaction, error) {
	return nil, nil
}
func (m *feeSlotChain) TransferWithFee(*bind.TransactOpts, []enygma.IEnygmaPoint, enygma.IEnygmaFeeProof, []*big.Int, string) (*types.Transaction, error) {
	return nil, nil
}
func (m *feeSlotChain) UsdrFixedFeeAmount(*bind.CallOpts) (*big.Int, error) { return m.fee, m.feeErr }
func (m *feeSlotChain) AddressToAccountId(_ *bind.CallOpts, a common.Address) (*big.Int, error) {
	if id, ok := m.accountID[a]; ok {
		return id, nil
	}
	return big.NewInt(0), nil
}

type feeSlotFixture struct {
	h            *Handler
	signal       [UsdrFeePublicSignalLen]*big.Int
	commitments  []enygma.IEnygmaPoint
	participants []*big.Int
	randomness   *big.Int
	relayerSlot  int
}

// newFeeSlotFixture builds a valid six-participant USDr leg in which account 6
// (the relayer) is paid `fee` with blinding `r`, and nobody else is.
func newFeeSlotFixture(t *testing.T) feeSlotFixture {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	relayer := crypto.PubkeyToAddress(*key.Public().(*ecdsa.PublicKey))
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(1337))
	if err != nil {
		t.Fatal(err)
	}
	fee := big.NewInt(10)
	chain := &feeSlotChain{fee: fee, accountID: map[common.Address]*big.Int{relayer: big.NewInt(6)}}
	cfg := &config.Config{VerifyFeeSlot: true, ChainID: big.NewInt(1337)}
	h := NewHandlerWithDeps(cfg, "0x0", auth, nil, chain)

	f := feeSlotFixture{h: h, randomness: big.NewInt(987654321), relayerSlot: 5}
	for i := 0; i < 6; i++ {
		f.participants = append(f.participants, big.NewInt(int64(i+1)))
	}
	for i := range f.signal {
		f.signal[i] = big.NewInt(0)
	}
	f.signal[usdrFeeAmountOffset] = new(big.Int).Set(fee)
	for i := 0; i < 6; i++ {
		var pt = pedersen(big.NewInt(0), big.NewInt(int64(1000+i))) // other slots: no fee
		if i == f.relayerSlot {
			pt = pedersen(fee, f.randomness)
		}
		f.signal[usdrTxCommitOffset+2*i], f.signal[usdrTxCommitOffset+2*i+1] = pt.X, pt.Y
		f.commitments = append(f.commitments, enygma.IEnygmaPoint{C1: pt.X, C2: pt.Y})
	}
	return f
}

func (f feeSlotFixture) verify(randomness string) error {
	return f.h.verifyFeeSlot(context.Background(), f.signal, f.commitments, f.participants, randomness)
}

func TestVerifyFeeSlot_AcceptsAPaidRelayer(t *testing.T) {
	f := newFeeSlotFixture(t)
	if err := f.verify(f.randomness.String()); err != nil {
		t.Fatalf("a transfer that pays the relayer the full fee was rejected: %v", err)
	}
}

func TestVerifyFeeSlot_RejectsWrongRandomness(t *testing.T) {
	f := newFeeSlotFixture(t)
	if err := f.verify("12345"); err == nil {
		t.Fatal("a randomness that does not open the relayer's commitment was accepted")
	}
}

func TestVerifyFeeSlot_RejectsFeePaidToAnotherSlot(t *testing.T) {
	f := newFeeSlotFixture(t)
	// The sender credits the fee to slot 2 instead, and leaves the relayer's
	// slot with a zero-value note.
	fee := f.signal[usdrFeeAmountOffset]
	to := pedersen(fee, big.NewInt(555))
	f.signal[usdrTxCommitOffset+2*2], f.signal[usdrTxCommitOffset+2*2+1] = to.X, to.Y
	f.commitments[2] = enygma.IEnygmaPoint{C1: to.X, C2: to.Y}
	own := pedersen(big.NewInt(0), f.randomness)
	f.signal[usdrTxCommitOffset+2*f.relayerSlot], f.signal[usdrTxCommitOffset+2*f.relayerSlot+1] = own.X, own.Y
	f.commitments[f.relayerSlot] = enygma.IEnygmaPoint{C1: own.X, C2: own.Y}

	err := f.verify(f.randomness.String())
	if err == nil || !strings.Contains(err.Error(), "not being paid") {
		t.Fatalf("a transfer that pays another slot was not rejected as unpaid: %v", err)
	}
}

func TestVerifyFeeSlot_RejectsPartialFee(t *testing.T) {
	f := newFeeSlotFixture(t)
	part := pedersen(big.NewInt(1), f.randomness) // 1 of a 10 fee
	f.signal[usdrTxCommitOffset+2*f.relayerSlot], f.signal[usdrTxCommitOffset+2*f.relayerSlot+1] = part.X, part.Y
	f.commitments[f.relayerSlot] = enygma.IEnygmaPoint{C1: part.X, C2: part.Y}
	if err := f.verify(f.randomness.String()); err == nil {
		t.Fatal("a fee note worth 1 of a fixed fee of 10 was accepted")
	}
}

func TestVerifyFeeSlot_RejectsRelayerNotAParticipant(t *testing.T) {
	f := newFeeSlotFixture(t)
	f.participants[f.relayerSlot] = big.NewInt(9) // account 6 is not in the set
	if err := f.verify(f.randomness.String()); err == nil {
		t.Fatal("a transfer whose participants exclude the relayer was accepted")
	}
}

func TestVerifyFeeSlot_RejectsUnregisteredRelayer(t *testing.T) {
	f := newFeeSlotFixture(t)
	f.h.instance.(*feeSlotChain).accountID = map[common.Address]*big.Int{}
	if err := f.verify(f.randomness.String()); err == nil {
		t.Fatal("an unregistered relayer address was accepted")
	}
}

func TestVerifyFeeSlot_RejectsFeeAmountThatIsNotTheContractsFee(t *testing.T) {
	f := newFeeSlotFixture(t)
	f.signal[usdrFeeAmountOffset] = big.NewInt(11)
	if err := f.verify(f.randomness.String()); err == nil {
		t.Fatal("a USDr proof carrying a different FeeAmount was accepted")
	}
}

func TestVerifyFeeSlot_RequiresTheRandomness(t *testing.T) {
	f := newFeeSlotFixture(t)
	if err := f.verify(""); err == nil {
		t.Fatal("a request without usdrFeeRandomness was accepted while verification is on")
	}
}

func TestVerifyFeeSlot_RejectsMismatchedCommitmentsArray(t *testing.T) {
	f := newFeeSlotFixture(t)
	f.commitments[f.relayerSlot] = enygma.IEnygmaPoint{C1: big.NewInt(1), C2: big.NewInt(2)}
	if err := f.verify(f.randomness.String()); err == nil {
		t.Fatal("usdrCommitments that disagree with the proof's public signals were accepted")
	}
}

func TestVerifyFeeSlot_ChainReadFailureIsALookupError(t *testing.T) {
	f := newFeeSlotFixture(t)
	f.h.instance.(*feeSlotChain).feeErr = context.DeadlineExceeded
	err := f.verify(f.randomness.String())
	if _, ok := err.(*feeSlotLookupError); !ok {
		t.Fatalf("a chain read failure should be a *feeSlotLookupError so the handler answers 502, got %T: %v", err, err)
	}
}

func TestVerifyFeeSlot_DisabledSkipsTheCheck(t *testing.T) {
	f := newFeeSlotFixture(t)
	f.h.cfg.VerifyFeeSlot = false
	if err := f.verify(""); err != nil {
		t.Fatalf("verification is off but the request was rejected: %v", err)
	}
}
