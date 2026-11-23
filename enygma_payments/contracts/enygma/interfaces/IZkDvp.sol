// SPDX-License-Identifier: GPL3
pragma solidity ^0.8.0;

interface IZkDvp {
    function depositERC20(
        uint256 _amount,
        address _erc20Address,
        uint256 _publicKey
    ) external returns (bool);

    function depositThroughEnygma(
        uint256[] memory depositParams
    ) external returns (bool, uint256);

    // Spends one unspent note depositThroughEnygma inserted with this
    // commitment; reverts if there is none.
    function withdrawThroughEnygma(
        uint256 commitment
    ) external returns (bool);
}
