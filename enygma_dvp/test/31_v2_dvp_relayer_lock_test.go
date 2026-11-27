package tests

// 31_v2_dvp_relayer_lock_test.go
//
// EnygmaDvp.lockReceiptNullifiers / unlockReceiptNullifiers, the relayer-only
// note lock behind SwapRelayer. A lock is taken only for a DvP leg that passes
// its vault's DvP checks (proof, Merkle root, unspent nullifier), and only the
// relayer that took it, presenting the same receipt, can release it.
// First written by Alex for #268, on the receipt reading #109 replaced.
//
// Deploys the real EnygmaDvp and Erc20CoinVault against AcceptAllVerifier, so
// only the contracts' own checks apply. Runs on go-ethereum's simulated
// backend; needs `npx hardhat compile` and the circomlibjs Poseidon artifacts
// (node scripts/regen_poseidon.js).

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"testing"
	"time"

	"enygma_dvp/layout"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestV2DvpRelayerLock_VerifiedAndOwned(t *testing.T) {
	otherKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	e := newLockEnv(t, otherKey)
	other, err := bind.NewKeyedTransactorWithChainID(otherKey, big.NewInt(1337))
	if err != nil {
		t.Fatal(err)
	}
	other.GasLimit = e.auth.GasLimit

	dvpABI, dvpBin := loadArtifact(t, "core/contracts/EnygmaDvp.sol/EnygmaDvp.json")
	dvp := e.deploy(dvpABI, dvpBin, e.wrapper, e.verifier)
	e.mustSend(dvpABI, dvp, "initializeDvp", e.verifier)

	tABI, tBin := loadArtifact(t, "erc20/contracts/RaylsERC20.sol/RaylsERC20.json")
	token := e.deploy(tABI, tBin, "Token", "TKN")
	vABI, vBin := loadArtifact(t, "core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json")
	vault := e.deploy(vABI, vBin, dvp)
	e.mustSend(dvpABI, dvp, "registerVault", vault, token, big.NewInt(1), big.NewInt(8))

	// Two authorized relayers: the deployer and another account.
	e.mustSend(dvpABI, dvp, "registerRelayer", e.auth.From)
	e.mustSend(dvpABI, dvp, "registerRelayer", other.From)

	amount := big.NewInt(100)
	e.mustSend(tABI, token, "mint", e.auth.From, amount)
	e.mustSend(tABI, token, "approve", vault, amount)
	e.mustSend(vABI, vault, "deposit", []*big.Int{amount, big.NewInt(123), big.NewInt(456), big.NewInt(0)})
	root := e.call(vABI, vault, "getRoot").(*big.Int)

	vaultID := big.NewInt(0)
	tree := big.NewInt(0)
	// A DvP initiator leg in the ERC-20 vault, in DvpInitiator's layout:
	// [message, tree, root, nullifier, commitB, commitA, revertCommitA, counterVault, deadline, groupId].
	leg := func(root *big.Int, nf, commitB int64) onchainProofReceipt {
		return legacyReceipt([]*big.Int{big.NewInt(0x5eed), tree, root, big.NewInt(nf),
			big.NewInt(commitB), big.NewInt(12), big.NewInt(13), new(big.Int).SetBytes(vault.Bytes()), big.NewInt(1_800_000_000), big.NewInt(0)}, 1, 1)
	}
	locked := func(nf int64) bool {
		return e.call(vABI, vault, "lockedNullifiers", tree, big.NewInt(nf)).(bool)
	}

	t.Run("a leg that fails its vault's DvP checks locks nothing", func(t *testing.T) {
		bad := leg(new(big.Int).Add(root, big.NewInt(1)), 701, 11)
		if got := e.send(dvpABI, dvp, "lockReceiptNullifiers", bad, vaultID); got != "InvalidMerkleRoot" {
			t.Fatalf("lock with an unknown Merkle root: got %q, want InvalidMerkleRoot", got)
		}
		if locked(701) {
			t.Fatal("a receipt with an unknown Merkle root locked its nullifier")
		}
	})

	t.Run("only the locking relayer, with the same receipt, can unlock", func(t *testing.T) {
		good := leg(root, 702, 11)
		e.mustSend(dvpABI, dvp, "lockReceiptNullifiers", good, vaultID)
		if !locked(702) {
			t.Fatal("a valid receipt was not locked")
		}

		if got := e.sendFrom(other, dvpABI, dvp, "unlockReceiptNullifiers", good, vaultID); got != "NotLockedByCaller" {
			t.Fatalf("unlock by another relayer: got %q, want NotLockedByCaller", got)
		}
		sameNote := leg(root, 702, 99)
		if got := e.send(dvpABI, dvp, "unlockReceiptNullifiers", sameNote, vaultID); got != "NotLockedByCaller" {
			t.Fatalf("unlock with another receipt for the same note: got %q, want NotLockedByCaller", got)
		}
		if !locked(702) {
			t.Fatal("the note was unlocked by a relayer or receipt that did not lock it")
		}

		e.mustSend(dvpABI, dvp, "unlockReceiptNullifiers", good, vaultID)
		if locked(702) {
			t.Fatal("the locking relayer could not unlock its own receipt")
		}
		// The lock is gone, so the same unlock cannot be repeated.
		if got := e.send(dvpABI, dvp, "unlockReceiptNullifiers", good, vaultID); got != "NotLockedByCaller" {
			t.Fatalf("second unlock: got %q, want NotLockedByCaller", got)
		}
	})

	t.Run("a lock in an unregistered vault is refused by name", func(t *testing.T) {
		if got := e.send(dvpABI, dvp, "lockReceiptNullifiers", leg(root, 704, 11), big.NewInt(5)); got != "InvalidVaultId" {
			t.Fatalf("lock in vault 5: got %q, want InvalidVaultId", got)
		}
	})

	t.Run("callers that are not relayers are refused", func(t *testing.T) {
		e.mustSend(dvpABI, dvp, "deregisterRelayer", other.From)
		good := leg(root, 703, 11)
		if got := e.sendFrom(other, dvpABI, dvp, "lockReceiptNullifiers", good, vaultID); got == "" {
			t.Fatal("a deregistered relayer could lock a note")
		}
	})
}

