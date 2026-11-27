const { expect } = require("chai");
const { ethers } = require("hardhat");
const { expectRevert } = require("./helpers.js");

const FIELD = ethers.BigNumber.from("21888242871839275222246405745257275088548364400416034343698204186575808495617");
const FIELD_MAX = FIELD.sub(1);

async function deployVault(vaultName, tokenName) {
  const [owner, recipient] = await ethers.getSigners();
  const token = await (await ethers.getContractFactory(tokenName)).deploy("Asset", "A");
  const hash = await (await ethers.getContractFactory("MockPoseidonWrapper")).deploy();
  const vault = await (await ethers.getContractFactory(vaultName)).deploy(owner.address);
  await (await vault.initializeVault(0, 1, token.address, 8, hash.address, ethers.constants.AddressZero, owner.address)).wait();
  return { owner, recipient, vault, token };
}

// This receipt has the right shape to reach the input checks. Its empty
// message and opening make both kinds of withdrawal stop before verification.
function emptyReceipt(numberOfOutputs = 2, length = 8) {
  return {
    proof: [[0, 0], [[0, 0], [0, 0]], [0, 0]],
    numberOfInputs: 1,
    numberOfOutputs,
    statement: Array(length).fill(0),
  };
}

describe("ERC20 vault field bounds", function () {
  let owner, recipient, vault, token;

  beforeEach(async function () {
    ({ owner, recipient, vault, token } = await deployVault("Erc20CoinVault", "RaylsERC20"));
  });

  const deposits = {
    deposit: (vault, params) => vault.deposit(params),
    depositV2: (vault, params) => vault.depositV2(params, "0x", "0x"),
  };
  for (const [name, deposit] of Object.entries(deposits)) {
    it(`${name} accepts r minus one and rejects r as the amount`, async function () {
      await (await token.mint(owner.address, FIELD)).wait();
      await (await token.approve(vault.address, FIELD)).wait();
      await expectRevert(vault, deposit(vault, [FIELD, 11, 22, 0]), "ValueOutOfField");
      await (await deposit(vault, [FIELD_MAX, 11, 22, 0])).wait();
      expect((await token.balanceOf(vault.address)).toString()).to.equal(FIELD_MAX.toString());
    });

    it(`${name} accepts r minus one and rejects r as the token ID`, async function () {
      await (await token.mint(owner.address, 1)).wait();
      await (await token.approve(vault.address, 1)).wait();
      await expectRevert(vault, deposit(vault, [1, 11, 22, FIELD]), "ValueOutOfField");
      await (await deposit(vault, [1, 11, 22, FIELD_MAX])).wait();
      expect((await token.balanceOf(vault.address)).toNumber()).to.equal(1);
    });
  }

  const withdrawals = {
    withdrawV2: (vault, params, recipient) => vault.withdrawV2(params, recipient, emptyReceipt()),
    partialWithdraw: (vault, params, recipient) => vault.partialWithdraw(params, recipient, emptyReceipt(), "0x", "0x"),
    partialWithdrawWithRelayerFee: (vault, params, recipient) => vault.partialWithdrawWithRelayerFee(params, recipient, emptyReceipt(3, 10), "0x", "0x"),
  };
  for (const [name, withdraw] of Object.entries(withdrawals)) {
    it(`${name} checks the amount and token ID before the withdrawal receipt`, async function () {
      await expectRevert(vault, withdraw(vault, [FIELD, 0], recipient.address), "ValueOutOfField");
      await expectRevert(vault, withdraw(vault, [1, FIELD], recipient.address), "ValueOutOfField");
      await expectRevert(vault, withdraw(vault, [FIELD_MAX, FIELD_MAX], recipient.address), "InvalidPaymentMessage");
    });
  }

  it("legacy withdraw checks the field bound before the opening", async function () {
    await expectRevert(vault, vault.withdraw([FIELD], recipient.address, emptyReceipt()), "ValueOutOfField");
    await expectRevert(vault, vault.withdraw([FIELD_MAX], recipient.address, emptyReceipt()), "InvalidOpening");
  });
});

describe("ERC721 vault field bounds", function () {
  let owner, recipient, vault, token;

  beforeEach(async function () {
    ({ owner, recipient, vault, token } = await deployVault("Erc721CoinVault", "RaylsERC721"));
  });

  it("deposit accepts a token ID of r minus one and rejects r", async function () {
    await (await token.mint(owner.address, FIELD_MAX)).wait();
    await (await token.mint(owner.address, FIELD)).wait();
    await (await token.setApprovalForAll(vault.address, true)).wait();

    await expectRevert(vault, vault.deposit([FIELD, 11, 22]), "ValueOutOfField");
    expect(await token.ownerOf(FIELD)).to.equal(owner.address);
    await (await vault.deposit([FIELD_MAX, 11, 22])).wait();
    expect(await token.ownerOf(FIELD_MAX)).to.equal(vault.address);
  });

  it("withdraw checks the token ID before the opening", async function () {
    await expectRevert(vault, vault.withdraw([FIELD], recipient.address, emptyReceipt()), "ValueOutOfField");
    await expectRevert(vault, vault.withdraw([FIELD_MAX], recipient.address, emptyReceipt()), "InvalidOpening");
  });
});
