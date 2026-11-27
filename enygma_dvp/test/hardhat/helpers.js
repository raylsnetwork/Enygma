const assert = require("node:assert/strict");
const { utils } = require("ethers");

const ERROR_STRING = utils.id("Error(string)").slice(0, 10);

// Hardhat and ethers can expose the revert bytes on the error, on its
// underlying RPC error, or as return data in the diagnostic.
function matchesRevert(contract, error, name) {
  const message = error?.message ?? String(error);
  const data = [
    error?.data,
    error?.data?.data,
    error?.error?.data,
    error?.error?.data?.data,
    message.match(/return data: (0x[0-9a-f]+)/i)?.[1],
  ].find((value) => utils.isHexString(value) && value !== "0x");

  if (!data) {
    return message.includes(`reverted with reason string '${name}`);
  }
  try {
    if (data.slice(0, 10).toLowerCase() === ERROR_STRING) {
      const [reason] = utils.defaultAbiCoder.decode(["string"], `0x${data.slice(10)}`);
      return reason.startsWith(name);
    }
    return contract.interface.parseError(data).name === name;
  } catch {
    return false;
  }
}

// Accept a named custom error or a revert reason starting with name.
async function expectRevert(contract, tx, name) {
  try {
    await (await tx).wait();
  } catch (error) {
    if (matchesRevert(contract, error, name)) return;
    assert.fail(`expected a revert with ${name}, got: ${error?.message ?? error}`);
  }
  assert.fail(`expected a revert with ${name}`);
}

module.exports = { expectRevert };