// relayerLockFixture is an EnygmaDvp with one funded ERC-20 vault (id 0) and
// the env's deployer registered as a relayer, for the lock-expiry tests.
type relayerLockFixture struct {
	e      *lockEnv
	dvpABI abi.ABI
	dvp    common.Address
	vABI   abi.ABI
	vault  common.Address
	root   *big.Int
}

func newRelayerLockFixture(t *testing.T, extra ...*ecdsa.PrivateKey) *relayerLockFixture {
	t.Helper()
	e := newLockEnv(t, extra...)
	f := &relayerLockFixture{e: e}
	var dvpBin []byte
	f.dvpABI, dvpBin = loadArtifact(t, "core/contracts/EnygmaDvp.sol/EnygmaDvp.json")
	f.dvp = e.deploy(f.dvpABI, dvpBin, e.wrapper, e.verifier)
	e.mustSend(f.dvpABI, f.dvp, "initializeDvp", e.verifier)

	tABI, tBin := loadArtifact(t, "erc20/contracts/RaylsERC20.sol/RaylsERC20.json")
	token := e.deploy(tABI, tBin, "Token", "TKN")
	var vBin []byte
	f.vABI, vBin = loadArtifact(t, "core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json")
	f.vault = e.deploy(f.vABI, vBin, f.dvp)
	e.mustSend(f.dvpABI, f.dvp, "registerVault", f.vault, token, big.NewInt(1), big.NewInt(8))
	e.mustSend(f.dvpABI, f.dvp, "registerRelayer", e.auth.From)

	amount := big.NewInt(100)
	e.mustSend(tABI, token, "mint", e.auth.From, amount)
	e.mustSend(tABI, token, "approve", f.vault, amount)
	e.mustSend(f.vABI, f.vault, "deposit", []*big.Int{amount, big.NewInt(123), big.NewInt(456), big.NewInt(0)})
	f.root = e.call(f.vABI, f.vault, "getRoot").(*big.Int)
	return f
}

// leg is a DvP initiator leg in vault 0, tree 0, spending nullifier nf.
func (f *relayerLockFixture) leg(nf int64) onchainProofReceipt {
	return legacyReceipt([]*big.Int{big.NewInt(0x5eed), big.NewInt(0), f.root, big.NewInt(nf),
		big.NewInt(11), big.NewInt(12), big.NewInt(13), new(big.Int).SetBytes(f.vault.Bytes()), big.NewInt(1_800_000_000), big.NewInt(0)}, 1, 1)
}

func (f *relayerLockFixture) locked(nf int64) bool {
	return f.e.call(f.vABI, f.vault, "lockedNullifiers", big.NewInt(0), big.NewInt(nf)).(bool)
}

func (f *relayerLockFixture) advance(d time.Duration) {
	f.e.t.Helper()
	if err := f.e.backend.AdjustTime(d); err != nil {
		f.e.t.Fatal(err)
	}
	f.e.backend.Commit()
}

// transactor signs with key, which must have been funded by the fixture.
func (f *relayerLockFixture) transactor(key *ecdsa.PrivateKey) *bind.TransactOpts {
	f.e.t.Helper()
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(1337))
	if err != nil {
		f.e.t.Fatal(err)
	}
	auth.GasLimit = f.e.auth.GasLimit
	return auth
}

