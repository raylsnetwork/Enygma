/*
Low-level proof generation.
These functions are directly calling proof generation functions
    from snarkJs library
*/
const snarkjs = require("snarkjs");
const utils = require("./utils");
const { babyJub } = require("@iden3/js-crypto");
const { GnarkProver } = require("./prover_gnark");

function getWasmPath(circuitName) {
  return "./build/" + circuitName + ".wasm";
}

function getZkeyPath(circuitName) {
  return "./build/" + circuitName + ".zkey";
}

function formatProof(proof) {
  return {
    a: proof.pi_a.slice(0, 2),
    b: proof.pi_b.map((x) => x.reverse()).slice(0, 2),
    c: proof.pi_c.slice(0, 2),
  };
}

async function prove(circuitName, inputs) {
  console.log(`Constructing proof of ${circuitName}`);
  // console.log("inputs: ", JSON.stringify(inputs, null, 4));

  const wasmPath = getWasmPath(circuitName);
  const zkeyPath = getZkeyPath(circuitName);

  console.log("Circuit paths: ", wasmPath, zkeyPath);

  const circuitInputs = prepareCircuitInputs(circuitName, inputs);
  GnarkProver(circuitInputs, zkeyPath);

  let curve = await snarkjs.curves.getCurveFromName("bn128");
  const fullProof = await snarkjs.groth16.fullProve(
    circuitInputs,
    wasmPath,
    zkeyPath
  );
  curve.terminate();

  if ("keysOut" in inputs) {
    return packProof(
      circuitInputs,
      formatProof(fullProof.proof),
      inputs.keysIn.length,
      inputs.keysOut.length
    );
  } else {
    if ("keysIn" in inputs) {
      return packProof(
        circuitInputs,
        formatProof(fullProof.proof),
        inputs.keysIn.length,
        0
      );
    } else {
      return packProof(circuitInputs, formatProof(fullProof.proof), 0, 0);
    }
  }
}

function prepareCircuitInputs(circuitName, inputs) {
  var circuitInputs = {};

  if (circuitName.includes("Auction")) {
    if (circuitName.includes("PrivateOpening")) {
      circuitInputs = {
        st_auctionId: inputs.auctionId,
        st_blindedBid: utils.pedersen(inputs.bidAmount, inputs.bidRandom),
        wt_bidAmount: inputs.bidAmount,
        wt_bidRandom: inputs.bidRandom,
      };
    } else if (circuitName.includes("NotWinning")) {
      const blindedBid = utils.pedersen(inputs.bidAmount, inputs.bidRandom);
      const blindedWinningBid = utils.pedersen(
        inputs.winningBidAmount,
        inputs.winningBidRandom
      );

      // adding utils.SNARK_SCALAR_FIELD to avoid negative values
      const blindedBidDifference =
        (blindedWinningBid - blindedBid + utils.SNARK_SCALAR_FIELD) %
        utils.SNARK_SCALAR_FIELD;

      circuitInputs = {
        st_auctionId: inputs.auctionId,
        st_blindedBidDifference: blindedBidDifference,
        st_bidBlockNumber: inputs.bidBlockNumber,
        st_winningBidBlockNumber: inputs.winningBidBlockNumber,
        wt_bidAmount: inputs.bidAmount,
        wt_bidRandom: inputs.bidRandom,
        wt_winningBidAmount: inputs.winningBidAmount,
        wt_winningBidRandom: inputs.winningBidRandom,
      };
    } else {
      if (circuitName.includes("Init")) {
        circuitInputs = prepareAuctionInitCircuitInputs(circuitName, inputs);
      } else if (circuitName.includes("Bid")) {
        circuitInputs = prepareAuctionBidCircuitInputs(circuitName, inputs);
        circuitInputs = prepareAuctioneerCircuitInputs(inputs, circuitInputs);
      }
      circuitInputs = prepareAuditorCircuitInputs(inputs, circuitInputs);
      circuitInputs = generateAssetGroupParams(inputs, circuitInputs);
    }
  } else {
    if (circuitName.includes("Erc20")) {
      circuitInputs = prepareErc20CircuitInputs(circuitName, inputs);
    } else if (circuitName.includes("Erc721")) {
      circuitInputs = prepareErc721CircuitInputs(circuitName, inputs);
    } else if (circuitName.includes("Erc1155")) {
      circuitInputs = prepareErc1155CircuitInputs(circuitName, inputs);
    }

    if (circuitName.includes("Auditor")) {
      circuitInputs = prepareAuditorCircuitInputs(inputs, circuitInputs);
    }

    // adding assetGroup circuitInputs
    if (circuitName.includes("Erc1155")) {
      circuitInputs = generateAssetGroupParams(inputs, circuitInputs);
    }
  }

  console.log("Circuit Inputs: ", JSON.stringify(circuitInputs, null, 4));

  return circuitInputs;
}

