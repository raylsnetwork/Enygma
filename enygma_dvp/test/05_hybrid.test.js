/* global describe it ethers before beforeEach afterEach overwriteArtifact */
const { expect } = require("chai");
const prover = require("../src/core/prover");
const jsUtils = require("../src/core/utils");
const MerkleTree = require("../src/core/merkle");
const userActions = require("./../src/core/endpoints/user.js");
const adminActions = require("./../src/core/endpoints/admin.js");

let merkleTree1155;
let merkleTree20;

let merkleTreeFungibles;
let merkleTreeNonFungibles;

const dvpConf = require("../zkdvp.config.json");
const TREE_DEPTH = dvpConf["tree-depth"];
const testHelpers = require("./testHelpers.js");

let aliceCoins = [];
let bobCoins = [];

let users;
let contracts;
let merkleTrees;

let alice;
let bob;

// loading bond data from dvpConfig
var bondsConf = dvpConf["metaTokens"];
// amounts that would be minted for bonds
const mintAmounts = [10, 20, 5];

// price of first bond that has been swapped in this demo
const bondPrice = 100n;


describe("Fungible Erc1155 Bond MetaToken and ERC20 Swap", () => {

    it(`ZkDvp should initialize properly `, async () => {
        
        let userCount = 2;
        [admin, users, contracts, merkleTrees] = await testHelpers.deployForTest(userCount);

        console.log("User: ",users);
        console.log("MerkleTree: ", merkleTrees);


        alice = users[0].wallet;
        bob = users[1].wallet;
        owner = admin;

        // let nftAlice = contracts["erc721"].connect(alice);

        merkleTree1155 = merkleTrees["ERC1155"].tree;
        merkleTree20 = merkleTrees["ERC20"].tree;
        merkleTreeFungibles = merkleTrees["fungibleGroup"].tree;
        merkleTreeNonFungibles = merkleTrees["nonFungibleGroup"].tree;

      console.log("Erc1155Bond-ERC20 Swap: ZkDvp initialization")

  });


 it(`Minting RaylsErc1155 subTokens `, async () => {

    console.log("Owner Minting RaylsErc1155 subTokens for herself ...");

    const subTokensConf = dvpConf["erc1155Tokens"];

    const subTokenIds = [];
    const subTokenMintAmounts = [];
    const fungibilities = [];
    for(var i = 0; i< subTokensConf.length; i++){
        console.log(`minting ${subTokensConf[i]["name"]}, ${subTokensConf[i]["mintAmount"]} units`);
        
        const subTokenId = subTokensConf[i].id;
        subTokenIds.push(subTokenId);
        const subTokenAmount = subTokensConf[i].mintAmount;
        subTokenMintAmounts.push(subTokenAmount);
        var subTokenFungibility = 0n;
        if(subTokensConf[i]["fungibility"] == "FUNGIBLE"){
            subTokenFungibility = 0n // fungible
        }
        else{
            subTokenFungibility = 1n; // non fungible
        }

        const tx = await contracts.erc1155.registerNewToken(
            0n,
            subTokenFungibility,
            subTokensConf[i]["name"],
            subTokensConf[i]["symbol"],
            subTokensConf[i]["id"],
            subTokensConf[i]["maxSupply"],
            subTokensConf[i]["decimals"],
            [], 
            [], 
            0,[]);

        console.log("Registered new subToken");

        await adminActions.mintErc1155(
            owner, 
            owner, 
            subTokenId, 
            subTokenAmount , 
            0, 
            contracts.erc1155
        );
    }


    // checking balances
    const bals = await contracts.erc1155.balanceOfBatch(
        [owner.address, owner.address, owner.address, owner.address], 
        [subTokenIds[0],subTokenIds[1],subTokenIds[2],subTokenIds[3]]);

    console.log("checking balances: ", bals);
  });



 it(`Registering RaylsErc1155 Bonds `, async () => {

    console.log("Registering RaylsErc1155 Bonds ...");

    for(var i = 0; i < bondsConf.length; i++){
        console.log(`Registering ${bondsConf[i]["name"]}, ${bondsConf[i]["symbol"]}`);


        let bondType = 0;
        let bondAdditionalAttrs = [];
        if(bondsConf[i]["type"] == "BOND"){
            bondType = 2;
        }
        const tx = await contracts.erc1155.registerNewToken(
            bondType,
            bondsConf[i]["fungibility"],
            bondsConf[i]["name"],
            bondsConf[i]["symbol"],
            bondsConf[i]["offchainId"],
            bondsConf[i]["maxSupply"],
            bondsConf[i]["decimals"],
            bondsConf[i]["subTokenIds"], 
            bondsConf[i]["subTokenValues"], 
            0,bondAdditionalAttrs);

        const bondData = await testHelpers.parseBondRegisteredEvent(tx);
        bondsConf[i]["onchainId"] = bondData.onchainId;

        console.log("BondRegistered event: ", JSON.stringify(bondData, null , 4));
    }



  });

 it(`Registering Bonds as fungible erc1155 token to Fungible AssetGroup `, async () => {

    console.log("Adding Bonds to fungible assetGroup...");

    for(var i = 0; i < bondsConf.length; i++){
        console.log(`Adding ${bondsConf[i]["name"]}, ${bondsConf[i]["symbol"]}`);

          console.log("Registering Fungible ERC-1155 tokenId to fungible AssetGroup");
          const tx2 = await contracts.zkdvp.addTokenToGroup(
              2,
              [0, bondsConf[i]["onchainId"]],
              0
            );

          // reading the added uniqueId, altenatively you can 
          // compute it off-chain by
          // const uidFung = jsUtils.erc1155UniqueId(BigInt(erc1155Address), fungId, 0n);

          const fungAddEvent = await testHelpers.parseTokenAddedEvent(tx2);
          const uidFung = fungAddEvent.tokenUniqueId;

          // keeping assetGroup's unique Id
          bondsConf[i].groupUniqueId = uidFung;

          // updating local fungibleGroup merkleTree
          merkleTreeFungibles.insertLeaves([uidFung]);
    }



  });

 it(`Minting RaylsErc1155 Bonds `, async () => {


    console.log("Minting RaylsErc1155 Bonds ...");

    for(var i = 0; i< bondsConf.length; i++){
        console.log(`Minting ${bondsConf[i]["onchainId"]}, ${mintAmounts[i]} units`);



        await adminActions.mintErc1155Fungible(
            owner,
            alice,
            bondsConf[i]["onchainId"],
            mintAmounts[i],
            contracts.erc1155
        );

    }

    const bals = await contracts.erc1155.balanceOfBatch(
        [alice.address, alice.address, alice.address],
        [bondsConf[0]["onchainId"],bondsConf[1]["onchainId"],bondsConf[2]["onchainId"]]);


    console.log("Alice bonds balances: ", bals);



  });

  it("Alice deposits ERC1155 bond in single transaction.", async () => {
        const erc1155Contract = contracts["erc1155"];
        const erc1155VaultContract = contracts["erc1155CoinVault"];
        const vaultAlice = erc1155VaultContract.connect(alice);

        const erc1155Alice = erc1155Contract.connect(alice);

        const depositKey = await jsUtils.newKeyPair();

        console.log("depositing ERC1155 bond");

        var depositCommitments = await userActions.depositErc1155(
            alice,
            bondsConf[0]["onchainId"],
            mintAmounts[0],
            0,
            depositKey,
            erc1155VaultContract,
            erc1155Contract,
        );


        console.log("Done Bond deposit.");

        console.log("Deposit commitments: ", depositCommitments);

        merkleTree1155.insertLeaves(depositCommitments);
        
        // inserting tokenId into fungibilityMerkle

        let fungibilityProof = merkleTreeFungibles.generateProof(bondsConf[0].groupUniqueId);
        let fungibilityRoot = merkleTreeFungibles.root;
        aliceCoins.push({   
                            "commitment": depositCommitments[0],
                            "amount": mintAmounts[0],
                            "tokenId": bondsConf[0]["onchainId"],
                            "proof": merkleTree1155.generateProof(depositCommitments[0]),
                            "root": merkleTree1155.root,
                            "treeNumber": merkleTree1155.lastTreeNumber,
                            "key": depositKey,
                            "isHybrid":true,
                            "group_treeNumber": merkleTreeFungibles.lastTreeNumber,
                            "group_proof": fungibilityProof
                        });        

        // console.log("Alice's Registered coins: ", aliceCoins);
  });

  it("Bob should deposit ERC20 coins.", async () => {
    const zkDvpContract = contracts["zkdvp"];
    const zkDvpAlice = zkDvpContract.connect(alice);
    const zkDvpBob = zkDvpContract.connect(bob);
    const erc20Contract = contracts["erc20"];
    const erc20Bob = erc20Contract.connect(bob);
    const erc20Owner = erc20Contract.connect(owner);
    const erc20VaultContract = contracts["erc20CoinVault"];
    const vault20Bob = erc20VaultContract.connect(bob);
        

    // minting ERC20 tokens for Bob
    tx = await erc20Owner.mint(bob.address, bondPrice);
    await tx.wait();

    // Approve ZkDvp to transfer tokens
    await erc20Bob.approve(erc20VaultContract.address, bondPrice);

    console.log("Bob deposits two ERC20 coins");
    // creating depositKeys for Bob to Deposit Erc20 tokens to ZkDvp
    const fundKeys = [jsUtils.newKeyPair(), jsUtils.newKeyPair()];

    cmt1 = await userActions.depositErc20(
      bob,
      bondPrice / 2n,
      fundKeys[0],
      erc20VaultContract,
      erc20Contract,
      merkleTree20);

    merkleTree20.insertLeaves([cmt1]);

    bobCoins.push({"commitment": cmt1,
                      "proof": merkleTree20.generateProof(cmt1),
                      "root": merkleTree20.root,
                      "treeNumber": merkleTree20.lastTreeNumber,
                      "key": fundKeys[0],
                      "amount": bondPrice / 2n
                    });

    cmt2 = await userActions.depositErc20(
      bob,
      bondPrice / 2n,
      fundKeys[1],
      erc20VaultContract,
      erc20Contract,
      merkleTree20);

    merkleTree20.insertLeaves([cmt2]);

    bobCoins.push({"commitment": cmt2,
                      "proof": merkleTree20.generateProof(cmt2),
                      "root": merkleTree20.root,
                      "treeNumber": merkleTree20.lastTreeNumber,
                      "key": fundKeys[1],
                      "amount": bondPrice / 2n
                    });

    console.log("Bob's deposited ERC20 coins: ", bobCoins);
  });


  it("Alice should be able to swap her bond ERC1155 coin with Bob's ERC20 coins.", async () => {

      const erc1155Contract = contracts["erc1155"];
      const erc1155Alice = erc1155Contract.connect(alice);
      const erc1155Bob = erc1155Contract.connect(bob);

      const erc20Contract = contracts["erc20"];
      const erc20Alice = erc20Contract.connect(alice);
      const erc20Bob = erc20Contract.connect(bob);

      const zkDvpContract = contracts["zkdvp"];
      const zkDvpAlice = zkDvpContract.connect(alice);
      const zkDvpBob = zkDvpContract.connect(bob);


    console.log("Bob generates New keys to receive the bond");


    // Bob generates a public key to receive the NFT
    let bobReceivingKey = jsUtils.newKeyPair();

    console.log("Bob shares the keys with Alice.");

    console.log("Bob also creates a new key for the excess amount.");

    const bobExcessKey = jsUtils.newKeyPair();


    console.log("Alice shares one key to receive ERC20 payment and shares it with Bob.");
    const aliceReceivingKey = jsUtils.newKeyPair();

    console.log("Also Alice and Bob settles on the price = ", bondPrice);


    console.log("Alice generates a ERC1155 proof for the old coins using Bob's generated publicKey for the new coins.");

    console.log("... Alice creates the swap proof message for erc1155 bond coin");


    console.log("... Alice re-computes first ERC20 out commitment");

    let erc20Uid = jsUtils.erc20UniqueId(erc20Contract.address, bondPrice);
    const erc1155ProofMessage = jsUtils.getCommitment(erc20Uid, aliceReceivingKey.publicKey);


    const dummyKey = jsUtils.newKeyPair();
    console.log("Alice generates Erc1155JoinSplit proof");

    const bondProof = await prover.prove(
        "JoinSplitErc1155",
        {
            message: erc1155ProofMessage,
            valuesIn: [aliceCoins[0].amount, 0n],
            keysIn: [aliceCoins[0].key, dummyKey],
            valuesOut: [aliceCoins[0].amount, 0n],
            keysOut: [{"publicKey": bobReceivingKey.publicKey}, dummyKey],
            merkleProofs: [aliceCoins[0].proof, {root: 0n}],
            treeNumbers: [aliceCoins[0].treeNumber, 0n],
            erc1155TokenId: aliceCoins[0]["tokenId"],
            erc1155ContractAddress: erc1155Contract.address,
            assetGroup_treeNumber: aliceCoins[0]["group_treeNumber"],
            assetGroup_merkleProof: aliceCoins[0]["group_proof"],
        }
    );

    console.log("Bob generates ERC20JoinSplit proof");

    console.log("... Bob regenerates ERC1155 bond coin commitment to generate the erc20 proof message.");

    const newUniqueId = jsUtils.erc1155UniqueId(
                                        erc1155Contract.address, 
                                        aliceCoins[0].tokenId, 
                                        aliceCoins[0].amount
                                                );
    const erc20ProofMessage =  jsUtils.getCommitment(newUniqueId, bobReceivingKey.publicKey);

    const jsParams = await prover.prove(
        "JoinSplitErc20",
        {
            message: erc20ProofMessage,
            valuesIn: [bobCoins[0].amount, bobCoins[1].amount],
            keysIn: [bobCoins[0].key, bobCoins[1].key],
            valuesOut: [bondPrice, 0n],
            keysOut: [aliceReceivingKey, bobExcessKey],
            merkleProofs: [bobCoins[0].proof, bobCoins[1].proof],
            treeNumbers: [bobCoins[0].treeNumber, bobCoins[1].treeNumber],
            erc20ContractAddress: erc20Contract.address,
        }
    );

    console.log("Bob submits his proof to relayer. [NOT IMPLEMENTED]")

    console.log("Alice submits her proof to relayer. [NOT IMPLEMENTED]")

    console.log("Relayer submitPartialSettlement for both proofReceipts");
    // await zkDvpContract.exchange(jsParams, bondProof, 0, 2);
    await zkDvpContract.submitPartialSettlement(jsParams, 0, 0);
    await zkDvpContract.submitPartialSettlement(bondProof, 2, 0);


    console.log("Adding commitments to local merkleTrees for next proof generations.")
    console.log("[TODO] For production, the commitments should be from the on-chain events.")

    console.log("Inserting leaves to local ERC20 merkleTree");
    let jsCommitments = [];
    for(var i = 0; i< jsParams.numberOfInputs; i++){
        jsCommitments.push(jsParams.statement[ 1 + i + 3 * jsParams.numberOfInputs]);
    }

    merkleTree20.insertLeaves(jsCommitments);
    aliceCoins.push({"commitment": jsCommitments[0],
                      "proof": merkleTree20.generateProof(jsCommitments[0]),
                      "root": merkleTree20.root,
                      "treeNumber": merkleTree20.lastTreeNumber,
                      "key": aliceReceivingKey,
                      "amount": bondPrice
                    });
   bobCoins.push({"commitment": jsCommitments[1],
                      "proof": merkleTree20.generateProof(jsCommitments[1]),
                      "root": merkleTree20.root,
                      "treeNumber": merkleTree20.lastTreeNumber,
                      "key": bobExcessKey,
                      "amount": 0n
                    });

    console.log("Inserting leaves to local ERC1155 merkleTree");

    console.log("BondProof.statement: ", bondProof.statement);
    const bondCommitments = [bondProof.statement[7], bondProof.statement[8]];

    merkleTree1155.insertLeaves(bondCommitments);

    let group_proof = merkleTreeFungibles.generateProof(bondsConf[0]["groupUniqueId"]);
    let group_treeNumber = merkleTreeFungibles.lastTreeNumber;
    bobCoins.push({"commitment": bondCommitments[0],
                    "amount": mintAmounts[0],
                    "tokenId": bondsConf[0].onchainId,
                    "proof": merkleTree1155.generateProof(bondCommitments[0]),
                    "treeNumber": merkleTree1155.lastTreeNumber,
                    "key": bobReceivingKey,
                    "group_treeNumber": group_treeNumber,
                    "group_proof": group_proof
                  });

    console.log(bobCoins.length);
    console.log("Hybrid Swap has been successful.");

    console.log("-------------------------------------");
});
  it("Bob should be able to withdraw his bondCoin into tokens.", async () => {

      const erc1155Contract = contracts["erc1155"];
      const erc1155Bob = erc1155Contract.connect(bob);

    const erc1155VaultContract = contracts["erc1155CoinVault"];

    console.log("Bob withdraws his bond coin froms ZkDvp.");

    console.log("... Bob generates a ERC1155 proof for the new coins.");

    const dummyKey = jsUtils.newKeyPair();
    const withdrawBatchProof = await prover.prove(
        "JoinSplitErc1155",
        {
            message: 0n,
            valuesIn: [bobCoins[3].amount, 0n],
            keysIn: [bobCoins[3].key, dummyKey],
            valuesOut: [bobCoins[3].amount, 0n],
            keysOut: [{"publicKey": bob.address}, dummyKey],
            merkleProofs: [bobCoins[3].proof, {root: 0n}],
            treeNumbers: [bobCoins[3].treeNumber, 0n],
            erc1155TokenId: bobCoins[3].tokenId,
            erc1155ContractAddress: erc1155Contract.address,
            assetGroup_treeNumber: bobCoins[3].group_treeNumber,
            assetGroup_merkleProof: bobCoins[3].group_proof,
        }
    );


    console.log("... generated bondProof for withdraw: ", withdrawBatchProof);
    var bobBalance = await erc1155Contract.balanceOf(
                                    bob.address, 
                                    bondsConf[0].onchainId
                                );

    console.log(`Bob balances on bond token before withdraw: id: ${bondsConf[0].onchainId} amount: ${bobBalance}`);

    await erc1155VaultContract.withdraw(
                                            [
                                                bobCoins[3].amount, 
                                                bobCoins[3].tokenId,
                                                0n,
                                                0n
                                            ], 
                                            bob.address, 
                                            withdrawBatchProof
                                        );

    console.log("done withdraw. Checking Bob's balance on bond token.");

    bobBalance = await erc1155Contract.balanceOf(
                            bob.address, 
                            bondsConf[0].onchainId
                        ) ;
    console.log(`Bob balances on bond token after withdraw: id: ${bondsConf[0].onchainId} amount: ${bobBalance}`);
    expect(bobBalance).to.equal(mintAmounts[0]);


    console.log("Bob's balances are as expected.");

    console.log("-------------------------------------");

  });

  it("Alice should be able to withdraw ERC20 coins equal to Hybrid ERC1155's price.", async () => {

    const erc20Contract = contracts["erc20"];
    const erc20Alice = erc20Contract.connect(alice);

    const zkDvpContract = contracts["zkdvp"];
    const zkDvpAlice = zkDvpContract.connect(alice);
    const erc20VaultContract = contracts["erc20CoinVault"];

    console.log("Alice generates ERC20JoinSplit proof.");
    const dummyKey = jsUtils.newKeyPair();
    // Bob generates a tx to send payment to Alice
    const aliceJS = await prover.prove(
        "JoinSplitErc20",
        {
            message: 0n,
            valuesIn: [bondPrice, 0n],
            keysIn: [aliceCoins[1].key, dummyKey],
            valuesOut: [bondPrice, 0n],
            keysOut: [{ publicKey: BigInt(alice.address) }, dummyKey],
            merkleProofs: [aliceCoins[1].proof, {"root": 0n}],
            treeNumbers: [aliceCoins[1].treeNumber, 0n],
            erc20ContractAddress: erc20Contract.address,
        }
    );

    const oldBalance = (
      await erc20Contract.balanceOf(alice.address)
    ).toBigInt();


    await erc20VaultContract.withdraw([bondPrice], alice.address, aliceJS);

    const newBalance = (
      await erc20Contract.balanceOf(alice.address)
    ).toBigInt();
    
    expect(newBalance).to.equal(oldBalance + bondPrice);
    console.log(`Alice withdrew ${bondPrice} to her ERC20 account.`);

    console.log(`Alice ERC20 balance is as expected.`);
    console.log("-------------------------------------");

  });

});
