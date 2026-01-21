/* global describe it ethers before beforeEach afterEach overwriteArtifact */
const { expect } = require("chai");
const poseidonGenContract = require("circomlibjs/src/poseidon_gencontract");
const prover = require("../src/core/prover");
const utils = require("../src/core/utils");
const MerkleTree = require("../src/core/merkle");
const { getVerificationKeys } = require("../src/core/dvpSnarks");
const crypto = require("crypto")

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
let merkleTree721;
let merkleTree20;

let depositCount;
let swapCount;


describe(`Testing ZkDvp functionalities for TREE_DEPTH = ${TREE_DEPTH}`, () => {

        
    it(`ZkDvp should initialize properly `, async () => {

        let userCount = 2;
        [owner, users, contracts, merkleTrees] = await testHelpers.deployForTest(userCount);
        console.log("SwapTest: ZkDvp initialization");
        console.log("User: ",users);
        console.log("MerkleTree: ", merkleTrees);


        alice = users[0].wallet;
        bob = users[1].wallet;

        // let nftAlice = contracts["erc721"].connect(alice);

        merkleTree721 = merkleTrees["ERC721"].tree;
        merkleTree20 = merkleTrees["ERC20"].tree;        

        depositCount = 50;
        swapCount = 50;
        console.log("loadTest: ZkDvp initialization")
    });


    it(`depositing ${depositCount} coins, then swapping random coins.`, async () => {
        const erc721Contract = contracts["erc721"];
        const erc721VaultContract = contracts["erc721CoinVault"];

        const erc721VaultAlice = erc721VaultContract.connect(alice);
        const erc721Alice = erc721Contract.connect(alice);
        const erc721Owner = erc721Contract.connect(owner);
        const zkDvpContract = contracts["zkdvp"];
        const zkDvpAlice = zkDvpContract.connect(alice);
        const zkDvpBob = zkDvpContract.connect(bob);
        const erc20Contract = contracts["erc20"];
        const erc20VaultContract = contracts["erc20CoinVault"];

        const erc20Bob = erc20Contract.connect(bob);
        const erc20Owner = erc20Contract.connect(owner);
        const erc20VaultBob = erc20VaultContract.connect(bob);


        counter = 0;
        let aliceErc721Coins = [];
        let aliceErc20Coins = [];
        let bobErc721Coins = [];
        let bobErc20Coins = [];

        while(counter < depositCount){

            // Mint NFT for Alice
            let NFT_ID = utils.buffer2BigInt(Buffer.from(crypto.randomBytes(3))) % utils.SNARK_SCALAR_FIELD;
            let nftKeyDeposit = utils.newKeyPair();

            await erc721Owner.mint(alice.address, NFT_ID);
            // Approve ZkDvp as an operator for Alice's NFT
            await erc721Alice.approve(erc721VaultContract.address, NFT_ID);
            // Deposit Alice's NFT into ZkDvp
            let tx = await erc721VaultAlice.deposit(
                [
                    NFT_ID,
                    nftKeyDeposit.publicKey
                ]
            );

            let cmt = await testHelpers.getCommitmentFromTx(tx);
            merkleTree721.insertLeaves([cmt]);
            let proof0 = merkleTree721.generateProof(cmt);
            aliceErc721Coins.push({"value":NFT_ID, "key":nftKeyDeposit, "commitment": cmt, "proof":proof0, "root":merkleTree721.root, "treeNumber":merkleTree721.lastTreeNumber});

            // Bob deposit 2x10 ethers into ZkDvp
            let depositAmount = utils.buffer2BigInt(Buffer.from(crypto.randomBytes(3)));
            // console.log("Deposit Amount: "+ depositAmount);

            // minting ERC20 tokens for Bob
            tx = await erc20Owner.mint(bob.address, depositAmount);
            await tx.wait();

            // Approve ZkDvp to transfer tokens
            await erc20Bob.approve(erc20VaultContract.address, depositAmount);

            // creating depositKeys for Bob to Deposit Erc20 tokens to ZkDvp
            let fundKey = utils.newKeyPair();

            // adding new erc20 coin to bob's erc20 coin list

            tx = await erc20VaultBob.deposit(
                [depositAmount,
                fundKey.publicKey]
            );

            cmt = await testHelpers.getCommitmentFromTx(tx);
            merkleTree20.insertLeaves([cmt]);
            let proof1 = merkleTree20.generateProof(cmt);

            bobErc20Coins.push({"value":depositAmount, "key":fundKey, "commitment":cmt, "proof":proof1, "root":merkleTree20.root, "treeNumber":merkleTree20.lastTreeNumber});

            console.log("Alice's new ERC721 coin:", aliceErc721Coins[counter]);
            console.log("Bob's new ERC20 coin:", bobErc20Coins[counter]);
            console.log(counter + ".created ERC721 coin for Alice and ERC20 for Bob.");
            counter = counter + 1;
        }


        console.log("-----------------------------------------------");
        console.log(`------- SWAPPING ---${swapCount} times-----------`);
        console.log("-----------------------------------------------");

        for(var i = 0 ;i<swapCount;i++){

            let aliceId = Math.floor(Math.random() * (aliceErc721Coins.length));
            let bobId1 = Math.floor(Math.random() * (bobErc20Coins.length));
            let bobId2 = Math.floor(Math.random() * (bobErc20Coins.length));

            console.log("Selected alice commitment: ", aliceId);
            console.log("Selected bob commitment1: ", bobId1);
            console.log("Selected bob commitment2: ", bobId2);

            NFT_ID = aliceErc721Coins[aliceId]["value"];
            nftKeyDeposit =    aliceErc721Coins[aliceId]["key"];
            treeNumber0 =    aliceErc721Coins[aliceId]["treeNumber"];
            proof0 = aliceErc721Coins[aliceId]["proof"];
            root0 = aliceErc721Coins[aliceId]["root"];

            let fundKeys = [bobErc20Coins[bobId1]["key"], bobErc20Coins[bobId2]["key"]];
            let depositAmount1 = bobErc20Coins[bobId1]["value"];
            let depositAmount2 = bobErc20Coins[bobId2]["value"];
            let treeNumber1 =    bobErc20Coins[bobId1]["treeNumber"];
            let proof1 =    bobErc20Coins[bobId1]["proof"];
            let root1 =    bobErc20Coins[bobId1]["root"];
            let treeNumber2 =    bobErc20Coins[bobId2]["treeNumber"];
            let proof2 =    bobErc20Coins[bobId2]["proof"];
            let root2 =    bobErc20Coins[bobId2]["root"];

            // Alice generates NFT commitment for Bob
            let uid = utils.erc721UniqueId(erc721Contract.address, NFT_ID);

            // Bob generates a public key to receive the NFT
            let bobNFTKey = utils.newKeyPair();
            // Bob generates a public to receive the change
            let bobChangeKey = utils.newKeyPair();
            // Alice generates a public key to receive the payment
            let alicePaymentKey = utils.newKeyPair();

            // nftCommitment will be used as a massage by Bob
            let nftCommitment = utils.getCommitment(uid, bobNFTKey.publicKey);

            // Bob generates payment commitment for Alice
            let paymentAmount = depositAmount1 + 10n;
            let changeAmount = depositAmount1 + depositAmount2 - paymentAmount;

            // creating unique erc20Commitment
            let erc20Uid = utils.erc20UniqueId(
                erc20Contract.address,
                paymentAmount
            );

            // paymentCommitment will be used as a massage by Alice
            let paymentCommitment = utils.getCommitment(
                erc20Uid,
                alicePaymentKey.publicKey,
            );

            const ownParams = await prover.prove(
                    "OwnershipErc721",
                    {
                            message: paymentCommitment,
                            values: [NFT_ID],
                            keysIn: [nftKeyDeposit],
                            keysOut: [bobNFTKey],
                            merkleProofs: [proof0],
                            treeNumbers: [treeNumber0],
                            erc721ContractAddress: erc721Contract.address
                    }
            );
            const jsParams = await prover.prove(
                    "JoinSplitErc20",
                    {
                            message: nftCommitment,
                            valuesIn: [depositAmount1, depositAmount2],
                            keysIn: fundKeys,
                            valuesOut: [paymentAmount, changeAmount],
                            keysOut: [alicePaymentKey, bobChangeKey],
                            merkleProofs: [proof1, proof2],
                            treeNumbers: [treeNumber1, treeNumber2],
                            erc20ContractAddress: erc20Contract.address,
                    }
            );

            // A relayer forwards both transactions to ZkDvp

            // if two coins are not the same
            // the swap should work properly.
            if(bobId1 != bobId2){

                    // await zkDvpContract.swap(jsParams, ownParams, 0, 1);
                    await zkDvpContract.submitPartialSettlement(jsParams, 0 ,0);
                    await zkDvpContract.submitPartialSettlement(ownParams, 1 ,1);
                    console.log("Done swap....");

                    // inserting new commitments to local merkleTree
                    let commitments = [jsParams.statement[7], jsParams.statement[8], ownParams.statement[4]];
                    merkleTree20.insertLeaves([commitments[0], commitments[1]]);
                    merkleTree721.insertLeaves([commitments[2]]);

                    // removing old coins from coin lists
                    if(bobId1 > bobId2){
                        bobErc20Coins.splice(bobId1, 1);
                        bobErc20Coins.splice(bobId2, 1);
                    }
                    else{
                        bobErc20Coins.splice(bobId2, 1);
                        bobErc20Coins.splice(bobId1, 1);
                    }
                    aliceErc721Coins.splice(aliceId, 1);

                    proof10 = merkleTree721.generateProof(commitments[2]);
                    proof11 = merkleTree20.generateProof(commitments[0]);
                    proof12 = merkleTree20.generateProof(commitments[1]);

                    root10 = merkleTree721.root;
                    root11 = merkleTree20.root;
                    root12 = merkleTree20.root;
                    // adding new coins to the coin lists
                    bobErc721Coins.push({"value": NFT_ID, "key":bobNFTKey, "commitment":commitments[2], "proof": proof10, "root":root10, "treeNumber": merkleTree721.lastTreeNumber});
                    aliceErc20Coins.push({"value": paymentAmount, "key":alicePaymentKey, "commitment":commitments[0], "proof": proof11, "root":root11, "treeNumber": merkleTree20.lastTreeNumber});
                    bobErc20Coins.push({"value": changeAmount, "key":bobChangeKey, "commitment":commitments[1], "proof": proof12, "root": root12, "treeNumber": merkleTree20.lastTreeNumber});

            }
            else{
                // if two coins are the same, the swap should fail.
                try{
                    await zkDvpContract.swap(jsParams, ownParams, 1, 0);
                    console.log('ZkDvp should have thrown error. Double Spent attack happened. DAMN!!!');
                }
                catch(ex){
                    console.log('ZkDvp has thrown error on Double-Spent. All good.')
                }

            }
        }

        console.log("-----------------------------------------------");
        console.log("-----------------------------------------------");
        console.log("-----------------------------------------------");
        console.log("Alice Erc20 coins: " + aliceErc20Coins.length);
        console.log("Alice Erc721 coins: " + aliceErc721Coins.length);
        console.log("Bob Erc20 coins: " + bobErc20Coins.length);
        console.log("Bob Erc721 coins: " + bobErc721Coins.length);

        console.log("-----------------------------------------------");
        console.log("-----------------------------------------------");
        console.log("-----------------------------------------------");

        console.log("Alice withdraws Erc20 coins...")

        for (const coin of aliceErc20Coins) { 

            let dummyKey = utils.newKeyPair();
            const aliceJS = await prover.prove(
                "JoinSplitErc20",
                {
                    message: 0n,
                    valuesIn: [coin["value"], 0n],
                    keysIn: [coin["key"], dummyKey],
                    valuesOut: [coin["value"], 0n],
                    keysOut: [{ publicKey: BigInt(alice.address) }, dummyKey],
                    merkleProofs: [coin["proof"], {"root": 0n}],
                    treeNumbers: [coin["treeNumber"], 0n],
                    erc20ContractAddress: erc20Contract.address,
                }
            );

            let oldBalance = (
                await erc20Contract.balanceOf(alice.address)
            ).toBigInt();

            tx = await erc20VaultContract.withdraw([coin["value"]], alice.address, aliceJS);
            let newBalance = (
                await erc20Contract.balanceOf(alice.address)
            ).toBigInt();
            console.log("Alice's ERC20 balance: " + newBalance);
            await expect(oldBalance + coin["value"]).to.equal(newBalance);

        }

        console.log("Alice withdraws Erc721 coins...")

        for (const coin of aliceErc721Coins) { 

            let uid = utils.erc721UniqueId(erc721Contract.address, coin["value"]);
            const aliceNFT = await prover.prove(
                "OwnershipErc721",
                {
                    message: 0n,
                    values: [coin["value"]],
                    keysIn: [coin["key"]],
                    keysOut: [{"publicKey": BigInt(alice.address)}],
                    treeNumbers: [coin["treeNumber"]],
                    merkleProofs: [coin["proof"]],
                    erc721ContractAddress: erc721Contract.address
                }
            );

            // TX sent by a relayer
            tx = await erc721VaultContract.withdraw([coin["value"]], alice.address, aliceNFT);

            let res = await erc721Contract.ownerOf(coin["value"]);
            expect(res).to.equal(alice.address);
            console.log("Alice withdrew ERC721 with id = " + coin["value"]);

        }

        console.log("Bob withdraws Erc20 coins...")

        for (const coin of bobErc20Coins) { 
            let dummyKey = utils.newKeyPair();
            const bobJs = await prover.prove(
                "JoinSplitErc20",
                {
                    message: 0n,
                    valuesIn: [coin["value"], 0n],
                    keysIn: [coin["key"], dummyKey],
                    valuesOut: [coin["value"], 0n],
                    keysOut: [{ publicKey: BigInt(bob.address) }, dummyKey],
                    merkleProofs: [coin["proof"], {"root": 0n}],
                    treeNumbers: [coin["treeNumber"], 0n],
                    erc20ContractAddress: erc20Contract.address,
                }
            );

            let oldBalance = (
                await erc20Contract.balanceOf(bob.address)
            ).toBigInt();
            console.log("Bob's ERC20 balance: " + oldBalance);

            tx = await erc20VaultContract.withdraw([coin["value"]], bob.address, bobJs);
            let newBalance = (
                await erc20Contract.balanceOf(bob.address)
            ).toBigInt();
            await expect(oldBalance + coin["value"]).to.equal(newBalance);
        }

        console.log("Bob withdraws Erc721 coins...")

        for (const coin of bobErc721Coins) { 

            const bobNFT = await prover.prove(
                "OwnershipErc721",
                {
                    message: 0n,
                    values: [coin["value"]],
                    keysIn: [coin["key"]],
                    keysOut: [{"publicKey": BigInt(bob.address)}],
                    treeNumbers: [coin["treeNumber"]],
                    merkleProofs: [coin["proof"]],
                    erc721ContractAddress: erc721Contract.address
                }
            );

            // TX sent by a relayer
            tx = await erc721VaultContract.withdraw([coin["value"]], bob.address, bobNFT);
            
            let res = await erc721Contract.ownerOf(coin["value"]);
            expect(res).to.equal(bob.address);

            console.log("Bob withdrew ERC721 with id = " + coin["value"]);

        }


        console.log("-----------------------------------------------");
        console.log("-----------------------------------------------");
        console.log("-----------------------------------------------");


    });

});
