package core

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/mlkem"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"

	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/iden3/go-iden3-crypto/poseidon"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/sha3"
)

// BASE_POINT_ORDER is the order of the BabyJubJub base point subgroup
var BASE_POINT_ORDER, _ = new(big.Int).SetString(
	"2736030358979909402780800718157159386076813972158567259200215660948447373041", 10)

// --- BabyJubJub helpers ---

// MulPointEscalar performs scalar multiplication: result = scalar * point
func MulPointEscalar(point *babyjub.Point, scalar *big.Int) *babyjub.Point {
	res := babyjub.NewPoint()
	res.Mul(scalar, point)
	return res
}

// AddPoints adds two BabyJubJub points using projective coordinates
func AddPoints(p1, p2 *babyjub.Point) *babyjub.Point {
	res := babyjub.NewPointProjective()
	res.Add(p1.Projective(), p2.Projective())
	return res.Affine()
}

// NegatePoint returns the negation of a BabyJubJub point (-x, y)
func NegatePoint(p *babyjub.Point) *babyjub.Point {
	negX := new(big.Int).Sub(SNARK_SCALAR_FIELD, p.X)
	return &babyjub.Point{
		X: negX,
		Y: new(big.Int).Set(p.Y),
	}
}

// --- File utilities ---

// WriteToJSON writes data to a JSON file with indentation
func WriteToJSON(filePath string, data interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return os.WriteFile(filePath, jsonData, 0644)
}

// --- Conversion utilities ---

// Buffer2BigInt converts a byte slice to *big.Int
func Buffer2BigInt(buf []byte) *big.Int {
	if len(buf) == 0 {
		return big.NewInt(0)
	}
	return new(big.Int).SetBytes(buf)
}

// StringifyBigInts converts a map with *big.Int values to string representations
func StringifyBigInts(input map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range input {
		switch val := v.(type) {
		case *big.Int:
			result[k] = val.String()
		case []*big.Int:
			strs := make([]string, len(val))
			for i, bi := range val {
				strs[i] = bi.String()
			}
			result[k] = strs
		case map[string]interface{}:
			result[k] = StringifyBigInts(val)
		default:
			result[k] = v
		}
	}
	return result
}

// --- Random generation ---

// RandomInField returns a random value in the SNARK scalar field
func RandomInField() (*big.Int, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	result := new(big.Int).SetBytes(b)
	result.Mod(result, SNARK_SCALAR_FIELD)
	return result, nil
}

// RandomNonce generates a random 128-bit non-zero nonce
func RandomNonce() (*big.Int, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	result := new(big.Int).SetBytes(b)
	result.Add(result, big.NewInt(1)) // ensure non-zero
	return result, nil
}

// --- Poseidon-based key operations ---

// NewKeyPair generates a new Poseidon-based key pair
func NewKeyPair() (privateKey, publicKey *big.Int, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return nil, nil, err
	}

	privateKey, err = poseidon.Hash([]*big.Int{new(big.Int).SetBytes(b)})
	if err != nil {
		return nil, nil, err
	}

	publicKey, err = poseidon.Hash([]*big.Int{privateKey})
	if err != nil {
		return nil, nil, err
	}

	return privateKey, publicKey, nil
}

// GetNullifier computes the nullifier from a private key and path indices.
// Deprecated: use GetNullifierWithTree for all new circuits to prevent cross-tree
// double-spend (HIGH-1). GetNullifier remains for backward-compatibility with legacy
// circuits (deprecated template files) that have not been regenerated yet.
func GetNullifier(privateKey, pathIndices *big.Int) (*big.Int, error) {
	return poseidon.Hash([]*big.Int{privateKey, pathIndices})
}

// GetNullifierWithTree computes a globally-unique nullifier that binds the leaf
// to a specific (treeNumber, pathIndex) pair within a vault.
//
// HIGH-1 fix: Poseidon(sk, pathIndex) alone is identical for the same slot
// in two different trees, enabling cross-tree double-spend (see NullifierWithTree
// in gnark_circuits/primitives/Nullifier.go for full explanation).
//
//	globalIdx      = treeNumber * treeCapacity + pathIndex
//	treeCapacity   = 2^treeDepth  (e.g. 256 for depth 8)
//	nullifier      = Poseidon(sk, globalIdx)
func GetNullifierWithTree(sk, treeNumber, pathIndex *big.Int, treeDepth int) (*big.Int, error) {
	capacity := new(big.Int).Lsh(big.NewInt(1), uint(treeDepth)) // 2^treeDepth
	globalIdx := new(big.Int).Add(
		new(big.Int).Mul(treeNumber, capacity),
		pathIndex,
	)
	return poseidon.Hash([]*big.Int{sk, globalIdx})
}

