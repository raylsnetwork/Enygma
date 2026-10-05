// SPDX-License-Identifier: BUSL-1.1
pragma solidity ^0.8.0;

import {IEnygmaDvp} from "../core/interfaces/IEnygmaDvp.sol";

/// @notice Test-only stand-in for the four EnygmaDvp functions SwapRelayer
///         calls. Nullifier locks follow Merkle.lock/unlock (no double lock,
///         no unlock of an unlocked nullifier); swap() only counts settlements.
contract SwapRelayerDvpMock {
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
