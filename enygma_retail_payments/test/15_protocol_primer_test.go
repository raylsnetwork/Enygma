package tests

// TestProtocolPrimer runs the Protocol primer's story against the live
// stack with fresh users (the Sender and Ana) and a fresh auditor, using the
// client library, and recomputes every equation the primer shows from the
// run's values and the chain. Only with PRIMER_WRITE=1 (make primer-refresh
// sets it), and when every check has run and held, does it rewrite the JSON
// data block inside the primer page (shield_demo/protocol.html), which is
// those values' only copy; any other run, the retail suite's included,
// writes nothing.
//
// The steps so far: keys (the Sender's and Ana's spend and view keys), the
// auditor's keys, the registration of both users, the Sender's shield, the
// Sender's tag channel to Ana (and a second to a throwaway view key, which
// Ana's key fails to open), and the Sender's payment to Ana (one Payment
// proof, sent from the Sender's own account), its Groth16 proof opened up (L and the pairing check, and the
// Verifier's answer), the payment's tag (published by the Sender in the block
// after settlement, on the tag channel to Ana), Ana's discovery of her note (her scan of the Payment
// events), her partial withdrawal from that note (the same Payment circuit
// with Message = 1), the auditor's recovery of her view key and opening of
// her notes, and the observer's verdict (what the chain's bytes name).
//
// Prerequisites: the stack shield_demo/run.sh --no-demo brings up (the
// chain and a deployment, with its TagChannelRegistry, and the gnark server
// on :8082 for the payment's proof; this test does not call the relayer).
//
// Run:
//
//	cd test && CC=/usr/bin/clang go test -run 'TestProtocolPrimer$' -v
//
// (with PRIMER_WRITE=1 to rewrite the data block; make primer-refresh is the
// way to make it)

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/hkdf"
	"crypto/mlkem"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"maps"
	"math/big"
	"math/bits"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"enygma_retail_payments/layout"

	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/fft"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iden3/go-iden3-crypto/constants"
	"github.com/iden3/go-iden3-crypto/poseidon"
	dvpcore "github.com/raylsnetwork/enygma_dvp/src/core"
	tags "github.com/raylsnetwork/enygma_retail_payments/private_tags/src"
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
	Vault        string `json:"vault"`
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
var primerChecks = []string{"keys", "auditor", "registration", "shield", "channel", "payment", "proof", "tag", "discovery", "withdrawal", "audit", "verdict"}

// shieldAmount is what the Sender shields: public RaylsERC20 the issuer
// mints to them, deposited whole into one note.
const shieldAmount = 100

// primerDepth is the vault's Merkle tree depth (scripts/cmd/init).
const primerDepth = 8

// payAmount is what the Sender pays Ana out of the shielded note; the rest
// is the Sender's change.
const payAmount = 60

// withdrawAmount is what Ana takes out of the pool from her note of
// payAmount; the rest stays hers, as a change note.
const withdrawAmount = 25

// paymentRange is the Payment circuit's exclusive bound on every value,
// 10^36 (gnark_circuits/templates/deployed.go).
var paymentRange = new(big.Int).Exp(big.NewInt(10), big.NewInt(36), nil)

// paymentPublic names the Payment statement's public inputs, each at its
// layout offset, in order; the primer's ledger lists them under these names.
var paymentPublic = []struct {
	name   string
	offset int
}{
	{"Message", layout.PaymentMessageOffset},
	{"TreeNumber", layout.PaymentTreeNumbersOffset},
	{"MerkleRoot", layout.PaymentMerkleRootsOffset},
	{"Nullifier", layout.PaymentNullifiersOffset},
	{"CommitmentOut[0]", layout.PaymentCommitmentsOutOffset},
	{"CommitmentOut[1]", layout.PaymentCommitmentsOutOffset + 1},
	{"ContractAddress", layout.PaymentContractAddressOffset},
}

// shieldedNote is the opening of the Sender's shielded note.
type shieldedNote struct{ commitment, salt, amount, block *big.Int }

// provedPayment is the payment's proof and public inputs, as its calldata
// carried them, and the block that settled it.
type provedPayment struct {
	proof     SnarkProof
	statement []*big.Int
	block     *big.Int
}

// paidNote is Ana's note as the payment made it: its commitment, the
// Sender's view of its opening, and the block that settled it.
type paidNote struct {
	commitment, salt, amount, token *big.Int
	block                           *big.Int
	capsule, payload                []byte // the Payment event's ciphertexts, once the note is discovered
}

// exitedNote is Ana's withdrawal as it settled: what left the pool, her
// change, and the ciphertexts published for it.
type exitedNote struct {
	amount, change, commitment, nullifier *big.Int
	ctI, ctII                             []byte
	tx                                    common.Hash
	block                                 *big.Int
}

// tagChannel is the Sender's tag channel to Ana: the Sender's ss, Ana's
// Decaps of its record, the record's index and the block that opened it.
type tagChannel struct {
	ss, anaSS  []byte
	idx, block uint64
}

// noteOpening is one trial decryption of a note's ciphertexts: the shared
// secret, the two HKDF outputs and the payload's parts. ok is false when
// the AES-GCM tag does not check.
type noteOpening struct {
	ss, saltBytes, key, nonce, sealed, plain []byte
	ok                                       bool
}

