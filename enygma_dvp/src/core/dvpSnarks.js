const snarkjs = require("snarkjs");
const crypto = require("crypto");
const { stringifyBigInts } = require("./utils");

const { zKey, r1cs, curves } = snarkjs;
const ptauPath = "./build/powersOfTau28_hez_final_20.ptau";
const fs = require("fs");


async function generateSnarkKeyForCircuit(filename){

  const filePath = "build/" + filename;
	const riscInfo = await r1cs.info(filePath + ".r1cs");
	console.log(
		 `\nFilename: ${filename}\nConstraints: ${riscInfo.nConstraints}\nWitness: ${riscInfo.nPrvInputs}\nStatement: ${riscInfo.nPubInputs}\n`,
	);

	await zKey.newZKey(filePath + ".r1cs", ptauPath, filePath + ".tmp");

}

async function generateSnarkKeys(circuitConfs, tags=["all"]) {
	console.log("Generating Snark keys ... ");

	// getting the curve to terminate at the end.
  let curve = await curves.getCurveFromName("bn128");

  // TODO:: add tag conditions 
  for(var i = 0; i < circuitConfs.length; i++){
    await generateSnarkKeyForCircuit(circuitConfs[i].filename);

  }
  curve.terminate();

	console.log("zkeys have been generated for all circuits... waiting for initialization task.");
}

async function contributeToCeremonies(circuitConfs, tags = ["all"]){
  for(var i = 0; i < circuitConfs.length; i++){
    await contributeToCeremony(circuitConfs[i].filename);
  }
}

async function contributeToCeremony(circuitName){
	let curve = await curves.getCurveFromName("bn128");

	const jsPK = `./build/${circuitName}.zkey`;
	const jsVK = `./build/${circuitName}.json`;
	const jsTmp = `./build/${circuitName}.tmp`;

	const random = crypto.randomBytes(32).toString("hex");
	await zKey.contribute(jsTmp, jsPK, "Alice", random);

	const vKey = await zKey.exportVerificationKey(jsPK);
	const vk = JSON.stringify(stringifyBigInts(vKey), null, 1);

	fs.writeFileSync(jsVK, vk);
	fs.rmSync(jsTmp);
	curve.terminate();

}
/* eslint-disable no-plusplus */

function formatVKey(vkey) {
  const ic = [];
  for (let i = 0; i < vkey.IC.length; i++) {
    ic.push({ x: BigInt(vkey.IC[i][0]), y: BigInt(vkey.IC[i][1]) });
  }
  return {
    alpha1: {
      x: BigInt(vkey.vk_alpha_1[0]),
      y: BigInt(vkey.vk_alpha_1[1]),
    },
    beta2: {
      x: [BigInt(vkey.vk_beta_2[0][1]), BigInt(vkey.vk_beta_2[0][0])],
      y: [BigInt(vkey.vk_beta_2[1][1]), BigInt(vkey.vk_beta_2[1][0])],
    },
    gamma2: {
      x: [BigInt(vkey.vk_gamma_2[0][1]), BigInt(vkey.vk_gamma_2[0][0])],
      y: [BigInt(vkey.vk_gamma_2[1][1]), BigInt(vkey.vk_gamma_2[1][0])],
    },
    delta2: {
      x: [BigInt(vkey.vk_delta_2[0][1]), BigInt(vkey.vk_delta_2[0][0])],
      y: [BigInt(vkey.vk_delta_2[1][1]), BigInt(vkey.vk_delta_2[1][0])],
    },
    ic,
  };
}

function getVerificationKeys(circuitConfs) {
  const verificationKeys = [];

  for(var i = 0; i< circuitConfs.length; i++){
    const filePath = "./build/" + circuitConfs[i].filename + ".json";
    console.log(filePath);
    const newVK = formatVKey(JSON.parse(fs.readFileSync(filePath)));

    verificationKeys.push(newVK);
  }  
  return verificationKeys;
}

module.exports = {
	generateSnarkKeys,
	contributeToCeremonies,
	getVerificationKeys
};
