// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package Enygma

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// IEnygmaBurnProof is an auto generated low-level Go binding around an user-defined struct.
type IEnygmaBurnProof struct {
	Proof        [8]*big.Int
	PublicSignal [9]*big.Int
}

// IEnygmaDepositParams is an auto generated low-level Go binding around an user-defined struct.
type IEnygmaDepositParams struct {
	Amount      *big.Int
	Erc20Adress common.Address
	PublicKey   *big.Int
}

// IEnygmaDepositProof is an auto generated low-level Go binding around an user-defined struct.
type IEnygmaDepositProof struct {
	Proof        [8]*big.Int
	PublicSignal [52]*big.Int
}

// IEnygmaFeeProof is an auto generated low-level Go binding around an user-defined struct.
type IEnygmaFeeProof struct {
	Proof        [8]*big.Int
	PublicSignal [55]*big.Int
}

// IEnygmaPoint is an auto generated low-level Go binding around an user-defined struct.
type IEnygmaPoint struct {
	C1 *big.Int
	C2 *big.Int
}

// IEnygmaProof is an auto generated low-level Go binding around an user-defined struct.
type IEnygmaProof struct {
	Proof        [8]*big.Int
	PublicSignal [81]*big.Int
}

// IEnygmaUsdrProof is an auto generated low-level Go binding around an user-defined struct.
type IEnygmaUsdrProof struct {
	Proof        [8]*big.Int
	PublicSignal [81]*big.Int
}

// IEnygmaWithdrawParams is an auto generated low-level Go binding around an user-defined struct.
type IEnygmaWithdrawParams struct {
	Transaction IZkDvpJoinSplitTransaction
}

// IEnygmaWithdrawProof is an auto generated low-level Go binding around an user-defined struct.
type IEnygmaWithdrawProof struct {
	Proof        [8]*big.Int
	PublicSignal [52]*big.Int
}

// IZkDvpG1Point is an auto generated low-level Go binding around an user-defined struct.
type IZkDvpG1Point struct {
	X *big.Int
	Y *big.Int
}

// IZkDvpG2Point is an auto generated low-level Go binding around an user-defined struct.
type IZkDvpG2Point struct {
	X [2]*big.Int
	Y [2]*big.Int
}

// IZkDvpJoinSplitTransaction is an auto generated low-level Go binding around an user-defined struct.
type IZkDvpJoinSplitTransaction struct {
	Proof           IZkDvpSnarkProof
	Statement       []*big.Int
	NumberOfInputs  *big.Int
	NumberOfOutputs *big.Int
}

// IZkDvpSnarkProof is an auto generated low-level Go binding around an user-defined struct.
type IZkDvpSnarkProof struct {
	A IZkDvpG1Point
	B IZkDvpG2Point
	C IZkDvpG1Point
}