func TestProtocolPrimer(t *testing.T) {
	if !chainAvailable() {
		t.Skip("chain not reachable on :8545; start the stack with shield_demo/run.sh --no-demo")
	}
	if _, err := os.Stat("../build/receipts.json"); err != nil {
		t.Skip("no deployment (build/receipts.json); start the stack with shield_demo/run.sh --no-demo")
	}
	ctx := context.Background()
	client := mustDial(t)
	receipts := loadOnchainReceipts(t)
	registryAddr := common.HexToAddress(receipts["UserRegistry"].ContractAddress)
	vaultAddr := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	erc20Addr := common.HexToAddress(receipts["ERC20"].ContractAddress)

	data := primerData{
		Run:   primerRun{ChainID: fmt.Sprint(hardhatChainID), UserRegistry: registryAddr.Hex(), Vault: vaultAddr.Hex()},
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
			tx, from, args := calldataOf(t, client, registryABI, logs[0].TxHash, "register", u.addr, registryAddr)
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

	// shielded is the note the shield made, which the payment spends; paid
	// is Ana's note, which the payment made and her discovery finds.
	var shielded *shieldedNote
	var paid *paidNote
	// proved is the payment's proof, which the proof check verifies.
	var proved *provedPayment
	// discovered is Ana's note as her scan opened it, which she withdraws from.
	var discovered *paidNote
	// exit is Ana's settled withdrawal, whose change note the audit opens.
	var exit *exitedNote
	// toAnaChannel is the Sender's tag channel to Ana, which the payment's tag rides on.
	var toAnaChannel *tagChannel
	// tagBlock is the block the payment's tag landed in, which Ana's tag
	// scan reaches.
	var tagBlock uint64

	check("shield", "the vault computes Poseidon(pk_spend, salt, amount, token) from the deposit's calldata and inserts it as a leaf of its depth-8 tree", func(t *testing.T) {
		sender := users[0]
		vaultABI := loadOnchainABI(t, "Erc20CoinVault")
		vault := bindContract(t, client, vaultAddr, vaultABI)
		erc20 := bindContract(t, client, erc20Addr, loadOnchainABI(t, "RaylsERC20"))
		amount, token := big.NewInt(shieldAmount), big.NewInt(0)

		// The issuer mints the Sender public RaylsERC20, which the Sender
		// approves to the vault and deposits whole.
		auth, err := bind.NewKeyedTransactorWithChainID(sender.key, big.NewInt(hardhatChainID))
		if err != nil {
			t.Fatal(err)
		}
		// mine(what)(tx, err) waits for a sent transaction to succeed.
		mine := func(what string) func(*types.Transaction, error) *types.Receipt {
			return func(tx *types.Transaction, err error) *types.Receipt {
				t.Helper()
				if err != nil {
					t.Fatalf("%s: %v", what, err)
				}
				r, err := bind.WaitMined(ctx, client, tx)
				if err != nil || r.Status != types.ReceiptStatusSuccessful {
					t.Fatalf("%s: receipt %v, err %v", what, r, err)
				}
				return r
			}
		}
		mine("mint")(erc20.Transact(hardhatAuth(t, client), "mint", sender.addr, amount))
		mine("approve")(erc20.Transact(auth, "approve", vaultAddr, amount))

		// The note's salt comes from an ML-KEM secret shared with its owner
		// (here the Sender's own view key), so its owner can derive it again.
		ss, capsule, err := rpcore.Encapsulate(sender.view.EncapsKey)
		if err != nil {
			t.Fatal(err)
		}
		saltB, err := rpcore.DerivePaymentSalt(ss)
		if err != nil {
			t.Fatal(err)
		}
		encKey, err := rpcore.DerivePaymentKey(ss)
		if err != nil {
			t.Fatal(err)
		}
		salt := rpcore.SaltBToField(saltB)
		sealed, err := rpcore.EncryptPayload(encKey, token, amount)
		if err != nil {
			t.Fatal(err)
		}
		deposit := mine("depositV2")(vault.Transact(auth, "depositV2", []*big.Int{amount, sender.spend.PublicKey, salt, token}, capsule, sealed))
		block := deposit.BlockNumber

		// The deposit as the chain holds it: its calldata is the opening.
		_, from, args := calldataOf(t, client, vaultABI, deposit.TxHash, "depositV2", sender.addr, vaultAddr)
		params, ctI, ctII := args[0].([]*big.Int), args[1].([]byte), args[2].([]byte)
		if len(params) != 4 {
			t.Fatalf("depositV2 params %v, want amount, pkSpend, salt, tokenId", params)
		}
		cdAmount, cdPk, cdSalt, cdToken := params[0], params[1], params[2], params[3]
		if cdAmount.Cmp(amount) != 0 || cdPk.Cmp(sender.spend.PublicKey) != 0 || cdSalt.Cmp(salt) != 0 || cdToken.Cmp(token) != 0 {
			t.Errorf("calldata opening (%s, %s, %s, %s), want the Sender's", cdAmount, cdPk, cdSalt, cdToken)
		}
		if !bytes.Equal(ctI, capsule) || !bytes.Equal(ctII, sealed) {
			t.Error("the calldata's ciphertexts are not the ones built")
		}
		for _, x := range params {
			inField(t, "a calldata value", x)
		}
		// The salt is HKDF-SHA256(ss, "note salt") reduced into the field,
		// recomputed here with the standard library.
		hk, err := hkdf.Key(sha256.New, ss, nil, "note salt", 32)
		if err != nil {
			t.Fatal(err)
		}
		if toField(hk).Cmp(cdSalt) != 0 {
			t.Error("salt is not HKDF-SHA256(ss, \"note salt\") mod p")
		}

		// The commitment, recomputed with the client library's Poseidon from
		// the public calldata, is the leaf the vault's Commitment event
		// logged: the vault computed it from the same four values.
		commitment, err := poseidon.Hash([]*big.Int{cdPk, cdSalt, cdAmount, cdToken})
		if err != nil {
			t.Fatal(err)
		}
		if lib, err := rpcore.Erc20CommitmentV2(cdPk, cdSalt, cdAmount, cdToken); err != nil || lib.Cmp(commitment) != 0 {
			t.Errorf("Erc20CommitmentV2 = %v (err %v), want Poseidon(pk_spend, salt, amount, token) %s", lib, err, commitment)
		}
		logged := commitmentsIn(deposit.Logs, vaultAddr)
		if len(logged) != 1 || logged[0].Cmp(commitment) != 0 {
			t.Fatalf("the deposit's Commitment events logged %v, want one, %s", logged, commitment)
		}
		// A guessed amount, checked against the commitment, needs the salt:
		// with any other salt the same amount hashes elsewhere.
		if other, _ := poseidon.Hash([]*big.Int{cdPk, new(big.Int).Add(cdSalt, big.NewInt(1)), cdAmount, cdToken}); other.Cmp(commitment) == 0 {
			t.Error("another salt hashes the same opening to the same commitment")
		}

		// The public RaylsERC20 moved into the vault.
		before, at := new(big.Int).Sub(block, big.NewInt(1)), block
		// bal is of's RaylsERC20 balance at block n.
		bal := func(of common.Address, n *big.Int) *big.Int {
			var out []any
			if err := erc20.Call(&bind.CallOpts{BlockNumber: n}, &out, "balanceOf", of); err != nil {
				t.Fatal(err)
			}
			return out[0].(*big.Int)
		}
		if got := new(big.Int).Sub(bal(vaultAddr, at), bal(vaultAddr, before)); got.Cmp(amount) != 0 || bal(sender.addr, at).Sign() != 0 {
			t.Errorf("the vault gained %s and the Sender holds %s, want %s and 0", got, bal(sender.addr, at), amount)
		}

		// vcall calls a one-value vault view at the deposit's block.
		vcall := func(method string, args ...any) any { t.Helper(); return vaultView(t, vault, at, method, args...) }
		// The tree, rebuilt from the vault's Commitment events up to the deposit.
		mt := rebuildVaultTree(t, client, vaultAddr, at)
		proof, err := mt.GenerateProof(commitment)
		if err != nil {
			t.Fatalf("the commitment is not in the rebuilt tree: %v", err)
		}
		tree := vcall("treeNumber").(*big.Int)
		if proof.TreeNumber != mt.TreeNumber() || int64(proof.TreeNumber) != tree.Int64() {
			t.Fatalf("leaf in rebuilt tree %d (current %d), the vault's tree is %s", proof.TreeNumber, mt.TreeNumber(), tree)
		}
		leaves := len(mt.GetLeaves())
		index := proof.Indices.Int64()
		if index != int64(leaves-1) || leaves >= 1<<primerDepth {
			t.Errorf("leaf #%d of %d, want the last of fewer than 2^%d", index, leaves, primerDepth)
		}
		if root := vcall("merkleRoot").(*big.Int); root.Cmp(proof.Root) != 0 {
			t.Errorf("the vault's merkleRoot %s, the rebuilt tree's %s", root, proof.Root)
		}
		if !vcall("isValidRoot", tree, proof.Root).(bool) {
			t.Errorf("isValidRoot(%s, %s) = false", tree, proof.Root)
		}
		zero := toField(crypto.Keccak256([]byte("ZkDvp")))
		if z := vcall("ZERO_VALUE").(*big.Int); z.Cmp(zero) != 0 {
			t.Errorf("the vault's ZERO_VALUE %s, keccak256(\"ZkDvp\") mod p is %s", z, zero)
		}

		// Poseidon's deployed parameters, as the client library holds them:
		// width t = inputs + 1, 8 full rounds, the partial rounds for t, and
		// the S-box x^5, a permutation of the field since gcd(5, p-1) = 1.
		pm1 := new(big.Int).Sub(constants.Q, big.NewInt(1))
		if new(big.Int).GCD(nil, nil, big.NewInt(5), pm1).Cmp(big.NewInt(1)) != 0 {
			t.Error("gcd(5, p-1) != 1: x^5 does not permute the field")
		}
		if poseidon.NROUNDSF != 8 {
			t.Errorf("Poseidon full rounds %d, want 8", poseidon.NROUNDSF)
		}
		// The partial rounds for widths 2, 3 and 5, as the circuit's Poseidon
		// (enygma_gnark_shared) and circomlib's on-chain contract use them.
		for t2, want := range map[int]int{2: 56, 3: 57, 5: 60} {
			if got := poseidon.NROUNDSP[t2-2]; got != want {
				t.Errorf("Poseidon partial rounds for t = %d: %d, want %d", t2, got, want)
			}
		}

		v := map[string]string{
			"p":          constants.Q.String(),
			"pBits":      fmt.Sprint(constants.Q.BitLen()),
			"msgSender":  from.Hex(),
			"tx":         deposit.TxHash.Hex(),
			"block":      block.String(),
			"vault":      vaultAddr.Hex(),
			"amount":     cdAmount.String(),
			"pkSpend":    cdPk.String(),
			"salt":       cdSalt.String(),
			"token":      cdToken.String(),
			"commitment": commitment.String(),
			"depth":      fmt.Sprint(primerDepth),
			"tree":       tree.String(),
			"leafIndex":  fmt.Sprint(index),
			"leaves":     fmt.Sprint(leaves),
			"root":       proof.Root.String(),
			"zero":       zero.String(),
			"rf":         fmt.Sprint(poseidon.NROUNDSF),
			// rpT: the partial rounds of width t, NROUNDSP[t-2].
			"rp2": fmt.Sprint(poseidon.NROUNDSP[0]),
			"rp3": fmt.Sprint(poseidon.NROUNDSP[1]),
			"rp5": fmt.Sprint(poseidon.NROUNDSP[3]),
		}
		shielded = &shieldedNote{commitment: commitment, salt: cdSalt, amount: cdAmount, block: block}
		label := "depositV2 · Sender"
		data.Steps["shield"] = primerEntry{Values: v, Ledger: []ledgerItem{
			{Tx: label, Field: "msg.sender", Value: from.Hex(), MsgSender: true},
			{Tx: label, Field: "amount", Value: cdAmount.String()},
			{Tx: label, Field: "pkSpend", Value: cdPk.String()},
			{Tx: label, Field: "salt", Value: cdSalt.String()},
			{Tx: label, Field: "tokenId", Value: cdToken.String()},
			{Tx: label, Field: "ciphertextI", Value: hexutil.Encode(ctI)},
			{Tx: label, Field: "ciphertextII", Value: hexutil.Encode(ctII)},
			{Tx: "Commitment event", Field: "commitment", Value: commitment.String()},
			{Tx: "Commitment event", Field: "leaf", Value: fmt.Sprintf("tree %s · leaf #%d", tree, index)},
		}}
	})

	check("channel", "the Sender's tag channel record opens to Ana's view key alone: her Decaps gives ss, HKDF k_ch, and c₂ opens with c₁ as AAD to msg ‖ the Sender's address; a record sealed to a throwaway view key decapsulates for her too, and its AEAD fails", func(t *testing.T) {
		sender, ana := users[0], users[1]
		channelsAddr := common.HexToAddress(receipts["TagChannelRegistry"].ContractAddress)
		channelsABI := tagChannelRegistryABI(t)
		channels := bindContract(t, client, channelsAddr, channelsABI)
		auth, err := bind.NewKeyedTransactorWithChainID(sender.key, big.NewInt(hardhatChainID))
		if err != nil {
			t.Fatal(err)
		}
		nUsers, err := rpcore.GetUserCount(client, registryAddr)
		if err != nil {
			t.Fatal(err)
		}
		anaIdx, err := tags.GetRecipientIndex(client, registryAddr, ana.addr)
		if err != nil {
			t.Fatal(err)
		}
		msg, senderID := "Payment channel ready", sender.addr.Hex()

		// opened is one tag channel the Sender opened: what it built, the
		// record's index and the openChannel transaction's receipt.
		type opened struct {
			ss, c1, c2, bitmap []byte
			idx                uint64
			receipt            *types.Receipt
		}
		// open has the Sender build a tag channel to ek with the client
		// library and send openChannel from their own account (no relayer).
		open := func(what string, ek []byte, idx int) opened {
			t.Helper()
			ss, c1, c2, bitmap, err := tags.PrepareChannelSetup(ek, []byte(msg), []byte(senderID), tags.PrivacyFull, nUsers, idx, nil)
			if err != nil {
				t.Fatalf("%s: PrepareChannelSetup: %v", what, err)
			}
			tx, err := channels.Transact(auth, "openChannel", c1, c2, bitmap)
			if err != nil {
				t.Fatalf("%s: openChannel: %v", what, err)
			}
			r, err := bind.WaitMined(ctx, client, tx)
			if err != nil || r.Status != types.ReceiptStatusSuccessful {
				t.Fatalf("%s: openChannel receipt %v, err %v", what, r, err)
			}
			var events []*types.Log
			for _, l := range r.Logs {
				if l.Address == channelsAddr && len(l.Topics) == 3 && l.Topics[0] == channelOpenedSig {
					events = append(events, l)
				}
			}
			if len(events) != 1 || common.BytesToAddress(events[0].Topics[2].Bytes()) != sender.addr {
				t.Fatalf("%s: %d ChannelOpened events naming the Sender, want 1", what, len(events))
			}
			channelIdx := events[0].Topics[1].Big().Uint64()
			// The record as TagChannelRegistry holds it is what was built,
			// and its sender is the Sender's account.
			var out []any
			if err := channels.Call(&bind.CallOpts{}, &out, "getChannel", new(big.Int).SetUint64(channelIdx)); err != nil {
				t.Fatal(err)
			}
			if out[0].(common.Address) != sender.addr || !bytes.Equal(out[1].([]byte), c1) || !bytes.Equal(out[2].([]byte), c2) || !bytes.Equal(out[3].([]byte), bitmap) {
				t.Fatalf("%s: tag channel #%d is not the record built, sent by the Sender", what, channelIdx)
			}
			if _, from, _ := calldataOf(t, client, channelsABI, r.TxHash, "openChannel", sender.addr, channelsAddr); from != sender.addr {
				t.Fatalf("%s: openChannel sent by %s", what, from)
			}
			return opened{ss, c1, c2, bitmap, channelIdx, r}
		}
		toAna := open("Sender → Ana", ana.view.EncapsKey, anaIdx)
		ss, c1, c2, bitmap, idx := toAna.ss, toAna.c1, toAna.c2, toAna.bitmap, toAna.idx

		// Recomputed with the standard library, as Ana's wallet does it:
		// Decaps of c₁, HKDF-SHA256 to k_ch, AES-256-GCM open of c₂ with c₁
		// as associated data, decoded to msg ‖ senderId.
		if len(c1) != mlkem.CiphertextSize768 {
			t.Errorf("c₁ is %d bytes, want %d", len(c1), mlkem.CiphertextSize768)
		}
		anaSS, err := ana.view.DecapsKey.Decapsulate(c1)
		if err != nil {
			t.Fatalf("Ana's Decaps: %v", err)
		}
		if !bytes.Equal(anaSS, ss) {
			t.Fatal("Ana's Decaps of c₁ is not the Sender's ss")
		}
		kch, err := hkdf.Key(sha256.New, ss, nil, "enygma-channel-key-v1", 32)
		if err != nil {
			t.Fatal(err)
		}
		if lib := tags.DeriveChannelKey(ss); !bytes.Equal(lib[:], kch) {
			t.Error("the library's channel key is not HKDF-SHA256(ss, \"enygma-channel-key-v1\")")
		}
		gotMsg, gotID, err := openChannelSealed(t, kch, c1, c2)
		if err != nil {
			t.Fatalf("c₂ does not open under k_ch with c₁ as AAD: %v", err)
		}
		if gotMsg != msg || gotID != senderID {
			t.Errorf("c₂ opens to %q ‖ %q, want %q ‖ the Sender's address %s", gotMsg, gotID, msg, senderID)
		}
		// The plaintext, as the library encodes it: each part a 4-byte
		// big-endian length, then its bytes.
		var plain []byte
		for _, part := range []string{msg, senderID} {
			plain = append(binary.BigEndian.AppendUint32(plain, uint32(len(part))), part...)
		}
		gcm := newGCM(t, kch)
		nonce, sealed := c2[:gcm.NonceSize()], c2[gcm.NonceSize():]
		if got, err := gcm.Open(nil, nonce, sealed, c1); err != nil || !bytes.Equal(got, plain) {
			t.Errorf("c₂ opens to %x (err %v), want msg ‖ senderId as %x", got, err, plain)
		}
		tampered := slices.Clone(c1)
		tampered[0] ^= 1
		if _, err := gcm.Open(nil, nonce, sealed, tampered); err == nil {
			t.Error("c₂ opened beside a tampered c₁")
		}
		// The bitmap is PrivacyFull: one bit per registered user, all set.
		set := 0
		for i := range len(bitmap) * 8 {
			if bitmap[i/8]&(1<<(i%8)) != 0 {
				if i >= nUsers {
					t.Errorf("the bitmap sets bit %d, past the registry's %d users", i, nUsers)
				}
				set++
			}
		}
		if set != nUsers || len(bitmap) != (nUsers+7)/8 {
			t.Errorf("the bitmap sets %d bits in %d bytes, the UserRegistry holds %d users", set, len(bitmap), nUsers)
		}

		// A second record, sealed to a throwaway view key: Ana's Decaps
		// still returns a secret (ML-KEM rejects implicitly), and the AEAD
		// under its key gives ⊥. The throwaway key's Decaps and AEAD open
		// it, so it is a well-formed record that is simply not hers.
		throwaway, err := mlkem.GenerateKey768()
		if err != nil {
			t.Fatal(err)
		}
		toOther := open("Sender → throwaway", throwaway.EncapsulationKey().Bytes(), anaIdx)
		otherSS, otherC1, otherC2 := toOther.ss, toOther.c1, toOther.c2
		anaOther, err := ana.view.DecapsKey.Decapsulate(otherC1)
		if err != nil {
			t.Fatalf("Ana's Decaps of the throwaway record: %v", err)
		}
		if bytes.Equal(anaOther, otherSS) || len(anaOther) != 32 {
			t.Errorf("Ana's Decaps of the throwaway record gives %x, want a 32-byte secret other than its ss", anaOther)
		}
		otherK, err := hkdf.Key(sha256.New, anaOther, nil, "enygma-channel-key-v1", 32)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := openChannelSealed(t, otherK, otherC1, otherC2); err == nil {
			t.Error("the throwaway record's c₂ opened under Ana's key")
		}
		own, err := throwaway.Decapsulate(otherC1)
		if err != nil || !bytes.Equal(own, otherSS) {
			t.Fatal("the throwaway key's Decaps does not give its record's ss")
		}
		ownK, err := hkdf.Key(sha256.New, own, nil, "enygma-channel-key-v1", 32)
		if err != nil {
			t.Fatal(err)
		}
		if gotMsg, gotID, err := openChannelSealed(t, ownK, otherC1, otherC2); err != nil || gotMsg != msg || gotID != senderID {
			t.Errorf("the throwaway key does not open its own record's c₂ (%q ‖ %q, err %v)", gotMsg, gotID, err)
		}

		// Ana's scan with the client library: of every record, hers alone opens.
		count, err := tags.GetChannelCount(client, channelsAddr)
		if err != nil {
			t.Fatal(err)
		}
		found, err := tags.ScanChannels(client, channelsAddr, ana.view.DecapsKey, 0, count)
		if err != nil {
			t.Fatal(err)
		}
		if len(found) != 1 || found[0].ChannelIdx != idx || !bytes.Equal(found[0].SharedSecret, ss) || string(found[0].SenderId) != senderID {
			t.Fatalf("Ana's scan of %d records found %d channels, want tag channel #%d from the Sender alone", count, len(found), idx)
		}

		v := map[string]string{
			"registry":   channelsAddr.Hex(),
			"anaEk":      hexutil.Encode(ana.view.EncapsKey),
			"ss":         hexutil.Encode(ss),
			"c1":         hexutil.Encode(c1),
			"kch":        hexutil.Encode(kch),
			"msg":        msg,
			"senderId":   senderID,
			"plaintext":  hexutil.Encode(plain),
			"nonce":      hexutil.Encode(nonce),
			"sealed":     hexutil.Encode(sealed),
			"c2":         hexutil.Encode(c2),
			"bitmap":     hexutil.Encode(bitmap),
			"users":      fmt.Sprint(nUsers),
			"channelIdx": fmt.Sprint(idx),
			"msgSender":  sender.addr.Hex(),
			"tx":         toAna.receipt.TxHash.Hex(),
			"block":      toAna.receipt.BlockNumber.String(),
			"anaSs":      hexutil.Encode(anaSS),
			"otherIdx":   fmt.Sprint(toOther.idx),
			"otherC1":    hexutil.Encode(otherC1),
			"otherSs":    hexutil.Encode(anaOther),
			"otherKey":   hexutil.Encode(otherK),
			"otherOpen":  "⊥",
			"scanned":    fmt.Sprint(count),
			"found":      fmt.Sprint(len(found)),
		}
		ledger := []ledgerItem{}
		for _, r := range []struct {
			label string
			o     opened
		}{{"openChannel · Sender", toAna}, {"openChannel · Sender, throwaway", toOther}} {
			ledger = append(ledger,
				ledgerItem{Tx: r.label, Field: "msg.sender", Value: sender.addr.Hex(), MsgSender: true},
				ledgerItem{Tx: r.label, Field: "c1", Value: hexutil.Encode(r.o.c1)},
				ledgerItem{Tx: r.label, Field: "c2", Value: hexutil.Encode(r.o.c2)},
				ledgerItem{Tx: r.label, Field: "bitmap", Value: hexutil.Encode(r.o.bitmap)},
				ledgerItem{Tx: "ChannelOpened event", Field: "channelIdx", Value: fmt.Sprint(r.o.idx)},
				ledgerItem{Tx: "ChannelOpened event", Field: "sender", Value: sender.addr.Hex()})
		}
		data.Steps["channel"] = primerEntry{Values: v, Ledger: ledger}
		toAnaChannel = &tagChannel{ss: ss, anaSS: anaSS, idx: idx, block: toAna.receipt.BlockNumber.Uint64()}
	})

	check("payment", "one Payment proof spends the Sender's note into Ana's and the change, and every constraint holds on the run's values and the chain", func(t *testing.T) {
		if shielded == nil {
			t.Fatal("no shielded note to spend: the shield check did not pass")
		}
		if !serverAvailable("localhost:8082") {
			t.Fatal("the gnark server is not running on :8082 (shield_demo/run.sh --no-demo starts it)")
		}
		sender, ana := users[0], users[1]
		vaultABI := loadOnchainABI(t, "Erc20CoinVault")
		vault := bindContract(t, client, vaultAddr, vaultABI)
		dvpABI := loadOnchainABI(t, "EnygmaDvp")
		dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)
		token, vaultID := big.NewInt(0), big.NewInt(0)
		valueIn := shielded.amount
		valuesOut := []*big.Int{big.NewInt(payAmount), new(big.Int).Sub(valueIn, big.NewInt(payAmount))}
		sk := sender.spend.PrivateKey

		// The Sender finds Ana's keys in the registry, as any payer would.
		anaKeys, err := rpcore.LookupKeys(client, registryAddr, ana.addr)
		if err != nil {
			t.Fatal(err)
		}
		if anaKeys.SpendKey.Cmp(ana.spend.PublicKey) != 0 || !bytes.Equal(anaKeys.ViewKey, ana.view.EncapsKey) {
			t.Fatal("the registry's keys for Ana are not hers")
		}
		senderKeys, err := rpcore.LookupKeys(client, registryAddr, sender.addr)
		if err != nil {
			t.Fatal(err)
		}

		// The Sender's Merkle path, from the tree rebuilt from the vault's
		// Commitment events.
		head, err := client.BlockNumber(ctx)
		if err != nil {
			t.Fatal(err)
		}
		mp, err := rebuildVaultTree(t, client, vaultAddr, new(big.Int).SetUint64(head)).GenerateProof(shielded.commitment)
		if err != nil {
			t.Fatalf("the shielded note is not in the vault's tree: %v", err)
		}
		tree := big.NewInt(int64(mp.TreeNumber))

		// The proof, from the gnark server, bound to this vault.
		res, err := rpcore.NewPaymentClient("").BoundPaymentProof(
			new(big.Int).SetBytes(vaultAddr.Bytes()), big.NewInt(0),
			[]*big.Int{valueIn}, []rpcore.KeyPair{{PrivateKey: sk, PublicKey: sender.spend.PublicKey}}, []*big.Int{shielded.salt},
			valuesOut, []*big.Int{anaKeys.SpendKey, senderKeys.SpendKey}, [][]byte{anaKeys.ViewKey, sender.view.EncapsKey},
			primerDepth, []*rpcore.MerkleProof{mp}, []*big.Int{tree}, token)
		if err != nil {
			t.Fatalf("BoundPaymentProof: %v", err)
		}
		pr := proofStringsToOnchain(t, res.Proof)
		receipt := ProofReceipt{
			Proof:     SnarkProof{A: G1Point{X: pr.A.X, Y: pr.A.Y}, B: G2Point{X: pr.B.X, Y: pr.B.Y}, C: G1Point{X: pr.C.X, Y: pr.C.Y}},
			Statement: res.ContractStatement(), NumberOfInputs: big.NewInt(1), NumberOfOutputs: big.NewInt(2),
		}

		// The Sender sends it from their own account: no relayer.
		auth, err := bind.NewKeyedTransactorWithChainID(sender.key, big.NewInt(hardhatChainID))
		if err != nil {
			t.Fatal(err)
		}
		mined, err := submitPayment(client, auth, dvpABI, dvpAddr, receipt, vaultID, res.CipherText, res.EncTxData)
		if err != nil || mined.Status != types.ReceiptStatusSuccessful {
			t.Fatalf("payment: receipt %v, err %v", mined, err)
		}
		block := mined.BlockNumber

		// The payment as the chain holds it: its calldata's statement is the
		// proof's public inputs, and nothing else of the witness.
		_, from, args := calldataOf(t, client, dvpABI, mined.TxHash, "payment", sender.addr, dvpAddr)
		cd := *abi.ConvertType(args[0], new(ProofReceipt)).(*ProofReceipt)
		st := cd.Statement
		if len(st) != layout.PaymentLen || len(st) != len(paymentPublic) {
			t.Fatalf("statement has %d public inputs, want %d", len(st), layout.PaymentLen)
		}
		if !slices.EqualFunc(st, res.Statement, func(a, b *big.Int) bool { return a.Cmp(b) == 0 }) {
			t.Error("the calldata's statement is not the proof's")
		}
		message, stTree, root, nf := st[layout.PaymentMessageOffset], st[layout.PaymentTreeNumbersOffset], st[layout.PaymentMerkleRootsOffset], st[layout.PaymentNullifiersOffset]
		c0, c1, contract := st[layout.PaymentCommitmentsOutOffset], st[layout.PaymentCommitmentsOutOffset+1], st[layout.PaymentContractAddressOffset]
		if !bytes.Equal(args[2].([]byte), res.CipherText) || !bytes.Equal(args[3].([]byte), res.EncTxData) || args[1].(*big.Int).Cmp(vaultID) != 0 {
			t.Error("the calldata's vault id and Ana's ciphertexts are not the ones sent")
		}

		// hash is Poseidon, failing the test on an error.
		hash := func(xs ...*big.Int) *big.Int { t.Helper(); return poseidonOf(t, xs...) }
		// vcall calls a one-value vault view at block n.
		vcall := func(n *big.Int, method string, args ...any) any {
			t.Helper()
			return vaultView(t, vault, n, method, args...)
		}

		// Message: a payment, 0 (the circuit allows 0 or 1; payment() only 0).
		if message.Sign() != 0 {
			t.Errorf("Message = %s, want 0", message)
		}
		// Range: every value 0 ≤ v < 10^36.
		for _, v := range append([]*big.Int{valueIn}, valuesOut...) {
			if v.Sign() < 0 || v.Cmp(paymentRange) >= 0 {
				t.Errorf("value %s is outside [0, 10^36)", v)
			}
		}
		// Ownership: pk derived from sk, the Sender's registered key.
		pkIn := hash(sk)
		if pkIn.Cmp(senderKeys.SpendKey) != 0 {
			t.Errorf("Poseidon(sk_in) = %s, the Sender registered %s", pkIn, senderKeys.SpendKey)
		}
		// The input commitment is the shielded leaf.
		cIn := hash(pkIn, shielded.salt, valueIn, token)
		if cIn.Cmp(shielded.commitment) != 0 || cIn.Cmp(mp.Element) != 0 {
			t.Errorf("Poseidon(pk, salt, v, token) = %s, the shielded leaf is %s", cIn, shielded.commitment)
		}
		// enable = 1 - isZero(v_in): the input is real, so its public root and
		// nullifier are the computed ones (checked below) and neither is 0,
		// which would make the vault skip its checks.
		enable := 1 - boolInt(valueIn.Sign() == 0)
		if valueIn.Sign() == 0 || root.Sign() == 0 || nf.Sign() == 0 {
			t.Errorf("v_in %s, root %s, nullifier %s: a real input is non-zero, with both non-zero", valueIn, root, nf)
		}
		// Merkle membership: the eight levels of Poseidon(left, right) from
		// the leaf to the root, the bit of the leaf index choosing the side.
		if stTree.Cmp(tree) != 0 {
			t.Errorf("TreeNumber %s, the note is in tree %s", stTree, tree)
		}
		v := map[string]string{}
		nodes := pathNodes(t, cIn, mp)
		for k, n := range nodes {
			p := fmt.Sprintf("path%d.", k)
			v[p+"bit"], v[p+"sibling"], v[p+"node"] = fmt.Sprint(mp.Indices.Bit(k)), mp.Elements[k].String(), n.String()
		}
		node := nodes[len(nodes)-1]
		if node.Cmp(root) != 0 || node.Cmp(mp.Root) != 0 {
			t.Errorf("the path hashes to %s, the statement's root is %s", node, root)
		}
		if !vcall(block, "isValidRoot", stTree, root).(bool) {
			t.Errorf("isValidRoot(%s, %s) = false", stTree, root)
		}
		// The nullifier: Poseidon(sk, tree·2^8 + leaf index), the one the
		// vault's Nullifier event recorded, spent from this block on.
		globalIndex := new(big.Int).Add(new(big.Int).Lsh(tree, primerDepth), mp.Indices)
		if want := hash(sk, globalIndex); want.Cmp(nf) != 0 {
			t.Errorf("Poseidon(sk, tree·2^8 + i) = %s, the statement's nullifier %s", want, nf)
		}
		checkOneNullifierEvent(t, mined.Logs, vaultAddr, vaultID, tree, nf)
		if vcall(new(big.Int).Sub(block, big.NewInt(1)), "isValidNullifier", tree, nf).(bool) || !vcall(block, "isValidNullifier", tree, nf).(bool) {
			t.Error("the nullifier is not unspent before the payment and spent after it")
		}
		// Spend-once: the same receipt, sent again, reverts.
		if _, err := bindContract(t, client, dvpAddr, dvpABI).Transact(&bind.TransactOpts{From: auth.From, Signer: auth.Signer, NoSend: true, Context: ctx},
			"payment", receipt, vaultID, res.CipherText, res.EncTxData); err == nil {
			t.Error("the spent receipt would settle a second time")
		}
		// The output commitments: Ana's (output 0) and the Sender's change
		// (output 1), both inserted by the settlement.
		if want := hash(anaKeys.SpendKey, res.SaltB, valuesOut[0], token); want.Cmp(c0) != 0 {
			t.Errorf("Poseidon(pk_Ana, salt0, v0, token) = %s, CommitmentOut[0] %s", want, c0)
		}
		if want := hash(pkIn, res.SaltA, valuesOut[1], token); want.Cmp(c1) != 0 {
			t.Errorf("Poseidon(pk_in, salt1, v1, token) = %s, CommitmentOut[1] %s (change ownership)", want, c1)
		}
		if c0.Cmp(c1) == 0 {
			t.Error("the two outputs share a commitment")
		}
		if logged := commitmentsIn(mined.Logs, vaultAddr); len(logged) != 2 || logged[0].Cmp(c0) != 0 || logged[1].Cmp(c1) != 0 {
			t.Errorf("the settlement's Commitment events %v, want CommitmentOut[0] and [1]", logged)
		}
		// The Payment event for Ana: her commitment, beside the capsule and
		// the sealed payload of the calldata.
		events := paymentEventsIn(t, mined.Logs, dvpAddr)
		if len(events) != 1 || events[0].Commitment.Cmp(c0) != 0 ||
			!bytes.Equal(events[0].CipherText, res.CipherText) || !bytes.Equal(events[0].EncTxData, res.EncTxData) {
			t.Errorf("Payment events %v, want one, for Ana's commitment with the calldata's ctxt and encTxData", events)
		}
		// Conservation.
		if sum := new(big.Int).Add(valuesOut[0], valuesOut[1]); sum.Cmp(valueIn) != 0 {
			t.Errorf("Σout = %s, Σin = %s", sum, valueIn)
		}
		// ContractAddress: this vault, and not 0.
		if contract.Sign() == 0 || contract.Cmp(new(big.Int).SetBytes(vaultAddr.Bytes())) != 0 {
			t.Errorf("ContractAddress %s, the vault is %s", contract, vaultAddr.Hex())
		}
		for _, x := range st {
			inField(t, "a public input", x)
		}

		// Where the outputs landed (the tree may have rolled over).
		mtAfter := rebuildVaultTree(t, client, vaultAddr, block)
		leafOf := func(c *big.Int) string {
			t.Helper()
			p, err := mtAfter.GenerateProof(c)
			if err != nil {
				t.Fatalf("output %s is not in the vault's tree: %v", c, err)
			}
			return fmt.Sprintf("tree %d · leaf #%s", p.TreeNumber, p.Indices)
		}

		maps.Copy(v, map[string]string{
			"msgSender":       from.Hex(),
			"tx":              mined.TxHash.Hex(),
			"block":           block.String(),
			"dvp":             dvpAddr.Hex(),
			"skSpend":         sk.String(),
			"pkSpend":         pkIn.String(),
			"valueIn":         valueIn.String(),
			"saltIn":          shielded.salt.String(),
			"token":           token.String(),
			"commitmentIn":    cIn.String(),
			"tree":            tree.String(),
			"leafIndex":       mp.Indices.String(),
			"globalIndex":     globalIndex.String(),
			"root":            root.String(),
			"nullifier":       nf.String(),
			"anaPk":           anaKeys.SpendKey.String(),
			"valueOut0":       valuesOut[0].String(),
			"saltOut0":        res.SaltB.String(),
			"commitmentOut0":  c0.String(),
			"leafOut0":        leafOf(c0),
			"valueOut1":       valuesOut[1].String(),
			"saltOut1":        res.SaltA.String(),
			"commitmentOut1":  c1.String(),
			"leafOut1":        leafOf(c1),
			"range":           paymentRange.String(),
			"enable":          fmt.Sprint(enable),
			"message":         message.String(),
			"contractAddress": contract.String(),
			"vault":           vaultAddr.Hex(),
		})
		label := "payment · Sender"
		ledger := []ledgerItem{
			{Tx: label, Field: "msg.sender", Value: from.Hex(), MsgSender: true},
			{Tx: label, Field: "vaultId", Value: vaultID.String()},
		}
		for _, in := range paymentPublic {
			ledger = append(ledger, ledgerItem{Tx: "payment · public inputs", Field: in.name, Value: st[in.offset].String()})
		}
		ledger = append(ledger,
			ledgerItem{Tx: label, Field: "proof", Value: strings.Join(res.Proof, " ")},
			ledgerItem{Tx: label, Field: "ctxt", Value: hexutil.Encode(res.CipherText)},
			ledgerItem{Tx: label, Field: "encTxData", Value: hexutil.Encode(res.EncTxData)},
			ledgerItem{Tx: "Nullifier event", Field: "nullifier", Value: nf.String()},
			ledgerItem{Tx: "Commitment event", Field: "commitment", Value: c0.String()},
			ledgerItem{Tx: "Commitment event", Field: "leaf", Value: leafOf(c0)},
			ledgerItem{Tx: "Commitment event", Field: "commitment", Value: c1.String()},
			ledgerItem{Tx: "Commitment event", Field: "leaf", Value: leafOf(c1)},
			ledgerItem{Tx: "Payment event", Field: "commitmentOut", Value: c0.String()},
			ledgerItem{Tx: "Payment event", Field: "ctxt", Value: hexutil.Encode(res.CipherText)},
			ledgerItem{Tx: "Payment event", Field: "encTxData", Value: hexutil.Encode(res.EncTxData)},
		)
		data.Steps["payment"] = primerEntry{Values: v, Ledger: ledger}
		paid = &paidNote{commitment: c0, salt: res.SaltB, amount: valuesOut[0], token: token, block: block}
		proved = &provedPayment{proof: cd.Proof, statement: st, block: block}
	})

	check("proof", "L = IC₀ + Σ xᵢ·ICᵢ from the verifying key and the payment's public inputs, e(A, B) = e(α, β)·e(L, γ)·e(C, δ), and the Verifier's verifyProof answers true", func(t *testing.T) {
		if proved == nil {
			t.Fatal("no payment proof: the payment check did not pass")
		}
		vk := loadPaymentVK(t)
		if len(vk.ic) != len(proved.statement)+1 {
			t.Fatalf("the verifying key has %d IC points for %d public inputs", len(vk.ic), len(proved.statement))
		}

		// The QAP's domain and the proving key's sizes, read from the proving
		// key the gnark server proves with. Its α, β and δ are the verifying
		// key's: the two halves of one setup.
		pk := readPaymentPK(t)
		n := pk.domain.Cardinality
		omega := pk.domain.Generator
		var omegaPow fr.Element
		if omegaPow.Exp(omega, new(big.Int).SetUint64(n)); !omegaPow.IsOne() {
			t.Error("ω^n ≠ 1")
		}
		if omegaPow.Exp(omega, new(big.Int).SetUint64(n/2)); omegaPow.IsOne() {
			t.Error("ω^(n/2) = 1: ω is not a primitive n-th root of unity")
		}
		if n&(n-1) != 0 {
			t.Errorf("the domain has %d points, not a power of 2", n)
		}
		if !pk.alpha.Equal(&vk.alpha) || !pk.beta2.Equal(&vk.beta) || !pk.delta2.Equal(&vk.delta) {
			t.Error("the proving key's α, β, δ are not the verifying key's")
		}
		if pk.commitments != 0 || pk.nbWires != uint64(len(pk.k)+len(vk.ic)) {
			t.Errorf("wires %d = private %d + public and one %d, with %d commitments: want the sum and none", pk.nbWires, len(pk.k), len(vk.ic), pk.commitments)
		}

		// L = IC₀ + Σ xᵢ·ICᵢ, the public inputs folded into one G1 point.
		xs := proved.statement
		var acc bn254.G1Jac
		acc.FromAffine(&vk.ic[0])
		for i, x := range xs {
			inField(t, fmt.Sprintf("x%d", i+1), x)
			var term bn254.G1Jac
			term.ScalarMultiplication(new(bn254.G1Jac).FromAffine(&vk.ic[i+1]), x)
			acc.AddAssign(&term)
		}
		var L bn254.G1Affine
		L.FromJacobian(&acc)

		// The proof, as the payment's calldata carried it.
		A, B, C := g1Of(proved.proof.A), g2Of(proved.proof.B), g1Of(proved.proof.C)
		for name, ok := range map[string]bool{"A": A.IsOnCurve() && A.IsInSubGroup(), "B": B.IsOnCurve() && B.IsInSubGroup(), "C": C.IsOnCurve() && C.IsInSubGroup()} {
			if !ok {
				t.Errorf("%s is not a point of its prime-order group", name)
			}
		}

		// The pairing check, both sides computed: e(A, B) and
		// e(α, β)·e(L, γ)·e(C, δ).
		lhs, err := bn254.Pair([]bn254.G1Affine{A}, []bn254.G2Affine{B})
		if err != nil {
			t.Fatal(err)
		}
		rhs, err := bn254.Pair([]bn254.G1Affine{vk.alpha, L, C}, []bn254.G2Affine{vk.beta, vk.gamma, vk.delta})
		if err != nil {
			t.Fatal(err)
		}
		if !lhs.Equal(&rhs) {
			t.Error("e(A, B) ≠ e(α, β)·e(L, γ)·e(C, δ)")
		}
		// With one public input changed, L moves and the equation fails.
		var Lbad bn254.G1Affine
		Lbad.Add(&L, &vk.ic[1+layout.PaymentNullifiersOffset])
		if bad, _ := bn254.Pair([]bn254.G1Affine{vk.alpha, Lbad, C}, []bn254.G2Affine{vk.beta, vk.gamma, vk.delta}); lhs.Equal(&bad) {
			t.Error("the pairing check holds with Nullifier + 1")
		}

		// The Verifier on chain, asked about the same proof and inputs.
		verifierAddr := common.HexToAddress(receipts["Verifier"].ContractAddress)
		verifier := bindContract(t, client, verifierAddr, loadOnchainABI(t, "Verifier"))
		ask := func(st []*big.Int) bool {
			t.Helper()
			var out []any
			if err := verifier.Call(&bind.CallOpts{BlockNumber: proved.block}, &out, "verifyProof", big.NewInt(paymentVKSlot), proved.proof, st); err != nil {
				t.Fatalf("verifyProof: %v", err)
			}
			return out[0].(bool)
		}
		accepted := ask(xs)
		if !accepted {
			t.Error("verifyProof(0, π, x) = false for the settled payment")
		}
		tampered := slices.Clone(xs)
		tampered[layout.PaymentNullifiersOffset] = new(big.Int).Add(xs[layout.PaymentNullifiersOffset], big.NewInt(1))
		acceptedTampered := ask(tampered)
		if acceptedTampered {
			t.Error("verifyProof accepts the proof with Nullifier + 1")
		}

		v := map[string]string{
			"n":           fmt.Sprint(n),
			"log2n":       fmt.Sprint(bits.TrailingZeros64(n)),
			"halfN":       fmt.Sprint(n / 2),
			"omega":       omega.String(),
			"nbWires":     fmt.Sprint(pk.nbWires),
			"nbPrivate":   fmt.Sprint(len(pk.k)),
			"nbPublic":    fmt.Sprint(len(xs)),
			"alpha":       g1String(vk.alpha),
			"beta":        g2String(vk.beta),
			"gamma":       g2String(vk.gamma),
			"delta":       g2String(vk.delta),
			"L":           g1String(L),
			"A":           g1String(A),
			"B":           g2String(B),
			"C":           g1String(C),
			"vkSlot":      fmt.Sprint(paymentVKSlot),
			"verifier":    verifierAddr.Hex(),
			"verifyProof": fmt.Sprint(accepted),
			"tampered":    fmt.Sprint(acceptedTampered),
		}
		for i := range vk.ic {
			v[fmt.Sprintf("ic%d", i)] = g1String(vk.ic[i])
		}
		for i, in := range paymentPublic {
			v[fmt.Sprintf("x%d", i+1)] = xs[in.offset].String()
			v[fmt.Sprintf("x%d.name", i+1)] = in.name
		}
		data.Steps["proof"] = primerEntry{Values: v, Ledger: []ledgerItem{}}
	})

	check("tag", "the Sender publishes the payment's tag in the block after settlement: t_b = Poseidon(b, pk_spend^Ana, ss mod p) and ct = AES-256-GCM under k_b = HKDF(ss, \"enygma-block-key-v1\" ‖ b ‖ pk_spend^Ana) of amount ‖ token ‖ salt₀; Ana's tag for b matches it alone and ct opens to her note; the reverse direction's tag and key differ", func(t *testing.T) {
		if paid == nil || toAnaChannel == nil {
			t.Fatal("no payment or tag channel to Ana: the payment or channel check did not pass")
		}
		sender, ana := users[0], users[1]
		ss := toAnaChannel.ss
		tagsAddr := common.HexToAddress(receipts["TagRegistry"].ContractAddress)
		tagsABI := privateTagsABI(t, "TagRegistry")
		registry := bindContract(t, client, tagsAddr, tagsABI)

		// The tag lands in the block after the payment's: with nothing sent
		// since, the next transaction is mined in it.
		head, err := client.BlockNumber(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if head != paid.block.Uint64() {
			t.Fatalf("the chain is at block %d, past the payment's block %s: the tag can't land in the block after settlement", head, paid.block)
		}
		b := head + 1

		// The Sender builds the tag and its payload with the client library,
		// and sends publishTag from their own account (no relayer).
		libTag, err := tags.DeriveTag(b, ana.spend.PublicKey, ss)
		if err != nil {
			t.Fatal(err)
		}
		plain := make([]byte, 96)
		paid.amount.FillBytes(plain[:32])
		paid.token.FillBytes(plain[32:64])
		paid.salt.FillBytes(plain[64:])
		libCt, err := tags.EncryptPayload(tags.DeriveBlockKey(ss, b, ana.spend.PublicKey), plain)
		if err != nil {
			t.Fatal(err)
		}
		auth, err := bind.NewKeyedTransactorWithChainID(sender.key, big.NewInt(hardhatChainID))
		if err != nil {
			t.Fatal(err)
		}
		tx, err := registry.Transact(auth, "publishTag", libTag, libCt)
		if err != nil {
			t.Fatalf("publishTag: %v", err)
		}
		r, err := bind.WaitMined(ctx, client, tx)
		if err != nil || r.Status != types.ReceiptStatusSuccessful {
			t.Fatalf("publishTag: receipt %v, err %v", r, err)
		}
		if r.BlockNumber.Uint64() != b {
			t.Fatalf("publishTag landed in block %s, want %d, the block after the payment's", r.BlockNumber, b)
		}

		// The tag as the chain holds it: the calldata, and the TagPublished
		// event naming the block, the tag and the Sender as publisher.
		_, from, args := calldataOf(t, client, tagsABI, r.TxHash, "publishTag", sender.addr, tagsAddr)
		cdTag, ct := args[0].([32]byte), args[1].([]byte)
		if cdTag != libTag || !bytes.Equal(ct, libCt) {
			t.Error("the calldata's tag and ctxt are not the ones built")
		}
		var published int
		for _, l := range r.Logs {
			if l.Address != tagsAddr || len(l.Topics) != 4 || l.Topics[0] != tagPublishedSig {
				continue
			}
			published++
			vals, err := tagsABI.Events["TagPublished"].Inputs.NonIndexed().Unpack(l.Data)
			if err != nil || len(vals) != 1 {
				t.Fatalf("decode TagPublished: %v", err)
			}
			if l.Topics[1].Big().Uint64() != b || l.Topics[2] != common.Hash(cdTag) || common.BytesToAddress(l.Topics[3].Bytes()) != sender.addr || !bytes.Equal(vals[0].([]byte), ct) {
				t.Error("the TagPublished event is not block b's tag and ctxt, published by the Sender")
			}
		}
		if published != 1 {
			t.Fatalf("%d TagPublished events, want 1", published)
		}

		// Recomputed with iden3's Poseidon and the standard library: ss
		// reduced into BN254's scalar field, the tag over (b, pk_spend^Ana,
		// ss mod p); k_b by HKDF-SHA256 with info "enygma-block-key-v1" ‖
		// uint64_be(b) ‖ be32(pk_spend^Ana); ct = nonce ‖ AES-256-GCM, no AAD.
		ssField := toField(ss)
		bInt := new(big.Int).SetUint64(b)
		tb := poseidonOf(t, bInt, ana.spend.PublicKey, ssField)
		if common.BigToHash(tb) != common.Hash(cdTag) {
			t.Errorf("Poseidon(b, pk_spend^Ana, ss mod p) = %x, the published tag %x", tb, cdTag)
		}
		// infoFor is k_b's HKDF info for the payee whose spend key is pk.
		infoFor := func(pk *big.Int) []byte {
			return append(binary.BigEndian.AppendUint64([]byte("enygma-block-key-v1"), b), pk.FillBytes(make([]byte, 32))...)
		}
		// blockKey is k_b for the payee whose spend key is pk.
		blockKey := func(pk *big.Int) []byte {
			t.Helper()
			k, err := hkdf.Key(sha256.New, ss, nil, string(infoFor(pk)), 32)
			if err != nil {
				t.Fatal(err)
			}
			return k
		}
		kb := blockKey(ana.spend.PublicKey)
		if lib := tags.DeriveBlockKey(ss, b, ana.spend.PublicKey); !bytes.Equal(lib[:], kb) {
			t.Error("the library's block key is not HKDF-SHA256(ss, \"enygma-block-key-v1\" ‖ b ‖ pk_spend)")
		}
		gcm := newGCM(t, kb)
		nonce, sealed := ct[:gcm.NonceSize()], ct[gcm.NonceSize():]
		opened, err := gcm.Open(nil, nonce, sealed, nil)
		if err != nil || !bytes.Equal(opened, plain) {
			t.Fatalf("ct opens under k_b to %x (err %v), want amount ‖ token ‖ salt₀", opened, err)
		}
		amount, token, salt := new(big.Int).SetBytes(opened[:32]), new(big.Int).SetBytes(opened[32:64]), new(big.Int).SetBytes(opened[64:])
		if c := poseidonOf(t, ana.spend.PublicKey, salt, amount, token); c.Cmp(paid.commitment) != 0 {
			t.Errorf("the opening hashes to %s, Ana's commitment C₀ is %s", c, paid.commitment)
		}

		// The reverse direction, Ana paying the Sender in the same block on the
		// same tag channel: the Sender's spend key in Ana's place gives
		// another tag and another key, which does not open ct.
		reverse := poseidonOf(t, bInt, sender.spend.PublicKey, ssField)
		if reverse.Cmp(tb) == 0 {
			t.Error("the reverse direction's tag is the tag")
		}
		if lib, _ := tags.DeriveTag(b, sender.spend.PublicKey, ss); common.BigToHash(reverse) != common.Hash(lib) {
			t.Error("the library's reverse tag differs from Poseidon(b, pk_spend^Sender, ss mod p)")
		}
		if _, err := newGCM(t, blockKey(sender.spend.PublicKey)).Open(nil, nonce, sealed, nil); err == nil {
			t.Error("the reverse direction's block key opens ct")
		}

		// Ana's side: from her own Decaps of the tag channel's record, her
		// tag for block b; of block b's entries in TagRegistry, it matches
		// the Sender's alone. The library's scan agrees, and the Sender's
		// scan for tags to them finds nothing: their own tag is not theirs.
		anaTag := common.BigToHash(poseidonOf(t, bInt, ana.spend.PublicKey, toField(toAnaChannel.anaSS)))
		entries, err := tags.GetEntries(client, tagsAddr, b)
		if err != nil {
			t.Fatal(err)
		}
		matched := 0
		for _, e := range entries {
			if common.Hash(e.Tag) == anaTag {
				matched++
				if e.Publisher != sender.addr || !bytes.Equal(e.Ctxt, ct) {
					t.Error("Ana's matching entry is not the Sender's tag and ctxt")
				}
			}
		}
		if matched != 1 {
			t.Errorf("Ana's tag matches %d of block %d's %d entries, want 1", matched, b, len(entries))
		}
		scanned, err := tags.ScanBlock(client, tagsAddr, b, []tags.Channel{{SharedSecret: toAnaChannel.anaSS, PkSpend: ana.spend.PublicKey}})
		if err != nil || len(scanned) != 1 {
			t.Fatalf("Ana's ScanBlock of block %d: %d matches (err %v), want 1", b, len(scanned), err)
		}
		note, err := tags.DecryptPaymentNote(toAnaChannel.anaSS, ana.spend.PublicKey, scanned[0].BlockNumber, scanned[0].Entry.Ctxt)
		if err != nil || note.Amount.Cmp(paid.amount) != 0 || note.TokenId.Cmp(paid.token) != 0 || note.Salt.Cmp(paid.salt) != 0 {
			t.Errorf("DecryptPaymentNote = %+v (err %v), want Ana's note", note, err)
		}
		if own, err := tags.ScanBlock(client, tagsAddr, b, []tags.Channel{{SharedSecret: ss, PkSpend: sender.spend.PublicKey}}); err != nil || len(own) != 0 {
			t.Errorf("the Sender's own scan of block %d: %d matches (err %v), want 0", b, len(own), err)
		}

		v := map[string]string{
			"registry":     tagsAddr.Hex(),
			"msgSender":    from.Hex(),
			"tx":           r.TxHash.Hex(),
			"b":            fmt.Sprint(b),
			"paymentBlock": paid.block.String(),
			"channelIdx":   fmt.Sprint(toAnaChannel.idx),
			"ss":           hexutil.Encode(ss),
			"ssField":      ssField.String(),
			"pkSpend":      ana.spend.PublicKey.String(),
			"tag":          hexutil.Encode(cdTag[:]),
			"info":         hexutil.Encode(infoFor(ana.spend.PublicKey)),
			"kb":           hexutil.Encode(kb),
			"plaintext":    hexutil.Encode(opened),
			"amount":       amount.String(),
			"token":        token.String(),
			"salt":         salt.String(),
			"nonce":        hexutil.Encode(nonce),
			"sealed":       hexutil.Encode(sealed),
			"ct":           hexutil.Encode(ct),
			"commitment":   paid.commitment.String(),
			"senderPk":     sender.spend.PublicKey.String(),
			"reverseTag":   hexutil.Encode(common.BigToHash(reverse).Bytes()),
			"reverseOpen":  "⊥",
			"anaTag":       anaTag.Hex(),
			"entries":      fmt.Sprint(len(entries)),
			"matched":      fmt.Sprint(matched),
		}
		tagBlock = b
		label := "publishTag · Sender"
		data.Steps["tag"] = primerEntry{Values: v, Ledger: []ledgerItem{
			{Tx: label, Field: "msg.sender", Value: from.Hex(), MsgSender: true},
			{Tx: label, Field: "tag", Value: hexutil.Encode(cdTag[:])},
			{Tx: label, Field: "ctxt", Value: hexutil.Encode(ct)},
		}}
	})

	check("discovery", "Ana trial-decrypts every Payment event's ciphertext with her view key, opens exactly her note, and its opening hashes to its commitment; her tag scan from the tag channel's block to the tag's matches one entry, which opens to the same note", func(t *testing.T) {
		if paid == nil {
			t.Fatal("no note paid to Ana: the payment check did not pass")
		}
		sender, ana := users[0], users[1]
		dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)

		// Every Payment event EnygmaDvp has emitted, up to the payment's block:
		// Ana's wallet cannot tell which is hers, so it reads them all.
		events := paymentEventsUpTo(t, ctx, client, dvpAddr, paid.block)
		var mine *dvpcore.OnChainErc20Event
		for i := range events {
			if events[i].Commitment.Cmp(paid.commitment) == 0 {
				mine = &events[i]
			}
		}
		if mine == nil {
			t.Fatalf("no Payment event for Ana's commitment %s among %d", paid.commitment, len(events))
		}
		capsule, payload := mine.CipherText, mine.EncTxData

		// The encryption, undone with Ana's view key: the capsule gives back
		// the Sender's shared secret, whose HKDF outputs are the note's salt
		// and the payload's key; the payload is token ‖ amount.
		if len(capsule) != mlkem.CiphertextSize768 {
			t.Errorf("capsule is %d bytes, want %d", len(capsule), mlkem.CiphertextSize768)
		}
		anaOpen := trialOpen(t, ana.view.DecapsKey, capsule, payload)
		if !anaOpen.ok {
			t.Fatal("Ana's view key does not open the Payment event addressed to her")
		}
		if len(anaOpen.ss) != mlkem.SharedKeySize || len(anaOpen.sealed) != 64+16 || len(anaOpen.plain) != 64 {
			t.Errorf("ss %d bytes, ct‖tag %d, plaintext %d: want 32, 80 and 64", len(anaOpen.ss), len(anaOpen.sealed), len(anaOpen.plain))
		}
		token, amount := anaOpen.tokenAmount()
		if token.Cmp(paid.token) != 0 || amount.Cmp(paid.amount) != 0 {
			t.Errorf("the payload opens to token %s, amount %s; the Sender paid token %s, amount %s", token, amount, paid.token, paid.amount)
		}
		salt := toField(anaOpen.saltBytes)
		if salt.Cmp(paid.salt) != 0 {
			t.Errorf("HKDF(ss, \"note salt\") mod p = %s, the Sender's salt_0 %s", salt, paid.salt)
		}
		// The client library's decryption agrees.
		if libToken, libAmount, err := rpcore.DecryptPayload(anaOpen.key, payload); err != nil || libToken.Cmp(token) != 0 || libAmount.Cmp(amount) != 0 {
			t.Errorf("DecryptPayload = (%v, %v, %v), want (%s, %s)", libToken, libAmount, err, token, amount)
		}

		// Ana checks the opening: Poseidon(pk_spend, salt, amount, token) is
		// the commitment the Payment event published.
		commitment, err := poseidon.Hash([]*big.Int{ana.spend.PublicKey, salt, amount, token})
		if err != nil {
			t.Fatal(err)
		}
		if commitment.Cmp(mine.Commitment) != 0 {
			t.Errorf("Poseidon(pk_Ana, salt, amount, token) = %s, the event's commitment %s", commitment, mine.Commitment)
		}

		// The scan: every ciphertext tried with Ana's key, only hers opens.
		// ML-KEM's Decaps rejects implicitly (a capsule made for another key
		// gives a pseudorandom secret, not an error), so it is AES-GCM's tag
		// that fails.
		opened := 0
		for _, e := range events {
			if trialOpen(t, ana.view.DecapsKey, e.CipherText, e.EncTxData).ok {
				opened++
				if e.Commitment.Cmp(paid.commitment) != 0 {
					t.Errorf("Ana's key opened the Payment event for %s, not hers", e.Commitment)
				}
			}
		}
		if opened != 1 {
			t.Errorf("Ana's key opened %d of %d Payment events, want exactly hers", opened, len(events))
		}
		// Under any other key, her ciphertext fails too: the Sender's view key
		// decapsulates it to another secret, and the tag does not check.
		if senderTry := trialOpen(t, sender.view.DecapsKey, capsule, payload); senderTry.ok || bytes.Equal(senderTry.ss, anaOpen.ss) {
			t.Error("the Sender's view key opens Ana's ciphertext")
		}
		// The client library's scan finds the same note, once.
		owned, err := dvpcore.ScanForErc20Notes(ana.view.DecapsKey, ana.spend.PublicKey, []dvpcore.OnChainErc20Event{*mine})
		if err != nil || len(owned) != 1 || owned[0].Amount.Cmp(amount) != 0 || owned[0].SaltBField.Cmp(salt) != 0 {
			t.Errorf("ScanForErc20Notes = %v (err %v), want Ana's note of %s", owned, err, amount)
		}

		// A mismatched ciphertext can't fool her: a payload sealed under her
		// key that claims one more than the note holds opens, but its opening
		// hashes elsewhere, and the library's scan refuses it.
		forged, err := rpcore.EncryptPayload(anaOpen.key, token, new(big.Int).Add(amount, big.NewInt(1)))
		if err != nil {
			t.Fatal(err)
		}
		forgedOpen := trialOpen(t, ana.view.DecapsKey, capsule, forged)
		if !forgedOpen.ok {
			t.Fatal("the forged payload, sealed under Ana's key, does not open")
		}
		claimed := new(big.Int).SetBytes(forgedOpen.plain[32:])
		if h, _ := poseidon.Hash([]*big.Int{ana.spend.PublicKey, salt, claimed, token}); h.Cmp(mine.Commitment) == 0 {
			t.Error("a payload claiming another amount hashes to the published commitment")
		}
		owned, rejected, err := dvpcore.ScanForErc20NotesWithRejected(ana.view.DecapsKey, ana.spend.PublicKey,
			[]dvpcore.OnChainErc20Event{{Commitment: mine.Commitment, CipherText: capsule, EncTxData: forged}, *mine})
		if err != nil || len(owned) != 1 || owned[0].Amount.Cmp(amount) != 0 {
			t.Errorf("the mismatched ciphertext cost Ana her note: owned %v, err %v", owned, err)
		}
		if !slices.Equal(rejected, []dvpcore.RejectedEvent{{Index: 0, Reason: dvpcore.RejectCommitmentMismatch}}) {
			t.Errorf("rejected = %v, want the mismatched ciphertext", rejected)
		}
		// A capsule of the wrong length costs any wallet only that event:
		// Decaps rejects it, and the scan skips it and goes on.
		short := dvpcore.OnChainErc20Event{Commitment: big.NewInt(1), CipherText: capsule[:len(capsule)-1], EncTxData: payload}
		for name, w := range map[string]struct {
			view  *mlkem.DecapsulationKey768
			spend *big.Int
			notes int
		}{"Ana": {ana.view.DecapsKey, ana.spend.PublicKey, 1}, "the Sender": {sender.view.DecapsKey, sender.spend.PublicKey, 0}} {
			owned, rejected, err := dvpcore.ScanForErc20NotesWithRejected(w.view, w.spend, []dvpcore.OnChainErc20Event{short, *mine})
			if err != nil || len(owned) != w.notes {
				t.Errorf("%s's scan with a short capsule: owned %v, err %v, want %d notes", name, owned, err, w.notes)
			}
			if !slices.Equal(rejected, []dvpcore.RejectedEvent{{Index: 0, Reason: dvpcore.RejectBadCapsule}}) {
				t.Errorf("%s's scan: rejected = %v, want the short capsule", name, rejected)
			}
		}

		// The way with a tag. Ana's wallet finds its tag channels once, by
		// trying every record of TagChannelRegistry (step 5), then, for each
		// block from the tag channel's on, computes one tag per tag channel
		// and reads that block's entries once. One entry matches, in the
		// tag's block, and one decryption opens it to this same note.
		if toAnaChannel == nil || tagBlock == 0 {
			t.Fatal("no tag channel or tag: the channel or tag check did not pass")
		}
		tagsAddr := common.HexToAddress(receipts["TagRegistry"].ContractAddress)
		records, found := scanTagChannels(t, client, receipts, ana.view.DecapsKey, "Ana's view key", toAnaChannel.idx)
		channels := []tags.Channel{{SharedSecret: found[0].SharedSecret, PkSpend: ana.spend.PublicKey}}
		from, to := toAnaChannel.block, tagBlock
		var tagsComputed, entriesRead int
		var hits []tags.ScannedTag
		for b := from; b <= to; b++ {
			entries, err := tags.GetEntries(client, tagsAddr, b)
			if err != nil {
				t.Fatal(err)
			}
			entriesRead += len(entries)
			// Recomputed with iden3's Poseidon, apart from the library's scan.
			want := map[common.Hash]bool{}
			for _, c := range channels {
				want[common.BigToHash(poseidonOf(t, new(big.Int).SetUint64(b), c.PkSpend, toField(c.SharedSecret)))] = true
				tagsComputed++
			}
			mine := 0
			for _, e := range entries {
				if want[common.Hash(e.Tag)] {
					mine++
				}
			}
			scanned, err := tags.ScanBlock(client, tagsAddr, b, channels)
			if err != nil {
				t.Fatal(err)
			}
			if len(scanned) != mine {
				t.Errorf("block %d: the library's scan matched %d entries, the recomputed tags %d", b, len(scanned), mine)
			}
			hits = append(hits, scanned...)
		}
		if len(hits) != 1 || hits[0].BlockNumber != tagBlock {
			t.Fatalf("Ana's tag scan of blocks %d–%d matched %d entries, want 1, in block %d", from, to, len(hits), tagBlock)
		}
		// One decryption per matched entry, each opening to this same note.
		decryptions := 0
		for _, h := range hits {
			byTag, err := tags.DecryptPaymentNote(found[0].SharedSecret, ana.spend.PublicKey, h.BlockNumber, h.Entry.Ctxt)
			decryptions++
			if err != nil || byTag.Amount.Cmp(amount) != 0 || byTag.TokenId.Cmp(token) != 0 || byTag.Salt.Cmp(salt) != 0 {
				t.Errorf("the matched entry opens to %+v (err %v), want the note the trial decryption found", byTag, err)
			}
		}
		// Trying every note over the same blocks: the Payment events in them.
		window, err := client.FilterLogs(ctx, ethereum.FilterQuery{Addresses: []common.Address{dvpAddr},
			Topics: [][]common.Hash{{paymentEventSig}}, FromBlock: new(big.Int).SetUint64(from), ToBlock: new(big.Int).SetUint64(to)})
		if err != nil {
			t.Fatal(err)
		}
		if len(window) < 1 || len(window) > len(events) {
			t.Errorf("%d Payment events in blocks %d–%d, want the payment's among the %d", len(window), from, to, len(events))
		}

		data.Steps["discovery"] = primerEntry{Values: map[string]string{
			"channelRecords": fmt.Sprint(records),
			"tagChannels":    fmt.Sprint(len(found)),
			"tagFrom":        fmt.Sprint(from),
			"tagTo":          fmt.Sprint(to),
			"tagBlocks":      fmt.Sprint(to - from + 1),
			"tagsComputed":   fmt.Sprint(tagsComputed),
			"tagEntries":     fmt.Sprint(entriesRead),
			"tagMatched":     fmt.Sprint(len(hits)),
			"tagOpened":      fmt.Sprint(decryptions),
			"windowEvents":   fmt.Sprint(len(window)),
			"anaEk":          hexutil.Encode(ana.view.EncapsKey),
			"capsule":        hexutil.Encode(capsule),
			"ss":             hexutil.Encode(anaOpen.ss),
			"saltBytes":      hexutil.Encode(anaOpen.saltBytes),
			"salt":           salt.String(),
			"key":            hexutil.Encode(anaOpen.key),
			"nonce":          hexutil.Encode(anaOpen.nonce),
			"sealed":         hexutil.Encode(anaOpen.sealed),
			"encTxData":      hexutil.Encode(payload),
			"plaintext":      hexutil.Encode(anaOpen.plain),
			"token":          token.String(),
			"amount":         amount.String(),
			"pkSpend":        ana.spend.PublicKey.String(),
			"commitment":     commitment.String(),
			"scanned":        fmt.Sprint(len(events)),
			"opened":         fmt.Sprint(opened),
			"forged":         claimed.String(),
		}, Ledger: []ledgerItem{}}
		discovered = &paidNote{commitment: commitment, salt: salt, amount: amount, token: token, block: paid.block, capsule: capsule, payload: payload}
	})

	check("withdrawal", "Ana's partial withdrawal from her discovered note: the Payment circuit with Message = 1, its amount and recipient public, her change a private note, and every equation holds on the run's values and the chain", func(t *testing.T) {
		if discovered == nil {
			t.Fatal("no note for Ana to withdraw from: the discovery check did not pass")
		}
		if !serverAvailable("localhost:8082") {
			t.Fatal("the gnark server is not running on :8082 (shield_demo/run.sh --no-demo starts it)")
		}
		sender, ana := users[0], users[1]
		e := newPartialWithdrawEnv(t)
		sk, pk := ana.spend.PrivateKey, ana.spend.PublicKey
		token, valueIn := discovered.token, discovered.amount
		amount := big.NewInt(withdrawAmount)
		change := new(big.Int).Sub(valueIn, amount)
		recipient := ana.addr
		recipientField := new(big.Int).SetBytes(recipient.Bytes())
		vaultID := big.NewInt(0)
		hash := func(xs ...*big.Int) *big.Int { t.Helper(); return poseidonOf(t, xs...) }

		// The note she opened, and its path in the tree the vault holds now.
		head, err := client.BlockNumber(ctx)
		if err != nil {
			t.Fatal(err)
		}
		mp, err := rebuildVaultTree(t, client, vaultAddr, new(big.Int).SetUint64(head)).GenerateProof(discovered.commitment)
		if err != nil {
			t.Fatalf("Ana's note is not in the vault's tree: %v", err)
		}
		tree := big.NewInt(int64(mp.TreeNumber))

		// The proof, from the gnark server, as the retail withdrawal path makes it.
		s := e.mustProveWithdraw(t, pwNote{spend: ana.spend, view: ana.view, salt: discovered.salt, value: valueIn, commit: discovered.commitment}, amount, change, recipient)

		// The receipt fixes the exit, and fits no other route.
		e.expectRevert(t, "InvalidOpening", amount, sender.addr, s.receipt, s.ctI, s.ctII)
		e.expectRevert(t, "InvalidOpening", new(big.Int).Add(amount, big.NewInt(1)), recipient, s.receipt, s.ctI, s.ctII)
		dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)
		dvp := bindContract(t, client, dvpAddr, loadOnchainABI(t, "EnygmaDvp"))
		if _, err := dvp.Transact(&bind.TransactOpts{From: e.owner.From, Signer: e.owner.Signer, NoSend: true, Context: ctx}, "payment", s.receipt, vaultID, s.ctI, s.ctII); err == nil || !strings.Contains(err.Error(), "InvalidPaymentMessage") {
			t.Errorf("payment() with a withdrawal receipt: %v, want InvalidPaymentMessage", err)
		}

		// Ana sends it from her own account: no relayer.
		balanceBefore, vaultBefore := e.balanceOf(t, recipient), e.balanceOf(t, vaultAddr)
		auth, err := bind.NewKeyedTransactorWithChainID(ana.key, big.NewInt(hardhatChainID))
		if err != nil {
			t.Fatal(err)
		}
		tx, err := e.vault.Transact(auth, "partialWithdraw", []*big.Int{amount, token}, recipient, s.receipt, s.ctI, s.ctII)
		if err != nil {
			t.Fatalf("partialWithdraw: %v", err)
		}
		mined, err := bind.WaitMined(ctx, client, tx)
		if err != nil || mined.Status != types.ReceiptStatusSuccessful {
			t.Fatalf("partialWithdraw: receipt %v, err %v", mined, err)
		}
		block := mined.BlockNumber

		// The events, the change note's ciphertexts opening for Ana, and the
		// change note rebuilt from her key and the events alone.
		e.checkSettled(t, mined, s)

		// The withdrawal as the chain holds it: its calldata.
		_, from, args := calldataOf(t, client, e.vaultABI, mined.TxHash, "partialWithdraw", ana.addr, vaultAddr)
		params, cdRecipient := args[0].([]*big.Int), args[1].(common.Address)
		cd := *abi.ConvertType(args[2], new(ProofReceipt)).(*ProofReceipt)
		ctI, ctII := args[3].([]byte), args[4].([]byte)
		if len(params) != 2 || params[0].Cmp(amount) != 0 || params[1].Cmp(token) != 0 || cdRecipient != recipient {
			t.Errorf("calldata params %v to %s, want amount %s, token %s to %s", params, cdRecipient.Hex(), amount, token, recipient.Hex())
		}
		if !bytes.Equal(ctI, s.ctI) || !bytes.Equal(ctII, s.ctII) {
			t.Error("the calldata's ciphertexts are not the ones sent")
		}
		st := cd.Statement
		if len(st) != layout.PaymentLen || len(st) != len(paymentPublic) {
			t.Fatalf("statement has %d public inputs, want %d", len(st), layout.PaymentLen)
		}
		message, stTree, root, nf := st[layout.PaymentMessageOffset], st[layout.PaymentTreeNumbersOffset], st[layout.PaymentMerkleRootsOffset], st[layout.PaymentNullifiersOffset]
		c0, c1, contract := st[layout.PaymentCommitmentsOutOffset], st[layout.PaymentCommitmentsOutOffset+1], st[layout.PaymentContractAddressOffset]
		for _, x := range st {
			inField(t, "a public input", x)
		}

		// Message: a withdrawal, 1 (WITHDRAW_MESSAGE).
		if message.Cmp(big.NewInt(1)) != 0 {
			t.Errorf("Message = %s, want 1", message)
		}
		// The same circuit: the Payment key, in the Verifier's slot 0.
		verified := e.verifiesAt(t, paymentVKSlot, s.receipt)
		if !verified {
			t.Errorf("verifyProof(%d, π, x) = false for the settled withdrawal", paymentVKSlot)
		}
		// Range, and conservation.
		for _, x := range []*big.Int{valueIn, amount, change} {
			if x.Sign() < 0 || x.Cmp(paymentRange) >= 0 {
				t.Errorf("value %s is outside [0, 10^36)", x)
			}
		}
		if sum := new(big.Int).Add(amount, change); sum.Cmp(valueIn) != 0 || change.Sign() <= 0 {
			t.Errorf("amount + change = %s, the note holds %s, change %s: want equal, with change left", sum, valueIn, change)
		}
		// Ownership, and the input commitment, which is the note she discovered.
		if got := hash(sk); got.Cmp(pk) != 0 {
			t.Errorf("Poseidon(sk_in) = %s, Ana registered %s", got, pk)
		}
		cIn := hash(pk, discovered.salt, valueIn, token)
		if cIn.Cmp(discovered.commitment) != 0 || cIn.Cmp(mp.Element) != 0 {
			t.Errorf("Poseidon(pk, salt, v, token) = %s, Ana's leaf is %s", cIn, discovered.commitment)
		}
		// Merkle membership: the eight levels, up to a root the vault knows.
		nodes := pathNodes(t, cIn, mp)
		if got := nodes[len(nodes)-1]; got.Cmp(root) != 0 || stTree.Cmp(tree) != 0 {
			t.Errorf("the path hashes to %s in tree %s, the statement's root is %s in tree %s", got, tree, root, stTree)
		}
		if !vaultView(t, e.vault, block, "isValidRoot", stTree, root).(bool) {
			t.Errorf("isValidRoot(%s, %s) = false", stTree, root)
		}
		// The nullifier, spent by this block and not before it.
		globalIndex := new(big.Int).Add(new(big.Int).Lsh(tree, primerDepth), mp.Indices)
		if want := hash(sk, globalIndex); want.Cmp(nf) != 0 || nf.Sign() == 0 {
			t.Errorf("Poseidon(sk, tree·2^8 + i) = %s, the statement's nullifier %s", want, nf)
		}
		checkOneNullifierEvent(t, mined.Logs, vaultAddr, vaultID, tree, nf)
		if vaultView(t, e.vault, new(big.Int).Sub(block, big.NewInt(1)), "isValidNullifier", tree, nf).(bool) || !vaultView(t, e.vault, block, "isValidNullifier", tree, nf).(bool) {
			t.Error("the nullifier is not unspent before the withdrawal and spent after it")
		}
		// Output 0 is the withdrawal: the recipient as its owner, salt 0. The
		// vault recomputes it and never inserts it.
		if want := hash(recipientField, big.NewInt(0), amount, token); want.Cmp(c0) != 0 {
			t.Errorf("Poseidon(recipient, 0, amount, token) = %s, CommitmentOut[0] %s", want, c0)
		}
		// Output 1 is the change: Ana's key, the salt G11 fixes.
		saltChange := hash(sk, nf)
		if saltChange.Cmp(s.change.salt) != 0 {
			t.Errorf("Poseidon(sk, N) = %s, the change salt %s", saltChange, s.change.salt)
		}
		if want := hash(pk, saltChange, change, token); want.Cmp(c1) != 0 {
			t.Errorf("Poseidon(pk, salt_1, change, token) = %s, CommitmentOut[1] %s", want, c1)
		}
		logged := commitmentsIn(mined.Logs, vaultAddr)
		if len(logged) != 1 || logged[0].Cmp(c1) != 0 {
			t.Errorf("the settlement's Commitment events %v, want only the change, %s", logged, c1)
		}
		mtAfter := rebuildVaultTree(t, client, vaultAddr, block)
		changeProof, err := mtAfter.GenerateProof(c1)
		if err != nil {
			t.Fatalf("the change note is not in the vault's tree: %v", err)
		}
		if _, err := mtAfter.GenerateProof(c0); err == nil {
			t.Error("the withdrawal's output 0 is in the vault's tree")
		}
		// The change, rebuilt with what Ana needs and nothing the ciphertexts
		// carry: her key, the Nullifier event's N (checked above), and her
		// note's value less the PartialWithdrawal event's amount.
		var pwEvents []*types.Log
		pwEvent := e.vaultABI.Events["PartialWithdrawal"]
		for _, l := range mined.Logs {
			if l.Address == vaultAddr && l.Topics[0] == pwEvent.ID {
				pwEvents = append(pwEvents, l)
			}
		}
		if len(pwEvents) != 1 {
			t.Fatalf("%d PartialWithdrawal events, want 1", len(pwEvents))
		}
		eventVals, err := pwEvent.Inputs.NonIndexed().Unpack(pwEvents[0].Data)
		if err != nil {
			t.Fatal(err)
		}
		_, rebuilt, err := dvpcore.WithdrawChangeNote(sk, nf, new(big.Int).Sub(valueIn, eventVals[0].(*big.Int)), token)
		if err != nil || rebuilt.Cmp(pwEvents[0].Topics[3].Big()) != 0 || rebuilt.Cmp(c1) != 0 {
			t.Errorf("the change rebuilt from the events is %v (err %v), the event's changeCommitment %s", rebuilt, err, pwEvents[0].Topics[3].Big())
		}
		// ContractAddress: this vault.
		if contract.Cmp(new(big.Int).SetBytes(vaultAddr.Bytes())) != 0 {
			t.Errorf("ContractAddress %s, the vault is %s", contract, vaultAddr.Hex())
		}
		// The exit: public tokens moved from the vault to the recipient.
		balanceAfter := e.balanceOf(t, recipient)
		if got := new(big.Int).Sub(balanceAfter, balanceBefore); got.Cmp(amount) != 0 {
			t.Errorf("the recipient gained %s, want %s", got, amount)
		}
		if got := new(big.Int).Sub(vaultBefore, e.balanceOf(t, vaultAddr)); got.Cmp(amount) != 0 {
			t.Errorf("the vault lost %s, want %s", got, amount)
		}
		// Spend-once: the same receipt, sent again, reverts.
		e.expectRevert(t, "InvalidNullifier", amount, recipient, s.receipt, s.ctI, s.ctII)

		leafChange := fmt.Sprintf("tree %d · leaf #%s", changeProof.TreeNumber, changeProof.Indices)
		label := "partialWithdraw · Ana"
		ledger := []ledgerItem{
			{Tx: label, Field: "msg.sender", Value: from.Hex(), MsgSender: true},
			{Tx: label, Field: "amount", Value: amount.String()},
			{Tx: label, Field: "recipient", Value: recipient.Hex()},
			{Tx: label, Field: "tokenId", Value: token.String()},
		}
		for _, in := range paymentPublic {
			ledger = append(ledger, ledgerItem{Tx: "partialWithdraw · public inputs", Field: in.name, Value: st[in.offset].String()})
		}
		ledger = append(ledger,
			ledgerItem{Tx: label, Field: "proof", Value: strings.Join(s.proofStrs, " ")},
			ledgerItem{Tx: label, Field: "ciphertextI", Value: hexutil.Encode(ctI)},
			ledgerItem{Tx: label, Field: "ciphertextII", Value: hexutil.Encode(ctII)},
			ledgerItem{Tx: "Nullifier event", Field: "nullifier", Value: nf.String()},
			ledgerItem{Tx: "Commitment event", Field: "commitment", Value: c1.String()},
			ledgerItem{Tx: "Commitment event", Field: "leaf", Value: leafChange},
			ledgerItem{Tx: "EncryptedNote event", Field: "commitment", Value: c1.String()},
			ledgerItem{Tx: "EncryptedNote event", Field: "ciphertextI", Value: hexutil.Encode(ctI)},
			ledgerItem{Tx: "EncryptedNote event", Field: "ciphertextII", Value: hexutil.Encode(ctII)},
			ledgerItem{Tx: "PartialWithdrawal event", Field: "recipient", Value: recipient.Hex()},
			ledgerItem{Tx: "PartialWithdrawal event", Field: "amount", Value: amount.String()},
			ledgerItem{Tx: "PartialWithdrawal event", Field: "changeCommitment", Value: c1.String()},
		)
		data.Steps["withdrawal"] = primerEntry{Values: map[string]string{
			"msgSender":        from.Hex(),
			"tx":               mined.TxHash.Hex(),
			"block":            block.String(),
			"skSpend":          sk.String(),
			"pkSpend":          pk.String(),
			"valueIn":          valueIn.String(),
			"saltIn":           discovered.salt.String(),
			"token":            token.String(),
			"commitmentIn":     cIn.String(),
			"tree":             tree.String(),
			"leafIndex":        mp.Indices.String(),
			"globalIndex":      globalIndex.String(),
			"root":             root.String(),
			"nullifier":        nf.String(),
			"message":          message.String(),
			"recipient":        recipient.Hex(),
			"recipientField":   recipientField.String(),
			"amount":           amount.String(),
			"commitment0":      c0.String(),
			"change":           change.String(),
			"saltChange":       saltChange.String(),
			"commitmentChange": c1.String(),
			"leafChange":       leafChange,
			"contractAddress":  contract.String(),
			"vkSlot":           fmt.Sprint(paymentVKSlot),
			"verifyProof":      fmt.Sprint(verified),
			"balanceBefore":    balanceBefore.String(),
			"balanceAfter":     balanceAfter.String(),
		}, Ledger: ledger}
		exit = &exitedNote{amount: amount, change: change, commitment: c1, nullifier: nf, ctI: ctI, ctII: ctII, tx: mined.TxHash, block: block}
	})

	check("audit", "the auditor recovers Ana's view key from the UserRegistry's audit ciphertexts, opens her notes with it, and finds no ciphertext for the Sender's change; the key opens the Sender's tag channel, whose c₂ declares the payment's msg.sender, and whose tag for b opens to C₀; records declaring someone else or no one open as declared", func(t *testing.T) {
		if discovered == nil || exit == nil {
			t.Fatal("no notes to audit: the discovery or withdrawal check did not pass")
		}
		sender, ana := users[0], users[1]
		dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)

		// All the auditor reads: what Ana registered, as the chain holds it.
		capsule, aesCt, err := rpcore.LookupAuditCiphertexts(client, registryAddr, ana.addr)
		if err != nil {
			t.Fatal(err)
		}
		keys, err := rpcore.LookupKeys(client, registryAddr, ana.addr)
		if err != nil {
			t.Fatal(err)
		}
		recovered, err := rpcore.AuditorDecryptViewKey(auditor.DecapsKey, capsule, aesCt)
		if err != nil {
			t.Fatalf("the auditor cannot open Ana's audit ciphertexts: %v", err)
		}

		// The recovery, recomputed with the standard library: Decaps,
		// HKDF-SHA256, then AES-256-GCM with the capsule as AAD.
		ss, err := auditor.DecapsKey.Decapsulate(capsule)
		if err != nil {
			t.Fatal(err)
		}
		k, err := hkdf.Key(sha256.New, ss, nil, auditHKDFInfo, 32)
		if err != nil {
			t.Fatal(err)
		}
		gcm := newGCM(t, k)
		nonce, sealed := aesCt[:gcm.NonceSize()], aesCt[gcm.NonceSize():]
		seed, err := gcm.Open(nil, nonce, sealed, capsule)
		if err != nil {
			t.Fatalf("AES-GCM open: %v", err)
		}
		if !bytes.Equal(seed, recovered.Bytes()) || !bytes.Equal(seed, ana.view.DecapsKey.Bytes()) {
			t.Errorf("the seed opened (%x) is not the library's, or not Ana's view key's", seed)
		}
		// The auditor's own check: the key it recovered is the one registered.
		ek := recovered.EncapsulationKey().Bytes()
		if !bytes.Equal(ek, keys.ViewKey) {
			t.Error("the recovered view key is not the one the UserRegistry holds for Ana")
		}
		// The audit ciphertexts open for this auditor only.
		other, err := rpcore.NewAuditorKeyPair()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := rpcore.AuditorDecryptViewKey(other.DecapsKey, capsule, aesCt); err == nil {
			t.Error("another auditor's key opened Ana's audit ciphertexts")
		}

		// Her note, opened from the Payment event with the recovered key and
		// checked against the commitment with her public, registered pk_spend.
		note := trialOpen(t, recovered, discovered.capsule, discovered.payload)
		if !note.ok {
			t.Fatal("the recovered view key does not open the payment's note")
		}
		token, amount := note.tokenAmount()
		salt := toField(note.saltBytes)
		if token.Cmp(discovered.token) != 0 || amount.Cmp(discovered.amount) != 0 || salt.Cmp(discovered.salt) != 0 {
			t.Errorf("the auditor opened (%s, %s, salt %s), Ana's note is (%s, %s, salt %s)", token, amount, salt, discovered.token, discovered.amount, discovered.salt)
		}
		commitment := poseidonOf(t, keys.SpendKey, salt, amount, token)
		if commitment.Cmp(discovered.commitment) != 0 {
			t.Errorf("Poseidon(registered pk_spend, salt, amount, token) = %s, the event's commitment %s", commitment, discovered.commitment)
		}

		// Her change, opened from the withdrawal's EncryptedNote ciphertexts.
		// The auditor cross-checks it against the public amount; it cannot
		// check it against C₁, whose salt needs her spend key.
		changeOpen := trialOpen(t, recovered, exit.ctI, exit.ctII)
		if !changeOpen.ok {
			t.Fatal("the recovered view key does not open the change note's ciphertexts")
		}
		changeToken, changeAmount := changeOpen.tokenAmount()
		if changeAmount.Cmp(exit.change) != 0 {
			t.Errorf("the change note opens to %s, Ana kept %s", changeAmount, exit.change)
		}
		if got := new(big.Int).Sub(amount, exit.amount); got.Cmp(changeAmount) != 0 {
			t.Errorf("her note less the public amount is %s, the change opened to %s", got, changeAmount)
		}
		if poseidonOf(t, keys.SpendKey, toField(changeOpen.saltBytes), changeAmount, changeToken).Cmp(exit.commitment) == 0 {
			t.Error("the change commitment opens from the capsule's salt; the page says its salt needs sk_spend")
		}

		// The Sender's change is never published: the payment emits no
		// EncryptedNote, and the Sender's own recovered key opens no Payment
		// event, so the auditor has nothing of theirs to decrypt.
		senderCapsule, senderAesCt, err := rpcore.LookupAuditCiphertexts(client, registryAddr, sender.addr)
		if err != nil {
			t.Fatal(err)
		}
		senderKey, err := rpcore.AuditorDecryptViewKey(auditor.DecapsKey, senderCapsule, senderAesCt)
		if err != nil {
			t.Fatalf("the auditor cannot open the Sender's audit ciphertexts: %v", err)
		}
		events := paymentEventsUpTo(t, ctx, client, dvpAddr, discovered.block)
		senderOpened := 0
		for _, e := range events {
			if trialOpen(t, senderKey, e.CipherText, e.EncTxData).ok {
				senderOpened++
			}
		}
		if senderOpened != 0 {
			t.Errorf("the Sender's view key opened %d Payment events, want none", senderOpened)
		}
		payment, err := client.TransactionReceipt(ctx, common.HexToHash(data.Steps["payment"].Values["tx"]))
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range payment.Logs {
			if len(l.Topics) > 0 && l.Topics[0] == encryptedNoteSig {
				t.Error("the payment emitted an EncryptedNote event; the page says the Sender's change is never published")
			}
		}

		// The payer, named through the tag channel. Ana's recovered view key
		// opens the records sealed to her, as her own wallet's scan does: the
		// Sender's tag channel alone. Its c₂ gives the senderId the Sender
		// declared, and its ss gives the tag that announced Ana's note.
		if toAnaChannel == nil || tagBlock == 0 {
			t.Fatal("no tag channel or tag: the channel or tag check did not pass")
		}
		channelsAddr := common.HexToAddress(receipts["TagChannelRegistry"].ContractAddress)
		tagsAddr := common.HexToAddress(receipts["TagRegistry"].ContractAddress)
		records, opened := scanTagChannels(t, client, receipts, recovered, "the recovered view key", toAnaChannel.idx)
		var record []any
		if err := bindContract(t, client, channelsAddr, tagChannelRegistryABI(t)).Call(&bind.CallOpts{}, &record, "getChannel", new(big.Int).SetUint64(toAnaChannel.idx)); err != nil {
			t.Fatal(err)
		}
		recordSender, c1, c2 := record[0].(common.Address), record[1].([]byte), record[2].([]byte)
		// Recomputed with the standard library: Decaps, HKDF, AES-GCM open.
		chSS, err := recovered.Decapsulate(c1)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(chSS, toAnaChannel.ss) || !bytes.Equal(opened[0].SharedSecret, chSS) {
			t.Error("the auditor's Decaps of c₁ is not the tag channel's ss")
		}
		chKey := channelKeyOf(t, chSS)
		chMsg, senderID, err := openChannelSealed(t, chKey, c1, c2)
		if err != nil {
			t.Fatalf("c₂ does not open under the auditor's k_ch: %v", err)
		}
		if string(opened[0].SenderId) != senderID || opened[0].Sender != recordSender {
			t.Errorf("the library's scan gives senderId %q from %s, the recomputation %q from %s", opened[0].SenderId, opened[0].Sender, senderID, recordSender)
		}
		// The declared identity is the payment's msg.sender: an honest payer.
		payTx, _, err := client.TransactionByHash(ctx, payment.TxHash)
		if err != nil {
			t.Fatal(err)
		}
		payer, err := types.Sender(types.LatestSignerForChainID(big.NewInt(hardhatChainID)), payTx)
		if err != nil {
			t.Fatal(err)
		}
		if senderID != payer.Hex() || recordSender != payer {
			t.Errorf("c₂ declares %s and the record's sender is %s; the payment was sent by %s", senderID, recordSender, payer)
		}
		// The tag ties the tag channel to Ana's note: from ss and her
		// registered pk_spend, the auditor's tag for the tag's block is an
		// entry there, and its ct opens to the opening of C₀ it verified.
		auditTag := common.BigToHash(poseidonOf(t, new(big.Int).SetUint64(tagBlock), keys.SpendKey, toField(chSS)))
		entries, err := tags.GetEntries(client, tagsAddr, tagBlock)
		if err != nil {
			t.Fatal(err)
		}
		var hit *tags.TagEntry
		for i := range entries {
			if common.Hash(entries[i].Tag) == auditTag {
				hit = &entries[i]
			}
		}
		if hit == nil {
			t.Fatalf("the auditor's tag %s is not among block %d's %d entries", auditTag, tagBlock, len(entries))
		}
		tagged, err := tags.DecryptPaymentNote(chSS, keys.SpendKey, tagBlock, hit.Ctxt)
		if err != nil {
			t.Fatal(err)
		}
		if poseidonOf(t, keys.SpendKey, tagged.Salt, tagged.Amount, tagged.TokenId).Cmp(commitment) != 0 {
			t.Error("the tagged payload does not open the commitment C₀ the auditor verified")
		}
		// A dishonest payer: nothing checks senderId. Records sealed off chain
		// to Ana's view key naming someone else, and naming no one, open
		// under the recovered key just as the honest one does.
		someone, err := crypto.GenerateKey()
		if err != nil {
			t.Fatal(err)
		}
		forgedID := crypto.PubkeyToAddress(someone.PublicKey).Hex()
		nUsers, err := rpcore.GetUserCount(client, registryAddr)
		if err != nil {
			t.Fatal(err)
		}
		declared := map[string]string{}
		for _, id := range []string{forgedID, ""} {
			_, fc1, fc2, _, err := tags.PrepareChannelSetup(ana.view.EncapsKey, []byte(chMsg), []byte(id), tags.PrivacyFull, nUsers, 0, nil)
			if err != nil {
				t.Fatal(err)
			}
			fss, err := recovered.Decapsulate(fc1)
			if err != nil {
				t.Fatal(err)
			}
			_, got, err := openChannelSealed(t, channelKeyOf(t, fss), fc1, fc2)
			if err != nil || got != id {
				t.Errorf("a record declaring %q opens to %q (err %v), want it opened as declared", id, got, err)
			}
			declared[id] = got
		}
		if declared[""] == "" {
			declared[""] = "(none)"
		}

		data.Steps["audit"] = primerEntry{Values: map[string]string{
			"channelRecords": fmt.Sprint(records),
			"channelsFound":  fmt.Sprint(len(opened)),
			"channelIdx":     fmt.Sprint(toAnaChannel.idx),
			"channelC1":      hexutil.Encode(c1),
			"channelC2":      hexutil.Encode(c2),
			"channelSs":      hexutil.Encode(chSS),
			"channelKey":     hexutil.Encode(chKey),
			"msg":            chMsg,
			"senderId":       senderID,
			"recordSender":   recordSender.Hex(),
			"tagBlock":       fmt.Sprint(tagBlock),
			"tag":            auditTag.Hex(),
			"forgedId":       declared[forgedID],
			"noneId":         declared[""],
			"address":        ana.addr.Hex(),
			"capsule":        hexutil.Encode(capsule),
			"aesCt":          hexutil.Encode(aesCt),
			"ss":             hexutil.Encode(ss),
			"auditKey":       hexutil.Encode(k),
			"nonce":          hexutil.Encode(nonce),
			"sealed":         hexutil.Encode(sealed),
			"seed":           hexutil.Encode(seed),
			"ek":             hexutil.Encode(ek),
			"otherAuditor":   "refused",
			"noteCapsule":    hexutil.Encode(discovered.capsule),
			"noteEncTxData":  hexutil.Encode(discovered.payload),
			"noteSs":         hexutil.Encode(note.ss),
			"pkSpend":        keys.SpendKey.String(),
			"salt":           salt.String(),
			"token":          token.String(),
			"amount":         amount.String(),
			"commitment":     commitment.String(),
			"changeCapsule":  hexutil.Encode(exit.ctI),
			"changeSealed":   hexutil.Encode(exit.ctII),
			"changeAmount":   changeAmount.String(),
			"withdrawn":      exit.amount.String(),
			"senderOpened":   fmt.Sprint(senderOpened),
			"scanned":        fmt.Sprint(len(events)),
		}, Ledger: []ledgerItem{}}
	})

	check("verdict", "every byte the payment and the withdrawal put on chain names who spent, but not whom the Sender paid, how much, or which note was spent", func(t *testing.T) {
		if shielded == nil || paid == nil || discovered == nil || exit == nil {
			t.Fatal("no story to judge: an earlier check did not pass")
		}
		sender, ana := users[0], users[1]
		payValues, exitValues := data.Steps["payment"].Values, data.Steps["withdrawal"].Values
		decimal := func(s string) *big.Int {
			t.Helper()
			x, ok := new(big.Int).SetString(s, 10)
			if !ok {
				t.Fatalf("%q is not a number", s)
			}
			return x
		}
		// hidden fails if b names any of xs, each a 32-byte word of its
		// calldata or logs.
		hidden := func(what string, b chainBytes, xs map[string]*big.Int) {
			t.Helper()
			for name, x := range xs {
				if b.words[common.BigToHash(x)] {
					t.Errorf("the %s's bytes on chain name %s (%s)", what, name, x)
				}
			}
		}
		anaAsWord, senderAsWord := new(big.Int).SetBytes(ana.addr.Bytes()), new(big.Int).SetBytes(sender.addr.Bytes())

		// The payment names the Sender, and neither Ana, nor an amount, nor
		// the note it spent. A word search of its nullifier shows the bytes
		// searched are the transaction's.
		payer, paidOnChain := chainBytesOf(t, ctx, client, common.HexToHash(payValues["tx"]))
		if payer != sender.addr {
			t.Errorf("the payment's msg.sender is %s, want the Sender's %s", payer.Hex(), sender.addr.Hex())
		}
		payNullifier := decimal(payValues["nullifier"])
		if !paidOnChain.words[common.BigToHash(payNullifier)] {
			t.Error("the payment's bytes do not hold its nullifier: the search reads the wrong bytes")
		}
		senderChange := new(big.Int).Sub(shielded.amount, paid.amount)
		hidden("payment", paidOnChain, map[string]*big.Int{
			"Ana's address":         anaAsWord,
			"Ana's pk_spend":        ana.spend.PublicKey,
			"the spent note":        shielded.commitment,
			"the spent note's salt": shielded.salt,
			"Ana's salt":            paid.salt,
			"the change's salt":     decimal(payValues["saltOut1"]),
			"the note's value":      shielded.amount,
			"Ana's amount":          paid.amount,
			"the Sender's change":   senderChange,
		})
		if bytes.Contains(paidOnChain.raw, ana.view.EncapsKey) {
			t.Error("the payment's bytes on chain hold Ana's view key")
		}

		// The withdrawal names Ana and its public amount, and neither the
		// Sender, nor what her note held, nor which note that was.
		withdrawer, exitOnChain := chainBytesOf(t, ctx, client, exit.tx)
		if withdrawer != ana.addr {
			t.Errorf("the withdrawal's msg.sender is %s, want Ana's %s", withdrawer.Hex(), ana.addr.Hex())
		}
		if !exitOnChain.words[common.BigToHash(exit.amount)] || !exitOnChain.words[common.BigToHash(anaAsWord)] {
			t.Error("the withdrawal's bytes do not hold its public amount and recipient: the search reads the wrong bytes")
		}
		hidden("withdrawal", exitOnChain, map[string]*big.Int{
			"the Sender's address":  senderAsWord,
			"the Sender's pk_spend": sender.spend.PublicKey,
			"the note spent":        discovered.commitment,
			"its salt":              discovered.salt,
			"the note's value":      discovered.amount,
			"Ana's change":          exit.change,
			"the change's salt":     decimal(exitValues["saltChange"]),
		})

		// The one sum an observer can do: the pool holds what came in less
		// what went out, as the vault's public balance says.
		erc20 := bindContract(t, client, erc20Addr, loadOnchainABI(t, "RaylsERC20"))
		balanceAt := func(block *big.Int) *big.Int {
			t.Helper()
			var got []any
			if err := erc20.Call(&bind.CallOpts{BlockNumber: block, Context: ctx}, &got, "balanceOf", vaultAddr); err != nil {
				t.Fatal(err)
			}
			return got[0].(*big.Int)
		}
		poolLeft := new(big.Int).Sub(balanceAt(exit.block), balanceAt(new(big.Int).Sub(shielded.block, big.NewInt(1))))
		if want := new(big.Int).Sub(shielded.amount, exit.amount); poolLeft.Cmp(want) != 0 {
			t.Errorf("the vault's balance grew by %s over the story, want %s", poolLeft, want)
		}

		data.Steps["verdict"] = primerEntry{Values: map[string]string{
			"payer":             payer.Hex(),
			"payNullifier":      payNullifier.String(),
			"commitmentOut0":    payValues["commitmentOut0"],
			"commitmentOut1":    payValues["commitmentOut1"],
			"withdrawer":        withdrawer.Hex(),
			"recipient":         exitValues["recipient"],
			"withdrawNullifier": exit.nullifier.String(),
			"shieldAmount":      shielded.amount.String(),
			"withdrawAmount":    exit.amount.String(),
			"poolLeft":          poolLeft.String(),
		}, Ledger: []ledgerItem{}}
	})
}

