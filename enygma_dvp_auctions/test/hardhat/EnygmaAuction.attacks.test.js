// Attack scenarios against EnygmaAuction. Each test states a property the
// contract should keep; a failure message means the property does not hold.
// Each block is a regression test for one audit finding.
const { expect } = require("chai");
const { ethers } = require("hardhat");
const { time } = require("@nomicfoundation/hardhat-network-helpers");

const ZERO_PROOF = Array(8).fill(0);
const DAY = 24 * 3600;

const paramsHash = (auctionId, deadline, settlementDeadline, floorPrice) =>
  ethers.keccak256(ethers.AbiCoder.defaultAbiCoder().encode(
    ["uint256", "uint256", "uint256", "uint256"], [auctionId, deadline, settlementDeadline, floorPrice]));

const lockStatement = ({ auctionId, nullifier, commitLocked, nftTokenId = 77 }) =>
  [auctionId, 0, 1, nullifier, commitLocked, nftTokenId, 0];
const bidStatement = ({ auctionId, nullifier, commitA, commitB }) =>
  [auctionId, 0, 1, nullifier, commitA, commitB, 0];
const batchStatement = ({ auctionId, commits, winnerCommit, winnerPk, winnerAmount }) => {
  const c = Array(100).fill(0);
  commits.forEach((v, i) => { c[i] = v; });
  return [auctionId, ...c, winnerCommit, winnerPk, winnerAmount];
};

async function deploy() {
  const verifier = await (await ethers.getContractFactory("MockVerifier")).deploy();
  const Vault = await ethers.getContractFactory("MockCoinVault");
  const erc20Vault = await Vault.deploy();
  const erc721Vault = await Vault.deploy();
  const auction = await (await ethers.getContractFactory("EnygmaAuction")).deploy(
    await verifier.getAddress(), await erc20Vault.getAddress(), await erc721Vault.getAddress(), 1, 2, 3, 4, 5, 6);
  return { verifier, erc20Vault, erc721Vault, auction };
}

async function openAuction(auction, { auctionId = 1, deadlineDelta = 3600, floorPrice = 10 } = {}) {
  const deadline = (await time.latest()) + deadlineDelta;
  await auction.announceAuction(paramsHash(auctionId, deadline, deadline + 2 * DAY + 3600, floorPrice));
  await auction.initAuction(ZERO_PROOF,
    lockStatement({ auctionId, nullifier: 1000 + auctionId, commitLocked: 200 }),
    deadline, deadline + 2 * DAY + 3600, floorPrice);
  return deadline;
}

describe("EnygmaAuction — attack scenarios", function () {
  // ── A. initAuction's timing/floor parameters must be the seller's ─────────
  describe("A: a front-runner cannot choose the parameters of Bob's auction", function () {
    const stmt = lockStatement({ auctionId: 1, nullifier: 1001, commitLocked: 200 });

    it("A1: the attacker's own parameters are rejected unless announced", async function () {
      const [, , attacker] = await ethers.getSigners();
      const { auction } = await deploy();
      const now = await time.latest();
      const atkDeadline = now + 30;
      await expect(
        auction.connect(attacker).initAuction(ZERO_PROOF, stmt, atkDeadline, atkDeadline + 10 * 365 * DAY, 0)
      ).to.be.revertedWithCustomError(auction, "InvalidSettlementDeadline"); // capped
      await expect(
        auction.connect(attacker).initAuction(ZERO_PROOF, stmt, atkDeadline, atkDeadline + 3 * DAY, 0)
      ).to.be.revertedWithCustomError(auction, "ParamsNotAnnounced");
    });

    it("A2: Bob's announced parameters go through; replaying them is harmless", async function () {
      const [, bob, attacker] = await ethers.getSigners();
      const { auction } = await deploy();
      const deadline = (await time.latest()) + 3600, settlement = deadline + 3 * DAY, floor = 10;
      await auction.connect(bob).announceAuction(paramsHash(1, deadline, settlement, floor));
      // The attacker sees Bob's init in the mempool and submits the same proof with Bob's own values.
      await auction.connect(attacker).initAuction(ZERO_PROOF, stmt, deadline, settlement, floor);
      const core = await auction.getAuctionCore(1);
      expect(core.floorPrice).to.equal(BigInt(floor));
      expect(core.deadline).to.equal(BigInt(deadline));
      expect(core.settlementDeadline).to.equal(BigInt(settlement));
    });

    it("A3: an announcement cannot be reused for a second auction", async function () {
      const { auction } = await deploy();
      const deadline = (await time.latest()) + 3600, settlement = deadline + 3 * DAY;
      await auction.announceAuction(paramsHash(1, deadline, settlement, 10));
      await auction.initAuction(ZERO_PROOF, stmt, deadline, settlement, 10);
      await expect(auction.initAuction(ZERO_PROOF, stmt, deadline, settlement, 10))
        .to.be.revertedWithCustomError(auction, "AuctionAlreadyExists");
    });

    it("A4: settlementDeadline is capped at deadline + 30 days", async function () {
      const { auction } = await deploy();
      const deadline = (await time.latest()) + 3600;
      const tooFar = deadline + 30 * DAY + 1;
      await auction.announceAuction(paramsHash(1, deadline, tooFar, 10));
      await expect(auction.initAuction(ZERO_PROOF, stmt, deadline, tooFar, 10))
        .to.be.revertedWithCustomError(auction, "InvalidSettlementDeadline");
      const ok = deadline + 30 * DAY;
      await auction.announceAuction(paramsHash(1, deadline, ok, 10));
      await auction.initAuction(ZERO_PROOF, stmt, deadline, ok, 10);
    });
  });

  describe("A5: Bob can cancel until the first batch", function () {
    it("cancels after the deadline while no batch exists", async function () {
      const { auction } = await deploy();
      await openAuction(auction, { deadlineDelta: 10 });
      await time.increase(20);
      await auction.revertAuction(ZERO_PROOF, [1, 200, 77, 999]);
      expect(Number((await auction.getAuctionCore(1)).state)).to.equal(4); // CANCELED
    });

    it("cannot cancel once a batch has been submitted", async function () {
      const { auction } = await deploy();
      await openAuction(auction, { deadlineDelta: 10 });
      await auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 500, commitA: 600, commitB: 700 }), "0x", "0x");
      await time.increase(20);
      await auction.submitBatch(1, ZERO_PROOF, batchStatement({ auctionId: 1, commits: [600], winnerCommit: 600, winnerPk: 900, winnerAmount: 1000 }));
      await expect(auction.revertAuction(ZERO_PROOF, [1, 200, 77, 999]))
        .to.be.revertedWithCustomError(auction, "BatchesAlreadySubmitted");
    });
  });
});
