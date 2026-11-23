package enygma_test

// deposit() credits the sender the amount of the DvP note its proof's Hash
// names (the deposit circuit binds Hash to the credit and to the prover's
// key: gnark-server/pkg/circuits/deposit/note_binding_test.go), and has the
// vault spend that note.
//
//   - TestDepositSpendsTheNoteItCredits, against MockZkDvp (which spends notes
//     the way EnygmaErc20CoinVault does): the note the vault spends is the
//     proof's Hash, and a deposit whose Hash is not an unspent note reverts
//     with nothing credited.
//   - TestDepositRedeemsARealVaultNote, against the real EnygmaErc20CoinVault
//     and circomlib Poseidon (enygma_dvp's artifacts): a withdraw() of 250
//     makes the note the deposit circuit's Hash names, a deposit() of that
//     Hash credits 250 back and spends it, the balance and check() come back
//     to where they started, and the note cannot be redeemed twice.
//
// Both use MockWithdrawVerifier/MockDepositVerifier (accept any proof): the
// proofs' soundness is the circuits' tests.
//
//	CC=/usr/bin/clang go test -run 'TestDepositSpendsTheNoteItCredits|TestDepositRedeemsARealVaultNote' -v -timeout 120s
//
// The real-vault test needs enygma_dvp compiled with the Poseidon artifacts:
// cd enygma_dvp && npx hardhat compile && node scripts/regen_poseidon.js

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"strings"
	"testing"

	enygma "enygma_payments/go_client/contracts"
	"enygma_payments/layout"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

const mockZkDvpNoteABI = `[
 {"type":"function","name":"addNote","inputs":[{"name":"commitment","type":"uint256"}],"outputs":[],"stateMutability":"nonpayable"},
 {"type":"function","name":"unspentEnygmaNotes","inputs":[{"name":"","type":"uint256"}],"outputs":[{"name":"","type":"uint256"}],"stateMutability":"view"},
 {"type":"function","name":"lastSpentNote","inputs":[],"outputs":[{"name":"","type":"uint256"}],"stateMutability":"view"}
]`

func mockZkDvpNotes(t *testing.T, client *ethclient.Client, bridge common.Address) *bind.BoundContract {
	t.Helper()
	parsed, err := abi.JSON(strings.NewReader(mockZkDvpNoteABI))
	if err != nil {
		t.Fatal(err)
	}
	return bind.NewBoundContract(bridge, parsed, client, client, client)
}

// addMockNote records an unspent note with this commitment in the mock vault,
// as a withdraw() that created it would have.
func addMockNote(t *testing.T, client *ethclient.Client, mkAuth func() *bind.TransactOpts,
	waitTx func(*types.Transaction, error) *types.Receipt, bridge common.Address, note *big.Int) {
	t.Helper()
	if r := waitTx(mockZkDvpNotes(t, client, bridge).Transact(mkAuth(), "addNote", note)); r.Status != 1 {
		t.Fatal("MockZkDvp.addNote failed")
	}
}

// noteView reads a note view (unspentEnygmaNotes, lastSpentNote) of the
// vault at bridge, the mock or the real one.
func noteView(t *testing.T, client *ethclient.Client, bridge common.Address, method string, args ...interface{}) *big.Int {
	t.Helper()
	var out []interface{}
	if err := mockZkDvpNotes(t, client, bridge).Call(&bind.CallOpts{Context: context.Background()}, &out, method, args...); err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	return out[0].(*big.Int)
}

// bridgeFixture is a fresh Enygma with mock withdraw and deposit verifiers,
// confirmed fingerprints, and the bridge newBridge deploys for it.
type bridgeFixture struct {
	instance   *enygma.Enygma
	enygmaAddr common.Address
	client     *ethclient.Client
	mkAuth     func() *bind.TransactOpts
	waitTx     func(*types.Transaction, error) *types.Receipt
	bridge     common.Address
	fp         [nBanks][nBanks]*big.Int
	ids        []*big.Int
}

