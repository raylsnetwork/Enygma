// SPDX-License-Identifier: SSPL-1.0
pragma solidity ^0.8.24;

// IEnygmaPayment is the interface for the on-chain payment phase.
//
// The implementation lives in EnygmaDvp (enygma_dvp module). This interface
// documents the subset called by retail channel payments and the event layout
// that the recipient must scan to discover incoming notes.
//
// Two encryption modes are supported — both use the same function signature;
// only the interpretation of cipherText / encTxData differs:
//
//   ML-KEM mode (standard):
//     cipherText = ML-KEM-768 capsule (1088 bytes)
//     encTxData  = AES-GCM ciphertext of {tokenId, amount} (key derived from ML-KEM ss)
//
//   Channel mode (after SetupChannel):
//     cipherText = channelId (bytes32 identifying the shared channel)
//     encTxData  = AES-256-GCM ciphertext of {amount(32), tokenId(32), salt(32)} (channel key)
interface IEnygmaPayment {

    struct G1Point  { uint256 x; uint256 y; }
    struct G2Point  { uint256[2] x; uint256[2] y; }
    struct SnarkProof { G1Point a; G2Point b; G1Point c; }

    struct ProofReceipt {
        SnarkProof proof;
        uint256[]  statement;
        uint256    numberOfInputs;
        uint256    numberOfOutputs;
    }

    // payment verifies the ZK proof, marks nullifiers spent, inserts output
    // commitments into the Merkle tree, and emits Payment + Nullifier events.
    function payment(
        ProofReceipt calldata receipt,
        uint256 vaultId,
        bytes calldata cipherText,
        bytes calldata encTxData
    ) external;

    // Payment is emitted once per output commitment that carries note data.
    // For channel-mode payments, cipherText = channelId and encTxData = channel-encrypted note.
    event Payment(
        uint256 indexed vaultId,
        uint256 indexed commitment,
        bytes cipherText,
        bytes encTxData
    );

    // Nullifier is emitted once per consumed input note.
    event Nullifier(
        uint256 indexed vaultId,
        uint256 indexed treeNum,
        uint256 indexed nullifier
    );
}
