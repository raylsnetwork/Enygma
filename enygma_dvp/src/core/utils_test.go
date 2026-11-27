package core

import (
	"encoding/hex"
	"math/big"
	"strings"
	"testing"
)

// The HKDF labels are protocol: a wallet that derives a note's salt or key
// under another label cannot open its notes. These are known answers for the
// shared secret 0x00 0x01 … 0x1f, computed apart from this code with RFC 5869
// HKDF-SHA256 (no salt, 32 bytes out) under each label.
func TestPaymentHKDF_KnownAnswers(t *testing.T) {
	ss := make([]byte, 32)
	for i := range ss {
		ss[i] = byte(i)
	}
	for _, c := range []struct {
		name   string
		derive func([]byte) ([]byte, error)
		want   string
	}{
		{"DerivePaymentSalt", DerivePaymentSalt, "a13ce1018125060c595930ff0661c413ea878a539d8a71149b22e47c6e6ec85a"}, // "note salt"
		{"DerivePaymentKey", DerivePaymentKey, "49fe6e355f1eb4c6add54b6ff3104ccf7a200f225cdda7f917048518cc5d636a"},   // "encryption key"; gitleaks:allow (a test vector, not a secret)
		{"DeriveDvpSaltInit", DeriveDvpSaltInit, "5bfa3bcc5165897cfe0c9134e49f2f067ef25ab5397d401e4ce4aab6fa7d95cf"}, // "Init Salt"
	} {
		got, err := c.derive(ss)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if hex.EncodeToString(got) != c.want {
			t.Errorf("%s = %x, want %s", c.name, got, c.want)
		}
	}
}

// EncryptPayload seals each value as one 32-byte word. A value no word holds
// (nil, negative, or 2^256 and more) is an error, not a panic or a sealed
// absolute value, and the error names the field, never the value, which is
// private; 2^256-1, the largest word, round-trips.
func TestEncryptPayload_RefusesAValueNoWordHolds(t *testing.T) {
	key := make([]byte, 32)
	over := new(big.Int).Lsh(big.NewInt(1), 256)
	top := new(big.Int).Sub(over, big.NewInt(1))
	for _, c := range []struct {
		name            string
		tokenId, amount *big.Int
		field           string   // the field the error names
		bad             *big.Int // the refused value, which the error must not print
	}{
		{"nil token id", nil, big.NewInt(1), "token id", nil},
		{"nil amount", big.NewInt(1), nil, "amount", nil},
		{"negative token id", big.NewInt(-1), big.NewInt(1), "token id", big.NewInt(-1)},
		{"negative amount", big.NewInt(1), big.NewInt(-5), "amount", big.NewInt(-5)},
		{"token id of 2^256", over, big.NewInt(1), "token id", over},
		{"amount of 2^256", big.NewInt(1), over, "amount", over},
	} {
		t.Run(c.name, func(t *testing.T) {
			ct, err := EncryptPayload(key, c.tokenId, c.amount)
			if err == nil {
				t.Fatalf("sealed %x, want an error", ct)
			}
			if !strings.Contains(err.Error(), "payload "+c.field+" ") {
				t.Errorf("error %q does not name the %s", err, c.field)
			}
			if c.bad != nil && strings.Contains(err.Error(), c.bad.String()) {
				t.Errorf("error %q prints the refused value %s", err, c.bad)
			}
		})
	}
	t.Run("2^256-1 round-trips", func(t *testing.T) {
		ct, err := EncryptPayload(key, top, top)
		if err != nil {
			t.Fatalf("EncryptPayload(2^256-1): %v", err)
		}
		tokenId, amount, err := DecryptPayload(key, ct)
		if err != nil || tokenId.Cmp(top) != 0 || amount.Cmp(top) != 0 {
			t.Fatalf("opened (%v, %v), err %v; want 2^256-1 twice", tokenId, amount, err)
		}
	})
}
