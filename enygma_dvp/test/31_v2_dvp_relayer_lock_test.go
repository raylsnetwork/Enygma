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
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
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
	// [message, tree, root, nullifier, commitB, commitA, revertCommitA, counterVault].
	leg := func(root *big.Int, nf, commitB int64) onchainProofReceipt {
		return legacyReceipt([]*big.Int{big.NewInt(0x5eed), tree, root, big.NewInt(nf),
			big.NewInt(commitB), big.NewInt(12), big.NewInt(13), new(big.Int).SetBytes(vault.Bytes())}, 1, 1)
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