// GetNullifierBound computes a contract-address-bound nullifier.
// Used by the retail payments circuit (VULN-5 fix): including the vault address
// prevents cross-deployment proof replay.
//
//	nf = Poseidon3(sk, leafIndex, contractAddress)
func GetNullifierBound(privateKey, pathIndices, contractAddress *big.Int) (*big.Int, error) {
	return poseidon.Hash([]*big.Int{privateKey, pathIndices, contractAddress})
}

// GetCommitment computes a commitment from a unique ID and public key
func GetCommitment(uniqueId, publicKey *big.Int) (*big.Int, error) {
	return poseidon.Hash([]*big.Int{uniqueId, publicKey})
}

// GetPublicKey derives a public key from a private key using Poseidon
func GetPublicKey(privateKey *big.Int) (*big.Int, error) {
	return poseidon.Hash([]*big.Int{privateKey})
}

// GetAuctionId computes an auction ID from a commitment
func GetAuctionId(commitment *big.Int) (*big.Int, error) {
	return poseidon.Hash([]*big.Int{commitment})
}

// BlindedPublicKey computes a blinded public key
func BlindedPublicKey(publicKey *big.Int) (*big.Int, error) {
	return poseidon.Hash([]*big.Int{publicKey})
}

// --- Dual key pair types (non-interactive flow) ---

// SpendKeyPair is a Poseidon-based key pair used to prove note ownership in ZK.
// PrivateKey is the spending secret; PublicKey = Poseidon(PrivateKey) goes into commitments.
type SpendKeyPair struct {
	PrivateKey *big.Int
	PublicKey  *big.Int
}

// ViewKeyPair is an ML-KEM-768 key pair used for non-interactive note delivery.
// The sender uses EncapsKey (public) to derive a shared salt.
// The recipient uses DecapsKey (private) to recover that same salt.
type ViewKeyPair struct {
	DecapsKey *mlkem.DecapsulationKey768
	EncapsKey []byte // serialised encapsulation key — 1184 bytes, safe to publish
}

// NewSpendKeyPair generates a fresh Poseidon-based spend key pair.
func NewSpendKeyPair() (*SpendKeyPair, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("failed to read random bytes: %w", err)
	}
	seed := new(big.Int).SetBytes(b)
	seed.Mod(seed, SNARK_SCALAR_FIELD)
	sk, err := poseidon.Hash([]*big.Int{seed})
	if err != nil {
		return nil, fmt.Errorf("failed to hash private key: %w", err)
	}
	pk, err := poseidon.Hash([]*big.Int{sk})
	if err != nil {
		return nil, fmt.Errorf("failed to derive public key: %w", err)
	}
	return &SpendKeyPair{PrivateKey: sk, PublicKey: pk}, nil
}

// NewViewKeyPair generates a fresh ML-KEM-768 view key pair.
func NewViewKeyPair() (*ViewKeyPair, error) {
	dk, err := mlkem.GenerateKey768()
	if err != nil {
		return nil, fmt.Errorf("failed to generate ML-KEM key: %w", err)
	}
	return &ViewKeyPair{
		DecapsKey: dk,
		EncapsKey: dk.EncapsulationKey().Bytes(),
	}, nil
}

// --- KEM: Encapsulate / Decapsulate ---

// Encapsulate performs ML-KEM-768 key encapsulation against the recipient's
// encapsulation key (pk_view).  Returns the raw 32-byte shared secret ss and
// the 1088-byte capsule cipherText.
//
// The caller must NOT use ss directly; derive the commitment salt and the
// encryption key via DerivePaymentSalt and DerivePaymentKey respectively.
//
//	ss          — 32-byte ML-KEM shared secret (keep private; used as HKDF IKM)
//	cipherText — 1088-byte ML-KEM capsule published on-chain alongside the commitment
func Encapsulate(encapsKey []byte) (ss, cipherText []byte, err error) {
	ek, err := mlkem.NewEncapsulationKey768(encapsKey)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid encapsulation key: %w", err)
	}
	sharedKey, ct := ek.Encapsulate()
	return sharedKey, ct, nil
}

