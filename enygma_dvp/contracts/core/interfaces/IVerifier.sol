// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;

import {IEnygmaDvp} from "../interfaces/IEnygmaDvp.sol";

interface IVerifier {
    error MaxNumberOfCircuitsExceeded();

    function initializeVerifier(
        address groth16Verifier_
    ) external returns (bool);

    function addVerificationKey(
        IEnygmaDvp.VerifyingKey memory vKey_
    ) external returns (bool);

    function verifyProof(
        uint256 verificationKeyIndex,
        IEnygmaDvp.SnarkProof memory proof,
        uint256[] memory inputs
    ) external view returns (bool);
}
