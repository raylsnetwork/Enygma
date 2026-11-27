// EnygmaDvp's owner role can move: its deployer also holds DEFAULT_ADMIN_ROLE,
// which administers DEFAULT_OWNER_ROLE, so the deployer can hand both to a
// Safe or a TimelockController and renounce its own. The coin vaults' roles
// keep no admin, so a vault stays bound to the EnygmaDvp that registered it.
// Every owner call with no event of its own but registerPrivateMintVerifier
// emits OwnerCall, and the two fixed fees refuse an amount no proof can
// carry.
const { expect } = require("chai");
const { ethers, network } = require("hardhat");
const { expectRevert } = require("./expectRevert.js");
const { OWNER_ROLE, ADMIN_ROLE, deployPoseidonWrapper, deployDvpAndVerifier, testKey } = require("./deployDvp.js");

const TREE_DEPTH = 8;

// Resolves when tx reverts with OpenZeppelin AccessControl's missing-role
// reason for account and role.
async function expectMissingRole(tx, account, role) {
  try {
    await (await tx).wait();
  } catch (e) {
    const want = `AccessControl: account ${account.toLowerCase()} is missing role ${role}`;
    if (e.message.includes(want)) return;
    expect.fail(`expected "${want}", got: ${e.message}`);
  }
  expect.fail(`expected a revert: ${account} is missing role ${role}`);
}

// The OwnerCall events dvp emitted in receipt.
function ownerCalls(dvp, receipt) {
  return receipt.logs
    .filter((l) => l.address === dvp.address)
    .map((l) => dvp.interface.parseLog(l))
    .filter((e) => e.name === "OwnerCall");
}

describe("EnygmaDvp owner role", function () {
  let deployer, safe, other, dvp;

  beforeEach(async function () {
    [deployer, safe, other] = await ethers.getSigners();
    const wrapper = await deployPoseidonWrapper();
    dvp = await (await ethers.getContractFactory("EnygmaDvp")).deploy(wrapper.address, ethers.constants.AddressZero);
  });

  it("is administered by DEFAULT_ADMIN_ROLE, which the deployer holds", async function () {
    expect(await dvp.getRoleAdmin(OWNER_ROLE)).to.equal(ADMIN_ROLE);
    expect(await dvp.hasRole(OWNER_ROLE, deployer.address)).to.equal(true);
    expect(await dvp.hasRole(ADMIN_ROLE, deployer.address)).to.equal(true);
  });

  it("moves to another account, and the deployer keeps no role after renouncing", async function () {
    await (await dvp.grantRole(OWNER_ROLE, safe.address)).wait();
    await (await dvp.grantRole(ADMIN_ROLE, safe.address)).wait();
    await (await dvp.renounceRole(OWNER_ROLE, deployer.address)).wait();
    await (await dvp.renounceRole(ADMIN_ROLE, deployer.address)).wait();

    await (await dvp.connect(safe).setRelayerFixedFee(5)).wait();
    expect((await dvp.relayerFixedFeeAmount()).toNumber()).to.equal(5);
    await expectMissingRole(dvp.setRelayerFixedFee(6), deployer.address, OWNER_ROLE);
    await expectMissingRole(dvp.grantRole(OWNER_ROLE, deployer.address), deployer.address, ADMIN_ROLE);

    // The new admin can hand the role on, and take it back.
    await (await dvp.connect(safe).grantRole(OWNER_ROLE, other.address)).wait();
    await (await dvp.connect(safe).revokeRole(OWNER_ROLE, other.address)).wait();
    expect(await dvp.hasRole(OWNER_ROLE, other.address)).to.equal(false);
  });

  it("can be held by an OpenZeppelin TimelockController, whose calls wait out its delay", async function () {
    const delay = 3600;
    const timelock = await (await ethers.getContractFactory("TimelockController")).deploy(
      delay, [deployer.address], [deployer.address], ethers.constants.AddressZero);
    await (await dvp.grantRole(OWNER_ROLE, timelock.address)).wait();
    await (await dvp.grantRole(ADMIN_ROLE, timelock.address)).wait();
    await (await dvp.renounceRole(OWNER_ROLE, deployer.address)).wait();
    await (await dvp.renounceRole(ADMIN_ROLE, deployer.address)).wait();

    const data = dvp.interface.encodeFunctionData("setRelayerFixedFee", [7]);
    const salt = ethers.utils.id("set the relayer fee to 7");
    const op = [dvp.address, 0, data, ethers.constants.HashZero, salt];
    await (await timelock.schedule(...op, delay)).wait();
    try {
      await (await timelock.execute(...op)).wait();
      expect.fail("the timelock executed before its delay");
    } catch (e) {
      expect(e.message).to.include("TimelockController: operation is not ready");
    }

    await network.provider.send("evm_increaseTime", [delay]);
    await network.provider.send("evm_mine");
    const receipt = await (await timelock.execute(...op)).wait();
    expect((await dvp.relayerFixedFeeAmount()).toNumber()).to.equal(7);
    const [call] = ownerCalls(dvp, receipt);
    expect(call.args.caller).to.equal(timelock.address);
    expect(call.args.data).to.equal(data);
  });

  it("leaves a coin vault's roles with no admin: the vault stays bound to its EnygmaDvp", async function () {
    const vault = await (await ethers.getContractFactory("Erc20CoinVault")).deploy(dvp.address);
    const dvpRole = await vault.DEFAULT_DVP_ROLE();
    expect(await vault.getRoleAdmin(dvpRole)).to.equal(ADMIN_ROLE);
    expect(await vault.hasRole(ADMIN_ROLE, deployer.address)).to.equal(false);
    expect(await vault.hasRole(ADMIN_ROLE, dvp.address)).to.equal(false);
    await expectMissingRole(vault.grantRole(dvpRole, other.address), deployer.address, ADMIN_ROLE);
  });
});

