package tests

// Checks of the shield demo's secret-value hop details (#172): the keys
// each actor made, what the UserRegistry received, the view keys the
// auditor recovered, and the notes each participant and the auditor
// decrypted. TestShieldDemoScenario and TestShieldDemoWalkthrough call them.

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"enygma_retail_payments/shield_demo/engine"
	dvpcore "github.com/raylsnetwork/enygma_dvp/src/core"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// hexBytes decodes a hop detail's 0x-hex byte string.
func hexBytes(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	if err != nil || !strings.HasPrefix(s, "0x") {
		t.Fatalf("hop detail value %q is not 0x-hex bytes", s)
	}
	return b
}

// checkDiscoverDetails checks that each participant's discovery carries one
// decryption hop detail, on a hop to them, opening the one note they were
// paid in the fan-out: its amount is their payout, its opening hashes to the
// commitment their payment's transaction inserted in the vault, and its
// ciphertext is the one on chain: with the view key, a Payment event's
// capsule and sealed amount that the participant's view key opens to the
// same note; with tags, the payload of their payment's TagPublished event.
func checkDiscoverDetails(t *testing.T, client *ethclient.Client, e *engine.Engine, hops []engine.Event, tagged bool) {
	t.Helper()
	receipts := loadOnchainReceipts(t)
	vaultAddr := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	paid := map[string]engine.Settlement{}
	for _, s := range e.FanOut() {
		paid[s.To] = s
	}
	byNode := map[string]engine.Actor{}
	for _, p := range e.Participants() {
		byNode[engine.ActorNode(p.ID)] = p
	}
	opened := map[string]int{}
	for _, h := range hops {
		if h.Step != "discover" {
			continue
		}
		for _, d := range detailsOf(h, engine.DetailDecryption) {
			p, ok := byNode[h.To]
			if !ok {
				t.Errorf("discover hop %q to %s carries a decryption detail, want it only on a hop to a participant", h.Label, h.To)
				continue
			}
			if len(d.Notes) != 1 {
				t.Errorf("%s's decryption detail opens %d notes, want the one payment of the fan-out", p.Name, len(d.Notes))
				continue
			}
			n, s := d.Notes[0], paid[p.Name]
			opened[p.Name]++
			if n.Owner != p.Name {
				t.Errorf("%s's decryption detail names owner %q", p.Name, n.Owner)
			}
			amount, ok := new(big.Int).SetString(n.Amount, 10)
			if !ok || s.Amount == nil || amount.Cmp(s.Amount) != 0 {
				t.Errorf("%s's discovered opening has amount %q, their payout was %v", p.Name, n.Amount, s.Amount)
				continue
			}
			salt, token, commitment := hexField(t, n.Salt), hexField(t, n.Token), hexField(t, n.Commitment)
			cm, err := rpcore.Erc20CommitmentV2(p.Spend.PublicKey, salt, amount, token)
			if err != nil || cm.Cmp(commitment) != 0 {
				t.Errorf("%s's discovered opening (pk_spend, salt, %s, token) hashes to 0x%x, not its commitment %s (err %v)", p.Name, amount, cm, n.Commitment, err)
			}
			inserted := false
			for _, c := range commitmentsIn(txLogs(t, client, s.TxHash), vaultAddr) {
				inserted = inserted || c.Cmp(commitment) == 0
			}
			if !inserted {
				t.Errorf("%s's discovered commitment %s is not one their payment %s inserted in the vault", p.Name, n.Commitment, s.TxHash.Hex())
			}
			if tagged {
				checkTagPayload(t, client, p.Name, s.TagTxHash, n)
				continue
			}
			if n.Via != "view key" || n.Capsule == "" {
				t.Errorf("%s's discovered note came via %q with capsule %q, want the view key's capsule", p.Name, n.Via, n.Capsule)
				continue
			}
			owned, err := dvpcore.ScanForErc20Notes(p.View.DecapsKey, p.Spend.PublicKey, []dvpcore.OnChainErc20Event{
				{Commitment: commitment, CipherText: hexBytes(t, n.Capsule), EncTxData: hexBytes(t, n.Sealed)}})
			if err != nil || len(owned) != 1 || owned[0].Amount.Cmp(amount) != 0 || owned[0].SaltBField.Cmp(salt) != 0 {
				t.Errorf("%s's view key does not open the ciphertext shown to the opening shown (err %v)", p.Name, err)
			}
			if !noteEventCarries(t, client, s.TxHash, commitment, hexBytes(t, n.Capsule), hexBytes(t, n.Sealed)) {
				t.Errorf("%s's discovered ciphertext is not the one their payment's Payment event published", p.Name)
			}
		}
	}
	for _, p := range e.Participants() {
		if opened[p.Name] != 1 {
			t.Errorf("%s's discovery carries %d decryption details, want 1", p.Name, opened[p.Name])
		}
	}
}

