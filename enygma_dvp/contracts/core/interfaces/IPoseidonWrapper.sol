// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;


interface IPoseidonWrapper {

    function poseidon(uint256[2] memory input) external pure returns (uint256);

}