function generateUniqueId(circuitName, inputs, index, selector) {
  if (circuitName.includes("AuctionInit")) {
    const vaultId = inputs.vaultId;
    const contractAddress = inputs.contractAddress;
    const idParams = inputs.idParams;

    if (vaultId == 0) {
      return utils.erc20UniqueId(contractAddress, idParams[0]);
    } else if (vaultId == 1) {
      return utils.erc721UniqueId(contractAddress, idParams[0]);
    } else if (vaultId == 2) {
      return utils.erc1155UniqueId(contractAddress, idParams[1], idParams[0]);
    }
  } else if (circuitName.includes("AuctionBid")) {
    var selectorString = "idParams" + selector;
    const vaultId = inputs.vaultId;
    const contractAddress = inputs.contractAddress;
    const idParams = inputs[selectorString];

    if (vaultId == 0) {
      return utils.erc20UniqueId(contractAddress, idParams[index][0]);
    } else if (vaultId == 1) {
      return utils.erc721UniqueId(contractAddress, idParams[index][0]);
    } else if (vaultId == 2) {
      return utils.erc1155UniqueId(
        contractAddress,
        idParams[index][1],
        idParams[index][0]
      );
    }
  } else {
    var selectorString = "values" + selector;
    if (!(selectorString in inputs)) {
      selectorString = "values";
    }
    const value = inputs[selectorString][index];
    if (circuitName.includes("Erc20")) {
      return utils.erc20UniqueId(inputs.erc20ContractAddress, value);
    } else if (circuitName.includes("Erc721")) {
      return utils.erc721UniqueId(inputs.erc721ContractAddress, value);
    } else if (circuitName.includes("Erc1155")) {
      var tokenId;
      if (circuitName.includes("JoinSplit")) {
        tokenId = inputs.erc1155TokenId;
      } else {
        tokenId = inputs.erc1155TokenIds[index];
      }
      // console.log("TokenId : ", tokenId, value, inputs.erc1155ContractAddress);
      return utils.erc1155UniqueId(
        inputs.erc1155ContractAddress,
        tokenId,
        value
      );
    }
  }

  return -1;
}

function generateParameters(circuitName, inputs) {
  const commitmentsOut = [];
  const nullifiers = [];
  const pathIndices = [];
  let pathElements = [];
  const uniqueIds = [];

  const merkleDepth = inputs.merkleProofs[0].elements.length;
  for (let i = 0; i < inputs.keysIn.length; i++) {
    var idIn = generateUniqueId(circuitName, inputs, i, "In");
    uniqueIds.push(idIn);
    // for compatibility with both fungible and non-fungible proofs
    if (
      ("valuesIn" in inputs && inputs.valuesIn[i] === 0n) ||
      ("values" in inputs && inputs.values[i] === 0n)
    ) {
      pathIndices[i] = 0;
      pathElements.push(new Array(merkleDepth).fill(0n));
    } else {
      pathIndices[i] = inputs.merkleProofs[i].indices;
      pathElements.push(inputs.merkleProofs[i].elements);
    }

    nullifiers[i] = utils.getNullifier(
      inputs.keysIn[i].privateKey,
      pathIndices[i]
    );
  }
  if ("keysOut" in inputs) {
    for (let i = 0; i < inputs.keysOut.length; i++) {
      var idOut = generateUniqueId(circuitName, inputs, i, "Out");
      commitmentsOut[i] = utils.getCommitment(
        idOut,
        inputs.keysOut[i].publicKey
      );
    }
  } else {
    if (circuitName.includes("AuctionInit")) {
      var idIn = generateUniqueId(circuitName, inputs, 0, "In");
      commitmentsOut.push(
        utils.getCommitment(idIn, inputs.keysIn[0].publicKey)
      );
    }
  }
  pathElements = pathElements.flat(1);

  return {
    pathElements,
    pathIndices,
    nullifiers,
    commitmentsOut,
    uniqueIds,
  };
}

