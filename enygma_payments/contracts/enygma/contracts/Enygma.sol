//SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;
import "./CurveBabyJubJub.sol";
import "../interfaces/IEnygma.sol";
import "../interfaces/IERC20.sol";
import "../interfaces/IZkDvp.sol";

contract Enygma is IEnygma {
    // ============================================
    // CONSTANTS
    // ============================================
    uint256 private constant STATUS_NOT_INITIALIZED = 0;
    uint256 private constant STATUS_INITIALIZED = 1;
    uint256 private constant DEFAULT_SIZE = 6;

    // Public signal offsets for the 50-signal layout (withdraw, fee, deposit circuits)
    // Layout (k=6): [HashSecrets×6][PublicKeys×6][PrevCommit×12][TxCommit×12][BlockNum][AnonSet×6][MsgTags×6][Nullifier]
    uint256 private constant ARRAY_HASH_SECRET_OFFSET = 0;
    uint256 private constant ARRAY_HASH_SECRET_SIZE = 6;
    uint256 private constant PUBLIC_KEY_OFFSET = 6;
    uint256 private constant PUBLIC_KEY_SIZE = 6;
    uint256 private constant PREVIOUS_COMMIT_OFFSET = 12;
    uint256 private constant PREVIOUS_COMMIT_SIZE = 12;
    uint256 private constant TX_COMMIT_OFFSET = 24;
    uint256 private constant TX_COMMIT_SIZE = 12;
    uint256 private constant BLOCK_NUMBER_OFFSET = 36;
    uint256 private constant K_INDEX_OFFSET = 37;
    uint256 private constant K_INDEX_SIZE = 6;
    uint256 private constant MESSAGE_TAGS_OFFSET = 43;
    uint256 private constant NULLIFIER_OFFSET = 49;

    // Fix M-13: the fee layout (enygma_fee circuit) extends the 50-signal
    // base with three more slots — the fee itself (50) and two aggregate
    // values the circuit computes but nothing on-chain previously read at
    // all: SumTxCommit.X/Y (51-52) and SumTxValuesWithFee (53). Only FEE_OFFSET
    // is used below; SumTxCommit/SumTxValuesWithFee are provably unusable as
    // an independent check (SumTxCommit is asserted equal to the constant
    // (0,1) two lines earlier in the circuit, and SumTxValuesWithFee is an
    // unbounded hint remainder) — see the fee-burn comment on transferWithFee
    // for why FEE_OFFSET alone is sufficient.
    uint256 private constant FEE_OFFSET = 50;

    // Fix M-14/C-09: withdraw and deposit are both genuinely 51-signal
    // layouts (the 50-signal constants above cover every index they
    // share; slot 50 is the extra one, meaning differs by function).
    // withdraw's slot 50 is TotalDepositValue (Fix C-09 — bound in-circuit
    // to Σ VPerDeposit == SenderTxValue; Enygma.sol's withdraw() checks it
    // against Σ depositParams[i].amount before calling out to the DvP
    // vault). deposit's slot 50 is Hash, the deposit note commitment
    // (already existed in the circuit; M-14's bug was the *contract*
    // treating this as a 50-signal array and never reading index 50 at
    // all — not a circuit change). Two separately-named constants at the
    // same numeric offset for self-documentation at each call site.
    uint256 private constant WITHDRAW_TOTAL_DEPOSIT_VALUE_OFFSET = 50;
    uint256 private constant DEPOSIT_HASH_OFFSET = 50;

    // Fix L-01: domain-separator offsets, one per circuit layout — the
    // slot immediately after the each layout's previous last signal, so
    // every layout gains exactly one signal. See _expectedDomainId() for
    // what value each is checked against.
    uint256 private constant BRIDGE_DOMAIN_OFFSET = 51; // withdraw/deposit (52-signal)

    // Public signal offsets for the 81-signal FingerPrint layout (main transfer circuit)
    // Layout (k=6): [FingerPrint×36][PublicKeys×6][PrevCommit×12][TxCommit×12][BlockNum][AnonSet×6][MsgTags×6][Nullifier][DomainId]
    uint256 private constant FP_FINGERPRINT_OFFSET = 0;
    uint256 private constant FP_FINGERPRINT_SIZE = 36;
    uint256 private constant FP_PUBLIC_KEY_OFFSET = 36;
    uint256 private constant FP_PUBLIC_KEY_SIZE = 6;
    uint256 private constant FP_PREVIOUS_COMMIT_OFFSET = 42;
    uint256 private constant FP_PREVIOUS_COMMIT_SIZE = 12;
    uint256 private constant FP_TX_COMMIT_OFFSET = 54;
    uint256 private constant FP_TX_COMMIT_SIZE = 12;
    uint256 private constant FP_BLOCK_NUMBER_OFFSET = 66;
    uint256 private constant FP_K_INDEX_OFFSET = 67;
    uint256 private constant FP_K_INDEX_SIZE = 6;
    uint256 private constant FP_MESSAGE_TAGS_OFFSET = 73;
    uint256 private constant FP_NULLIFIER_OFFSET = 79;
    uint256 private constant FP_DOMAIN_OFFSET = 80; // Fix L-01

    // Fix L-01: fee layout (54-signal base + FEE_OFFSET(50) = 55-signal
    // once the domain slot is added).
    uint256 private constant FEE_DOMAIN_OFFSET = 54;

    // Public signal offsets for the 9-signal burn circuit (Fix H-13).
    // Layout: [PublicKey, PrevCommit×2, NewCommit×2, Amount, BlockNum, Nullifier, DomainId]
    uint256 private constant BURN_PUBLIC_KEY_OFFSET = 0;
    uint256 private constant BURN_PREVIOUS_COMMIT_OFFSET = 1;
    uint256 private constant BURN_NEW_COMMIT_OFFSET = 3;
    uint256 private constant BURN_AMOUNT_OFFSET = 5;
    uint256 private constant BURN_BLOCK_NUMBER_OFFSET = 6;
    uint256 private constant BURN_NULLIFIER_OFFSET = 7;
    uint256 private constant BURN_DOMAIN_OFFSET = 8; // Fix L-01

    // ============================================
    // STATE VARIABLES
    // ============================================
    // Token metadata
    string private constant TOKEN_NAME = "Enygma";
    string private constant TOKEN_SYMBOL = "EN";
    uint8 private constant DECIMALS = 2;

    // Contract state
    uint256 private _status;
    // Fix H-08: was `immutable`. A fully-privileged permanent administrator
    // (mint, burn, seize-via-re-registration, verifier replacement) with no
    // rotation path is unrecoverable on key loss or compromise — and C-06
    // already demonstrated a key *can* leak. Two-step transfer below.
    address private _owner;
    address private _pendingOwner;
    bool private _paused;
    bool private _reentrancyLock; // Fix L-12
    uint256 public immutable epochInterval;
    uint256 public lastBlockNum;
    uint256 private _totalRegisteredParties;

    // Total supply as Pedersen commitment point (x, y)
    uint256 public totalSupplyX;
    uint256 public totalSupplyY;
    uint256 public totalSupplyAmount;

    /// @notice Fix M-13: the fee `transferWithFee`'s proof must carry at
    /// public_signal[FEE_OFFSET]. Enforced by exact match (not merely read)
    /// so the fee is mandatory, not advisory — a prover cannot submit
    /// fee=0 to skip it. Owner-configurable, defaults to 0 (no fee
    /// required) until explicitly set. See setProtocolFee / transferWithFee.
    uint256 public protocolFee;

    // Verifier addresses
    address private _transferVerifier;
    address private _withdrawVerifier;
    address private _depositVerifier;
    address private _zkDvpAddress;
    address private _feeVerifier;
    address private _burnVerifier;

    // ============================================
    // MAPPINGS
    // ============================================

    /// @notice Balance commitments per block per account
    mapping(uint256 => mapping(uint256 => Point)) public balanceCommitments;

    /// @notice Public spend keys for each account (Poseidon(sk,sk) mod P)
    mapping(uint256 => uint256) public publicKeys;

    /// @notice Public view keys for each account (ML-KEM-768 encapsulation key, 1184 bytes)
    mapping(uint256 => bytes) public viewKeys;

    /// @notice Maps Ethereum address to account ID
    mapping(address => uint256) public addressToAccountId;

    /// @notice Transfer verifier contracts by participant count
    mapping(uint256 => address) private _transferVerifiers;

    /// @notice Withdraw verifier contracts by split count
    mapping(uint256 => address) private _withdrawVerifiers;

    /// @notice Deposit verifier contracts
    mapping(uint256 => address) private _depositVerifiers;

    /// @notice ZkDvp contract addresses
    mapping(uint256 => address) private _zkDvpContracts;

    /// @notice Consumed nullifiers — prevents proof replay
    mapping(uint256 => bool) private _nullifiers;

    // ── Fix C-04 ────────────────────────────────────────────────────────
    // Each participant's blinding-factor shift is derived from SharedSecrets[i],
    // a private circuit witness the SENDER alone chooses — the
    // FingerPrintofSharedSecrets matrix that's supposed to bind it to a real
    // pairwise secret is only ever checked for self-consistency inside the
    // circuit (Poseidon(SharedSecrets[i]) == the sender's own claimed
    // fingerprint for that cell), never against anything the recipient
    // controls. That let any sender permanently freeze any other
    // registered account's balance by fabricating a shared secret the
    // victim never agreed to. Two-party confirmation closes this: a
    // fingerprint only becomes authoritative once BOTH parties have
    // independently submitted the identical value, so neither can write
    // one unilaterally, and _verifyFingerprints below requires every
    // pairwise fingerprint among a transfer's participants to be
    // confirmed AND to match exactly what's on chain — closing the loop
    // the circuit already half-built (it ties SharedSecrets[i], the value
    // that actually shifts recipient i's blinding factor, to that same
    // public signal for every non-sender row; the missing piece was
    // purely on this contract's side, which declared the offsets and
    // never read them).

    /// @notice What `caller` claims Poseidon(shared_secret) is with `other`.
    /// Indexed [callerId][otherId]; 0 means "not yet submitted" (a real
    /// Poseidon output landing on exactly 0 is negligible-probability,
    /// same convention balanceCommitments already relies on).
    mapping(uint256 => mapping(uint256 => uint256)) public pendingFingerprint;

    /// @notice The confirmed, symmetric fingerprint for a pair once both
    /// sides' submissions have matched. confirmedFingerprint[i][j] ==
    /// confirmedFingerprint[j][i] always.
    mapping(uint256 => mapping(uint256 => uint256)) public confirmedFingerprint;

    /// @notice Whether a pair's fingerprint has been mutually confirmed.
    mapping(uint256 => mapping(uint256 => bool)) public fingerprintConfirmed;

    // ============================================
    // EVENTS
    // ============================================

    event Commitment(uint256 indexed commitment);
    event FingerprintPending(uint256 indexed fromId, uint256 indexed toId, uint256 fingerprint);
    event FingerprintConfirmed(uint256 indexed partyA, uint256 indexed partyB, uint256 fingerprint);
    // Fix H-08
    event OwnershipTransferStarted(address indexed previousOwner, address indexed newOwner);
    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);
    event Paused(address account);
    event Unpaused(address account);
    // ============================================
    // ERRORS
    // ============================================

    error NotOwner();
    error NotRegistered();
    error AlreadyInitialized();
    error NotInitialized();
    error InvalidProof();
    error InvalidPublicInputs();
    error BalanceMismatch();
    error BurnExceedsModulus();
    error ZeroAddress();
    error VerifierNotFound();
    error ZkDvpOperationFailed();
    error InvalidBlockNumber();
    error NullifierAlreadyUsed();
    error ParticipantIdsLengthMismatch();
    error ParticipantIdsNotSorted();
    error InvalidParticipantCount();
    error InvalidFingerprintParty();
    error InvalidCommitmentPoint();
    error VerifierHasNoCode();
    error AlreadyRegistered();
    error UnregisteredParticipant();
    error InvalidAccountId();
    error InvalidPublicKey();
    error InvalidViewKeyLength();
    error NotPendingOwner();
    error NewOwnerIsZeroAddress();
    error ContractIsPaused();
    error FingerprintNotConfirmed();
    error InvalidFee(); // Fix M-13
    error FeeExceedsModulus(); // Fix M-13
    error DepositValueMismatch(); // Fix C-09
    error ReentrancyGuardReentrantCall(); // Fix L-12
    error InvalidDomain(); // Fix L-01

    // ============================================
    // MODIFIERS
    // ============================================

    modifier onlyOwner() {
        if (msg.sender != _owner) revert NotOwner();
        _;
    }

    modifier onlyRegistered() {
        if (addressToAccountId[msg.sender] == 0) revert NotRegistered();
        _;
    }

    modifier whenInitialized() {
        if (_status != STATUS_INITIALIZED) revert NotInitialized();
        _;
    }

    // Fix H-08: circuit breaker for value-moving/state-mutating entry
    // points. Deliberately does NOT gate transferOwnership/acceptOwnership/
    // pause/unpause themselves — those must stay reachable while paused,
    // or a pause could never be lifted.
    modifier whenNotPaused() {
        if (_paused) revert ContractIsPaused();
        _;
    }

    /// @notice Fix L-12: withdraw()/deposit() are the only two functions
    ///         that call out to an external contract (the owner-configured
    ///         ZkDvp bridge) — and, before this fix, did so before their
    ///         own state updates, a checks-effects-interactions violation
    ///         with no reentrancy guard anywhere in this contract (`grep
    ///         -rn "nonReentrant|ReentrancyGuard|_locked|_entered"` — zero
    ///         hits). The same-proof case was already safe (the nullifier
    ///         is consumed first), but a different valid proof against the
    ///         same stale pre-state was not. No hostile callee is reachable
    ///         in the intended deployment today (_zkDvpAddress is
    ///         owner-set), so this is defense in depth for a future vault,
    ///         not a live exploit closed — added regardless per the
    ///         audit's own "cheap, and should not wait for the bridge to
    ///         be wired."
    modifier nonReentrant() {
        if (_reentrancyLock) revert ReentrancyGuardReentrantCall();
        _reentrancyLock = true;
        _;
        _reentrancyLock = false;
    }

    // ============================================
    // CONSTRUCTOR
    // ============================================

    constructor(uint256 _epochInterval) {
        require(_epochInterval > 0, "epochInterval must be > 0");
        _owner = msg.sender;
        _status = STATUS_NOT_INITIALIZED;
        epochInterval = _epochInterval;
        lastBlockNum = _currentEpochStart(_epochInterval);
    }

    // ============================================
    // OWNERSHIP & PAUSE (Fix H-08)
    // ============================================
    //
    // Two-step transfer (propose, then accept from the new address) so a
    // typo in the target address can never permanently strand ownership —
    // the current owner keeps control until the pending owner actively
    // claims it. No owner() getter existed before this fix at all, so a
    // deployed instance's administrator was not even readable on-chain.
    //
    // Deliberately NOT included here: role separation (issuer / registrar
    // / verifier-admin as distinct addresses) and renounceOwnership. Both
    // are in the audit's remediation for H-08 but are a larger design
    // decision (how many roles, what each can do, migration for existing
    // deployments) than a two-step transfer + pause — left for a
    // follow-up rather than folded in here.

    /// @notice Current owner. There was no getter at all before this fix.
    function owner() external view returns (address) {
        return _owner;
    }

    /// @notice Address that has been proposed as the next owner but has
    ///         not yet accepted, or address(0) if none is pending.
    function pendingOwner() external view returns (address) {
        return _pendingOwner;
    }

    /// @notice Whether value-moving entry points are currently halted.
    function paused() external view returns (bool) {
        return _paused;
    }

    /// @notice Step 1 of 2: propose a new owner. Has no effect until that
    ///         address calls acceptOwnership() itself.
    function transferOwnership(address newOwner) external onlyOwner returns (bool) {
        if (newOwner == address(0)) revert NewOwnerIsZeroAddress();
        _pendingOwner = newOwner;
        emit OwnershipTransferStarted(_owner, newOwner);
        return true;
    }

    /// @notice Step 2 of 2: the pending owner claims ownership. Only
    ///         callable by the address transferOwnership named.
    function acceptOwnership() external returns (bool) {
        if (msg.sender != _pendingOwner) revert NotPendingOwner();
        address previousOwner = _owner;
        _owner = msg.sender;
        _pendingOwner = address(0);
        emit OwnershipTransferred(previousOwner, _owner);
        return true;
    }

    /// @notice Halt every whenNotPaused entry point (transfer,
    ///         transferWithFee, withdraw, deposit, burn, mintSupply,
    ///         registerAccount, registerFingerprint). Ownership and pause
    ///         management themselves are never gated by this.
    function pause() external onlyOwner returns (bool) {
        _paused = true;
        emit Paused(msg.sender);
        return true;
    }

    function unpause() external onlyOwner returns (bool) {
        _paused = false;
        emit Unpaused(msg.sender);
        return true;
    }

    /// @notice Fix M-13: set the fee transferWithFee's proof must carry
    ///         exactly (public_signal[FEE_OFFSET]). Not gated by
    ///         whenNotPaused — an administrative config change, not a
    ///         value-moving operation, same as addVerifier.
    function setProtocolFee(uint256 newFee) external onlyOwner returns (bool) {
        if (newFee >= CurveBabyJubJub.P) revert FeeExceedsModulus();
        emit ProtocolFeeUpdated(protocolFee, newFee);
        protocolFee = newFee;
        return true;
    }

    // ============================================
    // INITIALIZATION
    // ============================================

    /**
     * @notice Initialize contract and set total supply to neutral element
     * @return success True if initialization succeeds
     */
    function initialize() external onlyOwner returns (bool) {
        if (_status == STATUS_INITIALIZED) revert AlreadyInitialized();

        _status = STATUS_INITIALIZED;
        totalSupplyX = 0;
        totalSupplyY = 1; // Neutral element on Baby Jubjub

        return true;
    }

    // ============================================
    // ACCOUNT MANAGEMENT
    // ============================================

    /**
     * @notice Register new account with initial balance commitment
     * @param addr Ethereum address to register
     * @param accountId Unique account identifier
     * @param publicKey Institution public spend key (Poseidon(sk,sk) mod P)
     * @param randomness Randomness for initial balance commitment
     * @param viewKey Institution public view key (ML-KEM-768 encapsulation key, 1184 bytes)
     */

    /**
     * @dev Fix H-02 residual: `randomness` used to be a raw uint256
     *      published in plaintext calldata, and Com(0, randomness) was
     *      computed on chain from it — meaning any chain observer could
     *      read the account's entire initial blinding factor directly off
     *      the transaction, and (combined with mintSupply's r=0 issuance,
     *      see below) recover every subsequent balance until the account's
     *      first shielded transfer. initialCommit is now the account
     *      holder's own Com(0, r) computed OFF chain with a secret r that
     *      never appears in calldata; the contract only ever sees the
     *      resulting curve point, which the hiding property of Pedersen
     *      commitments protects regardless of what gets homomorphically
     *      added to it later (which is what makes fixing this alone —
     *      independent of mintSupply's r=0 — already sufficient to close
     *      the audit's "balance is fully open" finding: a single secret
     *      blinding contribution anywhere in a commitment's history is
     *      enough for perfect hiding of the total).
     */
    function registerAccount(
        address addr,
        uint256 accountId,
        uint256 publicKey,
        uint256 initialCommitX,
        uint256 initialCommitY,
        bytes calldata viewKey
    ) external onlyOwner whenInitialized whenNotPaused returns (bool) {
        // Fix L-02: was missing whenInitialized (every other participant
        // mutator has it; registerAccount and burn were the two
        // exceptions). initialize() unconditionally resets
        // totalSupplyX/Y to the neutral element (0,1) — if registerAccount
        // ran first, its initialCommit would already have been added to
        // totalSupply's (0,0) pre-initialize value, which is the
        // *absorbing* element for pointAdd (H-03), silently discarding
        // that contribution — and initialize() then overwrites
        // totalSupply with (0,1) regardless, permanently losing it with
        // no recovery function. Requiring initialize() first closes the
        // gap outright rather than relying on every deploy script
        // happening to call things in the right order.
        // Fix M-06: registerAccount was validation-free. A repeat call for
        // an already-live account reset its balance to the new
        // initialCommit (destroying it), added a second copy of that
        // point to totalSupply, and incremented _totalRegisteredParties a
        // second time — which is exactly how H-07's in-range-but-
        // unregistered ids got manufactured, since _verifyPublicInputsFP
        // reads getPublicValues(_totalRegisteredParties + 1). The demo's
        // own /run/register-bank handler logs a repeat call as "OK —
        // idempotent", which it was not.
        if (publicKeys[accountId] != 0) revert AlreadyRegistered();
        if (accountId == 0) revert InvalidAccountId();
        if (publicKey == 0) revert InvalidPublicKey();
        // viewKey is legitimately empty for accounts that never
        // participate in ZK circuits (e.g. the relayer's own self-
        // registration — see relayer/cmd/register/main.go); anything
        // else must be a real ML-KEM-768 encapsulation key (1184 bytes),
        // not a truncated or malformed one that would silently break key
        // agreement for whoever tries to use it later.
        if (viewKey.length != 0 && viewKey.length != 1184) revert InvalidViewKeyLength();
        if (!CurveBabyJubJub.isOnCurve(initialCommitX, initialCommitY)) {
            revert InvalidCommitmentPoint();
        }

        publicKeys[accountId] = publicKey;
        viewKeys[accountId] = viewKey;
        addressToAccountId[addr] = accountId;

        // Store the caller-supplied initial commitment directly — no
        // on-chain arithmetic on a secret value, matching how every other
        // commitment in this contract (TxCommit, NewCommit, ...) is
        // written verbatim rather than derived from a plaintext opening.
        balanceCommitments[lastBlockNum][accountId] = Point(initialCommitX, initialCommitY);

        // Include initial commitment in totalSupply so check() invariant holds:
        // Σ(balances) = Σ(registration commitments) + Σ(minted amounts) = totalSupply
        (totalSupplyX, totalSupplyY) = CurveBabyJubJub.pointAdd(
            totalSupplyX, totalSupplyY,
            initialCommitX, initialCommitY
        );

        unchecked {
            ++_totalRegisteredParties;
        }

        // Fix M-06: emit accountId, not the post-increment counter.
        emit AccountRegistered(addr, accountId);
        return true;
    }

    // ============================================
    // FINGERPRINT REGISTRY (Fix C-04)
    // ============================================

    /**
     * @notice Submit this account's claim of the Poseidon fingerprint of
     *         the shared secret with `otherPartyId`, derived off-chain via
     *         key agreement. Becomes authoritative for transfer() once
     *         `otherPartyId` submits the identical value in the other
     *         direction — until then it's just this account's pending
     *         claim, unusable by anyone (including this account) to pass
     *         _verifyFingerprints.
     * @param otherPartyId The counterparty account ID this fingerprint is for.
     * @param fingerprint  Poseidon(shared_secret) as computed off-chain.
     */
    function registerFingerprint(
        uint256 otherPartyId,
        uint256 fingerprint
    ) external onlyRegistered whenNotPaused returns (bool) {
        uint256 callerId = addressToAccountId[msg.sender];
        if (otherPartyId == 0 || otherPartyId == callerId) {
            revert InvalidFingerprintParty();
        }

        pendingFingerprint[callerId][otherPartyId] = fingerprint;
        emit FingerprintPending(callerId, otherPartyId, fingerprint);

        uint256 counterpartClaim = pendingFingerprint[otherPartyId][callerId];
        if (counterpartClaim != 0 && counterpartClaim == fingerprint) {
            confirmedFingerprint[callerId][otherPartyId] = fingerprint;
            confirmedFingerprint[otherPartyId][callerId] = fingerprint;
            fingerprintConfirmed[callerId][otherPartyId] = true;
            fingerprintConfirmed[otherPartyId][callerId] = true;
            emit FingerprintConfirmed(callerId, otherPartyId, fingerprint);
        }
        return true;
    }

    // ============================================
    // SUPPLY MANAGEMENT
    // ============================================

    /**
     * @notice Mint new tokens to specific account
     * @param amount Amount to mint
     * @param recipientId Account ID to receive tokens
     */
    /**
     * @dev Fix H-02 residual: minting used to add Com(amount, 0) —
     *      `derivePk(amount)`, zero blinding — computed on chain. mintCommit
     *      is now the issuer's own Com(amount, r_mint) computed OFF chain
     *      with a fresh, non-zero r_mint (which must reach the recipient
     *      off-chain, the same way registration's r does, so they can add
     *      it to their own running blinding factor). This trades the
     *      old, trivially-self-evident on-chain link between `amount` and
     *      the delta point for owner-asserted correctness — the same
     *      trust tier mintSupply already operates at (onlyOwner) — which
     *      is why _enforceInvariant() below now runs on every mint: if
     *      the owner ever supplies a mintCommit that doesn't actually
     *      open to `amount`, the mismatch surfaces immediately as a
     *      revert here rather than as a later, harder-to-diagnose
     *      check() failure.
     */
    function mintSupply(
        uint256 amount,
        uint256 recipientId,
        uint256 mintCommitX,
        uint256 mintCommitY
    ) external onlyOwner whenInitialized whenNotPaused returns (bool) {
        if (!CurveBabyJubJub.isOnCurve(mintCommitX, mintCommitY)) {
            revert InvalidCommitmentPoint();
        }

        // Update total supply commitment
        (totalSupplyX, totalSupplyY) = CurveBabyJubJub.pointAdd(
            totalSupplyX,
            totalSupplyY,
            mintCommitX,
            mintCommitY
        );

        unchecked {
            totalSupplyAmount += amount;
        }

        // Propagate balances to current epoch start
        _propagateBalancesExcept(recipientId);

        // Update recipient's balance
        Point storage recipientBalance = balanceCommitments[lastBlockNum][
            recipientId
        ];
        (uint256 newX, uint256 newY) = CurveBabyJubJub.pointAdd(
            recipientBalance.c1,
            recipientBalance.c2,
            mintCommitX,
            mintCommitY
        );

        uint256 epochStart = _currentEpochStart();
        balanceCommitments[epochStart][recipientId] = Point(newX, newY);
        lastBlockNum = epochStart;

        emit SupplyMinted(lastBlockNum, amount, recipientId);

        _enforceInvariant();

        return true;
    }

    /**
     * @notice Burn tokens from specific account
     * @dev Fix H-13: burn used to be plaintext arithmetic
     *      (Com(balance,r) + Com(P-amount,0)) on a hidden balance, with no
     *      check that amount did not exceed it — and fundamentally there
     *      could not be one, because the contract cannot open a Pedersen
     *      commitment. An over-burn silently wrapped the committed value
     *      to something just under P, with no revert and no distinguishable
     *      event. The account now proves in zero knowledge that it knows
     *      an opening of its current balance commitment, that the opened
     *      balance is >= amount, and what the correctly-updated commitment
     *      is (gnark-server/pkg/circuits/burn) — the contract performs no
     *      arithmetic on the hidden value at all, it just verifies the
     *      proof and writes the commitment the proof asserts. This also
     *      means burn is no longer purely owner-administrative: whoever
     *      calls it must have obtained a proof only the account holder's
     *      secret key could produce.
     * @param accountId Account to burn from
     * @param proof Zero-knowledge proof of solvency and correct new commitment
     */
    function burn(
        uint256 accountId,
        BurnProof calldata proof
    ) external onlyOwner whenInitialized whenNotPaused returns (bool) {
        address verifier = _burnVerifier;
        if (verifier == address(0)) revert VerifierNotFound();
        if (verifier.code.length == 0) revert VerifierHasNoCode();

        // Fix M-01: staticcall, not delegatecall — every committed verifier
        // is `public view` with zero sstore instructions, so this is
        // behaviour-identical for any verifier this repository ships, and
        // it closes the class of bug where a malicious/buggy verifier
        // rewrites Enygma's own storage instead of just answering a
        // yes/no. The code.length check above closes the other half: a
        // delegatecall/staticcall to a codeless address returns
        // success=true, which looked identical to a valid proof.
        (bool success, ) = verifier.staticcall(
            abi.encodeWithSignature("verifyProof(uint256[8],uint256[9])", proof)
        );
        if (!success) revert InvalidProof();

        // Fix L-01: see _expectedDomainId's doc comment.
        if (proof.public_signal[BURN_DOMAIN_OFFSET] != _expectedDomainId()) {
            revert InvalidDomain();
        }

        // Bind the proof to this specific, named account and its current
        // on-chain balance — otherwise a valid proof for ANY account could
        // be replayed against a different accountId.
        if (proof.public_signal[BURN_PUBLIC_KEY_OFFSET] != publicKeys[accountId]) {
            revert InvalidPublicInputs();
        }
        (uint256 balX, uint256 balY) = getBalance(accountId);
        if (
            proof.public_signal[BURN_PREVIOUS_COMMIT_OFFSET] != balX ||
            proof.public_signal[BURN_PREVIOUS_COMMIT_OFFSET + 1] != balY
        ) {
            revert InvalidPublicInputs();
        }

        uint256 amount = proof.public_signal[BURN_AMOUNT_OFFSET];
        // Defense in depth: the circuit already range-checks Amount to 64
        // bits, so this can't actually trigger from a valid proof, but a
        // cheap on-chain sanity bound costs nothing. (The audit's original
        // ">" vs ">=" bug lived in the plaintext-arithmetic branch this
        // fix removes entirely, rather than being patched in place.)
        if (amount >= CurveBabyJubJub.P) revert BurnExceedsModulus();

        if (
            uint256(proof.public_signal[BURN_BLOCK_NUMBER_OFFSET]) != lastBlockNum
        ) {
            revert InvalidBlockNumber();
        }

        uint256 nullifier = proof.public_signal[BURN_NULLIFIER_OFFSET];
        if (_nullifiers[nullifier]) revert NullifierAlreadyUsed();
        _nullifiers[nullifier] = true;

        // Propagate balances for all other accounts, then write the new
        // commitment the proof asserts — no arithmetic on the hidden value.
        _propagateBalancesExcept(accountId);

        uint256 epochStart = _currentEpochStart();
        balanceCommitments[epochStart][accountId] = Point(
            proof.public_signal[BURN_NEW_COMMIT_OFFSET],
            proof.public_signal[BURN_NEW_COMMIT_OFFSET + 1]
        );
        lastBlockNum = epochStart;

        // Supply accounting (the remediation's second half): decrement
        // totalSupplyAmount and homomorphically subtract amount*G from the
        // totalSupply commitment, mirroring mintSupply's addition in
        // reverse. Pre-fix, burn never touched totalSupply at all, so
        // Σ(balances) silently diverged from totalSupply in both
        // representations after every burn.
        (uint256 negAmountX, uint256 negAmountY) = pedCom(
            CurveBabyJubJub.P - amount,
            0
        );
        (totalSupplyX, totalSupplyY) = CurveBabyJubJub.pointAdd(
            totalSupplyX,
            totalSupplyY,
            negAmountX,
            negAmountY
        );
        unchecked {
            totalSupplyAmount -= amount;
        }

        emit BurnSuccessful(accountId, amount);

        // Remediation's explicit ask: "add a caller for check() so
        // invariant breaks surface" — a broken invariant now reverts the
        // burn itself instead of staying silently wrong until someone
        // manually queries check().
        _enforceInvariant();

        return true;
    }

    // ============================================
    // VERIFIER MANAGEMENT
    // ============================================
    /**
     * @notice Register transfer verifier contract
     * @param verifier Address of verifier contract
     */
    function addVerifier(address verifier) external onlyOwner returns (bool) {
        if (verifier == address(0)) revert ZeroAddress();
        if (verifier.code.length == 0) revert VerifierHasNoCode();

        _transferVerifiers[DEFAULT_SIZE] = verifier;
        _transferVerifier = verifier;

        emit VerifierRegistered(verifier, _totalRegisteredParties);
        return true;
    }
    /**
     * @notice Register withdraw verifier contract
     * @param verifier Address of verifier contract
     * @param splitCount Withdraw statement's participant count. Fix H-14:
     *        withdraw() now looks up _withdrawVerifiers[commitmentDeltas.length],
     *        not a DvP-split count, so only splitCount == DEFAULT_SIZE (6) is
     *        ever reachable — register the verifier under 6 regardless of how
     *        many ZkDvp deposit splits a withdrawal fans out into.
     */
    function addWithdrawVerifier(
        address verifier,
        uint256 splitCount
    ) external onlyOwner returns (bool) {
        if (verifier == address(0)) revert ZeroAddress();
        if (verifier.code.length == 0) revert VerifierHasNoCode();

        _withdrawVerifiers[splitCount] = verifier;
        _withdrawVerifier = verifier;

        emit VerifierRegistered(verifier, _totalRegisteredParties);
        return true;
    }
    /**
     * @notice Register deposit verifier contract
     * @param verifier Address of verifier contract
     */
    function addDepositVerifier(
        address verifier
    ) external onlyOwner returns (bool) {
        if (verifier == address(0)) revert ZeroAddress();
        if (verifier.code.length == 0) revert VerifierHasNoCode();

        _depositVerifiers[DEFAULT_SIZE] = verifier;
        _depositVerifier = verifier;

        emit VerifierRegistered(verifier, _totalRegisteredParties);
        return true;
    }

    /**
     * @notice Register fee transfer verifier contract (verifies 51-signal fee proofs)
     * @param verifier Address of fee verifier contract
     */
    function addFeeVerifier(address verifier) external onlyOwner returns (bool) {
        if (verifier == address(0)) revert ZeroAddress();
        if (verifier.code.length == 0) revert VerifierHasNoCode();

        _feeVerifier = verifier;

        emit VerifierRegistered(verifier, _totalRegisteredParties);
        return true;
    }

    /**
     * @notice Register burn verifier contract (Fix H-13)
     * @param verifier Address of verifier contract
     */
    function addBurnVerifier(address verifier) external onlyOwner returns (bool) {
        if (verifier == address(0)) revert ZeroAddress();
        if (verifier.code.length == 0) revert VerifierHasNoCode();

        _burnVerifier = verifier;

        emit VerifierRegistered(verifier, _totalRegisteredParties);
        return true;
    }

    /**
     * @notice Register ZkDvp contract
     * @param zkDvp Address of ZkDvp contract
     */
    function addZkDvp(address zkDvp) external onlyOwner returns (bool) {
        if (zkDvp == address(0)) revert ZeroAddress();

        _zkDvpContracts[DEFAULT_SIZE] = zkDvp;
        _zkDvpAddress = zkDvp;

        emit VerifierRegistered(zkDvp, _totalRegisteredParties);
        return true;
    }

    // ============================================
    // TRANSFER OPERATIONS
    // ============================================

    /**
     * @notice Execute confidential transfer
     * @param commitmentDeltas Balance changes for each participant
     * @param proof Zero-knowledge proof
     * @param participantIds Account IDs involved in transfer
     * @param bankTag Fix H-09: optional, unvalidated caller-supplied
     *        attribution string (typically the relayer's per-bank
     *        credential id) — see RelayAttribution's doc comment in
     *        IEnygma.sol. Pass "" for no attribution.
     */
    function transfer(
        Point[] calldata commitmentDeltas,
        Proof calldata proof,
        uint256[] calldata participantIds,
        string calldata bankTag
    ) external onlyRegistered whenInitialized whenNotPaused returns (bool) {
        // Verify zero-knowledge proof
        _verifyTransferProof(proof, commitmentDeltas.length);

        // Verify public inputs match current state and commitment deltas match proof
        _verifyPublicInputsFP(proof.public_signal, participantIds, commitmentDeltas);

        // Fix C-04: verify recipients' shared-secret fingerprints against
        // mutually-confirmed on-chain values, not just the sender's own
        // self-consistent claim.
        _verifyFingerprints(proof.public_signal, participantIds);

        // Verify block number freshness
        _verifyBlockNumberFP(proof.public_signal);

        // Record nullifier before state changes (Fix C-2)
        _consumeNullifierFP(proof.public_signal);

        // Update balances
        _updateBalancesForTransfer(commitmentDeltas, participantIds);

        emit TransactionSuccessful(msg.sender);
        // Fix H-09: same transaction as TransactionSuccessful above, so
        // the attribution is atomically tied to this specific transfer —
        // not a separate, independently-orderable call.
        emit RelayAttribution(msg.sender, bankTag);
        return true;
    }

    /**
     * @notice Execute confidential transfer with public fee (51-signal fee circuit)
     * @param commitmentDeltas Balance changes for each participant
     * @param proof Zero-knowledge fee proof (51 public signals; fee at index 50)
     * @param participantIds Account IDs involved in transfer
     * @dev C-04 NOT fixed here. enygma_fee's circuit exposes
     *      HashedSharedSecrets as a flat per-participant array (index i =
     *      "the secret between slot i and the sender"), not the k×k
     *      matrix transfer()'s circuit uses — so which entry corresponds
     *      to which pair can't be checked without knowing which slot is
     *      the sender, which this contract deliberately never learns.
     *      Closing this the same way _verifyFingerprints does needs the
     *      fee circuit's signal layout changed first (a new trusted setup
     *      either way, batched with the other circuit-touching findings).
     *      This function is unreachable today — _feeVerifier is unset and
     *      addFeeVerifier has no callers — do not call addFeeVerifier
     *      before C-04 is closed here too.
     */
    function transferWithFee(
        Point[] calldata commitmentDeltas,
        FeeProof calldata proof,
        uint256[] calldata participantIds,
        string calldata bankTag
    ) external onlyRegistered whenInitialized whenNotPaused returns (bool) {
        _verifyFeeTransferProof(proof);
        _verifyFeePublicInputs(proof.public_signal, participantIds, commitmentDeltas);
        _verifyFeeBlockNumber(proof.public_signal);
        _consumeFeeNullifier(proof.public_signal);

        // Fix M-13: the circuit hard-asserts Σ(commitmentDeltas) + Fee·G ==
        // identity — i.e. the participants' deltas alone always sum to
        // -Fee·G, not to the identity a fee-less transfer requires. That
        // means value is unconditionally leaving the accounted pool on
        // every fee transfer, whether or not anything here reads the fee
        // signal. Pre-fix, nothing did: check()'s Σ(balances)==totalSupply
        // invariant silently and permanently broke on the very first fee
        // transfer, with the missing amount unrecoverable off-chain (it's
        // a curve-point offset, not a plaintext difference). Requiring an
        // exact match against protocolFee (not merely reading the signal)
        // makes the fee mandatory rather than advisory, and burning it
        // from totalSupply — decrementing by exactly what the circuit's
        // own Pedersen equation already forces the balance sum to lose —
        // keeps check() consistent instead of permanently false.
        uint256 fee = proof.public_signal[FEE_OFFSET];
        if (fee != protocolFee) revert InvalidFee();
        _burnFeeFromSupply(fee);

        _updateBalancesForTransfer(commitmentDeltas, participantIds);
        emit TransactionSuccessful(msg.sender);
        emit RelayAttribution(msg.sender, bankTag); // Fix H-09
        return true;
    }

    /**
     * @notice Burns `fee` out of totalSupply, mirroring burn()'s
     *         totalSupply accounting exactly (Fix M-13). Homomorphically
     *         subtracts fee·G (zero blinding, matching the circuit's own
     *         `ScalarMul(G, Fee)` — no random factor) from the totalSupply
     *         commitment and decrements the plaintext totalSupplyAmount
     *         mirror by the same amount.
     */
    function _burnFeeFromSupply(uint256 fee) private {
        if (fee == 0) return;
        (uint256 negFeeX, uint256 negFeeY) = pedCom(CurveBabyJubJub.P - fee, 0);
        (totalSupplyX, totalSupplyY) = CurveBabyJubJub.pointAdd(
            totalSupplyX,
            totalSupplyY,
            negFeeX,
            negFeeY
        );
        unchecked {
            totalSupplyAmount -= fee;
        }
        emit FeeBurned(fee);
    }

    /**
     * @notice Withdraw from Enygma to ZkDvp
     * @param commitmentDeltas Balance changes for Enygma accounts
     * @param proof Zero-knowledge proof
     * @param depositParams Parameters for ZkDvp deposits
     * @param participantIds Accounts involved
     */
    function withdraw(
        Point[] calldata commitmentDeltas,
        WithdrawProof calldata proof,
        DepositParams[] calldata depositParams,
        uint256[] calldata participantIds
    ) external onlyRegistered whenInitialized whenNotPaused nonReentrant returns (bool, uint256[] memory) {
        // Fix H-14: the verifier used to be selected by depositParams.length
        // — a completely different array from participantIds/commitmentDeltas,
        // the ones _verifyPublicInputs and _updateBalances actually bind and
        // mutate. That let a caller submit a nonzero depositParams (selecting
        // a real, registered verifier) alongside EMPTY participantIds/
        // commitmentDeltas: _verifyPublicInputs's whole body is a loop over
        // participantIds.length, so an empty array skipped every public-key,
        // previous-commitment and tx-commitment check, while
        // _executeZkDvpDeposits still minted caller-chosen DvP value with no
        // corresponding Enygma-side debit. Selecting the verifier by
        // commitmentDeltas.length instead — the same array transfer()/
        // deposit() already key their verifier lookups on — ties verifier
        // selection to the array that is actually bound and mutated, and the
        // explicit length checks below close the empty/mismatched-array gap
        // directly rather than relying on that coincidence.
        if (participantIds.length != commitmentDeltas.length) {
            revert ParticipantIdsLengthMismatch();
        }
        if (commitmentDeltas.length != DEFAULT_SIZE) {
            revert InvalidParticipantCount();
        }

        // Verify withdrawal proof
        address verifier = _withdrawVerifiers[commitmentDeltas.length];
        if (verifier == address(0)) revert VerifierNotFound();
        if (verifier.code.length == 0) revert VerifierHasNoCode(); // Fix M-01

        // Fix M-14: was uint256[50] — the real (and, after Fix C-09,
        // still-real) circuit arity is 51.
        (bool success, ) = verifier.staticcall(
            abi.encodeWithSignature("verifyProof(uint256[8],uint256[52])", proof)
        );
        if (!success) revert InvalidProof();

        // Verify public inputs are bound to current on-chain state and deltas match proof
        _verifyPublicInputs52(proof.public_signal, participantIds, commitmentDeltas);

        // Verify proof was generated against the current block
        _verifyBlockNumber52(proof.public_signal);

        // Record nullifier before state changes (Fix C-2)
        _consumeNullifier52(proof.public_signal);

        // Fix C-09: the circuit's own Σ VPerDeposit == SenderTxValue
        // assertion (withdraw/circuit.go) is only meaningful once this
        // contract actually checks the resulting TotalDepositValue signal
        // against what depositParams — the array this contract itself
        // forwards to the DvP vault — claims to be creating. Before this
        // check, depositParams was 31 free private witnesses' worth of
        // caller-chosen value with nothing on this side relating it to
        // the shielded debit at all: a caller could name an arbitrarily
        // small (or zero) SenderTxValue while depositParams minted
        // arbitrary-value DvP notes. Reverting here, before
        // _executeZkDvpDeposits ever runs, is what makes the circuit's
        // binding actually load-bearing instead of decorative.
        _verifyDepositValueBinding(depositParams, proof.public_signal[WITHDRAW_TOTAL_DEPOSIT_VALUE_OFFSET]);

        // Fix L-12: this used to call out to the ZkDvp bridge (below)
        // *before* updating Enygma's own state — a classic
        // checks-effects-interactions violation with no reentrancy guard
        // anywhere in the contract. The same-proof case was already safe
        // (the nullifier above is consumed first), but a *different*
        // valid proof against the same stale pre-state was not: mid-call,
        // getBalance()/lastBlockNum/check() all still reported
        // pre-withdraw values. The owner-configured _zkDvpAddress means
        // no hostile callee is reachable in the intended deployment today
        // (defense in depth, not a live path — see nonReentrant above,
        // added regardless per the audit's "cheap, and should not wait
        // for the bridge to be wired"), but a future vault that reads
        // Enygma state mid-call would otherwise see stale balances with
        // no warning. State now updates first; the external call moves last.
        _updateBalances(commitmentDeltas, participantIds);

        // Fix M-15: withdraw() previously never touched totalSupply at
        // all, despite the circuit's own Pedersen equation forcing
        // Σ(commitmentDeltas) to be a genuine debit (Fix M-15's direction
        // fix above) — permanently breaking check()'s Σ(balances)==
        // totalSupply invariant on the first withdraw. See _applySupplyDelta.
        _applySupplyDelta(commitmentDeltas);

        // Execute ZkDvp deposits (Fix L-12: now last, after all state is settled)
        uint256[] memory zkDvpCommitments = _executeZkDvpDeposits(
            depositParams
        );

        return (true, zkDvpCommitments);
    }

    /**
     * @notice Fix C-09: reverts unless Σ depositParams[i].amount exactly
     *         matches requiredTotal (withdraw()'s proof-carried
     *         TotalDepositValue signal). Factored out of withdraw() to
     *         keep its own local-variable count low enough to avoid a
     *         stack-too-deep compile error (same reasoning as
     *         _propagateBalancesExcept / _burnFeeFromSupply elsewhere in
     *         this file).
     */
    function _verifyDepositValueBinding(
        DepositParams[] calldata depositParams,
        uint256 requiredTotal
    ) private pure {
        uint256 totalDepositAmount;
        for (uint256 i; i < depositParams.length; ) {
            totalDepositAmount += depositParams[i].amount;
            unchecked {
                ++i;
            }
        }
        if (totalDepositAmount != requiredTotal) {
            revert DepositValueMismatch();
        }
    }

    /**
     * @notice Fix M-15: homomorphically adds Σ(commitmentDeltas) to
     *         totalSupply — shared by deposit() (a credit, so the sum is
     *         positive) and withdraw() (a debit, so the sum is already
     *         negative by construction after this fix's circuit-side
     *         direction correction) since the same operation is correct
     *         for both once the sign is right. Factored out to keep
     *         deposit()/withdraw()'s own local-variable counts low enough
     *         to avoid a stack-too-deep compile error.
     */
    function _applySupplyDelta(Point[] calldata commitmentDeltas) private {
        uint256 sumDeltaX;
        uint256 sumDeltaY = 1;
        for (uint256 i; i < commitmentDeltas.length; ) {
            (sumDeltaX, sumDeltaY) = CurveBabyJubJub.pointAdd(
                sumDeltaX, sumDeltaY,
                commitmentDeltas[i].c1, commitmentDeltas[i].c2
            );
            unchecked {
                ++i;
            }
        }
        (totalSupplyX, totalSupplyY) = CurveBabyJubJub.pointAdd(
            totalSupplyX, totalSupplyY,
            sumDeltaX, sumDeltaY
        );
    }

    /**
     * @notice Deposit to Enygma from ZkDvp
     * @param commitmentDeltas Balance changes for Enygma accounts
     * @param proof Zero-knowledge proof
     * @param withdrawParam ZkDvp withdrawal parameters
     * @param participantIds Accounts involved
     */
    function deposit(
        Point[] calldata commitmentDeltas,
        DepositProof calldata proof,
        WithdrawParams calldata withdrawParam,
        uint256[] calldata participantIds
    ) external onlyRegistered whenInitialized whenNotPaused nonReentrant returns (bool) {
        // Verify deposit proof
        address verifier = _depositVerifiers[commitmentDeltas.length];
        if (verifier == address(0)) revert VerifierNotFound();
        if (verifier.code.length == 0) revert VerifierHasNoCode(); // Fix M-01

        // Fix M-14: was uint256[50] — the deposit circuit has always
        // produced 51 signals (HashedSharedSecrets..Nullifier at 0-49,
        // Hash — the deposit note commitment — at 50); this contract
        // declaring 50 made every deposit() call revert InvalidProof
        // unconditionally, regardless of proof validity.
        (bool success, ) = verifier.staticcall(
            abi.encodeWithSignature("verifyProof(uint256[8],uint256[52])", proof)
        );
        if (!success) revert InvalidProof();

        // Verify public inputs are bound to current on-chain state and deltas match proof
        _verifyPublicInputs52(proof.public_signal, participantIds, commitmentDeltas);

        // Verify proof was generated against the current block
        _verifyBlockNumber52(proof.public_signal);

        // Record nullifier before state changes (Fix C-2)
        _consumeNullifier52(proof.public_signal);

        // NOT fixed here (C-09's symmetric deposit-side gap): nothing
        // below binds proof.public_signal[DEPOSIT_HASH_OFFSET] (the
        // deposit note commitment) against withdrawParam.transaction —
        // the audit's own remediation asks for exactly that. It is
        // deliberately not attempted in this fix: withdrawParam.transaction
        // is an IZkDvp.JoinSplitTransaction (proof + opaque uint256[]
        // statement + input/output counts) whose statement layout is
        // defined entirely by the sibling enygma_dvp project's own
        // join-split circuit, not by anything in this repository — and
        // that project's real vault (EnygmaErc20CoinVault.withdrawThroughEnygma)
        // takes an IEnygmaDvp.ProofReceipt, not the JoinSplitTransaction
        // this interface declares, a type mismatch discovered while
        // scoping this fix that means this call cannot reach the real
        // vault via ABI-compatible calldata at all today. Binding
        // DEPOSIT_HASH_OFFSET correctly needs that mismatch resolved and
        // the real statement layout cross-referenced first; doing it
        // blindly here risked a binding that looks fixed but checks the
        // wrong thing, which is worse than the documented gap. The
        // withdraw leg's symmetric gap (Fix C-09 above) has no such
        // cross-repo dependency and is fully fixed.
        // Fix L-12: state now updates first; the external ZkDvp call
        // moves last — see withdraw()'s identical comment for the full
        // reasoning (checks-effects-interactions, no reentrancy guard
        // previously existed anywhere in this contract).
        _updateBalances(commitmentDeltas, participantIds);

        // Fix M-15: deposit() previously never touched totalSupply,
        // despite the circuit's own Pedersen equation (after this fix's
        // direction correction above) forcing Σ(commitmentDeltas) to be a
        // genuine credit — permanently breaking check() on the first
        // deposit. See _applySupplyDelta (shared with withdraw()).
        _applySupplyDelta(commitmentDeltas);

        IZkDvp zkDvp = IZkDvp(_zkDvpAddress);
        if (!zkDvp.withdrawThroughEnygma(withdrawParam.transaction)) {
            revert ZkDvpOperationFailed();
        }

        return true;
    }

    // ============================================
    // VIEW FUNCTIONS
    // ============================================

    /**
     * @notice Get balance commitment for account
     * @param accountId Account to query
     * @return x X-coordinate of balance commitment
     * @return y Y-coordinate of balance commitment
     */
    function getBalance(
        uint256 accountId
    ) public view returns (uint256 x, uint256 y) {
        Point storage balance = balanceCommitments[lastBlockNum][accountId];

        // Return neutral element if uninitialized
        if (balance.c1 == 0 && balance.c2 == 0) {
            return (0, 1);
        }

        return (balance.c1, balance.c2);
    }

    /**
     * @notice Get public values for all accounts
     * @param count Number of accounts to query
     * @return balances Array of balance commitments
     * @return keys Array of public keys
     */
    function getPublicValues(
        uint256 count
    ) public view returns (Point[] memory balances, uint256[] memory keys) {
        balances = new Point[](count);
        keys = new uint256[](count);

        for (uint256 i; i < count; ) {
            (balances[i].c1, balances[i].c2) = getBalance(i);
            keys[i] = publicKeys[i];

            unchecked {
                ++i;
            }
        }
        return (balances, keys);
    }

    /**
     * @notice Verify all balances sum to total supply
     * @return True if sum matches total supply
     */
    function check() external view returns (bool) {
        return _checkInvariant();
    }

    /**
     * @notice Sum every registered account's balance commitment and
     *         compare against totalSupplyX/Y. Shared by the public,
     *         view-only check() and by burn()'s internal
     *         _enforceInvariant() (Fix H-13).
     */
    function _checkInvariant() private view returns (bool) {
        uint256 sumX;
        uint256 sumY = 1; // Start with neutral element

        // AccountIds are 1-based: registered banks occupy slots 1.._totalRegisteredParties.
        for (uint256 i = 1; i <= _totalRegisteredParties; ) {
            (uint256 balX, uint256 balY) = getBalance(i);
            (sumX, sumY) = CurveBabyJubJub.pointAdd(sumX, sumY, balX, balY);

            unchecked {
                ++i;
            }
        }

        if (totalSupplyX != sumX || totalSupplyY != sumY) {
            revert BalanceMismatch();
        }

        return true;
    }

    /**
     * @notice Reverts if the balances/totalSupply invariant is broken.
     *         Called at the end of burn() so an accounting error surfaces
     *         immediately instead of staying silently wrong until someone
     *         manually calls check() (Fix H-13's explicit remediation ask).
     */
    function _enforceInvariant() private view {
        _checkInvariant();
    }

    // Getter functions
    function Name() external pure returns (string memory) {
        return TOKEN_NAME;
    }

    function Symbol() external pure returns (string memory) {
        return TOKEN_SYMBOL;
    }

    function TotalRegisteredBanks() external view returns (uint256) {
        return _totalRegisteredParties;
    }

    function TotalSupply() external view returns (uint256) {
        return totalSupplyAmount;
    }

    function VerifierAddress() external view returns (address) {
        return _transferVerifier;
    }

    function WithdrawVerifierAddress() external view returns (address) {
        return _withdrawVerifier;
    }

    function DepositVerifierAddress() external view returns (address) {
        return _depositVerifier;
    }

    function ZkdvpAddress() external view returns (address) {
        return _zkDvpAddress;
    }

    function GetBlckHash() external view returns (uint256) {
        return lastBlockNum;
    }

    // ============================================
    // INTERNAL FUNCTIONS
    // ============================================

    /**
     * @notice Returns the start block of the current epoch.
     *         epochStart = floor(block.number / epochInterval) * epochInterval
     *         All balance writes within the same epoch share the same storage slot,
     *         so transactions chain correctly inside an epoch and lastBlockNum only
     *         advances when the epoch rolls over.
     */
    function _currentEpochStart() private view returns (uint256) {
        return (block.number / epochInterval) * epochInterval;
    }

    // Overload used in the constructor before the immutable is set.
    function _currentEpochStart(uint256 interval) private view returns (uint256) {
        return (block.number / interval) * interval;
    }

    /**
     * @notice Verify zero-knowledge proof for transfer
     */
    function _verifyTransferProof(
        Proof calldata proof,
        uint256 participantCount
    ) private view {
        address verifier = _transferVerifiers[participantCount];
        if (verifier == address(0)) revert VerifierNotFound();
        if (verifier.code.length == 0) revert VerifierHasNoCode(); // Fix M-01

        (bool success, ) = verifier.staticcall(
            abi.encodeWithSignature(
                "verifyProof(uint256[8],uint256[81])",
                proof
            )
        );
        if (!success) revert InvalidProof();
    }

    /**
     * @notice Fix L-01: the value every circuit's new domain-separator
     *         signal must equal. Nothing previously bound a proof to a
     *         specific chain id or deployed contract address — the same
     *         proof verified against any fresh deployment sharing the
     *         same pre-state (demonstrated live: two contracts deployed
     *         in the same epoch both accepted the identical proof).
     *         Packs chain id and this contract's own address into one
     *         field element (chainId << 160 | address) rather than using
     *         two separate signals — cheaper for every circuit, and an
     *         injective encoding since a 160-bit address leaves 96 bits
     *         of headroom for chainId, far more than any real chain id
     *         needs. No Poseidon hashing involved (deliberately — mixing
     *         this into e.g. the nullifier is exactly the kind of change
     *         that hits the Poseidon gadget's t<=4 S-box limit this
     *         codebase has already been bitten by twice, H-01/H-02); a
     *         plain arithmetic public signal is fully sufficient, since
     *         Groth16's verification equation already binds every public
     *         input to the proof cryptographically — a prover cannot
     *         submit a proof valid for chain A's expected value and have
     *         it also verify against chain B's, without having generated
     *         a fresh proof against chain B's value in the first place.
     */
    function _expectedDomainId() private view returns (uint256) {
        return (block.chainid << 160) | uint256(uint160(address(this)));
    }

    /**
     * @notice Fix M-14/C-09/L-01: [52]-arity — used by both deposit() and
     *         withdraw() now that both circuits are genuinely 52-signal
     *         (see WITHDRAW_TOTAL_DEPOSIT_VALUE_OFFSET/DEPOSIT_HASH_OFFSET's
     *         doc comment for slot 50; slot 51 is the new L-01 domain
     *         separator, checked here since every caller needs it
     *         identically, unlike slot 50). This function used to be two
     *         separate [50]- and [51]-arity copies
     *         (_verifyPublicInputs / _verifyPublicInputs51); the [50] one
     *         had no remaining callers after the C-09/M-14 fix moved
     *         deposit()/withdraw() onto the 51-wide layout, so it was
     *         deleted here rather than widened uselessly.
     */
    function _verifyPublicInputs52(
        uint256[52] calldata public_signal,
        uint256[] calldata participantIds,
        Point[] calldata commitmentDeltas
    ) private view {
        // Fix L-01: nothing previously bound a proof to a specific chain
        // id or deployed contract address, so the identical proof
        // verified against any fresh deployment sharing the same
        // pre-state (demonstrated: two contracts deployed in the same
        // epoch both accepted the same proof). The exploitable window is
        // narrow (a fresh instance's bootstrap, closing on the first
        // mint) but real. Checking this first, before any other
        // per-participant work, rejects a cross-deployment replay as
        // cheaply as possible.
        if (public_signal[BRIDGE_DOMAIN_OFFSET] != _expectedDomainId()) {
            revert InvalidDomain();
        }

        (Point[] memory balances, uint256[] memory keys) = getPublicValues(
            _totalRegisteredParties + 1
        );

        uint256 len = participantIds.length;
        for (uint256 i; i < len; ) {
            uint256 accountId = participantIds[i];

            if (keys[accountId] == 0) revert UnregisteredParticipant(); // Fix H-07

            if (
                uint256(public_signal[PUBLIC_KEY_OFFSET + i]) !=
                keys[accountId]
            ) {
                revert InvalidPublicInputs();
            }

            uint256 commitOffset = PREVIOUS_COMMIT_OFFSET + (i << 1);
            if (
                uint256(public_signal[commitOffset]) !=
                balances[accountId].c1 ||
                uint256(public_signal[commitOffset + 1]) !=
                balances[accountId].c2
            ) {
                revert InvalidPublicInputs();
            }

            uint256 txOffset = TX_COMMIT_OFFSET + (i << 1);
            if (
                commitmentDeltas[i].c1 != public_signal[txOffset] ||
                commitmentDeltas[i].c2 != public_signal[txOffset + 1]
            ) {
                revert InvalidPublicInputs();
            }

            unchecked {
                ++i;
            }
        }
    }

    /**
     * @notice [52]-arity twin of _verifyBlockNumber. See _verifyPublicInputs52.
     */
    function _verifyBlockNumber52(uint256[52] calldata public_signal) private view {
        if (uint256(public_signal[BLOCK_NUMBER_OFFSET]) != lastBlockNum) {
            revert InvalidBlockNumber();
        }
    }

    /**
     * @notice [52]-arity twin of _consumeNullifier. See _verifyPublicInputs52.
     */
    function _consumeNullifier52(uint256[52] calldata public_signal) private {
        uint256 nullifier = public_signal[NULLIFIER_OFFSET];
        if (_nullifiers[nullifier]) revert NullifierAlreadyUsed();
        _nullifiers[nullifier] = true;
    }

    /**
     * @notice Verify public inputs for the 80-signal FingerPrint transfer circuit
     */
    function _verifyPublicInputsFP(
        uint256[81] calldata public_signal,
        uint256[] calldata participantIds,
        Point[] calldata commitmentDeltas
    ) private view {
        // Fix L-01: see _verifyPublicInputs52's identical comment.
        if (public_signal[FP_DOMAIN_OFFSET] != _expectedDomainId()) {
            revert InvalidDomain();
        }

        (Point[] memory balances, uint256[] memory keys) = getPublicValues(
            _totalRegisteredParties + 1
        );

        uint256 len = participantIds.length;
        for (uint256 i; i < len; ) {
            uint256 accountId = participantIds[i];

            if (keys[accountId] == 0) revert UnregisteredParticipant(); // Fix H-07

            if (uint256(public_signal[FP_PUBLIC_KEY_OFFSET + i]) != keys[accountId]) {
                revert InvalidPublicInputs();
            }

            uint256 commitOffset = FP_PREVIOUS_COMMIT_OFFSET + (i << 1);
            if (
                uint256(public_signal[commitOffset]) != balances[accountId].c1 ||
                uint256(public_signal[commitOffset + 1]) != balances[accountId].c2
            ) {
                revert InvalidPublicInputs();
            }

            uint256 txOffset = FP_TX_COMMIT_OFFSET + (i << 1);
            if (
                commitmentDeltas[i].c1 != public_signal[txOffset] ||
                commitmentDeltas[i].c2 != public_signal[txOffset + 1]
            ) {
                revert InvalidPublicInputs();
            }

            unchecked {
                ++i;
            }
        }
    }

    /**
     * @notice Fix C-04: verify every pairwise fingerprint among a
     *         transfer's participants is mutually confirmed on-chain and
     *         matches exactly what the proof publishes. FingerPrintofSharedSecrets
     *         is a row-major k×k matrix at FP_FINGERPRINT_OFFSET (k =
     *         participantIds.length, always DEFAULT_SIZE for this circuit
     *         — the offsets below already assume that, same as every
     *         other FP_* constant). Checked uniformly across every
     *         off-diagonal cell rather than only "the sender's column",
     *         because the contract has no way to identify which slot is
     *         the sender without breaking the anonymity set — and doesn't
     *         need to: the circuit itself already ties SharedSecrets[i]
     *         (the value that shifts recipient i's blinding factor) to
     *         FingerPrintofSharedSecrets[i][senderCol] for every i, so
     *         requiring the FULL matrix to match confirmed registry
     *         entries is sufficient to pin down whichever column turns
     *         out to be the sender's, and costs an honest prover nothing
     *         extra — every fingerprint they'd need is already public on
     *         this same registry.
     *
     *         Reverting when a pair isn't confirmed (rather than skipping
     *         the check for that pair) is the actual security property:
     *         an attacker can always find a pair with no on-chain
     *         confirmation — that's precisely how C-04 targets a victim
     *         who never agreed to transact with them — so "unconfirmed
     *         passes through" would leave the freeze attack fully intact.
     */
    function _verifyFingerprints(
        uint256[81] calldata public_signal,
        uint256[] calldata participantIds
    ) private view {
        uint256 len = participantIds.length;
        for (uint256 i; i < len; ) {
            uint256 idI = participantIds[i];
            for (uint256 j; j < len; ) {
                if (i != j) {
                    uint256 idJ = participantIds[j];
                    if (!fingerprintConfirmed[idI][idJ]) {
                        revert FingerprintNotConfirmed();
                    }
                    uint256 offset = FP_FINGERPRINT_OFFSET + i * len + j;
                    if (
                        uint256(public_signal[offset]) !=
                        confirmedFingerprint[idI][idJ]
                    ) {
                        revert InvalidPublicInputs();
                    }
                }
                unchecked {
                    ++j;
                }
            }
            unchecked {
                ++i;
            }
        }
    }

    /**
     * @notice Verify block number freshness for the 80-signal FingerPrint transfer circuit
     */
    function _verifyBlockNumberFP(uint256[81] calldata public_signal) private view {
        if (uint256(public_signal[FP_BLOCK_NUMBER_OFFSET]) != lastBlockNum) {
            revert InvalidBlockNumber();
        }
    }

    /**
     * @notice Record nullifier as spent for the 80-signal FingerPrint transfer circuit
     */
    function _consumeNullifierFP(uint256[81] calldata public_signal) private {
        uint256 nullifier = public_signal[FP_NULLIFIER_OFFSET];
        if (_nullifiers[nullifier]) revert NullifierAlreadyUsed();
        _nullifiers[nullifier] = true;
    }

    /**
     * @notice Update balances for transfer participants
     * @dev Fix C-5: previously this read each participant's old balance from
     *      `balanceCommitments[lastBlockNum]` but wrote the new balance to
     *      `balanceCommitments[epochStart]` — two different storage slots.
     *      Off epoch boundary (epochStart == lastBlockNum) that aliased
     *      correctly, but on the first transaction of a new epoch a
     *      duplicated id in `participantIds` would read the same
     *      pre-rollover value twice and let the later write silently
     *      discard the earlier one, minting or burning value with no
     *      counterparty. Fixed by making this a genuine read-modify-write
     *      on a single slot: every registered account (participant or not)
     *      is copied forward to `epochStart` first, and every subsequent
     *      read/write in this function targets that same slot. Combined
     *      with the strict-ordering check below (which also rejects
     *      duplicate and zero ids), a repeated id can no longer produce two
     *      independent deltas against the same stale balance.
     */
    function _updateBalancesForTransfer(
        Point[] calldata commitmentDeltas,
        uint256[] calldata participantIds
    ) private {
        uint256 len = participantIds.length;
        if (len != commitmentDeltas.length) {
            revert ParticipantIdsLengthMismatch();
        }

        // Copy every registered account's balance forward to the current
        // epoch slot first, so the participant loop below reads and writes
        // that same slot instead of straddling lastBlockNum/epochStart.
        // Reuses _propagateBalancesExcept with excludeId = 0, an id no
        // registered account ever has (accountIds are 1-based), i.e.
        // "propagate everyone" — avoids duplicating that 1-based loop here
        // (H-03's off-by-one lived in a since-removed 0-based copy of it)
        // and keeps this function's local-variable count low enough to
        // avoid a stack-too-deep compile error.
        _propagateBalancesExcept(0);
        uint256 epochStart = _currentEpochStart();

        // Require participantIds strictly increasing: this rejects
        // duplicate ids outright (each id can debit/credit at most once)
        // and, since the first id must already be >= 1 to keep increasing,
        // rejects id 0 as well.
        uint256 prevAccountId;
        for (uint256 i; i < len; ) {
            uint256 accountId = participantIds[i];
            if (accountId <= prevAccountId) {
                revert ParticipantIdsNotSorted();
            }
            prevAccountId = accountId;

            Point storage bal = balanceCommitments[epochStart][accountId];
            (uint256 newX, uint256 newY) = CurveBabyJubJub.pointAdd(
                bal.c1,
                bal.c2,
                commitmentDeltas[i].c1,
                commitmentDeltas[i].c2
            );
            bal.c1 = newX;
            bal.c2 = newY;

            unchecked {
                ++i;
            }
        }

        lastBlockNum = epochStart;
    }

    /**
     * @notice Update balances (used by withdraw/deposit)
     * @dev Fix H-15: this function had no propagation pass at all — every
     *      account not in participantIds simply kept whatever was in
     *      balanceCommitments[lastBlockNum][accountId], a slot that
     *      getBalance/getPublicValues/check() stop reading the moment
     *      lastBlockNum advances to epochStart below. So the first
     *      withdraw()/deposit() of a new epoch silently zeroed every
     *      non-participant's balance, with no debit, no event and no
     *      revert. Fixed the same way as _updateBalancesForTransfer (C-05):
     *      copy every registered account forward to the current epoch slot
     *      first (via _propagateBalancesExcept(0) — "except" no one, since
     *      account ids are 1-based and 0 is never one), then read and write
     *      that same slot for participants. This also closes the C-05-style
     *      duplicate-id gap on this path for free: participantIds must now
     *      be strictly increasing.
     */
    function _updateBalances(
        Point[] calldata commitmentDeltas,
        uint256[] calldata participantIds
    ) private {
        uint256 len = participantIds.length;
        if (len != commitmentDeltas.length) {
            revert ParticipantIdsLengthMismatch();
        }

        _propagateBalancesExcept(0);
        uint256 epochStart = _currentEpochStart();

        uint256 prevAccountId;
        for (uint256 i; i < len; ) {
            uint256 accountId = participantIds[i];
            if (accountId <= prevAccountId) {
                revert ParticipantIdsNotSorted();
            }
            prevAccountId = accountId;

            Point storage bal = balanceCommitments[epochStart][accountId];
            (uint256 newX, uint256 newY) = CurveBabyJubJub.pointAdd(
                bal.c1,
                bal.c2,
                commitmentDeltas[i].c1,
                commitmentDeltas[i].c2
            );
            bal.c1 = newX;
            bal.c2 = newY;

            unchecked {
                ++i;
            }
        }

        lastBlockNum = epochStart;
    }

    /**
     * @notice Execute deposits to ZkDvp
     */
    function _executeZkDvpDeposits(
        DepositParams[] calldata depositParams
    ) private returns (uint256[] memory) {
        IZkDvp zkDvp = IZkDvp(_zkDvpAddress);
        uint256 len = depositParams.length;
        uint256[] memory commitments = new uint256[](len);

        for (uint256 i; i < len; ) {
            uint256[] memory depositData = new uint256[](2);
            depositData[0] = depositParams[i].amount;
            depositData[1] = depositParams[i].publicKey;

            (bool success, uint256 commitment) = zkDvp.depositThroughEnygma(
                depositData
            );
            if (!success) revert ZkDvpOperationFailed();

            commitments[i] = commitment;
            emit Commitment(commitment);

            unchecked {
                ++i;
            }
        }

        return commitments;
    }

    /**
     * @notice Propagate balances to new block except one account
     */
    function _propagateBalancesExcept(uint256 excludeId) private {
        uint256 epochStart = _currentEpochStart();
        uint256 totalParties = _totalRegisteredParties;
        // AccountIds are 1-based: iterate 1.._totalRegisteredParties (not 0..n-1)
        for (uint256 i = 1; i <= totalParties; ) {
            _initializeBalanceIfNeeded(i);

            if (i != excludeId) {
                balanceCommitments[epochStart][i] = balanceCommitments[
                    lastBlockNum
                ][i];
            }

            unchecked {
                ++i;
            }
        }
    }

    /**
     * @notice Initialize balance to neutral element if unset
     */
    function _initializeBalanceIfNeeded(uint256 accountId) private {
        Point storage balance = balanceCommitments[lastBlockNum][accountId];
        if (balance.c1 == 0 && balance.c2 == 0) {
            balance.c2 = 1;
        }
    }


    /**
     * @notice Verify zero-knowledge fee proof (delegates to 54-signal fee verifier)
     */
    function _verifyFeeTransferProof(FeeProof calldata proof) private view {
        if (_feeVerifier == address(0)) revert VerifierNotFound();
        if (_feeVerifier.code.length == 0) revert VerifierHasNoCode(); // Fix M-01

        (bool success, ) = _feeVerifier.staticcall(
            abi.encodeWithSignature(
                "verifyProof(uint256[8],uint256[55])",
                proof
            )
        );
        if (!success) revert InvalidProof();
    }

    /**
     * @notice Verify fee public inputs match contract state (same offsets as base, 55-element array)
     */
    function _verifyFeePublicInputs(
        uint256[55] calldata public_signal,
        uint256[] calldata participantIds,
        Point[] calldata commitmentDeltas
    ) private view {
        // Fix L-01: see _verifyPublicInputs52's identical comment.
        if (public_signal[FEE_DOMAIN_OFFSET] != _expectedDomainId()) {
            revert InvalidDomain();
        }

        (Point[] memory balances, uint256[] memory keys) = getPublicValues(
            _totalRegisteredParties + 1
        );

        uint256 len = participantIds.length;
        for (uint256 i; i < len; ) {
            uint256 accountId = participantIds[i];

            if (keys[accountId] == 0) revert UnregisteredParticipant(); // Fix H-07

            if (uint256(public_signal[PUBLIC_KEY_OFFSET + i]) != keys[accountId]) {
                revert InvalidPublicInputs();
            }

            uint256 commitOffset = PREVIOUS_COMMIT_OFFSET + (i << 1);
            if (
                uint256(public_signal[commitOffset]) != balances[accountId].c1 ||
                uint256(public_signal[commitOffset + 1]) != balances[accountId].c2
            ) {
                revert InvalidPublicInputs();
            }

            uint256 txOffset = TX_COMMIT_OFFSET + (i << 1);
            if (
                commitmentDeltas[i].c1 != public_signal[txOffset] ||
                commitmentDeltas[i].c2 != public_signal[txOffset + 1]
            ) {
                revert InvalidPublicInputs();
            }

            unchecked {
                ++i;
            }
        }
    }

    /**
     * @notice Verify block number freshness for fee proof
     */
    function _verifyFeeBlockNumber(uint256[55] calldata public_signal) private view {
        if (uint256(public_signal[BLOCK_NUMBER_OFFSET]) != lastBlockNum) {
            revert InvalidBlockNumber();
        }
    }

    /**
     * @notice Record fee nullifier as spent
     */
    function _consumeFeeNullifier(uint256[55] calldata public_signal) private {
        uint256 nullifier = public_signal[NULLIFIER_OFFSET];
        if (_nullifiers[nullifier]) revert NullifierAlreadyUsed();
        _nullifiers[nullifier] = true;
    }

    // ============================================
    // CRYPTOGRAPHIC HELPERS
    // ============================================

    /**
     * @notice Derive Baby Jubjub point from value using generator G
     */
    function derivePk(
        uint256 value
    ) public view returns (uint256 x, uint256 y) {
        return CurveBabyJubJub.derivePk(value);
    }

    /**
     * @notice Derive Baby Jubjub point from randomness using generator H
     */
    function derivePkH(
        uint256 randomness
    ) public view returns (uint256 x, uint256 y) {
        return CurveBabyJubJub.derivePkH(randomness);
    }

    /**
     * @notice Add two Pedersen commitments
     */
    function addPedComm(
        uint256 p1x,
        uint256 p1y,
        uint256 p2x,
        uint256 p2y
    ) external view returns (uint256, uint256) {
        return CurveBabyJubJub.pointAdd(p1x, p1y, p2x, p2y);
    }

    /**
     * @notice Create Pedersen commitment: Com(v, r) = v*G + r*H
     */
    function pedCom(
        uint256 value,
        uint256 randomness
    ) public view returns (uint256, uint256) {
        (uint256 gX, uint256 gY) = derivePk(value);
        (uint256 hX, uint256 hY) = derivePkH(randomness);

        return CurveBabyJubJub.pointAdd(gX, gY, hX, hY);
    }
}
