package enygma_test

// withdraw()'s proof now carries the DvP commitment of every deposit it
// creates (signals 52-61), and the contract requires the notes the bridge
// actually inserts to be exactly these. Before, only the total amount was
// bound, so whoever submitted the proof chose the recipients' public keys:
// another registered bank could replay a pending withdrawal with its own key
// and the same total, and receive the DvP note while the victim was debited.
//
// Uses MockWithdrawVerifier (accepts any proof) and MockZkDvp (reports a
// deterministic commitment per (amount, publicKey)), so this is about
// Enygma.sol's own checks; the circuit side is covered by
// gnark-server/pkg/circuits/withdraw/deposit_binding_test.go.
//
//	CC=/usr/bin/clang go test -run TestWithdrawRecipientBinding -v -timeout 300s

import (
	"math/big"
	"strings"
	"testing"

	enygma "enygma_payments/go_client/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

func TestWithdrawRecipientBinding(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}

	const (
		withdrawAmount = 300
		victimKey      = 1111
		attackerKey    = 6666
	)

	// fixture deploys a fresh Enygma with the mock withdraw verifier and
	// bridge, mints to bank 0, and returns a withdraw proof (bank 0 debits
	// withdrawAmount) whose deposit commitments are commitments.
	type fixture struct {
		instance  *enygma.Enygma
		mkAuth    func() *bind.TransactOpts
		waitTx    func(*ethtypes.Transaction, error) *ethtypes.Receipt
		deltas    []enygma.IEnygmaPoint
		ids       []*big.Int
		proofWith func(commitments ...*big.Int) enygma.IEnygmaWithdrawProof
	}
	setup := func(t *testing.T) fixture {
		client, mkAuth, waitTx := scenarioClient(t)
		instance, enygmaAddr := freshSetup(t, client, mkAuth, waitTx)
		const artifactBase = "../../contracts/enygma/artifacts/contracts"
		verifier := deployFromArtifact(t, client, mkAuth(), artifactBase+"/mocks/MockWithdrawVerifier.sol/MockWithdrawVerifier.json")
		if r := waitTx(instance.AddWithdrawVerifier(mkAuth(), verifier, big.NewInt(nBanks))); r.Status != 1 {
			t.Fatal("addWithdrawVerifier failed")
		}
		bridge := deployFromArtifact(t, client, mkAuth(), artifactBase+"/mocks/MockZkDvp.sol/MockZkDvp.json")
		if r := waitTx(instance.AddZkDvp(mkAuth(), bridge)); r.Status != 1 {
			t.Fatal("addZkDvp failed")
		}
		mcx, mcy := mintCommitPt(big.NewInt(mintAmt), big.NewInt(senderMintR))
		if r := waitTx(instance.MintSupply(mkAuth(), big.NewInt(mintAmt), big.NewInt(1), mcx, mcy)); r.Status != 1 {
			t.Fatal("mintSupply failed")
		}
		blockHash, err := instance.GetBlckHash(&bind.CallOpts{})
		if err != nil {
			t.Fatal(err)
		}
		pubVals, err := instance.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
		if err != nil {
			t.Fatal(err)
		}
		keys6, balances6 := pubVals.Keys[1:], pubVals.Balances[1:]
		debit := pedersenCommitment(new(big.Int).Sub(curveP, big.NewInt(withdrawAmount)), big.NewInt(0))

		var base [62]*big.Int
		for i := range base {
			base[i] = big.NewInt(0)
		}
		deltas := make([]enygma.IEnygmaPoint, nBanks)
		ids := make([]*big.Int, nBanks)
		for i := 0; i < nBanks; i++ {
			base[6+i] = keys6[i]
			base[12+2*i] = balances6[i].C1
			base[12+2*i+1] = balances6[i].C2
			if i == 0 {
				base[24] = debit.X
				base[25] = debit.Y
				deltas[i] = enygma.IEnygmaPoint{C1: debit.X, C2: debit.Y}
			} else {
				base[24+2*i] = big.NewInt(0)
				base[24+2*i+1] = big.NewInt(1)
				deltas[i] = enygma.IEnygmaPoint{C1: big.NewInt(0), C2: big.NewInt(1)}
			}
			ids[i] = big.NewInt(int64(i + 1))
		}
		base[36] = blockHash
		base[49] = deterministicNullifier(t, "withdraw-recipient-binding")
		base[50] = big.NewInt(withdrawAmount)
		base[51] = expectedDomainId(enygmaAddr)

		return fixture{
			instance: instance, mkAuth: mkAuth, waitTx: waitTx, deltas: deltas, ids: ids,
			proofWith: func(commitments ...*big.Int) enygma.IEnygmaWithdrawProof {
				sig := base
				for i, c := range commitments {
					sig[52+i] = c
				}
				return enygma.IEnygmaWithdrawProof{Proof: [8]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)}, PublicSignal: sig}
			},
		}
	}
	deposit := func(amount, key int64) enygma.IEnygmaDepositParams {
		return enygma.IEnygmaDepositParams{Amount: big.NewInt(amount), Erc20Adress: common.Address{}, PublicKey: big.NewInt(key)}
	}
	expectRevert := func(t *testing.T, f fixture, proof enygma.IEnygmaWithdrawProof, params []enygma.IEnygmaDepositParams, want string) {
		t.Helper()
		_, err := f.instance.Withdraw(f.mkAuth(), f.deltas, proof, params, f.ids)
		if err == nil {
			t.Fatalf("withdraw succeeded, want %s", want)
		}
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("withdraw reverted, but not with %s: %v", want, err)
		}
	}

	t.Run("replay with the attacker's key is rejected; the victim's own submission then succeeds", func(t *testing.T) {
		f := setup(t)
		proof := f.proofWith(mockDvpCommitment(big.NewInt(withdrawAmount), big.NewInt(victimKey)))

		// The attacker copies the pending proof, keeps the total, swaps the key.
		expectRevert(t, f, proof, []enygma.IEnygmaDepositParams{deposit(withdrawAmount, attackerKey)}, "DepositCommitmentMismatch")
		t.Log("replay with the attacker's key rejected with DepositCommitmentMismatch")

		// Splitting the same total differently is rejected too.
		expectRevert(t, f, proof, []enygma.IEnygmaDepositParams{deposit(100, victimKey), deposit(200, victimKey)}, "DepositCommitmentMismatch")

		// The proof's own recipients still go through (its nullifier was not consumed).
		if r := f.waitTx(f.instance.Withdraw(f.mkAuth(), f.deltas, proof, []enygma.IEnygmaDepositParams{deposit(withdrawAmount, victimKey)}, f.ids)); r.Status != 1 {
			t.Fatal("the honest withdrawal reverted")
		}
		ok, err := f.instance.Check(&bind.CallOpts{})
		if err != nil || !ok {
			t.Fatalf("check() after the honest withdrawal: ok=%v err=%v", ok, err)
		}
		t.Log("honest withdrawal to the proof's own recipient settled; check() holds")
	})

	t.Run("an unused deposit slot must carry no commitment", func(t *testing.T) {
		f := setup(t)
		proof := f.proofWith(mockDvpCommitment(big.NewInt(withdrawAmount), big.NewInt(victimKey)), big.NewInt(42))
		expectRevert(t, f, proof, []enygma.IEnygmaDepositParams{deposit(withdrawAmount, victimKey)}, "DepositCommitmentMismatch")
	})

	t.Run("more than 10 deposits are rejected", func(t *testing.T) {
		f := setup(t)
		params := make([]enygma.IEnygmaDepositParams, 11)
		for i := range params {
			params[i] = deposit(0, victimKey)
		}
		params[0] = deposit(withdrawAmount, victimKey)
		expectRevert(t, f, f.proofWith(), params, "TooManyDeposits")
	})
}
