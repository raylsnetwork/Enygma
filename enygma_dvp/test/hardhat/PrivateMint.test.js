// EnygmaDvp.privateMint inserts a note that no deposit backs. These specs pin
// the rules that keep such notes apart from deposits:
// - a vault takes mints or deposits, never both: setVaultMintable closes the
//   vault to every deposit path for good, and refuses a vault that holds a
//   note while still open to deposits;
// - each commitment is minted once, in any vault, whatever proof comes with it;
// - the mint verifier is a contract, its registration is logged, and a mint
//   with none registered fails with a named error.
// Proofs go through AcceptAllPrivateMintVerifier, so only EnygmaDvp's and the
// vaults' own checks apply.
const { expect } = require("chai");
const { ethers, artifacts } = require("hardhat");
const { POSEIDON_SOURCE } = require("../../scripts/regen_poseidon.js");
const { expectRevert } = require("./expectRevert.js");

const TREE_DEPTH = 8;
const AMOUNT = 1000;
const DEPOSIT = [AMOUNT, 11, 22, 0]; // amount, pkSpend, salt, tokenId
const CAPSULE = "0x" + "ab".repeat(1088);
const NOTE = "0x" + "cd".repeat(48);
const NFT_ID = 7;

// Deploys a Poseidon library from its abi and bytecode: the artifacts the
// compile hook writes for these libraries lack deployedLinkReferences, which
// getContractFactory(name) wants.
async function deployPoseidonLibrary(name) {
  const artifact = await artifacts.readArtifact(`${POSEIDON_SOURCE}:${name}`);
  return (await ethers.getContractFactory(artifact.abi, artifact.bytecode)).deploy();
}

// Deploys contract `name` with args and waits for it.
async function deploy(name, ...args) {
  const contract = await (await ethers.getContractFactory(name)).deploy(...args);
  await contract.deployed();
  return contract;
}

// Sends a transaction and waits for it to be mined.
async function send(tx) {
  return (await tx).wait();
}

// A mint proof for commitment, bound to dvp, with proof words w: the
// accept-all verifier takes any words, so w stands for a different proof.
function mintProof(dvp, commitment, w = 1) {
  return {
    proof: Array(8).fill(w),
    public_signal: [commitment, ethers.BigNumber.from(dvp.address), 0, 9],
  };
}

