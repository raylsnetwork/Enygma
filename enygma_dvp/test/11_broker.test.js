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
// let relayer = {};
// let owner = {};

// let NFT_ID;
// let FT_ID;
// let groupUniqueIds = [];
// let bobDepositAmount;

// let tempStorage = [];

// describe("Broker v1 Test", () => {

//     it(`ZkDvp should initialize properly `, async () => {
        
//         let userCount = 3; // Alice:seller, Bob and Carl: Bidders
//         [admin, users, contracts, merkleTrees] = await testHelpers.deployForTest(userCount);

//         alice.wallet = users[0].wallet;
//         bob.wallet =   users[1].wallet;
//         carl.wallet =  users[2].wallet;

//         owner.wallet = admin

//         relayer.wallet = admin;
//         relayer.calls = {};
//         relayer.calls.zkdvp = contracts.zkdvp.connect(relayer.wallet);
//         relayer.receivedProofs = [];


//         alice.calls = {};
//         bob.calls = {};
//         carl.calls = {};
//         owner.calls = {};

//         alice.coins = [];
//         bob.coins = [];
//         carl.coins = [];

//         alice.knows = {};
//         bob.knows = {};
//         carl.knows = {};

//         owner.calls.erc1155 = contracts.erc1155.connect(owner.wallet);

//         alice.calls.zkdvp = contracts.zkdvp.connect(alice.wallet);
//         alice.calls.erc1155 = contracts.erc1155.connect(alice.wallet);
//         alice.calls.erc1155Vault = contracts.erc1155CoinVault.connect(alice.wallet);

//         bob.calls.zkdvp = contracts.zkdvp.connect(bob.wallet);
//         bob.calls.erc1155 = contracts.erc1155.connect(bob.wallet);
//         bob.calls.erc1155Vault = contracts.erc1155CoinVault.connect(bob.wallet);

//         carl.calls.zkdvp = contracts.zkdvp.connect(carl.wallet);
//         carl.calls.erc1155 = contracts.erc1155.connect(carl.wallet);
//         carl.calls.erc1155Vault = contracts.erc1155CoinVault.connect(carl.wallet);

//         merkleTree1155 = merkleTrees["ERC1155"].tree;
//         merkleTreeFungibles = merkleTrees["fungibleGroup"].tree;
//         merkleTreeNonFungibles = merkleTrees["nonFungibleGroup"].tree;

//         console.log("Broker v1 Test: Done ZkDvp initialization")

//     });


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

//         console.log("Broker v1 Test: Erc1155 tokens have been registered.");

//     });



//     it("Admin mints non-fungible Erc1155 for Alice and fungible Erc1155 for Bob.", async () => {
//         await owner.calls.erc1155.mint(
//             alice.wallet.address, 
//             NFT_ID, 
//             1 , 
//             0
//         );

//         console.log("Minted non-fungible token for Alice with tokenID = ", NFT_ID);
//         bobDepositAmount = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(4)));

//         await owner.calls.erc1155.mint(
//             bob.wallet.address, 
//             FT_ID, 
//             bobDepositAmount * 2n , 
//             0
//         );

//         console.log("Minted fungible token for Bob with tokenID = ", FT_ID);

//     });

//     it("Alice should deposit a non-fungible Erc1155 token.", async () => {
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
//                             "groupTreeNumber": merkleTreeNonFungibles.lastTreeNumber,
//                             "groupProof": merkleTreeNonFungibles.generateProof(groupUniqueIds[1])
//                         });
    
//         alice.knows.item = {};
//         alice.knows.item.tokenId = NFT_ID;
//         alice.knows.item.tokenAddress = contracts.erc1155.address;

//     });

//     it("Bob should deposit fungible ERC1155 tokens in form of two coins.", async () => {
//         console.log("Depositing two fungible erc1155 coins for Bob");

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
//                                 "groupTreeNumber": merkleTreeFungibles.lastTreeNumber,
//                                 "groupProof": merkleTreeFungibles.generateProof(groupUniqueIds[0])
//                             });

