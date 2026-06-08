package tags


import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/mlkem"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

)

// ── Privacy modes (paper §3 step 6, Table 2) ─────────────────────────────────

// PrivacyMode controls the anonymity set bitmap published alongside the channel.
type PrivacyMode uint8

const (
	// PrivacyNone discloses the recipient — only the recipient's bit is set.
	// An external observer can identify exactly who is receiving the channel.
	PrivacyNone PrivacyMode = 0

	// PrivacySubset provides k-anonymity — recipient + √N random decoys.
	// Observer knows the channel is for someone in the subset but not who.
	PrivacySubset PrivacyMode = 1

	// PrivacyRift dissociates from specific users — all bits set.
	// Equivalent to full privacy for the purpose of this implementation.
	PrivacyRift PrivacyMode = 2

	// PrivacyFull reveals no information — all users are in the anonymity set.
	// Observer only learns that two users in the system are communicating.
	PrivacyFull PrivacyMode = 3
)

// ── Types ─────────────────────────────────────────────────────────────────────

// ChannelRecord mirrors the on-chain TagChannelRegistry.ChannelRecord struct.
type ChannelRecord struct {
	Sender common.Address
	C1     []byte // ML-KEM-768 ciphertext (1088 bytes) — paper §3 step 3
	C2     []byte // AEAD.Encrypt(k, message, c1)       — paper §3 step 5
	Bitmap []byte // privacy bitmap over registered users
}


type ChannelPayload struct {
	Message  []byte // initial message from sender to recipient
	SenderId []byte // optional sender identity; nil = anonymous
}

// FoundChannel is returned by ScanChannels when a channel is addressed to the
// scanning recipient.
type FoundChannel struct {
	ChannelIdx   uint64
	SharedSecret []byte         // raw ML-KEM shared secret — use for DeriveTag
	Message      []byte         // decrypted initial message (may be empty)
	SenderId     []byte         // sender identity if disclosed; nil = anonymous
	Sender       common.Address // on-chain msg.sender (relayer addr if routed through relayer)
}

// ── Channel payload encoding (paper Table 2 — optional sender identity) ───────

// encodeChannelPayload serialises a ChannelPayload to bytes.
// Wire format: [4B msgLen BE][msg][4B senderIdLen BE][senderId]
func encodeChannelPayload(p ChannelPayload) []byte {
	msgLen      := uint32(len(p.Message))
	senderIdLen := uint32(len(p.SenderId))
	out := make([]byte, 4+msgLen+4+senderIdLen)
	binary.BigEndian.PutUint32(out[0:4], msgLen)
	copy(out[4:4+msgLen], p.Message)
	binary.BigEndian.PutUint32(out[4+msgLen:8+msgLen], senderIdLen)
	copy(out[8+msgLen:], p.SenderId)
	return out
}

// decodeChannelPayload deserialises bytes produced by encodeChannelPayload.
func decodeChannelPayload(data []byte) (ChannelPayload, error) {
	if len(data) < 4 {
		return ChannelPayload{}, fmt.Errorf("payload too short (%d bytes)", len(data))
	}
	msgLen := binary.BigEndian.Uint32(data[0:4])
	if uint32(len(data)) < 4+msgLen+4 {
		return ChannelPayload{}, fmt.Errorf("payload truncated at message")
	}
	msg := make([]byte, msgLen)
	copy(msg, data[4:4+msgLen])

	offset := 4 + msgLen
	senderIdLen := binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4
	if uint32(len(data)) < offset+senderIdLen {
		return ChannelPayload{}, fmt.Errorf("payload truncated at senderId")
	}
	var senderId []byte
	if senderIdLen > 0 {
		senderId = make([]byte, senderIdLen)
		copy(senderId, data[offset:offset+senderIdLen])
	}
	return ChannelPayload{Message: msg, SenderId: senderId}, nil
}

// ── Local-only computation (for relayer path) ────────────────────────────────

