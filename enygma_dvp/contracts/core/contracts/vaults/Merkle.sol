// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;


import {IMerkle} from"../../interfaces/vaults/IMerkle.sol";
import {IPoseidonWrapper} from "../../interfaces/IPoseidonWrapper.sol";

contract Merkle is IMerkle {
    
    address public poseidonWrapperAddress;
    address public zkDvpAddress;

    uint256 constant SNARK_SCALAR_FIELD = 21888242871839275222246405745257275088548364400416034343698204186575808495617;


    mapping(uint256 => mapping(uint256 => bool)) public nullifiers;
    mapping(uint256 => mapping(uint256 => bool)) public lockedNullifiers;

    uint256 internal treeDepth;

    // TODO:: consider using different ZERO-VALUES for different merkleTrees
    uint256 public constant ZERO_VALUE = uint256(keccak256("ZkDvp")) % SNARK_SCALAR_FIELD;
    uint256 internal nextLeafIndex;
    uint256 public merkleRoot;
    uint256 private newTreeRoot;
    uint256 public treeNumber;
    uint256[] public zeros;
    uint256[] private filledSubTrees;
    mapping(uint256 => mapping(uint256 => bool)) public rootHistory;

    uint256 private treeId;
    address private dvpAddress;

    constructor()
    { 
      // _setupRole(DEFAULT_ADMIN_ROLE, msg.sender);
    }
    
    function initializeMerkle(uint256 _treeDepth, uint256 _treeId, address _poseidonWrapperAddress) public {
      poseidonWrapperAddress = _poseidonWrapperAddress;

      treeDepth = _treeDepth;
      dvpAddress = msg.sender;
      treeId = _treeId;

      filledSubTrees = new uint256[](treeDepth);
      zeros = new uint256[](treeDepth);
      zeros[0] = ZERO_VALUE;
      uint256 currentZero = ZERO_VALUE;
      for (uint256 i = 0; i < treeDepth; i++) {
        zeros[i] = currentZero;
        currentZero = hashLeftRight(currentZero, currentZero);
      }
      newTreeRoot = merkleRoot = currentZero;
      rootHistory[treeNumber][currentZero] = true;

      // _setupRole(DEFAULT_DVP_ROLE, msg.sender);
      zkDvpAddress = msg.sender;

    }

  function hashLeftRight(uint256 _left, uint256 _right) public view returns (uint256) {
    uint256[2] memory numbers;
    numbers[0] = _left;
    numbers[1] = _right;

    return IPoseidonWrapper(poseidonWrapperAddress).poseidon(numbers);
  }


  function lock(uint256 _treeNumber, uint256 _nullifierId) public returns (bool){
    // TODO:: relaxed this requirement, needs audit
    // require(lockedNullifiers[_treeNumber][_nullifierId] == false, "Merkle: Nullifier already locked.");
    require(_nullifierId != 0, "Merkle: Nullifier can not be zero.");

    lockedNullifiers[_treeNumber][_nullifierId] = true;

    return true;
  }


  function isLocked(uint256 _treeNumber, uint256 _nullifierId) public returns (bool){
    return lockedNullifiers[_treeNumber][_nullifierId];
  }


  function unlock(uint256 _treeNumber, uint256 _nullifierId) public  returns (bool){
    require(_nullifierId != 0, "Merkle: Nullifier can not be zero.");
    require(nullifiers[_treeNumber][_nullifierId] == false, "Merkle: Nullifier already set.");
    require(lockedNullifiers[_treeNumber][_nullifierId] == true, "Merkle: Nullifier already unlocked.");

    lockedNullifiers[_treeNumber][_nullifierId] = false;

    return true;
  }


  function setNullifier(uint256 _treeNumber, uint256 _nullifierId) public {

    // Adding this require to original ones from the Aegis repo
    // to avoid entering the same coins' nullifiers in the
    // two input slots.
    require(nullifiers[_treeNumber][_nullifierId] == false, "Merkle: Nullifier slot already set.");
    require(_nullifierId != 0, "Merkle: Nullifier can not be zero.");
    nullifiers[_treeNumber][_nullifierId] = true;
  }


  function isValidNullifier(uint256 _treeNumber, uint256 _nullifierId) public view returns (bool){
    return nullifiers[_treeNumber][_nullifierId];
  }

  function isValidRoot(uint256 _treeNumber, uint256 _merkleRoot) public view returns (bool){

    return rootHistory[_treeNumber][_merkleRoot];
  }

  function currentRoot() public view returns (uint256){
    return merkleRoot;
  }

  function getTreeId() public view returns (uint256){
    return treeId;
  }

  function insertLeaves(uint256[] memory _leafHashes) public {
    uint256 count = _leafHashes.length;
    if ((nextLeafIndex + count) >= (2 ** treeDepth)) {
      newTree(); 
    }
    uint256 levelInsertionIndex = nextLeafIndex;
    nextLeafIndex += count;

    uint256 nextLevelHashIndex;
    uint256 nextLevelStartIndex;

    for (uint256 level = 0; level < treeDepth; level++) {
      nextLevelStartIndex = levelInsertionIndex >> 1;
      uint256 insertionElement = 0;

      if (levelInsertionIndex % 2 == 1) {
        nextLevelHashIndex = (levelInsertionIndex >> 1) - nextLevelStartIndex;
        _leafHashes[nextLevelHashIndex] = hashLeftRight(filledSubTrees[level], _leafHashes[insertionElement]);

        insertionElement += 1;
        levelInsertionIndex += 1;
      }

      for (insertionElement; insertionElement < count; insertionElement += 2) {
        uint256 right;

        if (insertionElement < count - 1) {
          right = _leafHashes[insertionElement + 1];
        } else {
          right = zeros[level];
        }

        if (insertionElement == count - 1 || insertionElement == count - 2) {
          filledSubTrees[level] = _leafHashes[insertionElement];
        }

        nextLevelHashIndex = (levelInsertionIndex >> 1) - nextLevelStartIndex;

        _leafHashes[nextLevelHashIndex] = hashLeftRight(_leafHashes[insertionElement], right);
        levelInsertionIndex += 2;
      }
      levelInsertionIndex = nextLevelStartIndex;
      count = nextLevelHashIndex + 1;
    }
     merkleRoot = _leafHashes[0];
    rootHistory[treeNumber][merkleRoot] = true;
  }

  function newTree() public {
    merkleRoot = newTreeRoot;
    nextLeafIndex = 0;
    treeNumber++;
  }

}