//         }

//     });

//     it("Bob registers Carl to be his broker.", async () => {

//         console.log("Bob wants to register Carl as his broker.");

//         console.log("Carl creates a new keyPair and shares the publicKey with Bob. Also they agee upon commission rate range");
//         let carlRecBrokerageKey = jsUtils.newKeyPair();

//         carl.knows.carlsMinCommissionRate = 10
//         carl.knows.carlsMaxCommissionRate = 15

//         bob.knows.carlRecBrokeragePublicKey = carlRecBrokerageKey.publicKey;
//         // putting it in carl.knows to share it later with Alice
//         carl.knows.carlRecBrokerageKey = carlRecBrokerageKey;
//         bob.knows.carlsMinCommissionRate = carl.knows.carlsMinCommissionRate
//         bob.knows.carlsMaxCommissionRate = carl.knows.carlsMaxCommissionRate

//         console.log("Bob generates BrokerRegistration proof.")

//         const beacon = 0n; // TODO:: connect beacon

//         // TODO:: connect assetGroup treeNumber
//         const assetGroup_treeNumber = merkleTreeFungibles.lastTreeNumber;
//         const assetGroup_merkleProof = merkleTreeFungibles.generateProof(groupUniqueIds[0]);

//         const brokerRegistrationProof = await prover.BrokerRegistrationProof(
//               beacon,
//               2n, // vaultId
//               0n, // groupId of fungibles
//               [bob.coins[0].key, bob.coins[1].key],
//               TREE_DEPTH,
//               [bob.coins[0].treeNumber, bob.coins[1].treeNumber],
//               [bob.coins[0].proof, bob.coins[1].proof],
//               [
//                 [bob.coins[0].value, bob.coins[0].tokenId, 0n,0n,0n],
//                 [bob.coins[1].value, bob.coins[1].tokenId, 0n,0n,0n]
//               ],
//               bob.knows.carlRecBrokeragePublicKey,
//               contracts.erc1155.address,
//               assetGroup_treeNumber,
//               assetGroup_merkleProof,
//               bob.knows.carlsMinCommissionRate,
//               bob.knows.carlsMaxCommissionRate
//           );

//         console.log("Bob sends the proof to a relayer and the relayer submit it on-chain.");
//         relayer.receivedProofs.push(brokerRegistrationProof);
//         const lastProofIndex = relayer.receivedProofs.length - 1;
//         console.log("Received proof: ", JSON.stringify(relayer.receivedProofs[lastProofIndex], null, 4));
//         await relayer.calls.zkdvp.registerBroker(relayer.receivedProofs[lastProofIndex]);
//         console.log("Carl sees BrokerRegistered event and checks the broker's blinded publicKey.");

//     });

//     it("[[OFF-CHAIN]] [[NOT-IMPLEMENTED]] Alice registers her coin to the ZkMarket anonymously. Carl finds the Alice's anonymous sell offer and shares her blindedPublicKey with Alice.", async () => {
    
//         alice.knows.carlBlindedPublicKey = jsUtils.blindedPublicKey(carl.knows.carlRecBrokerageKey.publicKey);
    
//         console.log(`Carl shares the blinded publicKey ${alice.knows.carlBlindedPublicKey} with Alice.`)
//     });

//     it("[[OFF-CHAIN]] Carl proves to Alice that she is a registered broker. ", async () => {

//     });

//     it("Alice sends a fresh Proof Of Ownership with message = blindedPublicKey.", async () => {

//         const message = alice.knows.carlBlindedPublicKey;
//         console.log("alice's proof message = ", message);

//         // Alice constructs a proof of ownership  and put message = challenge
//         console.log("Alice constructs a proof with output address = 0 and message = blindedPublicKey");
//         const assetGroup_merkleProof = merkleTreeNonFungibles.generateProof(groupUniqueIds[1]);
//         const assetGroup_treeNumber = merkleTreeNonFungibles.lastTreeNumber;

