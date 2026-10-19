package tests

// 28_v2_vault_legacy_withdraw_lock_test.go
//
// A note locked in a pending swap or auction (lockCoin) must not be spendable
// until it is unlocked. transfer, withdrawV2 and partialWithdraw check the lock
// through _nullifyFromReceipt, but the legacy withdraw functions of the ERC20,
// ERC721 and ERC1155 vaults set their nullifiers directly and skipped it.
//
// This deploys a vault against AcceptAllVerifier, so only the vault's own
// checks apply, deposits a note, locks its nullifier, and checks that the
// legacy withdraw refuses it with CantSpendLockedCoin, then succeeds once the
// note is unlocked, for each of the three vaults. Runs on go-ethereum's
// simulated backend; needs `npx hardhat compile` and the circomlibjs Poseidon
// artifacts (node scripts/regen_poseidon.js).

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/rpc"
)

type lockEnv struct {
	t        *testing.T
	backend  *simulated.Backend
	auth     *bind.TransactOpts
	wrapper  common.Address
	verifier common.Address
}

func newLockEnv(t *testing.T) *lockEnv {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	from := crypto.PubkeyToAddress(key.PublicKey)
	backend := simulated.NewBackend(types.GenesisAlloc{from: {Balance: new(big.Int).Lsh(big.NewInt(1), 100)}})
	t.Cleanup(func() { backend.Close() })
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(1337))
	if err != nil {
		t.Fatal(err)
	}
	auth.GasLimit = 15_000_000
	e := &lockEnv{t: t, backend: backend, auth: auth}

	libs := map[string]common.Address{}
	for _, name := range []string{"PoseidonT3", "PoseidonT5"} {
		a, bin := loadArtifact(t, "core/contracts/Poseidon.sol/"+name+".json")
		if len(bin) < 1000 {
			t.Skipf("%s artifact is a placeholder — run node scripts/regen_poseidon.js", name)
		}
		libs[name] = e.deploy(a, bin)
	}
	wABI, wBin := loadLinkedArtifact(t, "core/contracts/PoseidonWrapper.sol/PoseidonWrapper.json", libs)
	e.wrapper = e.deploy(wABI, wBin)
	vABI, vBin := loadArtifact(t, "test/AcceptAllVerifier.sol/AcceptAllVerifier.json")
	e.verifier = e.deploy(vABI, vBin)
	return e
}

// loadLinkedArtifact loads a Hardhat artifact and writes each linked library's
// address into its bytecode at the offsets listed in linkReferences.
func loadLinkedArtifact(t *testing.T, rel string, libs map[string]common.Address) (abi.ABI, []byte) {
	t.Helper()
	data, err := os.ReadFile("../artifacts/contracts/" + rel)
	if err != nil {
		t.Skipf("artifact %s missing (run `npx hardhat compile`): %v", rel, err)
	}
	var a struct {
		ABI            json.RawMessage `json:"abi"`
		Bytecode       string          `json:"bytecode"`
		LinkReferences map[string]map[string][]struct {
			Start  int `json:"start"`
			Length int `json:"length"`
		} `json:"linkReferences"`
	}
	if err := json.Unmarshal(data, &a); err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
	code := []byte(strings.TrimPrefix(a.Bytecode, "0x"))
	for _, refs := range a.LinkReferences {
		for name, offsets := range refs {
			addr, ok := libs[name]
			if !ok {
				t.Fatalf("%s links %s, which was not deployed", rel, name)
			}
			hexAddr := hex.EncodeToString(addr.Bytes())
			for _, o := range offsets {
				copy(code[2*o.Start:2*(o.Start+o.Length)], hexAddr)
			}
		}
	}
	parsed, err := abi.JSON(strings.NewReader(string(a.ABI)))
	if err != nil {
		t.Fatalf("parse ABI %s: %v", rel, err)
	}
	bin, err := hex.DecodeString(string(code))
	if err != nil {
		t.Fatalf("%s bytecode after linking: %v", rel, err)
	}
	return parsed, bin
}

func (e *lockEnv) deploy(a abi.ABI, bin []byte, args ...interface{}) common.Address {
	e.t.Helper()
	addr, _, _, err := bind.DeployContract(e.auth, a, bin, e.backend.Client(), args...)
	if err != nil {
		e.t.Fatalf("deploy: %v", err)
	}
	e.backend.Commit()
	return addr
}

// send simulates the call (to read the custom error), then mines it. It
// returns the custom error name, or "" on success.
func (e *lockEnv) send(a abi.ABI, to common.Address, method string, args ...interface{}) string {
	e.t.Helper()
	data, err := a.Pack(method, args...)
	if err != nil {
		e.t.Fatalf("pack %s: %v", method, err)
	}
	msg := ethereum.CallMsg{From: e.auth.From, To: &to, Gas: e.auth.GasLimit, Data: data}
	if _, err := e.backend.Client().CallContract(context.Background(), msg, nil); err != nil {
		var de rpc.DataError
		if errors.As(err, &de) {
			if s, ok := de.ErrorData().(string); ok {
				raw, _ := hex.DecodeString(strings.TrimPrefix(s, "0x"))
				for name, ce := range a.Errors {
					if len(raw) >= 4 && string(ce.ID[:4]) == string(raw[:4]) {
						return name
					}
				}
			}
		}
		return err.Error()
	}
	tx, err := bind.NewBoundContract(to, a, nil, e.backend.Client(), nil).RawTransact(e.auth, data)
	if err != nil {
		e.t.Fatalf("send %s: %v", method, err)
	}
	e.backend.Commit()
	rcpt, err := e.backend.Client().TransactionReceipt(context.Background(), tx.Hash())
	if err != nil || rcpt.Status != types.ReceiptStatusSuccessful {
		e.t.Fatalf("%s mined but reverted (%v)", method, err)
	}
	return ""
}

