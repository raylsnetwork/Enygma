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
///   - EnygmaDvp.payment*: one note, output 0's (ctxt, encTxData);
///   - Erc20CoinVault.transferV2: one per output;
///   - Erc20CoinVault.transfer: none (two empty arrays).
/// The array lengths are part of the encoding, so a receipt proved for one
/// entry point's ciphertexts cannot be settled through another's.
/// enygma_dvp/src/core.PaymentCiphertextHash computes the same value.
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
}
