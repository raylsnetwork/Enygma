//SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;
import "./CurveBabyJubJub.sol";
import "../interfaces/IEnygma.sol";
import "../interfaces/IERC20.sol";
import "../interfaces/IZkDvp.sol";

contract Enygma is IEnygma {
    uint256 private constant STATUS_NOT_INITIALIZED = 0;
    uint256 private constant STATUS_INITIALIZED = 1;

    string private name = "Enygma";
    string private symbol = "EN";
    uint8 private decimals = 2;
    uint256 public totalSupplyX = 0;
    uint256 public totalSupplyY = 0;
    uint256 public _totalSupply;
    uint256 private totalRegisteredParties = 0;
    uint256 private k = 6;
    uint256 private _status = 0;
    address private _verifierAddress;
    address private _withdrawVerifierAddress;
    address private _depositVerifierAddress;
    address private _zkDvPAddress;
    address owner;
    uint256 public lastblockNum;

    mapping(uint256 => mapping(uint256 => Point)) public referenceBalance;
    mapping(uint256 => Point) public pubKeys;
    mapping(address => uint256) public accounts;
    mapping(uint256 => address) public verifiers;
    mapping(uint256 => address) public withdrawVerifiers;
    mapping(uint256 => address) public depositVerifiers;
    mapping(uint256 => address) public zkdvps;
    event Commitment(uint256 indexed commitment);
    // Initializes the Enygma contract with the designated Verifier contract address
    // for proof checking
    constructor() {
        lastblockNum = block.number;
        owner = msg.sender;
        _status = STATUS_NOT_INITIALIZED;
        _totalSupply = 0;
    }

    // Checks if executor is the contract owner
    modifier checkOwner() {
        require(msg.sender == owner, "Only admin can register users");
        _;
    }
    // Checks if transactor is registered
    modifier checkAccount() {
        require(
            accounts[msg.sender] != 0,
            "Only registered accounts can transact"
        );
        _;
    }

    function initialize() public checkOwner returns (bool) {
        _status = STATUS_INITIALIZED;

        totalSupplyX = 0;
        totalSupplyY = 1;

        return true;
    }

    // Registers an account, only the contract owner can register
    function registerAccount(
        address addr,
        uint256 accountNum,
        uint256 k1,
        uint256 k2,
        uint256 r
    ) public checkOwner returns (bool) {
        pubKeys[accountNum].c1 = k1;
        pubKeys[accountNum].c2 = k2;
        accounts[addr] = accountNum;

        (uint256 x, uint256 y) = pedCom(0, r);

        referenceBalance[lastblockNum][accountNum] = Point(x, y);

        totalRegisteredParties++;
        emit AccountRegistered(addr, totalRegisteredParties);
        return true;
    }

    // Mints supply to a designated account, only the contract owner can mint tokens to an account
    function mintSupply(
        uint256 amount,
        uint256 to
    ) public checkOwner returns (bool) {
        (uint256 amtX, uint256 amtY) = derivePk(amount);
        (uint256 xAdd, uint256 yAdd) = CurveBabyJubJub.pointAdd(
            totalSupplyX,
            totalSupplyY,
            amtX,
            amtY
        );
        totalSupplyX = xAdd;
        totalSupplyY = yAdd;
        _totalSupply = _totalSupply + amount;

        for (uint256 i = 0; i < totalRegisteredParties; i = i + 1) {
            if (
                referenceBalance[lastblockNum][i].c1 == 0 &&
                referenceBalance[lastblockNum][i].c2 == 0
            ) {
                referenceBalance[lastblockNum][i].c2 = 1;
            }
            if (i != to) {
                referenceBalance[block.number][i].c1 = referenceBalance[
                    lastblockNum
                ][i].c1;
                referenceBalance[block.number][i].c2 = referenceBalance[
                    lastblockNum
                ][i].c2;
            }
        }

        (uint256 xAddTo, uint256 yAddTo) = CurveBabyJubJub.pointAdd(
            referenceBalance[lastblockNum][to].c1,
            referenceBalance[lastblockNum][to].c2,
            amtX,
            amtY
        );

        referenceBalance[block.number][to].c1 = xAddTo;
        referenceBalance[block.number][to].c2 = yAddTo;

        lastblockNum = block.number;

        emit SupplyMinted(lastblockNum, amount, to);
        return true;
    }

    // Checks that all the balances add up to the total supply
    function check() public view returns (bool) {
        uint256 x;
        uint256 y;

        for (uint256 i = 0; i < totalRegisteredParties; i = i + 1) {
            (uint256 xBalance, uint256 yBalance) = getBalance(i);
            (x, y) = CurveBabyJubJub.pointAdd(x, y, xBalance, yBalance);
        }
        require(totalSupplyX == x && totalSupplyY == y, "Values dont match");
        return true;
    }

    // Add address of ZK Verifier in the contract
    function addVerifier(address verifier) public returns (bool) {
        verifiers[k] = verifier;
        _verifierAddress = verifier;
        emit VerifierRegistered(verifier, totalRegisteredParties);
        return true;
    }

    // Integration ZkDvP - adding withdraw verifier
    function addWithdrawVerifier(
        address withdrawVerifier,
        uint256 splitNumber
    ) public returns (bool) {
        withdrawVerifiers[splitNumber] = withdrawVerifier;
        _withdrawVerifierAddress = withdrawVerifier;
        emit VerifierRegistered(withdrawVerifier, totalRegisteredParties);
        return true;
    }

    // Integration ZkDvP - adding deposit verifier
    function addDepositVerifier(address depositVerifier) public returns (bool) {
        depositVerifiers[k] = depositVerifier;
        _depositVerifierAddress = depositVerifier;
        emit VerifierRegistered(depositVerifier, totalRegisteredParties);
        return true;
    }

    // New Verifier for withdraw
    function addZkDvp(address zkdvpAddress) public returns (bool) {
        zkdvps[k] = zkdvpAddress;
        _zkDvPAddress = zkdvpAddress;

        emit VerifierRegistered(zkdvpAddress, totalRegisteredParties);
        return true;
    }

    // Returns the balance of an account, ALWAYS USE THIS TO GET BALANCES
    function getBalance(
        uint256 account
    ) public view returns (uint256 x, uint256 y) {
        if (
            referenceBalance[lastblockNum][account].c1 == 0 &&
            referenceBalance[lastblockNum][account].c2 == 0
        ) {
            return (0, 1);
        } else {
            return (
                referenceBalance[lastblockNum][account].c1,
                referenceBalance[lastblockNum][account].c2
            );
        }
    }

    // Returns the balance of an account
    function getPublicValues(
        uint256 size
    ) public view returns (Point[] memory, Point[] memory) {
        Point[] memory refBalances = new Point[](size);
        for (uint256 i = 0; i < size; i++) {
            (uint256 xBalance, uint256 yBalance) = getBalance(i);

            refBalances[i] = Point(xBalance, yBalance);
        }
        Point[] memory publicKeys = new Point[](size);
        for (uint256 i = 0; i < size; i++) {
            publicKeys[i] = pubKeys[i];
        }
        return (refBalances, publicKeys);
    }

    // Tranfers funds from one account to another by adding the pederson commitments in the commitments array
    // and cheking the proof and input of the account
    function transfer(
        Point[] memory commitments,
        Proof memory proof,
        uint256[] memory kIndex
    ) public checkAccount returns (bool) {
        string memory funcSig = "verifyProof(uint256[8],uint256[32])";

        address verifAddr = verifiers[commitments.length];
        (bool success, ) = verifAddr.delegatecall(
            abi.encodeWithSignature(funcSig, proof)
        );
        require(success, "Invalid Proof/Input");
        (Point[] memory balance, Point[] memory pk) = getPublicValues(
            totalRegisteredParties
        );

        bool checkPkAndBalance = true;
        for (uint256 j = 0; j < kIndex.length; j++) {
            if (
                uint256(proof.public_signal[j * 2]) != pk[kIndex[j]].c1 ||
                uint256(proof.public_signal[j * 2 + 1]) != pk[kIndex[j]].c2 ||
                uint256(proof.public_signal[j * 2 + 2 * k]) !=
                balance[kIndex[j]].c1 ||
                uint256(proof.public_signal[j * 2 + 2 * k + 1]) !=
                balance[kIndex[j]].c2
            ) {
                checkPkAndBalance = false;
            }
        }
        require(checkPkAndBalance, "Invalid Pk or Previous Commit");

        bool checkBlockNumber = true;
        // if (uint256(proof.public_signal[4 * k]) != lastblockNum) {
        //     checkBlockNumber = false;
        // }
        require(checkBlockNumber, "Invalid Block Number");

        //update balance per block for participants not in the tx
        for (uint256 i = 0; i < totalRegisteredParties; i = i + 1) {
            // Check if both c1 and c2 are 0, and update c2 to 1 if needed
            if (
                referenceBalance[lastblockNum][i].c1 == 0 &&
                referenceBalance[lastblockNum][i].c2 == 0
            ) {
                referenceBalance[lastblockNum][i].c2 = 1;
            }

            // If i is not in the k array, update referenceBalance for the current block number
            if (!isInK(kIndex, i)) {
                referenceBalance[block.number][i].c1 = referenceBalance[
                    lastblockNum
                ][i].c1;
                referenceBalance[block.number][i].c2 = referenceBalance[
                    lastblockNum
                ][i].c2;
            }
        }

        //update balance per block for participants IN the tx
        for (uint256 i = 0; i < commitments.length; i++) {
            (
                referenceBalance[block.number][kIndex[i]].c1,
                referenceBalance[block.number][kIndex[i]].c2
            ) = CurveBabyJubJub.pointAdd(
                referenceBalance[lastblockNum][kIndex[i]].c1,
                referenceBalance[lastblockNum][kIndex[i]].c2,
                commitments[i].c1,
                commitments[i].c2
            );
        }

        lastblockNum = block.number;

        emit TransactionSuccessful(msg.sender);
        return true;
    }
    // withdraw from Enygma and deposit to ZkDvp
    function withdraw(
        Point[] memory commitments,
        WithdrawProof memory proof,
        DepositParams[] memory deposistParam,
        uint256[] memory kIndex
    ) public returns (bool, uint256[] memory) {
        string memory funcSig = "verifyProof(uint256[8],uint256[1])";

        address withdrawVerifAddr = withdrawVerifiers[deposistParam.length];
        (bool success, ) = withdrawVerifAddr.delegatecall(
            abi.encodeWithSignature(funcSig, proof)
        );
        require(success, "Invalid Proof/Input");

        address depositDvPAddr = _zkDvPAddress;
        IZkDvp zkDvpVault = IZkDvp(depositDvPAddr);

        uint256[] memory commitmentDepositArray = new uint256[](
            deposistParam.length
        );
        for (uint256 i = 0; i < deposistParam.length; i++) {
            uint256[] memory depositVar = new uint256[](2);

            depositVar[0] = deposistParam[i].amount;

            depositVar[1] = deposistParam[i].publicKey;

            (bool sucessZkDvpCall, uint256 commitmentDeposit) = zkDvpVault
                .depositThroughEnygma(depositVar);

            require(sucessZkDvpCall, "Deposit failed");

            commitmentDepositArray[i] = commitmentDeposit;
            emit Commitment(commitmentDeposit);
        }
        updateBalances(commitments, kIndex);
        return (true, commitmentDepositArray);
    }

    // withdraw from ZkDvp and deposit to Enygma
    function deposit(
        Point[] memory commitments,
        DepositProof memory proof,
        WithdrawParams memory withdrawParam,
        uint256[] memory kIndex
    ) public returns (bool) {
        // Verfier proof to deposit
        string memory funcSig = "verifyProof(uint256[8],uint256[2])";

        address depositVerifAddr = depositVerifiers[commitments.length];
        (bool success, ) = depositVerifAddr.delegatecall(
            abi.encodeWithSignature(funcSig, proof)
        );
        require(success, "Invalid Proof/Input");

        //call zkDvPwithdraFunds
        address zkdvpAddr = _zkDvPAddress;
        IZkDvp zkdvp = IZkDvp(zkdvpAddr);

        bool sucessZkDvpCall = zkdvp.withdrawThroughEnygma(
            withdrawParam.transaction
        );

        require(sucessZkDvpCall, "Withdraw in ZkDvP failed");

        updateBalances(commitments, kIndex);

        return true;
    }

    // Update balance which is in pedersen commitment format
    function updateBalances(
        Point[] memory commitments,
        uint256[] memory kIndex
    ) internal {
        for (uint256 i = 0; i < commitments.length; i++) {
            (
                referenceBalance[block.number][kIndex[i]].c1,
                referenceBalance[block.number][kIndex[i]].c2
            ) = CurveBabyJubJub.pointAdd(
                referenceBalance[lastblockNum][kIndex[i]].c1,
                referenceBalance[lastblockNum][kIndex[i]].c2,
                commitments[i].c1,
                commitments[i].c2
            );
        }
        lastblockNum = block.number;
    }

    // Helper function to check if an index is in the array `k`
    function isInK(
        uint256[] memory _k,
        uint256 i
    ) internal pure returns (bool) {
        for (uint256 j = 0; j < _k.length; j++) {
            if (_k[j] == i) {
                return true; // i is in the k array
            }
        }
        return false; // i is not in the k array
    }

    // burn tokens
    function burn(
        uint256 bankIndex,
        uint256 burnValue
    ) public checkOwner returns (bool) {
        require(burnValue <= CurveBabyJubJub.P, "Error: burnValue > Q");
        (uint256 commX, uint256 commY) = pedCom(
            CurveBabyJubJub.P - burnValue,
            0
        );

        for (uint256 i = 0; i < totalRegisteredParties; i = i + 1) {
            if (
                referenceBalance[lastblockNum][i].c1 == 0 &&
                referenceBalance[lastblockNum][i].c2 == 0
            ) {
                referenceBalance[lastblockNum][i].c2 = 1;
            }
            if (i != bankIndex) {
                referenceBalance[block.number][i].c1 = referenceBalance[
                    lastblockNum
                ][i].c1;
                referenceBalance[block.number][i].c2 = referenceBalance[
                    lastblockNum
                ][i].c2;
            }
        }

        (
            referenceBalance[block.number][bankIndex].c1,
            referenceBalance[block.number][bankIndex].c2
        ) = CurveBabyJubJub.pointAdd(
            referenceBalance[lastblockNum][bankIndex].c1,
            referenceBalance[lastblockNum][bankIndex].c2,
            commX,
            commY
        );

        lastblockNum = block.number;

        emit BurnSuccessful(bankIndex, burnValue);

        return true;
    }

    // Derives a set of points in the BabyJubJub curve from an input with a Generator
    function derivePk(uint256 v) public view returns (uint256 x2, uint256 y2) {
        (x2, y2) = CurveBabyJubJub.derivePk(v);
    }

    // Derives a set of points in the BabyJubJub curve from an input with an H value
    function derivePkH(uint256 r) public view returns (uint256 x2, uint256 y2) {
        (x2, y2) = CurveBabyJubJub.derivePkH(r);
    }

    // BabyJubJub Curve Check functions
    // Adds two pederson commitments and prints the result
    function addPedComm(
        uint256 p1,
        uint256 p2,
        uint256 x2,
        uint256 y2
    ) public view returns (uint256, uint256) {
        (uint256 pedComX, uint256 pedComY) = CurveBabyJubJub.pointAdd(
            p1,
            p2,
            x2,
            y2
        );
        return (pedComX, pedComY);
    }

    /**
     * @dev Helper Function
     */

    // Pedersen Commitment function by inserting v and r its output a Pedersen Commitment
    function pedCom(
        uint256 v,
        uint256 r
    ) public view returns (uint256, uint256) {
        (uint256 gX, uint256 gY) = derivePk(v);
        (uint256 hX, uint256 hY) = derivePkH(r);
        (uint256 pedComX, uint256 pedComY) = CurveBabyJubJub.pointAdd(
            gX,
            gY,
            hX,
            hY
        );
        return (pedComX, pedComY);
    }

    function Name() public view returns (string memory) {
        return name;
    }

    function Symbol() public view returns (string memory) {
        return symbol;
    }

    function TotalRegisteredBanks() public view returns (uint256) {
        return totalRegisteredParties;
    }

    function TotalSupply() public view returns (uint256) {
        return _totalSupply;
    }

    function VerifierAddress() public view returns (address) {
        return _verifierAddress;
    }

    //ZkDvp integration
    function WithdrawVerifierAddress() public view returns (address) {
        return _withdrawVerifierAddress;
    }

    //ZkDvp integration
    function DepositVerifierAddress() public view returns (address) {
        return _depositVerifierAddress;
    }

    function ZkdvpAddress() public view returns (address) {
        return _zkDvPAddress;
    }

    function _getRevertMsg(
        bytes memory revertData
    ) internal pure returns (string memory) {
        if (revertData.length < 68) return "Transaction reverted silently";
        assembly {
            revertData := add(revertData, 0x04)
        }
        return abi.decode(revertData, (string));
    }
}
