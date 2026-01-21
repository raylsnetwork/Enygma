// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;

import {IZkDvp} from "../interfaces/IZkDvp.sol";

interface IGenericGroth16Verifier {

    function negate(IZkDvp.G1Point memory p) external pure returns (IZkDvp.G1Point memory) ;
    
    function add(IZkDvp.G1Point memory p1, IZkDvp.G1Point memory p2)
        external
        view
        returns (IZkDvp.G1Point memory);

    function scalarMul(IZkDvp.G1Point memory p, uint256 s)
        external
        view
        returns (IZkDvp.G1Point memory r);

    function pairing(
        IZkDvp.G1Point memory _a1,
        IZkDvp.G2Point memory _a2,
        IZkDvp.G1Point memory _b1,
        IZkDvp.G2Point memory _b2,
        IZkDvp.G1Point memory _c1,
        IZkDvp.G2Point memory _c2,
        IZkDvp.G1Point memory _d1,
        IZkDvp.G2Point memory _d2
    ) external view returns (bool);

    function verify(
        IZkDvp.VerifyingKey memory _vk,
        IZkDvp.SnarkProof memory _proof,
        uint256[] memory _input
    ) external view returns (bool);
}
