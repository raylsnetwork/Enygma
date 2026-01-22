const hre = require("hardhat");
const { getVerificationKeys } = require("../src/core/dvpSnarks");
const dvpConf = require("../enygmadvp.config.json");
const { EnygmaAddress } = require("./enygma");

async function initializeDvp() {
  const TREE_DEPTH = dvpConf["circom"]["meta-parameters"]["tree-depth"];

  console.log(`Merkle Tree depth: ${TREE_DEPTH}.`);

  const receipts = require("../build/receipts.json");

  const vkeys = getVerificationKeys(dvpConf.circom.circuits);

  console.log("Verification keys: ", vkeys);

  const verifierAddress = receipts["Verifier"]["contractAddress"];

  console.log(`Verifier Address: ${verifierAddress}`);

  const enygmaDvpContract = await hre.ethers.getContractAt(
    "EnygmaDvp",
    receipts["EnygmaDvp"]["contractAddress"],
  );

  console.log("initializing EnygmaDvp smart contract...");
  const rawTx = await enygmaDvpContract.initializeDvp(verifierAddress);

  const tx = await rawTx.wait();
  console.log(tx);

  for (var i = 0; i < vkeys.length; i++) {
    console.log(`registering VerificationKey no ${i}`);
    const txVer = await enygmaDvpContract.registerNewVerificationKey(vkeys[i]);
    await txVer.wait();
  }

  // Register PrivateMintVerifier
  console.log("Registering PrivateMintVerifier...");
  const privateMintVerifierAddress =
    receipts["PrivateMintVerifier"]["contractAddress"];
  console.log(`PrivateMintVerifier Address: ${privateMintVerifierAddress}`);

  const txPrivateMintVerifier =
    await enygmaDvpContract.registerPrivateMintVerifier(
      privateMintVerifierAddress,
    );
  await txPrivateMintVerifier.wait();
  console.log("... Registered PrivateMintVerifier");

  console.log("Registering CoinVaults to EnygmaDvp smart contract address. ");
  const tx1 = await enygmaDvpContract.registerVault(
    receipts["Erc20CoinVault"]["contractAddress"],
    receipts["ERC20"]["contractAddress"],
    1,
    TREE_DEPTH,
  );
  await tx1.wait();
  console.log(`... Registered Erc20CoinVault`);

  const tx2 = await enygmaDvpContract.registerVault(
    receipts["Erc721CoinVault"]["contractAddress"],
    receipts["ERC721"]["contractAddress"],
    1,
    TREE_DEPTH,
  );
  await tx2.wait();

  console.log(`... Registered Erc721CoinVault`);

  const tx3 = await enygmaDvpContract.registerVault(
    receipts["Erc1155CoinVault"]["contractAddress"],
    receipts["ERC1155"]["contractAddress"],
    2,
    TREE_DEPTH,
  );
  await tx3.wait();

  console.log(`... Registered Erc1155CoinVault`);

  const tx4 = await enygmaDvpContract.registerVault(
    receipts["EnygmaErc20CoinVault"]["contractAddress"],
    receipts["ERC20"]["contractAddress"],
    1,
    TREE_DEPTH,
  );
  await tx4.wait();
  console.log(`... Registered EnygmaErc20CoinVault`);

  console.log("Registering AssetGroups to EnygmaDvp smart contract. ");

  const tx5 = await enygmaDvpContract.registerAssetGroup(
    receipts["FungibleAssetGroup"]["contractAddress"],
    "Fungibles",
    true,
    TREE_DEPTH,
  );
  await tx5.wait();
  console.log(`... Registered FungibileAssetGroup`);

  const tx6 = await enygmaDvpContract.registerAssetGroup(
    receipts["NonFungibleAssetGroup"]["contractAddress"],
    "NonFungibles",
    false,
    TREE_DEPTH,
  );
  await tx6.wait();

  console.log(`... Registered NonFungibileAssetGroup`);

  console.log(
    "Registering Fungible-Fungible groupPair to valid exchange groupPairs. ",
  );

  const tx10 = await enygmaDvpContract.registerExchangeGroupPair(0, 0);
  await tx10.wait();

  console.log(
    "Registering Fungible-nonFungible groupPair to valid swap groupPairs. ",
  );
  const tx11 = await enygmaDvpContract.registerSwapGroupPair(0, 1);
  await tx11.wait();

  console.log("Registering Erc20 vaultId in Fungibles assetGroup");

  const tx12 = await enygmaDvpContract.addVaultToGroup(0, 0);
  await tx12.wait();

  console.log("Registering Erc721 vaultId in NonFungibles assetGroup");

  const tx13 = await enygmaDvpContract.addVaultToGroup(1, 1);
  await tx13.wait();

  console.log("Registering Enygma ERC20 vaultId in Fungibles assetGroup");

  const tx14 = await enygmaDvpContract.addVaultToGroup(3, 0);

  const enygmaVaultContract = await hre.ethers.getContractAt(
    "EnygmaErc20CoinVault",
    receipts["EnygmaErc20CoinVault"]["contractAddress"],
  );

  let enygmaAddress = EnygmaAddress();
  const addEnygmaTx = await enygmaVaultContract.addEnygma(
    enygmaAddress["enygmaAddress"],
  );

  const txEnygma = await addEnygmaTx.wait();
  console.log(txEnygma);
  console.log("enygma was added into EnygmaErc20CoinVault");

  console.log("EnygmaDvp has been initialized.");
}

initializeDvp();
