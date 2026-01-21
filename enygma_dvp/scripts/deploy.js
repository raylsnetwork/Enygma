const hre = require("hardhat");
const dvpConf = require("../zkdvp.config.json");
var path = require("path");
const jsWeb3 = require("../src/web3.js");
const jsUtils = require("../src/core/utils.js");
const poseidonGenContract = require("circomlibjs/src/poseidon_gencontract");

async function deploy() {
  [owner, alice, bob] = await ethers.getSigners();
  console.log("owner: ", owner.address);
  console.log("alice: ", alice.address);
  console.log("bob: ", bob.address);
  let receiptsData = {};

  /////////////////////////////////////////////////
  console.log("Deploying poseidonT3 smart contract...");

  await overwriteArtifact("PoseidonT3", poseidonGenContract.createCode(2));

  const PoseidonT3Fatory = await hre.ethers.getContractFactory(
    "PoseidonT3",
    owner
  );
  const poseidonT3Contract = await PoseidonT3Fatory.deploy();
  const posTxn = await poseidonT3Contract.deployTransaction.wait();

  console.log("poseidonT3 has been deployed to " + poseidonT3Contract.address);
  receiptsData["Poseidon"] = posTxn;

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
  receiptsData["Verifier"] = verifierTxn;

  /////////////////////////////////////////////////
  console.log("Deploying PrivateMintVerifier smart contract...");

  const privateMintVerifierFactory = await hre.ethers.getContractFactory(
    "PrivateMintVerifier",
    owner
  );
  const privateMintVerifierContract = await privateMintVerifierFactory.deploy();
  const privateMintVerifierTxn = await privateMintVerifierContract.deployTransaction.wait();

  console.log("PrivateMintVerifier has been deployed to " + privateMintVerifierContract.address);
  receiptsData["PrivateMintVerifier"] = privateMintVerifierTxn;

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
  receiptsData["PoseidonWrapper"] = posWrptxn;

  /////////////////////////////////////////////////
  console.log("Deploying ZkDvp smart contract...");

  const zkDvpFactory = await hre.ethers.getContractFactory("ZkDvp");
  const zkDvpContract = await zkDvpFactory.deploy(
    poseidonWrapperContract.address,
    g16VerifierContract.address
  );

  const dvpTxn = await zkDvpContract.deployTransaction.wait();
  console.log("zkDvp has been deployed to " + zkDvpContract.address);
  receiptsData["ZkDvp"] = dvpTxn;
  receiptsData["G16Verifier"] = g16VerifierTxn;

  /////////////////////////////////////////////////
  const erc20Factory = await jsWeb3.getRayls20Factory(owner);
  const erc20Contract = await erc20Factory.deploy("TestERC20", "Rayls20");
  const erc20Txn = await erc20Contract.deployTransaction.wait();
  console.log("RaylsERC20 has been deployed to " + erc20Contract.address);
  receiptsData["ERC20"] = erc20Txn;

  /////////////////////////////////////////////////
  const erc721Factory = await jsWeb3.getRayls721Factory(owner);
  const erc721Contract = await erc721Factory.deploy("TestERC721", "Rayls721");
  const erc721Txn = await erc721Contract.deployTransaction.wait();
  console.log("RaylsERC721 has been deployed to " + erc721Contract.address);
  receiptsData["ERC721"] = erc721Txn;

  /////////////////////////////////////////////////
  const erc1155Factory = await jsWeb3.getRayls1155Factory(owner);
  const erc1155Contract = await erc1155Factory.deploy("Rayls1155");

  const erc1155Txn = await erc1155Contract.deployTransaction.wait();
  console.log("RaylsERC1155 has been deployed to " + erc1155Contract.address);
  receiptsData["ERC1155"] = erc1155Txn;

  /////////////////////////////////////////////////
  const erc20CoinVaultFactory = await hre.ethers.getContractFactory("Erc20CoinVault");
  const erc20CoinVaultContract = await erc20CoinVaultFactory.deploy(
    zkDvpContract.address
  );
  const erc20CoinVaultTxn = await erc20CoinVaultContract.deployTransaction.wait();
  receiptsData["Erc20CoinVault"] = erc20CoinVaultTxn;
  console.log(`Erc20CoinVault has been deployed to ` + erc20CoinVaultContract.address);
  /////////////////////////////////////////////////
  const erc721CoinVaultFactory = await hre.ethers.getContractFactory("Erc721CoinVault");
  const erc721CoinVaultContract = await erc721CoinVaultFactory.deploy(
    zkDvpContract.address
  );
  const erc721CoinVaultTxn = await erc721CoinVaultContract.deployTransaction.wait();
  receiptsData["Erc721CoinVault"] = erc721CoinVaultTxn;
  console.log(`Erc721CoinVault has been deployed to ` + erc721CoinVaultContract.address);
  /////////////////////////////////////////////////
  const erc1155CoinVaultFactory = await hre.ethers.getContractFactory("Erc1155CoinVault");
  const erc1155CoinVaultContract = await erc1155CoinVaultFactory.deploy(
    zkDvpContract.address
  );
  const erc1155CoinVaultTxn = await erc1155CoinVaultContract.deployTransaction.wait();
  receiptsData["Erc1155CoinVault"] = erc1155CoinVaultTxn;
  console.log(`Erc1155CoinVault has been deployed to ` + erc1155CoinVaultContract.address);
  /////////////////////////////////////////////////
  const enygmaCoinVaultFactory = await hre.ethers.getContractFactory("EnygmaErc20CoinVault");
  const enygmaCoinVaultContract = await enygmaCoinVaultFactory.deploy(
    zkDvpContract.address
  );
  const enygmaCoinVaultTxn = await enygmaCoinVaultContract.deployTransaction.wait();
  receiptsData["EnygmaErc20CoinVault"] = enygmaCoinVaultTxn;
  console.log(`EnygmaErc20CoinVault has been deployed to ` + enygmaCoinVaultContract.address);
  /////////////////////////////////////////////////


    const assetGroupFactory = await hre.ethers.getContractFactory("AssetGroup");
    
    // deploying two default assetGroups for fungibility and non-fungibility

    // Deploying FungiibleAssetGroup
    const fungibleGroupContract = await assetGroupFactory.deploy(
      zkDvpContract.address
    );
    const fungibleGroupTxn = await fungibleGroupContract.deployTransaction.wait();
    console.log(`FungibleAssetGroup has been deployed to ` + fungibleGroupContract.address);
    receiptsData["FungibleAssetGroup"] = fungibleGroupTxn;
    // Deploying NonFungibleAssetGroup
    
    const nonFungibleGroupContract = await assetGroupFactory.deploy(
      zkDvpContract.address
    );
    const nonfungibleGroupTxn = await nonFungibleGroupContract.deployTransaction.wait();
    console.log(`NonFungibleAssetGroup has been deployed to ` + nonFungibleGroupContract.address);
    receiptsData["NonFungibleAssetGroup"] = nonfungibleGroupTxn;
    /////////////////////////////////////////////////


  jsUtils.writeToJson("./build/receipts.json", receiptsData);

  console.log("receipts has been save to ./build/receipts.json.");
}

deploy();
