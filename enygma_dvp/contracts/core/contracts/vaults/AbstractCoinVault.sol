// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;

import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/security/ReentrancyGuard.sol";

import {IEnygmaDvp} from "../../interfaces/IEnygmaDvp.sol";
import {
    IAbstractCoinVault
} from "../../interfaces/vaults/IAbstractCoinVault.sol";
import {Merkle} from "./Merkle.sol";
import {DvpReceipt} from "../DvpReceipt.sol";

abstract contract AbstractCoinVault is
    IAbstractCoinVault,
    Merkle,
    AccessControl,
    ReentrancyGuard
{
    bytes32 public constant DEFAULT_DVP_ROLE =
        keccak256(abi.encodePacked("DvpRole"));

    bytes32 public constant DEFAULT_AUCTION_ROLE =
        keccak256(abi.encodePacked("AuctionRole"));

    bytes32 public constant DEFAULT_OWNER_ROLE =
        keccak256(abi.encodePacked("ownerRole"));

    // SECURITY FIX: previously the auction contract was granted the full
    // DEFAULT_DVP_ROLE in initializeVault() below — the same role the DVP
    // contract itself holds, covering every DVP-gated function on this vault
    // (insertCommitmentsFromReceipt, nullifyFromReceipt, the DvP-receipt
    // functions, addPendingProofReceipt, even initializeVault itself), when
    // the auction contract only ever calls
    // lockCoin/unlockCoin/nullifyCoin/registerCoins.
    // DEFAULT_AUCTION_ROLE already existed as a declared-but-unused constant
    // here and on EnygmaDvp — wiring it up now to actually scope the auction
    // contract down to just those 4 functions via onlyDvpOrAuction() below,
    // instead of the full DVP role.
    modifier onlyDvpOrAuction() {
        require(
            hasRole(DEFAULT_DVP_ROLE, msg.sender) ||
                hasRole(DEFAULT_AUCTION_ROLE, msg.sender),
            "AbstractCoinVault: caller is not DVP or authorized auction"
        );
        _;
    }

    ///////////////////////////////////////////////
    //           Private attributes
    //////////////////////////////////////////////

    // name identifier for ZkDvp smart contract
    string internal _name;

    // the address of PoseidonWrapper smart contract
    address internal _hashContractAddress;

    // address of non-generic verifier that pack the proofs
    // and utilizes the generic Gorth16 verifier smart contract
    address internal _verifierContractAddress;

    address internal _zkDvpContractAddress;

    address internal _assetContractAddress;

    address internal _zkAuctionContractAddress;

    uint256 internal _vaultId;

    uint256 internal _numberOfIdentifiers;

    // utxo's uniqueId to proofReceipt
    mapping(uint256 => IEnygmaDvp.ProofReceipt) _pendingProofReceipts;

    function getVaultId() public view returns (uint256) {
        return _vaultId;
    }
    function getAssetContractAddress() public view returns (address) {
        return _assetContractAddress;
    }
    function getHashContractAddress() public view returns (address) {
        return _hashContractAddress;
    }
    function getVerifierContractAddress() public view returns (address) {
        return _verifierContractAddress;
    }

    function getNumberOfAssetIdentifiers() public view returns (uint256) {
        return _numberOfIdentifiers;
    }

    function getRoot() public view returns (uint256 root) {
        return currentRoot();
    }
    function verifyRoot(
        uint256 treeNumber,
        uint256 root
    ) public view returns (bool) {
        return isValidRoot(treeNumber, root);
    }

    constructor(address zkDvpAddress) Merkle() AccessControl() ReentrancyGuard() {
        // _hashContractAddress = hashContractAddress;
        _setupRole(DEFAULT_OWNER_ROLE, msg.sender);
        _setupRole(DEFAULT_DVP_ROLE, zkDvpAddress);
    }

    function initializeVault(
        uint256 vaultId,
        uint256 numberOfAssetIdentifiers,
        address assetContractAddress,
        uint256 treeDepth,
        address hashContractAddress,
        address verifierContractAddress,
        address zkAuctionContractAddress
    ) public onlyRole(DEFAULT_DVP_ROLE) returns (bool) {
        // H-5 fix: prevent re-initialization. After the first call, the auction contract
        // gains DEFAULT_DVP_ROLE and could call initializeVault again to replace the
        // verifier or hash contract with a malicious one, bypassing all proof checks.
        require(_hashContractAddress == address(0), "Vault: already initialized");
        _vaultId = vaultId;
        _zkDvpContractAddress = msg.sender;
        _hashContractAddress = hashContractAddress;
        _verifierContractAddress = verifierContractAddress;
        _assetContractAddress = assetContractAddress;
        _zkAuctionContractAddress = zkAuctionContractAddress;
        _numberOfIdentifiers = numberOfAssetIdentifiers;

        // SECURITY FIX: grant the narrower DEFAULT_AUCTION_ROLE instead of
        // DEFAULT_DVP_ROLE — see the matching comment where the role is
        // declared above.
        _setupRole(DEFAULT_AUCTION_ROLE, _zkAuctionContractAddress);

        initializeMerkle(treeDepth, _vaultId, _hashContractAddress);

        return true;
    }

    function _insertCommitmentsFromReceipt(
        IEnygmaDvp.ProofReceipt memory receipt
    ) internal returns (bool) {
        uint256 inputSize = receipt.numberOfInputs;
        uint256 outputSize = receipt.numberOfOutputs;
        uint nullifiersIndex = 1 + (2 * inputSize);
        uint commitmentsIndex = nullifiersIndex + inputSize;

        uint numberOfCommitments = 0;
        for (uint256 i = 0; i < outputSize; i++) {
            if (receipt.statement[commitmentsIndex + i] != 0) {
                numberOfCommitments++;
            }
        }

        uint256[] memory commitments = new uint256[](numberOfCommitments);

        for (uint256 i = 0; i < numberOfCommitments; i++) {
            commitments[i] = (receipt.statement[commitmentsIndex + i]);
            emit Commitment(_vaultId, commitments[i]);
        }

        insertLeaves(commitments);

        return true;
    }

    // publicly only accessible by ZkDvp
    function insertCommitmentsFromReceipt(
        IEnygmaDvp.ProofReceipt memory receipt
    ) public onlyRole(DEFAULT_DVP_ROLE) returns (bool) {
        return _insertCommitmentsFromReceipt(receipt);
    }

    function _nullifyFromReceipt(
        IEnygmaDvp.ProofReceipt memory receipt
    ) internal returns (bool) {
        uint256 inputSize = receipt.numberOfInputs;

        uint rootsIndex = 1 + (1 * inputSize);
        uint nullifiersIndex = 1 + (2 * inputSize);
        for (uint i = 0; i < inputSize; i++) {
            uint256 treeNumber = receipt.statement[1 + i];
            uint256 nullifier = receipt.statement[nullifiersIndex + i];
            if (receipt.statement[rootsIndex + i] != 0) {
                _spendNullifier(treeNumber, nullifier);
            }
        }
        // BUG FIX: falling off the end of a function declared `returns
        // (bool)` implicitly returns the type's zero value — false — not an
        // error, just an easy thing to miss. Every call site in this
        // codebase currently ignores this return value (Slither:
        // unused-return), so this was silently dormant, but the function's
        // own declared contract says "true means success", and it could
        // never actually return that.
        return true;
    }

    // Marks one input note spent. A note locked in a pending swap or auction
    // (lockCoin) cannot be spent until it is unlocked: every path that sets a
    // nullifier from a receipt, including the legacy withdraw functions, goes
    // through here.
    function _spendNullifier(uint256 treeNumber, uint256 nullifier) internal {
        if (isLocked(treeNumber, nullifier)) {
            revert CantSpendLockedCoin();
        }
        setNullifier(treeNumber, nullifier);
        emit Nullifier(_vaultId, treeNumber, nullifier);
    }

    // public access is only allowed for ZkDvp
    function nullifyFromReceipt(
        IEnygmaDvp.ProofReceipt memory receipt
    ) public onlyRole(DEFAULT_DVP_ROLE) returns (bool) {
        return _nullifyFromReceipt(receipt);
    }

    // A DvP leg's receipt is read through DvpReceipt (which reverts with
    // InvalidStatementSize unless it is DvpInitiator- or DvpDestination-
    // shaped), never by numberOfInputs/numberOfOutputs: those are
    // caller-supplied, and the leg spends exactly one note and inserts exactly
    // one commitment.

    // Spends the note a DvP leg spends; publicly only accessible by ZkDvp.
    function nullifyFromDvpReceipt(
        IEnygmaDvp.ProofReceipt memory receipt
    ) public onlyRole(DEFAULT_DVP_ROLE) returns (bool) {
        DvpReceipt.Leg memory leg = DvpReceipt.read(receipt.statement);
        if (leg.merkleRoot != 0) {
            _spendNullifier(leg.treeNumber, leg.nullifier);
        }
        return true;
    }

    // Inserts the commitment a DvP leg's settlement inserts; publicly only
    // accessible by ZkDvp.
    function insertCommitmentFromDvpReceipt(
        IEnygmaDvp.ProofReceipt memory receipt
    ) public onlyRole(DEFAULT_DVP_ROLE) returns (bool) {
        DvpReceipt.Leg memory leg = DvpReceipt.read(receipt.statement);
        if (leg.commitment != 0) {
            uint256[] memory commitments = new uint256[](1);
            commitments[0] = leg.commitment;
            emit Commitment(_vaultId, leg.commitment);
            insertLeaves(commitments);
        }
        return true;
    }

    // Releases the lock on the note a DvP leg spends; publicly only
    // accessible by ZkDvp.
    function unlockFromDvpReceipt(
        IEnygmaDvp.ProofReceipt memory receipt
    ) public onlyRole(DEFAULT_DVP_ROLE) returns (bool) {
        DvpReceipt.Leg memory leg = DvpReceipt.read(receipt.statement);
        if (leg.merkleRoot != 0) {
            if (isLocked(leg.treeNumber, leg.nullifier)) {
                unlockCoin(leg.treeNumber, leg.nullifier);
                emit CoinUnlocked(_vaultId, leg.treeNumber, leg.nullifier);
            } else {
                revert CoinAlreadyUnlocked();
            }
        }
        return true;
    }

    function lockCoin(
        uint256 treeNumber,
        uint256 nullifier
    ) public onlyDvpOrAuction returns (bool) {
        lock(treeNumber, nullifier);
        emit CoinLocked(_vaultId, treeNumber, nullifier);

        return true;
    }

    function unlockCoin(
        uint256 treeNumber,
        uint256 nullifier
    ) public onlyDvpOrAuction returns (bool) {
        unlock(treeNumber, nullifier);
        emit CoinUnlocked(_vaultId, treeNumber, nullifier);

        return true;
    }

    function nullifyCoin(
        uint256 treeNumber,
        uint256 nullifier
    ) public onlyDvpOrAuction returns (bool) {
        setNullifier(treeNumber, nullifier);
        emit Nullifier(_vaultId, treeNumber, nullifier);

        return true;
    }

    function registerCoins(
        uint256[] memory commitments
    ) public onlyDvpOrAuction returns (bool) {
        uint numberOfCommitments = 0;
        for (uint256 i = 0; i < commitments.length; i++) {
            if (commitments[i] != 0) {
                numberOfCommitments++;
            }
        }

        uint256[] memory commitmentsToInsert = new uint256[](
            numberOfCommitments
        );

        for (uint256 i = 0; i < numberOfCommitments; i++) {
            if (commitments[i] != 0) {
                commitmentsToInsert[i] = (commitments[i]);
                emit Commitment(_vaultId, commitmentsToInsert[i]);
            }
        }

        insertLeaves(commitmentsToInsert);

        return true;
    }

    // Parks a DvP initiator's leg under the commitment it inserts (its
    // CommitB) and locks the note it spends.
    function addPendingProofReceipt(
        IEnygmaDvp.ProofReceipt memory receipt
    ) public onlyRole(DEFAULT_DVP_ROLE) returns (bool) {
        DvpReceipt.Leg memory leg = DvpReceipt.read(receipt.statement);

        if (_pendingProofReceipts[leg.commitment].statement.length != 0) {
            // proofReceipt has been already added to the vault
            revert ProofReceiptAlreadyAdded();
        } else {
            _pendingProofReceipts[leg.commitment] = receipt;

            // lock the nullifier to avoid potential front-running cases
            lockCoin(leg.treeNumber, leg.nullifier);
        }

        return true;
    }

    function getPendingProofReceipt(
        uint256 proofUniqueId
    ) public returns (IEnygmaDvp.ProofReceipt memory proofReceipt) {
        return _pendingProofReceipts[proofUniqueId];
    }

}