// checkTagPayload checks that a tag-discovered note's ciphertext is the
// payload its payment's TagPublished event published.
func checkTagPayload(t *testing.T, client *ethclient.Client, name string, tagTx common.Hash, n engine.DecryptedNote) {
	t.Helper()
	if n.Via != "tag" || n.Capsule != "" {
		t.Errorf("%s's discovered note came via %q with capsule %q, want a tag's payload alone", name, n.Via, n.Capsule)
		return
	}
	bytesT, _ := abi.NewType("bytes", "", nil)
	for _, l := range txLogs(t, client, tagTx) {
		if len(l.Topics) == 0 || l.Topics[0] != tagPublishedSig {
			continue
		}
		vals, err := abi.Arguments{{Type: bytesT}}.Unpack(l.Data)
		if err == nil && len(vals) == 1 && bytes.Equal(vals[0].([]byte), hexBytes(t, n.Sealed)) {
			return
		}
	}
	t.Errorf("%s's discovered tag payload is not the one their payment's tag transaction %s published", name, tagTx.Hex())
}

// noteEventCarries reports whether tx logged a Payment or EncryptedNote
// event for commitment with capsule and sealed as its ciphertexts.
func noteEventCarries(t *testing.T, client *ethclient.Client, tx common.Hash, commitment *big.Int, capsule, sealed []byte) bool {
	t.Helper()
	bytesT, _ := abi.NewType("bytes", "", nil)
	for _, l := range txLogs(t, client, tx) {
		if len(l.Topics) < 3 || (l.Topics[0] != paymentEventSig && l.Topics[0] != encryptedNoteSig) || l.Topics[2].Big().Cmp(commitment) != 0 {
			continue
		}
		vals, err := abi.Arguments{{Type: bytesT}, {Type: bytesT}}.Unpack(l.Data)
		if err == nil && len(vals) == 2 && bytes.Equal(vals[0].([]byte), capsule) && bytes.Equal(vals[1].([]byte), sealed) {
			return true
		}
	}
	return false
}

// checkShortened checks that a key detail's private half is the key, in
// 0x-hex, shortened: its first 6 and last 4 hex digits, never in full.
func checkShortened(t *testing.T, what, got, full string) {
	t.Helper()
	if want := full[:8] + "…" + full[len(full)-4:]; got != want {
		t.Errorf("%s's private half shows %q, want the key shortened to %q", what, got, want)
	}
}

