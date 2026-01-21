/* global describe it ethers before beforeEach afterEach overwriteArtifact */
const { expect } = require("chai");
const prover = require("../src/core/prover");
const jsUtils = require("../src/core/utils");
const myWeb3 = require("../src/web3");
const adminActions = require("./../src/core/endpoints/admin.js");
const userActions = require("./../src/core/endpoints/user.js");
const relayerActions = require("./../src/core/endpoints/relayer.js");
const crypto = require("crypto");
const dvpConf = require("../zkdvp.config.json");
const TREE_DEPTH = dvpConf["tree-depth"];
const testHelpers = require("./testHelpers.js");
const circomlib = require("circomlibjs");
const ffjavascript = require("ffjavascript");
const { babyjub: babyJub } = require("circomlibjs");

let users;
let contracts;
let merkleTree1155;
let merkleTreeFungibles;
let merkleTreeNonFungibles;

let alice = {};
let bob = {};
let auditor = {};
let relayer = {};
let owner = {};

let NFT_ID;
let FT_ID;
let groupUniqueIds = [];
let bobDepositAmount;

let tempStorage = [];

let babyjub;
let F;

describe("Auditability Test", () => {
  it(`ZkDvp should initialize properly `, async () => {
    let userCount = 3; // Alice:seller, Bob and auditor: Bidders
    [admin, users, contracts, merkleTrees, auditors] =
      await testHelpers.deployForTest(userCount);

    alice.wallet = users[0].wallet;
    bob.wallet = users[1].wallet;
    auditor = auditors[0];

    owner.wallet = admin;

    relayer.wallet = admin;
    relayer.calls = {};
    relayer.calls.zkdvp = contracts.zkdvp.connect(relayer.wallet);
    relayer.receivedProofs = [];

    alice.calls = {};
    bob.calls = {};
    auditor.calls = {};
    owner.calls = {};

    alice.coins = [];
    bob.coins = [];

    alice.knows = {};
    bob.knows = {};

    auditor.knows = {};

    owner.calls.erc1155 = contracts.erc1155.connect(owner.wallet);

    alice.calls.zkdvp = contracts.zkdvp.connect(alice.wallet);
    alice.calls.erc1155 = contracts.erc1155.connect(alice.wallet);
    alice.calls.erc1155Vault = contracts.erc1155CoinVault.connect(alice.wallet);

    bob.calls.zkdvp = contracts.zkdvp.connect(bob.wallet);
    bob.calls.erc1155 = contracts.erc1155.connect(bob.wallet);
    bob.calls.erc1155Vault = contracts.erc1155CoinVault.connect(bob.wallet);

    auditor.calls.zkdvp = contracts.zkdvp.connect(auditor.wallet);
    auditor.calls.erc1155 = contracts.erc1155.connect(auditor.wallet);
    auditor.calls.erc1155Vault = contracts.erc1155CoinVault.connect(
      auditor.wallet,
    );

    merkleTree1155 = merkleTrees["ERC1155"].tree;
    merkleTreeFungibles = merkleTrees["fungibleGroup"].tree;
    merkleTreeNonFungibles = merkleTrees["nonFungibleGroup"].tree;

    console.log("Sharing Auditor's publicKey with Alice and Bob.");
    alice.knows.auditorPublicKey = auditor.keyPair.publicKey;
    bob.knows.auditorPublicKey = auditor.keyPair.publicKey;

    console.log("Auditability Test: Done ZkDvp initialization");
  });

  it(`Admin should register erc1155 tokens properly. `, async () => {
    NFT_ID =
      jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(1))) %
      jsUtils.SNARK_SCALAR_FIELD;
    FT_ID =
      jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(1))) %
      jsUtils.SNARK_SCALAR_FIELD;

    const erc1155Contract = contracts["erc1155"];
    const zkDvpContract = contracts["zkdvp"];

    // register a fungible token
    await erc1155Contract.registerNewToken(
      0n, // type
      0n, // fungiblity
      "Test Token 1", // name
      "TTT1", // symbol
      FT_ID, // offchainId
      10000000000000n, // maxSupply
      18n, // decimals
      [], // subTokenIds
      [], // subTokenValues
      0, // data
      [], // additionalAttrs
    );
    await erc1155Contract.registerNewToken(
      0n, // type
      1n, // fungiblity
      "Test Token 2", // name
      "TTT2", // symbol
      NFT_ID, // offchainId
      1, // maxSupply
      0n, // decimals
      [], // subTokenIds
      [], // subTokenValues
      0, // data
      [], // additionalAttrs
    );

    let tokenIds = [FT_ID, NFT_ID];
    let groupIds = [0, 1];

    for (var i = 0; i < tokenIds.length; i++) {
      const tx4 = await zkDvpContract.addTokenToGroup(
        2, // VAULT_ID_ERC1155,
        [0n, tokenIds[i]],
        groupIds[i],
      );

      // reading the added uniqueId, altenatively you can
      // compute it off-chain by
      // const uidNonFung = jsUtils.erc1155UniqueId(BigInt(erc1155Address), nonFungId, 1n);
      const addTokenEvent = await testHelpers.parseTokenAddedEvent(tx4);
      const newGroupUniqueId = addTokenEvent.tokenUniqueId;
      // updating local non-fungibleGroup merkleTree

      if (groupIds[i] == 0) {
        merkleTreeFungibles.insertLeaves([newGroupUniqueId]);
      } else {
        merkleTreeNonFungibles.insertLeaves([newGroupUniqueId]);
      }

      groupUniqueIds.push(newGroupUniqueId);
    }

    console.log("Broker v1 Test: Erc1155 tokens have been registered.");
  });

  it("Admin mints non-fungible Erc1155 for Alice and fungible Erc1155 for Bob.", async () => {
    await owner.calls.erc1155.mint(alice.wallet.address, NFT_ID, 1, 0);

    console.log("Minted non-fungible token for Alice with tokenID = ", NFT_ID);

    // limiting the size of the deposit to 2 bytes to avoid
    // receating a large decryption table each time.
    bobDepositAmount = jsUtils.buffer2BigInt(
      Buffer.from(crypto.randomBytes(1)),
    );
    await owner.calls.erc1155.mint(
      bob.wallet.address,
      FT_ID,
      bobDepositAmount * 2n,
      0,
    );

    console.log("Minted fungible token for Bob with tokenID = ", FT_ID);
  });

  it("Alice should deposit a non-fungible Erc1155 token.", async () => {
    let aliceNonFungKey = jsUtils.newKeyPair();

    await alice.calls.erc1155.setApprovalForAll(
      contracts.erc1155CoinVault.address,
      true,
    );

    let tx = await alice.calls.erc1155Vault.deposit([
      1n,
      NFT_ID,
      aliceNonFungKey.publicKey,
    ]);

    let cmt = await testHelpers.getCommitmentFromTx(tx);

    merkleTree1155.insertLeaves([cmt]);
    let proof0 = merkleTree1155.generateProof(cmt);
    alice.coins.push({
      vaultId: 2, //ERC1155 vaultId
      tokenId: NFT_ID,
      value: 1n,
      tokenAddress: contracts.erc1155.address,
      key: aliceNonFungKey,
      commitment: cmt,
      proof: proof0,
      root: merkleTree1155.root,
      treeNumber: merkleTree1155.lastTreeNumber,
      groupTreeNumber: merkleTreeNonFungibles.lastTreeNumber,
      groupProof: merkleTreeNonFungibles.generateProof(groupUniqueIds[1]),
    });

    alice.knows.delivery = {};
    alice.knows.delivery.tokenId = NFT_ID;
    alice.knows.delivery.tokenAddress = contracts.erc1155.address;
  });

  it("Bob should deposit fungible ERC1155 tokens in form of two coins.", async () => {
    console.log("Depositing two fungible erc1155 coins for Bob");

    // minting ERC20 tokens for Bob
    let tx = await owner.calls.erc1155.mint(
      bob.wallet.address,
      FT_ID,
      bobDepositAmount * 2n,
      0,
    );
    await tx.wait();

    await bob.calls.erc1155.setApprovalForAll(
      contracts.erc1155CoinVault.address,
      true,
    );

    for (var i = 0; i < 2; i++) {
      // creating depositKeys for Bob to Deposit Erc20 tokens to ZkDvp
      let bobErc1155Key = jsUtils.newKeyPair();

      // adding new erc20 coin to bob's erc20 coin list

      tx = await bob.calls.erc1155Vault.deposit([
        bobDepositAmount,
        FT_ID,
        bobErc1155Key.publicKey,
      ]);

      const bobCmt = await testHelpers.getCommitmentFromTx(tx);
      merkleTree1155.insertLeaves([bobCmt]);
      let bobProof = merkleTree1155.generateProof(bobCmt);

      bob.coins.push({
        vaultId: 2,
        value: bobDepositAmount,
        tokenAddress: contracts.erc1155.address,
        key: bobErc1155Key,
        commitment: bobCmt,
        proof: bobProof,
        root: merkleTree1155.root,
        treeNumber: merkleTree1155.lastTreeNumber,
        tokenId: FT_ID,
        groupTreeNumber: merkleTreeFungibles.lastTreeNumber,
        groupProof: merkleTreeFungibles.generateProof(groupUniqueIds[0]),
      });
    }
  });

  it("[[OFF-CHAIN]] [[NOT-IMPLEMENTED]] Alice and bob share needed information for the swap.", async () => {
    // generating receiving keypairs
    alice.knows.aliceRecCoinKey = jsUtils.newKeyPair();
    bob.knows.bobRecCoinKey = jsUtils.newKeyPair();

    // Alice sends Bob the crucial delivery information
    bob.knows.delivery = alice.knows.delivery;

    // Alice and Bob agree on a price
    const agreed_price = 10n;
    bob.knows.delivery.price = agreed_price;
    alice.knows.delivery.price = agreed_price;

    // Alice sends Bob her receiving publicKey
    bob.knows.aliceRecPublicKey = alice.knows.aliceRecCoinKey.publicKey;

    // Bob sends Alice his receiving publicKey
    alice.knows.bobRecPublicKey = bob.knows.bobRecCoinKey.publicKey;

    bob.knows.payment = {};
    bob.knows.payment.tokenAddress = bob.coins[0].tokenAddress;
    bob.knows.payment.tokenId = bob.coins[0].tokenId;

    alice.knows.payment = bob.knows.payment;
  });

  it("Alice submits her proof through a Relayer.", async () => {
    const uniqueId = jsUtils.erc1155UniqueId(
      alice.knows.payment.tokenAddress,
      alice.knows.payment.tokenId,
      alice.knows.delivery.price,
    );
    const deliveryProofMessage = jsUtils.getCommitment(
      uniqueId,
      alice.knows.aliceRecCoinKey.publicKey,
    );

    const deliveryProof = await prover.prove(
      "OwnershipErc1155NonFungibleWithAuditor",
      {
        message: deliveryProofMessage,
        values: [alice.coins[0].value],
        keysIn: [alice.coins[0].key],
        keysOut: [{ publicKey: alice.knows.bobRecPublicKey }],
        treeNumbers: [alice.coins[0].treeNumber],
        merkleProofs: [alice.coins[0].proof],
        erc1155ContractAddress: BigInt(contracts.erc1155.address),
        erc1155TokenIds: [alice.coins[0].tokenId],
        auditor_publicKey: alice.knows.auditorPublicKey,
        assetGroup_treeNumbers: [alice.coins[0].groupTreeNumber],
        assetGroup_merkleProofs: [alice.coins[0].groupProof],
      },
    );

    await relayer.calls.zkdvp.submitPartialSettlement(deliveryProof, 2, 1);

    for (var i = 0; i < deliveryProof.numberOfOutputs; i++) {
      tempStorage.push(
        deliveryProof.statement[1 + i + 3 * deliveryProof.numberOfInputs],
      );
    }

    console.log("[NOT-IMPLEMENTED] Auditor has access to calldata history. ");
    auditor.knows.aliceStatments = deliveryProof.statement;
  });

  it("Bob submits his proof through a Relayer. ", async () => {
    const excessAmount =
      bob.coins[0].value + bob.coins[1].value - bob.knows.delivery.price;

    const uniqueId = jsUtils.erc1155UniqueId(
      bob.knows.delivery.tokenAddress,
      bob.knows.delivery.tokenId,
      1n,
    );
    const paymentProofMessage = jsUtils.getCommitment(
      uniqueId,
      bob.knows.bobRecCoinKey.publicKey,
    );

    // generating new keypair for Bob for the excess amount
    let bobExcessCoinKey = jsUtils.newKeyPair();
    bob.knows.bobExcessCoinKey = bobExcessCoinKey;
    bob.knows.bobExcessAmount = excessAmount;

    const paymentProof = await prover.prove("JoinSplitErc1155WithAuditor", {
      message: paymentProofMessage,
      valuesIn: [bob.coins[0].value, bob.coins[1].value],
      keysIn: [bob.coins[0].key, bob.coins[1].key],
      valuesOut: [bob.knows.delivery.price, excessAmount],
      keysOut: [
        { publicKey: bob.knows.aliceRecPublicKey },
        { publicKey: bobExcessCoinKey.publicKey },
      ],
      treeNumbers: [bob.coins[0].treeNumber, bob.coins[1].treeNumber],
      merkleProofs: [bob.coins[0].proof, bob.coins[1].proof],
      erc1155ContractAddress: contracts.erc1155.address,
      erc1155TokenId: bob.coins[0].tokenId,
      auditor_publicKey: bob.knows.auditorPublicKey,
      assetGroup_treeNumber: bob.coins[0]["groupTreeNumber"],
      assetGroup_merkleProof: bob.coins[0]["groupProof"],
    });

    await relayer.calls.zkdvp.submitPartialSettlement(paymentProof, 2, 0);

    for (var i = 0; i < paymentProof.numberOfOutputs; i++) {
      tempStorage.push(
        paymentProof.statement[1 + i + 3 * paymentProof.numberOfInputs],
      );
    }

    console.log(
      "[NOT-IMPLEMENTED] Auditor has access to calldata history or receiving the events ",
    );
    auditor.knows.bobStatments = paymentProof.statement;
  });

  it("Auditor opens the Alice's transaction's private information.", async () => {
    console.log(
      "Decrypting Alice's statements' encryptedData with Auditor's privateKey",
    );

    // expected elements is from GOD-view.
    const expected = [
      1n,
      alice.knows.delivery.tokenId,
      BigInt(alice.knows.delivery.tokenAddress),
    ];

    const aStat = auditor.knows.aliceStatments;
    const authKey = aStat.slice(7, 9);
    const nonce = aStat[9];
    const encrypted = aStat.slice(10, 14);
    const length = expected.length;
    console.log("Expected values: ", JSON.stringify(expected, null, 4));

    const decrypted = jsUtils.poseidonDecryptWrapper(
      babyJub,
      encrypted,
      authKey,
      nonce,
      auditor.keyPair.privateKey,
      length,
    );
    console.log(`Decrypted value: ${decrypted}`);

    for (var i = 0; i < expected.length; i++) {
      expect(BigInt(decrypted[i])).to.equal(BigInt(expected[i]));
    }
  });

  it("Auditor opens the Bob's transaction's private information.", async () => {
    console.log(
      "Decrypting Bobs statements' encryptedData with Auditor's privateKey",
    );

    // expected elements is from GOD-view.
    expected = [
      bobDepositAmount,
      bobDepositAmount,
      bob.knows.delivery.price,
      bob.knows.bobExcessAmount,
      bob.knows.payment.tokenId,
      BigInt(contracts.erc1155.address),
    ];

    console.log("Expected values: ", JSON.stringify(expected, null, 4));

    const bStat = auditor.knows.bobStatments;
    const authKey = bStat.slice(11, 13);
    const nonce = bStat[13];
    const encrypted = bStat.slice(14, 21);
    const length = expected.length;

    const logDict = { authKey, nonce, encrypted, length };
    console.log(JSON.stringify(logDict, null, 4));
    const decrypted = jsUtils.poseidonDecryptWrapper(
      babyJub,
      encrypted,
      authKey,
      nonce,
      auditor.keyPair.privateKey,
      length,
    );
    console.log(`Decrypted value: ${decrypted}`);

    for (var i = 0; i < expected.length; i++) {
      expect(BigInt(decrypted[i])).to.equal(BigInt(expected[i]));
    }
  });

  it("Bob should be able to withdraw the bought non-fungible Erc1155 token.", async () => {});

  it("Alice should be able to withdraw the fungible Erc1155 price.", async () => {});

  it("Auditor should be able to verify withdraw transactions.", async () => {});
});