// EnygmaMetaData contains all meta data concerning the Enygma contract.
var EnygmaMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_epochInterval\",\"type\":\"uint256\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[],\"name\":\"AlreadyInitialized\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"AlreadyRegistered\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"BalanceMismatch\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"BurnExceedsModulus\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ContractIsPaused\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"DepositValueMismatch\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"FeeExceedsModulus\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"FingerprintNotConfirmed\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidAccountId\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidBlockNumber\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidCommitmentPoint\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidDomain\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidFee\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidFeeAmount\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidFingerprintParty\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidParticipantCount\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidProof\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidPublicInputs\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidPublicKey\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidViewKeyLength\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NewOwnerIsZeroAddress\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NotInitialized\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NotOwner\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NotPendingOwner\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NotRegistered\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NullifierAlreadyUsed\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ParticipantIdsLengthMismatch\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ParticipantIdsNotSorted\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ReentrancyGuardReentrantCall\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"UnregisteredParticipant\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"UsdrBindingMismatch\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"VerifierHasNoCode\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"VerifierNotFound\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ZeroAddress\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ZkDvpOperationFailed\",\"type\":\"error\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"addedBank\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"accountId\",\"type\":\"uint256\"}],\"name\":\"AccountRegistered\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"bankIndex\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"burnValue\",\"type\":\"uint256\"}],\"name\":\"BurnSuccessful\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"commitment\",\"type\":\"uint256\"}],\"name\":\"Commitment\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"fee\",\"type\":\"uint256\"}],\"name\":\"FeeBurned\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"partyA\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"partyB\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"fingerprint\",\"type\":\"uint256\"}],\"name\":\"FingerprintConfirmed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"fromId\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"toId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"fingerprint\",\"type\":\"uint256\"}],\"name\":\"FingerprintPending\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferStarted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"Paused\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"previousFee\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"newFee\",\"type\":\"uint256\"}],\"name\":\"ProtocolFeeUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"submitter\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"bankTag\",\"type\":\"string\"}],\"name\":\"RelayAttribution\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"lastblockNum\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"to\",\"type\":\"uint256\"}],\"name\":\"SupplyMinted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"maxBankCount\",\"type\":\"uint256\"}],\"name\":\"TokenInitialized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"senderAddress\",\"type\":\"address\"}],\"name\":\"TransactionSuccessful\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"Unpaused\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"verifierAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"totalRegisteredVerifiers\",\"type\":\"uint256\"}],\"name\":\"VerifierRegistered\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"DepositVerifierAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"GetBlckHash\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"Name\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"Symbol\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"TotalRegisteredBanks\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"TotalSupply\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"VerifierAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"WithdrawVerifierAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"ZkdvpAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"acceptOwnership\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"}],\"name\":\"addBurnVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"}],\"name\":\"addDepositVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"}],\"name\":\"addFeeVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"p1x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"p1y\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"p2x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"p2y\",\"type\":\"uint256\"}],\"name\":\"addPedComm\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"}],\"name\":\"addUsdrVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"}],\"name\":\"addVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"splitCount\",\"type\":\"uint256\"}],\"name\":\"addWithdrawVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"zkDvp\",\"type\":\"address\"}],\"name\":\"addZkDvp\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"addressToAccountId\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"balanceCommitments\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"accountId\",\"type\":\"uint256\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[9]\",\"name\":\"public_signal\",\"type\":\"uint256[9]\"}],\"internalType\":\"structIEnygma.BurnProof\",\"name\":\"proof\",\"type\":\"tuple\"}],\"name\":\"burn\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"check\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"checkUsdr\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"confirmedFingerprint\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"commitmentDeltas\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[52]\",\"name\":\"public_signal\",\"type\":\"uint256[52]\"}],\"internalType\":\"structIEnygma.DepositProof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"components\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"internalType\":\"structIZkDvp.G1Point\",\"name\":\"a\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"uint256[2]\",\"name\":\"x\",\"type\":\"uint256[2]\"},{\"internalType\":\"uint256[2]\",\"name\":\"y\",\"type\":\"uint256[2]\"}],\"internalType\":\"structIZkDvp.G2Point\",\"name\":\"b\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"internalType\":\"structIZkDvp.G1Point\",\"name\":\"c\",\"type\":\"tuple\"}],\"internalType\":\"structIZkDvp.SnarkProof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"internalType\":\"uint256[]\",\"name\":\"statement\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256\",\"name\":\"numberOfInputs\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"numberOfOutputs\",\"type\":\"uint256\"}],\"internalType\":\"structIZkDvp.JoinSplitTransaction\",\"name\":\"transaction\",\"type\":\"tuple\"}],\"internalType\":\"structIEnygma.WithdrawParams\",\"name\":\"withdrawParam\",\"type\":\"tuple\"},{\"internalType\":\"uint256[]\",\"name\":\"participantIds\",\"type\":\"uint256[]\"}],\"name\":\"deposit\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"}],\"name\":\"derivePk\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"randomness\",\"type\":\"uint256\"}],\"name\":\"derivePkH\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"epochInterval\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"fingerprintConfirmed\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"accountId\",\"type\":\"uint256\"}],\"name\":\"getBalance\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"count\",\"type\":\"uint256\"}],\"name\":\"getPublicValues\",\"outputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"balances\",\"type\":\"tuple[]\"},{\"internalType\":\"uint256[]\",\"name\":\"keys\",\"type\":\"uint256[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"accountId\",\"type\":\"uint256\"}],\"name\":\"getUsdrBalance\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"count\",\"type\":\"uint256\"}],\"name\":\"getUsdrPublicValues\",\"outputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"balances\",\"type\":\"tuple[]\"},{\"internalType\":\"uint256[]\",\"name\":\"keys\",\"type\":\"uint256[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"initialize\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"accountId\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"randomness\",\"type\":\"uint256\"}],\"name\":\"initializeUsdrBalance\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"lastBlockNum\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"recipientId\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"mintCommitX\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"mintCommitY\",\"type\":\"uint256\"}],\"name\":\"mintSupply\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"recipientId\",\"type\":\"uint256\"}],\"name\":\"mintUsdrSupply\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"pause\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"paused\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"randomness\",\"type\":\"uint256\"}],\"name\":\"pedCom\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"pendingFingerprint\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"pendingOwner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"protocolFee\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"publicKeys\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"addr\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"accountId\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"publicKey\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"initialCommitX\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"initialCommitY\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"viewKey\",\"type\":\"bytes\"}],\"name\":\"registerAccount\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"otherPartyId\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"fingerprint\",\"type\":\"uint256\"}],\"name\":\"registerFingerprint\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"newFee\",\"type\":\"uint256\"}],\"name\":\"setProtocolFee\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"setUsdrFixedFee\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalSupplyAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalSupplyX\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalSupplyY\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"commitmentDeltas\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[81]\",\"name\":\"public_signal\",\"type\":\"uint256[81]\"}],\"internalType\":\"structIEnygma.Proof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"usdrCommitmentDeltas\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[81]\",\"name\":\"public_signal\",\"type\":\"uint256[81]\"}],\"internalType\":\"structIEnygma.UsdrProof\",\"name\":\"usdrProof\",\"type\":\"tuple\"},{\"internalType\":\"uint256[]\",\"name\":\"participantIds\",\"type\":\"uint256[]\"},{\"internalType\":\"string\",\"name\":\"bankTag\",\"type\":\"string\"}],\"name\":\"transfer\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"commitmentDeltas\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[55]\",\"name\":\"public_signal\",\"type\":\"uint256[55]\"}],\"internalType\":\"structIEnygma.FeeProof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"internalType\":\"uint256[]\",\"name\":\"participantIds\",\"type\":\"uint256[]\"},{\"internalType\":\"string\",\"name\":\"bankTag\",\"type\":\"string\"}],\"name\":\"transferWithFee\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"unpause\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"usdrBalanceCommitments\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"usdrFixedFeeAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"usdrTotalSupplyAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"usdrTotalSupplyX\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"usdrTotalSupplyY\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"viewKeys\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"commitmentDeltas\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[52]\",\"name\":\"public_signal\",\"type\":\"uint256[52]\"}],\"internalType\":\"structIEnygma.WithdrawProof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"erc20Adress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"publicKey\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.DepositParams[]\",\"name\":\"depositParams\",\"type\":\"tuple[]\"},{\"internalType\":\"uint256[]\",\"name\":\"participantIds\",\"type\":\"uint256[]\"}],\"name\":\"withdraw\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"},{\"internalType\":\"uint256[]\",\"name\":\"\",\"type\":\"uint256[]\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
	Bin: "0x60a060405234801561001057600080fd5b506040516152e03803806152e083398101604081905261002f916100cd565b600081116100835760405162461bcd60e51b815260206004820152601960248201527f65706f6368496e74657276616c206d757374206265203e203000000000000000604482015260640160405180910390fd5b600180546001600160a01b031916331790556000805560808190526100a7816100b0565b6003555061012d565b6000816100bd81436100e6565b6100c79190610108565b92915050565b6000602082840312156100df57600080fd5b5051919050565b60008261010357634e487b7160e01b600052601260045260246000fd5b500490565b80820281158282048414176100c757634e487b7160e01b600052601160045260246000fd5b60805161519161014f6000396000818161032c01526133ee01526151916000f3fe608060405234801561001057600080fd5b50600436106102e05760003560e01c80630197d942146102e557806307da47ea1461030d57806309b1ef26146103275780630cf1839c1461035c578063132ce4d41461037c5780631660e58f1461039d5780631a4e1aa1146103b05780631e010439146103c157806325e74eb3146103d4578063272fe53c146103dd5780632c0457e8146103e65780633045aaf3146103f757806336899042146104155780633f4ba83a1461041e5780634e466c53146104265780635233389414610439578063545897c0146104415780635a54f35f146104545780635c975abb146104675780635dcc7650146104795780635f5377e01461048c5780635fbaf8411461049f57806367511a4d146104b257806367ec4693146104bb5780636da4d2b8146104dc5780636f5a2d541461050857806371929e2a14610529578063723dbbc414610532578063743873b414610545578063787dce3d1461054d578063795825a71461056057806379ba5097146105735780637d894a161461057b5780638052474d1461058e5780638129fc1c146105b057806383914157146105b85780638456cb59146105cb57806384aaa2de146105d35780638718dcaa146105db578063874ed5b5146105ee5780638d909dd7146105ff5780638da5cb5b146106085780638f48f7b5146106195780639000b3d614610644578063919840ad146106575780639edd41ff1461065f578063a44b47f714610672578063a605841c1461067a578063a61f3ae4146106a5578063a9c58a7e146106b8578063b0e21e8a146106cb578063c1ab48fc146106d4578063c680f410146106f4578063ce630c1814610714578063ce8cd40014610727578063e30c397814610759578063e52dc1881461076a578063ea0d457314610773578063ec4a09d0146107a5578063edda4a0a146107b8578063f2fde38b146107cb578063f828f50b146107de578063f8344434146107e7578063fe877fc9146107fa575b600080fd5b6102f86102f3366004614579565b61080d565b60405190151581526020015b60405180910390f35b600f546001600160a01b03165b6040516103049190614594565b61034e7f000000000000000000000000000000000000000000000000000000000000000081565b604051908152602001610304565b61036f61036a3660046145a8565b610914565b6040516103049190614611565b61038f61038a366004614624565b6109ae565b604051610304929190614656565b6102f86103ab3660046145a8565b6109cb565b6010546001600160a01b031661031a565b61038f6103cf3660046145a8565b610a02565b61034e600c5481565b61034e600a5481565b600e546001600160a01b031661031a565b60408051808201909152600281526122a760f11b602082015261036f565b61034e60035481565b6102f8610a56565b6102f8610434366004614579565b610acf565b6102f8610b8e565b6102f861044f366004614746565b610c04565b6102f8610462366004614846565b610d93565b600254600160a01b900460ff166102f8565b6102f861048736600461487b565b610f48565b6102f861049a366004614846565b61120e565b6102f86104ad366004614933565b6112aa565b61034e60065481565b6104ce6104c93660046145a8565b6116de565b6040516103049291906149a9565b6102f86104ea366004614846565b60208080526000928352604080842090915290825290205460ff1681565b61051b610516366004614a0d565b611810565b604051610304929190614afa565b61034e60055481565b61038f6105403660046145a8565b611aa2565b60035461034e565b6102f861055b3660046145a8565b611ab7565b6102f861056e366004614b1d565b611b57565b6102f8611cbc565b61038f610589366004614846565b611d4a565b604080518082019091526006815265456e79676d6160d01b602082015261036f565b6102f8611d89565b6102f86105c6366004614bce565b611df6565b6102f8612018565b60045461034e565b6102f86105e9366004614624565b612089565b600d546001600160a01b031661031a565b61034e600b5481565b6001546001600160a01b031661031a565b61034e610627366004614846565b601f60209081526000928352604080842090915290825290205481565b6102f8610652366004614579565b61221a565b6102f8612310565b61038f61066d3660046145a8565b61231f565b60075461034e565b61034e610688366004614846565b601e60209081526000928352604080842090915290825290205481565b6102f86106b3366004614846565b61235e565b6104ce6106c63660046145a8565b6124a5565b61034e60085481565b61034e6106e2366004614579565b60176020526000908152604090205481565b61034e6107023660046145a8565b60156020526000908152604090205481565b61038f6107223660046145a8565b6125d1565b61038f610735366004614846565b60146020908152600092835260408084209091529082529020805460019091015482565b6002546001600160a01b031661031a565b61034e60095481565b61038f610781366004614846565b60136020908152600092835260408084209091529082529020805460019091015482565b6102f86107b3366004614579565b6125dd565b6102f86107c6366004614579565b6126c8565b6102f86107d9366004614579565b612787565b61034e60075481565b6102f86107f5366004614579565b612833565b6102f8610808366004614c29565b6128fe565b6001546000906001600160a01b0316331461083b576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b0382166108625760405163d92e233d60e01b815260040160405180910390fd5b816001600160a01b03163b60000361088d576040516362d4176d60e11b815260040160405180910390fd5b6006600052601b6020527ff5ddd0b8f160eab91dc4f82b50a485a96cf6ab0bfb38460d73171763afb6d5cf80546001600160a01b0384166001600160a01b03199182168117909255600f8054909116821790556004546040516000805160206150bc833981519152916109039190815260200190565b60405180910390a25060015b919050565b6016602052600090815260409020805461092d90614c53565b80601f016020809104026020016040519081016040528092919081815260200182805461095990614c53565b80156109a65780601f1061097b576101008083540402835291602001916109a6565b820191906000526020600020905b81548152906001019060200180831161098957829003601f168201915b505050505081565b6000806109bd868686866129ea565b915091505b94509492505050565b6001546000906001600160a01b031633146109f9576040516330cd747160e01b815260040160405180910390fd5b50600c55600190565b600354600090815260136020908152604080832084845290915281208054829190158015610a3257506001810154155b15610a44575060009360019350915050565b80546001909101549094909350915050565b6001546000906001600160a01b03163314610a84576040516330cd747160e01b815260040160405180910390fd5b6002805460ff60a01b191690556040517f5db9ee0a495bf2e6ff9c91a7834c1ba4fdd244a5e8aa4e537bd38aeae4b073aa90610ac1903390614594565b60405180910390a150600190565b6001546000906001600160a01b03163314610afd576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b038216610b245760405163d92e233d60e01b815260040160405180910390fd5b816001600160a01b03163b600003610b4f576040516362d4176d60e11b815260040160405180910390fd5b601180546001600160a01b0319166001600160a01b0384169081179091556004546040519081526000805160206150bc83398151915290602001610903565b6000806001805b6004548111610bc957600080610baa8361231f565b91509150610bba858584846129ea565b90955093505050600101610b95565b5081600954141580610bdd575080600a5414155b15610bfb57604051631947c14d60e31b815260040160405180910390fd5b60019250505090565b336000908152601760205260408120548103610c335760405163aba4733960e01b815260040160405180910390fd5b600160005414610c56576040516321c4e35760e21b815260040160405180910390fd5b600254600160a01b900460ff1615610c81576040516306d39fcd60e41b815260040160405180910390fd5b610c8b898b612b3b565b610c958688612c56565b610ca789610100018761010001612c8c565b610cb8896101000186868e8e612d8f565b610cc9866101000186868b8b613008565b610cd889610100018686613238565b610ce5896101000161335d565b610cf2866101000161335d565b610cff896101000161338a565b610d0c866101000161338a565b6000610d166133ea565b9050610d25818d8d8989613421565b610d32818a8a8989613520565b6003819055604051339060008051602061509c83398151915290600090a2336001600160a01b03166000805160206150dc8339815191528585604051610d79929190614c87565b60405180910390a25060019b9a5050505050505050505050565b336000908152601760205260408120548103610dc25760405163aba4733960e01b815260040160405180910390fd5b600254600160a01b900460ff1615610ded576040516306d39fcd60e41b815260040160405180910390fd5b33600090815260176020526040902054831580610e0957508084145b15610e275760405163a5c3e7e160e01b815260040160405180910390fd5b6000818152601e602090815260408083208784528252918290208590559051848152859183917f2244c7409c0d13a2d9db63e7bfad1826bd85a2d6052ad998f150d2aab30c0d1f910160405180910390a36000848152601e602090815260408083208484529091529020548015801590610ea057508381145b15610f3b576000828152601f602081815260408084208985528252808420889055918152818320858452815281832087905580805281832088845281528183208054600160ff19918216811790925582805283852087865283529383902080549094161790925551858152869184917fe2070e33257d0615b4d0bff5bc34dcc22292a2ca3973074cc97d92dcc884549d910160405180910390a35b6001925050505b92915050565b336000908152601760205260408120548103610f775760405163aba4733960e01b815260040160405180910390fd5b600160005414610f9a576040516321c4e35760e21b815260040160405180910390fd5b600254600160a01b900460ff1615610fc5576040516306d39fcd60e41b815260040160405180910390fd5b600254600160a81b900460ff1615610ff057604051633ee5aeb560e01b815260040160405180910390fd5b6002805460ff60a81b1916600160a81b1790556000868152601b60205260409020546001600160a01b03168061103957604051633896c50b60e21b815260040160405180910390fd5b806001600160a01b03163b600003611064576040516362d4176d60e11b815260040160405180910390fd5b6000816001600160a01b0316876040516024016110819190614cce565b60408051601f198184030181529181526020820180516001600160e01b0316630f948af160e01b179052516110b69190614cdd565b600060405180830381855afa9150503d80600081146110f1576040519150601f19603f3d011682016040523d82523d6000602084013e6110f6565b606091505b5050905080611118576040516309bde33960e01b815260040160405180910390fd5b611129876101000186868c8c613662565b61113687610100016138d0565b61114387610100016138db565b61114f898987876138e5565b61115989896139ed565b6010546001600160a01b031680634ac058ed6111758980614cf9565b6040518263ffffffff1660e01b81526004016111919190614d42565b6020604051808303816000875af11580156111b0573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906111d49190614e1a565b6111f15760405163068fdd5760e41b815260040160405180910390fd5b50506002805460ff60a81b19169055506001979650505050505050565b6001546000906001600160a01b0316331461123c576040516330cd747160e01b815260040160405180910390fd5b60008061124a600085611d4a565b60408051808201825283815260208082018481526003546000908152601483528481208c8252909252929020905181559051600190910155600954600a549294509092506112999184846129ea565b600a55600955506001949350505050565b6001546000906001600160a01b031633146112d8576040516330cd747160e01b815260040160405180910390fd5b6001600054146112fb576040516321c4e35760e21b815260040160405180910390fd5b600254600160a01b900460ff1615611326576040516306d39fcd60e41b815260040160405180910390fd5b6012546001600160a01b03168061135057604051633896c50b60e21b815260040160405180910390fd5b806001600160a01b03163b60000361137b576040516362d4176d60e11b815260040160405180910390fd5b6000816001600160a01b0316846040516024016113989190614e35565b60408051601f198184030181529181526020820180516001600160e01b03166311475c8760e31b179052516113cd9190614cdd565b600060405180830381855afa9150503d8060008114611408576040519150601f19603f3d011682016040523d82523d6000602084013e61140d565b606091505b505090508061142f576040516309bde33960e01b815260040160405180910390fd5b304660a01b1761020085013514611459576040516375893cc160e11b815260040160405180910390fd5b6000858152601560205260409020546101008501351461148c576040516319dcebfb60e21b815260040160405180910390fd5b60008061149887610a02565b9092509050610120860135821415806114d557508061010087016114bd600180614e80565b600981106114cd576114cd614e54565b602002013514155b156114f3576040516319dcebfb60e21b815260040160405180910390fd5b6101a086013560008051602061513c8339815191528110611527576040516304b4b91960e11b815260040160405180910390fd5b6003546101c08801351461154e57604051631391e11b60e21b815260040160405180910390fd5b6101e08701356000818152601d602052604090205460ff161561158457604051636569570160e11b815260040160405180910390fd5b6000818152601d60205260409020805460ff191660011790556115a689613a58565b6115b06000613ac6565b60006115ba6133ea565b905060405180604001604052808a610100016003600981106115de576115de614e54565b602002013581526020018a61010001600360016115fb9190614e80565b6009811061160b5761160b614e54565b6020908102919091013590915260008381526013825260408082208e8352835281208351815592909101516001909201919091556003829055806116676116608660008051602061513c833981519152614e93565b6000611d4a565b9150915061167b60055460065484846129ea565b6006556005556007805486900390556040517f262a9a1794440b6af993000f5805d7f51b5a19d4c32fcb10a1c5216beb0616f4906116bc908e908890614656565b60405180910390a16116cc613b34565b5060019b9a5050505050505050505050565b606080826001600160401b038111156116f9576116f9614ea6565b60405190808252806020026020018201604052801561173257816020015b61171f614548565b8152602001906001900390816117175790505b509150826001600160401b0381111561174d5761174d614ea6565b604051908082528060200260200182016040528015611776578160200160208202803683370190505b50905060005b8381101561180a5761178d8161231f565b84838151811061179f5761179f614e54565b60200260200101516000018584815181106117bc576117bc614e54565b60200260200101516020018281525082815250505060156000828152602001908152602001600020548282815181106117f7576117f7614e54565b602090810291909101015260010161177c565b50915091565b3360009081526017602052604081205460609082036118425760405163aba4733960e01b815260040160405180910390fd5b600160005414611865576040516321c4e35760e21b815260040160405180910390fd5b600254600160a01b900460ff1615611890576040516306d39fcd60e41b815260040160405180910390fd5b600254600160a81b900460ff16156118bb57604051633ee5aeb560e01b815260040160405180910390fd5b6002805460ff60a81b1916600160a81b1790558288146118ee5760405163023f995760e61b815260040160405180910390fd5b6006881461190f576040516354fb304560e01b815260040160405180910390fd5b6000888152601a60205260409020546001600160a01b03168061194557604051633896c50b60e21b815260040160405180910390fd5b806001600160a01b03163b600003611970576040516362d4176d60e11b815260040160405180910390fd5b6000816001600160a01b03168960405160240161198d9190614cce565b60408051601f198184030181529181526020820180516001600160e01b0316630f948af160e01b179052516119c29190614cdd565b600060405180830381855afa9150503d80600081146119fd576040519150601f19603f3d011682016040523d82523d6000602084013e611a02565b606091505b5050905080611a24576040516309bde33960e01b815260040160405180910390fd5b611a35896101000187878e8e613662565b611a4289610100016138d0565b611a4f89610100016138db565b611a5f88886107408c0135613b3c565b611a6b8b8b88886138e5565b611a758b8b6139ed565b6000611a818989613b97565b6002805460ff60a81b1916905560019d909c509a5050505050505050505050565b600080611aae83613d83565b91509150915091565b6001546000906001600160a01b03163314611ae5576040516330cd747160e01b815260040160405180910390fd5b60008051602061513c8339815191528210611b135760405163bb22c5a960e01b815260040160405180910390fd5b7fb404cac19fb1cbeff98d325795b08886e3cd8fe8cb1a2f193aac66f13fb239c360085483604051611b46929190614656565b60405180910390a150600855600190565b336000908152601760205260408120548103611b865760405163aba4733960e01b815260040160405180910390fd5b600160005414611ba9576040516321c4e35760e21b815260040160405180910390fd5b600254600160a01b900460ff1615611bd4576040516306d39fcd60e41b815260040160405180910390fd5b611bdd86613e1c565b611bee866101000186868b8b613f2a565b611bfb86610100016138d0565b611c0886610100016138db565b600854610740870135908114611c31576040516358d620b360e01b815260040160405180910390fd5b611c3a81614198565b6000611c446133ea565b9050611c53818b8b8a8a613421565b611c5d6000613ac6565b6003819055604051339060008051602061509c83398151915290600090a2336001600160a01b03166000805160206150dc8339815191528686604051611ca4929190614c87565b60405180910390a25060019998505050505050505050565b6002546000906001600160a01b03163314611cea57604051630614e5c760e21b815260040160405180910390fd5b600180546001600160a01b0319808216339081179093556002805490911690556040516001600160a01b03909116919082907f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e090600090a3600191505090565b600080600080611d5986611aa2565b91509150600080611d69876125d1565b91509150611d79848484846129ea565b95509550505050505b9250929050565b6001546000906001600160a01b03163314611db7576040516330cd747160e01b815260040160405180910390fd5b600160005403611dd95760405162dc149f60e41b815260040160405180910390fd5b506001600081815560058190556006829055600955600a81905590565b6001546000906001600160a01b03163314611e24576040516330cd747160e01b815260040160405180910390fd5b600160005414611e47576040516321c4e35760e21b815260040160405180910390fd5b600254600160a01b900460ff1615611e72576040516306d39fcd60e41b815260040160405180910390fd5b60008781526015602052604090205415611e9f57604051630ea075bf60e21b815260040160405180910390fd5b86600003611ec057604051630d57928360e21b815260040160405180910390fd5b85600003611ee15760405163145a1fdd60e31b815260040160405180910390fd5b8115801590611ef257506104a08214155b15611f105760405163759f482960e11b815260040160405180910390fd5b611f1a858561421c565b611f375760405163ecd2690d60e01b815260040160405180910390fd5b600087815260156020908152604080832089905560169091529020611f5d838583614f08565b506001600160a01b03881660009081526017602090815260408083208a9055805180820182528881528083018881526003548552601384528285208c865290935292209151825551600190910155600554600654611fbd919087876129ea565b6006556005556004805460010190556040518781526001600160a01b038916907fefd1ddef00b1051abc144c2e895de70a10dbbc3ad8985118c74c15e40e3d391f9060200160405180910390a2506001979650505050505050565b6001546000906001600160a01b03163314612046576040516330cd747160e01b815260040160405180910390fd5b6002805460ff60a01b1916600160a01b1790556040517f62e78cea01bee320cd4e420270b5ea74000d11b0c9f74754ebdbfc544b05a25890610ac1903390614594565b6001546000906001600160a01b031633146120b7576040516330cd747160e01b815260040160405180910390fd5b6001600054146120da576040516321c4e35760e21b815260040160405180910390fd5b600254600160a01b900460ff1615612105576040516306d39fcd60e41b815260040160405180910390fd5b61210f838361421c565b61212c5760405163ecd2690d60e01b815260040160405180910390fd5b61213c60055460065485856129ea565b600655600555600780548601905561215384613a58565b61215d6000613ac6565b600354600090815260136020908152604080832087845290915281208054600182015491929182916121909188886129ea565b91509150600061219e6133ea565b60408051808201825285815260208082018681526000858152601383528481208e825290925290839020915182555160019091015560038290555190915081906000805160206150fc833981519152906121fb908c908c90614656565b60405180910390a261220b613b34565b50600198975050505050505050565b6001546000906001600160a01b03163314612248576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b03821661226f5760405163d92e233d60e01b815260040160405180910390fd5b816001600160a01b03163b60000361229a576040516362d4176d60e11b815260040160405180910390fd5b600660005260186020527f33d69b83f8d9644c8360a50d7275bf12a50019a4cd4e17926d0b315da648c58d80546001600160a01b0384166001600160a01b03199182168117909255600d8054909116821790556004546040516000805160206150bc833981519152916109039190815260200190565b600061231a6142c6565b905090565b600354600090815260146020908152604080832084845290915281208054829190158015610a3257506001810154610a44575060009360019350915050565b6001546000906001600160a01b0316331461238c576040516330cd747160e01b815260040160405180910390fd5b6001600054146123af576040516321c4e35760e21b815260040160405180910390fd5b6000806123bb85611aa2565b915091506123cf600954600a5484846129ea565b600a55600955600b8054860190556123e684613ac6565b6123f06000613a58565b600354600090815260146020908152604080832087845290915281208054600182015491929182916124239187876129ea565b9150915060006124316133ea565b60408051808201825285815260208082018681526000858152601483528481208e825290925290839020915182555160019091015560038290555190915081906000805160206150fc8339815191529061248e908c908c90614656565b60405180910390a250600198975050505050505050565b606080826001600160401b038111156124c0576124c0614ea6565b6040519080825280602002602001820160405280156124f957816020015b6124e6614548565b8152602001906001900390816124de5790505b509150826001600160401b0381111561251457612514614ea6565b60405190808252806020026020018201604052801561253d578160200160208202803683370190505b50905060005b8381101561180a5761255481610a02565b84838151811061256657612566614e54565b602002602001015160000185848151811061258357612583614e54565b60200260200101516020018281525082815250505060156000828152602001908152602001600020548282815181106125be576125be614e54565b6020908102919091010152600101612543565b600080611aae83614330565b6001546000906001600160a01b0316331461260b576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b0382166126325760405163d92e233d60e01b815260040160405180910390fd5b816001600160a01b03163b60000361265d576040516362d4176d60e11b815260040160405180910390fd5b600660005260196020527f4ca1d5f267eb2abf27b670f18ee76f6c205873a2168eb2e690c8c4babcc357dd80546001600160a01b0384166001600160a01b031990911681179091556004546040516000805160206150bc833981519152916109039190815260200190565b6001546000906001600160a01b031633146126f6576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b03821661271d5760405163d92e233d60e01b815260040160405180910390fd5b816001600160a01b03163b600003612748576040516362d4176d60e11b815260040160405180910390fd5b601280546001600160a01b0319166001600160a01b0384169081179091556004546040519081526000805160206150bc83398151915290602001610903565b6001546000906001600160a01b031633146127b5576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b0382166127dc57604051633a247dd760e11b815260040160405180910390fd5b600280546001600160a01b0319166001600160a01b03848116918217909255600154604051919216907f38d16b8cac22d99fc7c124b9cd0de2d3fa1faef420bfe791d8c362d765e2270090600090a3506001919050565b6001546000906001600160a01b03163314612861576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b0382166128885760405163d92e233d60e01b815260040160405180910390fd5b6006600052601c6020527fb2d730c3da277545f6c9f2c922ea3d0e9fcbd74e29d321b995a837ef61c283f080546001600160a01b0384166001600160a01b0319918216811790925560108054909116821790556004546040516000805160206150bc833981519152916109039190815260200190565b6001546000906001600160a01b0316331461292c576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b0383166129535760405163d92e233d60e01b815260040160405180910390fd5b826001600160a01b03163b60000361297e576040516362d4176d60e11b815260040160405180910390fd5b6000828152601a60205260409081902080546001600160a01b0386166001600160a01b03199182168117909255600e805490911682179055600454915190916000805160206150bc833981519152916129d991815260200190565b60405180910390a250600192915050565b600080851580156129fb5750846001145b15612a0a5750829050816109c2565b83158015612a185750826001145b15612a275750849050836109c2565b600060008051602061511c8339815191528588099050600060008051602061511c8339815191528588099050600060008051602061511c83398151915280838509620292f8099050600060008051602061511c83398151915280898b0960008051602061511c833981519152898d090890506000612ac88460008051602061511c83398151915287620292fc0960008051602061511c8339815191526143bc565b905060008051602061511c833981519152612af460008051602061511c833981519152856001086143f9565b8309965060008051602061511c833981519152612b29612b2460018660008051602061511c8339815191526143bc565b6143f9565b82099550505050505094509492505050565b6000818152601860205260409020546001600160a01b031680612b7157604051633896c50b60e21b815260040160405180910390fd5b806001600160a01b03163b600003612b9c576040516362d4176d60e11b815260040160405180910390fd5b6000816001600160a01b031684604051602401612bb99190614ff5565b60408051601f198184030181529181526020820180516001600160e01b0316633fdaa96b60e11b17905251612bee9190614cdd565b600060405180830381855afa9150503d8060008114612c29576040519150601f19603f3d011682016040523d82523d6000602084013e612c2e565b606091505b5050905080612c50576040516309bde33960e01b815260040160405180910390fd5b50505050565b6000818152601960205260409020546001600160a01b031680612b9c57604051633896c50b60e21b815260040160405180910390fd5b60245b612c9b60066024614e80565b811015612cf657818160518110612cb457612cb4614e54565b6020020135838260518110612ccb57612ccb614e54565b602002013514612cee57604051630663ff5760e11b815260040160405180910390fd5b600101612c8f565b5060435b612d0660066043614e80565b811015612d6157818160518110612d1f57612d1f614e54565b6020020135838260518110612d3657612d36614e54565b602002013514612d5957604051630663ff5760e11b815260040160405180910390fd5b600101612cfa565b50610840828101359082013514612d8b57604051630663ff5760e11b815260040160405180910390fd5b5050565b304660a01b17610a0086013514612db9576040516375893cc160e11b815260040160405180910390fd5b600080612dce60045460016106c69190614e80565b90925090508460005b81811015612ffd576000888883818110612df357612df3614e54565b905060200201359050838181518110612e0e57612e0e614e54565b6020026020010151600003612e365760405163c669128160e01b815260040160405180910390fd5b838181518110612e4857612e48614e54565b60200260200101518a836024612e5e9190614e80565b60518110612e6e57612e6e614e54565b602002013514612e91576040516319dcebfb60e21b815260040160405180910390fd5b6000612ea2600184901b602a614e80565b9050858281518110612eb657612eb6614e54565b6020026020010151600001518b8260518110612ed457612ed4614e54565b6020020135141580612f255750858281518110612ef357612ef3614e54565b6020026020010151602001518b826001612f0d9190614e80565b60518110612f1d57612f1d614e54565b602002013514155b15612f43576040516319dcebfb60e21b815260040160405180910390fd5b6000612f54600185901b6036614e80565b90508b8160518110612f6857612f68614e54565b6020020135898986818110612f7f57612f7f614e54565b90506040020160000135141580612fd157508b612f9d826001614e80565b60518110612fad57612fad614e54565b6020020135898986818110612fc457612fc4614e54565b9050604002016020013514155b15612fef576040516319dcebfb60e21b815260040160405180910390fd5b836001019350505050612dd7565b505050505050505050565b600c54610a008601351461302e5760405162a4671960e71b815260040160405180910390fd5b60008061304360045460016104c99190614e80565b90925090508460005b81811015612ffd57600088888381811061306857613068614e54565b90506020020135905083818151811061308357613083614e54565b60200260200101518a8360246130999190614e80565b605181106130a9576130a9614e54565b6020020135146130cc576040516319dcebfb60e21b815260040160405180910390fd5b60006130dd600184901b602a614e80565b90508582815181106130f1576130f1614e54565b6020026020010151600001518b826051811061310f5761310f614e54565b6020020135141580613160575085828151811061312e5761312e614e54565b6020026020010151602001518b8260016131489190614e80565b6051811061315857613158614e54565b602002013514155b1561317e576040516319dcebfb60e21b815260040160405180910390fd5b600061318f600185901b6036614e80565b90508b81605181106131a3576131a3614e54565b60200201358989868181106131ba576131ba614e54565b9050604002016000013514158061320c57508b6131d8826001614e80565b605181106131e8576131e8614e54565b60200201358989868181106131ff576131ff614e54565b9050604002016020013514155b1561322a576040516319dcebfb60e21b815260040160405180910390fd5b83600101935050505061304c565b8060005b8181101561335657600084848381811061325857613258614e54565b90506020020135905060005b8381101561334c5780831461334457600086868381811061328757613287614e54565b600086815260208080526040808320938202959095013580835292905292909220549192505060ff166132cd576040516310d9346760e21b815260040160405180910390fd5b6000826132da8787615004565b6132e5906000614e80565b6132ef9190614e80565b6000858152601f6020908152604080832086845290915290205490915089826051811061331e5761331e614e54565b602002013514613341576040516319dcebfb60e21b815260040160405180910390fd5b50505b600101613264565b505060010161323c565b5050505050565b6003548160425b60200201351461338757604051631391e11b60e21b815260040160405180910390fd5b50565b600081604f5b602090810291909101356000818152601d90925260409091205490915060ff16156133ce57604051636569570160e11b815260040160405180910390fd5b6000908152601d60205260409020805460ff1916600117905550565b60007f0000000000000000000000000000000000000000000000000000000000000000613417814361501b565b61231a9190615004565b808381146134425760405163023f995760e61b815260040160405180910390fd5b61344c6000613a58565b6000805b8281101561351657600085858381811061346c5761346c614e54565b9050602002013590508281116134955760405163f170f72d60e01b815260040160405180910390fd5b600089815260136020908152604080832084845290915281208054600182015493955085939192918291613500918d8d898181106134d5576134d5614e54565b905060400201600001358e8e8a8181106134f1576134f1614e54565b905060400201602001356129ea565b9084556001938401555050919091019050613450565b5050505050505050565b60045460015b818111613589576135368161442c565b613541848483614467565b61358157600354600090815260146020818152604080842085855282528084208b8552928252808420858552909152909120815481556001918201549101555b600101613526565b508360005b818110156135165760008585838181106135aa576135aa614e54565b9050602002013590506000601460006003548152602001908152602001600020600083815260200190815260200160002090506000806135ff836000015484600101548d8d898181106134d5576134d5614e54565b91509150604051806040016040528083815260200182815250601460008e8152602001908152602001600020600086815260200190815260200160002060008201518160000155602082015181600101559050508460010194505050505061358e565b304660a01b176106608601351461368c576040516375893cc160e11b815260040160405180910390fd5b6000806136a160045460016106c69190614e80565b90925090508460005b81811015612ffd5760008888838181106136c6576136c6614e54565b9050602002013590508381815181106136e1576136e1614e54565b60200260200101516000036137095760405163c669128160e01b815260040160405180910390fd5b83818151811061371b5761371b614e54565b60200260200101518a8360066137319190614e80565b6034811061374157613741614e54565b602002013514613764576040516319dcebfb60e21b815260040160405180910390fd5b6000613775600184901b600c614e80565b905085828151811061378957613789614e54565b6020026020010151600001518b82603481106137a7576137a7614e54565b60200201351415806137f857508582815181106137c6576137c6614e54565b6020026020010151602001518b8260016137e09190614e80565b603481106137f0576137f0614e54565b602002013514155b15613816576040516319dcebfb60e21b815260040160405180910390fd5b6000613827600185901b6018614e80565b90508b816034811061383b5761383b614e54565b602002013589898681811061385257613852614e54565b905060400201600001351415806138a457508b613870826001614e80565b6034811061388057613880614e54565b602002013589898681811061389757613897614e54565b9050604002016020013514155b156138c2576040516319dcebfb60e21b815260040160405180910390fd5b8360010193505050506136aa565b600354816024613364565b6000816031613390565b808381146139065760405163023f995760e61b815260040160405180910390fd5b6139106000613a58565b600061391a6133ea565b90506000805b838110156139d757600086868381811061393c5761393c614e54565b9050602002013590508281116139655760405163f170f72d60e01b815260040160405180910390fd5b6000848152601360209081526040808320848452909152812080546001820154939550859391929182916139c1918e8e898181106139a5576139a5614e54565b905060400201600001358f8f8a8181106134f1576134f1614e54565b9084556001938401555050919091019050613920565b506139e26000613ac6565b506003555050505050565b60006001815b83811015613a3b57613a2e8383878785818110613a1257613a12614e54565b905060400201600001358888868181106134f1576134f1614e54565b90935091506001016139f3565b50613a4c60055460065484846129ea565b60065560055550505050565b6000613a626133ea565b60045490915060015b818111612c5057613a7b816144b2565b838114613abe5760035460009081526013602081815260408084208585528252808420878552928252808420858552909152909120815481556001918201549101555b600101613a6b565b6000613ad06133ea565b60045490915060015b818111612c5057613ae98161442c565b838114613b2c5760035460009081526014602081815260408084208585528252808420878552928252808420858552909152909120815481556001918201549101555b600101613ad9565b6133876142c6565b6000805b83811015613b7757848482818110613b5a57613b5a614e54565b613b6d9260609091020135905083614e80565b9150600101613b40565b50818114612c50576040516202aef760e91b815260040160405180910390fd5b6010546060906001600160a01b0316826000816001600160401b03811115613bc157613bc1614ea6565b604051908082528060200260200182016040528015613bea578160200160208202803683370190505b50905060005b82811015613d7957604080516002808252606082018352600092602083019080368337019050509050878783818110613c2b57613c2b614e54565b9050606002016000013581600081518110613c4857613c48614e54565b602002602001018181525050878783818110613c6657613c66614e54565b9050606002016040013581600181518110613c8357613c83614e54565b602002602001018181525050600080866001600160a01b03166383bf2edd846040518263ffffffff1660e01b8152600401613cbe919061503d565b60408051808303816000875af1158015613cdc573d6000803e3d6000fd5b505050506040513d601f19601f82011682018060405250810190613d009190615050565b9150915081613d225760405163068fdd5760e41b815260040160405180910390fd5b80858581518110613d3557613d35614e54565b602090810291909101015260405181907fef61e988d9804d573b4fc504760f55d3507094e4168fddc9245ac56fbfc419e490600090a2836001019350505050613bf0565b5095945050505050565b600080827f1b46f45118b90335391ae7f66ffe16bdf117c9b9be79e1802d987d2347fcb12b7f21a94082e95c6df187baa045c30f4cb20e13eeb4b9ef2ef96bc2659242d50cda8360015b8415613e0f576001851615613dee57613de8828286866129ea565b90925090505b613df884846144ea565b9094509250613e0860028661501b565b9450613dcd565b9097909650945050505050565b6011546001600160a01b0316613e4557604051633896c50b60e21b815260040160405180910390fd5b6011546001600160a01b03163b600003613e72576040516362d4176d60e11b815260040160405180910390fd5b6011546040516000916001600160a01b031690613e9390849060240161507c565b60408051601f198184030181529181526020820180516001600160e01b031663f6fbf48960e01b17905251613ec89190614cdd565b600060405180830381855afa9150503d8060008114613f03576040519150601f19603f3d011682016040523d82523d6000602084013e613f08565b606091505b5050905080612d8b576040516309bde33960e01b815260040160405180910390fd5b304660a01b176106c086013514613f54576040516375893cc160e11b815260040160405180910390fd5b600080613f6960045460016106c69190614e80565b90925090508460005b81811015612ffd576000888883818110613f8e57613f8e614e54565b905060200201359050838181518110613fa957613fa9614e54565b6020026020010151600003613fd15760405163c669128160e01b815260040160405180910390fd5b838181518110613fe357613fe3614e54565b60200260200101518a836006613ff99190614e80565b6037811061400957614009614e54565b60200201351461402c576040516319dcebfb60e21b815260040160405180910390fd5b600061403d600184901b600c614e80565b905085828151811061405157614051614e54565b6020026020010151600001518b826037811061406f5761406f614e54565b60200201351415806140c0575085828151811061408e5761408e614e54565b6020026020010151602001518b8260016140a89190614e80565b603781106140b8576140b8614e54565b602002013514155b156140de576040516319dcebfb60e21b815260040160405180910390fd5b60006140ef600185901b6018614e80565b90508b816037811061410357614103614e54565b602002013589898681811061411a5761411a614e54565b9050604002016000013514158061416c57508b614138826001614e80565b6037811061414857614148614e54565b602002013589898681811061415f5761415f614e54565b9050604002016020013514155b1561418a576040516319dcebfb60e21b815260040160405180910390fd5b836001019350505050613f72565b806000036141a35750565b6000806141c16116608460008051602061513c833981519152614e93565b915091506141d560055460065484846129ea565b6006556005556007805484900390556040518381527fa551808c565cfbf20dfffdbcd44c549f835f9d06a82dcd546c61644b2f5ce7919060200160405180910390a1505050565b60008060008051602061511c8339815191528485099050600060008051602061511c8339815191528485099050600060008051602061511c8339815191528260008051602061511c83398151915285620292fc09089050600060008051602061511c833981519152808460008051602061511c83398151915287620292f8090960010890506142ba828260008051602061511c8339815191526143bc565b15979650505050505050565b6000806001805b6004548111614301576000806142e283610a02565b915091506142f2858584846129ea565b909550935050506001016142cd565b5081600554141580610bdd57508060065414610bfb57604051631947c14d60e31b815260040160405180910390fd5b600080827f16546696a66928d34f6be843f8a5afa2063161d92742811279454d60de5322527f109c1c7a758b3e8e54af1ce919fc24e1b986aab09a6b8082600f8694bb3c1b4b8360015b8415613e0f57600185161561439b57614395828286866129ea565b90925090505b6143a584846144ea565b90945092506143b560028661501b565b945061437a565b6000838381116143d3576143d08382614e80565b90505b82806143e1576143e1614fc7565b60006143ed8684614e93565b089150505b9392505050565b6000610f4282614418600260008051602061511c833981519152614e93565b60008051602061511c833981519152614504565b60035460009081526014602090815260408083208484529091529020805415801561445957506001810154155b15612d8b5760019081015550565b600082815b818110156144a6578386868381811061448757614487614e54565b905060200201350361449e576001925050506143f2565b60010161446c565b50600095945050505050565b60035460009081526013602090815260408083208484529091529020805415801561445957506001810154612d8b5760019081015550565b6000806144f9848486866129ea565b915091509250929050565b600060405160208152602080820152602060408201528460608201528360808201528260a082015260208160c08360055afa8080156102e057505051949350505050565b604051806040016040528060008152602001600081525090565b80356001600160a01b038116811461090f57600080fd5b60006020828403121561458b57600080fd5b6143f282614562565b6001600160a01b0391909116815260200190565b6000602082840312156145ba57600080fd5b5035919050565b60005b838110156145dc5781810151838201526020016145c4565b50506000910152565b600081518084526145fd8160208601602086016145c1565b601f01601f19169290920160200192915050565b6020815260006143f260208301846145e5565b6000806000806080858703121561463a57600080fd5b5050823594602084013594506040840135936060013592509050565b918252602082015260400190565b60008083601f84011261467657600080fd5b5081356001600160401b0381111561468d57600080fd5b6020830191508360208260061b8501011115611d8257600080fd5b6000610b2082840312156146bb57600080fd5b50919050565b60008083601f8401126146d357600080fd5b5081356001600160401b038111156146ea57600080fd5b6020830191508360208260051b8501011115611d8257600080fd5b60008083601f84011261471757600080fd5b5081356001600160401b0381111561472e57600080fd5b602083019150836020828501011115611d8257600080fd5b6000806000806000806000806000806116c08b8d03121561476657600080fd5b8a356001600160401b0381111561477c57600080fd5b6147888d828e01614664565b909b50995061479c90508c60208d016146a8565b9750610b408b01356001600160401b038111156147b857600080fd5b6147c48d828e01614664565b90985096506147d990508c610b608d016146a8565b94506116808b01356001600160401b038111156147f557600080fd5b6148018d828e016146c1565b9095509350506116a08b01356001600160401b0381111561482157600080fd5b61482d8d828e01614705565b915080935050809150509295989b9194979a5092959850565b6000806040838503121561485957600080fd5b50508035926020909101359150565b600061078082840312156146bb57600080fd5b6000806000806000806107e0878903121561489557600080fd5b86356001600160401b038111156148ab57600080fd5b6148b789828a01614664565b90975095506148cb90508860208901614868565b93506107a08701356001600160401b038111156148e757600080fd5b87016020818a0312156148f957600080fd5b92506107c08701356001600160401b0381111561491557600080fd5b61492189828a016146c1565b979a9699509497509295939492505050565b60008082840361024081121561494857600080fd5b83359250610220601f198201121561495f57600080fd5b506020830190509250929050565b600081518084526020840193506020830160005b8281101561499f578151865260209586019590910190600101614981565b5093949350505050565b6040808252835190820181905260009060208501906060840190835b818110156149ef5783518051845260209081015181850152909301926040909201916001016149c5565b50508381036020850152614a03818661496d565b9695505050505050565b60008060008060008060006107e0888a031215614a2957600080fd5b87356001600160401b03811115614a3f57600080fd5b614a4b8a828b01614664565b9098509650614a5f90508960208a01614868565b94506107a08801356001600160401b03811115614a7b57600080fd5b8801601f81018a13614a8c57600080fd5b80356001600160401b03811115614aa257600080fd5b8a6020606083028401011115614ab757600080fd5b602091909101945092506107c08801356001600160401b03811115614adb57600080fd5b614ae78a828b016146c1565b989b979a50959850939692959293505050565b8215158152604060208201526000614b15604083018461496d565b949350505050565b6000806000806000806000878903610840811215614b3a57600080fd5b88356001600160401b03811115614b5057600080fd5b614b5c8b828c01614664565b9099509750506107e0601f1982011215614b7557600080fd5b506020880194506108008801356001600160401b03811115614b9657600080fd5b614ba28a828b016146c1565b9095509350506108208801356001600160401b03811115614bc257600080fd5b614ae78a828b01614705565b600080600080600080600060c0888a031215614be957600080fd5b614bf288614562565b96506020880135955060408801359450606088013593506080880135925060a08801356001600160401b03811115614bc257600080fd5b60008060408385031215614c3c57600080fd5b614c4583614562565b946020939093013593505050565b600181811c90821680614c6757607f821691505b6020821081036146bb57634e487b7160e01b600052602260045260246000fd5b60208152816020820152818360408301376000818301604090810191909152601f909201601f19160101919050565b61010081833761068061010082016101008401375050565b6107808101610f428284614cb6565b60008251614cef8184602087016145c1565b9190910192915050565b6000823561015e19833603018112614cef57600080fd5b81835260006001600160fb1b03831115614d2957600080fd5b8260051b80836020870137939093016020019392505050565b602080825282358282015282013560408201526040808301606083013760406080830160a0830137614d8460e0820160c0840180358252602090810135910152565b6000610100830135601e19843603018112614d9e57600080fd5b83016020810190356001600160401b03811115614dba57600080fd5b8060051b3603821315614dcc57600080fd5b610160610120850152614de461018085018284614d10565b610120860135610140868101919091529095013561016090940193909352509192915050565b8051801515811461090f57600080fd5b600060208284031215614e2c57600080fd5b6143f282614e0a565b6102208101610100838337610120610100840161010084013792915050565b634e487b7160e01b600052603260045260246000fd5b634e487b7160e01b600052601160045260246000fd5b80820180821115610f4257610f42614e6a565b81810381811115610f4257610f42614e6a565b634e487b7160e01b600052604160045260246000fd5b601f821115614f0357806000526020600020601f840160051c81016020851015614ee35750805b601f840160051c820191505b818110156133565760008155600101614eef565b505050565b6001600160401b03831115614f1f57614f1f614ea6565b614f3383614f2d8354614c53565b83614ebc565b6000601f841160018114614f675760008515614f4f5750838201355b600019600387901b1c1916600186901b178355613356565b600083815260209020601f19861690835b82811015614f985786850135825560209485019460019092019101614f78565b5086821015614fb55760001960f88860031b161c19848701351681555b505060018560011b0183555050505050565b634e487b7160e01b600052601260045260246000fd5b610100818337610a2061010082016101008401375050565b610b208101610f428284614fdd565b8082028115828204841417610f4257610f42614e6a565b60008261503857634e487b7160e01b600052601260045260246000fd5b500490565b6020815260006143f2602083018461496d565b6000806040838503121561506357600080fd5b61506c83614e0a565b9150602083015190509250929050565b6107e081016101008383376106e061010084016101008401379291505056fee85c8c79cebe1b6656a265affa1c69c79539e5ae9a9c9229f5b5d89619781080983b8264b64c9863a439320eb632213f6e5ca279753b012988656784757d97751d5f56c1b8cdd9a3aa6495672f7e6917c2f5f1e98a43abd3e16e2e997c1160e1eae287c62f1ff4911334dee03f631d5dded5284b1b03ea7bc1d6282916c7249f30644e72e131a029b85045b68181585d2833e84879b9709143e1f593f0000001060c89ce5c263405370a08b6d0302b0bab3eedb83920ee0a677297dc392126f1a2646970667358221220c7697bf95521c0939c5e70dc1eb28975459a29151bc01cd78eca17825b67deae64736f6c634300081b0033",
}

