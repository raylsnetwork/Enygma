/*
Low-level proof generation.
These functions are directly calling proof generation functions
  from snarkJs library
*/
const snarkjs = require("snarkjs");
const utils = require("./utils");
const axios = require("axios");

async function GnarkProver(circuitInput, zkeyPath) {
  if (zkeyPath.includes("OwnershipErc721")) {
    return Erc721Proof(circuitInput);
  } else if (zkeyPath.includes("JoinSplitErc20")) {
    return Erc20Proof(circuitInput, zkeyPath);
  } else if (zkeyPath.includes("OwnershipErc1155NonFungibleWithAuditor")) {
    return Erc1155NonFungibleWithAuditorProof(circuitInput);
  } else if (zkeyPath.includes("OwnershipErc1155NonFungible")) {
    return Erc1155NonFungibleProof(circuitInput);
  } else if (zkeyPath.includes("JoinSplitErc1155WithAuditor")) {
    return Erc1155FungibleAuditorProof(circuitInput);
  } else if (zkeyPath.includes("JoinSplitErc1155")) {
    return Erc1155FungibleProof(circuitInput);
  } else if (zkeyPath.includes("AuctionInit_Auditor")) {
    return AuctionInitAuditorProof(circuitInput);
  } else if (zkeyPath.includes("AuctionBid_Auditor")) {
    return AuctionBidAuditorProof(circuitInput);
  } else if (zkeyPath.includes("AuctionPrivateOpening")) {
    return AuctionPrivateOpeningProof(circuitInput);
  } else if (zkeyPath.includes("AuctionNotWinningBid")) {
    console.log("AuctionNotWinningBidProof");
    console.log(circuitInput);
    return AuctionNotWinningBidProof(circuitInput);
  }
  return;
}
async function postRequestGnarkCircuit(url, data) {
  try {
    const res = await axios.post(url, data, {
      headers: {
        "Content-Type": "application/json",
      },
    });
    console.log("Response:", res.data);
  } catch (err) {
    if (err.response) {
      console.error("Error:", err.response.status, err.response.data);
    } else {
      console.error("Request failed:", err.message);
    }
  }
}

async function Erc20Proof(inputs, zkeyPath) {
  if (zkeyPath === "./build/JoinSplitErc20_10_2.zkey") {
    console.log("inputs 20_2", inputs);
  }
  let stringURL =
    zkeyPath == "./build/JoinSplitErc20_10_2.zkey"
      ? "http://localhost:8081/proof/joinSplitERC20_10_2"
      : "http://localhost:8081/proof/joinSplitERC20";

  split1 = [];
  split2 = [];

  const chunks = splitPathElements(inputs);

  let proofGnark = postRequestGnarkCircuit(stringURL, {
    StMessage: inputs.st_message.toString(),
    StTreeNumber: inputs.st_treeNumbers.map((k) => k.toString()),
    StMerkleRoots: inputs.st_merkleRoots,
    StNullifiers: inputs.st_nullifiers,
    StCommitmentOut: inputs.st_commitmentsOut,
    WtPrivateKeysIn: inputs.wt_privateKeysIn,
    WtPublicKeysOut: inputs.wt_publicKeysOut,
    WtPathElements: chunks,
    WtPathIndices: inputs.wt_pathIndices.map((k) => k.toString()),
    WtValuesIn: inputs.wt_valuesIn,
    WtValuesOut: inputs.wt_valuesOut,
    WtErc20ContractAddress: BigInt(inputs.wt_erc20ContractAddress).toString(),
  });

  return {
    status: 200,
    message: "ok",
  };
}