// chainBytes is everything a transaction put on the chain besides its
// selector and event IDs: its calldata and its logs' topics and data. Both
// are ABI-encoded in 32-byte words, so words holds each word whole; a run of
// zeros across two words can't pass for a small number.
type chainBytes struct {
	raw   []byte
	words map[common.Hash]bool
}

// chainBytesOf reads a mined transaction's sender and its chainBytes.
func chainBytesOf(t *testing.T, ctx context.Context, client *ethclient.Client, hash common.Hash) (common.Address, chainBytes) {
	t.Helper()
	tx, _, err := client.TransactionByHash(ctx, hash)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := client.TransactionReceipt(ctx, hash)
	if err != nil {
		t.Fatal(err)
	}
	from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
	if err != nil {
		t.Fatal(err)
	}
	b := chainBytes{words: map[common.Hash]bool{}}
	add := func(p []byte) {
		b.raw = append(b.raw, p...)
		for ; len(p) >= common.HashLength; p = p[common.HashLength:] {
			b.words[common.BytesToHash(p[:common.HashLength])] = true
		}
	}
	add(tx.Data()[4:])
	for _, l := range receipt.Logs {
		for _, topic := range l.Topics[1:] {
			add(topic.Bytes())
		}
		add(l.Data)
	}
	return from, b
}

