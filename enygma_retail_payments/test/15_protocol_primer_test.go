package tests

// TestProtocolPrimer runs the Protocol primer's story against the live
// stack with fresh users (the Sender and Ana) and a fresh auditor, using the
// client library, and recomputes every equation the primer shows from the
// run's values and the chain. Only when every check has run and held does it
// rewrite the JSON data block inside the primer page
// (shield_demo/protocol.html), which is those values' only copy; a filtered
// or failed run writes nothing.
//
// The steps so far: keys (the Sender's and Ana's spend and view keys), the
// auditor's keys, and the registration of both users.
//
// Prerequisites: the stack shield_demo/run.sh --no-demo brings up (the
// chain and a deployment; this test calls neither the gnark server nor the
// relayer yet).
//
// Run:
//
//	cd test && CC=/usr/bin/clang go test -run 'TestProtocolPrimer$' -v

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/hkdf"
	"crypto/mlkem"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"regexp"
	"slices"
	"testing"

	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iden3/go-iden3-crypto/constants"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

// primerPage is the Protocol primer, whose data block this test rewrites.
const primerPage = "../shield_demo/protocol.html"

// auditHKDFInfo is the HKDF info string the audit key is derived under
// (src/core/auditor); the primer shows it, so the test derives with it too.
const auditHKDFInfo = "enygma-audit-view-key-v1"

// primerData is the primer's data block: the run's values, one entry per
// step, keyed by step name. Every value is a decimal or 0x string, in full.
type primerData struct {
	Run   primerRun              `json:"run"`
	Steps map[string]primerEntry `json:"steps"`
}

type primerRun struct {
	ChainID      string `json:"chainId"`
	UserRegistry string `json:"userRegistry"`
}

// primerEntry is one step's values and the public items it adds to the
// observer's ledger.
type primerEntry struct {
	Values map[string]string `json:"values"`
	Ledger []ledgerItem      `json:"ledger"`
}

// ledgerItem is one public item an observer of the chain sees. MsgSender
// marks a transaction's msg.sender, the link the protocol without a relayer
// leaves (the page shows it amber).
type ledgerItem struct {
	Tx        string `json:"tx"`
	Field     string `json:"field"`
	Value     string `json:"value"`
	MsgSender bool   `json:"msgSender,omitempty"`
}

// primerUser is one of the story's users: their signing account and keys.
type primerUser struct {
	name, id string // "Sender"; "sender", the data block's key prefix
	key      *ecdsa.PrivateKey
	addr     common.Address
	spend    *rpcore.SpendKeyPair
	view     *rpcore.ViewKeyPair
}

// primerChecks are the checks a run must pass, every one, before it may
// write the data block.
var primerChecks = []string{"keys", "auditor", "registration"}