async function Erc721Proof(inputs) {
  let proofGnark = postRequestGnarkCircuit(
    "http://localhost:8081/proof/ownershipERC721",
    {
      StMessage: inputs.st_message,
      StTreeNumbers: inputs.st_treeNumbers.map((k) => k.toString()),
      StMerkleRoots: inputs.st_merkleRoots.map((k) => k.toString()),
      StNullifiers: inputs.st_nullifiers,
      StCommitmentOut: inputs.st_commitmentsOut,
      WtPrivateKeysIn: inputs.wt_privateKeysIn,
      WtValues: inputs.wt_values,
      WtPathElements: [inputs.wt_pathElements],
      WtPathIndices: inputs.wt_pathIndices,
      WtPublicKeysOut: inputs.wt_publicKeysOut,
    }
  );

  return {
    status: 200,
    message: "ok",
  };
}
async function Erc1155FungibleProof(inputs) {
  split1 = [];
  split2 = [];
  for (let i = 0; i < 16; i++) {
    if (i < 8) {
      split1.push(inputs.wt_pathElements[i]);
    } else {
      split2.push(inputs.wt_pathElements[i]);
    }
  }

  let url = "http://localhost:8081/proof/erc155Fungible";

  let proofGnark = postRequestGnarkCircuit(url, {
    StMessage: inputs.st_message,
    StTreeNumbers: inputs.st_treeNumbers.map((k) => k.toString()),
    StMerkleRoots: inputs.st_merkleRoots,
    StCommitmentOut: inputs.st_commitmentsOut,
    StNullifiers: inputs.st_nullifiers,
    StAssetGroupMerkleRoot: inputs.st_assetGroup_merkleRoot.toString(),
    StAssetGroupTreeNumber: inputs.st_assetGroup_treeNumber.toString(),
    WtPrivateKeysIn: inputs.wt_privateKeysIn,
    WtValuesIn: inputs.wt_valuesIn.map((k) => k.toString()),
    WtPathElements: [split1, split2],
    WtPathIndices: inputs.wt_pathIndices.map((k) => k.toString()),
    WtErc1155ContractAddress: inputs.wt_erc1155ContractAddress,
    WtErc1155TokenId: inputs.wt_erc1155TokenId,
    WtPublicKeysOut: inputs.wt_publicKeysOut,
    WtValuesOut: inputs.wt_valuesOut.map((k) => k.toString()),
    WtAssetGroupPathElements: inputs.wt_assetGroup_pathElements.map((k) =>
      k.toString()
    ),
    WtAssetGroupPathIndices: inputs.wt_assetGroup_pathIndices,
  });

  return {
    status: 200,
    message: "ok",
  };
}

async function Erc1155FungibleAuditorProof(inputs) {
  split1 = [];
  split2 = [];
  for (let i = 0; i < 16; i++) {
    if (i < 8) {
      split1.push(inputs.wt_pathElements[i]);
    } else {
      split2.push(inputs.wt_pathElements[i]);
    }
  }

  let url = "http://localhost:8081/proof/erc1155FungibleAuditor";

  let proofGnark = postRequestGnarkCircuit(url, {
    StMessage: inputs.st_message,
    StTreeNumbers: inputs.st_treeNumbers.map((k) => k.toString()),
    StMerkleRoots: inputs.st_merkleRoots,
    StCommitmentOut: inputs.st_commitmentsOut,
    StNullifiers: inputs.st_nullifiers,
    StAssetGroupMerkleRoot: inputs.st_assetGroup_merkleRoot.toString(),
    StAssetGroupTreeNumber: inputs.st_assetGroup_treeNumber.toString(),
    WtPrivateKeysIn: inputs.wt_privateKeysIn,
    WtValuesIn: inputs.wt_valuesIn.map((k) => k.toString()),
    WtPathElements: [split1, split2],
    WtPathIndices: inputs.wt_pathIndices.map((k) => k.toString()),
    WtErc1155ContractAddress: inputs.wt_erc1155ContractAddress,
    WtErc1155TokenId: inputs.wt_erc1155TokenId,
    WtPublicKeysOut: inputs.wt_publicKeysOut,
    WtValuesOut: inputs.wt_valuesOut.map((k) => k.toString()),
    WtAssetGroupPathElements: inputs.wt_assetGroup_pathElements.map((k) =>
      k.toString()
    ),
    WtAssetGroupPathIndices: inputs.wt_assetGroup_pathIndices,
    StAuditorPublickey: inputs.st_auditor_publicKey,
    StAuditorAuthKey: inputs.st_auditor_authKey,
    StAuditorNonce: inputs.st_auditor_nonce,
    StAuditorEncryptedValues: inputs.st_auditor_encryptedValues,
    WtAuditorRandom: inputs.wt_auditor_random,
  });

  return {
    status: 200,
    message: "ok",
  };
}

