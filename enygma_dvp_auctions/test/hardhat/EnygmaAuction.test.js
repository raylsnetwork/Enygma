const { expect } = require("chai");
const { ethers } = require("hardhat");
const { time } = require("@nomicfoundation/hardhat-network-helpers");

// AuctionState enum (IEnygmaAuction.sol)
const INACTIVE = 0;
const BIDDING = 1;
const PENDING_SETTLEMENT = 2;
const SETTLED = 3;
const CANCELED = 4;

const ZERO_PROOF = Array(8).fill(0);

function lockStatement({ auctionId, treeNumber = 0, merkleRoot = 1, nullifier, commitLocked, nftTokenId, revertCommit = 0 }) {
  return [auctionId, treeNumber, merkleRoot, nullifier, commitLocked, nftTokenId, revertCommit];
}

function bidStatement({ auctionId, treeNumber = 0, merkleRoot = 1, nullifier, commitA, commitB, revertCommit = 0 }) {
  return [auctionId, treeNumber, merkleRoot, nullifier, commitA, commitB, revertCommit];
}

function batchStatement({ auctionId, slots, winnerCommit, winnerPk, winnerAmount }) {
  const bidCommitA = Array(100).fill(0);
  slots.forEach((s, i) => { bidCommitA[i] = s.commitA; });
  return [auctionId, ...bidCommitA, winnerCommit, winnerPk, winnerAmount];
}

function finalStatement({
  auctionId, batches, overallWinnerCommit, winnerPk, winnerCommitB, winnerNftCommit,
  nftTokenId, winningAmount, floorPrice,
}) {
  const commits = Array(10).fill(0);
  const pks = Array(10).fill(0);
  const amounts = Array(10).fill(0);
  batches.forEach((b, i) => {
    if (b) { commits[i] = b.commit; pks[i] = b.pk; amounts[i] = b.amount; }
  });
  return [auctionId, ...commits, ...pks, ...amounts, overallWinnerCommit, winnerPk, winnerCommitB, winnerNftCommit, nftTokenId, winningAmount, floorPrice];
}

function revertStatement({ auctionId, commitLocked, nftTokenId, revertedCommit }) {
  return [auctionId, commitLocked, nftTokenId, revertedCommit];
}

function withdrawStatement({ auctionId, commitA }) {
  return [auctionId, commitA];
}

function batchIdsArr(map) {
  const arr = Array(10).fill(0);
  for (const [i, v] of Object.entries(map)) arr[Number(i)] = v;
  return arr;
}

