const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const script = path.join(__dirname, "../enygma_payments/run_scripts/deploy_node.js");
const source = fs.readFileSync(script, "utf8");

// Run the CLI with in-memory artifacts and a signer that cannot send transactions.
async function run(env = {}, rpcResult = "0x539") {
  const calls = [];
  const keys = [];
  const errors = [];
  const deployments = [];
  let status = 0;
  const ethers = {
    JsonRpcProvider: class {
      async send(method, params) {
        calls.push([method, params]);
        if (rpcResult instanceof Error) throw rpcResult;
        return rpcResult;
      }
      async getBlockNumber() { return 1; }
    },
    Wallet: class {
      constructor(key) { keys.push(key); }
      async getAddress() { return "deployer"; }
    },
    NonceManager: class {
      constructor(wallet) { return wallet; }
    },
    ContractFactory: class {
      async deploy(...args) {
        deployments.push(args);
        return {
          deploymentTransaction: () => ({ wait: async () => {} }),
          getAddress: async () => "contract",
        };
      }
    },
    keccak256: () => "runtime hash",
  };
  const modules = {
    "../contracts/enygma/node_modules/ethers": { ethers },
    "node:child_process": { execFileSync: () => {} },
    fs: {
      readFileSync: () => '{"abi":[],"bytecode":"0x01","deployedBytecode":"0x02"}',
      mkdirSync: () => {},
      writeFileSync: () => {},
    },
    path,
  };
  await vm.runInNewContext(source, {
    __dirname: path.dirname(script),
    require: (name) => modules[name],
    process: { env, exit: (code) => { status = code; } },
    console: { log: () => {}, error: (error) => errors.push(error.message) },
  }, { filename: script });
  return { calls, keys, errors, deployments, status };
}

function assertOwnerKeyRefused(result) {
  assert.equal(result.status, 1);
  assert.match(result.errors.join("\n"), /OWNER_KEY/);
  assert.deepEqual(result.keys, []);
  assert.deepEqual(result.deployments, []);
}

test("requires OWNER_KEY for a nonlocal deployment target", async () => {
  for (const target of [{ CHAIN_ID: "1" }, { RPC_URL: "https://rpc.invalid" }]) {
    const result = await run({ ...target, REVIEWED_VERIFIER_ARTIFACT: "reviewed.json" });
    assertOwnerKeyRefused(result);
  }
});

test("requires OWNER_KEY when the local RPC does not confirm chain 1337", async () => {
  for (const reply of ["0x1", "invalid", new Error("RPC unavailable")]) {
    const result = await run({}, reply);
    assertOwnerKeyRefused(result);
    assert.equal(result.calls[0]?.[0], "eth_chainId");
    assert.equal(result.calls[0]?.[1].length, 0);
  }
});

test("deploys with the demo key when the environment is unset and local RPC confirms chain 1337", async () => {
  const result = await run();
  assert.equal(result.status, 0, result.errors.join("\n"));
  assert.equal(result.calls.length, 1);
  assert.equal(result.calls[0][0], "eth_chainId");
  assert.equal(result.calls[0][1].length, 0);
  assert.deepEqual(result.keys, ["34d091c661db4c814d65c8ae9277b7055c0dde5a752ce5a3fdfd4ea11a8f7154"]);
  assert.deepEqual(result.deployments, [[1, 0], []]);
});

test("deploys with explicit OWNER_KEY on another chain", async () => {
  const ownerKey = "explicit-owner-key";
  const result = await run({
    OWNER_KEY: ownerKey,
    CHAIN_ID: "1",
    RPC_URL: "https://rpc.invalid",
    REVIEWED_VERIFIER_ARTIFACT: "reviewed.json",
  }, "0x1");
  assert.equal(result.status, 0, result.errors.join("\n"));
  assert.deepEqual(result.calls, []);
  assert.deepEqual(result.keys, [ownerKey]);
  assert.deepEqual(result.deployments, [[1, 0], []]);
});