// PrepareChannelSetup computes all channel setup blobs locally without publishing.
// Use this when routing through the relayer (POST /relay/channel) so that
// msg.sender = relayer address, not the actual sender (paper §6.3).
//
// Returns (sharedSecret, c1, c2, bitmap). Store sharedSecret for later DeriveTag.
// Send (c1, c2, bitmap) as hex-encoded fields to POST /relay/channel.
func PrepareChannelSetup(
	recipientPkView []byte,
	message []byte,
	senderId []byte,
	mode PrivacyMode,
	totalUsers int,
	recipientIdx int,
	excludedIndices []int,
) (sharedSecret, c1, c2, bitmap []byte, err error) {
	ek, err := mlkem.NewEncapsulationKey768(recipientPkView)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("ML-KEM.NewEncapsulationKey768: %w", err)
	}
	ss, c1Bytes := ek.Encapsulate()

	k := DeriveChannelKey(ss)

	payload := encodeChannelPayload(ChannelPayload{Message: message, SenderId: senderId})
	c2Bytes, err := aesGCMEncryptWithAAD(k, payload, c1Bytes)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("AEAD.Encrypt: %w", err)
	}

	bitmapBytes := buildBitmap(mode, totalUsers, recipientIdx, excludedIndices)
	return ss, c1Bytes, c2Bytes, bitmapBytes, nil
}

// ── Setup Phase — Recipient  ─────────────────────────────────────


func ScanChannels(
	client *ethclient.Client,
	channelRegistryAddr common.Address,
	dk *mlkem.DecapsulationKey768,
	fromIdx uint64,
	maxCount uint64,
) ([]FoundChannel, error) {
	a, err := loadChannelRegistryABI()
	if err != nil {
		return nil, err
	}
	c := bind.NewBoundContract(channelRegistryAddr, a, client, client, client)

	// Get total channel count.
	var countOut []interface{}
	if err := c.Call(&bind.CallOpts{}, &countOut, "getChannelCount"); err != nil {
		return nil, fmt.Errorf("TagChannelRegistry.getChannelCount: %w", err)
	}
	total := countOut[0].(*big.Int).Uint64()

	end := total
	if fromIdx+maxCount < end {
		end = fromIdx + maxCount
	}

	var found []FoundChannel
	for idx := fromIdx; idx < end; idx++ {
		var recOut []interface{}
		if err := c.Call(&bind.CallOpts{}, &recOut, "getChannel",
			new(big.Int).SetUint64(idx)); err != nil {
			continue
		}

		sender, _ := recOut[0].(common.Address)
		c1, _     := recOut[1].([]byte)
		c2, _     := recOut[2].([]byte)

		if len(c1) == 0 {
			continue
		}

		// Step 1: ss' ← ML-KEM.Decapsulate(sk_view, c1)
		// ML-KEM implicit rejection: always returns a value, never errors on well-formed ciphertext.
		ss, err := dk.Decapsulate(c1)
		if err != nil {
			continue
		}

		// Step 2: k' = HKDF(ss', context)
		k := DeriveChannelKey(ss)

		// Step 3: AEAD.Decrypt(k', c2, c1) — c1 as AAD
		// Authentication failure = "Message not for me" (Fig. 1)
		plaintext, err := aesGCMDecryptWithAAD(k, c2, c1)
		if err != nil {
			continue // not for this recipient
		}

		// Decode the structured payload {message, senderId}.
		p, err := decodeChannelPayload(plaintext)
		if err != nil {
			continue // malformed payload — skip
		}

		found = append(found, FoundChannel{
			ChannelIdx:   idx,
			SharedSecret: ss,
			Message:      p.Message,
			SenderId:     p.SenderId,
			Sender:       sender,
		})
	}
	return found, nil
}

// GetChannelCount returns the total number of published channel records.
func GetChannelCount(client *ethclient.Client, channelRegistryAddr common.Address) (uint64, error) {
	a, err := loadChannelRegistryABI()
	if err != nil {
		return 0, err
	}
	c := bind.NewBoundContract(channelRegistryAddr, a, client, client, client)
	var out []interface{}
	if err := c.Call(&bind.CallOpts{}, &out, "getChannelCount"); err != nil {
		return 0, fmt.Errorf("TagChannelRegistry.getChannelCount: %w", err)
	}
	return out[0].(*big.Int).Uint64(), nil
}

