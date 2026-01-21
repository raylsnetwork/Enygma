/* global describe it before beforeEach afterEach */
const fs = require('fs');
const snarkjs = require('snarkjs');
const crypto = require('crypto');
const { stringifyBigInts } = require('../src/core/utils');
const dvpConf = require("../zkdvp.config.json");
const TREE_DEPTH = dvpConf["tree-depth"];
const { zKey, r1cs } = snarkjs;

describe('Setup Phase', async () => {
  const ptauPath = './build/powersOfTau28_hez_final_20.ptau';
  let startTime = 0;
  let label = '';
  const seperator = '#########################################\n';

  describe('JoinSplitErc20 Setup', async () => {
    const path = './build/JoinSplitErc20';
    const jsTmp = './build/JoinSplitErc20.tmp';

    before(async () => {
      fs.appendFileSync(path, seperator);
      fs.appendFileSync(path, `Merkle Tree Depth = ${TREE_DEPTH}\n`);
    });

    beforeEach(async () => {
      startTime = performance.now();
    });

    afterEach(async () => {
      const time = Math.round(performance.now() - startTime);
      fs.appendFileSync(path, `${label}: ${time}\n`);
    });

    it('Should prepare phase-2 of the JoinSplit trusted setup', async () => {
      label = 'Setup';
      const jsR1CS = './build/JoinSplitErc20.r1cs';
      const data = await r1cs.info(jsR1CS);
      fs.appendFileSync(path, `Constraints: ${data.nConstraints}\nWitness: ${data.nPrvInputs}\nStatement: ${data.nPubInputs}\n`);
      await zKey.newZKey(jsR1CS, ptauPath, jsTmp);
    });

    it('Should contribute to JoinSplit setup', async () => {
      label = 'Contribute';
      const jsPK = './build/JoinSplitErc20.zkey';
      const jsVK = './build/JoinSplitErc20.json';
      const random = crypto.randomBytes(32).toString('hex');
      await zKey.contribute(jsTmp, jsPK, 'Alice', random);
      const vKey = await zKey.exportVerificationKey(jsPK);
      const vk = JSON.stringify(stringifyBigInts(vKey), null, 1);
      fs.writeFileSync(jsVK, vk);
      fs.rmSync(jsTmp);
    });
  });

  describe('Ownership setup', async () => {
    const path = './build/OwnershipErc721';
    const ownTmp = './build/OwnershipErc721.tmp';

    before(async () => {
      fs.appendFileSync(path, seperator);
      fs.appendFileSync(path, `Merkle Tree Depth = ${TREE_DEPTH}\n`);
    });

    beforeEach(async () => {
      startTime = performance.now();
    });

    afterEach(async () => {
      const time = Math.round(performance.now() - startTime);
      fs.appendFileSync(path, `${label}: ${time}ms\n`);
    });

    it('Should prepare phase-2 of the Ownership trusted setup', async () => {
      label = 'Setup';
      const ownR1CS = './build/OwnershipErc721.r1cs';
      const data = await r1cs.info(ownR1CS);
      fs.appendFileSync(path, `Constraints: ${data.nConstraints}\nWitness: ${data.nPrvInputs}\nStatement: ${data.nPubInputs}\n`);
      await zKey.newZKey(ownR1CS, ptauPath, ownTmp);
    });

    it('Should contribute to Ownership setup', async () => {
      label = 'Contribute';
      const ownPK = './build/OwnershipErc721.zkey';
      const ownVK = './build/OwnershipErc721.json';
      const random = crypto.randomBytes(32).toString('hex');
      await zKey.contribute(ownTmp, ownPK, 'Alice', random);
      const vKey = await zKey.exportVerificationKey(ownPK);
      const vk = JSON.stringify(stringifyBigInts(vKey), null, 1);
      fs.writeFileSync(ownVK, vk);
      fs.rmSync(ownTmp);
    });
  });
});
