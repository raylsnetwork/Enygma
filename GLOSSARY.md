# Glossary

**Statement layout**: the order of a circuit's public signals, as named fields (offset, size) with a total length. A circuit package derives its own from the circuit's public fields (every payments circuit: transfer, transfer-with-fee, USDr, withdraw, deposit and burn); `gnark-server/cmd/genlayout` renders the layouts into the Go package `enygma_payments/layout` and the Solidity library `StatementLayout`. enygma_retail_payments' Payment-family circuits derive theirs the same way, at each circuit's deployed config (`templates.PaymentConfig` and the like); `gnark_circuits/cmd/genlayout` renders them into the Go package `enygma_retail_payments/layout` and the Solidity library `PaymentStatementLayout` in enygma_dvp, whose `EnygmaDvp` and `Erc20CoinVault` verify retail receipts. enygma_dvp derives its own from its own circuits: the DvP and mint circuits (`DvpInitiator`, `DvpDestination`, `PrivateMint`) and its copies of the Payment-family circuits, at the same deployed configs; its `gnark_circuits/cmd/genlayout` renders all of them into the Go package `enygma_dvp/layout` (so enygma_dvp's Go code never imports retail's), and only the DvP and mint layouts into the Solidity library `DvpStatementLayout`. The Solidity Payment-family library stays retail's: a parity test in enygma_dvp's generator fails when its Payment-family templates stop deriving exactly `PaymentStatementLayout`. Derivation and rendering live in `enygma_gnark_shared/statement`.

**Statement length**: a statement layout's total number of public signals (`layout.<Circuit>Len` in Go, `<CIRCUIT>_LEN` in Solidity). It is the fixed arity of the circuit's public-signal array wherever one is declared: the relayer's accepted request length, `IEnygma`'s proof structs, and the test helpers that build signal arrays.

**Statement layout field names** (transfer circuit): `Fingerprint` (k×k matrix, row-major), `PublicKeys`, `PreviousCommitments` and `TxCommitments` (one (X, Y) point per participant, X first), `BlockNumber`, `AnonymitySet`, `MessageTags`, `Nullifier`, `DomainId`. The Solidity names are the same words in SCREAMING_SNAKE_CASE.

**Statement layout field names** (transfer-with-fee circuit, prefix `TransferFee`): `HashedSharedSecrets`, then the transfer circuit's `PublicKeys` through `Nullifier`, then `FeeAmount` (the public protocol fee), `SumTxCommitments` (the (X, Y) sum of the tx commitments), `SumTxValuesWithFee`, `DomainId`.

**Statement layout field names** (USDr circuit, prefix `Usdr`): the transfer circuit's `Fingerprint` through `Nullifier`, then `FeeAmount` (the fixed USDr fee), `DomainId`, `FeeRecipientKey` (spend public key of the account paid the fee).

**Statement layout field names** (withdraw circuit, prefix `Withdraw`): the transfer-with-fee circuit's `HashedSharedSecrets` through `Nullifier`, then `TotalDepositValue` (Σ of the DvP deposit amounts), `DomainId`, `DepositCommitments` (the DvP note commitment each of the 10 deposit slots creates, 0 if unused). Its first `DepositLen` signals share the deposit circuit's layout everywhere except the field after `Nullifier` (`TotalDepositValue` here, `Hash` in deposit).

**Statement layout field names** (deposit circuit, prefix `Deposit`): `HashedSharedSecrets` through `Nullifier` as in withdraw, then `Hash` (the deposit note commitment), `DomainId`.