// checkKeyDetails checks that keygen's key hops show each actor's spend
// and view key pairs, and the auditor's with auditor set (a walkthrough's):
// the public halves in full, as the actors hold them, and the private
// halves shortened.
func checkKeyDetails(t *testing.T, e *engine.Engine, hops []engine.Event, auditor bool) {
	t.Helper()
	type pair struct{ scheme, public, private string }
	want := map[string]pair{}
	for _, a := range append([]engine.Actor{e.Issuer(), e.Sender()}, e.Participants()...) {
		want[a.Name+" spend key"] = pair{"Poseidon", fmt.Sprintf("0x%064x", a.Spend.PublicKey), fmt.Sprintf("0x%064x", a.Spend.PrivateKey)}
		want[a.Name+" view key"] = pair{"ML-KEM-768", "0x" + hex.EncodeToString(a.View.EncapsKey), "0x" + hex.EncodeToString(a.View.DecapsKey.Bytes())}
	}
	if auditor {
		want["Auditor auditor key"] = pair{"ML-KEM-768", "0x" + hex.EncodeToString(e.Auditor().EncapsKey), "0x" + hex.EncodeToString(e.Auditor().DecapsKey.Bytes())}
	}
	seen := map[string]int{}
	for _, h := range hops {
		for _, d := range detailsOf(h, engine.DetailKey) {
			k := d.Key
			if h.Step != "keygen" || h.From != h.To || k == nil {
				t.Errorf("step %s: key detail on hop %q %s → %s, want it only on a keygen hop from an actor to itself", h.Step, h.Label, h.From, h.To)
				continue
			}
			id := k.Owner + " " + k.Name
			w, ok := want[id]
			if !ok {
				t.Errorf("key detail for %q, no such key", id)
				continue
			}
			seen[id]++
			if k.Scheme != w.scheme || k.Public != w.public {
				t.Errorf("%s: shows scheme %s public %.20s…, want %s %.20s…", id, k.Scheme, k.Public, w.scheme, w.public)
			}
			checkShortened(t, id, k.Private, w.private)
		}
	}
	for id := range want {
		if seen[id] != 1 {
			t.Errorf("%s: %d key details, want 1", id, seen[id])
		}
	}
}

// checkRegistryDetails checks that each registration hop shows what the
// UserRegistry holds for that user, and that each view-key recovery hop
// shows the audit ciphertexts it holds and the user's view key, its public
// half the registered one and its private half shortened.
func checkRegistryDetails(t *testing.T, client *ethclient.Client, e *engine.Engine, hops []engine.Event) {
	t.Helper()
	registryAddr := common.HexToAddress(loadOnchainReceipts(t)["UserRegistry"].ContractAddress)
	users := map[string]engine.Actor{}
	for _, a := range append([]engine.Actor{e.Sender()}, e.Participants()...) {
		users[a.Name] = a
	}
	type held struct {
		spendPk, viewPk string
		audit           engine.AuditCiphertexts
	}
	onChain := func(a engine.Actor) held {
		keys, err := rpcore.LookupKeys(client, registryAddr, a.Address)
		if err != nil {
			t.Fatalf("%s: LookupKeys: %v", a.Name, err)
		}
		mlKemCt, aesCt, err := rpcore.LookupAuditCiphertexts(client, registryAddr, a.Address)
		if err != nil {
			t.Fatalf("%s: LookupAuditCiphertexts: %v", a.Name, err)
		}
		return held{fmt.Sprintf("0x%064x", keys.SpendKey), "0x" + hex.EncodeToString(keys.ViewKey),
			engine.AuditCiphertexts{Capsule: "0x" + hex.EncodeToString(mlKemCt), Sealed: "0x" + hex.EncodeToString(aesCt)}}
	}
	registered, recovered := map[string]int{}, map[string]int{}
	for _, h := range hops {
		for _, d := range detailsOf(h, engine.DetailRegistration) {
			r := d.Registration
			a, ok := users[r.Owner]
			if !ok || h.To != engine.NodeUserRegistry || h.From != engine.ActorNode(a.ID) {
				t.Errorf("registration detail for %q on hop %s → %s, want it on that user's hop to the UserRegistry", r.Owner, h.From, h.To)
				continue
			}
			registered[r.Owner]++
			if w := onChain(a); r.SpendPk != w.spendPk || r.ViewPk != w.viewPk || r.Audit != w.audit {
				t.Errorf("%s: registration detail differs from what the UserRegistry holds", r.Owner)
			}
		}
		for _, d := range detailsOf(h, engine.DetailViewKeyRecovery) {
			r := d.Recovery
			a, ok := users[r.Owner]
			if !ok || h.From != engine.NodeUserRegistry || h.To != engine.NodeAuditor {
				t.Errorf("view key recovery detail for %q on hop %s → %s, want it on a hop from the UserRegistry to the auditor", r.Owner, h.From, h.To)
				continue
			}
			recovered[r.Owner]++
			if w := onChain(a); r.Audit != w.audit || r.ViewPk != w.viewPk {
				t.Errorf("%s: recovery detail's ciphertexts or view key differ from what the UserRegistry holds", r.Owner)
			}
			checkShortened(t, r.Owner+"'s recovered view key", r.ViewKey, "0x"+hex.EncodeToString(a.View.DecapsKey.Bytes()))
		}
	}
	for name := range users {
		if registered[name] != 1 || recovered[name] != 1 {
			t.Errorf("%s: %d registration and %d view key recovery details, want 1 each", name, registered[name], recovered[name])
		}
	}
}