func newBridgeFixture(t *testing.T, newBridge func(f *bridgeFixture) common.Address) *bridgeFixture {
	t.Helper()
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	client, mkAuth, waitTx := scenarioClient(t)
	instance, enygmaAddr := freshSetup(t, client, mkAuth, waitTx)
	f := &bridgeFixture{instance: instance, enygmaAddr: enygmaAddr, client: client, mkAuth: mkAuth, waitTx: waitTx}

	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	wv := deployFromArtifact(t, client, mkAuth(), artifactBase+"/mocks/MockWithdrawVerifier.sol/MockWithdrawVerifier.json")
	approveTestVerifier(t, client, instance, mkAuth(), testKindWithdraw, nBanks, wv)
	if r := waitTx(instance.AddWithdrawVerifier(mkAuth(), wv, big.NewInt(nBanks))); r.Status != 1 {
		t.Fatal("addWithdrawVerifier failed")
	}
	dv := deployFromArtifact(t, client, mkAuth(), artifactBase+"/mocks/MockDepositVerifier.sol/MockDepositVerifier.json")
	approveTestVerifier(t, client, instance, mkAuth(), testKindDeposit, 0, dv)
	if r := waitTx(instance.AddDepositVerifier(mkAuth(), dv)); r.Status != 1 {
		t.Fatal("addDepositVerifier failed")
	}
	f.bridge = newBridge(f)
	if r := waitTx(instance.AddZkDvp(mkAuth(), f.bridge)); r.Status != 1 {
		t.Fatal("addZkDvp failed")
	}
	f.fp = confirmFreshFingerprints(t, client, mkAuth, waitTx, instance)
	f.ids = make([]*big.Int, nBanks)
	for i := range f.ids {
		f.ids[i] = big.NewInt(int64(i + 1))
	}
	return f
}

// bank1Deltas moves bank 1's balance by value (a field element: P - v is a
// debit of v) and leaves the others.
func bank1Deltas(value *big.Int) []enygma.IEnygmaPoint {
	deltas := make([]enygma.IEnygmaPoint, nBanks)
	for i := range deltas {
		deltas[i] = enygma.IEnygmaPoint{C1: big.NewInt(0), C2: big.NewInt(1)}
	}
	c := pedersenCommitment(value, big.NewInt(0))
	deltas[0] = enygma.IEnygmaPoint{C1: c.X, C2: c.Y}
	return deltas
}

// sharedSignals fills the signals withdraw and deposit share (the first
// DepositLen) for deltas, against the current block and balances.
func (f *bridgeFixture) sharedSignals(t *testing.T, sig []*big.Int, deltas []enygma.IEnygmaPoint, seed string) {
	t.Helper()
	for i := range sig {
		sig[i] = big.NewInt(0)
	}
	blockHash, err := f.instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		t.Fatal(err)
	}
	pv, err := f.instance.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
	if err != nil {
		t.Fatal(err)
	}
	putFingerprints(sig, layout.DepositFingerprintOffset, f.fp)
	for i := 0; i < nBanks; i++ {
		sig[layout.DepositPublicKeysOffset+i] = pv.Keys[i+1]
		sig[layout.DepositPreviousCommitmentsOffset+2*i] = pv.Balances[i+1].C1
		sig[layout.DepositPreviousCommitmentsOffset+2*i+1] = pv.Balances[i+1].C2
		sig[layout.DepositTxCommitmentsOffset+2*i] = deltas[i].C1
		sig[layout.DepositTxCommitmentsOffset+2*i+1] = deltas[i].C2
	}
	sig[layout.DepositBlockNumberOffset] = blockHash
	sig[layout.DepositNullifierOffset] = deterministicNullifier(t, seed)
	sig[layout.DepositDomainIdOffset] = expectedDomainId(f.enygmaAddr)
}

func zeroProof() [8]*big.Int {
	var p [8]*big.Int
	for i := range p {
		p[i] = big.NewInt(0)
	}
	return p
}

// deposit submits a deposit crediting bank 1 by amount and redeeming note.
func (f *bridgeFixture) deposit(t *testing.T, amount int64, note *big.Int, seed string) (*types.Transaction, error) {
	t.Helper()
	deltas := bank1Deltas(big.NewInt(amount))
	var sig [layout.DepositLen]*big.Int
	f.sharedSignals(t, sig[:], deltas, seed)
	sig[layout.DepositHashOffset] = note
	return f.instance.Deposit(f.mkAuth(), deltas, enygma.IEnygmaDepositProof{Proof: zeroProof(), PublicSignal: sig}, f.ids)
}