**Statement layout field names** (burn circuit, prefix `Burn`): `PublicKey` (the burned account's spend public key), `PreviousCommitment` and `NewCommitment` (one (X, Y) balance commitment each, X first), `Amount`, `BlockNumber`, `Nullifier`, `DomainId`.

**Statement layout field names** (retail Payment-family circuits, prefixes `Payment` (1 input), `Payment2in` (the Payment circuit at 2 inputs), `PaymentFee`, `PaymentRelayerFee` (the PaymentRelayerFeePublic circuit) and `UsdrFee`, which avoids enygma_payments' `Usdr`): `Message` (0 for a payment, 1 for a withdrawal), `TreeNumbers`, `MerkleRoots` and `Nullifiers` (one per input), `CommitmentsOut` (one per output), `ContractAddress` (the vault the proof is bound to), then `Fee` (PaymentFee, PaymentRelayerFee, UsdrFee: the public fee) and `TokenId` (UsdrFee: USDr's public token id).

**Statement layout field names** (DvP initiator circuit, prefix `DvpInitiator`): `Message` (equals `CommitA`, linking the two legs of a swap), `TreeNumber`, `MerkleRoot` and `Nullifier` (of the initiator's one input note), `CommitB` (the counterparty receives the initiator's asset), `CommitA` (the initiator receives the counterparty's asset), `RevertCommitA` (the initiator's refund note if the swap times out), `CounterVault` (the vault `CommitA` must be paid from).

**Statement layout field names** (DvP destination circuit, prefix `DvpDestination`): `Message` (equals the initiator's `CommitB`), `TreeNumber`, `MerkleRoot` and `Nullifier` (of the counterparty's one input note), `CommitA` (the initiator's `CommitA`, recomputed), `CounterVault` (the vault `CommitB` must be paid from).

**Statement layout field names** (private mint circuit, prefix `PrivateMint`): `Commitment` (the minted note), `ContractAddress` (the deployment the proof is bound to), `TokenId`, `CipherText` (the note tag bound to that deployment).

**Note**: a private amount of one token, owned by a spend key. On chain it exists only as its commitment. A note is spent once and whole: a transfer spends one or two notes and creates new ones, the recipient's and the payer's change. _Avoid_: coin, UTXO.

**Commitment**: the public hash of a note's opening (the owner's `pkSpend`, a salt, the amount and the token id). A vault stores it as a leaf of its Merkle tree. Without the opening it reveals nothing about the note.

**Merkle tree**: a vault's append-only tree of commitments. When one fills, the vault starts the next, so each tree has a number counting up from 0. Spending a note proves its commitment is a leaf of the tree without revealing which leaf.

**Merkle root**: the hash that sums up a Merkle tree at one moment. The vault accepts any root its tree has ever had, so a proof made against an older root is still valid.

**Merkle path**: the sibling hashes from a commitment's leaf up to the Merkle root. The spender uses it privately, inside the proof, to show the note is in the tree.

**Nullifier**: the value published when a note is spent. It is derived from the owner's spend private key and the note's leaf position. The vault refuses a nullifier it has already seen, so each note is spent only once, and nobody else can link a nullifier to its commitment.

**Spend key**: a user's Poseidon key pair for owning notes; the public half (`pkSpend`) goes into every note commitment paid to them. _Avoid_: account key, address.

**View key**: a user's ML-KEM-768 key pair for finding notes; senders encrypt each note's opening to its public half (`pkView`), and the user decrypts with the private half. _Avoid_: viewing key, scan key.

**Audit ciphertexts**: the pair a user publishes at registration so the auditor can recover their view key: an ML-KEM-768 capsule to the auditor's public key (`mlKemAuditCt`) and the AES-256-GCM encryption of the view key's seed under the key derived from it (`aesAuditCt`).

**Registration**: a user's one-time publication of their spend key, view key and audit ciphertexts in the `UserRegistry`, from their own address; it cannot be repeated for that address. _Avoid_: sign-up, onboarding.

**Wallet**: a user's own software, which holds their spend key, view key and notes, makes their proofs, and builds their transactions. Private data never leaves it; only proofs, commitments, nullifiers and ciphertexts do. _Avoid_: frontend, client, app.

**Hop**: one interaction between two parts of the retail shield demo's system (an actor, the gnark server, the relayer or a contract), such as a proof request, a relay or a registration; the demo's architecture diagram animates one hop at a time. A **failed hop** is the one that failed its step; the diagram keeps it, in red, with the step's error. _Avoid_: edge, call, flow.

**Hop detail**: what a hop does to the protocol state (leaves added to a Merkle tree, a nullifier published, a proof's public inputs), shown on the retail shield demo's diagram as the hop plays. _Avoid_: component.

**Privacy line**: the boundary on the retail shield demo's diagram between the wallets, whose contents are private, and the network and chain, which see only what crosses it. _Avoid_: trust boundary, divider.

**Concept moment**: a short illustration of one privacy idea (hidden on chain, proof, relayer, finding your notes, auditor) that the retail shield demo plays once, before the step where the idea first happens, and holds until the presenter moves on. _Avoid_: explainer, intro, tutorial.