func (e *lockEnv) mustSend(a abi.ABI, to common.Address, method string, args ...interface{}) {
	e.t.Helper()
	if r := e.send(a, to, method, args...); r != "" {
		e.t.Fatalf("%s: %s", method, r)
	}
}

func (e *lockEnv) call(a abi.ABI, to common.Address, method string, args ...interface{}) interface{} {
	e.t.Helper()
	var out []interface{}
	if err := bind.NewBoundContract(to, a, e.backend.Client(), nil, nil).Call(nil, &out, method, args...); err != nil {
		e.t.Fatalf("call %s: %v", method, err)
	}
	return out[0]
}

// newVault deploys and initializes a vault, with this env's account as both
// the DvP and the auction contract, so it may lock and unlock notes.
func (e *lockEnv) newVault(artifact string, asset common.Address) (abi.ABI, common.Address) {
	vABI, vBin := loadArtifact(e.t, artifact)
	vault := e.deploy(vABI, vBin, e.auth.From)
	e.mustSend(vABI, vault, "initializeVault", big.NewInt(1), big.NewInt(1), asset,
		big.NewInt(8), e.wrapper, e.verifier, e.auth.From)
	return vABI, vault
}

// checkLockedThenUnlocked locks the note's nullifier, checks the legacy
// withdraw refuses it, unlocks it, and checks the same withdraw then succeeds
// and spends the nullifier.
func (e *lockEnv) checkLockedThenUnlocked(vABI abi.ABI, vault common.Address, nf *big.Int, withdraw func() string) {
	e.t.Helper()
	zero := big.NewInt(0)
	e.mustSend(vABI, vault, "lockCoin", zero, nf)
	if r := withdraw(); r != "CantSpendLockedCoin" {
		e.t.Fatalf("withdraw of a locked note: got %q, want CantSpendLockedCoin", r)
	}
	if e.call(vABI, vault, "isValidNullifier", zero, nf).(bool) {
		e.t.Fatal("refused withdraw still spent the nullifier")
	}
	e.mustSend(vABI, vault, "unlockCoin", zero, nf)
	if r := withdraw(); r != "" {
		e.t.Fatalf("withdraw of the unlocked note: %s", r)
	}
	if !e.call(vABI, vault, "isValidNullifier", zero, nf).(bool) {
		e.t.Fatal("withdraw did not spend the nullifier")
	}
}

func legacyReceipt(statement []*big.Int, nIn, nOut int64) onchainProofReceipt {
	z := big.NewInt(0)
	return onchainProofReceipt{
		Proof: onchainSnarkProof{
			A: onchainG1Point{X: z, Y: z},
			B: onchainG2Point{X: [2]*big.Int{z, z}, Y: [2]*big.Int{z, z}},
			C: onchainG1Point{X: z, Y: z},
		},
		Statement:       statement,
		NumberOfInputs:  big.NewInt(nIn),
		NumberOfOutputs: big.NewInt(nOut),
	}
}

func TestV2VaultLegacyWithdraw_RefusesLockedNote_Erc20(t *testing.T) {
	e := newLockEnv(t)
	tABI, tBin := loadArtifact(t, "erc20/contracts/RaylsERC20.sol/RaylsERC20.json")
	token := e.deploy(tABI, tBin, "Token", "TKN")
	vABI, vault := e.newVault("core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json", token)

	amount := big.NewInt(100)
	e.mustSend(tABI, token, "mint", e.auth.From, amount)
	e.mustSend(tABI, token, "approve", vault, amount)
	e.mustSend(vABI, vault, "deposit", []*big.Int{amount, big.NewInt(123), big.NewInt(456), big.NewInt(0)})

	recipient := common.HexToAddress("0x00000000000000000000000000000000000000b0")
	uid := e.call(vABI, vault, "generateUniqueId", []*big.Int{amount, new(big.Int).SetBytes(token.Bytes())}).(*big.Int)
	wABI, _ := loadLinkedArtifact(t, "core/contracts/PoseidonWrapper.sol/PoseidonWrapper.json",
		map[string]common.Address{"PoseidonT3": {}, "PoseidonT5": {}})
	opening := e.call(wABI, e.wrapper, "poseidon", [2]*big.Int{uid, new(big.Int).SetBytes(recipient.Bytes())}).(*big.Int)

	nf := big.NewInt(777)
	root := e.call(vABI, vault, "getRoot").(*big.Int)
	// [message, tree, root, nullifier, output0, output1, vault]
	rcpt := legacyReceipt([]*big.Int{big.NewInt(0), big.NewInt(0), root, nf, opening, big.NewInt(999),
		new(big.Int).SetBytes(vault.Bytes())}, 1, 2)

	e.checkLockedThenUnlocked(vABI, vault, nf, func() string {
		return e.send(vABI, vault, "withdraw", []*big.Int{amount}, recipient, rcpt)
	})
	if got := e.call(tABI, token, "balanceOf", recipient).(*big.Int); got.Cmp(amount) != 0 {
		t.Fatalf("recipient balance %s, want %s", got, amount)
	}
}

