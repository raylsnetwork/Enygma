// SPDX-License-Identifier: SSPL-1.0
pragma solidity ^0.8.24;

// Each entry is a tuple (t, ctxt) where:
//   t    = TH(nblock, pkRecipient, sharedSecret)  — private messaging tag
//   ctxt = AEAD.Encrypt(channelKey, payload, c1)  — encrypted payload
//
// Tag derivation (off-chain, Poseidon over BN254):
//   ss_field = sharedSecret mod BN254_p
//   t        = Poseidon3(blockNumber, pkRecipient, ss_field)
//
// The recipient scans: for each known channel (ss, pkSpend), compute
//   t' = Poseidon3(block, pkSpend, ss_field)
// and check if any on-chain t matches t'. Complexity: O(channels) per block.
contract TagRegistry {
    struct TagEntry {
        address publisher; // msg.sender — publicly visible
        bytes32 tag; // Poseidon3(blockNumber, pkRecipient, ss_field)
        bytes ctxt; // AEAD-encrypted payload
    }

    // blockNumber → list of (tag, ctxt) entries published in that block
    mapping(uint256 => TagEntry[]) private _entries;

    // O(1) duplicate guard: keccak256(blockNumber, tag) → published
    mapping(bytes32 => bool) private _tagExists;

    // Per-address per-block publish count for spam prevention
    // keccak256(blockNumber, publisher) → count
    mapping(bytes32 => uint256) private _blockPublishCount;

    // Maximum tags a single address may publish per block.
    uint256 private constant MAX_TAGS_PER_BLOCK = 10;

    // Maximum size of the encrypted payload.
    uint256 private constant CTXT_MAX = 4096;

    event TagPublished(
        uint256 indexed blockNumber,
        bytes32 indexed tag,
        address indexed publisher,
        bytes ctxt
    );

    error TagAlreadyExists(bytes32 tag, uint256 blockNumber);
    error RateLimitExceeded(address publisher, uint256 blockNumber);
    error CtxtTooLarge(uint256 got);

    // publishTag publishes a (tag, ctxt) tuple at the current block number.
    // Reverts if the (tag, block) pair is already registered, if the caller
    // has exceeded MAX_TAGS_PER_BLOCK in this block, or if ctxt exceeds CTXT_MAX.
    function publishTag(bytes32 tag, bytes calldata ctxt) external {
        uint256 bn = block.number;

        if (ctxt.length > CTXT_MAX) revert CtxtTooLarge(ctxt.length);

        // O(1) duplicate check.
        bytes32 tagKey = keccak256(abi.encode(bn, tag));
        if (_tagExists[tagKey]) revert TagAlreadyExists(tag, bn);

        // Per-address per-block rate limit.
        bytes32 senderKey = keccak256(abi.encode(bn, msg.sender));
        uint256 count = _blockPublishCount[senderKey];
        if (count >= MAX_TAGS_PER_BLOCK)
            revert RateLimitExceeded(msg.sender, bn);

        _tagExists[tagKey] = true;
        _blockPublishCount[senderKey] = count + 1;

        _entries[bn].push(
            TagEntry({publisher: msg.sender, tag: tag, ctxt: ctxt})
        );
        emit TagPublished(bn, tag, msg.sender, ctxt);
    }

    // getEntries returns all (tag, ctxt) pairs published at a given block number.
    function getEntries(
        uint256 blockNumber
    ) external view returns (TagEntry[] memory) {
        return _entries[blockNumber];
    }

    // getCount returns the number of tags published at a given block.
    function getCount(uint256 blockNumber) external view returns (uint256) {
        return _entries[blockNumber].length;
    }
}
