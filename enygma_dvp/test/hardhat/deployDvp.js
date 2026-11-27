// Helper for the specs in this directory, not a spec itself: deploys
// EnygmaDvp with a real PoseidonWrapper (a vault's or an asset group's Merkle
// tree hashes with it) and the Verifier it registers keys with, and builds
// test verifying keys.
const { ethers, artifacts } = require("hardhat");
const { POSEIDON_SOURCE } = require("../../scripts/regen_poseidon.js");

const OWNER_ROLE = ethers.utils.keccak256(ethers.utils.toUtf8Bytes("ownerRole"));
const ADMIN_ROLE = ethers.constants.HashZero;

// Deploys a Poseidon library from its abi and bytecode: the artifacts the
// compile hook writes for these libraries lack deployedLinkReferences, which
// getContractFactory(name) wants.
async function deployPoseidonLibrary(name) {
  const artifact = await artifacts.readArtifact(`${POSEIDON_SOURCE}:${name}`);
  return (await ethers.getContractFactory(artifact.abi, artifact.bytecode)).deploy();
}

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

// EnygmaDvp and a Verifier, both deployed by deployer, wired as init_payment
// wires them: EnygmaDvp holds the Verifier's owner role and points at it.
async function deployDvpAndVerifier(deployer) {
  const wrapper = await deployPoseidonWrapper();
  const dvp = await (await ethers.getContractFactory("EnygmaDvp", deployer)).deploy(wrapper.address, ethers.constants.AddressZero);
  const verifier = await (await ethers.getContractFactory("Verifier", deployer)).deploy();
  await (await verifier.grantRole(OWNER_ROLE, dvp.address)).wait();
  await (await dvp.initializeDvp(verifier.address)).wait();
  return { dvp, verifier, wrapper };
}

// testKey mirrors enygma_vkslots' testKey(seed, nPublic): a key with
// nPublic + 1 IC points, every coordinate distinct and counted up from
// seed * 1000, in the order the Verifier stores them. No real curve points:
// the Verifier only stores a key.
function testKey(seed, nPublic) {
  let n = seed * 1000;
  const next = () => ++n;
  const g1 = () => ({ x: next(), y: next() });
  const g2 = () => {
    const [x0, x1, y0, y1] = [next(), next(), next(), next()];
    return { x: [x0, x1], y: [y0, y1] };
  };
  const key = { alpha1: g1(), beta2: g2(), gamma2: g2(), delta2: g2(), ic: [] };
  for (let i = 0; i <= nPublic; i++) key.ic.push(g1());
  return key;
}

// keccak256(abi.encode(vk)), vk as the Verifier's addVerificationKey takes it.
function keyHash(verifier, vk) {
  const type = verifier.interface.getFunction("addVerificationKey").inputs[0];
  return ethers.utils.keccak256(ethers.utils.defaultAbiCoder.encode([type], [vk]));
}

module.exports = { OWNER_ROLE, ADMIN_ROLE, deployPoseidonWrapper, deployDvpAndVerifier, testKey, keyHash };
