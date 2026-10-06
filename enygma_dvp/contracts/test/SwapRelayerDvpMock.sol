// SPDX-License-Identifier: BUSL-1.1
pragma solidity ^0.8.0;

import {IEnygmaDvp} from "../core/interfaces/IEnygmaDvp.sol";

/// @notice Test-only stand-in for the EnygmaDvp functions SwapRelayer calls,
///         and for the vault it checks legs against (vaultById returns this
///         contract). Nullifier locks follow Merkle.lock/unlock (no double lock,
///         no unlock of an unlocked nullifier); swap() only counts settlements;
///         checkReceiptConditions rejects a receipt whose proof.a.x is BAD_PROOF.
contract SwapRelayerDvpMock {
    uint256 public constant BAD_PROOF = 0xbad;

    error InvalidProof();

    mapping(uint256 => mapping(uint256 => bool)) public locked;
    uint256 public swapCount;

    function lockReceiptNullifiers(
        IEnygmaDvp.ProofReceipt calldata receipt,
        uint256 vaultId
    ) external returns (bool) {
        uint256 nf = _nullifier(receipt);
        require(!locked[vaultId][nf], "Merkle: Nullifier already locked.");
        locked[vaultId][nf] = true;
        return true;
    }

    function unlockReceiptNullifiers(
        IEnygmaDvp.ProofReceipt calldata receipt,
        uint256 vaultId
    ) external returns (bool) {
        uint256 nf = _nullifier(receipt);
        require(locked[vaultId][nf], "Merkle: Nullifier already unlocked.");
        locked[vaultId][nf] = false;
        return true;
    }

    function vaultById(uint256) external view returns (address) {
        return address(this);
    }

    function checkReceiptConditions(
        IEnygmaDvp.ProofReceipt calldata receipt
    ) external pure returns (bool) {
        if (receipt.proof.a.x == BAD_PROOF) revert InvalidProof();
        return true;
    }

    function isRegisteredSwapGroupPair(uint256, uint256) external pure returns (bool) {
        return true;
    }

    function swap(
        IEnygmaDvp.ProofReceipt memory,
        IEnygmaDvp.ProofReceipt memory,
        uint256,
        uint256
    ) external returns (bool) {
        swapCount++;
        return true;
    }

    // Single-input receipts: statement = [msg, tree, root, nullifier, ...].
    function _nullifier(IEnygmaDvp.ProofReceipt calldata receipt) private pure returns (uint256) {
        return receipt.statement[1 + 2 * receipt.numberOfInputs];
    }
}
