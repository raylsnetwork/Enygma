// Erc20CoinVault takes only values a note can carry. A deposit's amount must
// be one a proof can spend (0 < amount < 10^36, the circuits' value bound) and
// its token id the vault's TOKEN_ID; a withdrawal's amount must be below
// 10^36, which keeps it below the field modulus p that the onchain Poseidon
// reduces by, and its token id TOKEN_ID. A vault is registered only with a
// verifier, and grants DEFAULT_AUCTION_ROLE only to a real auction address.
//
// The vaults are registered through the real EnygmaDvp with AcceptAllVerifier,
// so a refusal here is the vault's own check, not a proof's.
const { expect } = require("chai");
const { ethers, artifacts } = require("hardhat");
const { POSEIDON_SOURCE } = require("../../scripts/regen_poseidon.js");
const { expectRevert } = require("./expectRevert.js");

const { BigNumber, constants } = ethers;
const TREE_DEPTH = 8;
// BN254's scalar field modulus, by which the onchain Poseidon reduces inputs.
const P = BigNumber.from("21888242871839275222246405745257275088548364400416034343698204186575808495617");
const MAX_VALUE = BigNumber.from(10).pow(36);
const WITHDRAW_MESSAGE = 1;
const CAPSULE = "0x" + "ab".repeat(1088);
const NOTE = "0x" + "cd".repeat(48);

// Deploys a Poseidon library from its abi and bytecode: the artifacts the
// compile hook writes for these libraries lack deployedLinkReferences, which
// getContractFactory(name) wants.
async function deployPoseidonLibrary(name) {
  const artifact = await artifacts.readArtifact(`${POSEIDON_SOURCE}:${name}`);
  return (await ethers.getContractFactory(artifact.abi, artifact.bytecode)).deploy();
}

// The PoseidonWrapper the vaults hash with, and AcceptAllVerifier.
async function deployHashAndVerifier() {
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
  return { wrapper, verifier };
}

// A fresh EnygmaDvp (deployed and owned by the first signer) and an
// Erc20CoinVault for a RaylsERC20, not yet registered.
async function deployDvpAndVault(wrapper, verifier) {
  const dvp = await (await ethers.getContractFactory("EnygmaDvp")).deploy(wrapper.address, verifier.address);
  const token = await (await ethers.getContractFactory("RaylsERC20")).deploy("Rayls", "RLS");
  const vault = await (await ethers.getContractFactory("Erc20CoinVault")).deploy(dvp.address);
  return { dvp, token, vault };
}

// A 1-input/2-output Payment receipt bound to vault whose output 0 is out0:
// [message, tree, root, nullifier, out0, out1, vault, ciphertextHash]. Its
// input has root 0 (zero-value padding), so the vault spends nothing and only
// its own checks on the withdrawal apply.
function withdrawalReceipt(vault, out0) {
  const zero = BigNumber.from(0);
  return {
    proof: { a: { x: zero, y: zero }, b: { x: [zero, zero], y: [zero, zero] }, c: { x: zero, y: zero } },
    statement: [WITHDRAW_MESSAGE, 0, 0, 0, out0, 4242, BigNumber.from(vault.address), 0],
    numberOfInputs: 1,
    numberOfOutputs: 2,
  };
}

