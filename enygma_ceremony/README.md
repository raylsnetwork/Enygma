# Enygma setup ceremony

A multi-party trusted setup for Enygma's Groth16 circuits, so that no single machine ever holds the
secret randomness (the "toxic waste") behind the proving and verifying keys. `make retail-keygen` and
the other keygen targets run gnark's single-party `groth16.Setup`: whoever runs it could forge
proofs. This tool runs the same setup as a chain of contributions, on gnark's `mpcsetup` (the
two-phase MPC of [Bowe, Gabizon and Miers](https://eprint.iacr.org/2017/1050)). **The keys are
sound as long as one contributor in each phase ran a program the coordinator did not choose and
discarded their randomness**, however many others collude, the coordinator included. So the
coordinator serves no participant code: participants get the program from the ceremony's GitHub
release, built in the open from a tagged commit, or build it from source.

- **Phase 1** (the powers of τ) runs once and serves every circuit of every protocol, up to its
  domain size.
- **Phase 2** runs once per protocol, for all of that protocol's circuits at once.

The ceremony is always open: anyone with its address can contribute, with no invitation. A
participant runs one program, `ceremony`, from a terminal: it waits for their turn, adds the
machine's randomness to the latest state, uploads the result, and prints a receipt. The coordinator
checks every upload before anyone builds on it, and anyone can download the transcript and check
every step. The same program runs the coordinator and every check.

## Contributing

Open the ceremony's release on GitHub: the link is in the ceremony's announcement, and the
coordinator's page names it too. Take the command from the release page, not from the
coordinator's.

1. **Run the command for your system** from the release notes, in a terminal (PowerShell on
   Windows). It downloads the program from the release, checks its sha256 against the release, and
   runs it. Downloaded this way, with `curl` or `iwr`, the program raises no operating-system
   warning, signed or not. Or **build it from source** at the release's tag, the strongest option,
   since you trust only the code you build:

   ```bash
   git clone --depth 1 --branch <tag> https://github.com/<owner>/<repo>.git
   cd <repo>/enygma_ceremony
   go run ./cmd/ceremony join -url <the coordinator's address>
   ```

   The program first prints the commit it was built from, and its own sha256: compare them with
   the release.
2. **It joins.** Type a name to show in the transcript if you like, or leave it empty to stay
   anonymous. If the ceremony uses GitHub sign-in, it shows a code to enter at
   github.com/login/device.
3. **It waits for your turn.** Leave it running; you can use the computer meanwhile. If your laptop
   sleeps or your network drops for a few minutes you keep your place.
4. **On your turn** it downloads the latest state, adds your randomness and uploads the result:
   a few minutes at Phase 1's full size, a few seconds in Phase 2.
5. **You get a receipt:** your contribution's sha256. Publish it if you like; anyone can check it
   is there.

The randomness comes from your machine (`crypto/rand`). It is drawn inside gnark's `Contribute`, in
a separate process that exits as soon as it has written your contribution, with core dumps off: the
program never writes it anywhere, and the program you watch, which uploads the contribution and
prints your receipt, never holds it. Like anything in memory, it could still reach the disk if the
operating system moved that process's memory to swap or a hibernation file while it ran; full-disk
encryption (FileVault, BitLocker, LUKS) covers that.

The receipt is yours only if this program made it. If the coordinator says you have contributed
already, the program checks that against the contributions it made itself in this run, and against
the coordinator's transcript; for a contribution made in an earlier run, give the sha256 that run
printed: `ceremony join -mine <sha256>`. It never prints the coordinator's word as your receipt.

## Running a ceremony

### Quick start

1. Set the repository variable `CEREMONY_URL=https://ceremony.example.org`, and push a tag
   `ceremony-v1`. The `ceremony-release` workflow builds the program and publishes it as that tag's
   GitHub release (see "1. Release the participant program" below).
2. On a server with a DNS name pointing at it (ports 80 and 443 open), with the program from that
   release:

   ```bash
   ./ceremony-linux-amd64 phase1-new -size 262144 -out p1/state-0
   ./ceremony-linux-amd64 coordinate -dir p1 -acme-domain ceremony.example.org \
       -release-url https://github.com/<owner>/<repo>/releases/tag/ceremony-v1 \
       -close-at 2026-12-08T15:00:00Z
   ```

3. Announce the release's address and the coordinator's: that is all a participant needs.

### What the coordinator needs

- **A machine:** a Linux VM with 2 vCPUs, 4 GB of memory and 20 GB of disk, reachable on a DNS
  name with ports 80 and 443 open. It cannot run on a serverless host (Cloudflare Pages or
  Workers, Lambda): it keeps the queue in memory, the transcript on disk, and needs about 600 MB
  and 45 s of CPU to check each Phase 1 upload.
- **Disk:** 50 MB per Phase 1 contribution: 5 GB for 100 participants.
- **Slow steps:** auctions' Phase 2 start and seal take about 5 minutes and 550 MB each. They
  need not run on the server.

### 1. Release the participant program

Participants get the program from GitHub, never from the coordinator: the code that draws their
randomness must not be the coordinator's choice.

1. In the repository's settings, turn on immutable releases (so a release's files cannot be
   replaced), and set the variable `CEREMONY_URL` to the coordinator's https address.
