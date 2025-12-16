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

    // Public signal array offsets for proof verification
    uint256 private constant ARRAY_HASH_SECRET_OFFSET = 0;
    uint256 private constant ARRAY_HASH_SECRET_SIZE = 36;
    uint256 private constant PUBLIC_KEY_OFFSET = 36;
    uint256 private constant PUBLIC_KEY_SIZE = 6;
    uint256 private constant PREVIOUS_COMMIT_OFFSET = 42;
    uint256 private constant PREVIOUS_COMMIT_SIZE = 12;
    uint256 private constant BLOCK_NUMBER_OFFSET = 54;
    uint256 private constant K_INDEX_OFFSET = 55;
    uint256 private constant K_INDEX_SIZE = 6;
    uint256 private constant NULLIFIER_OFFSET = 61;

    // ============================================
    // STATE VARIABLES
    // ============================================
    // Token metadata
    string private constant TOKEN_NAME = "Enygma";
    string private constant TOKEN_SYMBOL = "EN";
    uint8 private constant DECIMALS = 2;

    // Contract state
    uint256 private _status;
    address private immutable _owner;
    uint256 public lastBlockNum;
    uint256 private _totalRegisteredParties;

    // Total supply as Pedersen commitment point (x, y)
    uint256 public totalSupplyX;
    uint256 public totalSupplyY;
    uint256 public totalSupplyAmount;

    // Verifier addresses
    address private _transferVerifier;
    address private _withdrawVerifier;
    address private _depositVerifier;
    address private _zkDvpAddress;

    // ============================================
    // MAPPINGS
    // ============================================

    /// @notice Balance commitments per block per account
    mapping(uint256 => mapping(uint256 => Point)) public balanceCommitments;

    /// @notice Public keys for each account
    mapping(uint256 => uint256) public publicKeys;

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

    // ============================================
    // EVENTS
    // ============================================

    event Commitment(uint256 indexed commitment);
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

    // ============================================
    // CONSTRUCTOR
    // ============================================

    constructor() {
        _owner = msg.sender;
        _status = STATUS_NOT_INITIALIZED;
        lastBlockNum = block.number;
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
     * @param publicKey Institution public key
     * @param randomness Randomness for initial balance commitment
     */

    function registerAccount(
        address addr,
        uint256 accountId,
        uint256 publicKey,
        uint256 randomness
    ) external onlyOwner returns (bool) {
        publicKeys[accountId] = publicKey;
        addressToAccountId[addr] = accountId;

        // Create initial balance commitment: Com(0, randomness)
        (uint256 commitX, uint256 commitY) = pedCom(0, randomness);
        balanceCommitments[lastBlockNum][accountId] = Point(commitX, commitY);

        unchecked {
            ++_totalRegisteredParties;
        }

        emit AccountRegistered(addr, _totalRegisteredParties);
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
    function mintSupply(
        uint256 amount,
        uint256 recipientId
    ) external onlyOwner whenInitialized returns (bool) {
        // Convert amount to Baby Jubjub point
        (uint256 amountX, uint256 amountY) = derivePk(amount);

        // Update total supply commitment
        (totalSupplyX, totalSupplyY) = CurveBabyJubJub.pointAdd(
            totalSupplyX,
            totalSupplyY,
            amountX,
            amountY
        );

        unchecked {
            totalSupplyAmount += amount;
        }

        // Propagate balances to new block
        _propagateBalancesExcept(recipientId);

        // Update recipient's balance
        Point storage recipientBalance = balanceCommitments[lastBlockNum][
            recipientId
        ];
        (uint256 newX, uint256 newY) = CurveBabyJubJub.pointAdd(
            recipientBalance.c1,
            recipientBalance.c2,
            amountX,
            amountY
        );

        balanceCommitments[block.number][recipientId] = Point(newX, newY);
        lastBlockNum = block.number;

        emit SupplyMinted(lastBlockNum, amount, recipientId);
        return true;
    }

    /**
     * @notice Burn tokens from specific account
     * @param accountId Account to burn from
     * @param amount Amount to burn
     */
    function burn(
        uint256 accountId,
        uint256 amount
    ) external onlyOwner returns (bool) {
        if (amount > CurveBabyJubJub.P) revert BurnExceedsModulus();

        // Create commitment for negative value: Com(-amount, 0)
        (uint256 negCommitX, uint256 negCommitY) = pedCom(
            CurveBabyJubJub.P - amount,
            0
        );

        // Propagate balances for all other accounts
        _propagateBalancesExcept(accountId);

        // Update burned account's balance
        Point storage accountBalance = balanceCommitments[lastBlockNum][
            accountId
        ];
        (uint256 newX, uint256 newY) = CurveBabyJubJub.pointAdd(
            accountBalance.c1,
            accountBalance.c2,
            negCommitX,
            negCommitY
        );

        balanceCommitments[block.number][accountId] = Point(newX, newY);
        lastBlockNum = block.number;

        emit BurnSuccessful(accountId, amount);
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

        _transferVerifiers[DEFAULT_SIZE] = verifier;
        _transferVerifier = verifier;

        emit VerifierRegistered(verifier, _totalRegisteredParties);
        return true;
    }
    /**
     * @notice Register withdraw verifier contract
     * @param verifier Address of verifier contract
     * @param splitCount Number of splits this verifier handles
     */
    function addWithdrawVerifier(
        address verifier,
        uint256 splitCount
    ) external onlyOwner returns (bool) {
        if (verifier == address(0)) revert ZeroAddress();

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

        _depositVerifiers[DEFAULT_SIZE] = verifier;
        _depositVerifier = verifier;

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
     */
    function transfer(
        Point[] calldata commitmentDeltas,
        Proof calldata proof,
        uint256[] calldata participantIds
    ) external onlyRegistered whenInitialized returns (bool) {
        // Verify zero-knowledge proof
        _verifyTransferProof(proof, commitmentDeltas.length);

        // Verify public inputs match current state
        _verifyPublicInputs(proof, participantIds);

        //  Verify block number freshness
        _verifyBlockNumber(proof);

        // Update balances
        _updateBalancesForTransfer(commitmentDeltas, participantIds);

        emit TransactionSuccessful(msg.sender);
        return true;
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
    ) external onlyRegistered returns (bool, uint256[] memory) {
        // Verify withdrawal proof
        address verifier = _withdrawVerifiers[depositParams.length];
        if (verifier == address(0)) revert VerifierNotFound();

        (bool success, ) = verifier.delegatecall(
            abi.encodeWithSignature("verifyProof(uint256[8],uint256[1])", proof)
        );
        if (!success) revert InvalidProof();

        // Execute ZkDvp deposits
        uint256[] memory zkDvpCommitments = _executeZkDvpDeposits(
            depositParams
        );

        // Update Enygma balances
        _updateBalances(commitmentDeltas, participantIds);

        return (true, zkDvpCommitments);
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
    ) external onlyRegistered returns (bool) {
        // Verify deposit proof
        address verifier = _depositVerifiers[commitmentDeltas.length];
        if (verifier == address(0)) revert VerifierNotFound();

        (bool success, ) = verifier.delegatecall(
            abi.encodeWithSignature("verifyProof(uint256[8],uint256[2])", proof)
        );
        if (!success) revert InvalidProof();

        // Execute ZkDvp withdrawal
        IZkDvp zkDvp = IZkDvp(_zkDvpAddress);
        if (!zkDvp.withdrawThroughEnygma(withdrawParam.transaction)) {
            revert ZkDvpOperationFailed();
        }

        // Update Enygma balances
        _updateBalances(commitmentDeltas, participantIds);

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
        uint256 sumX;
        uint256 sumY = 1; // Start with neutral element

        for (uint256 i; i < _totalRegisteredParties; ) {
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
     * @notice Verify zero-knowledge proof for transfer
     */
    function _verifyTransferProof(
        Proof calldata proof,
        uint256 participantCount
    ) private {
        address verifier = _transferVerifiers[participantCount];
        if (verifier == address(0)) revert VerifierNotFound();

        (bool success, ) = verifier.delegatecall(
            abi.encodeWithSignature(
                "verifyProof(uint256[8],uint256[62])",
                proof
            )
        );
        if (!success) revert InvalidProof();
    }

    /**
     * @notice Verify public inputs match contract state
     */
    function _verifyPublicInputs(
        Proof calldata proof,
        uint256[] calldata participantIds
    ) private view {
        (Point[] memory balances, uint256[] memory keys) = getPublicValues(
            _totalRegisteredParties
        );

        uint256 len = participantIds.length;
        for (uint256 i; i < len; ) {
            uint256 accountId = participantIds[i];

            // Verify public key
            if (
                uint256(proof.public_signal[PUBLIC_KEY_OFFSET + i]) !=
                keys[accountId]
            ) {
                revert InvalidPublicInputs();
            }

            // Verify balance commitment
            uint256 commitOffset = PREVIOUS_COMMIT_OFFSET + (i << 1); // i * 2
            if (
                uint256(proof.public_signal[commitOffset]) !=
                balances[accountId].c1 ||
                uint256(proof.public_signal[commitOffset + 1]) !=
                balances[accountId].c2
            ) {
                revert InvalidPublicInputs();
            }

            unchecked {
                ++i;
            }
        }
    }

    /**
     * @notice Update balances for transfer participants
     */
    function _updateBalancesForTransfer(
        Point[] calldata commitmentDeltas,
        uint256[] calldata participantIds
    ) private {
        // Copy balances for non-participants
        uint256 totalParties = _totalRegisteredParties;
        for (uint256 i; i < totalParties; ) {
            _initializeBalanceIfNeeded(i);

            if (!_isParticipant(participantIds, i)) {
                balanceCommitments[block.number][i] = balanceCommitments[
                    lastBlockNum
                ][i];
            }

            unchecked {
                ++i;
            }
        }

        // Update participant balances
        uint256 len = commitmentDeltas.length;
        for (uint256 i; i < len; ) {
            uint256 accountId = participantIds[i];
            Point storage oldBalance = balanceCommitments[lastBlockNum][
                accountId
            ];

            (uint256 newX, uint256 newY) = CurveBabyJubJub.pointAdd(
                oldBalance.c1,
                oldBalance.c2,
                commitmentDeltas[i].c1,
                commitmentDeltas[i].c2
            );

            balanceCommitments[block.number][accountId] = Point(newX, newY);

            unchecked {
                ++i;
            }
        }

        lastBlockNum = block.number;
    }

    /**
     * @notice Update balances (used by withdraw/deposit)
     */
    function _updateBalances(
        Point[] calldata commitmentDeltas,
        uint256[] calldata participantIds
    ) private {
        uint256 len = commitmentDeltas.length;
        for (uint256 i; i < len; ) {
            uint256 accountId = participantIds[i];
            Point storage oldBalance = balanceCommitments[lastBlockNum][
                accountId
            ];

            (uint256 newX, uint256 newY) = CurveBabyJubJub.pointAdd(
                oldBalance.c1,
                oldBalance.c2,
                commitmentDeltas[i].c1,
                commitmentDeltas[i].c2
            );

            balanceCommitments[block.number][accountId] = Point(newX, newY);

            unchecked {
                ++i;
            }
        }

        lastBlockNum = block.number;
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
        uint256 totalParties = _totalRegisteredParties;
        for (uint256 i; i < totalParties; ) {
            _initializeBalanceIfNeeded(i);

            if (i != excludeId) {
                balanceCommitments[block.number][i] = balanceCommitments[
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
     * @notice Check if account is in participant list
     */
    function _isParticipant(
        uint256[] calldata participants,
        uint256 accountId
    ) private pure returns (bool) {
        uint256 len = participants.length;
        for (uint256 i; i < len; ) {
            if (participants[i] == accountId) return true;
            unchecked {
                ++i;
            }
        }
        return false;
    }

    /**
     * @notice Check if sent blocknumber is the same as in the smart contract
     */
    function _verifyBlockNumber(Proof calldata proof) private view {
        uint256 proofBlockNumber = uint256(
            proof.public_signal[BLOCK_NUMBER_OFFSET]
        );

        // Option 1: Exact match (strict)
        if (proofBlockNumber != lastBlockNum) {
            revert InvalidBlockNumber();
        }
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
