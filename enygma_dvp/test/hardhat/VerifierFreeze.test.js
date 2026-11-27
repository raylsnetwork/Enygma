// The Verifier logs each key it takes and reports its key table:
// verificationKeyCount and verificationKeyHash, the keccak256(abi.encode(vk))
// that enygma_vkslots' manifests pin. freeze closes the table for good. The
// Payment deployment freezes its Verifier at slots 0 to 4, so slot 23, the
// DvpInitiator key Erc20CoinVault checks a DvP leg against, stays empty, and
// a vault registered against that Verifier never accepts a DvP leg, on any
// DvP entry point.
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
  let deployer, dvp, verifier, vault, usdrVault;

  before(async function () {
    [deployer] = await ethers.getSigners();
    ({ dvp, verifier } = await deployDvpAndVerifier(deployer));
    // As init_payment does: the five Payment-family keys through EnygmaDvp
    // (slots 0 to 4; the shapes do not matter here), freeze, then the two
    // vaults, vault ids 0 and 1.
    for (const [seed, nPublic] of [[1, 8], [2, 11], [3, 9], [4, 10], [5, 10]]) {
      await (await dvp.registerNewVerificationKey(testKey(seed, nPublic))).wait();
    }
    await (await verifier.freeze()).wait();
    const Erc20CoinVault = await ethers.getContractFactory("Erc20CoinVault");
    vault = await Erc20CoinVault.deploy(dvp.address);
    usdrVault = await Erc20CoinVault.deploy(dvp.address);
    await (await dvp.registerVault(vault.address, deployer.address, 1, 8)).wait();
    await (await dvp.registerVault(usdrVault.address, deployer.address, 1, 8)).wait();
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

  // Every route a DvP leg takes into these vaults checks it against slot 23:
  // anyone opens a swap with an initiator leg (submitPartialSettlement) once
  // vault 0 is in an asset group, and a registered relayer locks a leg's
  // note (lockReceiptNullifiers) or settles a pair of legs (swap, exchange)
  // on registered group pairs. The owner sets all of that up, after trying
  // to fill slots 5 to 23 (the spec above checks each refusal); each route
  // still stops at the vault's check of the leg. Runs last: it registers
  // groups and a relayer.
  it("refuses a forged DvP leg on every DvP entry point, whatever the owner sets up", async function () {
    const [, relayer, stranger] = await ethers.getSigners();
    for (let slot = 5; slot <= DVP_INITIATOR_SLOT; slot++) {
      try {
        await (await dvp.registerNewVerificationKey(testKey(100 + slot, 8))).wait();
      } catch (e) {
        // VerifierFrozen, which the spec above checks.
      }
    }
    const AssetGroup = await ethers.getContractFactory("AssetGroup");
    const fungible = await AssetGroup.deploy(dvp.address);
    const nonFungible = await AssetGroup.deploy(dvp.address);
    await (await dvp.registerAssetGroup(fungible.address, "fungible", true, 8)).wait();
    await (await dvp.registerAssetGroup(nonFungible.address, "non-fungible", false, 8)).wait();
    await (await dvp.addVaultToGroup(0, 0)).wait();
    await (await dvp.registerSwapGroupPair(0, 1)).wait();
    await (await dvp.registerExchangeGroupPair(0, 0)).wait();
    await (await dvp.registerRelayer(relayer.address)).wait();

    // An initiator leg into vault 0 that passes every check before the
    // proof, its message neither a payment's (0) nor a withdrawal's (1), and
    // a destination leg into vault 1 that pairs with it: each leg's message
    // is the other's commitment, and each names the other's vault.
    const leg = { proof: NO_PROOF, statement: [11, 0, await vault.merkleRoot(), 12, 13, 14, 15, usdrVault.address], numberOfInputs: 1, numberOfOutputs: 1 };
    const counterLeg = { proof: NO_PROOF, statement: [13, 0, await usdrVault.merkleRoot(), 16, 11, vault.address], numberOfInputs: 1, numberOfOutputs: 1 };
    const deadline = (await ethers.provider.getBlock("latest")).timestamp + 16 * 60;
    for (const [route, call] of [
      ["submitPartialSettlement", () => dvp.connect(stranger).submitPartialSettlement(leg, 0, 0, deadline)],
      ["lockReceiptNullifiers", () => dvp.connect(relayer).lockReceiptNullifiers(leg, 0)],
      ["swap", () => dvp.connect(relayer).swap(leg, counterLeg, 0, 1)],
      ["exchange", () => dvp.connect(relayer).exchange(leg, counterLeg, 0, 1)],
    ]) {
      let error;
      try {
        await (await call()).wait();
      } catch (e) {
        error = e;
      }
      expect(error && error.message, `${route} took a forged DvP leg`).to.include("Verifier: VK not registered");
    }
  });
});