// checkAuditDetails checks that the audit step's hop to the auditor shows
// every note the auditor decrypted (each audit entry but the USDr fee legs,
// which carry no ciphertext): its owner, amount and transaction those of an
// entry, its opening hashing to its commitment under the owner's spend key,
// and its ciphertext the one the transaction logged, which the owner's own
// view key opens to the same opening.
func checkAuditDetails(t *testing.T, client *ethclient.Client, e *engine.Engine, hops []engine.Event) {
	t.Helper()
	a, err := e.Audit(context.Background())
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	type key struct{ tx, owner, amount string }
	want := map[key]int{}
	for _, en := range a.Entries {
		if en.Kind != engine.AuditUsdrFee {
			want[key{strings.ToLower(en.TxHash.Hex()), en.To, en.Amount.String()}]++
		}
	}
	users := map[string]engine.Actor{}
	for _, u := range append([]engine.Actor{e.Sender()}, e.Participants()...) {
		users[u.Name] = u
	}
	var notes []engine.DecryptedNote
	for _, h := range hops {
		for _, d := range detailsOf(h, engine.DetailDecryption) {
			if h.Step == "audit" {
				if h.From != engine.NodeVault || h.To != engine.NodeAuditor {
					t.Errorf("audit decryption detail on hop %s → %s, want the vault → auditor hop", h.From, h.To)
				}
				notes = append(notes, d.Notes...)
			}
		}
	}
	if len(notes) != len(a.Entries)-countFeeLegs(a) {
		t.Errorf("the audit hop shows %d decrypted notes, want every audited note but the fee legs (%d)", len(notes), len(a.Entries)-countFeeLegs(a))
	}
	for _, n := range notes {
		k := key{strings.ToLower(n.Tx), n.Owner, n.Amount}
		if want[k] == 0 {
			t.Errorf("the audit hop shows %s's note of %s in tx %s, no audit entry left matches it", n.Owner, n.Amount, n.Tx)
			continue
		}
		want[k]--
		u, ok := users[n.Owner]
		if !ok {
			t.Errorf("audited note owned by %q, not a registered user", n.Owner)
			continue
		}
		amount, _ := new(big.Int).SetString(n.Amount, 10)
		commitment, salt := hexField(t, n.Commitment), hexField(t, n.Salt)
		cm, err := rpcore.Erc20CommitmentV2(u.Spend.PublicKey, salt, amount, hexField(t, n.Token))
		if err != nil || cm.Cmp(commitment) != 0 {
			t.Errorf("%s's audited opening of %s hashes to 0x%x, not its commitment %s", n.Owner, n.Amount, cm, n.Commitment)
		}
		if n.Via != "view key" {
			t.Errorf("%s's audited note came via %q, want the view key", n.Owner, n.Via)
			continue
		}
		capsule, sealed := hexBytes(t, n.Capsule), hexBytes(t, n.Sealed)
		owned, err := dvpcore.ScanForErc20Notes(u.View.DecapsKey, u.Spend.PublicKey, []dvpcore.OnChainErc20Event{{Commitment: commitment, CipherText: capsule, EncTxData: sealed}})
		if err != nil || len(owned) != 1 || owned[0].Amount.Cmp(amount) != 0 || owned[0].SaltBField.Cmp(salt) != 0 {
			t.Errorf("%s's own view key does not open the audited ciphertext to the opening shown (err %v)", n.Owner, err)
		}
		if !noteEventCarries(t, client, common.HexToHash(n.Tx), commitment, capsule, sealed) {
			t.Errorf("%s's audited ciphertext is not one tx %s logged", n.Owner, n.Tx)
		}
	}
}

// countFeeLegs counts an audit's USDr fee legs.
func countFeeLegs(a *engine.Audit) int {
	n := 0
	for _, en := range a.Entries {
		if en.Kind == engine.AuditUsdrFee {
			n++
		}
	}
	return n
}
