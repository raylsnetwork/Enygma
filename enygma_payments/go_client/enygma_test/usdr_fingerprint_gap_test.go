package enygma_test

// Regression test for the USDr leg of Fix C-04.
//
// The USDr proof carries its own 6x6 FingerPrintofSharedSecrets matrix
// (public_signal[0..35]) and its own blinding factors, derived from
// SharedSecrets[i] — a private witness the SENDER chooses, exactly like the
// main proof's. Fix C-04 made transfer() require every pairwise fingerprint
// of the MAIN proof to match a mutually confirmed on-chain value
// (_verifyFingerprints), so a sender cannot fabricate a secret with a victim
// and poison the victim's main balance.
//
// _verifyFingerprints only reads the main proof's 81-signal array, so
// _verifyUsdrMainBinding must tie the USDr matrix to it: without that, a
// sender could publish a fabricated matrix in the USDr proof, shift every
// non-sender participant's USDr commitment by a blinding factor the victim
// cannot recompute, and freeze their USDr balance (which every sender needs
// to pay the transfer fee). _verifyUsdrMainBinding therefore requires every
// off-diagonal cell of the USDr matrix to equal the main proof's, which
// _verifyFingerprints has already checked against the confirmed registry.
//
// Uses the mock verifiers (as c04_repro_test.go does) since the property
// under test is purely which public-signal cells the contract inspects.
//
// Sub-tests:
//   HonestBothMatricesConfirmed   control: main and USDr matrices both equal
//                                 the confirmed fingerprints -> must succeed.
//   UsdrMatrixNotConfirmed        main matrix honest, USDr matrix fabricated
//                                 -> must be rejected (UsdrBindingMismatch).
//
// Run:
//   ENYGMA_CHAIN_URL=http://127.0.0.1:8545 ENYGMA_CHAIN_ID=1337 \
//   MY_KEY=<hardhat key> go test -run TestC04_UsdrLegFingerprints -v .

import (
	"context"
	"math/big"
	"strings"
	"testing"

	enygma "enygma_payments/go_client/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
)

// confirmAllFingerprints registers a symmetric, distinct fingerprint for every
// unordered pair from both sides, and returns the matrix.
func confirmAllFingerprints(t *testing.T, client *ethclient.Client, instance *enygma.Enygma, banks []c04Bank) [nBanks][nBanks]*big.Int {
	t.Helper()
	var fp [nBanks][nBanks]*big.Int
	for i := 0; i < nBanks; i++ {
		for j := 0; j < nBanks; j++ {
			if i == j {
				continue
			}
			lo, hi := i, j
			if lo > hi {
				lo, hi = hi, lo
			}
			fp[i][j] = big.NewInt(int64(10000*(lo+1) + (hi + 1)))
		}
	}
	for i := 0; i < nBanks; i++ {
		for j := 0; j < nBanks; j++ {
			if i == j {
				continue
			}
			tx, err := instance.RegisterFingerprint(bankAuth(t, client, banks[i]), big.NewInt(banks[j].accountID), fp[i][j])
			if err != nil {
				t.Fatalf("register fingerprint (%d,%d): %v", i, j, err)
			}
			if _, err := bind.WaitMined(context.Background(), client, tx); err != nil {
				t.Fatalf("wait register fingerprint (%d,%d): %v", i, j, err)
			}
		}
	}
	return fp
}

func TestC04_UsdrLegFingerprints(t *testing.T) {
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

	transfer := func(nullifierSeed int64, usdrMatrix *[nBanks][nBanks]*big.Int) error {
		pubSig, deltas := buildTransferSignal(t, instance, enygmaAddr, fp, nullifierSeed)
		usdrDeltas, usdrProof := buildMockUsdrLeg(t, instance, enygmaAddr, pubSig, accountIds, banks[0].addr)
		for i := 0; i < nBanks; i++ {
			for j := 0; j < nBanks; j++ {
				if i == j {
					continue
				}
				usdrProof.PublicSignal[i*nBanks+j] = usdrMatrix[i][j]
			}
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

	t.Run("HonestBothMatricesConfirmed", func(t *testing.T) {
		if err := transfer(333, &fp); err != nil {
			t.Fatalf("control failed — honest transfer with both matrices confirmed was rejected: %v", err)
		}
		t.Log("control: transfer with both matrices equal to the confirmed fingerprints succeeds")
	})

	t.Run("UsdrMatrixNotConfirmed", func(t *testing.T) {
		// The sender fabricates the USDr matrix (values no recipient ever
		// agreed to), while leaving the main matrix honest.
		var fabricated [nBanks][nBanks]*big.Int
		for i := 0; i < nBanks; i++ {
			for j := 0; j < nBanks; j++ {
				fabricated[i][j] = big.NewInt(int64(7_000_000 + 1000*i + j))
			}
		}
		err := transfer(444, &fabricated)
		if err == nil {
			t.Errorf("VULNERABLE: transfer() accepted a USDr proof whose FingerPrintofSharedSecrets matrix matches no confirmed fingerprint — a sender can poison every participant's USDr blinding factor")
			return
		}
		if !strings.Contains(err.Error(), "UsdrBindingMismatch") {
			t.Errorf("rejected, but not with UsdrBindingMismatch: %v", err)
			return
		}
		t.Logf("SAFE: fabricated USDr matrix rejected: %v", err)
	})
}
