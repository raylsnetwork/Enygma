// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;

import {IEnygmaDvp} from "../interfaces/IEnygmaDvp.sol";

interface IGenericGroth16Verifier {
    function negate(
        IEnygmaDvp.G1Point memory p
    ) external pure returns (IEnygmaDvp.G1Point memory);

    function add(
        IEnygmaDvp.G1Point memory p1,
        IEnygmaDvp.G1Point memory p2
    ) external view returns (IEnygmaDvp.G1Point memory);

    function scalarMul(
        IEnygmaDvp.G1Point memory p,
        uint256 s
    ) external view returns (IEnygmaDvp.G1Point memory r);

    function pairing(
        IEnygmaDvp.G1Point memory _a1,
        IEnygmaDvp.G2Point memory _a2,
        IEnygmaDvp.G1Point memory _b1,
        IEnygmaDvp.G2Point memory _b2,
        IEnygmaDvp.G1Point memory _c1,
        IEnygmaDvp.G2Point memory _c2,
        IEnygmaDvp.G1Point memory _d1,
        IEnygmaDvp.G2Point memory _d2
    ) external view returns (bool);

    function verify(
        IEnygmaDvp.VerifyingKey memory _vk,
        IEnygmaDvp.SnarkProof memory _proof,
        uint256[] memory _input
    ) external view returns (bool);
}
