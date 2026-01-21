// /* global describe it ethers before beforeEach afterEach overwriteArtifact */
// const { expect } = require("chai");
// const prover = require("../src/core/prover");
// const jsUtils = require("../src/core/utils");
// const myWeb3 = require("../src/web3");
// const adminActions = require("./../src/core/endpoints/admin.js");
// const userActions = require("./../src/core/endpoints/user.js");
// const relayerActions = require("./../src/core/endpoints/relayer.js");
// const crypto = require("crypto")
// const dvpConf = require("../zkdvp.config.json");
// const TREE_DEPTH = dvpConf["tree-depth"];
// const testHelpers = require("./testHelpers.js");


// let users;
// let contracts;
// let merkleTree1155;
// let merkleTreeFungibles;
// let merkleTreeNonFungibles;

// let alice = {};
// let bob = {};
// let carl = {};
// let auctioneer = {};
// let owner = {};
// let auctionData;
// let winnerParams;
// let auctionSummary = {};

// let depositCount;
// let swapCount;

// let inKeys = [];
// let NFT_ID;
// let FT_ID;
// let groupUniqueIds = [];

// describe("ZkAuction with Erc1155 non-fungible item and Erc1155 fungible bids Test", () => {

//     it(`ZkDvp should initialize properly `, async () => {
        
//         let userCount = 3; // Alice:seller, Bob and Carl: Bidders
//         [admin, users, contracts, merkleTrees] = await testHelpers.deployForTest(userCount);

//         alice.wallet = users[0].wallet;
//         bob.wallet =   users[1].wallet;
//         carl.wallet =  users[2].wallet;
//         auctioneer.wallet = admin;

//         owner.wallet = admin

//         alice.calls = {};
//         bob.calls = {};
//         carl.calls = {};
//         owner.calls = {};
//         auctioneer.calls = {};

//         bob.bids = [];
//         carl.bids = [];

//         alice.coins = [];
//         bob.coins = [];
//         carl.coins = [];

//         auctioneer.openings = [];
//         auctioneer.bidLogs = [];

//         auctioneer.calls.zkdvp = contracts.zkdvp.connect(auctioneer.wallet);
//         auctioneer.calls.zkAuction = contracts.zkAuction.connect(auctioneer.wallet);
        

//         owner.calls.erc1155 = contracts.erc1155.connect(owner.wallet);

//         alice.calls.zkdvp = contracts.zkdvp.connect(alice.wallet);
//         alice.calls.zkAuction = contracts.zkAuction.connect(alice.wallet);
//         alice.calls.erc1155 = contracts.erc1155.connect(alice.wallet);
//         alice.calls.erc1155Vault = contracts.erc1155CoinVault.connect(alice.wallet);

//         bob.calls.zkdvp = contracts.zkdvp.connect(bob.wallet);
//         bob.calls.zkAuction = contracts.zkAuction.connect(bob.wallet);
//         bob.calls.erc1155 = contracts.erc1155.connect(bob.wallet);
//         bob.calls.erc1155Vault = contracts.erc1155CoinVault.connect(bob.wallet);

//         carl.calls.zkdvp = contracts.zkdvp.connect(carl.wallet);
//         carl.calls.zkAuction = contracts.zkAuction.connect(carl.wallet);
//         carl.calls.erc1155 = contracts.erc1155.connect(carl.wallet);
//         carl.calls.erc1155Vault = contracts.erc1155CoinVault.connect(carl.wallet);

//         merkleTree1155 = merkleTrees["ERC1155"].tree;
//         merkleTreeFungibles = merkleTrees["fungibleGroup"].tree;
//         merkleTreeNonFungibles = merkleTrees["nonFungibleGroup"].tree;

//         console.log("Auction Test: ZkDvp initialization")

//   });


//     it(`Admin should register erc1155 tokens properly. `, async () => {

//         NFT_ID = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(4))) % jsUtils.SNARK_SCALAR_FIELD;
//         FT_ID = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(4))) % jsUtils.SNARK_SCALAR_FIELD;

//         const erc1155Contract = contracts["erc1155"];
//         const zkDvpContract = contracts["zkdvp"];

