// SPDX-License-Identifier: BUSL-1.1
pragma solidity ^0.8.0;

import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";

/// @notice Test-only: an ERC20 that mints the recipient 1% more than every
///         transfer's amount, so the recipient receives more than was sent
///         (#432).
contract BonusOnTransferERC20 is ERC20 {
    constructor() ERC20("Bonus", "BNS") {}

    function mint(address to, uint256 amount) external {
        _mint(to, amount);
    }

    function _transfer(address from, address to, uint256 amount) internal override {
        super._transfer(from, to, amount);
        _mint(to, amount / 100);
    }
}