2. Push a tag `ceremony-v<n>` at the commit to release. The `ceremony-release` workflow
   (`.github/workflows/ceremony-release.yml`) runs `make ceremony-dist CEREMONY_URL=…` and publishes:
   - `ceremony-<os>-<arch>` for macOS, Linux and Windows on x86-64 and ARM64, which join that
     coordinator when run with no arguments;
   - `SHA256SUMS`, and `BUILD`, the exact build command;
   - a provenance attestation for each binary (`gh attestation verify <file> --repo <owner>/<repo>`);
   - release notes with a one-line command per system that downloads the program from the
     release, checks its sha256 and runs it.

The build is reproducible: `make ceremony-dist` uses the exact toolchain `go.mod` pins, with CGO
off and `-trimpath`, so anyone building the same commit from a clean clone, with the same URL, gets
the same files. Locally it writes them to `enygma_ceremony/dist/`, replacing what was there.

### 2. How participants join (`-auth`)

The ceremony is always open to anyone with its address. `-auth` only says whether they also sign
in:

| `-auth` | Joining | Identity in the transcript |
|---|---|---|
| `open` (default) | Run the program from the release. | A name they type (any language, spaces and punctuation; up to 64 characters), or `anonymous`. |
| `github` | The same, after signing in with a GitHub account older than `-github-min-age` (30 days). Each account contributes once. | Their GitHub login. |

- **Limits for open and GitHub phases.** At most `-max-per-address` participants (3) from one
  network address are queued at once, and an address registers at most 100 times an hour. One
  more from a full address waits for a place, and joins as soon as there is one.
