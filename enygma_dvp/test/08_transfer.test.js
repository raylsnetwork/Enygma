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
const dvpConf = require("../zkdvp.config.json");
const TREE_DEPTH = dvpConf["tree-depth"];
const testHelpers = require("./testHelpers.js");

let users;
let contracts;
let merkleTree721;
let merkleTree20;
let merkleTreeFungibles;
let merkleTreeNonFungibles;

let alice = {};
let bob = {};
let relayer = {};
let owner = {};
let inKeys = [];

let shouldInitialize = false;
let startTime, endTime;

let groupUniqueIds = [];

describe("Testing Transfer() functionality", () => {


    beforeEach(async () => {
        startTime = performance.now();

        if(shouldInitialize){
            let userCount = 2;
            let admin;
            [admin, users, contracts, merkleTrees] = await testHelpers.deployForTest(userCount);

            alice.wallet = users[0].wallet;
            bob.wallet =     users[1].wallet;
            relayer.wallet = admin;
            owner.wallet = admin;

            alice.calls = {};
            bob.calls = {};
            relayer.calls = {};
            owner.calls = {};

            alice.knows = {};
            bob.knows = {};

            alice.coins = [];
            bob.coins = [];

            relayer.calls.zkdvp = contracts.zkdvp.connect(relayer.wallet);
            relayer.calls.erc20Vault = contracts.erc20CoinVault.connect(relayer.wallet);
            relayer.calls.erc721Vault = contracts.erc721CoinVault.connect(relayer.wallet);
            relayer.calls.erc1155Vault = contracts.erc1155CoinVault.connect(relayer.wallet);

            bob.calls.erc20 = contracts.erc20.connect(bob.wallet);
            bob.calls.erc20Vault = contracts.erc20CoinVault.connect(bob.wallet);
            bob.calls.erc721 = contracts.erc721.connect(bob.wallet);
            bob.calls.erc721Vault = contracts.erc721CoinVault.connect(bob.wallet);
            bob.calls.erc1155 = contracts.erc1155.connect(bob.wallet);
            bob.calls.erc1155Vault = contracts.erc1155CoinVault.connect(bob.wallet);

            owner.calls.erc20 = contracts.erc20.connect(owner.wallet);
            owner.calls.erc721 = contracts.erc721.connect(owner.wallet);
            owner.calls.erc1155= contracts.erc1155.connect(owner.wallet);
            owner.calls.zkdvp = contracts.zkdvp.connect(owner.wallet);

            merkleTree20 = merkleTrees["ERC20"].tree;
            merkleTree721 = merkleTrees["ERC721"].tree;
            merkleTree1155 = merkleTrees["ERC1155"].tree;
            merkleTreeFungibles = merkleTrees["fungibleGroup"].tree;
            merkleTreeNonFungibles = merkleTrees["nonFungibleGroup"].tree;

            console.log("Erc20Vault.Transfer() Test: Done ZkDvp initialization")
            shouldInitialize = false;
        }
    });


    afterEach(async () => {
        const time = Math.round(performance.now() - startTime);
        console.log(` [TIME] : ${time} ms \n`);
    });

    shouldInitialize = true;


    it("Bob deposits random number (between 2 and 10 inclusive) of ERC20 coins.", async () => {

        numberOfBobsCoins = Math.floor(Math.random() * (10 - 2) + 2);


        var bobCoinAmounts = [];
        bobTotalDeposit = 0n;

        for(var j = 0; j < numberOfBobsCoins; j++){
            
            const newAmount = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(3)));
            bobTotalDeposit += newAmount;
            bobCoinAmounts.push(newAmount);
        }

        console.log(`Minting total value of ${bobTotalDeposit} erc20 tokens for Bob.`);

        // minting ERC20 tokens for Bob
        let tx = await owner.calls.erc20.mint(bob.wallet.address, bobTotalDeposit);
        await tx.wait();

        // Approve ZkDvp to transfer tokens
        await bob.calls.erc20.approve(contracts.erc20CoinVault.address, bobTotalDeposit);

        console.log(`Depositing ${numberOfBobsCoins} erc20 coins for Bob with amounts: ${bobCoinAmounts}`);

        for(var i = 0; i< numberOfBobsCoins; i++){
            // creating depositKeys for Bob to Deposit Erc20 tokens to ZkDvp
            let bobErc20Key = jsUtils.newKeyPair();

            // adding new erc20 coin to bob's erc20 coin list

            tx = await bob.calls.erc20Vault.deposit(
                [bobCoinAmounts[i],
                bobErc20Key.publicKey]
            );

            const bobCmt = await testHelpers.getCommitmentFromTx(tx);
            merkleTree20.insertLeaves([bobCmt]);
            let bobProof = merkleTree20.generateProof(bobCmt);

            bob.coins.push({"treeId": 1,
                            "value":bobCoinAmounts[i], 
                            "tokenAddress": contracts.erc20.address,
                            "key":bobErc20Key, 
                            "commitment":bobCmt, 
                            "proof":bobProof, 
                            "root":merkleTree20.root, 
                            "treeNumber":merkleTree20.lastTreeNumber
                    });

        }

        console.log("----------------------------")
        // console.log("Alice.coins:\n", JSON.stringify(alice.coins, null, 4)    + "\n");
        // console.log("Bob.coins:\n", JSON.stringify(bob.coins, null, 4) + "\n");

    });


    it("Bob transfers the value = summation of the coins to Alice.", async () => {

        // Alice generates a public key to receive the payment
        console.log("Alice creates a new keyPair to receive Bob's coins");
        const aliceReceivingKey = jsUtils.newKeyPair();

        console.log("Alice shares the public key part with Bob");
        bob.knows.aliceReceivingKey = {privateKey:0n, publicKey:aliceReceivingKey.publicKey};

        // TODO:: Make it interactive to be sure that alice can open the payment coin
        console.log("Alice knows how much Bob is sending to be able to spend the coin later.");
        alice.knows.bobTotalDeposit = bobTotalDeposit;

        jsInputs = {
            "inKeys": [],         
            "inValues": [],            
            "outKeys": [] ,        
            "outValues": [],         
            "treeNumbers": [],         
            "roots": [],         
            "nullifiers": [],         
            "outCommitments": [],    
            "proofs": []         
        };

        for(var i = 0; i < bob.coins.length; i++){
            jsInputs.inKeys.push(bob.coins[i].key);
            jsInputs.inValues.push(bob.coins[i].value);
            jsInputs.proofs.push(bob.coins[i].proof);
            jsInputs.treeNumbers.push(bob.coins[i].treeNumber);
            jsInputs.roots.push(bob.coins[i].root);
        }


        // Bob generates a public to receive the change
        let bobChangeKey = jsUtils.newKeyPair();
        console.log("Bob shared his receiving publicKey with Alice to receive the coin with excess amount.");

        // Bob generates payment commitment for Alice
        let paymentAmount = bobTotalDeposit;
        let excessAmount = 0n;

        // creating unique erc20Commitment
        let erc20Uid = jsUtils.erc20UniqueId(
            contracts.erc20.address,
            paymentAmount
        );


        const jsParams = await prover.prove(
            "JoinSplitErc20_10_2",
            {
                message: 0n,
                valuesIn: jsInputs.inValues,
                keysIn: jsInputs.inKeys,
                valuesOut: [paymentAmount, excessAmount],
                keysOut: [bob.knows.aliceReceivingKey, bobChangeKey],
                merkleProofs: jsInputs.proofs,
                treeNumbers: jsInputs.treeNumbers,
                erc20ContractAddress: contracts.erc20.address,
            }
        );

        // console.log(jsParams);

        console.log("Sending transfer ...");
        // A relayer forwards both transactions to ZkDvp
        tx = await relayer.calls.erc20Vault.transfer(jsParams);

        const [eventCommitments, eventNullifiers] = await testHelpers.getCommitmentFromTxIndirect(tx, 
                [
                    contracts.erc20CoinVault.address
                ]
            );
        console.log("Done transfer");
        // console.log(eventCommitments);

        merkleTree20.insertLeaves([eventCommitments[0], eventCommitments[1]]);
        alice.coins.push({"commitment":eventCommitments[0],
                                            "key":aliceReceivingKey,
                                         "treeNumber":merkleTree20.lastTreeNumber,
                                         "proof":merkleTree20.generateProof(eventCommitments[0]),
                                         "root": merkleTree20.root});
        bob.coins.push({"commitment":eventCommitments[1],
                                            "key":bobChangeKey,
                                         "treeNumber":merkleTree20.lastTreeNumber,
                                         "proof":merkleTree20.generateProof(eventCommitments[1]),
                                         "root": merkleTree20.root});

    });


    it("Alice withdraws ERC20 output coin.", async () => {
        const dummyKey = jsUtils.newKeyPair();

        const aliceJS = await prover.prove(
            "JoinSplitErc20",
            {
                message: 0n,
                valuesIn: [alice.knows.bobTotalDeposit, 0n],
                keysIn: [alice.coins[0].key, dummyKey],
                valuesOut: [alice.knows.bobTotalDeposit, 0n],
                keysOut: [{ publicKey: BigInt(alice.wallet.address) }, dummyKey],
                merkleProofs: [alice.coins[0]["proof"], {"root": 0n}],
                treeNumbers: [alice.coins[0]["treeNumber"], 0n],
                erc20ContractAddress: contracts.erc20.address,
            }
        );

        const oldBalance = (
            await contracts.erc20.balanceOf(alice.wallet.address)
        ).toBigInt();


        await relayer.calls.erc20Vault.withdraw([alice.knows.bobTotalDeposit], alice.wallet.address, aliceJS);
        const newBalance = (
            await contracts.erc20.balanceOf(alice.wallet.address)
        ).toBigInt();
        await expect(oldBalance + alice.knows.bobTotalDeposit).to.equal(newBalance);

        console.log("Alice balance is as expected. Done erc20Transfer test.")
        
        shouldInitialize = true;
    });


    /////////////////////////////////////////////////////////////////////////
    ///////////////// Erc721Vault.Transfer() ////////////////////////////////
    /////////////////////////////////////////////////////////////////////////

    const NFT_ID = 88888n

    it("Bob deposits one ERC721 coin.", async () => {


            console.log(`Minting Erc721 token for for Bob.`);

            // minting ERC20 tokens for Bob
            let tx = await owner.calls.erc721.mint(bob.wallet.address, NFT_ID);
            await tx.wait();

            // Approve ZkDvp to transfer tokens
            await bob.calls.erc721.approve(contracts.erc721CoinVault.address, NFT_ID);

            console.log(`Depositing Erc721 coin for Bob with id: ${NFT_ID}`);

            // creating depositKeys for Bob to Deposit Erc20 tokens to ZkDvp
            let bobErc721Key = jsUtils.newKeyPair();

            // adding new erc20 coin to bob's erc20 coin list
            tx = await bob.calls.erc721Vault.deposit(
                [
                    NFT_ID,
                    bobErc721Key.publicKey
                ]
            );

            const bobCmt = await testHelpers.getCommitmentFromTx(tx);
            merkleTree721.insertLeaves([bobCmt]);
            let bobProof = merkleTree721.generateProof(bobCmt);

            bob.coins.push({        "treeId": 1,
                                                    "value":NFT_ID, 
                                                    "tokenAddress": contracts.erc721.address,
                                                    "key":bobErc721Key, 
                                                    "commitment":bobCmt, 
                                                    "proof":bobProof, 
                                                    "root":merkleTree721.root, 
                                                    "treeNumber":merkleTree721.lastTreeNumber
                                            });


            console.log("----------------------------")
            // console.log("Alice.coins:\n", JSON.stringify(alice.coins, null, 4)    + "\n");
            // console.log("Bob.coins:\n", JSON.stringify(bob.coins, null, 4) + "\n");

    });


    it("Bob transfers the Erc721 coin to Alice.", async () => {

        // Alice generates a public key to receive the payment
        console.log("Alice creates a new keyPair to receive Bob's coins");
        const aliceReceivingKey = jsUtils.newKeyPair();

        console.log("Alice shares the public key part with Bob");
        bob.knows.aliceReceivingKey = {privateKey:0n, publicKey:aliceReceivingKey.publicKey};

        // TODO:: Make it interactive to be sure that alice can open the payment coin
        console.log("Alice knows the NFT_ID of the coin to be able to spend the coin later.");
        alice.knows.nftId = NFT_ID;

        // Bob generates a public to receive the change
        let bobChangeKey = jsUtils.newKeyPair();
        console.log("Bob shared his receiving publicKey with Alice to receive the coin with excess amount.");

        const ownParams = await prover.prove(
            "OwnershipErc721",
            {
                message: 0n,
                values: [NFT_ID],
                keysIn: [bob.coins[0].key],
                keysOut: [bob.knows.aliceReceivingKey],
                merkleProofs: [bob.coins[0].proof],
                treeNumbers: [bob.coins[0].treeNumber],
                erc721ContractAddress: contracts.erc721.address
            }
        );

        // console.log(ownParams);

        console.log("Sending transfer ...");
        // A relayer forwards both transactions to ZkDvp
        tx = await relayer.calls.erc721Vault.transfer(ownParams);

        const [eventCommitments, eventNullifiers] = await testHelpers.getCommitmentFromTxIndirect(tx, 
            [
                contracts.erc721CoinVault.address
            ]
        );
        console.log("Done transfer");
        // console.log(eventCommitments);

        merkleTree721.insertLeaves([eventCommitments[0]]);
        alice.coins.push({
            "commitment":eventCommitments[0],
            "key":aliceReceivingKey,
            "treeNumber":merkleTree721.lastTreeNumber,
            "proof":merkleTree721.generateProof(eventCommitments[0]),
            "root": merkleTree721.root
        });

    });


    it("Alice withdraws transferred Erc721 coin.", async () => {
        // Alice generates a tx to send her NFT to Bob
        const ownParams = await prover.prove(
            "OwnershipErc721",
            {
                message: 0n,
                values: [NFT_ID],
                keysIn: [alice.coins[0].key],
                keysOut: [{ publicKey: BigInt(alice.wallet.address) }],
                merkleProofs: [alice.coins[0].proof],
                treeNumbers: [alice.coins[0].treeNumber],
                erc721ContractAddress: contracts.erc721.address
            }
        );

        await relayer.calls.erc721Vault.withdraw([NFT_ID], alice.wallet.address, ownParams);
        const res = await contracts.erc721.ownerOf(NFT_ID);
        expect(res).to.equal(alice.wallet.address);
        console.log(`Alice is the new owner of NFT with id ${NFT_ID} as expected`);

        shouldInitialize = true;
    });



    /////////////////////////////////////////////////////////////////////////
    ///////////////// Erc1155Vault.Transfer() ////////////////////////////////
    /////////////////////////////////////////////////////////////////////////

    it("Owner registers 2 Erc1155 tokens, one fungible and one non-fungible.", async () => {
        const tokenIds = [100n, 101n];
        const tokenFungibilities = [1n, 0n];
        const tokenDecimals = [0, 10n]; // For NFT one, the value should be ignored
        const tokenMaxSupplies = [10000000n, 10000000n]; // For NFT one, the value should be ignored
        const tokenNames = ["NFToken", "FToken"];
        const tokenSymbols = ["NFT", "FT"];

        console.log("Owner registers RaylsErc1155 NORMAL tokens...");
        for(var i = 0; i< tokenIds.length; i++){
            console.log(`Registering ${tokenNames[i]}, id = ${tokenIds[i]}`);

            const tx = await owner.calls.erc1155.registerNewToken(
                    0, // type = NORMAL
                    tokenFungibilities[i],
                    tokenNames[i],
                    tokenSymbols[i],
                    tokenIds[i],
                    tokenMaxSupplies[i],
                    tokenDecimals[i],
                    [], // no subTokens ids 
                    [], // no subToken values
                    0,
                    []
            );

            const tx2 = await owner.calls.zkdvp.addTokenToGroup(
                    2,
                    [0, tokenIds[i]],
                    tokenFungibilities[i]
                );

            // reading the added uniqueId, altenatively you can 
            // compute it off-chain by
            // const uidFung = jsUtils.erc1155UniqueId(BigInt(erc1155Address), fungId, 0n);

            const fungAddEvent = await testHelpers.parseTokenAddedEvent(tx2);
            const uidFung = fungAddEvent.tokenUniqueId;

            // keeping assetGroup's unique Id
            groupUniqueIds.push(uidFung);

            if(tokenFungibilities[i] == 0n){
                    // updating local fungibleGroup merkleTree
                    merkleTreeFungibles.insertLeaves([uidFung]);                        
            }
            else{
                    // updating local fungibleGroup merkleTree
                    merkleTreeNonFungibles.insertLeaves([uidFung]);                        

            }
        }

    });


    it("Owner mints the two tokens.", async () => {


        const tokenIds = [100n, 101n];
        const tokenMintAmounts = [1n, 100n];

        for(var i = 0; i< tokenIds.length; i++){
            console.log(`minting ${tokenIds[i]}, ${tokenMintAmounts[i]} units`);

            const tx = await owner.calls.erc1155.mint(
                bob.wallet.address, 
                tokenIds[i], 
                tokenMintAmounts[i] , 
                0
            );
        }
        // checking balances
        const bals = await contracts.erc1155.balanceOfBatch(
            [bob.wallet.address, bob.wallet.address], 
            [tokenIds[0],tokenIds[1]]);

        console.log("checking Bob's balances on two minted coins: ", bals);
        expect(bals[0]).to.equal(tokenMintAmounts[0]);
        expect(bals[1]).to.equal(tokenMintAmounts[1]);

        console.log("Bob's balances are as expected.");


    });

        

    it("Bob deposits minted tokens.", async () => {

        const depositKey = await jsUtils.newKeyPair();

        const tokenIds = [100n, 101n];
        const tokenMintAmounts = [1n, 100n];
        const tokenFungibilities = [1n, 0n];

        for(var i = 0; i< tokenIds.length; i++){

            var depositCommitments = await userActions.depositErc1155(
                    bob.wallet,
                    tokenIds[i],
                    tokenMintAmounts[i],
                    0,
                    depositKey,
                    contracts.erc1155CoinVault,
                    contracts.erc1155,
            );

            merkleTree1155.insertLeaves(depositCommitments);

            let groupProof = {};
            let groupTreeNumber = 0n;

            if(tokenFungibilities[i] == 0n){
                groupProof = merkleTreeFungibles.generateProof(groupUniqueIds[i]);
                groupTreeNumber = merkleTreeFungibles.lastTreeNumber;
            }
            else{
                groupProof = merkleTreeNonFungibles.generateProof(groupUniqueIds[i]);
                groupTreeNumber = merkleTreeNonFungibles.lastTreeNumber;
            }

            bob.coins.push({
                "commitment": depositCommitments[0],
                "amount": tokenMintAmounts[i],
                "tokenId": tokenIds[i],
                "proof": merkleTree1155.generateProof(depositCommitments[0]),
                "root": merkleTree1155.root,
                "treeNumber": merkleTree1155.lastTreeNumber,
                "key": depositKey,
                "groupTreeNumber": groupTreeNumber,
                "groupProof": groupProof
            });                
        }
    });


    it("Bob transfers the fungible Erc1155 coin to Alice.", async () => {

        // Alice generates a public key to receive the payment
        console.log("Alice creates a new keyPair to receive Bob's coins");
        const aliceReceivingKey = jsUtils.newKeyPair();

        console.log("Alice shares the public key part with Bob");
        bob.knows.aliceReceivingKey = {privateKey:0n, publicKey:aliceReceivingKey.publicKey};

//         // TODO:: Make it interactive to be sure that alice can open the payment coin
//         console.log("Alice knows the NFT_ID of the coin to be able to spend the coin later.");
//         alice.knows.nftId = NFT_ID;

        let fungibilityProof = merkleTreeFungibles.generateProof(groupUniqueIds[1]);
        let fungibilityRoot = merkleTreeFungibles.root;

        const dummyKey = jsUtils.newKeyPair();
        const erc1155ProofReceipt = await prover.prove(
            "JoinSplitErc1155",
            {
                message: 0n,
                valuesIn: [bob.coins[1].amount, 0n],
                keysIn: [bob.coins[1].key, dummyKey],
                valuesOut: [bob.coins[1].amount, 0n],
                keysOut: [{"publicKey": bob.knows.aliceReceivingKey.publicKey}, dummyKey],
                merkleProofs: [bob.coins[1].proof, {root: 0n}],
                treeNumbers: [bob.coins[1].treeNumber, 0n],
                erc1155TokenId: bob.coins[1]["tokenId"],
                erc1155ContractAddress: contracts.erc1155.address,
                assetGroup_treeNumber: bob.coins[1]["groupTreeNumber"],
                assetGroup_merkleProof: bob.coins[1]["groupProof"],
            }
        );
        console.log("Sending transfer ...");
        tx = await relayer.calls.erc1155Vault.transfer(erc1155ProofReceipt);

        const [eventCommitments, eventNullifiers] = await testHelpers.getCommitmentFromTxIndirect(tx, 
            [
                contracts.erc1155CoinVault.address
            ]
        );
        console.log("Done transfer");

        merkleTree1155.insertLeaves([eventCommitments[0], eventCommitments[1]]);
        alice.coins.push({
            "commitment":eventCommitments[0],
            "tokenId": bob.coins[1].tokenId,
            "amount": bob.coins[1].amount,
            "key":aliceReceivingKey,
            "treeNumber":merkleTree1155.lastTreeNumber,
            "proof":merkleTree1155.generateProof(eventCommitments[0]),
            "root": merkleTree1155.root,
            "groupTreeNumber": bob.coins[1].groupTreeNumber,
            "groupProof":bob.coins[1].groupProof,
        });

    });


    it("Bob transfers the fungible Erc1155 coin to Alice.", async () => {

        // Alice generates a public key to receive the payment
        console.log("Alice creates a new keyPair to receive Bob's coins");
        const aliceReceivingKey2 = jsUtils.newKeyPair();

        console.log("Alice shares the public key part with Bob");
        bob.knows.aliceReceivingKey2 = {privateKey:0n, publicKey:aliceReceivingKey2.publicKey};

//         // TODO:: Make it interactive to be sure that alice can open the payment coin
//         console.log("Alice knows the NFT_ID of the coin to be able to spend the coin later.");
//         alice.knows.nftId = NFT_ID;


        const erc1155ProofReceipt = await prover.prove(
            "OwnershipErc1155NonFungible",
            {
                message: 0n,
                values: [bob.coins[0]["amount"]],
                keysIn: [bob.coins[0]["key"]],
                keysOut: [bob.knows.aliceReceivingKey2],
                merkleProofs: [bob.coins[0]["proof"]],
                treeNumbers: [bob.coins[0]["treeNumber"]],
                erc1155TokenIds: [bob.coins[0]["tokenId"]],
                erc1155ContractAddress: contracts.erc1155.address,
                assetGroup_merkleProofs: [bob.coins[0]["groupProof"]],
                assetGroup_treeNumbers: [bob.coins[0]["groupTreeNumber"]]
            }
        );

        tx = await relayer.calls.erc1155Vault.transfer(erc1155ProofReceipt);

        const [eventCommitments, eventNullifiers] = await testHelpers.getCommitmentFromTxIndirect(tx, 
            [
                contracts.erc1155CoinVault.address
            ]
        );
        console.log("Done transfer");

        merkleTree1155.insertLeaves([eventCommitments[0]]);
        alice.coins.push({
            "commitment":eventCommitments[0],
            "tokenId": bob.coins[0].tokenId,
            "amount": bob.coins[0].amount,
            "key":aliceReceivingKey2,
            "treeNumber":merkleTree1155.lastTreeNumber,
            "proof":merkleTree1155.generateProof(eventCommitments[0]),
            "groupTreeNumber": bob.coins[0].groupTreeNumber,
            "groupProof":bob.coins[0].groupProof,
        });

    });


    it("Alice withdraws transferred fungible ERC1155 coin.", async () => {

        const dummyKey = jsUtils.newKeyPair();
        const erc1155ProofReceipt = await prover.prove(
            "JoinSplitErc1155",
            {
                message: 0n,
                valuesIn: [alice.coins[0].amount, 0n],
                keysIn: [alice.coins[0].key, dummyKey],
                valuesOut: [alice.coins[0].amount, 0n],
                keysOut: [{publicKey: BigInt(alice.wallet.address)}, dummyKey],
                merkleProofs: [alice.coins[0].proof, {root: 0n}],
                treeNumbers: [alice.coins[0].treeNumber, 0n],
                erc1155TokenId:alice.coins[0]["tokenId"],
                erc1155ContractAddress: contracts.erc1155.address,
                assetGroup_treeNumber: alice.coins[0].groupTreeNumber,
                assetGroup_merkleProof: alice.coins[0].groupProof
            }
        );

        await relayer.calls.erc1155Vault.withdraw(
            [
                alice.coins[0].amount, 
                alice.coins[0].tokenId,
                0n,
                0n
            ], 
            alice.wallet.address, 
            erc1155ProofReceipt
        );


        // checking balances
        const bals = await contracts.erc1155.balanceOf(
            alice.wallet.address, 
            alice.coins[0].tokenId
        );

        expect(bals).to.equal(alice.coins[0].amount);

        console.log("Alice's fungible token balance is as expected.")
    });

    it("Alice withdraws transferred non-fungible ERC1155 coin.", async () => {

        const erc1155ProofReceipt = await prover.prove(
            "OwnershipErc1155NonFungible",
            {
                message: 0n,
                values: [alice.coins[1]["amount"]],
                keysIn: [alice.coins[1]["key"]],
                keysOut: [{publicKey: BigInt(alice.wallet.address) }],
                merkleProofs: [alice.coins[1]["proof"]],
                treeNumbers: [alice.coins[1]["treeNumber"]],
                erc1155TokenIds: [alice.coins[1]["tokenId"]],
                erc1155ContractAddress: contracts.erc1155.address,
                assetGroup_merkleProofs: [alice.coins[1]["groupProof"]],
                assetGroup_treeNumbers: [alice.coins[1]["groupTreeNumber"]]
            }
        );

        await relayer.calls.erc1155Vault.withdraw(
            [
                alice.coins[1].amount, 
                alice.coins[1].tokenId 
            ], 
            alice.wallet.address, 
            erc1155ProofReceipt
        );


        // checking balances
        const bals = await contracts.erc1155.balanceOf(
            alice.wallet.address, 
            alice.coins[1].tokenId
        );

        expect(bals).to.equal(alice.coins[1].amount);

        console.log("Alice's non-fungible token balance is as expected.")
    });




});