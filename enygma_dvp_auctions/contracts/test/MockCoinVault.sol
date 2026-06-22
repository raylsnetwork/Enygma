// SPDX-License-Identifier: BUSL-1.1
pragma solidity ^0.8.20;

/// @title MockCoinVault
/// @notice Test double for ICoinVault. Tracks calls in simple public state so
/// tests can assert on vault interactions without needing the real
/// Merkle-tree-backed vault implementation.
contract MockCoinVault {
    bool public rootValid = true;

    mapping(uint256 => bool) public locked;    // nullifier => locked
    mapping(uint256 => bool) public nullified; // nullifier => permanently spent
    uint256[] public registeredCoins;          // every commitment ever registered, in order

    function setRootValid(bool v) external {
        rootValid = v;
    }

    function verifyRoot(uint256 /*treeNumber*/, uint256 /*root*/) external view returns (bool) {
        return rootValid;
    }

    function lockCoin(uint256 /*treeNumber*/, uint256 nullifier) external returns (bool) {
        locked[nullifier] = true;
        return true;
    }

    function unlockCoin(uint256 /*treeNumber*/, uint256 nullifier) external returns (bool) {
        locked[nullifier] = false;
        return true;
    }

    function nullifyCoin(uint256 /*treeNumber*/, uint256 nullifier) external returns (bool) {
        nullified[nullifier] = true;
        return true;
    }

    function registerCoins(uint256[] calldata commitments) external returns (bool) {
        for (uint256 i = 0; i < commitments.length; i++) {
            registeredCoins.push(commitments[i]);
        }
        return true;
    }

    function registeredCoinsLength() external view returns (uint256) {
        return registeredCoins.length;
    }

    function allRegisteredCoins() external view returns (uint256[] memory) {
        return registeredCoins;
    }
}