//         // register a fungible token
//         await erc1155Contract.registerNewToken(
//             0n, // type 
//             0n, // fungiblity
//             "Test Token 1", // name
//             "TTT1", // symbol
//             FT_ID, // offchainId
//             10000000000000n, // maxSupply
//             18n, // decimals
//             [], // subTokenIds
//             [],  // subTokenValues
//             0, // data
//             [] // additionalAttrs
//         );
//         await erc1155Contract.registerNewToken(
//             0n, // type 
//             1n, // fungiblity
//             "Test Token 2", // name
//             "TTT2", // symbol
//             NFT_ID, // offchainId
//             1, // maxSupply
//             0n, // decimals
//             [], // subTokenIds
//             [],  // subTokenValues
//             0, // data
//             [] // additionalAttrs
//         );

//         let tokenIds = [FT_ID, NFT_ID];
//         let groupIds = [0 , 1];

//         for (var i = 0; i < tokenIds.length; i++) {
//           const tx4 = await zkDvpContract.addTokenToGroup(
//               2, // VAULT_ID_ERC1155,
//               [0n, tokenIds[i]],
//               groupIds[i]
//           );
          

//           // reading the added uniqueId, altenatively you can 
//           // compute it off-chain by
//           // const uidNonFung = jsUtils.erc1155UniqueId(BigInt(erc1155Address), nonFungId, 1n);
//           const addTokenEvent = await testHelpers.parseTokenAddedEvent(tx4);
//           const newGroupUniqueId = addTokenEvent.tokenUniqueId;
//           // updating local non-fungibleGroup merkleTree

//           if(groupIds[i] == 0){
//               merkleTreeFungibles.insertLeaves([newGroupUniqueId]);
//           }
//           else{
//               merkleTreeNonFungibles.insertLeaves([newGroupUniqueId]);
//           }

//           groupUniqueIds.push(newGroupUniqueId);

//         }

//         console.log("Auction1155 Test: Erc1155 tokens have been registered.");

//     });



//   it("Alice should deposit a non-fungible Erc11155 token.", async () => {

//         await owner.calls.erc1155.mint(
//             alice.wallet.address, 
//             NFT_ID, 
//             1 , 
//             0
//         );

//         console.log("Minted non-fungible token.");

//         let aliceNonFungKey = jsUtils.newKeyPair();
//         await alice.calls.erc1155.setApprovalForAll(contracts.erc1155CoinVault.address, true);


//         let tx = await alice.calls.erc1155Vault.deposit(
//           [1n, NFT_ID,
//           aliceNonFungKey.publicKey]
//         );

//         let cmt = await testHelpers.getCommitmentFromTx(tx);

//         merkleTree1155.insertLeaves([cmt]);
//         let proof0 = merkleTree1155.generateProof(cmt);
//         alice.coins.push({
//                             "vaultId": 2, //ERC1155 vaultId
//                             "tokenId":NFT_ID, 
//                             "value":1n, 
//                             "tokenAddress": contracts.erc1155.address,
//                             "key":aliceNonFungKey, 
//                             "commitment": cmt, 
//                             "proof":proof0, 
//                             "root":merkleTree1155.root, 
//                             "treeNumber":merkleTree1155.lastTreeNumber,
//                             "groupRoot": merkleTreeNonFungibles.root,
//                             "groupProof": merkleTreeNonFungibles.generateProof(groupUniqueIds[1])
//                         });
//   });

//   it("Bob and Carl should deposit fungible ERC1155 tokens.", async () => {

//         console.log("Depositing two fungible erc1155 coins for Bob");

//         // Bob deposit 2x10 ethers into ZkDvp
//         let bobDepositAmount = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(4)));


//         // minting ERC20 tokens for Bob
//         let tx = await owner.calls.erc1155.mint(bob.wallet.address, FT_ID, bobDepositAmount * 2n, 0);
//         await tx.wait();

//         await bob.calls.erc1155.setApprovalForAll(contracts.erc1155CoinVault.address, true);

//         for(var i = 0; i< 2; i++){
//             // creating depositKeys for Bob to Deposit Erc20 tokens to ZkDvp
//             let bobErc1155Key = jsUtils.newKeyPair();

//             // adding new erc20 coin to bob's erc20 coin list

//             tx = await bob.calls.erc1155Vault.deposit(
//               [bobDepositAmount, FT_ID,
//               bobErc1155Key.publicKey]
//             );

//             const bobCmt = await testHelpers.getCommitmentFromTx(tx);
//             merkleTree1155.insertLeaves([bobCmt]);
//             let bobProof = merkleTree1155.generateProof(bobCmt);

