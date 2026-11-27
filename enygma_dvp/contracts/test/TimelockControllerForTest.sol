// SPDX-License-Identifier: BUSL-1.1
pragma solidity ^0.8.0;

/// @notice Test-only: has Hardhat build OpenZeppelin's TimelockController, so
///         test/hardhat/OwnerGovernance.test.js can make one EnygmaDvp's owner.
import {TimelockController} from "@openzeppelin/contracts/governance/TimelockController.sol";
