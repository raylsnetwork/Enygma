package enygma_test

// Regression test for the USDr fee-recipient binding.
//
// The USDr circuit pins the whole fee to the participant whose public key is
// the FeeRecipientKey public signal, but that alone does not say WHO the
// recipient is: a sender could name any participant and have its transfer
// relayed for free. transfer() therefore requires FeeRecipientKey to equal the
// registered public key of msg.sender (the relayer, which pays the gas), or
// reverts InvalidFeeRecipient.
//
// Uses the mock verifiers (as usdr_fingerprint_gap_test.go does): the property
// under test is which public-signal cell the contract compares.
//
// Run:
//   ENYGMA_CHAIN_URL=http://127.0.0.1:8545 ENYGMA_CHAIN_ID=1337 \
//   MY_KEY=<hardhat key> go test -run TestUsdrFeeRecipientBinding -v .

import (
	"context"
	"math/big"
	"strings"
	"testing"

	enygma "enygma_payments/go_client/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestUsdrFeeRecipientBinding(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	client, err := ethclient.Dial(chainURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	instance, banks, enygmaAddr := c04Setup(t, client)
	fp := confirmAllFingerprints(t, client, instance, banks)

	participantIds := make([]*big.Int, nBanks)
	accountIds := make([]int64, nBanks)
	for i := 0; i < nBanks; i++ {
		participantIds[i] = big.NewInt(banks[i].accountID)
		accountIds[i] = banks[i].accountID
	}
	zeroProof := [8]*big.Int{}
	for i := range zeroProof {
		zeroProof[i] = big.NewInt(0)
	}

	// submit sends a transfer from banks[0] whose USDr leg names recipientKey as
	// the fee recipient (nil keeps the honest value: the submitter's own key).
	submit := func(nullifierSeed int64, recipientKey *big.Int) error {
		pubSig, deltas := buildTransferSignal(t, instance, enygmaAddr, fp, nullifierSeed)
		usdrDeltas, usdrProof := buildMockUsdrLeg(t, instance, enygmaAddr, pubSig, accountIds, banks[0].addr)
		if recipientKey != nil {
			usdrProof.PublicSignal[usdrHelperFeeRecipientOff] = recipientKey
		}
		tx, sendErr := instance.Transfer(bankAuth(t, client, banks[0]), deltas,
			enygma.IEnygmaProof{Proof: zeroProof, PublicSignal: pubSig},
			usdrDeltas, usdrProof, participantIds, "")
		if sendErr != nil {
			return sendErr
		}
		r, waitErr := bind.WaitMined(context.Background(), client, tx)
		if waitErr != nil {
			t.Fatalf("wait transfer: %v", waitErr)
		}
		if r.Status != 1 {
			return context.DeadlineExceeded // reverted on-chain
		}
		return nil
	}

	t.Run("RecipientIsSubmitter", func(t *testing.T) {
		if err := submit(555, nil); err != nil {
			t.Fatalf("control failed — transfer paying the submitter was rejected: %v", err)
		}
	})

	t.Run("RecipientIsAnotherParticipant", func(t *testing.T) {
		other, err := instance.PublicKeys(&bind.CallOpts{}, big.NewInt(banks[1].accountID))
		if err != nil {
			t.Fatalf("publicKeys: %v", err)
		}
		err = submit(556, other)
		if err == nil {
			t.Fatal("VULNERABLE: transfer() accepted a USDr proof whose fee recipient is not the submitter — the relayer can be made to relay for free")
		}
		if !strings.Contains(err.Error(), "InvalidFeeRecipient") {
			t.Fatalf("rejected, but not with InvalidFeeRecipient: %v", err)
		}
	})

	t.Run("RecipientIsUnregisteredKey", func(t *testing.T) {
		err := submit(557, big.NewInt(123456789))
		if err == nil || !strings.Contains(err.Error(), "InvalidFeeRecipient") {
			t.Fatalf("expected InvalidFeeRecipient, got: %v", err)
		}
	})
}