func TestProtocolPrimer(t *testing.T) {
	if !chainAvailable() {
		t.Skip("chain not reachable on :8545; start the stack with shield_demo/run.sh --no-demo")
	}
	if _, err := os.Stat("../build/receipts.json"); err != nil {
		t.Skip("no deployment (build/receipts.json); start the stack with shield_demo/run.sh --no-demo")
	}
	ctx := context.Background()
	client := mustDial(t)
	registryAddr := common.HexToAddress(loadOnchainReceipts(t)["UserRegistry"].ContractAddress)

	data := primerData{
		Run:   primerRun{ChainID: fmt.Sprint(hardhatChainID), UserRegistry: registryAddr.Hex()},
		Steps: map[string]primerEntry{},
	}
	// check runs one of primerChecks as a subtest and records it as passed
	// from inside it: t.Run also reports true for a subtest a -run filter
	// skipped.
	passed := map[string]bool{}
	t.Cleanup(func() { writePrimerDataIfPassed(t, data, passed) })
	check := func(name, desc string, f func(t *testing.T)) {
		t.Run(name+": "+desc, func(t *testing.T) {
			f(t)
			passed[name] = !t.Failed()
		})
	}

	// The story's actors, each with a fresh signing account funded for gas.
	users := []*primerUser{newPrimerUser(t, "Sender", "sender"), newPrimerUser(t, "Ana", "ana")}
	for _, u := range users {
		fundAccount(t, ctx, client, u.addr)
	}
	auditor, err := rpcore.NewAuditorKeyPair()
	if err != nil {
		t.Fatalf("NewAuditorKeyPair: %v", err)
	}

	check("keys", "pk_spend = Poseidon(sk_spend), and the view key is the ML-KEM-768 key pair of its seed", func(t *testing.T) {
		v := map[string]string{"p": constants.Q.String()}
		for _, u := range users {
			pk, err := poseidon.Hash([]*big.Int{u.spend.PrivateKey})
			if err != nil {
				t.Fatal(err)
			}
			if pk.Cmp(u.spend.PublicKey) != 0 {
				t.Errorf("%s: Poseidon(sk_spend) = %s, want pk_spend %s", u.name, pk, u.spend.PublicKey)
			}
			inField(t, u.name+"'s sk_spend", u.spend.PrivateKey)
			inField(t, u.name+"'s pk_spend", pk)
			seed := u.view.DecapsKey.Bytes()
			checkKEMSeed(t, u.name+"'s view key", seed, u.view.EncapsKey)
			v[u.id+".skSpend"] = u.spend.PrivateKey.String()
			v[u.id+".pkSpend"] = pk.String()
			v[u.id+".viewSeed"] = hexutil.Encode(seed)
			v[u.id+".viewEk"] = hexutil.Encode(u.view.EncapsKey)
		}
		data.Steps["keys"] = primerEntry{Values: v, Ledger: []ledgerItem{}}
	})

	check("auditor", "the auditor's key pair is the ML-KEM-768 key pair of its seed", func(t *testing.T) {
		seed := auditor.DecapsKey.Bytes()
		checkKEMSeed(t, "the auditor's key", seed, auditor.EncapsKey)
		data.Steps["auditor"] = primerEntry{Values: map[string]string{
			"auditor.seed": hexutil.Encode(seed),
			"auditor.ek":   hexutil.Encode(auditor.EncapsKey),
		}, Ledger: []ledgerItem{}}
	})

	check("registration", "the calldata publishes pk_spend, the view key and audit ciphertexts the auditor opens to the view key's seed", func(t *testing.T) {
		registryABI := loadOnchainABI(t, "UserRegistry")
		registered := registryABI.Events["UserRegistered"].ID
		v := map[string]string{"hkdfInfo": auditHKDFInfo}
		ledger := []ledgerItem{}
		for _, u := range users {
			mlKemCt, aesCt, err := rpcore.EncryptViewKeyForAuditor(auditor.EncapsKey, u.view.DecapsKey)
			if err != nil {
				t.Fatalf("%s: EncryptViewKeyForAuditor: %v", u.name, err)
			}
			auth, err := bind.NewKeyedTransactorWithChainID(u.key, big.NewInt(hardhatChainID))
			if err != nil {
				t.Fatal(err)
			}
			if err := rpcore.Register(client, auth, registryAddr, u.spend.PublicKey, u.view.EncapsKey, mlKemCt, aesCt); err != nil {
				t.Fatalf("%s: register: %v", u.name, err)
			}

			// The registration as the chain holds it: the UserRegistered
			// log names the user, and its transaction carries the calldata.
			logs, err := client.FilterLogs(ctx, ethereum.FilterQuery{Addresses: []common.Address{registryAddr},
				Topics: [][]common.Hash{{registered}, {common.BytesToHash(u.addr.Bytes())}}})
			if err != nil || len(logs) != 1 {
				t.Fatalf("%s: UserRegistered logs %d (err %v), want 1", u.name, len(logs), err)
			}
			tx, _, err := client.TransactionByHash(ctx, logs[0].TxHash)
			if err != nil {
				t.Fatal(err)
			}
			from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
			if err != nil {
				t.Fatal(err)
			}
			if from != u.addr || *tx.To() != registryAddr {
				t.Errorf("%s: registration sent by %s to %s, want %s to the UserRegistry", u.name, from.Hex(), tx.To().Hex(), u.addr.Hex())
			}
			method, err := registryABI.MethodById(tx.Data()[:4])
			if err != nil || method.Name != "register" {
				t.Fatalf("%s: registration calls %v (err %v), want register", u.name, method, err)
			}
			args, err := method.Inputs.Unpack(tx.Data()[4:])
			if err != nil {
				t.Fatal(err)
			}
			pkSpend, pkView, calldataCapsule, calldataAesCt := args[0].(*big.Int), args[1].([]byte), args[2].([]byte), args[3].([]byte)
			if pkSpend.Cmp(u.spend.PublicKey) != 0 || !bytes.Equal(pkView, u.view.EncapsKey) {
				t.Errorf("%s: the calldata's keys are not the user's", u.name)
			}
			if !bytes.Equal(calldataCapsule, mlKemCt) || !bytes.Equal(calldataAesCt, aesCt) {
				t.Errorf("%s: the calldata's audit ciphertexts are not the ones built", u.name)
			}
			keys, err := rpcore.LookupKeys(client, registryAddr, u.addr)
			if err != nil {
				t.Fatal(err)
			}
			if keys.SpendKey.Cmp(pkSpend) != 0 || !bytes.Equal(keys.ViewKey, pkView) {
				t.Errorf("%s: the UserRegistry's getKeys differs from the calldata", u.name)
			}
			heldKem, heldAes, err := rpcore.LookupAuditCiphertexts(client, registryAddr, u.addr)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(heldKem, calldataCapsule) || !bytes.Equal(heldAes, calldataAesCt) {
				t.Errorf("%s: the UserRegistry's getAuditCiphertexts differs from the calldata", u.name)
			}

			// The opening, recomputed as the auditor does it: Decaps, then
			// HKDF-SHA256, then AES-256-GCM with the capsule as AAD.
			if len(calldataCapsule) != mlkem.CiphertextSize768 {
				t.Errorf("%s: capsule is %d bytes, want %d", u.name, len(calldataCapsule), mlkem.CiphertextSize768)
			}
			ss, err := auditor.DecapsKey.Decapsulate(calldataCapsule)
			if err != nil {
				t.Fatalf("%s: Decaps: %v", u.name, err)
			}
			k, err := hkdf.Key(sha256.New, ss, nil, auditHKDFInfo, 32)
			if err != nil {
				t.Fatal(err)
			}
			gcm := newGCM(t, k)
			nonce, sealed := calldataAesCt[:gcm.NonceSize()], calldataAesCt[gcm.NonceSize():]
			opened, err := gcm.Open(nil, nonce, sealed, calldataCapsule)
			if err != nil {
				t.Fatalf("%s: AES-GCM open: %v", u.name, err)
			}
			if !bytes.Equal(opened, u.view.DecapsKey.Bytes()) {
				t.Errorf("%s: the audit ciphertexts open to %x, want the view key's seed", u.name, opened)
			}
			tampered := slices.Clone(calldataCapsule)
			tampered[0] ^= 1
			if _, err := gcm.Open(nil, nonce, sealed, tampered); err == nil {
				t.Errorf("%s: AES-GCM opened under a tampered capsule as AAD", u.name)
			}

			p := u.id + "."
			v[p+"address"] = u.addr.Hex()
			v[p+"tx"] = tx.Hash().Hex()
			v[p+"block"] = fmt.Sprint(logs[0].BlockNumber)
			v[p+"capsule"] = hexutil.Encode(calldataCapsule)
			v[p+"ss"] = hexutil.Encode(ss)
			v[p+"auditKey"] = hexutil.Encode(k)
			v[p+"nonce"] = hexutil.Encode(nonce)
			v[p+"sealed"] = hexutil.Encode(sealed)
			v[p+"aesCt"] = hexutil.Encode(calldataAesCt)
			v[p+"opened"] = hexutil.Encode(opened)
			label := "register · " + u.name
			ledger = append(ledger,
				ledgerItem{Tx: label, Field: "msg.sender", Value: from.Hex(), MsgSender: true},
				ledgerItem{Tx: label, Field: "pkSpend", Value: pkSpend.String()},
				ledgerItem{Tx: label, Field: "pkView", Value: hexutil.Encode(pkView)},
				ledgerItem{Tx: label, Field: "mlKemAuditCt", Value: hexutil.Encode(calldataCapsule)},
				ledgerItem{Tx: label, Field: "aesAuditCt", Value: hexutil.Encode(calldataAesCt)})
		}
		data.Steps["registration"] = primerEntry{Values: v, Ledger: ledger}
	})
}

