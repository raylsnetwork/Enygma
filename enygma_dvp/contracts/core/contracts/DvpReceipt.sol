// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.24;

import {IEnygmaDvp} from "../interfaces/IEnygmaDvp.sol";
import {DvpStatementLayout as Dsl, DVP_INITIATOR_LEN, DVP_DESTINATION_LEN} from "./DvpStatementLayout.sol";

/// @notice Reads one leg of a DvP swap by name. A leg is proved by
///         DvpInitiator (it opens a swap) or DvpDestination (it completes
///         one), and the two are told apart by statement length, which is
///         checked before any field is read. Length alone does not prove a
///         receipt is a DvP leg (PaymentFee's statement is as long as
///         DvpInitiator's): the vault that verifies the receipt pins its
///         circuit.
library DvpReceipt {
    /// The fields both DvP circuits carry.
    struct Leg {
        // Message: the commitment the counterparty's leg inserts.
        uint256 message;
        // The commitment this leg inserts: an initiator's CommitB, a
        // destination's CommitA.
        uint256 commitment;
        // CounterVault: the vault this leg's prover is paid from.
        uint256 counterVault;
    }

    function isLeg(uint256[] memory statement) internal pure returns (bool) {
        return statement.length == DVP_INITIATOR_LEN || statement.length == DVP_DESTINATION_LEN;
    }

    /// Reverts with InvalidStatementSize unless statement is a DvP leg's.
    function read(uint256[] memory statement) internal pure returns (Leg memory) {
        if (statement.length == DVP_INITIATOR_LEN) {
            return Leg(
                statement[Dsl.DVP_INITIATOR_MESSAGE_OFFSET],
                statement[Dsl.DVP_INITIATOR_COMMIT_B_OFFSET],
                statement[Dsl.DVP_INITIATOR_COUNTER_VAULT_OFFSET]
            );
        }
        if (statement.length == DVP_DESTINATION_LEN) {
            return Leg(
                statement[Dsl.DVP_DESTINATION_MESSAGE_OFFSET],
                statement[Dsl.DVP_DESTINATION_COMMIT_A_OFFSET],
                statement[Dsl.DVP_DESTINATION_COUNTER_VAULT_OFFSET]
            );
        }
        revert IEnygmaDvp.InvalidStatementSize();
    }
}
