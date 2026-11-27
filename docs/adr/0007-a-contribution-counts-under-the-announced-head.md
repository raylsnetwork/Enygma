# A contribution counts once its receipt is found under the announced head, in a copy the coordinator does not serve

The keys are sound if one contributor per phase kept their randomness secret, but only if that contribution is in the chain that is sealed. Before this, everything the participant's program checked of its contribution was the coordinator's answer: the state it built on, the transcript it read back (with its session token, so the coordinator knew whom it answered), and the `verify -url <coordinator> -mine` command it printed. A coordinator, or a network attacker holding a certificate for its name, could accept every honest contribution onto a private fork, show each participant their own fork, and seal a chain of its own contributions: every participant was told "Done" and got `OK` from the check they were told to run (audit H2). The same gap let an outsider who took every turn of an open phase look like a busy ceremony (H3).

The seal already binds the head announced at the close, and each contribution's proof names the state before it, so a receipt found in a chain ending at that head is in the chain sealed. What was missing was a check of membership there, made by the tools.

- **`verify -head`** refuses a chain that does not end at the announced head, and **`-mine`** takes the published receipts of every contributor one trusts, failing on any not in that chain.
- **`verify -static`** reads a copy of the phase's directory published apart from the coordinator, which it cannot answer each reader of differently.
- **Seals take `-include <receipts>`** and refuse a chain without them, on the bytes they replay; the seal record lists them. The close message tells the operator to check the contributors lined up are in, and to seal with `-include`.
- **The participant's program says what it knows:** the coordinator accepted the contribution. It prints the receipt as soon as the coordinator's reply names it, reads the transcript back with no session token, and prints the check that counts.
- **`verify` keeps evidence.** A host that now serves another history than a saved copy is refused before anything is written, and `-sync` moves what it contradicts aside rather than delete it.

## Considered Options

- **Signed acceptances** (a coordinator key baked into the release, signing each accepted contribution). Planned on top of this: it makes a coordinator's fork provable equivocation and reduces a network attacker without the key to denial, but a coordinator holding the key can still sign a fork, so it does not replace the head-bound check.
- **Asking the coordinator from another network address.** Not enough: a coordinator can tell a participant by more than their address, and a static copy is simpler to compare.

## Consequences

- **Every contributor's receipt must be published by the contributor, and checked after the close** by them or someone they trust; a contributor who is told "accepted" has not yet counted.
- **The organizers must line up contributors and pass their receipts to the seal** (`-include`). A phase sealed without `-include` says so.
- **Each phase's directory must be published somewhere static**, for `verify -static`.