//         const verifyOwnProof = await prover.Erc1155NonFungibleProof(
//           message,
//           [1n],
//           [alice.coins[0]["key"]],
//           [1n],
//           [{"publicKey": 0n}],
//           TREE_DEPTH,
//           alice.coins[0]["proof"],
//           alice.coins[0]["root"],
//           alice.coins[0]["treeNumber"],
//           contracts.erc1155.address,
//           alice.coins[0].tokenId,
//             [assetGroup_treeNumber],
//             [assetGroup_merkleProof],
//         );

//         console.log("Alice calls (through relayer) erc1155CoinVault.verifyOwnership() with the constructed proof and coin's attributes.");

//         tx = await alice.calls.erc1155Vault.verifyOwnership(
//           [1n, alice.coins[0].tokenId],
//           verifyOwnProof
//         );

//         const verifyReceipt = await testHelpers.parseVerifyOwnershipReceipt(tx);

//         console.log("Alice's verifyOwnership receipt: ", JSON.stringify(verifyReceipt, null, 4));

//         console.log("Carl sees the event, identifies her blindedPublicKey and checks the validity of the coin");
    

//     });

//     it("Carl sends a fresh proof of knowledge of privateKey corresponding to Carl's blindedPublicKey", async () => {

//         const st_beacon = 0n;

//         console.log("Carl constructs a proof of legit Broker");

//         const legitBrokerProof = await prover.LegitBrokerProof(
//           st_beacon,
//           carl.knows.carlRecBrokerageKey.privateKey
//         );

//         const tx = await carl.calls.zkdvp.verifyLegitBrokerReceipt(legitBrokerProof);
//         const legitReceipt = await testHelpers.parseLegitBrokerReceipt(tx);
//         console.log("Carl's LegitBrokerReceipt: ", JSON.stringify(legitReceipt, null, 4));

//         console.log("Alice sees the LegitBrokerReceipt event");
        
//         console.log("Alice recognizes the blinded publicKey");
//         expect(BigInt(legitReceipt.blindedBrokerPublicKey)).to.equal(BigInt(alice.knows.carlBlindedPublicKey));

//     });

//     it("Carl contacts Bob, tells him Alice's price offer. Bob finds the price reasonable as well as the brokerage percentage Carl is offering.", async () => {
//         bob.knows.item = {};
//         bob.knows.item.price = 100n;
//         bob.knows.item.tokenId = alice.knows.item.tokenId;
//         bob.knows.item.tokenAddress = alice.knows.item.tokenAddress;

//         bob.knows.payment = {};
//         bob.knows.payment.amount = bob.knows.item.price;
//         bob.knows.payment.tokenId = bob.coins[0].tokenId;
//         bob.knows.payment.tokenAddress = bob.coins[0].tokenAddress;

//         alice.knows.item.price = 100n;
//         alice.knows.payment = bob.knows.payment;


//         carl.knows.brokerageFee = 5n;
//         carl.knows.commission = {};
//         carl.knows.commission.fee = 5n;
//         carl.knows.commission.tokenId = bob.coins[0].tokenId;
//         bob.knows.brokerageFee = 5n;
//         bob.knows.brokeragePercentage = 5n;
//         carl.knows.brokeragePercentage = 5n;


//         const aliceRecCoinKey = jsUtils.newKeyPair();
//         alice.knows.aliceRecCoinKey = aliceRecCoinKey;
//         bob.knows.aliceRecPublicKey = aliceRecCoinKey.publicKey;
        
//         const bobRecCoinKey = jsUtils.newKeyPair();
//         bob.knows.bobRecCoinKey = bobRecCoinKey;
//         alice.knows.bobRecPublicKey = bobRecCoinKey.publicKey;
//     });

//     it("Bob generates the proper proof with information about Alice's coin, brokerage percentage and a publickey from Carl to generate new coin for her. Bob sends it to Relayer. Relayer submits it on-chain", async () => {

//         const brokerageFee = bob.knows.brokerageFee;
//         const excessAmount = (bob.coins[0].value + bob.coins[1].value) - 
//                                 (bob.knows.item.price + brokerageFee);
        