// trialOpen is one trial decryption of a note, as a wallet or the auditor
// makes it, with the standard library: Decaps, HKDF-SHA256 to the salt and
// the key, AES-256-GCM open with no associated data.
func trialOpen(t *testing.T, dk *mlkem.DecapsulationKey768, capsule, payload []byte) noteOpening {
	t.Helper()
	var got noteOpening
	if len(capsule) != mlkem.CiphertextSize768 || len(payload) < 12 {
		return got // not an ML-KEM-768 capsule and a sealed payload: not anyone's note (the library's scan skips it too; the discovery check shows it)
	}
	ss, err := dk.Decapsulate(capsule)
	if err != nil {
		t.Fatalf("Decaps: %v", err)
	}
	got.ss = ss
	if got.saltBytes, err = hkdf.Key(sha256.New, ss, nil, "note salt", 32); err != nil {
		t.Fatal(err)
	}
	if got.key, err = hkdf.Key(sha256.New, ss, nil, "encryption key", 32); err != nil {
		t.Fatal(err)
	}
	gcm := newGCM(t, got.key)
	got.nonce, got.sealed = payload[:gcm.NonceSize()], payload[gcm.NonceSize():]
	got.plain, err = gcm.Open(nil, got.nonce, got.sealed, nil)
	got.ok = err == nil
	return got
}

