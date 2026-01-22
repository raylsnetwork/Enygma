// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;
// pragma abicoder v2;

import {ERC721} from "@openzeppelin/contracts/token/ERC721/ERC721.sol";
import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";

contract RaylsERC721 is ERC721, AccessControl {
    bytes32 public constant DEFAULT_OWNER_ROLE =
        keccak256(abi.encodePacked("ownerRole"));

    constructor(
        string memory _name,
        string memory _symbol
    ) ERC721(_name, _symbol) AccessControl() {
        _setupRole(DEFAULT_OWNER_ROLE, msg.sender);
    }

    function supportsInterface(
        bytes4 interfaceId
    ) public view virtual override(ERC721, AccessControl) returns (bool) {
        return super.supportsInterface(interfaceId);
    }

    function mint(
        address _to,
        uint256 _tokenId
    ) public onlyRole(DEFAULT_OWNER_ROLE) {
        super._mint(_to, _tokenId);
    }
}
