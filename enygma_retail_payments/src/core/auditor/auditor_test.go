package auditor

import (
	"bytes"
	"crypto/mlkem"
	"testing"
)

// The contract checks only the lengths of a registration's audit ciphertexts
// (#434), so the auditor checks what they open to: the view key the user
// encrypted to it, which must be the one they registered.
func TestCheckAuditCiphertexts(t *testing.T) {
	auditor, err := NewAuditorKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	view := newViewKey(t)
	pkView := view.EncapsulationKey().Bytes()
	mlKemCt, aesCt, err := EncryptViewKeyForAuditor(auditor.EncapsKey, view)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("an honest registration opens to the registered view key", func(t *testing.T) {
		c := CheckAuditCiphertexts(auditor.DecapsKey, mlKemCt, aesCt, pkView)
		if !c.Opens() || !c.Matches {
			t.Fatalf("opens %v, matches %v; want both", c.Opens(), c.Matches)
		}
		if !bytes.Equal(c.ViewKey.Bytes(), view.Bytes()) {
			t.Error("recovered a view key other than the one encrypted")
		}
	})

	t.Run("well-sized junk does not open", func(t *testing.T) {
		// The lengths UserRegistry.register accepts: a capsule and a
		// nonce, the 64-byte seed and the GCM tag.
		junkKem, junkAes := bytes.Repeat([]byte{0xab}, 1088), bytes.Repeat([]byte{0xcd}, 92)
		c := CheckAuditCiphertexts(auditor.DecapsKey, junkKem, junkAes, pkView)
		if c.Opens() || c.Matches {
			t.Fatalf("opens %v, matches %v; want neither", c.Opens(), c.Matches)
		}
	})

	t.Run("ciphertexts encrypted to another auditor do not open", func(t *testing.T) {
		other, err := NewAuditorKeyPair()
		if err != nil {
			t.Fatal(err)
		}
		c := CheckAuditCiphertexts(other.DecapsKey, mlKemCt, aesCt, pkView)
		if c.Opens() || c.Matches {
			t.Fatalf("opens %v, matches %v; want neither", c.Opens(), c.Matches)
		}
	})

	t.Run("ciphertexts of another view key open but do not match", func(t *testing.T) {
		registered := newViewKey(t).EncapsulationKey().Bytes()
		c := CheckAuditCiphertexts(auditor.DecapsKey, mlKemCt, aesCt, registered)
		if !c.Opens() || c.Matches {
			t.Fatalf("opens %v, matches %v; want opens and no match", c.Opens(), c.Matches)
		}
	})
}

func newViewKey(t *testing.T) *mlkem.DecapsulationKey768 {
	t.Helper()
	k, err := mlkem.GenerateKey768()
	if err != nil {
		t.Fatal(err)
	}
	return k
}