// newPrimerUser makes a user with a fresh signing account and fresh keys.
func newPrimerUser(t *testing.T, name, id string) *primerUser {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	spend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	view, err := rpcore.NewViewKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	return &primerUser{name: name, id: id, key: key, addr: crypto.PubkeyToAddress(key.PublicKey), spend: spend, view: view}
}

// fundAccount sends addr 1 ETH of gas from Hardhat's account 0.
func fundAccount(t *testing.T, ctx context.Context, client *ethclient.Client, addr common.Address) {
	t.Helper()
	key, err := crypto.HexToECDSA(hardhatPrivKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	from := crypto.PubkeyToAddress(key.PublicKey)
	nonce, err := client.PendingNonceAt(ctx, from)
	if err != nil {
		t.Fatal(err)
	}
	price, err := client.SuggestGasPrice(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := types.SignTx(types.NewTx(&types.LegacyTx{Nonce: nonce, To: &addr, Value: big.NewInt(1e18), Gas: 21000, GasPrice: price}),
		types.LatestSignerForChainID(big.NewInt(hardhatChainID)), key)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SendTransaction(ctx, tx); err != nil {
		t.Fatalf("fund %s: %v", addr.Hex(), err)
	}
	if r, err := bind.WaitMined(ctx, client, tx); err != nil || r.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("fund %s: receipt %v, err %v", addr.Hex(), r, err)
	}
}

// inField fails unless 0 ≤ x < p, BN254's scalar field modulus.
func inField(t *testing.T, what string, x *big.Int) {
	t.Helper()
	if x.Sign() < 0 || x.Cmp(constants.Q) >= 0 {
		t.Errorf("%s = %s is not in BN254's scalar field", what, x)
	}
}

// checkKEMSeed fails unless seed is a 64-byte ML-KEM-768 seed whose key pair
// has encapsulation key ek (1184 bytes).
func checkKEMSeed(t *testing.T, what string, seed, ek []byte) {
	t.Helper()
	if len(seed) != mlkem.SeedSize || len(ek) != mlkem.EncapsulationKeySize768 {
		t.Errorf("%s: seed %d bytes, ek %d bytes, want %d and %d", what, len(seed), len(ek), mlkem.SeedSize, mlkem.EncapsulationKeySize768)
	}
	dk, err := mlkem.NewDecapsulationKey768(seed)
	if err != nil {
		t.Fatalf("%s: NewDecapsulationKey768: %v", what, err)
	}
	if !bytes.Equal(dk.EncapsulationKey().Bytes(), ek) {
		t.Errorf("%s: the seed's encapsulation key is not the published one", what)
	}
}

func newGCM(t *testing.T, key []byte) cipher.AEAD {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	return gcm
}

// primerBlock is the primer page's data block, its JSON in group 2.
var primerBlock = regexp.MustCompile(`(?s)(<script type="application/json" id="primer-data">\n)(.*?)(\n</script>)`)

// writePrimerDataIfPassed rewrites the primer page's data block with data,
// once t has passed and every one of primerChecks has run and passed; a
// filtered or failed run leaves the page as it was.
func writePrimerDataIfPassed(t *testing.T, data primerData, passed map[string]bool) {
	t.Helper()
	if t.Failed() {
		return
	}
	for _, c := range primerChecks {
		if !passed[c] {
			t.Logf("primer data not written: the %q check did not run (a filtered run); refresh it with an unfiltered run", c)
			return
		}
	}
	page, err := os.ReadFile(primerPage)
	if err != nil {
		t.Errorf("read the primer: %v", err)
		return
	}
	if !primerBlock.Match(page) {
		t.Errorf("%s has no data block (<script type=\"application/json\" id=\"primer-data\">)", primerPage)
		return
	}
	block, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		t.Errorf("marshal the primer data: %v", err)
		return
	}
	out := primerBlock.ReplaceAllFunc(page, func(m []byte) []byte {
		g := primerBlock.FindSubmatch(m)
		return slices.Concat(g[1], block, g[3])
	})
	if err := os.WriteFile(primerPage, out, 0o644); err != nil {
		t.Errorf("write the primer: %v", err)
	}
}