//             bob.coins.push({    "vaultId": 2,
//                                 "value":bobDepositAmount, 
//                                 "tokenAddress": contracts.erc1155.address,
//                                 "key":bobErc1155Key, 
//                                 "commitment":bobCmt, 
//                                 "proof":bobProof, 
//                                 "root":merkleTree1155.root, 
//                                 "treeNumber":merkleTree1155.lastTreeNumber,
//                                 "tokenId": FT_ID,
//                                 "groupProof": merkleTreeFungibles.generateProof(groupUniqueIds[0]),
//                                 "groupRoot": merkleTreeFungibles.root
//                             });

//         }


//         console.log("Depositing two fungible erc1155 coins for Carl");
//         let carlDepositAmount = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(3)));

//         // minting ERC20 tokens for Bob
//         tx = await owner.calls.erc1155.mint(carl.wallet.address, FT_ID, carlDepositAmount * 2n, 0);
//         await tx.wait();

//         // Approve ZkDvp to transfer tokens
//         await carl.calls.erc1155.setApprovalForAll(contracts.erc1155CoinVault.address, true);

//         for(var i = 0; i< 2; i++){

//             // creating depositKeys for Bob to Deposit Erc20 tokens to ZkDvp
//             let carlErc1155Key = jsUtils.newKeyPair();

//             // adding new erc20 coin to bob's erc20 coin list

//             tx = await carl.calls.erc1155Vault.deposit(
//               [carlDepositAmount, FT_ID,
//               carlErc1155Key.publicKey]
//             );

//             cmt = await testHelpers.getCommitmentFromTx(tx);
//             merkleTree1155.insertLeaves([cmt]);
//             let carlProof = merkleTree1155.generateProof(cmt);

//             carl.coins.push({    "vaultId": 2,
//                                 "tokenId": FT_ID,
//                                 "value":carlDepositAmount, 
//                                 "tokenAddress": contracts.erc1155.address,
//                                 "key":carlErc1155Key, 
//                                 "commitment":cmt, 
//                                 "proof":carlProof, 
//                                 "root":merkleTree1155.root, 
//                                 "treeNumber":merkleTree1155.lastTreeNumber,
//                                 "groupRoot": merkleTreeFungibles.root,
//                                 "groupProof": merkleTreeFungibles.generateProof(groupUniqueIds[0]),
//                             });


//         }
//         console.log("----------------------------")
//         // console.log("Alice.coins:\n", JSON.stringify(alice.coins, null, 4)  + "\n");
//         // console.log("Bob.coins:\n", JSON.stringify(bob.coins, null, 4) + "\n");
//         // console.log("Carl.coins:\n", JSON.stringify(carl.coins, null, 4) + "\n");

//   });

//   it("Alice should be able to start the auction.", async () => {

//         let aliceRecFundKey = jsUtils.newKeyPair();
//         const beacon = 0n; // TODO:: connect beacon

//         const assetGroup_merkleRoot = merkleTreeNonFungibles.root;
//         const assetGroup_merkleProof = merkleTreeNonFungibles.generateProof(groupUniqueIds[1]);

//         // console.log("assetGroup_merkleProof: " , JSON.stringify(assetGroup_merkleProof, null, 4));

//         const auctionInitProof = await prover.AuctionInitProof(
//               beacon,
//               alice.coins[0].tokenId,
//               alice.coins[0].tokenAddress,
//               alice.coins[0].key,
//               TREE_DEPTH,
//               alice.coins[0].proof,
//               alice.coins[0].root,
//               alice.coins[0].treeNumber,
//               2n, // vaultId
//               assetGroup_merkleRoot,
//               assetGroup_merkleProof,
//               [alice.coins[0].value, alice.coins[0].tokenId, 0n,0n,0n]
//           );

//         console.log("auctionInitProof: \n", JSON.stringify(auctionInitProof, null, 4));
//         const newAuctionTx = await alice.calls.zkAuction.newAuction(
//                                         [alice.coins[0].value, alice.coins[0].tokenId],
//                                         2, // itemVaultId
//                                         2, // bidVaultId
//                                         1, // non-fungibles assetGroup id
//                                         0, // fungibles assetGroup id
//                                         aliceRecFundKey.publicKey,
//                                         auctionInitProof
//                                     );
//         auctionData = await testHelpers.parseAuctionInitEvent(newAuctionTx);

