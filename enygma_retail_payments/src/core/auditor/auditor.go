package auditor

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/mlkem"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

// hkdfInfo domain-separates the audit key from the payment salt and channel key.
const hkdfInfo = "enygma-audit-view-key-v1"

// AuditorKeyPair is an ML-KEM-768 keypair held by the auditor.
// EncapsKey (1184 bytes) is published so users can encrypt for the auditor at
// registration time. DecapsKey is kept secret and used to recover user view keys.
type AuditorKeyPair struct {
	DecapsKey *mlkem.DecapsulationKey768
	EncapsKey []byte // 1184-byte encapsulation key, safe to publish
}

// NewAuditorKeyPair generates a fresh ML-KEM-768 auditor keypair.
func NewAuditorKeyPair() (*AuditorKeyPair, error) {
	dk, err := mlkem.GenerateKey768()
	if err != nil {
		return nil, fmt.Errorf("auditor keygen: %w", err)
	}
	return &AuditorKeyPair{
		DecapsKey: dk,
		EncapsKey: dk.EncapsulationKey().Bytes(),
	}, nil
}

// EncryptViewKeyForAuditor encrypts the user's ML-KEM decapsulation key (sk_view)
// for the auditor. Returns (mlKemCt, aesCt) to be published on-chain at registration.
//
// Protocol:
//
//	ss, mlKemCt = Encapsulate(auditorPK)
//	enc_key     = HKDF-SHA256(ss, info="enygma-audit-view-key-v1")
//	aesCt       = AES-256-GCM(enc_key, userDK.Bytes(), AAD=mlKemCt)
//
// Passing mlKemCt as AAD binds both ciphertexts: if mlKemCt is swapped on-chain,
// AES-GCM authentication fails and AuditorDecryptViewKey returns an error.
func EncryptViewKeyForAuditor(
	auditorPK []byte,
	userDK *mlkem.DecapsulationKey768,
) (mlKemCt, aesCt []byte, err error) {
	ek, err := mlkem.NewEncapsulationKey768(auditorPK)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid auditor encapsulation key: %w", err)
	}
	ss, mlKemCt := ek.Encapsulate()

	encKey := deriveAuditKey(ss)
	aesCt, err = aesGCMEncryptWithAAD(encKey, userDK.Bytes(), mlKemCt)
	if err != nil {
		return nil, nil, fmt.Errorf("encrypt view key for auditor: %w", err)
	}
	return mlKemCt, aesCt, nil
}

// AuditorDecryptViewKey recovers a user's ML-KEM decapsulation key (sk_view) from
// the ciphertext pair published on-chain at registration.
//
// Protocol:
//
//	ss      = Decapsulate(auditorDK, mlKemCt)
//	enc_key = HKDF-SHA256(ss, info="enygma-audit-view-key-v1")
//	seed    = AES-256-GCM-decrypt(enc_key, aesCt, AAD=mlKemCt)
//	sk_view = NewDecapsulationKey768(seed)
//
// Returns an error if auditorDK is wrong or if either ciphertext was tampered with.
func AuditorDecryptViewKey(
	auditorDK *mlkem.DecapsulationKey768,
	mlKemCt []byte,
	aesCt []byte,
) (*mlkem.DecapsulationKey768, error) {
	ss, err := auditorDK.Decapsulate(mlKemCt)
	if err != nil {
		return nil, fmt.Errorf("decapsulate audit capsule: %w", err)
	}

	encKey := deriveAuditKey(ss)
	seed, err := aesGCMDecryptWithAAD(encKey, aesCt, mlKemCt)
	if err != nil {
		return nil, fmt.Errorf("decrypt view key: %w", err)
	}

	userDK, err := mlkem.NewDecapsulationKey768(seed)
	if err != nil {
		return nil, fmt.Errorf("parse recovered decapsulation key: %w", err)
	}
	return userDK, nil
}

// deriveAuditKey runs HKDF-SHA256 over the ML-KEM shared secret to produce a
// 32-byte AES key. The fixed info string domain-separates this from the payment
// salt (info="Bob salt") and channel key (info="enygma-channel-key-v1").
func deriveAuditKey(ss []byte) [32]byte {
	r := hkdf.New(sha256.New, ss, nil, []byte(hkdfInfo))
	var key [32]byte
	io.ReadFull(r, key[:]) //nolint:errcheck — hkdf.Reader never errors
	return key
}

// aesGCMEncryptWithAAD encrypts plaintext with AES-256-GCM, authenticating aad
// as associated data without encrypting it. Output: nonce(12) || ciphertext+tag.
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
// Returns an error if the key, AAD, or ciphertext has been tampered with.
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
