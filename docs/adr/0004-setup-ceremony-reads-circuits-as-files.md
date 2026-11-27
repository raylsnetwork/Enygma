# The setup ceremony is one module that reads circuits as compiled files

Every protocol's production keys come from one setup ceremony tool, `enygma_ceremony`, a Go module at the repo root beside `enygma_gnark_shared`. It never imports a protocol: each protocol's gnark module gets a small `cmd/export_r1cs` that compiles its circuits and writes each one's constraint system as `<Name>.r1cs`, and the ceremony reads those files. A participant's `contribute` touches no circuit at all (gnark's `mpcsetup` updates and verifies a Phase 2 state without it); only starting and sealing Phase 2 read the `.r1cs` files. One Phase 1 serves every protocol: its domain covers the largest circuit, and each circuit's Phase 2 starts from the first powers of τ, cut to the circuit's own domain. Phase 2 runs once per protocol, one ceremony after another, for all of that protocol's circuits at once, and seals keys under the names the protocol's server already loads.

## Considered Options

- **A ceremony per protocol, inside each protocol's folder.** Rejected: four copies of the same security-critical code, and four binaries for participants to trust, when the contributor's work is the same for every protocol.
- **Put the ceremony in `enygma_gnark_shared`.** Rejected: the provers import that module, and a coordinator's dependencies don't belong in theirs.
- **Have the ceremony import each protocol's circuits.** Rejected: one module would depend on all four protocols, and a circuit change in any of them would rebuild the tool participants run.
- **A separate repository.** Rejected: a verifier recompiles the circuits at the commit the ceremony froze; one repository ties the tool and the circuits to that commit.
- **A Phase 1 per protocol, or per domain size.** Rejected: participants would contribute several times for no gain in soundness.

## Consequences

The `.r1cs` and state files are gnark's own encodings, so `enygma_ceremony` and every protocol's gnark module stay on one gnark version (v0.14.0 today; Dependabot already leaves gnark to hand upgrades). A circuit change after a protocol's Phase 2 needs a new Phase 2 for that protocol, not a new Phase 1, unless the circuit outgrows Phase 1's domain. `export_r1cs` lives outside the packages `cmd/keygen` builds, so adding it leaves `keys.src` unchanged; keys sealed by a ceremony are stamped with `scripts/keys_src.sh` as keygen's are. The keygen targets stay, for development keys. The protocols name and place their keys differently (`PaymentPK.key`, `EnygmaPk.key`, `keys/zkdvp/WithdrawPk6.key`, `AuctionBatch.pk`), so `export_r1cs` also writes a key manifest, `keys.json`, read from its server's own config, and the seal writes each key at the path the server loads it from.
