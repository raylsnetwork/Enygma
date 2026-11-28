# Turns are drawn at random among the queued whose network has been given the fewest

The ceremony is open: anyone with the coordinator's address may queue (ADR 0005), and the coordinator handed the turn to the first in line. Every limit bounded what one network address held at once, and an open phase gives each registration a new identity, so whoever queued about one entry per turn-length of the phase at its opening, from as few as 56 IPv6 `/64`s of one home `/56`, was ahead of every honest participant for the whole phase. Each of those turns could end with a real contribution made from secrets kept: the phase looked busy and healthy, and its trapdoor was theirs (audit H3). A contributor the organizers lined up, whose program was running before the opening, never got a turn.

The organizers want the ceremony to stay open to anyone, so the fix is not a list of who may contribute.

- **A free turn is drawn**: a *network*, an IPv4 `/24` or an IPv6 `/48` (`-fair-prefix4`, `-fair-prefix6`), is drawn at random (from the operating system's randomness) among the networks whose queued participants still asking (and, in a GitHub phase, whose accounts) have been given the fewest turns, and its earliest participant gets the turn. A network is one ticket however many it queued, and queues at most `-max-per-network` (10) at once; a full queue makes room for a network with at least two fewer queued than the one with the most, so filling it against newcomers takes about `-max-queue` networks. Every turn counts as it is given, used or not, so a program drawn that never comes back stalls the queue at its own cost.
- **Queueing first wins nothing.** Each network served moves behind every network not yet served, and among the least-served the order is a draw. To keep a participant from a network not yet served out with probability 1 − ε over T turns, someone needs fresh networks faster than turns are given, about T/ε of them; holding a turn without contributing is bounded by `-upload-within` from the first download, which no stalled or repeated download extends, and, once an upload starts, by `-turn`, so a phase holds many turns. A participant on a network already served is behind every network not yet served: someone with one fresh network per turn can keep them waiting, which is why the seal's `-include` remains the guarantee.
- **Capture is visible and never sealed.** The log names each request's address, the status page counts turns that ended without a contribution, and a seal's `-include` refuses a chain without the contributors lined up (ADR 0007). A lined-up contributor can always contribute by hand while the coordinator is stopped, added with `ceremony append`, and an operator who sees a network hold the queue denies it (`-deny-file`, re-read on SIGHUP) without a restart.

## Considered Options

- **Invitations, or a list of logins that go first.** Rejected: it closes the ceremony to some by configuration, which ADR 0005 already refused.
- **Fewest turns first, then first come.** Rejected: whoever queues many fresh networks at the opening still wins every tie.
- **Smaller per-address limits.** Not enough alone: they bound what an address holds at once, not over the phase.
- **A minimum upload pace.** Rejected: a trickle just above it holds the turn to `-turn` anyway, and it shuts out an honest participant on a slow uplink.
- **A draw among addresses within a network.** Not taken: one who holds a `/56` has hundreds of `/64`s in its `/48`, so it bounds little, and the network is what the bound counts.

## Consequences

- **Participants in the next draw are told how many networks it is among;** others, a rough place in line.
- **The draw is in memory:** a restart starts the counts afresh, like the queue. Those queued draw again.
- **Households and offices behind one `/24` or `/48` share turns,** and once one of them has contributed, the others wait behind every network not yet served. Within a network, its places and turns go first come, first served, so one who shares a network with someone queueing many entries waits behind them: a network, not an address, is what the draw is fair to.
