# 01 — ERC20 Private Mint

**Test:** `TestV2Erc20OnChain_PrivateMint`
**File:** `test/01_v2_erc20_private_mint_test.go`

The Issuer privately mints tokens directly into Alice's note without a public on-chain transfer.
No ERC20 `transfer` ever occurs — the token balance appears inside the vault atomically with the ZK proof.

---

## Diagram

```mermaid
sequenceDiagram
    participant Alice
    participant Issuer
    participant GnarkServer as Gnark Server :8081
    participant Verifier as PrivateMintVerifier
    participant DVP as EnygmaDvp
    participant Vault as Erc20CoinVault

    Note over Alice,Issuer: Step 1 — Off-chain key exchange
    Alice->>Issuer: pk_spend (public spend key)

    Note over Issuer,GnarkServer: Step 2 — Proof generation
    Issuer->>Issuer: pick random salt
    Issuer->>GnarkServer: POST /proof/privateMint<br/>{pk_spend, salt, amount=100, tokenId=0, contractAddr}
    GnarkServer->>GnarkServer: prove Poseidon4(pk_spend, salt, amount, tokenId) = commitment
    GnarkServer->>GnarkServer: prove Poseidon2(pk_spend, salt) = cipherText
    GnarkServer-->>Issuer: {proof[8], publicSignal[4]}<br/>publicSignal = [commitment, contractAddr, 0, cipherText]

    Note over Issuer,Vault: Step 3 — On-chain mint
    Issuer->>DVP: setVaultMintable(vaultId=0, true) (owner, once)
    Issuer->>DVP: privateMint(vaultId=0, commitment, proof)
    DVP->>DVP: require mintableVaults[0]
    DVP->>Verifier: verifyProof(proof, publicSignal)
    Verifier-->>DVP: ok (pairing check passes)
    DVP->>Vault: registerCoins([commitment])
    Vault->>Vault: insert commitment as Merkle leaf
    Vault-->>DVP: leafIndex
    DVP-->>Issuer: emit PrivateMint(vaultId=0, commitment, cipherText)

    Note over Alice: Step 4 — Note scanning
    Alice->>Alice: recompute Poseidon4(pk_spend, salt, 100, 0)<br/>== commitment ✓
    Alice->>Alice: recompute Poseidon2(pk_spend, salt)<br/>== cipherText ✓
    Alice->>Alice: note is spend-ready<br/>(WtSaltsIn=salt, WtValuesIn=100)
```

---

## Public Signal Layout

| Index | Name | Value |
|-------|------|-------|
| 0 | `commitment` | `Poseidon4(pk_spend, salt, amount, tokenId)` |
| 1 | `contractAddress` | `uint256(EnygmaDvp address)` |
| 2 | _(unused)_ | `0` |
| 3 | `cipherText` | `Poseidon2(pk_spend, salt)` |

## Key Contracts

| Contract | Function | Purpose |
|----------|----------|---------|
| `EnygmaDvp` | `privateMint(vaultId, commitment, proof)` | Entry point; role-gated to DEFAULT_OWNER_ROLE; only into a vault flagged mintable |
| `EnygmaDvp` | `setVaultMintable(vaultId, mintable)` | Owner flags a vault open (or closed) to private mints; every vault starts closed. A minted note has no deposit behind it, so flag only a vault whose notes are not redeemed from other users' deposits (the test deployment's issuer is the only depositor of its own token) |
| `PrivateMintVerifier` | `verifyProof(proof, publicSignal)` | Groth16 verifier with hardcoded VK |
| `Erc20CoinVault` | `registerCoins([commitment])` | Inserts Merkle leaf |