describe("EnygmaDvp owner calls with no event of their own emit OwnerCall", function () {
  // The deployment the calls below act on, filled in by before().
  const c = {};

  before(async function () {
    [c.deployer, c.other] = await ethers.getSigners();
    ({ dvp: c.dvp, verifier: c.verifier } = await deployDvpAndVerifier(c.deployer));
    c.vault = await (await ethers.getContractFactory("Erc20CoinVault")).deploy(c.dvp.address);
    c.auction = await (await ethers.getContractFactory("EnygmaAuction")).deploy(c.dvp.address);
    const AssetGroup = await ethers.getContractFactory("AssetGroup");
    c.groups = [];
    for (let i = 0; i < 3; i++) c.groups.push(await AssetGroup.deploy(c.dvp.address));
  });

  // Each owner call with no event of its own, and its arguments. In this
  // order each call succeeds: the vault before its USDr pin, the groups
  // before their pairs (groups 0 and 2 are fungible, group 1 is not).
  const CALLS = [
    ["initializeDvp", () => [c.verifier.address]],
    ["registerEnygmaAuction", () => [c.auction.address]],
    ["registerRelayer", () => [c.other.address]],
    ["deregisterRelayer", () => [c.other.address]],
    ["registerNewVerificationKey", () => [testKey(1, 2)]],
    ["registerAssetGroup", () => [c.groups[0].address, "fungible", true, TREE_DEPTH]],
    ["registerAssetGroup", () => [c.groups[1].address, "non-fungible", false, TREE_DEPTH]],
    ["registerAssetGroup", () => [c.groups[2].address, "fungible 2", true, TREE_DEPTH]],
    ["registerVault", () => [c.vault.address, c.other.address, 1, TREE_DEPTH]],
    ["setRelayerFixedFee", () => [3]],
    ["setUsdrFixedFee", () => [4]],
    ["setUsdrTokenId", () => [0]],
    ["setUsdrFeeVaultId", () => [0]],
    ["registerSwapGroupPair", () => [0, 1]],
    ["registerExchangeGroupPair", () => [0, 2]],
    ["registerVaultSwapPair", () => [0, 1]],
  ];

  CALLS.forEach(([name, args], i) => {
    it(`${name} (call ${i + 1}) refuses a caller without the owner role, and emits OwnerCall`, async function () {
      await expectMissingRole(c.dvp.connect(c.other)[name](...args()), c.other.address, OWNER_ROLE);

      const tx = await c.dvp[name](...args());
      const events = ownerCalls(c.dvp, await tx.wait());
      expect(events.length, `${name} emitted ${events.length} OwnerCall events`).to.equal(1);
      expect(events[0].args.caller).to.equal(c.deployer.address);
      expect(events[0].args.selector).to.equal(c.dvp.interface.getSighash(name));
      expect(events[0].args.data).to.equal(tx.data);
    });
  });
});

describe("EnygmaDvp fixed fees", function () {
  let dvp, max;

  before(async function () {
    const [deployer] = await ethers.getSigners();
    ({ dvp } = await deployDvpAndVerifier(deployer));
    const vault = await (await ethers.getContractFactory("Erc20CoinVault")).deploy(dvp.address);
    // The circuits' bound on every note value, as the vaults hold it.
    max = await vault.MAX_FEE_AMOUNT();
  });

  for (const [setter, getter] of [["setRelayerFixedFee", "relayerFixedFeeAmount"], ["setUsdrFixedFee", "usdrFixedFeeAmount"]]) {
    it(`${setter} takes amounts up to Erc20CoinVault.MAX_FEE_AMOUNT - 1 and refuses MAX_FEE_AMOUNT`, async function () {
      await (await dvp[setter](max.sub(1))).wait();
      expect((await dvp[getter]()).toString()).to.equal(max.sub(1).toString());

      await expectRevert(dvp, dvp[setter](max), "FeeOutOfRange");
      await expectRevert(dvp, dvp[setter](ethers.constants.MaxUint256), "FeeOutOfRange");
      expect((await dvp[getter]()).toString()).to.equal(max.sub(1).toString());

      await (await dvp[setter](0)).wait();
      expect((await dvp[getter]()).toNumber()).to.equal(0);
    });
  }
});