// A relayer lock expires MAX_SWAP_DURATION (30 days) after it is taken; from
// then on anyone may release it.
func TestV2DvpRelayerLock_Expiry(t *testing.T) {
	strangerKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	goneKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	f := newRelayerLockFixture(t, strangerKey, goneKey)
	stranger, gone := f.transactor(strangerKey), f.transactor(goneKey)
	vaultID := big.NewInt(0)

	lock := f.leg(801)
	f.e.mustSend(f.dvpABI, f.dvp, "lockReceiptNullifiers", lock, vaultID)
	// A second relayer locks another note, then is deregistered: nothing it
	// could call would release that lock any more.
	orphan := f.leg(802)
	f.e.mustSend(f.dvpABI, f.dvp, "registerRelayer", gone.From)
	if got := f.e.sendFrom(gone, f.dvpABI, f.dvp, "lockReceiptNullifiers", orphan, vaultID); got != "" {
		t.Fatalf("second relayer's lock: %s", got)
	}
	f.e.mustSend(f.dvpABI, f.dvp, "deregisterRelayer", gone.From)

	f.advance(30*24*time.Hour - time.Minute)
	if got := f.e.sendFrom(stranger, f.dvpABI, f.dvp, "releaseExpiredRelayerLock", lock, vaultID); got != "RelayerLockNotExpired" {
		t.Fatalf("a stranger's release a minute before 30 days: got %q, want RelayerLockNotExpired", got)
	}
	if !f.locked(801) {
		t.Fatal("a stranger released a lock before it expired")
	}

	f.advance(time.Minute)
	// The release must name the receipt that was locked: the same note's
	// statement with its Merkle root zeroed would make the vault skip the
	// unlock, and dropping the lock entry then would leave the note locked
	// with nothing able to release it.
	tampered := f.leg(801)
	tampered.Statement[2] = big.NewInt(0)
	if got := f.e.sendFrom(stranger, f.dvpABI, f.dvp, "releaseExpiredRelayerLock", tampered, vaultID); got != "NotRelayerLocked" {
		t.Fatalf("a release naming another statement for the note: got %q, want NotRelayerLocked", got)
	}
	if got := f.e.sendFrom(stranger, f.dvpABI, f.dvp, "releaseExpiredRelayerLock", lock, vaultID); got != "" {
		t.Fatalf("a stranger's release after 30 days: got %q, want success", got)
	}
	if f.locked(801) {
		t.Fatal("the note is still locked after its expired lock was released")
	}
	// The lock is gone: neither a second release nor the relayer's own
	// unlock finds it.
	if got := f.e.sendFrom(stranger, f.dvpABI, f.dvp, "releaseExpiredRelayerLock", lock, vaultID); got != "NotRelayerLocked" {
		t.Fatalf("second release: got %q, want NotRelayerLocked", got)
	}
	if got := f.e.send(f.dvpABI, f.dvp, "unlockReceiptNullifiers", lock, vaultID); got != "NotLockedByCaller" {
		t.Fatalf("relayer's unlock after release: got %q, want NotLockedByCaller", got)
	}

	if got := f.e.sendFrom(stranger, f.dvpABI, f.dvp, "releaseExpiredRelayerLock", orphan, vaultID); got != "" {
		t.Fatalf("a stranger's release of a deregistered relayer's expired lock: got %q, want success", got)
	}
	if f.locked(802) {
		t.Fatal("a deregistered relayer's note is still locked after its expired lock was released")
	}
}

