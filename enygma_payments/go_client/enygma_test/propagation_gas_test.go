package enygma_test

// #393: every ledger write copies each registered account's commitments
// forward on both ledgers, so its gas grows with the registry, and
// registerAccount stops at MAX_REGISTERED_ACCOUNTS. These tests fill the
// registry to that cap and settle a transfer and a withdraw on an epoch's
// first block, where every copy is a fresh storage write, with mock
// verifiers and a mock bridge (as c04_repro_test.go and
// withdraw_recipient_binding_test.go do). Each adds a measured allowance for
// what the real verifiers and vault cost on top, and must fit EIP-7825's
// per-transaction gas cap.
//
// Needs a chain at ENYGMA_CHAIN_URL (local Hardhat: `npx hardhat node` in
// contracts/enygma) and the Hardhat artifacts.

import (
	"context"
	"math/big"
	"testing"

	enygma "enygma_payments/go_client/contracts"
	"enygma_payments/layout"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

const (
	// txGasCap is EIP-7825's per-transaction gas cap, 2^24.
	txGasCap = 1 << 24

	// What the real contracts cost beyond the mocks, measured on Hardhat
	// (2026-11-26). A Groth16 verifier's verifyProof runs 710k gas at 81
	// signals and about 6.5k more per signal (83: 723k, 85: 736k), so the
	// withdraw verifier's 92 cost about 782k. A real EnygmaErc20CoinVault
	// depositThroughEnygma (Poseidon and a depth-8 Merkle insert) costs up to
	// 618k for a tree's first note and 458k after: 613,283 and 453,842 as
	// measured then, plus 4,510 each for the LeafInserted event the insert
	// now logs (measured 2026-11-27). Neither subtracts what the mock it
	// replaces costs.
	//
	// realTransferGas: the transfer (81) and USDr (83) verifiers.
	realTransferGas = 710_267 + 723_297
	// realWithdrawGas: the withdraw verifier (92) and withdrawMaxDeposits
	// vault deposits.
	realWithdrawGas = 782_000 + 617_793 + (withdrawMaxDeposits-1)*458_352
	// withdrawMaxDeposits is StatementLayout.WITHDRAW_DEPOSIT_COMMITMENTS_SIZE,
	// the most deposits a withdraw takes.
	withdrawMaxDeposits = 10
)

// fullRegistry is a fresh Enygma filled to MAX_REGISTERED_ACCOUNTS: c04Setup's
// six funded banks, with every pair's fingerprint confirmed, and accounts
// 7..capacity registered by the owner.
type fullRegistry struct {
	t            *testing.T
	client       *ethclient.Client
	rpc          *rpc.Client
	mkAuth       func() *bind.TransactOpts
	instance     *enygma.Enygma
	addr         common.Address
	banks        []c04Bank
	fingerprints [nBanks][nBanks]*big.Int
	capacity     int
	epoch        uint64
}

func newFullRegistry(t *testing.T) *fullRegistry {
	t.Helper()
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	client, mkAuth, _ := scenarioClient(t)
	rpcClient, err := rpc.Dial(chainURL)
	if err != nil {
		t.Fatalf("rpc dial: %v", err)
	}
	t.Cleanup(rpcClient.Close)

	instance, banks, addr := c04Setup(t, client)
	r := &fullRegistry{t: t, client: client, rpc: rpcClient, mkAuth: mkAuth, instance: instance, addr: addr, banks: banks}
	r.epoch = readEpochInterval(t, context.Background(), client, addr).Uint64()

	sel := crypto.Keccak256([]byte("MAX_REGISTERED_ACCOUNTS()"))[:4] // the Go bindings predate it
	out, err := client.CallContract(context.Background(), ethereum.CallMsg{To: &addr, Data: sel}, nil)
	if err != nil || len(out) == 0 {
		t.Fatalf("MAX_REGISTERED_ACCOUNTS(): %v (%d bytes)", err, len(out))
	}
	r.capacity = int(new(big.Int).SetBytes(out).Int64())

	for i := 0; i < nBanks; i++ {
		for j := 0; j < nBanks; j++ {
			lo, hi := i, j
			if lo > hi {
				lo, hi = hi, lo
			}
			r.fingerprints[i][j] = big.NewInt(int64(10000*(lo+1) + (hi + 1))) // symmetric per pair; the diagonal is unread
		}
	}
	for i := 0; i < nBanks; i++ {
		for j := i + 1; j < nBanks; j++ {
			r.wait(instance.RegisterFingerprint(bankAuth(t, client, banks[i]), big.NewInt(banks[j].accountID), r.fingerprints[i][j]))
			r.wait(instance.RegisterFingerprint(bankAuth(t, client, banks[j]), big.NewInt(banks[i].accountID), r.fingerprints[j][i]))
		}
	}
	for id := nBanks + 1; id <= r.capacity; id++ {
		r.wait(r.register(id))
	}
	return r
}

// wait fails the test unless tx mined and succeeded.
func (r *fullRegistry) wait(tx *ethtypes.Transaction, err error) *ethtypes.Receipt {
	r.t.Helper()
	if err != nil {
		r.t.Fatalf("send tx: %v", err)
	}
	rcpt, err := bind.WaitMined(context.Background(), r.client, tx)
	if err != nil {
		r.t.Fatalf("wait mined: %v", err)
	}
	if rcpt.Status != ethtypes.ReceiptStatusSuccessful {
		r.t.Fatalf("tx reverted: %s", tx.Hash())
	}
	return rcpt
}

// register sends the owner's registration of account id under a fresh
// address.
func (r *fullRegistry) register(id int) (*ethtypes.Transaction, error) {
	key, err := crypto.GenerateKey()
	if err != nil {
		r.t.Fatal(err)
	}
	pk, err := poseidon.Hash([]*big.Int{big.NewInt(int64(5000 + id)), big.NewInt(int64(5000 + id))})
	if err != nil {
		r.t.Fatal(err)
	}
	pk.Mod(pk, curveP)
	cx, cy := regCommit(big.NewInt(senderPrevR))
	return r.instance.RegisterAccount(r.mkAuth(), crypto.PubkeyToAddress(key.PublicKey), big.NewInt(int64(id)), pk, cx, cy, []byte{})
}

// toEpochBoundary mines until the next transaction lands on an epoch's
// first block.
func (r *fullRegistry) toEpochBoundary() {
	r.t.Helper()
	next := currentBlock(r.t, context.Background(), r.client).Uint64() + 1
	if rem := next % r.epoch; rem != 0 {
		mineBlocks(r.t, r.rpc, r.epoch-rem)
	}
}

// fitsAtEpochBoundary fails unless rcpt landed on an epoch's first block and
// its gas (with mocks) plus real, the real contracts' extra cost, fits
// txGasCap.
func (r *fullRegistry) fitsAtEpochBoundary(what string, rcpt *ethtypes.Receipt, real uint64) {
	r.t.Helper()
	if bn := rcpt.BlockNumber.Uint64(); bn%r.epoch != 0 {
		r.t.Fatalf("%s landed in block %d, not on an epoch boundary (interval %d)", what, bn, r.epoch)
	}
	if total := rcpt.GasUsed + real; total > txGasCap {
		r.t.Fatalf("%s at an epoch boundary with %d accounts: %d gas with mocks, %d with the real verifiers and vault, over the %d cap", what, r.capacity, rcpt.GasUsed, total, txGasCap)
	}
	r.t.Logf("%s at an epoch boundary with %d accounts: %d gas with mocks, about %d real", what, r.capacity, rcpt.GasUsed, rcpt.GasUsed+real)
}

func (r *fullRegistry) bankIds() []*big.Int {
	ids := make([]*big.Int, nBanks)
	for i := range r.banks {
		ids[i] = big.NewInt(r.banks[i].accountID)
	}
	return ids
}

// transfer settles a zero-delta transfer among the six banks on the next
// epoch's first block, with at most txGasCap gas.
func (r *fullRegistry) transfer() *ethtypes.Receipt {
	r.t.Helper()
	r.toEpochBoundary()
	pubSig, deltas := buildTransferSignal(r.t, r.instance, r.addr, r.fingerprints, 393)
	accountIds := make([]int64, nBanks)
	for i := range r.banks {
		accountIds[i] = r.banks[i].accountID
	}
	proof := enygma.IEnygmaProof{Proof: zeroProof(), PublicSignal: pubSig}
	usdrDeltas, usdrProof := buildMockUsdrLeg(r.t, r.instance, r.addr, pubSig, accountIds, r.banks[0].addr)
	auth := bankAuth(r.t, r.client, r.banks[0])
	auth.GasLimit = txGasCap
	return r.wait(r.instance.Transfer(auth, deltas, proof, usdrDeltas, usdrProof, r.bankIds(), ""))
}

// withdraw settles a zero-delta withdraw by bank 0 into withdrawMaxDeposits
// notes of a mock bridge on the next epoch's first block, with at most
// txGasCap gas.
func (r *fullRegistry) withdraw() *ethtypes.Receipt {
	r.t.Helper()
	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	verifier := deployFromArtifact(r.t, r.client, r.mkAuth(), artifactBase+"/mocks/MockWithdrawVerifier.sol/MockWithdrawVerifier.json")
	approveTestVerifier(r.t, r.client, r.instance, r.mkAuth(), testKindWithdraw, nBanks, verifier)
	r.wait(r.instance.AddWithdrawVerifier(r.mkAuth(), verifier, big.NewInt(nBanks)))
	bridge := deployFromArtifact(r.t, r.client, r.mkAuth(), artifactBase+"/mocks/MockZkDvp.sol/MockZkDvp.json")
	r.wait(r.instance.AddZkDvp(r.mkAuth(), bridge))

	r.toEpochBoundary()
	blockHash, err := r.instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		r.t.Fatal(err)
	}
	pubVals, err := r.instance.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
	if err != nil {
		r.t.Fatal(err)
	}
	var sig [layout.WithdrawLen]*big.Int
	for i := range sig {
		sig[i] = big.NewInt(0)
	}
	putFingerprints(sig[:], layout.WithdrawFingerprintOffset, r.fingerprints)
	deltas := make([]enygma.IEnygmaPoint, nBanks)
	for i := 0; i < nBanks; i++ {
		sig[layout.WithdrawPublicKeysOffset+i] = pubVals.Keys[i+1]
		sig[layout.WithdrawPreviousCommitmentsOffset+2*i] = pubVals.Balances[i+1].C1
		sig[layout.WithdrawPreviousCommitmentsOffset+2*i+1] = pubVals.Balances[i+1].C2
		sig[layout.WithdrawTxCommitmentsOffset+2*i+1] = big.NewInt(1) // TxCommit = neutral element (0,1)
		deltas[i] = enygma.IEnygmaPoint{C1: big.NewInt(0), C2: big.NewInt(1)}
	}
	params := make([]enygma.IEnygmaDepositParams, withdrawMaxDeposits)
	for i := range params {
		amount, key := big.NewInt(10), big.NewInt(int64(1000+i))
		params[i] = enygma.IEnygmaDepositParams{Amount: amount, PublicKey: key}
		sig[layout.WithdrawDepositCommitmentsOffset+i] = mockDvpCommitment(amount, key)
	}
	sig[layout.WithdrawTotalDepositValueOffset] = big.NewInt(10 * withdrawMaxDeposits)
	sig[layout.WithdrawBlockNumberOffset] = blockHash
	sig[layout.WithdrawNullifierOffset] = deterministicNullifier(r.t, "propagation-gas-withdraw")
	sig[layout.WithdrawDomainIdOffset] = expectedDomainId(r.addr)

	auth := bankAuth(r.t, r.client, r.banks[0])
	auth.GasLimit = txGasCap
	proof := enygma.IEnygmaWithdrawProof{Proof: zeroProof(), PublicSignal: sig}
	return r.wait(r.instance.Withdraw(auth, deltas, proof, params, r.bankIds()))
}

