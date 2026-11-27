const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const script = path.join(__dirname, "../enygma_payments/run_scripts/deploy_node.js");
const source = fs.readFileSync(script, "utf8");
const nonlocalTargets = [{ CHAIN_ID: "1" }, { RPC_URL: "https://rpc.invalid" }];

// Run the CLI with in-memory artifacts and a signer that cannot send transactions.
async function run(env = {}, rpcResult = "0x539", httpStatus = 200) {
  const calls = [];
  const targets = [];
  const keys = [];
  const errors = [];
  const deployments = [];
  let status = 0;
  const ethers = {
    FetchRequest: class {
      constructor(url) { this.url = url; }
      getUrlFunc = async () => ({ statusCode: httpStatus });
    },
    JsonRpcProvider: class {
      constructor(connection, chainId) {
        this.connection = connection;
        targets.push([connection.url ?? connection, chainId]);
      }
      async send(method, params) {
        calls.push([method, params]);
        await this.connection.getUrlFunc?.(this.connection);
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
  return { calls, targets, keys, errors, deployments, status };
}

function assertOwnerKeyRefused(result) {
  assert.equal(result.status, 1);
  assert.match(result.errors.join("\n"), /OWNER_KEY/);
  assert.deepEqual(result.keys, []);
  assert.deepEqual(result.deployments, []);
}

test("names both missing settings for a nonlocal deployment target", async () => {
  for (const target of nonlocalTargets) {
    const result = await run({ ...target, REVIEWED_VERIFIER_ARTIFACT: "reviewed.json" });
    assertOwnerKeyRefused(result);
    assert.match(result.errors.join("\n"), /VERIFIER_CHANGE_DELAY/);
  }
});

test("requires OWNER_KEY when the local RPC does not confirm chain 1337", async () => {
  for (const reply of ["0x1", "invalid", new Error("RPC unavailable")]) {
    const result = await run({ VERIFIER_CHANGE_DELAY: "86400" }, reply);
    assertOwnerKeyRefused(result);
    assert.equal(result.calls[0]?.[0], "eth_chainId");
    assert.equal(result.calls[0]?.[1].length, 0);
  }
});

test("deploys with the demo key when the environment is unset and local RPC confirms chain 1337", async () => {
  const result = await run();
  assert.equal(result.status, 0, result.errors.join("\n"));
  assert.deepEqual(result.targets, [["http://127.0.0.1:8545", 1337]]);
  assert.equal(result.calls.length, 1);
  assert.equal(result.calls[0][0], "eth_chainId");
  assert.equal(result.calls[0][1].length, 0);
  assert.deepEqual(result.keys, ["34d091c661db4c814d65c8ae9277b7055c0dde5a752ce5a3fdfd4ea11a8f7154"]);
  assert.deepEqual(result.deployments, [[1, 0], []]);
});

test("requires VERIFIER_CHANGE_DELAY outside local Hardhat", async () => {
  for (const target of nonlocalTargets) {
    const result = await run({
      ...target,
      OWNER_KEY: "explicit-owner-key",
      REVIEWED_VERIFIER_ARTIFACT: "reviewed.json",
    });
    assert.equal(result.status, 1);
    assert.match(result.errors.join("\n"), /VERIFIER_CHANGE_DELAY/);
    assert.deepEqual(result.deployments, []);
  }
});

test("requires VERIFIER_CHANGE_DELAY when the local RPC cannot confirm chain 1337", async () => {
  for (const [reply, status] of [["0x1", 200], [new Error("RPC unavailable"), 200], ["0x539", 307]]) {
    const result = await run({ OWNER_KEY: "explicit-owner-key" }, reply, status);
    assert.equal(result.status, 1);
    assert.match(result.errors.join("\n"), /VERIFIER_CHANGE_DELAY/);
    assert.deepEqual(result.deployments, []);
  }
});

test("uses zero delay with an explicit owner when the local RPC confirms chain 1337", async () => {
  const result = await run({ OWNER_KEY: "explicit-owner-key" });
  assert.equal(result.status, 0, result.errors.join("\n"));
  assert.deepEqual(result.keys, ["explicit-owner-key"]);
  assert.deepEqual(result.deployments, [[1, 0], []]);
});

test("deploys with explicit OWNER_KEY and VERIFIER_CHANGE_DELAY on another chain", async () => {
  const ownerKey = "explicit-owner-key";
  for (const delay of [0, 86400]) {
    const result = await run({
      OWNER_KEY: ownerKey,
      CHAIN_ID: "1",
      RPC_URL: "https://rpc.invalid",
      REVIEWED_VERIFIER_ARTIFACT: "reviewed.json",
      VERIFIER_CHANGE_DELAY: String(delay),
    }, "0x1");
    assert.equal(result.status, 0, result.errors.join("\n"));
    assert.deepEqual(result.targets, [["https://rpc.invalid", 1]]);
    assert.deepEqual(result.calls, []);
    assert.deepEqual(result.keys, [ownerKey]);
    assert.deepEqual(result.deployments, [[1, delay], []]);
  }
});

test("refuses an HTTP redirect before deploying with the demo key", async () => {
  assertOwnerKeyRefused(await run({}, "0x539", 307));
});
