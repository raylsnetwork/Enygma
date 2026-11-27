// RaylsERC1155.registerNewToken: only the owner registers, a registered id
// cannot be registered again, and a meta token cannot back another token (#414);
// safeBatchTransferFrom writes no token state and refuses id 0 (#445).
const { expect } = require("chai");
const { ethers } = require("hardhat");
const { expectRevert } = require("./helpers.js");

const NORMAL = 0;
const SUB_TOKEN = 1;
const META_TOKEN_BOND = 2;
const META_TOKEN_DLC = 3;
const FUNGIBLE = 0;
const NON_FUNGIBLE = 1;
const NOT_EXIST = 0;
const REGISTERED = 1;

// Registers a token with the given type, fungibility, ids and caps.
const register = (token, { tType = NORMAL, fungibility = FUNGIBLE, name = "S", symbol = "S", offchainId = 100, maxSupply = 1000, subIds = [], subValues = [] } = {}) =>
  token.registerNewToken(tType, fungibility, name, symbol, offchainId, maxSupply, 0, subIds, subValues, "0x", []);

// The bond these tests register: backed 10:1 by token 100.
const BOND = { tType: META_TOKEN_BOND, name: "Bond", symbol: "BND", offchainId: 7, maxSupply: 10, subIds: [100], subValues: [10] };

// The id registerNewToken gives the bond.
const bondId = (token, tType = META_TOKEN_BOND) =>
  token.tokenId([tType, 0, FUNGIBLE, BOND.name, BOND.symbol, BOND.offchainId, BOND.subIds, BOND.subValues, 0, BOND.maxSupply, 0, "0x", []]);

describe("RaylsERC1155.registerNewToken", function () {
  let token, owner, other, alice;

  beforeEach(async function () {
    [owner, other, alice] = await ethers.getSigners();
    token = await (await ethers.getContractFactory("RaylsERC1155")).deploy("uri");
    await token.deployed();
  });

  it("refuses a caller without the owner role", async function () {
    await expectRevert(token, register(token.connect(other)), "AccessControl:");
    expect((await token.metadata(100)).tState).to.equal(NOT_EXIST);
  });

  it("refuses an id that is already registered and keeps its metadata and supply", async function () {
    await (await register(token)).wait();
    await (await token.mint(owner.address, 100, 1000, "0x")).wait();

    await expectRevert(token, register(token), "TokenAlreadyRegistered");
    await expectRevert(token, register(token, { fungibility: NON_FUNGIBLE, maxSupply: 0 }), "TokenAlreadyRegistered");

    const m = await token.metadata(100);
    expect(m.tState).to.equal(REGISTERED);
    expect(m.tFungibility).to.equal(FUNGIBLE);
    expect(m.maxTotalSupply.toNumber()).to.equal(1000);
    expect(m.totalSupply.toNumber()).to.equal(1000);
  });

  it("refuses to re-register a bond under another meta type, so the bond still transfers", async function () {
    await (await register(token)).wait();
    await (await token.mint(owner.address, 100, 1000, "0x")).wait();
    await (await register(token, BOND)).wait();
    const id = await bondId(token);
    expect(await bondId(token, META_TOKEN_DLC)).to.deep.equal(id);
    await (await token.mint(owner.address, id, 5, "0x")).wait();

    await expectRevert(token, register(token, { ...BOND, tType: META_TOKEN_DLC }), "TokenAlreadyRegistered");

    const m = await token.metadata(id);
    expect(m.tType).to.equal(META_TOKEN_BOND);
    expect(m.totalSupply.toNumber()).to.equal(5);
    expect(m.maxTotalSupply.toNumber()).to.equal(BOND.maxSupply);
    expect(m.subTokenIds.map(Number)).to.deep.equal(BOND.subIds);
    expect(m.subTokenValues.map(Number)).to.deep.equal(BOND.subValues);
    await (await token.safeTransferFrom(owner.address, alice.address, id, 1, "0x")).wait();
    expect((await token.balanceOf(alice.address, id)).toNumber()).to.equal(1);
  });

  it("refuses a meta token as another token's sub-token and leaves its type alone", async function () {
    await (await register(token)).wait();
    await (await register(token, BOND)).wait();
    const id = await bondId(token);

    await expectRevert(
      token,
      register(token, { tType: META_TOKEN_BOND, name: "Wrap", symbol: "W", offchainId: 8, maxSupply: 10, subIds: [id], subValues: [1] }),
      "InvalidSubToken",
    );
    expect((await token.metadata(id)).tType).to.equal(META_TOKEN_BOND);
  });

  it("still backs a bond with a token that was minted before the bond was registered", async function () {
    await (await token.mint(owner.address, 100, 1000, "0x")).wait();
    expect((await token.metadata(100)).tState).to.equal(REGISTERED);

    await (await register(token, BOND)).wait();
    const id = await bondId(token);
    expect((await token.metadata(100)).tType).to.equal(SUB_TOKEN);

    await (await token.mint(owner.address, id, 5, "0x")).wait();
    expect((await token.balanceOf(owner.address, id)).toNumber()).to.equal(5);
    expect((await token.balanceOf(owner.address, 100)).toNumber()).to.equal(950);
  });
});

// safeBatchTransferFrom writes no token state and refuses id 0, as
// safeTransferFrom does (#445).
describe("RaylsERC1155.safeBatchTransferFrom", function () {
  let token, owner, other, alice;

  beforeEach(async function () {
    [owner, other, alice] = await ethers.getSigners();
    token = await (await ethers.getContractFactory("RaylsERC1155")).deploy("uri");
    await token.deployed();
  });

  it("leaves an unregistered id NOT_EXIST after a zero-value batch, so the owner can still register it", async function () {
    await (await token.connect(other).safeBatchTransferFrom(other.address, other.address, [100], [0], "0x")).wait();
    expect((await token.metadata(100)).tState).to.equal(NOT_EXIST);

    await (await register(token)).wait();
    expect((await token.metadata(100)).tState).to.equal(REGISTERED);
  });

  it("refuses id 0", async function () {
    await expectRevert(token, token.safeBatchTransferFrom(owner.address, alice.address, [100, 0], [0, 0], "0x"), "ZeroIdNotAllowed");
  });

  it("moves balances of registered NORMAL and bond tokens", async function () {
    await (await register(token)).wait();
    await (await token.mint(owner.address, 100, 1000, "0x")).wait();
    await (await register(token, BOND)).wait();
    const id = await bondId(token);
    await (await token.mint(owner.address, id, 5, "0x")).wait();

    await (await token.safeBatchTransferFrom(owner.address, alice.address, [100, id], [30, 2], "0x")).wait();

    expect((await token.balanceOf(alice.address, 100)).toNumber()).to.equal(30);
    expect((await token.balanceOf(owner.address, 100)).toNumber()).to.equal(920);
    expect((await token.balanceOf(alice.address, id)).toNumber()).to.equal(2);
    expect((await token.balanceOf(owner.address, id)).toNumber()).to.equal(3);
    expect((await token.metadata(100)).tState).to.equal(REGISTERED);
    expect((await token.metadata(id)).tState).to.equal(REGISTERED);
  });
});
