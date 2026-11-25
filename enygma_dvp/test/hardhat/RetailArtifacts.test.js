// enygma_retail_payments/contracts/abis/ holds copies of this project's
// artifacts, and the retail deploy deploys them as they are, so each copy must
// be this project's build of its source (#415, #421). The copies are the
// entries of enygma_retail_payments/scripts/copy_dvp_abis.sh. Poseidon.sol's
// build is circomlibjs's, which hardhat.config.js writes after each compile
// (PoseidonArtifacts.test.js checks it).
const { expect } = require("chai");
const { artifacts } = require("hardhat");
const fs = require("fs");
const path = require("path");

const RETAIL = path.join(__dirname, "../../../enygma_retail_payments");
const SCRIPT = path.join(RETAIL, "scripts/copy_dvp_abis.sh");
const ABIS = path.join(RETAIL, "contracts/abis");

// The files in contracts/abis/ that retail builds from its own sources, not
// copies of this project's build.
const RETAIL_BUILT = ["PrivateMintVerifier.json", "UserRegistry.json"];

// copiedArtifacts returns the ARTIFACTS entries of copy_dvp_abis.sh: each an
// artifact's path under enygma_dvp/artifacts, <sourceName>/<contract>.json.
function copiedArtifacts() {
  const block = fs.readFileSync(SCRIPT, "utf8").match(/^ARTIFACTS=\(\n([\s\S]*?)^\)/m);
  expect(block, "copy_dvp_abis.sh has no ARTIFACTS=( … ) list").to.not.equal(null);
  return block[1]
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line && !line.startsWith("#"));
}

describe("Retail's copies of enygma_dvp artifacts", function () {
  const entries = copiedArtifacts();

  it("lists at least the Verifier the retail deploy ships", function () {
    expect(entries).to.include("contracts/core/contracts/Verifier.sol/Verifier.json");
  });

  for (const entry of entries) {
    const sourceName = path.dirname(entry);
    const contract = path.basename(entry, ".json");
    const file = path.basename(entry);
    const copyPath = path.join(path.relative(path.join(RETAIL, ".."), ABIS), file);

    it(`${copyPath} is this project's build of ${sourceName}`, async function () {
      const built = await artifacts.readArtifact(`${sourceName}:${contract}`);
      const copy = JSON.parse(fs.readFileSync(path.join(ABIS, file), "utf8"));
      const fix = `: copy enygma_dvp/artifacts/${entry} over ${copyPath} (bash enygma_retail_payments/scripts/copy_dvp_abis.sh)`;
      expect(copy.sourceName, `${copyPath}'s sourceName` + fix).to.equal(sourceName);
      // === rather than equal: a failure would print both bytecodes, tens of KB.
      expect(copy.bytecode === built.bytecode, `${copyPath}'s creation bytecode differs from the build` + fix).to.equal(true);
      expect(copy.deployedBytecode === built.deployedBytecode, `${copyPath}'s runtime bytecode differs from the build` + fix).to.equal(true);
      expect(copy.abi, `${copyPath}'s ABI differs from the build` + fix).to.deep.equal(built.abi);
    });
  }

  it("finds each of retail's own builds in contracts/abis/", function () {
    const missing = RETAIL_BUILT.filter((file) => !fs.existsSync(path.join(ABIS, file)));
    expect(missing, "drop these from RetailArtifacts.test.js's RETAIL_BUILT, or rename them there").to.deep.equal([]);
  });

  it("holds no file in contracts/abis/ that copy_dvp_abis.sh does not copy, besides retail's own builds", function () {
    const copied = entries.map((entry) => path.basename(entry));
    const stray = fs
      .readdirSync(ABIS)
      .filter((file) => !copied.includes(file) && !RETAIL_BUILT.includes(file));
    expect(stray, "add these to copy_dvp_abis.sh's ARTIFACTS, or delete them if retail never loads them").to.deep.equal([]);
  });
});