//         console.log("AuctionData from Event: ", JSON.stringify(auctionData, null, 4));
//         auctionData.sellerKey = aliceRecFundKey;

//         // TODO:: winner should know this
//         auctionData.itemTokenId = alice.coins[0].value;

//   });

//   it("Alice should NOT be able to re-start the auction.", async () => {

//     // TODO:: implement
//   });

//   it("Bob should be able to Bid with his deposited coins.", async () => {

//     let bobBidAmount = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(3))) % jsUtils.SNARK_SCALAR_FIELD;
    
//     // if bid > totalDeposit the proof wont pass
//     while(bobBidAmount > (bob.coins[0].value + bob.coins[1].value)){
//         bobBidAmount = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(3))) % jsUtils.SNARK_SCALAR_FIELD;
//     }
//     let bobBidRandom = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(16))) % jsUtils.SNARK_SCALAR_FIELD;

//     let bobExcessFundKey = jsUtils.newKeyPair();
//     let bobItemReceivingKey = jsUtils.newKeyPair();

//     let bobExcessAmount = (bob.coins[0].value + bob.coins[1].value) - bobBidAmount;

//     console.log(`Bob's bid values: ${bobBidAmount}, ${bobExcessAmount}`);

//     const assetGroup_merkleRoot = merkleTreeFungibles.root;
//     const assetGroup_merkleProof = merkleTreeFungibles.generateProof(groupUniqueIds[0]);

//     const bobBidProof = await prover.AuctionBidProof(
//           auctionData.auctionId,
//           bobBidAmount,
//           bobBidRandom,
//           bob.coins[0].tokenAddress,
//           [bob.coins[0].value, bob.coins[1].value],
//           [bob.coins[0].key, bob.coins[1].key],
//           [bobBidAmount, bobExcessAmount] ,
//           [{"publicKey": auctionData.sellerFundCoinPublicKey}, {"publicKey": bobExcessFundKey.publicKey}],
//           TREE_DEPTH,
//           [bob.coins[0].proof, bob.coins[1].proof],
//           [bob.coins[0].root, bob.coins[1].root],
//           [bob.coins[0].treeNumber, bob.coins[1].treeNumber],
//           2, //erc1155 vaultId
//           [[bob.coins[0].value, bob.coins[0].tokenId,0,0,0], [bob.coins[1].value, bob.coins[1].tokenId,0,0,0]],
//           [[bobBidAmount,bob.coins[0].tokenId,0,0,0], [bobExcessAmount,bob.coins[1].tokenId,0,0,0]],
//           assetGroup_merkleRoot,
//           assetGroup_merkleProof,
//           ) 


//         // console.log("Bob's bid proof: ", JSON.stringify(bobBidProof, null, 4));

//         const bobBidTx = await bob.calls.zkAuction.submitBid(bobBidProof, bobItemReceivingKey.publicKey);

//         // storing Bob's bid data locally
//         bob.bids.push({
//                         "auctionId": auctionData.auctionId,
//                         "amount": bobBidAmount,
//                         "random": bobBidRandom,
//                         "proof": bobBidProof,
//                         "receivingKey": bobItemReceivingKey,
//                         "excessKey": bobExcessFundKey,
//                         "excessAmount": bobExcessAmount,
//                         "blindedBid": bobBidProof.statement[1]
//                         });

//         auctioneer.bidLogs.push({... bob.bids[0]});

//   });

//   it("Carl should be able to Bid.", async () => {


//     let carlBidAmount = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(3))) % jsUtils.SNARK_SCALAR_FIELD;
//     // if bid > totalDeposit the proof wont pass
//     while(carlBidAmount > (carl.coins[0].value + carl.coins[1].value)){
//         carlBidAmount = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(3))) % jsUtils.SNARK_SCALAR_FIELD;
//     }

//     let carlBidRandom = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(16))) % jsUtils.SNARK_SCALAR_FIELD;

//     let carlExcessFundKey = jsUtils.newKeyPair();
//     let carlItemReceivingKey = jsUtils.newKeyPair();

//     const carlExcessAmount = (carl.coins[0].value + carl.coins[1].value) - carlBidAmount;
//     console.log(`Carls's bid values: ${carlBidAmount}, ${carlExcessAmount}`);

//     const assetGroup_merkleRoot = merkleTreeFungibles.root;
//     const assetGroup_merkleProof = merkleTreeFungibles.generateProof(groupUniqueIds[0]);