describe("Erc20CoinVault inputs", function () {
  let owner, depositor, recipient, auction, wrapper, verifier;

  before(async function () {
    [owner, depositor, recipient, auction] = await ethers.getSigners();
    ({ wrapper, verifier } = await deployHashAndVerifier());
  });

  // A vault registered through EnygmaDvp, as init_payment registers one.
  async function registeredVault() {
    const { dvp, token, vault } = await deployDvpAndVault(wrapper, verifier);
    await (await dvp.initializeDvp(verifier.address)).wait();
    await (await dvp.registerVault(vault.address, token.address, 1, TREE_DEPTH)).wait();
    return { dvp, token, vault };
  }

  const deposits = {
    deposit: (vault, params) => vault.deposit(params),
    depositV2: (vault, params) => vault.depositV2(params, CAPSULE, NOTE),
  };

  for (const [name, call] of Object.entries(deposits)) {
    describe(name, function () {
      let token, vault;

      beforeEach(async function () {
        ({ token, vault } = await registeredVault());
      });

      // Funds and approves depositor for amount, then calls the deposit.
      async function depositAs(amount, tokenId) {
        await (await token.mint(depositor.address, amount)).wait();
        await (await token.connect(depositor).approve(vault.address, amount)).wait();
        return call(vault.connect(depositor), [amount, 11, 22, tokenId]);
      }

      // Fails unless the deposit of amount with tokenId reverts with error,
      // leaving the tree and the vault's balance as they were.
      async function expectRefused(amount, tokenId, error) {
        const rootBefore = await vault.merkleRoot();
        await expectRevert(vault, depositAs(amount, tokenId), error);
        expect((await vault.merkleRoot()).toString()).to.equal(rootBefore.toString());
        expect((await token.balanceOf(vault.address)).toString()).to.equal("0");
      }

      it("refuses an amount of 0, which no proof can spend", async function () {
        await expectRefused(0, 0, "ZeroDepositAmount");
      });

      it("refuses an amount of 10^36 or more, and one of p + 5, which would make a note worth 5", async function () {
        await expectRefused(MAX_VALUE, 0, "AmountOutOfRange");
        await expectRefused(P.add(5), 0, "AmountOutOfRange");
      });

      it("refuses a token id other than TOKEN_ID, and p, which hashes as 0", async function () {
        expect((await vault.TOKEN_ID()).toString()).to.equal("0");
        await expectRefused(1000, 7, "InvalidTokenId");
        await expectRefused(1000, P, "InvalidTokenId");
      });

      it("takes 10^36 - 1, the largest amount a note can carry", async function () {
        const amount = MAX_VALUE.sub(1);
        const rootBefore = await vault.merkleRoot();
        await (await depositAs(amount, 0)).wait();
        expect((await vault.merkleRoot()).toString()).to.not.equal(rootBefore.toString());
        expect((await token.balanceOf(vault.address)).toString()).to.equal(amount.toString());
      });
    });
  }

  const withdrawals = {
    withdrawV2: (vault, params, to, receipt) => vault.withdrawV2(params, to, receipt),
    partialWithdraw: (vault, params, to, receipt) => vault.partialWithdraw(params, to, receipt, "0x", "0x"),
  };

  for (const [name, call] of Object.entries(withdrawals)) {
    describe(name, function () {
      const v = BigNumber.from(40);
      let token, vault;

      beforeEach(async function () {
        ({ token, vault } = await registeredVault());
      });

      // Output 0 of a withdrawal to recipient of amount with tokenId, as
      // the vault recomputes it.
      async function opening(amount, tokenId) {
        return wrapper.poseidon4([BigNumber.from(recipient.address), 0, amount, tokenId]);
      }

      // Gives the vault balance tokens without a deposit (a deposit takes
      // less than 10^36).
      async function fundVault(balance) {
        await (await token.mint(vault.address, balance)).wait();
      }

      it("pays a withdrawal of v that opens output 0 (the receipt these tests change)", async function () {
        await fundVault(v);
        const receipt = withdrawalReceipt(vault, await opening(v, 0));
        await (await call(vault, [v, 0], recipient.address, receipt)).wait();
        expect((await token.balanceOf(recipient.address)).toString()).to.equal(v.toString());
      });

      it("refuses an amount of 10^36, even for a receipt that opens with it", async function () {
        await fundVault(MAX_VALUE);
        const receipt = withdrawalReceipt(vault, await opening(MAX_VALUE, 0));
        await expectRevert(vault, call(vault, [MAX_VALUE, 0], recipient.address, receipt), "AmountOutOfRange");
        expect((await token.balanceOf(recipient.address)).toString()).to.equal("0");
      });

      it("refuses v + p for a proof of v, though output 0 opens to it mod p", async function () {
        await fundVault(P.add(v));
        expect((await wrapper.poseidon4([BigNumber.from(recipient.address), 0, P.add(v), 0])).toString())
          .to.equal((await opening(v, 0)).toString());
        const receipt = withdrawalReceipt(vault, await opening(v, 0));
        await expectRevert(vault, call(vault, [P.add(v), 0], recipient.address, receipt), "AmountOutOfRange");
        expect((await token.balanceOf(recipient.address)).toString()).to.equal("0");
      });

      it("refuses a token id other than TOKEN_ID, even for a receipt that opens with it", async function () {
        await fundVault(v);
        const tagged = withdrawalReceipt(vault, await opening(v, 7));
        await expectRevert(vault, call(vault, [v, 7], recipient.address, tagged), "InvalidTokenId");
        const plain = withdrawalReceipt(vault, await opening(v, 0));
        await expectRevert(vault, call(vault, [v, P], recipient.address, plain), "InvalidTokenId");
        expect((await token.balanceOf(recipient.address)).toString()).to.equal("0");
      });
    });
  }

  describe("registration", function () {
    it("refuses registerVault before initializeDvp, whose verifier would be address 0", async function () {
      const { dvp, token, vault } = await deployDvpAndVault(wrapper, verifier);
      await expectRevert(vault, dvp.registerVault(vault.address, token.address, 1, TREE_DEPTH), "ZeroVerifierAddress");
      expect(await vault.getVerifierContractAddress()).to.equal(constants.AddressZero);

      await (await dvp.initializeDvp(verifier.address)).wait();
      await (await dvp.registerVault(vault.address, token.address, 1, TREE_DEPTH)).wait();
      expect(await vault.getVerifierContractAddress()).to.equal(verifier.address);
    });

    it("grants DEFAULT_AUCTION_ROLE to no account when no auction is registered", async function () {
      const { vault } = await registeredVault();
      const role = await vault.DEFAULT_AUCTION_ROLE();
      expect(await vault.hasRole(role, constants.AddressZero)).to.equal(false);
    });

    it("grants DEFAULT_AUCTION_ROLE to the auction address it is given", async function () {
      // owner acts as the vault's EnygmaDvp, so it may initialize the vault.
      const token = await (await ethers.getContractFactory("RaylsERC20")).deploy("Rayls", "RLS");
      const vault = await (await ethers.getContractFactory("Erc20CoinVault")).deploy(owner.address);
      await (await vault.initializeVault(0, 1, token.address, TREE_DEPTH, wrapper.address, verifier.address, auction.address)).wait();
      const role = await vault.DEFAULT_AUCTION_ROLE();
      expect(await vault.hasRole(role, auction.address)).to.equal(true);
      expect(await vault.hasRole(role, constants.AddressZero)).to.equal(false);
    });
  });
});
