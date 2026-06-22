// SPDX-License-Identifier: BUSL-1.1
pragma solidity ^0.8.20;

import {AccessControl}   from "@openzeppelin/contracts/access/AccessControl.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/security/ReentrancyGuard.sol";

interface IPoseidonWrapper {
    function poseidon(uint256[2] memory input) external pure returns (uint256);
}

/// @title AuctionCoinVault
/// @notice Poseidon-Merkle note vault for the auction protocol.
///
///         Implements ICoinVault (verifyRoot / lockCoin / unlockCoin /
///         nullifyCoin / registerCoins) so EnygmaAuction can call it.
///
///         Two instances are deployed: one for NFT notes, one for USDC notes.
///
///         Zero value: keccak256("ZkDvp") % SNARK_SCALAR_FIELD — identical
///         to the enygma_dvp vault so the same Go MerkleTree helper can be
///         used in off-chain proof generation.
///
///         Tree automatically rolls to a new sub-tree when full (treeNumber++).
contract AuctionCoinVault is AccessControl, ReentrancyGuard {

    // -----------------------------------------------------------------------
    // Roles
    // -----------------------------------------------------------------------

    bytes32 public constant OWNER_ROLE   = keccak256("OWNER_ROLE");
    bytes32 public constant AUCTION_ROLE = keccak256("AUCTION_ROLE");

    // -----------------------------------------------------------------------
    // Merkle tree constants
    // -----------------------------------------------------------------------

    uint256 private constant SNARK_SCALAR_FIELD =
        21888242871839275222246405745257275088548364400416034343698204186575808495617;

    uint256 public constant ZERO_VALUE = uint256(keccak256("ZkDvp")) % SNARK_SCALAR_FIELD;

    // -----------------------------------------------------------------------
    // Merkle tree state
    // -----------------------------------------------------------------------

    address  private _poseidonWrapper;
    uint256  private _treeDepth;
    uint256  private _nextLeafIndex;
    uint256  private _merkleRoot;
    uint256  private _treeNumber;        // current sub-tree index
    uint256[] private _zeros;            // zero values per level
    uint256[] private _filledSubTrees;   // rightmost filled node per level

    /// @dev rootHistory[treeNumber][root] = true once a root is observed.
    mapping(uint256 => mapping(uint256 => bool)) public rootHistory;

    // -----------------------------------------------------------------------
    // Nullifier / lock state
    // -----------------------------------------------------------------------

    mapping(uint256 => mapping(uint256 => bool)) public nullifiers;
    mapping(uint256 => mapping(uint256 => bool)) public lockedNullifiers;

    // -----------------------------------------------------------------------
    // Events
    // -----------------------------------------------------------------------

    event Commitment(uint256 indexed treeNumber, uint256 indexed commitment);
    event Nullified(uint256 indexed treeNumber, uint256 indexed nullifier);
    event Locked(uint256 indexed treeNumber, uint256 indexed nullifier);
    event Unlocked(uint256 indexed treeNumber, uint256 indexed nullifier);

    // -----------------------------------------------------------------------
    // Constructor
    // -----------------------------------------------------------------------

    /// @param poseidonWrapper_ Deployed PoseidonWrapper contract address.
    /// @param treeDepth_       Depth of each sub-tree (8 → max 256 leaves per sub-tree).
    constructor(address poseidonWrapper_, uint256 treeDepth_) AccessControl() ReentrancyGuard() {
        _poseidonWrapper = poseidonWrapper_;
        _treeDepth       = treeDepth_;

        _grantRole(OWNER_ROLE, msg.sender);
        _setRoleAdmin(AUCTION_ROLE, OWNER_ROLE);

        _initMerkle();
    }

    // -----------------------------------------------------------------------
    // Admin
    // -----------------------------------------------------------------------

    function grantAuctionRole(address auction) external onlyRole(OWNER_ROLE) {
        grantRole(AUCTION_ROLE, auction);
    }

    /// @notice Insert a single pre-computed commitment directly into the tree.
    ///         Used for test setup (seeding Bob's NFT note or Alice's USDC note)
    ///         without going through the real token deposit flow.
    function seedDeposit(uint256 commitment) external onlyRole(OWNER_ROLE) {
        uint256[] memory commits = new uint256[](1);
        commits[0] = commitment;
        _insertLeaves(commits);
        emit Commitment(_treeNumber, commitment);
    }

    // -----------------------------------------------------------------------
    // ICoinVault interface
    // -----------------------------------------------------------------------

    function verifyRoot(uint256 treeNumber_, uint256 root) external view returns (bool) {
        return rootHistory[treeNumber_][root];
    }

    function lockCoin(uint256 treeNumber_, uint256 nullifier) external onlyRole(AUCTION_ROLE) returns (bool) {
        require(nullifier != 0, "Vault: nullifier zero");
        lockedNullifiers[treeNumber_][nullifier] = true;
        emit Locked(treeNumber_, nullifier);
        return true;
    }

    function unlockCoin(uint256 treeNumber_, uint256 nullifier) external onlyRole(AUCTION_ROLE) returns (bool) {
        require(nullifier != 0, "Vault: nullifier zero");
        lockedNullifiers[treeNumber_][nullifier] = false;
        emit Unlocked(treeNumber_, nullifier);
        return true;
    }

    function nullifyCoin(uint256 treeNumber_, uint256 nullifier) external onlyRole(AUCTION_ROLE) returns (bool) {
        require(nullifier != 0, "Vault: nullifier zero");
        require(!nullifiers[treeNumber_][nullifier], "Vault: already nullified");
        nullifiers[treeNumber_][nullifier] = true;
        emit Nullified(treeNumber_, nullifier);
        return true;
    }

    function registerCoins(uint256[] calldata commitments) external onlyRole(AUCTION_ROLE) returns (bool) {
        // Filter zero placeholders (same convention as enygma_dvp vault).
        uint256 count = 0;
        for (uint256 i = 0; i < commitments.length; i++) {
            if (commitments[i] != 0) count++;
        }
        uint256[] memory real = new uint256[](count);
        uint256 k = 0;
        for (uint256 i = 0; i < commitments.length; i++) {
            if (commitments[i] != 0) {
                real[k++] = commitments[i];
                emit Commitment(_treeNumber, commitments[i]);
            }
        }
        if (count > 0) _insertLeaves(real);
        return true;
    }

    // -----------------------------------------------------------------------
    // View helpers
    // -----------------------------------------------------------------------

    function currentRoot() external view returns (uint256) { return _merkleRoot; }
    function currentTreeNumber() external view returns (uint256) { return _treeNumber; }
    function nextLeafIndex() external view returns (uint256) { return _nextLeafIndex; }

    // -----------------------------------------------------------------------
    // Internal — Merkle tree
    // -----------------------------------------------------------------------

    function _initMerkle() private {
        _zeros          = new uint256[](_treeDepth);
        _filledSubTrees = new uint256[](_treeDepth);

        uint256 currentZero = ZERO_VALUE;
        for (uint256 i = 0; i < _treeDepth; i++) {
            _zeros[i]          = currentZero;
            _filledSubTrees[i] = currentZero;
            currentZero        = _hash(currentZero, currentZero);
        }
        _merkleRoot = currentZero;
        rootHistory[0][currentZero] = true;
    }

    function _hash(uint256 left, uint256 right) internal view returns (uint256) {
        uint256[2] memory inp;
        inp[0] = left;
        inp[1] = right;
        return IPoseidonWrapper(_poseidonWrapper).poseidon(inp);
    }

    // Derived from enygma_dvp's Merkle.sol insertLeaves — incrementally builds
    // the tree level-by-level, updating _filledSubTrees as the rightmost path changes.
    function _insertLeaves(uint256[] memory leaves) internal {
        uint256 count = leaves.length;
        if ((_nextLeafIndex + count) >= (2 ** _treeDepth)) {
            _rollTree();
        }

        uint256 levelInsertionIndex = _nextLeafIndex;
        _nextLeafIndex += count;

        uint256 nextLevelHashIndex;
        uint256 nextLevelStartIndex;

        for (uint256 level = 0; level < _treeDepth; level++) {
            nextLevelStartIndex = levelInsertionIndex >> 1;
            uint256 insertionElement = 0;

            if (levelInsertionIndex % 2 == 1) {
                nextLevelHashIndex =
                    (levelInsertionIndex >> 1) - nextLevelStartIndex;
                leaves[nextLevelHashIndex] = _hash(
                    _filledSubTrees[level],
                    leaves[insertionElement]
                );
                insertionElement    += 1;
                levelInsertionIndex += 1;
            }

            for (; insertionElement < count; insertionElement += 2) {
                uint256 right;
                if (insertionElement < count - 1) {
                    right = leaves[insertionElement + 1];
                } else {
                    right = _zeros[level];
                }

                if (insertionElement == count - 1 || insertionElement == count - 2) {
                    _filledSubTrees[level] = leaves[insertionElement];
                }

                nextLevelHashIndex =
                    (levelInsertionIndex >> 1) - nextLevelStartIndex;

                leaves[nextLevelHashIndex] = _hash(
                    leaves[insertionElement],
                    right
                );
                levelInsertionIndex += 2;
            }

            levelInsertionIndex = nextLevelStartIndex;
            count               = nextLevelHashIndex + 1;
        }

        _merkleRoot = leaves[0];
        rootHistory[_treeNumber][_merkleRoot] = true;
    }

    function _rollTree() internal {
        _nextLeafIndex = 0;
        _treeNumber++;

        // Re-initialize filledSubTrees to the zero path for the new tree.
        for (uint256 i = 0; i < _treeDepth; i++) {
            _filledSubTrees[i] = _zeros[i];
        }

        rootHistory[_treeNumber][_merkleRoot] = true;
    }
}
