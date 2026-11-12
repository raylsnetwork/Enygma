// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;
// pragma abicoder v2;

import {IERC721} from "@openzeppelin/contracts/token/ERC721/IERC721.sol";
import {IERC1155} from "@openzeppelin/contracts/token/ERC1155/IERC1155.sol";
import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/security/ReentrancyGuard.sol";

import {IEnygmaDvp, WITHDRAW_MESSAGE, paymentCommitmentsOutOffset} from "../interfaces/IEnygmaDvp.sol";
import {PaymentStatementLayout as Psl} from "./PaymentStatementLayout.sol";
import {DvpStatementLayout as Dsl, DVP_INITIATOR_LEN} from "./DvpStatementLayout.sol";
import {DvpReceipt} from "./DvpReceipt.sol";
import {IEnygmaAuction} from "../interfaces/IEnygmaAuction.sol";
import {IAbstractCoinVault} from "../interfaces/vaults/IAbstractCoinVault.sol";
import {IMerkle} from "../interfaces/vaults/IMerkle.sol";
import {IAssetGroup} from "../interfaces/vaults/IAssetGroup.sol";
import {IPoseidonWrapper} from "../interfaces/IPoseidonWrapper.sol";
import {IVerifier} from "../interfaces/IVerifier.sol";
import {IPrivateMintVerifier} from "../interfaces/IPrivateMintVerifier.sol";

