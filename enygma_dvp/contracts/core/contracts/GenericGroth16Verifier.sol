// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;
// pragma abicoder v2;

import {IZkDvp} from "../interfaces/IZkDvp.sol";

contract GenericGroth16Verifier {
  uint256 constant SNARK_SCALAR_FIELD = 21888242871839275222246405745257275088548364400416034343698204186575808495617;

    uint256 private constant PRIME_Q =
        21888242871839275222246405745257275088696311157297823662689037894645226208583;
    uint256 private constant PAIRING_INPUT_SIZE = 24;
    uint256 private constant PAIRING_INPUT_WIDTH = 768; // PAIRING_INPUT_SIZE * 32

    function negate(IZkDvp.G1Point memory p) public pure returns (IZkDvp.G1Point memory) {
        if (p.x == 0 && p.y == 0) return IZkDvp.G1Point(0, 0);

        uint256 rh = mulmod(p.x, p.x, PRIME_Q);
        rh = mulmod(rh, p.x, PRIME_Q);
        rh = addmod(rh, 3, PRIME_Q);
        uint256 lh = mulmod(p.y, p.y, PRIME_Q);
        require(lh == rh, "Snark: ");

        return IZkDvp.G1Point(p.x, PRIME_Q - (p.y % PRIME_Q));
    }

    function add(IZkDvp.G1Point memory p1, IZkDvp.G1Point memory p2)
        public
        view
        returns (IZkDvp.G1Point memory)
    {
        uint256[4] memory input;
        input[0] = p1.x;
        input[1] = p1.y;
        input[2] = p2.x;
        input[3] = p2.y;

        bool success;
        IZkDvp.G1Point memory result;

        // solhint-disable-next-line no-inline-assembly
        assembly {
            success := staticcall(
                sub(gas(), 2000),
                6,
                input,
                0x80,
                result,
                0x40
            )
        }

        require(success, "Pairing: Add Failed");
        return result;
    }

    function scalarMul(IZkDvp.G1Point memory p, uint256 s)
        public
        view
        returns (IZkDvp.G1Point memory r)
    {
        uint256[3] memory input;
        input[0] = p.x;
        input[1] = p.y;
        input[2] = s;
        bool success;

        // solhint-disable-next-line no-inline-assembly
        assembly {
            success := staticcall(sub(gas(), 2000), 7, input, 0x60, r, 0x40)
        }
        require(success, "Pairing: Scalar Multiplication Failed");
    }

    function pairing(
        IZkDvp.G1Point memory _a1,
        IZkDvp.G2Point memory _a2,
        IZkDvp.G1Point memory _b1,
        IZkDvp.G2Point memory _b2,
        IZkDvp.G1Point memory _c1,
        IZkDvp.G2Point memory _c2,
        IZkDvp.G1Point memory _d1,
        IZkDvp.G2Point memory _d2
    ) public view returns (bool) {
        uint256[PAIRING_INPUT_SIZE] memory input = [
            _a1.x,
            _a1.y,
            _a2.x[0],
            _a2.x[1],
            _a2.y[0],
            _a2.y[1],
            _b1.x,
            _b1.y,
            _b2.x[0],
            _b2.x[1],
            _b2.y[0],
            _b2.y[1],
            _c1.x,
            _c1.y,
            _c2.x[0],
            _c2.x[1],
            _c2.y[0],
            _c2.y[1],
            _d1.x,
            _d1.y,
            _d2.x[0],
            _d2.x[1],
            _d2.y[0],
            _d2.y[1]
        ];

        uint256[1] memory out;
        bool success;

        // solhint-disable-next-line no-inline-assembly
        assembly {
            success := staticcall(
                sub(gas(), 2000),
                8,
                input,
                PAIRING_INPUT_WIDTH,
                out,
                0x20
            )
        }

        require(success, "Pairing: Pairing Verification Failed");
        return out[0] != 0;
    }

    function verify(
        IZkDvp.VerifyingKey memory _vk,
        IZkDvp.SnarkProof memory _proof,
        uint256[] memory _input
    ) public view returns (bool) {
        require(_input.length + 1 == _vk.ic.length, "verifier-bad-statement-length");
        IZkDvp.G1Point memory vkX = IZkDvp.G1Point(0, 0);
        for (uint256 i = 0; i < _input.length; i++) {
            require(
                _input[i] < SNARK_SCALAR_FIELD,
                "verifier-gte-snark-scalar-field"
            );
            vkX = add(vkX, scalarMul(_vk.ic[i + 1], _input[i]));
        }
        vkX = add(vkX, _vk.ic[0]);
        return
            pairing(
                negate(_proof.a),
                _proof.b,
                _vk.alpha1,
                _vk.beta2,
                vkX,
                _vk.gamma2,
                _proof.c,
                _vk.delta2
            );
    }
}
