// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;

import {IEnygmaDvp} from "../IEnygmaDvp.sol";
interface IEnygmaErc20CoinVault {
    function depositThroughEnygma(
        uint256[] memory depositParams
    ) external virtual returns (bool, uint256);

    function withdrawThroughEnygma(
        IEnygmaDvp.ProofReceipt memory receipt
    ) external virtual returns (bool);
}
