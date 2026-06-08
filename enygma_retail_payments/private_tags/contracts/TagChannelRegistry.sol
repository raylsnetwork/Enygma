// SPDX-License-Identifier: SSPL-1.0
pragma solidity ^0.8.24;

// Senders publish (c1, c2, bitmap) tuples where:
//   c1     = ML-KEM-768 ciphertext (1088 bytes) — paper §3 step 3
//   c2     = AEAD.Encrypt(k, message, c1)        — paper §3 step 5, c1 as AAD
//   bitmap = packed bit array over registered users indicating the anonymity set
//            (paper §3 step 6, Table 2)
//
// Recipients scan all published records and try to decapsulate each c1.
// AEAD authentication failure (using c1 as AAD) serves as implicit channel
// detection — the channel is for me iff decryption of c2 succeeds.
contract TagChannelRegistry {
    struct ChannelRecord {
        address sender;
        bytes c1; // ML-KEM-768 ciphertext (always 1088 bytes)
        bytes c2; // AEAD-encrypted initial message (c1 as AAD)
        bytes bitmap; // packed bitmap over registered user indices
    }

    // Sequential array — recipients scan from a known index onward.
    ChannelRecord[] private _channels;

    event ChannelOpened(uint256 indexed channelIdx, address indexed sender);

    error InvalidC1Length(uint256 got);
    error C2TooLarge(uint256 got);
    error BitmapTooLarge(uint256 got);
    error EmptyBitmap();
    error IndexOutOfRange(uint256 idx, uint256 total);

    // ML-KEM-768 ciphertext is always exactly 1088 bytes.
    uint256 private constant C1_SIZE = 1088;
    // Cap initial message ciphertext at 4 KB.
    uint256 private constant C2_MAX = 4096;
    // Cap bitmap at 16 KB — supports up to 131 072 registered users.
    uint256 private constant BITMAP_MAX = 16384;

    // openChannel publishes a new channel setup record.
    // Returns the sequential index assigned to this record.
    function openChannel(
        bytes calldata c1,
        bytes calldata c2,
        bytes calldata bitmap
    ) external returns (uint256 channelIdx) {
        if (c1.length != C1_SIZE) revert InvalidC1Length(c1.length);
        if (c2.length > C2_MAX) revert C2TooLarge(c2.length);
        if (bitmap.length > BITMAP_MAX) revert BitmapTooLarge(bitmap.length);
        // VUNL: vulnerability detected by Claude
        // Prevents a malicious sender from publishing a channel with an all-zero
        // bitmap, which would cause every recipient's wallet to skip scanning it.
        if (_popcount(bitmap) == 0) revert EmptyBitmap();

        channelIdx = _channels.length;
        _channels.push(
            ChannelRecord({sender: msg.sender, c1: c1, c2: c2, bitmap: bitmap})
        );
        emit ChannelOpened(channelIdx, msg.sender);
    }

    // _popcount counts the number of set bits across all bytes of a bitmap.
    // Uses Brian Kernighan's algorithm: each iteration clears the lowest set bit.
    function _popcount(bytes calldata b) internal pure returns (uint256 count) {
        for (uint256 i = 0; i < b.length; i++) {
            uint8 x = uint8(b[i]);
            while (x != 0) {
                x &= x - 1; // clear lowest set bit
                count++;
            }
        }
    }

    // getChannel returns the full record at a given index.
    function getChannel(
        uint256 idx
    )
        external
        view
        returns (
            address sender,
            bytes memory c1,
            bytes memory c2,
            bytes memory bitmap
        )
    {
        if (idx >= _channels.length)
            revert IndexOutOfRange(idx, _channels.length);
        ChannelRecord storage r = _channels[idx];
        return (r.sender, r.c1, r.c2, r.bitmap);
    }

    // getChannelCount returns the total number of published channel records.
    function getChannelCount() external view returns (uint256) {
        return _channels.length;
    }
}
