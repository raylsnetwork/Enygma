const hre = require("hardhat");
const jsUtils = require("../src/core/utils.js");
const MerkleTree = require("../src/core/merkle");
const dvpConf = require("../zkdvp.config.json");
const TREE_DEPTH = dvpConf["tree-depth"];

// action scripts
const userActions = require("./../src/core/endpoints/user.js");
const adminActions = require("./../src/core/endpoints/admin.js");
const relayerActions = require("./../src/core/endpoints/relayer.js");

const VAULT_ID_ERC1155 = 2;

async function getTokenAddedEvent(tx) {
  const rc = await tx.wait();
  const event = rc.events.find((ev) => ev.event === "TokenAddedToGroup");
  
  const eventData = {};
  eventData.groupId = event.args.groupId.toBigInt();
  eventData.vaultId = event.args.vaultId.toBigInt();
  eventData.tokenUniqueId = event.args.tokenUniqueId.toBigInt();

  return eventData;
}



async function erc1155Demo(tokenIds, depositAmounts, paymentAmount) {
  
  var merkleTree = new MerkleTree(TREE_DEPTH, "erc1155");

  var fungibleMerkleTree = new MerkleTree(TREE_DEPTH, "fungibleAssetGroup");
  var nonFungibleMerkleTree = new MerkleTree(TREE_DEPTH, "nonFungibleAssetGroup");

  let aliceCoins = [];
  let bobCoins = [];

  [owner, alice, bob] = await ethers.getSigners();
  console.log("owner: ", owner.address);
  console.log("alice: ", alice.address);
  console.log("bob: ", bob.address);

  const fungId = tokenIds[1];
  const nonFungId = tokenIds[0];
  const fungAmount = depositAmounts[1];
  const nonFungAmount = depositAmounts[0];
  
  const erc1155data = 0;

  const receipts = require("../build/receipts.json");
  const erc1155Address = receipts["ERC1155"]["contractAddress"];
  const erc1155VaultAddress = receipts["Erc1155CoinVault"]["contractAddress"];
  const zkDvpAddress = receipts["ZkDvp"]["contractAddress"];

  const zkDvpContract = await hre.ethers.getContractAt("ZkDvp", zkDvpAddress);

  const erc1155Contract = await hre.ethers.getContractAt("RaylsERC1155", erc1155Address);
  const erc1155VaultContract = await hre.ethers.getContractAt("Erc1155CoinVault", erc1155VaultAddress);


  console.log("Registering Fungible ERC-1155 tokenId to RaylsERC1155");
    const tx1 = await erc1155Contract.registerNewToken(
        0n, // type 
        0n, // fungiblity
        "Test Fungible", // name
        "TFT", // symbol
        fungId, // offchainId
        1000000000000000n, // maxSupply
        18n, // decimals
        [], // subTokenIds
        [],  // subTokenValues
        0, // data
        [] // additionalAttrs
    );

  console.log("Registering Fungible ERC-1155 tokenId to fungible AssetGroup");
  const tx2 = await zkDvpContract.addTokenToGroup(
      VAULT_ID_ERC1155,
      [0, fungId],
      0
    );

  // reading the added uniqueId, altenatively you can 
  // compute it off-chain by
  // const uidFung = jsUtils.erc1155UniqueId(BigInt(erc1155Address), fungId, 0n);

  const fungAddEvent = await getTokenAddedEvent(tx2);
  const uidFung = fungAddEvent.tokenUniqueId;
  // updating local fungibleGroup merkleTree
  fungibleMerkleTree.insertLeaves([uidFung]);

  console.log("Registering non-Fungible ERC-1155 tokenId to RaylsERC1155");
    const tx3 = await erc1155Contract.registerNewToken(
        0n, // type 
        1n, // fungiblity
        "Test Non-fungible", // name
        "TNFT", // symbol
        nonFungId, // offchainId
        0n, // maxSupply
        0n, // decimals
        [], // subTokenIds
        [],  // subTokenValues
        0, // data
        [] // additionalAttrs
    );

  console.log("Registering non-Fungible ERC-1155 tokenId to Non-fungible AssetGroup");

  const tx4 = await zkDvpContract.addTokenToGroup(
      VAULT_ID_ERC1155,
      [0, nonFungId],
      1
    );

  // reading the added uniqueId, altenatively you can 
  // compute it off-chain by
  // const uidNonFung = jsUtils.erc1155UniqueId(BigInt(erc1155Address), nonFungId, 1n);
  const nonFungAddEvent = await getTokenAddedEvent(tx4);
  const uidNonFung = nonFungAddEvent.tokenUniqueId;
  // updating local non-fungibleGroup merkleTree
  nonFungibleMerkleTree.insertLeaves([uidNonFung]);


  console.log("Minting non-Fungible ERC-1155");
  // Mint NFT for Alice
  await adminActions.mintErc1155(owner, alice, nonFungId, nonFungAmount, 0, erc1155Contract);

  const nftKeyDeposit = await jsUtils.newKeyPair();
  console.log("Depositing non-fungible ERC-1155");
  var cmt = await userActions.depositErc1155(
    alice,
    nonFungId,
    nonFungAmount,
    erc1155data,
    nftKeyDeposit,
    erc1155VaultContract,
    erc1155Contract,
  );

  merkleTree.insertLeaves([cmt]);
  aliceCoins.push({ "commitment":cmt,
                    "proof": merkleTree.generateProof(cmt),
                    "root": merkleTree.root,
                    "treeNumber": merkleTree.lastTreeNumber,
                    "group_treeNumber": nonFungibleMerkleTree.lastTreeNumber,
                    "group_proof": nonFungibleMerkleTree.generateProof(uidNonFung),
                    });
  // Bob deposit 2 * 1000 ERC20 token into ZkDvp
  const fundKeys = [jsUtils.newKeyPair(), jsUtils.newKeyPair()];

  console.log("Minting fungible ERC-1155");

  await adminActions.mintErc1155(owner, bob, fungId, fungAmount * 2n, 0, erc1155Contract);

  console.log("Depositing fungible ERC-1155");
  var cmt1 = await userActions.depositErc1155(
    bob,
    fungId,
    fungAmount,
    erc1155data,
    fundKeys[0],
    erc1155VaultContract,
    erc1155Contract,
    merkleTree,
  );

  merkleTree.insertLeaves([cmt1]);
  bobCoins.push({ "commitment":cmt1,
                    "proof": merkleTree.generateProof(cmt1),
                    "root": merkleTree.root,
                    "treeNumber": merkleTree.lastTreeNumber,
                    "group_treeNumber": fungibleMerkleTree.lastTreeNumber,
                    "group_proof": fungibleMerkleTree.generateProof(uidFung)
                    });


  console.log("Depositing second amount for fungible ERC-1155");
  var cmt2 = await userActions.depositErc1155(
    bob,
    fungId,
    fungAmount,
    erc1155data,
    fundKeys[1],
    erc1155VaultContract,
    erc1155Contract,
    merkleTree,
  );

  merkleTree.insertLeaves([cmt2]);
  bobCoins.push({ "commitment":cmt2,
                    "proof": merkleTree.generateProof(cmt2),
                    "root": merkleTree.root,
                    "treeNumber": merkleTree.lastTreeNumber,
                    "group_treeNumber": fungibleMerkleTree.lastTreeNumber,
                    "group_proof": fungibleMerkleTree.generateProof(uidFung)
                    });

  // Alice generates NFT commitment for Bob
  console.log("Alice generates commitment of non-fungible token for Bob");
  const uid2 = jsUtils.erc1155UniqueId(BigInt(erc1155Address), nonFungId, nonFungAmount);

  // Bob generates a public key to receive the NFT
  const bobNFTKey = jsUtils.newKeyPair();
  // Bob generates a public to receive the change
  const bobChangeKey = jsUtils.newKeyPair();
  // Alice generates a public key to receive the payment
  const alicePaymentKey = jsUtils.newKeyPair();

  console.log("Generating NFT commitment for Bob.");

  // nftCommitment will be used as a massage by Bob
  const nftCommitment = jsUtils.getCommitment(uid2, bobNFTKey.publicKey);
  // console.log("nftCommitment: ", nftCommitment)

  // Bob generates payment commitment for Alice
  const changeAmount = fungAmount * 2n - paymentAmount;
  // paymentCommitment will be used as a massage by Alice
  console.log("Generating Payment commitment for Alice.");

  const uid4 = jsUtils.erc1155UniqueId(BigInt(erc1155Address), fungId, paymentAmount);

  const paymentCommitment = jsUtils.getCommitment(
    uid4,
    alicePaymentKey.publicKey,
  );

  console.log("Alice generates a tx to send her NFT to Bob");
  const params1 = await userActions.generateSingleErc1155Proof(
    paymentCommitment,
    nonFungAmount,
    nftKeyDeposit,
    bobNFTKey,
    TREE_DEPTH,
    aliceCoins[0]["proof"],
    aliceCoins[0]["root"],
    aliceCoins[0]["treeNumber"],
    erc1155Contract.address,
    nonFungId,
    aliceCoins[0]["group_treeNumber"],
    aliceCoins[0]["group_proof"],
    false
  );

  // Alice generates a tx to send her NFT to Bob

  console.log("Bob generates a tx to send payment to Alice");
  const params2 = await userActions.generateErc1155JoinSplitProof(
    nftCommitment,
    [fungAmount, fungAmount],
    fundKeys,
    [paymentAmount, changeAmount],
    [alicePaymentKey, bobChangeKey],
    TREE_DEPTH,
    [bobCoins[0]["proof"], bobCoins[1]["proof"]],
    [bobCoins[0]["root"], bobCoins[1]["root"]],
    [bobCoins[0]["treeNumber"], bobCoins[1]["treeNumber"]],
    erc1155Contract.address,
    fungId,
    bobCoins[0]["group_treeNumber"],
    bobCoins[0]["group_proof"],
  );

  console.log("Swapping");
  // A relayer forwards both transactions to ZkDvp
  const cmt5 = await relayerActions.swap(
    owner,
    params2,
    params1,
    VAULT_ID_ERC1155,
    VAULT_ID_ERC1155,
    zkDvpContract,
  );
  merkleTree.insertLeaves(cmt5);


  aliceCoins.push({"commitment":cmt5[0],
                   "treeNumber":merkleTree.lastTreeNumber,
                   "proof":merkleTree.generateProof(cmt5[0]),
                   "root": merkleTree.root,
                    "group_treeNumber": fungibleMerkleTree.lastTreeNumber,
                    "group_proof": fungibleMerkleTree.generateProof(uidFung) 
                 });

  bobCoins.push({"commitment":cmt5[1],
                   "treeNumber":merkleTree.lastTreeNumber,
                   "proof":merkleTree.generateProof(cmt5[1]),
                   "root": merkleTree.root,
                    "group_treeNumber": fungibleMerkleTree.lastTreeNumber,
                    "group_proof": fungibleMerkleTree.generateProof(uidFung) 

                 });

   bobCoins.push({"commitment":cmt5[2],
                   "treeNumber":merkleTree.lastTreeNumber,
                   "proof":merkleTree.generateProof(cmt5[2]),
                   "root": merkleTree.root,
                    "group_treeNumber": nonFungibleMerkleTree.lastTreeNumber,
                    "group_proof": nonFungibleMerkleTree.generateProof(uidNonFung) 

                 });


  console.log("Alice withdraws fund");

  // TX sent by a relayer
  const oldBalance = (await erc1155Contract.balanceOf(alice.address, fungId)).toBigInt();

  const cmt4 = await userActions.withdrawERC1155(
    alice,
    paymentAmount,
    fungId,
    alicePaymentKey,
    erc1155VaultContract,
    erc1155Contract,
    TREE_DEPTH,
    aliceCoins[1]["proof"],
    aliceCoins[1]["root"],
    aliceCoins[1]["treeNumber"],
    aliceCoins[1]["group_treeNumber"],
    aliceCoins[1]["group_proof"],
    true
  );

  console.log("Alice withdrew!")
  const newBalance = (await erc1155Contract.balanceOf(alice.address, fungId)).toBigInt();

  console.log(
    `Alice's oldBalance ${oldBalance} + payment ${paymentAmount} = newBalance ${newBalance}`,
  );

  console.log("Bob withdraws bought Non-fungible ERC1155 token....");


  const cmt3 = await userActions.withdrawERC1155(
    bob,
    nonFungAmount,
    nonFungId,
    bobNFTKey,
    erc1155VaultContract,
    erc1155Contract,
    TREE_DEPTH,
    bobCoins[3]["proof"],
    bobCoins[3]["root"],
    bobCoins[3]["treeNumber"],
    bobCoins[3]["group_treeNumber"],
    bobCoins[3]["group_proof"],
    false
  );
  console.log("Bob withdrew bought Non-fungible Erc1155");


  const res = await erc1155Contract.balanceOf(bob.address, nonFungId);


  console.log(`Checking whether Bob is the owner of Non-fungible Erc1155 token ${nonFungId} = ${res}`);

  console.log(`---------------------------------`);
  console.log(`---------------------------------`);
  console.log(`---------------------------------`);
  console.log(`Testing EREC1155 batch mode:`);

  merkleTree.saveToFile("erc1155");

  console.log("Erc1155 Swap demo: DONE.")
}


erc1155Demo([777n, 111n], [1n,1000n], 100n);


 