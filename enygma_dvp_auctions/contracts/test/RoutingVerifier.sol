// SPDX-License-Identifier: BUSL-1.1
pragma solidity ^0.8.20;

import {SnarkProof} from "./MockVerifier.sol";

/// @title RoutingVerifier
/// @notice Test double used to measure how much gas the real verification path
/// of challengeSettlement() needs. Proofs for `routedVk` are run through the
/// real Verifier (so the gas cost is the real one: storage loads of the VK,
/// one ecMul per public input, the pairing), but the pairing result is
/// ignored and reported as valid, which models an HONEST claim. If the real
/// call runs out of gas the call reverts, exactly as an out-of-gas would.
/// Every other vkId is accepted, so the earlier phases can be set up without
/// producing proofs.
contract RoutingVerifier {
    address public immutable real;
    uint256 public immutable routedVk;

    constructor(address real_, uint256 routedVk_) {
        real = real_;
        routedVk = routedVk_;
    }

    function verifyProof(
        uint256 vkId,
        SnarkProof calldata proof,
        uint256[] calldata inputs
    ) external view returns (bool) {
        if (vkId != routedVk) return true;
        (bool ok, ) = real.staticcall(
            abi.encodeWithSignature(
                "verifyProof(uint256,((uint256,uint256),(uint256[2],uint256[2]),(uint256,uint256)),uint256[])",
                vkId, proof, inputs
            )
        );
        require(ok, "RoutingVerifier: real verifier call failed");
        return true;
    }
}
