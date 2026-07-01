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
    // 7 vkAuctionInit
    // 8 vkAuctionBid
    // 9 vkAuctionPrivateOpening
    // 10 vkAuctionNotWinningBid
    // 11 vkOwnershipErc1155Fungible:   fungible erc1155 1 to 1
    // 12-14: (reserved)
    // 15 vkJoinSplitErc1155FungibleWithAuditor
    // 16 vkOwnershipErc1155NonFungibleWithAuditor
    // ...
    // 23 vkDvPInitiator
    // 24 vkDvPDestination

    constructor() AccessControl() {
        // DVP-1 fix: grant both roles to the deployer at construction time so that:
        //  - DEFAULT_ADMIN_ROLE  lets the deployer call grantRole (e.g. to give EnygmaDvp
        //                        DEFAULT_OWNER_ROLE after it is deployed).
        //  - DEFAULT_OWNER_ROLE  lets the deployer call initializeVerifier directly,
        //                        closing the front-running window that existed when
        //                        initializeVerifier was world-callable.
        _setupRole(DEFAULT_ADMIN_ROLE, msg.sender);
        _setupRole(DEFAULT_OWNER_ROLE, msg.sender);
    }

    function initializeVerifier(
        address groth16Verifier_
    ) external onlyRole(DEFAULT_OWNER_ROLE) returns (bool) {
        // DVP-1 fix: prevent re-initialization — a second call would replace
        // _groth16Verifier with a malicious always-true contract.
        require(_groth16Verifier == address(0), "Verifier: already initialized");
        _groth16Verifier = groth16Verifier_;
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
        // HIGH-3 fix: reject out-of-range indices so an unregistered (all-zero) VK slot
        // can never be used. Without this check an index >= _vkLength silently reads
        // an uninitialised storage slot whose ic array has length 0, which would revert
        // inside GenericGroth16Verifier via "verifier-bad-statement-length" — safe today
        // but fragile; any future refactor could expose proof acceptance against a zero VK.
        require(verificationKeyIndex_ < _vkLength, "Verifier: VK not registered");
        return
            IGenericGroth16Verifier(_groth16Verifier).verify(
                _verificationKeys[verificationKeyIndex_],
                proof_,
                inputs_
            );
    }
}
