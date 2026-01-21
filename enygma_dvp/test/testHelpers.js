const crypto = require("crypto");
const { poseidon } = require("circomlibjs");
const poseidonGenContract = require("circomlibjs/src/poseidon_gencontract");
const prover = require("../src/core/prover");
const utils = require("../src/core/utils");
const MerkleTree = require("../src/core/merkle");
const { getVerificationKeys } = require("../src/core/dvpSnarks");
const { babyjub: babyJub } = require("circomlibjs");

const SNARK_SCALAR_FIELD = BigInt(
    "21888242871839275222246405745257275088548364400416034343698204186575808495617",
);

const MAX_NUMBER_OF_MERKLE_TREES = 10;

const dvpConf = require("../zkdvp.config.json");

const TREE_DEPTH = dvpConf["circom"]["meta-parameters"]["tree-depth"];

const TREE_ID_ERC721 = 0;
const TREE_ID_ERC20 = 1;
const TREE_ID_ERC1155 = 2;
const TREE_ID_ENYGMA = 3;


async function deployForTest(userCount) {

            console.log(`Deploying for test with userCount = ${userCount}`);

            let merkleTrees = {};
            let users = [];
            let auditors = [];
            let contracts = [];


            const vkeys = getVerificationKeys(dvpConf.circom.circuits);
            let wallets = await ethers.getSigners();

            let owner = wallets[0];
            for(var i = 0; i < userCount; i++){
                    users.push({"wallet": wallets[i + 1]});
            }

            auditors.push({"wallet": wallets[i], "keyPair": utils.babyKeyPair(babyJub), "auditGroupId":0, "offchainId":0});        
    
            const erc721Factory = await ethers.getContractFactory("RaylsERC721");
            const erc721Contract = await erc721Factory.deploy("Test NFT", "TNFT");
            contracts["erc721"] = erc721Contract;


            const erc20Factory = await ethers.getContractFactory("RaylsERC20");
            const erc20Contract = await erc20Factory.deploy("Test Token", "TTT");
            contracts["erc20"] = erc20Contract;

            const erc1155Factory = await ethers.getContractFactory("RaylsERC1155");
            const erc1155Contract = await erc1155Factory.deploy("RaylsERC1155");
            contracts["erc1155"] = erc1155Contract;

            await overwriteArtifact("PoseidonT3", poseidonGenContract.createCode(2));

            const PoseidonT3Factory = await hre.ethers.getContractFactory(
                "PoseidonT3",
                owner
            );
            const poseidonT3Contract = await PoseidonT3Factory.deploy();
            const posTxn = await poseidonT3Contract.deployTransaction.wait();

            console.log("poseidonT3 has been deployed to " + poseidonT3Contract.address);

            /////////////////////////////////////////////////
            console.log("Deploying GenericGroth16Verifier smart contract...");

            const g16VerifierFactory = await hre.ethers.getContractFactory(
                "GenericGroth16Verifier",
                owner
            );
            const g16VerifierContract = await g16VerifierFactory.deploy();
            const g16VerifierTxn = await g16VerifierContract.deployTransaction.wait();

            console.log(
                "GenericGroth16Verifier has been deployed to " + g16VerifierContract.address
            );

            /////////////////////////////////////////////////
            console.log("Deploying Verifier smart contract...");

            const verifierFactory = await hre.ethers.getContractFactory(
                "Verifier",
                owner
            );
            const verifierContract = await verifierFactory.deploy();
            const verifierTxn = await verifierContract.deployTransaction.wait();

            console.log("Verifier has been deployed to " + verifierContract.address);
            const verifierAddress = verifierContract.address;

            /////////////////////////////////////////////////
            console.log("Deploying PoseidonWrapper smart contract...");

            const poseidonWrapperFactory = await hre.ethers.getContractFactory(
                "PoseidonWrapper",
                {
                    libraries: {
                        PoseidonT3: poseidonT3Contract.address,
                    },
                },
                owner
            );
            const poseidonWrapperContract = await poseidonWrapperFactory.deploy();
            const posWrptxn = await poseidonWrapperContract.deployTransaction.wait();

            console.log(
                "poseidonWrapper has been deployed to " + poseidonWrapperContract.address
            );

            /////////////////////////////////////////////////
            console.log("Deploying ZkDvp smart contract...");

            const zkDvpFactory = await hre.ethers.getContractFactory("ZkDvp");
            const zkDvpContract = await zkDvpFactory.deploy(
                poseidonWrapperContract.address,
                g16VerifierContract.address
            );
            contracts["zkdvp"] = zkDvpContract;

        console.log("initializing ZkDvp smart contract...");
        const rawTx = await zkDvpContract.initializeDvp(verifierAddress);

        const tx = await rawTx.wait();
        // console.log(tx);

            /////////////////////////////////////////////////
            console.log("Deploying ZkAuction smart contract...");
            const zkAuctionFactory = await hre.ethers.getContractFactory("ZkAuction");
            const zkAuctionContract = await zkAuctionFactory.deploy(
                    zkDvpContract.address
            );
            contracts["zkAuction"] = zkAuctionContract;

        console.log("Registering ZkAuction smart contract...");
        await zkDvpContract.registerZkAuction(zkAuctionContract.address);

        // const tx = await rawTx.wait();


        // registering verificationKeys
        for(var i = 0;i< vkeys.length; i++){
            await zkDvpContract.registerNewVerificationKey(vkeys[i]);
        }

        const erc20CoinVaultFactory = await hre.ethers.getContractFactory("Erc20CoinVault");
        const erc20CoinVaultContract = await erc20CoinVaultFactory.deploy(
            zkDvpContract.address
        );
        const erc20CoinVaultTxn = await erc20CoinVaultContract.deployTransaction.wait();
        console.log(`Erc20CoinVault has been deployed to ` + erc20CoinVaultContract.address);

            contracts["erc20CoinVault"] = erc20CoinVaultContract;
        /////////////////////////////////////////////////
        const erc721CoinVaultFactory = await hre.ethers.getContractFactory("Erc721CoinVault");
        const erc721CoinVaultContract = await erc721CoinVaultFactory.deploy(
            zkDvpContract.address
        );
        const erc721CoinVaultTxn = await erc721CoinVaultContract.deployTransaction.wait();
        console.log(`Erc721CoinVault has been deployed to ` + erc721CoinVaultContract.address);
            contracts["erc721CoinVault"] = erc721CoinVaultContract;
        /////////////////////////////////////////////////
        const erc1155CoinVaultFactory = await hre.ethers.getContractFactory("Erc1155CoinVault");
        const erc1155CoinVaultContract = await erc1155CoinVaultFactory.deploy(
            zkDvpContract.address
        );
        const erc1155CoinVaultTxn = await erc1155CoinVaultContract.deployTransaction.wait();
        console.log(`Erc1155CoinVault has been deployed to ` + erc1155CoinVaultContract.address);
            contracts["erc1155CoinVault"] = erc1155CoinVaultContract;
        /////////////////////////////////////////////////

        const assetGroupFactory = await hre.ethers.getContractFactory("AssetGroup");
        
        // deploying two default assetGroups for fungibility and non-fungibility

        // Deploying FungiibleAssetGroup
        const fungibilityGroupContract = await assetGroupFactory.deploy(
            zkDvpContract.address
        );
        const fungibleGroupTxn = await fungibilityGroupContract.deployTransaction.wait();
        console.log(`fungibleAssetGroup has been deployed to ` + fungibilityGroupContract.address);
            contracts["fungibleAssetGroup"] = fungibilityGroupContract;
        // Deploying NonFungibleAssetGroup
        const nonFungibleGroupContract = await assetGroupFactory.deploy(
            zkDvpContract.address
        );
        const nonfungibleGroupTxn = await nonFungibleGroupContract.deployTransaction.wait();
        console.log(`fungibleAssetGroup has been deployed to ` + nonFungibleGroupContract.address);
            contracts["nonFungibleAssetGroup"] = nonFungibleGroupContract;
        /////////////////////////////////////////////////
        // initializing the Vaults
        console.log("Registering CoinVaults to ZkDvp smart contract. ");

        console.log("tree-depth: ", TREE_DEPTH);
            const tx1 = await zkDvpContract.registerVault(
                erc20CoinVaultContract.address,
                erc20Contract.address,
                1,
                TREE_DEPTH
            );
            console.log(`... Registered Erc20CoinVault`);

            const tx2 = await zkDvpContract.registerVault(
                erc721CoinVaultContract.address,
                erc721Contract.address,
                1,
                TREE_DEPTH
            );

            console.log(`... Registered Erc721CoinVault`);

            const tx3 = await zkDvpContract.registerVault(
                erc1155CoinVaultContract.address,
                erc1155Contract.address,
                2,
                TREE_DEPTH
            );

            console.log(`... Registered Erc1155CoinVault`);

        console.log("Registering AssetGroups to ZkDvp smart contract. ");

            const tx4 = await zkDvpContract.registerAssetGroup(
                fungibilityGroupContract.address,
                "Fungibles",
                true,
                TREE_DEPTH
            );
            console.log(`... Registered FungibileAssetGroup`);


            const tx5 = await zkDvpContract.registerAssetGroup(
                nonFungibleGroupContract.address,
                "NonFungibles",
                false,
                TREE_DEPTH
            );

            console.log(`... Registered NonFungibleAssetGroup`);

            console.log("Registering ZkAuction")

            console.log('Creating local Merkle Trees.')
            merkleTrees["ERC20"] = {"tree": new MerkleTree(TREE_DEPTH)};
            merkleTrees["ERC721"] = {"tree": new MerkleTree(TREE_DEPTH)};
            merkleTrees["ERC1155"] = {"tree": new MerkleTree(TREE_DEPTH)};

            console.log('Creating local Merkle Trees for default assetGroups.');

            merkleTrees["fungibleGroup"] = {"tree": new MerkleTree(TREE_DEPTH)};
            merkleTrees["nonFungibleGroup"] = {"tree": new MerkleTree(TREE_DEPTH)};

            console.log("Registering Erc20 vaultId in Fungibles assetGroup");


            await zkDvpContract.addVaultToGroup(
                0,
                0
            );

            console.log("Registering Erc721 vaultId in NonFungibles assetGroup");

            await zkDvpContract.addVaultToGroup(
                1,
                1
            );

            console.log("Registering Enygma ERC20 vaultId in Fungibles assetGroup");

            await zkDvpContract.addVaultToGroup(
                3,
                0
            );


        console.log("Registering Fungible-Fungible groupPair to valid exchange groupPairs. ");

        await zkDvpContract.registerExchangeGroupPair(0, 0);

        console.log("Registering Fungible-nonFungible groupPair to valid swap groupPairs. ");
        await zkDvpContract.registerSwapGroupPair(0, 1);

        console.log("Registering Auditors");
        await zkDvpContract.registerAuditor(
            auditors[0].offchainId, 
            auditors[0].auditGroupId, 
            auditors[0].keyPair.publicKey
        );

        console.log("Registered Auditor PrivateKey: ", auditors[0].keyPair.privateKey);
        console.log("Registered Auditor PublicKey: ", auditors[0].keyPair.publicKey);

        console.log("TestHelpers: Done ZkDvp initialization")
        return [owner, users, contracts, merkleTrees, auditors];
}
//////////////////////////////////////////////////////////////
//////////////////////////////////////////////////////////////
//////////////////////////////////////////////////////////////
// Helper functins to parse the events
//////////////////////////////////////////////////////////////
//////////////////////////////////////////////////////////////
//////////////////////////////////////////////////////////////
async function parseTokenAddedEvent(tx) {
        const rc = await tx.wait();
        const event = rc.events.find((ev) => ev.event === "TokenAddedToGroup");

        const eventData = {};
        eventData.groupId = event.args.groupId.toBigInt();
        eventData.vaultId = event.args.vaultId.toBigInt();
        eventData.tokenUniqueId = event.args.tokenUniqueId.toBigInt();

        return eventData;
}

