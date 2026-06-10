// Package core re-exports Phase 1 (user registration) symbols from
// src/core/user_registry for backward compatibility with existing callers.
package core

import (
	"github.com/raylsnetwork/enygma_retail_payments/src/core/auditor"
	ur "github.com/raylsnetwork/enygma_retail_payments/src/core/user_registry"
)

// UserKeys holds the two public keys a sender needs to pay a recipient.
type UserKeys = ur.UserKeys

// AuditorKeyPair is the auditor's ML-KEM-768 keypair.
type AuditorKeyPair = auditor.AuditorKeyPair

var (
	// User registry
	Register                = ur.Register
	LookupKeys              = ur.LookupKeys
	LookupAuditCiphertexts  = ur.LookupAuditCiphertexts
	GetUserCount            = ur.GetUserCount
	GetUserIndex            = ur.GetUserIndex
	GetUserAt               = ur.GetUserAt
	GetRegistrationFee      = ur.GetRegistrationFee
	SetRegistrationFee      = ur.SetRegistrationFee
	WithdrawFees            = ur.WithdrawFees

	// Auditor
	NewAuditorKeyPair         = auditor.NewAuditorKeyPair
	EncryptViewKeyForAuditor  = auditor.EncryptViewKeyForAuditor
	AuditorDecryptViewKey     = auditor.AuditorDecryptViewKey
)
