// Web3.js has been used for dev purposes
// such as deploying the smart contracts and 
// test scripts.

const ethers = require('ethers');
const web3 = require('web3');

const zkDvpJson = require("../artifacts/contracts/core/contracts/ZkDvp.sol/ZkDvp.json"); 
const rayls20Json = require("../artifacts/contracts/erc20/contracts/RaylsERC20.sol/RaylsERC20.json"); 
const rayls721Json = require("../artifacts/contracts/erc721/contracts/RaylsERC721.sol/RaylsERC721.json"); 
const rayls1155Json = require("../artifacts/contracts/erc1155/contracts/RaylsERC1155.sol/RaylsERC1155.json"); 

const erc20CoinVaultJson= require("../artifacts/contracts/core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json"); 

const circomLibJs = require('circomlibjs');

function errorToHash(errorString){
    return 
}

const customErrors = {
    "RaylsERC1155":[
        "SubTokenTransferNotAllowed()",
        "ZeroIdNotAllowed()",
        "ZeroValueMintNotAllowed()",
        "IdReservedForMetaToken()",
        "NotImplemented()",
        "ValueFungibilityInconsistency()",
        "InvalidMintData()",
        "IdsValuesMismatch()",
        "NotEnoughSubTokens()",
        "MaxSupplyExceeded()",
        "TokenAlreadyRegistered()",
        "InvalidMaxSupply()",
        "IdsValuesLengthMismatch()"
    ],
    "ZkDvp":[
     "AuctionIdExists()",
     "AuctionIdMismatch()",
     "BlindedBidMismatch()",
     "WinningBidOpeningMismatch()",
     "NotWinningBidsCountMismath()",
     "BidStateMismatch()",
     "RottenChallenge()",
     "InvalidOpening()",
     "InvalidChallenge()",
     "InvalidErc721Transfer()",
     "InvalidErc20Transfer()",
     "InvalidErc1155Transfer()",
     "InvalidErc1155BatchTransfer()",
     "JoinSplitWithSameCommitments()",
     "InvalidMerkleRoot()",
     "InvalidNullifier()",
     "InvalidNumberOfInputs()",
     "NotImplemented()",
     "GroupMembershipMismatch()"
    ],
    "FungibilityMerkle":[
     "InvalidFungibility()",
     "InvalidMerkleRoot()",
     "InvalidNumberOfInputs()",
     "WrongNumberOfIdentifiers()",
     "NotImplemented",
     "VaultAddressMismatch"
    ],
    "AbstractCoinVault":[
        "RottenChallenge()",
        "InvalidOpening()",
        "InvalidErc721Transfer()",
        "InvalidErc20Transfer()",
        "InvalidErc1155Transfer()",
        "InvalidErc1155BatchTransfer()",
        "JoinSplitWithSameCommitments()",
        "InvalidMerkleRoot()",
        "InvalidNullifier()",
        "InvalidNumberOfInputs()",
        "WrongNumberOfIdentifiers()",
        "NotImplemented()",
        "FungibilityMismatch()"
    ]

}

var customErrorDecoder;

    // struct Metadata{
    //     TokenType tType;
    //     TokenState tState;
    //     TokenFungibility tFungibility;
    //     string name;
    //     string symbol;
    //     uint256 offchainId;
    //     uint256[] subTokenIds;
    //     uint256[] subTokenValues;
    //     uint256 totalSupply;
    //     uint256 maxTotalSupply;
    //     uint256 decimals;
    //     bytes data; // reserved for compatibality with Erc1155
    //     uint256[] attrs; // reserved for more complex instruments
    // }

async function getWallet(privateKey){
    var customWsProvider = new ethers.providers.WebSocketProvider("http://localhost:8545");
    const wallet = new ethers.Wallet(privateKey, customWsProvider);
    return wallet;
} 

async function getZkDvpFactory(wallet){
    return await new ethers.ContractFactory(zkDvpJson["abi"], zkDvpJson["bytecode"], wallet);
}


async function getErc20CoinVaultFactory(wallet){
    return await new ethers.ContractFactory(erc20CoinVaultJson["abi"], erc20CoinVaultJson["bytecode"], wallet);
}

async function getRayls20Factory(wallet){
    return await new ethers.ContractFactory(rayls20Json["abi"], rayls20Json["bytecode"], wallet);
}

async function getRayls721Factory(wallet){
    return await new ethers.ContractFactory(rayls721Json["abi"], rayls721Json["bytecode"], wallet);
}


async function getRayls1155Factory(wallet){
    return await new ethers.ContractFactory(rayls1155Json["abi"], rayls1155Json["bytecode"], wallet);
}


async function getPoseidonFactory(wallet){
    const poseidonContract = circomLibJs.poseidonContract;
    return await new ethers.ContractFactory(poseidonContract.generateABI(2),poseidonContract.createCode(2), wallet);
}

function parseCustomError(e, contractFileName){
    try{
        if (e?.innerError?.errorSignature) {
            return e.innerError;
        }
        if (!e.innerError) {
            if (e?.data?.data.startsWith('0x')) {
              e.innerError = new web3.Eip838ExecutionError(e.data);
            }
        }
        if (e?.innerError?.data?.startsWith('0x')) {
            return customErrorDecoder[contractFileName][e.data.data];
        }
    }
    catch(err){
          return "CustomErrorNotFound";

    }
    return "CustomErrorNotFound";
}


function loadCustomErrorsHash(){
    customErrorDecoder = {};
    customErrorDecoder["RaylsERC1155"] = {};
    for(var i = 0; i< customErrors["RaylsERC1155"].length;i++){
        const errString = customErrors["RaylsERC1155"][i];
        const errHash = web3.utils.sha3(errString).substring(0,10);
        customErrorDecoder["RaylsERC1155"][errHash] = errString
    }
}

loadCustomErrorsHash();

module.exports = {
    getWallet,
    getZkDvpFactory,
    getRayls20Factory,
    getRayls721Factory,
    getRayls1155Factory,
    getPoseidonFactory,
    getErc20CoinVaultFactory,
    parseCustomError
}