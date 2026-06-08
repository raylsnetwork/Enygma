// # Protocol overview
//
// After two users establish a channel (Phase 2 of the retail payment protocol),
// they share a common sharedSecret. They can use this secret to attach private
// messaging tags to any on-chain transaction, allowing the recipient to scan
// efficiently without trying to decrypt every transaction.
//
// # Tag derivation (Poseidon over BN254)
//
//	ss_field = sharedSecret mod BN254_p               (reduce to field element)
//	t        = Poseidon(blockNumber, pkRecipient, ss_field)
//
// Poseidon is the ZK-friendly hash used throughout Enygma (commitments,
// nullifiers, public keys). Using it here keeps the tag computable inside a ZK
// circuit if needed in the future, and is consistent with the paper's suggestion
// of "secure hashing (e.g., SHA3)" — the paper leaves TH abstract.
//
// # Payload encryption
//
//	channelKey = HKDF-SHA256(sharedSecret, salt=nil, info="enygma-channel-key-v1")
//	ctxt       = AES-256-GCM(key=channelKey, plaintext=payload)
//
// # Sender workflow (via relayer for sender privacy)
//
//	tag,  err := DeriveTag(blockNumber, bob.PkSpend, sharedSecret)
//	ctxt, err := EncryptPayload(DeriveChannelKey(sharedSecret), payload)
//	// POST /relay/tag { tag: hex(tag), ctxt: hex(ctxt) }
//
// # Recipient scanning
//
//	for each block:
//	    matched, err := ScanBlock(client, registryAddr, blockNumber, channels)
//	    for _, m := range matched:
//	        payload, err := DecryptPayload(DeriveChannelKey(m.SharedSecret), m.Entry.Ctxt)
package tags

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/iden3/go-iden3-crypto/poseidon"
	"golang.org/x/crypto/hkdf"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// bn254ScalarField is the BN254 scalar field prime p.
// All Poseidon inputs must be reduced mod p.
var bn254ScalarField, _ = new(big.Int).SetString(
	"21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)

// ── Types ─────────────────────────────────────────────────────────────────────

// TagEntry mirrors the on-chain TagRegistry.TagEntry struct.
type TagEntry struct {
	Publisher common.Address
	Tag       [32]byte
	Ctxt      []byte
}

// Channel holds the information a recipient needs to scan for their tags.
type Channel struct {
	SharedSecret []byte   // 32-byte shared secret from the channel setup phase
	PkSpend      *big.Int // recipient's spend public key (tweak / domain separator)
}

// ScannedTag is returned when a recipient finds a matching tag.
type ScannedTag struct {
	BlockNumber  uint64
	Entry        TagEntry
	SharedSecret []byte // the channel shared secret that matched
}

// ── Tag derivation ────────────────────────────────────────────────────────────

// DeriveTag computes the private messaging tag for a given block and recipient.
//
//	ss_field = sharedSecret mod BN254_p
//	t        = Poseidon(blockNumber, pkRecipient, ss_field)
func DeriveTag(blockNumber uint64, recipientPkSpend *big.Int, sharedSecret []byte) ([32]byte, error) {
	// Reduce 32-byte shared secret to a BN254 scalar field element.
	ssField := new(big.Int).Mod(new(big.Int).SetBytes(sharedSecret), bn254ScalarField)

	blockInt := new(big.Int).SetUint64(blockNumber)

	// t = Poseidon3(blockNumber, pkRecipient, ss_field)
	result, err := poseidon.Hash([]*big.Int{blockInt, recipientPkSpend, ssField})
	if err != nil {
		return [32]byte{}, fmt.Errorf("poseidon tag: %w", err)
	}

	var tag [32]byte
	result.FillBytes(tag[:])
	return tag, nil
}

// DeriveChannelKey derives the 32-byte AES channel key from a shared secret.
//
//	channelKey = HKDF-SHA256(sharedSecret, salt=nil, info="enygma-channel-key-v1")
//
// Identical to channel_setup.DeriveChannelKey — both packages must produce
// the same key so that payloads encrypted by one can be decrypted by the other.
func DeriveChannelKey(sharedSecret []byte) [32]byte {
	r := hkdf.New(sha256.New, sharedSecret, nil, []byte("enygma-channel-key-v1"))
	var key [32]byte
	io.ReadFull(r, key[:]) //nolint:errcheck — hkdf.Reader.Read never errors
	return key
}

// DeriveTagWindow derives tags for a consecutive window of blocks.
// Returns tags for blocks [startBlock, startBlock+1, ..., startBlock+windowSize-1].
//
// Use this when submitting through the relayer: include the full window in the
// request so the relayer can pick the correct tag for whichever block the tx
// actually lands in, eliminating prediction mismatch.
func DeriveTagWindow(startBlock uint64, windowSize int, recipientPkSpend *big.Int, sharedSecret []byte) ([][32]byte, error) {
	tags := make([][32]byte, windowSize)
	for i := 0; i < windowSize; i++ {
		t, err := DeriveTag(startBlock+uint64(i), recipientPkSpend, sharedSecret)
		if err != nil {
			return nil, fmt.Errorf("DeriveTag(block=%d): %w", startBlock+uint64(i), err)
		}
		tags[i] = t
	}
	return tags, nil
}

// PrepareTagWithWindow prepares a tag window + encrypted payload for relayer
// submission, querying the chain to set the correct startBlock.
//
// windowSize controls how many consecutive blocks to cover (recommend 3–5).
// Returns (startBlock, tags, ctxt) ready to pass to POST /relay/tag as window mode.
//
//	startBlock, tags, ctxt, err := PrepareTagWithWindow(client, 3, pkSpend, ss, key, payload)
//	// POST /relay/tag { "tags": hexTags, "startBlock": startBlock, "ctxt": hex(ctxt) }
func PrepareTagWithWindow(
	client *ethclient.Client,
	windowSize int,
	recipientPkSpend *big.Int,
	sharedSecret []byte,
	channelKey [32]byte,
	payload []byte,
) (startBlock uint64, tags [][32]byte, ctxt []byte, err error) {
	currentBlock, err := client.BlockNumber(context.Background())
	if err != nil {
		return 0, nil, nil, fmt.Errorf("BlockNumber: %w", err)
	}
	startBlock = currentBlock + 1 // earliest possible landing block

	tags, err = DeriveTagWindow(startBlock, windowSize, recipientPkSpend, sharedSecret)
	if err != nil {
		return 0, nil, nil, err
	}

	ctxt, err = EncryptPayload(channelKey, payload)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("EncryptPayload: %w", err)
	}
	return startBlock, tags, ctxt, nil
}