// Generates proof for transfer of the ownership of an Erc1155 coin
// Erc1155NonFungibleTemplate with size 1
async function Erc1155NonFungibleProof(inputs) {
  let proofGnark = postRequestGnarkCircuit(
    "http://localhost:8081/proof/erc1155NonFungible",
    {
      StMessage: inputs.st_message.toString(),
      StTreeNumbers: inputs.st_treeNumbers.map((k) => k.toString()),
      StMerkleRoots: inputs.st_merkleRoots,
      StNullifiers: inputs.st_nullifiers,
      StCommitmentOut: inputs.st_commitmentsOut,
      StAssetGroupTreeNumber: inputs.st_assetGroup_treeNumbers.map((k) =>
        k.toString()
      ),
      StAssetGroupMerkleRoot: inputs.st_assetGroup_merkleRoots,
      WtPrivateKeysIn: inputs.wt_privateKeysIn,
      WtValues: inputs.wt_values,
      WtPathElements: [inputs.wt_pathElements],
      WtPathIndices: inputs.wt_pathIndices,
      WtErc1155TokenId: inputs.wt_erc1155TokenIds,
      WtPublicKeysOut: inputs.wt_publicKeysOut,
      WtErc1155ContractAddress: inputs.wt_erc1155ContractAddress,
      WtAssetGroupPathElements: inputs.wt_assetGroup_pathElements.map(
        (arrayOfBigInts) =>
          arrayOfBigInts.map((bigIntValue) => bigIntValue.toString())
      ),
      WtAssetGroupPathIndices: inputs.wt_assetGroup_pathIndices,
    }
  );

  return {
    status: 200,
    message: "ok",
  };
}

async function Erc1155NonFungibleWithAuditorProof(inputs) {
  let proofGnark = postRequestGnarkCircuit(
    "http://localhost:8081/proof/erc1155NonFungibleAuditor",
    {
      StMessage: inputs.st_message.toString(),
      StTreeNumbers: inputs.st_treeNumbers.map((k) => k.toString()),
      StMerkleRoots: inputs.st_merkleRoots,
      StNullifiers: inputs.st_nullifiers,
      StCommitmentOut: inputs.st_commitmentsOut,

      StAssetGroupTreeNumber: inputs.st_assetGroup_treeNumbers.map((k) =>
        k.toString()
      ),
      StAssetGroupMerkleRoot: inputs.st_assetGroup_merkleRoots,
      WtPrivateKeysIn: inputs.wt_privateKeysIn,
      WtValues: inputs.wt_values,
      WtPathElements: [inputs.wt_pathElements],
      WtPathIndices: inputs.wt_pathIndices,
      WtErc1155TokenIds: inputs.wt_erc1155TokenIds,
      WtPublicKeysOut: inputs.wt_publicKeysOut,
      WtErc1155ContractAddress: inputs.wt_erc1155ContractAddress,
      WtAssetGroupPathElements: inputs.wt_assetGroup_pathElements.map(
        (arrayOfBigInts) =>
          arrayOfBigInts.map((bigIntValue) => bigIntValue.toString())
      ),

      WtAssetGroupPathIndices: inputs.wt_assetGroup_pathIndices,
      StAuditorPublickey: inputs.st_auditor_publicKey,
      StAuditorAuthKey: inputs.st_auditor_authKey,
      StAuditorNonce: inputs.st_auditor_nonce,
      StAuditorEncryptedValues: inputs.st_auditor_encryptedValues,
      WtAuditorRandom: inputs.wt_auditor_random,
    }
  );

  return {
    status: 200,
    message: "ok",
  };
}

