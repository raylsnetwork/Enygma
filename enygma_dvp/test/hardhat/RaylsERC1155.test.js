// RaylsERC1155.registerNewToken: only the owner registers, a registered id
// cannot be registered again, and a meta token cannot back another token (#414).
const { expect } = require("chai");
const { ethers } = require("hardhat");

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

// Resolves when tx reverts with the custom error `name`, or a revert string
// starting with `name` (OpenZeppelin's AccessControl). The error is decoded
// from the revert data with the contract's ABI: on the first run after a
// recompile, Hardhat reports "unrecognized custom error (return data: 0x…)".
async function expectRevert(token, tx, name) {
  try {
    await (await tx).wait();
  } catch (e) {
    if (e.message.includes(`reverted with reason string '${name}`)) return;
    const data = (e.message.match(/return data: (0x[0-9a-f]+)/) || e.message.match(/custom error '([A-Za-z]+)\(/) || [])[1];
    let got = data;
    if (data && data.startsWith("0x")) {
      try { got = token.interface.parseError(data).name; } catch (_) { got = undefined; }
    }
    if (got === name) return;
    expect.fail(`expected a revert with ${name}, got: ${e.message}`);
  }
  expect.fail(`expected a revert with ${name}`);
}

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