// withdraw submits a withdraw debiting bank 1 by amount into one DvP note
// for publicKey, whose commitment is note.
func (f *bridgeFixture) withdraw(t *testing.T, amount int64, publicKey, note *big.Int, seed string) (*types.Transaction, error) {
	t.Helper()
	deltas := bank1Deltas(new(big.Int).Sub(curveP, big.NewInt(amount)))
	var sig [layout.WithdrawLen]*big.Int
	f.sharedSignals(t, sig[:], deltas, seed)
	sig[layout.WithdrawTotalDepositValueOffset] = big.NewInt(amount)
	sig[layout.WithdrawDepositCommitmentsOffset] = note
	params := []enygma.IEnygmaDepositParams{{Amount: big.NewInt(amount), Erc20Adress: common.Address{}, PublicKey: publicKey}}
	return f.instance.Withdraw(f.mkAuth(), deltas, enygma.IEnygmaWithdrawProof{Proof: zeroProof(), PublicSignal: sig}, params, f.ids)
}

func (f *bridgeFixture) balance1(t *testing.T) enygma.IEnygmaPoint {
	t.Helper()
	x, err := f.instance.GetBalance(&bind.CallOpts{}, big.NewInt(1))
	if err != nil {
		t.Fatal(err)
	}
	return enygma.IEnygmaPoint{C1: x.X, C2: x.Y}
}

func (f *bridgeFixture) mustCheck(t *testing.T, when string) {
	t.Helper()
	ok, err := f.instance.Check(&bind.CallOpts{})
	if err != nil || !ok {
		t.Fatalf("check() %s: ok=%v err=%v", when, ok, err)
	}
}

// wantUnknownNote requires a deposit of note to revert UnknownEnygmaNote
// (by name, or by selector where the node lacks the vault's artifacts) and
// to leave bank 1's balance as it was.
func (f *bridgeFixture) wantUnknownNote(t *testing.T, note *big.Int, seed string) {
	t.Helper()
	before := f.balance1(t)
	_, err := f.deposit(t, 250, note, seed)
	selector := hex.EncodeToString(crypto.Keccak256([]byte("UnknownEnygmaNote()"))[:4])
	if err == nil || !(strings.Contains(err.Error(), "UnknownEnygmaNote") || strings.Contains(err.Error(), selector)) {
		t.Fatalf("deposit redeeming a note the vault does not hold: got %v, want UnknownEnygmaNote", err)
	}
	if after := f.balance1(t); after.C1.Cmp(before.C1) != 0 || after.C2.Cmp(before.C2) != 0 {
		t.Fatal("a refused deposit moved bank 1's balance")
	}
}

func TestDepositSpendsTheNoteItCredits(t *testing.T) {
	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	f := newBridgeFixture(t, func(f *bridgeFixture) common.Address {
		return deployFromArtifact(t, f.client, f.mkAuth(), artifactBase+"/mocks/MockZkDvp.sol/MockZkDvp.json")
	})

	note := big.NewInt(0xbeef)
	f.wantUnknownNote(t, note, "note-before")

	addMockNote(t, f.client, f.mkAuth, f.waitTx, f.bridge, note)
	if r := f.waitTx(f.deposit(t, 250, note, "note-spend")); r.Status != 1 {
		t.Fatal("deposit redeeming a note the vault holds reverted")
	}
	if got := noteView(t, f.client, f.bridge, "lastSpentNote"); got.Cmp(note) != 0 {
		t.Fatalf("the vault spent note %s, want the proof's Hash %s", got, note)
	}
	if got := noteView(t, f.client, f.bridge, "unspentEnygmaNotes", note); got.Sign() != 0 {
		t.Fatalf("%s unspent notes left after the deposit, want 0", got)
	}
	f.mustCheck(t, "after the deposit")

	// A second deposit (a new nullifier) cannot redeem the same note again.
	f.wantUnknownNote(t, note, "note-again")
}

// dvpArtifacts holds enygma_dvp's compiled contracts.
const dvpArtifacts = "../../../enygma_dvp/artifacts/contracts/core/contracts/"

