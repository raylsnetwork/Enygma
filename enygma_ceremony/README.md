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

The arithmetic on the randomness, gnark's, does not take the same time whatever the randomness
is. Someone else running code on the same machine while it runs could in principle learn some of it
from timing. So contribute from a machine nobody else uses, with other programs, and browser tabs,
closed until the program says you are done. One contribution kept secret is what the ceremony
needs: yours counts if it stays yours.

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

- **Limits for open and GitHub phases.** They guard the queue, so honest participants get their
  turn:
  - at most `-max-per-address` participants (3) from one network address are queued, or hold the
    turn, at once. A participant who comes back from another address is counted there only if it
    has room. One more from a full address waits for a place, and joins as soon as there is one;
  - an address registers at most 100 times an hour, and only from a participant's program
    (a JSON request no other site's page can make its visitors send);
  - at most `-max-queue` (1000) are queued in all;
  - one identity is given at most `-max-turns` (3) turns without contributing;
  - the phase takes at most `-max-contributions` (1000) contributions, and
    `-max-contributions-per-address` (10) from one network address. The per-address count starts
    again when the coordinator restarts. Each contribution is a state file kept for the phase's
    life (50 MB at 2^18);
  - no turn is given while the disk under `-dir` has less free space than another state plus
    `-min-free-mib` (1024). The coordinator logs a warning, and turns resume once there is room;
  - it keeps at most `-max-sessions` (10,000) registrations at once;
  - anyone may download the transcript's states (`verify` does), 16 downloads at once and one per
    network address. Each must keep up a pace of about 34 KiB/s. `verify` waits for a free slot.

  A "network address" is an IPv4 address, or an IPv6 `/64` (`-ipv6-prefix`): one host is given a
  whole `/64`. These limits cannot be turned off by accident: `-max-per-address` is at least 1,
  and `-no-address-limit` lifts it and the per-address contribution limit, for a phase whose
  participants all come from one address.
- **One coordinator to a phase.** `coordinate` locks `<dir>/.lock` (on Linux and macOS), and
  refuses a directory another coordinator is serving. It syncs each accepted contribution, and
  the directory, to disk before telling its participant it is in.
- **The GitHub sign-in needs an OAuth app.**
  1. Create one under Settings → Developer settings → OAuth Apps → New OAuth App (any homepage
     and callback URL), and tick **Enable Device Flow**.
  2. Pass its client ID as `-github-client-id`.
  3. Generate a client secret and put it in a file only the coordinator's user can read
     (`chmod 600`). Pass that file as `-github-client-secret-file`, never as a flag value.

  With the secret, the coordinator asks GitHub whether each participant's token belongs to the
  app, and admits no other token: a token another site or another coordinator was given for the
  same account is not a sign-in here.
- **How the sign-in works, and what the coordinator sees.**
  - A participant's program signs in on GitHub's device flow, directly with GitHub: they enter a
    code at https://github.com/login/device. The coordinator relays nothing.
  - The program then sends the token it was granted to the coordinator, which reads the
    account's login, id and age with it. So the coordinator sees each participant's token.
  - The token needs no scope, so it opens nothing but the public profile. A token that carries
    any access (because the coordinator named an app the participant once gave more) is refused
    by the program before it is sent.
  - The tokens stay valid after the ceremony:
    - a participant can revoke theirs under Settings → Applications → Authorized OAuth Apps;
    - the operator can revoke them all, under the app's settings, once the phase closes.
- **Soundness doesn't depend on the mode.** Contributions from many identities controlled by one
  person cannot weaken the keys while one honest participant contributes. What the limits and
  GitHub sign-in guard is the queue, so that honest participants get their turn.

### 3. Phase 1, once for every protocol

1. Start Phase 1: `ceremony phase1-new -size 262144 -out p1/state-0`. The domain must be a power
   of two from 2 to 2^28 (the largest BN254's FFT supports), and at least the largest circuit's
   constraint count. Each `export_r1cs` prints the counts. A domain past 2^18 also needs
   participants to raise `join -max-state`. The largest circuits:

   | Protocol | Largest circuit | Constraints |
   |---|---|---|
   | `enygma_dvp_auctions` | AuctionBatch | 159,468 |
   | `enygma_payments` | Withdraw6 | 105,371 |
   | `enygma_dvp` | Payment2in | 11,331 |
   | `enygma_retail_payments` | Payment2in | 11,327 |

   So 2^18 = 262,144 serves all four.
2. Pick when Phase 1 closes, and the beacon: a round of [drand](https://drand.love)'s quicknet
   that comes out after the close and after the last turn can end, so nobody can know it while
   contributing. The last turn handed out before the close may run `-turn` past it, so
   `ceremony beacon -close-at 2026-12-08T15:00:00Z -turn 60m` names the first round at least that
   long after the close. Announce both.
3. Serve it:

   ```bash
   ceremony coordinate -dir p1 -auth open -acme-domain ceremony.example.org \
       -release-url https://github.com/<owner>/<repo>/releases/tag/ceremony-v1 \
       -close-at 2026-12-08T15:00:00Z \
       -notice "Phase 1 closes 8 Dec 15:00 UTC; the beacon is drand quicknet round <R>."
   ```

   - **Turns:** a turn lasts at most `-turn` (60 minutes), room for a slow laptop. The
     participant's program starts downloading the state at once, and sends a heartbeat three times
     every `-turn-presence` (2 minutes), at least every 10 seconds; the coordinator tells it how
     often. One that goes silent for `-turn-presence`, such as a laptop closed mid-turn, or that
     has not started downloading by then, loses the turn to the next in line.
   - **Waiting:** queued participants' programs ask every `-poll` (5 seconds). The turn goes to the
     first in line who has asked within `-presence` (30 seconds); one who has not, such as a laptop
     asleep, is passed over but keeps their place. One quiet for `-away` (10 minutes) leaves the
     queue. `-presence` must be at least twice `-poll`, and `-away` at least `-presence`.

   - **TLS:** `coordinate` serves participants over TLS, and will not start without one of:
     - `-acme-domain`, which gets and renews a certificate from Let's Encrypt (on ports 443 and
       80; port 80 sends everyone else to HTTPS). It keeps the certificate and the ACME account
       key in `-acme-cache`, an absolute path (by default `acme-cache` beside the phase's
       directory); give `-acme-email` for Let's Encrypt's notices.
     - `-tls-cert` and `-tls-key`, a certificate of your own: on `:443` by default, with port 80
       sending everyone to HTTPS.
     - `-trusted-proxy <address or CIDR,…>`, behind a TLS proxy of your own: plain HTTP on
       `127.0.0.1:8080` unless `-listen` says otherwise. A request from the proxy comes from the
       last address it put in `X-Forwarded-For`, so the proxy must append the client's address;
       a request from anywhere else counts as its own connection's address, whatever it says.
     - `-insecure-http`: plain HTTP on `127.0.0.1:8080`, for tests on this machine only.

     HTTPS answers carry `Strict-Transport-Security`, so a browser that has visited once never
     uses plain HTTP again. The participant's program refuses plain `http://` but to this machine.
     For the public name, set a CAA record that names your certificate authority, and watch
     Certificate Transparency logs for certificates issued for it while the ceremony runs.
   - **Scheduled close:** at `-close-at` it hands out no more turns, waits for the one under way,
     prints the transcript's **head** (the last contribution's sha256) and the seal command, and
     exits. **Announce the head at once, before the beacon round is out:** the seal takes it, and
     refuses a chain that does not end there, so no contribution can be added after the beacon is
     known, nor any left off the end. The phase stays closed: the directory records it
     (`closed`), and a coordinator started on it again serves its transcript, read-only, for
     anyone to check. Publish the directory somewhere static as well (a release, or object
     storage): the next phase is served at the same address.
   - **Interrupting:** Ctrl-C (or a systemd stop) stops handing out turns early and waits for the
     one under way; a second Ctrl-C stops at once. An interrupted phase is not closed: started
     again, it carries on. Queued participants are told to keep waiting meanwhile, so to end a
     phase and tell its participants, close it with `-close-at`: after an interrupt, their programs
     keep trying to reach the coordinator for 10 minutes before they give up.
   - **Restarts are safe:** the directory is the transcript, and participants' programs join
     again by themselves. The queue, sessions and the hourly registration counts are in memory,
     and start afresh.
   - **The start is checked** before the phase is served: a Phase 1's `state-0` must be where every
     Phase 1 of its size starts. For a Phase 2, give `-srs` and `-r1cs` (as `phase2-seal` takes
     them) to check its start too.

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
   # A stop hands out no more turns and lets the turn under way finish, for up to -turn;
   # started again, the phase carries on (a phase closed at -close-at stays closed).
   KillSignal=SIGINT
   TimeoutStopSec=65min

   [Install]
   WantedBy=multi-user.target
   ```

4. Seal it, with the head announced at the close:

   ```bash
   ceremony phase1-seal -transcript p1 -head <head> -drand <R> -close-at 2026-12-08T15:00:00Z -out phase1.srs
   ```

   This refuses round R unless it came out after the close and after the last contribution, then
   fetches it and checks its signature against quicknet's public key (pinned in `drand/`). It
   replays and checks every contribution up to the head (about 45 s each at 2^18), writes the
   SRS, and beside it `phase1.srs.json`: the head, the number of contributions, the round. The
   contributions can also be given as files, in order, instead of `-transcript`, for a chain
   handed over by hand: the last must be the head. For tests and rehearsals only,
   `-unverified-beacon <hex>` takes a beacon nothing checks, says so loudly, and records it.

### 4. Phase 2, once per protocol

1. Freeze the protocol's circuits at a commit, and export them there: `go run ./cmd/export_r1cs
   -out <r1cs dir>`, from the protocol's gnark module (`enygma_retail_payments/gnark_circuits`,
   `enygma_dvp/gnark_circuits`, `enygma_payments/gnark-server` or
   `enygma_dvp_auctions/gnark_circuits`).
   - It writes `<Name>.r1cs` for each circuit the protocol's gnark server proves, and `keys.json`,
     the paths that server loads each circuit's keys from.
   - Compiling is deterministic, so anyone re-exporting at that commit gets the same files.
   - The circuits are exactly those `keys.json` names: every `.r1cs` file in the directory must be
     one of them. Without a `keys.json`, name them with `-circuits A,B,…`. Every command that takes
     `-r1cs` prints each circuit with its sha256.
2. Start the phase: `ceremony phase2-new -ceremony <protocol> -srs phase1.srs -r1cs <r1cs dir> -out
   p2/state-0`.
3. Serve it, with a new close time and beacon round, and the SRS and circuits so its start is
   checked: `ceremony coordinate -dir p2 -srs phase1.srs -r1cs <r1cs dir> …`, same address, same
   page.
4. Seal it, with the head announced at the close:

   ```bash
   ceremony phase2-seal -ceremony <protocol> -srs phase1.srs -r1cs <r1cs dir> \
       -transcript p2 -head <head> -drand <R> -close-at <the close> -keys <keys dir>
   ```

   - It writes each key at its `keys.json` path, so `<keys dir>` has the server's own layout
     (`scripts/keys/PaymentPK.key`, `keys/zkdvp/WithdrawPk6.key`, `scripts/keys/AuctionBatch.pk`,
     …), and `ceremony-seal.json`: the head, the round, and each circuit's and key's sha256.
   - The keys appear all together or not at all, and never over a key already there.
   - Copy it over the gnark module's directory.
5. Stamp the keys and re-export what is built from them, as the protocol's keygen target does.
   For example, retail's README, "Regenerating Proving/Verifying Keys", covers `keys.src`, the VK
   exports and the verifier contracts.

DvP's server proves PrivateMint with retail's keys (the verifier contract is shared), so DvP's
export leaves PrivateMint out. Take its keys from retail's Phase 2, as `make dvp-keygen` copies
them today.

Other commands:
- `ceremony info <state>` says what a state file holds.
- `ceremony check -prev A -next B` checks one step: that B is a valid contribution on top of A.
  It does not check A, or how A was reached; `verify` checks a phase from its start.
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
  `-max-bytes` (64 GiB) in all and `-max-state` (64 MiB) for one state. Without `-url`, it checks
  `<dir>` as it is. It prints names exactly as quoted strings, and refuses a transcript whose names
  hold control characters.
- **State files:** a state is accepted only as the one encoding of what it holds (points written
  compressed), so its sha256 names it; a Phase 2 must hold at least one circuit, and each circuit
  at most 255 commitments, as gnark allows. `join` also downloads at most `-max-state` (64 MiB,
  room for Phase 1 at 2^18) for a state, which bounds the memory a coordinator can make a
  participant's program use; raise it only for a ceremony announced as larger.
- **The keys:**
  1. Re-export the circuits at the announced commit and compare their sha256s.
  2. Run `phase1-seal` and `phase2-seal` over the transcripts with the announced heads and
     rounds.
  3. Compare the keys' sha256s with the deployed ones, and with the seal's record
     (`ceremony-seal.json`).

  A seal recomputes each phase's start itself, so a transcript that does not chain from the real
  start fails. It seals up to the announced head and no other chain, and takes only a drand round
  out after the close and the last contribution, so someone resealing with the announced head and
  round gets the same keys, or finds out.

## What the coordinator can and cannot do

- **It cannot learn the keys' secret,** as long as one contributor in each phase ran a program it
  did not choose. It serves no participant code: participants run the program from the ceremony's
  GitHub release (built by the `ceremony-release` workflow from a tag, with `SHA256SUMS` and
  provenance attestations) or build it from source. A contributor who ran a program the
  coordinator served would be trusting the coordinator with their randomness; that is why there is
  no such path. Line up at least one contributor per phase whom outsiders trust, and who builds
  from source.
- **It cannot bias the keys,** given the procedure: contributions carry proofs that they build on
  the state before them; the coordinator checks them, and so does every seal. The head is
  announced at the close, before the beacon round is out, and the seals take nothing else; the
  beacon is a drand round out after the close and the last contribution, which the seals check.
  Someone who reseals with the announced head and round, and compares the keys, would see a
  coordinator or sealer who did otherwise.
- **It can leave someone out,** by refusing or reordering turns. That is why participants publish
  their hashes, and why one honest contributor per phase suffices.
- **Uploads come from anyone admitted,** so the coordinator bounds what one costs before checking
  it:
  - at most the previous state's size plus 64 KiB;
  - read by the turn's deadline;
  - decoded so that no length a file declares makes a decoder allocate more than a small multiple
    of the file's size (`ceremony`'s `body.bounded`; 23 times for the worst crafted Phase 2, as
    measured). Every decode is bounded this way, so a participant reading a coordinator's state is
    protected too, within `-max-state`;
  - accepted only as the one encoding of what they hold, so a state has one sha256;
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
  | `ceremony` `FuzzReencode` | As `FuzzCheckEdits`, with a point of the contribution first rewritten uncompressed, an edit no byte mutation makes: `Check` still accepts the genuine file only. |
  | `ceremony` `FuzzPhase2Shapes` | Contributing to a well-formed Phase 2 of any shape (circuits, commitments, lengths, challenge) never panics. |
  | `transcript` `FuzzLoad` | Whatever its entries hold, a transcript loads only as a chain. |
  | `transcript` `FuzzFetch` | Whatever a coordinator serves, `verify` writes only state files and their entries, inside its directory, and only states with the sha256 the transcript gives them. |
  | `drand` `FuzzVerify` | No signature passes but drand's own. |
  | `coordinator` `FuzzCleanName` | A name is shown tidied, with no control, invisible or unprintable characters, and at least one letter or digit. |
  | `coordinator` `FuzzRequests` | No request makes the coordinator panic or answer 5xx, or breaks its transcript. |
  | `coordinator` `FuzzUpload` | A turn holder's upload is accepted only if it is a genuine contribution, not edited. The transcript holds what was accepted, and only that. |

  The targets that need a genuine contribution make one in each fuzzing process. The fuzzer
  mutates an edit script applied to it (`internal/fuzzedit`), so the script means the same
  edits in every process.

## Measured

On an Apple-silicon laptop:

- **Retail Phase 2** (six circuits, 802 to 7,439 constraints, on a domain of 8,192), measured
  while the payment trees were depth 8; at depth 16 the largest is 11,327 constraints, on a
  domain of 16,384, and this has not been measured again:
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