// Decapsulate recovers the raw ML-KEM shared secret ss from a capsule using the
// recipient's decapsulation key (sk_view).  Per the ML-KEM spec, a wrong
// ciphertext yields a deterministic but unpredictable key (implicit rejection).
func Decapsulate(dk *mlkem.DecapsulationKey768, cipherText []byte) (ss []byte, err error) {
	sharedKey, err := dk.Decapsulate(cipherText)
	if err != nil {
		return nil, fmt.Errorf("decapsulation failed: %w", err)
	}
	return sharedKey, nil
}

// SaltBToField reduces a 32-byte salt to a SNARK scalar field element so it
// can be used inside Poseidon commitments inside the ZK circuit.
func SaltBToField(saltB []byte) *big.Int {
	n := new(big.Int).SetBytes(saltB)
	return n.Mod(n, SNARK_SCALAR_FIELD)
}

// --- HKDF-based payment key derivation ---

// hkdfDerive is a helper that runs HKDF-SHA256 with the given IKM and info
// label, producing `length` bytes.  Salt is omitted (nil) — the ML-KEM shared
// secret already carries sufficient entropy.
func hkdfDerive(ikm []byte, info string, length int) ([]byte, error) {
	r := hkdf.New(sha256.New, ikm, nil, []byte(info))
	out := make([]byte, length)
	if _, err := io.ReadFull(r, out); err != nil {
		return nil, fmt.Errorf("hkdf derivation failed: %w", err)
	}
	return out, nil
}

// DerivePaymentSalt derives the recipient's commitment salt from the raw ML-KEM
// shared secret ss:
//
//	salt_B = HKDF-SHA256(ikm=ss, info="note salt")  →  32 bytes
//
// The ss is already unique per recipient (derived from their ML-KEM key), so the
// info label just provides domain separation from DerivePaymentKey.
// Reduce the result with SaltBToField before using it in a Poseidon commitment.
func DerivePaymentSalt(ss []byte) ([]byte, error) {
	return hkdfDerive(ss, "note salt", 32)
}

// DerivePaymentKey derives the AES-GCM encryption key from the raw ML-KEM
// shared secret ss:
//
//	k = HKDF-SHA256(ikm=ss, info="encryption key")  →  32 bytes
//
// Pass the result to EncryptPayload / DecryptPayload.
func DerivePaymentKey(ss []byte) ([]byte, error) {
	return hkdfDerive(ss, "encryption key", 32)
}

// DeriveDvpSaltInit derives the initiator's receiving commitment salt for the DvP protocol:
//
//	salt_A = HKDF-SHA256(ikm=ss, info="Init Salt")  →  32 bytes
//
// Alice computes this off-chain when building her initiator proof.
// Bob re-derives it after decapsulating CTXT to independently verify COMMIT_A.
func DeriveDvpSaltInit(ss []byte) ([]byte, error) {
	return hkdfDerive(ss, "Init Salt", 32)
}

// --- AEAD payload encryption ---

// GenerateRandomValue generates cryptographically random bytes of the given size.
// It is a thin wrapper around crypto/rand.Read.
func GenerateRandomValue(size int) ([]byte, error) {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return b, nil
}

// EncryptSwapPayload encrypts the ZkDvp swap payload (tokenId || amount || saltStar)
// using ChaCha20-Poly1305 keyed by saltB (the ML-KEM shared secret).
//
//   - tokenId, amount  — details of the asset Alice will receive (output commitment C')
//   - saltStar         — the random salt used in C' = Poseidon(aliceSpendPk, SaltBToField(saltStar), amount, tokenId)
//
// Bob decrypts this to verify that C' is well-formed before completing the swap.
// Layout on wire: [ 12-byte nonce | ciphertext | 16-byte Poly1305 tag ]
func EncryptSwapPayload(saltB []byte, tokenId, amount *big.Int, saltStar []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(saltB)
	if err != nil {
		return nil, fmt.Errorf("failed to create AEAD cipher: %w", err)
	}

	// plaintext = tokenId (32 bytes) || amount (32 bytes) || saltStar (len(saltStar) bytes)
	plaintext := make([]byte, 64+len(saltStar))
	tokenId.FillBytes(plaintext[:32])
	amount.FillBytes(plaintext[32:64])
	copy(plaintext[64:], saltStar)

	nonce := make([]byte, aead.NonceSize()) // 12 bytes
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	return aead.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptSwapPayload decrypts a ZkDvp swap ciphertext produced by EncryptSwapPayload.
// Returns tokenId, amount, and saltStar. A non-nil error means the payload is not
// addressed to this key (authentication failure or wrong key).
func DecryptSwapPayload(saltB, encTxData []byte) (tokenId, amount *big.Int, saltStar []byte, err error) {
	aead, err := chacha20poly1305.New(saltB)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create AEAD cipher: %w", err)
	}

	nonceSize := aead.NonceSize()
	if len(encTxData) < nonceSize {
		return nil, nil, nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ct := encTxData[:nonceSize], encTxData[nonceSize:]
	plaintext, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("decryption failed: payload not addressed to this key")
	}

	if len(plaintext) < 64 {
		return nil, nil, nil, fmt.Errorf("unexpected plaintext length: %d", len(plaintext))
	}

	tokenId = new(big.Int).SetBytes(plaintext[:32])
	amount = new(big.Int).SetBytes(plaintext[32:64])
	saltStar = plaintext[64:]
	return tokenId, amount, saltStar, nil
}

