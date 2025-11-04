// SPDX-License-Identifier: GPL3
pragma solidity ^0.8.0;

interface IZkDvp {
    struct G1Point {
        uint256 x;
        uint256 y;
    }

    struct WithdrawParams {
        JoinSplitTransaction transaction;
    }
    struct G2Point {
        uint256[2] x;
        uint256[2] y;
    }

    struct JoinSplitTransaction {
        SnarkProof proof;
        uint256[] statement;
        uint256 numberOfInputs;
        uint256 numberOfOutputs;
    }

    struct SnarkProof {
        G1Point a;
        G2Point b;
        G1Point c;
    }
    function depositERC20(
        uint256 _amount,
        address _erc20Address,
        uint256 _publicKey
    ) external returns (bool);

    function depositThroughEnygma(
        uint256[] memory depositParams
    ) external returns (bool, uint256);

    function withdrawThroughEnygma(
        JoinSplitTransaction memory _tx
    ) external returns (bool);
}
