package core

import (
	"encoding/json"
	"fmt"
	"math/big"
)

// PoseidonEncrypt calls the gnark server's /util/poseidonEncrypt endpoint to
// encrypt plaintext values using the Poseidon sponge cipher.
// key is [X, Y] of the BabyJubJub shared key (authKey = mulPointEscalar(auditorPubKey, random)).
// nonce must be < 2^128.
// realLength is the number of meaningful plaintext values.
// Returns the encrypted values including MAC (length = ceil(realLength/3)*3 + 1).
func (c *GnarkClient) PoseidonEncrypt(key [2]*big.Int, nonce *big.Int, realLength int, plaintext []*big.Int) ([]*big.Int, error) {
	plaintextStrs := make([]string, len(plaintext))
	for i, v := range plaintext {
		plaintextStrs[i] = v.String()
	}
	payload := map[string]interface{}{
		"key":        [2]string{key[0].String(), key[1].String()},
		"nonce":      nonce.String(),
		"realLength": realLength,
		"plaintext":  plaintextStrs,
	}
	body, err := c.PostProof("/util/poseidonEncrypt", payload)
	if err != nil {
		return nil, fmt.Errorf("poseidonEncrypt request failed: %w", err)
	}
	var resp struct {
		Encrypted []json.Number `json:"encrypted"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse poseidonEncrypt response: %w", err)
	}
	result := make([]*big.Int, len(resp.Encrypted))
	for i, n := range resp.Encrypted {
		v, ok := new(big.Int).SetString(n.String(), 10)
		if !ok {
			return nil, fmt.Errorf("invalid big int in encrypted response index %d", i)
		}
		result[i] = v
	}
	return result, nil
}

// AuditorDecrypt decrypts values encrypted by the auditor circuit using the
// Poseidon sponge cipher via the gnark server's /util/poseidonDecrypt endpoint.
//
// authKeyX, authKeyY is the shared encryption key: StAuditorAuthKey = mulPointEscalar(pubKey, random).
// nonce is StAuditorNonce from the proof data.
// encrypted is StAuditorEncryptedValues from the proof data.
// realLength is the number of plaintext values (6 for fungible, 3 for non-fungible).
//
// Returns an error if the MAC does not match (wrong key or tampered ciphertext).
func (c *GnarkClient) AuditorDecrypt(authKeyX, authKeyY, nonce *big.Int, encrypted []*big.Int, realLength int) ([]*big.Int, error) {
	encStrs := make([]string, len(encrypted))
	for i, v := range encrypted {
		encStrs[i] = v.String()
	}
	payload := map[string]interface{}{
		"key":        [2]string{authKeyX.String(), authKeyY.String()},
		"nonce":      nonce.String(),
		"realLength": realLength,
		"encrypted":  encStrs,
	}
	body, err := c.PostProof("/util/poseidonDecrypt", payload)
	if err != nil {
		return nil, fmt.Errorf("poseidonDecrypt request failed: %w", err)
	}
	var resp struct {
		Plaintext []json.Number `json:"plaintext"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse poseidonDecrypt response: %w", err)
	}
	result := make([]*big.Int, len(resp.Plaintext))
	for i, n := range resp.Plaintext {
		v, ok := new(big.Int).SetString(n.String(), 10)
		if !ok {
			return nil, fmt.Errorf("invalid big int in plaintext response index %d", i)
		}
		result[i] = v
	}
	return result, nil
}
