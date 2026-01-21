/* global describe it ethers before hre */
const { expect } = require("chai");
const testHelpers = require("./testHelpers.js");
const utils = require("../src/core/utils");
const { PrivateMintProof } = require("../src/core/prover_gnark");
const dvpConf = require("../zkdvp.config.json");

const TREE_DEPTH = dvpConf["circom"]["meta-parameters"]["tree-depth"];

let owner;
let users;
let contracts;
let merkleTrees;
let enygmaAssetAddress;
let gnarkServerAvailable = false;

// Helper to check if gnark server is available
async function checkGnarkServer() {
  try {
    const axios = require("axios");
    // Try a dummy POST to see if server responds (will fail validation but confirms server is up)
    await axios.post(
      "http://localhost:8081/proof/privateMint",
      {},
      { timeout: 2000 },
    );
    return true;
  } catch (err) {
    // If we get a 400 (bad request), server is up but request was invalid - that's OK
    if (err.response && err.response.status === 400) {
      return true;
    }
    // Connection refused or timeout means server is not available
    return false;
  }
}

describe("PrivateMint with ZK Proof Verification", () => {
  before(async () => {
    // Check if gnark server is available
    gnarkServerAvailable = await checkGnarkServer();
    if (!gnarkServerAvailable) {
      console.log(
        "⚠️  Gnark server not available at localhost:8081. Some tests will be skipped.",
      );
    }
    // Deploy ZkDvp infrastructure
    let userCount = 2;
    [owner, users, contracts, merkleTrees] =
      await testHelpers.deployForTest(userCount);

    // Deploy and register PrivateMintVerifier
    console.log("Deploying PrivateMintVerifier...");
    const privateMintVerifierFactory = await hre.ethers.getContractFactory(
      "PrivateMintVerifier",
    );
    const privateMintVerifierContract =
      await privateMintVerifierFactory.deploy();
    await privateMintVerifierContract.deployTransaction.wait();
    contracts["privateMintVerifier"] = privateMintVerifierContract;
    console.log(
      `PrivateMintVerifier deployed to: ${privateMintVerifierContract.address}`,
    );

    // Register the PrivateMintVerifier with ZkDvp
    console.log("Registering PrivateMintVerifier with ZkDvp...");
    await contracts.zkdvp.registerPrivateMintVerifier(
      privateMintVerifierContract.address,
    );
    console.log("PrivateMintVerifier registered!");

    // Deploy and register EnygmaErc20CoinVault for testing
    const enygmaCoinVaultFactory = await hre.ethers.getContractFactory(
      "EnygmaErc20CoinVault",
    );
    const enygmaCoinVaultContract = await enygmaCoinVaultFactory.deploy(
      contracts.zkdvp.address,
    );
    await enygmaCoinVaultContract.deployTransaction.wait();
    contracts["enygmaCoinVault"] = enygmaCoinVaultContract;

    enygmaAssetAddress = contracts.erc20.address;

    // Register the Enygma vault (vaultId will be 3)
    await contracts.zkdvp.registerVault(
      enygmaCoinVaultContract.address,
      enygmaAssetAddress,
      1,
      TREE_DEPTH,
    );
    console.log("EnygmaCoinVault registered with vaultId: 3");
  });

  it(`Should generate a valid ZK proof for privateMint`, async function () {
    // Generate receiver's key pair
    const keyPair = utils.newKeyPair();
    console.log("Generated receiver key pair:");
    console.log(`  privateKey: ${keyPair.privateKey}`);
    console.log(`  publicKey: ${keyPair.publicKey}`);

    // Commitment parameters
    const tokenId = 1n; // Token ID
    const amount = 1000n;
    const salt = utils.randomInField(); // Random salt for commitment privacy
    const contractAddress = BigInt(enygmaAssetAddress);

    // Compute commitment matching the circuit:
    // erc1155uniqueId = poseidon(poseidon(contractAddress, tokenId), amount)
    // commitmentPart1 = poseidon(uniqueId, publicKey)
    // commitment = poseidon(commitmentPart1, salt)
    const uniqueId = utils.erc1155UniqueId(contractAddress, tokenId, amount);
    const commitmentPart1 = utils.getCommitment(uniqueId, keyPair.publicKey);
    const commitment = utils.getCommitment(commitmentPart1, salt);

    // Compute cipherText = poseidon(publicKey, salt)
    const cipherText = utils.getCommitment(keyPair.publicKey, salt);

    console.log(`\nCommitment parameters:`);
    console.log(`  tokenId: ${tokenId}`);
    console.log(`  amount: ${amount}`);
    console.log(`  salt: ${salt}`);
    console.log(`  commitment: ${commitment}`);
    console.log(`  cipherText: ${cipherText}`);

    // Generate ZK proof using the gnark prover
    console.log("\nGenerating ZK proof...");
    const proofResult = await PrivateMintProof({
      commitment: commitment,
      contractAddress: contractAddress,
      tokenId: tokenId,
      salt: salt,
      amount: amount,
      publicKey: keyPair.publicKey,
      cipherText: cipherText,
    });

    console.log("Proof generated successfully!");
    console.log(`  proof length: ${proofResult.proof.length}`);
    console.log(`  publicSignal length: ${proofResult.publicSignal.length}`);

    expect(proofResult.proof.length).to.equal(8);
    expect(proofResult.publicSignal.length).to.equal(4);
  });

  it(`Should call privateMint with valid ZK proof`, async function () {
    if (!gnarkServerAvailable) {
      this.skip();
      return;
    }

    // Generate receiver's key pair
    const keyPair = utils.newKeyPair();

    // Commitment parameters
    const tokenId = 1n;
    const amount = 500n;
    const salt = utils.randomInField(); // Random salt for commitment privacy
    const contractAddress = BigInt(enygmaAssetAddress);

    // Compute commitment matching the circuit:
    // commitmentPart1 = poseidon(uniqueId, publicKey)
    // commitment = poseidon(commitmentPart1, salt)
    const uniqueId = utils.erc1155UniqueId(contractAddress, tokenId, amount);
    const commitmentPart1 = utils.getCommitment(uniqueId, keyPair.publicKey);
    const commitment = utils.getCommitment(commitmentPart1, salt);

    // Compute cipherText = poseidon(publicKey, salt)
    const cipherText = utils.getCommitment(keyPair.publicKey, salt);

    console.log(`\nGenerating proof for privateMint...`);

    // Generate ZK proof
    const proofResult = await PrivateMintProof({
      commitment: commitment,
      contractAddress: contractAddress,
      tokenId: tokenId,
      salt: salt,
      amount: amount,
      publicKey: keyPair.publicKey,
      cipherText: cipherText,
    });

    // Prepare proof struct for contract call
    console.log(proofResult);
    const proofStruct = {
      proof: proofResult.proof.map((p) => BigInt(p)),
      public_signal: proofResult.publicSignal.map((p) => BigInt(p)),
    };

    const vaultId = 3; // Enygma vault

    console.log(`\nCalling privateMint with:`);
    console.log(`  vaultId: ${vaultId}`);
    console.log(`  commitment: ${commitment}`);
    console.log(`  cipherText: ${cipherText}`);

    // Call privateMint with proof (tokenId removed from function signature)
    const tx = await contracts.zkdvp.privateMint(
      vaultId,
      commitment,
      proofStruct,
    );
    const receipt = await tx.wait();

    // Find PrivateMint event
    const privateMintEvent = receipt.events.find(
      (ev) => ev.event === "PrivateMint",
    );

    expect(privateMintEvent).to.not.be.undefined;
    expect(privateMintEvent.args.vaultId.toBigInt()).to.equal(BigInt(vaultId));
    expect(privateMintEvent.args.commitment.toBigInt()).to.equal(commitment);
    expect(privateMintEvent.args.cipherText.toBigInt()).to.equal(cipherText);

    console.log("\n✓ PrivateMint with ZK proof verification successful!");
  });

  it(`Should add commitment to merkle tree after verified privateMint`, async function () {
    if (!gnarkServerAvailable) {
      this.skip();
      return;
    }

    // Get root before mint
    const rootBefore = await contracts.enygmaCoinVault.getRoot();
    console.log(`Merkle root before: ${rootBefore}`);

    // Generate commitment
    const keyPair = utils.newKeyPair();
    const tokenId = 1n;
    const amount = 2000n;
    const salt = utils.randomInField(); // Random salt for commitment privacy
    const contractAddress = BigInt(enygmaAssetAddress);

    // Compute commitment matching the circuit:
    // commitmentPart1 = poseidon(uniqueId, publicKey)
    // commitment = poseidon(commitmentPart1, salt)
    const uniqueId = utils.erc1155UniqueId(contractAddress, tokenId, amount);
    const commitmentPart1 = utils.getCommitment(uniqueId, keyPair.publicKey);
    const commitment = utils.getCommitment(commitmentPart1, salt);

    // Compute cipherText = poseidon(publicKey, salt)
    const cipherText = utils.getCommitment(keyPair.publicKey, salt);

    // Generate proof
    const proofResult = await PrivateMintProof({
      commitment: commitment,
      contractAddress: contractAddress,
      tokenId: tokenId,
      salt: salt,
      amount: amount,
      publicKey: keyPair.publicKey,
      cipherText: cipherText,
    });

    const proofStruct = {
      proof: proofResult.proof.map((p) => BigInt(p)),
      public_signal: proofResult.publicSignal.map((p) => BigInt(p)),
    };

    // Call privateMint (tokenId removed from function signature)
    await contracts.zkdvp.privateMint(3, commitment, proofStruct);

    // Get root after mint
    const rootAfter = await contracts.enygmaCoinVault.getRoot();
    console.log(`Merkle root after: ${rootAfter}`);

    // Root should change after inserting new commitment
    expect(rootAfter).to.not.equal(rootBefore);
    console.log("✓ Merkle root updated after verified privateMint!");
  });

  it(`Should fail privateMint with invalid proof`, async function () {
    const commitment = utils.randomInField();
    const cipherText = utils.randomInField();

    // Create an invalid proof (random values that won't verify)
    // Public signals order: [commitment, contractAddress, tokenId, cipherText]
    const invalidProofStruct = {
      proof: [
        1n,
        2n, // Invalid A point
        3n,
        4n,
        5n,
        6n, // Invalid B point
        7n,
        8n, // Invalid C point
      ],
      public_signal: [commitment, 0n, 0n, cipherText],
    };

    let hasReverted = false;
    try {
      await contracts.zkdvp.privateMint(3, commitment, invalidProofStruct);
    } catch (err) {
      hasReverted = true;
      console.log("✓ Transaction reverted as expected for invalid proof");
    }

    expect(hasReverted).to.be.true;
  });

  it(`Should fail if called by non-owner`, async function () {
    const commitment = utils.randomInField();
    const cipherText = utils.randomInField();

    // Use a dummy proof struct (doesn't matter since access control fails first)
    // Public signals order: [commitment, contractAddress, tokenId, cipherText]
    const proofStruct = {
      proof: [1n, 2n, 3n, 4n, 5n, 6n, 7n, 8n],
      public_signal: [commitment, 0n, 0n, cipherText],
    };

    // Connect as non-owner (alice)
    const alice = users[0].wallet;
    const zkdvpAsAlice = contracts.zkdvp.connect(alice);

    let hasReverted = false;
    try {
      await zkdvpAsAlice.privateMint(3, commitment, proofStruct);
    } catch (err) {
      hasReverted = true;
      console.log("✓ Transaction reverted as expected for non-owner");
    }

    expect(hasReverted).to.be.true;
  });

  it(`Should handle multiple privateMints with proofs correctly`, async function () {
    if (!gnarkServerAvailable) {
      this.skip();
      return;
    }

    const mints = [];

    console.log("\nPreparing multiple privateMints with proofs...");

    // Prepare 3 different mints
    for (let i = 0; i < 3; i++) {
      const keyPair = utils.newKeyPair();
      const tokenId = BigInt(i + 1); // Different token IDs
      const amount = BigInt((i + 1) * 100);
      const salt = utils.randomInField(); // Random salt for commitment privacy
      const contractAddress = BigInt(enygmaAssetAddress);

      // Compute commitment matching the circuit:
      // commitmentPart1 = poseidon(uniqueId, publicKey)
      // commitment = poseidon(commitmentPart1, salt)
      const uniqueId = utils.erc1155UniqueId(contractAddress, tokenId, amount);
      const commitmentPart1 = utils.getCommitment(uniqueId, keyPair.publicKey);
      const commitment = utils.getCommitment(commitmentPart1, salt);

      // Compute cipherText = poseidon(publicKey, salt)
      const cipherText = utils.getCommitment(keyPair.publicKey, salt);

      // Generate proof
      const proofResult = await PrivateMintProof({
        commitment: commitment,
        contractAddress: contractAddress,
        tokenId: tokenId,
        salt: salt,
        amount: amount,
        publicKey: keyPair.publicKey,
        cipherText: cipherText,
      });

      mints.push({
        commitment,
        cipherText,
        amount,
        proofStruct: {
          proof: proofResult.proof.map((p) => BigInt(p)),
          public_signal: proofResult.publicSignal.map((p) => BigInt(p)),
        },
      });
    }

    console.log("\nMinting multiple commitments with proofs:");
    for (const mint of mints) {
      const tx = await contracts.zkdvp.privateMint(
        3,
        mint.commitment,
        mint.proofStruct,
      );
      const receipt = await tx.wait();

      const privateMintEvent = receipt.events.find(
        (ev) => ev.event === "PrivateMint",
      );
      expect(privateMintEvent.args.commitment.toBigInt()).to.equal(
        mint.commitment,
      );
      expect(privateMintEvent.args.cipherText.toBigInt()).to.equal(
        mint.cipherText,
      );

      console.log(`  ✓ Minted amount=${mint.amount} with verified proof`);
    }

    console.log(
      `\n✓ Successfully minted ${mints.length} private commitments with ZK proof verification`,
    );
  });
});
