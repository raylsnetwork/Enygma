package enygma_test

// A verifier that reports an invalid proof by RETURNING false rather than
// reverting must not make transfer() accept the proof. The generated gnark
// verifiers revert, so a bare "did the staticcall revert" check happens to
// work for them — but enygma_dvp's GenericGroth16Verifier returns false, and
// the same unchecked pattern there let forged proofs through (fixed in
// enygma_dvp 4dc5181). MockFalseVerifier reproduces that behaviour.
//
// Each sub-test first runs a control transfer with a verifier that returns
// true (must succeed), so a rejection in the second step is attributable to
// the verifier's result and not to a malformed signal.
//
// Run:
//   ENYGMA_CHAIN_URL=http://127.0.0.1:8545 ENYGMA_CHAIN_ID=1337 \
//   MY_KEY=<hardhat key> go test -run TestVerifierReturningFalseIsRejected -v .

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"testing"

	enygma "enygma_payments/go_client/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestVerifierReturningFalseIsRejected(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	ctx := context.Background()
	client, err := ethclient.Dial(chainURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	instance, banks, enygmaAddr := c04Setup(t, client)
	fp := confirmAllFingerprints(t, client, instance, banks)

	ownerKey := mustPrivKey(t)
	ownerAddr := crypto.PubkeyToAddress(*ownerKey.Public().(*ecdsa.PublicKey))
	ownerAuth := func() *bind.TransactOpts {
		nonce, _ := client.PendingNonceAt(ctx, ownerAddr)
		gasPrice, _ := client.SuggestGasPrice(ctx)
		a, _ := bind.NewKeyedTransactorWithChainID(ownerKey, big.NewInt(chainID))
		a.Nonce = big.NewInt(int64(nonce))
		a.Value = big.NewInt(0)
		a.GasLimit = 16_000_000
		a.GasPrice = gasPrice
		return a
	}

	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	falseVerifier := deployFromArtifact(t, client, ownerAuth(),
		artifactBase+"/mocks/MockFalseVerifier.sol/MockFalseVerifier.json")
	trueMainVerifier := deployFromArtifact(t, client, ownerAuth(),
		artifactBase+"/mocks/MockTransferVerifier.sol/MockTransferVerifier.json")

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

	transfer := func(nullifierSeed int64) error {
		pubSig, deltas := buildTransferSignal(t, instance, enygmaAddr, fp, nullifierSeed)
		usdrDeltas, usdrProof := buildMockUsdrLeg(t, instance, enygmaAddr, pubSig, accountIds, banks[0].addr)
		tx, sendErr := instance.Transfer(bankAuth(t, client, banks[0]), deltas,
			enygma.IEnygmaProof{Proof: zeroProof, PublicSignal: pubSig},
			usdrDeltas, usdrProof, participantIds, "")
		if sendErr != nil {
			return sendErr
		}
		r, waitErr := bind.WaitMined(ctx, client, tx)
		if waitErr != nil {
			t.Fatalf("wait transfer: %v", waitErr)
		}
		if r.Status != 1 {
			return context.DeadlineExceeded
		}
		return nil
	}

	t.Run("MainVerifierReturnsFalse", func(t *testing.T) {
		if err := transfer(5001); err != nil {
			t.Fatalf("control transfer with a true-returning verifier failed: %v", err)
		}
		tx, err := instance.AddVerifier(ownerAuth(), falseVerifier)
		if err != nil {
			t.Fatalf("addVerifier(false): %v", err)
		}
		bind.WaitMined(ctx, client, tx)
		wantErr(t, "transfer with a main verifier that returns false", "InvalidProof", transfer(5002))
		// restore for the next sub-test
		tx, err = instance.AddVerifier(ownerAuth(), trueMainVerifier)
		if err != nil {
			t.Fatalf("addVerifier(true): %v", err)
		}
		bind.WaitMined(ctx, client, tx)
	})

	t.Run("UsdrVerifierReturnsFalse", func(t *testing.T) {
		if err := transfer(5003); err != nil {
			t.Fatalf("control transfer failed: %v", err)
		}
		tx, err := instance.AddUsdrVerifier(ownerAuth(), falseVerifier)
		if err != nil {
			t.Fatalf("addUsdrVerifier(false): %v", err)
		}
		bind.WaitMined(ctx, client, tx)
		wantErr(t, "transfer with a USDr verifier that returns false", "InvalidProof", transfer(5004))
	})
}
