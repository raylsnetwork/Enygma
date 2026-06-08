// SPDX-License-Identifier: SSPL-1.0
pragma solidity ^0.8.24;

// ChannelRegistry stores ML-KEM channel setup records on-chain.
//
// Each record contains:
//   ctKEM   — the ML-KEM-768 ciphertext (1088 bytes). Only the intended
//             recipient can decapsulate this to recover the shared secret.
//   anonSet — a packed bitmap of registered user indices. Bits set to 1
//             indicate users who might be the recipient, providing sender-
//             privacy according to the chosen mode (none/subset/rift/full).
//
// channelId is derived off-chain as SHA256(0x01 || sharedSecret) so both
// sender and recipient can compute it independently from the shared secret.
contract ChannelRegistry {

    struct ChannelRecord {
        address sender;
        bytes   ctKEM;   // ML-KEM-768 ciphertext (1088 bytes)
        bytes   anonSet; // packed bitmap over registered user indices
    }

    mapping(bytes32 => ChannelRecord) private _channels;

    event ChannelOpened(
        bytes32 indexed channelId,
        address indexed sender
    );

    error ChannelAlreadyExists(bytes32 channelId);
    error ChannelNotFound(bytes32 channelId);

    // openChannel records a new channel setup on-chain.
    // channelId must be unique — use SHA256(0x01 || sharedSecret).
    function openChannel(
        bytes32 channelId,
        bytes calldata ctKEM,
        bytes calldata anonSet
    ) external {
        if (_channels[channelId].sender != address(0)) revert ChannelAlreadyExists(channelId);
        _channels[channelId] = ChannelRecord({
            sender:  msg.sender,
            ctKEM:   ctKEM,
            anonSet: anonSet
        });
        emit ChannelOpened(channelId, msg.sender);
    }

    // getChannel returns the channel record for a given channelId.
    function getChannel(bytes32 channelId) external view returns (
        address sender,
        bytes memory ctKEM,
        bytes memory anonSet
    ) {
        ChannelRecord storage r = _channels[channelId];
        if (r.sender == address(0)) revert ChannelNotFound(channelId);
        return (r.sender, r.ctKEM, r.anonSet);
    }
}
