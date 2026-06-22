// SPDX-License-Identifier: BUSL-1.1
pragma solidity ^0.8.20;

// ---------------------------------------------------------------------------
// Local type definitions (same tuple shapes as IEnygmaDvp in the DVP project;
// ABI encoding is structural so cross-contract calls are compatible).
// ---------------------------------------------------------------------------

struct G1Point { uint256 x; uint256 y; }
struct G2Point { uint256[2] x; uint256[2] y; }

struct SnarkProof {
    G1Point a;
    G2Point b;
    G1Point c;
}

struct VerifyingKey {
    G1Point   alpha1;
    G2Point   beta2;
    G2Point   gamma2;
    G2Point   delta2;
    G1Point[] ic;
}

/// @title GenericGroth16Verifier
/// @notice Stateless BN254 pairing verifier for Groth16 proofs.
///         Identical logic to the DVP project's verifier; adapted to use
///         local struct definitions instead of IEnygmaDvp namespace types.
contract GenericGroth16Verifier {
    uint256 private constant SNARK_SCALAR_FIELD =
        21888242871839275222246405745257275088548364400416034343698204186575808495617;
    uint256 private constant PRIME_Q =
        21888242871839275222246405745257275088696311157297823662689037894645226208583;
    uint256 private constant PAIRING_INPUT_SIZE  = 24;
    uint256 private constant PAIRING_INPUT_WIDTH = 768; // 24 * 32

    function negate(G1Point memory p) public pure returns (G1Point memory) {
        if (p.x == 0 && p.y == 0) return G1Point(0, 0);
        uint256 rh = mulmod(p.x, p.x, PRIME_Q);
        rh = mulmod(rh, p.x, PRIME_Q);
        rh = addmod(rh, 3, PRIME_Q);
        uint256 lh = mulmod(p.y, p.y, PRIME_Q);
        require(lh == rh, "Snark: point not on curve");
        return G1Point(p.x, PRIME_Q - (p.y % PRIME_Q));
    }

    function add(G1Point memory p1, G1Point memory p2) public view returns (G1Point memory) {
        uint256[4] memory input;
        input[0] = p1.x; input[1] = p1.y;
        input[2] = p2.x; input[3] = p2.y;
        bool success;
        G1Point memory result;
        // solhint-disable-next-line no-inline-assembly
        assembly {
            success := staticcall(sub(gas(), 2000), 6, input, 0x80, result, 0x40)
        }
        require(success, "Pairing: add failed");
        return result;
    }

    function scalarMul(G1Point memory p, uint256 s) public view returns (G1Point memory r) {
        uint256[3] memory input;
        input[0] = p.x; input[1] = p.y; input[2] = s;
        bool success;
        // solhint-disable-next-line no-inline-assembly
        assembly {
            success := staticcall(sub(gas(), 2000), 7, input, 0x60, r, 0x40)
        }
        require(success, "Pairing: scalar mul failed");
    }

    function pairing(
        G1Point memory _a1, G2Point memory _a2,
        G1Point memory _b1, G2Point memory _b2,
        G1Point memory _c1, G2Point memory _c2,
        G1Point memory _d1, G2Point memory _d2
    ) public view returns (bool) {
        uint256[PAIRING_INPUT_SIZE] memory input = [
            _a1.x, _a1.y,
            _a2.x[0], _a2.x[1], _a2.y[0], _a2.y[1],
            _b1.x, _b1.y,
            _b2.x[0], _b2.x[1], _b2.y[0], _b2.y[1],
            _c1.x, _c1.y,
            _c2.x[0], _c2.x[1], _c2.y[0], _c2.y[1],
            _d1.x, _d1.y,
            _d2.x[0], _d2.x[1], _d2.y[0], _d2.y[1]
        ];
        uint256[1] memory out;
        bool success;
        // solhint-disable-next-line no-inline-assembly
        assembly {
            success := staticcall(sub(gas(), 2000), 8, input, PAIRING_INPUT_WIDTH, out, 0x20)
        }
        require(success, "Pairing: pairing check failed");
        return out[0] != 0;
    }

    function verify(
        VerifyingKey memory _vk,
        SnarkProof   memory _proof,
        uint256[]    memory _input
    ) public view returns (bool) {
        require(_input.length + 1 == _vk.ic.length, "verifier-bad-statement-length");
        G1Point memory vkX = G1Point(0, 0);
        for (uint256 i = 0; i < _input.length; i++) {
            require(_input[i] < SNARK_SCALAR_FIELD, "verifier-gte-snark-scalar-field");
            vkX = add(vkX, scalarMul(_vk.ic[i + 1], _input[i]));
        }
        vkX = add(vkX, _vk.ic[0]);
        return pairing(
            negate(_proof.a), _proof.b,
            _vk.alpha1,       _vk.beta2,
            vkX,              _vk.gamma2,
            _proof.c,         _vk.delta2
        );
    }
}
