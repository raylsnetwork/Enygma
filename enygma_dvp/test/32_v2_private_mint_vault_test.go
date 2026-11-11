package tests

// 32_v2_private_mint_vault_test.go
//
// EnygmaDvp.privateMint inserts a note that no deposit backs, so it only
// accepts a vault the owner has flagged with setVaultMintable; every vault
// starts unflagged. Deploys the real EnygmaDvp and Erc20CoinVault with
// AcceptAllPrivateMintVerifier, so only EnygmaDvp's own checks apply. Runs on
// go-ethereum's simulated backend; needs `npx hardhat compile` and the
// circomlibjs Poseidon artifacts (node scripts/regen_poseidon.js).

import (
	"math/big"
	"testing"
)

func TestV2PrivateMint_OnlyIntoMintableVaults(t *testing.T) {
	e := newLockEnv(t)

	dvpABI, dvpBin := loadArtifact(t, "core/contracts/EnygmaDvp.sol/EnygmaDvp.json")
	dvp := e.deploy(dvpABI, dvpBin, e.wrapper, e.verifier)
	e.mustSend(dvpABI, dvp, "initializeDvp", e.verifier)
	pmABI, pmBin := loadArtifact(t, "test/AcceptAllPrivateMintVerifier.sol/AcceptAllPrivateMintVerifier.json")
	e.mustSend(dvpABI, dvp, "registerPrivateMintVerifier", e.deploy(pmABI, pmBin))

	tABI, tBin := loadArtifact(t, "erc20/contracts/RaylsERC20.sol/RaylsERC20.json")
	token := e.deploy(tABI, tBin, "Token", "TKN")
	vABI, vBin := loadArtifact(t, "core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json")
	vault := e.deploy(vABI, vBin, dvp)
	e.mustSend(dvpABI, dvp, "registerVault", vault, token, big.NewInt(1), big.NewInt(8))
	vaultID := big.NewInt(0)

	mint := func(commitment int64) string {
		var p onchainPrivateMintProof
		for i := range p.Proof {
			p.Proof[i] = big.NewInt(0)
		}
		p.PublicSignal = [4]*big.Int{big.NewInt(commitment), new(big.Int).SetBytes(dvp.Bytes()), big.NewInt(0), big.NewInt(9)}
		return e.send(dvpABI, dvp, "privateMint", vaultID, big.NewInt(commitment), p)
	}
	root := func() *big.Int { return e.call(vABI, vault, "getRoot").(*big.Int) }

	before := root()
	if r := mint(4242); r != "VaultNotMintable" {
		t.Fatalf("mint into an unflagged vault: got %q, want VaultNotMintable", r)
	}
	if root().Cmp(before) != 0 {
		t.Fatal("a refused mint changed the vault's tree")
	}

	if r := e.send(dvpABI, dvp, "setVaultMintable", big.NewInt(5), true); r != "InvalidVaultId" {
		t.Fatalf("flagging an unregistered vault: got %q, want InvalidVaultId", r)
	}

	e.mustSend(dvpABI, dvp, "setVaultMintable", vaultID, true)
	if !e.call(dvpABI, dvp, "mintableVaults", vaultID).(bool) {
		t.Fatal("mintableVaults(0) is false after setVaultMintable(0, true)")
	}
	if r := mint(4242); r != "" {
		t.Fatalf("mint into a flagged vault: %s", r)
	}
	if root().Cmp(before) == 0 {
		t.Fatal("the mint did not insert a note")
	}

	e.mustSend(dvpABI, dvp, "setVaultMintable", vaultID, false)
	if r := mint(4343); r != "VaultNotMintable" {
		t.Fatalf("mint after unflagging: got %q, want VaultNotMintable", r)
	}
}
