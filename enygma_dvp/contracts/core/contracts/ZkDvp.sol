// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;
// pragma abicoder v2;

import {IERC721} from "@openzeppelin/contracts/token/ERC721/IERC721.sol";
import {IERC1155} from "@openzeppelin/contracts/token/ERC1155/IERC1155.sol";
import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";

import {IZkDvp} from "../interfaces/IZkDvp.sol";
import {IZkAuction} from "../interfaces/IZkAuction.sol";
import {IAbstractCoinVault} from "../interfaces/vaults/IAbstractCoinVault.sol";
import {IMerkle} from "../interfaces/vaults/IMerkle.sol";
import {IAssetGroup} from "../interfaces/vaults/IAssetGroup.sol";
import {IPoseidonWrapper} from "../interfaces/IPoseidonWrapper.sol";
import {IVerifier} from "../interfaces/IVerifier.sol";
import {IPrivateMintVerifier} from "../interfaces/IPrivateMintVerifier.sol";

contract ZkDvp is IZkDvp, AccessControl {
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
    // used directly in ZkdDvp
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

    // name identifier for ZkDvp smart contract
    string private _name;

    // the address of PoseidonWrapper smart contract
    address private _hashContractAddress;
    // address of generic groth16 verifier smart contract
    address private _genericVerifierContractAddress;
    // address of non-generic verifier that pack the proofs
    // and utilizes the generic Gorth16 verifier smart contract
    address private _verifierContractAddress;

    address private _zkAuctionContractAddress;

    // address of private mint verifier smart contract
    address private _privateMintVerifierAddress;

    // mapping from merkleTreeId to merkleTreeAddress
    // to keep track of the asset merkle trees.
    address[] private _coinVaults;
    uint256 _coinVaultsCount;

    mapping(uint256 => bool) private _rottenChallenges;

    mapping(uint256 => bool) private _swapGroupPairs;
    mapping(uint256 => bool) private _exchangeGroupPairs;

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
        _name = "ZkDVP smart contract";
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

    function registerZkAuction(
        address zkAuctionContractAddress
    ) public onlyRole(DEFAULT_OWNER_ROLE) returns (bool) {
        _zkAuctionContractAddress = zkAuctionContractAddress;

        // initializing the verifier by registering Snark circuits' verification keys.
        // and the genericGroth16 verifier
        IZkAuction(_zkAuctionContractAddress).initializeZkAuction(
            _hashContractAddress,
            _verifierContractAddress
        );

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
            _zkAuctionContractAddress
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

    function registerSwapGroupPair(
        uint256 groupId1,
        uint256 groupId2
    ) public returns (bool) {
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
    ) public returns (bool) {
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

    function submitPartialSettlement(
        ProofReceipt memory receipt,
        uint256 vaultId,
        uint256 groupId
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

        // checking if the other leg exists
        if (
            _pendingTransactions[receiptMessage].targetReceiptId ==
            receiptUniqueId
        ) {
            // if yes => do the settlement

            // TODO:: for now that both with-broker and without-broker
            // settlements are active, there should be a check here
            // if the proof has three outputs,
            // then it is with broker and the broker blinded_publickey should match.

            uint256 vaultId2 = _pendingTransactions[receiptMessage].vaultId;
            uint256 groupId2 = _pendingTransactions[receiptMessage].groupId;
            ProofReceipt memory receipt2 = IAbstractCoinVault(
                _coinVaults[vaultId2]
            ).getPendingProofReceipt(receiptMessage);

            IAbstractCoinVault(_coinVaults[vaultId2]).unlockFromReceipt(
                receipt2
            );

            if (!IAssetGroup(_assetGroups[groupId2]).isFungible()) {
                // Dvp with delivery submitted first
                swapOnGroupPair(
                    receipt,
                    receipt2,
                    vaultId,
                    vaultId2,
                    groupId,
                    groupId2
                );
            } else if (!IAssetGroup(_assetGroups[groupId]).isFungible()) {
                // Dvp with payment submitted first
                // checkIfBrokerEnabled(receipt2, vaultId2, groupId2);
                swapOnGroupPair(
                    receipt2,
                    receipt,
                    vaultId2,
                    vaultId,
                    groupId2,
                    groupId
                );
            } else {
                // TODO:: resolve broker situation for PvP
                // pvp
                exchangeOnGroupPair(
                    receipt2,
                    receipt,
                    vaultId2,
                    vaultId,
                    groupId2,
                    groupId
                );
            }
        } else {
            // if no => save the receipt for later settlement

            _pendingTransactions[receiptUniqueId].vaultId = vaultId;
            _pendingTransactions[receiptUniqueId].groupId = groupId;
            _pendingTransactions[receiptUniqueId]
                .targetReceiptId = receiptMessage;

            IAbstractCoinVault(_coinVaults[vaultId]).addPendingProofReceipt(
                receipt
            );
            emit PendingProofAddedToVault(
                vaultId,
                groupId,
                receiptMessage,
                receipt
            );
        }
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

        // inserting payment commitments into payment vault
        paymentCoinVault.insertCommitmentsFromReceipt(receipt1);
        // inserting delivery commitments into delivery vault
        deliveryCoinVault.insertCommitmentsFromReceipt(receipt2);

        // Nullifying the old coins
        deliveryCoinVault.nullifyFromReceipt(receipt2);
        paymentCoinVault.nullifyFromReceipt(receipt1);

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
    //          Private Mint functions
    //////////////////////////////////////////////

    function privateMint(
        uint256 vaultId,
        uint256 commitment,
        ZkPrivateMintProof calldata proof
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
