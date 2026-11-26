# Enygma setup ceremony

A multi-party trusted setup for Enygma's Groth16 circuits, so that no single machine ever holds the
secret randomness (the "toxic waste") behind the proving and verifying keys. `make retail-keygen` and
the other keygen targets run gnark's single-party `groth16.Setup`: whoever runs it could forge
proofs. This tool runs the same setup as a chain of contributions, on gnark's `mpcsetup` (the
two-phase MPC of [Bowe, Gabizon and Miers](https://eprint.iacr.org/2017/1050)). **The keys are
sound as long as one contributor in each phase discarded their randomness**, however many others
collude, the coordinator included.

- **Phase 1** (the powers of τ) runs once and serves every circuit of every protocol, up to its
  domain size.
- **Phase 2** runs once per protocol, for all of that protocol's circuits at once.

The ceremony is always open: anyone with its address can contribute, with no installation and
no invitation. A participant opens the ceremony's page and presses **Contribute from this
browser**. The page waits for their turn, adds
the machine's randomness to the latest state in the browser itself, uploads the result, and hands
back a receipt. The coordinator checks every upload before anyone builds on it, and anyone can
download the transcript and check every step. The same program, `ceremony`, also runs from a
terminal for those who prefer it, and runs the coordinator and every check.

## Contributing

Open the link you were sent and press **Contribute from this browser**.

1. **It joins.** Type a name to show in the transcript if you like, or leave it empty to stay
   anonymous. If the ceremony uses GitHub sign-in, it shows a code to enter at
   github.com/login/device.
2. **It waits for your turn.** You can use other tabs and apps meanwhile. If your laptop sleeps
   or your network drops for a few minutes you keep your place. A chime sounds when your turn
   comes.
3. **On your turn, keep the tab in front** for a few minutes. Your browser downloads the latest
   state, adds your randomness and uploads the result: about 5 minutes at Phase 1's full size, a
   few seconds in Phase 2. In the background, the computer gives the tab less power and it takes
   several times longer; it still finishes.
4. **You get a receipt:** your contribution's sha256, to copy or save, and your row is marked in
   the transcript on the page. Publish the receipt if you like; anyone can check it is there.

The randomness comes from your machine (`crypto.getRandomValues` in the browser, `crypto/rand` in
the program). It is drawn inside gnark's `Contribute`, never written anywhere, and gone when the
tab or the program closes.

To run it as a program instead, open "Prefer to run a program?" on the page and paste the command
for your system. To trust nothing but the source, run `go run ./cmd/ceremony join -url <address>`
from this directory, at the commit the page names.

## Running a ceremony

### Quick start

On a server with a DNS name pointing at it (ports 80 and 443 open):

```bash
make ceremony-dist CEREMONY_URL=https://ceremony.example.org         # on your machine; copy dist/ over
./dist/ceremony-linux-amd64 phase1-new -size 262144 -out p1/state-0
./dist/ceremony-linux-amd64 coordinate -dir p1 -acme-domain ceremony.example.org -downloads dist \
    -close-at 2026-12-08T15:00:00Z
```

Then send `https://ceremony.example.org` to anyone you like, or post it: that is all a
participant needs.

### What the coordinator needs

- **A machine:** a Linux VM with 2 vCPUs, 4 GB of memory and 20 GB of disk, reachable on a DNS
  name with ports 80 and 443 open. It cannot run on a serverless host (Cloudflare Pages or
  Workers, Lambda): it keeps the queue in memory, the transcript on disk, and needs about 600 MB
  and 45 s of CPU to check each Phase 1 upload.
- **Disk:** 50 MB per Phase 1 contribution: 5 GB for 100 participants.
- **Slow steps:** auctions' Phase 2 start and seal take about 5 minutes and 550 MB each. They
  need not run on the server.

### 1. Build the participant program

```bash
make ceremony-dist CEREMONY_URL=https://ceremony.example.org
```

This writes `enygma_ceremony/dist/`:
- `ceremony.wasm` and `wasm_exec.js`, the program built for the browser, which the page's
  Contribute button runs;
- `ceremony` for macOS, Linux and Windows on x86-64 and ARM64, which join that coordinator when
  run with no arguments;
- `SHA256SUMS`.

The coordinator serves them all from `-downloads`. The build is reproducible: anyone building the
same commit with the same URL gets the same files.

### 2. How participants join (`-auth`)

The ceremony is always open to anyone with its address. `-auth` only says whether they also sign
in:

| `-auth` | Joining | Identity in the transcript |
|---|---|---|
| `open` (default) | Open the page and press Contribute. | A name they type (any language, spaces and punctuation; up to 64 characters), or `anonymous`. |
| `github` | The same, after signing in with a GitHub account older than `-github-min-age` (30 days). Each account contributes once. | Their GitHub login. |

- **Limits for open and GitHub phases.** At most `-max-per-address` participants (3) from one
  network address are queued at once, and an address registers at most 100 times an hour. One
  more from a full address waits for a place, and joins as soon as there is one.
- **The GitHub sign-in needs an OAuth app.** Create one under Settings → Developer settings →
  OAuth Apps → New OAuth App (any homepage and callback URL), tick **Enable Device Flow**, and
  pass its client ID as `-github-client-id`. There is no secret to keep: the device flow needs
  none. Browsers cannot call GitHub's sign-in themselves, so the coordinator relays those two
  requests for its own app.
- **Soundness doesn't depend on the mode.** Contributions from many identities controlled by one
  person cannot weaken the keys while one honest participant contributes. What the limits and
  GitHub sign-in guard is the queue, so that honest participants get their turn.

### 3. Phase 1, once for every protocol

1. Start Phase 1: `ceremony phase1-new -size 262144 -out p1/state-0`. The domain must be a power
   of two at least the largest circuit's constraint count; each `export_r1cs` prints the counts:

   | Protocol | Largest circuit | Constraints |
   |---|---|---|
   | `enygma_dvp_auctions` | AuctionBatch | 159,468 |
   | `enygma_payments` | Withdraw6 | 105,371 |
   | `enygma_dvp` | Payment2in | 7,443 |
   | `enygma_retail_payments` | Payment2in | 7,439 |

   So 2^18 = 262,144 serves all four.
2. Pick when Phase 1 closes, and the beacon: a round of [drand](https://drand.love)'s quicknet
   that comes out after the close, so nobody can know it while contributing.
   `ceremony beacon -at 2026-12-08T16:00:00Z` names the round. Announce both.
3. Serve it:

   ```bash
   ceremony coordinate -dir p1 -auth open -acme-domain ceremony.example.org \
       -downloads enygma_ceremony/dist -close-at 2026-12-08T15:00:00Z \
       -notice "Phase 1 closes 8 Dec 15:00 UTC; the beacon is drand quicknet round <R>."
   ```

   - **Turns:** a turn lasts at most `-turn` (60 minutes), room for a slow laptop or a tab left
     in the background. The participant's program sends a heartbeat every 10 seconds during its
     turn. One silent for `-turn-presence` (2 minutes), such as a laptop closed mid-turn, loses
     the turn to the next in line.

   - **TLS:** `-acme-domain` gets and renews a certificate from Let's Encrypt (on ports 443 and
     80). Behind a TLS proxy of your own, use `-listen :8080 -behind-proxy` instead.
   - **Scheduled close:** at `-close-at` it hands out no more turns, waits for the one under way,
     prints the states to seal, and exits. Ctrl-C does the same early; a second Ctrl-C stops at
     once.
   - **Restarts are safe:** the directory is the transcript, and participants' programs join
     again by themselves.

   To keep it running as a service, a systemd unit:

   ```ini
   [Unit]
   Description=Enygma setup ceremony coordinator
   After=network-online.target

   [Service]
   WorkingDirectory=/srv/ceremony
   ExecStart=/srv/ceremony/ceremony coordinate -dir p1 -acme-domain ceremony.example.org -downloads dist -close-at 2026-12-08T15:00:00Z
   AmbientCapabilities=CAP_NET_BIND_SERVICE
   Restart=on-failure
   # A stop closes the phase and lets the turn under way finish, for up to -turn.
   KillSignal=SIGINT
   TimeoutStopSec=65min

   [Install]
   WantedBy=multi-user.target
   ```

4. Seal it: `ceremony phase1-seal -drand <R> -out phase1.srs p1/state-1 … p1/state-<k>`. This
   fetches round R, checks its signature against quicknet's public key (pinned in `drand/`),
   replays and checks every contribution (about 45 s each at 2^18), and writes the SRS.

### 4. Phase 2, once per protocol

1. Freeze the protocol's circuits at a commit, and export them there: `go run ./cmd/export_r1cs
   -out <r1cs dir>`, from the protocol's gnark module (`enygma_retail_payments/gnark_circuits`,
   `enygma_dvp/gnark_circuits`, `enygma_payments/gnark-server` or
   `enygma_dvp_auctions/gnark_circuits`).
   - It writes `<Name>.r1cs` for each circuit the protocol's gnark server proves, and `keys.json`,
     the paths that server loads each circuit's keys from.
   - Compiling is deterministic, so anyone re-exporting at that commit gets the same files.
2. Start the phase: `ceremony phase2-new -ceremony <protocol> -srs phase1.srs -r1cs <r1cs dir> -out
   p2/state-0`.
3. Serve it, with a new close time and beacon round:
   `ceremony coordinate -dir p2 …`, same address, same page.
4. Seal it: `ceremony phase2-seal -ceremony <protocol> -srs phase1.srs -r1cs <r1cs dir> -drand <R>
   -keys <keys dir> p2/state-1 … p2/state-<k>`.
   - It writes each key at its `keys.json` path, so `<keys dir>` has the server's own layout
     (`scripts/keys/PaymentPK.key`, `keys/zkdvp/WithdrawPk6.key`, `scripts/keys/AuctionBatch.pk`,
     …).
   - Copy it over the gnark module's directory.
5. Stamp the keys and re-export what is built from them, as the protocol's keygen target does.
   For example, retail's README, "Regenerating Proving/Verifying Keys", covers `keys.src`, the VK
   exports and the verifier contracts.

DvP's server proves PrivateMint with retail's keys (the verifier contract is shared), so DvP's
export leaves PrivateMint out. Take its keys from retail's Phase 2, as `make dvp-keygen` copies
them today.

Other commands:
- `ceremony info <state>` says what a state file holds.
- `ceremony check -prev A -next B` checks one step.
- `ceremony contribute -in A -out B` contributes to a file handed over by hand.

### Checking a ceremony

Anyone can check a ceremony without trusting the coordinator:
- **Every step:** `ceremony verify -url <address> -dir <dir>` downloads a phase's transcript and
  checks it from end to end. Every state must have the sha256 the transcript gives it and follow
  the one before it, and a Phase 1 must start where every Phase 1 of its size starts.
- **The keys:**
  1. Re-export the circuits at the announced commit and compare their sha256s.
  2. Run `phase1-seal` and `phase2-seal` over the transcripts with the announced rounds.
  3. Compare the keys' sha256s with the deployed ones.

  A seal recomputes each phase's start itself, so a transcript that does not chain from the real
  start fails.

## What the coordinator can and cannot do

- **It cannot learn or bias the keys.** Contributions carry proofs that they build on the state
  before them; the coordinator checks them, and so does every seal. The beacon is fixed by drand
  after the close.
- **It can leave someone out,** by refusing or reordering turns. That is why participants publish
  their hashes, and why one honest contributor per phase suffices.
- **Uploads come from anyone admitted,** so the coordinator bounds what one costs before checking
  it:
  - at most the previous state's size plus 64 KiB;
  - read by the turn's deadline;
  - decoded so that no length a file declares makes a decoder allocate more than a small multiple
    of the file's size (`ceremony`'s `body.bounded`). Every decode is bounded this way, so a
    participant reading a coordinator's state is protected too;
  - decoded only with gnark's own domain size equal to the state's (`domainOf`). gnark sizes a
    Phase 1 or an SRS from a domain size inside it, and a crafted 0 made its decoder panic.
- **The page shows participants' names as text,** and is served with a Content Security Policy
  that runs nothing but its own script and its two workers. The workers may load the program
  from the coordinator and talk to nothing else.
- **The browser client is as trustworthy as the downloaded program,** because the coordinator
  serves both. A participant who trusts neither runs `go run ./cmd/ceremony join` from source,
  at the commit the page names.

## Testing

`go test ./...` in `enygma_ceremony` runs whole ceremonies on small circuits, and more:
- **A busy phase** (`participant`'s `TestManyAtOnce`): 40 programs join at once. Alongside them,
  participants misbehave on their turn: they go silent, upload garbage, upload back the state
  they were given, or upload a real contribution after their turn has passed. Others give up
  while they wait. Every program finishes, each contribution once; nothing else is accepted; the
  transcript checks from end to end.
- **The fuzz targets' seeds.** `make ceremony-fuzz` (`FUZZTIME`, 1 minute by default, per target)
  then searches for inputs that fail them, in everything the ceremony reads from others:

  | Target | Holds that |
  |---|---|
  | `ceremony` `FuzzDecode` | No bytes make a state's decoder panic, or allocate more than 64 times their size. Every refusal gives a reason. A state that decodes writes back byte for byte, so it has one sha256. |
  | `ceremony` `FuzzCheck` | `Check` never panics on any two files, and accepts only a different file numbered one past the state it builds on. |
  | `ceremony` `FuzzCheckEdits` | `Check` accepts a genuine contribution, and no edit of it: any changed, added or removed byte fails it. |
  | `ceremony` `FuzzContribute` | Contributing to any bytes never panics. |
  | `transcript` `FuzzLoad` | Whatever its entries hold, a transcript loads only as a chain. |
  | `transcript` `FuzzFetch` | Whatever a coordinator serves, `verify` writes only state files and their entries, inside its directory, and only states with the sha256 the transcript gives them. |
  | `drand` `FuzzVerify` | No signature passes but drand's own. |
  | `coordinator` `FuzzCleanName` | A name is shown tidied, with no control or invisible characters. |
  | `coordinator` `FuzzRequests` | No request makes the coordinator panic or answer 5xx, or breaks its transcript. |
  | `coordinator` `FuzzUpload` | A turn holder's upload is accepted only if it is a genuine contribution, not edited. The transcript holds what was accepted, and only that. |

  The targets that need a genuine contribution make one in each fuzzing process. The fuzzer
  mutates an edit script applied to it (`internal/fuzzedit`), so the script means the same
  edits in every process.

## Measured

On an Apple-silicon laptop:

- **Retail Phase 2** (six circuits, 802 to 7,439 constraints, on a domain of 8,192):
  - a contribution takes 0.5 s and a check 0.2 to 1.4 s; states are 1.5 to 2.2 MB;
  - `phase2-new` and `phase2-seal` take about 40 s;
  - the sealed keys pass every retail prove-handler test.
- **Phase 1 at 2^18:** states are 50 MB. A contribution takes 75 to 85 s on one core, at 280 MB.
  A check takes 46 s, at 580 MB.
- **In the browser** (the WebAssembly build, on V8, the engine of Chrome, Edge and Brave):
  - The contribution's arithmetic for Phase 1 at 2^18 takes 5.3 minutes on V8 at full speed,
    at 380 MB.
  - A full rehearsal in Brave, with the tab hidden behind another window, ran on macOS's slow
    cores (the renderer at background priority): the whole Phase 1 turn at 2^18 took 25
    minutes, downloads, upload and check included, held by heartbeats. Retail's Phase 2 turn
    took 79 s. A visible tab runs at full speed: a whole turn at 8,192 took 9 s, as on V8 alone.
  - The browser download is 3.5 MB gzipped.
- **A full rehearsal at the real size,** one native participant and one in the browser per
  phase, both phases sealed with real drand rounds:
  - Phase 1 at 2^18 sealed in 108 s; retail's Phase 2 started in 80 s and sealed in 62 s.
  - Retail's prove-handler tests, run with the sealed keys, prove with every one of its six
    circuits.
- **Auctions Phase 2 on the 2^18 SRS** (six circuits, AuctionBatch the largest at 159,468
  constraints):
  - `phase2-new` takes 5 minutes (310 s) at 545 MB, nearly all of it gnark evaluating
    AuctionBatch's constraints on one core. `phase2-seal` recomputes that start, so it takes at
    least as long.
  - A contribution takes about 3 s and a check 1 s; states are 14 MB.
- **Rehearsals on localhost:**
  - A participant opened the page in Brave and pressed the button. They waited while another
    participant's program held the turn, then contributed from the browser, got a receipt, and
    saw their row marked in the transcript.
  - Open admission, with four copies of the program downloaded from the coordinator's page and
    run with no arguments, from one address. The fourth waited for a place.
  - Then `verify`, and a seal with a real drand round.
  - Both phases on retail's circuits, with keys at retail's paths.

## Not built yet

- **Other browsers tested.** The browser path has run in a Chromium browser (Brave) and on V8 under
  Node. Safari and Firefox support everything it uses (WebAssembly, workers, the Wake Lock API),
  but they have not been tried.
- **Signed binaries,** for those who run the program rather than the page. macOS notarization
  needs an Apple Developer ID, and Windows signing needs a code-signing certificate: the
  organization's to obtain. The browser path needs neither, and the page's pasted commands avoid
  the systems' warnings.
- **Reusing a public Powers of Tau** (such as the Perpetual Powers of Tau) for Phase 1, instead of
  running our own.

## Layout

| Path | What |
|---|---|
| `ceremony/` | The library: state files, Phase 1, Phase 2, contribute, check, seal, key manifests. |
| `coordinator/` | The coordinator service: registration (open, or with GitHub sign-in), the queue, turns and heartbeats, checking uploads, the page, its workers and downloads. |
| `participant/` | A participant's side, in the program and the browser alike: joining, waiting, contributing in memory, uploading, heartbeats, riding out restarts. |
| `transcript/` | A phase's transcript: its directory layout, fetching it, checking it end to end. |
| `drand/` | Fetching a drand quicknet round and checking its signature. |
| `cmd/ceremony/` | The program. |
| `cmd/ceremony-web/` | The same participant, built for the browser (`ceremony.wasm`), which the page runs in a Web Worker. |
| `internal/testcircuits/` | Two small circuits (one with an in-circuit commitment) the tests run whole ceremonies for. |
| `internal/fuzzedit/` | Edit scripts, for fuzz targets that edit a contribution made in the fuzzing process. |

The module pins the gnark version every protocol's circuits are compiled with (v0.14.0): the
`.r1cs` files and state files are gnark's own encodings, which can change between versions.
