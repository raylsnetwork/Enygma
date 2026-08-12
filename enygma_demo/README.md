# Enygma Demo

An interactive, single-file walkthrough of the Enygma protocols. Open
[`index.html`](./index.html) in a browser — there is no build step, no server and no network
access of any kind.

The demo is deliberately **not** a mock-up. Commitments are real curve points, payloads are real
AES-GCM ciphertexts, and the conservation checks are computed rather than asserted. See
[What is real](#what-is-real-and-what-is-substituted) for the exact boundary.

## The four tabs

| Tab | What it shows |
| --- | --- |
| **Key Setup** | The registry of public keys, pairwise ML-KEM key agreement, the symmetric shared-secret matrix, and the regulator's audit of it. A guided walkthrough replays the whole sequence from an empty registry. |
| **Payments** | Confidential balances as Pedersen commitments; composing one payment by hand and stepping through the arithmetic that produces the envelope; the envelope as it lands on chain; and the recipient side, where a bank trials the messaging tag at its own slot to discover whether the payment was for it. |
| **DvP** | Bridging value from the account model into hash-commitment notes, the Merkle tree of notes, and an atomic two-legged swap with both terminal outcomes — settlement and the deadline revert. |
| **Protocol** | A written explainer of the whole protocol, bound to live state from the other tabs, plus a technical FAQ. |

Everything is viewed through a **perspective** selector — Bank, Private Network Hub (the chain),
Regulator, or Operator. The same ledger renders differently for each, which is the point: what a
party can see is a function of the keys it holds, not of a UI permission flag.

## Provenance

The behaviour is taken from the protocol documents in this repository, not invented:

- [`enygma_payments/protocol_description.md`](../enygma_payments/protocol_description.md) —
  key generation and registration (§2–3), key agreement (§4), issuance (§5), the private-transfer
  envelope, nullifier, messaging tags, proof statement and Retrieve procedure (§6), and auditing (§7).
- [`enygma_payments/payload.mmd`](../enygma_payments/payload.mmd) — the wire layout of a transaction.
- [`enygma_dvp/dvp_protocol.md`](../enygma_dvp/dvp_protocol.md) — note commitments, nullifiers,
  Alice's leg, Bob's retrieval and completion, and the revert path.
- [`enygma_payments/contracts/enygma/contracts/Enygma.sol`](../enygma_payments/contracts/enygma/contracts/Enygma.sol) —
  `transfer`, `withdraw`, `deposit` and the supply `check`.

## What is real, and what is substituted

**Computed for real**, in the browser, using WebCrypto and a hand-written secp256k1 implementation:

- every Pedersen commitment `C = v·G + r·H`, and the point additions that apply a payment
- the conservation invariants `Σ rᵢ ≡ 0 (mod N)` and `Σ Cᵢ = 𝒪`
- `H` derived nothing-up-my-sleeve by hashing a constant to the curve
- all HKDF-SHA256 derivations — blinding factors, messaging tags, per-block content keys — under
  separate domain labels
- AES-256-GCM payloads. The failed trial decryptions are genuine AEAD authentication failures,
  not a rendered "no"
- the tag trial that discovers a payment, and the `C − r·H = v·G` check that opens it
- DvP note hashes, nullifiers and the Merkle root

**Substituted**, because a static page cannot do otherwise:

| Production | Here | Why |
| --- | --- | --- |
| Poseidon | SHA-256 | WebCrypto has no Poseidon. Structure is unchanged. |
| BN254 | secp256k1 | Same commitment algebra; no pairing is needed for what is demonstrated. |
| ML-KEM-768 encapsulation | a consistent pairwise secret | Running a lattice KEM in JavaScript is out of scope; every value *derived* from the secret is real. |
| Groth16 proof | a placeholder digest | Proof generation is not a browser-page operation. |

No claim the page makes about conservation, discovery or attribution depends on a substituted
part. The test suites re-derive each of them independently from the exposed state.

## Tests

Ten headless suites drive the page in Chromium and check the cryptography from the outside —
re-deriving values with their own code and comparing, rather than trusting what is rendered.

```bash
cd enygma_demo
npm install            # playwright
npx playwright install chromium
npm test               # 307 checks
```

To run a single suite: `node tests/env-test.cjs`.

If Playwright or Chromium live outside the project, point at them explicitly:

```bash
PLAYWRIGHT_PATH=/path/to/playwright CHROMIUM_PATH=/path/to/chrome npm test
```

| Suite | Covers |
| --- | --- |
| `env-test.cjs` | Envelope shape against §6; blinding factors are re-derivable by each recipient; the sender's slot tag is *not* derivable from any channel secret; per-bank Retrieve; a non-recipient cannot decrypt another slot. |
| `pk-test.cjs` | Interbank-settlement and client-payment legs in one transaction; fixed-length payloads, so a decoy, a settlement and a customer batch are byte-identical on the wire. |
| `flow-test.cjs` | Tab and pane wiring, the guided walkthrough end to end, a manual payment with its six derivation steps, and a cold start from an empty registry. |
| `inv-test.cjs` | Fresh accounts are `Com(0,0) = 𝒪` and identical for every bank; total supply is invariant across transfers. |
| `aud-test.cjs` | Decapsulating a disclosed capsule verifies that index; a commitment that disagrees is *not* marked audited; a bank opening its own channel is not an audit. |
| `dvp-test.cjs` | Bridge conservation, note-hash binding, Merkle root, owner unattributability per persona, swap open/retrieve/settle, the revert path, and escrow reconciliation against live notes. |
| `nf-test.cjs` | The notes table exposes no per-leaf spend state, the published nullifier set never names a leaf, and only a leaf's own holder can decide by recomputing its nullifier. |
| `frz-test.cjs`, `frz-test2.cjs` | The operator's halt blocks `transfer`, `withdraw` and `deposit` — including mid-walkthrough — and grants no visibility. |
| `faq-test.cjs` | The protocol explainer and FAQ: placement, expand/collapse, grounding in the spec's own terms, and layout down to 430px. |

## Notes

- Single light theme by design; no dark variant.
- Self-contained: no external scripts, styles, fonts or images. The published version runs under a
  content-security policy that blocks every external host, so everything is inline.
