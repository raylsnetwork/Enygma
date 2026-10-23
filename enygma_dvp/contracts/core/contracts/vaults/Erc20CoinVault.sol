// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;
// pragma abicoder v2;

import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";

import {IEnygmaDvp, WITHDRAW_MESSAGE} from "../../interfaces/IEnygmaDvp.sol";
import {
    PaymentStatementLayout as Psl,
    PAYMENT_LEN,
    PAYMENT2IN_LEN,
    PAYMENT_FEE_LEN,
    PAYMENT_RELAYER_FEE_LEN,
    USDR_FEE_LEN
} from "../PaymentStatementLayout.sol";
import {DvpStatementLayout as Dsl, DVP_INITIATOR_LEN} from "../DvpStatementLayout.sol";
import {DvpReceipt} from "../DvpReceipt.sol";
import {IPoseidonWrapper} from "../../interfaces/IPoseidonWrapper.sol";
import {IVerifier} from "../../interfaces/IVerifier.sol";
import {AbstractCoinVault} from "./AbstractCoinVault.sol";

contract Erc20CoinVault is AbstractCoinVault {
    // SECURITY: transferFrom/transfer return a bool per the ERC20 standard,
    // but several widely-used real tokens (most notably USDT on Ethereum
    // mainnet) don't reliably follow that convention — some return false on
    // failure instead of reverting, others don't return a value at all.
    // Calling these unchecked let a failed transfer still be treated as a
    // successful deposit/withdrawal (Slither: unchecked-transfer). SafeERC20
    // reverts on any of those failure shapes instead of silently continuing.
    using SafeERC20 for IERC20;

    ///////////////////////////////////////////////
    //              Constants
    //////////////////////////////////////////////

    // 1-input / 2-output retail payment circuit (VK slot 0).
    uint256 public constant VK_ID_ERC20_JOINSPLIT = 0;
    // 2-input / 2-output consolidation/payment circuit (VK slot 1).
    // VULN-2 fix: 1-in and 2-in circuits have different R1CS constraint counts and
    // therefore different Groth16 VKs.  They MUST NOT share the same VK slot.
    uint256 public constant VK_ID_ERC20_JOINSPLIT_2INPUT = 1;
    // 1-input / 2-output fee payment circuit (VK slot 2).
    // Same topology as JOINSPLIT but has an extra public StFee.
    uint256 public constant VK_ID_ERC20_JOINSPLIT_FEE = 2;
    // 1-input / 3-output relayer fee payment circuit (VK slot 3).
    // Alice pays Bob (output 0), keeps change (output 1), relayer earns spendable note (output 2).
    uint256 public constant VK_ID_ERC20_JOINSPLIT_RELAYER = 3;
    // 1-input / 2-output USDr circuit (VK slot 4) — a second, independent
    // relayer-fee asset. Same topology as JOINSPLIT_FEE but StTokenId is a
    // PUBLIC signal here (private everywhere else), giving it a statement
    // length (USDR_FEE_LEN) distinct from JOINSPLIT's and JOINSPLIT_FEE's, so
    // it gets its own dispatch case below rather than colliding with either.
    // Fee is paid (output[0], a real spendable note), not burned like FEE's.
    uint256 public constant VK_ID_ERC20_USDR = 4;
    // Former 10-input join-split slot. No longer accepted: its circuit was
    // removed, and its nullifier, Poseidon(sk, pathIndex), differs from the
    // tree-bound one every current circuit uses for notes past the first tree,
    // which would let one note be spent once through each.
    uint256 public constant VK_ID_ERC20_10INPUT = 6;
    // DvP Initiator circuit: circuit id=24 in enygmadvp.config.json → VK slot 23 (0-indexed)
    uint256 public constant VK_ID_DVP_INITIATOR = 23;

    // Exclusive upper bound on the public StFee of a PaymentFee proof: the range
    // bound the circuits put on every note value (TmRange, 10^36 in the gnark
    // handlers). No honest fee reaches it, since a fee is part of an input note's
    // value.
    uint256 public constant MAX_FEE_AMOUNT = 10 ** 36;

    // One row of the Payment-family table (see _paymentCircuit): a circuit's
    // verifying key, its numbers of inputs and outputs, and the offset of each
    // field in its statement layout.
    struct PaymentCircuit {
        uint256 vkId;
        uint256 inputs;
        uint256 outputs;
        uint256 messageOffset;
        uint256 treeNumbersOffset;
        uint256 merkleRootsOffset;
        uint256 nullifiersOffset;
        uint256 commitmentsOutOffset;
        uint256 contractAddressOffset;
    }

    ///////////////////////////////////////////////
    //              Constructor
    //////////////////////////////////////////////

    constructor(
        address zkDvpContractAddress
    ) AbstractCoinVault(zkDvpContractAddress) {
        _name = "ZkDvp - ERC20 Coin Vault";
    }

    // C-1 fix: commitment is now computed on-chain from the caller-supplied key material
    // so the deposited amount is binding.  Previously the caller passed a pre-computed
    // commitment that could encode any amount, allowing vault drain via inflated deposits.
    //
    // params[0] = amount to deposit
    // params[1] = pkSpend  — recipient's BabyJubJub spend public key (x-coordinate)
    // params[2] = salt     — random field element chosen by the depositor
    // params[3] = tokenId  — token identifier (0 for plain ERC20 vaults)
    function deposit(uint256[] memory params) public override returns (bool) {
        uint256 amount  = params[0];
        uint256 pkSpend = params[1];
        uint256 salt    = params[2];
        uint256 tokenId = params[3];

        IERC20(_assetContractAddress).safeTransferFrom(
            msg.sender,
            address(this),
            amount
        );

        // Commitment = Poseidon4(pkSpend, salt, amount, tokenId) — on-chain, binding amount.
        uint256 commitment = IPoseidonWrapper(_hashContractAddress).poseidon4(
            [pkSpend, salt, amount, tokenId]
        );

        uint256[] memory commitments = new uint256[](1);
        commitments[0] = commitment;

        insertLeaves(commitments);

        emit Commitment(_vaultId, commitment);

        return true;
    }

    // depositV2 — non-interactive flow.
    // Same as deposit but also emits EncryptedNote so that the recipient can
    // scan the chain and discover notes sent to them without prior interaction.
    //
    // params[0]    = amount to deposit
    // params[1]    = pkSpend  — recipient's BabyJubJub spend public key (x-coordinate)
    // params[2]    = salt     — random field element chosen by the depositor
    // params[3]    = tokenId  — token identifier (0 for plain ERC20 vaults)
    // ciphertextI  = ML-KEM-768 capsule (1088 bytes)
    // ciphertextII = ChaCha20-Poly1305 ciphertext of tokenId||amount
    function depositV2(
        uint256[] memory params,
        bytes calldata ciphertextI,
        bytes calldata ciphertextII
    ) public returns (bool) {
        uint256 amount  = params[0];
        uint256 pkSpend = params[1];
        uint256 salt    = params[2];
        uint256 tokenId = params[3];

        IERC20(_assetContractAddress).safeTransferFrom(
            msg.sender,
            address(this),
            amount
        );

        // C-1 fix: compute commitment on-chain to bind it to the actual transferred amount.
        uint256 commitment = IPoseidonWrapper(_hashContractAddress).poseidon4(
            [pkSpend, salt, amount, tokenId]
        );

        uint256[] memory commitments = new uint256[](1);
        commitments[0] = commitment;

        insertLeaves(commitments);

        emit Commitment(_vaultId, commitment);
        emit EncryptedNote(_vaultId, commitment, ciphertextI, ciphertextII);

        return true;
    }

    // transfer()/transferV2() are open to any caller, so they must only settle
    // receipts that are complete on their own. A DvP Initiator receipt
    // (non-zero StMessage) is one half of a swap: it must only settle through
    // EnygmaDvp.submitPartialSettlement, together with the counterparty's leg,
    // or anyone holding it (the counterparty, or a mempool observer) could
    // spend the initiator's input note and insert commitB with nothing
    // delivered in return. checkReceiptConditions already refuses every DvP
    // leg (only checkDvpReceiptConditions takes one); this guard is extra
    // protection. Retail transfer proofs carry StMessage == 0 and at least two
    // outputs.
    function _requireStandaloneTransfer(
        IEnygmaDvp.ProofReceipt memory receipt
    ) internal pure {
        if (receipt.statement[Psl.PAYMENT_MESSAGE_OFFSET] != 0) {
            revert IEnygmaDvp.InvalidPaymentMessage();
        }
        if (receipt.numberOfOutputs < 2) {
            revert IEnygmaDvp.InvalidNumberOfOutputs();
        }
    }

    function transfer(
        IEnygmaDvp.ProofReceipt memory receipt
    ) public override nonReentrant returns (bool) {
        _requireStandaloneTransfer(receipt);
        checkReceiptConditions(receipt);
        // NEW-1 fix: nullify inputs before inserting outputs (CEI).
        // Reversed order opened a reentrancy window via the Poseidon precompile
        // call inside insertLeaves() — a re-entrant transfer() would find the
        // input notes un-nullified and could spend them a second time.
        _nullifyFromReceipt(receipt);
        _insertCommitmentsFromReceipt(receipt);
        return true;
    }

    // transferV2 — non-interactive flow.
    // Same as transfer but also emits one EncryptedNote per output commitment
    // so that recipients can scan for notes sent to them.
    //
    // ciphertextI[i]  — ML-KEM capsule for output note i
    // ciphertextII[i] — AEAD ciphertext of tokenId||amount for output note i
    function transferV2(
        IEnygmaDvp.ProofReceipt memory receipt,
        bytes[] calldata ciphertextI,
        bytes[] calldata ciphertextII
    ) public nonReentrant returns (bool) {
        require(
            ciphertextI.length == receipt.numberOfOutputs &&
            ciphertextII.length == receipt.numberOfOutputs,
            "Erc20CoinVault: ciphertext length mismatch"
        );

        _requireStandaloneTransfer(receipt);

        checkReceiptConditions(receipt);
        // NEW-1 fix: nullify inputs before inserting outputs (CEI — matches transfer()).
        _nullifyFromReceipt(receipt);
        _insertCommitmentsFromReceipt(receipt);

        // Emit EncryptedNote for each output so recipients can scan
        PaymentCircuit memory circuit = _paymentCircuit(receipt);
        for (uint256 i = 0; i < circuit.outputs; i++) {
            uint256 commitment = receipt.statement[circuit.commitmentsOutOffset + i];
            emit EncryptedNote(_vaultId, commitment, ciphertextI[i], ciphertextII[i]);
        }

        return true;
    }

    // withdrawV2 — non-interactive flow.
    // Output commitment encodes the recipient as pk_spend with salt=0:
    //   commitment = Poseidon4(uint160(recipient), 0, amount, tokenId)
    //
    // withdrawParams[0] = amount
    // withdrawParams[1] = tokenId (must match the value used when input notes were created)
    //
    // The same as partialWithdraw without the change note's EncryptedNote: it
    // takes the same withdrawal receipt (StMessage WITHDRAW_MESSAGE) and also
    // inserts the change note, so a withdrawal settled here instead never
    // loses its change. It used to accept a payment receipt (StMessage 0) and
    // drop output[1], and it marked inputs spent without refusing locked
    // ones; both made a pending withdrawal destroyable by whoever settled its
    // receipt first through another function.
    function withdrawV2(
        uint256[] memory withdrawParams,
        address recipient,
        IEnygmaDvp.ProofReceipt memory receipt
    ) public nonReentrant returns (bool) {
        _requirePlainPaymentShape(receipt);
        _settlePartialWithdraw(withdrawParams, recipient, receipt, "", "");
        return true;
    }

    // partialWithdraw — take part of a note out of the vault and keep the rest
    // as a new note, e.g. withdraw 10 from a note of 100 and get a note of 90.
    //
    // The receipt is a plain Payment proof (1-in/2-out, VK slot 0, or
    // 2-in/2-out, VK slot 1) bound to this vault:
    //   output[0] = Poseidon4(uint160(recipient), 0, amount, tokenId) — the
    //               withdrawn part; checked here, never inserted, paid out.
    //   output[1] = the change note, Poseidon4(pkSpend, salt, rest, tokenId);
    //               inserted into the tree. The circuit enforces
    //               sum(in) == amount + rest, so the change holds exactly the
    //               remainder.
    // withdrawV2 settles the same receipt without the change note's
    // EncryptedNote.
    //
    // The VK slots below are the dedicated Payment deployment's (and retail's):
    // Payment 0, Payment2in 1, PaymentRelayerFeePublic 3. The main deployment
    // registers other circuits in slots 1-3 (OwnershipErc721, ...), so there
    // only the 1-input form works; a 2-input receipt, and any
    // partialWithdrawWithRelayerFee receipt, fails verification (the
    // statement length does not match those keys) and reverts.
    //
    // withdrawParams[0] = amount
    // withdrawParams[1] = tokenId (must match the value used when input notes were created)
    // ciphertextI/II    = EncryptedNote payload for the change note, so the
    //                     owner's wallet can rediscover it from chain events.
    //                     The proof does not bind them, so whoever settles the
    //                     receipt first can replace them; the circuit therefore
    //                     fixes the change salt to Poseidon(sk, nullifier0), and
    //                     the owner can always rebuild the change note from its
    //                     key and this transaction's Nullifier and
    //                     PartialWithdrawal events (core.WithdrawChangeNote).
    function partialWithdraw(
        uint256[] memory withdrawParams,
        address recipient,
        IEnygmaDvp.ProofReceipt memory receipt,
        bytes calldata ciphertextI,
        bytes calldata ciphertextII
    ) public nonReentrant returns (bool) {
        _requirePlainPaymentShape(receipt);
        _settlePartialWithdraw(withdrawParams, recipient, receipt, ciphertextI, ciphertextII);
        return true;
    }

    // Only the plain Payment circuits: Payment (1 input) or Payment2in, 2
    // outputs. The fee circuits would burn or pay part of the input outside
    // the two outputs.
    function _requirePlainPaymentShape(
        IEnygmaDvp.ProofReceipt memory receipt
    ) internal pure {
        if (receipt.numberOfInputs != 1 && receipt.numberOfInputs != 2) {
            revert InvalidNumberOfInputs();
        }
        if (receipt.numberOfOutputs != 2) {
            revert IEnygmaDvp.InvalidNumberOfOutputs();
        }
        uint256 statementLen = receipt.numberOfInputs == 1 ? PAYMENT_LEN : PAYMENT2IN_LEN;
        if (receipt.statement.length != statementLen) {
            revert InvalidStatmentSize();
        }
    }

    // partialWithdrawWithRelayerFee — partialWithdraw for a relayer, which
    // submits the transaction and is paid from the note instead of the owner
    // paying gas, e.g. withdraw 10 from a note of 100, pay the relayer 1 and
    // keep a note of 89.
    //
    // The receipt is a PaymentRelayerFeePublic proof (1-in/3-out, VK slot 3)
    // bound to this vault:
    //   output[0] = Poseidon4(uint160(recipient), 0, amount, tokenId) — paid out
    //   output[1] = the change note; the circuit requires the sender's own key
    //   output[2] = the relayer's fee note, value StFee
    // and sum(in) == amount + change + StFee. The change and fee notes are
    // inserted. Anyone may submit the receipt: the recipient, the amount and
    // the fee note's owner are all fixed by the proof, so a different
    // submitter cannot redirect anything. The fee is the one the owner proved;
    // a relayer checks that it is acceptable, and addressed to its key, before
    // relaying.
    //
    // withdrawParams, ciphertextI/II: as for partialWithdraw (the ciphertexts
    // are the change note's). The relayer keeps its fee note's opening itself.
    function partialWithdrawWithRelayerFee(
        uint256[] memory withdrawParams,
        address recipient,
        IEnygmaDvp.ProofReceipt memory receipt,
        bytes calldata ciphertextI,
        bytes calldata ciphertextII
    ) public nonReentrant returns (bool) {
        if (receipt.numberOfInputs != 1) {
            revert InvalidNumberOfInputs();
        }
        if (receipt.numberOfOutputs != 3) {
            revert IEnygmaDvp.InvalidNumberOfOutputs();
        }
        if (receipt.statement.length != PAYMENT_RELAYER_FEE_LEN) {
            revert InvalidStatmentSize();
        }
        _settlePartialWithdraw(withdrawParams, recipient, receipt, ciphertextI, ciphertextII);
        return true;
    }

    // Shared by partialWithdraw and partialWithdrawWithRelayerFee once each
    // has checked the receipt's shape: output[0] is the withdrawal, every
    // later output (the change note, then any relayer fee note) is inserted.
    function _settlePartialWithdraw(
        uint256[] memory withdrawParams,
        address recipient,
        IEnygmaDvp.ProofReceipt memory receipt,
        bytes memory ciphertextI,
        bytes memory ciphertextII
    ) internal {
        uint256 amount  = withdrawParams[0];
        uint256 tokenId = withdrawParams[1];

        if (amount == 0) {
            revert ZeroWithdrawAmount();
        }
        PaymentCircuit memory circuit = _paymentCircuit(receipt);
        // A withdrawal proof, not a payment one (see WITHDRAW_MESSAGE).
        if (receipt.statement[circuit.messageOffset] != WITHDRAW_MESSAGE) {
            revert IEnygmaDvp.InvalidPaymentMessage();
        }

        uint256 withdrawCommitment = IPoseidonWrapper(_hashContractAddress).poseidon4(
            [uint256(uint160(recipient)), 0, amount, tokenId]
        );
        if (receipt.statement[circuit.commitmentsOutOffset] != withdrawCommitment) {
            revert InvalidOpening();
        }

        // Checks the vault binding, distinct outputs, roots, unspent
        // nullifiers and the proof.
        checkReceiptConditions(receipt);

        // Effects: nullify the inputs (refusing locked ones), then insert
        // every output but the withdrawal.
        _nullifyFromReceipt(receipt);

        uint256[] memory commitments = new uint256[](circuit.outputs - 1);
        for (uint256 i = 0; i < commitments.length; i++) {
            commitments[i] = receipt.statement[circuit.commitmentsOutOffset + 1 + i];
            emit Commitment(_vaultId, commitments[i]);
        }
        insertLeaves(commitments);

        // Read from the statement, not commitments[0]: insertLeaves uses its
        // argument as scratch space for the tree's inner hashes.
        uint256 changeCommitment = receipt.statement[circuit.commitmentsOutOffset + 1];
        if (ciphertextI.length != 0) {
            emit EncryptedNote(_vaultId, changeCommitment, ciphertextI, ciphertextII);
        }
        emit PartialWithdrawal(_vaultId, recipient, amount, changeCommitment);

        // Interaction: transfer tokens after all state changes
        IERC20(_assetContractAddress).safeTransfer(recipient, amount);
    }

    function withdraw(
        uint256[] memory withdrawParams,
        address recipient,
        IEnygmaDvp.ProofReceipt memory receipt
    ) public override nonReentrant returns (bool) {
        // Legacy uid-commitment withdrawal: output[0] must be
        // Poseidon(uid, recipient). The receipt is Payment-family, read
        // through its circuit's layout.
        uint256 amount = withdrawParams[0];
        PaymentCircuit memory circuit = _paymentCircuit(receipt);

        uint256[] memory assetParams = new uint256[](2);
        assetParams[0] = amount;
        assetParams[1] = uint256(uint160(_assetContractAddress));
        uint256 uid = generateUniqueId(assetParams);

        uint256 commitment = IPoseidonWrapper(_hashContractAddress).poseidon(
            [uid, uint256(uint160(recipient))]
        );

        if (receipt.statement[circuit.commitmentsOutOffset] != commitment) {
            revert InvalidOpening();
        }

        checkReceiptConditions(receipt);

        // Effects: nullify input coins before external transfer (checks-effects-interactions)
        for (uint256 i = 0; i < circuit.inputs; i++) {
            if (receipt.statement[circuit.nullifiersOffset + i] != 0) {
                _spendNullifier(
                    receipt.statement[circuit.treeNumbersOffset + i],
                    receipt.statement[circuit.nullifiersOffset + i]
                );
            }
        }

        // Interaction: transfer tokens after all state changes
        IERC20(_assetContractAddress).safeTransfer(recipient, amount);

        return true;
    }

    ///////////////////////////////////////////////
    //       Generic functions
    //////////////////////////////////////////////
    function generateUniqueId(
        uint256[] memory params
    ) public view override returns (uint256) {
        uint256 amount = params[0];
        return
            IPoseidonWrapper(_hashContractAddress).poseidon(
                [uint256(uint160(_assetContractAddress)), amount]
            );
    }

    // The Payment-family table: the receipt's (inputs, outputs) shape picks
    // a row, and its statement must be that row's circuit's length, before
    // anything is read from it.
    //
    //   (inputs, outputs)  length                   circuit
    //   (1, 2)             PAYMENT_LEN              Payment
    //   (1, 2)             PAYMENT_FEE_LEN          PaymentFee
    //   (1, 2)             USDR_FEE_LEN             UsdrFee
    //   (1, 3)             PAYMENT_RELAYER_FEE_LEN  PaymentRelayerFee
    //   (2, 2)             PAYMENT2IN_LEN           Payment2in
    //
    // Length alone cannot pick the row (PaymentRelayerFee and UsdrFee are
    // both 9 signals), nor tell a payment from a DvP leg (DvpInitiator is as
    // long as PaymentFee), so no DvP leg is in the table: those go to
    // checkDvpReceiptConditions.
    function _paymentCircuit(
        IEnygmaDvp.ProofReceipt memory receipt
    ) internal pure returns (PaymentCircuit memory) {
        if (receipt.numberOfInputs == 2) {
            if (receipt.numberOfOutputs != 2) revert InvalidNumberOfOutputs();
            if (receipt.statement.length != PAYMENT2IN_LEN) revert InvalidStatmentSize();
            return PaymentCircuit({
                vkId: VK_ID_ERC20_JOINSPLIT_2INPUT,
                inputs: Psl.PAYMENT2IN_NULLIFIERS_SIZE,
                outputs: Psl.PAYMENT2IN_COMMITMENTS_OUT_SIZE,
                messageOffset: Psl.PAYMENT2IN_MESSAGE_OFFSET,
                treeNumbersOffset: Psl.PAYMENT2IN_TREE_NUMBERS_OFFSET,
                merkleRootsOffset: Psl.PAYMENT2IN_MERKLE_ROOTS_OFFSET,
                nullifiersOffset: Psl.PAYMENT2IN_NULLIFIERS_OFFSET,
                commitmentsOutOffset: Psl.PAYMENT2IN_COMMITMENTS_OUT_OFFSET,
                contractAddressOffset: Psl.PAYMENT2IN_CONTRACT_ADDRESS_OFFSET
            });
        }
        if (receipt.numberOfInputs != 1) revert InvalidNumberOfInputs();
        if (receipt.numberOfOutputs == 3) {
            if (receipt.statement.length != PAYMENT_RELAYER_FEE_LEN) revert InvalidStatmentSize();
            return PaymentCircuit({
                vkId: VK_ID_ERC20_JOINSPLIT_RELAYER,
                inputs: Psl.PAYMENT_RELAYER_FEE_NULLIFIERS_SIZE,
                outputs: Psl.PAYMENT_RELAYER_FEE_COMMITMENTS_OUT_SIZE,
                messageOffset: Psl.PAYMENT_RELAYER_FEE_MESSAGE_OFFSET,
                treeNumbersOffset: Psl.PAYMENT_RELAYER_FEE_TREE_NUMBERS_OFFSET,
                merkleRootsOffset: Psl.PAYMENT_RELAYER_FEE_MERKLE_ROOTS_OFFSET,
                nullifiersOffset: Psl.PAYMENT_RELAYER_FEE_NULLIFIERS_OFFSET,
                commitmentsOutOffset: Psl.PAYMENT_RELAYER_FEE_COMMITMENTS_OUT_OFFSET,
                contractAddressOffset: Psl.PAYMENT_RELAYER_FEE_CONTRACT_ADDRESS_OFFSET
            });
        }
        if (receipt.numberOfOutputs != 2) revert InvalidNumberOfOutputs();
        if (receipt.statement.length == PAYMENT_LEN) {
            return PaymentCircuit({
                vkId: VK_ID_ERC20_JOINSPLIT,
                inputs: Psl.PAYMENT_NULLIFIERS_SIZE,
                outputs: Psl.PAYMENT_COMMITMENTS_OUT_SIZE,
                messageOffset: Psl.PAYMENT_MESSAGE_OFFSET,
                treeNumbersOffset: Psl.PAYMENT_TREE_NUMBERS_OFFSET,
                merkleRootsOffset: Psl.PAYMENT_MERKLE_ROOTS_OFFSET,
                nullifiersOffset: Psl.PAYMENT_NULLIFIERS_OFFSET,
                commitmentsOutOffset: Psl.PAYMENT_COMMITMENTS_OUT_OFFSET,
                contractAddressOffset: Psl.PAYMENT_CONTRACT_ADDRESS_OFFSET
            });
        }
        if (receipt.statement.length == PAYMENT_FEE_LEN) {
            return PaymentCircuit({
                vkId: VK_ID_ERC20_JOINSPLIT_FEE,
                inputs: Psl.PAYMENT_FEE_NULLIFIERS_SIZE,
                outputs: Psl.PAYMENT_FEE_COMMITMENTS_OUT_SIZE,
                messageOffset: Psl.PAYMENT_FEE_MESSAGE_OFFSET,
                treeNumbersOffset: Psl.PAYMENT_FEE_TREE_NUMBERS_OFFSET,
                merkleRootsOffset: Psl.PAYMENT_FEE_MERKLE_ROOTS_OFFSET,
                nullifiersOffset: Psl.PAYMENT_FEE_NULLIFIERS_OFFSET,
                commitmentsOutOffset: Psl.PAYMENT_FEE_COMMITMENTS_OUT_OFFSET,
                contractAddressOffset: Psl.PAYMENT_FEE_CONTRACT_ADDRESS_OFFSET
            });
        }
        if (receipt.statement.length == USDR_FEE_LEN) {
            return PaymentCircuit({
                vkId: VK_ID_ERC20_USDR,
                inputs: Psl.USDR_FEE_NULLIFIERS_SIZE,
                outputs: Psl.USDR_FEE_COMMITMENTS_OUT_SIZE,
                messageOffset: Psl.USDR_FEE_MESSAGE_OFFSET,
                treeNumbersOffset: Psl.USDR_FEE_TREE_NUMBERS_OFFSET,
                merkleRootsOffset: Psl.USDR_FEE_MERKLE_ROOTS_OFFSET,
                nullifiersOffset: Psl.USDR_FEE_NULLIFIERS_OFFSET,
                commitmentsOutOffset: Psl.USDR_FEE_COMMITMENTS_OUT_OFFSET,
                contractAddressOffset: Psl.USDR_FEE_CONTRACT_ADDRESS_OFFSET
            });
        }
        revert InvalidStatmentSize();
    }

    // Checks a Payment-family receipt; a DvP leg is refused (it is not in the
    // table) and goes to checkDvpReceiptConditions.
    function checkReceiptConditions(
        IEnygmaDvp.ProofReceipt memory receipt
    ) public view override returns (bool) {
        PaymentCircuit memory circuit = _paymentCircuit(receipt);

        // VULN-5 fix: verify the proof is bound to THIS vault contract.
        require(
            receipt.statement[circuit.contractAddressOffset] == uint256(uint160(address(this))),
            "proof not bound to this vault"
        );

        if (
            receipt.statement[circuit.commitmentsOutOffset] ==
            receipt.statement[circuit.commitmentsOutOffset + 1]
        ) {
            revert JoinSplitWithSameCommitments();
        }

        for (uint256 i = 0; i < circuit.inputs; i++) {
            if (receipt.statement[circuit.merkleRootsOffset + i] != 0) {
                if (
                    !isValidRoot(
                        receipt.statement[circuit.treeNumbersOffset + i],
                        receipt.statement[circuit.merkleRootsOffset + i]
                    )
                ) {
                    revert InvalidMerkleRoot();
                }

                if (
                    isValidNullifier(
                        receipt.statement[circuit.treeNumbersOffset + i],
                        receipt.statement[circuit.nullifiersOffset + i]
                    )
                ) {
                    revert InvalidNullifier();
                }
            }
        }

        // PaymentFeeCircuit enforces  sum(in) == sum(out) + StFee  in the
        // field but did not range-check StFee, so a prover could pick
        // StFee = Fr - d and spend a note of value v into outputs worth
        // v + d: value created from nothing, then withdrawn from other
        // depositors' funds. payment() does not look at the fee at all and
        // paymentWithFee() only compares it with a value the caller
        // supplies, so the bound is enforced here, on every path that
        // verifies a receipt of this shape.
        if (
            circuit.vkId == VK_ID_ERC20_JOINSPLIT_FEE &&
            receipt.statement[Psl.PAYMENT_FEE_FEE_OFFSET] >= MAX_FEE_AMOUNT
        ) {
            revert IEnygmaDvp.FeeOutOfRange();
        }

        if (!IVerifier(_verifierContractAddress).verifyProof(
            circuit.vkId,
            receipt.proof,
            receipt.statement
        )) revert InvalidProof();

        return true;
    }

    // Checks a DvpInitiator leg: the fungible side of a swap. It is read only
    // through DvpReceipt and the DvP layout, never by numberOfInputs or
    // numberOfOutputs, and any other receipt is refused before a field is
    // read.
    function checkDvpReceiptConditions(
        IEnygmaDvp.ProofReceipt memory receipt
    ) public view override returns (bool) {
        if (receipt.statement.length != DVP_INITIATOR_LEN) revert InvalidStatmentSize();
        DvpReceipt.Leg memory leg = DvpReceipt.read(receipt.statement);

        if (leg.commitment == receipt.statement[Dsl.DVP_INITIATOR_COMMIT_A_OFFSET]) {
            revert JoinSplitWithSameCommitments();
        }
        if (!isValidRoot(leg.treeNumber, leg.merkleRoot)) {
            revert InvalidMerkleRoot();
        }
        if (isValidNullifier(leg.treeNumber, leg.nullifier)) {
            revert InvalidNullifier();
        }
        if (!IVerifier(_verifierContractAddress).verifyProof(
            VK_ID_DVP_INITIATOR,
            receipt.proof,
            receipt.statement
        )) revert InvalidProof();

        return true;
    }

    // Spends and inserts a Payment-family receipt's notes, read through its
    // circuit's layout (see _paymentCircuit), not by the legacy walker.
    function _nullifyFromReceipt(
        IEnygmaDvp.ProofReceipt memory receipt
    ) internal override returns (bool) {
        PaymentCircuit memory circuit = _paymentCircuit(receipt);
        for (uint256 i = 0; i < circuit.inputs; i++) {
            if (receipt.statement[circuit.merkleRootsOffset + i] != 0) {
                _spendNullifier(
                    receipt.statement[circuit.treeNumbersOffset + i],
                    receipt.statement[circuit.nullifiersOffset + i]
                );
            }
        }
        return true;
    }

    function _insertCommitmentsFromReceipt(
        IEnygmaDvp.ProofReceipt memory receipt
    ) internal override returns (bool) {
        PaymentCircuit memory circuit = _paymentCircuit(receipt);
        uint256 numberOfCommitments = 0;
        for (uint256 i = 0; i < circuit.outputs; i++) {
            if (receipt.statement[circuit.commitmentsOutOffset + i] != 0) {
                numberOfCommitments++;
            }
        }

        uint256[] memory commitments = new uint256[](numberOfCommitments);
        uint256 n = 0;
        for (uint256 i = 0; i < circuit.outputs; i++) {
            uint256 commitment = receipt.statement[circuit.commitmentsOutOffset + i];
            if (commitment != 0) {
                commitments[n++] = commitment;
                emit Commitment(_vaultId, commitment);
            }
        }

        insertLeaves(commitments);

        return true;
    }

    function verifyOwnership(
        uint256[] memory params_,
        IEnygmaDvp.ProofReceipt memory receipt_
    ) public override returns (bool) {
        // params:
        // 0: amount
        uint256 amount = params_[0];

        // A Payment-family receipt whose message is the challenge and whose
        // output[0] is the opened commitment.
        PaymentCircuit memory circuit = _paymentCircuit(receipt_);
        uint256 challenge = receipt_.statement[circuit.messageOffset];

        IEnygmaDvp(_zkDvpContractAddress).checkAndRegisterChallenge(challenge);

        uint256[] memory uparams = new uint256[](2);
        uparams[0] = amount;
        // regenerating uniqueId and commitment to verify
        uint256 uid = generateUniqueId(uparams);

        uint256 commitment = IPoseidonWrapper(_hashContractAddress).poseidon(
            [uid, uint256(uint160(0))]
        );

        if (receipt_.statement[circuit.commitmentsOutOffset] != commitment) {
            revert InvalidOpening();
        }

        // checking generic conditions of Ownership receipt.

        checkReceiptConditions(receipt_);
        // firing the receipt event
        emit OwnershipVerificationReceipt(
            challenge,
            _vaultId,
            0, // there is no token Id for erc20
            amount
        );

        return true;
    }
}