contract EnygmaDvp is IEnygmaDvp, AccessControl, ReentrancyGuard {
    ///////////////////////////////////////////////
    //              Constants
    //////////////////////////////////////////////

    uint256 constant SNARK_SCALAR_FIELD =
        21888242871839275222246405745257275088548364400416034343698204186575808495617;

    // Hard-corded asset Ids
    uint256 constant VAULT_ID_ERC20 = 0;
    uint256 constant VAULT_ID_ERC721 = 1;
    uint256 constant VAULT_ID_ERC1155 = 2;
    uint256 constant VAULT_ID_ENYGMA = 3;
    uint256 constant MAX_NUMBER_OF_VAULTS = 1000;

    uint256 constant GROUP_ID_FUNGIBLES = 0;
    uint256 constant GROUP_ID_NON_FUNGIBLES = 1;

    uint256 constant MAX_NUMBER_OF_GROUPS = 1000;

    // Index of verification keys that has been
    // used directly in EnygmadDvp
    uint256 public constant VK_ID_AUCTION_INIT = 6;
    uint256 public constant VK_ID_AUCTION_BID = 7;
    uint256 public constant VK_ID_AUCTION_PRIVATE_OPENING = 8;
    uint256 public constant VK_ID_AUCTION_NOT_WINNING_BID = 9;

    // Upper bound on a pending swap's deadline, measured from the first-leg
    // submission. The deadline is a plain argument (not a public signal of the
    // initiator proof), so whoever submits the first leg picks it; without a
    // ceiling a front-runner could lock the initiator's note until an
    // arbitrarily distant timestamp.
    uint256 public constant MAX_SWAP_DURATION = 30 days;

    // Floor on the same deadline. claimSwapTimeout is callable by anyone, so
    // without it a front-runner who submits the initiator's public receipt
    // first could set a deadline a second away and cancel the swap in the
    // next block, before the counterparty can deliver. The counterparty always
    // gets at least this long.
    uint256 public constant MIN_SWAP_DURATION = 15 minutes;

    bytes32 public constant DEFAULT_OWNER_ROLE =
        keccak256(abi.encodePacked("ownerRole"));

    bytes32 public constant DEFAULT_AUCTION_ROLE =
        keccak256(abi.encodePacked("AuctionRole"));
    ///////////////////////////////////////////////
    //           Private attributes
    //////////////////////////////////////////////

    // name identifier for Enyg,aDvp smart contract
    string private _name;

    // the address of PoseidonWrapper smart contract
    address private _hashContractAddress;
    // address of generic groth16 verifier smart contract
    address private _genericVerifierContractAddress;
    // address of non-generic verifier that pack the proofs
    // and utilizes the generic Gorth16 verifier smart contract
    address private _verifierContractAddress;

    address private _enygmaAuctionContractAddress;

    // address of private mint verifier smart contract
    address private _privateMintVerifierAddress;

    // mapping from merkleTreeId to merkleTreeAddress
    // to keep track of the asset merkle trees.
    address[] private _coinVaults;
    uint256 _coinVaultsCount;

    mapping(uint256 => bool) private _rottenChallenges;

    mapping(uint256 => bool) private _swapGroupPairs;
    mapping(uint256 => bool) private _exchangeGroupPairs;
    // HIGH-11 fix: direct vault-pair registry populated alongside group-pair registration.
    mapping(uint256 => bool) private _registeredVaultPairs;

    mapping(uint256 => AuctionData) private _auctions;

    address[] private _assetGroups;
    uint256 _assetGroupsCount;

    // mapping from uniqueUTXO id and the vaultId
    // we are using the first output commitment of the receipt as the
    // uniqueId of the receipt
    mapping(uint256 => TransactionMetadata) private _pendingTransactions;

    // Swaps (keyed like _pendingTransactions, by commitB) that have settled or
    // been reclaimed via claimSwapTimeout. Deleting the pending record alone let
    // a late counterparty leg fall through to the first-leg branch and open a
    // new swap that could never settle, locking that leg's note until its own
    // deadline.
    mapping(uint256 => bool) private _closedSwaps;

    // auditor's mapping AuditorId -> AuditorData
    mapping(uint256 => AuditorData) private _registeredAuditors;

    // authorized on-chain relayer contracts (set by owner)
    mapping(address => bool) public authorizedRelayers;

    // Governance-configured fixed fee that every paymentWithRelayerFee() proof's
    // public StFee signal must equal — see setRelayerFixedFee / paymentWithRelayerFee.
    uint256 public relayerFixedFeeAmount;

    // USDr — a second, independent relayer-fee asset (its own vault/token,
    // its own circuit — see UsdrFeeCircuit / paymentWithUsdrFee).
    // usdrFixedFeeAmount is the governance-configured fee amount every
    // UsdrFee proof's public StFee signal must equal. usdrTokenId is the
    // fixed convention value (0, matching every other circuit's WtTokenId
    // convention in this codebase) the USDr circuit's public StTokenId
    // signal must equal — config hygiene, not a security-critical check,
    // since each vault already has its own independent tree/nullifier space.
    uint256 public usdrFixedFeeAmount;
    uint256 public usdrTokenId;
    // The vault that holds the USDr asset. paymentWithUsdrFee() only accepts a
    // fee leg against this vault: the proof binds the fee note to whichever
    // vault it was built for, so without this pin a sender could pay the "USDr"
    // fee in any other vault whose proof shape matches (for instance the main
    // token's own vault), and the relayer would be paid in the wrong asset.
    // usdrFeeVaultSet distinguishes "unset" from vault id 0.
    uint256 public usdrFeeVaultId;
    bool public usdrFeeVaultSet;
    ///////////////////////////////////////////////
    //              Constructor
    //////////////////////////////////////////////

    // hashContractAddress: poseidon Wrapper contract address
    // genericVerifierContractAddress: Groth16 generic verifier address.
    constructor(
        address hashContractAddress,
        address genericVerifierContractAddress
    ) AccessControl() {
        _name = "EnygmaDVP smart contract";
        _hashContractAddress = hashContractAddress;
        _genericVerifierContractAddress = genericVerifierContractAddress;
        _coinVaults = new address[](MAX_NUMBER_OF_VAULTS);
        _assetGroups = new address[](MAX_NUMBER_OF_GROUPS);
        _setupRole(DEFAULT_OWNER_ROLE, msg.sender);
    }

    ///////////////////////////////////////////////
    //              Public Getters
    //////////////////////////////////////////////
    function name() public view returns (string memory) {
        return _name;
    }

    // Get merkleTree by the hard-coded IDs
    function vaultById(uint256 treeId) public view returns (address) {
        return _coinVaults[treeId];
    }

    function erc721Vault() public view returns (address) {
        return _coinVaults[VAULT_ID_ERC721];
    }

    function erc20Vault() public view returns (address) {
        return _coinVaults[VAULT_ID_ERC20];
    }

    function erc1155Vault() public view returns (address) {
        return _coinVaults[VAULT_ID_ERC1155];
    }

    function enygmaVault() public view returns (address) {
        return _coinVaults[VAULT_ID_ENYGMA];
    }

    function hashContractAddress() public view returns (address) {
        return _hashContractAddress;
    }

    function verifierContractAddress() public view returns (address) {
        return _verifierContractAddress;
    }

    ///////////////////////////////////////////////
    //       initialization functions
    //////////////////////////////////////////////
    function initializeDvp(
        address verifierAddress
    ) public onlyRole(DEFAULT_OWNER_ROLE) returns (bool) {
        // DVP-1 fix: Verifier is now initialized directly by the deployer (via init.go)
        // before this function is called. initializeVerifier is no longer called here
        // because EnygmaDvp.initializeDvp was world-accessible between Verifier deployment
        // and the first initializeDvp call, allowing an attacker to front-run initialization.
        _verifierContractAddress = verifierAddress;
        return true;
    }

    function registerEnygmaAuction(
        address enygmaAuctionContractAddress
    ) public onlyRole(DEFAULT_OWNER_ROLE) returns (bool) {
        _enygmaAuctionContractAddress = enygmaAuctionContractAddress;

        // initializing the verifier by registering Snark circuits' verification keys.
        // and the genericGroth16 verifier
        IEnygmaAuction(_enygmaAuctionContractAddress).initializeEnygmaAuction(
            _hashContractAddress,
            _verifierContractAddress
        );

        return true;
    }

    function registerRelayer(
        address relayer
    ) public onlyRole(DEFAULT_OWNER_ROLE) returns (bool) {
        authorizedRelayers[relayer] = true;
        return true;
    }

    function deregisterRelayer(
        address relayer
    ) public onlyRole(DEFAULT_OWNER_ROLE) returns (bool) {
        authorizedRelayers[relayer] = false;
        return true;
    }

    function registerPrivateMintVerifier(
        address privateMintVerifierAddress
    ) public onlyRole(DEFAULT_OWNER_ROLE) returns (bool) {
        _privateMintVerifierAddress = privateMintVerifierAddress;
        return true;
    }

    function registerNewVerificationKey(
        VerifyingKey memory vk_
    ) public onlyRole(DEFAULT_OWNER_ROLE) returns (bool) {
        IVerifier(_verifierContractAddress).addVerificationKey(vk_);
        return true;
    }

    function registerAssetGroup(
        address assetGroupContractAddress,
        string memory assetGroupName,
        bool isAssetGroupFungible,
        uint256 treeDepth
    ) public onlyRole(DEFAULT_OWNER_ROLE) nonReentrant returns (bool) {
        uint256 groupId = _assetGroupsCount;
        _assetGroups[groupId] = assetGroupContractAddress;
        IAssetGroup(assetGroupContractAddress).initializeAssetGroup(
            groupId,
            assetGroupName,
            isAssetGroupFungible,
            treeDepth
        );

        _assetGroupsCount++;
        return true;
    }

    function registerVault(
        address vaultContractAddress,
        address assetContractAddress,
        uint256 vaultIdentifiersCount,
        uint256 treeDepth
    ) public onlyRole(DEFAULT_OWNER_ROLE) nonReentrant returns (bool) {
        // registering the tree smart contract address and treeId

        uint256 vaultId = _coinVaultsCount;
        _coinVaults[vaultId] = vaultContractAddress;

        // initializing CoinVault
        IAbstractCoinVault(_coinVaults[vaultId]).initializeVault(
            vaultId,
            vaultIdentifiersCount,
            assetContractAddress,
            treeDepth,
            _hashContractAddress,
            _verifierContractAddress,
            _enygmaAuctionContractAddress
        );

        _coinVaultsCount++;

        return true;
    }

    // setRelayerFixedFee configures the fixed fee every paymentWithRelayerFee()
    // proof's public StFee signal is checked against on-chain (see that
    // function below). Without this, any fee amount the prover claims in the
    // proof passes verification — the circuit only proves internal consistency
    // (WtValuesOut[2] == StFee), not that the amount matches any protocol value.
    function setRelayerFixedFee(
        uint256 amount
    ) external onlyRole(DEFAULT_OWNER_ROLE) returns (bool) {
        relayerFixedFeeAmount = amount;
        return true;
    }

    // setUsdrFixedFee / setUsdrTokenId configure paymentWithUsdrFee()'s two
    // on-chain checks against the USDr proof's public StFee/StTokenId
    // signals — see that function below.
    function setUsdrFixedFee(
        uint256 amount
    ) external onlyRole(DEFAULT_OWNER_ROLE) returns (bool) {
        usdrFixedFeeAmount = amount;
        return true;
    }

    function setUsdrTokenId(
        uint256 tokenId_
    ) external onlyRole(DEFAULT_OWNER_ROLE) returns (bool) {
        usdrTokenId = tokenId_;
        return true;
    }

    // setUsdrFeeVaultId pins the vault paymentWithUsdrFee() accepts for its
    // USDr leg. It must already be registered. Until it is called,
    // paymentWithUsdrFee() reverts InvalidUsdrVault.
    function setUsdrFeeVaultId(
        uint256 vaultId_
    ) external onlyRole(DEFAULT_OWNER_ROLE) returns (bool) {
        if (_coinVaults[vaultId_] == address(0)) revert InvalidVaultId();
        usdrFeeVaultId = vaultId_;
        usdrFeeVaultSet = true;
        return true;
    }

    function registerAuditor(
        uint256 auditorOffchainId,
        uint256 auditorGroupId,
        uint256[2] memory auditorPublicKey
    ) public onlyRole(DEFAULT_OWNER_ROLE) returns (bool) {
        // TODO:: check auditorPublicKey be on curve.

        uint256 auditorOnchainId = uint256(
            keccak256(
                abi.encodePacked(auditorPublicKey[0], auditorPublicKey[1])
            )
        );

        // Auditor has not been registered
        if (_registeredAuditors[auditorOnchainId].auditorPublicKey[0] == 0) {
            AuditorData memory newAuditor;
            newAuditor.auditorOffchainId = auditorOffchainId;
            newAuditor.auditorGroupId = auditorGroupId;
            newAuditor.auditorPublicKey = auditorPublicKey;
            _registeredAuditors[auditorOnchainId] = newAuditor;

            emit AuditorRegistered(
                auditorOnchainId,
                auditorOffchainId,
                auditorGroupId,
                auditorPublicKey
            );
        } else {
            revert AuditorAlreadyRegistered(auditorOffchainId, auditorGroupId);
        }

        return true;
    }

    function unregisterAuditor(
        uint256 auditorOnchainId
    ) public onlyRole(DEFAULT_OWNER_ROLE) returns (bool) {
        if (_registeredAuditors[auditorOnchainId].auditorPublicKey[0] == 0) {
            revert AuditorNotRegistered(auditorOnchainId);
        } else {
            delete _registeredAuditors[auditorOnchainId];
            emit AuditorUnregistered(auditorOnchainId);
        }

        return true;
    }

    function isAuditorRegistered(
        uint256[2] memory publicKey
    ) public view returns (bool) {
        uint256 onchainAuditorId = uint256(
            keccak256(abi.encodePacked(publicKey[0], publicKey[1]))
        );

        if (_registeredAuditors[onchainAuditorId].auditorPublicKey[0] != 0) {
            return true;
        } else {
            return false;
        }
    }
    ///////////////////////////////////////////////
    //       Asset Group functions
    //////////////////////////////////////////////

    // HIGH-1 fix: restricted to owner — any caller could register arbitrary swap pairs.
    function registerSwapGroupPair(
        uint256 groupId1,
        uint256 groupId2
    ) public onlyRole(DEFAULT_OWNER_ROLE) returns (bool) {
        if (groupId1 >= _assetGroupsCount || groupId2 >= _assetGroupsCount) {
            revert GroupIdOutOfRange();
        }

        // checking group 1 to be fungible
        IAssetGroup assetGroup1 = IAssetGroup(_assetGroups[groupId1]);
        if (!assetGroup1.isFungible()) {
            revert GroupFungibilityMismatch();
        }

        // checking group 2 not to be fungible
        IAssetGroup assetGroup2 = IAssetGroup(_assetGroups[groupId2]);
        if (assetGroup2.isFungible()) {
            revert GroupFungibilityMismatch();
        }

        // generating the groupPairId
        uint256 groupPairId = uint256(
            keccak256(abi.encodePacked(groupId1, groupId2))
        );

        // if already registered revert
        if (_swapGroupPairs[groupPairId]) {
            revert GroupPairAlreadyRegistered();
        } else {
            // if not already registered, register it
            _swapGroupPairs[groupPairId] = true;
        }

        return true;
    }

    function registerExchangeGroupPair(
        uint256 groupId1,
        uint256 groupId2
    ) public onlyRole(DEFAULT_OWNER_ROLE) returns (bool) {
        if (groupId1 >= _assetGroupsCount || groupId2 >= _assetGroupsCount) {
            revert GroupIdOutOfRange();
        }

        // checking group 1 to be fungible
        IAssetGroup assetGroup1 = IAssetGroup(_assetGroups[groupId1]);
        if (!assetGroup1.isFungible()) {
            revert GroupFungibilityMismatch();
        }

        // checking group 2 to be fungible
        IAssetGroup assetGroup2 = IAssetGroup(_assetGroups[groupId2]);
        if (!assetGroup2.isFungible()) {
            revert GroupFungibilityMismatch();
        }

        uint256 groupPairId = groupPairId(groupId1, groupId2);
        if (_exchangeGroupPairs[groupPairId]) {
            revert GroupPairAlreadyRegistered();
        } else {
            _exchangeGroupPairs[groupPairId] = true;
        }

        return true;
    }

    function isValidSwapGroupPair(
        uint256 groupId1,
        uint256 groupId2
    ) public view returns (bool) {
        uint256 groupPairId = groupPairId(groupId1, groupId2);
        return _swapGroupPairs[groupPairId];
    }

    // HIGH-11 fix: vault-level pair check used by SwapRelayer to validate the
    // (paymentVaultId, deliveryVaultId) combination before settling.
    function isRegisteredSwapGroupPair(
        uint256 paymentVaultId,
        uint256 deliveryVaultId
    ) external view returns (bool) {
        uint256 pairKey = uint256(keccak256(abi.encodePacked(paymentVaultId, deliveryVaultId)));
        return _registeredVaultPairs[pairKey];
    }

    // registerVaultSwapPair is called by the owner after deploying a new vault pair
    // to authorise it as a valid payment→delivery combination in SwapRelayer.
    function registerVaultSwapPair(
        uint256 paymentVaultId,
        uint256 deliveryVaultId
    ) public onlyRole(DEFAULT_OWNER_ROLE) {
        uint256 pairKey = uint256(keccak256(abi.encodePacked(paymentVaultId, deliveryVaultId)));
        _registeredVaultPairs[pairKey] = true;
    }

    function isValidExchangeGroupPair(
        uint256 groupId1,
        uint256 groupId2
    ) public view returns (bool) {
        uint256 groupPairId = groupPairId(groupId1, groupId2);
        return _exchangeGroupPairs[groupPairId];
    }

    function groupPairId(
        uint256 groupId1,
        uint256 groupId2
    ) public pure returns (uint256) {
        return uint256(keccak256(abi.encodePacked(groupId1, groupId2)));
    }

    function addTokenToGroup(
        uint256 vaultId,
        uint256[] memory uniqueIdParams,
        uint256 groupId
    ) public onlyRole(DEFAULT_OWNER_ROLE) returns (bool) {
        IAssetGroup assetGroupContract = IAssetGroup(_assetGroups[groupId]);
        IAbstractCoinVault vaultContract = IAbstractCoinVault(
            _coinVaults[vaultId]
        );

        uint256 uniqueId = vaultContract.generateUniqueId(uniqueIdParams);
        assetGroupContract.insertTokenMember(vaultId, uniqueId);

        emit TokenAddedToGroup(vaultId, uniqueId, groupId);
        return true;
    }

    function addVaultToGroup(
        uint256 vaultId,
        uint256 groupId
    ) public onlyRole(DEFAULT_OWNER_ROLE) returns (bool) {
        IAssetGroup assetGroupContract = IAssetGroup(_assetGroups[groupId]);
        assetGroupContract.insertVaultMember(vaultId);
        emit VaultAddedToGroup(vaultId, groupId);
        return true;
    }

    function isTokenMemberOf(
        uint256 vaultId,
        uint256[] memory uniqueIdParams,
        uint256 groupId
    ) public view returns (bool) {
        uint256[] memory p2 = new uint256[](5);
        // first paramater is reserved for value.
        // copying the rest
        // TODO:: check the length < 5
        for (uint256 i = 1; i < uniqueIdParams.length; i++) {
            p2[i] = uniqueIdParams[i];
        }

        IAssetGroup assetGroupContract = IAssetGroup(_assetGroups[groupId]);

        uint256 tokenUniqueId = IAbstractCoinVault(_coinVaults[vaultId])
            .generateUniqueId(p2);

        return assetGroupContract.isTokenMember(vaultId, tokenUniqueId);
    }

    function isVaultMemberOf(
        uint256 vaultId,
        uint256 groupId
    ) public view returns (bool) {
        IAssetGroup assetGroupContract = IAssetGroup(_assetGroups[groupId]);

        return assetGroupContract.isVaultMember(vaultId);
    }

    function isMemberOfFromProofReceipt(
        uint256 vaultId,
        ProofReceipt memory receipt,
        uint256 groupId
    ) public view returns (bool) {
        IAssetGroup assetGroupContract = IAssetGroup(_assetGroups[groupId]);

        return assetGroupContract.isMemberFromProofReceipt(vaultId, receipt);
    }

    ///////////////////////////////////////////////
    //          Swap/exchange functions
    //////////////////////////////////////////////

    // Offset of the output commitments in a receipt settled by the payment
    // entry points. They settle against any registered vault: Erc20CoinVault
    // takes only Payment-family receipts, but EnygmaErc20CoinVault and
    // Erc1155CoinVault also take legacy joinsplit receipts with more inputs.
    // Those circuits have no template in enygma_dvp, so no statement layout
    // (#96); they share the joinsplit prefix [message, trees, roots,
    // nullifiers], one signal per input each.
    function _commitmentsOutOffset(uint256 numberOfInputs) internal pure returns (uint256) {
        if (numberOfInputs == 1 || numberOfInputs == 2) return paymentCommitmentsOutOffset(numberOfInputs);
        return 1 + 3 * numberOfInputs;
    }

    // Offset of a fee-carrying receipt's fee: PaymentFee's (1 input, 2 outputs)
    // or PaymentRelayerFee's (1 input, 3 outputs). Any other shape has no
    // Payment-family fee; for it this is the signal after the contract address
    // that follows the output commitments, as for those two. Only the legacy
    // joinsplit receipts described at _commitmentsOutOffset reach it.
    function _feeOffset(uint256 numberOfInputs, uint256 numberOfOutputs) internal pure returns (uint256) {
        if (numberOfInputs == 1 && numberOfOutputs == 2) return Psl.PAYMENT_FEE_FEE_OFFSET;
        if (numberOfInputs == 1 && numberOfOutputs == 3) return Psl.PAYMENT_RELAYER_FEE_FEE_OFFSET;
        return _commitmentsOutOffset(numberOfInputs) + numberOfOutputs + 1;
    }

    modifier onlyRelayer() {
        require(authorizedRelayers[msg.sender], "EnygmaDvp: caller is not an authorized relayer");
        _;
    }

    // Lock all input nullifiers in a receipt so the prover cannot double-spend
    // while the counterparty's receipt is pending in SwapRelayer.
    // Only callable by an authorized SwapRelayer contract.
    function lockReceiptNullifiers(
        ProofReceipt calldata receipt,
        uint256 vaultId
    ) public onlyRelayer returns (bool) {
        // Reverts with InvalidStatementSize unless the receipt is a DvP leg.
        DvpReceipt.Leg memory leg = DvpReceipt.read(receipt.statement);
        IAbstractCoinVault(_coinVaults[vaultId]).lockCoin(leg.treeNumber, leg.nullifier);
        return true;
    }

    // Release the nullifier locks placed by lockReceiptNullifiers.
    // Called by SwapRelayer when a swap is cancelled after expiry.
    // Only callable by an authorized SwapRelayer contract.
    function unlockReceiptNullifiers(
        ProofReceipt calldata receipt,
        uint256 vaultId
    ) public onlyRelayer returns (bool) {
        IAbstractCoinVault(_coinVaults[vaultId]).unlockFromDvpReceipt(receipt);
        return true;
    }

    function submitPartialSettlement(
        ProofReceipt memory receipt,
        uint256 vaultId,
        uint256 groupId,
        uint256 deadline
    ) public nonReentrant returns (bool) {
        // to avoid submission of a payment (0) or withdrawal (WITHDRAW_MESSAGE)
        // proof: neither is a swap leg, and parking one here would lock its
        // note and, on timeout, insert outputs that are not swap refunds. A
        // receipt that is not DvP-shaped is refused before anything is read.
        if (!DvpReceipt.isLeg(receipt.statement)) {
            revert InvalidPartialProofReceipt();
        }
        DvpReceipt.Leg memory leg = DvpReceipt.read(receipt.statement);
        uint256 receiptMessage = leg.message;
        uint256 receiptUniqueId = leg.commitment;
        if (receiptMessage == 0 || receiptMessage == WITHDRAW_MESSAGE || receiptUniqueId == 0) {
            revert InvalidPartialProofReceipt();
        }

        // checking the receipt, vaultId and groupId validity
        IAbstractCoinVault(_coinVaults[vaultId]).checkDvpReceiptConditions(
            receipt
        );

        // asserting item proof shows that item belongs to group1
        if (
            !IAssetGroup(_assetGroups[groupId]).isMemberFromProofReceipt(
                vaultId,
                receipt
            )
        ) {
            revert GroupMembershipMismatch();
        }

        // HIGH-2 fix: Bob's StMessage = swap_id (from DvPDestinationCircuit).
        // _pendingTransactions is now keyed by swapId so Bob's lookup finds Alice's record.
        // HIGH-10 fix: targetReceiptId = commitA (what Bob's receiptUniqueId will be).
        if (
            _pendingTransactions[receiptMessage].targetReceiptId ==
            receiptUniqueId
        ) {
            // Second leg found — check deadline and settle.
            if (block.timestamp > _pendingTransactions[receiptMessage].deadline) {
                revert SwapDeadlineExpired();
            }

            uint256 vaultId2 = _pendingTransactions[receiptMessage].vaultId;
            uint256 groupId2 = _pendingTransactions[receiptMessage].groupId;
            // HIGH-2 fix: look up vault receipt by commitB (Alice's receiptUniqueId),
            // not by receiptMessage (swap_id) which is a different value.
            uint256 commitB2 = _pendingTransactions[receiptMessage].commitB;
            ProofReceipt memory receipt2 = IAbstractCoinVault(
                _coinVaults[vaultId2]
            ).getPendingProofReceipt(commitB2);

            IAbstractCoinVault(_coinVaults[vaultId2]).unlockFromDvpReceipt(receipt2);
            delete _pendingTransactions[receiptMessage];
            _closedSwaps[receiptMessage] = true;

            // Call _settleOnGroupPair directly (bypassing onlyRelayer on the public wrappers)
            // since submitPartialSettlement is itself a permissionless entry point.
            if (!IAssetGroup(_assetGroups[groupId2]).isFungible()) {
                if (!isValidSwapGroupPair(groupId, groupId2)) revert InvalidSwapGroupPair();
                _settleOnGroupPair(receipt, receipt2, vaultId, vaultId2, groupId, groupId2);
            } else if (!IAssetGroup(_assetGroups[groupId]).isFungible()) {
                if (!isValidSwapGroupPair(groupId2, groupId)) revert InvalidSwapGroupPair();
                _settleOnGroupPair(receipt2, receipt, vaultId2, vaultId, groupId2, groupId);
            } else {
                if (!isValidExchangeGroupPair(groupId2, groupId)) revert InvalidExchangeGroupPair();
                _settleOnGroupPair(receipt2, receipt, vaultId2, vaultId, groupId2, groupId);
            }
        } else {
            // A counterparty leg carries the swap key (commitB) as its message.
            // If that swap already settled or timed out, reject the leg rather
            // than opening a new swap with it.
            if (_closedSwaps[receiptMessage]) {
                revert SwapClosed();
            }
            // Only an initiator leg (which carries commitA and revertCommitA)
            // can open a swap; a destination leg has no pending swap to
            // complete.
            if (receipt.statement.length != DVP_INITIATOR_LEN) {
                revert SwapNotFound();
            }
            if (deadline <= block.timestamp) {
                revert SwapDeadlineMustBeInFuture();
            }
            if (deadline < block.timestamp + MIN_SWAP_DURATION) {
                revert SwapDeadlineTooSoon();
            }
            if (deadline > block.timestamp + MAX_SWAP_DURATION) {
                revert SwapDeadlineTooFar();
            }

            // receiptUniqueId is the initiator's CommitB; its CommitA is what
            // Alice expects Bob to deliver.
            //
            // HIGH-2 fix (corrected): key _pendingTransactions by commitB.
            // The DvPDestinationCircuit enforces StMessage = commitB, so Bob's
            // receiptMessage = commitB — this is the lookup key Bob uses when settling.
            // Keying by commitB (not a derived swapId) aligns circuit and contract.
            // initiator is informational only (msg.sender of the first leg, which
            // may be a relayer or a front-runner, not the note's owner) — it is
            // deliberately NOT an authorization input for claimSwapTimeout.
            uint256 commitB          = receiptUniqueId;
            // A swap is keyed by commitB, so commitB is what must be fresh: the
            // check above reads this leg's own message (commitA for an
            // initiator), which is not the swap key. Without these, anyone who
            // knows commitB's opening (the counterparty does) could open a
            // second swap with it from another vault in the group, overwriting
            // the pending record: the first initiator's note would stay locked
            // with no record left to time it out, and a closed swap could be
            // reopened.
            if (_closedSwaps[commitB]) {
                revert SwapClosed();
            }
            if (_pendingTransactions[commitB].deadline != 0) {
                revert SwapAlreadyPending();
            }
            uint256 commitA_expected = receipt.statement[Dsl.DVP_INITIATOR_COMMIT_A_OFFSET];
            // CRIT-1 fix: extract revertCommitA from the circuit's public statement instead of
            // accepting it as a caller-supplied parameter. The circuit constrains it to
            // Poseidon4(pkAlice, revertSalt, valueIn, tokenIdIn) so only Alice can spend it.
            // A caller-supplied value could be front-run to redirect the timeout refund.
            uint256 revertCommitA    = receipt.statement[Dsl.DVP_INITIATOR_REVERT_COMMIT_A_OFFSET];

            _pendingTransactions[commitB].vaultId        = vaultId;
            _pendingTransactions[commitB].groupId         = groupId;
            _pendingTransactions[commitB].targetReceiptId = commitA_expected;
            _pendingTransactions[commitB].deadline        = deadline;
            _pendingTransactions[commitB].swapId          = commitB;
            _pendingTransactions[commitB].revertCommitA   = revertCommitA;
            _pendingTransactions[commitB].commitB         = commitB;
            _pendingTransactions[commitB].initiator        = msg.sender;

            IAbstractCoinVault(_coinVaults[vaultId]).addPendingProofReceipt(receipt);

            emit SwapInitiated(commitB, commitA_expected, commitB, deadline);
            emit PendingProofAddedToVault(vaultId, groupId, receiptMessage, receipt);
        }
    }

    // Reclaim a timed-out pending swap.
    // Once the deadline stored in the pending transaction has passed,
    // anyone can call this to unlock the initiator's nullifiers so
    // the original note can be re-spent (e.g. via REVERT_COMMIT_A).
    // CRIT-2 fix: properly handle swap timeout by:
    //   1. Spending (nullifying) Alice's input nullifiers — not just unlocking.
    //   2. Inserting revertCommitA into the vault — Alice can spend this note.
    //
    // Deliberately callable by ANYONE (this replaces the HIGH-10 restriction to
    // the first-leg submitter). The submitter is not authenticated: the first
    // leg's receipt is public, so a front-runner can submit Alice's receipt
    // first, become "initiator" and choose the deadline — under the old
    // restriction only that party could ever release Alice's note. Permitting
    // any caller costs nothing: the outcome is fixed by Alice's proof, since
    // revertCommitA comes from the circuit's public statement and is
    // spendable only with her key, so a third party triggering the refund can
    // only ever return the note to its owner. The deadline ceiling
    // (MAX_SWAP_DURATION) bounds how long a hijacked swap can hold the note,
    // and its floor (MIN_SWAP_DURATION) how soon one can be timed out.
    function claimSwapTimeout(uint256 pendingReceiptId) public nonReentrant returns (bool) {
        TransactionMetadata storage meta = _pendingTransactions[pendingReceiptId];

        if (meta.deadline == 0) {
            revert SwapNotFound();
        }
        if (block.timestamp <= meta.deadline) {
            revert SwapNotExpiredYet();
        }
        uint256 vaultId       = meta.vaultId;
        uint256 revertCommitA = meta.revertCommitA;

        // Look up the vault receipt using commitB (Alice's receiptUniqueId).
        ProofReceipt memory receipt = IAbstractCoinVault(_coinVaults[vaultId])
            .getPendingProofReceipt(meta.commitB);

        // CRIT-2 fix: nullify Alice's input (mark as permanently spent)
        // then insert revertCommitA so Alice can reclaim her asset.
        // The nullifier was locked by addPendingProofReceipt; unlock first so nullifyFromDvpReceipt succeeds.
        IAbstractCoinVault(_coinVaults[vaultId]).unlockFromDvpReceipt(receipt);
        IAbstractCoinVault(_coinVaults[vaultId]).nullifyFromDvpReceipt(receipt);
        uint256[] memory revertCommits = new uint256[](1);
        revertCommits[0] = revertCommitA;
        IAbstractCoinVault(_coinVaults[vaultId]).registerCoins(revertCommits);

        delete _pendingTransactions[pendingReceiptId];
        _closedSwaps[pendingReceiptId] = true;

        emit SwapTimedOut(pendingReceiptId);
        return true;
    }

    function swap(
        ProofReceipt memory paymentReceipt,
        ProofReceipt memory deliveryReceipt,
        uint256 paymentVaultId,
        uint256 deliveryVaultId
    ) public onlyRelayer returns (bool) {
        // Hard-coded groupIds
        // to enforce fungible payment
        // and non-fungible delivery
        swapOnGroupPair(
            paymentReceipt,
            deliveryReceipt,
            paymentVaultId,
            deliveryVaultId,
            GROUP_ID_FUNGIBLES,
            GROUP_ID_NON_FUNGIBLES
        );
        return true;
    }

    // BUGFIX: nonReentrant moved here from _settleOnGroupPair (see the matching
    // comment there) — this is the actual external entry point for the
    // swap/exchange call family that has no other guard on its call chain.
    function swapOnGroupPair(
        ProofReceipt memory receipt1,
        ProofReceipt memory receipt2,
        uint256 vaultId1,
        uint256 vaultId2,
        uint256 groupId1,
        uint256 groupId2
    ) public onlyRelayer nonReentrant returns (bool) {
        // checking groupId1 and groupId2
        // to be in _swapGroupPairs
        if (!isValidSwapGroupPair(groupId1, groupId2)) {
            revert InvalidSwapGroupPair();
        }

        return
            _settleOnGroupPair(
                receipt1,
                receipt2,
                vaultId1,
                vaultId2,
                groupId1,
                groupId2
            );
    }

    function exchange(
        ProofReceipt memory paymentReceipt1,
        ProofReceipt memory paymentReceipt2,
        uint256 paymentVaultId1,
        uint256 paymentVaultId2
    ) public onlyRelayer returns (bool) {
        // Hard-coded groupIds
        // to enforce fungible payment
        // and fungible payment
        exchangeOnGroupPair(
            paymentReceipt1,
            paymentReceipt2,
            paymentVaultId1,
            paymentVaultId2,
            GROUP_ID_FUNGIBLES,
            GROUP_ID_FUNGIBLES
        );

        return true;
    }

    // BUGFIX: nonReentrant moved here from _settleOnGroupPair (see the matching
    // comment there) — this is the actual external entry point for the
    // swap/exchange call family that has no other guard on its call chain.
    function exchangeOnGroupPair(
        ProofReceipt memory receipt1,
        ProofReceipt memory receipt2,
        uint256 vaultId1,
        uint256 vaultId2,
        uint256 groupId1,
        uint256 groupId2
    ) public onlyRelayer nonReentrant returns (bool) {
        // checking groupId1 and groupId2
        // to be in _exchangeGroupPairs
        if (!isValidExchangeGroupPair(groupId1, groupId2)) {
            revert InvalidExchangeGroupPair();
        }

        return
            _settleOnGroupPair(
                receipt1,
                receipt2,
                vaultId1,
                vaultId2,
                groupId1,
                groupId2
            );
    }

    // CRIT-5 fix: reentrancy protection via vault callbacks.
    // Settlement order: nullify inputs FIRST, then insert outputs — prevents
    // the double-spend window where commitments exist before nullifiers are spent.
    //
    // BUGFIX: nonReentrant was on THIS internal function, but it's called from
    // three different places on this same contract: swapOnGroupPair(),
    // exchangeOnGroupPair(), and submitPartialSettlement() (which is itself
    // already nonReentrant). Since OpenZeppelin's ReentrancyGuard tracks one
    // shared _status flag per contract (not per call site), having a guard
    // here AND on submitPartialSettlement meant a legitimate call from
    // submitPartialSettlement into this function tripped "ReentrancyGuard:
    // reentrant call" on itself — a false positive that broke the DvP full-
    // swap settlement path entirely (caught by TestV2DvP_WithDeadline/FullSwap).
    // Moved the guard instead onto swapOnGroupPair()/exchangeOnGroupPair() —
    // the two callers that had no guard elsewhere in their own call chain —
    // so every actual external entry point still has exactly one guard, with
    // no nested double-guard on the shared internal path.
    function _settleOnGroupPair(
        ProofReceipt memory receipt1,
        ProofReceipt memory receipt2,
        uint256 vaultId1,
        uint256 vaultId2,
        uint256 groupId1,
        uint256 groupId2
    ) internal returns (bool) {
        //-----------------------
        // [[VERIFICATION]]
        //-----------------------

        // Each leg's message is the commitment its counterparty's leg inserts.
        DvpReceipt.Leg memory leg1 = DvpReceipt.read(receipt1.statement);
        DvpReceipt.Leg memory leg2 = DvpReceipt.read(receipt2.statement);
        if (leg1.message != leg2.commitment) {
            revert InvalidPaymentMessage();
        }
        if (leg2.message != leg1.commitment) {
            revert InvalidDeliveryMessage();
        }

        IAbstractCoinVault paymentCoinVault = IAbstractCoinVault(
            _coinVaults[vaultId1]
        );
        IAbstractCoinVault deliveryCoinVault = IAbstractCoinVault(
            _coinVaults[vaultId2]
        );

        // Each DvP proof names the vault its prover expects to be paid from
        // (CounterVault): receipt1's prover is paid from vault2 (where
        // receipt2's outputs are inserted) and vice versa. The commitments do
        // not name a vault, so without this a counterparty could pay from
        // another vault in the same asset group.
        if (leg1.counterVault != uint256(uint160(address(deliveryCoinVault)))) {
            revert CounterVaultMismatch();
        }
        if (leg2.counterVault != uint256(uint160(address(paymentCoinVault)))) {
            revert CounterVaultMismatch();
        }

        // asserting item proof shows that item belongs to group1
        if (
            !IAssetGroup(_assetGroups[groupId1]).isMemberFromProofReceipt(
                vaultId1,
                receipt1
            )
        ) {
            revert GroupMembershipMismatch();
        }

        paymentCoinVault.checkDvpReceiptConditions(receipt1);

        // asserting item proof shows that item belongs to group2
        if (
            !IAssetGroup(_assetGroups[groupId2]).isMemberFromProofReceipt(
                vaultId2,
                receipt2
            )
        ) {
            revert GroupMembershipMismatch();
        }

        // checking other conditions of the
        deliveryCoinVault.checkDvpReceiptConditions(receipt2);

        //-----------------------
        // [[SETTLEMENT]]
        //-----------------------

        // CRIT-5 fix: nullify inputs BEFORE inserting outputs.
        // This closes the double-spend window: if output insertion fails, inputs
        // are already spent so they cannot be reused in another transaction.
        paymentCoinVault.nullifyFromDvpReceipt(receipt1);
        deliveryCoinVault.nullifyFromDvpReceipt(receipt2);

        // Insert each leg's commitment after inputs are permanently spent.
        paymentCoinVault.insertCommitmentFromDvpReceipt(receipt1);
        deliveryCoinVault.insertCommitmentFromDvpReceipt(receipt2);

        emit Settled(leg2.message, leg1.message);

        return true;
    }

    ///////////////////////////////////////////////
    //          Random oracle functions
    //////////////////////////////////////////////

    function checkAndRegisterChallenge(
        uint256 challenge_
    ) public returns (bool) {
        bool isRotten = _rottenChallenges[challenge_];

        if (isRotten) {
            revert RottenChallenge();
        }

        _rottenChallenges[challenge_] = true;

        return true;
    }

    ///////////////////////////////////////////////
    //          Payment function
    //////////////////////////////////////////////

    // payment nullifies Alice's input notes and inserts all output commitments.
    //
    // Off-chain flow:
    //   ss, ctxt         = ML-KEM.Encapsulate(bob_view_pk)
    //   saltB            = HKDF(ss, "Bob salt")
    //   encKey           = HKDF(ss, "encryption key")
    //   Commitment_B     = Poseidon(bob_spend_pk, SaltBToField(saltB), amount_B, tokenId)
    //   saltA           <- random (only Alice knows this; stored locally)
    //   Commitment_A     = Poseidon(alice_spend_pk, saltA, amount_A, tokenId)
    //   encTxData        = AES-GCM-ENC(encKey, tokenId || amount_B)
    //
    // Both commitments are inserted atomically into the Merkle tree.
    // Only Bob's ciphertext is published on-chain; Alice recovers her change via saltA.
    // A single Payment event is emitted for Bob's commitment (output 0).
    function payment(
        ProofReceipt memory receipt,
        uint256 vaultId,
        bytes calldata ctxt,
        bytes calldata encTxData
    ) public returns (bool) {
        if (receipt.numberOfOutputs == 0) revert InvalidNumberOfOutputs();
        if (_coinVaults[vaultId] == address(0)) revert InvalidVaultId();

        // NEW-3 fix: ERC20 JoinSplit circuit does not constrain StMessage in-circuit.
        // Enforce here that retail payment proofs carry message == 0, preventing a
        // prover from embedding arbitrary data in the message which could confuse
        // future protocol features that interpret that slot.
        if (receipt.statement[Psl.PAYMENT_MESSAGE_OFFSET] != 0) revert InvalidPaymentMessage();

        IAbstractCoinVault vault = IAbstractCoinVault(_coinVaults[vaultId]);

        // Verify ZK proof and check nullifier/root validity.
        vault.checkReceiptConditions(receipt);

        // MEDIUM-5 fix: nullify inputs BEFORE inserting outputs — same order as
        // _settleOnGroupPair. Inputs are permanently spent before outputs are
        // created, preventing any double-spend window between the two calls.
        vault.nullifyFromReceipt(receipt);

        // Insert all output commitments into the Merkle tree.
        vault.insertCommitmentsFromReceipt(receipt);

        // Emit a Payment event for Bob's output (index 0) so he can scan and claim his note.
        uint256 commitmentOut = receipt.statement[_commitmentsOutOffset(receipt.numberOfInputs)];
        emit Payment(vaultId, commitmentOut, ctxt, encTxData);

        return true;
    }

    // paymentWithFee is the fee-aware variant of payment().
    // The ZK circuit enforces valueIn == Σ(valuesOut) + StFee, so the fee is
    // absorbed in Alice's input commitment rather than collected as an ERC-20 transfer.
    // The on-chain check here confirms that the fee embedded in the proof matches
    // the protocolFee passed by the caller, preventing a prover from substituting
    // a different fee amount.
    //
    // The fee is read at _feeOffset: PaymentFee's for 1 input and 2
    // outputs, PaymentRelayerFee's for 1 input and 3.
    function paymentWithFee(
        ProofReceipt memory receipt,
        uint256 vaultId,
        uint256 protocolFee,
        bytes calldata ctxt,
        bytes calldata encTxData
    ) public returns (bool) {
        if (receipt.numberOfOutputs == 0) revert InvalidNumberOfOutputs();
        if (_coinVaults[vaultId] == address(0)) revert InvalidVaultId();
        if (receipt.statement[Psl.PAYMENT_MESSAGE_OFFSET] != 0) revert InvalidPaymentMessage();

        uint256 feeOffset = _feeOffset(receipt.numberOfInputs, receipt.numberOfOutputs);
        require(receipt.statement[feeOffset] == protocolFee, "EnygmaDvp: fee mismatch");

        IAbstractCoinVault vault = IAbstractCoinVault(_coinVaults[vaultId]);

        // Verify ZK proof with fee VK (checkReceiptConditions picks it by shape and statement length).
        vault.checkReceiptConditions(receipt);

        vault.nullifyFromReceipt(receipt);
        vault.insertCommitmentsFromReceipt(receipt);

        uint256 commitmentOut = receipt.statement[_commitmentsOutOffset(receipt.numberOfInputs)];
        emit Payment(vaultId, commitmentOut, ctxt, encTxData);

        return true;
    }

    // paymentWithRelayerFee is the 3-output variant of payment().
    // Alice pays Bob (output 0), receives change (output 1), and the relayer earns
    // a spendable note (output 2). The ZK circuit enforces:
    //   Σ(valuesIn) == valOut[0] + valOut[1] + valOut[2]   (no value burned)
    // Only output[1] is constrained to Alice's senderPk in-circuit. The
    // circuit binds the fee to valOut[2]; it is checked here against
    // relayerFixedFeeAmount so the contract (not just the circuit's internal
    // consistency) enforces the exact fee amount.
    //
    // ctxt / encTxData are Bob's note discovery data (ML-KEM capsule / AEAD ciphertext).
    function paymentWithRelayerFee(
        ProofReceipt memory receipt,
        uint256 vaultId,
        bytes calldata ctxt,
        bytes calldata encTxData
    ) public returns (bool) {
        if (receipt.numberOfOutputs != 3) revert InvalidNumberOfOutputs();
        if (_coinVaults[vaultId] == address(0)) revert InvalidVaultId();
        if (receipt.statement[Psl.PAYMENT_RELAYER_FEE_MESSAGE_OFFSET] != 0) revert InvalidPaymentMessage();

        // Without this check any fee the prover claims in the proof would pass
        // verification — the circuit only proves valOut[2] == StFee, not that
        // StFee matches any protocol-configured value.
        uint256 feeOffset = _feeOffset(receipt.numberOfInputs, receipt.numberOfOutputs);
        if (receipt.statement[feeOffset] != relayerFixedFeeAmount) revert InvalidRelayerFee();

        IAbstractCoinVault vault = IAbstractCoinVault(_coinVaults[vaultId]);

        vault.checkReceiptConditions(receipt);
        vault.nullifyFromReceipt(receipt);
        vault.insertCommitmentsFromReceipt(receipt);

        uint256 commitmentBob = receipt.statement[_commitmentsOutOffset(receipt.numberOfInputs)];
        emit Payment(vaultId, commitmentBob, ctxt, encTxData);

        return true;
    }

    // paymentWithUsdrFee settles two independent proofs atomically in one
    // call: a normal payment (any of the 1-in/2-out shapes payment()
    // accepts) against `vaultId`, and a UsdrFeeCircuit proof — a second,
    // independent relayer-fee asset with its own vault/token — against
    // `usdrVaultId`. Both settle or neither does.
    //
    // Unlike paymentWithRelayerFee (fee note is a 3rd output of the SAME
    // proof, same token), USDr is a genuinely separate asset: its own ERC20,
    // its own Erc20CoinVault, its own circuit. The two proofs are otherwise
    // unrelated — there is no cross-proof binding the way enygma_payments'
    // dual-proof transfer() needs, because each proof already nullifies
    // against its own vault's own tree; nothing here lets one proof be
    // replayed via the other.
    //
    // usdrReceipt is a UsdrFee proof: output[0] is the relayer's fee note,
    // output[1] the sender's change. Its fee is checked against
    // usdrFixedFeeAmount and its token id against usdrTokenId (config
    // hygiene, see that state var's doc comment — not security-critical).
    // usdrVaultId must equal usdrFeeVaultId (set by setUsdrFeeVaultId).
    //
    // ctxt/encTxData are the main payment's Bob note-discovery data;
    // usdrCtxt/usdrEncTxData are the USDr fee note's (typically the relayer's
    // own — it built the proof and already knows its own note's contents,
    // but the data is threaded through uniformly with the main leg anyway).
    function paymentWithUsdrFee(
        ProofReceipt memory receipt,
        uint256 vaultId,
        bytes calldata ctxt,
        bytes calldata encTxData,
        ProofReceipt memory usdrReceipt,
        uint256 usdrVaultId,
        bytes calldata usdrCtxt,
        bytes calldata usdrEncTxData
    ) external returns (bool) {
        // The USDr leg must settle in the configured USDr vault, not one the
        // caller picks — see usdrFeeVaultId.
        if (!usdrFeeVaultSet || usdrVaultId != usdrFeeVaultId) revert InvalidUsdrVault();

        // ── main leg — identical checks/settlement to payment() ──
        if (receipt.numberOfOutputs == 0) revert InvalidNumberOfOutputs();
        if (_coinVaults[vaultId] == address(0)) revert InvalidVaultId();
        if (receipt.statement[Psl.PAYMENT_MESSAGE_OFFSET] != 0) revert InvalidPaymentMessage();

        IAbstractCoinVault vault = IAbstractCoinVault(_coinVaults[vaultId]);
        vault.checkReceiptConditions(receipt);
        vault.nullifyFromReceipt(receipt);
        vault.insertCommitmentsFromReceipt(receipt);

        emit Payment(vaultId, receipt.statement[_commitmentsOutOffset(receipt.numberOfInputs)], ctxt, encTxData);

        // ── USDr leg — checked against relayerFixedFeeAmount's USDr counterpart ──
        if (usdrReceipt.numberOfOutputs != 2) revert InvalidNumberOfOutputs();
        if (_coinVaults[usdrVaultId] == address(0)) revert InvalidVaultId();
        if (usdrReceipt.statement[Psl.USDR_FEE_MESSAGE_OFFSET] != 0) revert InvalidPaymentMessage();

        if (usdrReceipt.statement[Psl.USDR_FEE_FEE_OFFSET] != usdrFixedFeeAmount) revert InvalidUsdrFee();
        if (usdrReceipt.statement[Psl.USDR_FEE_TOKEN_ID_OFFSET] != usdrTokenId) revert InvalidUsdrFee();

        IAbstractCoinVault usdrVault = IAbstractCoinVault(_coinVaults[usdrVaultId]);
        usdrVault.checkReceiptConditions(usdrReceipt);
        usdrVault.nullifyFromReceipt(usdrReceipt);
        usdrVault.insertCommitmentsFromReceipt(usdrReceipt);

        emit Payment(usdrVaultId, usdrReceipt.statement[_commitmentsOutOffset(usdrReceipt.numberOfInputs)], usdrCtxt, usdrEncTxData);

        return true;
    }

    ///////////////////////////////////////////////
    //          Private Mint functions
    //////////////////////////////////////////////

    function privateMint(
        uint256 vaultId,
        uint256 commitment,
        EnygmaPrivateMintProof calldata proof
    ) public onlyRole(DEFAULT_OWNER_ROLE) returns (bool) {
        // Validate vault exists
        if (_coinVaults[vaultId] == address(0)) {
            revert InvalidVaultId();
        }

        // Verify the ZK proof. public_signal is sized to PrivateMint's
        // statement length, so its fields are read by name below.
        // Reverts with ProofInvalid if verification fails
        IPrivateMintVerifier(_privateMintVerifierAddress).verifyProof(
            proof.proof,
            proof.public_signal
        );

        // CRIT-1 fix: the caller-supplied commitment must match the proof's Commitment.
        // Without this check the owner could insert an arbitrary commitment that was
        // never constrained by the ZK circuit, allowing phantom notes to be minted.
        if (commitment != proof.public_signal[Dsl.PRIVATE_MINT_COMMITMENT_OFFSET]) {
            revert PublicSignalMismatch();
        }

        // DVP-7 fix: the circuit binds the proof to a specific deployment
        // (ContractAddress). Without this check a proof generated for a different
        // EnygmaDvp deployment can be replayed here to mint a note that was never
        // authorised for this contract.
        require(
            proof.public_signal[Dsl.PRIVATE_MINT_CONTRACT_ADDRESS_OFFSET] == uint256(uint160(address(this))),
            "PrivateMint: proof bound to wrong contract"
        );

        uint256 cipherText = proof.public_signal[Dsl.PRIVATE_MINT_CIPHER_TEXT_OFFSET];

        // Emit event with commitment and cipherText only
        emit PrivateMint(vaultId, commitment, cipherText);

        // Add commitment to the specified vault's merkle tree
        uint256[] memory commitments = new uint256[](1);
        commitments[0] = commitment;
        IAbstractCoinVault(_coinVaults[vaultId]).registerCoins(commitments);

        return true;
    }
}
