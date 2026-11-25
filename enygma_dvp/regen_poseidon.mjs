// Regenerates real PoseidonT3/T5 artifacts from circomlibjs; retail's
// setup.sh and shield_demo/run.sh run it after `npx hardhat compile`, which
// overwrites them with placeholders. scripts/regen_poseidon.js holds the
// artifacts, so this and enygma_dvp's own runs of it write the same ones.
import { createRequire } from 'module';

const require = createRequire(import.meta.url);
require('./scripts/regen_poseidon.js').writePoseidonArtifacts();