async function AuctionBidAuditorProof(inputs) {
  split1 = [];
  split2 = [];
  for (let i = 0; i < 16; i++) {
    if (i < 8) {
      split1.push(inputs.wt_pathElements[i]);
    } else {
      split2.push(inputs.wt_pathElements[i]);
    }
  }

  let proofGnark = postRequestGnarkCircuit(
    "http://localhost:8081/proof/auctionBidAuditor",
    {
      stBeacon: inputs.st_beacon.toString(),
      stAuctionId: inputs.st_auctionId.toString(),
      stBlindedBid: inputs.st_blindedBid.toString(),
      stVaultId: inputs.st_vaultId.toString(),
      stTreeNumbers: inputs.st_treeNumbers.map((k) => k.toString()),
      stMerkleRoots: inputs.st_merkleRoots.map((k) => k.toString()),
      stNullifiers: inputs.st_nullifiers.map((k) => k.toString()),
      stCommitmentsOuts: inputs.st_commitmentsOut.map((k) => k.toString()),
      stAssetGroupTreeNumber: inputs.st_assetGroup_treeNumber.toString(),
      stAssetGrupoMerkleRoot: inputs.st_assetGroup_merkleRoot.toString(),
      // Auctioneer fields
      stAuctioneerPublicKey: inputs.st_auctioneer_publicKey.map((k) =>
        k.toString()
      ),
      stAuctioneerAuthKey: inputs.st_auctioneer_authKey.map((k) =>
        k.toString()
      ),
      stAuctioneerNonce: inputs.st_auctioneer_nonce.toString(),
      stAuctioneerEncryptedValues: inputs.st_auctioneer_encryptedValues.map(
        (k) => k.toString()
      ),
      wtAuctioneerRandom: inputs.wt_auctioneer_random.toString(),
      // Auditor fields
      stAuditorPublicKey: inputs.st_auditor_publicKey.map((k) => k.toString()),
      stAuditorAuthKey: inputs.st_auditor_authKey.map((k) => k.toString()),
      stAuditorNonce: inputs.st_auditor_nonce.toString(),
      stAuditorEncryptedValues: inputs.st_auditor_encryptedValues.map((k) =>
        k.toString()
      ),
      wtAuditoRandom: inputs.wt_auditor_random.toString(),
      wtBidAmount: inputs.wt_bidAmount.toString(),
      wtBidRandom: inputs.wt_bidRandom.toString(),
      wtPrivateKeysIn: inputs.wt_privateKeysIn.map((k) => k.toString()),
      wtPathElements: [
        split1.map((k) => k.toString()),
        split2.map((k) => k.toString()),
      ],
      wtPathIndices: inputs.wt_pathIndices.map((k) => k.toString()),
      wtContractAddress: inputs.wt_contractAddress.toString(),
      wtPublicKeysOut: inputs.wt_publicKeysOut.map((k) => k.toString()),
      wtAssetGroupPathElements: inputs.wt_assetGroup_pathElements.map((k) =>
        k.toString()
      ),
      wtAssetGroupPathIndices: inputs.wt_assetGroup_pathIndices.toString(),
      wtIdParamsIn: inputs.wt_idParamsIn.map((k) => k.map((x) => x.toString())),
      wtIdParamsOut: inputs.wt_idParamsOut.map((k) =>
        k.map((x) => x.toString())
      ),
    }
  );

  return {
    status: 200,
    message: "ok",
  };
}

// Generating proof for AuctionBidErc20.circom

