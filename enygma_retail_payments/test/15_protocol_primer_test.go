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
// auditor's keys, the registration of both users, the Sender's shield, and
// the Sender's payment to Ana (one Payment proof, sent from the Sender's own
// account).
//
// Prerequisites: the stack shield_demo/run.sh --no-demo brings up (the
// chain and a deployment, and the gnark server on :8082 for the payment's
// proof; this test does not call the relayer).
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
	"maps"
	"math/big"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"enygma_retail_payments/layout"

	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

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
var primerChecks = []string{"keys", "auditor", "registration", "shield", "payment"}

// shieldAmount is what the Sender shields: public RaylsERC20 the issuer
// mints to them, deposited whole into one note.
const shieldAmount = 100

// primerDepth is the vault's Merkle tree depth (scripts/cmd/init).
const primerDepth = 8

// payAmount is what the Sender pays Ana out of the shielded note; the rest
// is the Sender's change.
const payAmount = 60

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
type shieldedNote struct{ commitment, salt, amount *big.Int }

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

	// shielded is the note the shield made, which the payment spends.
	var shielded *shieldedNote

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
		shielded = &shieldedNote{commitment: commitment, salt: cdSalt, amount: cdAmount}
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
		hash := func(xs ...*big.Int) *big.Int {
			t.Helper()
			h, err := poseidon.Hash(xs)
			if err != nil {
				t.Fatal(err)
			}
			return h
		}
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
		node := cIn
		for k := 0; k < primerDepth; k++ {
			bit, sib := mp.Indices.Bit(k), mp.Elements[k]
			left, right := node, sib
			if bit == 1 {
				left, right = sib, node
			}
			node = hash(left, right)
			p := fmt.Sprintf("path%d.", k)
			v[p+"bit"], v[p+"sibling"], v[p+"node"] = fmt.Sprint(bit), sib.String(), node.String()
		}
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
		var nfLogged []*big.Int
		for _, l := range mined.Logs {
			if l.Address == vaultAddr && len(l.Topics) == 4 && l.Topics[0] == nullifierEventSig {
				if l.Topics[1].Big().Cmp(vaultID) != 0 || l.Topics[2].Big().Cmp(tree) != 0 {
					t.Errorf("Nullifier event for vault %s tree %s, want %s and %s", l.Topics[1].Big(), l.Topics[2].Big(), vaultID, tree)
				}
				nfLogged = append(nfLogged, l.Topics[3].Big())
			}
		}
		if len(nfLogged) != 1 || nfLogged[0].Cmp(nf) != 0 {
			t.Errorf("the settlement's Nullifier events %v, want one, %s", nfLogged, nf)
		}
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
		var paid []*big.Int
		for _, l := range mined.Logs {
			if l.Address == dvpAddr && len(l.Topics) >= 3 && l.Topics[0] == paymentEventSig {
				paid = append(paid, l.Topics[2].Big())
			}
		}
		if len(paid) != 1 || paid[0].Cmp(c0) != 0 {
			t.Errorf("Payment events %v, want one, for Ana's commitment", paid)
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
		)
		data.Steps["payment"] = primerEntry{Values: v, Ledger: ledger}
	})
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
