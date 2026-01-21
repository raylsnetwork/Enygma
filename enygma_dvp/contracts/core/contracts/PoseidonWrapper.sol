// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;
// pragma abicoder v2;

import {IZkDvp} from "../interfaces/IZkDvp.sol";
import {PoseidonT3} from "./Poseidon.sol";

contract PoseidonWrapper {
    function poseidon(uint256[2] memory input) public pure returns (uint256){
        return PoseidonT3.poseidon(input);
    }

}