// tokenAmount is the opened plaintext's token ‖ amount, 32 bytes each.
func (o noteOpening) tokenAmount() (token, amount *big.Int) {
	return new(big.Int).SetBytes(o.plain[:32]), new(big.Int).SetBytes(o.plain[32:])
}

// paymentEventsUpTo is every Payment event EnygmaDvp has emitted up to block.
func paymentEventsUpTo(t *testing.T, ctx context.Context, client *ethclient.Client, dvp common.Address, block *big.Int) []dvpcore.OnChainErc20Event {
	t.Helper()
	logs, err := client.FilterLogs(ctx, ethereum.FilterQuery{Addresses: []common.Address{dvp},
		Topics: [][]common.Hash{{paymentEventSig}}, ToBlock: block})
	if err != nil {
		t.Fatal(err)
	}
	logPtrs := make([]*types.Log, len(logs))
	for i := range logs {
		logPtrs[i] = &logs[i]
	}
	return paymentEventsIn(t, logPtrs, dvp)
}

// poseidonOf is Poseidon of xs, failing the test on an error.
func poseidonOf(t *testing.T, xs ...*big.Int) *big.Int {
	t.Helper()
	h, err := poseidon.Hash(xs)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// pathNodes hashes leaf up its Merkle path, Poseidon(left, right) at each
// level with the leaf index's bit choosing the side; nodes[k] is level k's
// output, the last one the root.
func pathNodes(t *testing.T, leaf *big.Int, mp *rpcore.MerkleProof) []*big.Int {
	t.Helper()
	nodes := make([]*big.Int, primerDepth)
	node := leaf
	for k := range nodes {
		left, right := node, mp.Elements[k]
		if mp.Indices.Bit(k) == 1 {
			left, right = right, left
		}
		node = poseidonOf(t, left, right)
		nodes[k] = node
	}
	return nodes
}

// checkOneNullifierEvent fails unless logs hold exactly one Nullifier event
// of vault, for vaultID, tree and nf.
func checkOneNullifierEvent(t *testing.T, logs []*types.Log, vault common.Address, vaultID, tree, nf *big.Int) {
	t.Helper()
	var logged []*big.Int
	for _, l := range logs {
		if l.Address == vault && len(l.Topics) == 4 && l.Topics[0] == nullifierEventSig {
			if l.Topics[1].Big().Cmp(vaultID) != 0 || l.Topics[2].Big().Cmp(tree) != 0 {
				t.Errorf("Nullifier event for vault %s tree %s, want %s and %s", l.Topics[1].Big(), l.Topics[2].Big(), vaultID, tree)
			}
			logged = append(logged, l.Topics[3].Big())
		}
	}
	if len(logged) != 1 || logged[0].Cmp(nf) != 0 {
		t.Errorf("the settlement's Nullifier events %v, want one, %s", logged, nf)
	}
}

// paymentVKSlot is the Verifier's slot for the Payment circuit's key
// (Erc20CoinVault.VK_ID_ERC20_JOINSPLIT; scripts/cmd/init registers
// build/Payment.json first).
const paymentVKSlot = 0

// paymentVK is the Payment circuit's Groth16 verifying key.
type paymentVK struct {
	alpha              bn254.G1Affine
	beta, gamma, delta bn254.G2Affine
	ic                 []bn254.G1Affine
}

// loadPaymentVK reads build/Payment.json, the verifying key init registered
// with the Verifier (G2 coordinates in EIP-197 order: imaginary first).
func loadPaymentVK(t *testing.T) paymentVK {
	t.Helper()
	raw, err := os.ReadFile("../build/Payment.json")
	if err != nil {
		t.Fatal(err)
	}
	var j struct {
		Alpha []string   `json:"vk_alpha_1"`
		Beta  [][]string `json:"vk_beta_2"`
		Gamma [][]string `json:"vk_gamma_2"`
		Delta [][]string `json:"vk_delta_2"`
		IC    [][]string `json:"IC"`
	}
	if err := json.Unmarshal(raw, &j); err != nil {
		t.Fatal(err)
	}
	// num parses a decimal coordinate.
	num := func(s string) *big.Int {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			t.Fatalf("bad coordinate %q", s)
		}
		return n
	}
	g1 := func(p []string) bn254.G1Affine { return g1Of(G1Point{X: num(p[0]), Y: num(p[1])}) }
	g2 := func(p [][]string) bn254.G2Affine {
		return g2Of(G2Point{X: [2]*big.Int{num(p[0][0]), num(p[0][1])}, Y: [2]*big.Int{num(p[1][0]), num(p[1][1])}})
	}
	vk := paymentVK{alpha: g1(j.Alpha), beta: g2(j.Beta), gamma: g2(j.Gamma), delta: g2(j.Delta)}
	for _, p := range j.IC {
		vk.ic = append(vk.ic, g1(p))
	}
	return vk
}