async function getCommitmentFromTx(tx) {
    const rc = await tx.wait();
    const event = rc.events.find((ev) => ev.event === "Commitment");
    const commitment = event.args.commitment;

    return commitment.toBigInt();
}

async function parseAuctionPublicOpeningEvent(tx){
    const rc = await tx.wait();
    const event = rc.events.find((ev) => ev.event === "AuctionBidOpenedPublicly");
    const auctionId = event.args.auctionId.toBigInt();
    const blindedBid = event.args.blindedBid.toBigInt();
    const bidAmount = event.args.bidAmount.toBigInt();
    const bidRandom = event.args.bidRandom.toBigInt();
    return {
                        "auctionId": auctionId, 
                        "blindedBid":blindedBid, 
                        "bidAmount": bidAmount, 
                        "bidRandom": bidRandom
                };

}

async function parseAuctionInitEvent(tx){
        const rc = await tx.wait();
        const event = rc.events.find((ev) => ev.event === "AuctionInitialized");
        const auctionId = event.args.auctionId.toBigInt();
        const vaultId = event.args.vaultId.toBigInt();
        const bidVaultId = event.args.bidVaultId.toBigInt();
        const itemUniqueId = event.args.itemUniqueId.toBigInt();
        const assetAddress = event.args.assetAddress;
        const sellerFundCoinPublicKey = event.args.sellerFundCoinPublicKey.toBigInt();
        return {
                        "auctionId": auctionId, 
                        "vaultId":vaultId, 
                        "bidVaultId":bidVaultId, 
                        "itemUniqueId": itemUniqueId, 
                        "sellerFundCoinPublicKey": sellerFundCoinPublicKey
                };

}