describe("EnygmaAuction", function () {
  let deployer, outsider;
  let verifier, erc20Vault, erc721Vault, auction;

  beforeEach(async function () {
    [deployer, outsider] = await ethers.getSigners();

    const Verifier = await ethers.getContractFactory("MockVerifier");
    verifier = await Verifier.deploy();

    const Vault = await ethers.getContractFactory("MockCoinVault");
    erc20Vault = await Vault.deploy();
    erc721Vault = await Vault.deploy();

    const Auction = await ethers.getContractFactory("EnygmaAuction");
    auction = await Auction.deploy(
      await verifier.getAddress(),
      await erc20Vault.getAddress(),
      await erc721Vault.getAddress(),
      1, 2, 3, 4, 5, 6, // VK_LOCK..VK_WITHDRAW — arbitrary, MockVerifier ignores them
    );
  });

  // lockAuction: creates an auction with deadline=now+deadlineDelta,
  // settlementDeadline=deadline+7200. Returns the deadline timestamp.
  async function lockAuction({ auctionId, nftTokenId = 77, commitLocked, deadlineDelta = 3600, floorPrice = 10 }) {
    const deadline = (await time.latest()) + deadlineDelta;
    const settlementDeadline = deadline + 7200;
    await auction.initAuction(
      ZERO_PROOF,
      lockStatement({ auctionId, nullifier: 1000 + auctionId, commitLocked, nftTokenId }),
      deadline,
      settlementDeadline,
      floorPrice,
    );
    return deadline;
  }

  // ── initAuction ──────────────────────────────────────────────────────────

  describe("initAuction", function () {
    it("opens BIDDING and records deadline/floorPrice", async function () {
      const deadline = await lockAuction({ auctionId: 1, commitLocked: 200 });
      const core = await auction.getAuctionCore(1);
      expect(core.state).to.equal(BIDDING);
      expect(core.nftTokenId).to.equal(77n);
      expect(core.commitLocked).to.equal(200n);
      expect(core.deadline).to.equal(BigInt(deadline));
      expect(core.floorPrice).to.equal(10n);
    });

    it("rejects a second init for the same auctionId", async function () {
      await lockAuction({ auctionId: 1, commitLocked: 200 });
      await expect(lockAuction({ auctionId: 1, commitLocked: 200 }))
        .to.be.revertedWithCustomError(auction, "AuctionAlreadyExists");
    });

    it("rejects a deadline in the past", async function () {
      const now = await time.latest();
      const settlementDeadline = now + 7200;
      await expect(
        auction.initAuction(ZERO_PROOF, lockStatement({ auctionId: 1, nullifier: 1, commitLocked: 200, nftTokenId: 77 }), now - 1, settlementDeadline, 10)
      ).to.be.revertedWithCustomError(auction, "InvalidDeadline");
    });

    it("rejects a settlementDeadline that is not after the bidding deadline", async function () {
      const now = await time.latest();
      const deadline = now + 3600;
      await expect(
        auction.initAuction(ZERO_PROOF, lockStatement({ auctionId: 1, nullifier: 1, commitLocked: 200, nftTokenId: 77 }), deadline, deadline, 10)
      ).to.be.revertedWithCustomError(auction, "InvalidSettlementDeadline");
    });
  });

  // ── submitBid ────────────────────────────────────────────────────────────

  describe("submitBid", function () {
    it("registers an active bid and locks (not nullifies) the USDC note", async function () {
      await lockAuction({ auctionId: 1, commitLocked: 200 });
      await auction.submitBid(
        ZERO_PROOF,
        bidStatement({ auctionId: 1, nullifier: 500, commitA: 600, commitB: 700 }),
        "0x", "0x",
      );
      const bid = await auction.getBid(1, 600);
      expect(bid.active).to.equal(true);
      expect(bid.commitB).to.equal(700n);
      // submitBid now calls lockCoin, not nullifyCoin
      expect(await erc20Vault.locked(500n)).to.equal(true);
      expect(await erc20Vault.nullified(500n)).to.equal(false);
    });

    it("rejects a bid after the deadline", async function () {
      await lockAuction({ auctionId: 1, commitLocked: 200, deadlineDelta: 10 });
      await time.increase(20);
      await expect(
        auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 500, commitA: 600, commitB: 700 }), "0x", "0x")
      ).to.be.revertedWithCustomError(auction, "BiddingClosed");
    });

    it("rejects a duplicate commitA", async function () {
      await lockAuction({ auctionId: 1, commitLocked: 200 });
      await auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 500, commitA: 600, commitB: 700 }), "0x", "0x");
      await expect(
        auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 501, commitA: 600, commitB: 701 }), "0x", "0x")
      ).to.be.revertedWithCustomError(auction, "BidAlreadyExists");
    });
  });

  // ── withdrawBid ──────────────────────────────────────────────────────────

  describe("withdrawBid", function () {
    it("lets a bidder withdraw before the deadline: spends note, inserts revertCommit, deactivates bid", async function () {
      await lockAuction({ auctionId: 1, commitLocked: 200 });
      await auction.submitBid(
        ZERO_PROOF,
        bidStatement({ auctionId: 1, nullifier: 500, commitA: 600, commitB: 700, revertCommit: 800 }),
        "0x", "0x",
      );

      await auction.withdrawBid(ZERO_PROOF, withdrawStatement({ auctionId: 1, commitA: 600 }));

      const bid = await auction.getBid(1, 600);
      expect(bid.active).to.equal(false);
      expect(bid.claimed).to.equal(true);

      // note permanently spent
      expect(await erc20Vault.locked(500n)).to.equal(false);
      expect(await erc20Vault.nullified(500n)).to.equal(true);

      // revertCommit inserted into USDC tree
      expect(await erc20Vault.registeredCoinsLength()).to.equal(1);
      expect(await erc20Vault.registeredCoins(0)).to.equal(800n);

      // bidCount decremented
      const core = await auction.getAuctionCore(1);
      expect(core.bidCount).to.equal(0n);
    });

    it("rejects withdrawal after the deadline", async function () {
      await lockAuction({ auctionId: 1, commitLocked: 200, deadlineDelta: 10 });
      await auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 500, commitA: 600, commitB: 700 }), "0x", "0x");
      await time.increase(20);
      await expect(
        auction.withdrawBid(ZERO_PROOF, withdrawStatement({ auctionId: 1, commitA: 600 }))
      ).to.be.revertedWithCustomError(auction, "WithdrawalDeadlinePassed");
    });

    it("rejects withdrawal of a non-existent bid", async function () {
      await lockAuction({ auctionId: 1, commitLocked: 200 });
      await expect(
        auction.withdrawBid(ZERO_PROOF, withdrawStatement({ auctionId: 1, commitA: 999 }))
      ).to.be.revertedWithCustomError(auction, "BidNotFound");
    });

    it("rejects double withdrawal — bid is deactivated after first withdrawal", async function () {
      await lockAuction({ auctionId: 1, commitLocked: 200 });
      await auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 500, commitA: 600, commitB: 700 }), "0x", "0x");
      await auction.withdrawBid(ZERO_PROOF, withdrawStatement({ auctionId: 1, commitA: 600 }));
      // active=false after withdrawal, so second call shows BidNotFound (not BidAlreadyClaimed)
      await expect(
        auction.withdrawBid(ZERO_PROOF, withdrawStatement({ auctionId: 1, commitA: 600 }))
      ).to.be.revertedWithCustomError(auction, "BidNotFound");
    });

    it("withdrawn bids are rejected by submitBatch (active == false)", async function () {
      await lockAuction({ auctionId: 1, commitLocked: 200, deadlineDelta: 10 });
      await auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 500, commitA: 600, commitB: 700 }), "0x", "0x");
      await auction.withdrawBid(ZERO_PROOF, withdrawStatement({ auctionId: 1, commitA: 600 }));
      await time.increase(20);
      await expect(
        auction.submitBatch(1, ZERO_PROOF, batchStatement({ auctionId: 1, slots: [{ commitA: 600 }], winnerCommit: 600, winnerPk: 900, winnerAmount: 1000 }))
      ).to.be.revertedWithCustomError(auction, "UnknownBid");
    });

    it("emits BidWithdrawn", async function () {
      await lockAuction({ auctionId: 1, commitLocked: 200 });
      await auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 500, commitA: 600, commitB: 700 }), "0x", "0x");
      await expect(
        auction.withdrawBid(ZERO_PROOF, withdrawStatement({ auctionId: 1, commitA: 600 }))
      ).to.emit(auction, "BidWithdrawn").withArgs(1, 600);
    });
  });

  // ── submitBatch ──────────────────────────────────────────────────────────

  describe("submitBatch", function () {
    it("rejects a batch submitted before the deadline", async function () {
      await lockAuction({ auctionId: 1, commitLocked: 200 });
      await auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 500, commitA: 600, commitB: 700 }), "0x", "0x");
      await expect(
        auction.submitBatch(1, ZERO_PROOF, batchStatement({ auctionId: 1, slots: [{ commitA: 600 }], winnerCommit: 600, winnerPk: 900, winnerAmount: 1000 }))
      ).to.be.revertedWithCustomError(auction, "BidsStillOpen");
    });

    it("rejects a slot referencing an unknown bid", async function () {
      await lockAuction({ auctionId: 1, commitLocked: 200, deadlineDelta: 10 });
      await time.increase(20);
      await expect(
        auction.submitBatch(1, ZERO_PROOF, batchStatement({ auctionId: 1, slots: [{ commitA: 999 }], winnerCommit: 999, winnerPk: 1, winnerAmount: 1 }))
      ).to.be.revertedWithCustomError(auction, "UnknownBid");
    });

    it("rejects calls from a non-auctioneer", async function () {
      await lockAuction({ auctionId: 1, commitLocked: 200, deadlineDelta: 10 });
      await time.increase(20);
      await expect(
        auction.connect(outsider).submitBatch(1, ZERO_PROOF, batchStatement({ auctionId: 1, slots: [], winnerCommit: 0, winnerPk: 0, winnerAmount: 0 }))
      ).to.be.reverted; // AccessControl revert string
    });

    it("accepts a valid batch and records the result", async function () {
      await lockAuction({ auctionId: 1, commitLocked: 200, deadlineDelta: 10 });
      await auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 500, commitA: 600, commitB: 700 }), "0x", "0x");
      await time.increase(20);
      await auction.submitBatch(1, ZERO_PROOF, batchStatement({ auctionId: 1, slots: [{ commitA: 600 }], winnerCommit: 600, winnerPk: 900, winnerAmount: 1000 }));
      const br = await auction.getBatchResult(1, 1);
      expect(br.submitted).to.equal(true);
      expect(br.batchWinnerAmount).to.equal(1000n);
    });
  });

  // ── ZK-optimistic settlement ────────────────────────────────────────────

  describe("settlement (optimistic path)", function () {
    async function setUpThroughBatch(auctionId, { floorPrice = 10 } = {}) {
      await lockAuction({ auctionId, commitLocked: 200, deadlineDelta: 10, floorPrice });
      // loser
      await auction.submitBid(ZERO_PROOF, bidStatement({ auctionId, nullifier: 500, commitA: 600, commitB: 700 }), "0x", "0x");
      // winner
      await auction.submitBid(ZERO_PROOF, bidStatement({ auctionId, nullifier: 501, commitA: 601, commitB: 701 }), "0x", "0x");
      await time.increase(20);
      await auction.submitBatch(auctionId, ZERO_PROOF, batchStatement({
        auctionId, slots: [{ commitA: 600 }, { commitA: 601 }], winnerCommit: 601, winnerPk: 900, winnerAmount: 1000,
      }));
    }

    const finalStmt = (auctionId, { floorPrice = 10 } = {}) => finalStatement({
      auctionId, batches: [{ commit: 601, pk: 900, amount: 1000 }],
      overallWinnerCommit: 601, winnerPk: 900, winnerCommitB: 1234, winnerNftCommit: 5678,
      nftTokenId: 77, winningAmount: 1000, floorPrice,
    });

    it("settles via optimistic path: PENDING → challenge window → SETTLED", async function () {
      await setUpThroughBatch(1);
      await auction.settleOptimistic(ZERO_PROOF, finalStmt(1), batchIdsArr({ 0: 1 }));
      expect((await auction.getAuctionCore(1)).state).to.equal(PENDING_SETTLEMENT);

      await time.increase(2 * 24 * 3600); // past the default 1-day challengeWindow
      await auction.finalizeSettlement(1);

      const core = await auction.getAuctionCore(1);
      expect(core.state).to.equal(SETTLED);
      expect(core.overallWinnerCommit).to.equal(601n);

      expect(await erc721Vault.registeredCoinsLength()).to.equal(1);
      expect(await erc721Vault.registeredCoins(0)).to.equal(5678n);
      // winner's locked USDC note was permanently spent at settlement
      expect(await erc20Vault.locked(501n)).to.equal(false);
      expect(await erc20Vault.nullified(501n)).to.equal(true);
      // Bob's USDC payout commitment (winnerCommitB) was inserted
      expect(await erc20Vault.registeredCoinsLength()).to.equal(1);
      expect(await erc20Vault.registeredCoins(0)).to.equal(1234n);

      const winnerBid = await auction.getBid(1, 601);
      expect(winnerBid.claimed).to.equal(true);
    });

    it("rejects settleOptimistic before the deadline", async function () {
      await lockAuction({ auctionId: 1, commitLocked: 200, deadlineDelta: 3600 });
      await expect(
        auction.settleOptimistic(ZERO_PROOF, finalStmt(1), batchIdsArr({}))
      ).to.be.revertedWithCustomError(auction, "DeadlineNotReached");
    });

    it("rejects a floor price that doesn't match what was recorded at init", async function () {
      await setUpThroughBatch(1, { floorPrice: 10 });
      await expect(
        auction.settleOptimistic(ZERO_PROOF, finalStmt(1, { floorPrice: 999 }), batchIdsArr({ 0: 1 }))
      ).to.be.revertedWithCustomError(auction, "FloorPriceMismatch");
    });

    it("rejects a batch slot whose claimed result doesn't match what was actually submitted", async function () {
      await setUpThroughBatch(1);
      await expect(
        auction.settleOptimistic(
          ZERO_PROOF,
          finalStatement({
            auctionId: 1, batches: [{ commit: 601, pk: 900, amount: 9999 }], // wrong amount
            overallWinnerCommit: 601, winnerPk: 900, winnerCommitB: 1234, winnerNftCommit: 5678,
            nftTokenId: 77, winningAmount: 9999, floorPrice: 10,
          }),
          batchIdsArr({ 0: 1 }),
        )
      ).to.be.revertedWithCustomError(auction, "BatchResultMismatch");
    });

    it("rejects calls from a non-auctioneer", async function () {
      await setUpThroughBatch(1);
      await expect(
        auction.connect(outsider).settleOptimistic(ZERO_PROOF, finalStmt(1), batchIdsArr({ 0: 1 }))
      ).to.be.reverted; // AccessControl revert string
    });

    it("moves to PENDING_SETTLEMENT without verifying the proof", async function () {
      await setUpThroughBatch(1);
      await auction.settleOptimistic(ZERO_PROOF, finalStmt(1), batchIdsArr({ 0: 1 }));
      expect((await auction.getAuctionCore(1)).state).to.equal(PENDING_SETTLEMENT);
    });

    it("challenge with an invalid proof voids the claim and reopens BIDDING — no ETH transfers", async function () {
      await setUpThroughBatch(1);
      await auction.settleOptimistic(ZERO_PROOF, finalStmt(1), batchIdsArr({ 0: 1 }));

      await verifier.setResult(false); // simulate a fraudulent claim
      const tx = await auction.connect(outsider).challengeSettlement(1);
      await expect(tx).to.emit(auction, "AuctionSettlementChallenged").withArgs(1, outsider.address, true);
      expect((await auction.getAuctionCore(1)).state).to.equal(BIDDING);
    });

    it("challenge with a valid proof finalizes as SETTLED — no ETH transfers", async function () {
      await setUpThroughBatch(1);
      await auction.settleOptimistic(ZERO_PROOF, finalStmt(1), batchIdsArr({ 0: 1 }));

      // verifier.result defaults to true — claim is honest.
      const tx = await auction.connect(outsider).challengeSettlement(1);
      await expect(tx).to.emit(auction, "AuctionSettlementChallenged").withArgs(1, outsider.address, false);
      await expect(tx).to.emit(auction, "AuctionSettled");

      expect((await auction.getAuctionCore(1)).state).to.equal(SETTLED);
      expect(await erc721Vault.registeredCoinsLength()).to.equal(1);
    });

    it("finalizes an unchallenged claim after the window elapses, trusting it without verification", async function () {
      await setUpThroughBatch(1);
      await auction.settleOptimistic(ZERO_PROOF, finalStmt(1), batchIdsArr({ 0: 1 }));

      await expect(auction.finalizeSettlement(1)).to.be.revertedWithCustomError(auction, "ChallengeWindowOpen");

      await time.increase(2 * 24 * 3600); // past the default 1-day challengeWindow
      await auction.finalizeSettlement(1);
      expect((await auction.getAuctionCore(1)).state).to.equal(SETTLED);
    });
  });

  // ── revertAuction ────────────────────────────────────────────────────────

  describe("revertAuction", function () {
    it("allows Bob to cancel before the deadline even with active bids", async function () {
      await lockAuction({ auctionId: 1, commitLocked: 200, deadlineDelta: 3600 });
      await auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 500, commitA: 600, commitB: 700 }), "0x", "0x");

      // Bob cancels immediately — no time advance needed
      await auction.revertAuction(ZERO_PROOF, revertStatement({ auctionId: 1, commitLocked: 200, nftTokenId: 77, revertedCommit: 999 }));
      expect((await auction.getAuctionCore(1)).state).to.equal(CANCELED);
      expect(await erc721Vault.registeredCoinsLength()).to.equal(1);
      expect(await erc721Vault.registeredCoins(0)).to.equal(999n);
    });

    it("allows Bob to cancel after the deadline as well", async function () {
      await lockAuction({ auctionId: 1, commitLocked: 200, deadlineDelta: 3600 });
      await auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 500, commitA: 600, commitB: 700 }), "0x", "0x");
      await time.increase(3700);
      await auction.revertAuction(ZERO_PROOF, revertStatement({ auctionId: 1, commitLocked: 200, nftTokenId: 77, revertedCommit: 999 }));
      expect((await auction.getAuctionCore(1)).state).to.equal(CANCELED);
    });

    it("rejects a commitLocked that doesn't match what was recorded at init", async function () {
      await lockAuction({ auctionId: 1, commitLocked: 200, deadlineDelta: 3600 });
      await expect(
        auction.revertAuction(ZERO_PROOF, revertStatement({ auctionId: 1, commitLocked: 999, nftTokenId: 77, revertedCommit: 1 }))
      ).to.be.revertedWithCustomError(auction, "CommitLockedMismatch");
    });

    it("the reverted commitment must differ from commitLocked to preserve privacy (app-level expectation)", async function () {
      // The contract itself doesn't enforce revertedCommit != commitLocked —
      // that guarantee comes from the AuctionRevert circuit (see AuctionRevert.go).
      await lockAuction({ auctionId: 1, commitLocked: 200, deadlineDelta: 3600 });
      await auction.revertAuction(ZERO_PROOF, revertStatement({ auctionId: 1, commitLocked: 200, nftTokenId: 77, revertedCommit: 999 }));
      expect(await erc721Vault.registeredCoins(0)).to.not.equal(200n);
    });
  });

  // ── reclaimBid ───────────────────────────────────────────────────────────

  describe("reclaimBid", function () {
    it("lets a loser reclaim after SETTLED, blocks the winner, blocks double-claim", async function () {
      await lockAuction({ auctionId: 1, commitLocked: 200, deadlineDelta: 10 });
      await auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 500, commitA: 600, commitB: 700 }), "0x", "0x"); // loser
      await auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 501, commitA: 601, commitB: 701 }), "0x", "0x"); // winner
      await time.increase(20);
      await auction.submitBatch(1, ZERO_PROOF, batchStatement({
        auctionId: 1, slots: [{ commitA: 600 }, { commitA: 601 }], winnerCommit: 601, winnerPk: 900, winnerAmount: 1000,
      }));
      await auction.settleOptimistic(
        ZERO_PROOF,
        finalStatement({
          auctionId: 1, batches: [{ commit: 601, pk: 900, amount: 1000 }],
          overallWinnerCommit: 601, winnerPk: 900, winnerCommitB: 1234, winnerNftCommit: 5678,
          nftTokenId: 77, winningAmount: 1000, floorPrice: 10,
        }),
        batchIdsArr({ 0: 1 }),
      );
      await time.increase(2 * 24 * 3600); // past challenge window
      await auction.finalizeSettlement(1);

      // _applySettlement() already marks the winner's bid claimed=true, so the
      // `claimed` check is hit before the `WinnerCannotReclaim` check.
      await expect(auction.reclaimBid(1, 601)).to.be.revertedWithCustomError(auction, "BidAlreadyClaimed");

      await auction.reclaimBid(1, 600); // loser succeeds
      expect((await auction.getBid(1, 600)).claimed).to.equal(true);
      // loser's locked USDC note was permanently spent on reclaim
      expect(await erc20Vault.nullified(500n)).to.equal(true);

      // active=false after reclaim, so second call shows BidNotFound (not BidAlreadyClaimed)
      await expect(auction.reclaimBid(1, 600)).to.be.revertedWithCustomError(auction, "BidNotFound");
    });

    it("lets anyone reclaim after CANCELED (Bob canceled before the deadline)", async function () {
      await lockAuction({ auctionId: 1, commitLocked: 200, deadlineDelta: 3600 });
      await auction.submitBid(ZERO_PROOF, bidStatement({ auctionId: 1, nullifier: 500, commitA: 600, commitB: 700 }), "0x", "0x");

      // Bob cancels before the deadline — no time advance
      await auction.revertAuction(ZERO_PROOF, revertStatement({ auctionId: 1, commitLocked: 200, nftTokenId: 77, revertedCommit: 999 }));

      await auction.connect(outsider).reclaimBid(1, 600);
      expect((await auction.getBid(1, 600)).claimed).to.equal(true);
    });
  });
});
