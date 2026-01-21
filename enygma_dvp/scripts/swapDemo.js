const hre = require("hardhat");
const jsUtils = require("../src/core/utils.js");
const MerkleTree = require("../src/core/merkle");
const dvpConf = require("../zkdvp.config.json");
const TREE_DEPTH = dvpConf["circom"]["meta-parameters"]["tree-depth"];
const crypto = require("crypto")

// action scripts
const userActions = require("./../src/core/endpoints/user.js");
const adminActions = require("./../src/core/endpoints/admin.js");
const relayerActions = require("./../src/core/endpoints/relayer.js");

async function demo() {
  var erc20MerkleTree = new MerkleTree(TREE_DEPTH, "erc20");
  var erc721MerkleTree = new MerkleTree(TREE_DEPTH, "erc721");


  // demo parameters
  // nft_id that Alice wants to swap with Bob
  // paymentAmount is the amount that Bob has to pay to Alice
  // depositAmount of each of erc20 coins that Bob deposits
  const nft_id = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(3))) % jsUtils.SNARK_SCALAR_FIELD;
  const paymentAmount  = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(2))) * 2n;
  const depositAmount  = paymentAmount / 2n;

  [owner, alice, bob] = await ethers.getSigners();
  console.log("owner: ", owner.address);
  console.log("alice: ", alice.address);
  console.log("bob: ", bob.address);

  const receipts = require("../build/receipts.json");
  const erc721VaultAddress = receipts["Erc721CoinVault"]["contractAddress"];
  const erc721Address = receipts["ERC721"]["contractAddress"];
  const erc20VaultAddress = receipts["Erc20CoinVault"]["contractAddress"];
  const erc20Address = receipts["ERC20"]["contractAddress"];
  const zkDvpAddress = receipts["ZkDvp"]["contractAddress"];

  const zkDvpContract = await hre.ethers.getContractAt(
      "ZkDvp",
      zkDvpAddress,
  );

  const erc20Contract = await hre.ethers.getContractAt(
      "RaylsERC20",
      erc20Address,
  );

  const erc721Contract = await hre.ethers.getContractAt(
      "RaylsERC721",
      erc721Address,
  );

  const erc20VaultContract = await hre.ethers.getContractAt(
      "Erc20CoinVault",
      erc20VaultAddress,
  );

  const erc721VaultContract = await hre.ethers.getContractAt(
      "Erc721CoinVault",
      erc721VaultAddress,
  );


  let aliceCoins = [];
  let bobCoins = [];

  console.log("Minting ERC721");
  // Mint NFT for Alice
  const mintTx = await adminActions.mintErc721(owner, alice, nft_id, erc721Contract);
  await mintTx.wait()
  
  const nftKeyDeposit = await jsUtils.newKeyPair();
  console.log("Depositing ERC721");
  var erc721Commitment = await userActions.depositErc721(
    alice,
    nft_id,
    nftKeyDeposit,
    erc721VaultContract,
    erc721Contract,
    erc721MerkleTree,
  );

  erc721MerkleTree.insertLeaves([erc721Commitment]);
  aliceCoins.push({"commitment":erc721Commitment,"treeNumber": erc721MerkleTree.lastTreeNumber,"root": erc721MerkleTree.root, "proof":erc721MerkleTree.generateProof(erc721Commitment)});

  // Bob deposit 2 * 1000 ERC20 token into ZkDvp
  const fundKeys = [jsUtils.newKeyPair(), jsUtils.newKeyPair()];

  console.log("Minting ERC20");

  await adminActions.mintErc20(owner, bob, depositAmount * 2n, erc20Contract);

  console.log("Depositing first ERC20 coin");
  const erc20Commitment1 = await userActions.depositErc20(
    bob,
    depositAmount,
    fundKeys[0],
    erc20VaultContract,
    erc20Contract,
    erc20MerkleTree,
  );

  erc20MerkleTree.insertLeaves([erc20Commitment1]);

  bobCoins.push({ "commitment":erc20Commitment1,
                  "treeNumber": erc20MerkleTree.lastTreeNumber,
                  "root": erc20MerkleTree.root, 
                  "proof":erc20MerkleTree.generateProof(erc20Commitment1)});

  console.log("Depositing second ERC20 coin");
  const erc20Commitment2 = await userActions.depositErc20(
    bob,
    depositAmount,
    fundKeys[1],
    erc20VaultContract,
    erc20Contract,
    erc20MerkleTree,
  );

  erc20MerkleTree.insertLeaves([erc20Commitment2]);
  // console.log("second ERC20 deposit: ", erc20MerkleTree.rootOfPrevTree(0));
  bobCoins.push({ "commitment":erc20Commitment2,
                  "treeNumber": erc20MerkleTree.lastTreeNumber,
                  "root": erc20MerkleTree.root, 
                  "proof":erc20MerkleTree.generateProof(erc20Commitment2)});

  // Alice generates NFT commitment for Bob
  console.log("Alice generates NFT commitment for Bob");
  const uid = jsUtils.erc721UniqueId(erc721Address, nft_id);

  // Bob generates a public key to receive the NFT
  const bobNFTKey = jsUtils.newKeyPair();
  // Bob generates a public to receive the change
  const bobChangeKey = jsUtils.newKeyPair();
  // Alice generates a public key to receive the payment
  const alicePaymentKey = jsUtils.newKeyPair();

  console.log("Generating NFT commitment for Bob.");

  // nftCommitment will be used as a massage by Bob
  const nftCommitment = jsUtils.getCommitment(uid, bobNFTKey.publicKey);

  // Bob generates payment commitment for Alice
  const changeAmount = depositAmount * 2n - paymentAmount;
  // paymentCommitment will be used as a massage by Alice
  console.log("Generating Payment commitment for Alice.");

  const erc20Uid = jsUtils.erc20UniqueId(erc20Address, paymentAmount);

  const paymentCommitment = jsUtils.getCommitment(
    erc20Uid,
    alicePaymentKey.publicKey,
  );

  console.log("Alice generates SNARK proof of ownership to send her NFT to Bob");
  const ownParams = await userActions.generateOwnershipProof(
    paymentCommitment,
    uid,
    nftKeyDeposit,
    bobNFTKey,
    TREE_DEPTH,
    aliceCoins[0]["proof"],
    aliceCoins[0]["root"],
    aliceCoins[0]["treeNumber"],
  );


  // Alice generates a tx to send her NFT to Bob
  console.log("Bob generates a tx to send payment to Alice");

  const jsParams = await userActions.generateErc20JoinSplitProof(
    nftCommitment,
    [depositAmount, depositAmount],
    fundKeys,
    [paymentAmount, changeAmount],
    [alicePaymentKey, bobChangeKey],
    TREE_DEPTH,
    [bobCoins[0]["proof"], bobCoins[1]["proof"]],
    [bobCoins[0]["root"], bobCoins[1]["root"]],
    [bobCoins[0]["treeNumber"], bobCoins[1]["treeNumber"]],
    erc20Address,
  );
  // Bob generates a tx to send payment to Alice

  console.log(jsParams);

  console.log(ownParams);

  console.log("Swapping");
  // A relayer forwards both transactions to ZkDvp
  const swapCommitments = await relayerActions.swap(
    owner,
    jsParams,
    ownParams,
    0,
    1,
    zkDvpContract
  );
  

  console.log("Swap was successful, updating local merkleTrees");
  console.log(swapCommitments);
  
  erc20MerkleTree.insertLeaves([swapCommitments[0], swapCommitments[1]]);

  aliceCoins.push({"commitment":swapCommitments[0],
                   "treeNumber":erc20MerkleTree.lastTreeNumber,
                   "proof":erc20MerkleTree.generateProof(swapCommitments[0]),
                   "root": erc20MerkleTree.root});
  bobCoins.push({"commitment":swapCommitments[1],
                   "treeNumber":erc20MerkleTree.lastTreeNumber,
                   "proof":erc20MerkleTree.generateProof(swapCommitments[1]),
                   "root": erc20MerkleTree.root});

  erc721MerkleTree.insertLeaves([swapCommitments[2]]);
   bobCoins.push({"commitment":swapCommitments[2],
                   "treeNumber":erc721MerkleTree.lastTreeNumber,
                   "proof":erc721MerkleTree.generateProof(swapCommitments[2]),
                   "root": erc721MerkleTree.root});

  console.log("Alice withdraws fund");

  // TX sent by a relayer
  const oldBalance = (await erc20Contract.balanceOf(alice.address)).toBigInt();

  const cmt4 = await userActions.withdrawErc20(
    alice,
    paymentAmount,
    alicePaymentKey,
    erc20VaultContract,
    erc20Contract,
    TREE_DEPTH,
    aliceCoins[1]["proof"],
    aliceCoins[1]["root"],
    aliceCoins[1]["treeNumber"],
  );

  const newBalance = (await erc20Contract.balanceOf(alice.address)).toBigInt();

  console.log(
    `Alice's oldBalance ${oldBalance} + payment ${paymentAmount} = newBalance ${newBalance}`,
  );

  console.log("Bob withdraws bought NFT");

  var cmt3 = await userActions.withdrawErc721(
    bob,
    nft_id,
    bobNFTKey,
    erc721VaultContract,
    erc721Contract,
    TREE_DEPTH,
    bobCoins[3]["proof"],
    bobCoins[3]["root"],
    bobCoins[3]["treeNumber"],
  );

  const res = await erc721Contract.ownerOf(nft_id);
  console.log(`nft_id = ${nft_id}, owner = ${res}`);

  erc20MerkleTree.saveToFile("erc20");
  erc721MerkleTree.saveToFile("erc721");
}

demo();


 