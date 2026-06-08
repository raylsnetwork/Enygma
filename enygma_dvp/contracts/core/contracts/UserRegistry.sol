// SPDX-License-Identifier: SSPL-1.0
pragma solidity ^0.8.24;

// UserRegistry stores each user's spend public key and ML-KEM view public key
// fully on-chain so any sender can retrieve them without scanning event logs.
//
// Gas note: pkView is 1184 bytes (ML-KEM-768 encapsulation key).
// Storing it on-chain costs ~900k gas on first write. This is intentional —
// registration is a one-time operation per user.
contract UserRegistry {

    struct UserKeys {
        uint256 pkSpend; // Poseidon(sk_spend) — used to build output commitments
        bytes   pkView;  // ML-KEM-768 encapsulation key (1184 bytes)
    }

    mapping(address => UserKeys) private _keys;
    mapping(address => uint32)   private _userIndex; // 0-based index in _userList
    address[]                    private _userList;  // ordered by registration

    event UserRegistered(
        address indexed user,
        uint256 pkSpend,
        bytes   pkView
    );

    error AlreadyRegistered(address user);
    error NotRegistered(address user);
    error IndexOutOfRange(uint256 index, uint256 length);

    // register publishes the caller's spend and view public keys on-chain.
    // Each address can only register once.
    function register(uint256 pkSpend, bytes calldata pkView) external {
        if (_keys[msg.sender].pkSpend != 0) revert AlreadyRegistered(msg.sender);
        _userIndex[msg.sender] = uint32(_userList.length);
        _userList.push(msg.sender);
        _keys[msg.sender] = UserKeys({pkSpend: pkSpend, pkView: pkView});
        emit UserRegistered(msg.sender, pkSpend, pkView);
    }

    // getKeys returns the spend and view public keys for a registered address.
    function getKeys(address user) external view returns (uint256 pkSpend, bytes memory pkView) {
        UserKeys storage k = _keys[user];
        if (k.pkSpend == 0) revert NotRegistered(user);
        return (k.pkSpend, k.pkView);
    }

    // isRegistered returns true if the address has called register().
    function isRegistered(address user) external view returns (bool) {
        return _keys[user].pkSpend != 0;
    }

    // getUserCount returns the total number of registered users.
    function getUserCount() external view returns (uint256) {
        return _userList.length;
    }

    // getUserAt returns the address at position index in registration order.
    function getUserAt(uint256 index) external view returns (address) {
        if (index >= _userList.length) revert IndexOutOfRange(index, _userList.length);
        return _userList[index];
    }

    // getUserIndex returns the 0-based registration index for a registered user.
    function getUserIndex(address user) external view returns (uint32) {
        if (_keys[user].pkSpend == 0) revert NotRegistered(user);
        return _userIndex[user];
    }

    // getAllUsers returns all registered addresses in registration order.
    // Use only off-chain — gas cost grows with the number of registered users.
    function getAllUsers() external view returns (address[] memory) {
        return _userList;
    }
}