describe("EnygmaDvp private mint", function () {
  let owner, user, enygma, wrapper, dvp, mintVerifier, token, nft;

  // A fresh EnygmaDvp with an accept-all Verifier and mint verifier, and a
  // token and an NFT that `user` holds and has approved nothing for yet.
  beforeEach(async function () {
    [owner, user, enygma] = await ethers.getSigners();
    const poseidonT3 = await deployPoseidonLibrary("PoseidonT3");
    const poseidonT5 = await deployPoseidonLibrary("PoseidonT5");
    wrapper = await (
      await ethers.getContractFactory("PoseidonWrapper", {
        libraries: {
          [`${POSEIDON_SOURCE}:PoseidonT3`]: poseidonT3.address,
          [`${POSEIDON_SOURCE}:PoseidonT5`]: poseidonT5.address,
        },
      })
    ).deploy();
    const verifier = await deploy("AcceptAllVerifier");
    dvp = await deploy("EnygmaDvp", wrapper.address, verifier.address);
    await send(dvp.initializeDvp(verifier.address));
    mintVerifier = await deploy("AcceptAllPrivateMintVerifier");
    token = await deploy("RaylsERC20", "Token", "TKN");
    await send(token.mint(user.address, 10 * AMOUNT));
    nft = await deploy("RaylsERC721", "Nft", "NFT");
    await send(nft.mint(user.address, NFT_ID));
  });

  // Deploys vault contract `name` over asset and registers it with dvp.
  async function registerVault(name, asset) {
    const vault = await deploy(name, dvp.address);
    await send(dvp.registerVault(vault.address, asset.address, 1, TREE_DEPTH));
    return { vault, id: await vault.getVaultId() };
  }

  // Registers an Erc20CoinVault over token and flags it mintable while it is
  // empty, with the mint verifier registered.
  async function mintVault() {
    const v = await registerVault("Erc20CoinVault", token);
    await send(dvp.registerPrivateMintVerifier(mintVerifier.address));
    await send(dvp.setVaultMintable(v.id, true));
    return v;
  }

  // Approves vault for user's tokens and deposits DEPOSIT into it.
  async function depositInto(vault) {
    await send(token.connect(user).approve(vault.address, 10 * AMOUNT));
    return send(vault.connect(user).deposit(DEPOSIT));
  }

  describe("setVaultMintable", function () {
    it("refuses a vault that holds a deposit, and leaves it a deposit vault", async function () {
      const { vault, id } = await registerVault("Erc20CoinVault", token);
      await depositInto(vault);

      await expectRevert(vault, dvp.setVaultMintable(id, true), "VaultHoldsNotes");

      expect(await dvp.mintableVaults(id)).to.equal(false);
      expect(await vault.depositsClosed()).to.equal(false);
      await send(vault.connect(user).deposit(DEPOSIT));
    });

    it("closes an empty ERC-20 vault to deposit and depositV2 for good", async function () {
      const { vault, id } = await registerVault("Erc20CoinVault", token);
      await send(token.connect(user).approve(vault.address, 10 * AMOUNT));

      await send(dvp.setVaultMintable(id, true));
      expect(await vault.depositsClosed()).to.equal(true);
      await expectRevert(vault, vault.connect(user).deposit(DEPOSIT), "DepositsClosed");
      await expectRevert(vault, vault.connect(user).depositV2(DEPOSIT, CAPSULE, NOTE), "DepositsClosed");

      // Clearing the flag stops mints, and leaves deposits closed.
      await send(dvp.setVaultMintable(id, false));
      expect(await vault.depositsClosed()).to.equal(true);
      await expectRevert(vault, vault.connect(user).deposit(DEPOSIT), "DepositsClosed");
      await expectRevert(vault, vault.connect(user).depositV2(DEPOSIT, CAPSULE, NOTE), "DepositsClosed");
      expect((await token.balanceOf(vault.address)).toNumber()).to.equal(0);
    });

    it("closes an empty ERC-721 vault to deposit", async function () {
      const { vault, id } = await registerVault("Erc721CoinVault", nft);
      await send(nft.connect(user).approve(vault.address, NFT_ID));

      await send(dvp.setVaultMintable(id, true));

      await expectRevert(vault, vault.connect(user).deposit([NFT_ID, 11, 22]), "DepositsClosed");
      expect(await nft.ownerOf(NFT_ID)).to.equal(user.address);
    });

    it("closes an empty Enygma vault to depositThroughEnygma", async function () {
      const { vault, id } = await registerVault("EnygmaErc20CoinVault", token);
      await send(vault.addEnygma(enygma.address));

      await send(dvp.setVaultMintable(id, true));

      await expectRevert(vault, vault.connect(enygma).depositThroughEnygma([AMOUNT, 11]), "DepositsClosed");
    });

    it("flags a mint-only vault again after clearing, though it holds minted notes", async function () {
      const { vault, id } = await mintVault();
      await send(dvp.privateMint(id, 101, mintProof(dvp, 101)));
      await send(dvp.setVaultMintable(id, false));
      await expectRevert(dvp, dvp.privateMint(id, 102, mintProof(dvp, 102)), "VaultNotMintable");

      await send(dvp.setVaultMintable(id, true));

      await send(dvp.privateMint(id, 102, mintProof(dvp, 102)));
      expect(await vault.depositsClosed()).to.equal(true);
    });

    it("lets only EnygmaDvp close a vault's deposits", async function () {
      const { vault } = await registerVault("Erc20CoinVault", token);

      await expectRevert(vault, vault.connect(owner).closeDeposits(), "AccessControl: account");

      expect(await vault.depositsClosed()).to.equal(false);
    });
  });

  describe("privateMint", function () {
    it("mints each commitment once, whatever the proof and the vault", async function () {
      const a = await mintVault();
      const b = await registerVault("Erc20CoinVault", token);
      await send(dvp.setVaultMintable(b.id, true));
      await send(dvp.privateMint(a.id, 4242, mintProof(dvp, 4242)));
      const rootA = (await a.vault.merkleRoot()).toString();
      const rootB = (await b.vault.merkleRoot()).toString();

      // The same proof, another proof of the same commitment, another vault.
      await expectRevert(dvp, dvp.privateMint(a.id, 4242, mintProof(dvp, 4242)), "CommitmentAlreadyMinted");
      await expectRevert(dvp, dvp.privateMint(a.id, 4242, mintProof(dvp, 4242, 2)), "CommitmentAlreadyMinted");
      await expectRevert(dvp, dvp.privateMint(b.id, 4242, mintProof(dvp, 4242)), "CommitmentAlreadyMinted");

      expect((await a.vault.merkleRoot()).toString()).to.equal(rootA);
      expect((await b.vault.merkleRoot()).toString()).to.equal(rootB);
      await send(dvp.privateMint(b.id, 4343, mintProof(dvp, 4343)));
      expect((await b.vault.merkleRoot()).toString()).to.not.equal(rootB);
    });

    it("refuses a mint while no verifier is registered", async function () {
      const { id } = await registerVault("Erc20CoinVault", token);
      await send(dvp.setVaultMintable(id, true));

      await expectRevert(dvp, dvp.privateMint(id, 4242, mintProof(dvp, 4242)), "PrivateMintVerifierNotRegistered");
    });
  });

  describe("registerPrivateMintVerifier", function () {
    it("refuses an address with no code", async function () {
      for (const address of [user.address, ethers.constants.AddressZero]) {
        await expectRevert(dvp, dvp.registerPrivateMintVerifier(address), "PrivateMintVerifierHasNoCode");
      }
    });

    it("logs the verifier it registers", async function () {
      const receipt = await send(dvp.registerPrivateMintVerifier(mintVerifier.address));

      const logged = receipt.events.filter((e) => e.event === "PrivateMintVerifierRegistered").map((e) => e.args.verifier);
      expect(logged, "PrivateMintVerifierRegistered events").to.deep.equal([mintVerifier.address]);
    });
  });
});