async function parseVerifyOwnershipReceipt(tx){
                // emit OwnershipVerificationReceipt(
                //         challenge,
                //         _vaultId,
                //         tokenId,
                //         amountOrOne
                // );

        const rc = await tx.wait();
        const event = rc.events.find((ev) => ev.event === "OwnershipVerificationReceipt");
        const challenge = event.args.challenge.toBigInt();
        const vaultId = event.args.vaultId.toBigInt();
        const tokenId = event.args.tokenId.toBigInt();
        const amount = event.args.amount.toBigInt();
        return {
                        "challenge": challenge, 
                        "vaultId":vaultId, 
                        "tokenId":tokenId, 
                        "amount": amount 
                };
}


async function parseLegitBrokerReceipt(tx){
                // emit LegitBrokerReceipt(
                //        beacon,
                //        blindedPublicKey
                // );

        const rc = await tx.wait();
        const event = rc.events.find((ev) => ev.event === "LegitBrokerReceipt");
        const beacon = event.args.beacon.toBigInt();
        const blindedBrokerPublicKey = event.args.blindedBrokerPublicKey.toBigInt();
        return {
                        "beacon": beacon, 
                        "blindedBrokerPublicKey":blindedBrokerPublicKey
                };
}

async function parseAuctionConcludedEvent(tx){
    const rc = await tx.wait();
    const event = rc.events.find((ev) => ev.event === "AuctionConcluded");
    const auctionId = event.args.auctionId.toBigInt();
    const winningBlindedBid = event.args.winningBlindedBid.toBigInt();
    const winningBlockNumber = event.args.winningBlockNumber.toBigInt();
    const winningBid = event.args.winningBid.toBigInt();
    const winningRandom = event.args.winningRandom.toBigInt();
    const commitments = event.args.outCommitments;
    return {
                        "auctionId": auctionId, 
                        "winningBlindedBid":winningBlindedBid, 
                        "winningBlockNumber": winningBlockNumber, 
                        "winningBid": winningBid, 
                        "winningRandom": winningRandom,
                        "commitments": commitments
                    };

}
//////////////////////////////////////////////////////////////
//////////////////////////////////////////////////////////////
//////////////////////////////////////////////////////////////
//////////////////////////////////////////////////////////////

