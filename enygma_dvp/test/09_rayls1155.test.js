/* global describe it ethers before beforeEach afterEach overwriteArtifact */
const { expect } = require("chai");
const prover = require("../src/core/prover");
const jsUtils = require("../src/core/utils");
const MerkleTree = require("../src/core/merkle");
const myWeb3 = require("../src/web3");
const userActions = require("./../src/core/endpoints/user.js");
const adminActions = require("./../src/core/endpoints/admin.js");
const ethAbi = require('web3-eth-abi');
const web3 = require("web3");
// decodeContractErrorData
let merkleTree1155;

const dvpConf = require("../zkdvp.config.json");
const TREE_DEPTH = dvpConf["tree-depth"];
const testHelpers = require("./testHelpers.js");

let owner;
let users;
let contracts;
let merkleTrees;

let alice;
let bob;

// loading bond data from dvpConfig
var bondsConf = dvpConf["metaTokens"];
// amounts that would be minted for bonds
const mintAmounts = [10, 20, 5];

// price of first bond that has been swapped in this demo
const bondPrice = 100n;

const fungibleToken = { id: 1, };

let randomBondData;

describe("Thorough Rayls1155 coverage testing", () => {

    it(`ZkDvp should initialize properly `, async () => {
        
        let userCount = 2;
        [owner, users, contracts, merkleTrees] = await testHelpers.deployForTest(userCount);

        alice = users[0].wallet;
        bob = users[1].wallet;
        merkleTree1155 = merkleTrees["ERC1155"].tree;

        console.log("RaylsErc1155 test: ZkDvp initialization")

    });

     it(`Minting should work without registering tokens for backward-compatibility. `, async () => {
        // TODO:: this feature must be disabled for production.

        console.log("Minting with null data will register a fungible token with default decimals and maxTotalSupply");
        // console.log("mintErc1155Fungible data = ", data);
        const erc1155Admin = contracts.erc1155.connect(owner);
        const accountAddress = await owner.getAddress();
        var tx =  await erc1155Admin.mint(accountAddress, 1n, 1000n, 0x00);


     });


     it(`Registering RaylsErc1155 NORMAL fungible token and then minting twice. Should pass. `, async () => {
          console.log("Minting RaylsErc1155 NORMAL fungible tokens");
          await contracts.erc1155.registerNewToken(
                0, // type 
                0, // fungiblity
                "Test Token", // name
                "TTT", // symbol
                1234, // offchainId
                10000000, // maxSupply
                10, // decimals
                [], // subTokenIds
                [],  // subTokenValues
                0, // data
                [] // additionalAttrs
            );

          await adminActions.mintErc1155Fungible(owner, alice, 1234n, 1000n, contracts.erc1155);
          await adminActions.mintErc1155Fungible(owner, alice, 1234n, 1000n, contracts.erc1155);
      });

     it(`Registering RaylsErc1155 NORMAL non-fungible token and then minting twice, should fail. `, async () => {
          const tx = await contracts.erc1155.registerNewToken(
                0n, // type 
                1n, // fungiblity
                "Test Token 2", // name
                "TTT2", // symbol
                77777n, // offchainId
                0n, // maxSupply
                0n, // decimals
                [], // subTokenIds
                [],  // subTokenValues
                0, // data
                [] // additionalAttrs
            );

          const eventParams = await testHelpers.parseBondRegisteredEvent(tx);
          console.log(eventParams, null, 4);
          // this should pass
          await adminActions.mintErc1155NonFungible(
                owner, 
                owner, 
                77777n, 
                contracts.erc1155
            );
      
          var hasBeenReverted = false;

          // this should fail and go into catch
          try{
                await adminActions.mintErc1155NonFungible(
                    owner, 
                    owner, 
                    77777n, 
                    contracts.erc1155
                );
          }
          catch(err){
            const errorText = myWeb3.parseCustomError(err.error, "RaylsErc1155");
            console.log("Error Text: ", errorText);
            hasBeenReverted = true;
            if(errorText == "ValueFungibilityInconsistency()" || err.toString().includes("ValueFungibilityInconsistency")){
                console.log("RaylsErc1155 threw exception on double mint of non-fungible token. All good.")

            }else{
                console.log(err.toString());
            }
          }
          if(!hasBeenReverted){

            throw new Error("Rayls1155 let double mint of non-fungible token. Wrong behavior.");
          }
      });


     it(`Batch-Minting RaylsErc1155 NORMAL tokens `, async () => {
            const batchIds =            [ 100n, 101n, 102n, 103n];
            const batchValues =         [1000n,   1n,   1n, 200n];
            await adminActions.mintErc1155Batch(
                owner, 
                owner, 
                batchIds, 
                batchValues,
                0,
                contracts.erc1155
            );
      });



     it(`Batch-Minting RaylsErc1155 NORMAL tokens  with inconsistent fungibilities, should fail.`, async () => {
            const batchIds =            [ 200n, 201n, 202n, 203n];
            const batchValues =         [1000n,   1n,   1n, 200n];
            const batchFungiblities =   [   1n,   1n,   0n,   0n];
            const batchDecimals =       [   20n,   0n,   10n,   20n];
            const batchMaxSupplies =    [   100000n,   100000n,   100000n,   100000n];

            var hasBeenReverted = false;
            try{

                for(var i = 0;i < batchIds.length;i++){
                    await contracts.erc1155.registerNewToken(
                            0n, // type 
                            batchFungiblities[i], // fungiblity
                            "Test Token 2", // name
                            "TTT2", // symbol
                            batchIds[i], // offchainId
                            batchMaxSupplies[i], // maxSupply
                            batchDecimals[i], // decimals
                            [], // subTokenIds
                            [],  // subTokenValues
                            0, // data
                            [] // additionalAttrs
                        );
                }

                await adminActions.mintErc1155Batch(
                    owner, 
                    owner, 
                    batchIds, 
                    batchValues, 
                    0, 
                    contracts.erc1155
                );
            }
            catch(err){
                const errorText = myWeb3.parseCustomError(err.error, "RaylsERC1155");
                console.log("Error Text: ", errorText);
                if(errorText == "ValueFungibilityInconsistency()" || err.toString().includes("ValueFungibilityInconsistency")){
                    console.log("RaylsErc1155 threw exception on mint of non-fungible with value > 1. All good.")
                    hasBeenReverted = true;

                }else{
                    console.log(err.toString());
                }
            }

            if(!hasBeenReverted){
                throw new Error("Rayls1155 let mint of non-fungible with value > 1. Wrong behavior.");
            }
      });



     it(`Minting RaylsErc1155 subTokens `, async () => {

        console.log("Owner Minting RaylsErc1155 subTokens for herself ...");

        const subTokensConf = dvpConf["erc1155Tokens"];

        const subTokenIds = [];
        const subTokenMintAmounts = [];
        for(var i = 0; i< subTokensConf.length; i++){
            console.log(`minting ${subTokensConf[i]["name"]}, ${subTokensConf[i]["mintAmount"]} units`);
            subTokenIds.push(subTokensConf[i].id);
            subTokenMintAmounts.push(subTokensConf[i].mintAmount);
        }

        await adminActions.mintErc1155Batch(
            owner, 
            owner, 
            subTokenIds, 
            subTokenMintAmounts , 
            0, 
            contracts.erc1155
        );

        // checking balances
        const bals = await contracts.erc1155.balanceOfBatch(
            [owner.address, owner.address, owner.address, owner.address], 
            [subTokenIds[0],subTokenIds[1],subTokenIds[2],subTokenIds[3]]);

        console.log("checking balances: ", bals);
      });


     it(`Registering RaylsErc1155 Bond token `, async () => {


        console.log("Registering RaylsErc1155 Bonds ...");


        for(var i = 0; i< bondsConf.length; i++){
            console.log(`Registering ${bondsConf[i]["name"]}, ${bondsConf[i]["symbol"]}`);

            let bondType = 0;
            let bondAdditionalAttrs = [];
            if(bondsConf[i]["type"] == "BOND"){
                bondType = 2;
            }
            const tx = await contracts.erc1155.registerNewToken(
                bondType,
                0n, // bond fungibility
                bondsConf[i]["name"],
                bondsConf[i]["symbol"],
                bondsConf[i]["offchainId"],
                bondsConf[i]["maxSupply"],
                bondsConf[i]["decimals"],
                bondsConf[i]["subTokenIds"], 
                bondsConf[i]["subTokenValues"], 
                0,bondAdditionalAttrs);

            const bondData = await testHelpers.parseBondRegisteredEvent(tx);
            bondsConf[i]["onchainId"] = bondData.onchainId;

            console.log("BondRegistered event: ", JSON.stringify(bondData, null , 4));
        }



      });



      it("Minting Bond with sufficient subTokens.", async () => {

            for(var i = 0; i< bondsConf.length; i++){
                console.log(`Minting ${bondsConf[i]["onchainId"]}, ${mintAmounts[i]} units`);

                await adminActions.mintErc1155Fungible(
                    owner,
                    alice,
                    bondsConf[i]["onchainId"],
                    mintAmounts[i],
                    contracts.erc1155
                );

                const balance = await contracts.erc1155.balanceOf(alice.address, bondsConf[i]["onchainId"]);
            
                console.log(mintAmounts[i], balance);
            }
      });


      it("Minting Bond with insufficient subTokens. Should Fail.", async () => {

            console.log("Registering a bond with random subTokens and \
then trying to mint. Mint should fail because of insufficient subTokens");
            
            const tx = await contracts.erc1155.registerNewToken(
                2n,
                0n, // bond fungibility
                "Test bond",
                "TBT",
                1234n,
                100000000000000000n,
                20n,
                [500n, 501n], // random unused ids that does not exist 
                [100n, 200n], // arbitrary values
                0, // data
                [] 
            );

            randomBondData = await testHelpers.parseBondRegisteredEvent(tx);
            console.log("Random Bond: ", JSON.stringify(randomBondData, null, 4));

            let hasBeenReverted = false;
            try{
                await adminActions.mintErc1155Fungible(
                    owner,
                    alice,
                    randomBondData.onchainId,
                    10000,
                    contracts.erc1155
                );

            }
            catch(err){
                const errorText = myWeb3.parseCustomError(err.error, "RaylsERC1155");
                console.log("Error Text: ", errorText);
                if(err.toString().includes("burn amount exceeds balance")){
                    console.log("RaylsErc1155 threw exception on mint of a bond with insufficient subTokens.")
                    hasBeenReverted = true;

                }else{
                    console.log(err.toString());
                }                
            }

            if(!hasBeenReverted){
                throw new Error("Not reverted properly on insufficient subTokens. Wrong behavior.")
            }
      });

      it("Transferring Bond.", async () => {
            console.log("Trasferring Bond MetaToken");

            console.log("Checking alice and Bob's balance for the bonds before transfer");
            const erc1155Alice = contracts.erc1155.connect(alice);
            for(var i = 0;i< bondsConf.length;i++){
                const balance1 = await contracts.erc1155.balanceOf(alice.address, bondsConf[i].onchainId);
                const balance2 = await contracts.erc1155.balanceOf(bob.address, bondsConf[i].onchainId);

                console.log(`Bond #${i}, Alice balance: ${balance1} , Bob balance ${balance2}`);

                await erc1155Alice.safeTransferFrom(alice.address, bob.address, bondsConf[i].onchainId, 5, 0);

                console.log("Checking alice and Bob's balance for the bonds after transfer");
                const balance3 = await contracts.erc1155.balanceOf(alice.address, bondsConf[i].onchainId);
                const balance4 = await contracts.erc1155.balanceOf(bob.address, bondsConf[i].onchainId);

                console.log(`Bond #${i}, Alice balance: ${balance3} , Bob balance ${balance4}`);

                expect(balance1.toBigInt() + balance2.toBigInt()).to.equal(balance3.toBigInt() + balance4.toBigInt());
                expect(balance1.toBigInt() - balance3.toBigInt()).to.equal(5n);
                expect(balance4.toBigInt() - balance2.toBigInt()).to.equal(5n);

            }
      });

      it("Transferring mixture of tokens by batch-transfer.", async () => {
            const newTokenId = 99999n
            console.log("Minting a fungible token.");

            await adminActions.mintErc1155Fungible(
                owner,
                alice,
                newTokenId,
                10000n,
                contracts.erc1155
            );

            const erc1155Alice = contracts.erc1155.connect(alice);


            console.log("Checking balances before batch-transfer");
            const aliceBalancesBeforeTransfer = [
                                    await contracts.erc1155.balanceOf(alice.address, newTokenId),
                                    await contracts.erc1155.balanceOf(alice.address, bondsConf[0].onchainId)
                                ];

            const bobBalancesBeforeTransfer = [
                                    await contracts.erc1155.balanceOf(bob.address, newTokenId),
                                    await contracts.erc1155.balanceOf(bob.address, bondsConf[0].onchainId)
                                ];

            console.log("Alice balances before batch-transfer");
            console.log(aliceBalancesBeforeTransfer);

            console.log("Bob balances before batch-transfer");
            console.log(bobBalancesBeforeTransfer);

            console.log("Transfering a normal token beside a bond in Batch Transfer");
            await erc1155Alice.safeBatchTransferFrom(
                alice.address, 
                bob.address, 
                [newTokenId, bondsConf[0].onchainId], 
                [5n, 5n], 
                0
            );

            console.log("Checking balances after batch-transfer");
            const aliceBalancesAfterTransfer = [
                                    await contracts.erc1155.balanceOf(alice.address, newTokenId),
                                    await contracts.erc1155.balanceOf(alice.address, bondsConf[0].onchainId)
                                ];

            const bobBalancesAfterTransfer = [
                                    await contracts.erc1155.balanceOf(bob.address, newTokenId),
                                    await contracts.erc1155.balanceOf(bob.address, bondsConf[0].onchainId)
                                ]

            console.log("Alice balances after batch-transfer");
            console.log(aliceBalancesAfterTransfer);

            console.log("Bob balances after batch-transfer");
            console.log(bobBalancesAfterTransfer);

            expect(aliceBalancesBeforeTransfer[0].toBigInt() - aliceBalancesAfterTransfer[0].toBigInt()).to.equal(5n);
            expect(aliceBalancesBeforeTransfer[1].toBigInt() - aliceBalancesAfterTransfer[1].toBigInt()).to.equal(5n);

            expect(bobBalancesAfterTransfer[0].toBigInt() - bobBalancesBeforeTransfer[0].toBigInt()).to.equal(5n);
            expect(bobBalancesAfterTransfer[1].toBigInt() - bobBalancesBeforeTransfer[1].toBigInt()).to.equal(5n);


      });

});
