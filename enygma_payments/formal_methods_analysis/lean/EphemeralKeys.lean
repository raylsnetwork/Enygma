namespace Enygma
namespace EphemeralKeys

noncomputable section

abbrev Party := Nat
abbrev BlockNumber := Nat

axiom Secret        : Type
axiom SymmetricKey  : Type
axiom Plaintext     : Type
axiom Ciphertext    : Type

structure Channel where
  src : Party
  dst : Party

axiom sharedSecret : Channel → Secret
axiom HKDF        : Secret → BlockNumber → SymmetricKey
axiom encrypt     : SymmetricKey → Plaintext → Ciphertext
axiom decrypt     : SymmetricKey → Ciphertext → Option Plaintext

/-- Epoch key: K_{i,j}^n = HKDF(s_{i,j}, n). -/
def epochKey (c : Channel) (n : BlockNumber) : SymmetricKey :=
  HKDF (sharedSecret c) n

/-- Correctness for the right epoch key: decrypt(K^n, enc(K^n, m)) = m. -/
axiom decrypt_encrypt_same_epoch :
  ∀ (c : Channel) (n : BlockNumber) (m : Plaintext),
    decrypt (epochKey c n) (encrypt (epochKey c n) m) = some m

/-- HKDF outputs different keys for different epochs n ≠ n'. -/
axiom hkdf_epoch_separation :
  ∀ (s : Secret) {n n' : BlockNumber},
    n ≠ n' → HKDF s n ≠ HKDF s n'

/-- Wrong-key locality: decrypt(K^n, enc(K^{n'}, m)) ≠ m when n ≠ n'. -/
axiom wrong_epoch_decrypt_ne :
  ∀ (s : Secret) {n n' : BlockNumber} (_h : n ≠ n') (m : Plaintext),
    decrypt (HKDF s n) (encrypt (HKDF s n') m) ≠ some m

/-- If you hold K_{i,j}^n, you can decrypt epoch-n ciphertexts for channel c. -/
theorem can_decrypt_same_epoch
  (c : Channel) (n : BlockNumber) (m : Plaintext) :
  decrypt (epochKey c n) (encrypt (epochKey c n) m) = some m :=
  decrypt_encrypt_same_epoch c n m

/-- If you hold K_{i,j}^n, you cannot decrypt ciphertexts from any n' ≠ n. -/
theorem cannot_decrypt_other_epoch
  (c : Channel) {n n' : BlockNumber} (h : n ≠ n') (m : Plaintext) :
  decrypt (epochKey c n) (encrypt (epochKey c n') m) ≠ some m := by
  unfold epochKey
  exact wrong_epoch_decrypt_ne (sharedSecret c) h m


/------------------------------------------------------------------------------
  1. Use hkdf_epoch_separation to derive separation for epochKey
------------------------------------------------------------------------------/

/- Epoch keys for the same channel but different epochs are distinct. -/
theorem epochKey_separation
  (c : Channel) {n n' : BlockNumber} (h : n ≠ n') :
  epochKey c n ≠ epochKey c n' := by
  -- unfold epochKey so we can apply hkdf_epoch_separation
  unfold epochKey
  exact hkdf_epoch_separation (sharedSecret c) h


/------------------------------------------------------------------------------
  2. Lift to a "payload" notion: channel + epoch + ciphertext
------------------------------------------------------------------------------/

/- A concrete payload on chain: which channel, which epoch, and the ciphertext. -/
structure Payload where
  chan  : Channel
  epoch : BlockNumber
  body  : Ciphertext

/-- Encrypt a payload for channel c and epoch n under epochKey c n. -/
def encryptPayload (c : Channel) (n : BlockNumber) (m : Plaintext) : Payload :=
  { chan  := c
  , epoch := n
  , body  := encrypt (epochKey c n) m }

/-- Decrypt a payload with a given symmetric key. -/
def decryptWith (k : SymmetricKey) (p : Payload) : Option Plaintext :=
  decrypt k p.body

/-- Holding K_{i,j}^n is enough to decrypt payloads for (c, n). -/
theorem decryptPayload_same_epoch
  (c : Channel) (n : BlockNumber) (m : Plaintext) :
  decryptWith (epochKey c n) (encryptPayload c n m) = some m := by
  -- unfold wrapper definitions and use can_decrypt_same_epoch
  unfold decryptWith encryptPayload
  simp [can_decrypt_same_epoch (c := c) (n := n) (m := m)]

/-- Holding K_{i,j}^n is *not* enough to decrypt payloads from (c, n') with n' ≠ n. -/
theorem decryptPayload_other_epoch_ne
  (c : Channel) {n n' : BlockNumber} (h : n ≠ n') (m : Plaintext) :
  decryptWith (epochKey c n) (encryptPayload c n' m) ≠ some m := by
  -- unfold wrapper definitions and use cannot_decrypt_other_epoch
  unfold decryptWith encryptPayload
  -- now it's exactly the ciphertext-level theorem
  exact cannot_decrypt_other_epoch (c := c) (n := n) (n' := n') h m

end  -- noncomputable section
end EphemeralKeys
end Enygma
