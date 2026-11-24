// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;

import {IEnygmaDvp} from "../../interfaces/IEnygmaDvp.sol";
import {IPoseidonWrapper} from "../../interfaces/IPoseidonWrapper.sol";
import {AbstractCoinVault} from "./AbstractCoinVault.sol";

// Retired (docs/adr/0003-retire-the-legacy-erc1155-vault.md). Every receipt
// this vault took came from a legacy ERC1155 circuit (VK slots 2, 3, 4, 5,
// 15, 16) with no source, prover route or keygen in the repo, and its checks
// on them were weaker than the other vaults' (#417). The vault stays deployed
// as vault 2 so the vault ids keep their numbering, but takes no tokens and
// settles no receipt: every entry point that did reverts
// Erc1155VaultRetired. It is no ERC1155 receiver either, so a token sent to
// it directly with safeTransferFrom is refused rather than stranded. Its DvP
// hooks are AbstractCoinVault's, which refuse every DvP leg.
contract Erc1155CoinVault is AbstractCoinVault {
    error Erc1155VaultRetired();

    constructor(
        address zkDvpContractAddress
    ) AbstractCoinVault(zkDvpContractAddress) {
        _name = "ZkDvp - ERC1155 Coin Vault";
    }

    function deposit(uint256[] memory) public pure override returns (bool) {
        revert Erc1155VaultRetired();
    }

    function transfer(
        IEnygmaDvp.ProofReceipt memory
    ) public pure override returns (bool) {
        revert Erc1155VaultRetired();
    }

    function withdraw(
        uint256[] memory,
        address,
        IEnygmaDvp.ProofReceipt memory
    ) public pure override returns (bool) {
        revert Erc1155VaultRetired();
    }

    function verifyOwnership(
        uint256[] memory,
        IEnygmaDvp.ProofReceipt memory
    ) public pure override returns (bool) {
        revert Erc1155VaultRetired();
    }

    // EnygmaDvp's payment entry points call this on the vault they settle
    // against before nullifying, so it refuses too.
    function checkReceiptConditions(
        IEnygmaDvp.ProofReceipt memory
    ) public pure override returns (bool) {
        revert Erc1155VaultRetired();
    }

    // Poseidon(Poseidon(asset contract, tokenId), amountOrOne), as before.
    function generateUniqueId(
        uint256[] memory params
    ) public view override returns (uint256) {
        uint256 amountOrOne = params[0];
        uint256 tokenId = params[1];

        uint256 uid1 = IPoseidonWrapper(_hashContractAddress).poseidon(
            [uint256(uint160(_assetContractAddress)), tokenId]
        );
        return IPoseidonWrapper(_hashContractAddress).poseidon(
            [uid1, amountOrOne]
        );
    }
}
