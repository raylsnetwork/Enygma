// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;
// pragma abicoder v2;

import {IEnygmaDvp} from "../interfaces/IEnygmaDvp.sol";
import {
    IGenericGroth16Verifier
} from "../interfaces/IGenericGroth16Verifier.sol";
import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";
import {IVerifier} from "../interfaces/IVerifier.sol";

contract Verifier is IVerifier, AccessControl {
    uint256 constant MAX_NUMBER_OF_CIRCUITS = 1000;

    bytes32 public constant DEFAULT_OWNER_ROLE =
        keccak256(abi.encodePacked("ownerRole"));

    IEnygmaDvp.VerifyingKey[MAX_NUMBER_OF_CIRCUITS] private _verificationKeys;
    uint256 _vkLength = 0;

    address private _groth16Verifier;

    // circuit indices:
    // 0 vkJS:                          ERC20JoinSplit
    // 1 vkOwn:                         Erc721Ownership
    // 2 vkOwnErc1155NonFungible:       Erc1155 1-in 1-out for nonFungible ERC1155
    // 3 vkOwnErc1155Fungible:          Erc1155 1-in 1-out for fungible ERC1155
    // 4 vkJSErc1155:                   Fungible Erc1155 two input / joinSplit
    // 5 vkBatchErc1155Fungible:        Fungible Erc1155 10 input / batch logic
    // 6 vkJS102:                       Erc20 10 input / JoinSplitLogic
    // 7 vkAuctionInit,
    // 8 vkAuctionBid,
    // 9 vkAuctionPrivateOpening,
    // 10 vkAuctionNotWinningBid
    // 11 vkOwnershipErc1155Fungible:   fungible erc1155 1 to 1
    // 12 vkBrokerRegistration:         2 input broker registration
    // 13 vkLegitBroker
    // 14 vkJoinSplitErc20WithBrokerV1
    // 15 vkJoinSplitErc1155WithBrokerV1

    constructor() AccessControl() {}

    function initializeVerifier(
        address groth16Verifier_
    ) public returns (bool) {
        _groth16Verifier = groth16Verifier_;
        _setupRole(DEFAULT_OWNER_ROLE, msg.sender);
        return true;
    }

    function addVerificationKey(
        IEnygmaDvp.VerifyingKey memory vKey_
    ) public onlyRole(DEFAULT_OWNER_ROLE) returns (bool) {
        if (_vkLength == MAX_NUMBER_OF_CIRCUITS) {
            revert MaxNumberOfCircuitsExceeded();
        }

        // adding the key at the last index
        uint i = _vkLength;

        _verificationKeys[i].alpha1.x = vKey_.alpha1.x;
        _verificationKeys[i].alpha1.y = vKey_.alpha1.y;

        for (uint256 j = 0; j < 2; j++) {
            // Beta
            _verificationKeys[i].beta2.x[j] = vKey_.beta2.x[j];
            _verificationKeys[i].beta2.y[j] = vKey_.beta2.y[j];
            // Gamma
            _verificationKeys[i].gamma2.x[j] = vKey_.gamma2.x[j];
            _verificationKeys[i].gamma2.y[j] = vKey_.gamma2.y[j];
            // Delta
            _verificationKeys[i].delta2.x[j] = vKey_.delta2.x[j];
            _verificationKeys[i].delta2.y[j] = vKey_.delta2.y[j];
        }
        for (uint8 j = 0; j < vKey_.ic.length; j++) {
            // IC
            _verificationKeys[i].ic.push(
                IEnygmaDvp.G1Point(vKey_.ic[j].x, vKey_.ic[j].y)
            );
        }

        _vkLength++;
    }

    function verifyProof(
        uint256 verificationKeyIndex_,
        IEnygmaDvp.SnarkProof memory proof_,
        uint256[] memory inputs_
    ) public view returns (bool) {
        return
            IGenericGroth16Verifier(_groth16Verifier).verify(
                _verificationKeys[verificationKeyIndex_],
                proof_,
                inputs_
            );
    }
}