// TestRegisterAccount_RefusesAccountPastCapacity: the registry takes accounts
// up to MAX_REGISTERED_ACCOUNTS and refuses the next one.
func TestRegisterAccount_RefusesAccountPastCapacity(t *testing.T) {
	r := newFullRegistry(t)
	total, err := r.instance.TotalRegisteredBanks(&bind.CallOpts{})
	if err != nil {
		t.Fatalf("TotalRegisteredBanks: %v", err)
	}
	if int(total.Int64()) != r.capacity {
		t.Fatalf("registered %d accounts, want %d", total.Int64(), r.capacity)
	}
	_, err = r.register(r.capacity + 1)
	wantErr(t, "registering one account past capacity", "RegistryFull", err)
}

// TestFullRegistry_SettlesAtEpochBoundary: with every account slot taken, a
// transfer, and a withdraw into the most deposits a withdraw takes, each fit
// the per-transaction gas cap on an epoch's first block, where propagation
// writes every account's commitments into fresh storage on both ledgers.
// The first boundary after the accounts register costs most (their USDr
// slots are initialised then too), so each runs on a registry of its own.
func TestFullRegistry_SettlesAtEpochBoundary(t *testing.T) {
	t.Run("transfer", func(t *testing.T) {
		r := newFullRegistry(t)
		r.fitsAtEpochBoundary("transfer", r.transfer(), realTransferGas)
	})
	t.Run("withdraw", func(t *testing.T) {
		r := newFullRegistry(t)
		r.fitsAtEpochBoundary("withdraw", r.withdraw(), realWithdrawGas)
	})
}