function generateAssetGroupParams(inputs, circuitInputs) {
  var wt_assetGroup_pathIndices = [];
  var wt_assetGroup_pathElements = [];

  const merkleDepth = inputs.merkleProofs[0].elements.length;

  var assetGroup_circuitInputs;

  if ("values" in inputs) {
    // no in/out values => non-fungible => batch mode => multiple assetGroups
    // TODO:: weak condition, might cause problem later
    for (let i = 0; i < inputs.values.length; i++) {
      if (inputs.values[i] === 0n) {
        wt_assetGroup_pathIndices.push(0);
        wt_assetGroup_pathElements.push(new Array().fill(0n));
      } else {
        wt_assetGroup_pathIndices.push(
          inputs.assetGroup_merkleProofs[i].indices
        );
        wt_assetGroup_pathElements.push(
          inputs.assetGroup_merkleProofs[i].elements
        );
      }
    }
    st_assetGroup_pathElements = wt_assetGroup_pathElements.flat(1);

    const st_assetGroup_merkleRoots = inputs.assetGroup_merkleProofs.map(
      (k) => k.root
    );
    const st_assetGroup_treeNumbers = inputs.assetGroup_treeNumbers;

    assetGroup_circuitInputs = {
      st_assetGroup_treeNumbers,
      st_assetGroup_merkleRoots,
      wt_assetGroup_pathElements,
      wt_assetGroup_pathIndices,
    };
  } else {
    if (inputs.assetGroup_merkleProof.root != 0) {
      wt_assetGroup_pathIndices = inputs.assetGroup_merkleProof.indices;
      wt_assetGroup_pathElements = inputs.assetGroup_merkleProof.elements;

      assetGroup_circuitInputs = {
        st_assetGroup_treeNumber: inputs.assetGroup_treeNumber,
        st_assetGroup_merkleRoot: inputs.assetGroup_merkleProof.root,
        wt_assetGroup_pathElements,
        wt_assetGroup_pathIndices,
      };
    } else {
      wt_assetGroup_pathElements = new Array(merkleDepth).fill(0n);

      assetGroup_circuitInputs = {
        st_assetGroup_treeNumber: 0n,
        st_assetGroup_merkleRoot: 0n,
        wt_assetGroup_pathElements,
        wt_assetGroup_pathIndices: 0n,
      };
    }
  }
  return { ...circuitInputs, ...assetGroup_circuitInputs };
}

function prepareEncryptionInput(inputs, tags) {
  var dataToEncrypt = [];

  for (var i = 0; i < tags.length; i++) {
    const tag = tags[i];
    if (tag in inputs) {
      console.log("Tag: ", tag);
      if (Array.isArray(inputs[tag])) {
        console.log("is array: ", inputs[tag]);
        for (var j = 0; j < inputs[tag].length; j++) {
          if (Array.isArray(inputs[tag][j])) {
            console.log("is array: ", inputs[tag][j]);
            for (var z = 0; z < inputs[tag][j].length; z++) {
              dataToEncrypt.push(BigInt(inputs[tag][j][z]));
            }
          } else {
            dataToEncrypt.push(BigInt(inputs[tag][j]));
          }
        }
      } else {
        dataToEncrypt.push(BigInt(inputs[tag]));
      }
    }
  }

  // TODO:: there is a problem with PoseidonDecrypt that does not work with length % 3 = 2
  if (dataToEncrypt.length % 3 == 2) {
    dataToEncrypt.push(0n);
  }
  // console.log("Data to encrypt: ")
  // console.log(dataToEncrypt);

  return dataToEncrypt;
}