async function AuctionInitAuditorProof(inputs) {
  let proofGnark = postRequestGnarkCircuit(
    "http://localhost:8081/proof/auctionInitAuditor",
    {
      StBeacon: inputs.st_beacon.toString(),
      StAuctionId: inputs.st_auctionId.toString(),
      StVaultId: inputs.st_vaultId.toString(),
      StTreeNumber: inputs.st_treeNumber.toString(),
      StMerkleRoot: inputs.st_merkleRoot,
      StNullifier: inputs.st_nullifier,

      StAuditorPublicKey: inputs.st_auditor_publicKey.map((k) => k.toString()),
      StAuditorAuthKey: inputs.st_auditor_authKey.map((k) => k.toString()),
      StAuditorNonce: inputs.st_auditor_nonce,
      StAuditorEncryptedValues: inputs.st_auditor_encryptedValues.map((k) =>
        k.toString()
      ),
      WtAuditorRandom: inputs.wt_auditor_random,

      StAssetGroupTreeNumber: inputs.st_assetGroup_treeNumber,
      StAssetGroupMerkleRoot: inputs.st_assetGroup_merkleRoot.toString(),

      WtCommitment: inputs.wt_commitment,
      WtPathElements: inputs.wt_pathElements,
      WtPathIndices: inputs.wt_pathIndices.toString(),
      WtPrivateKey: inputs.wt_privateKey,
      WtIdParams: inputs.wt_idParams,
      WtContractAddress: inputs.wt_contractAddress,

      WtAssetGroupPathElements: inputs.wt_assetGroup_pathElements.map((k) =>
        k.toString()
      ),
      WtAssetGroupPathIndices: inputs.wt_assetGroup_pathIndices,
    }
  );

  return {
    status: 200,
    message: "ok",
  };
}

