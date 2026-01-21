// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;
// pragma abicoder v2;

import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";

contract RaylsERC20 is ERC20, AccessControl {

    bytes32 public constant DEFAULT_OWNER_ROLE =
        keccak256(abi.encodePacked("ownerRole"));

    // solhint-disable-next-line no-empty-blocks
    constructor(string memory _name, string memory _symbol)
        ERC20(_name, _symbol) AccessControl()
    { 
      _setupRole(DEFAULT_OWNER_ROLE, msg.sender);
    }

    function mint(address to_, uint256 value_) onlyRole(DEFAULT_OWNER_ROLE) public {
        super._mint(to_, value_);
    }
}