// EnygmaABI is the input ABI used to generate the binding from.
// Deprecated: Use EnygmaMetaData.ABI instead.
var EnygmaABI = EnygmaMetaData.ABI

// EnygmaBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use EnygmaMetaData.Bin instead.
var EnygmaBin = EnygmaMetaData.Bin

// DeployEnygma deploys a new Ethereum contract, binding an instance of Enygma to it.
func DeployEnygma(auth *bind.TransactOpts, backend bind.ContractBackend, _epochInterval *big.Int) (common.Address, *types.Transaction, *Enygma, error) {
	parsed, err := EnygmaMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(EnygmaBin), backend, _epochInterval)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &Enygma{EnygmaCaller: EnygmaCaller{contract: contract}, EnygmaTransactor: EnygmaTransactor{contract: contract}, EnygmaFilterer: EnygmaFilterer{contract: contract}}, nil
}

// Enygma is an auto generated Go binding around an Ethereum contract.
type Enygma struct {
	EnygmaCaller     // Read-only binding to the contract
	EnygmaTransactor // Write-only binding to the contract
	EnygmaFilterer   // Log filterer for contract events
}

// EnygmaCaller is an auto generated read-only Go binding around an Ethereum contract.
type EnygmaCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EnygmaTransactor is an auto generated write-only Go binding around an Ethereum contract.
type EnygmaTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EnygmaFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type EnygmaFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EnygmaSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type EnygmaSession struct {
	Contract     *Enygma           // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// EnygmaCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type EnygmaCallerSession struct {
	Contract *EnygmaCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts // Call options to use throughout this session
}

// EnygmaTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type EnygmaTransactorSession struct {
	Contract     *EnygmaTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// EnygmaRaw is an auto generated low-level Go binding around an Ethereum contract.
type EnygmaRaw struct {
	Contract *Enygma // Generic contract binding to access the raw methods on
}

// EnygmaCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type EnygmaCallerRaw struct {
	Contract *EnygmaCaller // Generic read-only contract binding to access the raw methods on
}

// EnygmaTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type EnygmaTransactorRaw struct {
	Contract *EnygmaTransactor // Generic write-only contract binding to access the raw methods on
}

