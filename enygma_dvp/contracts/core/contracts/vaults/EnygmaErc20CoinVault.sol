// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;
// pragma abicoder v2;

import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";

import {IEnygmaDvp} from "../../interfaces/IEnygmaDvp.sol";
import {IPoseidonWrapper} from "../../interfaces/IPoseidonWrapper.sol";
import {AbstractCoinVault} from "./AbstractCoinVault.sol";
import {
    IEnygmaErc20CoinVault
} from "../../interfaces/vaults/IEnygmaErc20CoinVault.sol";

contract EnygmaErc20CoinVault is AbstractCoinVault, IEnygmaErc20CoinVault {
    ///////////////////////////////////////////////
    //              Constants
    //////////////////////////////////////////////

    bytes32 public constant DEFAULT_ENYGMA_ROLE =
        keccak256(abi.encodePacked("EnygmaRole"));

    address private _enygmaAddress;

    // Unspent count per commitment of the notes depositThroughEnygma inserted;
    // withdrawThroughEnygma spends from it.
    mapping(uint256 => uint256) private _unspentEnygmaNotes;
    ///////////////////////////////////////////////
    //              Constructor
    //////////////////////////////////////////////

    constructor(
        address zkDvpContractAddress
    ) AbstractCoinVault(zkDvpContractAddress) {
        _name = "ZkDvp - ERC20 Coin Vault";
    }

    function addEnygma(
        address enygmaAddress
    ) public onlyRole(DEFAULT_OWNER_ROLE) returns (bool) {
        _assetContractAddress = enygmaAddress;
        _enygmaAddress = enygmaAddress;
        _setupRole(DEFAULT_ENYGMA_ROLE, _enygmaAddress);
        return true;
    }

    function depositThroughEnygma(
        uint256[] memory depositParams
    ) public override onlyRole(DEFAULT_ENYGMA_ROLE) returns (bool, uint256) {
        uint256 amount = depositParams[0];
        uint256 publicKey = depositParams[1];

        uint256[] memory assetParams = new uint256[](1);
        assetParams[0] = amount;

        uint256 uid = generateUniqueId(assetParams);

        // Generating the commitment based on the ERC20 uniqueId and the publickey
        uint256 commitment = IPoseidonWrapper(_hashContractAddress).poseidon(
            [uid, publicKey]
        );

        uint256[] memory commitments = new uint256[](1);
        commitments[0] = commitment;

        insertLeaves(commitments);
        _unspentEnygmaNotes[commitment] += 1;

        emit Commitment(_vaultId, commitment);

        return (true, commitment);
    }

    // Spends one Enygma note: a commitment depositThroughEnygma inserted and
    // no earlier call spent. Enygma's deposit proof shows its submitter owns
    // the note (knows sk with publicKey = Poseidon(sk)) and credits exactly the
    // note's amount, so the note it names is the value this call consumes.
    // A commitment inserted n times (the same amount and key, n withdrawals)
    // can be spent n times.
    function withdrawThroughEnygma(
        uint256 commitment
    ) public override onlyRole(DEFAULT_ENYGMA_ROLE) returns (bool) {
        if (_unspentEnygmaNotes[commitment] == 0) {
            revert UnknownEnygmaNote();
        }
        _unspentEnygmaNotes[commitment] -= 1;
        emit EnygmaNoteSpent(_vaultId, commitment);
        return true;
    }

    // The number of unspent Enygma notes with this commitment.
    function unspentEnygmaNotes(
        uint256 commitment
    ) public view returns (uint256) {
        return _unspentEnygmaNotes[commitment];
    }

    // Standards that are currently supported: ERC20, ERC721, ERC1155
    function deposit(
        uint256[] memory params
    ) public override onlyRole(DEFAULT_ENYGMA_ROLE) returns (bool) {
        revert NotImplemented();
    }

    function transfer(
        IEnygmaDvp.ProofReceipt memory receipt
    ) public override returns (bool) {
        revert NotImplemented();
    }

    function withdraw(
        uint256[] memory withdrawParams,
        address recipient,
        IEnygmaDvp.ProofReceipt memory receipt
    ) public override returns (bool) {
        revert NotImplemented();

        return true;
    }

    function verifyOwnership(
        uint256[] memory params,
        IEnygmaDvp.ProofReceipt memory receipt
    ) public override returns (bool) {
        revert NotImplemented();
    }

    ///////////////////////////////////////////////
    //       Generic functions
    //////////////////////////////////////////////
    function generateUniqueId(
        uint256[] memory params
    ) public view override returns (uint256) {
        uint256 amount = params[0];
        return
            IPoseidonWrapper(_hashContractAddress).poseidon(
                [uint256(uint160(_assetContractAddress)), amount]
            );
    }

    // This vault settles no receipts: its notes are spent only through
    // withdrawThroughEnygma, by the Enygma deposit that redeems them. EnygmaDvp's
    // payment paths check a receipt here before nullifying or inserting
    // through a vault, so refusing every receipt keeps a note from being spent
    // there as well as redeemed in Enygma.
    function checkReceiptConditions(
        IEnygmaDvp.ProofReceipt memory
    ) public pure override returns (bool) {
        revert NotImplemented();
    }
}
