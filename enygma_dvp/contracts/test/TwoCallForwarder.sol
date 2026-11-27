// SPDX-License-Identifier: BUSL-1.1
pragma solidity ^0.8.0;

/// @notice Test-only: forwards calls, one or two in a transaction, as
///         itself. A vault deployed with it as its DvP (and initialized
///         with it as its auction) takes its calls to the role-gated
///         functions, so a test can make two inserts into the vault in one
///         transaction, as any contract making two deposits in one call can.
contract TwoCallForwarder {
    function forward(address target, bytes calldata data) external {
        _call(target, data);
    }

    function forwardTwice(address target, bytes calldata first, bytes calldata second) external {
        _call(target, first);
        _call(target, second);
    }

    // Calls target with data, bubbling up its revert data.
    function _call(address target, bytes calldata data) private {
        (bool ok, bytes memory ret) = target.call(data);
        if (!ok) {
            assembly {
                revert(add(ret, 32), mload(ret))
            }
        }
    }
}
