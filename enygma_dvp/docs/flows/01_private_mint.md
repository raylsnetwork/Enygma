# 01 — ERC20 Private Mint

**Test:** `TestV2Erc20OnChain_PrivateMint`
**File:** `test/01_v2_erc20_private_mint_test.go`

The Issuer privately mints a note to Alice without a public onchain transfer.
No token moves: the note has no deposit behind it, and its value is only the Issuer's promise.
So it goes into a mint-only vault of its own, which takes no deposits, never into a vault that holds other users' deposits.
The Issuer backs the vault itself, for example by transferring tokens to it, before a minted note is withdrawn.

---

## Diagram

```mermaid
sequenceDiagram
    participant Alice
    participant Issuer
    participant GnarkServer as Gnark Server :8081
    participant Verifier as PrivateMintVerifier
    participant DVP as EnygmaDvp
    participant Vault as Erc20CoinVault (mint-only)

    Note over Alice,Issuer: Step 1 — Offchain key exchange
    Alice->>Issuer: pk_spend (public spend key)

    Note over Issuer,GnarkServer: Step 2 — Proof generation
    Issuer->>Issuer: pick a random, non-zero salt
    Issuer->>GnarkServer: POST /proof/privateMint<br/>{pk_spend, salt, amount=100, tokenId=0, contractAddr}
    GnarkServer->>GnarkServer: prove Poseidon4(pk_spend, salt, amount, tokenId) = commitment
    GnarkServer->>GnarkServer: prove Poseidon(pk_spend, salt, contractAddr) = cipherText
    GnarkServer-->>Issuer: {proof[8], publicSignal[4]}<br/>publicSignal = [commitment, contractAddr, tokenId, cipherText]

    Note over Issuer,Vault: Step 3 — Onchain mint into a mint-only vault
    Issuer->>DVP: registerVault(vault, token, 1, 8) (owner)
    Issuer->>DVP: setVaultMintable(vaultId, true) (owner, while the vault is empty)
    DVP->>Vault: closeDeposits()
    Vault->>Vault: require no note yet; refuse deposits from now on
    Issuer->>DVP: privateMint(vaultId, commitment, proof)
    DVP->>DVP: require mintableVaults[vaultId] and a registered verifier
    DVP->>Verifier: verifyProof(proof, publicSignal)
    Verifier-->>DVP: ok (pairing check passes)
    DVP->>DVP: require commitment = signal 0, contractAddr = this, commitment not minted before
    DVP->>Vault: registerCoins([commitment])
    Vault->>Vault: insert commitment as Merkle leaf
    DVP-->>Issuer: emit PrivateMint(vaultId, commitment, cipherText)

    Note over Alice,Issuer: Step 4 — Offchain delivery
    Issuer->>Alice: salt and amount

    Note over Alice: Step 5 — Check the note
    Alice->>Alice: recompute Poseidon4(pk_spend, salt, 100, 0)<br/>== commitment ✓
    Alice->>Alice: recompute Poseidon(pk_spend, salt, contractAddr)<br/>== cipherText ✓
    Alice->>Alice: note is spend-ready<br/>(WtSaltsIn=salt, WtValuesIn=100)

    Note over Issuer,DVP: Step 6 — No second mint
    Issuer->>DVP: privateMint(vaultId, commitment, proof) again
    DVP-->>Issuer: revert CommitmentAlreadyMinted
```

---

## Public Signal Layout

| Index | Name | Value |
|-------|------|-------|
| 0 | `commitment` | `Poseidon4(pk_spend, salt, amount, tokenId)` |
| 1 | `contractAddress` | `uint256(EnygmaDvp address)` |
| 2 | `tokenId` | the minted note's token id; `privateMint` does not check it |
| 3 | `cipherText` | `Poseidon(pk_spend, salt, contractAddress)`, the note tag |

The amount is a private witness, bounded by the circuit to below 10^36.
No signal, event or token transfer shows it.

## What reaches Alice, and how

`cipherText` is a tag, not an encryption.
From the commitment and the tag, nobody can recover the salt or the amount, Alice included: she learns both only from the Issuer, offchain.
No wallet in this repository scans `PrivateMint` events, so a lost message from the Issuer means a lost note.

The salt must be random.
With salt 0, anyone could compute `Poseidon(pk_spend, 0, contractAddress)` for every spend key in the `UserRegistry` and find the recipient, then find the amount by trying values against the commitment.
The client's `Erc20PrivateMintProof` and `PrivateMintProof` refuse a salt that is missing or 0.

## Key Contracts

| Contract | Function | Purpose |
|----------|----------|---------|
| `EnygmaDvp` | `privateMint(vaultId, commitment, proof)` | Entry point; role-gated to DEFAULT_OWNER_ROLE; only into a vault flagged mintable, with a registered verifier (`PrivateMintVerifierNotRegistered`); each commitment once, in any vault (`CommitmentAlreadyMinted`) |
| `EnygmaDvp` | `setVaultMintable(vaultId, mintable)` | Owner flags a vault open (or closed) to private mints; every vault starts closed. Flagging calls the vault's `closeDeposits`, which closes it to deposits for good and is refused once the vault holds a note while open to deposits (`VaultHoldsNotes`). Clearing the flag stops mints only. So a vault takes mints or deposits, never both. Flag a new vault right after registering it: a deposit in between makes it a deposit vault for good |
| `EnygmaDvp` | `registerPrivateMintVerifier(verifier)` | Owner sets the mint verifier; it must hold code (`PrivateMintVerifierHasNoCode`), and each call logs `PrivateMintVerifierRegistered(verifier)`. The Payment deployments register none unless their config's `private-mint` switch is on |
| `PrivateMintVerifier` | `verifyProof(proof, publicSignal)` | Groth16 verifier with hardcoded VK |
| `Erc20CoinVault` | `registerCoins([commitment])` | Inserts Merkle leaf |
| `Erc20CoinVault` | `deposit`, `depositV2` | Revert `DepositsClosed` in a mint-only vault (`depositsClosed()` is true) |
