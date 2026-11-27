const assert = require("node:assert/strict");
const { utils } = require("ethers");
const { expectRevert } = require("./helpers.js");

const abi = new utils.Interface(["error WrongAmount(uint256 amount)", "error WrongReceiver()"]);
const contract = { interface: abi };
const amountError = abi.encodeErrorResult("WrongAmount", [12]);

function reasonError(reason) {
  const encoded = utils.defaultAbiCoder.encode(["string"], [reason]);
  return Object.assign(new Error("execution reverted"), { data: `0x08c379a0${encoded.slice(2)}` });
}

describe("expectRevert", function () {
  it("decodes a custom error from structured revert data", async function () {
    for (const fields of [
      { data: amountError },
      { data: { data: amountError } },
      { error: { data: amountError } },
      { data: "0x", error: { data: { data: amountError } } },
    ]) {
      const error = Object.assign(new Error("execution reverted"), fields);
      await expectRevert(contract, Promise.reject(error), "WrongAmount");
    }
  });

  it("decodes a custom error from Hardhat's return data text", async function () {
    const error = new Error(`unrecognized custom error (return data: ${amountError})`);
    await expectRevert(contract, Promise.reject(error), "WrongAmount");
  });

  it("matches the prefix of an ABI-encoded revert reason after waiting", async function () {
    const tx = { wait: async () => { throw reasonError("AccessControl: missing role"); } };
    await expectRevert(contract, Promise.resolve(tx), "AccessControl:");
  });

  it("matches the prefix of a revert reason in Hardhat's message", async function () {
    const error = new Error("VM Exception while processing transaction: reverted with reason string 'AccessControl: missing role'");
    await expectRevert(contract, Promise.reject(error), "AccessControl:");
  });

  it("rejects a successful transaction", async function () {
    const tx = { wait: async () => ({ status: 1 }) };
    await assert.rejects(expectRevert(contract, Promise.resolve(tx), "WrongAmount"), /expected a revert with WrongAmount/);
  });

  it("rejects undecodable data even when the message names the expected error", async function () {
    const error = Object.assign(new Error("reverted with custom error 'WrongAmount(12)'"), { data: "0x11223344" });
    await assert.rejects(expectRevert(contract, Promise.reject(error), "WrongAmount"), /expected a revert with WrongAmount/);
  });

  it("rejects a different custom error", async function () {
    const error = Object.assign(new Error("execution reverted"), { data: abi.encodeErrorResult("WrongReceiver") });
    await assert.rejects(expectRevert(contract, Promise.reject(error), "WrongAmount"), /expected a revert with WrongAmount/);
  });

  it("rejects a reason that contains the expected text later in the string", async function () {
    const error = reasonError("Registry: AccessControl: missing role");
    await assert.rejects(expectRevert(contract, Promise.reject(error), "AccessControl:"), /expected a revert with AccessControl:/);
  });
});
