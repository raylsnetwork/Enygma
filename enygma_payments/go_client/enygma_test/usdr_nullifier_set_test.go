package enygma_test

// Regression test for the separate USDr nullifier set.
//
// The USDr circuit's nullifier formula is Poseidon(Poseidon(prevR, sk) mod P,
// BlockNumber), the same as the main circuit's, with no asset tag. An account
// whose main and USDr balances share a blinding factor therefore produces the
// same nullifier for both proofs of one transfer(). With one shared set the
// second insert reverted NullifierAlreadyUsed and the account was locked out.
// The USDr proof now records its nullifier in its own set.
//
// Uses the mock verifiers: the property under test is which set the contract
// writes each nullifier to.
//
// Run:
//   ENYGMA_CHAIN_URL=http://127.0.0.1:8545 ENYGMA_CHAIN_ID=1337 \
//   MY_KEY=<hardhat key> go test -run TestUsdrNullifierSeparateSet -v .

import (
	"context"
	"math/big"
	"strings"
	"testing"

	enygma "enygma_payments/go_client/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestUsdrNullifierSeparateSet(t *testing.T) {
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

	// submit sends a transfer from banks[0]. When shareNullifier is set, the
	// USDr leg reuses the main proof's nullifier value; otherwise it keeps the
	// helper's fresh one. usdrNullifier, if non-nil, overrides it.
	submit := func(nullifierSeed int64, shareNullifier bool, usdrNullifier *big.Int) error {
		pubSig, deltas := buildTransferSignal(t, instance, enygmaAddr, fp, nullifierSeed)
		usdrDeltas, usdrProof := buildMockUsdrLeg(t, instance, enygmaAddr, pubSig, accountIds, banks[0].addr)
		if shareNullifier {
			usdrProof.PublicSignal[usdrHelperNullifierOff] = pubSig[usdrHelperNullifierOff]
		}
		if usdrNullifier != nil {
			usdrProof.PublicSignal[usdrHelperNullifierOff] = usdrNullifier
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

	t.Run("MainAndUsdrShareNullifierValue", func(t *testing.T) {
		if err := submit(7001, true, nil); err != nil {
			t.Fatalf("a transfer whose two proofs carry the same nullifier value was rejected: %v", err)
		}
	})

	t.Run("UsdrNullifierReplayStillRejected", func(t *testing.T) {
		replay := big.NewInt(880_000_001)
		if err := submit(7002, false, replay); err != nil {
			t.Fatalf("first use of the USDr nullifier failed: %v", err)
		}
		err := submit(7003, false, replay)
		if err == nil {
			t.Fatal("VULNERABLE: a spent USDr nullifier was accepted a second time")
		}
		if !strings.Contains(err.Error(), "NullifierAlreadyUsed") {
			t.Fatalf("rejected, but not with NullifierAlreadyUsed: %v", err)
		}
	})
}
