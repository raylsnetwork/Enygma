// EnygmaDvp's payment entry points each take one receipt shape, and the USDr
// fee leg of paymentWithUsdrFee and channelPaymentWithUsdrFee binds the
// statement of the payment it pays for (CiphertextHash.ofUsdrFeeLeg), so it
// settles only in the same call as that payment. These run against
// AcceptAllVerifier, so only EnygmaDvp's and the vaults' own checks apply:
// the statements are built here as the circuits lay them out
// (PaymentStatementLayout), against each vault's real Merkle root, ending in
// the ciphertext hash the contracts recompute.
const { expect } = require("chai");
const { ethers, artifacts } = require("hardhat");
const { POSEIDON_SOURCE } = require("../../scripts/regen_poseidon.js");
const { expectRevert } = require("./expectRevert.js");

const P = ethers.BigNumber.from("21888242871839275222246405745257275088548364400416034343698204186575808495617");
const coder = ethers.utils.defaultAbiCoder;
const TREE_DEPTH = 8;
const MAIN_VAULT = 0;
const USDR_VAULT = 1;
const USDR_FEE = 7;
const USDR_TOKEN_ID = 0;
const RELAYER_FEE = 5;
// The payee's note ciphertexts: an ML-KEM capsule and an AEAD ciphertext.
const CTXT = "0x" + "ab".repeat(1088);
const ENC = "0x" + "cd".repeat(48);
const NO_PROOF = { a: { x: 0, y: 0 }, b: { x: [0, 0], y: [0, 0] }, c: { x: 0, y: 0 } };

// CiphertextHash.ofNote: the one note a payment publishes.
function ofNote(ctxt, enc) {
  return ethers.BigNumber.from(ethers.utils.keccak256(coder.encode(["bytes[]", "bytes[]"], [[ctxt], [enc]]))).mod(P);
}

// CiphertextHash.ofChannelPayment: a channel payment's block, tag and note.
function ofChannelPayment(blockNumber, tag, ctxt) {
  const domain = ethers.utils.keccak256(ethers.utils.toUtf8Bytes("enygma-channel-payment-v1"));
  return ethers.BigNumber.from(
    ethers.utils.keccak256(coder.encode(["bytes32", "uint256", "bytes32", "bytes"], [domain, blockNumber, tag, ctxt]))
  ).mod(P);
}

// CiphertextHash.ofUsdrFeeLeg: the statement of the payment a USDr fee leg
// pays for, and the fee note's ciphertexts.
function ofUsdrFeeLeg(mainStatement, ctxt, enc) {
  const domain = ethers.utils.keccak256(ethers.utils.toUtf8Bytes("enygma-usdr-fee-leg-v1"));
  const main = ethers.utils.keccak256(coder.encode(["uint256[]"], [mainStatement]));
  return ethers.BigNumber.from(
    ethers.utils.keccak256(coder.encode(["bytes32", "bytes32", "bytes", "bytes"], [domain, main, ctxt, enc]))
  ).mod(P);
}

// A distinct nonzero signal, for a nullifier or a commitment.
let lastSignal = 1000;
const fresh = () => ++lastSignal;

function receipt(statement, numberOfInputs, numberOfOutputs) {
  return { proof: NO_PROOF, statement, numberOfInputs, numberOfOutputs };
}

// Each circuit's statement, spending notes of vault v (tree 0, its current
// root) and ending in ciphertextHash. The nullifier of input 0 is at index 3
// except in Payment2in (index 5).
const shapes = {
  payment: (v, ciphertextHash) => receipt([0, 0, v.root, fresh(), fresh(), fresh(), v.address, ciphertextHash], 1, 2),
  payment2in: (v, ciphertextHash) =>
    receipt([0, 0, 0, v.root, v.root, fresh(), fresh(), fresh(), fresh(), v.address, ciphertextHash], 2, 2),
  paymentFee: (v, fee, ciphertextHash) =>
    receipt([0, 0, v.root, fresh(), fresh(), fresh(), v.address, fee, ciphertextHash], 1, 2),
  relayerFee: (v, fee, ciphertextHash) =>
    receipt([0, 0, v.root, fresh(), fresh(), fresh(), fresh(), v.address, fee, ciphertextHash], 1, 3),
  usdrFee: (v, fee, tokenId, ciphertextHash) =>
    receipt([0, 0, v.root, fresh(), fresh(), fresh(), v.address, fee, tokenId, ciphertextHash], 1, 2),
};

// The USDr fee leg (USDR_FEE in the USDr vault) proved for the payment main,
// relayed with empty fee-note ciphertexts as the relayers do.
const feeLegFor = (env, main) => shapes.usdrFee(env.usdr, USDR_FEE, USDR_TOKEN_ID, ofUsdrFeeLeg(main.statement, "0x", "0x"));

