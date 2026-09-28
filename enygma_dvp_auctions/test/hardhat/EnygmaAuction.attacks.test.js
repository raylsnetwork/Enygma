// Attack scenarios against EnygmaAuction. Each test states a property the
// contract should keep; a failure message starting with "VULNERABLE:" means the
// property does not hold. Each block is a regression test for one audit finding. Uses the same mocks as EnygmaAuction.test.js.
const { expect } = require("chai");
const { ethers, network } = require("hardhat");
const { time, takeSnapshot } = require("@nomicfoundation/hardhat-network-helpers");

const BIDDING = 1;
const PENDING_SETTLEMENT = 2;
const SETTLED = 3;
const ZERO_PROOF = Array(8).fill(0);
const DAY = 24 * 3600;

const paramsHash = (auctionId, deadline, settlementDeadline, floorPrice) =>
  ethers.keccak256(ethers.AbiCoder.defaultAbiCoder().encode(
    ["uint256", "uint256", "uint256", "uint256"], [auctionId, deadline, settlementDeadline, floorPrice]));

const lockStatement = ({ auctionId, nullifier, commitLocked, nftTokenId = 77 }) =>
  [auctionId, 0, 1, nullifier, commitLocked, nftTokenId, 0];
const FR = 21888242871839275222246405745257275088548364400416034343698204186575808495617n;
// keccak256(abi.encodePacked(keccak256(ctxt1), keccak256(ctxt2))) mod Fr — the bid's StCtxtHash.
const ctxtHash = (c1 = "0x", c2 = "0x") =>
  BigInt(ethers.keccak256(ethers.concat([ethers.keccak256(c1), ethers.keccak256(c2)]))) % FR;
const bidStatement = ({ auctionId, nullifier, commitA, commitB, ctxt1 = "0x", ctxt2 = "0x" }) =>
  [auctionId, 0, 1, nullifier, commitA, commitB, 0, ctxtHash(ctxt1, ctxt2)];
const batchStatement = ({ auctionId, commits, winnerCommit, winnerPk, winnerAmount }) => {
  const c = Array(100).fill(0);
  commits.forEach((v, i) => { c[i] = v; });
  return [auctionId, ...c, winnerCommit, winnerPk, winnerAmount];
};
const finalStatement = ({ auctionId, batches, winner, winnerPk, winnerCommitB, winnerNftCommit = 5678, nftTokenId = 77, winningAmount, floorPrice = 10 }) => {
  const commits = Array(10).fill(0), pks = Array(10).fill(0), amts = Array(10).fill(0);
  batches.forEach((b, i) => { commits[i] = b.commit; pks[i] = b.pk; amts[i] = b.amount; });
  return [auctionId, ...commits, ...pks, ...amts, winner, winnerPk, winnerCommitB, winnerNftCommit, nftTokenId, winningAmount, floorPrice];
};
const batchIdsArr = (ids) => { const a = Array(10).fill(0); ids.forEach((v, i) => { a[i] = v; }); return a; };

