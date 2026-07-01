# Protocol Description

For formal definitions (commitment structure, nullifier, key generation,private issuance, ZK proof statements, security goals, and performance),see [dvp_protocol.md](./dvp_protocol.md).

## Protocol Flow

### Assumptions

In the flow exposed below, we assume two users (Alice and Bob), each with a private version of their assets already.

```mermaid

---
config:
  theme: redux
---
sequenceDiagram
    autonumber
    participant Alice
    participant Chain as Blockchain
    participant Bob

    note over Chain: Runs a DvP<br>Smart Contract

    Note over Alice: Owns an existing note<br>Commitment_A = H(spend_pkA, saltA, amount=5, token_id=10)<br>(represents 5 USDT)
    Note over Bob: Owns an existing note<br>Commitment_B = H(spend_pkB, saltB, amount=1, token_id=25)<br>(represents 1 concert ticket)

```

### a . Off-chain Agreement

Before initiating any on-chain transaction, Alice and Bob agree on the terms of the trade through an off-chain channel.They establish which assets are being exchanged — including the token identifiers and amounts on each side — without any  
blockchain interaction.

```mermaid

---
config:
  theme: redux
---
sequenceDiagram
    autonumber
    participant Alice
    participant Chain as Blockchain
    participant Bob

    note over Alice, Bob: < Agree on trade parameters. Swap 5 USDT (token_id = 10) for 1 concert ticket(token_id = 25) >
```

### b. Initiating the DvP

Alice initiates the swap by constructing the transaction by following these steps

1. _Key encapsulation_ — Alice runs ML-KEM encapsulation on Bob's view public key, obtaining a shared secret `ss_B` and ciphertext `CTXT`.
2. _Salt derivation_ — Alice derives `salt_B` and `salt_A` deterministically from `ss_B` via HKDF, so that Bob can later reconstruct them after decapsulation.
3. _k generation_ - The shared secret `ss_B` is used as the key materiall; "encryption key" is a domain label that scopes this derivation to symmetric encryption.
4. _Payload encrpytion ENC TX DATA_ - Alices encrypted the payload with the derived key `k`. AES-GCM provides authenticated encryption, so any tampering with the ciphertext is detectable.
5. _Commitment_ — Alice creates `COMMIT_B` (the commitment Bob will receive) and `COMMIT_A` (the commitment Alice will receive from Bob),using the derived salts and the agreed token_ids and amounts.
6. _Payload encryption_ — Alice encrypts (`token_id`, `amount`) under a symmetric key derived from ss_B, producing ENC_TX_DATA. This allows Bob to verify the commitment without Alice revealing the plaintext publicly.
7. _Revert commitment_ — Alice creates `REVERT_COMMIT_A`, a self-addressed commitment over the same asset she is spending. If Bob does not respond before the deadline, the chain inserts this commitment instead, returning Alice's funds.
8. _Zero-knowledge proof_ — Alice generates the `nf_A` =`H(spend_pk_A, leafIndexA)`. She also generates a proof `π_A` attesting that she knows the spend key for the commitment she is nullifying, the nullifier is well-formed, the revert and destination commitments are consistent with the spent asset, and the commitment exists in the Merkle tree.