//         const uniqueId = jsUtils.erc1155UniqueId(bob.knows.item.tokenAddress, bob.knows.item.tokenId, 1n);
//         const paymentProofMessage = jsUtils.getCommitment(uniqueId, bob.knows.bobRecCoinKey.publicKey);
//         let bobExcessCoinKey = jsUtils.newKeyPair();
//         bob.knows.bobExcessCoinKey = bobExcessCoinKey;
//         bob.knows.bobExcessAmount = excessAmount;

//         const paymentProof = await prover.Erc1155FungibleWithBrokerV1Proof(
//                                 paymentProofMessage,
//                                 [
//                                     bob.coins[0].value, 
//                                     bob.coins[1].value
//                                 ],
//                                 [
//                                     bob.coins[0].key, 
//                                     bob.coins[1].key
//                                 ],
//                                 [
//                                     bob.knows.item.price, 
//                                     excessAmount,
//                                     brokerageFee
//                                 ],
//                                 [
//                                     {"publicKey": bob.knows.aliceRecPublicKey},
//                                     {"publicKey": bobExcessCoinKey.publicKey},
//                                     {"publicKey": bob.knows.carlRecBrokeragePublicKey}
//                                 ],
//                                 TREE_DEPTH,
//                                 [bob.coins[0].treeNumber, bob.coins[1].treeNumber],
//                                 [bob.coins[0].proof, bob.coins[1].proof],
//                                 contracts.erc1155.address,
//                                 bob.coins[0].tokenId,
//                                 bob.coins[0]["groupTreeNumber"],
//                                 bob.coins[0]["groupProof"],
//                                 bob.knows.carlRecBrokeragePublicKey,
//                                 bob.knows.brokeragePercentage
//                             );


        
//         await relayer.calls.zkdvp.submitPartialSettlement(paymentProof, 2, 0);

//         for(var i = 0; i < paymentProof.numberOfOutputs; i++){
//             tempStorage.push(paymentProof.statement[1 + i + 3 * paymentProof.numberOfInputs])
//         }
//     });

//     it("Carl checks the submitted proof, she finds the percentage and the new coin satisfying. She creates broker proof, sends it to the Relayer. Relayer submits it on-chain.", async () => {

//     });

//     it("Alice checks two proofs and submit hers. ZkDvp does the settlement", async () => {
//         const uniqueId = jsUtils.erc1155UniqueId(
//                             alice.knows.payment.tokenAddress, 
//                             alice.knows.payment.tokenId, 
//                             alice.knows.payment.amount
//                         );
//         const deliveryProofMessage = jsUtils.getCommitment(uniqueId, alice.knows.aliceRecCoinKey.publicKey);

//         const deliveryProof = await prover.Erc1155NonFungibleProof(
//                                 deliveryProofMessage,
//                                 [1n],
//                                 [alice.coins[0].key],
//                                 [1n],
//                                 [{"publicKey": alice.knows.bobRecPublicKey}],
//                                 TREE_DEPTH,
//                                 alice.coins[0].proof,
//                                 alice.coins[0].proof.root,
//                                 alice.coins[0].treeNumber,
//                                 contracts.erc1155.address,
//                                 alice.coins[0].tokenId,
//                                 [alice.coins[0].groupTreeNumber],
//                                 [alice.coins[0].groupProof]
//                             );

//         const tx = await relayer.calls.zkdvp.submitPartialSettlement(deliveryProof, 2, 1);
//         // TODO:: get the commmitments from on-chain event

//         const deliveryCommitment = deliveryProof.statement[1 + 3 * deliveryProof.numberOfInputs];
//         merkleTree1155.insertLeaves(tempStorage);