// Deploys a Poseidon library from its abi and bytecode: the artifacts the
// compile hook writes for these libraries lack deployedLinkReferences, which
// getContractFactory(name) wants.
async function deployPoseidonLibrary(name) {
  const artifact = await artifacts.readArtifact(`${POSEIDON_SOURCE}:${name}`);
  return (await ethers.getContractFactory(artifact.abi, artifact.bytecode)).deploy();
}

// An EnygmaDvp on AcceptAllVerifier with two Erc20CoinVaults, the main token's
// (vault 0) and USDr's (vault 1, the pinned USDr fee vault), and the fees set.
async function deployDvp() {
  const poseidonT3 = await deployPoseidonLibrary("PoseidonT3");
  const poseidonT5 = await deployPoseidonLibrary("PoseidonT5");
  const wrapper = await (
    await ethers.getContractFactory("PoseidonWrapper", {
      libraries: {
        [`${POSEIDON_SOURCE}:PoseidonT3`]: poseidonT3.address,
        [`${POSEIDON_SOURCE}:PoseidonT5`]: poseidonT5.address,
      },
    })
  ).deploy();
  const verifier = await (await ethers.getContractFactory("AcceptAllVerifier")).deploy();
  const dvp = await (await ethers.getContractFactory("EnygmaDvp")).deploy(wrapper.address, ethers.constants.AddressZero);
  await (await dvp.initializeDvp(verifier.address)).wait();
  const vaults = [];
  for (const symbol of ["TKN", "USDR"]) {
    const token = await (await ethers.getContractFactory("RaylsERC20")).deploy(symbol, symbol);
    const vault = await (await ethers.getContractFactory("Erc20CoinVault")).deploy(dvp.address);
    await (await dvp.registerVault(vault.address, token.address, 1, TREE_DEPTH)).wait();
    vaults.push({ contract: vault, address: ethers.BigNumber.from(vault.address), root: await vault.merkleRoot() });
  }
  await (await dvp.setUsdrFeeVaultId(USDR_VAULT)).wait();
  await (await dvp.setUsdrFixedFee(USDR_FEE)).wait();
  await (await dvp.setUsdrTokenId(USDR_TOKEN_ID)).wait();
  await (await dvp.setRelayerFixedFee(RELAYER_FEE)).wait();
  return { dvp, main: vaults[MAIN_VAULT], usdr: vaults[USDR_VAULT] };
}

// Whether input 0 of r, a receipt against vault v, is spent.
async function spent(v, r) {
  const nullifier = r.numberOfInputs === 2 ? r.statement[5] : r.statement[3];
  return v.contract.nullifiers(0, nullifier);
}

