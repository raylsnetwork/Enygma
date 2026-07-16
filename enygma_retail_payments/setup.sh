#!/usr/bin/env bash
# setup.sh — run from enygma_retail_payments/ root after:
#   1. npx hardhat node          (Terminal 1, keep running — run from enygma_dvp/)
#   2. cd gnark_circuits && go run generation.go   (run once when circuit changes)
#   3. bash setup.sh             (this script)
# Then separately:
#   4. cd gnark_circuits && go run main.go         (Terminal 2, keep running)
#   5. cd test && CC=/usr/bin/clang go test ./... -v -timeout 600s

set -e

ROOT="$(cd "$(dirname "$0")" && pwd)"
DVP_ROOT="$(cd "$ROOT/../enygma_dvp" && pwd)"
cd "$ROOT"

echo "==> [1/5] Compiling Solidity contracts (enygma_dvp)..."
cd "$DVP_ROOT"
npx hardhat compile

echo "==> [2/5] Regenerating Poseidon artifacts..."
node "$DVP_ROOT/regen_poseidon.mjs"

echo "==> [3/5] Copying updated ABIs to retail payments..."
cp "$DVP_ROOT/artifacts/contracts/core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json" \
   "$ROOT/contracts/abis/Erc20CoinVault.json"
cp "$DVP_ROOT/artifacts/contracts/core/contracts/EnygmaDvp.sol/EnygmaDvp.json" \
   "$ROOT/contracts/abis/EnygmaDvp.json"
cd "$ROOT"

echo "==> [4/5] Exporting VKs and regenerating PrivateMintVerifier..."
cd gnark_circuits
go run ./cmd/export_vk/ ../build

# Re-export the PrivateMint Solidity verifier from the current VK key,
# then recompile and update contracts/abis/PrivateMintVerifier.json.
# This must run every time after go run generation.go because Groth16
# Setup() produces fresh random keys — the on-chain verifier bytecode
# must always match the keys the server is using.
go run ./cmd/export_verifier/ /tmp/PrivateMintVerifier.sol
python3 - << 'PYEOF'
import json, subprocess, re

sol = "/tmp/PrivateMintVerifier.sol"
cr = subprocess.run(["solc","--bin","--optimize","--no-cbor-metadata",sol],capture_output=True,text=True)
rt = subprocess.run(["solc","--bin-runtime","--optimize","--no-cbor-metadata",sol],capture_output=True,text=True)
ab = subprocess.run(["solc","--abi","--optimize",sol],capture_output=True,text=True)

creation_bc = "0x" + re.search(r'Binary:\n([0-9a-f]+)',cr.stdout).group(1)
runtime_bc  = "0x" + re.search(r'Binary of the runtime part:\n([0-9a-f]+)',rt.stdout).group(1)
abi         = json.loads(re.search(r'Contract JSON ABI\n(\[.*\])',ab.stdout).group(1))

path = "../contracts/abis/PrivateMintVerifier.json"
with open(path) as f: artifact = json.load(f)
artifact["abi"] = abi
artifact["bytecode"] = creation_bc
artifact["deployedBytecode"] = runtime_bc
with open(path,"w") as f: json.dump(artifact,f,indent=2)
print(f"  PrivateMintVerifier.json updated ({len(creation_bc)} bytes bytecode)")
PYEOF

cd "$ROOT"

echo "==> [5/5] Deploying and initialising contracts..."
CC=/usr/bin/clang go build -C scripts -o /tmp/rp_deploy deploy.go && /tmp/rp_deploy
CC=/usr/bin/clang go build -C scripts -o /tmp/rp_init init.go   && /tmp/rp_init

echo ""
echo "Setup complete."
echo ""
echo "Next steps:"
echo "  Terminal A: cd gnark_circuits && go run main.go"
echo "  Terminal B: cd test && CC=/usr/bin/clang go test ./... -v -timeout 600s"