// A SwapRelayer leg whose lock was released by expiry can still be cancelled:
// the swap is closed and the note stays free.
func TestV2DvpRelayerLock_CancelSwapAfterExpiredRelease(t *testing.T) {
	strangerKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	f := newRelayerLockFixture(t, strangerKey)
	stranger := f.transactor(strangerKey)
	vaultID := big.NewInt(0)

	gABI, gBin := loadArtifact(t, "core/contracts/vaults/AssetGroup.sol/AssetGroup.json")
	group := f.e.deploy(gABI, gBin, f.dvp)
	f.e.mustSend(f.dvpABI, f.dvp, "registerAssetGroup", group, "Fungibles", true, big.NewInt(8))
	f.e.mustSend(f.dvpABI, f.dvp, "addVaultToGroup", vaultID, big.NewInt(0))

	rABI, rBin := loadArtifact(t, "core/contracts/SwapRelayer.sol/SwapRelayer.json")
	relayer := f.e.deploy(rABI, rBin, f.dvp)
	f.e.mustSend(f.dvpABI, f.dvp, "registerRelayer", relayer)

	leg := f.leg(901)
	swapID := f.e.call(rABI, relayer, "swapIdOf", leg, true).([32]byte)
	head, err := f.e.backend.Client().HeaderByNumber(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	expiry := new(big.Int).SetUint64(head.Time + 30*24*3600)
	leg.Statement[layout.DvpInitiatorDeadlineOffset] = expiry
	if got := f.e.send(rABI, relayer, "submitReceipt", swapID, leg, true, vaultID, expiry, []byte{1}, []byte{2}); got != "" {
		t.Fatalf("submitReceipt: %s", got)
	}

	f.advance(30*24*time.Hour + time.Minute)
	if got := f.e.sendFrom(stranger, f.dvpABI, f.dvp, "releaseExpiredRelayerLock", leg, vaultID); got != "" {
		t.Fatalf("release of the expired leg lock: %s", got)
	}
	if got := f.e.sendFrom(stranger, rABI, relayer, "cancelSwap", swapID); got != "" {
		t.Fatalf("cancelSwap after the leg's lock was released by expiry: got %q, want success", got)
	}
	if !f.e.call(rABI, relayer, "closed", swapID).(bool) {
		t.Fatal("the cancelled swap is not closed")
	}
	if f.locked(901) {
		t.Fatal("the note is locked after cancel")
	}
}

// Batch settlement and direct submission use the same proof-bound terms and
// vault-pair approval. This test exercises the batch entry point in isolation.
func TestV2DvpAtomicSettlementTerms(t *testing.T) {
	f := newRelayerLockFixture(t)
	gABI, gBin := loadArtifact(t, "core/contracts/vaults/AssetGroup.sol/AssetGroup.json")
	for _, group := range []struct {
		name     string
		fungible bool
	}{{"Fungibles", true}, {"NFTs", false}} {
		addr := f.e.deploy(gABI, gBin, f.dvp)
		f.e.mustSend(f.dvpABI, f.dvp, "registerAssetGroup", addr, group.name, group.fungible, big.NewInt(8))
	}
	zero, one := big.NewInt(0), big.NewInt(1)
	f.e.mustSend(f.dvpABI, f.dvp, "registerSwapGroupPair", zero, one)

	head, err := f.e.backend.Client().HeaderByNumber(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, want        string
		groupID, deadline uint64
	}{
		{"expired proof", "SwapDeadlineExpired", 0, head.Time - 1},
		{"different group", "SwapGroupMismatch", 1, head.Time + 3600},
		{"unregistered vault pair", "InvalidSwapGroupPair", 0, head.Time + 3600},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pay := f.leg(911)
			pay.Statement[layout.DvpInitiatorGroupIdOffset] = new(big.Int).SetUint64(tc.groupID)
			pay.Statement[layout.DvpInitiatorDeadlineOffset] = new(big.Int).SetUint64(tc.deadline)
			delivery := legacyReceipt([]*big.Int{
				pay.Statement[layout.DvpInitiatorCommitBOffset], zero, f.root, big.NewInt(912),
				pay.Statement[layout.DvpInitiatorMessageOffset], new(big.Int).SetBytes(f.vault.Bytes()),
			}, 1, 1)
			if got := f.e.send(f.dvpABI, f.dvp, "swapOnGroupPair", pay, delivery, zero, one, zero, one); got != tc.want {
				t.Fatalf("swapOnGroupPair reverted with %q, want %s", got, tc.want)
			}
		})
	}
}

func TestV2DvPPartialSettlementTerms(t *testing.T) {
	f := newRelayerLockFixture(t)
	abi, bin := loadArtifact(t, "core/contracts/vaults/AssetGroup.sol/AssetGroup.json")
	group := f.e.deploy(abi, bin, f.dvp)
	zero := big.NewInt(0)
	f.e.mustSend(f.dvpABI, f.dvp, "registerAssetGroup", group, "Fungibles", true, big.NewInt(8))
	f.e.mustSend(f.dvpABI, f.dvp, "addVaultToGroup", zero, zero)
	head, err := f.e.backend.Client().HeaderByNumber(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	leg := f.leg(913)
	deadline := new(big.Int).SetUint64(head.Time + 3600)
	leg.Statement[layout.DvpInitiatorDeadlineOffset] = deadline

	for _, tc := range []struct {
		name, want      string
		group, deadline *big.Int
	}{
		{"different deadline", "SwapDeadlineMismatch", zero, new(big.Int).Add(deadline, big.NewInt(60))},
		{"different group", "SwapGroupMismatch", big.NewInt(1), deadline},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := f.e.send(f.dvpABI, f.dvp, "submitPartialSettlement", leg, zero, tc.group, tc.deadline)
			if got != tc.want {
				t.Fatalf("submitPartialSettlement reverted with %q, want %s", got, tc.want)
			}
		})
	}
	f.e.mustSend(f.dvpABI, f.dvp, "submitPartialSettlement", leg, zero, zero, deadline)
	if !f.locked(913) {
		t.Fatal("the proven terms did not open a swap")
	}
}
