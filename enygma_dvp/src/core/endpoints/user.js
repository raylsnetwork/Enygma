const prover = require("../prover");
const jsUtils = require("../utils");
const ethers = require("ethers");
const myWeb3 = require("../../web3");

async function getCommitmentFromTx(tx) {
  const rc = await tx.wait();
  // console.log("Gas used: " + rc["gasUsed"].toString());

  const commitmentEvents = rc.events.filter((ev) => ev.event === "Commitment");

  const commitments = commitmentEvents.map((element) => {
    return element.args.commitment;
  });

  console.log("new commitments: ", commitments);

  return commitments;
}

async function depositErc20(
  account,
  depositAmount,
  depositKey,
  erc20VaultContract,
  erc20Contract,
  merkleTree
) {
  let vaultAccount = erc20VaultContract.connect(account);
  let erc20Account = erc20Contract.connect(account);

  var tx2 = await erc20Account.approve(erc20VaultContract.address, depositAmount);
  var rc2 = await tx2.wait();
  console.log("...approves the transfer of Erc20 to ZkDvp");
  console.log("gasUsed: " + rc2["gasUsed"].toString());

  const tx = await vaultAccount.deposit(
    [depositAmount, depositKey.publicKey]
  );

  const cmt = await getCommitmentFromTx(tx);
  // merkleTree.insertLeaves([cmt]);

  return cmt;
}

async function withdrawErc20(
    account,
    amount,
    withdrawKey,
    erc20VaultContract,
    erc20Contract,
    merkleDepth,
    merkleProof,
    merkleRoot,
    treeNumber
) {
  const dummyKey = jsUtils.newKeyPair();
  const accountAddress = await account.getAddress();
  const erc20Address = erc20Contract.address;

  const proof = await generateErc20JoinSplitProof(
    0n,
    [amount, 0n],
    [withdrawKey, dummyKey],
    [amount, 0n],
    [{ publicKey: BigInt(accountAddress) }, dummyKey],
    merkleDepth,
    [merkleProof, 0n],
    [merkleRoot, 0n],
    [treeNumber, 0],
    erc20Address
  );

  const vaultAccount = erc20VaultContract.connect(account);

  const tx = await vaultAccount.withdraw(
    [amount],
    accountAddress,
    proof
  );

  var rc = await tx.wait();

  return proof.statement[1 + proof.numberOfInputs * 3];
}

async function depositErc721(
  account,
  nft_id,
  depositKey,
  erc721VaultContract,
  erc721Contract,
  merkleTree
) {
  let vaultAccount = erc721VaultContract.connect(account);
  let erc721Account = erc721Contract.connect(account);
  const erc721Address = erc721Contract.address;

  var tx2 = await erc721Account.approve(erc721VaultContract.address, nft_id);
  var rc2 = await tx2.wait();
  console.log("...approves the transfer of NFT to ZkDvp");
  console.log("gasUsed: " + rc2["gasUsed"].toString());

  // Deposit NFT into ZkDvp
  let tx = await vaultAccount.deposit(
    [nft_id,
    depositKey.publicKey]
  );

  let cmt = await getCommitmentFromTx(tx);

  return cmt;
}

async function depositErc1155(
    account,
    tokenId,
    amount,
    data,
    depositKey,
    erc1155VaultContract,
    erc1155Contract
) {
    let vaultAccount = erc1155VaultContract.connect(account);
    let erc1155Account = erc1155Contract.connect(account);
    const erc1155Address = erc1155Contract.address;

    var tx2 = await erc1155Account.setApprovalForAll(vaultAccount.address, true);
    var rc2 = await tx2.wait();
    console.log("...approved the transfer of Erc1155 to ZkDvp");

    try{
        // Deposit NFT into ZkDvp
        let tx = await vaultAccount.deposit(
          [amount, tokenId, depositKey.publicKey]
        );

        let cmt = await getCommitmentFromTx(tx);

        return cmt;
    }
    catch(err){
        const errorText = myWeb3.parseCustomError(err.error, "FungibilityMerkle");
        console.log("Error Text: ", errorText);
        // if(errorText == "ValueFungibilityInconsistency()"){
        //     console.log("RaylsErc1155 threw exception on double mint of non-fungible token. All good.")
        //     hasBeenReverted = true;

        // }else{
        //     console.log(err.toString());
        // }
    }

}

async function depositErc1155Batch(
  account,
  tokenIds,
  amounts,
  data,
  depositKeys,
  erc1155VaultContract,
  erc1155Contract
) {
  let vaultAccount = erc1155VaultContract.connect(account);
  let erc1155Account = erc1155Contract.connect(account);
  const erc1155Address = erc1155Contract.address;

  var tx2 = await erc1155Account.setApprovalForAll(erc1155VaultContract.address, true);
  var rc2 = await tx2.wait();
  console.log("...approves the BatchTransfer of Erc1155 to ZkDvp");
  // console.log("gasUsed: " + rc2["gasUsed"].toString());

  let publickeys = [];

  for (var i = 0; i < depositKeys.length; i++) {
    publickeys.push(depositKeys[i].publicKey);
  }

  let params = [];
  params = params.concat(tokenIds).concat(amounts).concat(publickeys);
  // Deposit NFT into ZkDvp

  console.log(tokenIds, amounts, publickeys);
  let tx = await vaultAccount.deposit(params);

  let cmt = await getCommitmentFromTx(tx);

  return cmt;
}