async function parseBondRegisteredEvent(tx){
    const rc = await tx.wait();
    const event = rc.events.find((ev) => ev.event === "NewTokenRegistered");
    const onchainId = event.args.onchainId.toBigInt();
    const offchainId = event.args.offchainId.toBigInt();
    const maxSupply = event.args.maxTotalSupply.toBigInt();
    return {
                        "onchainId": onchainId, 
                        "offchainId":offchainId, 
                        "maxSupply": maxSupply
                };

}


async function parseTokenAddedEvent(tx) {
    const rc = await tx.wait();
    const event = rc.events.find((ev) => ev.event === "TokenAddedToGroup");
    
    const eventData = {};
    eventData.groupId = event.args.groupId.toBigInt();
    eventData.vaultId = event.args.vaultId.toBigInt();
    eventData.tokenUniqueId = event.args.tokenUniqueId.toBigInt();

    return eventData;
}


async function getCommitmentFromTxIndirect(tx, vaultAddresses) {
        const rc = await tx.wait();
        const commitments = [];
        const nullifiers = [];
            // console.log(rc.events);
            // console.log(vaultAddresses);
        for(var j = 0; j < vaultAddresses.length; j++){
            // const commitmentEvent = ethers.utils.id("Commitment(uint256 indexed, uint256 indexed)");
            // console.log(commitmentEvent);
            events = rc.events.filter((ev) => ev.address === vaultAddresses[j]);
            // console.log(JSON.stringify(events, null, 4));
            for(var i = 0; i< events.length;i++){
                
                // TODO:: fix it. insecure
                if(events[i].topics.length == 3){

                    commitments.push(BigInt(events[i].topics[2]));
                }
                else{
                    nullifiers.push(BigInt(events[i].topics[3]));

                }
            }
        }

        // console.log(commitments);
        return [commitments, nullifiers];
}

module.exports = {
    deployForTest,
    parseTokenAddedEvent,
    parseAuctionInitEvent,
    parseAuctionConcludedEvent,
    parseAuctionPublicOpeningEvent,
    getCommitmentFromTx,
    parseBondRegisteredEvent,
    getCommitmentFromTxIndirect,
    parseVerifyOwnershipReceipt,
    parseLegitBrokerReceipt
};
