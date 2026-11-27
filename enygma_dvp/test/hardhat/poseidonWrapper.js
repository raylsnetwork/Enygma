// Helper for the specs in this directory, not a spec itself.
const { ethers, artifacts } = require("hardhat");
const { POSEIDON_SOURCE } = require("../../scripts/regen_poseidon.js");

// Deploys a Poseidon library from its abi and bytecode: the artifacts the
// compile hook writes for these libraries lack deployedLinkReferences, which
// getContractFactory(name) wants.
async function deployPoseidonLibrary(name) {
  const artifact = await artifacts.readArtifact(`${POSEIDON_SOURCE}:${name}`);
  return (await ethers.getContractFactory(artifact.abi, artifact.bytecode)).deploy();
}

// A PoseidonWrapper linked to freshly deployed PoseidonT3 and PoseidonT5.
async function deployPoseidonWrapper() {
  const poseidonT3 = await deployPoseidonLibrary("PoseidonT3");
  const poseidonT5 = await deployPoseidonLibrary("PoseidonT5");
  return (
    await ethers.getContractFactory("PoseidonWrapper", {
      libraries: {
        [`${POSEIDON_SOURCE}:PoseidonT3`]: poseidonT3.address,
        [`${POSEIDON_SOURCE}:PoseidonT5`]: poseidonT5.address,
      },
    })
  ).deploy();
}

module.exports = { deployPoseidonWrapper };