- **The GitHub sign-in needs an OAuth app.** Create one under Settings → Developer settings →
  OAuth Apps → New OAuth App (any homepage and callback URL), tick **Enable Device Flow**, and
  pass its client ID as `-github-client-id`. There is no secret to keep: the device flow needs
  none. The coordinator relays the device flow's two requests for its own app.
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
       -release-url https://github.com/<owner>/<repo>/releases/tag/ceremony-v1 \
       -close-at 2026-12-08T15:00:00Z \
       -notice "Phase 1 closes 8 Dec 15:00 UTC; the beacon is drand quicknet round <R>."
   ```

   - **Turns:** a turn lasts at most `-turn` (60 minutes), room for a slow laptop. The
     participant's program sends a heartbeat every 10 seconds during its turn. One silent for
     `-turn-presence` (2 minutes), such as a laptop closed mid-turn, loses the turn to the next in
     line.

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
   ExecStart=/srv/ceremony/ceremony coordinate -dir p1 -acme-domain ceremony.example.org -release-url https://github.com/<owner>/<repo>/releases/tag/ceremony-v1 -close-at 2026-12-08T15:00:00Z
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
  checks it from end to end, each step as it arrives, stopping at the first that fails. Every state
  must have the sha256 the transcript gives it and follow the one before it, and the phase must
  start where it should:
  - a Phase 1 where every Phase 1 of its size starts;
  - a Phase 2 where its SRS and circuits make it start. Give them, as `phase2-seal` takes them:
    `-srs phase1.srs -r1cs <r1cs dir>`. Without them, `verify` checks every step but ends `NOT
    VERIFIED`, with exit status 1.

  `verify` only adds to `<dir>`: it refuses a directory holding another phase, or states past the
  transcript's end, rather than overwrite them (`-sync` lets it), and downloads at most
  `-max-bytes` (64 GiB) in all. Without `-url`, it checks `<dir>` as it is. It prints names exactly
  as quoted strings, and refuses a transcript whose names hold control characters.
- **The keys:**
  1. Re-export the circuits at the announced commit and compare their sha256s.
  2. Run `phase1-seal` and `phase2-seal` over the transcripts with the announced rounds.
  3. Compare the keys' sha256s with the deployed ones.

  A seal recomputes each phase's start itself, so a transcript that does not chain from the real
  start fails.

## What the coordinator can and cannot do

- **It cannot learn the keys' secret,** as long as one contributor in each phase ran a program it
  did not choose. It serves no participant code: participants run the program from the ceremony's
  GitHub release (built by the `ceremony-release` workflow from a tag, with `SHA256SUMS` and
  provenance attestations) or build it from source. A contributor who ran a program the
  coordinator served would be trusting the coordinator with their randomness; that is why there is
  no such path. Line up at least one contributor per phase whom outsiders trust, and who builds
  from source.
- **It cannot bias the keys,** given the procedure: contributions carry proofs that they build on
  the state before them; the coordinator checks them, and so does every seal. The beacon is fixed by
  drand after the close.
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
  that runs nothing but its own script. It follows the phase and links to the release; it runs no
  participant code.

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
- **A full rehearsal at the real size,** one native participant and one in the browser (a path
  since removed) per phase, both phases sealed with real drand rounds:
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
  - Open admission, with four copies of the program run with no arguments, from one address.
    The fourth waited for a place.
  - Then `verify`, and a seal with a real drand round.
  - Both phases on retail's circuits, with keys at retail's paths.

## Not built yet

- **Signed binaries.** The release's commands do not need them: `curl` and `iwr` do not mark what
  they download as coming from the internet, so neither macOS nor Windows warns. A binary
  downloaded with a browser and opened from the desktop does warn; signing it would take an Apple
  Developer ID and a Windows code-signing certificate.
- **Reusing a public Powers of Tau** (such as the Perpetual Powers of Tau) for Phase 1, instead of
  running our own.

## Layout

| Path | What |
|---|---|
| `ceremony/` | The library: state files, Phase 1, Phase 2, contribute, check, seal, key manifests. |
| `coordinator/` | The coordinator service: registration (open, or with GitHub sign-in), the queue, turns and heartbeats, checking uploads, the phase's page. It serves no participant code. |
| `participant/` | A participant's side: joining, waiting, contributing in memory, uploading, heartbeats, riding out restarts. |
| `transcript/` | A phase's transcript: its directory layout, fetching it, checking it end to end. |
| `drand/` | Fetching a drand quicknet round and checking its signature. |
| `cmd/ceremony/` | The program. |
| `internal/testcircuits/` | Two small circuits (one with an in-circuit commitment) the tests run whole ceremonies for. |
| `internal/fuzzedit/` | Edit scripts, for fuzz targets that edit a contribution made in the fuzzing process. |

The module pins the gnark version every protocol's circuits are compiled with (v0.14.0): the
`.r1cs` files and state files are gnark's own encodings, which can change between versions.