// EncryptPayload encrypts tokenId||amount using AES-256-GCM keyed by encKey.
// encKey must be the 32-byte value from DerivePaymentKey(ss).
// Wire layout: [ 12-byte nonce | AES-GCM ciphertext | 16-byte GCM tag ]
func EncryptPayload(encKey []byte, tokenId, amount *big.Int) ([]byte, error) {
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// plaintext = tokenId (32 bytes big-endian) || amount (32 bytes big-endian)
	plaintext := make([]byte, 64)
	tokenId.FillBytes(plaintext[:32])
	amount.FillBytes(plaintext[32:])

	nonce := make([]byte, gcm.NonceSize()) // 12 bytes
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Seal prepends nonce so the receiver can split it off
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptPayload decrypts encTxData produced by EncryptPayload.
// encKey must be the 32-byte value from DerivePaymentKey(ss).
// Returns a non-nil error if encKey is wrong or the ciphertext was tampered —
// this is the implicit "not for me" signal Bob uses when scanning the chain.
func DecryptPayload(encKey, encTxData []byte) (tokenId, amount *big.Int, err error) {
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(encTxData) < nonceSize {
		return nil, nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ct := encTxData[:nonceSize], encTxData[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		// Deliberately vague: wrong key and tampered ciphertext are indistinguishable
		return nil, nil, fmt.Errorf("decryption failed: payload not addressed to this key")
	}

	if len(plaintext) != 64 {
		return nil, nil, fmt.Errorf("unexpected plaintext length: %d", len(plaintext))
	}

	tokenId = new(big.Int).SetBytes(plaintext[:32])
	amount = new(big.Int).SetBytes(plaintext[32:])
	return tokenId, amount, nil
}

// --- BabyJubJub encryption (ElGamal-like) ---

// EncodeMessage encodes an integer as a BabyJubJub point: m * Base8
func EncodeMessage(m *big.Int) *babyjub.Point {
	return MulPointEscalar(babyjub.B8, m)
}

// DecodeMessage recovers the integer from a BabyJubJub point by brute-force search
func DecodeMessage(point *babyjub.Point, maxM *big.Int) (*big.Int, error) {
	fmt.Printf("Allowed range: %s\n", maxM.String())
	m := new(big.Int)
	for m.Cmp(maxM) < 0 {
		candidate := EncodeMessage(m)
		if candidate.X.Cmp(point.X) == 0 && candidate.Y.Cmp(point.Y) == 0 {
			return new(big.Int).Set(m), nil
		}
		m.Add(m, big.NewInt(1))
	}
	return nil, fmt.Errorf("message not found")
}

// BabyEncrypt performs ElGamal-like encryption on BabyJubJub
// Returns (c1, c2) where c1 = r*G, c2 = m*G + r*pubKey
func BabyEncrypt(m, r *big.Int, pubKey *babyjub.Point) (c1, c2 *babyjub.Point) {
	// c1 = r * G
	c1 = MulPointEscalar(babyjub.B8, r)

	// rPub = r * publicKey
	rPub := MulPointEscalar(pubKey, r)

	// mG = m * G
	mG := EncodeMessage(m)

	// c2 = mG + rPub
	c2 = AddPoints(mG, rPub)

	return c1, c2
}

// BabyDecrypt performs ElGamal-like decryption on BabyJubJub
// Returns the decrypted message or nil if not found in allowed range
func BabyDecrypt(c1, c2 *babyjub.Point, privateKey *big.Int, allowedRange *big.Int) *big.Int {
	if allowedRange == nil {
		allowedRange = big.NewInt(1000)
	}

	// rPub = privateKey * c1
	rPub := MulPointEscalar(c1, privateKey)

	// M_dec = c2 + (-rPub) = c2 - rPub
	negRPub := NegatePoint(rPub)
	mDec := AddPoints(c2, negRPub)

	decrypted, err := DecodeMessage(mDec, allowedRange)
	if err != nil {
		fmt.Println("Invalid value")
		return nil
	}
	return decrypted
}

// BabyKeyPair generates a BabyJubJub key pair
func BabyKeyPair() (privateKey *big.Int, publicKey *babyjub.Point, err error) {
	for {
		privateKey, err = RandomInField()
		if err != nil {
			return nil, nil, err
		}
		privateKey.Mod(privateKey, babyjub.SubOrder)
		if privateKey.Sign() != 0 {
			break
		}
	}
	publicKey = MulPointEscalar(babyjub.B8, privateKey)
	return privateKey, publicKey, nil
}

// AuditorKeyPair holds a BabyJubJub key pair used for auditor encryption.
// The public key (X, Y) is passed to the circuit as StAuditorPublickey.
// The private key is used off-chain to decrypt auditor-encrypted values.
type AuditorKeyPair struct {
	PrivateKey *big.Int
	PublicKeyX *big.Int
	PublicKeyY *big.Int
}

// NewAuditorKeyPair generates a fresh BabyJubJub key pair for auditor encryption.
func NewAuditorKeyPair() (*AuditorKeyPair, error) {
	sk, pk, err := BabyKeyPair()
	if err != nil {
		return nil, err
	}
	return &AuditorKeyPair{
		PrivateKey: sk,
		PublicKeyX: pk.X,
		PublicKeyY: pk.Y,
	}, nil
}

// RandomAuditorScalar returns a random scalar in (0, BASE_POINT_ORDER) for use
// as the auditor's random blinding factor (WtAuditorRandom in the circuit).
func RandomAuditorScalar() (*big.Int, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	r := new(big.Int).SetBytes(b)
	r.Mod(r, BASE_POINT_ORDER)
	// Ensure non-zero (circuit requires random > 0)
	if r.Sign() == 0 {
		r.SetInt64(1)
	}
	return r, nil
}

// AuditorEncKey computes the shared encryption key for Poseidon encryption:
//
//	authKey = MulPointEscalar(auditorPublicKey, random)
//
// Returns (authKey.X, authKey.Y) — used as the key for NativePoseidonEncrypt / PoseidonDecrypt.
// StAuditorAuthKey in the circuit is informational; the circuit re-derives it from
// StAuditorPublickey and WtAuditorRandom.
func AuditorEncKey(pubKeyX, pubKeyY, random *big.Int) (*big.Int, *big.Int) {
	pk := &babyjub.Point{X: pubKeyX, Y: pubKeyY}
	auth := MulPointEscalar(pk, random)
	return auth.X, auth.Y
}

// --- Poseidon encryption (maci-crypto compatible) ---

// PoseidonEncryptResult holds the result of Poseidon encryption
type PoseidonEncryptResult struct {
	Encrypted  []*big.Int
	Nonce      *big.Int
	RandomVal  *big.Int
	SharedKey  *babyjub.Point
	AuthKey    *babyjub.Point
}

// PoseidonEncryptWrapper encrypts inputs using Poseidon sponge encryption
// TODO: Requires a Go port of maci-crypto's poseidonEncrypt
func PoseidonEncryptWrapper(inputs []*big.Int, publicKey *babyjub.Point) (*PoseidonEncryptResult, error) {
	nonce, err := RandomNonce()
	if err != nil {
		return nil, err
	}

	randomValue, err := RandomInField()
	if err != nil {
		return nil, err
	}
	randomValue.Div(randomValue, big.NewInt(10))

	authKey := MulPointEscalar(babyjub.B8, randomValue)
	sharedKey := MulPointEscalar(publicKey, randomValue)

	// TODO: implement poseidonEncrypt(inputs, sharedKey, nonce)
	// This requires porting maci-crypto's Poseidon sponge encryption to Go.
	// When implemented, replace the return below with:
	//   encrypted := poseidonEncrypt(inputs, sharedKey, nonce)
	//   return &PoseidonEncryptResult{Encrypted: encrypted, Nonce: nonce,
	//     RandomVal: randomValue, SharedKey: sharedKey, AuthKey: authKey}, nil
	_, _, _ = nonce, sharedKey, authKey
	return nil, fmt.Errorf("poseidonEncrypt not yet implemented: requires maci-crypto Go port")
}

// PoseidonDecryptWrapper decrypts data using Poseidon sponge decryption
// TODO: Requires a Go port of maci-crypto's poseidonDecrypt
func PoseidonDecryptWrapper(encrypted []*big.Int, authKey *babyjub.Point, nonce, privateKey *big.Int, length int) ([]*big.Int, error) {
	sharedKey := MulPointEscalar(authKey, privateKey)

	// TODO: implement poseidonDecrypt(encrypted, sharedKey, nonce, length)
	// This requires porting maci-crypto's Poseidon sponge decryption to Go
	_ = sharedKey
	return nil, fmt.Errorf("poseidonDecrypt not yet implemented: requires maci-crypto Go port")
}

// --- Token unique IDs ---

// Erc20UniqueId computes the unique ID for an ERC-20 token commitment
func Erc20UniqueId(contractAddress, amount *big.Int) (*big.Int, error) {
	return poseidon.Hash([]*big.Int{contractAddress, amount})
}

// Erc20Commitment computes a single-hash ERC20 commitment (legacy, interactive flow).
// Deprecated: use Erc20CommitmentV2 for the non-interactive flow.
func Erc20Commitment(contractAddress, amount, publicKey, salt *big.Int) (*big.Int, error) {
	return poseidon.Hash([]*big.Int{contractAddress, amount, publicKey, salt})
}

// Erc20CommitmentV2 computes the ERC20 commitment for the non-interactive flow.
//
//	C = Poseidon(pk_spend, saltB_field, amount, tokenId)
//
// saltB_field must be the output of SaltBToField — the KEM shared secret
// reduced mod SNARK_SCALAR_FIELD so it fits inside the ZK circuit.
func Erc20CommitmentV2(pkSpend, saltBField, amount, tokenId *big.Int) (*big.Int, error) {
	return poseidon.Hash([]*big.Int{pkSpend, saltBField, amount, tokenId})
}

// Erc721UniqueId computes the unique ID for an ERC-721 token commitment
func Erc721UniqueId(contractAddress, tokenId *big.Int) (*big.Int, error) {
	return poseidon.Hash([]*big.Int{contractAddress, tokenId})
}

// Erc721Commitment computes the ERC721 commitment using the unified V2 formula:
//
//	C = Poseidon(pk_spend, salt, 1, tokenId)
//
// The amount is always 1 for non-fungible tokens.
func Erc721Commitment(tokenId, publicKey, salt *big.Int) (*big.Int, error) {
	return Erc20CommitmentV2(publicKey, salt, big.NewInt(1), tokenId)
}

// Erc1155UniqueId computes the unique ID for an ERC-1155 token commitment
func Erc1155UniqueId(contractAddress, tokenId, amount *big.Int) (*big.Int, error) {
	uid1, err := poseidon.Hash([]*big.Int{contractAddress, tokenId})
	if err != nil {
		return nil, err
	}
	return poseidon.Hash([]*big.Int{uid1, amount})
}

// Erc1155Commitment computes the ERC1155 commitment using the unified V2 formula:
//
//	C = Poseidon(pk_spend, salt, amount, tokenId)
//
// contractAddress is no longer part of the commitment; it is handled separately
// via the asset-group Merkle proof.
func Erc1155Commitment(tokenId, amount, publicKey, salt *big.Int) (*big.Int, error) {
	return Erc20CommitmentV2(publicKey, salt, amount, tokenId)
}

// --- Hash functions ---

// Pedersen computes a Pedersen-like commitment using Poseidon: H(amount, random)
func Pedersen(amount, random *big.Int) (*big.Int, error) {
	return poseidon.Hash([]*big.Int{amount, random})
}

// Keccak256 computes keccak256(preimage) mod SNARK_SCALAR_FIELD
func Keccak256(preimage []byte) *big.Int {
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(preimage)
	hash := hasher.Sum(nil)

	result := new(big.Int).SetBytes(hash)
	result.Mod(result, SNARK_SCALAR_FIELD)
	return result
}
