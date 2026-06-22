// SPDX-License-Identifier: BUSL-1.1
pragma solidity ^0.8.20;

import {PoseidonT3} from "./Poseidon.sol";
import {PoseidonT5} from "./Poseidon.sol";

/// @notice Thin wrapper that exposes the circomlibjs Poseidon libraries
///         as a regular contract callable via an address pointer.
contract PoseidonWrapper {
    function poseidon(uint256[2] memory input) public pure returns (uint256) {
        return PoseidonT3.poseidon(input);
    }

    function poseidon4(uint256[4] memory input) public pure returns (uint256) {
        return PoseidonT5.poseidon(input);
    }
}
