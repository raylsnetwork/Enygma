// Package core re-exports Phase 1 (user registration) symbols from
// src/core/user_registry for backward compatibility with existing callers.
package core

import ur "github.com/raylsnetwork/enygma_retail_payments/src/core/user_registry"

// UserKeys holds the two public keys a sender needs to pay a recipient.
type UserKeys = ur.UserKeys

var (
	Register              = ur.Register
	LookupKeys            = ur.LookupKeys
	GetUserCount          = ur.GetUserCount
	GetUserIndex          = ur.GetUserIndex
	GetUserAt             = ur.GetUserAt
	GetRegistrationFee    = ur.GetRegistrationFee
	SetRegistrationFee    = ur.SetRegistrationFee
	WithdrawFees          = ur.WithdrawFees
)