async function AuctionBidProof(
  st_auctionId,
  wt_bidAmount,
  wt_bidRandom,
  assetAddress,
  wt_valuesIn,
  keysIn,
  wt_valuesOut,
  keysOut,
  merkleDepth,
  merkleProofs,
  st_merkleRoots,
  st_treeNumbers,
  st_vaultId,
  wt_idParamsIn,
  wt_idParamsOut,
  st_assetGroup_merkleRoot,
  assetGroup_merkleProof
) {
  const st_blindedBid = utils.pedersen(wt_bidAmount, wt_bidRandom);
  const st_commitmentsOut = [];
  const st_nullifiers = [];
  const wt_pathIndices = [];
  let wt_pathElements = [];

  for (let i = 0; i < wt_valuesIn.length; i++) {
    let uniqueId;
    if (st_vaultId == 0) {
      uniqueId = utils.erc20UniqueId(assetAddress, wt_idParamsIn[i][0]);
    } else if (st_vaultId == 1) {
      uniqueId = utils.erc721UniqueId(assetAddress, wt_idParamsIn[i][0]);
    } else if (st_vaultId == 2) {
      uniqueId = utils.erc1155UniqueId(
        assetAddress,
        wt_idParamsIn[i][1],
        wt_idParamsIn[i][0]
      );
    }
    const cmt = utils.getCommitment(uniqueId, keysIn[i].publicKey);

    if (wt_valuesIn[i] === 0n) {
      wt_pathIndices[i] = 0;
      wt_pathElements.push(new Array(merkleDepth).fill(0n));
    } else {
      wt_pathIndices[i] = merkleProofs[i].indices;
      wt_pathElements.push(merkleProofs[i].elements);
    }
    st_nullifiers.push(
      utils.getNullifier(keysIn[i].privateKey, wt_pathIndices[i])
    );
  }
  for (let i = 0; i < keysOut.length; i++) {
    let uniqueIdOut;
    if (st_vaultId == 0) {
      uniqueIdOut = utils.erc20UniqueId(assetAddress, wt_idParamsOut[i][0]);
    } else if (st_vaultId == 1) {
      uniqueIdOut = utils.erc721UniqueId(assetAddress, wt_idParamsOut[i][0]);
    } else if (st_vaultId == 2) {
      uniqueIdOut = utils.erc1155UniqueId(
        assetAddress,
        wt_idParamsOut[i][1],
        wt_idParamsOut[i][0]
      );
    }

    st_commitmentsOut.push(
      utils.getCommitment(uniqueIdOut, keysOut[i].publicKey)
    );
  }
  wt_pathElements = wt_pathElements.flat(1);

  const { ws, pk } = { ws: wasm.auctionBidErc20, pk: pKeys.auctionBidErc20 };

  let wt_assetGroup_pathElements;
  if (st_assetGroup_merkleRoot != 0) {
    wt_assetGroup_pathElements = assetGroup_merkleProof.elements;
  } else {
    wt_assetGroup_pathElements = [];
    for (var i = 0; i < merkleDepth; i++) {
      wt_assetGroup_pathElements.push(0n);
    }
  }

  let wt_assetGroup_pathIndices = 0;
  if (st_assetGroup_merkleRoot != 0) {
    wt_assetGroup_pathIndices = assetGroup_merkleProof.indices;
  }

  const circuitInputs = {
    st_auctionId,
    st_blindedBid,
    st_vaultId,
    st_merkleRoots,
    st_nullifiers,
    st_treeNumbers,
    st_commitmentsOut,
    wt_bidAmount,
    wt_bidRandom,
    wt_privateKeys: keysIn.map((k) => k.privateKey),
    wt_valuesIn,
    wt_pathElements,
    wt_pathIndices,
    wt_assetContractAddress: BigInt(assetAddress),
    wt_recipientPK: keysOut.map((k) => k.publicKey),
    wt_valuesOut,
    st_assetGroup_merkleRoot,
    wt_assetGroup_pathIndices,
    wt_assetGroup_pathElements,
    wt_idParamsIn: wt_idParamsIn.flat(1),
    wt_idParamsOut: wt_idParamsOut.flat(1),
  };
  let curve = await snarkjs.curves.getCurveFromName("bn128");

  const fullProof = await snarkjs.groth16.fullProve(circuitInputs, ws, pk);
  const solidityProof = formatProof(fullProof.proof);

  let pathElementsString = [];
  let first = [];
  let second = [];
  for (i = 0; i < 8; i++) {
    first[i] = wt_pathElements[i].toString(); // elements  0..7
    second[i] = wt_pathElements[i + 8].toString(); // elements  8..15
  }
  pathElementsString.push(first);
  pathElementsString.push(second);

  let In = wt_idParamsIn.flat(1);
  let wtIn = [];
  let firstWtIn = [];
  let secondWtIn = [];
  for (i = 0; i < 5; i++) {
    firstWtIn[i] = In[i].toString(); // elements  0..7
    secondWtIn[i] = In[i + 5].toString(); // elements  8..15
  }
  wtIn.push(firstWtIn);
  wtIn.push(secondWtIn);

  let Out = wt_idParamsOut.flat(1);
  let wtOut = [];
  let firstWtOut = [];
  let secondWtOut = [];
  for (i = 0; i < 5; i++) {
    firstWtOut[i] = Out[i].toString(); // elements  0..7
    secondWtOut[i] = Out[i + 5].toString(); // elements  8..15
  }
  wtOut.push(firstWtOut);
  wtOut.push(secondWtOut);

  let proofGnark = postRequestGnarkCircuit(
    "http://localhost:8081/proof/auctionBid",
    {
      StAuctionId: st_auctionId.toString(),
      StBlindedBid: st_blindedBid,
      StVaultId: st_vaultId.toString(),
      StTreeNumber: st_treeNumbers.map((k) => k.toString()),
      StMerkleRoot: st_merkleRoots,
      StNullifier: st_nullifiers,
      StCommitmentsOuts: st_commitmentsOut,
      StAssetGroupMerkleRoot: st_assetGroup_merkleRoot,
      WtBidAmount: wt_bidAmount,
      WtBidRandom: wt_bidRandom,
      WtPrivateKeys: keysIn.map((k) => k.privateKey),
      WtValuesIn: wt_valuesIn,
      WtPathElements: pathElementsString,
      WtPathIndices: wt_pathIndices.map((k) => k.toString()),
      WtContractAddress: BigInt(assetAddress).toString(),
      WtRecipientPK: keysOut.map((k) => k.publicKey.toString()),
      WtValuesOut: wt_valuesOut,
      WtAssetGroupPathElements: wt_assetGroup_pathElements,
      WtAssetGroupPathIndices: wt_assetGroup_pathIndices.toString(),
      WtIdParamsIn: wtIn,
      WtIdParamsOut: wtOut,
    }
  );

  let statement = [st_auctionId, st_blindedBid, st_vaultId]
    .concat(st_treeNumbers)
    .concat(st_merkleRoots)
    .concat(st_nullifiers)
    .concat(st_commitmentsOut)
    .concat([st_assetGroup_merkleRoot]);
  curve.terminate();
  return {
    proof: solidityProof,
    statement,
    numberOfInputs: 2,
    numberOfOutputs: 2,
  };
}