function prepareAuctioneerCircuitInputs(inputs, circuitInputs) {
  inputsForEncryption = prepareEncryptionInput(inputs, [
    "bidAmount",
    "bidRandom",
  ]);

  // console.log(inputsForEncryption);

  const encPack = utils.poseidonEncryptWrapper(
    babyJub,
    inputsForEncryption,
    inputs.auctioneer_publicKey
  );

  const auctioneer_circuitInputs = {
    st_auctioneer_publicKey: inputs.auctioneer_publicKey,
    st_auctioneer_authKey: encPack.authKey,
    st_auctioneer_nonce: encPack.nonce,
    st_auctioneer_encryptedValues: encPack.encrypted,
    wt_auctioneer_random: encPack.randomValue,
  };

  return { ...circuitInputs, ...auctioneer_circuitInputs };
}

function prepareAuditorCircuitInputs(inputs, circuitInputs) {
  // The list of values that needs encryption if exist in the inputs
  // The order of these are important in the order of the arguments in statement
  const tags = [
    "values",
    "valuesIn",
    "valuesOut",
    "idParams",
    "idParamsIn",
    "idParamsOut",
    "erc1155TokenId",
    "erc1155TokenIds",
    "erc1155ContractAddress",
    "contractAddress",
    "bidAmount",
    "bidRandom",
  ];
  inputsForEncryption = prepareEncryptionInput(inputs, tags);

  console.log(inputsForEncryption);

  const encPack = utils.poseidonEncryptWrapper(
    babyJub,
    inputsForEncryption,
    inputs.auditor_publicKey
  );

  const auditor_circuitInputs = {
    st_auditor_publicKey: inputs.auditor_publicKey,
    st_auditor_authKey: encPack.authKey,
    st_auditor_nonce: encPack.nonce,
    st_auditor_encryptedValues: encPack.encrypted,
    wt_auditor_random: encPack.randomValue,
  };

  return { ...circuitInputs, ...auditor_circuitInputs };
}

function prepareErc20CircuitInputs(circuitName, inputs) {
  if (circuitName == "JoinSplitErc20_10_2") {
    if (inputs.valuesIn.length > 10 || inputs.valuesOut.length > 10) {
      throw Error("Two many inputs for JoinSplit 10-2 Circuit");
    }
    for (var i = inputs.valuesIn.length; i < 10; i++) {
      inputs.valuesIn.push(0n);
      inputs.keysIn.push({ publicKey: 0n, privateKey: 0n });
      inputs.treeNumbers.push(0n);
      inputs.merkleProofs.push({ root: 0n });
    }
  } else if (circuitName == "JoinSplitErc20") {
    if (inputs.valuesIn.length != 2 || inputs.valuesOut.length != 2) {
      throw Error("Two many inputs for JoinSplit Circuit");
    }
  } else {
    throw Error("Two many inputs for JoinSplit Circuit");
  }

  computedParams = generateParameters(circuitName, inputs);
  const circuitInputs = packCommonCircuitInputs(inputs, computedParams);
  circuitInputs.wt_valuesIn = inputs.valuesIn;
  circuitInputs.wt_valuesOut = inputs.valuesOut;
  circuitInputs.wt_erc20ContractAddress = inputs.erc20ContractAddress;

  return circuitInputs;
}

function prepareErc721CircuitInputs(circuitName, inputs) {
  if (circuitName == "OwnershipErc721") {
    if (inputs.values.length != 1) {
      throw Error("Wrong number of inputs for OwnershipErc721");
    }
  } else if (circuitName == "BatchErc721") {
    throw Error("Not implemented.");
  }

  params = generateParameters(circuitName, inputs);
  const circuitInputs = packCommonCircuitInputs(inputs, params);

  circuitInputs.wt_values = params.uniqueIds;

  return circuitInputs;
}

// computing the proof parameters based on raw input parameters
function prepareErc1155CircuitInputs(circuitName, inputs) {
  params = generateParameters(circuitName, inputs);

  const circuitInputs = packCommonCircuitInputs(inputs, params);
  circuitInputs.wt_erc1155ContractAddress = BigInt(
    inputs.erc1155ContractAddress
  );

  if (circuitName.includes("Ownership")) {
    circuitInputs.wt_values = inputs.values;
    circuitInputs.wt_erc1155TokenIds = inputs.erc1155TokenIds;

    if (inputs.values.length == 1) {
    } else {
      throw Error("Wrong number of inputs for Erc1155 circuits");
    }
  } else if (circuitName.includes("JoinSplit")) {
    circuitInputs.wt_valuesIn = inputs.valuesIn;
    circuitInputs.wt_valuesOut = inputs.valuesOut;
    circuitInputs.wt_erc1155TokenId = inputs.erc1155TokenId;
  }

  return circuitInputs;
}

