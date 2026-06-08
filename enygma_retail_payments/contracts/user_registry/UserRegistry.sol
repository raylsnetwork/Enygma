// SPDX-License-Identifier: SSPL-1.0
pragma solidity ^0.8.24;

// UserRegistry stores each user's spend public key and ML-KEM view public key
// fully on-chain so any sender can retrieve them without scanning event logs.
//
// Sybil Attack Protection (paper §6.2):
// The owner sets registrationFee > 0 to require a stake per account.
// This makes flooding the system economically costly — an attacker must pay
// registrationFee per fake identity. The default is 0 (free registration).
//
// Gas note: pkView is 1184 bytes (ML-KEM-768 encapsulation key).
// Storing it on-chain costs ~900k gas on first write. This is intentional —
// registration is a one-time operation per user.
contract UserRegistry {

    struct UserKeys {
        bool    registered; // VULN-8 fix: explicit flag instead of pkSpend != 0 sentinel
        uint256 pkSpend;    // Poseidon(sk_spend) — used to build output commitments
        bytes   pkView;     // ML-KEM-768 encapsulation key (exactly 1184 bytes)
    }

    mapping(address => UserKeys) private _keys;
    mapping(address => uint32)   private _userIndex; // 0-based index in _userList
    address[]                    private _userList;  // ordered by registration

    // ── Sybil protection ───────────────────────────────────────────────────────
    address private _owner;

    // registrationFee is the ETH amount required per registration.
    // Default: 0 (free). Owner sets this to deter Sybil attacks.
    uint256 public registrationFee;

    // ── Events ─────────────────────────────────────────────────────────────────
    event UserRegistered(address indexed user, uint256 pkSpend, bytes pkView);
    event RegistrationFeeUpdated(uint256 oldFee, uint256 newFee);
    event FeesWithdrawn(address indexed to, uint256 amount);

    // ── Errors ─────────────────────────────────────────────────────────────────
    error AlreadyRegistered(address user);
    error NotRegistered(address user);
    error IndexOutOfRange(uint256 index, uint256 length);
    error InvalidPkViewLength(uint256 got);             // pkView must be 1184 bytes
    error InsufficientFee(uint256 required, uint256 provided);
    error NotOwner();
    error WithdrawFailed();

    modifier onlyOwner() {
        if (msg.sender != _owner) revert NotOwner();
        _;
    }

    constructor() {
        _owner = msg.sender;
    }

    // ── Registration ───────────────────────────────────────────────────────────

    // register publishes the caller's spend and view public keys on-chain.
    // Requires msg.value >= registrationFee (Sybil protection when fee > 0).
    // Each address can only register once.
    function register(uint256 pkSpend, bytes calldata pkView) external payable {
        if (msg.value < registrationFee)
            revert InsufficientFee(registrationFee, msg.value);
        if (pkView.length != 1184)
            revert InvalidPkViewLength(pkView.length);
        if (_keys[msg.sender].registered)
            revert AlreadyRegistered(msg.sender);
        _userIndex[msg.sender] = uint32(_userList.length);
        _userList.push(msg.sender);
        _keys[msg.sender] = UserKeys({registered: true, pkSpend: pkSpend, pkView: pkView});
        emit UserRegistered(msg.sender, pkSpend, pkView);
    }

    // ── Key lookups ────────────────────────────────────────────────────────────

    // getKeys returns the spend and view public keys for a registered address.
    function getKeys(address user) external view returns (uint256 pkSpend, bytes memory pkView) {
        UserKeys storage k = _keys[user];
        if (!k.registered) revert NotRegistered(user);
        return (k.pkSpend, k.pkView);
    }

    // isRegistered returns true if the address has called register().
    function isRegistered(address user) external view returns (bool) {
        return _keys[user].registered;
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
        if (!_keys[user].registered) revert NotRegistered(user);
        return _userIndex[user];
    }

    // getAllUsers returns all registered addresses in registration order.
    // Use only off-chain — gas cost grows with the number of registered users.
    function getAllUsers() external view returns (address[] memory) {
        return _userList;
    }

    // ── Sybil protection admin ─────────────────────────────────────────────────

    // setRegistrationFee sets the ETH fee required per registration.
    // Set to 0 to allow free registration (default).
    // Set to > 0 to make Sybil attacks economically costly.
    function setRegistrationFee(uint256 fee) external onlyOwner {
        uint256 old = registrationFee;
        registrationFee = fee;
        emit RegistrationFeeUpdated(old, fee);
    }

    // withdrawFees transfers all collected registration fees to the owner.
    function withdrawFees() external onlyOwner {
        uint256 balance = address(this).balance;
        (bool ok,) = _owner.call{value: balance}("");
        if (!ok) revert WithdrawFailed();
        emit FeesWithdrawn(_owner, balance);
    }

    // owner returns the contract owner address.
    function owner() external view returns (address) {
        return _owner;
    }
}
