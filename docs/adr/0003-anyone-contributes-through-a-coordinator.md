# Anyone can contribute to the ceremony from a browser, which a coordinator queues and checks

A setup ceremony phase is run by a coordinator, `ceremony coordinate`, an HTTP service. The ceremony is always open: a participant needs nothing but its address. They open the coordinator's page and press one button. The page runs the participant's program, built for the browser as WebAssembly (`cmd/ceremony-web`), in a Web Worker: it joins, waits its turn, contributes in the browser, uploads, and hands back a receipt. The same program also runs natively (`ceremony`, built for each system with the coordinator's address inside), for those who prefer it.

- **Joining.** The participant's program registers itself, under a name the participant may type, and gets a session token. Optionally (`-auth github`) it signs in on GitHub's device flow first, with an account old enough, and contributes once per account.
- **Queue.** The coordinator queues participants in the order they first ask and hands the turn to one at a time. The turn holder downloads the latest state, contributes, and uploads within a time limit.
- **Check.** It checks each upload against the state before it (`ceremony.Check`) before the next turn starts.
- **Passing turns on.** A queued participant keeps their place through a short absence (a laptop asleep, a network down), but is passed over if not asking when their turn comes. A turn is long (60 minutes), room for a slow laptop or a tab left in the background. But the turn holder's program sends a heartbeat every 10 seconds, and one silent for 2 minutes, such as a laptop closed mid-turn, loses the turn. In the browser, a second worker sends the heartbeats, since the first is busy contributing.
- **Limits.** One network address may have only a few participants queued at once, and register only so often.
- **Transcript.** The phase's directory is the transcript (package `transcript`): `state-<i>` and `state-<i>.json`, written in that order on acceptance. It can be published as it stands, a restarted coordinator carries on from it, and `ceremony verify` downloads and checks it from end to end.
- **The page.** The coordinator serves a page with the downloads, a one-line command per system, the live queue and the transcript. It gets its own TLS certificate (`-acme-domain`).
- **Beacon.** A phase is sealed with a drand quicknet round, announced before it closes and checked against quicknet's pinned public key.

## Considered Options

- **Participants pass state files by hand** (the first version: `contribute`, `check`). Kept for offline use, but a ceremony anyone can join needs the passing automated.
- **A downloaded program as the only way to contribute.** Kept as an option. As the only way it fails the participant with no terminal: a browser-downloaded program loses its permission to run on macOS and is blocked as unsigned there, and Windows warns about it. Signing needs the organization's Apple and Windows certificates. The browser needs no installation, no certificate, and no troubleshooting, at the cost of running about 4 times slower than native: 5 minutes for a Phase 1 contribution at 2^18.
- **A browser client written apart, in JavaScript.** Rejected: two implementations of the contribution to keep in step and audit. The browser runs the same Go code as the program and the coordinator's checks.
- **Invitations.** Built and removed. A personal link per participant gave verified names, but it meant someone had to send every participant a token, and a ceremony that is meant to be open to all should never be closed by its configuration. Names are self-chosen and marked so in the transcript.
- **An open lobby where whoever asks first when the turn is free takes it**, as Ethereum's KZG ceremony sequencer does. Rejected: the order is a race, so a participant can wait indefinitely.
- **GitHub sign-in as the only open mode.** Rejected as the default: it adds a browser step, and soundness does not need identities. Many identities controlled by one person cannot weaken the keys while one honest participant contributes. Identities guard only the queue's fairness, which the per-address limits cover for most ceremonies; GitHub sign-in is there when they do not.
- **A database for the transcript.** Rejected: the state files are the transcript a verifier replays, so the directory that holds them is the record, and there is nothing else to keep in step with it.
- **A reverse proxy for TLS.** Kept as an option (`-behind-proxy`), but the coordinator gets its own certificate so that running a ceremony needs one program and a DNS name.

## Consequences

- **Session tokens are bearer secrets,** so the coordinator serves TLS. A session lives in memory: after a restart the programs register again by themselves.
- **The coordinator can leave a participant out but cannot bias the keys.** Every contribution carries proofs that every seal checks again. Participants publish their contribution's sha256 to catch an omission.
- **Anyone admitted can upload,** so uploads are bounded before they are checked:
  - in size: the previous state's size plus 64 KiB;
  - in time: the turn's deadline;
  - in what decoding them may allocate: every decode refuses a declared length the rest of its file could not hold.
- **Unsigned programs.** The native binaries are not code-signed (that needs the organization's Apple and Windows certificates). The browser path does not need them, and the page's one-line commands download the binaries without the systems' warnings.
- **The browser client is served by the coordinator,** as the binaries are: trusting either means trusting the coordinator's build, which is reproducible from the commit the page names. A participant who trusts neither runs the program from source.
- **Browsers slow a tab in the background,** so the page asks for the tab in front during the turn: it chimes when the turn comes, changes the tab's title, and keeps the screen awake. The long turn absorbs those who do not.
