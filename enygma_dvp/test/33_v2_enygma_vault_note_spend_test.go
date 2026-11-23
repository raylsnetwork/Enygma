package tests

// 33_v2_enygma_vault_note_spend_test.go
//
// Enygma Payments moves value between its shielded balances and
// EnygmaErc20CoinVault: a withdraw debits a balance and inserts a note through
// depositThroughEnygma; a deposit credits a balance and spends a note through
// withdrawThroughEnygma. The deposit proof credits the amount of the note it
// opens, Hash = Poseidon(Poseidon(asset, amount), Poseidon(sk)), and Enygma
// passes that Hash to withdrawThroughEnygma. This checks, on the real vault
// and circomlib Poseidon, that:
//   - the note depositThroughEnygma inserts is that Hash, so the deposit proof
//     and the vault refer to the same note and amount;
//   - withdrawThroughEnygma spends each inserted note once, refuses a
//     commitment the vault never inserted, and refuses any caller but Enygma;
//   - the vault refuses every receipt (checkReceiptConditions), so EnygmaDvp's
//     payment paths cannot spend its notes too.
// Runs on go-ethereum's simulated backend; needs `npx hardhat compile` and the
// circomlibjs Poseidon artifacts (node scripts/regen_poseidon.js).

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

// depositProofHash is the note Enygma Payments' deposit circuit opens.
func depositProofHash(t *testing.T, asset, amount, sk *big.Int) *big.Int {
	t.Helper()
	hash := func(in ...*big.Int) *big.Int {
		h, err := poseidon.Hash(in)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	return hash(hash(asset, amount), hash(sk))
}

func TestV2EnygmaVault_SpendsTheNoteItInserted(t *testing.T) {
	outsiderKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	e := newLockEnv(t, outsiderKey)
	vABI, vault := e.newVault("core/contracts/vaults/EnygmaErc20CoinVault.sol/EnygmaErc20CoinVault.json", e.auth.From)
	// This env's account acts as Enygma, the vault's asset.
	e.mustSend(vABI, vault, "addEnygma", e.auth.From)
	asset := new(big.Int).SetBytes(e.auth.From.Bytes())

	amount, sk := big.NewInt(250), big.NewInt(777)
	pk, err := poseidon.Hash([]*big.Int{sk})
	if err != nil {
		t.Fatal(err)
	}
	note := depositProofHash(t, asset, amount, sk)
	unspent := func() int64 { return e.call(vABI, vault, "unspentEnygmaNotes", note).(*big.Int).Int64() }

	if r := e.send(vABI, vault, "withdrawThroughEnygma", note); r != "UnknownEnygmaNote" {
		t.Fatalf("spend before the note exists: got %q, want UnknownEnygmaNote", r)
	}

	// Two withdrawals of the same amount to the same key insert the same
	// commitment twice: both are spendable, and no more.
	for i := 0; i < 2; i++ {
		e.mustSend(vABI, vault, "depositThroughEnygma", []*big.Int{amount, pk})
	}
	if got := unspent(); got != 2 {
		t.Fatalf("depositThroughEnygma inserted %d notes with the deposit proof's Hash, want 2 (vault and circuit disagree on the note)", got)
	}

	outsider, err := bind.NewKeyedTransactorWithChainID(outsiderKey, big.NewInt(1337))
	if err != nil {
		t.Fatal(err)
	}
	outsider.GasLimit = e.auth.GasLimit
	if r := e.sendFrom(outsider, vABI, vault, "withdrawThroughEnygma", note); !strings.Contains(r, "AccessControl") {
		t.Fatalf("spend by a non-Enygma account: got %q, want an AccessControl revert", r)
	}

	other := depositProofHash(t, asset, big.NewInt(251), sk)
	if r := e.send(vABI, vault, "withdrawThroughEnygma", other); r != "UnknownEnygmaNote" {
		t.Fatalf("spend of a note with another amount: got %q, want UnknownEnygmaNote", r)
	}

	for want := int64(1); want >= 0; want-- {
		e.mustSend(vABI, vault, "withdrawThroughEnygma", note)
		if got := unspent(); got != want {
			t.Fatalf("after a spend: %d unspent, want %d", got, want)
		}
	}
	if r := e.send(vABI, vault, "withdrawThroughEnygma", note); r != "UnknownEnygmaNote" {
		t.Fatalf("third spend of a note inserted twice: got %q, want UnknownEnygmaNote", r)
	}

	// EnygmaDvp's payment paths check a receipt through the vault before
	// nullifying or inserting; this vault refuses every receipt, so its notes
	// have no spend but withdrawThroughEnygma.
	n := big.NewInt
	receipt := legacyReceipt([]*big.Int{n(0), n(0), n(1), n(1), n(2), n(3), n(4), n(5)}, 2, 2)
	if r := e.send(vABI, vault, "checkReceiptConditions", receipt); r != "NotImplemented" {
		t.Fatalf("checkReceiptConditions on the Enygma vault: got %q, want NotImplemented", r)
	}
}
