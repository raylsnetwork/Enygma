package tests

// 34_v2_erc1155_vault_retired_test.go
//
// Erc1155CoinVault's receipts all came from legacy ERC1155 circuits (VK slots
// 2, 3, 4, 5, 15, 16) that have no source, prover route or keygen in the
// repo, and its checks on them were weaker than the other vaults' (#417). The
// vault is retired (docs/adr/0003): every entry point that takes tokens or a
// receipt reverts Erc1155VaultRetired, and it is no ERC1155 receiver, so no
// token can enter the vault and no legacy receipt can spend a note.
//
// The vault is deployed against AcceptAllVerifier, so a receipt reaching a
// verifier would pass: a refusal is the vault's own. Runs on go-ethereum's
// simulated backend; needs `npx hardhat compile` and the circomlibjs Poseidon
// artifacts (node scripts/regen_poseidon.js).

import (
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const erc1155VaultRetired = "Erc1155VaultRetired"

type retiredVaultEnv struct {
	*lockEnv
	tABI, vABI   abi.ABI
	token, vault common.Address
}

// newRetiredVaultEnv deploys a RaylsERC1155 and an Erc1155CoinVault for it,
// and mints the deployer 100 of token 7, approved for the vault.
// Any extra accounts given are funded.
func newRetiredVaultEnv(t *testing.T, extra ...*ecdsa.PrivateKey) *retiredVaultEnv {
	e := newLockEnv(t, extra...)
	tABI, tBin := loadArtifact(t, "erc1155/contracts/RaylsERC1155.sol/RaylsERC1155.json")
	token := e.deploy(tABI, tBin, "uri")
	vABI, vault := e.newVault("core/contracts/vaults/Erc1155CoinVault.sol/Erc1155CoinVault.json", token)
	e.mustSend(tABI, token, "mint", e.auth.From, big.NewInt(7), big.NewInt(100), []byte{})
	e.mustSend(tABI, token, "setApprovalForAll", vault, true)
	return &retiredVaultEnv{lockEnv: e, tABI: tABI, vABI: vABI, token: token, vault: vault}
}

func TestV2Erc1155Vault_RefusesDeposit(t *testing.T) {
	e := newRetiredVaultEnv(t)
	n := big.NewInt
	cases := map[string][]*big.Int{
		"single": {n(100), n(7), n(123), n(456)},
		"batch":  {n(7), n(7), n(40), n(60), n(123), n(124), n(456), n(457)},
	}
	for name, params := range cases {
		if r := e.send(e.vABI, e.vault, "deposit", params); r != erc1155VaultRetired {
			t.Errorf("%s deposit: got %q, want %s", name, r, erc1155VaultRetired)
		}
	}
	// Nor does a token sent straight to it: it is no ERC1155 receiver.
	if r := e.send(e.tABI, e.token, "safeTransferFrom", e.auth.From, e.vault, n(7), n(10), []byte{}); r == "" {
		t.Error("safeTransferFrom into the vault succeeded")
	}
	if got := e.call(e.tABI, e.token, "balanceOf", e.vault, n(7)).(*big.Int); got.Sign() != 0 {
		t.Fatalf("vault holds %s of token 7 after refused deposits", got)
	}
}

// ownershipReceipt is a 1-in/1-out legacy ownership receipt (3 + 3*1 + 1 = 7
// signals) spending nullifier nf against the vault's current root, with
// StMessage message: [message, tree 0, root, nf, output0, ...].
func (e *retiredVaultEnv) ownershipReceipt(message, nf, output0 *big.Int) onchainProofReceipt {
	z := big.NewInt(0)
	root := e.call(e.vABI, e.vault, "getRoot").(*big.Int)
	return legacyReceipt([]*big.Int{message, z, root, nf, output0, z, z}, 1, 1)
}

// A zero-param withdraw checked no opening and paid nothing, yet spent every
// non-zero nullifier: anyone holding someone else's unsettled receipt could
// burn their notes with it (R-009).
func TestV2Erc1155Vault_RefusesWithdraw(t *testing.T) {
	stranger, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	e := newRetiredVaultEnv(t, stranger)
	strangerAuth, err := bind.NewKeyedTransactorWithChainID(stranger, big.NewInt(1337))
	if err != nil {
		t.Fatal(err)
	}
	strangerAuth.GasLimit = e.auth.GasLimit
	z, nf := big.NewInt(0), big.NewInt(777)
	rcpt := e.ownershipReceipt(z, nf, big.NewInt(999))
	recipient := crypto.PubkeyToAddress(stranger.PublicKey)

	if r := e.sendFrom(strangerAuth, e.vABI, e.vault, "withdraw", []*big.Int{z, z}, recipient, rcpt); r != erc1155VaultRetired {
		t.Errorf("zero-param withdraw of another's receipt: got %q, want %s", r, erc1155VaultRetired)
	}
	if r := e.send(e.vABI, e.vault, "withdraw", []*big.Int{big.NewInt(100), big.NewInt(7)}, recipient, rcpt); r != erc1155VaultRetired {
		t.Errorf("withdraw: got %q, want %s", r, erc1155VaultRetired)
	}
	if e.call(e.vABI, e.vault, "isValidNullifier", z, nf).(bool) {
		t.Fatal("a refused withdraw spent the receipt's nullifier")
	}
}

// transfer took any legacy receipt with StMessage 0, a withdraw receipt
// included (R-064). One with a non-zero StMessage is a swap leg, which must
// settle only through EnygmaDvp with its counterparty's.
func TestV2Erc1155Vault_RefusesTransfer(t *testing.T) {
	e := newRetiredVaultEnv(t)
	z, nf := big.NewInt(0), big.NewInt(777)
	for _, message := range []int64{0, 42} {
		rcpt := e.ownershipReceipt(big.NewInt(message), nf, big.NewInt(999))
		if r := e.send(e.vABI, e.vault, "transfer", rcpt); r != erc1155VaultRetired {
			t.Errorf("transfer with StMessage %d: got %q, want %s", message, r, erc1155VaultRetired)
		}
	}
	if e.call(e.vABI, e.vault, "isValidNullifier", z, nf).(bool) {
		t.Fatal("a refused transfer spent the receipt's nullifier")
	}
}

// checkReceiptConditions is what EnygmaDvp's payment entry points call on a
// vault before nullifying, so it must refuse every legacy shape too. These
// are the shapes it sent to each retired VK slot: 1-in/1-out ownership (2),
// 2-in/2-out joinsplit (4), the 10-note batch (5), the fungible auditor
// joinsplit (15) and the non-fungible auditor ownership (16). Slot 3 shared
// slot 2's shape and was never selected.
func TestV2Erc1155Vault_RefusesLegacyReceipts(t *testing.T) {
	e := newRetiredVaultEnv(t)
	root := e.call(e.vABI, e.vault, "getRoot").(*big.Int)
	// statement fills a legacy statement of the given size: message 0, every
	// input in tree 0 under the current root, distinct non-zero nullifiers
	// and commitments.
	statement := func(size, nIn int) []*big.Int {
		st := make([]*big.Int, size)
		for i := range st {
			st[i] = big.NewInt(int64(100 + i))
		}
		st[0] = big.NewInt(0)
		for i := 0; i < nIn; i++ {
			st[1+i] = big.NewInt(0)
			st[1+nIn+i] = root
		}
		return st
	}
	cases := []struct {
		name            string
		size, nIn, nOut int
	}{
		{"ownership (slot 2)", 7, 1, 1},
		{"joinsplit (slot 4)", 11, 2, 2},
		{"batch (slot 5)", 61, 10, 10},
		{"fungible auditor (slot 15)", 23, 2, 2},
		{"non-fungible auditor (slot 16)", 16, 1, 1},
	}
	for _, c := range cases {
		rcpt := legacyReceipt(statement(c.size, c.nIn), int64(c.nIn), int64(c.nOut))
		if r := e.send(e.vABI, e.vault, "checkReceiptConditions", rcpt); r != erc1155VaultRetired {
			t.Errorf("checkReceiptConditions on %s: got %q, want %s", c.name, r, erc1155VaultRetired)
		}
	}
}

func TestV2Erc1155Vault_RefusesVerifyOwnership(t *testing.T) {
	e := newRetiredVaultEnv(t)
	amount, tokenId := big.NewInt(100), big.NewInt(7)
	uid := e.call(e.vABI, e.vault, "generateUniqueId", []*big.Int{amount, tokenId}).(*big.Int)
	wABI, _ := loadLinkedArtifact(t, "core/contracts/PoseidonWrapper.sol/PoseidonWrapper.json",
		map[string]common.Address{"PoseidonT3": {}, "PoseidonT5": {}})
	opening := e.call(wABI, e.wrapper, "poseidon", [2]*big.Int{uid, big.NewInt(0)}).(*big.Int)
	rcpt := e.ownershipReceipt(big.NewInt(42), big.NewInt(777), opening)
	if r := e.send(e.vABI, e.vault, "verifyOwnership", []*big.Int{amount, tokenId}, rcpt); r != erc1155VaultRetired {
		t.Errorf("verifyOwnership: got %q, want %s", r, erc1155VaultRetired)
	}
}
