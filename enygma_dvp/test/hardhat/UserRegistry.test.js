const { expect } = require("chai");
const { ethers } = require("hardhat");
const { expectRevert } = require("./helpers.js");

describe("DvP UserRegistry", function () {
  let registry, user;

  beforeEach(async function () {
    [user] = await ethers.getSigners();
    registry = await (await ethers.getContractFactory("UserRegistry")).deploy();
    await registry.deployed();
  });

  it("rejects a zero spend key without adding a registration", async function () {
    await expectRevert(registry, registry.register(0, "0x12"), "ZeroPkSpend");
    expect((await registry.getUserCount()).toNumber()).to.equal(0);
    expect(await registry.isRegistered(user.address)).to.equal(false);
  });

  it("rejects a second registration without changing the count or keys", async function () {
    await (await registry.register(42, "0x12")).wait();
    await expectRevert(registry, registry.register(43, "0x34"), "AlreadyRegistered");
    expect((await registry.getUserCount()).toNumber()).to.equal(1);
    expect(await registry.getUserAt(0)).to.equal(user.address);
    const keys = await registry.getKeys(user.address);
    expect(keys.pkSpend.toNumber()).to.equal(42);
    expect(keys.pkView).to.equal("0x12");
  });
});
