/* global describe it ethers before beforeEach afterEach overwriteArtifact */
const { expect } = require("chai");
const poseidonGenContract = require("circomlibjs/src/poseidon_gencontract");
const prover = require("../src/core/prover");
const jsUtils = require("../src/core/utils");
const MerkleTree = require("../src/core/merkle");
const { getVerificationKeys } = require("../src/core/dvpSnarks");
const adminActions = require("./../src/core/endpoints/admin.js");
const userActions = require("./../src/core/endpoints/user.js");
const relayerActions = require("./../src/core/endpoints/relayer.js");
const crypto = require("crypto")
const myWeb3 = require("../src/web3");

let merkleTree1155;
let merkleTreeFungibles;
let merkleTreeNonFungibles;

const dvpConf = require("../zkdvp.config.json");
const TREE_DEPTH = dvpConf["tree-depth"];
const testHelpers = require("./testHelpers.js");

let aliceCoins = [];
let bobCoins = [];

let owner;
let users;
let contracts;
let merkleTrees;

let alice;
let bob;

let depositCount;
let swapCount;

let erc1155TokenIds = [100n, 101n, 101n, 102n, 103n, 104n];
let erc1155Amounts = [1n, 5000n, 5500n, 2000n, 1n, 1n];
let erc1155IsFungible = [false, true, true, true, false, false];
let erc1155GroupUniqueIds = [];
let inKeys = []

