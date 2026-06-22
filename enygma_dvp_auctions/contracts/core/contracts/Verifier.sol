// SPDX-License-Identifier: BUSL-1.1
pragma solidity ^0.8.20;

import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";

// ---------------------------------------------------------------------------
// Shared BN254 types (same tuple shapes as GenericGroth16Verifier and the
// inline structs in EnygmaAuction.sol; ABI encoding is purely structural).
// ---------------------------------------------------------------------------

struct G1Point { uint256 x; uint256 y; }
struct G2Point { uint256[2] x; uint256[2] y; }

struct SnarkProof {
    G1Point a;
    G2Point b;
    G1Point c;
}

struct VerifyingKey {
    G1Point   alpha1;
    G2Point   beta2;
    G2Point   gamma2;
    G2Point   delta2;
    G1Point[] ic;
}

interface IGenericGroth16Verifier {
    function verify(
        VerifyingKey memory _vk,
        SnarkProof   memory _proof,
        uint256[]    memory _input
    ) external view returns (bool);
}

/// @title Verifier
/// @notice VK registry + Groth16 proof dispatcher for the auction circuits.
///         VKs are registered in insertion order; the index assigned to each
///         circuit is its slot in the EnygmaAuction constructor:
///           0 = AuctionLock   (VK_LOCK)
///           1 = AuctionBid    (VK_BID)
///           2 = AuctionBatch  (VK_BATCH)
///           3 = AuctionFinal  (VK_FINAL)
///           4 = AuctionRevert (VK_REVERT)
contract Verifier is AccessControl {
    uint256 private constant MAX_CIRCUITS = 1000;

    bytes32 public constant OWNER_ROLE = keccak256("OWNER_ROLE");

    VerifyingKey[MAX_CIRCUITS] private _vks;
    uint256 private _vkLength;

    address private _g16Verifier;

    error MaxCircuitsExceeded();

    constructor() AccessControl() {
        _grantRole(OWNER_ROLE, msg.sender);
    }

    function initializeVerifier(address g16Verifier_) external onlyRole(OWNER_ROLE) returns (bool) {
        _g16Verifier = g16Verifier_;
        return true;
    }

    /// @notice Append a new VK. Called once per circuit in index order.
    function addVerificationKey(VerifyingKey memory vKey_) external onlyRole(OWNER_ROLE) returns (bool) {
        if (_vkLength == MAX_CIRCUITS) revert MaxCircuitsExceeded();
        uint256 i = _vkLength;

        _vks[i].alpha1.x = vKey_.alpha1.x;
        _vks[i].alpha1.y = vKey_.alpha1.y;

        for (uint256 j = 0; j < 2; j++) {
            _vks[i].beta2.x[j]  = vKey_.beta2.x[j];
            _vks[i].beta2.y[j]  = vKey_.beta2.y[j];
            _vks[i].gamma2.x[j] = vKey_.gamma2.x[j];
            _vks[i].gamma2.y[j] = vKey_.gamma2.y[j];
            _vks[i].delta2.x[j] = vKey_.delta2.x[j];
            _vks[i].delta2.y[j] = vKey_.delta2.y[j];
        }
        for (uint256 j = 0; j < vKey_.ic.length; j++) {
            _vks[i].ic.push(G1Point(vKey_.ic[j].x, vKey_.ic[j].y));
        }
        _vkLength++;
        return true;
    }

    function verifyProof(
        uint256    vkId_,
        SnarkProof memory proof_,
        uint256[]  memory inputs_
    ) external view returns (bool) {
        return IGenericGroth16Verifier(_g16Verifier).verify(_vks[vkId_], proof_, inputs_);
    }

    function vkLength() external view returns (uint256) { return _vkLength; }
}
