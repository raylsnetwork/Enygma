// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;

interface IMerkle {
    // Fired by insertLeaves for each leaf it inserts, once it has decided
    // whether the batch starts a new tree: vaultId is the id the tree was
    // initialized with (a vault's id; an asset group's tree logs its own),
    // commitment the leaf, and treeNumber and leafIndex where it went. The
    // vault decides the rollover once per insertLeaves call, and one
    // transaction can make several calls, so the Commitment event, which
    // carries no position, does not say where a leaf went: a wallet
    // rebuilds a vault's trees from these positions.
    event LeafInserted(
        uint256 indexed vaultId,
        uint256 indexed commitment,
        uint256 treeNumber,
        uint256 leafIndex
    );

    // insertLeaves was given no leaves.
    error EmptyLeafBatch();
    // insertLeaves was given count leaves, more than a fresh tree takes
    // short of its last slot, which no insert fills.
    error LeafBatchTooLarge(uint256 count);

    function hashLeftRight(
        uint256 _left,
        uint256 _right
    ) external view returns (uint256);

    function isValidNullifier(
        uint256 _treeNumber,
        uint256 _nullifierId
    ) external view returns (bool);

    function isValidRoot(
        uint256 _treeNumber,
        uint256 _merkleRoot
    ) external view returns (bool);

    function currentRoot() external view returns (uint256);

    function getTreeId() external view returns (uint256);

    function getNextLeafIndex() external view returns (uint256);

    function isLocked(
        uint256 _treeNumber,
        uint256 _nullifierId
    ) external returns (bool);
}