async function withdrawERC1155(
  account,
  amount,
  tokenId,
  withdrawKey,
  vaultContract,
  erc1155Contract,
  merkleDepth,
  merkleProof,
  merkleRoot,
  treeNumber,
  assetGroup_merkleRoot,
  assetGroup_merkleProof,
  isFungible
) {
  const dummyKey = jsUtils.newKeyPair();
  const accountAddress = await account.getAddress();
  const erc1155Address = erc1155Contract.address;


  let proof = await generateSingleErc1155Proof(
    0n,
    amount,
    withdrawKey,
    { publicKey: BigInt(accountAddress) },
    merkleDepth,
    merkleProof,
    merkleRoot,
    treeNumber,
    erc1155Address,
    tokenId,
    assetGroup_merkleRoot,
    assetGroup_merkleProof,
    isFungible
  );

  const vaultAccount = vaultContract.connect(account);

  var tx = await vaultAccount.withdraw(
    [
      amount,
      tokenId
    ],
    accountAddress,
    proof
  );
  var rc = await tx.wait();
  console.log("gasUsed: " + rc["gasUsed"].toString());
  return proof.commitment;
}


async function withdrawERC1155Batch(
  account,
  amounts,
  tokenIds,
  withdrawKeys,
  zkDvpContract,
  erc1155Contract,
  merkleDepth,
  merkleProofs,
  merkleRoots,
  treeNumbers
) {
  const dummyKey = jsUtils.newKeyPair();
  const accountAddress = await account.getAddress();
  const erc1155Address = erc1155Contract.address;

  const proof = await generateErc1155BatchProof(
    0n,
    amount,
    withdrawKey,
    { publicKey: BigInt(accountAddress) },
    merkleDepth,
    merkleProof,
    merkleRoot,
    treeNumber,
    erc1155Address,
    tokenId
  );

  const dvpAccount = zkDvpContract.connect(account);

  var tx = await dvpAccount.withdrawERC1155(
    amount,
    tokenId,
    erc1155Address,
    accountAddress,
    proof
  );
  var rc = await tx.wait();
  console.log("gasUsed: " + rc["gasUsed"].toString());
  return proof.commitment;
}

async function withdrawErc721(
  account,
  nft_id,
  withdrawKey,
  erc721VaultContract,
  erc721Contract,
  merkleDepth,
  merkleProof,
  merkleRoot,
  treeNumber
) {
  const dummyKey = jsUtils.newKeyPair();
  const accountAddress = await account.getAddress();
  const erc721Address = erc721Contract.address;

  const uid = jsUtils.erc721UniqueId(erc721Address, nft_id);

  const proof = await generateOwnershipProof(
    0n,
    uid,
    withdrawKey,
    { publicKey: BigInt(accountAddress) },
    merkleDepth,
    merkleProof,
    merkleRoot,
    treeNumber
  );

  const vaultAccount = erc721VaultContract.connect(account);

  var tx = await vaultAccount.withdraw(
    [nft_id],
    accountAddress,
    proof
  );
  var rc = await tx.wait();
  console.log("gasUsed: " + rc["gasUsed"].toString());
  return proof.commitment;
}

async function checkOwnership(account, nft_id, erc721Contract) {
  try {
    const erc721Account = erc721Contract.connect(account);

    const tokenOwner = await erc721Account.ownerOf(nft_id);
    const accountAddress = await account.getAddress();
    console.log(
      "nft " + nft_id + " belongs to " + tokenOwner + " " + accountAddress
    );

    return tokenOwner.toString() == accountAddress.toString();
  } catch (err) {
    console.log("error in checkOwnership");
    console.log(err);
    return false;
  }
}

async function mixErc20(
  account,
  inAmounts,
  inKeys,
  outAmounts,
  outKeys,
  zkDvpContract,
  erc20Contract,
  merkleTree
) {
  const accountAddress = await account.getAddress();
  const erc20Address = erc20Contract.address;

  var dummyKey;
  var proof;
  var commitments;
  if (inAmounts.length < 2 && inKeys.length < 2) {
    dummyKey = jsUtils.newKeyPair();

    proof = await generateErc20JoinSplitProof(
      0n,
      [inAmounts[0], 0n],
      [inKeys[0], dummyKey],
      [inAmounts[0], 0n],
      [outKeys[0], dummyKey],
      erc20Address,
      merkleTree
    );
  } else {
    proof = await generateErc20JoinSplitProof(
      0n,
      inAmounts,
      inKeys,
      outAmounts,
      outKeys,
      erc20Address,
      merkleTree
    );
  }


  return proof;
}