func TestV2VaultLegacyWithdraw_RefusesLockedNote_Erc721(t *testing.T) {
	e := newLockEnv(t)
	tABI, tBin := loadArtifact(t, "erc721/contracts/RaylsERC721.sol/RaylsERC721.json")
	token := e.deploy(tABI, tBin, "NFT", "NFT")
	vABI, vault := e.newVault("core/contracts/vaults/Erc721CoinVault.sol/Erc721CoinVault.json", token)

	nftId := big.NewInt(5)
	e.mustSend(tABI, token, "mint", e.auth.From, nftId)
	e.mustSend(tABI, token, "approve", vault, nftId)
	e.mustSend(vABI, vault, "deposit", []*big.Int{nftId, big.NewInt(123), big.NewInt(456)})

	recipient := common.HexToAddress("0x00000000000000000000000000000000000000b0")
	uid := e.call(vABI, vault, "generateUniqueId", []*big.Int{nftId}).(*big.Int)
	wABI, _ := loadLinkedArtifact(t, "core/contracts/PoseidonWrapper.sol/PoseidonWrapper.json",
		map[string]common.Address{"PoseidonT3": {}, "PoseidonT5": {}})
	opening := e.call(wABI, e.wrapper, "poseidon", [2]*big.Int{uid, new(big.Int).SetBytes(recipient.Bytes())}).(*big.Int)

	nf := big.NewInt(777)
	root := e.call(vABI, vault, "getRoot").(*big.Int)
	// legacy ownership layout: [message 0, tree 0, root, nullifier, output0]
	rcpt := legacyReceipt([]*big.Int{big.NewInt(0), big.NewInt(0), root, nf, opening}, 1, 1)

	e.checkLockedThenUnlocked(vABI, vault, nf, func() string {
		return e.send(vABI, vault, "withdraw", []*big.Int{nftId}, recipient, rcpt)
	})
	if got := e.call(tABI, token, "ownerOf", nftId).(common.Address); got != recipient {
		t.Fatalf("NFT owner %s, want %s", got.Hex(), recipient.Hex())
	}
}

func TestV2VaultLegacyWithdraw_RefusesLockedNote_Erc1155(t *testing.T) {
	e := newLockEnv(t)
	tABI, tBin := loadArtifact(t, "erc1155/contracts/RaylsERC1155.sol/RaylsERC1155.json")
	token := e.deploy(tABI, tBin, "uri")
	vABI, vault := e.newVault("core/contracts/vaults/Erc1155CoinVault.sol/Erc1155CoinVault.json", token)

	// Minting an unregistered id registers it as a fungible token.
	tokenId, amount := big.NewInt(7), big.NewInt(100)
	e.mustSend(tABI, token, "mint", e.auth.From, tokenId, amount, []byte{})
	e.mustSend(tABI, token, "setApprovalForAll", vault, true)
	e.mustSend(vABI, vault, "deposit", []*big.Int{amount, tokenId, big.NewInt(123), big.NewInt(456)})

	recipient := common.HexToAddress("0x00000000000000000000000000000000000000b0")
	uid := e.call(vABI, vault, "generateUniqueId", []*big.Int{amount, tokenId}).(*big.Int)
	wABI, _ := loadLinkedArtifact(t, "core/contracts/PoseidonWrapper.sol/PoseidonWrapper.json",
		map[string]common.Address{"PoseidonT3": {}, "PoseidonT5": {}})
	opening := e.call(wABI, e.wrapper, "poseidon", [2]*big.Int{uid, new(big.Int).SetBytes(recipient.Bytes())}).(*big.Int)

	nf := big.NewInt(777)
	root := e.call(vABI, vault, "getRoot").(*big.Int)
	// 1-in/1-out ownership layout (3 + 3*1 + 1 = 7): [message, tree, root, nullifier, output0, ...2 more]
	rcpt := legacyReceipt([]*big.Int{big.NewInt(0), big.NewInt(0), root, nf, opening, big.NewInt(0), big.NewInt(0)}, 1, 1)

	e.checkLockedThenUnlocked(vABI, vault, nf, func() string {
		return e.send(vABI, vault, "withdraw", []*big.Int{amount, tokenId}, recipient, rcpt)
	})
	if got := e.call(tABI, token, "balanceOf", recipient, tokenId).(*big.Int); got.Cmp(amount) != 0 {
		t.Fatalf("recipient balance %s, want %s", got, amount)
	}
}