async function deploy(verifierFactoryName = "MockVerifier", verifierArgs = [], vkIds = [1, 2, 3, 4, 5, 6]) {
  const verifier = await (await ethers.getContractFactory(verifierFactoryName)).deploy(...verifierArgs);
  const Vault = await ethers.getContractFactory("MockCoinVault");
  const erc20Vault = await Vault.deploy();
  const erc721Vault = await Vault.deploy();
  const auction = await (await ethers.getContractFactory("EnygmaAuction")).deploy(
    await verifier.getAddress(), await erc20Vault.getAddress(), await erc721Vault.getAddress(), ...vkIds);
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

    it("A3b: an announcement made in the same block as initAuction does not count", async function () {
      const [, , attacker] = await ethers.getSigners();
      const { auction } = await deploy();
      // The attacker sees Bob's initAuction in the mempool, so it knows the auctionId
      // (statement[0]). It announces parameters of its own and calls initAuction with
      // them in the same block.
      const deadline = (await time.latest()) + 60, settlement = deadline + 3 * DAY;
      await network.provider.send("evm_setAutomine", [false]);
      try {
        const announceTx = await auction.connect(attacker).announceAuction(paramsHash(1, deadline, settlement, 0), { gasLimit: 200000 });
        const initTx = await auction.connect(attacker).initAuction(ZERO_PROOF, stmt, deadline, settlement, 0, { gasLimit: 2000000 });
        await network.provider.send("evm_mine");
        expect((await ethers.provider.getTransactionReceipt(announceTx.hash)).status).to.equal(1);
        expect((await ethers.provider.getTransactionReceipt(initTx.hash)).status).to.equal(0);
      } finally {
        await network.provider.send("evm_setAutomine", [true]);
      }
      expect(Number((await auction.getAuctionCore(1)).state)).to.equal(0); // INACTIVE: no auction was opened
    });

    it("A3c: Bob's older announcement still wins against a same-block attacker", async function () {
      const [, bob, attacker] = await ethers.getSigners();
      const { auction } = await deploy();
      const deadline = (await time.latest()) + 3600, settlement = deadline + 3 * DAY, floor = 10;
      await auction.connect(bob).announceAuction(paramsHash(1, deadline, settlement, floor)); // mined earlier
      await network.provider.send("evm_setAutomine", [false]);
      try {
        // Same block, attacker first: announce its own parameters and try to init with them...
        const atkAnnounce = await auction.connect(attacker).announceAuction(paramsHash(1, deadline, settlement, 0), { gasLimit: 200000 });
        const atkInit = await auction.connect(attacker).initAuction(ZERO_PROOF, stmt, deadline, settlement, 0, { gasLimit: 2000000 });
        // ...then Bob's own init, which relies on the older announcement.
        const bobInit = await auction.connect(bob).initAuction(ZERO_PROOF, stmt, deadline, settlement, floor, { gasLimit: 2000000 });
        await network.provider.send("evm_mine");
        expect((await ethers.provider.getTransactionReceipt(atkAnnounce.hash)).status).to.equal(1);
        expect((await ethers.provider.getTransactionReceipt(atkInit.hash)).status).to.equal(0);
        expect((await ethers.provider.getTransactionReceipt(bobInit.hash)).status).to.equal(1);
      } finally {
        await network.provider.send("evm_setAutomine", [true]);
      }
      expect((await auction.getAuctionCore(1)).floorPrice).to.equal(BigInt(floor));
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

  // ── B. A claim that cannot be applied must not lock the auction ───────────
  describe("B: stuck settlement claims", function () {
    async function upToBatch(auction) {
      await openAuction(auction, { deadlineDelta: 10 });
      await auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 500, commitA: 600, commitB: 700 }), "0x", "0x");
      await time.increase(20);
      await auction.submitBatch(1, ZERO_PROOF, batchStatement({ auctionId: 1, commits: [600], winnerCommit: 600, winnerPk: 900, winnerAmount: 1000 }));
    }
    const claim = (over = {}) => finalStatement({ auctionId: 1, batches: [{ commit: 600, pk: 900, amount: 1000 }],
      winner: 600, winnerPk: 900, winnerCommitB: 700, winningAmount: 1000, ...over });

    it("B1: a payout commitment that is not the winner's commitB is refused at proposal", async function () {
      const { auction } = await deploy();
      await upToBatch(auction);
      await expect(auction.settleOptimistic(ZERO_PROOF, claim({ winnerCommitB: 12345 }), batchIdsArr([1])))
        .to.be.revertedWithCustomError(auction, "PayoutCommitMismatch");
      expect(Number((await auction.getAuctionCore(1)).state)).to.equal(BIDDING);
    });

    it("B2: a claim that can never finalize can be cancelled after a grace period", async function () {
      const { auction, erc20Vault } = await deploy();
      await upToBatch(auction);
      await auction.settleOptimistic(ZERO_PROOF, claim(), batchIdsArr([1]));
      expect(Number((await auction.getAuctionCore(1)).state)).to.equal(PENDING_SETTLEMENT);

      // The USDC vault cannot apply the settlement, so no path finalizes it.
      await erc20Vault.setNullifyReverts(true);
      await time.increase(2 * DAY);
      await expect(auction.finalizeSettlement(1)).to.be.reverted;
      // Too early to cancel: the claim's challenge window plus the grace period is not over.
      await expect(auction.recoverAuction(1)).to.be.revertedWithCustomError(auction, "SettlementDeadlineNotReached");

      await time.increase(30 * DAY);
      await auction.recoverAuction(1);
      expect(Number((await auction.getAuctionCore(1)).state)).to.equal(4); // CANCELED
      expect((await auction.getOptimisticClaim(1)).pending).to.equal(false);

      // Bidders get their notes back once the vault works again.
      await erc20Vault.setNullifyReverts(false);
      await auction.reclaimBid(1, 600);
    });

    it("B3: a healthy claim can still be finalized by anyone once its window closes", async function () {
      const { auction } = await deploy();
      await upToBatch(auction);
      await auction.settleOptimistic(ZERO_PROOF, claim(), batchIdsArr([1]));
      await expect(auction.recoverAuction(1)).to.be.revertedWithCustomError(auction, "SettlementDeadlineNotReached");
      await time.increase(2 * DAY);
      await auction.finalizeSettlement(1);
      expect(Number((await auction.getAuctionCore(1)).state)).to.equal(SETTLED);
    });
  });

  // ── C. Settlement must cover every live bid ───────────────────────────────
  describe("C: no bid can be left out", function () {
    it("C1: a claim that omits a live bid is refused", async function () {
      const { auction } = await deploy();
      await openAuction(auction, { deadlineDelta: 10 });
      await auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 500, commitA: 600, commitB: 700 }), "0x", "0x");
      await auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 501, commitA: 601, commitB: 701 }), "0x", "0x"); // the higher bid
      await time.increase(20);
      await auction.submitBatch(1, ZERO_PROOF, batchStatement({ auctionId: 1, commits: [600], winnerCommit: 600, winnerPk: 900, winnerAmount: 50 }));
      const stmt = finalStatement({ auctionId: 1, batches: [{ commit: 600, pk: 900, amount: 50 }],
        winner: 600, winnerPk: 900, winnerCommitB: 700, winningAmount: 50 });
      await expect(auction.settleOptimistic(ZERO_PROOF, stmt, batchIdsArr([1])))
        .to.be.revertedWithCustomError(auction, "BidsNotAllBatched");
    });

    it("C2: a withdrawn bid does not have to be batched", async function () {
      const { auction } = await deploy();
      await openAuction(auction, { deadlineDelta: 3600 });
      await auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 500, commitA: 600, commitB: 700 }), "0x", "0x");
      await auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 501, commitA: 601, commitB: 701 }), "0x", "0x");
      await auction.withdrawBid(ZERO_PROOF, [1, 601]);
      await time.increase(3700);
      await auction.submitBatch(1, ZERO_PROOF, batchStatement({ auctionId: 1, commits: [600], winnerCommit: 600, winnerPk: 900, winnerAmount: 50 }));
      await auction.settleOptimistic(ZERO_PROOF, finalStatement({ auctionId: 1, batches: [{ commit: 600, pk: 900, amount: 50 }],
        winner: 600, winnerPk: 900, winnerCommitB: 700, winningAmount: 50 }), batchIdsArr([1]));
    });

    it("C3: bids stop at MAX_BIDS so all of them can always be batched", async function () {
      this.timeout(300000);
      const { auction } = await deploy();
      await openAuction(auction, { deadlineDelta: 100000 });
      const max = Number(await auction.MAX_BIDS());
      for (let i = 0; i < max; i++) {
        await auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 10000 + i, commitA: 20000 + i, commitB: 30000 + i }), "0x", "0x");
      }
      await expect(auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 99999, commitA: 99998, commitB: 99997 }), "0x", "0x"))
        .to.be.revertedWithCustomError(auction, "MaxBidsReached");
    });
  });

  // ── D. The claim must name the highest batch winner ───────────────────────
  describe("D: winner selection is checked on-chain", function () {
    async function twoBatches(auction, { lowAmount = 50, highAmount = 5000, floorPrice = 10 } = {}) {
      await openAuction(auction, { deadlineDelta: 10, floorPrice });
      await auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 500, commitA: 600, commitB: 700 }), "0x", "0x");
      await auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 501, commitA: 601, commitB: 701 }), "0x", "0x");
      await time.increase(20);
      await auction.submitBatch(1, ZERO_PROOF, batchStatement({ auctionId: 1, commits: [600], winnerCommit: 600, winnerPk: 900, winnerAmount: lowAmount }));
      await auction.submitBatch(2, ZERO_PROOF, batchStatement({ auctionId: 1, commits: [601], winnerCommit: 601, winnerPk: 901, winnerAmount: highAmount }));
    }
    const batches = (low = 50, high = 5000) => [{ commit: 600, pk: 900, amount: low }, { commit: 601, pk: 901, amount: high }];

    it("D1: a lower batch winner cannot be crowned", async function () {
      const { auction } = await deploy();
      await twoBatches(auction);
      const stmt = finalStatement({ auctionId: 1, batches: batches(), winner: 600, winnerPk: 900, winnerCommitB: 700, winningAmount: 50 });
      await expect(auction.settleOptimistic(ZERO_PROOF, stmt, batchIdsArr([1, 2])))
        .to.be.revertedWithCustomError(auction, "WinnerNotHighest");
    });

    it("D2: the highest batch winner is accepted", async function () {
      const { auction } = await deploy();
      await twoBatches(auction);
      const stmt = finalStatement({ auctionId: 1, batches: batches(), winner: 601, winnerPk: 901, winnerCommitB: 701, winningAmount: 5000 });
      await auction.settleOptimistic(ZERO_PROOF, stmt, batchIdsArr([1, 2]));
    });

    it("D3: the claimed winning amount must be the winner's batch amount", async function () {
      const { auction } = await deploy();
      await twoBatches(auction);
      const stmt = finalStatement({ auctionId: 1, batches: batches(), winner: 601, winnerPk: 901, winnerCommitB: 701, winningAmount: 4000 });
      await expect(auction.settleOptimistic(ZERO_PROOF, stmt, batchIdsArr([1, 2])))
        .to.be.revertedWithCustomError(auction, "WinnerAmountMismatch");
    });

    it("D4: the claimed winner key must be the winner's batch key", async function () {
      const { auction } = await deploy();
      await twoBatches(auction);
      const stmt = finalStatement({ auctionId: 1, batches: batches(), winner: 601, winnerPk: 999, winnerCommitB: 701, winningAmount: 5000 });
      await expect(auction.settleOptimistic(ZERO_PROOF, stmt, batchIdsArr([1, 2])))
        .to.be.revertedWithCustomError(auction, "BatchResultMismatch");
    });

    it("D5: a winning amount below the floor price is refused", async function () {
      const { auction } = await deploy();
      await twoBatches(auction, { lowAmount: 5, highAmount: 8, floorPrice: 10 });
      const stmt = finalStatement({ auctionId: 1, batches: batches(5, 8), winner: 601, winnerPk: 901, winnerCommitB: 701, winningAmount: 8, floorPrice: 10 });
      await expect(auction.settleOptimistic(ZERO_PROOF, stmt, batchIdsArr([1, 2])))
        .to.be.revertedWithCustomError(auction, "BelowFloorPrice");
    });
  });

  // ── F. batchId 0 ──────────────────────────────────────────────────────────
  it("F: batchId 0 is refused", async function () {
    const { auction } = await deploy();
    await openAuction(auction, { deadlineDelta: 10 });
    await auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 500, commitA: 600, commitB: 700 }), "0x", "0x");
    await time.increase(20);
    await expect(auction.submitBatch(0, ZERO_PROOF, batchStatement({ auctionId: 1, commits: [600], winnerCommit: 600, winnerPk: 900, winnerAmount: 1000 })))
      .to.be.revertedWithCustomError(auction, "InvalidBatchId");
  });

  // ── G. Bid ciphertexts are bound to the proof ─────────────────────────────
  describe("G: bid ciphertexts", function () {
    it("G1: a front-runner cannot register the bid with ciphertexts of its own", async function () {
      const [, bidder, attacker] = await ethers.getSigners();
      const { auction } = await deploy();
      await openAuction(auction);
      const real1 = "0x1111", real2 = "0x2222";
      const stmt = bidStatement({ auctionId: 1, nullifier: 500, commitA: 600, commitB: 700, ctxt1: real1, ctxt2: real2 });
      await expect(auction.connect(attacker).submitBid(ZERO_PROOF, stmt, "0xdeadbeef", "0xdeadbeef"))
        .to.be.revertedWithCustomError(auction, "CiphertextMismatch");
      // The bidder's own submission still goes through.
      await auction.connect(bidder).submitBid(ZERO_PROOF, stmt, real1, real2);
      expect((await auction.getBid(1, 600)).active).to.equal(true);
    });

    it("G2: the hash is over both ciphertexts, in order", async function () {
      const { auction } = await deploy();
      await openAuction(auction);
      const stmt = bidStatement({ auctionId: 1, nullifier: 500, commitA: 600, commitB: 700, ctxt1: "0x11", ctxt2: "0x22" });
      await expect(auction.submitBid(ZERO_PROOF, stmt, "0x22", "0x11")).to.be.revertedWithCustomError(auction, "CiphertextMismatch");
      await expect(auction.submitBid(ZERO_PROOF, stmt, "0x1122", "0x")).to.be.revertedWithCustomError(auction, "CiphertextMismatch");
    });
  });
});