```mermaid

---
config:
  theme: redux
---
sequenceDiagram
  autonumber
  participant Alice
  participant Chain as Blockchain
  participant Bob
  note over Alice: Alice initiates the DvP


    rect rgb(191, 223, 255)

        note left of Alice: Create TX payload for Bob

        note over Alice: Generate new ('encrypted') salt:<br><br>ss_B, CTXT = ML-KEM.Encapsulate(view_pk)

        note over Alice: Set TX DATA: <br><br>m = (token_id || amount)<br><br>k = HKDF(ss_B, "encryption key")

        note over Alice: Encrypt TX Data: <br><br> ENC_TX_DATA = AES-GCM-ENC(k, m)

        note over Alice: Derive salt_B: <br><br> salt_B = HKDF(ss_B, "Bob salt")

        note over Alice: Create destination commitment:<br><br>COMMIT_B = H(spend_pk_B, salt_B, amount_1, token_id_1)

        note over Alice: Generate new salt:<br><br>salt_A = HKDF(ss_B, "Alice salt")

        %% Bob also needs to be able to generate the value of salt_A in order to prove that token_ids and amounts match across commitments

        note over Alice: Create (self) destination commitment:<br><br>COMMIT_A = H(spend_pk_A, salt_A, amount_2, token_id_2)

    end

    rect rgb(191, 223, 255)

        note left of Alice: Creation of Revert Commitment<br>(if TX fails, should revert to new commit)

        note over Alice: Generate new (revert) salt:<br><br>revert_salt_A
        note over Alice: Create revert commitment:<br><br>REVERT_COMMIT_A = H(spend_pk_A, revert_salt_A, amount_1, token_id_1)

    end



    rect rgb(191, 223, 255)

        note left of Alice: Create the ZKP for the transaction

        note over Alice: Create nullifier for tx: <br><br>nf_A = H(spend_sk_A, leafIndex_A)

        note over Alice: Create zero-knowledge proof (π_A):<br><br> - "I know the spend secret key for this commit"<br><br> - "This nullifier is well-formed"<br><br>- "The revert commit has the same amount and token_id as commit I'm spending"<br><br>- "The nullifier, the (self) destination commitment (COMMIT_A), and the REVERT_COMMIT_A are all associated with the same spend pk"<br><br>-"The destination (COMMIT_B) has the same token_id_1 and amount_1 as the commit I'm spending"<br><br>-"I know a merkle path that proves that the commitment I'm spending is in the tree"

    end

```

### c. Transaction verification

Upon receiving Alice's submission, the smart contract performs three independent checks before accepting the transaction:

1. Deadline validity — the contract confirms that the supplied deadline has not yet passed, ensuring the swap can still be completed within the agreed time window.
2. Nullifier status — the contract checks that `nf_A` has not already been spent or locked by a prior transaction, preventing double-spends.
3. Zero-knowledge proof — the contract verifies `π_A` on-chain is proof is valid or not.

If all theree conditions are valid simultaneously. The smart contract do the following

- Creates a `swap_id` = `H(COMMIT_A, REVERT_COMMIT_A, nf_A, COMMIT_B, deadline)`
- Marks `nf_A` as locked
- Emits the following data <`CTXT`, `COMMIT_B`, `ENC_TX_DATA`, `COMMIT_A`, `swap_id`>

```mermaid
---
config:
  theme: redux
---
sequenceDiagram
    autonumber
    participant Alice
    participant Chain as Blockchain
    participant Bob
   Alice->>Chain: < π_A, CTXT, COMMIT_B, ENC_TX_DATA, COMMIT_A, REVERT_COMMIT_A, nf_A, deadline>


    note over Chain: Check if<br>deadline is valid

    note over Chain: Check if nf_A<br>has been spent
    note over Chain: Verify ZK Proof


    alt (Verify(π_A) = TRUE) && (nf_A NOT MARKED AS SPENT) && (DEADLINE IS VALID)

        note over Chain: Create swap_id<br><br>swap_id = H(COMMIT_A, REVERT_COMMIT_A, nf_A, COMMIT_B, deadline)
        note over Chain: Mark nf_A as locked
        Chain ->> Alice: TX OK


    else (Verify(π_A) = FALSE) || (nf_A IS MARKED AS SPENT) || (DEADLINE IS NOT VALID)
      note over Chain: Reject TX
      Chain ->> Alice: TX ERROR

    end
```

### d. Bob receiving the transaction

Once the swap is recorded on-chain, Bob is notified of the pending transaction and receives (`CTXT`, `COMMIT_B`, `ENC_TX_DATA`, `COMMIT_A`,`swap_id`).

Bob begins by recovering the shared secret Alice used when constructing the payload:

