// Helper for the specs in this directory, not a spec itself.
const { expect } = require("chai");

// Resolves when tx reverts with the custom error `name`, or a revert string
// starting with `name` (OpenZeppelin's AccessControl). The error is decoded
// from the revert data with the contract's ABI: on the first run after a
// recompile, Hardhat reports "unrecognized custom error (return data: 0x…)".
async function expectRevert(contract, tx, name) {
  try {
    await (await tx).wait();
  } catch (e) {
    if (e.message.includes(`reverted with reason string '${name}`)) return;
    const data = (e.message.match(/return data: (0x[0-9a-f]+)/) || e.message.match(/custom error '([A-Za-z]+)\(/) || [])[1];
    let got = data;
    if (data && data.startsWith("0x")) {
      try { got = contract.interface.parseError(data).name; } catch (_) { got = undefined; }
    }
    if (got === name) return;
    expect.fail(`expected a revert with ${name}, got: ${e.message}`);
  }
  expect.fail(`expected a revert with ${name}`);
}

module.exports = { expectRevert };
