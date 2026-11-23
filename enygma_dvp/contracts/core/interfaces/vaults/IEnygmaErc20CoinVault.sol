// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;

interface IEnygmaErc20CoinVault {
    // withdrawThroughEnygma spent this note.
    event EnygmaNoteSpent(uint256 indexed vaultId, uint256 indexed commitment);

    // The commitment is not an unspent note depositThroughEnygma inserted.
    error UnknownEnygmaNote();

    function depositThroughEnygma(
        uint256[] memory depositParams
    ) external virtual returns (bool, uint256);

    function withdrawThroughEnygma(
        uint256 commitment
    ) external virtual returns (bool);
}