// NewEnygma creates a new instance of Enygma, bound to a specific deployed contract.
func NewEnygma(address common.Address, backend bind.ContractBackend) (*Enygma, error) {
	contract, err := bindEnygma(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Enygma{EnygmaCaller: EnygmaCaller{contract: contract}, EnygmaTransactor: EnygmaTransactor{contract: contract}, EnygmaFilterer: EnygmaFilterer{contract: contract}}, nil
}

// NewEnygmaCaller creates a new read-only instance of Enygma, bound to a specific deployed contract.
func NewEnygmaCaller(address common.Address, caller bind.ContractCaller) (*EnygmaCaller, error) {
	contract, err := bindEnygma(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &EnygmaCaller{contract: contract}, nil
}

// NewEnygmaTransactor creates a new write-only instance of Enygma, bound to a specific deployed contract.
func NewEnygmaTransactor(address common.Address, transactor bind.ContractTransactor) (*EnygmaTransactor, error) {
	contract, err := bindEnygma(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &EnygmaTransactor{contract: contract}, nil
}

// NewEnygmaFilterer creates a new log filterer instance of Enygma, bound to a specific deployed contract.
func NewEnygmaFilterer(address common.Address, filterer bind.ContractFilterer) (*EnygmaFilterer, error) {
	contract, err := bindEnygma(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &EnygmaFilterer{contract: contract}, nil
}

// bindEnygma binds a generic wrapper to an already deployed contract.
func bindEnygma(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := EnygmaMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Enygma *EnygmaRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Enygma.Contract.EnygmaCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Enygma *EnygmaRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Enygma.Contract.EnygmaTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Enygma *EnygmaRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Enygma.Contract.EnygmaTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Enygma *EnygmaCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Enygma.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Enygma *EnygmaTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Enygma.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Enygma *EnygmaTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Enygma.Contract.contract.Transact(opts, method, params...)
}

// DepositVerifierAddress is a free data retrieval call binding the contract method 0x07da47ea.
//
// Solidity: function DepositVerifierAddress() view returns(address)
func (_Enygma *EnygmaCaller) DepositVerifierAddress(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "DepositVerifierAddress")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// DepositVerifierAddress is a free data retrieval call binding the contract method 0x07da47ea.
//
// Solidity: function DepositVerifierAddress() view returns(address)
func (_Enygma *EnygmaSession) DepositVerifierAddress() (common.Address, error) {
	return _Enygma.Contract.DepositVerifierAddress(&_Enygma.CallOpts)
}

// DepositVerifierAddress is a free data retrieval call binding the contract method 0x07da47ea.
//
// Solidity: function DepositVerifierAddress() view returns(address)
func (_Enygma *EnygmaCallerSession) DepositVerifierAddress() (common.Address, error) {
	return _Enygma.Contract.DepositVerifierAddress(&_Enygma.CallOpts)
}

// GetBlckHash is a free data retrieval call binding the contract method 0x743873b4.
//
// Solidity: function GetBlckHash() view returns(uint256)
func (_Enygma *EnygmaCaller) GetBlckHash(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "GetBlckHash")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetBlckHash is a free data retrieval call binding the contract method 0x743873b4.
//
// Solidity: function GetBlckHash() view returns(uint256)
func (_Enygma *EnygmaSession) GetBlckHash() (*big.Int, error) {
	return _Enygma.Contract.GetBlckHash(&_Enygma.CallOpts)
}

// GetBlckHash is a free data retrieval call binding the contract method 0x743873b4.
//
// Solidity: function GetBlckHash() view returns(uint256)
func (_Enygma *EnygmaCallerSession) GetBlckHash() (*big.Int, error) {
	return _Enygma.Contract.GetBlckHash(&_Enygma.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x8052474d.
//
// Solidity: function Name() pure returns(string)
func (_Enygma *EnygmaCaller) Name(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "Name")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Name is a free data retrieval call binding the contract method 0x8052474d.
//
// Solidity: function Name() pure returns(string)
func (_Enygma *EnygmaSession) Name() (string, error) {
	return _Enygma.Contract.Name(&_Enygma.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x8052474d.
//
// Solidity: function Name() pure returns(string)
func (_Enygma *EnygmaCallerSession) Name() (string, error) {
	return _Enygma.Contract.Name(&_Enygma.CallOpts)
}

// Symbol is a free data retrieval call binding the contract method 0x3045aaf3.
//
// Solidity: function Symbol() pure returns(string)
func (_Enygma *EnygmaCaller) Symbol(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "Symbol")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Symbol is a free data retrieval call binding the contract method 0x3045aaf3.
//
// Solidity: function Symbol() pure returns(string)
func (_Enygma *EnygmaSession) Symbol() (string, error) {
	return _Enygma.Contract.Symbol(&_Enygma.CallOpts)
}

// Symbol is a free data retrieval call binding the contract method 0x3045aaf3.
//
// Solidity: function Symbol() pure returns(string)
func (_Enygma *EnygmaCallerSession) Symbol() (string, error) {
	return _Enygma.Contract.Symbol(&_Enygma.CallOpts)
}

// TotalRegisteredBanks is a free data retrieval call binding the contract method 0x84aaa2de.
//
// Solidity: function TotalRegisteredBanks() view returns(uint256)
func (_Enygma *EnygmaCaller) TotalRegisteredBanks(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "TotalRegisteredBanks")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TotalRegisteredBanks is a free data retrieval call binding the contract method 0x84aaa2de.
//
// Solidity: function TotalRegisteredBanks() view returns(uint256)
func (_Enygma *EnygmaSession) TotalRegisteredBanks() (*big.Int, error) {
	return _Enygma.Contract.TotalRegisteredBanks(&_Enygma.CallOpts)
}

// TotalRegisteredBanks is a free data retrieval call binding the contract method 0x84aaa2de.
//
// Solidity: function TotalRegisteredBanks() view returns(uint256)
func (_Enygma *EnygmaCallerSession) TotalRegisteredBanks() (*big.Int, error) {
	return _Enygma.Contract.TotalRegisteredBanks(&_Enygma.CallOpts)
}

// TotalSupply is a free data retrieval call binding the contract method 0xa44b47f7.
//
// Solidity: function TotalSupply() view returns(uint256)
func (_Enygma *EnygmaCaller) TotalSupply(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "TotalSupply")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TotalSupply is a free data retrieval call binding the contract method 0xa44b47f7.
//
// Solidity: function TotalSupply() view returns(uint256)
func (_Enygma *EnygmaSession) TotalSupply() (*big.Int, error) {
	return _Enygma.Contract.TotalSupply(&_Enygma.CallOpts)
}

// TotalSupply is a free data retrieval call binding the contract method 0xa44b47f7.
//
// Solidity: function TotalSupply() view returns(uint256)
func (_Enygma *EnygmaCallerSession) TotalSupply() (*big.Int, error) {
	return _Enygma.Contract.TotalSupply(&_Enygma.CallOpts)
}

// VerifierAddress is a free data retrieval call binding the contract method 0x874ed5b5.
//
// Solidity: function VerifierAddress() view returns(address)
func (_Enygma *EnygmaCaller) VerifierAddress(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "VerifierAddress")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// VerifierAddress is a free data retrieval call binding the contract method 0x874ed5b5.
//
// Solidity: function VerifierAddress() view returns(address)
func (_Enygma *EnygmaSession) VerifierAddress() (common.Address, error) {
	return _Enygma.Contract.VerifierAddress(&_Enygma.CallOpts)
}

// VerifierAddress is a free data retrieval call binding the contract method 0x874ed5b5.
//
// Solidity: function VerifierAddress() view returns(address)
func (_Enygma *EnygmaCallerSession) VerifierAddress() (common.Address, error) {
	return _Enygma.Contract.VerifierAddress(&_Enygma.CallOpts)
}

// WithdrawVerifierAddress is a free data retrieval call binding the contract method 0x2c0457e8.
//
// Solidity: function WithdrawVerifierAddress() view returns(address)
func (_Enygma *EnygmaCaller) WithdrawVerifierAddress(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "WithdrawVerifierAddress")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// WithdrawVerifierAddress is a free data retrieval call binding the contract method 0x2c0457e8.
//
// Solidity: function WithdrawVerifierAddress() view returns(address)
func (_Enygma *EnygmaSession) WithdrawVerifierAddress() (common.Address, error) {
	return _Enygma.Contract.WithdrawVerifierAddress(&_Enygma.CallOpts)
}

// WithdrawVerifierAddress is a free data retrieval call binding the contract method 0x2c0457e8.
//
// Solidity: function WithdrawVerifierAddress() view returns(address)
func (_Enygma *EnygmaCallerSession) WithdrawVerifierAddress() (common.Address, error) {
	return _Enygma.Contract.WithdrawVerifierAddress(&_Enygma.CallOpts)
}

// ZkdvpAddress is a free data retrieval call binding the contract method 0x1a4e1aa1.
//
// Solidity: function ZkdvpAddress() view returns(address)
func (_Enygma *EnygmaCaller) ZkdvpAddress(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "ZkdvpAddress")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// ZkdvpAddress is a free data retrieval call binding the contract method 0x1a4e1aa1.
//
// Solidity: function ZkdvpAddress() view returns(address)
func (_Enygma *EnygmaSession) ZkdvpAddress() (common.Address, error) {
	return _Enygma.Contract.ZkdvpAddress(&_Enygma.CallOpts)
}

// ZkdvpAddress is a free data retrieval call binding the contract method 0x1a4e1aa1.
//
// Solidity: function ZkdvpAddress() view returns(address)
func (_Enygma *EnygmaCallerSession) ZkdvpAddress() (common.Address, error) {
	return _Enygma.Contract.ZkdvpAddress(&_Enygma.CallOpts)
}

// AddPedComm is a free data retrieval call binding the contract method 0x132ce4d4.
//
// Solidity: function addPedComm(uint256 p1x, uint256 p1y, uint256 p2x, uint256 p2y) view returns(uint256, uint256)
func (_Enygma *EnygmaCaller) AddPedComm(opts *bind.CallOpts, p1x *big.Int, p1y *big.Int, p2x *big.Int, p2y *big.Int) (*big.Int, *big.Int, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "addPedComm", p1x, p1y, p2x, p2y)

	if err != nil {
		return *new(*big.Int), *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	out1 := *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)

	return out0, out1, err

}

// AddPedComm is a free data retrieval call binding the contract method 0x132ce4d4.
//
// Solidity: function addPedComm(uint256 p1x, uint256 p1y, uint256 p2x, uint256 p2y) view returns(uint256, uint256)
func (_Enygma *EnygmaSession) AddPedComm(p1x *big.Int, p1y *big.Int, p2x *big.Int, p2y *big.Int) (*big.Int, *big.Int, error) {
	return _Enygma.Contract.AddPedComm(&_Enygma.CallOpts, p1x, p1y, p2x, p2y)
}

// AddPedComm is a free data retrieval call binding the contract method 0x132ce4d4.
//
// Solidity: function addPedComm(uint256 p1x, uint256 p1y, uint256 p2x, uint256 p2y) view returns(uint256, uint256)
func (_Enygma *EnygmaCallerSession) AddPedComm(p1x *big.Int, p1y *big.Int, p2x *big.Int, p2y *big.Int) (*big.Int, *big.Int, error) {
	return _Enygma.Contract.AddPedComm(&_Enygma.CallOpts, p1x, p1y, p2x, p2y)
}

// AddressToAccountId is a free data retrieval call binding the contract method 0xc1ab48fc.
//
// Solidity: function addressToAccountId(address ) view returns(uint256)
func (_Enygma *EnygmaCaller) AddressToAccountId(opts *bind.CallOpts, arg0 common.Address) (*big.Int, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "addressToAccountId", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// AddressToAccountId is a free data retrieval call binding the contract method 0xc1ab48fc.
//
// Solidity: function addressToAccountId(address ) view returns(uint256)
func (_Enygma *EnygmaSession) AddressToAccountId(arg0 common.Address) (*big.Int, error) {
	return _Enygma.Contract.AddressToAccountId(&_Enygma.CallOpts, arg0)
}

// AddressToAccountId is a free data retrieval call binding the contract method 0xc1ab48fc.
//
// Solidity: function addressToAccountId(address ) view returns(uint256)
func (_Enygma *EnygmaCallerSession) AddressToAccountId(arg0 common.Address) (*big.Int, error) {
	return _Enygma.Contract.AddressToAccountId(&_Enygma.CallOpts, arg0)
}

// BalanceCommitments is a free data retrieval call binding the contract method 0xea0d4573.
//
// Solidity: function balanceCommitments(uint256 , uint256 ) view returns(uint256 c1, uint256 c2)
func (_Enygma *EnygmaCaller) BalanceCommitments(opts *bind.CallOpts, arg0 *big.Int, arg1 *big.Int) (struct {
	C1 *big.Int
	C2 *big.Int
}, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "balanceCommitments", arg0, arg1)

	outstruct := new(struct {
		C1 *big.Int
		C2 *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.C1 = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.C2 = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// BalanceCommitments is a free data retrieval call binding the contract method 0xea0d4573.
//
// Solidity: function balanceCommitments(uint256 , uint256 ) view returns(uint256 c1, uint256 c2)
func (_Enygma *EnygmaSession) BalanceCommitments(arg0 *big.Int, arg1 *big.Int) (struct {
	C1 *big.Int
	C2 *big.Int
}, error) {
	return _Enygma.Contract.BalanceCommitments(&_Enygma.CallOpts, arg0, arg1)
}

// BalanceCommitments is a free data retrieval call binding the contract method 0xea0d4573.
//
// Solidity: function balanceCommitments(uint256 , uint256 ) view returns(uint256 c1, uint256 c2)
func (_Enygma *EnygmaCallerSession) BalanceCommitments(arg0 *big.Int, arg1 *big.Int) (struct {
	C1 *big.Int
	C2 *big.Int
}, error) {
	return _Enygma.Contract.BalanceCommitments(&_Enygma.CallOpts, arg0, arg1)
}

// Check is a free data retrieval call binding the contract method 0x919840ad.
//
// Solidity: function check() view returns(bool)
func (_Enygma *EnygmaCaller) Check(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "check")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// Check is a free data retrieval call binding the contract method 0x919840ad.
//
// Solidity: function check() view returns(bool)
func (_Enygma *EnygmaSession) Check() (bool, error) {
	return _Enygma.Contract.Check(&_Enygma.CallOpts)
}

// Check is a free data retrieval call binding the contract method 0x919840ad.
//
// Solidity: function check() view returns(bool)
func (_Enygma *EnygmaCallerSession) Check() (bool, error) {
	return _Enygma.Contract.Check(&_Enygma.CallOpts)
}

// CheckUsdr is a free data retrieval call binding the contract method 0x52333894.
//
// Solidity: function checkUsdr() view returns(bool)
func (_Enygma *EnygmaCaller) CheckUsdr(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "checkUsdr")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// CheckUsdr is a free data retrieval call binding the contract method 0x52333894.
//
// Solidity: function checkUsdr() view returns(bool)
func (_Enygma *EnygmaSession) CheckUsdr() (bool, error) {
	return _Enygma.Contract.CheckUsdr(&_Enygma.CallOpts)
}

// CheckUsdr is a free data retrieval call binding the contract method 0x52333894.
//
// Solidity: function checkUsdr() view returns(bool)
func (_Enygma *EnygmaCallerSession) CheckUsdr() (bool, error) {
	return _Enygma.Contract.CheckUsdr(&_Enygma.CallOpts)
}

// ConfirmedFingerprint is a free data retrieval call binding the contract method 0x8f48f7b5.
//
// Solidity: function confirmedFingerprint(uint256 , uint256 ) view returns(uint256)
func (_Enygma *EnygmaCaller) ConfirmedFingerprint(opts *bind.CallOpts, arg0 *big.Int, arg1 *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "confirmedFingerprint", arg0, arg1)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ConfirmedFingerprint is a free data retrieval call binding the contract method 0x8f48f7b5.
//
// Solidity: function confirmedFingerprint(uint256 , uint256 ) view returns(uint256)
func (_Enygma *EnygmaSession) ConfirmedFingerprint(arg0 *big.Int, arg1 *big.Int) (*big.Int, error) {
	return _Enygma.Contract.ConfirmedFingerprint(&_Enygma.CallOpts, arg0, arg1)
}

// ConfirmedFingerprint is a free data retrieval call binding the contract method 0x8f48f7b5.
//
// Solidity: function confirmedFingerprint(uint256 , uint256 ) view returns(uint256)
func (_Enygma *EnygmaCallerSession) ConfirmedFingerprint(arg0 *big.Int, arg1 *big.Int) (*big.Int, error) {
	return _Enygma.Contract.ConfirmedFingerprint(&_Enygma.CallOpts, arg0, arg1)
}

// DerivePk is a free data retrieval call binding the contract method 0x723dbbc4.
//
// Solidity: function derivePk(uint256 value) view returns(uint256 x, uint256 y)
func (_Enygma *EnygmaCaller) DerivePk(opts *bind.CallOpts, value *big.Int) (struct {
	X *big.Int
	Y *big.Int
}, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "derivePk", value)

	outstruct := new(struct {
		X *big.Int
		Y *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.X = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.Y = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// DerivePk is a free data retrieval call binding the contract method 0x723dbbc4.
//
// Solidity: function derivePk(uint256 value) view returns(uint256 x, uint256 y)
func (_Enygma *EnygmaSession) DerivePk(value *big.Int) (struct {
	X *big.Int
	Y *big.Int
}, error) {
	return _Enygma.Contract.DerivePk(&_Enygma.CallOpts, value)
}

// DerivePk is a free data retrieval call binding the contract method 0x723dbbc4.
//
// Solidity: function derivePk(uint256 value) view returns(uint256 x, uint256 y)
func (_Enygma *EnygmaCallerSession) DerivePk(value *big.Int) (struct {
	X *big.Int
	Y *big.Int
}, error) {
	return _Enygma.Contract.DerivePk(&_Enygma.CallOpts, value)
}

// DerivePkH is a free data retrieval call binding the contract method 0xce630c18.
//
// Solidity: function derivePkH(uint256 randomness) view returns(uint256 x, uint256 y)
func (_Enygma *EnygmaCaller) DerivePkH(opts *bind.CallOpts, randomness *big.Int) (struct {
	X *big.Int
	Y *big.Int
}, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "derivePkH", randomness)

	outstruct := new(struct {
		X *big.Int
		Y *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.X = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.Y = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// DerivePkH is a free data retrieval call binding the contract method 0xce630c18.
//
// Solidity: function derivePkH(uint256 randomness) view returns(uint256 x, uint256 y)
func (_Enygma *EnygmaSession) DerivePkH(randomness *big.Int) (struct {
	X *big.Int
	Y *big.Int
}, error) {
	return _Enygma.Contract.DerivePkH(&_Enygma.CallOpts, randomness)
}

// DerivePkH is a free data retrieval call binding the contract method 0xce630c18.
//
// Solidity: function derivePkH(uint256 randomness) view returns(uint256 x, uint256 y)
func (_Enygma *EnygmaCallerSession) DerivePkH(randomness *big.Int) (struct {
	X *big.Int
	Y *big.Int
}, error) {
	return _Enygma.Contract.DerivePkH(&_Enygma.CallOpts, randomness)
}

// EpochInterval is a free data retrieval call binding the contract method 0x09b1ef26.
//
// Solidity: function epochInterval() view returns(uint256)
func (_Enygma *EnygmaCaller) EpochInterval(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "epochInterval")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// EpochInterval is a free data retrieval call binding the contract method 0x09b1ef26.
//
// Solidity: function epochInterval() view returns(uint256)
func (_Enygma *EnygmaSession) EpochInterval() (*big.Int, error) {
	return _Enygma.Contract.EpochInterval(&_Enygma.CallOpts)
}

// EpochInterval is a free data retrieval call binding the contract method 0x09b1ef26.
//
// Solidity: function epochInterval() view returns(uint256)
func (_Enygma *EnygmaCallerSession) EpochInterval() (*big.Int, error) {
	return _Enygma.Contract.EpochInterval(&_Enygma.CallOpts)
}

// FingerprintConfirmed is a free data retrieval call binding the contract method 0x6da4d2b8.
//
// Solidity: function fingerprintConfirmed(uint256 , uint256 ) view returns(bool)
func (_Enygma *EnygmaCaller) FingerprintConfirmed(opts *bind.CallOpts, arg0 *big.Int, arg1 *big.Int) (bool, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "fingerprintConfirmed", arg0, arg1)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// FingerprintConfirmed is a free data retrieval call binding the contract method 0x6da4d2b8.
//
// Solidity: function fingerprintConfirmed(uint256 , uint256 ) view returns(bool)
func (_Enygma *EnygmaSession) FingerprintConfirmed(arg0 *big.Int, arg1 *big.Int) (bool, error) {
	return _Enygma.Contract.FingerprintConfirmed(&_Enygma.CallOpts, arg0, arg1)
}

// FingerprintConfirmed is a free data retrieval call binding the contract method 0x6da4d2b8.
//
// Solidity: function fingerprintConfirmed(uint256 , uint256 ) view returns(bool)
func (_Enygma *EnygmaCallerSession) FingerprintConfirmed(arg0 *big.Int, arg1 *big.Int) (bool, error) {
	return _Enygma.Contract.FingerprintConfirmed(&_Enygma.CallOpts, arg0, arg1)
}

// GetBalance is a free data retrieval call binding the contract method 0x1e010439.
//
// Solidity: function getBalance(uint256 accountId) view returns(uint256 x, uint256 y)
func (_Enygma *EnygmaCaller) GetBalance(opts *bind.CallOpts, accountId *big.Int) (struct {
	X *big.Int
	Y *big.Int
}, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "getBalance", accountId)

	outstruct := new(struct {
		X *big.Int
		Y *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.X = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.Y = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// GetBalance is a free data retrieval call binding the contract method 0x1e010439.
//
// Solidity: function getBalance(uint256 accountId) view returns(uint256 x, uint256 y)
func (_Enygma *EnygmaSession) GetBalance(accountId *big.Int) (struct {
	X *big.Int
	Y *big.Int
}, error) {
	return _Enygma.Contract.GetBalance(&_Enygma.CallOpts, accountId)
}

// GetBalance is a free data retrieval call binding the contract method 0x1e010439.
//
// Solidity: function getBalance(uint256 accountId) view returns(uint256 x, uint256 y)
func (_Enygma *EnygmaCallerSession) GetBalance(accountId *big.Int) (struct {
	X *big.Int
	Y *big.Int
}, error) {
	return _Enygma.Contract.GetBalance(&_Enygma.CallOpts, accountId)
}

// GetPublicValues is a free data retrieval call binding the contract method 0xa9c58a7e.
//
// Solidity: function getPublicValues(uint256 count) view returns((uint256,uint256)[] balances, uint256[] keys)
func (_Enygma *EnygmaCaller) GetPublicValues(opts *bind.CallOpts, count *big.Int) (struct {
	Balances []IEnygmaPoint
	Keys     []*big.Int
}, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "getPublicValues", count)

	outstruct := new(struct {
		Balances []IEnygmaPoint
		Keys     []*big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Balances = *abi.ConvertType(out[0], new([]IEnygmaPoint)).(*[]IEnygmaPoint)
	outstruct.Keys = *abi.ConvertType(out[1], new([]*big.Int)).(*[]*big.Int)

	return *outstruct, err

}

// GetPublicValues is a free data retrieval call binding the contract method 0xa9c58a7e.
//
// Solidity: function getPublicValues(uint256 count) view returns((uint256,uint256)[] balances, uint256[] keys)
func (_Enygma *EnygmaSession) GetPublicValues(count *big.Int) (struct {
	Balances []IEnygmaPoint
	Keys     []*big.Int
}, error) {
	return _Enygma.Contract.GetPublicValues(&_Enygma.CallOpts, count)
}

// GetPublicValues is a free data retrieval call binding the contract method 0xa9c58a7e.
//
// Solidity: function getPublicValues(uint256 count) view returns((uint256,uint256)[] balances, uint256[] keys)
func (_Enygma *EnygmaCallerSession) GetPublicValues(count *big.Int) (struct {
	Balances []IEnygmaPoint
	Keys     []*big.Int
}, error) {
	return _Enygma.Contract.GetPublicValues(&_Enygma.CallOpts, count)
}

// GetUsdrBalance is a free data retrieval call binding the contract method 0x9edd41ff.
//
// Solidity: function getUsdrBalance(uint256 accountId) view returns(uint256 x, uint256 y)
func (_Enygma *EnygmaCaller) GetUsdrBalance(opts *bind.CallOpts, accountId *big.Int) (struct {
	X *big.Int
	Y *big.Int
}, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "getUsdrBalance", accountId)

	outstruct := new(struct {
		X *big.Int
		Y *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.X = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.Y = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// GetUsdrBalance is a free data retrieval call binding the contract method 0x9edd41ff.
//
// Solidity: function getUsdrBalance(uint256 accountId) view returns(uint256 x, uint256 y)
func (_Enygma *EnygmaSession) GetUsdrBalance(accountId *big.Int) (struct {
	X *big.Int
	Y *big.Int
}, error) {
	return _Enygma.Contract.GetUsdrBalance(&_Enygma.CallOpts, accountId)
}

// GetUsdrBalance is a free data retrieval call binding the contract method 0x9edd41ff.
//
// Solidity: function getUsdrBalance(uint256 accountId) view returns(uint256 x, uint256 y)
func (_Enygma *EnygmaCallerSession) GetUsdrBalance(accountId *big.Int) (struct {
	X *big.Int
	Y *big.Int
}, error) {
	return _Enygma.Contract.GetUsdrBalance(&_Enygma.CallOpts, accountId)
}

// GetUsdrPublicValues is a free data retrieval call binding the contract method 0x67ec4693.
//
// Solidity: function getUsdrPublicValues(uint256 count) view returns((uint256,uint256)[] balances, uint256[] keys)
func (_Enygma *EnygmaCaller) GetUsdrPublicValues(opts *bind.CallOpts, count *big.Int) (struct {
	Balances []IEnygmaPoint
	Keys     []*big.Int
}, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "getUsdrPublicValues", count)

	outstruct := new(struct {
		Balances []IEnygmaPoint
		Keys     []*big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Balances = *abi.ConvertType(out[0], new([]IEnygmaPoint)).(*[]IEnygmaPoint)
	outstruct.Keys = *abi.ConvertType(out[1], new([]*big.Int)).(*[]*big.Int)

	return *outstruct, err

}

// GetUsdrPublicValues is a free data retrieval call binding the contract method 0x67ec4693.
//
// Solidity: function getUsdrPublicValues(uint256 count) view returns((uint256,uint256)[] balances, uint256[] keys)
func (_Enygma *EnygmaSession) GetUsdrPublicValues(count *big.Int) (struct {
	Balances []IEnygmaPoint
	Keys     []*big.Int
}, error) {
	return _Enygma.Contract.GetUsdrPublicValues(&_Enygma.CallOpts, count)
}

// GetUsdrPublicValues is a free data retrieval call binding the contract method 0x67ec4693.
//
// Solidity: function getUsdrPublicValues(uint256 count) view returns((uint256,uint256)[] balances, uint256[] keys)
func (_Enygma *EnygmaCallerSession) GetUsdrPublicValues(count *big.Int) (struct {
	Balances []IEnygmaPoint
	Keys     []*big.Int
}, error) {
	return _Enygma.Contract.GetUsdrPublicValues(&_Enygma.CallOpts, count)
}

// LastBlockNum is a free data retrieval call binding the contract method 0x36899042.
//
// Solidity: function lastBlockNum() view returns(uint256)
func (_Enygma *EnygmaCaller) LastBlockNum(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "lastBlockNum")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// LastBlockNum is a free data retrieval call binding the contract method 0x36899042.
//
// Solidity: function lastBlockNum() view returns(uint256)
func (_Enygma *EnygmaSession) LastBlockNum() (*big.Int, error) {
	return _Enygma.Contract.LastBlockNum(&_Enygma.CallOpts)
}

// LastBlockNum is a free data retrieval call binding the contract method 0x36899042.
//
// Solidity: function lastBlockNum() view returns(uint256)
func (_Enygma *EnygmaCallerSession) LastBlockNum() (*big.Int, error) {
	return _Enygma.Contract.LastBlockNum(&_Enygma.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Enygma *EnygmaCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Enygma *EnygmaSession) Owner() (common.Address, error) {
	return _Enygma.Contract.Owner(&_Enygma.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Enygma *EnygmaCallerSession) Owner() (common.Address, error) {
	return _Enygma.Contract.Owner(&_Enygma.CallOpts)
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() view returns(bool)
func (_Enygma *EnygmaCaller) Paused(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "paused")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() view returns(bool)
func (_Enygma *EnygmaSession) Paused() (bool, error) {
	return _Enygma.Contract.Paused(&_Enygma.CallOpts)
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() view returns(bool)
func (_Enygma *EnygmaCallerSession) Paused() (bool, error) {
	return _Enygma.Contract.Paused(&_Enygma.CallOpts)
}

// PedCom is a free data retrieval call binding the contract method 0x7d894a16.
//
// Solidity: function pedCom(uint256 value, uint256 randomness) view returns(uint256, uint256)
func (_Enygma *EnygmaCaller) PedCom(opts *bind.CallOpts, value *big.Int, randomness *big.Int) (*big.Int, *big.Int, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "pedCom", value, randomness)

	if err != nil {
		return *new(*big.Int), *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	out1 := *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)

	return out0, out1, err

}

// PedCom is a free data retrieval call binding the contract method 0x7d894a16.
//
// Solidity: function pedCom(uint256 value, uint256 randomness) view returns(uint256, uint256)
func (_Enygma *EnygmaSession) PedCom(value *big.Int, randomness *big.Int) (*big.Int, *big.Int, error) {
	return _Enygma.Contract.PedCom(&_Enygma.CallOpts, value, randomness)
}

// PedCom is a free data retrieval call binding the contract method 0x7d894a16.
//
// Solidity: function pedCom(uint256 value, uint256 randomness) view returns(uint256, uint256)
func (_Enygma *EnygmaCallerSession) PedCom(value *big.Int, randomness *big.Int) (*big.Int, *big.Int, error) {
	return _Enygma.Contract.PedCom(&_Enygma.CallOpts, value, randomness)
}

// PendingFingerprint is a free data retrieval call binding the contract method 0xa605841c.
//
// Solidity: function pendingFingerprint(uint256 , uint256 ) view returns(uint256)
func (_Enygma *EnygmaCaller) PendingFingerprint(opts *bind.CallOpts, arg0 *big.Int, arg1 *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "pendingFingerprint", arg0, arg1)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// PendingFingerprint is a free data retrieval call binding the contract method 0xa605841c.
//
// Solidity: function pendingFingerprint(uint256 , uint256 ) view returns(uint256)
func (_Enygma *EnygmaSession) PendingFingerprint(arg0 *big.Int, arg1 *big.Int) (*big.Int, error) {
	return _Enygma.Contract.PendingFingerprint(&_Enygma.CallOpts, arg0, arg1)
}

// PendingFingerprint is a free data retrieval call binding the contract method 0xa605841c.
//
// Solidity: function pendingFingerprint(uint256 , uint256 ) view returns(uint256)
func (_Enygma *EnygmaCallerSession) PendingFingerprint(arg0 *big.Int, arg1 *big.Int) (*big.Int, error) {
	return _Enygma.Contract.PendingFingerprint(&_Enygma.CallOpts, arg0, arg1)
}

// PendingOwner is a free data retrieval call binding the contract method 0xe30c3978.
//
// Solidity: function pendingOwner() view returns(address)
func (_Enygma *EnygmaCaller) PendingOwner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "pendingOwner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// PendingOwner is a free data retrieval call binding the contract method 0xe30c3978.
//
// Solidity: function pendingOwner() view returns(address)
func (_Enygma *EnygmaSession) PendingOwner() (common.Address, error) {
	return _Enygma.Contract.PendingOwner(&_Enygma.CallOpts)
}

// PendingOwner is a free data retrieval call binding the contract method 0xe30c3978.
//
// Solidity: function pendingOwner() view returns(address)
func (_Enygma *EnygmaCallerSession) PendingOwner() (common.Address, error) {
	return _Enygma.Contract.PendingOwner(&_Enygma.CallOpts)
}

// ProtocolFee is a free data retrieval call binding the contract method 0xb0e21e8a.
//
// Solidity: function protocolFee() view returns(uint256)
func (_Enygma *EnygmaCaller) ProtocolFee(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "protocolFee")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ProtocolFee is a free data retrieval call binding the contract method 0xb0e21e8a.
//
// Solidity: function protocolFee() view returns(uint256)
func (_Enygma *EnygmaSession) ProtocolFee() (*big.Int, error) {
	return _Enygma.Contract.ProtocolFee(&_Enygma.CallOpts)
}

// ProtocolFee is a free data retrieval call binding the contract method 0xb0e21e8a.
//
// Solidity: function protocolFee() view returns(uint256)
func (_Enygma *EnygmaCallerSession) ProtocolFee() (*big.Int, error) {
	return _Enygma.Contract.ProtocolFee(&_Enygma.CallOpts)
}

// PublicKeys is a free data retrieval call binding the contract method 0xc680f410.
//
// Solidity: function publicKeys(uint256 ) view returns(uint256)
func (_Enygma *EnygmaCaller) PublicKeys(opts *bind.CallOpts, arg0 *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "publicKeys", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// PublicKeys is a free data retrieval call binding the contract method 0xc680f410.
//
// Solidity: function publicKeys(uint256 ) view returns(uint256)
func (_Enygma *EnygmaSession) PublicKeys(arg0 *big.Int) (*big.Int, error) {
	return _Enygma.Contract.PublicKeys(&_Enygma.CallOpts, arg0)
}

// PublicKeys is a free data retrieval call binding the contract method 0xc680f410.
//
// Solidity: function publicKeys(uint256 ) view returns(uint256)
func (_Enygma *EnygmaCallerSession) PublicKeys(arg0 *big.Int) (*big.Int, error) {
	return _Enygma.Contract.PublicKeys(&_Enygma.CallOpts, arg0)
}

// TotalSupplyAmount is a free data retrieval call binding the contract method 0xf828f50b.
//
// Solidity: function totalSupplyAmount() view returns(uint256)
func (_Enygma *EnygmaCaller) TotalSupplyAmount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "totalSupplyAmount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TotalSupplyAmount is a free data retrieval call binding the contract method 0xf828f50b.
//
// Solidity: function totalSupplyAmount() view returns(uint256)
func (_Enygma *EnygmaSession) TotalSupplyAmount() (*big.Int, error) {
	return _Enygma.Contract.TotalSupplyAmount(&_Enygma.CallOpts)
}

// TotalSupplyAmount is a free data retrieval call binding the contract method 0xf828f50b.
//
// Solidity: function totalSupplyAmount() view returns(uint256)
func (_Enygma *EnygmaCallerSession) TotalSupplyAmount() (*big.Int, error) {
	return _Enygma.Contract.TotalSupplyAmount(&_Enygma.CallOpts)
}

// TotalSupplyX is a free data retrieval call binding the contract method 0x71929e2a.
//
// Solidity: function totalSupplyX() view returns(uint256)
func (_Enygma *EnygmaCaller) TotalSupplyX(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "totalSupplyX")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TotalSupplyX is a free data retrieval call binding the contract method 0x71929e2a.
//
// Solidity: function totalSupplyX() view returns(uint256)
func (_Enygma *EnygmaSession) TotalSupplyX() (*big.Int, error) {
	return _Enygma.Contract.TotalSupplyX(&_Enygma.CallOpts)
}

// TotalSupplyX is a free data retrieval call binding the contract method 0x71929e2a.
//
// Solidity: function totalSupplyX() view returns(uint256)
func (_Enygma *EnygmaCallerSession) TotalSupplyX() (*big.Int, error) {
	return _Enygma.Contract.TotalSupplyX(&_Enygma.CallOpts)
}

// TotalSupplyY is a free data retrieval call binding the contract method 0x67511a4d.
//
// Solidity: function totalSupplyY() view returns(uint256)
func (_Enygma *EnygmaCaller) TotalSupplyY(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "totalSupplyY")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TotalSupplyY is a free data retrieval call binding the contract method 0x67511a4d.
//
// Solidity: function totalSupplyY() view returns(uint256)
func (_Enygma *EnygmaSession) TotalSupplyY() (*big.Int, error) {
	return _Enygma.Contract.TotalSupplyY(&_Enygma.CallOpts)
}

// TotalSupplyY is a free data retrieval call binding the contract method 0x67511a4d.
//
// Solidity: function totalSupplyY() view returns(uint256)
func (_Enygma *EnygmaCallerSession) TotalSupplyY() (*big.Int, error) {
	return _Enygma.Contract.TotalSupplyY(&_Enygma.CallOpts)
}

// UsdrBalanceCommitments is a free data retrieval call binding the contract method 0xce8cd400.
//
// Solidity: function usdrBalanceCommitments(uint256 , uint256 ) view returns(uint256 c1, uint256 c2)
func (_Enygma *EnygmaCaller) UsdrBalanceCommitments(opts *bind.CallOpts, arg0 *big.Int, arg1 *big.Int) (struct {
	C1 *big.Int
	C2 *big.Int
}, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "usdrBalanceCommitments", arg0, arg1)

	outstruct := new(struct {
		C1 *big.Int
		C2 *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.C1 = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.C2 = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// UsdrBalanceCommitments is a free data retrieval call binding the contract method 0xce8cd400.
//
// Solidity: function usdrBalanceCommitments(uint256 , uint256 ) view returns(uint256 c1, uint256 c2)
func (_Enygma *EnygmaSession) UsdrBalanceCommitments(arg0 *big.Int, arg1 *big.Int) (struct {
	C1 *big.Int
	C2 *big.Int
}, error) {
	return _Enygma.Contract.UsdrBalanceCommitments(&_Enygma.CallOpts, arg0, arg1)
}

// UsdrBalanceCommitments is a free data retrieval call binding the contract method 0xce8cd400.
//
// Solidity: function usdrBalanceCommitments(uint256 , uint256 ) view returns(uint256 c1, uint256 c2)
func (_Enygma *EnygmaCallerSession) UsdrBalanceCommitments(arg0 *big.Int, arg1 *big.Int) (struct {
	C1 *big.Int
	C2 *big.Int
}, error) {
	return _Enygma.Contract.UsdrBalanceCommitments(&_Enygma.CallOpts, arg0, arg1)
}

// UsdrFixedFeeAmount is a free data retrieval call binding the contract method 0x25e74eb3.
//
// Solidity: function usdrFixedFeeAmount() view returns(uint256)
func (_Enygma *EnygmaCaller) UsdrFixedFeeAmount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "usdrFixedFeeAmount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// UsdrFixedFeeAmount is a free data retrieval call binding the contract method 0x25e74eb3.
//
// Solidity: function usdrFixedFeeAmount() view returns(uint256)
func (_Enygma *EnygmaSession) UsdrFixedFeeAmount() (*big.Int, error) {
	return _Enygma.Contract.UsdrFixedFeeAmount(&_Enygma.CallOpts)
}

// UsdrFixedFeeAmount is a free data retrieval call binding the contract method 0x25e74eb3.
//
// Solidity: function usdrFixedFeeAmount() view returns(uint256)
func (_Enygma *EnygmaCallerSession) UsdrFixedFeeAmount() (*big.Int, error) {
	return _Enygma.Contract.UsdrFixedFeeAmount(&_Enygma.CallOpts)
}

// UsdrTotalSupplyAmount is a free data retrieval call binding the contract method 0x8d909dd7.
//
// Solidity: function usdrTotalSupplyAmount() view returns(uint256)
func (_Enygma *EnygmaCaller) UsdrTotalSupplyAmount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "usdrTotalSupplyAmount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// UsdrTotalSupplyAmount is a free data retrieval call binding the contract method 0x8d909dd7.
//
// Solidity: function usdrTotalSupplyAmount() view returns(uint256)
func (_Enygma *EnygmaSession) UsdrTotalSupplyAmount() (*big.Int, error) {
	return _Enygma.Contract.UsdrTotalSupplyAmount(&_Enygma.CallOpts)
}

// UsdrTotalSupplyAmount is a free data retrieval call binding the contract method 0x8d909dd7.
//
// Solidity: function usdrTotalSupplyAmount() view returns(uint256)
func (_Enygma *EnygmaCallerSession) UsdrTotalSupplyAmount() (*big.Int, error) {
	return _Enygma.Contract.UsdrTotalSupplyAmount(&_Enygma.CallOpts)
}

// UsdrTotalSupplyX is a free data retrieval call binding the contract method 0xe52dc188.
//
// Solidity: function usdrTotalSupplyX() view returns(uint256)
func (_Enygma *EnygmaCaller) UsdrTotalSupplyX(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "usdrTotalSupplyX")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// UsdrTotalSupplyX is a free data retrieval call binding the contract method 0xe52dc188.
//
// Solidity: function usdrTotalSupplyX() view returns(uint256)
func (_Enygma *EnygmaSession) UsdrTotalSupplyX() (*big.Int, error) {
	return _Enygma.Contract.UsdrTotalSupplyX(&_Enygma.CallOpts)
}

// UsdrTotalSupplyX is a free data retrieval call binding the contract method 0xe52dc188.
//
// Solidity: function usdrTotalSupplyX() view returns(uint256)
func (_Enygma *EnygmaCallerSession) UsdrTotalSupplyX() (*big.Int, error) {
	return _Enygma.Contract.UsdrTotalSupplyX(&_Enygma.CallOpts)
}

// UsdrTotalSupplyY is a free data retrieval call binding the contract method 0x272fe53c.
//
// Solidity: function usdrTotalSupplyY() view returns(uint256)
func (_Enygma *EnygmaCaller) UsdrTotalSupplyY(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "usdrTotalSupplyY")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// UsdrTotalSupplyY is a free data retrieval call binding the contract method 0x272fe53c.
//
// Solidity: function usdrTotalSupplyY() view returns(uint256)
func (_Enygma *EnygmaSession) UsdrTotalSupplyY() (*big.Int, error) {
	return _Enygma.Contract.UsdrTotalSupplyY(&_Enygma.CallOpts)
}

// UsdrTotalSupplyY is a free data retrieval call binding the contract method 0x272fe53c.
//
// Solidity: function usdrTotalSupplyY() view returns(uint256)
func (_Enygma *EnygmaCallerSession) UsdrTotalSupplyY() (*big.Int, error) {
	return _Enygma.Contract.UsdrTotalSupplyY(&_Enygma.CallOpts)
}

// ViewKeys is a free data retrieval call binding the contract method 0x0cf1839c.
//
// Solidity: function viewKeys(uint256 ) view returns(bytes)
func (_Enygma *EnygmaCaller) ViewKeys(opts *bind.CallOpts, arg0 *big.Int) ([]byte, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "viewKeys", arg0)

	if err != nil {
		return *new([]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([]byte)).(*[]byte)

	return out0, err

}

// ViewKeys is a free data retrieval call binding the contract method 0x0cf1839c.
//
// Solidity: function viewKeys(uint256 ) view returns(bytes)
func (_Enygma *EnygmaSession) ViewKeys(arg0 *big.Int) ([]byte, error) {
	return _Enygma.Contract.ViewKeys(&_Enygma.CallOpts, arg0)
}

// ViewKeys is a free data retrieval call binding the contract method 0x0cf1839c.
//
// Solidity: function viewKeys(uint256 ) view returns(bytes)
func (_Enygma *EnygmaCallerSession) ViewKeys(arg0 *big.Int) ([]byte, error) {
	return _Enygma.Contract.ViewKeys(&_Enygma.CallOpts, arg0)
}

// AcceptOwnership is a paid mutator transaction binding the contract method 0x79ba5097.
//
// Solidity: function acceptOwnership() returns(bool)
func (_Enygma *EnygmaTransactor) AcceptOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "acceptOwnership")
}

// AcceptOwnership is a paid mutator transaction binding the contract method 0x79ba5097.
//
// Solidity: function acceptOwnership() returns(bool)
func (_Enygma *EnygmaSession) AcceptOwnership() (*types.Transaction, error) {
	return _Enygma.Contract.AcceptOwnership(&_Enygma.TransactOpts)
}

// AcceptOwnership is a paid mutator transaction binding the contract method 0x79ba5097.
//
// Solidity: function acceptOwnership() returns(bool)
func (_Enygma *EnygmaTransactorSession) AcceptOwnership() (*types.Transaction, error) {
	return _Enygma.Contract.AcceptOwnership(&_Enygma.TransactOpts)
}

// AddBurnVerifier is a paid mutator transaction binding the contract method 0xedda4a0a.
//
// Solidity: function addBurnVerifier(address verifier) returns(bool)
func (_Enygma *EnygmaTransactor) AddBurnVerifier(opts *bind.TransactOpts, verifier common.Address) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "addBurnVerifier", verifier)
}

// AddBurnVerifier is a paid mutator transaction binding the contract method 0xedda4a0a.
//
// Solidity: function addBurnVerifier(address verifier) returns(bool)
func (_Enygma *EnygmaSession) AddBurnVerifier(verifier common.Address) (*types.Transaction, error) {
	return _Enygma.Contract.AddBurnVerifier(&_Enygma.TransactOpts, verifier)
}

// AddBurnVerifier is a paid mutator transaction binding the contract method 0xedda4a0a.
//
// Solidity: function addBurnVerifier(address verifier) returns(bool)
func (_Enygma *EnygmaTransactorSession) AddBurnVerifier(verifier common.Address) (*types.Transaction, error) {
	return _Enygma.Contract.AddBurnVerifier(&_Enygma.TransactOpts, verifier)
}

// AddDepositVerifier is a paid mutator transaction binding the contract method 0x0197d942.
//
// Solidity: function addDepositVerifier(address verifier) returns(bool)
func (_Enygma *EnygmaTransactor) AddDepositVerifier(opts *bind.TransactOpts, verifier common.Address) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "addDepositVerifier", verifier)
}

// AddDepositVerifier is a paid mutator transaction binding the contract method 0x0197d942.
//
// Solidity: function addDepositVerifier(address verifier) returns(bool)
func (_Enygma *EnygmaSession) AddDepositVerifier(verifier common.Address) (*types.Transaction, error) {
	return _Enygma.Contract.AddDepositVerifier(&_Enygma.TransactOpts, verifier)
}

// AddDepositVerifier is a paid mutator transaction binding the contract method 0x0197d942.
//
// Solidity: function addDepositVerifier(address verifier) returns(bool)
func (_Enygma *EnygmaTransactorSession) AddDepositVerifier(verifier common.Address) (*types.Transaction, error) {
	return _Enygma.Contract.AddDepositVerifier(&_Enygma.TransactOpts, verifier)
}

// AddFeeVerifier is a paid mutator transaction binding the contract method 0x4e466c53.
//
// Solidity: function addFeeVerifier(address verifier) returns(bool)
func (_Enygma *EnygmaTransactor) AddFeeVerifier(opts *bind.TransactOpts, verifier common.Address) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "addFeeVerifier", verifier)
}

// AddFeeVerifier is a paid mutator transaction binding the contract method 0x4e466c53.
//
// Solidity: function addFeeVerifier(address verifier) returns(bool)
func (_Enygma *EnygmaSession) AddFeeVerifier(verifier common.Address) (*types.Transaction, error) {
	return _Enygma.Contract.AddFeeVerifier(&_Enygma.TransactOpts, verifier)
}

// AddFeeVerifier is a paid mutator transaction binding the contract method 0x4e466c53.
//
// Solidity: function addFeeVerifier(address verifier) returns(bool)
func (_Enygma *EnygmaTransactorSession) AddFeeVerifier(verifier common.Address) (*types.Transaction, error) {
	return _Enygma.Contract.AddFeeVerifier(&_Enygma.TransactOpts, verifier)
}

// AddUsdrVerifier is a paid mutator transaction binding the contract method 0xec4a09d0.
//
// Solidity: function addUsdrVerifier(address verifier) returns(bool)
func (_Enygma *EnygmaTransactor) AddUsdrVerifier(opts *bind.TransactOpts, verifier common.Address) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "addUsdrVerifier", verifier)
}

// AddUsdrVerifier is a paid mutator transaction binding the contract method 0xec4a09d0.
//
// Solidity: function addUsdrVerifier(address verifier) returns(bool)
func (_Enygma *EnygmaSession) AddUsdrVerifier(verifier common.Address) (*types.Transaction, error) {
	return _Enygma.Contract.AddUsdrVerifier(&_Enygma.TransactOpts, verifier)
}

// AddUsdrVerifier is a paid mutator transaction binding the contract method 0xec4a09d0.
//
// Solidity: function addUsdrVerifier(address verifier) returns(bool)
func (_Enygma *EnygmaTransactorSession) AddUsdrVerifier(verifier common.Address) (*types.Transaction, error) {
	return _Enygma.Contract.AddUsdrVerifier(&_Enygma.TransactOpts, verifier)
}

// AddVerifier is a paid mutator transaction binding the contract method 0x9000b3d6.
//
// Solidity: function addVerifier(address verifier) returns(bool)
func (_Enygma *EnygmaTransactor) AddVerifier(opts *bind.TransactOpts, verifier common.Address) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "addVerifier", verifier)
}

// AddVerifier is a paid mutator transaction binding the contract method 0x9000b3d6.
//
// Solidity: function addVerifier(address verifier) returns(bool)
func (_Enygma *EnygmaSession) AddVerifier(verifier common.Address) (*types.Transaction, error) {
	return _Enygma.Contract.AddVerifier(&_Enygma.TransactOpts, verifier)
}

// AddVerifier is a paid mutator transaction binding the contract method 0x9000b3d6.
//
// Solidity: function addVerifier(address verifier) returns(bool)
func (_Enygma *EnygmaTransactorSession) AddVerifier(verifier common.Address) (*types.Transaction, error) {
	return _Enygma.Contract.AddVerifier(&_Enygma.TransactOpts, verifier)
}

// AddWithdrawVerifier is a paid mutator transaction binding the contract method 0xfe877fc9.
//
// Solidity: function addWithdrawVerifier(address verifier, uint256 splitCount) returns(bool)
func (_Enygma *EnygmaTransactor) AddWithdrawVerifier(opts *bind.TransactOpts, verifier common.Address, splitCount *big.Int) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "addWithdrawVerifier", verifier, splitCount)
}

// AddWithdrawVerifier is a paid mutator transaction binding the contract method 0xfe877fc9.
//
// Solidity: function addWithdrawVerifier(address verifier, uint256 splitCount) returns(bool)
func (_Enygma *EnygmaSession) AddWithdrawVerifier(verifier common.Address, splitCount *big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.AddWithdrawVerifier(&_Enygma.TransactOpts, verifier, splitCount)
}

// AddWithdrawVerifier is a paid mutator transaction binding the contract method 0xfe877fc9.
//
// Solidity: function addWithdrawVerifier(address verifier, uint256 splitCount) returns(bool)
func (_Enygma *EnygmaTransactorSession) AddWithdrawVerifier(verifier common.Address, splitCount *big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.AddWithdrawVerifier(&_Enygma.TransactOpts, verifier, splitCount)
}

// AddZkDvp is a paid mutator transaction binding the contract method 0xf8344434.
//
// Solidity: function addZkDvp(address zkDvp) returns(bool)
func (_Enygma *EnygmaTransactor) AddZkDvp(opts *bind.TransactOpts, zkDvp common.Address) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "addZkDvp", zkDvp)
}

// AddZkDvp is a paid mutator transaction binding the contract method 0xf8344434.
//
// Solidity: function addZkDvp(address zkDvp) returns(bool)
func (_Enygma *EnygmaSession) AddZkDvp(zkDvp common.Address) (*types.Transaction, error) {
	return _Enygma.Contract.AddZkDvp(&_Enygma.TransactOpts, zkDvp)
}

// AddZkDvp is a paid mutator transaction binding the contract method 0xf8344434.
//
// Solidity: function addZkDvp(address zkDvp) returns(bool)
func (_Enygma *EnygmaTransactorSession) AddZkDvp(zkDvp common.Address) (*types.Transaction, error) {
	return _Enygma.Contract.AddZkDvp(&_Enygma.TransactOpts, zkDvp)
}

// Burn is a paid mutator transaction binding the contract method 0x5fbaf841.
//
// Solidity: function burn(uint256 accountId, (uint256[8],uint256[9]) proof) returns(bool)
func (_Enygma *EnygmaTransactor) Burn(opts *bind.TransactOpts, accountId *big.Int, proof IEnygmaBurnProof) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "burn", accountId, proof)
}

// Burn is a paid mutator transaction binding the contract method 0x5fbaf841.
//
// Solidity: function burn(uint256 accountId, (uint256[8],uint256[9]) proof) returns(bool)
func (_Enygma *EnygmaSession) Burn(accountId *big.Int, proof IEnygmaBurnProof) (*types.Transaction, error) {
	return _Enygma.Contract.Burn(&_Enygma.TransactOpts, accountId, proof)
}

// Burn is a paid mutator transaction binding the contract method 0x5fbaf841.
//
// Solidity: function burn(uint256 accountId, (uint256[8],uint256[9]) proof) returns(bool)
func (_Enygma *EnygmaTransactorSession) Burn(accountId *big.Int, proof IEnygmaBurnProof) (*types.Transaction, error) {
	return _Enygma.Contract.Burn(&_Enygma.TransactOpts, accountId, proof)
}

// Deposit is a paid mutator transaction binding the contract method 0x5dcc7650.
//
// Solidity: function deposit((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[52]) proof, ((((uint256,uint256),(uint256[2],uint256[2]),(uint256,uint256)),uint256[],uint256,uint256)) withdrawParam, uint256[] participantIds) returns(bool)
func (_Enygma *EnygmaTransactor) Deposit(opts *bind.TransactOpts, commitmentDeltas []IEnygmaPoint, proof IEnygmaDepositProof, withdrawParam IEnygmaWithdrawParams, participantIds []*big.Int) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "deposit", commitmentDeltas, proof, withdrawParam, participantIds)
}

// Deposit is a paid mutator transaction binding the contract method 0x5dcc7650.
//
// Solidity: function deposit((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[52]) proof, ((((uint256,uint256),(uint256[2],uint256[2]),(uint256,uint256)),uint256[],uint256,uint256)) withdrawParam, uint256[] participantIds) returns(bool)
func (_Enygma *EnygmaSession) Deposit(commitmentDeltas []IEnygmaPoint, proof IEnygmaDepositProof, withdrawParam IEnygmaWithdrawParams, participantIds []*big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.Deposit(&_Enygma.TransactOpts, commitmentDeltas, proof, withdrawParam, participantIds)
}

// Deposit is a paid mutator transaction binding the contract method 0x5dcc7650.
//
// Solidity: function deposit((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[52]) proof, ((((uint256,uint256),(uint256[2],uint256[2]),(uint256,uint256)),uint256[],uint256,uint256)) withdrawParam, uint256[] participantIds) returns(bool)
func (_Enygma *EnygmaTransactorSession) Deposit(commitmentDeltas []IEnygmaPoint, proof IEnygmaDepositProof, withdrawParam IEnygmaWithdrawParams, participantIds []*big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.Deposit(&_Enygma.TransactOpts, commitmentDeltas, proof, withdrawParam, participantIds)
}

// Initialize is a paid mutator transaction binding the contract method 0x8129fc1c.
//
// Solidity: function initialize() returns(bool)
func (_Enygma *EnygmaTransactor) Initialize(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "initialize")
}

// Initialize is a paid mutator transaction binding the contract method 0x8129fc1c.
//
// Solidity: function initialize() returns(bool)
func (_Enygma *EnygmaSession) Initialize() (*types.Transaction, error) {
	return _Enygma.Contract.Initialize(&_Enygma.TransactOpts)
}

// Initialize is a paid mutator transaction binding the contract method 0x8129fc1c.
//
// Solidity: function initialize() returns(bool)
func (_Enygma *EnygmaTransactorSession) Initialize() (*types.Transaction, error) {
	return _Enygma.Contract.Initialize(&_Enygma.TransactOpts)
}

// InitializeUsdrBalance is a paid mutator transaction binding the contract method 0x5f5377e0.
//
// Solidity: function initializeUsdrBalance(uint256 accountId, uint256 randomness) returns(bool)
func (_Enygma *EnygmaTransactor) InitializeUsdrBalance(opts *bind.TransactOpts, accountId *big.Int, randomness *big.Int) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "initializeUsdrBalance", accountId, randomness)
}

// InitializeUsdrBalance is a paid mutator transaction binding the contract method 0x5f5377e0.
//
// Solidity: function initializeUsdrBalance(uint256 accountId, uint256 randomness) returns(bool)
func (_Enygma *EnygmaSession) InitializeUsdrBalance(accountId *big.Int, randomness *big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.InitializeUsdrBalance(&_Enygma.TransactOpts, accountId, randomness)
}

// InitializeUsdrBalance is a paid mutator transaction binding the contract method 0x5f5377e0.
//
// Solidity: function initializeUsdrBalance(uint256 accountId, uint256 randomness) returns(bool)
func (_Enygma *EnygmaTransactorSession) InitializeUsdrBalance(accountId *big.Int, randomness *big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.InitializeUsdrBalance(&_Enygma.TransactOpts, accountId, randomness)
}

// MintSupply is a paid mutator transaction binding the contract method 0x8718dcaa.
//
// Solidity: function mintSupply(uint256 amount, uint256 recipientId, uint256 mintCommitX, uint256 mintCommitY) returns(bool)
func (_Enygma *EnygmaTransactor) MintSupply(opts *bind.TransactOpts, amount *big.Int, recipientId *big.Int, mintCommitX *big.Int, mintCommitY *big.Int) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "mintSupply", amount, recipientId, mintCommitX, mintCommitY)
}

// MintSupply is a paid mutator transaction binding the contract method 0x8718dcaa.
//
// Solidity: function mintSupply(uint256 amount, uint256 recipientId, uint256 mintCommitX, uint256 mintCommitY) returns(bool)
func (_Enygma *EnygmaSession) MintSupply(amount *big.Int, recipientId *big.Int, mintCommitX *big.Int, mintCommitY *big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.MintSupply(&_Enygma.TransactOpts, amount, recipientId, mintCommitX, mintCommitY)
}

// MintSupply is a paid mutator transaction binding the contract method 0x8718dcaa.
//
// Solidity: function mintSupply(uint256 amount, uint256 recipientId, uint256 mintCommitX, uint256 mintCommitY) returns(bool)
func (_Enygma *EnygmaTransactorSession) MintSupply(amount *big.Int, recipientId *big.Int, mintCommitX *big.Int, mintCommitY *big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.MintSupply(&_Enygma.TransactOpts, amount, recipientId, mintCommitX, mintCommitY)
}

// MintUsdrSupply is a paid mutator transaction binding the contract method 0xa61f3ae4.
//
// Solidity: function mintUsdrSupply(uint256 amount, uint256 recipientId) returns(bool)
func (_Enygma *EnygmaTransactor) MintUsdrSupply(opts *bind.TransactOpts, amount *big.Int, recipientId *big.Int) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "mintUsdrSupply", amount, recipientId)
}

