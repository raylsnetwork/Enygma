const { expect } = require("chai");
const poseidonGenContract = require("circomlibjs/src/poseidon_gencontract");
const prover = require("../src/core/prover");
const jsUtils = require("../src/core/utils");
const MerkleTree = require("../src/core/merkle");
const { getVerificationKeys } = require("../src/core/dvpSnarks");
const adminActions = require("./../src/core/endpoints/admin.js");
const userActions = require("./../src/core/endpoints/user.js");
const relayerActions = require("./../src/core/endpoints/relayer.js");
const crypto = require("crypto")
const dvpConf = require("../zkdvp.config.json");
const TREE_DEPTH = dvpConf["tree-depth"];
const testHelpers = require("./testHelpers.js");

let users;
let contracts;
let merkleTree721;
let merkleTree20;

let alice = {};
let bob = {};
let relayer = {};
let owner = {};

let inKeys = [];
let nftPrice = 0n;

let numberOfBobsCoins = 0;
let bobTotalDeposit = 0n;



describe("JoinSplitErc20_10_2 Test", () => {

    it(`ZkDvp should initialize properly `, async () => {
        
        let userCount = 2;
        [admin, users, contracts, merkleTrees] = await testHelpers.deployForTest(userCount);

        alice.wallet = users[0].wallet;
        bob.wallet =   users[1].wallet;
        relayer.wallet = admin;
        owner.wallet = admin;

        alice.calls = {};
        bob.calls = {};
        relayer.calls = {};
        owner.calls = {};

        alice.knows = {};
        bob.knows = {};

        alice.coins = [];
        bob.coins = [];

        relayer.calls.zkdvp = contracts.zkdvp.connect(relayer.wallet);
        relayer.calls.erc20Vault = contracts.erc20CoinVault.connect(relayer.wallet);
        relayer.calls.erc721Vault = contracts.erc721CoinVault.connect(relayer.wallet);

        alice.calls.zkdvp = contracts.zkdvp.connect(alice.wallet);
        alice.calls.erc721 = contracts.erc721.connect(alice.wallet);
        alice.calls.erc721Vault = contracts.erc721CoinVault.connect(alice.wallet);

        bob.calls.zkdvp = contracts.zkdvp.connect(bob.wallet);
        bob.calls.erc20 = contracts.erc20.connect(bob.wallet);
        bob.calls.erc20Vault = contracts.erc20CoinVault.connect(bob.wallet);
        
        owner.calls.erc721 = contracts.erc721.connect(owner.wallet);
        owner.calls.erc20 = contracts.erc20.connect(owner.wallet);

        merkleTree721 = merkleTrees["ERC721"].tree;
        merkleTree20 = merkleTrees["ERC20"].tree;

        console.log("JoinSplitErc20_10_2 Test: ZkDvp initialization")

  });

  it("Alice should deposit an ERC721 token.", async () => {

        let NFT_ID = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(3))) % jsUtils.SNARK_SCALAR_FIELD;

        let aliceErc721Key = jsUtils.newKeyPair();

        await owner.calls.erc721.mint(alice.wallet.address, NFT_ID);

        // Approve ZkDvp as an operator for Alice's NFT
        await alice.calls.erc721.approve(contracts.erc721CoinVault.address, NFT_ID);
        // Deposit Alice's NFT into ZkDvp
        let tx = await alice.calls.erc721Vault.deposit(
          [NFT_ID,
          aliceErc721Key.publicKey]
        );

        let cmt = await testHelpers.getCommitmentFromTx(tx);

        merkleTree721.insertLeaves([cmt]);
        let proof0 = merkleTree721.generateProof(cmt);
        alice.coins.push({
                            "treeId": 0, //ERC721 TreeId
                            "value":NFT_ID, 
                            "tokenAddress": contracts.erc721.address,
                            "key":aliceErc721Key, 
                            "commitment": cmt, 
                            "proof":proof0, 
                            "root":merkleTree721.root, 
                            "treeNumber":merkleTree721.lastTreeNumber
                        });
  });

  it("Bob deposits random number (between 2 and 10 inclusive) of ERC20 coins.", async () => {

        numberOfBobsCoins = Math.floor(Math.random() * (10 - 2) + 2);


        var bobCoinAmounts = [];
        bobTotalDeposit = 0n;

        for(var j = 0; j < numberOfBobsCoins; j++){
          
          const newAmount = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(3)));
          bobTotalDeposit += newAmount;
          bobCoinAmounts.push(newAmount);
        }

        console.log(`Minting total value of ${bobTotalDeposit} erc20 tokens for Bob.`);

        // minting ERC20 tokens for Bob
        let tx = await owner.calls.erc20.mint(bob.wallet.address, bobTotalDeposit);
        await tx.wait();

        // Approve ZkDvp to transfer tokens
        await bob.calls.erc20.approve(contracts.erc20CoinVault.address, bobTotalDeposit);

        console.log(`Depositing ${numberOfBobsCoins} erc20 coins for Bob with amounts: ${bobCoinAmounts}`);

        for(var i = 0; i< numberOfBobsCoins; i++){
            // creating depositKeys for Bob to Deposit Erc20 tokens to ZkDvp
            let bobErc20Key = jsUtils.newKeyPair();

            // adding new erc20 coin to bob's erc20 coin list

            tx = await bob.calls.erc20Vault.deposit(
              [bobCoinAmounts[i],
              bobErc20Key.publicKey]
            );

            const bobCmt = await testHelpers.getCommitmentFromTx(tx);
            merkleTree20.insertLeaves([bobCmt]);
            let bobProof = merkleTree20.generateProof(bobCmt);

            bob.coins.push({    "treeId": 1,
                                "value":bobCoinAmounts[i], 
                                "tokenAddress": contracts.erc20.address,
                                "key":bobErc20Key, 
                                "commitment":bobCmt, 
                                "proof":bobProof, 
                                "root":merkleTree20.root, 
                                "treeNumber":merkleTree20.lastTreeNumber
                            });

        }

        console.log("----------------------------")
        console.log("Alice.coins:\n", JSON.stringify(alice.coins, null, 4)  + "\n");
        console.log("Bob.coins:\n", JSON.stringify(bob.coins, null, 4) + "\n");

  });


  it("Bob and Alice swap.", async () => {



      NFT_ID = alice.coins[0]["value"];
      nftKeyDeposit =  alice.coins[0]["key"];
      treeNumber0 =  alice.coins[0]["treeNumber"];
      proof0 = alice.coins[0]["proof"];
      root0 = alice.coins[0]["root"];


      console.log("Alice sends bob the ERC721 information");
      bob.knows["nftId"] = NFT_ID;
      bob.knows["erc721Address"] = contracts.erc721.address;
      

      console.log("Alice and Bob agree upon a price.");
      nftPrice = jsUtils.buffer2BigInt(Buffer.from(crypto.randomBytes(3))) % jsUtils.SNARK_SCALAR_FIELD;
      bob.knows["nftPrice"] = nftPrice;
      alice.knows["nftPrice"] = nftPrice;

      jsInputs = {
          "inKeys": [],     
          "inValues": [],      
          "outKeys": [] ,    
          "outValues": [],     
          "treeNumbers": [],     
          "roots": [],     
          "nullifiers": [],     
          "outCommitments": [],  
          "proofs": []     
      };

      console.log("--------------------------");
      console.log("Generating 10-2 JoinSplit");
      console.log("bob's coins' length: ", bob.coins.length);

      for(var i = 0; i < bob.coins.length; i++){
        jsInputs.inKeys.push(bob.coins[i].key);
        jsInputs.inValues.push(bob.coins[i].value);
        jsInputs.proofs.push(bob.coins[i].proof);
        jsInputs.treeNumbers.push(bob.coins[i].treeNumber);
      }


      console.log("jsInputs: ", JSON.stringify(jsInputs, null, 4));
      // Alice generates NFT commitment for Bob
      let uid = jsUtils.erc721UniqueId(contracts.erc721.address, NFT_ID);

      // Bob generates a public key to receive the NFT
      let bobNFTKey = jsUtils.newKeyPair();
      // Bob generates a public to receive the change
      let bobChangeKey = jsUtils.newKeyPair();
      console.log("Bob shared his receiving publicKey with Alice to receive the coin with excess amount.");
      alice.knows["bobReceivingKey"] = {publicKey: bobChangeKey.publicKey};
      // Alice generates a public key to receive the payment

      let alicePaymentKey = jsUtils.newKeyPair();
      console.log("Alice shared her receiving publicKey with Bob to receive the new ERC721 coin");
      bob.knows["aliceReceivingKey"] = {publicKey: alicePaymentKey.publicKey};

      // nftCommitment will be used as a massage by Bob
      let nftCommitment = jsUtils.getCommitment(uid, bobNFTKey.publicKey);

      // Bob generates payment commitment for Alice
      let paymentAmount = nftPrice;
      let excessAmount = bobTotalDeposit - paymentAmount;

      // creating unique erc20Commitment
      let erc20Uid = jsUtils.erc20UniqueId(
        contracts.erc20.address,
        paymentAmount,
      );

      // paymentCommitment will be used as a massage by Alice
      let paymentCommitment = jsUtils.getCommitment(
        erc20Uid,
        alicePaymentKey.publicKey,
      );

        const ownParams = await prover.prove(
            "OwnershipErc721",
            {
                message: paymentCommitment,
                values: [NFT_ID],
                keysIn: [nftKeyDeposit],
                keysOut: [bobNFTKey],
                merkleProofs: [proof0],
                treeNumbers: [treeNumber0],
                erc721ContractAddress: contracts.erc721.address
            }
        );

        const jsParams = await prover.prove(
            "JoinSplitErc20_10_2",
            {
                message: nftCommitment,
                valuesIn: jsInputs.inValues,
                keysIn: jsInputs.inKeys,
                valuesOut: [paymentAmount, excessAmount],
                keysOut: [alicePaymentKey, bobChangeKey],
                merkleProofs: jsInputs.proofs,
                treeNumbers: jsInputs.treeNumbers,
                erc20ContractAddress: contracts.erc20.address,
            }
        );

      console.log(jsParams);

      console.log("SWAPPING");
      // A relayer forwards both transactions to ZkDvp
      await relayer.calls.zkdvp.submitPartialSettlement(ownParams, 1, 1);
      tx = await relayer.calls.zkdvp.submitPartialSettlement(jsParams, 0, 0);
      
      // TODO:: fix getCommitmentFromTxIndirect() to not be dependant on the order

      const [eventCommitments, eventNullifiers] = await testHelpers.getCommitmentFromTxIndirect(tx, 
          [
            contracts.erc20CoinVault.address,
            contracts.erc721CoinVault.address
          ]
        );
      console.log("Done swap");
      console.log(eventCommitments);

      merkleTree20.insertLeaves([eventCommitments[0], eventCommitments[1]]);
      alice.coins.push({"commitment":eventCommitments[0],
                        "key":alicePaymentKey,
                       "treeNumber":merkleTree20.lastTreeNumber,
                       "proof":merkleTree20.generateProof(eventCommitments[0]),
                       "root": merkleTree20.root});
      bob.coins.push({"commitment":eventCommitments[1],
                        "key":bobChangeKey,
                       "treeNumber":merkleTree20.lastTreeNumber,
                       "proof":merkleTree20.generateProof(eventCommitments[1]),
                       "root": merkleTree20.root});

      merkleTree721.insertLeaves([eventCommitments[2]]);
     bob.coins.push({"commitment":eventCommitments[2],
                      "key":bobNFTKey,
                     "treeNumber":merkleTree721.lastTreeNumber,
                     "proof":merkleTree721.generateProof(eventCommitments[2]),
                     "root": merkleTree721.root});

  });


  it("Alice withdraws ERC20 output coin.", async () => {

    const dummyKey = jsUtils.newKeyPair();
    // Bob generates a tx to send payment to Alice
    const aliceJS = await prover.prove(
        "JoinSplitErc20",
        {
            message: 0n,
            valuesIn: [nftPrice, 0n],
            keysIn: [alice.coins[1].key, dummyKey],
            valuesOut: [nftPrice, 0n],
            keysOut: [{ publicKey: BigInt(alice.wallet.address) }, dummyKey],
            merkleProofs: [alice.coins[1]["proof"], {"root": 0n}],
            treeNumbers: [alice.coins[1]["treeNumber"], 0n],
            erc20ContractAddress: contracts.erc20.address,
        }
    );

    const oldBalance = (
      await contracts.erc20.balanceOf(alice.wallet.address)
    ).toBigInt();


    await relayer.calls.erc20Vault.withdraw([nftPrice], alice.wallet.address, aliceJS);
    const newBalance = (
      await contracts.erc20.balanceOf(alice.wallet.address)
    ).toBigInt();
    await expect(oldBalance + nftPrice).to.equal(newBalance);

  });


  it("Bob withdraws bought ERC721.", async () => {

    const bobNftId = bob.knows.nftId;
    console.log("Bob withdraws NFT");
    // Bob withdraws his NFT from ZkDvp
    const uid = jsUtils.erc721UniqueId(bob.knows.erc721Address, bobNftId);

    const bobNftCoinIndex = numberOfBobsCoins + 1;
    const bobNFT = await prover.prove(
        "OwnershipErc721",
        {
            message: 0n,
            values: [NFT_ID],
            keysIn: [bob.coins[bobNftCoinIndex].key],
            keysOut: [{"publicKey": BigInt(bob.wallet.address)}],
            treeNumbers: [bob.coins[bobNftCoinIndex].treeNumber],
            merkleProofs: [bob.coins[bobNftCoinIndex].proof],
            erc721ContractAddress: contracts.erc721.address
        }
    );
    // TX sent by a relayer
    await relayer.calls.erc721Vault.withdraw([bobNftId], bob.wallet.address, bobNFT);
    const res = await contracts.erc721.ownerOf(bobNftId);
    expect(res).to.equal(bob.wallet.address);

  });


  it("Alice withdraws excess ERC20 coin.", async () => {
    const bobNftCoinIndex = numberOfBobsCoins;
    const dummyKey = {"privateKey": 0n, "publicKey":0n};
    const excessAmount = bobTotalDeposit - bob.knows.nftPrice;

    const bobJs = await prover.prove(
        "JoinSplitErc20",
        {
            message: 0n,
            valuesIn: [excessAmount, 0n],
            keysIn: [bob.coins[bobNftCoinIndex].key, dummyKey],
            valuesOut: [excessAmount, 0n],
            keysOut: [{ publicKey: BigInt(bob.wallet.address) }, dummyKey],
            merkleProofs: [bob.coins[bobNftCoinIndex].proof, {"root": 0n}],
            treeNumbers: [bob.coins[bobNftCoinIndex].treeNumber, 0n],
            erc20ContractAddress: contracts.erc20.address,
        }
    );

    const oldBalance = (
      await contracts.erc20.balanceOf(bob.wallet.address)
    ).toBigInt();


    await relayer.calls.erc20Vault.withdraw([excessAmount], bob.wallet.address, bobJs);
    const newBalance = (
      await contracts.erc20.balanceOf(bob.wallet.address)
    ).toBigInt();
    await expect(oldBalance + excessAmount).to.equal(newBalance);

  });


});
