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
	PublicSignal [82]*big.Int
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
	ABI: "[{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_epochInterval\",\"type\":\"uint256\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[],\"name\":\"AlreadyInitialized\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"AlreadyRegistered\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"BalanceMismatch\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"BurnExceedsModulus\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ContractIsPaused\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"DepositValueMismatch\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"FeeExceedsModulus\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"FingerprintNotConfirmed\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidAccountId\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidBlockNumber\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidCommitmentPoint\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidDomain\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidFee\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidFeeAmount\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidFingerprintParty\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidParticipantCount\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidProof\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidPublicInputs\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidPublicKey\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidViewKeyLength\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NewOwnerIsZeroAddress\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NotInitialized\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NotOwner\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NotPendingOwner\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NotRegistered\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NullifierAlreadyUsed\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ParticipantIdsLengthMismatch\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ParticipantIdsNotSorted\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ReentrancyGuardReentrantCall\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"UnregisteredParticipant\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"UsdrBindingMismatch\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"VerifierHasNoCode\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"VerifierNotFound\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ZeroAddress\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ZkDvpOperationFailed\",\"type\":\"error\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"addedBank\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"accountId\",\"type\":\"uint256\"}],\"name\":\"AccountRegistered\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"bankIndex\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"burnValue\",\"type\":\"uint256\"}],\"name\":\"BurnSuccessful\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"commitment\",\"type\":\"uint256\"}],\"name\":\"Commitment\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"fee\",\"type\":\"uint256\"}],\"name\":\"FeeBurned\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"partyA\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"partyB\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"fingerprint\",\"type\":\"uint256\"}],\"name\":\"FingerprintConfirmed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"fromId\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"toId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"fingerprint\",\"type\":\"uint256\"}],\"name\":\"FingerprintPending\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferStarted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"Paused\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"previousFee\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"newFee\",\"type\":\"uint256\"}],\"name\":\"ProtocolFeeUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"submitter\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"bankTag\",\"type\":\"string\"}],\"name\":\"RelayAttribution\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"lastblockNum\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"to\",\"type\":\"uint256\"}],\"name\":\"SupplyMinted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"maxBankCount\",\"type\":\"uint256\"}],\"name\":\"TokenInitialized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"senderAddress\",\"type\":\"address\"}],\"name\":\"TransactionSuccessful\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"Unpaused\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"verifierAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"totalRegisteredVerifiers\",\"type\":\"uint256\"}],\"name\":\"VerifierRegistered\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"DepositVerifierAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"GetBlckHash\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"Name\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"Symbol\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"TotalRegisteredBanks\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"TotalSupply\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"VerifierAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"WithdrawVerifierAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"ZkdvpAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"acceptOwnership\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"}],\"name\":\"addBurnVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"}],\"name\":\"addDepositVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"}],\"name\":\"addFeeVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"p1x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"p1y\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"p2x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"p2y\",\"type\":\"uint256\"}],\"name\":\"addPedComm\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"}],\"name\":\"addUsdrVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"}],\"name\":\"addVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"splitCount\",\"type\":\"uint256\"}],\"name\":\"addWithdrawVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"zkDvp\",\"type\":\"address\"}],\"name\":\"addZkDvp\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"addressToAccountId\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"balanceCommitments\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"accountId\",\"type\":\"uint256\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[9]\",\"name\":\"public_signal\",\"type\":\"uint256[9]\"}],\"internalType\":\"structIEnygma.BurnProof\",\"name\":\"proof\",\"type\":\"tuple\"}],\"name\":\"burn\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"check\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"checkUsdr\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"confirmedFingerprint\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"commitmentDeltas\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[52]\",\"name\":\"public_signal\",\"type\":\"uint256[52]\"}],\"internalType\":\"structIEnygma.DepositProof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"components\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"internalType\":\"structIZkDvp.G1Point\",\"name\":\"a\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"uint256[2]\",\"name\":\"x\",\"type\":\"uint256[2]\"},{\"internalType\":\"uint256[2]\",\"name\":\"y\",\"type\":\"uint256[2]\"}],\"internalType\":\"structIZkDvp.G2Point\",\"name\":\"b\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"internalType\":\"structIZkDvp.G1Point\",\"name\":\"c\",\"type\":\"tuple\"}],\"internalType\":\"structIZkDvp.SnarkProof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"internalType\":\"uint256[]\",\"name\":\"statement\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256\",\"name\":\"numberOfInputs\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"numberOfOutputs\",\"type\":\"uint256\"}],\"internalType\":\"structIZkDvp.JoinSplitTransaction\",\"name\":\"transaction\",\"type\":\"tuple\"}],\"internalType\":\"structIEnygma.WithdrawParams\",\"name\":\"withdrawParam\",\"type\":\"tuple\"},{\"internalType\":\"uint256[]\",\"name\":\"participantIds\",\"type\":\"uint256[]\"}],\"name\":\"deposit\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"}],\"name\":\"derivePk\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"randomness\",\"type\":\"uint256\"}],\"name\":\"derivePkH\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"epochInterval\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"fingerprintConfirmed\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"accountId\",\"type\":\"uint256\"}],\"name\":\"getBalance\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"count\",\"type\":\"uint256\"}],\"name\":\"getPublicValues\",\"outputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"balances\",\"type\":\"tuple[]\"},{\"internalType\":\"uint256[]\",\"name\":\"keys\",\"type\":\"uint256[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"accountId\",\"type\":\"uint256\"}],\"name\":\"getUsdrBalance\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"count\",\"type\":\"uint256\"}],\"name\":\"getUsdrPublicValues\",\"outputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"balances\",\"type\":\"tuple[]\"},{\"internalType\":\"uint256[]\",\"name\":\"keys\",\"type\":\"uint256[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"initialize\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"accountId\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"initialUsdrCommitX\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"initialUsdrCommitY\",\"type\":\"uint256\"}],\"name\":\"initializeUsdrBalance\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"lastBlockNum\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"recipientId\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"mintCommitX\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"mintCommitY\",\"type\":\"uint256\"}],\"name\":\"mintSupply\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"recipientId\",\"type\":\"uint256\"}],\"name\":\"mintUsdrSupply\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"pause\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"paused\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"randomness\",\"type\":\"uint256\"}],\"name\":\"pedCom\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"pendingFingerprint\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"pendingFingerprintSet\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"pendingOwner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"protocolFee\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"publicKeys\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"addr\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"accountId\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"publicKey\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"initialCommitX\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"initialCommitY\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"viewKey\",\"type\":\"bytes\"}],\"name\":\"registerAccount\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"otherPartyId\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"fingerprint\",\"type\":\"uint256\"}],\"name\":\"registerFingerprint\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"newFee\",\"type\":\"uint256\"}],\"name\":\"setProtocolFee\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"setUsdrFixedFee\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalSupplyAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalSupplyX\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalSupplyY\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"commitmentDeltas\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[81]\",\"name\":\"public_signal\",\"type\":\"uint256[81]\"}],\"internalType\":\"structIEnygma.Proof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"usdrCommitmentDeltas\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[82]\",\"name\":\"public_signal\",\"type\":\"uint256[82]\"}],\"internalType\":\"structIEnygma.UsdrProof\",\"name\":\"usdrProof\",\"type\":\"tuple\"},{\"internalType\":\"uint256[]\",\"name\":\"participantIds\",\"type\":\"uint256[]\"},{\"internalType\":\"string\",\"name\":\"bankTag\",\"type\":\"string\"}],\"name\":\"transfer\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"commitmentDeltas\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[55]\",\"name\":\"public_signal\",\"type\":\"uint256[55]\"}],\"internalType\":\"structIEnygma.FeeProof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"internalType\":\"uint256[]\",\"name\":\"participantIds\",\"type\":\"uint256[]\"},{\"internalType\":\"string\",\"name\":\"bankTag\",\"type\":\"string\"}],\"name\":\"transferWithFee\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"unpause\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"usdrBalanceCommitments\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"usdrFixedFeeAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"usdrInitialized\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"usdrTotalSupplyAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"usdrTotalSupplyX\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"usdrTotalSupplyY\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"viewKeys\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"commitmentDeltas\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[52]\",\"name\":\"public_signal\",\"type\":\"uint256[52]\"}],\"internalType\":\"structIEnygma.WithdrawProof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"erc20Adress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"publicKey\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.DepositParams[]\",\"name\":\"depositParams\",\"type\":\"tuple[]\"},{\"internalType\":\"uint256[]\",\"name\":\"participantIds\",\"type\":\"uint256[]\"}],\"name\":\"withdraw\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"},{\"internalType\":\"uint256[]\",\"name\":\"\",\"type\":\"uint256[]\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
	Bin: "0x60a060405234801561001057600080fd5b506040516155ee3803806155ee83398101604081905261002f916100cd565b600081116100835760405162461bcd60e51b815260206004820152601960248201527f65706f6368496e74657276616c206d757374206265203e203000000000000000604482015260640160405180910390fd5b600180546001600160a01b031916331790556000805560808190526100a7816100b0565b6003555061012d565b6000816100bd81436100e6565b6100c79190610108565b92915050565b6000602082840312156100df57600080fd5b5051919050565b60008261010357634e487b7160e01b600052601260045260246000fd5b500490565b80820281158282048414176100c757634e487b7160e01b600052601160045260246000fd5b60805161549f61014f600039600081816103420152613296015261549f6000f3fe608060405234801561001057600080fd5b50600436106102f65760003560e01c80630197d942146102fb57806307da47ea1461032357806309b1ef261461033d5780630cf1839c14610372578063132ce4d4146103925780631660e58f146103b35780631a4e1aa1146103c65780631e010439146103d757806325e74eb3146103ea578063272fe53c146103f35780632c0457e8146103fc5780633045aaf31461040d578063368990421461042b57806337640ad5146104345780633f4ba83a146104475780634e466c531461044f57806352333894146104625780635a54f35f1461046a5780635c975abb1461047d5780635dcc76501461048f5780635fbaf841146104a257806367511a4d146104b557806367ec4693146104be5780636da4d2b8146104df5780636f5a2d541461050d57806371929e2a1461052e578063723dbbc414610537578063743873b41461054a578063787dce3d14610552578063795825a71461056557806379ba5097146105785780637d894a16146105805780638052474d146105935780638129fc1c146105b557806383914157146105bd5780638456cb59146105d057806384aaa2de146105d85780638718dcaa146105e0578063874ed5b5146105f35780638d909dd7146106045780638da5cb5b1461060d5780638f48f7b51461061e5780639000b3d614610649578063919840ad1461065c57806393c43876146106645780639edd41ff14610690578063a44b47f7146106a3578063a605841c146106ab578063a61f3ae4146106d6578063a8cc45d6146106e9578063a9c58a7e146106fc578063b0e21e8a1461070f578063c1ab48fc14610718578063c680f41014610738578063ce630c1814610758578063ce8cd4001461076b578063da171cf21461079d578063e30c3978146107c0578063e52dc188146107d1578063ea0d4573146107da578063ec4a09d01461080c578063edda4a0a1461081f578063f2fde38b14610832578063f828f50b14610845578063f83444341461084e578063fe877fc914610861575b600080fd5b61030e61030936600461481d565b610874565b60405190151581526020015b60405180910390f35b600f546001600160a01b03165b60405161031a9190614838565b6103647f000000000000000000000000000000000000000000000000000000000000000081565b60405190815260200161031a565b61038561038036600461484c565b61097b565b60405161031a91906148b5565b6103a56103a03660046148c8565b610a15565b60405161031a9291906148fa565b61030e6103c136600461484c565b610a32565b6010546001600160a01b0316610330565b6103a56103e536600461484c565b610a69565b610364600c5481565b610364600a5481565b600e546001600160a01b0316610330565b60408051808201909152600281526122a760f11b6020820152610385565b61036460035481565b61030e610442366004614908565b610abd565b61030e610c62565b61030e61045d36600461481d565b610cdb565b61030e610d9a565b61030e610478366004614934565b610da9565b600254600160a01b900460ff1661030e565b61030e61049d3660046149f7565b610f91565b61030e6104b0366004614aaf565b6111e6565b61036460065481565b6104d16104cc36600461484c565b6115a9565b60405161031a929190614b25565b61030e6104ed366004614934565b602260209081526000928352604080842090915290825290205460ff1681565b61052061051b366004614b89565b6116db565b60405161031a929190614c76565b61036460055481565b6103a561054536600461484c565b6118cd565b600354610364565b61030e61056036600461484c565b6118e2565b61030e610573366004614cda565b611982565b61030e611ae7565b6103a561058e366004614934565b611b75565b604080518082019091526006815265456e79676d6160d01b6020820152610385565b61030e611bb4565b61030e6105cb366004614d8b565b611c21565b61030e611e70565b600454610364565b61030e6105ee3660046148c8565b611ee1565b600d546001600160a01b0316610330565b610364600b5481565b6001546001600160a01b0316610330565b61036461062c366004614934565b602160209081526000928352604080842090915290825290205481565b61030e61065736600461481d565b6120a0565b61030e612196565b61030e610672366004614934565b60208080526000928352604080842090915290825290205460ff1681565b6103a561069e36600461484c565b6121a0565b600754610364565b6103646106b9366004614934565b601f60209081526000928352604080842090915290825290205481565b61030e6106e4366004614934565b6121df565b61030e6106f7366004614e0c565b612388565b6104d161070a36600461484c565b612517565b61036460085481565b61036461072636600461481d565b60186020526000908152604090205481565b61036461074636600461484c565b60166020526000908152604090205481565b6103a561076636600461484c565b612643565b6103a5610779366004614934565b60146020908152600092835260408084209091529082529020805460019091015482565b61030e6107ab36600461484c565b60156020526000908152604090205460ff1681565b6002546001600160a01b0316610330565b61036460095481565b6103a56107e8366004614934565b60136020908152600092835260408084209091529082529020805460019091015482565b61030e61081a36600461481d565b61264f565b61030e61082d36600461481d565b61273a565b61030e61084036600461481d565b6127f9565b61036460075481565b61030e61085c36600461481d565b6128a5565b61030e61086f366004614f0c565b612970565b6001546000906001600160a01b031633146108a2576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b0382166108c95760405163d92e233d60e01b815260040160405180910390fd5b816001600160a01b03163b6000036108f4576040516362d4176d60e11b815260040160405180910390fd5b6006600052601c6020527fb2d730c3da277545f6c9f2c922ea3d0e9fcbd74e29d321b995a837ef61c283f080546001600160a01b0384166001600160a01b03199182168117909255600f8054909116821790556004546040516000805160206153ca8339815191529161096a9190815260200190565b60405180910390a25060015b919050565b6017602052600090815260409020805461099490614f36565b80601f01602080910402602001604051908101604052809291908181526020018280546109c090614f36565b8015610a0d5780601f106109e257610100808354040283529160200191610a0d565b820191906000526020600020905b8154815290600101906020018083116109f057829003601f168201915b505050505081565b600080610a2486868686612a5c565b915091505b94509492505050565b6001546000906001600160a01b03163314610a60576040516330cd747160e01b815260040160405180910390fd5b50600c55600190565b600354600090815260136020908152604080832084845290915281208054829190158015610a9957506001810154155b15610aab575060009360019350915050565b80546001909101549094909350915050565b6001546000906001600160a01b03163314610aeb576040516330cd747160e01b815260040160405180910390fd5b600160005414610b0e576040516321c4e35760e21b815260040160405180910390fd5b6000848152601660205260408120549003610b3c5760405163c669128160e01b815260040160405180910390fd5b60008481526015602052604090205460ff1615610b6c57604051630ea075bf60e21b815260040160405180910390fd5b600354600090815260146020908152604080832087845290915290208054151580610baa5750600181015415801590610baa57508060010154600114155b15610bc857604051630ea075bf60e21b815260040160405180910390fd5b610bd28484612bad565b610bef5760405163ecd2690d60e01b815260040160405180910390fd5b6000858152601560209081526040808320805460ff19166001908117909155815180830183528881528084018881526003548652601485528386208b875290945291909320905181559051910155600954600a54610c4f91908686612a5c565b600a5560095550600190505b9392505050565b6001546000906001600160a01b03163314610c90576040516330cd747160e01b815260040160405180910390fd5b6002805460ff60a01b191690556040517f5db9ee0a495bf2e6ff9c91a7834c1ba4fdd244a5e8aa4e537bd38aeae4b073aa90610ccd903390614838565b60405180910390a150600190565b6001546000906001600160a01b03163314610d09576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b038216610d305760405163d92e233d60e01b815260040160405180910390fd5b816001600160a01b03163b600003610d5b576040516362d4176d60e11b815260040160405180910390fd5b601180546001600160a01b0319166001600160a01b0384169081179091556004546040519081526000805160206153ca8339815191529060200161096a565b6000610da4612bea565b905090565b336000908152601860205260408120548103610dd85760405163aba4733960e01b815260040160405180910390fd5b600254600160a01b900460ff1615610e03576040516306d39fcd60e41b815260040160405180910390fd5b33600090815260186020526040902054831580610e1f57508084145b15610e3d5760405163a5c3e7e160e01b815260040160405180910390fd5b6000818152601f602090815260408083208784528252808320869055838352818052808320878452825291829020805460ff191660011790559051848152859183917f2244c7409c0d13a2d9db63e7bfad1826bd85a2d6052ad998f150d2aab30c0d1f910160405180910390a36000848152601f602090815260408083208484528252808320548784528280528184208585529092529091205460ff168015610ee557508381145b15610f8457600082815260216020818152604080842089855282528084208890559181528183208584528152818320879055602280825282842089855282528284208054600160ff199182168117909255918352838520878652835293839020805490911690931790925551858152869184917fe2070e33257d0615b4d0bff5bc34dcc22292a2ca3973074cc97d92dcc884549d910160405180910390a35b6001925050505b92915050565b336000908152601860205260408120548103610fc05760405163aba4733960e01b815260040160405180910390fd5b600160005414610fe3576040516321c4e35760e21b815260040160405180910390fd5b600254600160a01b900460ff161561100e576040516306d39fcd60e41b815260040160405180910390fd5b600254600160a81b900460ff161561103957604051633ee5aeb560e01b815260040160405180910390fd5b6002805460ff60a81b1916600160a81b1790556000868152601c60205260409020546001600160a01b03168061108257604051633896c50b60e21b815260040160405180910390fd5b806001600160a01b03163b6000036110ad576040516362d4176d60e11b815260040160405180910390fd5b6110f181876040516024016110c29190614f82565b60408051601f198184030181529190526020810180516001600160e01b0316630f948af160e01b179052612c60565b611102866101000185858b8b612d2e565b61110f8661010001612fa7565b61111c8661010001612fd4565b61112888888686613034565b611132888861314b565b6010546001600160a01b031680634ac058ed61114e8880614f91565b6040518263ffffffff1660e01b815260040161116a9190614fe4565b6020604051808303816000875af1158015611189573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906111ad91906150bc565b6111ca5760405163068fdd5760e41b815260040160405180910390fd5b50506002805460ff60a81b191690555060019695505050505050565b6001546000906001600160a01b03163314611214576040516330cd747160e01b815260040160405180910390fd5b600160005414611237576040516321c4e35760e21b815260040160405180910390fd5b600254600160a01b900460ff1615611262576040516306d39fcd60e41b815260040160405180910390fd5b6012546001600160a01b03168061128c57604051633896c50b60e21b815260040160405180910390fd5b806001600160a01b03163b6000036112b7576040516362d4176d60e11b815260040160405180910390fd5b6112fb81846040516024016112cc91906150d7565b60408051601f198184030181529190526020810180516001600160e01b03166311475c8760e31b179052612c60565b304660a01b1761020084013514611325576040516375893cc160e11b815260040160405180910390fd5b60008481526016602052604090205461010084013514611358576040516319dcebfb60e21b815260040160405180910390fd5b60008061136486610a69565b9092509050610120850135821415806113a15750806101008601611389600180615122565b60098110611399576113996150f6565b602002013514155b156113bf576040516319dcebfb60e21b815260040160405180910390fd5b6101a085013560008051602061544a83398151915281106113f3576040516304b4b91960e11b815260040160405180910390fd5b6003546101c08701351461141a57604051631391e11b60e21b815260040160405180910390fd5b6101e08601356000818152601e602052604090205460ff161561145057604051636569570160e11b815260040160405180910390fd5b6000818152601e60205260409020805460ff19166001179055611472886131b6565b61147c6000613224565b6000611486613292565b9050604051806040016040528089610100016003600981106114aa576114aa6150f6565b602002013581526020018961010001600360016114c79190615122565b600981106114d7576114d76150f6565b6020908102919091013590915260008381526013825260408082208d83528352812083518155929091015160019092019190915560038290558061153361152c8660008051602061544a833981519152615135565b6000611b75565b915091506115476005546006548484612a5c565b6006556005556007805486900390556040517f262a9a1794440b6af993000f5805d7f51b5a19d4c32fcb10a1c5216beb0616f490611588908d9088906148fa565b60405180910390a16115986132c9565b5060019a9950505050505050505050565b606080826001600160401b038111156115c4576115c4615148565b6040519080825280602002602001820160405280156115fd57816020015b6115ea6147ec565b8152602001906001900390816115e25790505b509150826001600160401b0381111561161857611618615148565b604051908082528060200260200182016040528015611641578160200160208202803683370190505b50905060005b838110156116d557611658816121a0565b84838151811061166a5761166a6150f6565b6020026020010151600001858481518110611687576116876150f6565b60200260200101516020018281525082815250505060166000828152602001908152602001600020548282815181106116c2576116c26150f6565b6020908102919091010152600101611647565b50915091565b33600090815260186020526040812054606090820361170d5760405163aba4733960e01b815260040160405180910390fd5b600160005414611730576040516321c4e35760e21b815260040160405180910390fd5b600254600160a01b900460ff161561175b576040516306d39fcd60e41b815260040160405180910390fd5b600254600160a81b900460ff161561178657604051633ee5aeb560e01b815260040160405180910390fd5b6002805460ff60a81b1916600160a81b1790558288146117b95760405163023f995760e61b815260040160405180910390fd5b600688146117da576040516354fb304560e01b815260040160405180910390fd5b6000888152601b60205260409020546001600160a01b03168061181057604051633896c50b60e21b815260040160405180910390fd5b806001600160a01b03163b60000361183b576040516362d4176d60e11b815260040160405180910390fd5b61185081896040516024016110c29190614f82565b611861886101000186868d8d612d2e565b61186e8861010001612fa7565b61187b8861010001612fd4565b61188b87876107408b01356132d1565b6118978a8a8787613034565b6118a18a8a61314b565b60006118ad888861332c565b6002805460ff60a81b1916905560019c909b509950505050505050505050565b6000806118d983613518565b91509150915091565b6001546000906001600160a01b03163314611910576040516330cd747160e01b815260040160405180910390fd5b60008051602061544a833981519152821061193e5760405163bb22c5a960e01b815260040160405180910390fd5b7fb404cac19fb1cbeff98d325795b08886e3cd8fe8cb1a2f193aac66f13fb239c3600854836040516119719291906148fa565b60405180910390a150600855600190565b3360009081526018602052604081205481036119b15760405163aba4733960e01b815260040160405180910390fd5b6001600054146119d4576040516321c4e35760e21b815260040160405180910390fd5b600254600160a01b900460ff16156119ff576040516306d39fcd60e41b815260040160405180910390fd5b611a08866135b1565b611a19866101000186868b8b613658565b611a2686610100016138c6565b611a3386610100016138d1565b600854610740870135908114611a5c576040516358d620b360e01b815260040160405180910390fd5b611a65816138db565b6000611a6f613292565b9050611a7e818b8b8a8a61395f565b611a886000613224565b600381905560405133906000805160206153aa83398151915290600090a2336001600160a01b03166000805160206153ea8339815191528686604051611acf92919061515e565b60405180910390a25060019998505050505050505050565b6002546000906001600160a01b03163314611b1557604051630614e5c760e21b815260040160405180910390fd5b600180546001600160a01b0319808216339081179093556002805490911690556040516001600160a01b03909116919082907f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e090600090a3600191505090565b600080600080611b84866118cd565b91509150600080611b9487612643565b91509150611ba484848484612a5c565b95509550505050505b9250929050565b6001546000906001600160a01b03163314611be2576040516330cd747160e01b815260040160405180910390fd5b600160005403611c045760405162dc149f60e41b815260040160405180910390fd5b506001600081815560058190556006829055600955600a81905590565b6001546000906001600160a01b03163314611c4f576040516330cd747160e01b815260040160405180910390fd5b600160005414611c72576040516321c4e35760e21b815260040160405180910390fd5b600254600160a01b900460ff1615611c9d576040516306d39fcd60e41b815260040160405180910390fd5b60008781526016602052604090205415611cca57604051630ea075bf60e21b815260040160405180910390fd5b86600003611ceb57604051630d57928360e21b815260040160405180910390fd5b600454611cf9906001615122565b8714611d1857604051630d57928360e21b815260040160405180910390fd5b85600003611d395760405163145a1fdd60e31b815260040160405180910390fd5b8115801590611d4a57506104a08214155b15611d685760405163759f482960e11b815260040160405180910390fd5b611d728585612bad565b611d8f5760405163ecd2690d60e01b815260040160405180910390fd5b600087815260166020908152604080832089905560179091529020611db58385836151d4565b506001600160a01b03881660009081526018602090815260408083208a9055805180820182528881528083018881526003548552601384528285208c865290935292209151825551600190910155600554600654611e1591908787612a5c565b6006556005556004805460010190556040518781526001600160a01b038916907fefd1ddef00b1051abc144c2e895de70a10dbbc3ad8985118c74c15e40e3d391f9060200160405180910390a2506001979650505050505050565b6001546000906001600160a01b03163314611e9e576040516330cd747160e01b815260040160405180910390fd5b6002805460ff60a01b1916600160a01b1790556040517f62e78cea01bee320cd4e420270b5ea74000d11b0c9f74754ebdbfc544b05a25890610ccd903390614838565b6001546000906001600160a01b03163314611f0f576040516330cd747160e01b815260040160405180910390fd5b600160005414611f32576040516321c4e35760e21b815260040160405180910390fd5b600254600160a01b900460ff1615611f5d576040516306d39fcd60e41b815260040160405180910390fd5b6000848152601660205260408120549003611f8b5760405163c669128160e01b815260040160405180910390fd5b611f958383612bad565b611fb25760405163ecd2690d60e01b815260040160405180910390fd5b611fc26005546006548585612a5c565b6006556005556007805486019055611fd9846131b6565b611fe36000613224565b60035460009081526013602090815260408083208784529091528120805460018201549192918291612016918888612a5c565b915091506000612024613292565b60408051808201825285815260208082018681526000858152601383528481208e8252909252908390209151825551600190910155600382905551909150819060008051602061540a83398151915290612081908c908c906148fa565b60405180910390a26120916132c9565b50600198975050505050505050565b6001546000906001600160a01b031633146120ce576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b0382166120f55760405163d92e233d60e01b815260040160405180910390fd5b816001600160a01b03163b600003612120576040516362d4176d60e11b815260040160405180910390fd5b600660005260196020527f4ca1d5f267eb2abf27b670f18ee76f6c205873a2168eb2e690c8c4babcc357dd80546001600160a01b0384166001600160a01b03199182168117909255600d8054909116821790556004546040516000805160206153ca8339815191529161096a9190815260200190565b6000610da4613a4f565b600354600090815260146020908152604080832084845290915281208054829190158015610a9957506001810154610aab575060009360019350915050565b6001546000906001600160a01b0316331461220d576040516330cd747160e01b815260040160405180910390fd5b600160005414612230576040516321c4e35760e21b815260040160405180910390fd5b600254600160a01b900460ff161561225b576040516306d39fcd60e41b815260040160405180910390fd5b60008281526016602052604081205490036122895760405163c669128160e01b815260040160405180910390fd5b600080612295856118cd565b915091506122a9600954600a548484612a5c565b600a55600955600b8054860190556122c084613224565b6122ca60006131b6565b600354600090815260146020908152604080832087845290915281208054600182015491929182916122fd918787612a5c565b91509150600061230b613292565b60408051808201825285815260208082018681526000858152601483528481208e8252909252908390209151825551600190910155600382905551909150819060008051602061540a83398151915290612368908c908c906148fa565b60405180910390a2612378612bea565b5060019998505050505050505050565b3360009081526018602052604081205481036123b75760405163aba4733960e01b815260040160405180910390fd5b6001600054146123da576040516321c4e35760e21b815260040160405180910390fd5b600254600160a01b900460ff1615612405576040516306d39fcd60e41b815260040160405180910390fd5b61240f898b613ab9565b6124198688613b63565b61242b89610100018761010001613c08565b61243c896101000186868e8e613daa565b61244d866101000186868b8b614018565b61245c896101000186866142ac565b61246989610100016143d3565b61247686610100016143d3565b61248389610100016143de565b61249086610100016143de565b600061249a613292565b90506124a9818d8d898961395f565b6124b6818a8a89896143e8565b600381905560405133906000805160206153aa83398151915290600090a2336001600160a01b03166000805160206153ea83398151915285856040516124fd92919061515e565b60405180910390a25060019b9a5050505050505050505050565b606080826001600160401b0381111561253257612532615148565b60405190808252806020026020018201604052801561256b57816020015b6125586147ec565b8152602001906001900390816125505790505b509150826001600160401b0381111561258657612586615148565b6040519080825280602002602001820160405280156125af578160200160208202803683370190505b50905060005b838110156116d5576125c681610a69565b8483815181106125d8576125d86150f6565b60200260200101516000018584815181106125f5576125f56150f6565b6020026020010151602001828152508281525050506016600082815260200190815260200160002054828281518110612630576126306150f6565b60209081029190910101526001016125b5565b6000806118d98361452a565b6001546000906001600160a01b0316331461267d576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b0382166126a45760405163d92e233d60e01b815260040160405180910390fd5b816001600160a01b03163b6000036126cf576040516362d4176d60e11b815260040160405180910390fd5b6006600052601a6020527fe7513fcd4f864b78baf560f46c15980b6aa41b90911efc4ce7454b83cce613b180546001600160a01b0384166001600160a01b031990911681179091556004546040516000805160206153ca8339815191529161096a9190815260200190565b6001546000906001600160a01b03163314612768576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b03821661278f5760405163d92e233d60e01b815260040160405180910390fd5b816001600160a01b03163b6000036127ba576040516362d4176d60e11b815260040160405180910390fd5b601280546001600160a01b0319166001600160a01b0384169081179091556004546040519081526000805160206153ca8339815191529060200161096a565b6001546000906001600160a01b03163314612827576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b03821661284e57604051633a247dd760e11b815260040160405180910390fd5b600280546001600160a01b0319166001600160a01b03848116918217909255600154604051919216907f38d16b8cac22d99fc7c124b9cd0de2d3fa1faef420bfe791d8c362d765e2270090600090a3506001919050565b6001546000906001600160a01b031633146128d3576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b0382166128fa5760405163d92e233d60e01b815260040160405180910390fd5b6006600052601d6020527fe05f340630215c2ef0683a72fde801183a1a4ecac14ded57e11c794e9bcb206980546001600160a01b0384166001600160a01b0319918216811790925560108054909116821790556004546040516000805160206153ca8339815191529161096a9190815260200190565b6001546000906001600160a01b0316331461299e576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b0383166129c55760405163d92e233d60e01b815260040160405180910390fd5b826001600160a01b03163b6000036129f0576040516362d4176d60e11b815260040160405180910390fd5b6000828152601b60205260409081902080546001600160a01b0386166001600160a01b03199182168117909255600e805490911682179055600454915190916000805160206153ca83398151915291612a4b91815260200190565b60405180910390a250600192915050565b60008085158015612a6d5750846001145b15612a7c575082905081610a29565b83158015612a8a5750826001145b15612a99575084905083610a29565b600060008051602061542a8339815191528588099050600060008051602061542a8339815191528588099050600060008051602061542a83398151915280838509620292f8099050600060008051602061542a83398151915280898b0960008051602061542a833981519152898d090890506000612b3a8460008051602061542a83398151915287620292fc0960008051602061542a8339815191526145b6565b905060008051602061542a833981519152612b6660008051602061542a833981519152856001086145f1565b8309965060008051602061542a833981519152612b9b612b9660018660008051602061542a8339815191526145b6565b6145f1565b82099550505050505094509492505050565b600060008051602061542a83398151915283108015612bd9575060008051602061542a83398151915282105b8015610c5b5750610c5b8383614624565b6000806001805b6004548111612c2557600080612c06836121a0565b91509150612c1685858484612a5c565b90955093505050600101612bf1565b5081600954141580612c39575080600a5414155b15612c5757604051631947c14d60e31b815260040160405180910390fd5b60019250505090565b600080836001600160a01b031683604051612c7b91906152a9565b600060405180830381855afa9150503d8060008114612cb6576040519150601f19603f3d011682016040523d82523d6000602084013e612cbb565b606091505b509150915081612cde576040516309bde33960e01b815260040160405180910390fd5b805115612d285780516020141580612d0a575080806020019051810190612d0591906152bb565b600114155b15612d28576040516309bde33960e01b815260040160405180910390fd5b50505050565b304660a01b1761066086013514612d58576040516375893cc160e11b815260040160405180910390fd5b600080612d6d600454600161070a9190615122565b90925090508460005b81811015612f9c576000888883818110612d9257612d926150f6565b905060200201359050838181518110612dad57612dad6150f6565b6020026020010151600003612dd55760405163c669128160e01b815260040160405180910390fd5b838181518110612de757612de76150f6565b60200260200101518a836006612dfd9190615122565b60348110612e0d57612e0d6150f6565b602002013514612e30576040516319dcebfb60e21b815260040160405180910390fd5b6000612e41600184901b600c615122565b9050858281518110612e5557612e556150f6565b6020026020010151600001518b8260348110612e7357612e736150f6565b6020020135141580612ec45750858281518110612e9257612e926150f6565b6020026020010151602001518b826001612eac9190615122565b60348110612ebc57612ebc6150f6565b602002013514155b15612ee2576040516319dcebfb60e21b815260040160405180910390fd5b6000612ef3600185901b6018615122565b90508b8160348110612f0757612f076150f6565b6020020135898986818110612f1e57612f1e6150f6565b90506040020160000135141580612f7057508b612f3c826001615122565b60348110612f4c57612f4c6150f6565b6020020135898986818110612f6357612f636150f6565b9050604002016020013514155b15612f8e576040516319dcebfb60e21b815260040160405180910390fd5b836001019350505050612d76565b505050505050505050565b6003548160245b602002013514612fd157604051631391e11b60e21b815260040160405180910390fd5b50565b60008160315b602090810291909101356000818152601e90925260409091205490915060ff161561301857604051636569570160e11b815260040160405180910390fd5b6000908152601e60205260409020805460ff1916600117905550565b808381146130555760405163023f995760e61b815260040160405180910390fd5b61305f60006131b6565b6000613069613292565b90506000805b8381101561313557600086868381811061308b5761308b6150f6565b9050602002013590508281116130b45760405163f170f72d60e01b815260040160405180910390fd5b60008481526013602090815260408083208484529091528120805460018201549395508593919291829161311f918e8e898181106130f4576130f46150f6565b905060400201600001358f8f8a818110613110576131106150f6565b90506040020160200135612a5c565b908455600193840155505091909101905061306f565b506131406000613224565b506003555050505050565b60006001815b838110156131995761318c8383878785818110613170576131706150f6565b90506040020160000135888886818110613110576131106150f6565b9093509150600101613151565b506131aa6005546006548484612a5c565b60065560055550505050565b60006131c0613292565b60045490915060015b818111612d28576131d9816146ce565b83811461321c5760035460009081526013602081815260408084208585528252808420878552928252808420858552909152909120815481556001918201549101555b6001016131c9565b600061322e613292565b60045490915060015b818111612d28576132478161470b565b83811461328a5760035460009081526014602081815260408084208585528252808420878552928252808420858552909152909120815481556001918201549101555b600101613237565b60007f00000000000000000000000000000000000000000000000000000000000000006132bf81436152d4565b610da491906152f6565b612fd1613a4f565b6000805b8381101561330c578484828181106132ef576132ef6150f6565b6133029260609091020135905083615122565b91506001016132d5565b50818114612d28576040516202aef760e91b815260040160405180910390fd5b6010546060906001600160a01b0316826000816001600160401b0381111561335657613356615148565b60405190808252806020026020018201604052801561337f578160200160208202803683370190505b50905060005b8281101561350e576040805160028082526060820183526000926020830190803683370190505090508787838181106133c0576133c06150f6565b90506060020160000135816000815181106133dd576133dd6150f6565b6020026020010181815250508787838181106133fb576133fb6150f6565b9050606002016040013581600181518110613418576134186150f6565b602002602001018181525050600080866001600160a01b03166383bf2edd846040518263ffffffff1660e01b8152600401613453919061530d565b60408051808303816000875af1158015613471573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906134959190615320565b91509150816134b75760405163068fdd5760e41b815260040160405180910390fd5b808585815181106134ca576134ca6150f6565b602090810291909101015260405181907fef61e988d9804d573b4fc504760f55d3507094e4168fddc9245ac56fbfc419e490600090a2836001019350505050613385565b5095945050505050565b600080827f1b46f45118b90335391ae7f66ffe16bdf117c9b9be79e1802d987d2347fcb12b7f21a94082e95c6df187baa045c30f4cb20e13eeb4b9ef2ef96bc2659242d50cda8360015b84156135a45760018516156135835761357d82828686612a5c565b90925090505b61358d8484614743565b909450925061359d6002866152d4565b9450613562565b9097909650945050505050565b6011546001600160a01b03166135da57604051633896c50b60e21b815260040160405180910390fd5b6011546001600160a01b03163b600003613607576040516362d4176d60e11b815260040160405180910390fd5b601154604051612fd1916001600160a01b03169061362990849060240161534c565b60408051601f198184030181529190526020810180516001600160e01b031663f6fbf48960e01b179052612c60565b304660a01b176106c086013514613682576040516375893cc160e11b815260040160405180910390fd5b600080613697600454600161070a9190615122565b90925090508460005b81811015612f9c5760008888838181106136bc576136bc6150f6565b9050602002013590508381815181106136d7576136d76150f6565b60200260200101516000036136ff5760405163c669128160e01b815260040160405180910390fd5b838181518110613711576137116150f6565b60200260200101518a8360066137279190615122565b60378110613737576137376150f6565b60200201351461375a576040516319dcebfb60e21b815260040160405180910390fd5b600061376b600184901b600c615122565b905085828151811061377f5761377f6150f6565b6020026020010151600001518b826037811061379d5761379d6150f6565b60200201351415806137ee57508582815181106137bc576137bc6150f6565b6020026020010151602001518b8260016137d69190615122565b603781106137e6576137e66150f6565b602002013514155b1561380c576040516319dcebfb60e21b815260040160405180910390fd5b600061381d600185901b6018615122565b90508b8160378110613831576138316150f6565b6020020135898986818110613848576138486150f6565b9050604002016000013514158061389a57508b613866826001615122565b60378110613876576138766150f6565b602002013589898681811061388d5761388d6150f6565b9050604002016020013514155b156138b8576040516319dcebfb60e21b815260040160405180910390fd5b8360010193505050506136a0565b600354816024612fae565b6000816031612fda565b806000036138e65750565b60008061390461152c8460008051602061544a833981519152615135565b915091506139186005546006548484612a5c565b6006556005556007805484900390556040518381527fa551808c565cfbf20dfffdbcd44c549f835f9d06a82dcd546c61644b2f5ce7919060200160405180910390a1505050565b808381146139805760405163023f995760e61b815260040160405180910390fd5b61398a60006131b6565b6000805b82811015613a455760008585838181106139aa576139aa6150f6565b9050602002013590508281116139d35760405163f170f72d60e01b815260040160405180910390fd5b600089815260136020908152604080832084845290915281208054600182015493955085939192918291613a2f918d8d89818110613a1357613a136150f6565b905060400201600001358e8e8a818110613110576131106150f6565b908455600193840155505091909101905061398e565b5050505050505050565b6000806001805b6004548111613a8a57600080613a6b83610a69565b91509150613a7b85858484612a5c565b90955093505050600101613a56565b5081600554141580612c3957508060065414612c5757604051631947c14d60e31b815260040160405180910390fd5b6000818152601960205260409020546001600160a01b031680613aef57604051633896c50b60e21b815260040160405180910390fd5b806001600160a01b03163b600003613b1a576040516362d4176d60e11b815260040160405180910390fd5b613b5e8184604051602401613b2f919061536b565b60408051601f198184030181529190526020810180516001600160e01b0316633fdaa96b60e11b179052612c60565b505050565b6000818152601a60205260409020546001600160a01b031680613b9957604051633896c50b60e21b815260040160405180910390fd5b806001600160a01b03163b600003613bc4576040516362d4176d60e11b815260040160405180910390fd5b613b5e8184604051602401613bd9919061538a565b60408051601f198184030181529190526020810180516001600160e01b0316635d964ab160e01b179052612c60565b60245b613c1760066024615122565b811015613c7257818160528110613c3057613c306150f6565b6020020135838260518110613c4757613c476150f6565b602002013514613c6a57604051630663ff5760e11b815260040160405180910390fd5b600101613c0b565b5060435b613c8260066043615122565b811015613cdd57818160528110613c9b57613c9b6150f6565b6020020135838260518110613cb257613cb26150f6565b602002013514613cd557604051630663ff5760e11b815260040160405180910390fd5b600101613c76565b50610840828101359082013514613d0757604051630663ff5760e11b815260040160405180910390fd5b60005b6006811015613b5e5760005b6006811015613da157808214613d9957600081613d346006856152f6565b613d3f906000615122565b613d499190615122565b9050838160528110613d5d57613d5d6150f6565b6020020135858260518110613d7457613d746150f6565b602002013514613d9757604051630663ff5760e11b815260040160405180910390fd5b505b600101613d16565b50600101613d0a565b304660a01b17610a0086013514613dd4576040516375893cc160e11b815260040160405180910390fd5b600080613de9600454600161070a9190615122565b90925090508460005b81811015612f9c576000888883818110613e0e57613e0e6150f6565b905060200201359050838181518110613e2957613e296150f6565b6020026020010151600003613e515760405163c669128160e01b815260040160405180910390fd5b838181518110613e6357613e636150f6565b60200260200101518a836024613e799190615122565b60518110613e8957613e896150f6565b602002013514613eac576040516319dcebfb60e21b815260040160405180910390fd5b6000613ebd600184901b602a615122565b9050858281518110613ed157613ed16150f6565b6020026020010151600001518b8260518110613eef57613eef6150f6565b6020020135141580613f405750858281518110613f0e57613f0e6150f6565b6020026020010151602001518b826001613f289190615122565b60518110613f3857613f386150f6565b602002013514155b15613f5e576040516319dcebfb60e21b815260040160405180910390fd5b6000613f6f600185901b6036615122565b90508b8160518110613f8357613f836150f6565b6020020135898986818110613f9a57613f9a6150f6565b90506040020160000135141580613fec57508b613fb8826001615122565b60518110613fc857613fc86150f6565b6020020135898986818110613fdf57613fdf6150f6565b9050604002016020013514155b1561400a576040516319dcebfb60e21b815260040160405180910390fd5b836001019350505050613df2565b304660a01b17610a2086013514614042576040516375893cc160e11b815260040160405180910390fd5b600c54610a00860135146140685760405162a4671960e71b815260040160405180910390fd5b60008061407d60045460016104cc9190615122565b90925090508460005b81811015612f9c5760008888838181106140a2576140a26150f6565b9050602002013590508381815181106140bd576140bd6150f6565b60200260200101516000036140e55760405163c669128160e01b815260040160405180910390fd5b8381815181106140f7576140f76150f6565b60200260200101518a83602461410d9190615122565b6052811061411d5761411d6150f6565b602002013514614140576040516319dcebfb60e21b815260040160405180910390fd5b6000614151600184901b602a615122565b9050858281518110614165576141656150f6565b6020026020010151600001518b8260528110614183576141836150f6565b60200201351415806141d457508582815181106141a2576141a26150f6565b6020026020010151602001518b8260016141bc9190615122565b605281106141cc576141cc6150f6565b602002013514155b156141f2576040516319dcebfb60e21b815260040160405180910390fd5b6000614203600185901b6036615122565b90508b8160528110614217576142176150f6565b602002013589898681811061422e5761422e6150f6565b9050604002016000013514158061428057508b61424c826001615122565b6052811061425c5761425c6150f6565b6020020135898986818110614273576142736150f6565b9050604002016020013514155b1561429e576040516319dcebfb60e21b815260040160405180910390fd5b836001019350505050614086565b8060005b818110156143cc5760008484838181106142cc576142cc6150f6565b90506020020135905060005b838110156143c2578083146143ba5760008686838181106142fb576142fb6150f6565b6000868152602260209081526040808320938202959095013580835292905292909220549192505060ff16614343576040516310d9346760e21b815260040160405180910390fd5b60008261435087876152f6565b61435b906000615122565b6143659190615122565b6000858152602160209081526040808320868452909152902054909150898260518110614394576143946150f6565b6020020135146143b7576040516319dcebfb60e21b815260040160405180910390fd5b50505b6001016142d8565b50506001016142b0565b5050505050565b600354816042612fae565b600081604f612fda565b60045460015b818111614451576143fe8161470b565b61440984848361475d565b61444957600354600090815260146020818152604080842085855282528084208b8552928252808420858552909152909120815481556001918201549101555b6001016143ee565b508360005b81811015613a45576000858583818110614472576144726150f6565b9050602002013590506000601460006003548152602001908152602001600020600083815260200190815260200160002090506000806144c7836000015484600101548d8d89818110613a1357613a136150f6565b91509150604051806040016040528083815260200182815250601460008e81526020019081526020016000206000868152602001908152602001600020600082015181600001556020820151816001015590505084600101945050505050614456565b600080827f16546696a66928d34f6be843f8a5afa2063161d92742811279454d60de5322527f109c1c7a758b3e8e54af1ce919fc24e1b986aab09a6b8082600f8694bb3c1b4b8360015b84156135a45760018516156145955761458f82828686612a5c565b90925090505b61459f8484614743565b90945092506145af6002866152d4565b9450614574565b6000838381116145cd576145ca8382615122565b90505b82806145db576145db615293565b60006145e78684615135565b0895945050505050565b6000610f8b82614610600260008051602061542a833981519152615135565b60008051602061542a8339815191526147a8565b60008060008051602061542a8339815191528485099050600060008051602061542a8339815191528485099050600060008051602061542a8339815191528260008051602061542a83398151915285620292fc09089050600060008051602061542a833981519152808460008051602061542a83398151915287620292f8090960010890506146c2828260008051602061542a8339815191526145b6565b15979650505050505050565b6003546000908152601360209081526040808320848452909152902080541580156146fb57506001810154155b15614707576001818101555b5050565b6003546000908152601460209081526040808320848452909152902080541580156146fb575060018101546147075760019081015550565b60008061475284848686612a5c565b915091509250929050565b600082815b8181101561479c578386868381811061477d5761477d6150f6565b905060200201350361479457600192505050610c5b565b600101614762565b50600095945050505050565b600060405160208152602080820152602060408201528460608201528360808201528260a082015260208160c08360055afa8080156102f657505051949350505050565b604051806040016040528060008152602001600081525090565b80356001600160a01b038116811461097657600080fd5b60006020828403121561482f57600080fd5b610c5b82614806565b6001600160a01b0391909116815260200190565b60006020828403121561485e57600080fd5b5035919050565b60005b83811015614880578181015183820152602001614868565b50506000910152565b600081518084526148a1816020860160208601614865565b601f01601f19169290920160200192915050565b602081526000610c5b6020830184614889565b600080600080608085870312156148de57600080fd5b5050823594602084013594506040840135936060013592509050565b918252602082015260400190565b60008060006060848603121561491d57600080fd5b505081359360208301359350604090920135919050565b6000806040838503121561494757600080fd5b50508035926020909101359150565b60008083601f84011261496857600080fd5b5081356001600160401b0381111561497f57600080fd5b6020830191508360208260061b8501011115611bad57600080fd5b600061078082840312156149ad57600080fd5b50919050565b60008083601f8401126149c557600080fd5b5081356001600160401b038111156149dc57600080fd5b6020830191508360208260051b8501011115611bad57600080fd5b6000806000806000806107e08789031215614a1157600080fd5b86356001600160401b03811115614a2757600080fd5b614a3389828a01614956565b9097509550614a479050886020890161499a565b93506107a08701356001600160401b03811115614a6357600080fd5b87016020818a031215614a7557600080fd5b92506107c08701356001600160401b03811115614a9157600080fd5b614a9d89828a016149b3565b979a9699509497509295939492505050565b600080828403610240811215614ac457600080fd5b83359250610220601f1982011215614adb57600080fd5b506020830190509250929050565b600081518084526020840193506020830160005b82811015614b1b578151865260209586019590910190600101614afd565b5093949350505050565b6040808252835190820181905260009060208501906060840190835b81811015614b6b578351805184526020908101518185015290930192604090920191600101614b41565b50508381036020850152614b7f8186614ae9565b9695505050505050565b60008060008060008060006107e0888a031215614ba557600080fd5b87356001600160401b03811115614bbb57600080fd5b614bc78a828b01614956565b9098509650614bdb90508960208a0161499a565b94506107a08801356001600160401b03811115614bf757600080fd5b8801601f81018a13614c0857600080fd5b80356001600160401b03811115614c1e57600080fd5b8a6020606083028401011115614c3357600080fd5b602091909101945092506107c08801356001600160401b03811115614c5757600080fd5b614c638a828b016149b3565b989b979a50959850939692959293505050565b8215158152604060208201526000614c916040830184614ae9565b949350505050565b60008083601f840112614cab57600080fd5b5081356001600160401b03811115614cc257600080fd5b602083019150836020828501011115611bad57600080fd5b6000806000806000806000878903610840811215614cf757600080fd5b88356001600160401b03811115614d0d57600080fd5b614d198b828c01614956565b9099509750506107e0601f1982011215614d3257600080fd5b506020880194506108008801356001600160401b03811115614d5357600080fd5b614d5f8a828b016149b3565b9095509350506108208801356001600160401b03811115614d7f57600080fd5b614c638a828b01614c99565b600080600080600080600060c0888a031215614da657600080fd5b614daf88614806565b96506020880135955060408801359450606088013593506080880135925060a08801356001600160401b03811115614d7f57600080fd5b6000610b2082840312156149ad57600080fd5b6000610b4082840312156149ad57600080fd5b6000806000806000806000806000806116e08b8d031215614e2c57600080fd5b8a356001600160401b03811115614e4257600080fd5b614e4e8d828e01614956565b909b509950614e6290508c60208d01614de6565b9750610b408b01356001600160401b03811115614e7e57600080fd5b614e8a8d828e01614956565b9098509650614e9f90508c610b608d01614df9565b94506116a08b01356001600160401b03811115614ebb57600080fd5b614ec78d828e016149b3565b9095509350506116c08b01356001600160401b03811115614ee757600080fd5b614ef38d828e01614c99565b915080935050809150509295989b9194979a5092959850565b60008060408385031215614f1f57600080fd5b614f2883614806565b946020939093013593505050565b600181811c90821680614f4a57607f821691505b6020821081036149ad57634e487b7160e01b600052602260045260246000fd5b61010081833761068061010082016101008401375050565b6107808101610f8b8284614f6a565b6000823561015e19833603018112614fa857600080fd5b9190910192915050565b81835260006001600160fb1b03831115614fcb57600080fd5b8260051b80836020870137939093016020019392505050565b602080825282358282015282013560408201526040808301606083013760406080830160a083013761502660e0820160c0840180358252602090810135910152565b6000610100830135601e1984360301811261504057600080fd5b83016020810190356001600160401b0381111561505c57600080fd5b8060051b360382131561506e57600080fd5b61016061012085015261508661018085018284614fb2565b610120860135610140868101919091529095013561016090940193909352509192915050565b8051801515811461097657600080fd5b6000602082840312156150ce57600080fd5b610c5b826150ac565b6102208101610100838337610120610100840161010084013792915050565b634e487b7160e01b600052603260045260246000fd5b634e487b7160e01b600052601160045260246000fd5b80820180821115610f8b57610f8b61510c565b81810381811115610f8b57610f8b61510c565b634e487b7160e01b600052604160045260246000fd5b60208152816020820152818360408301376000818301604090810191909152601f909201601f19160101919050565b601f821115613b5e57806000526020600020601f840160051c810160208510156151b45750805b601f840160051c820191505b818110156143cc57600081556001016151c0565b6001600160401b038311156151eb576151eb615148565b6151ff836151f98354614f36565b8361518d565b6000601f841160018114615233576000851561521b5750838201355b600019600387901b1c1916600186901b1783556143cc565b600083815260209020601f19861690835b828110156152645786850135825560209485019460019092019101615244565b50868210156152815760001960f88860031b161c19848701351681555b505060018560011b0183555050505050565b634e487b7160e01b600052601260045260246000fd5b60008251614fa8818460208701614865565b6000602082840312156152cd57600080fd5b5051919050565b6000826152f157634e487b7160e01b600052601260045260246000fd5b500490565b8082028115828204841417610f8b57610f8b61510c565b602081526000610c5b6020830184614ae9565b6000806040838503121561533357600080fd5b61533c836150ac565b6020939093015192949293505050565b6107e081016101008383376106e0610100840161010084013792915050565b610b208101610100838337610a20610100840161010084013792915050565b610b408101610100838337610a4061010084016101008401379291505056fee85c8c79cebe1b6656a265affa1c69c79539e5ae9a9c9229f5b5d89619781080983b8264b64c9863a439320eb632213f6e5ca279753b012988656784757d97751d5f56c1b8cdd9a3aa6495672f7e6917c2f5f1e98a43abd3e16e2e997c1160e1eae287c62f1ff4911334dee03f631d5dded5284b1b03ea7bc1d6282916c7249f30644e72e131a029b85045b68181585d2833e84879b9709143e1f593f0000001060c89ce5c263405370a08b6d0302b0bab3eedb83920ee0a677297dc392126f1a2646970667358221220236bf615acd3ca7b792e5c29cf90be7f6b35ccac34ba5300003d49925e6bd97a64736f6c634300081b0033",
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

// PendingFingerprintSet is a free data retrieval call binding the contract method 0x93c43876.
//
// Solidity: function pendingFingerprintSet(uint256 , uint256 ) view returns(bool)
func (_Enygma *EnygmaCaller) PendingFingerprintSet(opts *bind.CallOpts, arg0 *big.Int, arg1 *big.Int) (bool, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "pendingFingerprintSet", arg0, arg1)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// PendingFingerprintSet is a free data retrieval call binding the contract method 0x93c43876.
//
// Solidity: function pendingFingerprintSet(uint256 , uint256 ) view returns(bool)
func (_Enygma *EnygmaSession) PendingFingerprintSet(arg0 *big.Int, arg1 *big.Int) (bool, error) {
	return _Enygma.Contract.PendingFingerprintSet(&_Enygma.CallOpts, arg0, arg1)
}

// PendingFingerprintSet is a free data retrieval call binding the contract method 0x93c43876.
//
// Solidity: function pendingFingerprintSet(uint256 , uint256 ) view returns(bool)
func (_Enygma *EnygmaCallerSession) PendingFingerprintSet(arg0 *big.Int, arg1 *big.Int) (bool, error) {
	return _Enygma.Contract.PendingFingerprintSet(&_Enygma.CallOpts, arg0, arg1)
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

// UsdrInitialized is a free data retrieval call binding the contract method 0xda171cf2.
//
// Solidity: function usdrInitialized(uint256 ) view returns(bool)
func (_Enygma *EnygmaCaller) UsdrInitialized(opts *bind.CallOpts, arg0 *big.Int) (bool, error) {
	var out []interface{}
	err := _Enygma.contract.Call(opts, &out, "usdrInitialized", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// UsdrInitialized is a free data retrieval call binding the contract method 0xda171cf2.
//
// Solidity: function usdrInitialized(uint256 ) view returns(bool)
func (_Enygma *EnygmaSession) UsdrInitialized(arg0 *big.Int) (bool, error) {
	return _Enygma.Contract.UsdrInitialized(&_Enygma.CallOpts, arg0)
}

// UsdrInitialized is a free data retrieval call binding the contract method 0xda171cf2.
//
// Solidity: function usdrInitialized(uint256 ) view returns(bool)
func (_Enygma *EnygmaCallerSession) UsdrInitialized(arg0 *big.Int) (bool, error) {
	return _Enygma.Contract.UsdrInitialized(&_Enygma.CallOpts, arg0)
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

// InitializeUsdrBalance is a paid mutator transaction binding the contract method 0x37640ad5.
//
// Solidity: function initializeUsdrBalance(uint256 accountId, uint256 initialUsdrCommitX, uint256 initialUsdrCommitY) returns(bool)
func (_Enygma *EnygmaTransactor) InitializeUsdrBalance(opts *bind.TransactOpts, accountId *big.Int, initialUsdrCommitX *big.Int, initialUsdrCommitY *big.Int) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "initializeUsdrBalance", accountId, initialUsdrCommitX, initialUsdrCommitY)
}

// InitializeUsdrBalance is a paid mutator transaction binding the contract method 0x37640ad5.
//
// Solidity: function initializeUsdrBalance(uint256 accountId, uint256 initialUsdrCommitX, uint256 initialUsdrCommitY) returns(bool)
func (_Enygma *EnygmaSession) InitializeUsdrBalance(accountId *big.Int, initialUsdrCommitX *big.Int, initialUsdrCommitY *big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.InitializeUsdrBalance(&_Enygma.TransactOpts, accountId, initialUsdrCommitX, initialUsdrCommitY)
}

// InitializeUsdrBalance is a paid mutator transaction binding the contract method 0x37640ad5.
//
// Solidity: function initializeUsdrBalance(uint256 accountId, uint256 initialUsdrCommitX, uint256 initialUsdrCommitY) returns(bool)
func (_Enygma *EnygmaTransactorSession) InitializeUsdrBalance(accountId *big.Int, initialUsdrCommitX *big.Int, initialUsdrCommitY *big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.InitializeUsdrBalance(&_Enygma.TransactOpts, accountId, initialUsdrCommitX, initialUsdrCommitY)
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

// Transfer is a paid mutator transaction binding the contract method 0xa8cc45d6.
//
// Solidity: function transfer((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[81]) proof, (uint256,uint256)[] usdrCommitmentDeltas, (uint256[8],uint256[82]) usdrProof, uint256[] participantIds, string bankTag) returns(bool)
func (_Enygma *EnygmaTransactor) Transfer(opts *bind.TransactOpts, commitmentDeltas []IEnygmaPoint, proof IEnygmaProof, usdrCommitmentDeltas []IEnygmaPoint, usdrProof IEnygmaUsdrProof, participantIds []*big.Int, bankTag string) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "transfer", commitmentDeltas, proof, usdrCommitmentDeltas, usdrProof, participantIds, bankTag)
}

// Transfer is a paid mutator transaction binding the contract method 0xa8cc45d6.
//
// Solidity: function transfer((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[81]) proof, (uint256,uint256)[] usdrCommitmentDeltas, (uint256[8],uint256[82]) usdrProof, uint256[] participantIds, string bankTag) returns(bool)
func (_Enygma *EnygmaSession) Transfer(commitmentDeltas []IEnygmaPoint, proof IEnygmaProof, usdrCommitmentDeltas []IEnygmaPoint, usdrProof IEnygmaUsdrProof, participantIds []*big.Int, bankTag string) (*types.Transaction, error) {
	return _Enygma.Contract.Transfer(&_Enygma.TransactOpts, commitmentDeltas, proof, usdrCommitmentDeltas, usdrProof, participantIds, bankTag)
}

// Transfer is a paid mutator transaction binding the contract method 0xa8cc45d6.
//
// Solidity: function transfer((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[81]) proof, (uint256,uint256)[] usdrCommitmentDeltas, (uint256[8],uint256[82]) usdrProof, uint256[] participantIds, string bankTag) returns(bool)
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