// MintUsdrSupply is a paid mutator transaction binding the contract method 0xa61f3ae4.
//
// Solidity: function mintUsdrSupply(uint256 amount, uint256 recipientId) returns(bool)
func (_Enygma *EnygmaSession) MintUsdrSupply(amount *big.Int, recipientId *big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.MintUsdrSupply(&_Enygma.TransactOpts, amount, recipientId)
}

// MintUsdrSupply is a paid mutator transaction binding the contract method 0xa61f3ae4.
//
// Solidity: function mintUsdrSupply(uint256 amount, uint256 recipientId) returns(bool)
func (_Enygma *EnygmaTransactorSession) MintUsdrSupply(amount *big.Int, recipientId *big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.MintUsdrSupply(&_Enygma.TransactOpts, amount, recipientId)
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns(bool)
func (_Enygma *EnygmaTransactor) Pause(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "pause")
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns(bool)
func (_Enygma *EnygmaSession) Pause() (*types.Transaction, error) {
	return _Enygma.Contract.Pause(&_Enygma.TransactOpts)
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns(bool)
func (_Enygma *EnygmaTransactorSession) Pause() (*types.Transaction, error) {
	return _Enygma.Contract.Pause(&_Enygma.TransactOpts)
}

// RegisterAccount is a paid mutator transaction binding the contract method 0x83914157.
//
// Solidity: function registerAccount(address addr, uint256 accountId, uint256 publicKey, uint256 initialCommitX, uint256 initialCommitY, bytes viewKey) returns(bool)
func (_Enygma *EnygmaTransactor) RegisterAccount(opts *bind.TransactOpts, addr common.Address, accountId *big.Int, publicKey *big.Int, initialCommitX *big.Int, initialCommitY *big.Int, viewKey []byte) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "registerAccount", addr, accountId, publicKey, initialCommitX, initialCommitY, viewKey)
}

