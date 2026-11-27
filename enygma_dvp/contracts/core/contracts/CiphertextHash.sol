// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;

/// @notice The StCiphertextHash a Payment-family proof binds: the hash of the
///         note ciphertexts its payment publishes, so that a receipt settles
///         only with the ciphertexts its prover chose.
///
/// The ciphertexts are plain call arguments, which the circuit cannot read,
/// so the prover hashes them off-chain and proves the hash as a public
/// signal, and the contract that settles the receipt recomputes it from the
/// ciphertexts it was given and compares. Without this whoever settled a
/// receipt first, through any entry point, could publish other ciphertexts
/// or none: the input notes were spent and the outputs inserted, but the
/// recipient's scan never found the note.
///
/// The hash is keccak256(abi.encode(ciphertextI, ciphertextII)) reduced into
/// the BN254 scalar field, ciphertextI[i]/ciphertextII[i] being the ML-KEM
/// capsule and the AEAD ciphertext of one note:
///   - EnygmaDvp.payment*'s payment (not a USDr fee leg, below): one note,
///     output 0's (ctxt, encTxData);
///   - Erc20CoinVault.transferV2: one per output;
///   - Erc20CoinVault.transfer: none (two empty arrays).
/// The array lengths are part of the encoding, so a receipt proved for one
/// entry point's ciphertexts cannot be settled through another's.
/// enygma_dvp/src/core.PaymentCiphertextHash computes the same value.
///
/// A channel payment (EnygmaDvp.channelPayment, #451) publishes no capsule:
/// its proof binds the block its tag is for, the tag and the sealed note,
/// encoded after a domain word (ofChannelPayment). The domain word is never
/// an array offset, which ofNotes' encoding starts with, so neither kind of
/// receipt settles as the other.
///
/// The USDr fee leg of EnygmaDvp.paymentWithUsdrFee and
/// channelPaymentWithUsdrFee binds, after a domain word of its own, the
/// statement of the payment it pays for and the fee note's ciphertexts
/// (ofUsdrFeeLeg). So it settles only as the fee leg of that one payment, in
/// the same call: not on its own through another entry point, and not next
/// to another payment.
library CiphertextHash {
    uint256 internal constant SNARK_SCALAR_FIELD =
        21888242871839275222246405745257275088548364400416034343698204186575808495617;

    function ofNotes(
        bytes[] memory ciphertextI,
        bytes[] memory ciphertextII
    ) internal pure returns (uint256) {
        return uint256(keccak256(abi.encode(ciphertextI, ciphertextII))) % SNARK_SCALAR_FIELD;
    }

    /// The hash of a single note's ciphertexts, as EnygmaDvp.payment* publishes them.
    function ofNote(
        bytes calldata ctxt,
        bytes calldata encTxData
    ) internal pure returns (uint256) {
        bytes[] memory ciphertextI = new bytes[](1);
        bytes[] memory ciphertextII = new bytes[](1);
        ciphertextI[0] = ctxt;
        ciphertextII[0] = encTxData;
        return ofNotes(ciphertextI, ciphertextII);
    }

    /// keccak256("enygma-channel-payment-v1"), the first word of a channel
    /// payment's encoding.
    bytes32 internal constant CHANNEL_PAYMENT_DOMAIN = keccak256("enygma-channel-payment-v1");

    /// The hash a channel payment's proof binds: its block_n, tag and note
    /// ciphertext. enygma_dvp/src/core.ChannelPaymentCiphertextHash computes
    /// the same value.
    function ofChannelPayment(
        uint256 blockNumber,
        bytes32 tag,
        bytes calldata ctxt
    ) internal pure returns (uint256) {
        return uint256(keccak256(abi.encode(CHANNEL_PAYMENT_DOMAIN, blockNumber, tag, ctxt))) % SNARK_SCALAR_FIELD;
    }

    /// keccak256("enygma-usdr-fee-leg-v1"), the first word of a USDr fee
    /// leg's encoding.
    bytes32 internal constant USDR_FEE_LEG_DOMAIN = keccak256("enygma-usdr-fee-leg-v1");

    /// The hash a USDr fee leg's proof binds: the statement of the payment
    /// the fee pays for (the main leg's public signals) and the fee note's
    /// ciphertexts. enygma_dvp/src/core.UsdrFeeLegCiphertextHash computes the
    /// same value.
    function ofUsdrFeeLeg(
        uint256[] memory mainStatement,
        bytes calldata usdrCtxt,
        bytes calldata usdrEncTxData
    ) internal pure returns (uint256) {
        return uint256(keccak256(abi.encode(
            USDR_FEE_LEG_DOMAIN, keccak256(abi.encode(mainStatement)), usdrCtxt, usdrEncTxData
        ))) % SNARK_SCALAR_FIELD;
    }
}