// DeployTagChannelRegistry deploys a new TagChannelRegistry and returns its address.
func DeployTagChannelRegistry(
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

// ── Privacy bitmap (paper §3 step 6, Table 2) ────────────────────────────────

// buildBitmap constructs the packed-bit anonymity-set bitmap for the given mode.
//
// Privacy modes (paper Table 2):
//
//	PrivacyNone   — only recipient bit set; observer identifies recipient exactly.
//	PrivacySubset — recipient + √N random decoys; observer knows one of ~√N users.
//	PrivacyRift   — all users EXCEPT excludedIndices; sender dissociates from
//	                specific known parties. Recipient bit always remains set.
//	PrivacyFull   — all N users; observer only knows "someone is communicating".
func buildBitmap(mode PrivacyMode, totalUsers int, recipientIdx int, excludedIndices []int) []byte {
	if totalUsers <= 0 {
		return nil
	}
	byteLen := (totalUsers + 7) / 8
	bits := make([]byte, byteLen)

	set   := func(idx int) { bits[idx/8] |= 1 << (uint(idx) % 8) }
	clear := func(idx int) { bits[idx/8] &^= 1 << (uint(idx) % 8) }
	get   := func(idx int) bool { return bits[idx/8]&(1<<(uint(idx)%8)) != 0 }

	switch mode {
	case PrivacyNone:
		// Only the recipient — discloses recipient identity to chain observers.
		set(recipientIdx)

	case PrivacySubset:
		// k-anonymity: recipient + √N random decoys.
		set(recipientIdx)
		decoys := int(math.Max(1, math.Sqrt(float64(totalUsers))))
		for added := 0; added < decoys; {
			r, err := rand.Int(rand.Reader, big.NewInt(int64(totalUsers)))
			if err != nil {
				break
			}
			idx := int(r.Int64())
			if idx == recipientIdx || get(idx) {
				continue
			}
			set(idx)
			added++
		}

	case PrivacyRift:
		// All users EXCEPT explicitly excluded ones.
		// Recipient bit is always preserved — cannot exclude the recipient.
		for i := 0; i < totalUsers; i++ {
			set(i)
		}
		excluded := make(map[int]bool, len(excludedIndices))
		for _, idx := range excludedIndices {
			excluded[idx] = true
		}
		for idx := range excluded {
			if idx >= 0 && idx < totalUsers && idx != recipientIdx {
				clear(idx)
			}
		}

	case PrivacyFull:
		// All users — observer only learns "two users in the system are communicating".
		for i := 0; i < totalUsers; i++ {
			set(i)
		}
	}
	return bits
}

// BitmapDescription returns a human-readable description of a privacy mode.
// Useful for UIs and logs to explain the chosen mode to the user.
func BitmapDescription(mode PrivacyMode) string {
	switch mode {
	case PrivacyNone:
		return "None — recipient identity is disclosed on-chain. " +
			"Observer can identify exactly who this channel is for."
	case PrivacySubset:
		return "Subset (k-anonymity) — recipient + √N random decoys. " +
			"Observer knows the channel is for one of ~√N users but not which."
	case PrivacyRift:
		return "Rift — all users except explicitly excluded ones. " +
			"Sender dissociates from specific known parties while maximising the anonymity set."
	case PrivacyFull:
		return "Full — all N registered users in the anonymity set. " +
			"Observer only learns that two users in the system are communicating."
	default:
		return "Unknown privacy mode"
	}
}

// ── AEAD with AAD  ──────────────────────────────────────────

// aesGCMEncryptWithAAD encrypts plaintext with AES-256-GCM using aad as
// additional authenticated data. The c1 ML-KEM ciphertext is passed as aad
// per paper §3 step 5: "c1 acts as the additional data of the authenticated
// encryption scheme".
//
// Output: nonce(12) || ciphertext+tag
func aesGCMEncryptWithAAD(key [32]byte, plaintext, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key[:])
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
	return gcm.Seal(nonce, nonce, plaintext, aad), nil
}

// aesGCMDecryptWithAAD decrypts a ciphertext produced by aesGCMEncryptWithAAD.
// Returns an error if the key or AAD is wrong — the caller uses this as the
// "not for me" signal (paper §3.2 Fig. 1).
func aesGCMDecryptWithAAD(key [32]byte, ct, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key[:])
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
	return gcm.Open(nil, nonce, ciphertext, aad)
}

// ── On-chain helpers ──────────────────────────────────────────────────────────

// ── ABI loader ────────────────────────────────────────────────────────────────

func loadChannelRegistryABI() (abi.ABI, error) {
	dir, err := os.Getwd()
	if err != nil {
		return abi.ABI{}, err
	}
	for {
		candidate := filepath.Join(dir, "private_tags", "contracts", "TagChannelRegistry.json")
		if data, err := os.ReadFile(candidate); err == nil {
			var artifact struct {
				ABI json.RawMessage `json:"abi"`
			}
			if err := json.Unmarshal(data, &artifact); err != nil {
				return abi.ABI{}, fmt.Errorf("parse TagChannelRegistry artifact: %w", err)
			}
			return abi.JSON(strings.NewReader(string(artifact.ABI)))
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return abi.ABI{}, fmt.Errorf("TagChannelRegistry.json not found")
}
