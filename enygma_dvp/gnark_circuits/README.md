# gnark_circuits

REST API server (port **8081**) that generates Groth16 proofs using [gnark](https://github.com/ConsenSys/gnark).
Go module name: `enygma_dvp/gnark_circuits`.

---

## Directory structure

```
gnark_circuits/
├── server/
│   ├── api/           ← HTTP route wiring
│   ├── circuits/      ← Per-circuit proof handlers (one sub-package per circuit)
│   └── config/        ← Port and key file paths (config.go)
├── templates/         ← gnark circuit definitions (Define() methods); the Payment family's are enygma_retail_payments' own
├── primitives/        ← Shared circuit gadgets (MerkleProof, Poseidon, nullifier, etc.)
├── poseidon/          ← Native Go Poseidon implementation used outside circuits
├── scripts/
│   └── keys/          ← Proving and verifying key files (*.key) — gitignored, from make dvp-keygen
├── cmd/
│   ├── server/        ← API server entry point  (go run ./cmd/server)
│   ├── keygen/        ← Generates the DvP and PrivateMint keys only (make dvp-keygen runs it)
│   ├── export_vk_init/ ← Exports VKs to circom JSON format consumed by scripts/cmd/init
│   └── export_vk_payment/ ← Exports the Payment deployment's VKs, for scripts/cmd/init_payment
├── retailkeys/        ← Refuses Payment-family keys that are not enygma_retail_payments'
└── utils/             ← Misc helpers
```

---

## Starting the server

**Start the server with `go run ./cmd/server`**

```bash
cd gnark_circuits
go run ./cmd/server
# Server starts on :8081
```

The server (`cmd/server`) and key generation (`cmd/keygen`) are separate `package main`
entry points under `cmd/`, so `go build ./...` and `go vet ./...` work from this directory.

The server must be started from the `gnark_circuits/` directory because key paths in
`server/config/config.go` are relative (e.g. `./scripts/keys/JoinErc20PK.key`).

---

## Proving/verifying keys

The keys in `scripts/keys/` are gitignored. Make them from the repo root, and again
after you change a circuit's `Define()` method in `templates/`:

```bash
make dvp-keygen
# Writes the *.key files and their stamp, keys.src, to enygma_dvp/gnark_circuits/scripts/keys/
```

`make dvp-keygen` runs `cmd/keygen`, which generates the DvP and PrivateMint keys. It
then copies enygma_retail_payments' committed PrivateMint and Payment-family keys
(`Payment`, `Payment2in`, `PaymentFee`, `PaymentRelayerFeePublic`, `UsdrFee`; the
Makefile's `DVP_RETAIL_KEYS`) into `scripts/keys/`. The committed
`PrivateMintVerifier.sol` is for retail's PrivateMint keys, and the server proves the
Payment-family circuits from retail's templates. `make dvp-stack` fails while the
stamp names other circuit sources, while a copied key is not retail's, or while
retail's own stamp names other retail circuit sources.

A Payment-family circuit is changed in `enygma_retail_payments/gnark_circuits`: run
`make retail-keygen`, then `make dvp-keygen`. `export_vk_init` and `export_vk_payment`
refuse Payment-family keys that are not, byte for byte, retail's.

Don't run `make dvp-keygen` while the server is running — key files are overwritten in place.

After regenerating, re-export the verifying keys so `scripts/cmd/init` picks up the changes:

```bash
cd gnark_circuits
go run ./cmd/export_vk_init/ ../build
# Writes one JSON file per circuit to build/
```

Then re-run `scripts/cmd/init` to register the new VKs onchain. The Payment
deployment's VKs are exported by `go run ./cmd/export_vk_payment ../build` and
registered by `scripts/cmd/init_payment`. `make dvp-stack` runs all of these steps on a
fresh chain.

---

## Configuration

Port and key file paths are hardcoded in `server/config/config.go`.
To change the port, edit the `Port` field in `Load()`.

---

## Adding a new circuit

1. Add the circuit definition in `templates/` (implement `frontend.Circuit` and `Define()`).
2. Add a proof handler in `server/circuits/<circuitName>/`.
3. Wire the route in `server/api/server.go`.
4. Add the new PK/VK paths to `server/config/config.go`.
5. Add the circuit setup call in `cmd/keygen/main.go` (`GenerationVkPk()`).
6. Regenerate keys and re-export VKs (see above).
