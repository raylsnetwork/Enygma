// Package core re-exports payment symbols from src/core/payment.
package core

import pay "github.com/raylsnetwork/enygma_retail_payments/src/core/payment"

// ── DVP type aliases ──────────────────────────────────────────────────────────

type (
	KeyPair       = pay.KeyPair
	SpendKeyPair  = pay.SpendKeyPair
	ViewKeyPair   = pay.ViewKeyPair
	MerkleProof   = pay.MerkleProof
	MerkleTree    = pay.MerkleTree
	PaymentResult = pay.PaymentResult
	ChannelNote   = pay.ChannelNote
)

// ── Function re-exports ───────────────────────────────────────────────────────

var (
	NewPaymentClient   = pay.NewPaymentClient
	NewSpendKeyPair    = pay.NewSpendKeyPair
	NewViewKeyPair     = pay.NewViewKeyPair
	NewMerkleTree      = pay.NewMerkleTree
	GetNullifier       = pay.GetNullifier
	Erc20CommitmentV2  = pay.Erc20CommitmentV2
	Encapsulate        = pay.Encapsulate
	Decapsulate        = pay.Decapsulate
	DerivePaymentSalt  = pay.DerivePaymentSalt
	DerivePaymentKey   = pay.DerivePaymentKey
	SaltBToField       = pay.SaltBToField
	EncryptPayload     = pay.EncryptPayload
	DecryptPayload     = pay.DecryptPayload
	RandomInField      = pay.RandomInField
	// DeriveChannelKey derives the AES key used by the private-tag system.
	DeriveChannelKey   = pay.DeriveChannelKey
	ChannelEncryptNote = pay.ChannelEncryptNote
	ChannelDecryptNote = pay.ChannelDecryptNote
)
