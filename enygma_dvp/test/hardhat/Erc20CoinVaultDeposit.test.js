// Erc20CoinVault.deposit and depositV2 commit to the caller's amount, so they
// refuse a transfer that leaves the vault holding anything else (#432): a
// fee-on-transfer token would otherwise credit more than the vault holds.
const { expect } = require("chai");
const { ethers } = require("hardhat");
const { buildPoseidon } = require("circomlibjs");
const { expectRevert } = require("./helpers.js");
const { deployPoseidonWrapper } = require("./poseidonWrapper.js");

const TREE_DEPTH = 8;
const AMOUNT = 1000;
const PK_SPEND = 11;
const SALT = 22;
const TOKEN_ID = 0;
const PARAMS = [AMOUNT, PK_SPEND, SALT, TOKEN_ID];
const CAPSULE = "0x" + "ab".repeat(1088);
const NOTE = "0x" + "cd".repeat(48);

// A vault over `token`, initialized by `dvp` (who holds its DVP role), with
// `depositor` funded and approved for AMOUNT.
async function deployVault(token, dvp, depositor) {
  const wrapper = await deployPoseidonWrapper();
  const vault = await (await ethers.getContractFactory("Erc20CoinVault")).deploy(dvp.address);
  await vault.deployed();
  await (await vault.connect(dvp).initializeVault(0, 1, token.address, TREE_DEPTH, wrapper.address, ethers.constants.AddressZero, dvp.address)).wait();
  await (await token.mint(depositor.address, AMOUNT)).wait();
  await (await token.connect(depositor).approve(vault.address, AMOUNT)).wait();
  return vault;
}

const deposits = {
  deposit: (vault) => vault.deposit(PARAMS),
  depositV2: (vault) => vault.depositV2(PARAMS, CAPSULE, NOTE),
};

// Test tokens that deliver the vault something other than the amount sent.
const skewedTokens = {
  "a fee-on-transfer token (delivers 1% less)": "FeeOnTransferERC20",
  "a bonus-on-transfer token (delivers 1% more)": "BonusOnTransferERC20",
};

describe("Erc20CoinVault deposits", function () {
  let dvp, depositor, expectedCommitment;

  before(async function () {
    [dvp, depositor] = await ethers.getSigners();
    const poseidon = await buildPoseidon();
    expectedCommitment = poseidon.F.toObject(poseidon([PK_SPEND, SALT, AMOUNT, TOKEN_ID])).toString();
  });

  for (const [name, call] of Object.entries(deposits)) {
    describe(name, function () {
      for (const [label, contract] of Object.entries(skewedTokens)) {
        it(`refuses ${label} and inserts no leaf`, async function () {
          const token = await (await ethers.getContractFactory(contract)).deploy();
          const vault = await deployVault(token, dvp, depositor);
          const rootBefore = await vault.merkleRoot();

          await expectRevert(vault, call(vault.connect(depositor)), "DepositAmountMismatch");

          expect((await vault.merkleRoot()).toString()).to.equal(rootBefore.toString());
          expect((await token.balanceOf(vault.address)).toNumber()).to.equal(0);
        });
      }

      it("inserts Poseidon4(pkSpend, salt, amount, tokenId) for a RaylsERC20 deposit", async function () {
        const token = await (await ethers.getContractFactory("RaylsERC20")).deploy("Rayls", "RLS");
        const vault = await deployVault(token, dvp, depositor);
        const rootBefore = await vault.merkleRoot();

        const receipt = await (await call(vault.connect(depositor))).wait();
        const commitments = receipt.events.filter((e) => e.event === "Commitment").map((e) => e.args.commitment.toString());
        expect(commitments).to.deep.equal([expectedCommitment]);
        expect((await vault.merkleRoot()).toString()).to.not.equal(rootBefore.toString());
        expect((await token.balanceOf(vault.address)).toNumber()).to.equal(AMOUNT);
      });
    });
  }
});
