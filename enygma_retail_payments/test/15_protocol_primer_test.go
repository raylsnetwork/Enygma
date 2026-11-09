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
// account), its Groth16 proof opened up (L and the pairing check, and the
// Verifier's answer), and Ana's discovery of her note (her scan of the
// Payment events).
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
var primerChecks = []string{"keys", "auditor", "registration", "shield", "payment", "proof", "discovery"}

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

	check("discovery", "Ana trial-decrypts every Payment event's ciphertext with her view key, opens exactly her note, and its opening hashes to its commitment", func(t *testing.T) {
		if paid == nil {
			t.Fatal("no note paid to Ana: the payment check did not pass")
		}
		sender, ana := users[0], users[1]
		dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)

		// Every Payment event EnygmaDvp has emitted, up to the payment's block:
		// Ana's wallet cannot tell which is hers, so it reads them all.
		logs, err := client.FilterLogs(ctx, ethereum.FilterQuery{Addresses: []common.Address{dvpAddr},
			Topics: [][]common.Hash{{paymentEventSig}}, ToBlock: paid.block})
		if err != nil {
			t.Fatal(err)
		}
		logPtrs := make([]*types.Log, len(logs))
		for i := range logs {
			logPtrs[i] = &logs[i]
		}
		events := paymentEventsIn(t, logPtrs, dvpAddr)
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

		// open is one trial decryption, as Ana's wallet makes it, with the
		// standard library: Decaps, HKDF-SHA256 to the key, AES-256-GCM open
		// with no associated data. ok is false when the tag does not check.
		type opening struct {
			ss, saltBytes, key, nonce, sealed, plain []byte
			ok                                       bool
		}
		open := func(dk *mlkem.DecapsulationKey768, capsule, payload []byte) opening {
			t.Helper()
			var got opening
			if len(capsule) != mlkem.CiphertextSize768 || len(payload) < 12 {
				return got // not an ML-KEM-768 capsule and a sealed payload: not anyone's note (the library's scan stops here instead; checked below)
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

		// The encryption, undone with Ana's view key: the capsule gives back
		// the Sender's shared secret, whose HKDF outputs are the note's salt
		// and the payload's key; the payload is token ‖ amount.
		if len(capsule) != mlkem.CiphertextSize768 {
			t.Errorf("capsule is %d bytes, want %d", len(capsule), mlkem.CiphertextSize768)
		}
		anaOpen := open(ana.view.DecapsKey, capsule, payload)
		if !anaOpen.ok {
			t.Fatal("Ana's view key does not open the Payment event addressed to her")
		}
		if len(anaOpen.ss) != mlkem.SharedKeySize || len(anaOpen.sealed) != 64+16 || len(anaOpen.plain) != 64 {
			t.Errorf("ss %d bytes, ct‖tag %d, plaintext %d: want 32, 80 and 64", len(anaOpen.ss), len(anaOpen.sealed), len(anaOpen.plain))
		}
		token, amount := new(big.Int).SetBytes(anaOpen.plain[:32]), new(big.Int).SetBytes(anaOpen.plain[32:])
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
			if open(ana.view.DecapsKey, e.CipherText, e.EncTxData).ok {
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
		if senderTry := open(sender.view.DecapsKey, capsule, payload); senderTry.ok || bytes.Equal(senderTry.ss, anaOpen.ss) {
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
		forgedOpen := open(ana.view.DecapsKey, capsule, forged)
		if !forgedOpen.ok {
			t.Fatal("the forged payload, sealed under Ana's key, does not open")
		}
		claimed := new(big.Int).SetBytes(forgedOpen.plain[32:])
		if h, _ := poseidon.Hash([]*big.Int{ana.spend.PublicKey, salt, claimed, token}); h.Cmp(mine.Commitment) == 0 {
			t.Error("a payload claiming another amount hashes to the published commitment")
		}
		if owned, err := dvpcore.ScanForErc20Notes(ana.view.DecapsKey, ana.spend.PublicKey,
			[]dvpcore.OnChainErc20Event{{Commitment: mine.Commitment, CipherText: capsule, EncTxData: forged}}); err == nil || len(owned) != 0 {
			t.Errorf("ScanForErc20Notes accepted the mismatched ciphertext: %v", owned)
		}
		// A capsule of the wrong length stops the library's scan too, for any
		// wallet: Decaps rejects it, and the scan returns the error instead
		// of skipping the event. The page names this stall.
		short := dvpcore.OnChainErc20Event{Commitment: big.NewInt(1), CipherText: capsule[:len(capsule)-1], EncTxData: payload}
		if _, err := dvpcore.ScanForErc20Notes(sender.view.DecapsKey, sender.spend.PublicKey, []dvpcore.OnChainErc20Event{*mine, short}); err == nil {
			t.Error("ScanForErc20Notes skipped a capsule of the wrong length; the page says it stops the scan")
		}

		data.Steps["discovery"] = primerEntry{Values: map[string]string{
			"anaEk":      hexutil.Encode(ana.view.EncapsKey),
			"capsule":    hexutil.Encode(capsule),
			"ss":         hexutil.Encode(anaOpen.ss),
			"saltBytes":  hexutil.Encode(anaOpen.saltBytes),
			"salt":       salt.String(),
			"key":        hexutil.Encode(anaOpen.key),
			"nonce":      hexutil.Encode(anaOpen.nonce),
			"sealed":     hexutil.Encode(anaOpen.sealed),
			"encTxData":  hexutil.Encode(payload),
			"plaintext":  hexutil.Encode(anaOpen.plain),
			"token":      token.String(),
			"amount":     amount.String(),
			"pkSpend":    ana.spend.PublicKey.String(),
			"commitment": commitment.String(),
			"scanned":    fmt.Sprint(len(events)),
			"opened":     fmt.Sprint(opened),
			"forged":     claimed.String(),
		}, Ledger: []ledgerItem{}}
	})
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
