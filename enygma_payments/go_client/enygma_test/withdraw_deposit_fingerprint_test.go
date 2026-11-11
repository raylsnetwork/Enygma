package enygma_test

// withdraw() and deposit() shift every participant's balance commitment by
// a blinding factor derived from the sender's shared secret with that
// participant, exactly as transfer() does, but checked none of the
// shared-secret fingerprints transfer() checks (C-04). Any registered bank
// could submit a zero-value withdraw (no funds, no deposit params) built on
// shared secrets it made up, shifting each other participant's commitment
// by a blinding factor the victim cannot compute: the victim could never
// open its balance again, with no recovery path (registerAccount reverts
// AlreadyRegistered). Their circuits now publish the transfer circuit's k×k
// fingerprint matrix, and both functions require it to equal the
// fingerprints each pair confirmed on chain.
//
// Uses MockWithdrawVerifier / MockDepositVerifier (accept any proof) and
// MockZkDvp, so this is about Enygma.sol's own checks; the circuit side is
// covered by gnark-server/pkg/circuits/{withdraw,deposit}/fingerprint_binding_test.go.
//
//	CC=/usr/bin/clang go test -run TestWithdrawDepositFingerprints -v -timeout 600s

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"testing"

	enygma "enygma_payments/go_client/contracts"
	"enygma_payments/layout"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

// freshBankKey is bank i's own signing key in a freshSetup deployment, so
// each bank can confirm its own fingerprints (registerFingerprint acts for
// msg.sender's account). Deterministic, and only ever funded on a local node.
func freshBankKey(i int) *ecdsa.PrivateKey {
	k, err := crypto.ToECDSA(crypto.Keccak256([]byte(fmt.Sprintf("enygma-test-bank-%d", i))))
	if err != nil {
		panic(err)
	}
	return k
}

// freshBankAddr is the address bank i is registered to: its own key's,
// except the last bank, which keeps the owner's address so the owner (every
// test's msg.sender) stays a registered caller.
func freshBankAddr(i int, owner common.Address) common.Address {
	if i == nBanks-1 {
		return owner
	}
	return crypto.PubkeyToAddress(freshBankKey(i).PublicKey)
}

// testFingerprints is a symmetric, distinct fingerprint for every unordered
// pair of banks (0 on the diagonal).
func testFingerprints() [nBanks][nBanks]*big.Int {
	var fp [nBanks][nBanks]*big.Int
	for i := 0; i < nBanks; i++ {
		for j := 0; j < nBanks; j++ {
			lo, hi := i, j
			if lo > hi {
				lo, hi = hi, lo
			}
			fp[i][j] = big.NewInt(int64(10000*(lo+1) + (hi + 1)))
			if i == j {
				fp[i][j] = big.NewInt(0)
			}
		}
	}
	return fp
}

// confirmFreshFingerprints has every bank of a deployment registered with
// freshBankAddr confirm testFingerprints() with every other bank, funding the
// bank keys from the owner first, and returns the matrix.
func confirmFreshFingerprints(t *testing.T, client *ethclient.Client, mkAuth func() *bind.TransactOpts,
	waitTx func(*ethtypes.Transaction, error) *ethtypes.Receipt, instance *enygma.Enygma) [nBanks][nBanks]*big.Int {
	t.Helper()
	ctx := context.Background()
	auths := make([]func() *bind.TransactOpts, nBanks)
	for i := 0; i < nBanks; i++ {
		if i == nBanks-1 {
			auths[i] = mkAuth
			continue
		}
		key := freshBankKey(i)
		addr := crypto.PubkeyToAddress(key.PublicKey)
		if bal, err := client.BalanceAt(ctx, addr, nil); err != nil || bal.Cmp(big.NewInt(1e17)) < 0 {
			owner := mkAuth()
			tx := ethtypes.NewTx(&ethtypes.LegacyTx{Nonce: owner.Nonce.Uint64(), To: &addr,
				Value: big.NewInt(1e18), Gas: 21000, GasPrice: owner.GasPrice})
			signed, err := owner.Signer(owner.From, tx)
			if err != nil {
				t.Fatalf("sign funding tx: %v", err)
			}
			if err := client.SendTransaction(ctx, signed); err != nil {
				t.Fatalf("fund bank %d: %v", i, err)
			}
			waitTx(signed, nil)
		}
		auths[i] = func() *bind.TransactOpts {
			a, _ := bind.NewKeyedTransactorWithChainID(key, big.NewInt(chainID))
			nonce, _ := client.PendingNonceAt(ctx, addr)
			a.Nonce = big.NewInt(int64(nonce))
			a.GasLimit = 1_000_000
			a.GasPrice, _ = client.SuggestGasPrice(ctx)
			return a
		}
	}
	fp := testFingerprints()
	for i := 0; i < nBanks; i++ {
		for j := 0; j < nBanks; j++ {
			if i == j {
				continue
			}
			if r := waitTx(instance.RegisterFingerprint(auths[i](), big.NewInt(int64(j+1)), fp[i][j])); r.Status != 1 {
				t.Fatalf("registerFingerprint (%d,%d) failed", i, j)
			}
		}
	}
	return fp
}

