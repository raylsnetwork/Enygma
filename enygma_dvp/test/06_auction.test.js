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
const dvpConf = require("../zkdvp.config.json");
const TREE_DEPTH = dvpConf["tree-depth"];
const testHelpers = require("./testHelpers.js");
const { babyJub } = require("@iden3/js-crypto");

let users;
let contracts;
let merkleTree721;
let merkleTree20;
let merkleTreeFungibles;
let merkleTreeNonFungibles;

let alice = {};
let bob = {};
let carl = {};
let auctioneer = {};
let owner = {};
let auctionData;
let winnerParams;
let auctionSummary = {};

let depositCount;
let swapCount;

let inKeys = [];
describe("ZkAuction Test", () => {

    it(`ZkDvp should initialize properly `, async () => {
            
        let userCount = 3; // Alice:seller, Bob and Carl: Bidders
        [admin, users, contracts, merkleTrees, auditors] = await testHelpers.deployForTest(userCount);

        auditor = auditors[0];

        alice.wallet = users[0].wallet;
        bob.wallet =     users[1].wallet;
        carl.wallet =    users[2].wallet;
        auctioneer.wallet = admin;

        owner.wallet = admin

        alice.calls = {};
        bob.calls = {};
        carl.calls = {};
        owner.calls = {};
        auctioneer.calls = {};
        auditor.calls = {};

        alice.knows = {};
        bob.knows = {};
        carl.knows = {};
        auditor.knows = {};

        bob.bids = [];
        carl.bids = [];

        alice.coins = [];
        bob.coins = [];
        carl.coins = [];

        auctioneer.openings = [];
        auctioneer.bidLogs = [];


        auctioneer.offchainId = 0;
        auctioneer.auctionGroupId = 0;

        auctioneer.calls.zkdvp = contracts.zkdvp.connect(auctioneer.wallet);
        auctioneer.calls.zkAuction = contracts.zkAuction.connect(auctioneer.wallet);
        auctioneer.keyPair = jsUtils.babyKeyPair(babyJub);

        auditor.calls.zkdvp = contracts.zkdvp.connect(auditor.wallet);
        auditor.calls.erc1155 = contracts.erc1155.connect(auditor.wallet);
        auditor.calls.erc1155Vault = contracts.erc1155CoinVault.connect(auditor.wallet);

        owner.calls.erc721 = contracts.erc721.connect(owner.wallet);
        owner.calls.erc20 = contracts.erc20.connect(owner.wallet);

        alice.calls.zkdvp = contracts.zkdvp.connect(alice.wallet);
        alice.calls.zkAuction = contracts.zkAuction.connect(alice.wallet);
        alice.calls.erc721 = contracts.erc721.connect(alice.wallet);
        alice.calls.erc721Vault = contracts.erc721CoinVault.connect(alice.wallet);

        bob.calls.zkdvp = contracts.zkdvp.connect(bob.wallet);
        bob.calls.zkAuction = contracts.zkAuction.connect(bob.wallet);
        bob.calls.erc20 = contracts.erc20.connect(bob.wallet);
        bob.calls.erc20Vault = contracts.erc20CoinVault.connect(bob.wallet);

        carl.calls.zkdvp = contracts.zkdvp.connect(carl.wallet);
        carl.calls.zkAuction = contracts.zkAuction.connect(carl.wallet);
        carl.calls.erc20 = contracts.erc20.connect(carl.wallet);
        carl.calls.erc20Vault = contracts.erc20CoinVault.connect(carl.wallet);

        merkleTree721 = merkleTrees["ERC721"].tree;
        merkleTree20 = merkleTrees["ERC20"].tree;
        merkleTreeFungibles = merkleTrees["fungibleGroup"].tree;
        merkleTreeNonFungibles = merkleTrees["nonFungibleGroup"].tree;

        console.log("Sharing Auditor's publicKey with Alice and Bob.")
        alice.knows.auditorPublicKey = auditor.keyPair.publicKey;
        bob.knows.auditorPublicKey = auditor.keyPair.publicKey;
        carl.knows.auditorPublicKey = auditor.keyPair.publicKey;

        alice.knows.auctioneerPublicKey = auctioneer.keyPair.publicKey;
        bob.knows.auctioneerPublicKey = auctioneer.keyPair.publicKey;
        carl.knows.auctioneerPublicKey = auctioneer.keyPair.publicKey;

        console.log("Auction Test: ZkDvp initialization")

    });

    it("System Admin registers Auctioneer.", async () => {
        await auctioneer.calls.zkAuction.registerAuctioneer(
            auctioneer.offchainId, 
            auctioneer.auctionGroupId, 
            auctioneer.keyPair.publicKey
        );

    });    

    it("Alice should deposit an ERC721 token.", async () => {

        let NFT_ID = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(4))) % jsUtils.SNARK_SCALAR_FIELD;
        let aliceErc721Key = jsUtils.newKeyPair();

        await owner.calls.erc721.mint(alice.wallet.address, NFT_ID);

        // Approve ZkDvp as an operator for Alice's NFT
        await alice.calls.erc721.approve(contracts.erc721CoinVault.address, NFT_ID);
        // Deposit Alice's NFT into ZkDvp
        let tx = await alice.calls.erc721Vault.deposit(
            [NFT_ID,
            aliceErc721Key.publicKey]
        );

        let cmt = await testHelpers.getCommitmentFromTx(tx);

        merkleTree721.insertLeaves([cmt]);
        let proof0 = merkleTree721.generateProof(cmt);
        alice.coins.push(
            {
                "value":NFT_ID, 
                "tokenAddress": contracts.erc721.address,
                "key":aliceErc721Key, 
                "commitment": cmt, 
                "proof":proof0, 
                "root":merkleTree721.root, 
                "treeNumber":merkleTree721.lastTreeNumber,
                "vaultId": 1n
            }
        );
    });

    it("Bob and Carl should deposit ERC20 tokens.", async () => {

        console.log("Depositing two erc20 coins for Bob");

        // Bob deposit 2x10 ethers into ZkDvp
        let bobDepositAmount = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(4)));
        // console.log("Deposit Amount: "+ depositAmount);

        // minting ERC20 tokens for Bob
        let tx = await owner.calls.erc20.mint(bob.wallet.address, bobDepositAmount * 2n);
        await tx.wait();

        // Approve ZkDvp to transfer tokens
        await bob.calls.erc20.approve(contracts.erc20CoinVault.address, bobDepositAmount * 2n);

        for(var i = 0; i< 2; i++){
            // creating depositKeys for Bob to Deposit Erc20 tokens to ZkDvp
            let bobErc20Key = jsUtils.newKeyPair();

            // adding new erc20 coin to bob's erc20 coin list

            tx = await bob.calls.erc20Vault.deposit(
                [bobDepositAmount,
                bobErc20Key.publicKey]
            );

            const bobCmt = await testHelpers.getCommitmentFromTx(tx);
            merkleTree20.insertLeaves([bobCmt]);
            let bobProof = merkleTree20.generateProof(bobCmt);

            bob.coins.push(
                {        
                    "vaultId": 0n,
                    "value":bobDepositAmount, 
                    "tokenAddress": contracts.erc20.address,
                    "key":bobErc20Key, 
                    "commitment":bobCmt, 
                    "proof":bobProof, 
                    "root":merkleTree20.root, 
                    "treeNumber":merkleTree20.lastTreeNumber
                }
            );

        }


        console.log("Depositing two erc20 coins for Carl");
        let carlDepositAmount = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(3)));

        // minting ERC20 tokens for Bob
        tx = await owner.calls.erc20.mint(carl.wallet.address, carlDepositAmount * 2n);
        await tx.wait();

        // Approve ZkDvp to transfer tokens
        await carl.calls.erc20.approve(contracts.erc20CoinVault.address, carlDepositAmount * 2n);

        for(var i = 0; i< 2; i++){

            // creating depositKeys for Bob to Deposit Erc20 tokens to ZkDvp
            let carlErc20Key = jsUtils.newKeyPair();

            // adding new erc20 coin to bob's erc20 coin list

            tx = await carl.calls.erc20Vault.deposit(
                [carlDepositAmount,
                carlErc20Key.publicKey]
            );

            cmt = await testHelpers.getCommitmentFromTx(tx);
            merkleTree20.insertLeaves([cmt]);
            let carlProof = merkleTree20.generateProof(cmt);

            carl.coins.push(
                {        
                    "vaultId": 0n,
                    "value":carlDepositAmount, 
                    "tokenAddress": contracts.erc20.address,
                    "key":carlErc20Key, 
                    "commitment":cmt, 
                    "proof":carlProof, 
                    "root":merkleTree20.root, 
                    "treeNumber":merkleTree20.lastTreeNumber
                }
            );


        }
        console.log("----------------------------")
        // console.log("Alice.coins:\n", JSON.stringify(alice.coins, null, 4)    + "\n");
        // console.log("Bob.coins:\n", JSON.stringify(bob.coins, null, 4) + "\n");
        // console.log("Carl.coins:\n", JSON.stringify(carl.coins, null, 4) + "\n");

    });

    it("Alice should be able to start the auction.", async () => {
    
        let aliceRecFundKey = jsUtils.newKeyPair();
        const beacon = 0n; // TODO:: connect beacon

        const auctionInitProof = await prover.prove(
            "AuctionInit_Auditor",
            {
                beacon: 0n,
                vaultId: alice.coins[0].vaultId,
                keysIn: [alice.coins[0].key],
                treeNumbers: [alice.coins[0].treeNumber],
                merkleProofs: [alice.coins[0].proof],
                contractAddress: BigInt(contracts.erc721.address),
                idParams: [alice.coins[0].value, 0n, 0n, 0n, 0n],
                auditor_publicKey: alice.knows.auditorPublicKey,
                assetGroup_treeNumber: 0n,
                assetGroup_merkleProof: {root: 0n}
            }
        );
        // console.log("auctionInitProof: \n", JSON.stringify(auctionInitProof, null, 4));
        const newAuctionTx = await alice.calls.zkAuction.newAuction(
                                                                        [alice.coins[0].value],
                                                                        1, // itemVaultId
                                                                        0, // bidVaultId
                                                                        1, // non-fungible assetGroup id
                                                                        0, // fungible assetGroup id
                                                                        aliceRecFundKey.publicKey,
                                                                        auctionInitProof
                                                                );
        auctionData = await testHelpers.parseAuctionInitEvent(newAuctionTx);

        console.log("AuctionData from Event: ", JSON.stringify(auctionData, null, 4));
        auctionData.sellerKey = aliceRecFundKey;

        // TODO:: winner should know this
        auctionData.itemTokenId = alice.coins[0].value;

    });

    it("Alice should NOT be able to re-start the auction.", async () => {

        // TODO:: implement
    });

    it("Bob should be able to Bid with his deposited coins.", async () => {

        let bobBidAmount = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(3))) % jsUtils.SNARK_SCALAR_FIELD;
        
        // if bid > totalDeposit the proof wont pass
        while(bobBidAmount > (bob.coins[0].value + bob.coins[1].value)){
                bobBidAmount = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(3))) % jsUtils.SNARK_SCALAR_FIELD;
        }
        let bobBidRandom = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(16))) % jsUtils.SNARK_SCALAR_FIELD;

        let bobExcessFundKey = jsUtils.newKeyPair();
        let bobItemReceivingKey = jsUtils.newKeyPair();

        console.log(`Bob's bid values: ${bobBidAmount}, ${(bob.coins[0].value + bob.coins[1].value) - bobBidAmount}`);
        const excessAmount = (bob.coins[0].value + bob.coins[1].value) - bobBidAmount;

        const bobBidProof = await prover.prove(
            "AuctionBid_Auditor",
            {
                beacon: 0n,
                auctionId: auctionData.auctionId,
                bidAmount: bobBidAmount, 
                bidRandom: bobBidRandom,
                vaultId: bob.coins[0].vaultId,
                keysIn: [bob.coins[0].key, bob.coins[1].key],
                keysOut: [
                    {"publicKey": auctionData.sellerFundCoinPublicKey}, 
                    {"publicKey": bobExcessFundKey.publicKey}
                ],
                treeNumbers: [bob.coins[0].treeNumber, bob.coins[1].treeNumber],
                merkleProofs: [bob.coins[0].proof, bob.coins[1].proof],
                contractAddress: BigInt(bob.coins[0].tokenAddress),
                auditor_publicKey: bob.knows.auditorPublicKey,
                assetGroup_treeNumber: 0n,
                assetGroup_merkleProof: {root: 0n},
                idParamsIn: [[bob.coins[0].value,0n,0n,0n,0n], [bob.coins[1].value,0n,0n,0n,0n]],
                idParamsOut: [[bobBidAmount,0n,0n,0n,0n], [excessAmount,0n,0n,0n,0n]],
                auctioneer_publicKey: bob.knows.auctioneerPublicKey
            }
        );


        console.log("Bob's bid proof: ", JSON.stringify(bobBidProof, null, 4));
        // const testUid = jsUtils.erc20UniqueId(contracts.erc20.address, excessAmount);
        // console.log("Bobs's excess amount: ", excessAmount);
        // console.log("Bobs's excess uid: ", testUid);

        const bobBidTx = await bob.calls.zkAuction.submitBid(bobBidProof, bobItemReceivingKey.publicKey);

        // storing Bob's bid data locally
        bob.bids.push(
            {
                "auctionId": auctionData.auctionId,
                "amount": bobBidAmount,
                "random": bobBidRandom,
                "proof": bobBidProof,
                "receivingKey": bobItemReceivingKey,
                "excessKey": bobExcessFundKey,
                "excessAmount": (bob.coins[0].value + bob.coins[1].value) - bobBidAmount,
                "blindedBid": bobBidProof.statement[2]
            }
        );

        auctioneer.bidLogs.push({... bob.bids[0]});

    });

    it("Carl should be able to Bid.", async () => {


        let carlBidAmount = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(3))) % jsUtils.SNARK_SCALAR_FIELD;
        // if bid > totalDeposit the proof wont pass
        while(carlBidAmount > (carl.coins[0].value + carl.coins[1].value)){
                carlBidAmount = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(3))) % jsUtils.SNARK_SCALAR_FIELD;
        }

        let carlBidRandom = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(16))) % jsUtils.SNARK_SCALAR_FIELD;

        let carlExcessFundKey = jsUtils.newKeyPair();
        let carlItemReceivingKey = jsUtils.newKeyPair();
        console.log(`Carls's bid values: ${carlBidAmount}, ${(carl.coins[0].value + carl.coins[1].value) - carlBidAmount}`);
        const excessAmount2 = (carl.coins[0].value + carl.coins[1].value) - carlBidAmount;

        const carlBidProof = await prover.prove(
            "AuctionBid_Auditor",
            {
                beacon: 0n,
                auctionId: auctionData.auctionId,
                bidAmount: carlBidAmount, 
                bidRandom: carlBidRandom,
                vaultId: carl.coins[0].vaultId,
                keysIn: [carl.coins[0].key, carl.coins[1].key],
                keysOut: [
                    {"publicKey": auctionData.sellerFundCoinPublicKey}, 
                    {"publicKey": carlExcessFundKey.publicKey}
                ],
                treeNumbers: [carl.coins[0].treeNumber, carl.coins[1].treeNumber],
                merkleProofs: [carl.coins[0].proof, carl.coins[1].proof],
                contractAddress: BigInt(carl.coins[0].tokenAddress),
                auditor_publicKey: carl.knows.auditorPublicKey,
                assetGroup_treeNumber: 0n,
                assetGroup_merkleProof: {root: 0n},
                idParamsIn: [[carl.coins[0].value,0n,0n,0n,0n], [carl.coins[1].value,0n,0n,0n,0n]],
                idParamsOut: [[carlBidAmount,0n,0n,0n,0n], [excessAmount2,0n,0n,0n,0n]],
                auctioneer_publicKey: carl.knows.auctioneerPublicKey
            }
        );


        const carlBidTx = await carl.calls.zkAuction.submitBid(carlBidProof, carlItemReceivingKey.publicKey);

        // storing Bob's bid data locally
        carl.bids.push(
            {
                "auctionId": auctionData.auctionId,
                "amount": carlBidAmount,
                "random": carlBidRandom,
                "proof": carlBidProof,
                "receivingKey": carlItemReceivingKey,
                "excessKey": carlExcessFundKey,
                "excessAmount": (carl.coins[0].value + carl.coins[1].value) - carlBidAmount,
                "blindedBid": carlBidProof.statement[2]
            }
        );
        auctioneer.bidLogs.push({... carl.bids[0]});


    });

    // it("Carl should be able to open the blindedBid publicly on-chain.", async () => {

    //             console.log("Carl's bid: ", JSON.stringify(carl.bids[0], null, 4));
    //             const bidInfo = await carl.calls.zkAuction.getBid(carl.bids[0].auctionId, carl.bids[0].blindedBid);
                
    //             const bidInfoState = bidInfo[0];
    //             await expect(bidInfo[0]).to.equal(1); //BID_SEALED state
 
    //             const publicOpeningTx = await carl.calls.zkAuction.publicOpeningReceipt(    
    //                                                                                                     carl.bids[0].auctionId,
    //                                                                                                     carl.bids[0].amount,
    //                                                                                                     carl.bids[0].random
    //                                                                                             )

    //             const bidInfo2 = await carl.calls.zkAuction.getBid(carl.bids[0].auctionId, carl.bids[0].blindedBid);
    //             const bidInfo2State = bidInfo2[0];
    //             await expect(bidInfo2[0]).to.equal(2); //BID_OPENED_PUBLICLY

    //             const bidInfo2Struct = {
    //                                                                     "bidState": bidInfo2State,
    //                                                                     "blindedBid": bidInfo2[1].toBigInt(),
    //                                                                     "bidAmount": bidInfo2[2].toBigInt(),
    //                                                                     "bidRandom": bidInfo2[3].toBigInt(),
    //                                                             };
    //             console.log("Carl's bid data after opening: ", JSON.stringify(bidInfo2Struct, null, 4));


    //             console.log("Auctioneer listens to AuctionBidOpenedPublicly event");
    //             const openingData = await testHelpers.parseAuctionPublicOpeningEvent(publicOpeningTx);

    //             auctioneer.openings.push({         
    //                                                                             "blindedBid": openingData.blindedBid,
    //                                                                             "auctionId": openingData.auctionId,
    //                                                                             "amount": openingData.bidAmount,
    //                                                                             "random": openingData.bidRandom
    //                                                                     });

    // });


    // it("Bob should be able to open the blindedBid privately to auctioneer.", async () => {

    //     const bidInfo = await bob.calls.zkAuction.getBid(bob.bids[0].auctionId, bob.bids[0].blindedBid);
    //     const bidInfoState = bidInfo[0];
    //     await expect(bidInfo[0]).to.equal(1); //BID_SEALED state

    //     console.log("Bob privately sends his bid's opening info to auctioneer");
    //     const receivedAuctionId = bob.bids[0].auctionId;
    //     const receivedBlindedBid = bob.bids[0].blindedBid;
    //     const receivedBidAmount = bob.bids[0].amount;
    //     const receivedBidRandom = bob.bids[0].random;

    //     await expect(
    //         jsUtils.pedersen(
    //                         receivedBidAmount, 
    //                         receivedBidRandom)
    //         ).to.equal(receivedBlindedBid);

    //     console.log("Auctioneer should confirm the auctionId and bid state from on-chain data. [NOT IMPlEMENTED]");

    //     auctioneer.openings.push( 
    //         {         
    //                 "blindedBid": receivedBlindedBid,
    //                 "auctionId": receivedAuctionId,
    //                 "amount": receivedBidAmount,
    //                 "random": receivedBidRandom
    //         }
    //     );


    //     console.log(auctioneer.openings[1]);

    //     console.log("Auctioneer generates privateOpeningProof");
    //     const privateOpeninigProof = await prover.prove(
    //         "AuctionPrivateOpening",
    //         {
    //             "auctionId": auctioneer.openings[1].auctionId,
    //             "blindedBid": auctioneer.openings[1].blindedBid,
    //             "bidAmount": auctioneer.openings[1].amount,
    //             "bidRandom": auctioneer.openings[1].random
    //         }
    //     );

    //     console.log("Auctioneer sends privateOpeningProof on-chain");
    //     console.log(privateOpeninigProof);

    //     await auctioneer.calls.zkAuction.privateOpeningReceipt(privateOpeninigProof);

    //     const bidInfo2 = await bob.calls.zkAuction.getBid(bob.bids[0].auctionId, bob.bids[0].blindedBid);
    //     const bidInfo2State = bidInfo2[0];
    //     await expect(bidInfo2[0]).to.equal(3); //BID_OPENED_PRIVATELY

    //     const bidInfo2Struct = {
    //                                 "bidState": bidInfo2State,
    //                                 "blindedBid": bidInfo2[1].toBigInt(),
    //                                 "bidAmount": bidInfo2[2].toBigInt(),
    //                                 "bidRandom": bidInfo2[3].toBigInt(),
    //                         };
    //     console.log("Bob's bid data after private opening: ", JSON.stringify(bidInfo2Struct, null, 4));


    // });


    it("Auctioneer should declare the correct winner.", async () => {


        console.log(JSON.stringify(auctioneer.bidLogs, null, 4));

        console.log("Auctioneer decrypts bid amounts");

        for (var i = 0; i < auctioneer.bidLogs.length; i++) {

            const bStat = auctioneer.bidLogs[i].proof.statement;

            const authKey = bStat.slice(14,16);
            const nonce = bStat[16];
            const encrypted = bStat.slice(17,21);
            const length = 3;

            const logDict = {authKey, nonce, encrypted, length};
            console.log(JSON.stringify(logDict, null, 4));

            const decrypted = jsUtils.poseidonDecryptWrapper(
                    babyJub, 
                    encrypted, 
                    authKey, 
                    nonce, 
                    auctioneer.keyPair.privateKey, 
                    length
                );
            console.log(`Decrypted values: ${decrypted}`);

            auctioneer.openings.push(
                {
                    // TODO:: get auctionId and blindedBid from the public statement
                    auctionId: auctioneer.bidLogs[i].auctionId,
                    blindedBid: auctioneer.bidLogs[i].blindedBid,
                    amount: decrypted[0],
                    random: decrypted[1]
                }
            )

        }

        const sortedBids = auctioneer.openings.sort(
            (a,b) =>{
                if(a.amount < b.amount) {
                    return 1;
                } else if (a.amount > b.amount){
                    return -1;
                } else {
                    return 0;
                }
            }
        );

        // console.log("Sorted bids: \n", JSON.stringify(sortedBids, null, 4));

        const winningBidData = sortedBids[0];
        const winningBidAmount = winningBidData.amount;
        const winningBidRandom = winningBidData.random;
        const auctionId = winningBidData.auctionId;


        // TODO:: the test is only for one auctionId
        // some structure should be added to group bids of each
        // auction Ids.

        const notWinningProofs = [];
        for(var i = 1; i < sortedBids.length; i++){

            // TODO:: connect the blockNumbers
            const bidBlockNumber = 0n;
            const winningBidBlockNumber = 0n;
            const currentBidAmount = sortedBids[i].amount;
            const currentBidRandom = sortedBids[i].random;


            const newProof = await prover.prove(
                "AuctionNotWinningBid",
                {
                    "auctionId": auctionId,
                    "bidBlockNumber": bidBlockNumber,
                    "winningBidBlockNumber": winningBidBlockNumber,
                    "bidAmount": currentBidAmount,
                    "bidRandom": currentBidRandom,
                    "winningBidAmount": winningBidAmount,
                    "winningBidRandom": winningBidRandom
                }
            );
            notWinningProofs.push(newProof);
        }

        const declareWinnerTx = await auctioneer.calls.zkAuction.declareWinner(
                                                        auctionId, 
                                                        winningBidAmount, 
                                                        winningBidRandom, 
                                                        notWinningProofs);

        winnerParams = await testHelpers.parseAuctionConcludedEvent(declareWinnerTx);

        console.log("AuctionConcluded event data: ", JSON.stringify(winnerParams, null, 4) );

        await expect(auctionData.auctionId).to.equal(winnerParams.auctionId);

        let winnerBid;
        if(bob.bids[0].amount > carl.bids[0].amount){
            winnerBid = bob.bids[0];
            winnerParams.winnerName = "Bob";
            winnerParams.bid = bob.bids[0];
            winnerParams.address = bob.wallet.address;

            auctionSummary.winner = "Bob";
            auctionSummary.winnerBidAmount = bob.bids[0].amount;
            auctionSummary.loser = "Carl";
            auctionSummary.loserBidAmount = carl.bids[0].amount;
        }
        else{
            winnerBid = carl.bids[0];
            winnerParams.winnerName = "Carl";
            winnerParams.bid = carl.bids[0];
            winnerParams.address = carl.wallet.address;


            auctionSummary.winner = "Carl";
            auctionSummary.winnerBidAmount = carl.bids[0].amount;
            auctionSummary.loser = "Bob";
            auctionSummary.loserBidAmount = bob.bids[0].amount;
        }

        await expect(winnerBid.blindedBid).to.equal(winnerParams.winningBlindedBid);
        await expect(winnerBid.bidAmount).to.equal(winnerParams.winningBidAmount);
        await expect(winnerBid.bidRandom).to.equal(winnerParams.winningBidRandom);


        merkleTree20.insertLeaves([winnerParams.commitments[0], winnerParams.commitments[1]]);
        merkleTree721.insertLeaves([winnerParams.commitments[2]]);

        console.log(`Winner (${winnerParams.winnerName}) checks commitments to see the commitments that is generated by her receivingKey.`);

    });

    it("Winner should be able to withdraw the ERC721 item.", async () => {

        console.log(`Winner (${winnerParams.winnerName}) withdraws ERC721 item.`);
        
        const winnerMerkleProof = merkleTree721.generateProof(winnerParams.commitments[2]);

        const winnerTx = await prover.prove(
            "OwnershipErc721",
            {
                message: 0n,
                values:
                [
                    auctionData.itemTokenId
                ],
                keysIn: [winnerParams.bid.receivingKey],
                keysOut: [{ publicKey: BigInt(winnerParams.address) }],
                treeNumbers: [ merkleTree721.lastTreeNumber ],
                merkleProofs: [winnerMerkleProof],
                erc721ContractAddress: contracts.erc721.address
            }
        );

        // console.log("Winner proof: ", JSON.stringify(winnerTx, null, 4));
        console.log("Winner knows the id of the token she won. [TODO::check it.]");
        // TX sent by a relayer
        await contracts.erc721CoinVault.withdraw([auctionData.itemTokenId], winnerParams.address, winnerTx);
        const res = await contracts.erc721.ownerOf(auctionData.itemTokenId);
        expect(res).to.equal(winnerParams.address);

    });


    it("Winner should be able to withdraw the excess ERC20 coin.", async () => {

        console.log(`Winner ${winnerParams.winnerName} withdraws excess Erc20 tokens.`);

        const dummyKey = jsUtils.newKeyPair();
        const winnerMerkleProof = merkleTree20.generateProof(winnerParams.commitments[1]);

        const winnerTx = await prover.prove(
            "JoinSplitErc20",
            {
                message: 0n,
                valuesIn: [winnerParams.bid.excessAmount, 0n],
                keysIn: [winnerParams.bid.excessKey, dummyKey],
                valuesOut: [winnerParams.bid.excessAmount, 0n],
                keysOut: [{ publicKey: BigInt(winnerParams.address) }, dummyKey],
                merkleProofs: [winnerMerkleProof, {root: 0n}],
                treeNumbers: [merkleTree20.lastTreeNumber, 0n],
                erc20ContractAddress: contracts.erc20.address,
            }
        );


        const oldBalance = (
            await contracts.erc20.balanceOf(winnerParams.address)
        ).toBigInt();


        await contracts.erc20CoinVault.withdraw([winnerParams.bid.excessAmount], winnerParams.address, winnerTx);

        const newBalance = (
            await contracts.erc20.balanceOf(winnerParams.address)
        ).toBigInt();
        await expect(oldBalance + winnerParams.bid.excessAmount).to.equal(newBalance);

    });


    it("Seller should be able to withdraw the erc20 price.", async () => {

        const dummyKey = jsUtils.newKeyPair();
        const sellerMerkleProof = merkleTree20.generateProof(winnerParams.commitments[0]);


        const sellerTx = await prover.prove(
            "JoinSplitErc20",
            {
                message: 0n,
                valuesIn: [winnerParams.bid.amount, 0n],
                keysIn: [auctionData.sellerKey, dummyKey],
                valuesOut: [winnerParams.bid.amount, 0n],
                keysOut: [{ publicKey: BigInt(alice.wallet.address) }, dummyKey],
                merkleProofs: [sellerMerkleProof, {root: 0n}],
                treeNumbers: [merkleTree20.lastTreeNumber, 0n],
                erc20ContractAddress: contracts.erc20.address,
            }
        );

        const oldBalance = (
            await contracts.erc20.balanceOf(alice.wallet.address)
        ).toBigInt();


        await contracts.erc20CoinVault.withdraw([winnerParams.bid.amount], alice.wallet.address, sellerTx);

        const newBalance = (
            await contracts.erc20.balanceOf(alice.wallet.address)
        ).toBigInt();
        await expect(oldBalance + winnerParams.bid.amount).to.equal(newBalance);

    });


    it("Losers should be able to withdraw the locked coins.", async () => {

        const dummyKey = jsUtils.newKeyPair();

        let loserBid;
        let loserCommitments;
        let loserAddress;
        let loserCoins;
        let loserName;
        let winnerName;
        let winnerBid;
        if(bob.bids[0].amount > carl.bids[0].amount){
            // carl is the loser
            loserName = "Carl";
            loserBid = carl.bids[0];
            loserCoins = [carl.coins[0], carl.coins[1]];
            loserAddress = carl.wallet.address;

            winnerName = "Bob";

        }
        else{
            // bob is the loser
            loserBid = bob.bids[0];
            loserCoins = [bob.coins[0], bob.coins[1]];
            loserAddress = bob.wallet.address;
            // console.log("Bob is the loser");
        }


        const oldBalance = (
            await contracts.erc20.balanceOf(loserAddress)
        ).toBigInt();

        const loserTx = await prover.prove(
            "JoinSplitErc20",
            {
                message: 0n,
                valuesIn: [loserCoins[0].value, loserCoins[1].value],
                keysIn: [loserCoins[0].key, loserCoins[1].key],
                valuesOut: [loserCoins[0].value + loserCoins[1].value, 0n],
                keysOut: [{ publicKey: BigInt(loserAddress) }, dummyKey],
                merkleProofs: [loserCoins[0].proof, loserCoins[1].proof],
                treeNumbers: [loserCoins[0].treeNumber, loserCoins[1].treeNumber],
                erc20ContractAddress: contracts.erc20.address,
            }
        );

        await contracts.erc20CoinVault.withdraw(
            [
                loserCoins[0].value + loserCoins[1].value
            ], 
            loserAddress, 
            loserTx
        );

        const newBalance = (
            await contracts.erc20.balanceOf(loserAddress)
        ).toBigInt();
        await expect(oldBalance + loserCoins[0].value + loserCoins[1].value).to.equal(newBalance);


        console.log("--------------------------------");
        console.log("--------------------------------");
        console.log("         Auction summary");
        console.log(JSON.stringify(auctionSummary, null, 4));
    });


});