1. _Decapsulation_ — Bob runs ML-KEM decapsulation on `CTXT` using his view secret key, recovering `ss_B`.
2. _Key and salt derivation_ — Bob derives the symmetric encryption key k` = HKDF(ss_B, "encryption key")` and his salt `salt_B = HKDF(ss_B, "Bob salt")`.
3. _Payload decryption_ — Bob decrypts `ENC_TX_DATA` using `k`, obtaining `token_id_1` and `amount_1` in plaintext.

Bob then performs two commitment checks to verify the swap is legitimate and intended for him:

- He recomputes `C = H(spend_pk_B, salt_B, amount_1, token_id_1)` and checks that `C == COMMIT_B`.
- He derives `salt_A = HKDF(ss_B, "Alice salt")`, recomputes `C* = H(spend_pk_A, salt_A, amount_2, token_id_2)`, and checks that `C* == COMMIT_A`.

If both checks pass, Bob construct his response:

1. _Nullifier_ :Bob computes `nf_B = H(spend_sk_B, leafIndex_B)`, the nullifier for the commitment he is spending.
2. _Zero-knowledge proof_: Bob generates `π_B` attesting that he knows the spend secret key for his commitment, the nullifier is well-formed, the destination commitment `COMMIT_A` encodes the same `token_id` and amount as the commitment he is spending, and the commitment exists in the Merkle tree.

```mermaid
---
config:
  theme: redux
  look: handDrawn
---
sequenceDiagram
    autonumber

    participant Alice
    participant Chain as Blockchain
    participant Bob

    note over Bob: Decapsulate CTXT and<br>obtain ss_B


    note over Bob: Obtain symmetric key:<br><br> k = HKDF(ss_B, "encryption key")

    note over Bob: Obtain salt:<br><br> salt_B = HKDF(ss_B, "Bob salt")


    note over Bob: Decrypt ENC_TX_DATA<br>(using symmetric key k)<br><br>obtain token_id_1 & amount_1

    note over Bob: Obtain commitment<br><br>C = H(spend_pk_B, salt_B, amount_1, token_id_1)
    note over Bob: Check if commitments match: <br><br>C == COMMIT_B

    note over Bob: Obtain salt_A<br><br>salt_A = HKDF(ss_B, "Alice salt")

    note over Bob: Obtain (Alice's) commitment:<br><br>C* = H(spent_pk_A, salt_A, amount_2, token_id_2)

    note over Bob: Check if commitments match:<br><br>C* == COMMIT_A

    note over Bob: Create nullifier for tx: <br><br>nf_B = H(spend_sk_B, leafIndex_B)

    note over Bob: Create zero-knowledge proof (π_B):<br><br> - "I know the spend sk for this commitment"<br><br> - "This nullifier is well-formed"<br><br>- "The destination commit (COMMIT_A) has the same token_id<br>and amount as the commit I'm spending"<br><br>-"I know a merkle path that proves that the commitment I'm spending is in the tree"


```

### e. Smart Contract checks Bob TX

Bob submits (`π_B`, `COMMIT_A`, `nf_B`, `swap_id`) to the smart contract. The contract must validate Bob's submission before finalizing the swap.

Three conditions must hold simultaneously:

1. _Deadline not exceeded_: the contract checks that Bob's submission arrives before the deadline embedded in `swap_id`.
2. _Nullifier status_ : the contract verifies that `nf_B` has not already been spent, preventing Bob from double-spending his commitment.
3. _Zero-knowledge proof_:the contract verifies`π_B` on-chain, confirming that Bob knows the spend key for his commitment, the nullifier is correctly formed, the destination commitment `COMMIT_A` is consistent with the asset Bob is delivering, and Bob's commitment exists in the Merkle tree.

If all conditions are met, the contract finalizes the swap atomically:

- `nf_A` is marked as spent
- `COMMIT_A` (Bob's asset, now owned by Alice) and `COMMIT_B` (Alice's asset, now owned by Bob) are inserted into the Merkle tree
- the swap is marked as completed.

If the deadline has passed before Bob submits, the contract executes the revert path instead:

- nf_A is marked as spent
- `REVERT_COMMIT_A` is inserted into the Merkle tree, returning Alice's original asset to her under a fresh commitment
- the swap is marked as failed (`timeout`), and Alice is notified.

