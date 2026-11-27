# Anyone can contribute to the ceremony with the released program, which a coordinator queues and checks

A setup ceremony phase is run by a coordinator, `ceremony coordinate`, an HTTP service. The ceremony is always open: a participant needs nothing but its announcement. They get the participant's program from the ceremony's GitHub release, built by the `ceremony-release` workflow from a `ceremony-v*` tag with `SHA256SUMS` and provenance attestations, or build it from source, and run it. Built with the coordinator's address inside, it joins, waits its turn, contributes, uploads, and prints a receipt once the coordinator's transcript holds its contribution. The coordinator serves no participant code.

- **Joining.** The participant's program registers itself, under a name the participant may type, and gets a session token. Optionally (`-auth github`) it signs in on GitHub's device flow first, with an account old enough, and contributes once per account.
- **Queue.** The coordinator queues participants in the order they first ask and hands the turn to one at a time. The turn holder downloads the latest state, contributes, and uploads within a time limit.
- **Check.** It checks each upload against the state before it (`ceremony.Check`) before the next turn starts.
- **Passing turns on.** A queued participant keeps their place through a short absence (a laptop asleep, a network down), but is passed over if not asking when their turn comes. A turn is long (60 minutes), room for a slow laptop. But the turn holder's program sends a heartbeat every 10 seconds, and one silent for 2 minutes, such as a laptop closed mid-turn, loses the turn. So does a holder that never starts downloading, and an identity is given only a few turns without contributing.
- **Limits.** One network address may have only a few participants queued at once, register only so often, and make only so many contributions; the queue, the registrations, the downloads and the connections are bounded in all.
- **Transcript.** The phase's directory is the transcript (package `transcript`): `state-<i>` and `state-<i>.json`, written in that order on acceptance. It can be published as it stands, a restarted coordinator carries on from it, and `ceremony verify` downloads and checks it from end to end.
- **The page.** The coordinator serves a page that follows the phase, the live queue and the transcript, and links to the release (`-release-url`). It serves TLS with its own certificate (`-acme-domain`), with a given one (`-tls-cert`), or behind a TLS proxy of the operator's (`-trusted-proxy`).
- **Beacon.** A phase is sealed up to the head announced at its close, with a drand quicknet round that comes out after the close and the last contribution, checked against quicknet's pinned public key.

## Considered Options

- **Participants pass state files by hand** (the first version: `contribute`, `check`). Kept for offline use, but a ceremony anyone can join needs the passing automated.
- **A browser client, served by the coordinator.** Built and removed. The page ran the participant's program as WebAssembly, with no installation and no warnings about an unsigned program, and the coordinator served the native binaries too. But then the coordinator served the code that draws each participant's randomness: a malicious or compromised coordinator could know every contribution made with it, and with them a phase's whole secret. The program now comes from the release, which the coordinator does not control.
- **Invitations.** Built and removed. A personal link per participant gave verified names, but it meant someone had to send every participant a token, and a ceremony that is meant to be open to all should never be closed by its configuration. Names are self-chosen and marked so in the transcript.
- **An open lobby where whoever asks first when the turn is free takes it**, as Ethereum's KZG ceremony sequencer does. Rejected: the order is a race, so a participant can wait indefinitely.
- **GitHub sign-in as the only open mode.** Rejected as the default: it adds a browser step, and soundness does not need identities. Many identities controlled by one person cannot weaken the keys while one honest participant contributes. Identities guard only the queue's fairness, which the per-address limits cover for most ceremonies; GitHub sign-in is there when they do not.
- **A database for the transcript.** Rejected: the state files are the transcript a verifier replays, so the directory that holds them is the record, and there is nothing else to keep in step with it.
- **A reverse proxy for TLS.** Kept as an option (`-trusted-proxy`), but the coordinator can get its own certificate so that running a ceremony needs one program and a DNS name.

## Consequences

- **Soundness needs one contributor per phase who ran a program the coordinator did not choose:** from the release, checked against `SHA256SUMS` or its attestation, or built from source.
- **Session tokens are bearer secrets,** so the coordinator serves TLS; plain HTTP needs `-insecure-http`, for tests and rehearsals on one machine. A session lives in memory: after a restart the programs register again by themselves.
- **The coordinator can leave a participant out but cannot bias the keys.** Every contribution carries proofs that every seal checks again. The program accepts the coordinator's word that it is done only for a contribution it made itself, once the transcript holds it, and participants publish their contribution's sha256 to catch an omission.
- **Anyone admitted can upload,** so uploads are bounded before they are checked:
  - in size: the previous state's size plus 64 KiB;
  - in time: the turn's deadline;
  - in what decoding them may allocate: every decode refuses a declared length the rest of its file could not hold.
- **Unsigned programs.** The binaries are not code-signed (that needs the organization's Apple and Windows certificates). The release's one-line command per system downloads the program without the systems' warnings, checks its sha256, and runs it.
