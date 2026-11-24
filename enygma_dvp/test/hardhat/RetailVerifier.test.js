// The Verifier the retail deploy ships (enygma_retail_payments/contracts/abis/
// Verifier.json) is this project's build of Verifier.sol, so only its deployer
// may initialize it, and only once (#415).
const { expect } = require("chai");
const { ethers, artifacts } = require("hardhat");
const path = require("path");

const RETAIL_VERIFIER = path.join(__dirname, "../../../enygma_retail_payments/contracts/abis/Verifier.json");

// The Groth16 backend these tests install: initializeVerifier stores any
// address, and no proof is verified here.
const BACKEND = "0x000000000000000000000000000000000000dEaD";

// Resolves when tx reverts; fails if it succeeds.
async function expectRevert(tx, what) {
  try {
    await (await tx).wait();
  } catch (e) {
    return e;
  }
  expect.fail(`expected ${what} to revert`);
}

describe("Retail Verifier.json", function () {
  let verifier, deployer, other;

  beforeEach(async function () {
    [deployer, other] = await ethers.getSigners();
    const artifact = require(RETAIL_VERIFIER);
    verifier = await new ethers.ContractFactory(artifact.abi, artifact.bytecode, deployer).deploy();
    await verifier.deployed();
  });

  it("refuses initializeVerifier from an account other than its deployer", async function () {
    await expectRevert(verifier.connect(other).initializeVerifier(BACKEND), "a stranger's initializeVerifier");
    const ownerRole = await verifier.DEFAULT_OWNER_ROLE();
    expect(await verifier.hasRole(ownerRole, other.address)).to.equal(false);
  });

  it("lets its deployer initialize it once, then refuses a second initializeVerifier", async function () {
    await (await verifier.initializeVerifier(BACKEND)).wait();
    const e = await expectRevert(verifier.initializeVerifier(other.address), "a second initializeVerifier");
    expect(e.message).to.include("Verifier: already initialized");
  });

  it("is this project's build of contracts/core/contracts/Verifier.sol", async function () {
    const built = await artifacts.readArtifact("contracts/core/contracts/Verifier.sol:Verifier");
    const copy = require(RETAIL_VERIFIER);
    const fix = "; copy enygma_dvp/artifacts/contracts/core/contracts/Verifier.sol/Verifier.json over it";
    expect(copy.bytecode === built.bytecode, "the retail copy's creation bytecode differs from the build of Verifier.sol" + fix).to.equal(true);
    expect(copy.deployedBytecode === built.deployedBytecode, "the retail copy's runtime bytecode differs from the build of Verifier.sol" + fix).to.equal(true);
    expect(copy.abi).to.deep.equal(built.abi);
  });
});
