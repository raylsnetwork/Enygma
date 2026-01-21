/* global describe it ethers before beforeEach afterEach overwriteArtifact */
const { expect } = require("chai");
const poseidonGenContract = require("circomlibjs/src/poseidon_gencontract");
const prover = require("../src/core/prover");
const utils = require("../src/core/utils");
const MerkleTree = require("../src/core/merkle");
const { getVerificationKeys } = require("../src/core/dvpSnarks");

const dvpConf = require("../zkdvp.config.json");
const TREE_DEPTH = dvpConf["tree-depth"];

let aliceCoins = [];
let bobCoins = [];

let owner;
let users;
let contracts;
let merkleTrees;

let alice;
let bob;
let merkleTree721;
let merkleTree20;

let nftKeyDeposit;

const testHelpers = require("./testHelpers.js");
const NFT_ID = 9999n;

describe("ZkDvp Erc20-Erc721 Swap testing", () => {
  it(`ZkDvp should initialize properly `, async () => {
    let userCount = 2;
    [owner, users, contracts, merkleTrees] = await testHelpers.deployForTest(
      userCount
    );

    console.log("SwapTest: ZkDvp initialization");
    console.log("User: ", users);
    console.log("MerkleTree: ", merkleTrees);

    alice = users[0].wallet;
    bob = users[1].wallet;

    // let nftAlice = contracts["erc721"].connect(alice);

    merkleTree721 = merkleTrees["ERC721"].tree;
    merkleTree20 = merkleTrees["ERC20"].tree;
  });

  it("Alice should be able to deposit her ERC721 token.", async () => {
    const erc721Contract = contracts["erc721"];
    const erc721Alice = erc721Contract.connect(alice);
    const erc721Owner = erc721Contract.connect(owner);
    const vaultContract = contracts["erc721CoinVault"];
    const vaultAlice = vaultContract.connect(alice);

    nftKeyDeposit = utils.newKeyPair();

    // Mint NFT for Alice
    await erc721Owner.mint(alice.address, NFT_ID);
    // Approve ZkDvp as an operator for Alice's NFT
    await erc721Alice.approve(vaultContract.address, NFT_ID);
    // Deposit Alice's NFT into ZkDvp
    console.log("Alice deposits ERC721 coin.");

    let tx = await vaultAlice.deposit([NFT_ID, nftKeyDeposit.publicKey]);

    let cmt = await testHelpers.getCommitmentFromTx(tx);
    merkleTree721.insertLeaves([cmt]);
    aliceCoins.push({
      commitment: cmt,
      proof: merkleTree721.generateProof(cmt),
      root: merkleTree721.root,
      treeNumber: merkleTree721.lastTreeNumber,
    });
  });

  it("Alice should be able to prove Bob the Ownership of the deposited ERC721 token.", async () => {
    console.log("Alice wants to prove to Bob that she owns the NFT.");

    const erc721Contract = contracts["erc721"];
    const erc721Alice = erc721Contract.connect(alice);
    const zkDvpContract = contracts["zkdvp"];
    const zkDvpAlice = zkDvpContract.connect(alice);
    const vaultContract = contracts["erc721CoinVault"];
    const vaultAlice = vaultContract.connect(alice);

    // Bob generate a fresh challenge
    console.log("Bob chooses a random fresh challenge and sends it to Alice.");
    const challenge = utils.randomInField();
    console.log("Challenge = ", challenge);
    console.log(
      "[TODO] you can make it non-interactive using a random oracle."
    );

    // Alice constructs a proof of ownership    and put message = challenge
    console.log(
      "Alice constructs a proof with output address = 0 and message = challenge"
    );
    const nftUniqueId = utils.erc721UniqueId(erc721Contract.address, NFT_ID);

    const verifyOwnProof = await prover.prove("OwnershipErc721", {
      message: challenge,
      values: [NFT_ID],
      keysIn: [nftKeyDeposit],
      keysOut: [{ publicKey: 0n }],
      merkleProofs: [aliceCoins[0]["proof"]],
      treeNumbers: [aliceCoins[0]["treeNumber"]],
      erc721ContractAddress: erc721Contract.address,
    });

    console.log(
      "Alice calls vaultErc721.verifyOwnership() with the constructed proof and coin's asttributes."
    );

    // TODO:: should be relayed.
    tx = await vaultAlice.verifyOwnership([NFT_ID], verifyOwnProof);

    // Bob listens to VerifyOwnershipReceipt events
    const verifyRc = await tx.wait();
    const verifyEvent = verifyRc.events.find(
      (ev) => ev.event === "OwnershipVerificationReceipt"
    );

    const verifyTokenId = verifyEvent.args.tokenId;
    const verifyAmount = verifyEvent.args.amount;
    const verifyVaultId = verifyEvent.args.vaultId;
    const verifyChallenge = verifyEvent.args.challenge;

    console.log("Bob Verifies the OwnershipVerificationReceipt event.");

    expect(verifyTokenId).to.equal(NFT_ID);
    expect(verifyAmount).to.equal(1n);
    expect(verifyChallenge).to.equal(challenge);
    expect(verifyVaultId).to.equal(1n); // erc721 vaultId = 1

    console.log("Bob verified the token's attributes and the challenge.");
  });

  it("Alice should swap her NFT for payment from Bob", async () => {
    const erc721Contract = contracts["erc721"];
    const erc721Alice = erc721Contract.connect(alice);
    const erc721VaultContract = contracts["erc721CoinVault"];
    const erc20VaultContract = contracts["erc20CoinVault"];
    const bobVault20 = erc20VaultContract.connect(bob);
    const aliceVault721 = erc721VaultContract.connect(alice);
    const zkDvpContract = contracts["zkdvp"];
    const zkDvpAlice = zkDvpContract.connect(alice);
    const zkDvpBob = zkDvpContract.connect(bob);
    const erc20Contract = contracts["erc20"];
    const erc20Bob = erc20Contract.connect(bob);
    const erc20Owner = erc20Contract.connect(owner);
    // Bob deposit 2x10 ethers into ZkDvp
    const depositAmount = 10n;
    // Alice generates NFT commitment for Bob

    // minting ERC20 tokens for Bob
    tx = await erc20Owner.mint(bob.address, depositAmount * 2n);
    await tx.wait();

    // Approve ZkDvp to transfer tokens
    await erc20Bob.approve(erc20VaultContract.address, depositAmount * 2n);

    console.log("Bob deposits two ERC20 coins");
    // creating depositKeys for Bob to Deposit Erc20 tokens to ZkDvp
    const fundKeys = [utils.newKeyPair(), utils.newKeyPair()];
    tx = await bobVault20.deposit([depositAmount, fundKeys[0].publicKey]);
    cmt2 = await testHelpers.getCommitmentFromTx(tx);
    merkleTree20.insertLeaves([cmt2]);
    bobCoins.push({
      commitment: cmt2,
      proof: merkleTree20.generateProof(cmt2),
      root: merkleTree20.root,
      treeNumber: merkleTree20.lastTreeNumber,
    });
    tx = await bobVault20.deposit([depositAmount, fundKeys[1].publicKey]);
    cmt3 = await testHelpers.getCommitmentFromTx(tx);
    merkleTree20.insertLeaves([cmt3]);
    bobCoins.push({
      commitment: cmt3,
      proof: merkleTree20.generateProof(cmt3),
      root: merkleTree20.root,
      treeNumber: merkleTree20.lastTreeNumber,
    });
    // Alice generates NFT commitment for Bob
    const uid = utils.erc721UniqueId(erc721Contract.address, NFT_ID);

    // Bob generates a public key to receive the NFT
    const bobNFTKey = utils.newKeyPair();
    // Bob generates a public to receive the change
    const bobChangeKey = utils.newKeyPair();
    // Alice generates a public key to receive the payment
    const alicePaymentKey = utils.newKeyPair();

    // nftCommitment will be used as a massage by Bob
    const nftCommitment = utils.getCommitment(uid, bobNFTKey.publicKey);

    // Bob generates payment commitment for Alice
    const paymentAmount = 15n;
    const changeAmount = depositAmount * 2n - paymentAmount;

    // creating unique erc20Commitment
    const erc20Uid = utils.erc20UniqueId(erc20Contract.address, paymentAmount);

    // paymentCommitment will be used as a massage by Alice
    const paymentCommitment = utils.getCommitment(
      erc20Uid,
      alicePaymentKey.publicKey
    );

    // Alice generates a tx to send her NFT to Bob
    const ownParams = await prover.prove("OwnershipErc721", {
      message: paymentCommitment,
      values: [NFT_ID],
      keysIn: [nftKeyDeposit],
      keysOut: [bobNFTKey],
      merkleProofs: [aliceCoins[0]["proof"]],
      treeNumbers: [aliceCoins[0]["treeNumber"]],
      erc721ContractAddress: erc721Contract.address,
    });

    // Bob generates a tx to send payment to Alice
    const jsParams = await prover.prove("JoinSplitErc20", {
      message: nftCommitment,
      valuesIn: [depositAmount, depositAmount],
      keysIn: fundKeys,
      valuesOut: [paymentAmount, changeAmount],
      keysOut: [alicePaymentKey, bobChangeKey],
      merkleProofs: [bobCoins[0]["proof"], bobCoins[1]["proof"]],
      treeNumbers: [bobCoins[0]["treeNumber"], bobCoins[1]["treeNumber"]],
      erc20ContractAddress: erc20Contract.address,
    });

    console.log("SWAPPING");
    // A relayer forwards both transactions to ZkDvp

    // instead of swapping, we will submit proofs one by one

    tx = await zkDvpContract.submitPartialSettlement(jsParams, 0, 0);
    console.log("Payment partial settlement has been submitted to ZkDvp.");

    tx = await zkDvpContract.submitPartialSettlement(ownParams, 1, 1);
    console.log("Delivery partial settlement has been submitted to ZkDvp.");

    const rc = await tx.wait();

    console.log("Done swap");
    // inserting new commitments to local merkleTree
    const commitments = [
      jsParams.statement[7],
      jsParams.statement[8],
      ownParams.statement[4],
    ];
    merkleTree20.insertLeaves([commitments[0], commitments[1]]);
    aliceCoins.push({
      commitment: commitments[0],
      treeNumber: merkleTree20.lastTreeNumber,
      proof: merkleTree20.generateProof(commitments[0]),
      root: merkleTree20.root,
    });
    bobCoins.push({
      commitment: commitments[1],
      treeNumber: merkleTree20.lastTreeNumber,
      proof: merkleTree20.generateProof(commitments[1]),
      root: merkleTree20.root,
    });

    merkleTree721.insertLeaves([commitments[2]]);
    bobCoins.push({
      commitment: commitments[2],
      treeNumber: merkleTree721.lastTreeNumber,
      proof: merkleTree721.generateProof(commitments[2]),
      root: merkleTree721.root,
    });
    // Alice withdraws her payment from ZkDvp
    // dummyKey to be used as another input
    console.log("Done swap, Alice withdraws fund");

    const dummyKey = utils.newKeyPair();
    // Bob generates a tx to send payment to Alice
    const aliceJS = await prover.prove("JoinSplitErc20", {
      message: 0n,
      valuesIn: [paymentAmount, 0n],
      keysIn: [alicePaymentKey, dummyKey],
      valuesOut: [paymentAmount, 0n],
      keysOut: [{ publicKey: BigInt(alice.address) }, dummyKey],
      merkleProofs: [aliceCoins[1]["proof"], { root: 0n }],
      treeNumbers: [aliceCoins[1]["treeNumber"], 0n],
      erc20ContractAddress: erc20Contract.address,
    });

    const oldBalance = (
      await erc20Contract.balanceOf(alice.address)
    ).toBigInt();

    await erc20VaultContract.withdraw([paymentAmount], alice.address, aliceJS);
    const newBalance = (
      await erc20Contract.balanceOf(alice.address)
    ).toBigInt();
    await expect(oldBalance + paymentAmount).to.equal(newBalance);

    console.log("Bob withdraws NFT");
    // Bob withdraws his NFT from ZkDvp
    const bobNFT = await prover.prove("OwnershipErc721", {
      message: 0n,
      values: [NFT_ID],
      keysIn: [bobNFTKey],
      keysOut: [{ publicKey: BigInt(bob.address) }],
      treeNumbers: [bobCoins[3]["treeNumber"]],
      merkleProofs: [bobCoins[3]["proof"]],
      erc721ContractAddress: erc721Contract.address,
    });
    // TX sent by a relayer
    await erc721VaultContract.withdraw([NFT_ID], bob.address, bobNFT);
    const res = await erc721Contract.ownerOf(NFT_ID);
    expect(res).to.equal(bob.address);
  });
});
