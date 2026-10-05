// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;

import {IEnygmaDvp} from "../interfaces/IEnygmaDvp.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/security/ReentrancyGuard.sol";

/// @title SwapRelayer
/// @notice On-chain relayer that collects both parties' ProofReceipts and
///         submits them atomically to EnygmaDvp.swap().
///
/// Flow:
///   1. Alice calls submitReceipt(swapId, aliceReceipt, isPayment=true, ...)
///        → nullifiers locked immediately via EnygmaDvp.lockReceiptNullifiers()
///        → emits SwapReceiptSubmitted with ctI/ctII so Bob can discover the swap
///   2. Bob scans chain, decapsulates ctI, generates his proof
///   3. Bob calls submitReceipt(swapId, bobReceipt, isPayment=false, ...)
///        → nullifiers locked
///        → both sides present → EnygmaDvp.swap() called atomically ✓
///
/// Cancellation:
///   If the counterparty never submits, the initiator can call cancelSwap()
///   after expiry to unlock their nullifiers and recover their note.
///
/// swapId derivation (off-chain, both parties compute independently):
///   swapId = keccak256(abi.encode(commitmentB, C'))
///   where commitmentB and C' are the pre-computed cross-commitments from Step 3.
contract SwapRelayer is ReentrancyGuard {

    struct PendingSwap {
        IEnygmaDvp.ProofReceipt paymentReceipt;
        IEnygmaDvp.ProofReceipt deliveryReceipt;
        uint256 paymentVaultId;
        uint256 deliveryVaultId;
        address paymentParty;
        address deliveryParty;
        bool    paymentSubmitted;
        bool    deliverySubmitted;
        uint256 expiry;
    }

    IEnygmaDvp public immutable dvp;

    mapping(bytes32 => PendingSwap) public swaps;

    event SwapReceiptSubmitted(
        bytes32 indexed swapId,
        bool            isPayment,
        bytes           ctI,
        bytes           ctII
    );
    event SwapSettled(bytes32 indexed swapId);
    event SwapCancelled(bytes32 indexed swapId);

    error SwapExpired();
    error AlreadySubmitted();
    error SwapNotExpiredYet();
    error NothingToCancel();
    error NotYourSwap();
    error BothSidesAlreadyIn();

    constructor(address dvpAddress) {
        dvp = IEnygmaDvp(dvpAddress);
    }

    /// @notice Submit one leg of a swap.
    /// @param swapId        keccak256(abi.encode(commitmentB, C')) — agreed off-chain
    /// @param receipt       The ZK ProofReceipt for this leg
    /// @param isPayment     true = payment (fungible) leg, false = delivery (non-fungible) leg
    /// @param vaultId       Vault index for this receipt (0=ERC20, 1=ERC721, 2=ERC1155)
    /// @param expiry        Unix timestamp after which the initiator can cancel (set on first call)
    /// @param ctI           KEM capsule — emitted so counterparty can discover the swap on-chain
    /// @param ctII          Encrypted payload — emitted alongside ctI
    function submitReceipt(
        bytes32                          swapId,
        IEnygmaDvp.ProofReceipt calldata receipt,
        bool                             isPayment,
        uint256                          vaultId,
        uint256                          expiry,
        bytes   calldata                 ctI,
        bytes   calldata                 ctII
    ) external nonReentrant {
        PendingSwap storage s = swaps[swapId];

        if (s.expiry != 0 && block.timestamp >= s.expiry) revert SwapExpired();

        if (isPayment) {
            if (s.paymentSubmitted) revert AlreadySubmitted();
            s.paymentReceipt   = receipt;
            s.paymentVaultId   = vaultId;
            s.paymentParty     = msg.sender;
            s.paymentSubmitted = true;
        } else {
            if (s.deliverySubmitted) revert AlreadySubmitted();
            s.deliveryReceipt   = receipt;
            s.deliveryVaultId   = vaultId;
            s.deliveryParty     = msg.sender;
            s.deliverySubmitted = true;
        }

        // set expiry on first submission
        if (s.expiry == 0) {
            // LOW-6 fix: reject zero or past expiry so the first submitter cannot
            // immediately cancel their own leg via cancelSwap (expiry=0 passes the
            // "block.timestamp < s.expiry" check as false for any uint256 timestamp).
            require(expiry > block.timestamp, "SwapRelayer: expiry must be in the future");
            s.expiry = expiry;
        }

        // lock nullifiers immediately — prevents double-spend during the gap
        dvp.lockReceiptNullifiers(receipt, vaultId);

        // emit ctI/ctII so the counterparty can discover the swap by scanning
        emit SwapReceiptSubmitted(swapId, isPayment, ctI, ctII);

        // if both sides are in → validate vault pair then settle atomically
        if (s.paymentSubmitted && s.deliverySubmitted) {
            // HIGH-11 fix: verify (paymentVaultId, deliveryVaultId) is a registered
            // swap pair before settling. Without this check the second submitter can
            // route the settlement through an unrelated vault of their choosing.
            require(
                dvp.isRegisteredSwapGroupPair(s.paymentVaultId, s.deliveryVaultId),
                "SwapRelayer: vault pair not registered"
            );
            // H-6 fix: unlock both nullifier sets before calling swap().
            // lockReceiptNullifiers() was called when each leg was submitted.
            // setNullifier() (inside _nullifyFromReceipt) checks lockedNullifiers
            // and reverts with CantSpendLockedCoin if the nullifier is still locked —
            // so settlement would always fail without unlocking first.
            dvp.unlockReceiptNullifiers(s.paymentReceipt, s.paymentVaultId);
            dvp.unlockReceiptNullifiers(s.deliveryReceipt, s.deliveryVaultId);
            dvp.swap(
                s.paymentReceipt,
                s.deliveryReceipt,
                s.paymentVaultId,
                s.deliveryVaultId
            );
            delete swaps[swapId];
            emit SwapSettled(swapId);
        }
    }

    /// @notice Cancel a pending swap after expiry.
    ///         Unlocks the caller's nullifiers so their note can be used elsewhere.
    ///         Only callable by the party who submitted their leg, after expiry.
    function cancelSwap(bytes32 swapId) external nonReentrant {
        PendingSwap storage s = swaps[swapId];

        if (block.timestamp < s.expiry) revert SwapNotExpiredYet();
        if (!s.paymentSubmitted && !s.deliverySubmitted) revert NothingToCancel();
        if (s.paymentSubmitted && s.deliverySubmitted) revert BothSidesAlreadyIn();

        if (s.paymentSubmitted && msg.sender == s.paymentParty) {
            dvp.unlockReceiptNullifiers(s.paymentReceipt, s.paymentVaultId);
        } else if (s.deliverySubmitted && msg.sender == s.deliveryParty) {
            dvp.unlockReceiptNullifiers(s.deliveryReceipt, s.deliveryVaultId);
        } else {
            revert NotYourSwap();
        }

        delete swaps[swapId];
        emit SwapCancelled(swapId);
    }
}
