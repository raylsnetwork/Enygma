// SPDX-License-Identifier: GPL3
pragma solidity ^0.8.24;
import "./IZkDvp.sol";

interface IEnygma {
    struct Point {
        uint256 c1;
        uint256 c2;
    }
    // Encoding of field elements is: X[0] * z + X[1]

    struct Proof {
        uint256[8] proof;
        uint256[80] public_signal;
    }

    /// @notice USDr proofs carry one extra public signal (81, not 80): the
    /// fee amount is public (not hidden like the main asset's transfer
    /// amount) so the contract can enforce it equals usdrFixedFeeAmount —
    /// see USDrCircuit.Define's FeeAmount field.
    struct UsdrProof {
        uint256[8] proof;
        uint256[81] public_signal;
    }

    struct WithdrawProof {
        uint256[8] proof;
        uint256[50] public_signal;
    }

    struct DepositProof {
        uint256[8] proof;
        uint256[50] public_signal;
    }

    struct SnarkProof {
        uint256[2] pi_a;
        uint256[2][2] pi_b;
        uint256[2] pi_c;
        uint256[1] public_signal;
    }

    struct FeeProof {
        uint256[8] proof;
        uint256[54] public_signal;
    }

    struct DepositParams {
        uint256 amount;
        address erc20Adress;
        uint256 publicKey;
    }

    struct WithdrawParams {
        IZkDvp.JoinSplitTransaction transaction;
    }

    event TokenInitialized(uint maxBankCount);

    event AccountRegistered(
        address indexed addedBank,
        uint totalRegisteredParties
    );

    event SupplyMinted(uint indexed lastblockNum, uint amount, uint to);

    event VerifierRegistered(
        address indexed verifierAddress,
        uint totalRegisteredVerifiers
    );

    event TransactionSuccessful(address indexed senderAddress);

    event BurnSuccessful(uint256 bankIndex, uint256 burnValue);

    function Name() external view returns (string memory);
    function Symbol() external view returns (string memory);
    function TotalRegisteredBanks() external view returns (uint256);
    function TotalSupply() external view returns (uint256);
    function VerifierAddress() external view returns (address);

    function registerAccount(
        address addr,
        uint256 accountNum,
        uint256 k,
        uint256 r,
        bytes calldata viewKey
    ) external returns (bool);

    function initialize() external returns (bool);

    function mintSupply(uint256 amount, uint256 to) external returns (bool);

    /// @notice Mint USDr to a specific account — the only way anyone
    /// acquires USDr to pay relayer fees with (initializeUsdrBalance()
    /// alone only ever creates a zero balance).
    function mintUsdrSupply(uint256 amount, uint256 to) external returns (bool);
    function check() external view returns (bool);
    function addVerifier(address verifier) external returns (bool);

    function getBalance(
        uint256 account
    ) external view returns (uint256 x, uint256 y);

    function getPublicValues(
        uint256 size
    ) external view returns (Point[] memory, uint256[] memory);

    // usdrCommitments/usdrProof are a second, independent proof over a
    // second private balance ("USDr") tracked per account alongside the
    // main balance above — see usdrBalanceCommitments/addUsdrVerifier/
    // initializeUsdrBalance. Both proofs are verified and settled
    // atomically in this one call, using the same participantIds for both
    // (the same k=6 anonymity set). usdrProof's fee amount (public signal
    // index 80) must equal usdrFixedFeeAmount.
    function transfer(
        Point[] memory commitments,
        Proof memory proof,
        Point[] memory usdrCommitments,
        UsdrProof memory usdrProof,
        uint256[] memory k
    ) external returns (bool);

    function burn(uint256 bankIndex, uint256 burnValue) external returns (bool);

    function addFeeVerifier(address verifier) external returns (bool);

    /// @notice Register the verifier for USDr transfer proofs (81-signal
    /// shape — one more than the main transfer proof's 80, since the fee
    /// amount is a public signal here — different verifying key).
    function addUsdrVerifier(address verifier) external returns (bool);

    /// @notice Set the fixed USDr relayer fee. Every transfer()'s usdrProof
    /// must carry exactly this amount as its public FeeAmount signal
    /// (index 80) or the call reverts InvalidFeeAmount() — see
    /// USDrCircuit.Define. Governance-adjustable (a public input, not a
    /// circuit constant) so changing it never requires new proving/
    /// verifying keys.
    function setUsdrFixedFee(uint256 amount) external returns (bool);

    function usdrFixedFeeAmount() external view returns (uint256);

    /// @notice Create an account's initial USDr balance commitment. Must be
    /// called once per account before that account can be a participant in
    /// a transfer() that carries a USDr proof, or checkUsdr() reverts for it.
    function initializeUsdrBalance(
        uint256 accountId,
        uint256 randomness
    ) external returns (bool);

    function getUsdrBalance(
        uint256 accountId
    ) external view returns (uint256 x, uint256 y);

    function getUsdrPublicValues(
        uint256 count
    ) external view returns (Point[] memory, uint256[] memory);

    /// @notice Verify Σ(usdrBalanceCommitments) == usdrTotalSupply, the USDr
    /// analog of check().
    function checkUsdr() external view returns (bool);

    function transferWithFee(
        Point[] memory commitments,
        FeeProof memory proof,
        uint256[] memory k
    ) external returns (bool);

    function derivePk(uint256 v) external view returns (uint256 x2, uint256 y2);
    function derivePkH(
        uint256 r
    ) external view returns (uint256 x2, uint256 y2);

    function addPedComm(
        uint256 p1,
        uint256 p2,
        uint256 x2,
        uint256 y2
    ) external view returns (uint256, uint256);

    function pedCom(
        uint256 v,
        uint256 r
    ) external view returns (uint256, uint256);
}