describe("ZkDvp Erc1155 Fungible and non-fungible Swap testing", () => {
    
    it(`ZkDvp should initialize properly `, async () => {


        let userCount = 2;
        [owner, users, contracts, merkleTrees] = await testHelpers.deployForTest(userCount);
        console.log("SwapTest: ZkDvp initialization");
        console.log("User: ",users);
        console.log("MerkleTree: ", merkleTrees);


        alice = users[0].wallet;
        bob = users[1].wallet;

        merkleTree1155 = merkleTrees["ERC1155"].tree;
        merkleTreeFungibles = merkleTrees["fungibleGroup"].tree;
        merkleTreeNonFungibles = merkleTrees["nonFungibleGroup"].tree;

      console.log("Batch Test: ZkDvp initialization")

    });


    it(`Admin should register erc1155 tokens properly. `, async () => {


        const erc1155Contract = contracts["erc1155"];
        const zkDvpContract = contracts["zkdvp"];

        for (var i = 0; i < erc1155TokenIds.length; i++) {
            console.log("Registering token no "+ i.toString());
            if(erc1155IsFungible[i]){
                // register a fungible token
                await erc1155Contract.registerNewToken(
                    0n, // type 
                    0n, // fungiblity
                    "Test Token "+i.toString(), // name
                    "TTT"+ i.toString(), // symbol
                    erc1155TokenIds[i], // offchainId
                    10000000000000n, // maxSupply
                    18n, // decimals
                    [], // subTokenIds
                    [],  // subTokenValues
                    0, // data
                    [] // additionalAttrs
                );




            }
            else{
                await erc1155Contract.registerNewToken(
                    0n, // type 
                    1n, // fungiblity
                    "Test Token "+i.toString(), // name
                    "TTT"+ i.toString(), // symbol
                    erc1155TokenIds[i], // offchainId
                    1, // maxSupply
                    0n, // decimals
                    [], // subTokenIds
                    [],  // subTokenValues
                    0, // data
                    [] // additionalAttrs
                );
            }

            const groupUniqueId = jsUtils.erc1155UniqueId(erc1155Contract.address, erc1155TokenIds[i], 0n);
            let isTokenMember = false;
            if(erc1155IsFungible[i]){
                isTokenMember = await zkDvpContract.isTokenMemberOf(2, [0n, erc1155TokenIds[i]], 0);
            }
            else{
                isTokenMember = await zkDvpContract.isTokenMemberOf(2,  [0n, erc1155TokenIds[i]], 1);
            }


            if(isTokenMember){
                erc1155GroupUniqueIds.push(groupUniqueId);
            }
            else{
                let groupId = 1;
                if(erc1155IsFungible[i]){
                    groupId = 0;
                }

                const tx4 = await zkDvpContract.addTokenToGroup(
                    2, // VAULT_ID_ERC1155,
                    [0, erc1155TokenIds[i]],
                    groupId
                );

                // reading the added uniqueId, altenatively you can 
                // compute it off-chain by
                // const uidNonFung = jsUtils.erc1155UniqueId(BigInt(erc1155Address), nonFungId, 1n);
                const addTokenEvent = await testHelpers.parseTokenAddedEvent(tx4);
                const newGroupUniqueId = addTokenEvent.tokenUniqueId;
                // updating local non-fungibleGroup merkleTree

                if(erc1155IsFungible[i]){
                    merkleTreeFungibles.insertLeaves([newGroupUniqueId]);
                }
                else{
                    merkleTreeNonFungibles.insertLeaves([newGroupUniqueId]);
                }

                erc1155GroupUniqueIds.push(newGroupUniqueId);

                // keeping uniqueIds to reuse later
                // it can be recalculated by generating erc1155 unique id with amount = 0
            }

        }

        console.log("Batch Test: Erc1155 tokens have been registered.");

    });



    it("Alice should deposit a batch of ERC1155 tokens in single transaction.", async () => {
        const erc1155Contract = contracts["erc1155"];
        const erc1155VaultContract = contracts["erc1155CoinVault"];
        const vaultAlice = erc1155VaultContract.connect(alice);
        const vaultBob = erc1155VaultContract.connect(bob);
        const erc1155Alice = erc1155Contract.connect(alice);
        const erc1155Bob = erc1155Contract.connect(bob);
        const zkDvpContract = contracts["zkdvp"];
        const zkDvpAlice = zkDvpContract.connect(alice);
        const zkDvpBob = zkDvpContract.connect(bob);

        console.log("Minting batch of ERC-1155");

        await adminActions.mintErc1155Batch(owner, alice, erc1155TokenIds, erc1155Amounts, 0, erc1155Contract);
        
        for (var i = 0; i < erc1155TokenIds.length; i++) {
          const newKey = await jsUtils.newKeyPair();
          inKeys.push(newKey);
        }


        const addrs = [alice.address,alice.address,alice.address,alice.address,alice.address,alice.address ]
        let bals = await erc1155Contract.balanceOfBatch(addrs, erc1155TokenIds);
      
        console.log("Alice erc1155 balances after mint: ", bals);

        console.log("Batch Depositing ERC1155 tokens");

        var depositCommitments = await userActions.depositErc1155Batch(
          alice,
          erc1155TokenIds,
          erc1155Amounts,
          0,
          inKeys,
          erc1155VaultContract,
          erc1155Contract,
        );


        console.log("Done Batch deposit.");

        bals = await erc1155Contract.balanceOfBatch(addrs, erc1155TokenIds);
      
        console.log("Alice erc1155 balances after deposit: ", bals);


        // console.log("Registered Commitments: ", depositCommitments);
        // for(var i = 0; i<depositCommitments.length; i++){

        // }
        merkleTree1155.insertLeaves(depositCommitments);

        for(var i = 0; i < erc1155TokenIds.length; i++){
            let groupProof = {};
            let groupTreeNumber = 0n;

            if(erc1155IsFungible[i]){
                groupProof = merkleTreeFungibles.generateProof(erc1155GroupUniqueIds[i]);
                groupTreeNumber = merkleTreeFungibles.lastTreeNumber;

            }
            else{
                groupProof = merkleTreeNonFungibles.generateProof(erc1155GroupUniqueIds[i]);
                groupTreeNumber = merkleTreeNonFungibles.lastTreeNumber;
            }
            aliceCoins.push({
                                "commitment": depositCommitments[i],
                                "amount": erc1155Amounts[i],
                                "tokenId": erc1155TokenIds[i],
                                "proof": merkleTree1155.generateProof(depositCommitments[i]),
                                "root": merkleTree1155.root,
                                "treeNumber": merkleTree1155.lastTreeNumber,
                                "key": inKeys[i],
                                "isFungible":erc1155IsFungible[i],
                                "groupProof": groupProof,
                                "groupTreeNumber": groupTreeNumber
                            });        
        }


        const currentMerkleRoot = await vaultAlice.currentRoot();
        const offchainMekleRoot = merkleTree1155.root;

        console.log("off-chain merkleRoot after deposit: ", offchainMekleRoot);
        console.log("On-chain merkleRoot after deposit: ", currentMerkleRoot);

        expect(offchainMekleRoot).to.equal(currentMerkleRoot);

    });

    it("Alice should be able to swap her fungible and non-fungible ERC1155 tokens.", async () => {
        const erc1155Contract = contracts["erc1155"];
        const erc1155Alice = erc1155Contract.connect(alice);
        const erc1155Bob = erc1155Contract.connect(bob);
        const zkDvpContract = contracts["zkdvp"];
        const zkDvpAlice = zkDvpContract.connect(alice);
        const zkDvpBob = zkDvpContract.connect(bob);

        // Alice generates new non-fungible coin 
        const nonfungId = jsUtils.erc1155UniqueId(BigInt(erc1155Contract.address), aliceCoins[0].tokenId, aliceCoins[0].amount);
        const newKey1 = jsUtils.newKeyPair();

        // Alice generates two new fungible coins 

        const sumAmount = aliceCoins[1].amount + aliceCoins[2].amount;
        console.log("sumAmount: ", sumAmount);

        const newAmount1 = 100n;
        const newAmount2  = sumAmount - newAmount1;
        const newNonFungId = jsUtils.erc1155UniqueId(BigInt(erc1155Contract.address), aliceCoins[0].tokenId, 1n);
        const newFungId = jsUtils.erc1155UniqueId(BigInt(erc1155Contract.address), aliceCoins[1].tokenId, newAmount1);

        const newKeyForFungible1 = jsUtils.newKeyPair();
        const newKeyForFungible2 = jsUtils.newKeyPair();

        const newFungCommitment = jsUtils.getCommitment(newFungId, newKeyForFungible1.publicKey);
        const newNonFungCommitment = jsUtils.getCommitment(newNonFungId, newKey1.publicKey);

        console.log("Alice generates a proof of transfer of ownership of non-fungible ERC1155 coin");
        const ownProof = await prover.prove(
            "OwnershipErc1155NonFungible",
            {
                message: newFungCommitment,
                values: [1n],
                keysIn: [aliceCoins[0]["key"]],
                keysOut: [newKey1],
                merkleProofs: [aliceCoins[0]["proof"]],
                treeNumbers: [aliceCoins[0]["treeNumber"]],
                erc1155TokenIds: [aliceCoins[0]["tokenId"]],
                erc1155ContractAddress: erc1155Contract.address,
                assetGroup_merkleProofs: [aliceCoins[0]["groupProof"]],
                assetGroup_treeNumbers: [aliceCoins[0]["groupTreeNumber"]]
            }
        );

        console.log("Alice generates a joinSplit proof of two fungible erc1155 coin.");
        const jsProof = await prover.prove(
            "JoinSplitErc1155",
            {
                message: newNonFungCommitment,
                valuesIn: [aliceCoins[1]["amount"], aliceCoins[2]["amount"]],
                keysIn: [aliceCoins[1]["key"], aliceCoins[2]["key"]],
                valuesOut: [newAmount1, newAmount2],
                keysOut: [newKeyForFungible1, newKeyForFungible2],
                merkleProofs: [aliceCoins[1]["proof"], aliceCoins[2]["proof"]],
                treeNumbers: [aliceCoins[1]["treeNumber"], aliceCoins[2]["treeNumber"]],
                erc1155TokenId: aliceCoins[1]["tokenId"],
                erc1155ContractAddress: erc1155Contract.address,
                assetGroup_treeNumber: aliceCoins[1]["groupTreeNumber"],
                assetGroup_merkleProof: aliceCoins[1]["groupProof"],
            }
        );

        // console.log(ownProof);
        // console.log(jsProof);

        console.log("Relayer forwards each proofs.");
        await relayerActions.submitPartialSettlement(owner, jsProof, 2, 0 ,zkDvpContract);
        await relayerActions.submitPartialSettlement(owner, ownProof, 2, 1 ,zkDvpContract);
        // console.log("swapCommitments: ", swapCommitments);
      const swapCommitments = [jsProof.statement[7], 
                                jsProof.statement[8], 
                                ownProof.statement[4]
                            ]
      merkleTree1155.insertLeaves(swapCommitments);
      
      // the uniqueId is the same as the aliceCoin[0]
      // these proofs can be reused. no need to regenerate it each time. 
      const groupProof0 = merkleTreeNonFungibles.generateProof(erc1155GroupUniqueIds[0]);
      const groupTreeNumber0 = merkleTreeNonFungibles.lastTreeNumber;
      aliceCoins.push({"commitment": swapCommitments[2],
                        "amount": 1n,
                        "tokenId": aliceCoins[0].tokenId,
                        "proof": merkleTree1155.generateProof(swapCommitments[2]),
                        "root": merkleTree1155.root,
                        "treeNumber": merkleTree1155.lastTreeNumber,
                        "key": newKey1,
                        "isFungible": false,
                        "groupProof": groupProof0,
                        "groupTreeNumber": groupTreeNumber0
                      });

      // the uniqueId is the same as the aliceCoin[1]
      const groupProof1 = merkleTreeFungibles.generateProof(erc1155GroupUniqueIds[1]);
      const groupTreeNumber1 = merkleTreeFungibles.lastTreeNumber;

      aliceCoins.push({"commitment": swapCommitments[0],
                        "amount": newAmount1,
                        "tokenId": aliceCoins[1].tokenId,
                        "proof": merkleTree1155.generateProof(swapCommitments[0]),
                        "root": merkleTree1155.root,
                        "treeNumber": merkleTree1155.lastTreeNumber,
                        "key": newKeyForFungible1,
                        "isFungible": true,
                        "groupProof": groupProof1,
                        "groupTreeNumber": groupTreeNumber1
                      });


      aliceCoins.push({"commitment": swapCommitments[1],
                        "amount": newAmount2,
                        "tokenId": aliceCoins[2].tokenId,
                        "proof": merkleTree1155.generateProof(swapCommitments[1]),
                        "root": merkleTree1155.root,
                        "treeNumber": merkleTree1155.lastTreeNumber,
                        "key": newKeyForFungible2,
                        "isFungible": true,
                        "groupProof": groupProof1,
                        "groupTreeNumber": groupTreeNumber1
                      });

      console.log("Swap successful");

  });

  // it("Alice should be able to batch-withdraw Erc1155 tokens.", async () => {
  //     const erc1155Contract = contracts["erc1155"];
  //     const erc1155Alice = erc1155Contract.connect(alice);
  //     const erc1155Bob = erc1155Contract.connect(bob);
  //     const zkDvpContract = contracts["zkdvp"];
  //     const zkDvpAlice = zkDvpContract.connect(alice);
  //     const zkDvpBob = zkDvpContract.connect(bob);
  //     const erc1155VaultContract = contracts["erc1155CoinVault"];
  //     const vaultAlice = erc1155VaultContract.connect(alice);
  //     const vaultBob = erc1155VaultContract.connect(bob);
  //   // Now Alice still has 5 unspent coins: aliceCoins[3..7]


  //   // Alice generates 5 new coins with same attributes and different keys
  //   let newKeysForBatch1 = [];
  //   let newIdsForBatch = [];
  //   let oldIdsForBatch = [];
  //   let valuesForBatch1 = [];
  //   let oldKeysForBatch1 = [];
  //   let proofsForBatch1 = [];
  //   let rootsForBatch1 = [];
  //   let treeNumbersForBatch1 = [];
  //   let tokenIdsforBatch = [];
  //   let groupTreeNumbersforBatch = [];
  //   let groupProofsforBatch = [];
  //   const accountAddress = alice.address;
  //   console.log("withdrawing to address: ", accountAddress);

  //   // console.log("AliceCoins: ", JSON.stringify(aliceCoins, null , 4));

  //   for(let i = 3; i < 8;i++)
  //   {
  //       const oldUniqueId = jsUtils.erc1155UniqueId(
  //                                           BigInt(erc1155Contract.address), 
  //                                           aliceCoins[i].tokenId, 
  //                                           aliceCoins[i].amount
  //                                                   );

  //       const newUniqueId = jsUtils.erc1155UniqueId(
  //                                           BigInt(erc1155Contract.address), 
  //                                           aliceCoins[i].tokenId, 
  //                                           aliceCoins[i].amount
  //                                                   );
  //       newIdsForBatch.push(newUniqueId);
  //       newKeysForBatch1.push({"publicKey": accountAddress});
  //       oldKeysForBatch1.push(aliceCoins[i].key);
  //       oldIdsForBatch.push(oldUniqueId);

  //       valuesForBatch1.push(aliceCoins[i]["amount"]);
  //       proofsForBatch1.push(aliceCoins[i]["proof"]);
  //       rootsForBatch1.push(aliceCoins[i]["root"]);
  //       treeNumbersForBatch1.push(aliceCoins[i]["treeNumber"]);
  //       tokenIdsforBatch.push(aliceCoins[i]["tokenId"]);

  //       groupTreeNumbersforBatch.push(aliceCoins[i]["groupTreeNumber"]);
  //       groupProofsforBatch.push(aliceCoins[i]["groupProof"]);
        
  //   }

  //   console.log("Alice generates a ERC1155Batch proof for the coins.");
  //   const batchProof1 = await userActions.generateErc1155BatchProof(
  //     0n,
  //     valuesForBatch1,
  //     oldKeysForBatch1,
  //     newKeysForBatch1,
  //     TREE_DEPTH,
  //     proofsForBatch1,
  //     rootsForBatch1,
  //     treeNumbersForBatch1,
  //     erc1155Contract.address,
  //     tokenIdsforBatch,
  //     groupTreeNumbersforBatch,
  //     groupProofsforBatch
  //   );

  //   // console.log("BatchProof: ", batchProof1);

  //   console.log("Alice successfully generated BatchProof for ERC1155 ownership.");

  //   let withdrawParams = [].concat(valuesForBatch1).concat(tokenIdsforBatch);

  //   await erc1155VaultContract.withdraw(withdrawParams, alice.address, batchProof1);

  //   console.log("Alice withdrew Batch ERC1155 coins");

  //   const addrs = [alice.address,alice.address,alice.address,alice.address,alice.address,alice.address ]
  //   const bals = await erc1155Contract.balanceOfBatch(addrs, erc1155TokenIds);
  
  //   console.log("Alice Erc1155 tokens: ", erc1155TokenIds);
  //   console.log("Alice Erc1155 balances: ", bals);
  // });

});
