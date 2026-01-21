// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;

interface IMerkle {

    function initializeMerkle(uint256 _treeDepth, uint256 _treeId, address _poseidonWrapperAddress) external ;

    function hashLeftRight(uint256 _left, uint256 _right) external view returns (uint256);

    function setNullifier(uint256 _treeNumber, uint256 _nullifierId)  external ;

    function isValidNullifier(uint256 _treeNumber, uint256 _nullifierId)  external view returns (bool);

    function isValidRoot(uint256 _treeNumber, uint256 _merkleRoot)  external view returns (bool);

    function currentRoot() external view returns (uint256);

    function insertLeaves(uint256[] memory _leafHashes)  external ;

    function getTreeId() external view returns (uint256);

    function lock(uint256 _treeNumber, uint256 _nullifierId) external returns (bool);

    function isLocked(uint256 _treeNumber, uint256 _nullifierId) external returns (bool);

    function unlock(uint256 _treeNumber, uint256 _nullifierId) external returns (bool);


}