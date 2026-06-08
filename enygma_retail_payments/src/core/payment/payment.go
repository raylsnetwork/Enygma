package payment

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"math/big"

	dvpcore "github.com/raylsnetwork/enygma_dvp/src/core"
	"golang.org/x/crypto/hkdf"
)

// ── DVP type aliases — callers only need to import this package ────────────────

type (
	KeyPair       = dvpcore.KeyPair
	SpendKeyPair  = dvpcore.SpendKeyPair
	ViewKeyPair   = dvpcore.ViewKeyPair
	MerkleProof   = dvpcore.MerkleProof
	MerkleTree    = dvpcore.MerkleTree
	PaymentResult = dvpcore.PaymentResult
)

// ── DVP function re-exports ────────────────────────────────────────────────────

var (
	NewSpendKeyPair     = dvpcore.NewSpendKeyPair
	NewViewKeyPair      = dvpcore.NewViewKeyPair
	NewMerkleTree       = dvpcore.NewMerkleTree
	GetNullifier        = dvpcore.GetNullifier
	GetNullifierBound   = dvpcore.GetNullifierBound
	Erc20CommitmentV2   = dvpcore.Erc20CommitmentV2
	Encapsulate         = dvpcore.Encapsulate
	Decapsulate         = dvpcore.Decapsulate
	DerivePaymentSalt   = dvpcore.DerivePaymentSalt
	DerivePaymentKey    = dvpcore.DerivePaymentKey
	SaltBToField        = dvpcore.SaltBToField
	EncryptPayload      = dvpcore.EncryptPayload
	DecryptPayload      = dvpcore.DecryptPayload
	RandomInField       = dvpcore.RandomInField
)

// NewPaymentClient creates a GnarkClient targeting the retail payments gnark server.
// Defaults to http://localhost:8082 if baseURL is empty.
func NewPaymentClient(baseURL string) *dvpcore.GnarkClient {
	if baseURL == "" {
		baseURL = "http://localhost:8082"
	}
	return dvpcore.NewGnarkClient(baseURL)
}

// ── Shared-secret key derivation (used by payment and tag system) ─────────────

// DeriveChannelKey derives a 32-byte AES key from a shared secret using HKDF-SHA256.
// Used by the private-tag system to encrypt per-channel payloads.
//
//	key = HKDF-SHA256(sharedSecret, salt=nil, info="enygma-channel-key-v1")
func DeriveChannelKey(sharedSecret []byte) [32]byte {
	r := hkdf.New(sha256.New, sharedSecret, nil, []byte("enygma-channel-key-v1"))
	var key [32]byte
	io.ReadFull(r, key[:]) //nolint:errcheck — hkdf.Reader.Read never errors
	return key
}

// ── Channel payment ────────────────────────────────────────────────────────────

// ChannelNote holds note data recovered from a channel-encrypted payload.
type ChannelNote struct {
	Amount  *big.Int
	TokenId *big.Int
	Salt    *big.Int
}

// ChannelEncryptNote encrypts note data using the established channel key (AES-256-GCM).
//
// Plaintext layout (96 bytes): amount(32 BE) || tokenId(32 BE) || salt(32 BE).
// Output: nonce(12) || ciphertext+tag = 124 bytes.
//
// The recipient decrypts with ChannelDecryptNote, recomputes the commitment, and
// confirms the note is theirs without a per-payment ML-KEM encapsulation.
func ChannelEncryptNote(channelKey [32]byte, amount, tokenId, salt *big.Int) ([]byte, error) {
	plaintext := make([]byte, 96)
	amount.FillBytes(plaintext[0:32])
	tokenId.FillBytes(plaintext[32:64])
	salt.FillBytes(plaintext[64:96])
	return aesGCMEncrypt(channelKey, plaintext)
}

// ChannelDecryptNote decrypts a channel-encrypted note payload.
func ChannelDecryptNote(channelKey [32]byte, ct []byte) (*ChannelNote, error) {
	plaintext, err := aesGCMDecrypt(channelKey, ct)
	if err != nil {
		return nil, fmt.Errorf("channel decrypt: %w", err)
	}
	if len(plaintext) != 96 {
		return nil, fmt.Errorf("unexpected plaintext length: got %d, want 96", len(plaintext))
	}
	return &ChannelNote{
		Amount:  new(big.Int).SetBytes(plaintext[0:32]),
		TokenId: new(big.Int).SetBytes(plaintext[32:64]),
		Salt:    new(big.Int).SetBytes(plaintext[64:96]),
	}, nil
}

// ── AES-256-GCM helpers ────────────────────────────────────────────────────────

func aesGCMEncrypt(key [32]byte, plaintext []byte) ([]byte, error) {
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
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func aesGCMDecrypt(key [32]byte, ct []byte) ([]byte, error) {
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
	return gcm.Open(nil, nonce, ciphertext, nil)
}
