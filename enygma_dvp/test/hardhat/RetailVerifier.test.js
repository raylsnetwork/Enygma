// The Verifier the retail deploy ships (enygma_retail_payments/contracts/abis/
// Verifier.json) is this project's build of Verifier.sol (RetailArtifacts.test.js
// checks that), so only its deployer may initialize it, and only once (#415).
const { expect } = require("chai");
const { ethers } = require("hardhat");
const path = require("path");
const { expectRevert } = require("./helpers.js");

const RETAIL_VERIFIER = path.join(__dirname, "../../../enygma_retail_payments/contracts/abis/Verifier.json");

// The Groth16 backend these tests install: initializeVerifier stores any
// address, and no proof is verified here.
const BACKEND = "0x000000000000000000000000000000000000dEaD";

describe("Retail Verifier.json", function () {
  let verifier, deployer, other;

  beforeEach(async function () {
    [deployer, other] = await ethers.getSigners();
    const artifact = require(RETAIL_VERIFIER);
    verifier = await new ethers.ContractFactory(artifact.abi, artifact.bytecode, deployer).deploy();
    await verifier.deployed();
  });

  it("refuses initializeVerifier from an account other than its deployer", async function () {
    const ownerRole = await verifier.DEFAULT_OWNER_ROLE();
    const reason = `AccessControl: account ${other.address.toLowerCase()} is missing role ${ownerRole}`;
    await expectRevert(verifier, verifier.connect(other).initializeVerifier(BACKEND), reason);
    expect(await verifier.hasRole(ownerRole, other.address)).to.equal(false);
  });

  it("lets its deployer initialize it once, then refuses a second initializeVerifier", async function () {
    await (await verifier.initializeVerifier(BACKEND)).wait();
    await expectRevert(verifier, verifier.initializeVerifier(other.address), "Verifier: already initialized");
  });
});