// paymentPK is what the primer reads of the Payment circuit's proving key:
// the QAP's evaluation domain, the α, β, δ it shares with the verifying
// key, the private wires' K points, the wire count and the number of
// commitment keys (gnark's commitment extension; none here).
type paymentPK struct {
	domain        fft.Domain
	alpha         bn254.G1Affine
	beta2, delta2 bn254.G2Affine
	k             []bn254.G1Affine
	nbWires       uint64
	commitments   uint32
}

// readPaymentPK decodes the gnark server's proving key
// (gnark_circuits/scripts/keys/PaymentPK.key) as far as the commitment-key
// count. The field order copies gnark's ProvingKey.writeTo (backend/groth16/
// bn254/marshal.go): the test module has gnark-crypto but not gnark. A
// layout change would fail the Z-count check here and the α, β, δ check in
// the proof check, not pass silently.
func readPaymentPK(t *testing.T) paymentPK {
	t.Helper()
	f, err := os.Open("../gnark_circuits/scripts/keys/PaymentPK.key")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var pk paymentPK
	if _, err := pk.domain.ReadFrom(f); err != nil {
		t.Fatalf("the proving key's domain: %v", err)
	}
	var beta1, delta1 bn254.G1Affine
	var a, b, z []bn254.G1Affine
	var b2 []bn254.G2Affine
	var nbInfA, nbInfB uint64
	var infA, infB []bool
	dec := bn254.NewDecoder(f)
	for _, v := range []any{&pk.alpha, &beta1, &delta1, &a, &b, &z, &pk.k, &pk.beta2, &pk.delta2, &b2, &pk.nbWires, &nbInfA, &nbInfB, &infA, &infB, &pk.commitments} {
		if err := dec.Decode(v); err != nil {
			t.Fatalf("the proving key: %v", err)
		}
	}
	if uint64(len(z)) != pk.domain.Cardinality-1 {
		t.Errorf("the proving key has %d Z points for a domain of %d", len(z), pk.domain.Cardinality)
	}
	return pk
}

