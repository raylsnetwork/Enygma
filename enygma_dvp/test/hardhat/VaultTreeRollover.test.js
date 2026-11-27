// A vault starts a new Merkle tree, numbered one up, when a transaction's
// leaves would reach its last slot, and still accepts spends against the
// roots of the trees it closed. The deployments' trees (depth 16) hold too
// many notes for the onchain suites to fill one, so this checks rollover on
// a depth-3 vault (8 slots), with AcceptAllVerifier standing in for the
// proofs: the vault's own root, nullifier and tree checks still apply.
const { expect } = require("chai");
const { ethers, artifacts } = require("hardhat");
const { POSEIDON_SOURCE } = require("../../scripts/regen_poseidon.js");
const { expectRevert } = require("./expectRevert.js");

const DEPTH = 3;
const SLOTS = 2 ** DEPTH;
const AMOUNT = 100;
const TOKEN_ID = 0;
const WITHDRAW_MESSAGE = 1; // IEnygmaDvp.sol: the StMessage of a withdrawal proof
const ZERO_PROOF = { a: { x: 0, y: 0 }, b: { x: [0, 0], y: [0, 0] }, c: { x: 0, y: 0 } };

// Deploys a Poseidon library from its abi and bytecode: the artifacts the
// compile hook writes for these libraries lack deployedLinkReferences, which
// getContractFactory(name) wants.
async function deployPoseidonLibrary(name) {
  const artifact = await artifacts.readArtifact(`${POSEIDON_SOURCE}:${name}`);
  return (await ethers.getContractFactory(artifact.abi, artifact.bytecode)).deploy();
}

// A depth-DEPTH vault over a fresh RaylsERC20, initialized by `dvp` with
// AcceptAllVerifier as its verifier, and `user` funded and approved for
// every deposit a test makes.
async function deployVault(dvp, user) {
  const poseidonT3 = await deployPoseidonLibrary("PoseidonT3");
  const poseidonT5 = await deployPoseidonLibrary("PoseidonT5");
  const wrapper = await (
    await ethers.getContractFactory("PoseidonWrapper", {
      libraries: {
        [`${POSEIDON_SOURCE}:PoseidonT3`]: poseidonT3.address,
        [`${POSEIDON_SOURCE}:PoseidonT5`]: poseidonT5.address,
      },
    })
  ).deploy();
  const verifier = await (await ethers.getContractFactory("AcceptAllVerifier")).deploy();
  const token = await (await ethers.getContractFactory("RaylsERC20")).deploy("Rayls", "RLS");
  const vault = await (await ethers.getContractFactory("Erc20CoinVault")).deploy(dvp.address);
  await vault.deployed();
  await (await vault.connect(dvp).initializeVault(0, 1, token.address, DEPTH, wrapper.address, verifier.address, dvp.address)).wait();
  const funds = 4 * SLOTS * AMOUNT;
  await (await token.mint(user.address, funds)).wait();
  await (await token.connect(user).approve(vault.address, funds)).wait();
  return { vault, wrapper, user };
}

let nextSalt = 1;

// deposit makes one note of AMOUNT for `user` and returns its commitment,
// the one leaf it inserts.
async function deposit({ vault, user }) {
  const receipt = await (await vault.connect(user).deposit([AMOUNT, 7, nextSalt++, TOKEN_ID])).wait();
  const leaves = receipt.events.filter((e) => e.event === "Commitment").map((e) => e.args.commitment);
  expect(leaves).to.have.length(1);
  return leaves[0];
}

// withdraw settles a 1-input withdrawal of half a note, its proof naming
// `tree`, `root` and `nullifier` (the Payment statement layout, see
// PaymentStatementLayout.sol), and returns the transaction. Its change note
// is the one leaf it inserts.
async function withdraw({ vault, wrapper, user }, tree, root, nullifier) {
  const amount = AMOUNT / 2;
  const out0 = await wrapper.poseidon4([user.address, 0, amount, TOKEN_ID]);
  const change = nextSalt++;
  const statement = [WITHDRAW_MESSAGE, tree, root, nullifier, out0, change, vault.address, 0];
  const receipt = { proof: ZERO_PROOF, statement, numberOfInputs: 1, numberOfOutputs: 2 };
  return vault.connect(user).withdrawV2([amount, TOKEN_ID], user.address, receipt);
}

describe("Erc20CoinVault Merkle tree rollover", function () {
  let dvp, user;

  before(async function () {
    [dvp, user] = await ethers.getSigners();
  });

  it(`fills tree 0's first ${SLOTS - 1} slots, then starts tree 1: the last slot is never used`, async function () {
    const f = await deployVault(dvp, user);
    for (let i = 0; i < SLOTS - 1; i++) {
      await deposit(f);
      expect((await f.vault.treeNumber()).toNumber(), `after deposit ${i + 1}`).to.equal(0);
    }
    const closedRoot = await f.vault.merkleRoot();

    await deposit(f);
    expect((await f.vault.treeNumber()).toNumber()).to.equal(1);
    const openRoot = await f.vault.merkleRoot();
    expect(openRoot.eq(closedRoot)).to.equal(false);
    expect(await f.vault.isValidRoot(0, closedRoot)).to.equal(true);
    expect(await f.vault.isValidRoot(1, openRoot)).to.equal(true);
    expect(await f.vault.isValidRoot(1, closedRoot)).to.equal(false);
  });

  it("spends a note of a closed tree against that tree's last root, under its own tree number", async function () {
    const f = await deployVault(dvp, user);
    for (let i = 0; i < SLOTS - 1; i++) await deposit(f);
    const closedRoot = await f.vault.merkleRoot();
    await deposit(f); // starts tree 1
    expect((await f.vault.treeNumber()).toNumber()).to.equal(1);

    const nullifier = 4242;
    // The proof must name the tree the root belongs to.
    await expectRevert(f.vault, withdraw(f, 1, closedRoot, nullifier), "InvalidMerkleRoot");

    const receipt = await (await withdraw(f, 0, closedRoot, nullifier)).wait();
    const spent = receipt.events.filter((e) => e.event === "Nullifier");
    expect(spent).to.have.length(1);
    expect(spent[0].args.treeNumber.toNumber()).to.equal(0);
    expect(spent[0].args.nullifier.toNumber()).to.equal(nullifier);
    expect(await f.vault.isValidNullifier(0, nullifier)).to.equal(true);
    // The change note lands in the open tree, not the closed one.
    expect((await f.vault.treeNumber()).toNumber()).to.equal(1);
    expect(await f.vault.isValidRoot(1, await f.vault.merkleRoot())).to.equal(true);

    // The closed tree still refuses its spent nullifier.
    await expectRevert(f.vault, withdraw(f, 0, closedRoot, nullifier), "InvalidNullifier");
  });
});