//         let fungibilityProof1 = merkleTreeFungibles.generateProof(groupUniqueIds[0]);
//         let fungibilityTreeNumber1 = merkleTreeFungibles.root;
//         alice.coins.push({"commitment": tempStorage[0],
//                           "proof": merkleTree1155.generateProof(tempStorage[0]),
//                           "root": merkleTree1155.root,
//                           "treeNumber": merkleTree1155.lastTreeNumber,
//                           "key": alice.knows.aliceRecCoinKey,
//                           "amount": alice.knows.item.price,
//                         "groupTreeNumber": fungibilityTreeNumber1,
//                         "groupProof": fungibilityProof1
//                         });
//        bob.coins.push({"commitment": tempStorage[1],
//                           "proof": merkleTree1155.generateProof(tempStorage[1]),
//                           "root": merkleTree1155.root,
//                           "treeNumber": merkleTree1155.lastTreeNumber,
//                           "key": bob.knows.bobExcessCoinKey,
//                           "amount": bob.knows.bobExcessAmount,
//                         "groupTreeNumber": fungibilityTreeNumber1,
//                         "groupProof": fungibilityProof1
//                         });

//        carl.coins.push({"commitment": tempStorage[2],
//                           "proof": merkleTree1155.generateProof(tempStorage[2]),
//                           "root": merkleTree1155.root,
//                           "treeNumber": merkleTree1155.lastTreeNumber,
//                           "key": carl.knows.carlRecBrokerageKey,
//                           "amount": carl.knows.brokerageFee,
//                         "groupTreeNumber": fungibilityTreeNumber1,
//                         "groupProof": fungibilityProof1
//                         });

//         merkleTree1155.insertLeaves([deliveryCommitment]);


//         let fungibilityProof2 = merkleTreeNonFungibles.generateProof(groupUniqueIds[1]);
//         let fungibilityTreeNumber2 = merkleTreeNonFungibles.root;
//         bob.coins.push({"commitment": deliveryCommitment,
//                         "amount": 1n,
//                         "tokenId": bob.knows.item.tokenId,
//                         "proof": merkleTree1155.generateProof(deliveryCommitment),
//                         "root": merkleTree1155.root,
//                         "treeNumber": merkleTree1155.lastTreeNumber,
//                         "key": bob.knows.bobRecCoinKey,
//                         "groupTreeNumber": fungibilityTreeNumber2,
//                         "groupProof": fungibilityProof2
//                       });
//     });

//     it("Bob should be able to withdraw the bougth non-fungible Erc1155 token.", async () =>{

//         // last saved coin for bob is the non-fungible one.

//         const boughtCoinIndex = bob.coins.length - 1;

//         const bobWithdrawTx = await userActions.generateSingleErc1155Proof(
//           0n,
//           1n,
//           bob.coins[boughtCoinIndex].key,
//           { publicKey: BigInt(bob.wallet.address) },
//           TREE_DEPTH,
//           bob.coins[boughtCoinIndex].proof,
//           bob.coins[boughtCoinIndex].proof.root,
//           merkleTree1155.lastTreeNumber,
//           contracts.erc1155.address,
//           NFT_ID,
//           bob.coins[boughtCoinIndex].groupTreeNumber,
//           bob.coins[boughtCoinIndex].groupProof,
//           false,
//         );

//         await contracts.erc1155CoinVault.withdraw([1n, NFT_ID], bob.wallet.address, bobWithdrawTx);
//         const res = await contracts.erc1155.balanceOf(bob.wallet.address, NFT_ID);
//         expect(res).to.equal(1n);

//         console.log("Asserted Bob has the ownership of the bought Erc1155 NFT.");


//     });

//     it("Bob should be able to withdraw the excess Fungible Erc1155 coin.", async () => {
        
//         // Bob's second last pushed coin
//         const boughtCoinIndex = bob.coins.length - 2;

//         // console.log(JSON.stringify())
//         const bobWithdrawTx = await userActions.generateSingleErc1155Proof(
//           0n,
//           bob.coins[boughtCoinIndex].amount,
//           bob.coins[boughtCoinIndex].key,
//           { publicKey: BigInt(bob.wallet.address) },
//           TREE_DEPTH,
//           bob.coins[boughtCoinIndex].proof,
//           bob.coins[boughtCoinIndex].proof.root,
//           merkleTree1155.lastTreeNumber,
//           contracts.erc1155.address,
//           bob.knows.payment.tokenId,
//           bob.coins[boughtCoinIndex].groupTreeNumber,
//           bob.coins[boughtCoinIndex].groupProof,
//           true
//         );

