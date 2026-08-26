// SPDX-License-Identifier: GPL3
pragma solidity ^0.8.24;
import "./IZkDvp.sol";

interface IEnygma {
    struct Point {
        uint256 c1;
        uint256 c2;
    }
    // Encoding of field elements is: X[0] * z + X[1]

    // Fix L-01: +1 signal on every layout below — a domain separator
    // (chain id and this contract's own address, packed into one field
    // element) binding each proof to a specific deployment. See
    // Enygma.sol's _expectedDomainId() doc comment.
    struct Proof {
        uint256[8] proof;
        uint256[81] public_signal;
    }

    // Fix M-14/C-09/L-01: withdraw/deposit are genuinely [52]-signal —
    // the real circuit arity in both cases (see Enygma.sol's
    // WITHDRAW_TOTAL_DEPOSIT_VALUE_OFFSET/DEPOSIT_HASH_OFFSET doc
    // comment for slot 50; slot 51 is the L-01 domain separator).
    // deposit's original [50] mismatch (circuit produced 51, this
    // struct declared 50) made deposit() revert InvalidProof
    // unconditionally before Fix M-14.
    struct WithdrawProof {
        uint256[8] proof;
        uint256[52] public_signal;
    }

    struct DepositProof {
        uint256[8] proof;
        uint256[52] public_signal;
    }

    struct SnarkProof {
        uint256[2] pi_a;
        uint256[2][2] pi_b;
        uint256[2] pi_c;
        uint256[1] public_signal;
    }

    struct FeeProof {
        uint256[8] proof;
        uint256[55] public_signal;
    }

    // Fix H-13: burn used to be plaintext arithmetic on a hidden balance,
    // with no way to verify amount <= balance because the balance is
    // committed, not stored in the clear. public_signal layout (9, the
    // 9th being the Fix L-01 domain separator):
    // [PublicKey, PrevCommit.c1, PrevCommit.c2, NewCommit.c1, NewCommit.c2,
    //  Amount, BlockNumber, Nullifier, DomainId] — see gnark-server/pkg/circuits/burn.
    struct BurnProof {
        uint256[8] proof;
        uint256[9] public_signal;
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

    // Fix M-06: used to emit the post-increment _totalRegisteredParties
    // counter, not the accountId actually registered — an off-chain
    // monitor keyed on this event recorded the wrong account.
    event AccountRegistered(
        address indexed addedBank,
        uint accountId
    );

    event SupplyMinted(uint indexed lastblockNum, uint amount, uint to);

    event VerifierRegistered(
        address indexed verifierAddress,
        uint totalRegisteredVerifiers
    );

    event TransactionSuccessful(address indexed senderAddress);

    // Fix H-09 (item 4 of the remediation list): the relayer is the sole
    // possible transaction submitter (registerAccount binds every bank to
    // the OWNER's address, not its own — see _expectedDomainId... no,
    // see onlyRegistered), so TransactionSuccessful's senderAddress is
    // always the relayer, never the bank that actually asked for this
    // submission. bankTag is a caller-supplied, unvalidated string
    // (typically the relayer's own per-bank credential identifier,
    // Fix H-06) letting the chain's own event log carry that
    // attribution instead of relying solely on the relayer's off-chain
    // logs. Deliberately NOT the anonymity-set sender's real accountId —
    // that stays hidden by design (H-01/H-02); this is "which caller of
    // the relayer asked for this submission", an orthogonal, non-private
    // fact the relayer already tracks off-chain per bank.
    event RelayAttribution(address indexed submitter, string bankTag);

    event BurnSuccessful(uint256 bankIndex, uint256 burnValue);

    // Fix M-13: transferWithFee's fee previously had no destination at
    // all — the circuit hard-asserted the pool shrinks by `fee`, but
    // nothing on-chain read the fee signal or accounted for where the
    // value went, permanently breaking the check() invariant on the first
    // fee transfer. The fee is now burned (totalSupply decremented to
    // match the pool's already-mandatory shrinkage) and its exact value
    // is enforced against protocolFee, not merely read.
    event ProtocolFeeUpdated(uint256 previousFee, uint256 newFee);
    event FeeBurned(uint256 fee);

    function Name() external view returns (string memory);
    function Symbol() external view returns (string memory);
    function TotalRegisteredBanks() external view returns (uint256);
    function TotalSupply() external view returns (uint256);
    function VerifierAddress() external view returns (address);

    // Fix H-02 residual: initialCommit is the account holder's own
    // Com(0, r) computed off-chain with a secret r — registerAccount no
    // longer takes the raw randomness r in calldata, per the audit's
    // "do not publish registration randomness in calldata" remediation.
    function registerAccount(
        address addr,
        uint256 accountNum,
        uint256 k,
        uint256 initialCommitX,
        uint256 initialCommitY,
        bytes calldata viewKey
    ) external returns (bool);

    function initialize() external returns (bool);

    // Fix H-08: two-step ownership transfer + pause, none of which existed
    // before (there was not even an owner() getter).
    function owner() external view returns (address);
    function pendingOwner() external view returns (address);
    function paused() external view returns (bool);
    function transferOwnership(address newOwner) external returns (bool);
    function acceptOwnership() external returns (bool);
    function pause() external returns (bool);
    function unpause() external returns (bool);

    // Fix H-02 residual: mintCommit is Com(amount, r_mint) computed
    // off-chain by the issuer with a fresh, non-zero r_mint — mintSupply
    // no longer commits issuance with r=0, per the audit's "do not commit
    // issuance with r=0" remediation. r_mint must reach the recipient
    // off-chain (the same way registration's r reaches the account
    // holder) for them to derive their account's updated blinding factor.
    function mintSupply(uint256 amount, uint256 to, uint256 mintCommitX, uint256 mintCommitY) external returns (bool);
    function check() external view returns (bool);
    function addVerifier(address verifier) external returns (bool);

    // Fix M-13: the required fee for transferWithFee, enforced by exact
    // match against public_signal[FEE_OFFSET] — not merely advisory.
    function protocolFee() external view returns (uint256);
    function setProtocolFee(uint256 newFee) external returns (bool);

    function getBalance(
        uint256 account
    ) external view returns (uint256 x, uint256 y);

    function getPublicValues(
        uint256 size
    ) external view returns (Point[] memory, uint256[] memory);

    // Fix H-09: bankTag is optional (pass "" for no attribution — every
    // direct on-chain caller other than the relayer has no bank
    // credential to report) and unvalidated; see RelayAttribution's doc.
    function transfer(
        Point[] memory commitments,
        Proof memory proof,
        uint256[] memory k,
        string memory bankTag
    ) external returns (bool);

    function burn(uint256 accountId, BurnProof memory proof) external returns (bool);

    function addBurnVerifier(address verifier) external returns (bool);

    function addFeeVerifier(address verifier) external returns (bool);

    // Fix H-09: see transfer()'s identical bankTag doc comment above.
    function transferWithFee(
        Point[] memory commitments,
        FeeProof memory proof,
        uint256[] memory k,
        string memory bankTag
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