//     const carlBidProof = await prover.AuctionBidProof(
//             auctionData.auctionId,
//             carlBidAmount,
//             carlBidRandom,
//             carl.coins[0].tokenAddress,
//             [carl.coins[0].value, carl.coins[1].value],
//             [carl.coins[0].key, carl.coins[1].key],
//             [carlBidAmount, carlExcessAmount] ,
//             [{"publicKey": auctionData.sellerFundCoinPublicKey}, {"publicKey": carlExcessFundKey.publicKey}],
//             TREE_DEPTH,
//             [carl.coins[0].proof, carl.coins[1].proof],
//             [carl.coins[0].root, carl.coins[1].root],
//             [carl.coins[0].treeNumber, carl.coins[1].treeNumber],
//           2, //erc1155 vaultId
//           [
//             [
//               carl.coins[0].value, 
//               carl.coins[0].tokenId,
//               0,
//               0,
//               0
//             ], 
//             [
//               carl.coins[1].value, 
//               carl.coins[1].tokenId,
//               0,
//               0,
//               0
//             ]
//           ],
//           [
//             [
//               carlBidAmount,
//               carl.coins[0].tokenId,
//               0,
//               0,
//               0
//             ], 
//             [
//               carlExcessAmount,
//               carl.coins[1].tokenId,
//               0,
//               0,
//               0
//             ]
//           ],
//           assetGroup_merkleRoot,
//           assetGroup_merkleProof,
//           ) 

//       // console.log("Carl's bid proof: ", JSON.stringify(carlBidProof, null, 4));

//       const carlBidTx = await carl.calls.zkAuction.submitBid(carlBidProof, carlItemReceivingKey.publicKey);

//       // storing Bob's bid data locally
//       carl.bids.push({
//                       "auctionId": auctionData.auctionId,
//                       "amount": carlBidAmount,
//                       "random": carlBidRandom,
//                       "proof": carlBidProof,
//                       "receivingKey": carlItemReceivingKey,
//                       "excessKey": carlExcessFundKey,
//                       "excessAmount": (carl.coins[0].value + carl.coins[1].value) - carlBidAmount,
//                       "blindedBid": carlBidProof.statement[1]
//                       });
//       auctioneer.bidLogs.push({... carl.bids[0]});


//   });

//   it("Carl should be able to open the blindedBid publicly on-chain.", async () => {

//         // console.log("Carl's bid: ", JSON.stringify(carl.bids[0], null, 4));
//         const bidInfo = await carl.calls.zkAuction.getBid(carl.bids[0].auctionId, carl.bids[0].blindedBid);
//         const bidInfoState = bidInfo[0];
//         await expect(bidInfo[0]).to.equal(1); //BID_SEALED state
 
//         const publicOpeningTx = await carl.calls.zkAuction.publicOpeningReceipt(  
//             carl.bids[0].auctionId,
//             carl.bids[0].amount,
//             carl.bids[0].random
//         )

//         const bidInfo2 = await carl.calls.zkAuction.getBid(carl.bids[0].auctionId, carl.bids[0].blindedBid);
//         const bidInfo2State = bidInfo2[0];
//         await expect(bidInfo2[0]).to.equal(2); //BID_OPENED_PUBLICLY

//         const bidInfo2Struct = {
//                                     "bidState": bidInfo2State,
//                                     "blindedBid": bidInfo2[1].toBigInt(),
//                                     "bidAmount": bidInfo2[2].toBigInt(),
//                                     "bidRandom": bidInfo2[3].toBigInt(),
//                                 };
//         console.log("Carl's bid data after opening: ", JSON.stringify(bidInfo2Struct, null, 4));


//         console.log("Auctioneer listens to AuctionBidOpenedPublicly event");
//         const openingData = await testHelpers.parseAuctionPublicOpeningEvent(publicOpeningTx);

//         auctioneer.openings.push({     
//                                         "blindedBid": openingData.blindedBid,
//                                         "auctionId": openingData.auctionId,
//                                         "amount": openingData.bidAmount,
//                                         "random": openingData.bidRandom
//                                     });

//   });


//   it("Bob should be able to open the blindedBid privately to auctioneer.", async () => {

//         const bidInfo = await bob.calls.zkAuction.getBid(bob.bids[0].auctionId, bob.bids[0].blindedBid);
//         const bidInfoState = bidInfo[0];
//         await expect(bidInfo[0]).to.equal(1); //BID_SEALED state

