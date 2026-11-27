// The Verifier logs each key it takes and reports its key table:
// verificationKeyCount and verificationKeyHash, the keccak256(abi.encode(vk))
// that enygma_vkslots' manifests pin. freeze closes the table for good. The
// Payment deployment freezes its Verifier at slots 0 to 4, so slot 23, the
// DvpInitiator key Erc20CoinVault checks a DvP leg against, stays empty, and
// a vault registered against that Verifier never accepts a DvP leg.
const { expect } = require("chai");
const { ethers } = require("hardhat");
const { expectRevert } = require("./expectRevert.js");
const { OWNER_ROLE, deployDvpAndVerifier, testKey, keyHash } = require("./deployDvp.js");

// keccak256(abi.encode(vk)) of testKey(1, 2), as enygma_vkslots'
// TestHashIsKeccakOfTheAbiEncodedVerifyingKey pins it.
const VKSLOTS_TEST_KEY_HASH = "0xe8f844f28c8cabce64ce2eeba33af5962d212bf08860cccc3a980b63538a7b1f";
const BACKEND = "0x000000000000000000000000000000000000dEaD";
// The Erc20CoinVault slot of the DvpInitiator key (VK_ID_DVP_INITIATOR).
const DVP_INITIATOR_SLOT = 23;
const NO_PROOF = { a: { x: 0, y: 0 }, b: { x: [0, 0], y: [0, 0] }, c: { x: 0, y: 0 } };

// The events named `name` that contract emitted in receipt.
function eventsNamed(contract, receipt, name) {
  return receipt.logs
    .filter((l) => l.address === contract.address)
    .map((l) => contract.interface.parseLog(l))
    .filter((e) => e.name === name);
}

describe("Verifier key table", function () {
  let deployer, other, verifier;

  beforeEach(async function () {
    [deployer, other] = await ethers.getSigners();
    verifier = await (await ethers.getContractFactory("Verifier")).deploy();
  });

  it("addVerificationKey returns true and logs each key's slot and hash, which verificationKeyHash returns", async function () {
    expect(await verifier.callStatic.addVerificationKey(testKey(1, 2))).to.equal(true);
    for (const [slot, key] of [testKey(1, 2), testKey(2, 8)].entries()) {
      const receipt = await (await verifier.addVerificationKey(key)).wait();
      const [added] = eventsNamed(verifier, receipt, "VerificationKeyAdded");
      expect(added.args.index.toNumber()).to.equal(slot);
      expect(added.args.keyHash).to.equal(keyHash(verifier, key));
      expect(await verifier.verificationKeyHash(slot)).to.equal(keyHash(verifier, key));
      expect((await verifier.verificationKeyCount()).toNumber()).to.equal(slot + 1);
    }
    expect(await verifier.verificationKeyHash(0)).to.equal(VKSLOTS_TEST_KEY_HASH);
    expect(await verifier.verificationKeyHash(2)).to.equal(ethers.constants.HashZero);
  });

  it("initializeVerifier logs the backend, which groth16Verifier returns", async function () {
    expect(await verifier.groth16Verifier()).to.equal(ethers.constants.AddressZero);
    const receipt = await (await verifier.initializeVerifier(BACKEND)).wait();
    expect(eventsNamed(verifier, receipt, "Groth16VerifierSet").map((e) => e.args.groth16Verifier)).to.deep.equal([BACKEND]);
    expect(await verifier.groth16Verifier()).to.equal(BACKEND);
  });

  it("freeze, owner-only and once, logs the key count and ends addVerificationKey", async function () {
    await (await verifier.initializeVerifier(BACKEND)).wait();
    await (await verifier.addVerificationKey(testKey(1, 2))).wait();
    try {
      await (await verifier.connect(other).freeze()).wait();
      expect.fail("a stranger froze the Verifier");
    } catch (e) {
      expect(e.message).to.include(`AccessControl: account ${other.address.toLowerCase()} is missing role ${OWNER_ROLE}`);
    }
    expect(await verifier.frozen()).to.equal(false);

    const receipt = await (await verifier.freeze()).wait();
    expect(eventsNamed(verifier, receipt, "KeysFrozen").map((e) => e.args.keyCount.toNumber())).to.deep.equal([1]);
    expect(await verifier.frozen()).to.equal(true);

    await expectRevert(verifier, verifier.addVerificationKey(testKey(2, 2)), "VerifierFrozen");
    await expectRevert(verifier, verifier.freeze(), "VerifierFrozen");
    expect((await verifier.verificationKeyCount()).toNumber()).to.equal(1);
  });

  it("frozen before initializeVerifier, keeps no backend", async function () {
    await (await verifier.freeze()).wait();
    await expectRevert(verifier, verifier.initializeVerifier(BACKEND), "VerifierFrozen");
    expect(await verifier.groth16Verifier()).to.equal(ethers.constants.AddressZero);
  });
});

describe("Payment deployment's frozen Verifier", function () {
  let deployer, dvp, verifier, vault;

  before(async function () {
    [deployer] = await ethers.getSigners();
    ({ dvp, verifier } = await deployDvpAndVerifier(deployer));
    // As init_payment does: the five Payment-family keys through EnygmaDvp
    // (slots 0 to 4; the shapes do not matter here), freeze, then the vault.
    for (const [seed, nPublic] of [[1, 8], [2, 11], [3, 9], [4, 10], [5, 10]]) {
      await (await dvp.registerNewVerificationKey(testKey(seed, nPublic))).wait();
    }
    await (await verifier.freeze()).wait();
    vault = await (await ethers.getContractFactory("Erc20CoinVault")).deploy(dvp.address);
    await (await dvp.registerVault(vault.address, deployer.address, 1, 8)).wait();
  });

  it("refuses a sixth key, through EnygmaDvp or directly", async function () {
    expect((await verifier.verificationKeyCount()).toNumber()).to.equal(5);
    await expectRevert(verifier, dvp.registerNewVerificationKey(testKey(6, 8)), "VerifierFrozen");
    await expectRevert(verifier, verifier.addVerificationKey(testKey(6, 8)), "VerifierFrozen");
    expect((await verifier.verificationKeyCount()).toNumber()).to.equal(5);
  });

  it("keeps slot 23 empty, so the vault refuses every DvP leg", async function () {
    expect((await vault.VK_ID_DVP_INITIATOR()).toNumber()).to.equal(DVP_INITIATOR_SLOT);
    expect(await vault.getVerifierContractAddress()).to.equal(verifier.address);
    for (let slot = 5; slot <= DVP_INITIATOR_SLOT; slot++) {
      await expectRevert(verifier, dvp.registerNewVerificationKey(testKey(slot + 1, 8)), "VerifierFrozen");
    }
    // A DvpInitiator leg that passes every check before the proof: the
    // vault's live root, an unspent nullifier, CommitB distinct from CommitA.
    const root = await vault.merkleRoot();
    const leg = { proof: NO_PROOF, statement: [1, 0, root, 2, 3, 4, 5, 0], numberOfInputs: 1, numberOfOutputs: 1 };
    for (const call of [
      () => vault.checkDvpReceiptConditions(leg),
      () => verifier.verifyProof(DVP_INITIATOR_SLOT, NO_PROOF, leg.statement),
    ]) {
      let error;
      try {
        await call();
      } catch (e) {
        error = e;
      }
      expect(error && error.message, "a DvP leg reached a key at slot 23").to.include("Verifier: VK not registered");
    }
  });
});
