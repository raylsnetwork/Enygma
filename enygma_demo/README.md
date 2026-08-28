# Enygma Demo

An interactive, single-file walkthrough of the Enygma protocols. Open
[`index.html`](./index.html) in a browser — there is no build step, no server and no network
access of any kind.

The demo is deliberately **not** a mock-up. Commitments are real curve points, payloads are real
AES-GCM ciphertexts, and the conservation checks are computed rather than asserted. See
[What is real](#what-is-real-and-what-is-substituted) for the exact boundary.

## One identity, four networks

Enygma is four protocols, and each is deployed as its own network with its own `UserRegistry`,
its own participants and its own circuits. They are not one system, and the demo does not pretend
they are. What they genuinely share is your identity: a keypair minted once works on all four.

That is the shape of the walkthrough:

| Step | What happens |
| --- | --- |
| **1 · Keys** | You generate one spend keypair and one ML-KEM view keypair, for real, in the browser. Nothing is registered and no network is loaded until you do. |
| **2 · Choose** | Which network to open. Each card names the keys that network needs, and is ticked once your keys are on its registry. |
| **3 · Flow** | The network itself. |

**The first network you open forms in front of you**: an empty registry, then ten members
registering one at a time — yours first, through every derivation stage, because that is the part
worth reading. Whatever else the network needs to be usable follows (pairwise channels and opening
balances for Institutional; opening notes for DvP and Auctions).

**Every network after that, you simply join**: the other nine were registered before you arrived,
and joining is one `register(pk_spend, pk_view)` call carrying the keys you already hold. About
two seconds, no second key generation, ever. Each network offers *Replay how this network formed*
if you want the long version again.

Each network consumes the subset of your identity its circuits need:

| Protocol | spend keypair | view keypair (ML-KEM-768) | extras |
| --- | --- | --- | --- |
| **Institutional Payments** | ✅ | ✅ | a pairwise channel with every other bank |
| **Retail Payments** | ✅ | ✅ | an auditor escrow of your view key |
| **DvP** | ✅ | ✅ | — |
| **Auctions** | ✅ | — | the auction house's own key, not yours |

Each network is deep-linkable: `#/institutional`, `#/retail`, `#/dvp`, `#/auctions`. Opening one
before generating keys lands you back on step 1 — the identity is the prerequisite, not a formality.
The single-step ▶ control is still there for registering one more party by hand.

## The four protocols

| Product | What it shows |
| --- | --- |
| **Institutional Payments** | Confidential bank balances as Pedersen commitments; composing one payment by hand and stepping through the arithmetic that produces the envelope; the envelope as it lands on chain; and the recipient side, where a bank trials the messaging tag at its own slot. Its **Bridge** tab is where the two commitment schemes meet: `withdraw` turns an account balance into a note and `deposit` brings it back, spending the note and publishing a nullifier. |
| **Retail Payments** | Shielded ERC-20 transfers between people. One note in, two notes out — a payment and your change — with the recipient scanning the chain by trial decapsulation, and an auditor escrow on the side. |
| **DvP** | An atomic two-legged swap: an asset against cash, with both terminal outcomes — settlement and the deadline revert. Swapping lives here and only here; the Institutional Bridge tab hands off to it rather than duplicating it. |
| **Auctions** | Sealed-bid NFT auctions. Every bid is encrypted to a single auctioneer key, and only the winner's is ever opened. |

Institutional Payments is additionally viewed through a **perspective** selector — Bank, Private
Network Hub (the chain), Regulator, or Operator. The same ledger renders differently for each,
which is the point: what a party can see is a function of the keys it holds, not of a UI
permission flag.

## Provenance

The behaviour is taken from the protocol documents in this repository, not invented:

- [`enygma_payments/protocol_description.md`](../enygma_payments/protocol_description.md) —
  key generation and registration (§2–3), key agreement (§4), issuance (§5), the private-transfer
  envelope, nullifier, messaging tags, proof statement and Retrieve procedure (§6), and auditing (§7).
- [`enygma_payments/payload.mmd`](../enygma_payments/payload.mmd) — the wire layout of a transaction.
- [`enygma_retail_payments/protocol_description.md`](../enygma_retail_payments/protocol_description.md) —
  the retail note model, registration with auditor escrow, and the scan-by-decapsulation path.
- [`enygma_dvp/dvp_protocol.md`](../enygma_dvp/dvp_protocol.md) — note commitments, nullifiers,
  Alice's leg, Bob's retrieval and completion, and the revert path.
- [`enygma_dvp_auctions/docs/auction_protocol_v2.md`](../enygma_dvp_auctions/docs/auction_protocol_v2.md) —
  the sealed-bid auction layer built on top of DvP.
- [`enygma_payments/contracts/enygma/contracts/Enygma.sol`](../enygma_payments/contracts/enygma/contracts/Enygma.sol) —
  `transfer`, `withdraw`, `deposit` and the supply `check`.

Constraint counts and gas figures quoted in the pages are measured from the real circuits —
the DvP numbers come from [`enygma_dvp/README.md`](../enygma_dvp/README.md).

## What is real, and what is substituted

**Computed for real**, in the browser, using WebCrypto and a hand-written secp256k1 implementation:

- the identity in step 1: `sk ← crypto.getRandomValues(32)`, and both public halves derived under
  separate domain labels — `pk_spend = H("enygma/spend/v1" ‖ sk_spend)`,
  `pk_view = H("enygma/mlkem768/ek/v1" ‖ sk_view)`
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

Twelve headless suites drive the page in Chromium and check the cryptography from the outside —
re-deriving values with their own code and comparing, rather than trusting what is rendered.

```bash
cd enygma_demo
npm install            # playwright
npx playwright install chromium
npm test
```

To run a single suite: `node tests/keys-test.cjs`.

If Playwright or Chromium live outside the project, point at them explicitly:

```bash
PLAYWRIGHT_PATH=/path/to/playwright CHROMIUM_PATH=/path/to/chrome npm test
```

`DEMO_PAGE=/path/to/other.html` runs the suites against a different build of the page.

| Suite | Covers |
| --- | --- |
| `keys-test.cjs` | The shell: the page opens on key generation and nothing is seeded before it; both public keys re-derive independently from the secrets; regeneration draws fresh entropy; every registry starts empty and fills incrementally rather than in one jump; you are the first of the ten registered, carrying the `pk_spend` from step 1, in all four products; deep links are gated on holding keys; each product boots exactly once. |
| `env-test.cjs` | Envelope shape against §6; blinding factors are re-derivable by each recipient; the sender's slot tag is *not* derivable from any channel secret; per-bank Retrieve; a non-recipient cannot decrypt another slot. |
| `pk-test.cjs` | Interbank-settlement and client-payment legs in one transaction; fixed-length payloads, so a decoy, a settlement and a customer batch are byte-identical on the wire. |
| `flow-test.cjs` | Tab and pane wiring, the guided walkthrough end to end, a manual payment with its six derivation steps, and a cold start from an empty registry. |
| `inv-test.cjs` | Fresh accounts are `Com(0,0) = 𝒪` and identical for every bank; total supply is invariant across transfers. |
| `aud-test.cjs` | Decapsulating a disclosed capsule verifies that index; a commitment that disagrees is *not* marked audited; a bank opening its own channel is not an audit. |
| `bridge-test.cjs` | The Institutional Bridge tab: supply is unchanged in both directions, note-hash binding, Merkle root, owner unattributability per persona, that bringing a note back publishes a nullifier and spends the leaf without deleting it, double-spend rejection, escrow reconciliation, and that a freeze blocks both legs. Also that no swap UI survives here. |
| `swap-test.cjs` | The DvP network: all three output commitments re-derived from their openings, both nullifiers, salts as HKDF of the shared secret under separate labels, a fresh IV per sealing, a UI run end to end, and the revert path returning your own asset. |
| `nf-test.cjs` | The notes table exposes no per-leaf spend state, the published nullifier set never names a leaf, the contract's check order is stated where spending happens, and only a leaf's own holder can decide by recomputing its nullifier. |
| `frz-test.cjs`, `frz-test2.cjs` | The operator's halt blocks `transfer`, `withdraw` and `deposit` — including mid-walkthrough — and grants no visibility. |
| `faq-test.cjs` | The protocol explainer and FAQ: placement, expand/collapse, grounding in the spec's own terms, and layout down to 430px. |

Suites that exercise a network call `enterProduct(pg, 'institutional')` from
[`tests/_env.cjs`](./tests/_env.cjs), which walks step 1 and step 2 and then takes the **join**
path — the same two-second path a returning visitor takes. Pass `{ mode: 'formation' }` to watch a
network form from empty instead (what `keys-test.cjs` does), or `{ mode: 'empty' }` to stop at the
empty registry.

## Notes

- **Light theme only, one palette.** The four networks used to carry four grounds, four accents and
  three different dark variants; they now share a single light palette and one header rhythm, so
  moving between them reads as moving around one product rather than between four.
- Self-contained: no external scripts, styles, fonts or images. The published version runs under a
  content-security policy that blocks every external host, so everything is inline.
- Each network ends with a **provenance strip** linking the actual files it is a picture of —
  protocol descriptions, contracts and circuits in this repository — and every FAQ closes with a
  *This build* section stating plainly what is real cryptography here and what stands in for
  something a browser cannot do.
- The four networks share a document, and some element ids still appear in more than one of them
  (the FAQ ids are now namespaced per network; others are not). Each scopes every lookup to its own
  root element, so this is invisible at runtime — but a query written against `document` may
  resolve into a network you did not mean. Scope to `#app-payment`, `#app-retail`, `#app-dvp`
  or `#app-auctions`.