async function generateErc20JoinSplitProof(
  nftCommitment,
  inAmounts,
  inKeys,
  outAmounts,
  outKeys,
  treeDepth,
  proofs,
  roots,
  treeNumbers,
  erc20ContractAddress
) {
  const params = await prover.Erc20Proof(
    nftCommitment,
    inAmounts,
    inKeys,
    outAmounts,
    outKeys,
    treeDepth,
    proofs,
    roots,
    treeNumbers,
    erc20ContractAddress
  );

  return params;
}

async function generateOwnershipProof(
  paymentCommitment,
  uid,
  inKey,
  outKey,
  merkleDepth,
  merkleProof,
  merkleRoot,
  treeNumber
) {
  const params = await prover.Erc721Proof(
    paymentCommitment,
    [uid],
    [inKey],
    [uid],
    [outKey],
    merkleDepth,
    merkleProof,
    merkleRoot,
    treeNumber
  );

  return params;
}

async function generateSingleErc1155Proof(
  message,
  amountOrOne,
  inKey,
  outKey,
  merkleDepth,
  merkleProof,
  merkleRoot,
  treeNumber,
  erc1155ContractAddress,
  erc1155TokenId,
  assetGroup_treeNumber,
  assetGroup_merkleProof,
  isFungible
) {

  let params;

  if(isFungible){

      params = await prover.Erc1155FungibleProof(
        message,
        [amountOrOne],
        [inKey],
        [amountOrOne],
        [outKey],
        merkleDepth,
        [merkleProof],
        [merkleRoot],
        [treeNumber],
        erc1155ContractAddress,
        erc1155TokenId,
        assetGroup_treeNumber,
        assetGroup_merkleProof
      );
  }
  else{
      params = await prover.Erc1155NonFungibleProof(
        message,
        [amountOrOne],
        [inKey],
        [amountOrOne],
        [outKey],
        merkleDepth,
        merkleProof,
        merkleRoot,
        treeNumber,
        erc1155ContractAddress,
        erc1155TokenId,
        [assetGroup_treeNumber],
        [assetGroup_merkleProof]
      );

  }

  // console.log("proof: ", JSON.stringify(params,null , 4));
  return params;
}

async function generateErc1155JoinSplitProof(
  message,
  inAmounts,
  inKeys,
  outAmounts,
  outKeys,
  merkleDepth,
  merkleProofs,
  merkleRoots,
  treeNumbers,
  erc1155ContractAddress,
  erc1155TokenId,
  fungibilityMerkleTreeNumber,
  fungibilityMerkleProof
) {
  const params = await prover.Erc1155FungibleProof(
    message,
    inAmounts,
    inKeys,
    outAmounts,
    outKeys,
    merkleDepth,
    merkleProofs,
    merkleRoots,
    treeNumbers,
    erc1155ContractAddress,
    erc1155TokenId,
    fungibilityMerkleTreeNumber,
    fungibilityMerkleProof
  );

  return params;
}

async function generateErc1155BatchProof(
  message,
  amounts,
  inKeys,
  outKeys,
  merkleDepth,
  merkleProofs,
  merkleRoots,
  treeNumbers,
  erc1155ContractAddress,
  erc1155TokenIds,
  assetGroup_treeNumbers,
  assetGroup_merkleProofs
) {


  const params = await prover.Erc1155BatchProof(
      message,
      amounts,
      inKeys,
      outKeys,
      merkleDepth,
      merkleProofs,
      merkleRoots,
      treeNumbers,
      erc1155ContractAddress,
      erc1155TokenIds,
      assetGroup_treeNumbers,
      assetGroup_merkleProofs
  );

  return params;
}

/// Enygma
async function withdrawEnygma(
  publicKey,
  amount,
  withdrawKey,
  enygmaAddress,
  commitmentProof,
  merkleTree
) {
  const dummyKey = jsUtils.newKeyPair();

  const proof = await generateErc20JoinSplitProof(
    0n,
    [amount, 0n],
    [
      {
        publicKey: BigInt(withdrawKey.publicKey),
        privateKey: BigInt(withdrawKey.privateKey),
      },
      dummyKey,
    ],
    [amount, 0n],
    [{ publicKey: BigInt(publicKey) }, dummyKey],
    merkleTree.depth,
    [commitmentProof, 0n],
    [merkleTree.tree[merkleTree.depth][0], 0n],
    [BigInt(merkleTree.treeNumber), 0n],
    enygmaAddress
  );

  return proof;
}

///

module.exports = {
  getCommitmentFromTx,
  depositErc20,
  withdrawErc20,
  depositErc721,
  withdrawErc721,
  depositErc1155,
  depositErc1155Batch,
  withdrawERC1155,
  mixErc20,
  generateErc20JoinSplitProof,
  generateSingleErc1155Proof,
  generateErc1155JoinSplitProof, // fungible with size 10
  generateErc1155BatchProof,
  generateOwnershipProof,
  checkOwnership,
  withdrawEnygma,
};
