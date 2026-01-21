const dvpSnarks = require("../src/core/dvpSnarks.js");
const dvpConf = require("../zkdvp.config.json");

async function initializeDvpCircuits() {
    console.log("initializing Circom circuits...");

    const circuitConfs = dvpConf.circom.circuits;
    await dvpSnarks.generateSnarkKeys(circuitConfs);

    await dvpSnarks.contributeToCeremonies(circuitConfs);

    console.log("Circom circuits have been initialized.");
}

initializeDvpCircuits();