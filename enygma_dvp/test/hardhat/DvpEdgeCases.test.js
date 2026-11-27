// Four EnygmaDvp edge cases from the reconciled audit (#477): R-110, R-111,
// R-112 and R-107.
const { expect } = require("chai");
const { ethers } = require("hardhat");
const { deployPoseidonWrapper } = require("./poseidonWrapper.js");

const TREE_DEPTH = 8;
const NFT_ID = 7;
const ZERO_PROOF = { a: { x: 0, y: 0 }, b: { x: [0, 0], y: [0, 0] }, c: { x: 0, y: 0 } };

function receipt(statement, numberOfInputs = 0, numberOfOutputs = 0) {
  return { proof: ZERO_PROOF, statement, numberOfInputs, numberOfOutputs };
}

// An EnygmaDvp owned by `owner`, over AcceptAllVerifier, with an
// Erc721CoinVault as vault 0 and a non-fungible AssetGroup as group 0 whose
// tree is `groupDepth` deep.
async function deployDvp(owner, groupDepth = TREE_DEPTH) {
  const wrapper = await deployPoseidonWrapper();
  const verifier = await (await ethers.getContractFactory("AcceptAllVerifier")).deploy();
  const dvp = await (await ethers.getContractFactory("EnygmaDvp")).deploy(wrapper.address, verifier.address);
  await (await dvp.initializeDvp(verifier.address)).wait();

  const vault = await (await ethers.getContractFactory("Erc721CoinVault")).deploy(dvp.address);
  await (await dvp.registerVault(vault.address, owner.address, 1, TREE_DEPTH)).wait();

  const group = await (await ethers.getContractFactory("AssetGroup")).deploy(dvp.address);
  await (await dvp.registerAssetGroup(group.address, "NFTs", false, groupDepth)).wait();

  return { dvp, vault, group };
}

describe("EnygmaDvp edge cases (#477)", function () {
  let owner;

  before(async function () {
    [owner] = await ethers.getSigners();
  });

  // R-110: the ERC721 vault reads the token id from params[0].
  it("finds an ERC721 token with a non-zero id in the group it was added to", async function () {
    const { dvp } = await deployDvp(owner);
    await (await dvp.addTokenToGroup(0, [NFT_ID], 0)).wait();

    expect(await dvp.isTokenMemberOf(0, [NFT_ID], 0)).to.equal(true);
    expect(await dvp.isTokenMemberOf(0, [NFT_ID + 1], 0)).to.equal(false);
  });

  // R-111: a depth-2 group tree holds 3 members, so the 4th opens tree 1.
  it("accepts a proof against a group root in the group's second tree", async function () {
    const { dvp, group } = await deployDvp(owner, 2);
    for (let id = 1; id <= 3; id++) {
      await (await dvp.addTokenToGroup(0, [id], 0)).wait();
    }
    const tree0Root = await group.merkleRoot();
    await (await dvp.addTokenToGroup(0, [4], 0)).wait();
    expect((await group.treeNumber()).toNumber()).to.equal(1);

    // Vault 0 was never added to the group, so only the root can admit it.
    const root = await group.merkleRoot();
    expect(await group.isMemberFromProofReceipt(0, receipt([root]))).to.equal(true);
    expect(await group.isMemberFromProofReceipt(0, receipt([tree0Root]))).to.equal(true);
    expect(await group.isMemberFromProofReceipt(0, receipt([root.add(1)]))).to.equal(false);
  });

  // R-112: the vault skips zero commitments, which can leave nothing to insert.
  // A standalone Erc721CoinVault whose DvP role is the owner, so its
  // DvP-only inserts can be called directly.
  async function deployStandaloneVault() {
    const wrapper = await deployPoseidonWrapper();
    const vault = await (await ethers.getContractFactory("Erc721CoinVault")).deploy(owner.address);
    await (await vault.initializeVault(0, 1, owner.address, TREE_DEPTH, wrapper.address, ethers.constants.AddressZero, owner.address)).wait();
    // A real leaf first, so the root is not the empty tree's (which a
    // rollover into a new tree would also leave).
    await (await vault.registerCoins([42])).wait();
    return vault;
  }

  async function expectTreeUnchanged(vault, insert) {
    const rootBefore = (await vault.merkleRoot()).toString();
    const treeBefore = (await vault.treeNumber()).toNumber();
    await (await insert()).wait();
    expect((await vault.merkleRoot()).toString()).to.equal(rootBefore);
    expect((await vault.treeNumber()).toNumber()).to.equal(treeBefore);
  }

  it("inserts nothing for a receipt whose commitments are all zero", async function () {
    const vault = await deployStandaloneVault();
    const root = await vault.merkleRoot();
    // Legacy joinsplit layout, one input and two outputs:
    // [message, treeNumber, merkleRoot, nullifier, commitment0, commitment1].
    await expectTreeUnchanged(vault, () => vault.insertCommitmentsFromReceipt(receipt([1, 0, root, 5, 0, 0], 1, 2)));
  });

  it("inserts nothing for registerCoins with only zero commitments", async function () {
    const vault = await deployStandaloneVault();
    await expectTreeUnchanged(vault, () => vault.registerCoins([0, 0]));
  });

  // R-107: an initiator leg opens a swap, parks its receipt and locks its note.
  it("reports an opened swap as true and reads its parked leg without a transaction", async function () {
    const { dvp, group } = await deployDvp(owner);
    const erc20Vault = await (await ethers.getContractFactory("Erc20CoinVault")).deploy(dvp.address);
    await (await dvp.registerVault(erc20Vault.address, owner.address, 1, TREE_DEPTH)).wait();
    await (await dvp.addVaultToGroup(1, 0)).wait();

    const [treeNumber, nullifier, commitB, commitA, revertCommitA] = [0, 5, 11, 12, 13];
    const deadline = (await ethers.provider.getBlock("latest")).timestamp + 24 * 60 * 60;
    // DvpInitiator layout: [message, treeNumber, merkleRoot, nullifier,
    // commitB, commitA, revertCommitA, counterVault, deadline, groupId].
    const leg = receipt([commitA, treeNumber, await erc20Vault.merkleRoot(), nullifier, commitB, commitA, revertCommitA, 0, deadline, 0]);

    expect(await dvp.callStatic.submitPartialSettlement(leg, 1, 0, deadline)).to.equal(true);
    await (await dvp.submitPartialSettlement(leg, 1, 0, deadline)).wait();

    const parked = await erc20Vault.getPendingProofReceipt(commitB);
    expect(parked.statement.map(String)).to.deep.equal(leg.statement.map(String));
    expect(await erc20Vault.isLocked(treeNumber, nullifier)).to.equal(true);
  });
});