// deployDvpLinked deploys an enygma_dvp artifact, writing each linked
// library's address into its bytecode.
func deployDvpLinked(t *testing.T, f *bridgeFixture, rel string, libs map[string]common.Address, args ...interface{}) (abi.ABI, common.Address) {
	t.Helper()
	data, err := os.ReadFile(dvpArtifacts + rel)
	if err != nil {
		t.Skipf("enygma_dvp artifact %s missing (cd enygma_dvp && npx hardhat compile): %v", rel, err)
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
			for _, o := range offsets {
				copy(code[2*o.Start:2*(o.Start+o.Length)], hex.EncodeToString(addr.Bytes()))
			}
		}
	}
	parsed, err := abi.JSON(strings.NewReader(string(a.ABI)))
	if err != nil {
		t.Fatalf("parse ABI %s: %v", rel, err)
	}
	bin, err := hex.DecodeString(string(code))
	if err != nil {
		t.Fatalf("%s bytecode: %v", rel, err)
	}
	if len(bin) < 500 && strings.HasPrefix(rel, "Poseidon.sol/") {
		t.Skipf("%s is a placeholder — cd enygma_dvp && node scripts/regen_poseidon.js", rel)
	}
	addr, tx, _, err := bind.DeployContract(f.mkAuth(), parsed, bin, f.client, args...)
	if err != nil {
		t.Fatalf("deploy %s: %v", rel, err)
	}
	if r := f.waitTx(tx, nil); r.Status != 1 {
		t.Fatalf("deploy %s reverted", rel)
	}
	return parsed, addr
}

func TestDepositRedeemsARealVaultNote(t *testing.T) {
	f := newBridgeFixture(t, func(f *bridgeFixture) common.Address {
		libs := map[string]common.Address{}
		for _, name := range []string{"PoseidonT3", "PoseidonT5"} {
			_, libs[name] = deployDvpLinked(t, f, "Poseidon.sol/"+name+".json", nil)
		}
		_, wrapper := deployDvpLinked(t, f, "PoseidonWrapper.sol/PoseidonWrapper.json", libs)
		me := f.mkAuth().From
		// This account acts as the vault's EnygmaDvp, so it may initialize it.
		vABI, vault := deployDvpLinked(t, f, "vaults/EnygmaErc20CoinVault.sol/EnygmaErc20CoinVault.json", nil, me)
		v := bind.NewBoundContract(vault, vABI, f.client, f.client, f.client)
		if r := f.waitTx(v.Transact(f.mkAuth(), "initializeVault", big.NewInt(3), big.NewInt(1), f.enygmaAddr,
			big.NewInt(8), wrapper, wrapper, me)); r.Status != 1 {
			t.Fatal("initializeVault failed")
		}
		if r := f.waitTx(v.Transact(f.mkAuth(), "addEnygma", f.enygmaAddr)); r.Status != 1 {
			t.Fatal("addEnygma failed")
		}
		return vault
	})

	// The note a withdrawal of 250 to the owner of sk makes, as the deposit
	// circuit computes it: Poseidon(Poseidon(asset, amount), Poseidon(sk)).
	hash := func(in ...*big.Int) *big.Int {
		h, err := poseidon.Hash(in)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	const amount = 250
	sk := big.NewInt(777)
	pk := hash(sk)
	note := hash(hash(new(big.Int).SetBytes(f.enygmaAddr.Bytes()), big.NewInt(amount)), pk)

	start := f.balance1(t)
	f.wantUnknownNote(t, note, "real-before")

	if r := f.waitTx(f.withdraw(t, amount, pk, note, "real-withdraw")); r.Status != 1 {
		t.Fatal("withdraw of 250 into the real vault reverted (the vault's note is not the deposit circuit's Hash?)")
	}
	if got := noteView(t, f.client, f.bridge, "unspentEnygmaNotes", note); got.Int64() != 1 {
		t.Fatalf("after the withdraw the vault holds %s notes with the deposit circuit's Hash, want 1", got)
	}
	f.mustCheck(t, "after the withdraw")

	if r := f.waitTx(f.deposit(t, amount, note, "real-deposit")); r.Status != 1 {
		t.Fatal("deposit redeeming the real vault's note reverted")
	}
	if got := noteView(t, f.client, f.bridge, "unspentEnygmaNotes", note); got.Sign() != 0 {
		t.Fatalf("after the deposit the vault still holds %s of the note, want 0", got)
	}
	f.mustCheck(t, "after the deposit")
	if end := f.balance1(t); end.C1.Cmp(start.C1) != 0 || end.C2.Cmp(start.C2) != 0 {
		t.Fatal("bank 1's balance after withdrawing 250 and depositing the note back differs from its start")
	}

	f.wantUnknownNote(t, note, "real-again")
}
