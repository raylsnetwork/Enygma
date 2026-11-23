// SPDX-License-Identifier: GPL3
pragma solidity ^0.8.24;

/// @notice Minimal test-only stand-in for the ZkDvp bridge contract.
/// @dev Implements depositThroughEnygma (called by Enygma.withdraw() ->
///      _executeZkDvpDeposits) and withdrawThroughEnygma (called by
///      Enygma.deposit()). Tracks call counts / totals so a test can
///      assert on them directly, instead of inferring success from
///      Enygma-side state that H-14 is precisely about not trusting.
///      Like EnygmaErc20CoinVault, withdrawThroughEnygma spends one unspent
///      note with the given commitment and reverts UnknownEnygmaNote if
///      there is none; addNote stands in for a note a withdraw() made.
///
///      Used by go_client/enygma_test/h14_repro_test.go,
///      m14_m15_c09_repro_test.go and deposit_note_binding_test.go.
contract MockZkDvp {
    error UnknownEnygmaNote();

    uint256 public totalMinted;
    uint256 public callCount;
    uint256 public withdrawCallCount;
    uint256 public lastSpentNote;
    mapping(uint256 => uint256) public unspentEnygmaNotes;

    function depositThroughEnygma(
        uint256[] memory depositParams
    ) external returns (bool, uint256) {
        totalMinted += depositParams[0]; // amount
        callCount += 1;
        uint256 commitment = commitmentOf(depositParams[0], depositParams[1]);
        unspentEnygmaNotes[commitment] += 1;
        return (true, commitment);
    }

    /// @notice The commitment this mock "inserts" for (amount, publicKey):
    ///         deterministic, so a test can put it in a withdraw proof's
    ///         deposit-commitment signals (the real vault computes
    ///         Poseidon(Poseidon(asset, amount), publicKey)).
    function commitmentOf(uint256 amount, uint256 publicKey) public pure returns (uint256) {
        return uint256(keccak256(abi.encode(amount, publicKey))) %
            21888242871839275222246405745257275088548364400416034343698204186575808495617;
    }

    /// @notice Records an unspent note with this commitment.
    function addNote(uint256 commitment) external {
        unspentEnygmaNotes[commitment] += 1;
    }

    function withdrawThroughEnygma(uint256 commitment) external returns (bool) {
        if (unspentEnygmaNotes[commitment] == 0) revert UnknownEnygmaNote();
        unspentEnygmaNotes[commitment] -= 1;
        lastSpentNote = commitment;
        withdrawCallCount += 1;
        return true;
    }
}
