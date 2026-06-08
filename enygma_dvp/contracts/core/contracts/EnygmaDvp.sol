// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;
// pragma abicoder v2;

import {IERC721} from "@openzeppelin/contracts/token/ERC721/IERC721.sol";
import {IERC1155} from "@openzeppelin/contracts/token/ERC1155/IERC1155.sol";
import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/security/ReentrancyGuard.sol";

import {IEnygmaDvp} from "../interfaces/IEnygmaDvp.sol";
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
    uint256 public constant VK_ID_BROKER_REGISTRATION = 11;
    uint256 public constant VK_ID_LEGIT_BROKER = 12;

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

    // first broker_blindedPublicKey =>
    mapping(uint256 => ProofReceipt) private _registeredBrokers;

    // auditor's mapping AuditorId -> AuditorData
    mapping(uint256 => AuditorData) private _registeredAuditors;

    // authorized on-chain relayer contracts (set by owner)
    mapping(address => bool) public authorizedRelayers;
    ///////////////////////////////////////////////
    //              Constructor
    //////////////////////////////////////////////

    // hashContractAddress: poseidon Wrapper contract address
    // genericVerifierContractAddress: Groth16 generic verifier address.
    // TODO:: some form of verification is needed
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
        // registering the verifier smart contract address
        _verifierContractAddress = verifierAddress;

        // initializing the verifier by registering Snark circuits' verification keys.
        // and the genericGroth16 verifier
        IVerifier(_verifierContractAddress).initializeVerifier(
            _genericVerifierContractAddress
        );
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
    ) public onlyRole(DEFAULT_OWNER_ROLE) returns (bool) {
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
    ) public onlyRole(DEFAULT_OWNER_ROLE) returns (bool) {
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
        // TODO:: implement it
        IAssetGroup assetGroupContract = IAssetGroup(_assetGroups[groupId]);

        return assetGroupContract.isVaultMember(vaultId);
    }

    function isMemberOfFromProofReceipt(
        uint256 vaultId,
        ProofReceipt memory receipt,
        uint256 groupId
    ) public view returns (bool) {
        // TODO:: implement it
        IAssetGroup assetGroupContract = IAssetGroup(_assetGroups[groupId]);

        return assetGroupContract.isMemberFromProofReceipt(vaultId, receipt);
    }

    ///////////////////////////////////////////////
    //          Broker functions
    //////////////////////////////////////////////

    function registerBroker(
        ProofReceipt memory brokerRegistrationProof
    ) public returns (bool) {
        checkRegisterBrokerProof(brokerRegistrationProof);

        uint256 vaultId = brokerRegistrationProof.statement[1];

        uint256 blindedPublicKeyIndex = 3 +
            brokerRegistrationProof.numberOfInputs *
            3;
        uint256 blindedPublicKey = brokerRegistrationProof.statement[
            blindedPublicKeyIndex
        ];

        // Checking the key has not been set
        // TODO:: needs audit
        if (_registeredBrokers[blindedPublicKey].numberOfInputs == 0) {
            _registeredBrokers[blindedPublicKey] = brokerRegistrationProof;

            emit BrokerRegistered(vaultId, blindedPublicKey);
        } else {
            revert BrokerAlreadyRegistered();
        }

        return true;
    }

    function checkRegisterBrokerProof(
        ProofReceipt memory receipt
    ) internal returns (bool) {
        // signal input st_beacon;
        // signal input st_vaultId;
        // signal input st_groupId;
        // signal input st_delegator_treeNumbers[tm_numOfInputs];
        // signal input st_delegator_merkleRoots[tm_numOfInputs];
        // signal input st_delegator_nullifiers[tm_numOfInputs];
        // signal input st_broker_blindedPublicKey;

        // signal input st_assetGroup_treeNumber;
        // signal input st_assetGroup_merkleRoot;

        // TODO:: connect beacon
        // bind numberOfInputs and numberOfOutputs to receipt.statement.length

        if (receipt.numberOfInputs < 2) {
            revert InvalidNumberOfInputs();
        }

        if (receipt.numberOfOutputs != 0) {
            revert InvalidNumberOfOutputs();
        }

        uint256 vaultId = receipt.statement[1];
        uint256 groupId = receipt.statement[2];
        uint256 numberOfInputs = receipt.numberOfInputs;
        uint256 nullifiersIndex = 3 + (numberOfInputs * 2);
        uint256 blindedPkIndex = 3 + (numberOfInputs * 3);

        // asserting item proof shows that item belongs to group1
        if (
            !IAssetGroup(_assetGroups[groupId]).isMemberFromProofReceipt(
                vaultId,
                receipt
            )
        ) {
            revert GroupMembershipMismatch();
        }

        IAbstractCoinVault(_coinVaults[vaultId])
            .checkRegisterBrokerProofConditions(receipt);

        IVerifier(_verifierContractAddress).verifyProof(
            VK_ID_BROKER_REGISTRATION,
            receipt.proof,
            receipt.statement
        );

        return true;
    }

    function verifyLegitBrokerReceipt(
        ProofReceipt memory receipt
    ) public returns (bool) {
        uint256 beacon = receipt.statement[0];
        uint256 blindedPublicKey = receipt.statement[1];

        IVerifier(_verifierContractAddress).verifyProof(
            VK_ID_LEGIT_BROKER,
            receipt.proof,
            receipt.statement
        );
        emit LegitBrokerReceipt(beacon, blindedPublicKey);
        return true;
    }

    ///////////////////////////////////////////////
    //          Swap/exchange functions
    //////////////////////////////////////////////

    function _proofType(
        ProofReceipt memory receipt
    ) internal returns (uint256) {
        uint256 expectedBatchSize = 1 + 6 * receipt.numberOfInputs;
        uint256 expectedNormalSize = 3 +
            3 *
            receipt.numberOfInputs +
            receipt.numberOfOutputs;
        uint256 expectedBrokerSize = 5 +
            3 *
            receipt.numberOfInputs +
            receipt.numberOfOutputs;

        // TODO:: needs audit, not secure
        if (receipt.statement.length == expectedNormalSize) {
            return 1;
        } else if (receipt.statement.length == expectedBatchSize) {
            return 2;
        } else if (receipt.statement.length == expectedBrokerSize) {
            return 3;
        }

        revert InvalidStatementSize();
        return 0;
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
        uint treeNumbersIndex = 1;
        uint nullifiersIndex  = 1 + (2 * receipt.numberOfInputs);

        for (uint256 i = 0; i < receipt.numberOfInputs; i++) {
            IAbstractCoinVault(_coinVaults[vaultId]).lockCoin(
                receipt.statement[treeNumbersIndex + i],
                receipt.statement[nullifiersIndex + i]
            );
        }
        return true;
    }

    // Release the nullifier locks placed by lockReceiptNullifiers.
    // Called by SwapRelayer when a swap is cancelled after expiry.
    // Only callable by an authorized SwapRelayer contract.
    function unlockReceiptNullifiers(
        ProofReceipt calldata receipt,
        uint256 vaultId
    ) public onlyRelayer returns (bool) {
        IAbstractCoinVault(_coinVaults[vaultId]).unlockFromReceipt(receipt);
        return true;
    }

    function submitPartialSettlement(
        ProofReceipt memory receipt,
        uint256 vaultId,
        uint256 groupId,
        uint256 deadline,
        uint256 revertCommitA
    ) public returns (bool) {
        // uint256 prootType = _proofType(receipt);
        // uint inputSize = receipt.numberOfInputs;
        // uint treeNumbersIndex = 1;
        uint nullifiersIndex = 1 + (2 * receipt.numberOfInputs);
        uint commitmentsIndex = nullifiersIndex + receipt.numberOfInputs;

        uint256 receiptMessage = receipt.statement[0];
        uint256 receiptUniqueId = receipt.statement[commitmentsIndex];

        // to avoid submission of the withdraw proof
        if (receiptMessage == 0 || receiptUniqueId == 0) {
            revert InvalidPartialProofReceipt();
        }

        // checking the receipt, vaultId and groupId validity
        IAbstractCoinVault(_coinVaults[vaultId]).checkReceiptConditions(
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

            IAbstractCoinVault(_coinVaults[vaultId2]).unlockFromReceipt(receipt2);
            delete _pendingTransactions[receiptMessage];

            if (!IAssetGroup(_assetGroups[groupId2]).isFungible()) {
                swapOnGroupPair(receipt, receipt2, vaultId, vaultId2, groupId, groupId2);
            } else if (!IAssetGroup(_assetGroups[groupId]).isFungible()) {
                swapOnGroupPair(receipt2, receipt, vaultId2, vaultId, groupId2, groupId);
            } else {
                exchangeOnGroupPair(receipt2, receipt, vaultId2, vaultId, groupId2, groupId);
            }
        } else {
            if (deadline <= block.timestamp) {
                revert SwapDeadlineMustBeInFuture();
            }

            // commitB = receiptUniqueId (Alice's statement[commitmentsIndex] = StCommitB)
            // commitA = statement[commitmentsIndex+1] (Alice's StCommitA — what Alice expects)
            // commitB is used by Bob as his receiptUniqueId (DvP Destination statement[4] = commitA)
            // Wait — in DvPInitiator: statement = [msg, tree, root, nf, commitB, commitA, revertCommitA]
            // commitmentsIndex = 4 → receiptUniqueId = commitB
            // commitmentsIndex+1 = 5 → commitA
            uint256 commitB    = receiptUniqueId;
            uint256 commitA_expected = receipt.statement[commitmentsIndex + 1]; // StCommitA
            uint256 nfA        = receipt.statement[nullifiersIndex];

            uint256 innerHash = IPoseidonWrapper(_hashContractAddress).poseidon4(
                [commitB, revertCommitA, nfA, receiptMessage]
            );
            uint256 swapId = IPoseidonWrapper(_hashContractAddress).poseidon(
                [innerHash, deadline]
            );

            // HIGH-2 fix: key by swapId (Bob's StMessage = swap_id in DvP Destination circuit).
            // targetReceiptId = commitA_expected (Bob's receiptUniqueId = DvP Destination statement[4]).
            // HIGH-10 fix: store initiator to restrict claimSwapTimeout.
            _pendingTransactions[swapId].vaultId        = vaultId;
            _pendingTransactions[swapId].groupId         = groupId;
            _pendingTransactions[swapId].targetReceiptId = commitA_expected;
            _pendingTransactions[swapId].deadline        = deadline;
            _pendingTransactions[swapId].swapId          = swapId;
            _pendingTransactions[swapId].revertCommitA   = revertCommitA;
            _pendingTransactions[swapId].commitB         = commitB;
            _pendingTransactions[swapId].initiator        = msg.sender;

            IAbstractCoinVault(_coinVaults[vaultId]).addPendingProofReceipt(receipt);

            emit SwapInitiated(swapId, commitA_expected, commitB, deadline);
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
    // HIGH-10 fix: restrict to the swap initiator — prevents griefing by third parties.
    function claimSwapTimeout(uint256 pendingReceiptId) public returns (bool) {
        TransactionMetadata storage meta = _pendingTransactions[pendingReceiptId];

        if (meta.deadline == 0) {
            revert SwapNotFound();
        }
        if (block.timestamp <= meta.deadline) {
            revert SwapNotExpiredYet();
        }
        // HIGH-10: only the original initiator can claim the timeout.
        if (msg.sender != meta.initiator) {
            revert Unauthorized();
        }

        uint256 vaultId       = meta.vaultId;
        uint256 revertCommitA = meta.revertCommitA;

        // Look up the vault receipt using commitB (Alice's receiptUniqueId).
        ProofReceipt memory receipt = IAbstractCoinVault(_coinVaults[vaultId])
            .getPendingProofReceipt(meta.commitB);

        // CRIT-2 fix: nullify Alice's input (mark as permanently spent)
        // then insert revertCommitA so Alice can reclaim her asset.
        IAbstractCoinVault(_coinVaults[vaultId]).nullifyFromReceipt(receipt);
        uint256[] memory revertCommits = new uint256[](1);
        revertCommits[0] = revertCommitA;
        IAbstractCoinVault(_coinVaults[vaultId]).registerCoins(revertCommits);

        delete _pendingTransactions[pendingReceiptId];

        emit SwapTimedOut(pendingReceiptId);
        return true;
    }

    function swap(
        ProofReceipt memory paymentReceipt,
        ProofReceipt memory deliveryReceipt,
        uint256 paymentVaultId,
        uint256 deliveryVaultId
    ) public returns (bool) {
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

    function swapOnGroupPair(
        ProofReceipt memory receipt1,
        ProofReceipt memory receipt2,
        uint256 vaultId1,
        uint256 vaultId2,
        uint256 groupId1,
        uint256 groupId2
    ) public returns (bool) {
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
    ) public returns (bool) {
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

    function exchangeOnGroupPair(
        ProofReceipt memory receipt1,
        ProofReceipt memory receipt2,
        uint256 vaultId1,
        uint256 vaultId2,
        uint256 groupId1,
        uint256 groupId2
    ) public returns (bool) {
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

    // CRIT-5 fix: nonReentrant prevents reentrancy via vault callbacks.
    // Settlement order: nullify inputs FIRST, then insert outputs — prevents
    // the double-spend window where commitments exist before nullifiers are spent.
    function _settleOnGroupPair(
        ProofReceipt memory receipt1,
        ProofReceipt memory receipt2,
        uint256 vaultId1,
        uint256 vaultId2,
        uint256 groupId1,
        uint256 groupId2
    ) internal nonReentrant returns (bool) {
        //-----------------------
        // [[VERIFICATION]]
        //-----------------------

        // receipt1.inputs;
        // message;
        // treeNumbers[numberOfInputs];
        // merkleRoots[numberOfInputs];
        // nullifiers[numberOfInputs];
        // commitments[numberOfOutputs];
        // assetGroup_merkleRoot

        uint nullifiersIndex = 1 + (2 * receipt1.numberOfInputs);
        uint commitmentsIndex = nullifiersIndex + receipt1.numberOfInputs;

        // ownReceipt.inputs:
        // message;
        // treeNumbers[numberOfInputs];
        // merkleRoots[numberOfInputs];
        // nullifiers[numberOfInputs];
        // commitments[numberOfOutputs];
        // assetGroup_merkleRoot

        uint256 commitmentsIndex2 = 1 + 3 * receipt2.numberOfInputs;
        // TODO:: fix hardcoded 4
        if (receipt1.statement[0] != receipt2.statement[commitmentsIndex2]) {
            revert InvalidPaymentMessage();
        }
        if (receipt2.statement[0] != receipt1.statement[commitmentsIndex]) {
            revert InvalidDeliveryMessage();
        }

        IAbstractCoinVault paymentCoinVault = IAbstractCoinVault(
            _coinVaults[vaultId1]
        );
        IAbstractCoinVault deliveryCoinVault = IAbstractCoinVault(
            _coinVaults[vaultId2]
        );

        // asserting item proof shows that item belongs to group1
        if (
            !IAssetGroup(_assetGroups[groupId1]).isMemberFromProofReceipt(
                vaultId1,
                receipt1
            )
        ) {
            revert GroupMembershipMismatch();
        }

        paymentCoinVault.checkReceiptConditions(receipt1);

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
        deliveryCoinVault.checkReceiptConditions(receipt2);

        //-----------------------
        // [[SETTLEMENT]]
        //-----------------------

        // CRIT-5 fix: nullify inputs BEFORE inserting outputs.
        // This closes the double-spend window: if output insertion fails, inputs
        // are already spent so they cannot be reused in another transaction.
        paymentCoinVault.nullifyFromReceipt(receipt1);
        deliveryCoinVault.nullifyFromReceipt(receipt2);

        // Insert output commitments after inputs are permanently spent.
        paymentCoinVault.insertCommitmentsFromReceipt(receipt1);
        deliveryCoinVault.insertCommitmentsFromReceipt(receipt2);

        emit Settled(receipt2.statement[0], receipt1.statement[0]);

        return true;
    }

    ///////////////////////////////////////////////
    //          Random oracle functions
    //////////////////////////////////////////////

    // TODO:: this can later check the freshness of the challenge
    // with Random Oracle
    function checkAndRegisterChallenge(
        uint256 challenge_
    ) public returns (bool) {
        bool isRotten = _rottenChallenges[challenge_];

        if (isRotten) {
            revert RottenChallenge();
        }

        // TODO:: Require to check that challenge != valid address

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

        IAbstractCoinVault vault = IAbstractCoinVault(_coinVaults[vaultId]);

        // Verify ZK proof and check nullifier/root validity.
        vault.checkReceiptConditions(receipt);

        // Insert all output commitments into the Merkle tree (atomically).
        vault.insertCommitmentsFromReceipt(receipt);

        // Mark all input nullifiers as spent.
        vault.nullifyFromReceipt(receipt);

        // Statement layout (non-interleaved / ContractStatement):
        //   [msg, treeNums[nIn], roots[nIn], nullifiers[nIn], cmts[nOut]]
        uint256 commitmentsIndex = 1 + 3 * receipt.numberOfInputs;

        // Emit a Payment event for Bob's output (index 0) so he can scan and claim his note.
        uint256 commitmentOut = receipt.statement[commitmentsIndex];
        emit Payment(vaultId, commitmentOut, ctxt, encTxData);

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

        // Verify the ZK proof
        // Public signals order: [commitment, contractAddress, tokenId, cipherText]
        // Reverts with ProofInvalid if verification fails
        IPrivateMintVerifier(_privateMintVerifierAddress).verifyProof(
            proof.proof,
            proof.public_signal
        );

        // CRIT-1 fix: the caller-supplied commitment must match proof.public_signal[0].
        // Without this check the owner could insert an arbitrary commitment that was
        // never constrained by the ZK circuit, allowing phantom notes to be minted.
        if (commitment != proof.public_signal[0]) {
            revert PublicSignalMismatch();
        }

        // Extract cipherText from public signals (index 3)
        uint256 cipherText = proof.public_signal[3];

        // Emit event with commitment and cipherText only
        emit PrivateMint(vaultId, commitment, cipherText);

        // Add commitment to the specified vault's merkle tree
        uint256[] memory commitments = new uint256[](1);
        commitments[0] = commitment;
        IAbstractCoinVault(_coinVaults[vaultId]).registerCoins(commitments);

        return true;
    }
}