// g1Of and g2Of turn the ABI's points into gnark-crypto's (a G2 coordinate
// is [imaginary, real], as EIP-197 orders it).
func g1Of(p G1Point) bn254.G1Affine {
	var q bn254.G1Affine
	q.X.SetBigInt(p.X)
	q.Y.SetBigInt(p.Y)
	return q
}

func g2Of(p G2Point) bn254.G2Affine {
	var q bn254.G2Affine
	q.X.A1.SetBigInt(p.X[0])
	q.X.A0.SetBigInt(p.X[1])
	q.Y.A1.SetBigInt(p.Y[0])
	q.Y.A0.SetBigInt(p.Y[1])
	return q
}

// g1String and g2String show a point as the primer does: (x, y), a G2
// coordinate as re + im·u in 𝔽_q[u]/(u² + 1).
func g1String(p bn254.G1Affine) string {
	return fmt.Sprintf("(%s, %s)", p.X.String(), p.Y.String())
}

func g2String(p bn254.G2Affine) string {
	return fmt.Sprintf("(%s + %s·u, %s + %s·u)", p.X.A0.String(), p.X.A1.String(), p.Y.A0.String(), p.Y.A1.String())
}

// paymentEventsIn decodes EnygmaDvp's Payment events (vaultId and the
// commitment indexed; the capsule ctxt and the sealed payload encTxData as
// data) among logs.
func paymentEventsIn(t *testing.T, logs []*types.Log, dvp common.Address) []dvpcore.OnChainErc20Event {
	t.Helper()
	bytesT, _ := abi.NewType("bytes", "", nil)
	args := abi.Arguments{{Type: bytesT}, {Type: bytesT}}
	var events []dvpcore.OnChainErc20Event
	for _, l := range logs {
		if l.Address != dvp || len(l.Topics) != 3 || l.Topics[0] != paymentEventSig {
			continue
		}
		vals, err := args.Unpack(l.Data)
		if err != nil {
			t.Fatalf("decode Payment event: %v", err)
		}
		events = append(events, dvpcore.OnChainErc20Event{Commitment: l.Topics[2].Big(), CipherText: vals[0].([]byte), EncTxData: vals[1].([]byte)})
	}
	return events
}