```mermaid
---
config:
  theme: redux
  look: handDrawn
---
sequenceDiagram
    autonumber

    participant Alice
    participant Chain as Blockchain
    participant Bob

    alt (TX On Time) && (Verify(π_B) = TRUE) && (nf_B NOT MARKED AS SPENT)

      Bob->>Chain: < π_B, COMMIT_A, nf_B, swap_id >

      note over Chain: Mark nf_A as spent
      note over Chain: Mark nf_B as spent
      note over Chain: Insert COMMIT_A into tree
      note over Chain: Insert COMMIT_B into tree

      note over Chain: Mark DvP TX as completed


    else (TX Timeout)
      note over Chain: Mark nf_A as spent
      note over Chain: Insert REVERT_COMMIT_A into tree
      note over Chain: Mark DvP TX as failed (i.e., timeout)

      Chain ->> Alice: TX Reverted
    end


```

## Full Protocol

```mermaid
---
config:
  theme: redux
  look: handDrawn
---
sequenceDiagram
    autonumber

    participant Alice
    participant Chain as Blockchain
    participant Bob

    note over Chain: runs DvP Smart Contract


    note over Alice: Has commitment<br><br>Commitment_A = H(spend_pk_A, saltA, amount_1, token_id_1)

    note over Bob: Has commitment<br><br>Commitment_B = H(spend_pk_B, saltB, amount_2, token_id_2)


    note over Alice, Bob: Agree on trade parameters.<br>Swap 5 USDT (token_id = 10) for 1 concert ticket (token_id = 25)
    note over Alice, Bob: DvP can now start

    note over Alice: Alice initiates the DvP

    rect rgb(191, 223, 255)

        note left of Alice: Create encrypted payload<br>and output commitments

        note over Alice: Encapsulate to Bob's view key:<br><br>ss_B, CTXT = ML-KEM.Encapsulate(view_pk_B)

        note over Alice: Set transaction data:<br><br>m = (token_id_1 || amount_1)<br><br>k = HKDF(ss_B, "encryption key")

        note over Alice: Encrypt transaction data:<br><br>ENC_TX_DATA = AES-GCM.Enc(k, m)

        note over Alice: Derive Bob output salt:<br><br>salt_B = HKDF(ss_B, "Bob salt")

        note over Alice: Create Bob output commitment:<br><br>OUTPUT_COMMIT_B = H(spend_pk_B, salt_B, amount_1, token_id_1)

        note over Alice: Derive Alice output salt:<br><br>salt_A = HKDF(ss_B, "Alice salt")

        note over Alice: Create Alice output commitment:<br><br>OUTPUT_COMMIT_A = H(spend_pk_A, salt_A, amount_2, token_id_2)

    end

    rect rgb(191, 223, 255)

        note left of Alice: Create revert commitment<br>for timeout recovery

        note over Alice: Generate revert salt:<br><br>revert_salt_A

        note over Alice: Create revert commitment:<br><br>REVERT_COMMIT_A = H(spend_pk_A, revert_salt_A, amount_1, token_id_1)

    end

    rect rgb(191, 223, 255)

        note left of Alice: Create the zero-knowledge proof

        note over Alice: Create nullifier:<br><br>nf_A = H(spend_sk_A, leafIndex_A)

        note over Alice: Create zero-knowledge proof (π_A):<br><br>- "I know the spend secret key for the input commitment"<br><br>- "This nullifier is well-formed"<br><br>- "The revert commitment has the same amount and token_id as the input commitment being spent"<br><br>- "The nullifier, OUTPUT_COMMIT_A, and REVERT_COMMIT_A are all associated with the same spend public key"<br><br>- "OUTPUT_COMMIT_B has the same token_id_1 and amount_1 as the input commitment being spent"<br><br>- "I know a Merkle path proving that the input commitment is in the tree"

    end

    Alice->>Chain: < π_A, CTXT, OUTPUT_COMMIT_B, ENC_TX_DATA, OUTPUT_COMMIT_A, REVERT_COMMIT_A, nf_A, deadline >

    note over Chain: Check if:<br><br>- deadline field is well-formed<br>- current time is before deadline<br>- nf_A is not already locked or spent<br>- zero-knowledge proof π_A is valid

    alt (current time <= deadline) && (Verify(π_A) = TRUE) && (nf_A fresh)

        note over Chain: Create swap_id<br><br>swap_id = H(OUTPUT_COMMIT_A, REVERT_COMMIT_A, nf_A, OUTPUT_COMMIT_B, deadline)

        note over Chain: Mark nf_A as locked

        Chain->>Alice: TX OK

    else invalid proof, invalid deadline, or nf_A not fresh

        note over Chain: Reject transaction

        Chain->>Alice: TX ERROR

    end

    Chain->>Bob: < CTXT, OUTPUT_COMMIT_B, ENC_TX_DATA, OUTPUT_COMMIT_A, swap_id >

    note over Bob: Decapsulate CTXT:<br><br>ss_B = ML-KEM.Decapsulate(sk_view_B, CTXT)

    note over Bob: Derive symmetric key:<br><br>k = HKDF(ss_B, "encryption key")

    note over Bob: Derive Bob output salt:<br><br>salt_B = HKDF(ss_B, "Bob salt")

    note over Bob: Decrypt transaction data:<br><br>(token_id_1, amount_1) = AES-GCM.Dec(k, ENC_TX_DATA)

    note over Bob: Recompute expected Bob output commitment:<br><br>OBTAINED_COMMIT_B = H(spend_pk_B, salt_B, amount_1, token_id_1)

    note over Bob: Derive Alice output salt:<br><br>salt_A = HKDF(ss_B, "Alice salt")

    note over Bob: Recompute expected Alice output commitment:<br><br>OBTAINED_COMMIT_A = H(spend_pk_A, salt_A, amount_2, token_id_2)

    alt (OBTAINED_COMMIT_B != OUTPUT_COMMIT_B) || <br> (OBTAINED_COMMIT_A != OUTPUT_COMMIT_A)

        note over Bob: Abort

    else (OBTAINED_COMMIT_B == OUTPUT_COMMIT_B) &&<br> (OBTAINED_COMMIT_A == OUTPUT_COMMIT_A)


        note over Bob: Create nullifier:<br><br>nf_B = H(spend_sk_B, leafIndex_B)

        note over Bob: Create zero-knowledge proof (π_B):<br><br>- "I know the spend secret key for the input commitment"<br><br>- "This nullifier is well-formed"<br><br>- "OUTPUT_COMMIT_A has the same token_id_2 and amount_2 as Bob's input commitment"<br><br>- "I know a Merkle path proving that the input commitment is in the tree"

        Bob->>Chain: < π_B, OUTPUT_COMMIT_A, nf_B, swap_id >

    end

    note over Chain: Check if:<br><br>- current time is before deadline<br>- nf_B is not already spent<br>- swap_id is active<br>- zero-knowledge proof π_B is valid

    alt (current time < deadline) && (Verify(π_B) = TRUE) && (nf_B fresh)

        note over Chain: - Mark nf_A as spent<br><br>- Mark nf_B as spent<br><br>- Insert OUTPUT_COMMIT_A into the tree<br><br>- Insert OUTPUT_COMMIT_B into the tree

        note over Chain: Mark DvP transaction as completed

    else timeout

        note over Chain: Mark nf_A as spent

        note over Chain: Insert REVERT_COMMIT_A into the tree

        note over Chain: Mark DvP transaction as failed

        Chain->>Alice: TX reverted<br>(funds sent to REVERT_COMMIT_A)

    end






%%     note over Alice, Bob: Alice was able to send funds to Bob.<br><br>Only Bob can spend the received commitment<br><br>The protocol does not require any interaction from Bob.

```

#### Additional Remarks

Alice was able to send funds to Bob. Only Bob can spend the received commitment. The protocol does not require any interaction from Bob.

## Auditing

Our design supports different types of auditing. Concretely,