// ── Payload encryption ────────────────────────────────────────────────────────

// EncryptPayload encrypts a tag payload using the channel key (AES-256-GCM).
// Output format: nonce(12 bytes) || ciphertext+tag.
func EncryptPayload(channelKey [32]byte, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(channelKey[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptPayload decrypts a tag payload encrypted with EncryptPayload.
func DecryptPayload(channelKey [32]byte, ct []byte) ([]byte, error) {
	block, err := aes.NewCipher(channelKey[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ct) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ct[:gcm.NonceSize()], ct[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// ── Dummy messages (paper §4.2) ───────────────────────────────────────────────

// dummyMsgMarker is the well-known plaintext encrypted in a dummy tag payload.
// All participants agree on this value as a system parameter.
const dummyMsgMarker = "enygma-dummy-v1"

// PrepareDummyTag computes the (tag, ctxt) pair for a dummy message locally,
// without publishing on-chain. Use when routing through the relayer so that
// msg.sender = relayer address (same privacy as real tags).
//
//	tag, ctxt, err := PrepareDummyTag(blockNumber, recipientPkSpend, sharedSecret)
//	// POST /relay/tag { tag: hex(tag), ctxt: hex(ctxt) }
func PrepareDummyTag(blockNumber uint64, recipientPkSpend *big.Int, sharedSecret []byte) ([32]byte, []byte, error) {
	tag, err := DeriveTag(blockNumber, recipientPkSpend, sharedSecret)
	if err != nil {
		return [32]byte{}, nil, fmt.Errorf("DeriveTag (dummy): %w", err)
	}
	k := DeriveChannelKey(sharedSecret)
	ctxt, err := EncryptPayload(k, []byte(dummyMsgMarker))
	if err != nil {
		return [32]byte{}, nil, fmt.Errorf("EncryptPayload (dummy): %w", err)
	}
	return tag, ctxt, nil
}

// IsDummyPayload returns true if a decrypted tag payload is a dummy message.
// Recipients call this after DecryptPayload to discard cover-traffic entries.
//
//	plaintext, err := DecryptPayload(channelKey, entry.Ctxt)
//	if err == nil && tags.IsDummyPayload(plaintext) { continue }
func IsDummyPayload(plaintext []byte) bool {
	return string(plaintext) == dummyMsgMarker
}

// ── On-chain read operations ──────────────────────────────────────────────────

// GetEntries reads all (publisher, tag, ctxt) entries published at a given block.
func GetEntries(
	client *ethclient.Client,
	registryAddr common.Address,
	blockNumber uint64,
) ([]TagEntry, error) {
	a, err := loadABI()
	if err != nil {
		return nil, err
	}
	c := bind.NewBoundContract(registryAddr, a, client, client, client)

	var result []interface{}
	if err := c.Call(&bind.CallOpts{}, &result, "getEntries",
		new(big.Int).SetUint64(blockNumber)); err != nil {
		return nil, fmt.Errorf("TagRegistry.getEntries: %w", err)
	}

	// go-ethereum's ABI decoder adds json struct tags to dynamically generated
	// tuple types. The assertion must match those tags exactly.
	raw, ok := result[0].([]struct {
		Publisher common.Address `json:"publisher"`
		Tag       [32]uint8      `json:"tag"`
		Ctxt      []uint8        `json:"ctxt"`
	})
	if !ok {
		return nil, fmt.Errorf("unexpected type from getEntries: %T", result[0])
	}

	entries := make([]TagEntry, len(raw))
	for i, r := range raw {
		entries[i] = TagEntry{Publisher: r.Publisher, Tag: r.Tag, Ctxt: r.Ctxt}
	}
	return entries, nil
}

// ScanBlock fetches all tags at blockNumber and returns those matching any of
// the caller's channels.
//
// For each channel the recipient computes:
//
//	expected = Poseidon(blockNumber, channel.PkSpend, ss_field)
//
// and checks if it appears in the on-chain entries.
// Complexity: O(len(channels) × entries_per_block).
func ScanBlock(
	client *ethclient.Client,
	registryAddr common.Address,
	blockNumber uint64,
	channels []Channel,
) ([]ScannedTag, error) {
	entries, err := GetEntries(client, registryAddr, blockNumber)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}

	var matches []ScannedTag
	for _, ch := range channels {
		expected, err := DeriveTag(blockNumber, ch.PkSpend, ch.SharedSecret)
		if err != nil {
			return nil, fmt.Errorf("DeriveTag: %w", err)
		}
		for _, entry := range entries {
			if entry.Tag == expected {
				matches = append(matches, ScannedTag{
					BlockNumber:  blockNumber,
					Entry:        entry,
					SharedSecret: ch.SharedSecret,
				})
			}
		}
	}
	return matches, nil
}

// DeployTagRegistry deploys a new TagRegistry and returns its address.
func DeployTagRegistry(
	client *ethclient.Client,
	auth *bind.TransactOpts,
	artifactPath string,
) (common.Address, error) {
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		return common.Address{}, fmt.Errorf("read artifact: %w", err)
	}
	var artifact struct {
		ABI      json.RawMessage `json:"abi"`
		Bytecode string          `json:"bytecode"`
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		return common.Address{}, fmt.Errorf("parse artifact: %w", err)
	}
	parsed, err := abi.JSON(strings.NewReader(string(artifact.ABI)))
	if err != nil {
		return common.Address{}, fmt.Errorf("parse ABI: %w", err)
	}
	addr, tx, _, err := bind.DeployContract(auth, parsed, common.FromHex(artifact.Bytecode), client)
	if err != nil {
		return common.Address{}, fmt.Errorf("deploy: %w", err)
	}
	if _, err := bind.WaitMined(context.Background(), client, tx); err != nil {
		return common.Address{}, fmt.Errorf("wait mined: %w", err)
	}
	return addr, nil
}

// ── ABI loader ─────────────────────────────────────────────────────────────────

func loadABI() (abi.ABI, error) {
	dir, err := os.Getwd()
	if err != nil {
		return abi.ABI{}, err
	}
	for {
		candidate := filepath.Join(dir, "private_tags", "contracts", "TagRegistry.json")
		if data, err := os.ReadFile(candidate); err == nil {
			var artifact struct {
				ABI json.RawMessage `json:"abi"`
			}
			if err := json.Unmarshal(data, &artifact); err != nil {
				return abi.ABI{}, fmt.Errorf("parse TagRegistry artifact: %w", err)
			}
			return abi.JSON(strings.NewReader(string(artifact.ABI)))
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return abi.ABI{}, fmt.Errorf("TagRegistry.json not found")
}