// vaultView calls the one-value view method of vault at block n.
func vaultView(t *testing.T, vault *bind.BoundContract, n *big.Int, method string, args ...any) any {
	t.Helper()
	var out []any
	if err := vault.Call(&bind.CallOpts{BlockNumber: n}, &out, method, args...); err != nil || len(out) != 1 {
		t.Fatalf("%s: %v (%d values)", method, err, len(out))
	}
	return out[0]
}

// boolInt is 1 for true, 0 for false.
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// rebuildVaultTree rebuilds the vault's Merkle trees with the client library
// from its Commitment events up to block to, a transaction's commitments
// inserted as one batch, as Merkle.sol does (a batch that would reach a
// tree's last slot opens the next tree).
func rebuildVaultTree(t *testing.T, client *ethclient.Client, vault common.Address, to *big.Int) *rpcore.MerkleTree {
	t.Helper()
	logs, err := client.FilterLogs(context.Background(), ethereum.FilterQuery{Addresses: []common.Address{vault},
		Topics: [][]common.Hash{{commitmentEventSig}}, ToBlock: to})
	if err != nil {
		t.Fatal(err)
	}
	mt := rpcore.NewMerkleTree(primerDepth)
	for i := 0; i < len(logs); {
		var batch []*big.Int
		for j := i; j < len(logs) && logs[j].TxHash == logs[i].TxHash; j++ {
			batch = append(batch, logs[j].Topics[2].Big())
		}
		mt.InsertLeaves(batch)
		i += len(batch)
	}
	return mt
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

// calldataOf reads the transaction txHash back from the chain and returns it,
// its signer and its calldata's arguments, failing unless wantFrom sent it
// to wantTo calling method of a.
func calldataOf(t *testing.T, client *ethclient.Client, a abi.ABI, txHash common.Hash, method string, wantFrom, wantTo common.Address) (*types.Transaction, common.Address, []any) {
	t.Helper()
	tx, _, err := client.TransactionByHash(context.Background(), txHash)
	if err != nil {
		t.Fatal(err)
	}
	from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
	if err != nil {
		t.Fatal(err)
	}
	if from != wantFrom || *tx.To() != wantTo {
		t.Errorf("%s sent by %s to %s, want %s to %s", method, from.Hex(), tx.To().Hex(), wantFrom.Hex(), wantTo.Hex())
	}
	m, err := a.MethodById(tx.Data()[:4])
	if err != nil || m.Name != method {
		t.Fatalf("tx %s calls %v (err %v), want %s", txHash.Hex(), m, err, method)
	}
	args, err := m.Inputs.Unpack(tx.Data()[4:])
	if err != nil {
		t.Fatal(err)
	}
	return tx, from, args
}

// scanTagChannels has the holder of dk try every record of
// TagChannelRegistry with the client library, failing the test unless it
// opens tag channel #want alone. It returns the record count and what opened.
func scanTagChannels(t *testing.T, client *ethclient.Client, receipts map[string]onchainReceiptEntry, dk *mlkem.DecapsulationKey768, who string, want uint64) (uint64, []tags.FoundChannel) {
	t.Helper()
	addr := common.HexToAddress(receipts["TagChannelRegistry"].ContractAddress)
	records, err := tags.GetChannelCount(client, addr)
	if err != nil {
		t.Fatal(err)
	}
	found, err := tags.ScanChannels(client, addr, dk, 0, records)
	if err != nil || len(found) != 1 || found[0].ChannelIdx != want {
		t.Fatalf("%s opened %v of %d records (err %v), want tag channel #%d alone", who, found, records, err, want)
	}
	return records, found
}

// channelKeyOf is a tag channel's k_ch, HKDF-SHA256(ss, "enygma-channel-key-v1").
func channelKeyOf(t *testing.T, ss []byte) []byte {
	t.Helper()
	k, err := hkdf.Key(sha256.New, ss, nil, "enygma-channel-key-v1", 32)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// toField reduces big-endian bytes b into BN254's scalar field.
func toField(b []byte) *big.Int {
	return new(big.Int).Mod(new(big.Int).SetBytes(b), constants.Q)
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
// with PRIMER_WRITE=1, once t has passed and every one of primerChecks has
// run and passed; any other run leaves the page as it was.
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
	if os.Getenv("PRIMER_WRITE") != "1" {
		t.Log("primer data not written: set PRIMER_WRITE=1 to rewrite it (make primer-refresh does)")
		return
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
