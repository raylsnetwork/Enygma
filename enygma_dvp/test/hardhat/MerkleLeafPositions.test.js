// Merkle.insertLeaves logs where each leaf goes (LeafInserted: the tree and
// the index), decides a new tree once per call, not per transaction, and
// refuses an empty batch or one no fresh tree takes. getNextLeafIndex reads
// the current tree's next index. Wallets rebuild a vault's trees from these
// positions, since a transaction can make several inserts.
const { expect } = require("chai");
const { ethers, artifacts } = require("hardhat");
const { POSEIDON_SOURCE } = require("../../scripts/regen_poseidon.js");
const { expectRevert } = require("./helpers.js");

// A depth-3 tree has 8 slots; a batch that would fill the last one, index 7,
// goes into the next tree.
const DEPTH = 3;
const TREE_ID = 9;

// Deploys a Poseidon library from its abi and bytecode: the artifacts the
// compile hook writes for these libraries lack deployedLinkReferences, which
// getContractFactory(name) wants.
async function deployPoseidonLibrary(name) {
  const artifact = await artifacts.readArtifact(`${POSEIDON_SOURCE}:${name}`);
  return (await ethers.getContractFactory(artifact.abi, artifact.bytecode)).deploy();
}

async function deployPoseidonWrapper() {
  const poseidonT3 = await deployPoseidonLibrary("PoseidonT3");
  const poseidonT5 = await deployPoseidonLibrary("PoseidonT5");
  return (
    await ethers.getContractFactory("PoseidonWrapper", {
      libraries: {
        [`${POSEIDON_SOURCE}:PoseidonT3`]: poseidonT3.address,
        [`${POSEIDON_SOURCE}:PoseidonT5`]: poseidonT5.address,
      },
    })
  ).deploy();
}

// leaves is n distinct leaves from first on.
const leaves = (first, n) => Array.from({ length: n }, (_, i) => first + i);

// positions is each LeafInserted event in receipt, in log order, as
// [vaultId, commitment, treeNumber, leafIndex].
function positions(contract, receipt) {
  return receipt.logs
    .map((l) => { try { return contract.interface.parseLog(l); } catch (_) { return null; } })
    .filter((e) => e && e.name === "LeafInserted")
    .map((e) => [e.args.vaultId, e.args.commitment, e.args.treeNumber, e.args.leafIndex].map((v) => v.toNumber()));
}

describe("Merkle leaf positions", function () {
  let wrapper;

  before(async function () {
    wrapper = await deployPoseidonWrapper();
  });

  // A fresh depth-3 tree with the given leaves already in it, one call each batch.
  async function treeWith(...batches) {
    const tree = await (await ethers.getContractFactory("MerkleInsertHarness")).deploy(DEPTH, TREE_ID, wrapper.address);
    for (const b of batches) await (await tree.insert(b)).wait();
    return tree;
  }

  it("logs each leaf's tree and index as inserted, and getNextLeafIndex is the next index", async function () {
    const tree = await treeWith();
    expect((await tree.getNextLeafIndex()).toNumber()).to.equal(0);

    const receipt = await (await tree.insert([11, 12, 13])).wait();
    expect(positions(tree, receipt)).to.deep.equal([[TREE_ID, 11, 0, 0], [TREE_ID, 12, 0, 1], [TREE_ID, 13, 0, 2]]);
    expect((await tree.getNextLeafIndex()).toNumber()).to.equal(3);

    // A batch starting at an odd index.
    const next = await (await tree.insert([14, 15])).wait();
    expect(positions(tree, next)).to.deep.equal([[TREE_ID, 14, 0, 3], [TREE_ID, 15, 0, 4]]);
    expect((await tree.getNextLeafIndex()).toNumber()).to.equal(5);
  });

  it("puts a batch that would fill the last slot into the next tree, and says so", async function () {
    const tree = await treeWith(leaves(1, 6));
    const receipt = await (await tree.insert([21, 22])).wait();
    expect(positions(tree, receipt)).to.deep.equal([[TREE_ID, 21, 1, 0], [TREE_ID, 22, 1, 1]]);
    expect((await tree.treeNumber()).toNumber()).to.equal(1);
    expect((await tree.getNextLeafIndex()).toNumber()).to.equal(2);
  });

  it("decides the new tree per call: two calls in one transaction land in two trees", async function () {
    const tree = await treeWith(leaves(1, 6));
    const receipt = await (await tree.insertTwice([31], [32, 33])).wait();
    // 6 + 1 leaves fit tree 0; 7 + 2 would fill its last slot.
    expect(positions(tree, receipt)).to.deep.equal([[TREE_ID, 31, 0, 6], [TREE_ID, 32, 1, 0], [TREE_ID, 33, 1, 1]]);
    expect((await tree.treeNumber()).toNumber()).to.equal(1);
    expect((await tree.getNextLeafIndex()).toNumber()).to.equal(2);
  });

  it("refuses an empty batch with EmptyLeafBatch", async function () {
    for (const tree of [await treeWith(), await treeWith([1])]) {
      await expectRevert(tree, tree.insert([]), "EmptyLeafBatch");
    }
  });

  it("refuses a batch of 2^depth leaves with LeafBatchTooLarge, and takes 2^depth - 1 into a fresh tree", async function () {
    const tree = await treeWith([1]);
    await expectRevert(tree, tree.insert(leaves(100, 8)), "LeafBatchTooLarge");
    expect((await tree.treeNumber()).toNumber()).to.equal(0);
    expect((await tree.getNextLeafIndex()).toNumber()).to.equal(1);

    const receipt = await (await tree.insert(leaves(100, 7))).wait();
    expect(positions(tree, receipt)).to.deep.equal(leaves(0, 7).map((i) => [TREE_ID, 100 + i, 1, i]));
    expect((await tree.getNextLeafIndex()).toNumber()).to.equal(7);
  });

  it("a vault logs LeafInserted with its vault id beside each Commitment, and refuses an all-zero registerCoins", async function () {
    const [dvp] = await ethers.getSigners();
    const vault = await (await ethers.getContractFactory("Erc20CoinVault")).deploy(dvp.address);
    await (await vault.initializeVault(4, 1, dvp.address, 8, wrapper.address, ethers.constants.AddressZero, dvp.address)).wait();

    const receipt = await (await vault.registerCoins([41, 0, 42])).wait();
    expect(positions(vault, receipt)).to.deep.equal([[4, 41, 0, 0], [4, 42, 0, 1]]);
    const commitments = receipt.events.filter((e) => e.event === "Commitment").map((e) => e.args.commitment.toNumber());
    expect(commitments).to.deep.equal([41, 42]);
    expect((await vault.getNextLeafIndex()).toNumber()).to.equal(2);

    await expectRevert(vault, vault.registerCoins([0, 0]), "EmptyLeafBatch");
  });
});
