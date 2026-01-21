const hre = require("hardhat");
const web3 = require("web3");

async function addAssetToGroup(admin, vaultId, uniqueIdParams, groupId, zkDvpContract) {
    const zkDvpAdmin = zkDvpContract.connect(admin);
    var tx = await zkDvpAdmin.addAssetToGroup(vaultId, uniqueIdParams, groupId);
    console.log(`Asset has been added to group ${groupId}`);

    return tx;
}

async function mintErc20(admin, account, depositAmount, erc20Contract) {
    const erc20Admin = erc20Contract.connect(admin);
    const accountAddress = await account.getAddress();
    var tx = await erc20Admin.mint(accountAddress, depositAmount);
    console.log("Minted ERC20 token for " + accountAddress);
    return tx;
}

async function mintErc721(admin, account, nft_id, erc721Contract) {
    const erc721Admin = erc721Contract.connect(admin);
    const accountAddress = await account.getAddress();
    var tx = await erc721Admin.mint(accountAddress, nft_id);
    console.log("Minted NFt for " + accountAddress);
    return tx;
}


async function mintErc1155(admin, account, token_id, amount, data, erc1155Contract) {
    // console.log("erc1155Address: "+ erc1155Contract.address);
    // console.log("tokenId: "+ BigInt(token_id));
    // console.log("amount: "+ amount);
    const erc1155Admin = erc1155Contract.connect(admin);
    const accountAddress = await account.getAddress();
    var tx =  await erc1155Admin.mint(accountAddress, token_id, amount, 0);
    console.log("Minted Erc1155 for " + accountAddress);

    return tx;
}


async function mintErc1155Batch(admin, account, tokenIds, amounts, data, erc1155Contract) {
    console.log("Admin.js: minting batch of Erc1155 ...");

    const erc1155Admin = erc1155Contract.connect(admin);
    const accountAddress = await account.getAddress();
    // const data = web3.eth.abi.encodeParameter('uint256[]', fungibilities);
    // console.log("mintBatch data: ", data);
    var tx =  await erc1155Admin.mintBatch(accountAddress, tokenIds, amounts, data);
    console.log("Minted Batch Erc1155 for " + accountAddress);

    return tx;
}

async function mintErc1155Fungible(admin, account, token_id, amount, erc1155Contract) {
    console.log("Admin.js: minting fungible Erc1155 ...");
    
    // data -> one fungible token
    const data = web3.eth.abi.encodeParameter('uint256[]', [0n]);
    // console.log("mintErc1155Fungible data = ", data);
    const erc1155Admin = erc1155Contract.connect(admin);
    const accountAddress = await account.getAddress();
    var tx =  await erc1155Admin.mint(accountAddress, token_id, amount, data);
    console.log(`Minted fungible Erc1155 with id=${token_id}, amount=${amount} for ${accountAddress}`);

    return tx;
}


async function mintErc1155NonFungible(admin, account, token_id, erc1155Contract) {
    console.log("Admin.js: minting non-fungible Erc1155 ...");
    const data = web3.eth.abi.encodeParameter('uint256[]', [1n]);
    const erc1155Admin = erc1155Contract.connect(admin);
    const accountAddress = await account.getAddress();
    var tx =  await erc1155Admin.mint(accountAddress, token_id, 1n, data);
    console.log(`Minted non-fungible Erc1155 with id=${token_id} for ${accountAddress}`);

    return tx;
}


module.exports = {
    addAssetToGroup,
    mintErc20,
    mintErc721,
    mintErc1155,
    mintErc1155Batch,
    mintErc1155Fungible,
    mintErc1155NonFungible
};