// computing the proof parameters based on raw input parameters
function prepareAuctionInitCircuitInputs(circuitName, inputs) {
  params = generateParameters(circuitName, inputs);

  const circuitInputs = packAuctionInitCircuitInputs(inputs, params);
  circuitInputs.wt_contractAddress = BigInt(inputs.contractAddress);
  circuitInputs.st_auctionId = utils.getAuctionId(circuitInputs.wt_commitment);

  return circuitInputs;
}

function prepareAuctionBidCircuitInputs(circuitName, inputs) {
  params = generateParameters(circuitName, inputs);

  const circuitInputs = packAuctionBidCircuitInputs(inputs, params);

  return circuitInputs;
}

function packCommonCircuitInputs(inputs, generatedParams) {
  return {
    st_message: inputs.message,
    st_treeNumbers: inputs.treeNumbers,
    st_merkleRoots: inputs.merkleProofs.map((k) => k.root),
    st_nullifiers: generatedParams.nullifiers,
    st_commitmentsOut: generatedParams.commitmentsOut,
    wt_privateKeysIn: inputs.keysIn.map((k) => k.privateKey),
    wt_publicKeysOut: inputs.keysOut.map((k) => k.publicKey),
    wt_pathElements: generatedParams.pathElements,
    wt_pathIndices: generatedParams.pathIndices,
  };
}

function packAuctionInitCircuitInputs(inputs, generatedParams) {
  return {
    st_beacon: inputs.beacon,
    st_auctionId: inputs.auctionId,
    st_vaultId: inputs.vaultId,
    st_treeNumber: inputs.treeNumbers[0],
    st_merkleRoot: inputs.merkleProofs[0].root,
    st_nullifier: generatedParams.nullifiers[0],
    wt_privateKey: inputs.keysIn[0].privateKey,
    wt_pathElements: generatedParams.pathElements,
    wt_pathIndices: generatedParams.pathIndices,
    wt_commitment: generatedParams.commitmentsOut[0],
    wt_idParams: inputs.idParams,
  };
}

function packAuctionBidCircuitInputs(inputs, generatedParams) {
  return {
    st_beacon: inputs.beacon,
    st_auctionId: inputs.auctionId,
    st_blindedBid: utils.pedersen(inputs.bidAmount, inputs.bidRandom),
    st_vaultId: inputs.vaultId,
    st_treeNumbers: inputs.treeNumbers,
    st_merkleRoots: inputs.merkleProofs.map((k) => k.root),
    st_nullifiers: generatedParams.nullifiers,
    st_commitmentsOut: generatedParams.commitmentsOut,
    wt_privateKeysIn: inputs.keysIn.map((k) => k.privateKey),
    wt_publicKeysOut: inputs.keysOut.map((k) => k.publicKey),
    wt_pathElements: generatedParams.pathElements,
    wt_pathIndices: generatedParams.pathIndices,
    wt_idParamsIn: inputs.idParamsIn,
    wt_idParamsOut: inputs.idParamsOut,
    wt_bidAmount: inputs.bidAmount,
    wt_bidRandom: inputs.bidRandom,
    wt_contractAddress: BigInt(inputs.contractAddress),
  };
}

function packProof(circuitInputs, proof, numberOfInputs, numberOfOutputs) {
  const statement = [];

  for (const key in circuitInputs) {
    if (key.startsWith("st_")) {
      // if statement argument is an array => push the elements.
      if (Array.isArray(circuitInputs[key])) {
        for (var i = 0; i < circuitInputs[key].length; i++) {
          statement.push(circuitInputs[key][i]);
        }
      } else {
        // else => push the argument itself.
        statement.push(circuitInputs[key]);
      }
    }
  }

  console.log("statement: ", JSON.stringify(statement, null, 4));
  console.log("statement.length: ", statement.length);

  return {
    proof,
    numberOfInputs,
    numberOfOutputs,
    statement,
  };
}

module.exports = {
  prove,
};