//         console.log("Bob privately sends his bid's opening info to auctioneer");
//         const receivedAuctionId = bob.bids[0].auctionId;
//         const receivedBlindedBid = bob.bids[0].blindedBid;
//         const receivedBidAmount = bob.bids[0].amount;
//         const receivedBidRandom = bob.bids[0].random;

//         await expect(
//                     jsUtils.pedersen(
//                             receivedBidAmount, 
//                             receivedBidRandom)
//                     ).to.equal(receivedBlindedBid);

//         console.log("Auctioneer should confirm the auctionId and bid state from on-chain data. [NOT IMPlEMENTED]");

//         auctioneer.openings.push( {     
//                                         "blindedBid": receivedBlindedBid,
//                                         "auctionId": receivedAuctionId,
//                                         "amount": receivedBidAmount,
//                                         "random": receivedBidRandom
//                                     });


//         console.log(auctioneer.openings[1]);

//         console.log("Auctioneer generates privateOpeningProof");
//         const privateOpeninigProof = await prover.AuctionPrivateOpeningProof(
//               auctioneer.openings[1].auctionId,
//               auctioneer.openings[1].blindedBid,
//               auctioneer.openings[1].amount,
//               auctioneer.openings[1].random
//           ) 

//         console.log("Auctioneer sends privateOpeningProof on-chain");
//         // console.log(privateOpeninigProof);
 
//         await auctioneer.calls.zkAuction.privateOpeningReceipt(privateOpeninigProof);

//         const bidInfo2 = await bob.calls.zkAuction.getBid(bob.bids[0].auctionId, bob.bids[0].blindedBid);
//         const bidInfo2State = bidInfo2[0];
//         await expect(bidInfo2[0]).to.equal(3); //BID_OPENED_PRIVATELY

//         const bidInfo2Struct = {
//                                     "bidState": bidInfo2State,
//                                     "blindedBid": bidInfo2[1].toBigInt(),
//                                     "bidAmount": bidInfo2[2].toBigInt(),
//                                     "bidRandom": bidInfo2[3].toBigInt(),
//                                 };
//         console.log("Bob's bid data after private opening: ", JSON.stringify(bidInfo2Struct, null, 4));


//   });


//   it("Auctioneer should declare the correct winner.", async () => {

//     const sortedBids = auctioneer.openings.sort((a,b) =>{
//           if(a.amount < b.amount) {
//             return 1;
//           } else if (a.amount > b.amount){
//             return -1;
//           } else {
//             return 0;
//           }
//         });

//     // console.log("Sorted bids: \n", JSON.stringify(sortedBids, null, 4));

//     const winningBidData = sortedBids[0];
//     const winningBidAmount = winningBidData.amount;
//     const winningBidRandom = winningBidData.random;
//     const auctionId = winningBidData.auctionId;


//     // TODO:: the test is only for one auctionId
//     // some structure should be added to group bids of each
//     // auction Ids.

//     const notWinningProofs = [];
//     for(var i = 1; i < sortedBids.length; i++){

//         // TODO:: connect the blockNumbers
//         const st_bidBlockNumber = 0n;
//         const st_winningBidBlockNumber = 0n;
//         const currentBidAmount = sortedBids[i].amount;
//         const currentBidRandom = sortedBids[i].random;


//         const newProof = await prover.AuctionNotWinningBidProof(
//               auctionId,
//               st_bidBlockNumber,
//               st_winningBidBlockNumber,
//               currentBidAmount,
//               currentBidRandom,
//               winningBidAmount,
//               winningBidRandom
//           ) 

//         notWinningProofs.push(newProof);
//     }

//     const declareWinnerTx = await auctioneer.calls.zkAuction.declareWinner(
//                             auctionId, 
//                             winningBidAmount, 
//                             winningBidRandom, 
//                             notWinningProofs);

//     winnerParams = await testHelpers.parseAuctionConcludedEvent(declareWinnerTx);

//     console.log("AuctionConcluded event data: ", JSON.stringify(winnerParams, null, 4) );

//     await expect(auctionData.auctionId).to.equal(winnerParams.auctionId);

//     let winnerBid;
//     if(bob.bids[0].amount > carl.bids[0].amount){
//       winnerBid = bob.bids[0];
//       winnerParams.winnerName = "Bob";
//       winnerParams.bid = bob.bids[0];
//       winnerParams.address = bob.wallet.address;

