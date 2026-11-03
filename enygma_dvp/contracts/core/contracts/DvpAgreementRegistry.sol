// SPDX-License-Identifier: SSPL-1.0
pragma solidity ^0.8.24;

// DvpAgreementRegistry moves the DvP trade agreement on-chain. It replaces the
// off-chain step in which Alice and Bob agree on the swap terms and Alice hands
// Bob the swap secret.
//
// Two message types, both opaque to the contract:
//
//   Offer (channel setup), published by the initiator (Alice):
//     c1     = ML-KEM-768 ciphertext to the counterparty's view key (1088 bytes)
//     c2     = AES-GCM(k, Offer, aad = c1), k = HKDF(ss, channel-key label)
//     bitmap = anonymity set over UserRegistry indices (counterparty's bit + decoys)
//
//   Reply (private tag), published by either party on an open channel:
//     tag  = Poseidon(blockNumber, pkSpend_recipient, ss mod p)
//     ctxt = AES-GCM(k, Accept | Reject | Counter)
//
// Both are emitted as events, not stored: only the two parties need them, and
// keeping the 1088-byte ciphertext out of storage cuts the cost of an offer
// from ~750k gas to ~30k. The contract keeps just an offer counter and a
// (block, tag) duplicate guard.
//
// Callers are expected to go through the DvP relayer so that msg.sender does
// not reveal who is trading. The contract deliberately does not record senders.
contract DvpAgreementRegistry {
    // ML-KEM-768 ciphertext is always exactly 1088 bytes.
    uint256 public constant C1_SIZE = 1088;
    // Cap on an encrypted offer or reply.
    uint256 public constant CTXT_MAX = 4096;
    // Cap on the anonymity bitmap: 16 KB covers 131 072 registered users.
    uint256 public constant BITMAP_MAX = 16384;
    // Maximum number of candidate tags in one reply (see publishReply).
    uint256 public constant TAG_WINDOW_MAX = 16;

    // Number of offers published so far; also the next offer index.
    uint256 public offerCount;

    // (blockNumber, tag) pairs already used — keccak256(abi.encode(bn, tag)).
    mapping(bytes32 => bool) private _tagUsed;

    event OfferOpened(uint256 indexed offerIdx, bytes c1, bytes c2, bytes bitmap);
    event ReplyPublished(uint256 indexed blockNumber, bytes32 indexed tag, bytes ctxt);

    error InvalidC1Length(uint256 got);
    error CtxtTooLarge(uint256 got);
    error BitmapTooLarge(uint256 got);
    error EmptyBitmap();
    error EmptyTagWindow();
    error TagWindowTooLarge(uint256 got);
    error BlockOutsideTagWindow(uint256 blockNumber, uint256 startBlock, uint256 windowSize);
    error TagAlreadyUsed(uint256 blockNumber, bytes32 tag);

    // openOffer publishes an encrypted trade offer to one counterparty.
    function openOffer(
        bytes calldata c1,
        bytes calldata c2,
        bytes calldata bitmap
    ) external returns (uint256 offerIdx) {
        if (c1.length != C1_SIZE) revert InvalidC1Length(c1.length);
        if (c2.length > CTXT_MAX) revert CtxtTooLarge(c2.length);
        if (bitmap.length > BITMAP_MAX) revert BitmapTooLarge(bitmap.length);
        // An all-zero bitmap would make every wallet skip the offer.
        if (!_anyBitSet(bitmap)) revert EmptyBitmap();

        offerIdx = offerCount++;
        emit OfferOpened(offerIdx, c1, c2, bitmap);
    }

    // publishReply publishes an encrypted reply under a private tag.
    //
    // A tag depends on the block it lands in, which the sender cannot know in
    // advance (especially through a relayer). The sender therefore supplies a
    // window of tags for blocks startBlock .. startBlock + tags.length - 1 and
    // the contract publishes the one for the current block.
    function publishReply(
        uint256 startBlock,
        bytes32[] calldata tags,
        bytes calldata ctxt
    ) external returns (bytes32 tag) {
        if (tags.length == 0) revert EmptyTagWindow();
        if (tags.length > TAG_WINDOW_MAX) revert TagWindowTooLarge(tags.length);
        if (ctxt.length > CTXT_MAX) revert CtxtTooLarge(ctxt.length);

        uint256 bn = block.number;
        if (bn < startBlock || bn - startBlock >= tags.length) {
            revert BlockOutsideTagWindow(bn, startBlock, tags.length);
        }
        tag = tags[bn - startBlock];

        bytes32 key = keccak256(abi.encode(bn, tag));
        if (_tagUsed[key]) revert TagAlreadyUsed(bn, tag);
        _tagUsed[key] = true;

        emit ReplyPublished(bn, tag, ctxt);
    }

    // _anyBitSet reports whether any byte of the bitmap is non-zero.
    function _anyBitSet(bytes calldata b) private pure returns (bool) {
        for (uint256 i = 0; i < b.length; i++) {
            if (b[i] != 0) return true;
        }
        return false;
    }
}
