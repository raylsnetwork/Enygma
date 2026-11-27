// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;

import {IEnygmaDvp} from "../interfaces/IEnygmaDvp.sol";

interface IVerifier {
    error MaxNumberOfCircuitsExceeded();
    // initializeVerifier, addVerificationKey or freeze on a frozen Verifier.
    error VerifierFrozen();

    // initializeVerifier stored the Groth16 backend.
    event Groth16VerifierSet(address groth16Verifier);
    // addVerificationKey filled slot index with the key whose
    // keccak256(abi.encode(vk)) is keyHash.
    event VerificationKeyAdded(uint256 indexed index, bytes32 keyHash);
    // freeze closed the key table at keyCount keys.
    event KeysFrozen(uint256 keyCount);

    function initializeVerifier(
        address groth16Verifier_
    ) external returns (bool);

    function addVerificationKey(
        IEnygmaDvp.VerifyingKey memory vKey_
    ) external returns (bool);

    function freeze() external returns (bool);

    function frozen() external view returns (bool);

    function groth16Verifier() external view returns (address);

    function verificationKeyCount() external view returns (uint256);

    function verificationKeyHash(uint256 index) external view returns (bytes32);

    function verifyProof(
        uint256 verificationKeyIndex,
        IEnygmaDvp.SnarkProof memory proof,
        uint256[] memory inputs
    ) external view returns (bool);
}