// RegisterAccount is a paid mutator transaction binding the contract method 0x83914157.
//
// Solidity: function registerAccount(address addr, uint256 accountId, uint256 publicKey, uint256 initialCommitX, uint256 initialCommitY, bytes viewKey) returns(bool)
func (_Enygma *EnygmaSession) RegisterAccount(addr common.Address, accountId *big.Int, publicKey *big.Int, initialCommitX *big.Int, initialCommitY *big.Int, viewKey []byte) (*types.Transaction, error) {
	return _Enygma.Contract.RegisterAccount(&_Enygma.TransactOpts, addr, accountId, publicKey, initialCommitX, initialCommitY, viewKey)
}

// RegisterAccount is a paid mutator transaction binding the contract method 0x83914157.
//
// Solidity: function registerAccount(address addr, uint256 accountId, uint256 publicKey, uint256 initialCommitX, uint256 initialCommitY, bytes viewKey) returns(bool)
func (_Enygma *EnygmaTransactorSession) RegisterAccount(addr common.Address, accountId *big.Int, publicKey *big.Int, initialCommitX *big.Int, initialCommitY *big.Int, viewKey []byte) (*types.Transaction, error) {
	return _Enygma.Contract.RegisterAccount(&_Enygma.TransactOpts, addr, accountId, publicKey, initialCommitX, initialCommitY, viewKey)
}

// RegisterFingerprint is a paid mutator transaction binding the contract method 0x5a54f35f.
//
// Solidity: function registerFingerprint(uint256 otherPartyId, uint256 fingerprint) returns(bool)
func (_Enygma *EnygmaTransactor) RegisterFingerprint(opts *bind.TransactOpts, otherPartyId *big.Int, fingerprint *big.Int) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "registerFingerprint", otherPartyId, fingerprint)
}

// RegisterFingerprint is a paid mutator transaction binding the contract method 0x5a54f35f.
//
// Solidity: function registerFingerprint(uint256 otherPartyId, uint256 fingerprint) returns(bool)
func (_Enygma *EnygmaSession) RegisterFingerprint(otherPartyId *big.Int, fingerprint *big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.RegisterFingerprint(&_Enygma.TransactOpts, otherPartyId, fingerprint)
}

// RegisterFingerprint is a paid mutator transaction binding the contract method 0x5a54f35f.
//
// Solidity: function registerFingerprint(uint256 otherPartyId, uint256 fingerprint) returns(bool)
func (_Enygma *EnygmaTransactorSession) RegisterFingerprint(otherPartyId *big.Int, fingerprint *big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.RegisterFingerprint(&_Enygma.TransactOpts, otherPartyId, fingerprint)
}

// SetProtocolFee is a paid mutator transaction binding the contract method 0x787dce3d.
//
// Solidity: function setProtocolFee(uint256 newFee) returns(bool)
func (_Enygma *EnygmaTransactor) SetProtocolFee(opts *bind.TransactOpts, newFee *big.Int) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "setProtocolFee", newFee)
}

// SetProtocolFee is a paid mutator transaction binding the contract method 0x787dce3d.
//
// Solidity: function setProtocolFee(uint256 newFee) returns(bool)
func (_Enygma *EnygmaSession) SetProtocolFee(newFee *big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.SetProtocolFee(&_Enygma.TransactOpts, newFee)
}

// SetProtocolFee is a paid mutator transaction binding the contract method 0x787dce3d.
//
// Solidity: function setProtocolFee(uint256 newFee) returns(bool)
func (_Enygma *EnygmaTransactorSession) SetProtocolFee(newFee *big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.SetProtocolFee(&_Enygma.TransactOpts, newFee)
}

// SetUsdrFixedFee is a paid mutator transaction binding the contract method 0x1660e58f.
//
// Solidity: function setUsdrFixedFee(uint256 amount) returns(bool)
func (_Enygma *EnygmaTransactor) SetUsdrFixedFee(opts *bind.TransactOpts, amount *big.Int) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "setUsdrFixedFee", amount)
}

// SetUsdrFixedFee is a paid mutator transaction binding the contract method 0x1660e58f.
//
// Solidity: function setUsdrFixedFee(uint256 amount) returns(bool)
func (_Enygma *EnygmaSession) SetUsdrFixedFee(amount *big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.SetUsdrFixedFee(&_Enygma.TransactOpts, amount)
}

// SetUsdrFixedFee is a paid mutator transaction binding the contract method 0x1660e58f.
//
// Solidity: function setUsdrFixedFee(uint256 amount) returns(bool)
func (_Enygma *EnygmaTransactorSession) SetUsdrFixedFee(amount *big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.SetUsdrFixedFee(&_Enygma.TransactOpts, amount)
}

// Transfer is a paid mutator transaction binding the contract method 0x545897c0.
//
// Solidity: function transfer((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[81]) proof, (uint256,uint256)[] usdrCommitmentDeltas, (uint256[8],uint256[81]) usdrProof, uint256[] participantIds, string bankTag) returns(bool)
func (_Enygma *EnygmaTransactor) Transfer(opts *bind.TransactOpts, commitmentDeltas []IEnygmaPoint, proof IEnygmaProof, usdrCommitmentDeltas []IEnygmaPoint, usdrProof IEnygmaUsdrProof, participantIds []*big.Int, bankTag string) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "transfer", commitmentDeltas, proof, usdrCommitmentDeltas, usdrProof, participantIds, bankTag)
}

// Transfer is a paid mutator transaction binding the contract method 0x545897c0.
//
// Solidity: function transfer((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[81]) proof, (uint256,uint256)[] usdrCommitmentDeltas, (uint256[8],uint256[81]) usdrProof, uint256[] participantIds, string bankTag) returns(bool)
func (_Enygma *EnygmaSession) Transfer(commitmentDeltas []IEnygmaPoint, proof IEnygmaProof, usdrCommitmentDeltas []IEnygmaPoint, usdrProof IEnygmaUsdrProof, participantIds []*big.Int, bankTag string) (*types.Transaction, error) {
	return _Enygma.Contract.Transfer(&_Enygma.TransactOpts, commitmentDeltas, proof, usdrCommitmentDeltas, usdrProof, participantIds, bankTag)
}