describe("EnygmaDvp payment entry points", function () {
  let env, dvp;

  beforeEach(async function () {
    env = await deployDvp();
    dvp = env.dvp;
  });

  describe("the USDr fee leg", function () {
    it("settles in the same call as the payment it was proved for", async function () {
      const main = shapes.payment(env.main, ofNote(CTXT, ENC));
      const fee = feeLegFor(env, main);
      const rc = await (await dvp.paymentWithUsdrFee(main, MAIN_VAULT, CTXT, ENC, fee, USDR_VAULT, "0x", "0x")).wait();

      const payments = rc.events.filter((e) => e.event === "Payment").map((e) => e.args.vaultId.toNumber());
      expect(payments).to.deep.equal([MAIN_VAULT, USDR_VAULT]);
      expect(await spent(env.main, main)).to.equal(true);
      expect(await spent(env.usdr, fee)).to.equal(true);
    });

    it("settles in the same call as the channel payment it was proved for", async function () {
      const blockNumber = await ethers.provider.getBlockNumber();
      const tag = ethers.utils.keccak256(ethers.utils.toUtf8Bytes("a tag"));
      const main = shapes.payment(env.main, ofChannelPayment(blockNumber, tag, CTXT));
      const fee = feeLegFor(env, main);
      await (await dvp.channelPaymentWithUsdrFee(main, MAIN_VAULT, blockNumber, tag, CTXT, fee, USDR_VAULT, "0x", "0x")).wait();

      expect(await spent(env.main, main)).to.equal(true);
      expect(await spent(env.usdr, fee)).to.equal(true);
    });

    it("does not settle next to another payment", async function () {
      const main = shapes.payment(env.main, ofNote(CTXT, ENC));
      const other = shapes.payment(env.main, ofNote(CTXT, ENC));
      const fee = feeLegFor(env, main);
      await expectRevert(dvp, dvp.paymentWithUsdrFee(other, MAIN_VAULT, CTXT, ENC, fee, USDR_VAULT, "0x", "0x"), "CiphertextHashMismatch");
      expect(await spent(env.main, other)).to.equal(false);
      expect(await spent(env.usdr, fee)).to.equal(false);

      const blockNumber = await ethers.provider.getBlockNumber();
      const tag = ethers.utils.keccak256(ethers.utils.toUtf8Bytes("a tag"));
      const channelMain = shapes.payment(env.main, ofChannelPayment(blockNumber, tag, CTXT));
      const channelOther = shapes.payment(env.main, ofChannelPayment(blockNumber, tag, CTXT));
      const channelFee = feeLegFor(env, channelMain);
      await expectRevert(
        dvp,
        dvp.channelPaymentWithUsdrFee(channelOther, MAIN_VAULT, blockNumber, tag, CTXT, channelFee, USDR_VAULT, "0x", "0x"),
        "CiphertextHashMismatch"
      );

      // The same fee leg settles next to its own payment.
      await (await dvp.paymentWithUsdrFee(main, MAIN_VAULT, CTXT, ENC, fee, USDR_VAULT, "0x", "0x")).wait();
      expect(await spent(env.usdr, fee)).to.equal(true);
    });

    it("does not settle when it names no payment: a fee leg bound like a plain payment", async function () {
      const main = shapes.payment(env.main, ofNote(CTXT, ENC));
      const unbound = shapes.usdrFee(env.usdr, USDR_FEE, USDR_TOKEN_ID, ofNote("0x", "0x"));
      await expectRevert(dvp, dvp.paymentWithUsdrFee(main, MAIN_VAULT, CTXT, ENC, unbound, USDR_VAULT, "0x", "0x"), "CiphertextHashMismatch");
      expect(await spent(env.usdr, unbound)).to.equal(false);
    });

    it("does not settle on its own through payment() or paymentWithFee()", async function () {
      const main = shapes.payment(env.main, ofNote(CTXT, ENC));
      const fee = feeLegFor(env, main);
      // A leg bound like a plain payment, as a fee leg was before it named its
      // payment, matches what payment() recomputes, so only the shape stops it.
      const unbound = shapes.usdrFee(env.usdr, USDR_FEE, USDR_TOKEN_ID, ofNote("0x", "0x"));
      for (const leg of [fee, unbound]) {
        await expectRevert(dvp, dvp.payment(leg, USDR_VAULT, "0x", "0x"), "InvalidStatementSize");
        await expectRevert(dvp, dvp.paymentWithFee(leg, USDR_VAULT, USDR_FEE, "0x", "0x"), "InvalidStatementSize");
        expect(await spent(env.usdr, leg)).to.equal(false);
      }
    });

    it("does not settle on its own through the channel calls or the vault's transfers", async function () {
      const main = shapes.payment(env.main, ofNote(CTXT, ENC));
      const fee = feeLegFor(env, main);
      const blockNumber = await ethers.provider.getBlockNumber();
      const tag = ethers.utils.keccak256(ethers.utils.toUtf8Bytes("a tag"));
      // Each channel call takes one other shape.
      await expectRevert(dvp, dvp.channelPayment(fee, USDR_VAULT, blockNumber, tag, "0x"), "InvalidStatementSize");
      await expectRevert(dvp, dvp.channelPaymentWithFee(fee, USDR_VAULT, USDR_FEE, blockNumber, tag, "0x"), "InvalidStatementSize");
      await expectRevert(dvp, dvp.channelPaymentWithRelayerFee(fee, USDR_VAULT, blockNumber, tag, "0x"), "InvalidStatementSize");
      // The vault's own transfers take the shape, but bind ciphertexts with
      // CiphertextHash.ofNotes, which the fee leg's hash never equals.
      const vault = env.usdr.contract;
      await expectRevert(vault, vault.transfer(fee), "CiphertextHashMismatch");
      await expectRevert(vault, vault.transferV2(fee, ["0x", "0x"], ["0x", "0x"]), "CiphertextHashMismatch");
      expect(await spent(env.usdr, fee)).to.equal(false);

      // It still settles next to its payment.
      await (await dvp.paymentWithUsdrFee(main, MAIN_VAULT, CTXT, ENC, fee, USDR_VAULT, "0x", "0x")).wait();
      expect(await spent(env.usdr, fee)).to.equal(true);
    });
  });

  describe("one receipt shape per entry point", function () {
    it("payment() takes only Payment and Payment2in receipts", async function () {
      const ct = ofNote(CTXT, ENC);
      for (const r of [shapes.payment(env.main, ct), shapes.payment2in(env.main, ct)]) {
        await (await dvp.payment(r, MAIN_VAULT, CTXT, ENC)).wait();
        expect(await spent(env.main, r)).to.equal(true);
      }
      for (const r of [
        shapes.paymentFee(env.main, 3, ct),
        shapes.relayerFee(env.main, RELAYER_FEE, ct),
        shapes.usdrFee(env.main, USDR_FEE, USDR_TOKEN_ID, ct),
      ]) {
        await expectRevert(dvp, dvp.payment(r, MAIN_VAULT, CTXT, ENC), "InvalidStatementSize");
        expect(await spent(env.main, r)).to.equal(false);
      }
    });

    it("paymentWithFee() takes only PaymentFee receipts, checking the fee its caller expects", async function () {
      const ct = ofNote(CTXT, ENC);
      const honest = shapes.paymentFee(env.main, 3, ct);
      await expectRevert(dvp, dvp.paymentWithFee(honest, MAIN_VAULT, 4, CTXT, ENC), "EnygmaDvp: fee mismatch");
      await (await dvp.paymentWithFee(honest, MAIN_VAULT, 3, CTXT, ENC)).wait();
      expect(await spent(env.main, honest)).to.equal(true);

      // protocolFee set to the signal each shape had read as its fee: for
      // Payment and Payment2in their StCiphertextHash.
      const payment = shapes.payment(env.main, ct);
      const payment2in = shapes.payment2in(env.main, ct);
      const relayer = shapes.relayerFee(env.main, RELAYER_FEE, ct);
      const usdr = shapes.usdrFee(env.main, USDR_FEE, USDR_TOKEN_ID, ct);
      for (const [r, readAsFee] of [[payment, ct], [payment2in, ct], [relayer, RELAYER_FEE], [usdr, USDR_FEE]]) {
        await expectRevert(dvp, dvp.paymentWithFee(r, MAIN_VAULT, readAsFee, CTXT, ENC), "InvalidStatementSize");
        expect(await spent(env.main, r)).to.equal(false);
      }
    });

    it("paymentWithRelayerFee() takes only PaymentRelayerFee receipts", async function () {
      const ct = ofNote(CTXT, ENC);
      await expectRevert(dvp, dvp.paymentWithRelayerFee(shapes.relayerFee(env.main, RELAYER_FEE + 1, ct), MAIN_VAULT, CTXT, ENC), "InvalidRelayerFee");
      // Its exact shape: 1 input, 3 outputs and PAYMENT_RELAYER_FEE_LEN
      // signals, so a receipt one signal short, or claiming 2 inputs, is
      // refused before its fee is read.
      const short = shapes.relayerFee(env.main, RELAYER_FEE, ct);
      short.statement = short.statement.slice(0, -1);
      const twoInputs = shapes.relayerFee(env.main, RELAYER_FEE, ct);
      twoInputs.numberOfInputs = 2;
      for (const r of [shapes.payment(env.main, ct), shapes.paymentFee(env.main, RELAYER_FEE, ct), short, twoInputs]) {
        await expectRevert(dvp, dvp.paymentWithRelayerFee(r, MAIN_VAULT, CTXT, ENC), "InvalidStatementSize");
      }
      const honest = shapes.relayerFee(env.main, RELAYER_FEE, ct);
      await (await dvp.paymentWithRelayerFee(honest, MAIN_VAULT, CTXT, ENC)).wait();
      expect(await spent(env.main, honest)).to.equal(true);
    });

    it("paymentWithUsdrFee()'s payment is a Payment or Payment2in receipt", async function () {
      const ct = ofNote(CTXT, ENC);
      for (const main of [
        shapes.relayerFee(env.main, RELAYER_FEE, ct),
        shapes.paymentFee(env.main, 3, ct),
        shapes.usdrFee(env.main, USDR_FEE, USDR_TOKEN_ID, ct),
      ]) {
        await expectRevert(
          dvp,
          dvp.paymentWithUsdrFee(main, MAIN_VAULT, CTXT, ENC, feeLegFor(env, main), USDR_VAULT, "0x", "0x"),
          "InvalidStatementSize"
        );
        expect(await spent(env.main, main)).to.equal(false);
      }
      const main = shapes.payment2in(env.main, ct);
      const fee = feeLegFor(env, main);
      await (await dvp.paymentWithUsdrFee(main, MAIN_VAULT, CTXT, ENC, fee, USDR_VAULT, "0x", "0x")).wait();
      expect(await spent(env.main, main)).to.equal(true);
      expect(await spent(env.usdr, fee)).to.equal(true);
    });
  });
});