//         const beforeWithdraw = await contracts.erc1155.balanceOf(bob.wallet.address, bob.knows.payment.tokenId);

//         await contracts.erc1155CoinVault.withdraw([bob.coins[boughtCoinIndex].amount, bob.knows.payment.tokenId], bob.wallet.address, bobWithdrawTx);
//         const res = await contracts.erc1155.balanceOf(bob.wallet.address, bob.knows.payment.tokenId);
//         expect(BigInt(res) - BigInt(beforeWithdraw)).to.equal(bob.knows.bobExcessAmount);
//         console.log("Asserted Bob's withdrew amount to be equal to Bob's excess amount.");

//     });

//     it("Alice should be able to withdraw the fungible Erc1155 price.", async () => {
//         // Alice's last pushed coin
//         const boughtCoinIndex = alice.coins.length - 1;

//         // console.log(JSON.stringify())
//         const aliceWithdrawTx = await userActions.generateSingleErc1155Proof(
//           0n,
//           alice.coins[boughtCoinIndex].amount,
//           alice.coins[boughtCoinIndex].key,
//           { publicKey: BigInt(alice.wallet.address) },
//           TREE_DEPTH,
//           alice.coins[boughtCoinIndex].proof,
//           alice.coins[boughtCoinIndex].proof.root,
//           merkleTree1155.lastTreeNumber,
//           contracts.erc1155.address,
//           alice.knows.payment.tokenId,
//           alice.coins[boughtCoinIndex].groupTreeNumber,
//           alice.coins[boughtCoinIndex].groupProof,
//           true
//         );

//         const beforeWithdraw = await contracts.erc1155.balanceOf(alice.wallet.address, alice.knows.payment.tokenId);

//         await contracts.erc1155CoinVault.withdraw([alice.coins[boughtCoinIndex].amount, alice.knows.payment.tokenId], alice.wallet.address, aliceWithdrawTx);
//         const res = await contracts.erc1155.balanceOf(alice.wallet.address, alice.knows.payment.tokenId);
//         expect(BigInt(res) - BigInt(beforeWithdraw)).to.equal(alice.knows.item.price);

//         console.log("Asserted Alice's withdrew amount to be equal to item's price.");
//     });

//     it("Carl should be able to withdraw her new fungible coin.", async () => {
//         // Carl's last pushed coin
//         const boughtCoinIndex = carl.coins.length - 1;

//         // console.log(JSON.stringify())
//         const carlWithdrawTx = await userActions.generateSingleErc1155Proof(
//           0n,
//           carl.coins[boughtCoinIndex].amount,
//           carl.coins[boughtCoinIndex].key,
//           { publicKey: BigInt(carl.wallet.address) },
//           TREE_DEPTH,
//           carl.coins[boughtCoinIndex].proof,
//           carl.coins[boughtCoinIndex].proof.root,
//           merkleTree1155.lastTreeNumber,
//           contracts.erc1155.address,
//           carl.knows.commission.tokenId,
//           carl.coins[boughtCoinIndex].groupTreeNumber,
//           carl.coins[boughtCoinIndex].groupProof,
//           true
//         );

//         const beforeWithdraw = await contracts.erc1155.balanceOf(carl.wallet.address, carl.knows.commission.tokenId);

//         await contracts.erc1155CoinVault.withdraw([carl.coins[boughtCoinIndex].amount, carl.knows.commission.tokenId], carl.wallet.address, carlWithdrawTx);
//         const res = await contracts.erc1155.balanceOf(carl.wallet.address, carl.knows.commission.tokenId);
//         expect(BigInt(res) - BigInt(beforeWithdraw)).to.equal(carl.knows.commission.fee);

//         console.log("Asserted Carl's withdrew amount to be equal to commission fee.");
//     });


// });