// Transfer is a paid mutator transaction binding the contract method 0x545897c0.
//
// Solidity: function transfer((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[81]) proof, (uint256,uint256)[] usdrCommitmentDeltas, (uint256[8],uint256[81]) usdrProof, uint256[] participantIds, string bankTag) returns(bool)
func (_Enygma *EnygmaTransactorSession) Transfer(commitmentDeltas []IEnygmaPoint, proof IEnygmaProof, usdrCommitmentDeltas []IEnygmaPoint, usdrProof IEnygmaUsdrProof, participantIds []*big.Int, bankTag string) (*types.Transaction, error) {
	return _Enygma.Contract.Transfer(&_Enygma.TransactOpts, commitmentDeltas, proof, usdrCommitmentDeltas, usdrProof, participantIds, bankTag)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns(bool)
func (_Enygma *EnygmaTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns(bool)
func (_Enygma *EnygmaSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _Enygma.Contract.TransferOwnership(&_Enygma.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns(bool)
func (_Enygma *EnygmaTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _Enygma.Contract.TransferOwnership(&_Enygma.TransactOpts, newOwner)
}

// TransferWithFee is a paid mutator transaction binding the contract method 0x795825a7.
//
// Solidity: function transferWithFee((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[55]) proof, uint256[] participantIds, string bankTag) returns(bool)
func (_Enygma *EnygmaTransactor) TransferWithFee(opts *bind.TransactOpts, commitmentDeltas []IEnygmaPoint, proof IEnygmaFeeProof, participantIds []*big.Int, bankTag string) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "transferWithFee", commitmentDeltas, proof, participantIds, bankTag)
}

// TransferWithFee is a paid mutator transaction binding the contract method 0x795825a7.
//
// Solidity: function transferWithFee((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[55]) proof, uint256[] participantIds, string bankTag) returns(bool)
func (_Enygma *EnygmaSession) TransferWithFee(commitmentDeltas []IEnygmaPoint, proof IEnygmaFeeProof, participantIds []*big.Int, bankTag string) (*types.Transaction, error) {
	return _Enygma.Contract.TransferWithFee(&_Enygma.TransactOpts, commitmentDeltas, proof, participantIds, bankTag)
}

// TransferWithFee is a paid mutator transaction binding the contract method 0x795825a7.
//
// Solidity: function transferWithFee((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[55]) proof, uint256[] participantIds, string bankTag) returns(bool)
func (_Enygma *EnygmaTransactorSession) TransferWithFee(commitmentDeltas []IEnygmaPoint, proof IEnygmaFeeProof, participantIds []*big.Int, bankTag string) (*types.Transaction, error) {
	return _Enygma.Contract.TransferWithFee(&_Enygma.TransactOpts, commitmentDeltas, proof, participantIds, bankTag)
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns(bool)
func (_Enygma *EnygmaTransactor) Unpause(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "unpause")
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns(bool)
func (_Enygma *EnygmaSession) Unpause() (*types.Transaction, error) {
	return _Enygma.Contract.Unpause(&_Enygma.TransactOpts)
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns(bool)
func (_Enygma *EnygmaTransactorSession) Unpause() (*types.Transaction, error) {
	return _Enygma.Contract.Unpause(&_Enygma.TransactOpts)
}

// Withdraw is a paid mutator transaction binding the contract method 0x6f5a2d54.
//
// Solidity: function withdraw((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[52]) proof, (uint256,address,uint256)[] depositParams, uint256[] participantIds) returns(bool, uint256[])
func (_Enygma *EnygmaTransactor) Withdraw(opts *bind.TransactOpts, commitmentDeltas []IEnygmaPoint, proof IEnygmaWithdrawProof, depositParams []IEnygmaDepositParams, participantIds []*big.Int) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "withdraw", commitmentDeltas, proof, depositParams, participantIds)
}

// Withdraw is a paid mutator transaction binding the contract method 0x6f5a2d54.
//
// Solidity: function withdraw((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[52]) proof, (uint256,address,uint256)[] depositParams, uint256[] participantIds) returns(bool, uint256[])
func (_Enygma *EnygmaSession) Withdraw(commitmentDeltas []IEnygmaPoint, proof IEnygmaWithdrawProof, depositParams []IEnygmaDepositParams, participantIds []*big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.Withdraw(&_Enygma.TransactOpts, commitmentDeltas, proof, depositParams, participantIds)
}

// Withdraw is a paid mutator transaction binding the contract method 0x6f5a2d54.
//
// Solidity: function withdraw((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[52]) proof, (uint256,address,uint256)[] depositParams, uint256[] participantIds) returns(bool, uint256[])
func (_Enygma *EnygmaTransactorSession) Withdraw(commitmentDeltas []IEnygmaPoint, proof IEnygmaWithdrawProof, depositParams []IEnygmaDepositParams, participantIds []*big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.Withdraw(&_Enygma.TransactOpts, commitmentDeltas, proof, depositParams, participantIds)
}

// EnygmaAccountRegisteredIterator is returned from FilterAccountRegistered and is used to iterate over the raw logs and unpacked data for AccountRegistered events raised by the Enygma contract.
type EnygmaAccountRegisteredIterator struct {
	Event *EnygmaAccountRegistered // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EnygmaAccountRegisteredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EnygmaAccountRegistered)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EnygmaAccountRegistered)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EnygmaAccountRegisteredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EnygmaAccountRegisteredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EnygmaAccountRegistered represents a AccountRegistered event raised by the Enygma contract.
type EnygmaAccountRegistered struct {
	AddedBank common.Address
	AccountId *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterAccountRegistered is a free log retrieval operation binding the contract event 0xefd1ddef00b1051abc144c2e895de70a10dbbc3ad8985118c74c15e40e3d391f.
//
// Solidity: event AccountRegistered(address indexed addedBank, uint256 accountId)
func (_Enygma *EnygmaFilterer) FilterAccountRegistered(opts *bind.FilterOpts, addedBank []common.Address) (*EnygmaAccountRegisteredIterator, error) {

	var addedBankRule []interface{}
	for _, addedBankItem := range addedBank {
		addedBankRule = append(addedBankRule, addedBankItem)
	}

	logs, sub, err := _Enygma.contract.FilterLogs(opts, "AccountRegistered", addedBankRule)
	if err != nil {
		return nil, err
	}
	return &EnygmaAccountRegisteredIterator{contract: _Enygma.contract, event: "AccountRegistered", logs: logs, sub: sub}, nil
}

// WatchAccountRegistered is a free log subscription operation binding the contract event 0xefd1ddef00b1051abc144c2e895de70a10dbbc3ad8985118c74c15e40e3d391f.
//
// Solidity: event AccountRegistered(address indexed addedBank, uint256 accountId)
func (_Enygma *EnygmaFilterer) WatchAccountRegistered(opts *bind.WatchOpts, sink chan<- *EnygmaAccountRegistered, addedBank []common.Address) (event.Subscription, error) {

	var addedBankRule []interface{}
	for _, addedBankItem := range addedBank {
		addedBankRule = append(addedBankRule, addedBankItem)
	}

	logs, sub, err := _Enygma.contract.WatchLogs(opts, "AccountRegistered", addedBankRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EnygmaAccountRegistered)
				if err := _Enygma.contract.UnpackLog(event, "AccountRegistered", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseAccountRegistered is a log parse operation binding the contract event 0xefd1ddef00b1051abc144c2e895de70a10dbbc3ad8985118c74c15e40e3d391f.
//
// Solidity: event AccountRegistered(address indexed addedBank, uint256 accountId)
func (_Enygma *EnygmaFilterer) ParseAccountRegistered(log types.Log) (*EnygmaAccountRegistered, error) {
	event := new(EnygmaAccountRegistered)
	if err := _Enygma.contract.UnpackLog(event, "AccountRegistered", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EnygmaBurnSuccessfulIterator is returned from FilterBurnSuccessful and is used to iterate over the raw logs and unpacked data for BurnSuccessful events raised by the Enygma contract.
type EnygmaBurnSuccessfulIterator struct {
	Event *EnygmaBurnSuccessful // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EnygmaBurnSuccessfulIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EnygmaBurnSuccessful)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EnygmaBurnSuccessful)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EnygmaBurnSuccessfulIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EnygmaBurnSuccessfulIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EnygmaBurnSuccessful represents a BurnSuccessful event raised by the Enygma contract.
type EnygmaBurnSuccessful struct {
	BankIndex *big.Int
	BurnValue *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterBurnSuccessful is a free log retrieval operation binding the contract event 0x262a9a1794440b6af993000f5805d7f51b5a19d4c32fcb10a1c5216beb0616f4.
//
// Solidity: event BurnSuccessful(uint256 bankIndex, uint256 burnValue)
func (_Enygma *EnygmaFilterer) FilterBurnSuccessful(opts *bind.FilterOpts) (*EnygmaBurnSuccessfulIterator, error) {

	logs, sub, err := _Enygma.contract.FilterLogs(opts, "BurnSuccessful")
	if err != nil {
		return nil, err
	}
	return &EnygmaBurnSuccessfulIterator{contract: _Enygma.contract, event: "BurnSuccessful", logs: logs, sub: sub}, nil
}

// WatchBurnSuccessful is a free log subscription operation binding the contract event 0x262a9a1794440b6af993000f5805d7f51b5a19d4c32fcb10a1c5216beb0616f4.
//
// Solidity: event BurnSuccessful(uint256 bankIndex, uint256 burnValue)
func (_Enygma *EnygmaFilterer) WatchBurnSuccessful(opts *bind.WatchOpts, sink chan<- *EnygmaBurnSuccessful) (event.Subscription, error) {

	logs, sub, err := _Enygma.contract.WatchLogs(opts, "BurnSuccessful")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EnygmaBurnSuccessful)
				if err := _Enygma.contract.UnpackLog(event, "BurnSuccessful", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseBurnSuccessful is a log parse operation binding the contract event 0x262a9a1794440b6af993000f5805d7f51b5a19d4c32fcb10a1c5216beb0616f4.
//
// Solidity: event BurnSuccessful(uint256 bankIndex, uint256 burnValue)
func (_Enygma *EnygmaFilterer) ParseBurnSuccessful(log types.Log) (*EnygmaBurnSuccessful, error) {
	event := new(EnygmaBurnSuccessful)
	if err := _Enygma.contract.UnpackLog(event, "BurnSuccessful", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EnygmaCommitmentIterator is returned from FilterCommitment and is used to iterate over the raw logs and unpacked data for Commitment events raised by the Enygma contract.
type EnygmaCommitmentIterator struct {
	Event *EnygmaCommitment // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EnygmaCommitmentIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EnygmaCommitment)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EnygmaCommitment)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EnygmaCommitmentIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EnygmaCommitmentIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EnygmaCommitment represents a Commitment event raised by the Enygma contract.
type EnygmaCommitment struct {
	Commitment *big.Int
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterCommitment is a free log retrieval operation binding the contract event 0xef61e988d9804d573b4fc504760f55d3507094e4168fddc9245ac56fbfc419e4.
//
// Solidity: event Commitment(uint256 indexed commitment)
func (_Enygma *EnygmaFilterer) FilterCommitment(opts *bind.FilterOpts, commitment []*big.Int) (*EnygmaCommitmentIterator, error) {

	var commitmentRule []interface{}
	for _, commitmentItem := range commitment {
		commitmentRule = append(commitmentRule, commitmentItem)
	}

	logs, sub, err := _Enygma.contract.FilterLogs(opts, "Commitment", commitmentRule)
	if err != nil {
		return nil, err
	}
	return &EnygmaCommitmentIterator{contract: _Enygma.contract, event: "Commitment", logs: logs, sub: sub}, nil
}

// WatchCommitment is a free log subscription operation binding the contract event 0xef61e988d9804d573b4fc504760f55d3507094e4168fddc9245ac56fbfc419e4.
//
// Solidity: event Commitment(uint256 indexed commitment)
func (_Enygma *EnygmaFilterer) WatchCommitment(opts *bind.WatchOpts, sink chan<- *EnygmaCommitment, commitment []*big.Int) (event.Subscription, error) {

	var commitmentRule []interface{}
	for _, commitmentItem := range commitment {
		commitmentRule = append(commitmentRule, commitmentItem)
	}

	logs, sub, err := _Enygma.contract.WatchLogs(opts, "Commitment", commitmentRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EnygmaCommitment)
				if err := _Enygma.contract.UnpackLog(event, "Commitment", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseCommitment is a log parse operation binding the contract event 0xef61e988d9804d573b4fc504760f55d3507094e4168fddc9245ac56fbfc419e4.
//
// Solidity: event Commitment(uint256 indexed commitment)
func (_Enygma *EnygmaFilterer) ParseCommitment(log types.Log) (*EnygmaCommitment, error) {
	event := new(EnygmaCommitment)
	if err := _Enygma.contract.UnpackLog(event, "Commitment", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EnygmaFeeBurnedIterator is returned from FilterFeeBurned and is used to iterate over the raw logs and unpacked data for FeeBurned events raised by the Enygma contract.
type EnygmaFeeBurnedIterator struct {
	Event *EnygmaFeeBurned // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EnygmaFeeBurnedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EnygmaFeeBurned)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EnygmaFeeBurned)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EnygmaFeeBurnedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EnygmaFeeBurnedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EnygmaFeeBurned represents a FeeBurned event raised by the Enygma contract.
type EnygmaFeeBurned struct {
	Fee *big.Int
	Raw types.Log // Blockchain specific contextual infos
}

// FilterFeeBurned is a free log retrieval operation binding the contract event 0xa551808c565cfbf20dfffdbcd44c549f835f9d06a82dcd546c61644b2f5ce791.
//
// Solidity: event FeeBurned(uint256 fee)
func (_Enygma *EnygmaFilterer) FilterFeeBurned(opts *bind.FilterOpts) (*EnygmaFeeBurnedIterator, error) {

	logs, sub, err := _Enygma.contract.FilterLogs(opts, "FeeBurned")
	if err != nil {
		return nil, err
	}
	return &EnygmaFeeBurnedIterator{contract: _Enygma.contract, event: "FeeBurned", logs: logs, sub: sub}, nil
}

// WatchFeeBurned is a free log subscription operation binding the contract event 0xa551808c565cfbf20dfffdbcd44c549f835f9d06a82dcd546c61644b2f5ce791.
//
// Solidity: event FeeBurned(uint256 fee)
func (_Enygma *EnygmaFilterer) WatchFeeBurned(opts *bind.WatchOpts, sink chan<- *EnygmaFeeBurned) (event.Subscription, error) {

	logs, sub, err := _Enygma.contract.WatchLogs(opts, "FeeBurned")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EnygmaFeeBurned)
				if err := _Enygma.contract.UnpackLog(event, "FeeBurned", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseFeeBurned is a log parse operation binding the contract event 0xa551808c565cfbf20dfffdbcd44c549f835f9d06a82dcd546c61644b2f5ce791.
//
// Solidity: event FeeBurned(uint256 fee)
func (_Enygma *EnygmaFilterer) ParseFeeBurned(log types.Log) (*EnygmaFeeBurned, error) {
	event := new(EnygmaFeeBurned)
	if err := _Enygma.contract.UnpackLog(event, "FeeBurned", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EnygmaFingerprintConfirmedIterator is returned from FilterFingerprintConfirmed and is used to iterate over the raw logs and unpacked data for FingerprintConfirmed events raised by the Enygma contract.
type EnygmaFingerprintConfirmedIterator struct {
	Event *EnygmaFingerprintConfirmed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EnygmaFingerprintConfirmedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EnygmaFingerprintConfirmed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EnygmaFingerprintConfirmed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EnygmaFingerprintConfirmedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EnygmaFingerprintConfirmedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EnygmaFingerprintConfirmed represents a FingerprintConfirmed event raised by the Enygma contract.
type EnygmaFingerprintConfirmed struct {
	PartyA      *big.Int
	PartyB      *big.Int
	Fingerprint *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterFingerprintConfirmed is a free log retrieval operation binding the contract event 0xe2070e33257d0615b4d0bff5bc34dcc22292a2ca3973074cc97d92dcc884549d.
//
// Solidity: event FingerprintConfirmed(uint256 indexed partyA, uint256 indexed partyB, uint256 fingerprint)
func (_Enygma *EnygmaFilterer) FilterFingerprintConfirmed(opts *bind.FilterOpts, partyA []*big.Int, partyB []*big.Int) (*EnygmaFingerprintConfirmedIterator, error) {

	var partyARule []interface{}
	for _, partyAItem := range partyA {
		partyARule = append(partyARule, partyAItem)
	}
	var partyBRule []interface{}
	for _, partyBItem := range partyB {
		partyBRule = append(partyBRule, partyBItem)
	}

	logs, sub, err := _Enygma.contract.FilterLogs(opts, "FingerprintConfirmed", partyARule, partyBRule)
	if err != nil {
		return nil, err
	}
	return &EnygmaFingerprintConfirmedIterator{contract: _Enygma.contract, event: "FingerprintConfirmed", logs: logs, sub: sub}, nil
}

// WatchFingerprintConfirmed is a free log subscription operation binding the contract event 0xe2070e33257d0615b4d0bff5bc34dcc22292a2ca3973074cc97d92dcc884549d.
//
// Solidity: event FingerprintConfirmed(uint256 indexed partyA, uint256 indexed partyB, uint256 fingerprint)
func (_Enygma *EnygmaFilterer) WatchFingerprintConfirmed(opts *bind.WatchOpts, sink chan<- *EnygmaFingerprintConfirmed, partyA []*big.Int, partyB []*big.Int) (event.Subscription, error) {

	var partyARule []interface{}
	for _, partyAItem := range partyA {
		partyARule = append(partyARule, partyAItem)
	}
	var partyBRule []interface{}
	for _, partyBItem := range partyB {
		partyBRule = append(partyBRule, partyBItem)
	}

	logs, sub, err := _Enygma.contract.WatchLogs(opts, "FingerprintConfirmed", partyARule, partyBRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EnygmaFingerprintConfirmed)
				if err := _Enygma.contract.UnpackLog(event, "FingerprintConfirmed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseFingerprintConfirmed is a log parse operation binding the contract event 0xe2070e33257d0615b4d0bff5bc34dcc22292a2ca3973074cc97d92dcc884549d.
//
// Solidity: event FingerprintConfirmed(uint256 indexed partyA, uint256 indexed partyB, uint256 fingerprint)
func (_Enygma *EnygmaFilterer) ParseFingerprintConfirmed(log types.Log) (*EnygmaFingerprintConfirmed, error) {
	event := new(EnygmaFingerprintConfirmed)
	if err := _Enygma.contract.UnpackLog(event, "FingerprintConfirmed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EnygmaFingerprintPendingIterator is returned from FilterFingerprintPending and is used to iterate over the raw logs and unpacked data for FingerprintPending events raised by the Enygma contract.
type EnygmaFingerprintPendingIterator struct {
	Event *EnygmaFingerprintPending // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EnygmaFingerprintPendingIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EnygmaFingerprintPending)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EnygmaFingerprintPending)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EnygmaFingerprintPendingIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EnygmaFingerprintPendingIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EnygmaFingerprintPending represents a FingerprintPending event raised by the Enygma contract.
type EnygmaFingerprintPending struct {
	FromId      *big.Int
	ToId        *big.Int
	Fingerprint *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterFingerprintPending is a free log retrieval operation binding the contract event 0x2244c7409c0d13a2d9db63e7bfad1826bd85a2d6052ad998f150d2aab30c0d1f.
//
// Solidity: event FingerprintPending(uint256 indexed fromId, uint256 indexed toId, uint256 fingerprint)
func (_Enygma *EnygmaFilterer) FilterFingerprintPending(opts *bind.FilterOpts, fromId []*big.Int, toId []*big.Int) (*EnygmaFingerprintPendingIterator, error) {

	var fromIdRule []interface{}
	for _, fromIdItem := range fromId {
		fromIdRule = append(fromIdRule, fromIdItem)
	}
	var toIdRule []interface{}
	for _, toIdItem := range toId {
		toIdRule = append(toIdRule, toIdItem)
	}

	logs, sub, err := _Enygma.contract.FilterLogs(opts, "FingerprintPending", fromIdRule, toIdRule)
	if err != nil {
		return nil, err
	}
	return &EnygmaFingerprintPendingIterator{contract: _Enygma.contract, event: "FingerprintPending", logs: logs, sub: sub}, nil
}

// WatchFingerprintPending is a free log subscription operation binding the contract event 0x2244c7409c0d13a2d9db63e7bfad1826bd85a2d6052ad998f150d2aab30c0d1f.
//
// Solidity: event FingerprintPending(uint256 indexed fromId, uint256 indexed toId, uint256 fingerprint)
func (_Enygma *EnygmaFilterer) WatchFingerprintPending(opts *bind.WatchOpts, sink chan<- *EnygmaFingerprintPending, fromId []*big.Int, toId []*big.Int) (event.Subscription, error) {

	var fromIdRule []interface{}
	for _, fromIdItem := range fromId {
		fromIdRule = append(fromIdRule, fromIdItem)
	}
	var toIdRule []interface{}
	for _, toIdItem := range toId {
		toIdRule = append(toIdRule, toIdItem)
	}

	logs, sub, err := _Enygma.contract.WatchLogs(opts, "FingerprintPending", fromIdRule, toIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EnygmaFingerprintPending)
				if err := _Enygma.contract.UnpackLog(event, "FingerprintPending", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseFingerprintPending is a log parse operation binding the contract event 0x2244c7409c0d13a2d9db63e7bfad1826bd85a2d6052ad998f150d2aab30c0d1f.
//
// Solidity: event FingerprintPending(uint256 indexed fromId, uint256 indexed toId, uint256 fingerprint)
func (_Enygma *EnygmaFilterer) ParseFingerprintPending(log types.Log) (*EnygmaFingerprintPending, error) {
	event := new(EnygmaFingerprintPending)
	if err := _Enygma.contract.UnpackLog(event, "FingerprintPending", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EnygmaOwnershipTransferStartedIterator is returned from FilterOwnershipTransferStarted and is used to iterate over the raw logs and unpacked data for OwnershipTransferStarted events raised by the Enygma contract.
type EnygmaOwnershipTransferStartedIterator struct {
	Event *EnygmaOwnershipTransferStarted // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EnygmaOwnershipTransferStartedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EnygmaOwnershipTransferStarted)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EnygmaOwnershipTransferStarted)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EnygmaOwnershipTransferStartedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EnygmaOwnershipTransferStartedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EnygmaOwnershipTransferStarted represents a OwnershipTransferStarted event raised by the Enygma contract.
type EnygmaOwnershipTransferStarted struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferStarted is a free log retrieval operation binding the contract event 0x38d16b8cac22d99fc7c124b9cd0de2d3fa1faef420bfe791d8c362d765e22700.
//
// Solidity: event OwnershipTransferStarted(address indexed previousOwner, address indexed newOwner)
func (_Enygma *EnygmaFilterer) FilterOwnershipTransferStarted(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*EnygmaOwnershipTransferStartedIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _Enygma.contract.FilterLogs(opts, "OwnershipTransferStarted", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &EnygmaOwnershipTransferStartedIterator{contract: _Enygma.contract, event: "OwnershipTransferStarted", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferStarted is a free log subscription operation binding the contract event 0x38d16b8cac22d99fc7c124b9cd0de2d3fa1faef420bfe791d8c362d765e22700.
//
// Solidity: event OwnershipTransferStarted(address indexed previousOwner, address indexed newOwner)
func (_Enygma *EnygmaFilterer) WatchOwnershipTransferStarted(opts *bind.WatchOpts, sink chan<- *EnygmaOwnershipTransferStarted, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _Enygma.contract.WatchLogs(opts, "OwnershipTransferStarted", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EnygmaOwnershipTransferStarted)
				if err := _Enygma.contract.UnpackLog(event, "OwnershipTransferStarted", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseOwnershipTransferStarted is a log parse operation binding the contract event 0x38d16b8cac22d99fc7c124b9cd0de2d3fa1faef420bfe791d8c362d765e22700.
//
// Solidity: event OwnershipTransferStarted(address indexed previousOwner, address indexed newOwner)
func (_Enygma *EnygmaFilterer) ParseOwnershipTransferStarted(log types.Log) (*EnygmaOwnershipTransferStarted, error) {
	event := new(EnygmaOwnershipTransferStarted)
	if err := _Enygma.contract.UnpackLog(event, "OwnershipTransferStarted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EnygmaOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the Enygma contract.
type EnygmaOwnershipTransferredIterator struct {
	Event *EnygmaOwnershipTransferred // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EnygmaOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EnygmaOwnershipTransferred)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EnygmaOwnershipTransferred)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EnygmaOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EnygmaOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EnygmaOwnershipTransferred represents a OwnershipTransferred event raised by the Enygma contract.
type EnygmaOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Enygma *EnygmaFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*EnygmaOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _Enygma.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &EnygmaOwnershipTransferredIterator{contract: _Enygma.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Enygma *EnygmaFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *EnygmaOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _Enygma.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EnygmaOwnershipTransferred)
				if err := _Enygma.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseOwnershipTransferred is a log parse operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Enygma *EnygmaFilterer) ParseOwnershipTransferred(log types.Log) (*EnygmaOwnershipTransferred, error) {
	event := new(EnygmaOwnershipTransferred)
	if err := _Enygma.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EnygmaPausedIterator is returned from FilterPaused and is used to iterate over the raw logs and unpacked data for Paused events raised by the Enygma contract.
type EnygmaPausedIterator struct {
	Event *EnygmaPaused // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EnygmaPausedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EnygmaPaused)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EnygmaPaused)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EnygmaPausedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EnygmaPausedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EnygmaPaused represents a Paused event raised by the Enygma contract.
type EnygmaPaused struct {
	Account common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterPaused is a free log retrieval operation binding the contract event 0x62e78cea01bee320cd4e420270b5ea74000d11b0c9f74754ebdbfc544b05a258.
//
// Solidity: event Paused(address account)
func (_Enygma *EnygmaFilterer) FilterPaused(opts *bind.FilterOpts) (*EnygmaPausedIterator, error) {

	logs, sub, err := _Enygma.contract.FilterLogs(opts, "Paused")
	if err != nil {
		return nil, err
	}
	return &EnygmaPausedIterator{contract: _Enygma.contract, event: "Paused", logs: logs, sub: sub}, nil
}

// WatchPaused is a free log subscription operation binding the contract event 0x62e78cea01bee320cd4e420270b5ea74000d11b0c9f74754ebdbfc544b05a258.
//
// Solidity: event Paused(address account)
func (_Enygma *EnygmaFilterer) WatchPaused(opts *bind.WatchOpts, sink chan<- *EnygmaPaused) (event.Subscription, error) {

	logs, sub, err := _Enygma.contract.WatchLogs(opts, "Paused")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EnygmaPaused)
				if err := _Enygma.contract.UnpackLog(event, "Paused", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParsePaused is a log parse operation binding the contract event 0x62e78cea01bee320cd4e420270b5ea74000d11b0c9f74754ebdbfc544b05a258.
//
// Solidity: event Paused(address account)
func (_Enygma *EnygmaFilterer) ParsePaused(log types.Log) (*EnygmaPaused, error) {
	event := new(EnygmaPaused)
	if err := _Enygma.contract.UnpackLog(event, "Paused", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EnygmaProtocolFeeUpdatedIterator is returned from FilterProtocolFeeUpdated and is used to iterate over the raw logs and unpacked data for ProtocolFeeUpdated events raised by the Enygma contract.
type EnygmaProtocolFeeUpdatedIterator struct {
	Event *EnygmaProtocolFeeUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EnygmaProtocolFeeUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EnygmaProtocolFeeUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EnygmaProtocolFeeUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EnygmaProtocolFeeUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EnygmaProtocolFeeUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EnygmaProtocolFeeUpdated represents a ProtocolFeeUpdated event raised by the Enygma contract.
type EnygmaProtocolFeeUpdated struct {
	PreviousFee *big.Int
	NewFee      *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterProtocolFeeUpdated is a free log retrieval operation binding the contract event 0xb404cac19fb1cbeff98d325795b08886e3cd8fe8cb1a2f193aac66f13fb239c3.
//
// Solidity: event ProtocolFeeUpdated(uint256 previousFee, uint256 newFee)
func (_Enygma *EnygmaFilterer) FilterProtocolFeeUpdated(opts *bind.FilterOpts) (*EnygmaProtocolFeeUpdatedIterator, error) {

	logs, sub, err := _Enygma.contract.FilterLogs(opts, "ProtocolFeeUpdated")
	if err != nil {
		return nil, err
	}
	return &EnygmaProtocolFeeUpdatedIterator{contract: _Enygma.contract, event: "ProtocolFeeUpdated", logs: logs, sub: sub}, nil
}

// WatchProtocolFeeUpdated is a free log subscription operation binding the contract event 0xb404cac19fb1cbeff98d325795b08886e3cd8fe8cb1a2f193aac66f13fb239c3.
//
// Solidity: event ProtocolFeeUpdated(uint256 previousFee, uint256 newFee)
func (_Enygma *EnygmaFilterer) WatchProtocolFeeUpdated(opts *bind.WatchOpts, sink chan<- *EnygmaProtocolFeeUpdated) (event.Subscription, error) {

	logs, sub, err := _Enygma.contract.WatchLogs(opts, "ProtocolFeeUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EnygmaProtocolFeeUpdated)
				if err := _Enygma.contract.UnpackLog(event, "ProtocolFeeUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseProtocolFeeUpdated is a log parse operation binding the contract event 0xb404cac19fb1cbeff98d325795b08886e3cd8fe8cb1a2f193aac66f13fb239c3.
//
// Solidity: event ProtocolFeeUpdated(uint256 previousFee, uint256 newFee)
func (_Enygma *EnygmaFilterer) ParseProtocolFeeUpdated(log types.Log) (*EnygmaProtocolFeeUpdated, error) {
	event := new(EnygmaProtocolFeeUpdated)
	if err := _Enygma.contract.UnpackLog(event, "ProtocolFeeUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EnygmaRelayAttributionIterator is returned from FilterRelayAttribution and is used to iterate over the raw logs and unpacked data for RelayAttribution events raised by the Enygma contract.
type EnygmaRelayAttributionIterator struct {
	Event *EnygmaRelayAttribution // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EnygmaRelayAttributionIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EnygmaRelayAttribution)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EnygmaRelayAttribution)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EnygmaRelayAttributionIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EnygmaRelayAttributionIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EnygmaRelayAttribution represents a RelayAttribution event raised by the Enygma contract.
type EnygmaRelayAttribution struct {
	Submitter common.Address
	BankTag   string
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterRelayAttribution is a free log retrieval operation binding the contract event 0x1d5f56c1b8cdd9a3aa6495672f7e6917c2f5f1e98a43abd3e16e2e997c1160e1.
//
// Solidity: event RelayAttribution(address indexed submitter, string bankTag)
func (_Enygma *EnygmaFilterer) FilterRelayAttribution(opts *bind.FilterOpts, submitter []common.Address) (*EnygmaRelayAttributionIterator, error) {

	var submitterRule []interface{}
	for _, submitterItem := range submitter {
		submitterRule = append(submitterRule, submitterItem)
	}

	logs, sub, err := _Enygma.contract.FilterLogs(opts, "RelayAttribution", submitterRule)
	if err != nil {
		return nil, err
	}
	return &EnygmaRelayAttributionIterator{contract: _Enygma.contract, event: "RelayAttribution", logs: logs, sub: sub}, nil
}

// WatchRelayAttribution is a free log subscription operation binding the contract event 0x1d5f56c1b8cdd9a3aa6495672f7e6917c2f5f1e98a43abd3e16e2e997c1160e1.
//
// Solidity: event RelayAttribution(address indexed submitter, string bankTag)
func (_Enygma *EnygmaFilterer) WatchRelayAttribution(opts *bind.WatchOpts, sink chan<- *EnygmaRelayAttribution, submitter []common.Address) (event.Subscription, error) {

	var submitterRule []interface{}
	for _, submitterItem := range submitter {
		submitterRule = append(submitterRule, submitterItem)
	}

	logs, sub, err := _Enygma.contract.WatchLogs(opts, "RelayAttribution", submitterRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EnygmaRelayAttribution)
				if err := _Enygma.contract.UnpackLog(event, "RelayAttribution", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseRelayAttribution is a log parse operation binding the contract event 0x1d5f56c1b8cdd9a3aa6495672f7e6917c2f5f1e98a43abd3e16e2e997c1160e1.
//
// Solidity: event RelayAttribution(address indexed submitter, string bankTag)
func (_Enygma *EnygmaFilterer) ParseRelayAttribution(log types.Log) (*EnygmaRelayAttribution, error) {
	event := new(EnygmaRelayAttribution)
	if err := _Enygma.contract.UnpackLog(event, "RelayAttribution", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EnygmaSupplyMintedIterator is returned from FilterSupplyMinted and is used to iterate over the raw logs and unpacked data for SupplyMinted events raised by the Enygma contract.
type EnygmaSupplyMintedIterator struct {
	Event *EnygmaSupplyMinted // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EnygmaSupplyMintedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EnygmaSupplyMinted)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EnygmaSupplyMinted)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EnygmaSupplyMintedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EnygmaSupplyMintedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EnygmaSupplyMinted represents a SupplyMinted event raised by the Enygma contract.
type EnygmaSupplyMinted struct {
	LastblockNum *big.Int
	Amount       *big.Int
	To           *big.Int
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterSupplyMinted is a free log retrieval operation binding the contract event 0xeae287c62f1ff4911334dee03f631d5dded5284b1b03ea7bc1d6282916c7249f.
//
// Solidity: event SupplyMinted(uint256 indexed lastblockNum, uint256 amount, uint256 to)
func (_Enygma *EnygmaFilterer) FilterSupplyMinted(opts *bind.FilterOpts, lastblockNum []*big.Int) (*EnygmaSupplyMintedIterator, error) {

	var lastblockNumRule []interface{}
	for _, lastblockNumItem := range lastblockNum {
		lastblockNumRule = append(lastblockNumRule, lastblockNumItem)
	}

	logs, sub, err := _Enygma.contract.FilterLogs(opts, "SupplyMinted", lastblockNumRule)
	if err != nil {
		return nil, err
	}
	return &EnygmaSupplyMintedIterator{contract: _Enygma.contract, event: "SupplyMinted", logs: logs, sub: sub}, nil
}

// WatchSupplyMinted is a free log subscription operation binding the contract event 0xeae287c62f1ff4911334dee03f631d5dded5284b1b03ea7bc1d6282916c7249f.
//
// Solidity: event SupplyMinted(uint256 indexed lastblockNum, uint256 amount, uint256 to)
func (_Enygma *EnygmaFilterer) WatchSupplyMinted(opts *bind.WatchOpts, sink chan<- *EnygmaSupplyMinted, lastblockNum []*big.Int) (event.Subscription, error) {

	var lastblockNumRule []interface{}
	for _, lastblockNumItem := range lastblockNum {
		lastblockNumRule = append(lastblockNumRule, lastblockNumItem)
	}

	logs, sub, err := _Enygma.contract.WatchLogs(opts, "SupplyMinted", lastblockNumRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EnygmaSupplyMinted)
				if err := _Enygma.contract.UnpackLog(event, "SupplyMinted", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseSupplyMinted is a log parse operation binding the contract event 0xeae287c62f1ff4911334dee03f631d5dded5284b1b03ea7bc1d6282916c7249f.
//
// Solidity: event SupplyMinted(uint256 indexed lastblockNum, uint256 amount, uint256 to)
func (_Enygma *EnygmaFilterer) ParseSupplyMinted(log types.Log) (*EnygmaSupplyMinted, error) {
	event := new(EnygmaSupplyMinted)
	if err := _Enygma.contract.UnpackLog(event, "SupplyMinted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EnygmaTokenInitializedIterator is returned from FilterTokenInitialized and is used to iterate over the raw logs and unpacked data for TokenInitialized events raised by the Enygma contract.
type EnygmaTokenInitializedIterator struct {
	Event *EnygmaTokenInitialized // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EnygmaTokenInitializedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EnygmaTokenInitialized)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EnygmaTokenInitialized)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EnygmaTokenInitializedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EnygmaTokenInitializedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EnygmaTokenInitialized represents a TokenInitialized event raised by the Enygma contract.
type EnygmaTokenInitialized struct {
	MaxBankCount *big.Int
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterTokenInitialized is a free log retrieval operation binding the contract event 0x10e8ab53866dbf444b164da1c9d4531e71008f9bc55e85ab2302f97f862389be.
//
// Solidity: event TokenInitialized(uint256 maxBankCount)
func (_Enygma *EnygmaFilterer) FilterTokenInitialized(opts *bind.FilterOpts) (*EnygmaTokenInitializedIterator, error) {

	logs, sub, err := _Enygma.contract.FilterLogs(opts, "TokenInitialized")
	if err != nil {
		return nil, err
	}
	return &EnygmaTokenInitializedIterator{contract: _Enygma.contract, event: "TokenInitialized", logs: logs, sub: sub}, nil
}

// WatchTokenInitialized is a free log subscription operation binding the contract event 0x10e8ab53866dbf444b164da1c9d4531e71008f9bc55e85ab2302f97f862389be.
//
// Solidity: event TokenInitialized(uint256 maxBankCount)
func (_Enygma *EnygmaFilterer) WatchTokenInitialized(opts *bind.WatchOpts, sink chan<- *EnygmaTokenInitialized) (event.Subscription, error) {

	logs, sub, err := _Enygma.contract.WatchLogs(opts, "TokenInitialized")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EnygmaTokenInitialized)
				if err := _Enygma.contract.UnpackLog(event, "TokenInitialized", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseTokenInitialized is a log parse operation binding the contract event 0x10e8ab53866dbf444b164da1c9d4531e71008f9bc55e85ab2302f97f862389be.
//
// Solidity: event TokenInitialized(uint256 maxBankCount)
func (_Enygma *EnygmaFilterer) ParseTokenInitialized(log types.Log) (*EnygmaTokenInitialized, error) {
	event := new(EnygmaTokenInitialized)
	if err := _Enygma.contract.UnpackLog(event, "TokenInitialized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EnygmaTransactionSuccessfulIterator is returned from FilterTransactionSuccessful and is used to iterate over the raw logs and unpacked data for TransactionSuccessful events raised by the Enygma contract.
type EnygmaTransactionSuccessfulIterator struct {
	Event *EnygmaTransactionSuccessful // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EnygmaTransactionSuccessfulIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EnygmaTransactionSuccessful)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EnygmaTransactionSuccessful)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EnygmaTransactionSuccessfulIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EnygmaTransactionSuccessfulIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EnygmaTransactionSuccessful represents a TransactionSuccessful event raised by the Enygma contract.
type EnygmaTransactionSuccessful struct {
	SenderAddress common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterTransactionSuccessful is a free log retrieval operation binding the contract event 0xe85c8c79cebe1b6656a265affa1c69c79539e5ae9a9c9229f5b5d89619781080.
//
// Solidity: event TransactionSuccessful(address indexed senderAddress)
func (_Enygma *EnygmaFilterer) FilterTransactionSuccessful(opts *bind.FilterOpts, senderAddress []common.Address) (*EnygmaTransactionSuccessfulIterator, error) {

	var senderAddressRule []interface{}
	for _, senderAddressItem := range senderAddress {
		senderAddressRule = append(senderAddressRule, senderAddressItem)
	}

	logs, sub, err := _Enygma.contract.FilterLogs(opts, "TransactionSuccessful", senderAddressRule)
	if err != nil {
		return nil, err
	}
	return &EnygmaTransactionSuccessfulIterator{contract: _Enygma.contract, event: "TransactionSuccessful", logs: logs, sub: sub}, nil
}

// WatchTransactionSuccessful is a free log subscription operation binding the contract event 0xe85c8c79cebe1b6656a265affa1c69c79539e5ae9a9c9229f5b5d89619781080.
//
// Solidity: event TransactionSuccessful(address indexed senderAddress)
func (_Enygma *EnygmaFilterer) WatchTransactionSuccessful(opts *bind.WatchOpts, sink chan<- *EnygmaTransactionSuccessful, senderAddress []common.Address) (event.Subscription, error) {

	var senderAddressRule []interface{}
	for _, senderAddressItem := range senderAddress {
		senderAddressRule = append(senderAddressRule, senderAddressItem)
	}

	logs, sub, err := _Enygma.contract.WatchLogs(opts, "TransactionSuccessful", senderAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EnygmaTransactionSuccessful)
				if err := _Enygma.contract.UnpackLog(event, "TransactionSuccessful", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseTransactionSuccessful is a log parse operation binding the contract event 0xe85c8c79cebe1b6656a265affa1c69c79539e5ae9a9c9229f5b5d89619781080.
//
// Solidity: event TransactionSuccessful(address indexed senderAddress)
func (_Enygma *EnygmaFilterer) ParseTransactionSuccessful(log types.Log) (*EnygmaTransactionSuccessful, error) {
	event := new(EnygmaTransactionSuccessful)
	if err := _Enygma.contract.UnpackLog(event, "TransactionSuccessful", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EnygmaUnpausedIterator is returned from FilterUnpaused and is used to iterate over the raw logs and unpacked data for Unpaused events raised by the Enygma contract.
type EnygmaUnpausedIterator struct {
	Event *EnygmaUnpaused // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EnygmaUnpausedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EnygmaUnpaused)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EnygmaUnpaused)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EnygmaUnpausedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EnygmaUnpausedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EnygmaUnpaused represents a Unpaused event raised by the Enygma contract.
type EnygmaUnpaused struct {
	Account common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterUnpaused is a free log retrieval operation binding the contract event 0x5db9ee0a495bf2e6ff9c91a7834c1ba4fdd244a5e8aa4e537bd38aeae4b073aa.
//
// Solidity: event Unpaused(address account)
func (_Enygma *EnygmaFilterer) FilterUnpaused(opts *bind.FilterOpts) (*EnygmaUnpausedIterator, error) {

	logs, sub, err := _Enygma.contract.FilterLogs(opts, "Unpaused")
	if err != nil {
		return nil, err
	}
	return &EnygmaUnpausedIterator{contract: _Enygma.contract, event: "Unpaused", logs: logs, sub: sub}, nil
}

// WatchUnpaused is a free log subscription operation binding the contract event 0x5db9ee0a495bf2e6ff9c91a7834c1ba4fdd244a5e8aa4e537bd38aeae4b073aa.
//
// Solidity: event Unpaused(address account)
func (_Enygma *EnygmaFilterer) WatchUnpaused(opts *bind.WatchOpts, sink chan<- *EnygmaUnpaused) (event.Subscription, error) {

	logs, sub, err := _Enygma.contract.WatchLogs(opts, "Unpaused")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EnygmaUnpaused)
				if err := _Enygma.contract.UnpackLog(event, "Unpaused", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseUnpaused is a log parse operation binding the contract event 0x5db9ee0a495bf2e6ff9c91a7834c1ba4fdd244a5e8aa4e537bd38aeae4b073aa.
//
// Solidity: event Unpaused(address account)
func (_Enygma *EnygmaFilterer) ParseUnpaused(log types.Log) (*EnygmaUnpaused, error) {
	event := new(EnygmaUnpaused)
	if err := _Enygma.contract.UnpackLog(event, "Unpaused", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EnygmaVerifierRegisteredIterator is returned from FilterVerifierRegistered and is used to iterate over the raw logs and unpacked data for VerifierRegistered events raised by the Enygma contract.
type EnygmaVerifierRegisteredIterator struct {
	Event *EnygmaVerifierRegistered // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EnygmaVerifierRegisteredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EnygmaVerifierRegistered)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EnygmaVerifierRegistered)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EnygmaVerifierRegisteredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EnygmaVerifierRegisteredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EnygmaVerifierRegistered represents a VerifierRegistered event raised by the Enygma contract.
type EnygmaVerifierRegistered struct {
	VerifierAddress          common.Address
	TotalRegisteredVerifiers *big.Int
	Raw                      types.Log // Blockchain specific contextual infos
}

// FilterVerifierRegistered is a free log retrieval operation binding the contract event 0x983b8264b64c9863a439320eb632213f6e5ca279753b012988656784757d9775.
//
// Solidity: event VerifierRegistered(address indexed verifierAddress, uint256 totalRegisteredVerifiers)
func (_Enygma *EnygmaFilterer) FilterVerifierRegistered(opts *bind.FilterOpts, verifierAddress []common.Address) (*EnygmaVerifierRegisteredIterator, error) {

	var verifierAddressRule []interface{}
	for _, verifierAddressItem := range verifierAddress {
		verifierAddressRule = append(verifierAddressRule, verifierAddressItem)
	}

	logs, sub, err := _Enygma.contract.FilterLogs(opts, "VerifierRegistered", verifierAddressRule)
	if err != nil {
		return nil, err
	}
	return &EnygmaVerifierRegisteredIterator{contract: _Enygma.contract, event: "VerifierRegistered", logs: logs, sub: sub}, nil
}

// WatchVerifierRegistered is a free log subscription operation binding the contract event 0x983b8264b64c9863a439320eb632213f6e5ca279753b012988656784757d9775.
//
// Solidity: event VerifierRegistered(address indexed verifierAddress, uint256 totalRegisteredVerifiers)
func (_Enygma *EnygmaFilterer) WatchVerifierRegistered(opts *bind.WatchOpts, sink chan<- *EnygmaVerifierRegistered, verifierAddress []common.Address) (event.Subscription, error) {

	var verifierAddressRule []interface{}
	for _, verifierAddressItem := range verifierAddress {
		verifierAddressRule = append(verifierAddressRule, verifierAddressItem)
	}

	logs, sub, err := _Enygma.contract.WatchLogs(opts, "VerifierRegistered", verifierAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EnygmaVerifierRegistered)
				if err := _Enygma.contract.UnpackLog(event, "VerifierRegistered", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseVerifierRegistered is a log parse operation binding the contract event 0x983b8264b64c9863a439320eb632213f6e5ca279753b012988656784757d9775.
//
// Solidity: event VerifierRegistered(address indexed verifierAddress, uint256 totalRegisteredVerifiers)
func (_Enygma *EnygmaFilterer) ParseVerifierRegistered(log types.Log) (*EnygmaVerifierRegistered, error) {
	event := new(EnygmaVerifierRegistered)
	if err := _Enygma.contract.UnpackLog(event, "VerifierRegistered", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