// ── E. Is the 750,000-gas floor of challengeSettlement() enough? ─────────────
describe("EnygmaAuction — challengeSettlement gas floor", function () {
  const G1 = ["1", "2"];
  const G2 = [
    ["11559732032986387107991004021392285783925812861821192530917403151452391805634",
     "10857046999023057135944570762232829481370756359578518086990519993285655852781"],
    ["4082367875863433681332203403145435568316851327593401208105741076214120093531",
     "8495653923123431417604973247489272438418190587263600148770280649306958101930"],
  ];

  async function realVerifierWithFinalVk(finalSlot) {
    const generic = await (await ethers.getContractFactory("GenericGroth16Verifier")).deploy();
    const real = await (await ethers.getContractFactory("Verifier")).deploy();
    await real.initializeVerifier(await generic.getAddress());
    const mkVk = (nIc) => ({ alpha1: G1, beta2: G2, gamma2: G2, delta2: G2, ic: Array.from({ length: nIc }, () => G1) });
    for (let i = 0; i <= finalSlot; i++) await real.addVerificationKey(mkVk(i === finalSlot ? 39 : 2));
    return real;
  }

  it("E: a valid optimistic claim is not voided by a challenger who supplies just enough gas", async function () {
    const FINAL = 3;
    const real = await realVerifierWithFinalVk(FINAL);
    const { auction } = await deploy("RoutingVerifier", [await real.getAddress(), FINAL], [10, 11, 12, FINAL, 14, 15]);
    // Mirror EnygmaAuction.test.js's honest single-batch path; the routed proof is always "valid".
    await openAuction(auction, { deadlineDelta: 10 });
    await auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 500, commitA: 600, commitB: 700 }), "0x", "0x");
    await time.increase(20);
    await auction.submitBatch(1, ZERO_PROOF, batchStatement({ auctionId: 1, commits: [600], winnerCommit: 600, winnerPk: 900, winnerAmount: 1000 }));
    const stmt = finalStatement({ auctionId: 1, batches: [{ commit: 600, pk: 900, amount: 1000 }],
      winner: 600, winnerPk: 900, winnerCommitB: 700, winningAmount: 1000 });
    // 38 real field elements as public inputs so the verifier performs all 38 scalar multiplications.
    await auction.settleOptimistic(ZERO_PROOF, stmt, batchIdsArr([1]));

    const need = await ethers.provider.estimateGas({
      to: await auction.getAddress(),
      data: auction.interface.encodeFunctionData("verifyFinalProof", [1]),
    });
    console.log(`      gas to run verifyFinalProof (incl. 21k intrinsic + calldata): ${need}`);

    // Find the smallest gas limit at which a challenge finalizes an honest claim,
    // and the smallest at which it gets past the contract's own 750k gasleft() check.
    const snap = await takeSnapshot();
    const outcome = async (gasLimit) => {
      try {
        await auction.challengeSettlement(1, { gasLimit });
        const s = Number((await auction.getAuctionCore(1)).state);
        return s === SETTLED ? "finalized" : s === BIDDING ? "VOIDED" : `state ${s}`;
      } catch (e) {
        return e.message.includes("insufficient gas") ? "rejected(gas)" : "reverted";
      } finally { await snap.restore(); }
    };
    let firstPastCheck = null, firstFinalized = null, last = null;
    for (let g = 700_000; g <= 1_500_000; g += 10_000) {
      const r = await outcome(g);
      if (r !== last) { console.log(`      gasLimit ${g}: ${r}`); last = r; }
      if (r !== "rejected(gas)" && firstPastCheck === null) firstPastCheck = { g, r };
      if (r === "finalized" && firstFinalized === null) firstFinalized = g;
      if (firstFinalized) break;
    }
    console.log(`      first gasLimit past the 750k check: ${firstPastCheck && firstPastCheck.g} -> ${firstPastCheck && firstPastCheck.r}`);
    console.log(`      first gasLimit that finalizes the honest claim: ${firstFinalized}`);
    if (firstPastCheck && firstPastCheck.r === "VOIDED") {
      expect.fail(`VULNERABLE: with gasLimit=${firstPastCheck.g} the challenge passes the gas check but voids an HONEST claim ` +
        `(needs ${firstFinalized} to finalize)`);
    }
  });
});