async function AuctionPrivateOpeningProof(inputs) {
  let proofGnark = postRequestGnarkCircuit(
    "http://localhost:8081/proof/auctionPrivateOpening",
    {
      StVaultId: inputs.st_auctionId.toString(),
      StBlindedBid: inputs.st_blindedBid.toString(),
      WtBidAmount: inputs.wt_bidAmount.toString(),
      WtBidRandom: inputs.wt_bidRandom.toString(),
    }
  );

  return {
    status: 200,
    message: "ok",
  };
}

// Generates proof for AuctionNotWinningBid.circom
// proves that a bid is less than the winning bid.
async function AuctionNotWinningBidProof(inputs) {
  let proofGnark = postRequestGnarkCircuit(
    "http://localhost:8081/proof/auctionNotWinning",
    {
      StVaultId: inputs.st_auctionId.toString(),
      StBlindedBidDifference: inputs.st_blindedBidDifference.toString(),
      StBidBlockNumber: inputs.st_bidBlockNumber.toString(),
      StWinningBidBlockNumber: inputs.st_winningBidBlockNumber.toString(),

      WtBidAmount: inputs.wt_bidAmount.toString(),
      WtBidRandom: inputs.wt_bidRandom.toString(),
      WtWinningBidAmount: inputs.wt_winningBidAmount.toString(),
      WtWinningBidRandom: inputs.wt_winningBidRandom.toString(),
    }
  );

  return {
    status: 200,
    message: "ok",
  };
}

function chunkArray(arr, chunkSize = 8) {
  console.log(typeof arr);
  console.log(arr);

  const result = [];

  // Split array into chunks
  for (let i = 0; i < arr.length; i += chunkSize) {
    const chunk = [];

    // Fill current chunk
    for (let j = 0; j < chunkSize; j++) {
      chunk.push(arr[i + j]);
    }

    result.push(chunk);
  }

  return result;
}
function splitPathElements(inputs) {
  return chunkArray(inputs.wt_pathElements, 8);
}

/**
 * Generate a ZK proof for private minting
 * @param {Object} inputs - The inputs for the proof
 * @param {string} inputs.commitment - The commitment to mint
 * @param {string} inputs.contractAddress - The contract address
 * @param {string} inputs.tokenId - The token ID
 * @param {string} inputs.salt - Random salt for commitment privacy
 * @param {string} inputs.amount - The amount to mint
 * @param {string} inputs.publicKey - The receiver's public key
 * @returns {Promise<{proof: string[], publicSignal: string[]}>}
 */
async function PrivateMintProof(inputs) {
  const url = "http://localhost:8081/proof/privateMint";

  try {
    const res = await axios.post(
      url,
      {
        commitment: inputs.commitment.toString(),
        contractAddress: inputs.contractAddress.toString(),
        tokenId: inputs.tokenId.toString(),
        salt: inputs.salt ? inputs.salt.toString() : "0",
        amount: inputs.amount.toString(),
        publicKey: inputs.publicKey.toString(),
        cipherText: inputs.cipherText.toString(),
      },
      {
        headers: {
          "Content-Type": "application/json",
        },
      }
    );

    console.log("PrivateMint proof generated successfully");
    return {
      proof: res.data.proof.map((p) => p.toString()),
      publicSignal: res.data.publicSignal.map((p) => p.toString()),
    };
  } catch (err) {
    if (err.response) {
      console.error("Error:", err.response.status, err.response.data);
      throw new Error(
        `PrivateMint proof generation failed: ${err.response.data}`
      );
    } else {
      console.error("Request failed:", err.message);
      throw new Error(`PrivateMint proof generation failed: ${err.message}`);
    }
  }
}

module.exports = {
  GnarkProver,
  PrivateMintProof,
};