// putFingerprints writes fp into a statement's row-major k×k fingerprint
// matrix at offset.
func putFingerprints(sig []*big.Int, offset int, fp [nBanks][nBanks]*big.Int) {
	for i := 0; i < nBanks; i++ {
		for j := 0; j < nBanks; j++ {
			sig[offset+i*nBanks+j] = new(big.Int).Set(fp[i][j])
		}
	}
}

// fabricatedColumn is fp with the sender's column (bank 0) replaced by the
// fingerprints of shared secrets the sender made up: what a freeze attempt
// has to publish for its circuit to accept it.
func fabricatedColumn(t *testing.T, fp [nBanks][nBanks]*big.Int) [nBanks][nBanks]*big.Int {
	t.Helper()
	out := fp
	for i := 1; i < nBanks; i++ {
		h, err := poseidon.Hash([]*big.Int{new(big.Int).SetUint64(0xdeadbeef00 + uint64(i))})
		if err != nil {
			t.Fatal(err)
		}
		out[i][0] = h.Mod(h, curveP)
	}
	return out
}

func TestWithdrawDepositFingerprints(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}

	type fixture struct {
		instance  *enygma.Enygma
		client    *ethclient.Client
		mkAuth    func() *bind.TransactOpts
		waitTx    func(*ethtypes.Transaction, error) *ethtypes.Receipt
		deltas    []enygma.IEnygmaPoint
		ids       []*big.Int
		withdraw  func(fp [nBanks][nBanks]*big.Int, seed string) enygma.IEnygmaWithdrawProof
		deposit   func(fp [nBanks][nBanks]*big.Int, seed string) enygma.IEnygmaDepositProof
		balancesN func() []enygma.IEnygmaPoint
	}
	setup := func(t *testing.T) fixture {
		client, mkAuth, waitTx := scenarioClient(t)
		instance, enygmaAddr := freshSetup(t, client, mkAuth, waitTx)
		const artifactBase = "../../contracts/enygma/artifacts/contracts"
		wv := deployFromArtifact(t, client, mkAuth(), artifactBase+"/mocks/MockWithdrawVerifier.sol/MockWithdrawVerifier.json")
		if r := waitTx(instance.AddWithdrawVerifier(mkAuth(), wv, big.NewInt(nBanks))); r.Status != 1 {
			t.Fatal("addWithdrawVerifier failed")
		}
		dv := deployFromArtifact(t, client, mkAuth(), artifactBase+"/mocks/MockDepositVerifier.sol/MockDepositVerifier.json")
		if r := waitTx(instance.AddDepositVerifier(mkAuth(), dv)); r.Status != 1 {
			t.Fatal("addDepositVerifier failed")
		}
		bridge := deployFromArtifact(t, client, mkAuth(), artifactBase+"/mocks/MockZkDvp.sol/MockZkDvp.json")
		if r := waitTx(instance.AddZkDvp(mkAuth(), bridge)); r.Status != 1 {
			t.Fatal("addZkDvp failed")
		}
		f := fixture{instance: instance, client: client, mkAuth: mkAuth, waitTx: waitTx}
		f.balancesN = func() []enygma.IEnygmaPoint {
			pv, err := instance.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
			if err != nil {
				t.Fatal(err)
			}
			return pv.Balances[1:]
		}
		// A zero-value operation: every delta is the identity, no deposit
		// params. That is all the freeze needed (with the real blinding
		// shifts in place of the identity; the mock verifier does not care).
		f.deltas = make([]enygma.IEnygmaPoint, nBanks)
		f.ids = make([]*big.Int, nBanks)
		for i := 0; i < nBanks; i++ {
			f.deltas[i] = enygma.IEnygmaPoint{C1: big.NewInt(0), C2: big.NewInt(1)}
			f.ids[i] = big.NewInt(int64(i + 1))
		}
		// shared fills the prefix withdraw shares with deposit.
		shared := func(sig []*big.Int, fp [nBanks][nBanks]*big.Int, seed string) {
			for i := range sig {
				sig[i] = big.NewInt(0)
			}
			blockHash, err := instance.GetBlckHash(&bind.CallOpts{})
			if err != nil {
				t.Fatal(err)
			}
			pv, err := instance.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
			if err != nil {
				t.Fatal(err)
			}
			putFingerprints(sig, layout.DepositFingerprintOffset, fp)
			for i := 0; i < nBanks; i++ {
				sig[layout.DepositPublicKeysOffset+i] = pv.Keys[i+1]
				sig[layout.DepositPreviousCommitmentsOffset+2*i] = pv.Balances[i+1].C1
				sig[layout.DepositPreviousCommitmentsOffset+2*i+1] = pv.Balances[i+1].C2
				sig[layout.DepositTxCommitmentsOffset+2*i] = big.NewInt(0)
				sig[layout.DepositTxCommitmentsOffset+2*i+1] = big.NewInt(1)
			}
			sig[layout.DepositBlockNumberOffset] = blockHash
			sig[layout.DepositNullifierOffset] = deterministicNullifier(t, seed)
			sig[layout.DepositDomainIdOffset] = expectedDomainId(enygmaAddr)
		}
		zero := [8]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)}
		f.withdraw = func(fp [nBanks][nBanks]*big.Int, seed string) enygma.IEnygmaWithdrawProof {
			var sig [layout.WithdrawLen]*big.Int
			shared(sig[:], fp, seed)
			sig[layout.WithdrawTotalDepositValueOffset] = big.NewInt(0)
			return enygma.IEnygmaWithdrawProof{Proof: zero, PublicSignal: sig}
		}
		f.deposit = func(fp [nBanks][nBanks]*big.Int, seed string) enygma.IEnygmaDepositProof {
			var sig [layout.DepositLen]*big.Int
			shared(sig[:], fp, seed)
			sig[layout.DepositHashOffset] = big.NewInt(424242)
			return enygma.IEnygmaDepositProof{Proof: zero, PublicSignal: sig}
		}
		return f
	}
	wantRevert := func(t *testing.T, err error, want string) {
		t.Helper()
		if err == nil {
			t.Fatalf("call succeeded, want %s", want)
		}
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("call reverted, but not with %s: %v", want, err)
		}
	}

	t.Run("no confirmed fingerprints: zero-value withdraw and deposit are rejected", func(t *testing.T) {
		f := setup(t)
		fp := testFingerprints()
		_, err := f.instance.Withdraw(f.mkAuth(), f.deltas, f.withdraw(fp, "fp-unconfirmed-w"), []enygma.IEnygmaDepositParams{}, f.ids)
		wantRevert(t, err, "FingerprintNotConfirmed")
		_, err = f.instance.Deposit(f.mkAuth(), f.deltas, f.deposit(fp, "fp-unconfirmed-d"), emptyWithdrawParams(), f.ids)
		wantRevert(t, err, "FingerprintNotConfirmed")
	})

	t.Run("made-up shared secrets are rejected; the confirmed ones pass", func(t *testing.T) {
		f := setup(t)
		fp := confirmFreshFingerprints(t, f.client, f.mkAuth, f.waitTx, f.instance)
		before := f.balancesN()

		fake := fabricatedColumn(t, fp)
		_, err := f.instance.Withdraw(f.mkAuth(), f.deltas, f.withdraw(fake, "fp-fake-w"), []enygma.IEnygmaDepositParams{}, f.ids)
		wantRevert(t, err, "InvalidPublicInputs")
		_, err = f.instance.Deposit(f.mkAuth(), f.deltas, f.deposit(fake, "fp-fake-d"), emptyWithdrawParams(), f.ids)
		wantRevert(t, err, "InvalidPublicInputs")
		after := f.balancesN()
		for i := range before {
			if before[i].C1.Cmp(after[i].C1) != 0 || before[i].C2.Cmp(after[i].C2) != 0 {
				t.Fatalf("bank %d's commitment changed after rejected calls", i)
			}
		}
		t.Log("withdraw and deposit built on made-up shared secrets rejected with InvalidPublicInputs; no balance moved")

		if r := f.waitTx(f.instance.Withdraw(f.mkAuth(), f.deltas, f.withdraw(fp, "fp-real-w"), []enygma.IEnygmaDepositParams{}, f.ids)); r.Status != 1 {
			t.Fatal("withdraw with the confirmed fingerprints reverted")
		}
		if r := f.waitTx(f.instance.Deposit(f.mkAuth(), f.deltas, f.deposit(fp, "fp-real-d"), emptyWithdrawParams(), f.ids)); r.Status != 1 {
			t.Fatal("deposit with the confirmed fingerprints reverted")
		}
		ok, err := f.instance.Check(&bind.CallOpts{})
		if err != nil || !ok {
			t.Fatalf("check() after the honest calls: ok=%v err=%v", ok, err)
		}
		t.Log("withdraw and deposit with the confirmed fingerprints settled; check() holds")
	})

	t.Run("deposit needs exactly DEFAULT_SIZE participants", func(t *testing.T) {
		f := setup(t)
		fp := testFingerprints()
		_, err := f.instance.Deposit(f.mkAuth(), f.deltas, f.deposit(fp, "fp-len-d"), emptyWithdrawParams(), f.ids[:nBanks-1])
		wantRevert(t, err, "ParticipantIdsLengthMismatch")
	})
}