//       auctionSummary.winner = "Bob";
//       auctionSummary.winnerBidAmount = bob.bids[0].amount;
//       auctionSummary.loser = "Carl";
//       auctionSummary.loserBidAmount = carl.bids[0].amount;
//       winnerParams.user = bob;

//     }
//     else{
//       winnerBid = carl.bids[0];
//       winnerParams.winnerName = "Carl";
//       winnerParams.bid = carl.bids[0];
//       winnerParams.address = carl.wallet.address;
//       winnerParams.user = carl;


//       auctionSummary.winner = "Carl";
//       auctionSummary.winnerBidAmount = carl.bids[0].amount;
//       auctionSummary.loser = "Bob";
//       auctionSummary.loserBidAmount = bob.bids[0].amount;
//     }

//     await expect(winnerBid.blindedBid).to.equal(winnerParams.winningBlindedBid);
//     await expect(winnerBid.bidAmount).to.equal(winnerParams.winningBidAmount);
//     await expect(winnerBid.bidRandom).to.equal(winnerParams.winningBidRandom);


//     merkleTree1155.insertLeaves([winnerParams.commitments[0], winnerParams.commitments[1]]);
//     merkleTree1155.insertLeaves([winnerParams.commitments[2]]);

//     console.log(`Winner (${winnerParams.winnerName}) checks commitments to see the commitments that is generated by her receivingKey.`);

//   });

//   it("Winner should be able to withdraw the ERC1155 NFT item.", async () => {

//     console.log(`Winner (${winnerParams.winnerName}) withdraws Non-fungible Erc1155 item.`);
//     assetGroup_merkleRoot = merkleTreeNonFungibles.root;
//     assetGroup_merkleProof = merkleTreeNonFungibles.generateProof(groupUniqueIds[1]);

//     const winnerMerkleProof = merkleTree1155.generateProof(winnerParams.commitments[2]);
//     console.log("auctionData.itemUniqueId: ", auctionData.itemUniqueId);
//     const winnerTx = await userActions.generateSingleErc1155Proof(
//       0n,
//       1n,
//       winnerParams.bid.receivingKey,
//       { publicKey: BigInt(winnerParams.address) },
//       TREE_DEPTH,
//       winnerMerkleProof,
//       winnerMerkleProof.root,
//       merkleTree1155.lastTreeNumber,
//       contracts.erc1155.address,
//       NFT_ID,
//       assetGroup_merkleRoot,
//       assetGroup_merkleProof,
//       false,
//     );

//     // console.log("Winner proof: ", JSON.stringify(winnerTx, null, 4));
//     console.log("Winner knows the id of the token she won. [TODO::check it.]");
//     // TX sent by a relayer
//     await contracts.erc1155CoinVault.withdraw([1n, NFT_ID], winnerParams.address, winnerTx);
//     const res = await contracts.erc1155.balanceOf(winnerParams.address, NFT_ID);
//     expect(res).to.equal(1n);

//   });


//   it("Winner should be able to withdraw the excess Fungible Erc1155 coin.", async () => {

//     console.log(`Winner ${winnerParams.winnerName} withdraws excess Fungible Erc1155 tokens.`);

//     const dummyKey = jsUtils.newKeyPair();
//     const winnerMerkleProof = merkleTree1155.generateProof(winnerParams.commitments[1]);
//     assetGroup_merkleRoot = merkleTreeFungibles.root;
//     assetGroup_merkleProof = merkleTreeFungibles.generateProof(groupUniqueIds[0]);


//     const winnerTx = await userActions.generateSingleErc1155Proof(
//       0n,
//       winnerParams.bid.excessAmount,
//       winnerParams.bid.excessKey,
//       { publicKey: BigInt(winnerParams.address) },
//       TREE_DEPTH,
//       winnerMerkleProof,
//       winnerMerkleProof.root,
//       merkleTree1155.lastTreeNumber,
//       contracts.erc1155.address,
//       FT_ID,
//       assetGroup_merkleRoot,
//       assetGroup_merkleProof,
//       true,
//     );
//     const oldBalance = (
//       await contracts.erc1155.balanceOf(winnerParams.address, FT_ID)
//     ).toBigInt();


