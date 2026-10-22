// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;

import {PRIVATE_MINT_LEN} from "../contracts/DvpStatementLayout.sol";

interface IPrivateMintVerifier {
    /// @notice Verify a Groth16 proof for private mint
    /// @param proof The proof points (A, B, C) - 8 uint256 values
    /// @param input The public inputs, in PrivateMint's statement layout
    function verifyProof(
        uint256[8] calldata proof,
        uint256[PRIVATE_MINT_LEN] calldata input
    ) external view;
}