//     await contracts.erc1155CoinVault.withdraw([winnerParams.bid.excessAmount, FT_ID], winnerParams.address, winnerTx);

//     const newBalance = (
//       await contracts.erc1155.balanceOf(winnerParams.address, FT_ID)
//     ).toBigInt();
//     await expect(oldBalance + winnerParams.bid.excessAmount).to.equal(newBalance);

//   });


//   it("Seller should be able to withdraw the fungible Erc1155 price.", async () => {

//     const dummyKey = jsUtils.newKeyPair();
//     const sellerMerkleProof = merkleTree1155.generateProof(winnerParams.commitments[0]);

//     assetGroup_merkleRoot = merkleTreeFungibles.root;
//     assetGroup_merkleProof = merkleTreeFungibles.generateProof(groupUniqueIds[0]);


//     const sellerTx = await userActions.generateSingleErc1155Proof(
//       0n,
//       winnerParams.bid.amount,
//       auctionData.sellerKey,
//       { publicKey: BigInt(alice.wallet.address) },
//       TREE_DEPTH,
//       sellerMerkleProof,
//       sellerMerkleProof.root,
//       merkleTree1155.lastTreeNumber,
//       contracts.erc1155.address,
//       FT_ID,
//       assetGroup_merkleRoot,
//       assetGroup_merkleProof,
//       true,
//     );

//     const oldBalance = (
//       await contracts.erc1155.balanceOf(alice.wallet.address, FT_ID)
//     ).toBigInt();


//     await contracts.erc1155CoinVault.withdraw([winnerParams.bid.amount, FT_ID], alice.wallet.address, sellerTx);

//     const newBalance = (
//       await contracts.erc1155.balanceOf(alice.wallet.address, FT_ID)
//     ).toBigInt();
//     await expect(oldBalance + winnerParams.bid.amount).to.equal(newBalance);

//   });


//   it("Losers should be able to withdraw the locked coins.", async () => {

//     const dummyKey = jsUtils.newKeyPair();

//     let loserBid;
//     let loserCommitments;
//     let loserAddress;
//     let loserCoins;
//     let loserName;
//     let winnerName;
//     let winnerBid;
//     if(bob.bids[0].amount > carl.bids[0].amount){
//       // carl is the loser
//       loserName = "Carl";
//       loserBid = carl.bids[0];
//       loserCoins = [carl.coins[0], carl.coins[1]];
//       loserAddress = carl.wallet.address;

//       winnerName = "Bob";

//     }
//     else{
//       // bob is the loser
//       loserBid = bob.bids[0];
//       loserCoins = [bob.coins[0], bob.coins[1]];
//       loserAddress = bob.wallet.address;
//     }


//     const oldBalance = (
//       await contracts.erc1155.balanceOf(loserAddress,FT_ID)
//     ).toBigInt();
//     assetGroup_merkleRoot = merkleTreeFungibles.root;
//     assetGroup_merkleProof = merkleTreeFungibles.generateProof(groupUniqueIds[0]);

//     const loserTx = await userActions.generateErc1155JoinSplitProof(
//       0n,
//       [loserCoins[0].value, loserCoins[1].value],
//       [loserCoins[0].key, loserCoins[1].key],
//       [loserCoins[0].value + loserCoins[1].value, 0n],
//       [{ publicKey: BigInt(loserAddress) }, dummyKey],
//       TREE_DEPTH,
//       [loserCoins[0].proof, loserCoins[1].proof],
//       [loserCoins[0].root, loserCoins[1].root],
//       [loserCoins[0].treeNumber, loserCoins[1].treeNumber],
//       contracts.erc1155.address,
//       FT_ID,
//       assetGroup_merkleRoot,
//       assetGroup_merkleProof,
//     );


//         await contracts.erc1155CoinVault.withdraw(
//                 [
//                   loserCoins[0].value + loserCoins[1].value,
//                   0,
//                   FT_ID,
//                   0
//                 ], 
//                 loserAddress, 
//                 loserTx
//               );

//     const newBalance = (
//       await contracts.erc1155.balanceOf(loserAddress, FT_ID)
//     ).toBigInt();
//     await expect(oldBalance + loserCoins[0].value + loserCoins[1].value).to.equal(newBalance);


//     console.log("--------------------------------");
//     console.log("--------------------------------");
//     console.log("     Auction summary");
//     console.log(JSON.stringify(auctionSummary, null, 4));
//   });


// });