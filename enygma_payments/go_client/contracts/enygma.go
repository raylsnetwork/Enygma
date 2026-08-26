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
	ABI: "[{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_epochInterval\",\"type\":\"uint256\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[],\"name\":\"AlreadyInitialized\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"AlreadyRegistered\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"BalanceMismatch\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"BurnExceedsModulus\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ContractIsPaused\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"DepositValueMismatch\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"FeeExceedsModulus\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"FingerprintNotConfirmed\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidAccountId\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidBlockNumber\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidCommitmentPoint\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidDomain\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidFee\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidFingerprintParty\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidParticipantCount\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidProof\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidPublicInputs\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidPublicKey\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidViewKeyLength\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NewOwnerIsZeroAddress\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NotInitialized\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NotOwner\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NotPendingOwner\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NotRegistered\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NullifierAlreadyUsed\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ParticipantIdsLengthMismatch\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ParticipantIdsNotSorted\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ReentrancyGuardReentrantCall\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"UnregisteredParticipant\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"VerifierHasNoCode\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"VerifierNotFound\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ZeroAddress\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ZkDvpOperationFailed\",\"type\":\"error\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"addedBank\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"accountId\",\"type\":\"uint256\"}],\"name\":\"AccountRegistered\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"bankIndex\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"burnValue\",\"type\":\"uint256\"}],\"name\":\"BurnSuccessful\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"commitment\",\"type\":\"uint256\"}],\"name\":\"Commitment\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"fee\",\"type\":\"uint256\"}],\"name\":\"FeeBurned\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"partyA\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"partyB\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"fingerprint\",\"type\":\"uint256\"}],\"name\":\"FingerprintConfirmed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"fromId\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"toId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"fingerprint\",\"type\":\"uint256\"}],\"name\":\"FingerprintPending\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferStarted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"Paused\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"previousFee\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"newFee\",\"type\":\"uint256\"}],\"name\":\"ProtocolFeeUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"submitter\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"bankTag\",\"type\":\"string\"}],\"name\":\"RelayAttribution\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"lastblockNum\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"to\",\"type\":\"uint256\"}],\"name\":\"SupplyMinted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"maxBankCount\",\"type\":\"uint256\"}],\"name\":\"TokenInitialized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"senderAddress\",\"type\":\"address\"}],\"name\":\"TransactionSuccessful\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"Unpaused\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"verifierAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"totalRegisteredVerifiers\",\"type\":\"uint256\"}],\"name\":\"VerifierRegistered\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"DepositVerifierAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"GetBlckHash\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"Name\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"Symbol\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"TotalRegisteredBanks\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"TotalSupply\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"VerifierAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"WithdrawVerifierAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"ZkdvpAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"acceptOwnership\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"}],\"name\":\"addBurnVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"}],\"name\":\"addDepositVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"}],\"name\":\"addFeeVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"p1x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"p1y\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"p2x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"p2y\",\"type\":\"uint256\"}],\"name\":\"addPedComm\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"}],\"name\":\"addVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"splitCount\",\"type\":\"uint256\"}],\"name\":\"addWithdrawVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"zkDvp\",\"type\":\"address\"}],\"name\":\"addZkDvp\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"addressToAccountId\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"balanceCommitments\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"accountId\",\"type\":\"uint256\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[9]\",\"name\":\"public_signal\",\"type\":\"uint256[9]\"}],\"internalType\":\"structIEnygma.BurnProof\",\"name\":\"proof\",\"type\":\"tuple\"}],\"name\":\"burn\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"check\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"confirmedFingerprint\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"commitmentDeltas\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[52]\",\"name\":\"public_signal\",\"type\":\"uint256[52]\"}],\"internalType\":\"structIEnygma.DepositProof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"components\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"internalType\":\"structIZkDvp.G1Point\",\"name\":\"a\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"uint256[2]\",\"name\":\"x\",\"type\":\"uint256[2]\"},{\"internalType\":\"uint256[2]\",\"name\":\"y\",\"type\":\"uint256[2]\"}],\"internalType\":\"structIZkDvp.G2Point\",\"name\":\"b\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"internalType\":\"structIZkDvp.G1Point\",\"name\":\"c\",\"type\":\"tuple\"}],\"internalType\":\"structIZkDvp.SnarkProof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"internalType\":\"uint256[]\",\"name\":\"statement\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256\",\"name\":\"numberOfInputs\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"numberOfOutputs\",\"type\":\"uint256\"}],\"internalType\":\"structIZkDvp.JoinSplitTransaction\",\"name\":\"transaction\",\"type\":\"tuple\"}],\"internalType\":\"structIEnygma.WithdrawParams\",\"name\":\"withdrawParam\",\"type\":\"tuple\"},{\"internalType\":\"uint256[]\",\"name\":\"participantIds\",\"type\":\"uint256[]\"}],\"name\":\"deposit\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"}],\"name\":\"derivePk\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"randomness\",\"type\":\"uint256\"}],\"name\":\"derivePkH\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"epochInterval\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"fingerprintConfirmed\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"accountId\",\"type\":\"uint256\"}],\"name\":\"getBalance\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"count\",\"type\":\"uint256\"}],\"name\":\"getPublicValues\",\"outputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"balances\",\"type\":\"tuple[]\"},{\"internalType\":\"uint256[]\",\"name\":\"keys\",\"type\":\"uint256[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"initialize\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"lastBlockNum\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"recipientId\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"mintCommitX\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"mintCommitY\",\"type\":\"uint256\"}],\"name\":\"mintSupply\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"pause\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"paused\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"randomness\",\"type\":\"uint256\"}],\"name\":\"pedCom\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"pendingFingerprint\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"pendingOwner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"protocolFee\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"publicKeys\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"addr\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"accountId\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"publicKey\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"initialCommitX\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"initialCommitY\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"viewKey\",\"type\":\"bytes\"}],\"name\":\"registerAccount\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"otherPartyId\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"fingerprint\",\"type\":\"uint256\"}],\"name\":\"registerFingerprint\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"newFee\",\"type\":\"uint256\"}],\"name\":\"setProtocolFee\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalSupplyAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalSupplyX\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalSupplyY\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"commitmentDeltas\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[81]\",\"name\":\"public_signal\",\"type\":\"uint256[81]\"}],\"internalType\":\"structIEnygma.Proof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"internalType\":\"uint256[]\",\"name\":\"participantIds\",\"type\":\"uint256[]\"},{\"internalType\":\"string\",\"name\":\"bankTag\",\"type\":\"string\"}],\"name\":\"transfer\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"commitmentDeltas\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[55]\",\"name\":\"public_signal\",\"type\":\"uint256[55]\"}],\"internalType\":\"structIEnygma.FeeProof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"internalType\":\"uint256[]\",\"name\":\"participantIds\",\"type\":\"uint256[]\"},{\"internalType\":\"string\",\"name\":\"bankTag\",\"type\":\"string\"}],\"name\":\"transferWithFee\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"unpause\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"viewKeys\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"commitmentDeltas\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[52]\",\"name\":\"public_signal\",\"type\":\"uint256[52]\"}],\"internalType\":\"structIEnygma.WithdrawProof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"erc20Adress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"publicKey\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.DepositParams[]\",\"name\":\"depositParams\",\"type\":\"tuple[]\"},{\"internalType\":\"uint256[]\",\"name\":\"participantIds\",\"type\":\"uint256[]\"}],\"name\":\"withdraw\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"},{\"internalType\":\"uint256[]\",\"name\":\"\",\"type\":\"uint256[]\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
	Bin: "0x60a060405234801561001057600080fd5b5060405161466f38038061466f83398101604081905261002f916100cd565b600081116100835760405162461bcd60e51b815260206004820152601960248201527f65706f6368496e74657276616c206d757374206265203e203000000000000000604482015260640160405180910390fd5b600180546001600160a01b031916331790556000805560808190526100a7816100b0565b6003555061012d565b6000816100bd81436100e6565b6100c79190610108565b92915050565b6000602082840312156100df57600080fd5b5051919050565b60008261010357634e487b7160e01b600052601260045260246000fd5b500490565b80820281158282048414176100c757634e487b7160e01b600052601160045260246000fd5b60805161452061014f600039600081816103630152612a6601526145206000f3fe608060405234801561001057600080fd5b506004361061030c5760003560e01c80638052474d1161019d578063a9c58a7e116100e9578063e30c3978116100a2578063f2fde38b1161007c578063f2fde38b1461073b578063f828f50b1461074e578063f834443414610757578063fe877fc91461076a57600080fd5b8063e30c3978146106e5578063ea0d4573146106f6578063edda4a0a1461072857600080fd5b8063a9c58a7e14610655578063b0e21e8a14610676578063b26720fd1461067f578063c1ab48fc14610692578063c680f410146106b2578063ce630c18146106d257600080fd5b8063874ed5b5116101565780639000b3d6116101305780639000b3d614610607578063919840ad1461061a578063a44b47f714610622578063a605841c1461062a57600080fd5b8063874ed5b5146105ba5780638da5cb5b146105cb5780638f48f7b5146105dc57600080fd5b80638052474d1461055a5780638129fc1c1461057c57806383914157146105845780638456cb591461059757806384aaa2de1461059f5780638718dcaa146105a757600080fd5b80635c975abb1161025c57806371929e2a11610215578063787dce3d116101ef578063787dce3d14610519578063795825a71461052c57806379ba50971461053f5780637d894a161461054757600080fd5b806371929e2a146104f5578063723dbbc4146104fe578063743873b41461051157600080fd5b80635c975abb146104655780635dcc7650146104775780635fbaf8411461048a57806367511a4d1461049d5780636da4d2b8146104a65780636f5a2d54146104d457600080fd5b80631e010439116102c957806336899042116102a3578063368990421461042e5780633f4ba83a146104375780634e466c531461043f5780635a54f35f1461045257600080fd5b80631e010439146103ec5780632c0457e8146103ff5780633045aaf31461041057600080fd5b80630197d9421461031157806307da47ea1461033957806309b1ef261461035e5780630cf1839c14610393578063132ce4d4146103b35780631a4e1aa1146103db575b600080fd5b61032461031f3660046139f9565b61077d565b60405190151581526020015b60405180910390f35b600b546001600160a01b03165b6040516001600160a01b039091168152602001610330565b6103857f000000000000000000000000000000000000000000000000000000000000000081565b604051908152602001610330565b6103a66103a1366004613a1b565b610884565b6040516103309190613a84565b6103c66103c1366004613a97565b61091e565b60408051928352602083019190915201610330565b600c546001600160a01b0316610346565b6103c66103fa366004613a1b565b61093b565b600a546001600160a01b0316610346565b60408051808201909152600281526122a760f11b60208201526103a6565b61038560035481565b61032461098f565b61032461044d3660046139f9565b610a04565b610324610460366004613ac9565b610ac3565b600254600160a01b900460ff16610324565b610324610485366004613b8c565b610c7c565b610324610498366004613c44565b610f42565b61038560065481565b6103246104b4366004613ac9565b601a60209081526000928352604080842090915290825290205460ff1681565b6104e76104e2366004613c7e565b61136b565b604051610330929190613da7565b61038560055481565b6103c661050c366004613a1b565b6115fd565b600354610385565b610324610527366004613a1b565b611612565b61032461053a366004613e0b565b6116b3565b61032461181f565b6103c6610555366004613ac9565b6118ad565b604080518082019091526006815265456e79676d6160d01b60208201526103a6565b6103246118ec565b610324610592366004613ebc565b61194f565b610324611b72565b600454610385565b6103246105b5366004613a97565b611be3565b6009546001600160a01b0316610346565b6001546001600160a01b0316610346565b6103856105ea366004613ac9565b601960209081526000928352604080842090915290825290205481565b6103246106153660046139f9565b611d7b565b610324611e71565b600754610385565b610385610638366004613ac9565b601860209081526000928352604080842090915290825290205481565b610668610663366004613a1b565b611e80565b604051610330929190613f17565b61038560085481565b61032461068d366004613f7b565b611fbe565b6103856106a03660046139f9565b60126020526000908152604090205481565b6103856106c0366004613a1b565b60106020526000908152604090205481565b6103c66106e0366004613a1b565b6120f1565b6002546001600160a01b0316610346565b6103c6610704366004613ac9565b600f6020908152600092835260408084209091529082529020805460019091015482565b6103246107363660046139f9565b6120fd565b6103246107493660046139f9565b6121bc565b61038560075481565b6103246107653660046139f9565b612268565b610324610778366004614020565b612333565b6001546000906001600160a01b031633146107ab576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b0382166107d25760405163d92e233d60e01b815260040160405180910390fd5b816001600160a01b03163b6000036107fd576040516362d4176d60e11b815260040160405180910390fd5b600660005260156020527f25847c9ccf691da811a9f934d6b3b92e6062ef92feb71bf4cb08cbb4fad8d65280546001600160a01b0384166001600160a01b03199182168117909255600b80549091168217905560045460405160008051602061448b833981519152916108739190815260200190565b60405180910390a25060015b919050565b6011602052600090815260409020805461089d9061404a565b80601f01602080910402602001604051908101604052809291908181526020018280546108c99061404a565b80156109165780601f106108eb57610100808354040283529160200191610916565b820191906000526020600020905b8154815290600101906020018083116108f957829003601f168201915b505050505081565b60008061092d8686868661241f565b915091505b94509492505050565b6003546000908152600f602090815260408083208484529091528120805482919015801561096b57506001810154155b1561097d575060009360019350915050565b80546001909101549094909350915050565b6001546000906001600160a01b031633146109bd576040516330cd747160e01b815260040160405180910390fd5b6002805460ff60a01b191690556040513381527f5db9ee0a495bf2e6ff9c91a7834c1ba4fdd244a5e8aa4e537bd38aeae4b073aa906020015b60405180910390a150600190565b6001546000906001600160a01b03163314610a32576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b038216610a595760405163d92e233d60e01b815260040160405180910390fd5b816001600160a01b03163b600003610a84576040516362d4176d60e11b815260040160405180910390fd5b600d80546001600160a01b0319166001600160a01b03841690811790915560045460405190815260008051602061448b83398151915290602001610873565b336000908152601260205260408120548103610af25760405163aba4733960e01b815260040160405180910390fd5b600254600160a01b900460ff1615610b1d576040516306d39fcd60e41b815260040160405180910390fd5b33600090815260126020526040902054831580610b3957508084145b15610b575760405163a5c3e7e160e01b815260040160405180910390fd5b60008181526018602090815260408083208784528252918290208590559051848152859183917f2244c7409c0d13a2d9db63e7bfad1826bd85a2d6052ad998f150d2aab30c0d1f910160405180910390a360008481526018602090815260408083208484529091529020548015801590610bd057508381145b15610c6f57600082815260196020818152604080842089855282528084208890559181528183208584528152818320879055601a80825282842089855282528284208054600160ff199182168117909255918352838520878652835293839020805490911690931790925551858152869184917fe2070e33257d0615b4d0bff5bc34dcc22292a2ca3973074cc97d92dcc884549d910160405180910390a35b6001925050505b92915050565b336000908152601260205260408120548103610cab5760405163aba4733960e01b815260040160405180910390fd5b600160005414610cce576040516321c4e35760e21b815260040160405180910390fd5b600254600160a01b900460ff1615610cf9576040516306d39fcd60e41b815260040160405180910390fd5b600254600160a81b900460ff1615610d2457604051633ee5aeb560e01b815260040160405180910390fd5b6002805460ff60a81b1916600160a81b1790556000868152601560205260409020546001600160a01b031680610d6d57604051633896c50b60e21b815260040160405180910390fd5b806001600160a01b03163b600003610d98576040516362d4176d60e11b815260040160405180910390fd5b6000816001600160a01b031687604051602401610db59190614096565b60408051601f198184030181529181526020820180516001600160e01b0316630f948af160e01b17905251610dea91906140a5565b600060405180830381855afa9150503d8060008114610e25576040519150601f19603f3d011682016040523d82523d6000602084013e610e2a565b606091505b5050905080610e4c576040516309bde33960e01b815260040160405180910390fd5b610e5d876101000186868c8c612570565b610e6a87610100016127e9565b610e778761010001612816565b610e8389898787612876565b610e8d8989612983565b600c546001600160a01b031680634ac058ed610ea989806140c1565b6040518263ffffffff1660e01b8152600401610ec5919061410a565b6020604051808303816000875af1158015610ee4573d6000803e3d6000fd5b505050506040513d601f19601f82011682018060405250810190610f0891906141e2565b610f255760405163068fdd5760e41b815260040160405180910390fd5b50506002805460ff60a81b19169055506001979650505050505050565b6001546000906001600160a01b03163314610f70576040516330cd747160e01b815260040160405180910390fd5b600160005414610f93576040516321c4e35760e21b815260040160405180910390fd5b600254600160a01b900460ff1615610fbe576040516306d39fcd60e41b815260040160405180910390fd5b600e546001600160a01b031680610fe857604051633896c50b60e21b815260040160405180910390fd5b806001600160a01b03163b600003611013576040516362d4176d60e11b815260040160405180910390fd5b6000816001600160a01b03168460405160240161103091906141fd565b60408051601f198184030181529181526020820180516001600160e01b03166311475c8760e31b1790525161106591906140a5565b600060405180830381855afa9150503d80600081146110a0576040519150601f19603f3d011682016040523d82523d6000602084013e6110a5565b606091505b50509050806110c7576040516309bde33960e01b815260040160405180910390fd5b304660a01b17610200850135146110f1576040516375893cc160e11b815260040160405180910390fd5b60008581526010602052604090205461010085013514611124576040516319dcebfb60e21b815260040160405180910390fd5b6000806111308761093b565b90925090506101208601358214158061116d5750806101008701611155600180614248565b600981106111655761116561421c565b602002013514155b1561118b576040516319dcebfb60e21b815260040160405180910390fd5b6101a08601356000805160206144cb83398151915281106111bf576040516304b4b91960e11b815260040160405180910390fd5b6003546101c0880135146111e657604051631391e11b60e21b815260040160405180910390fd5b6101e087013560008181526017602052604090205460ff161561121c57604051636569570160e11b815260040160405180910390fd5b6000818152601760205260409020805460ff1916600117905561123e896129ee565b6000611248612a62565b905060405180604001604052808a6101000160036009811061126c5761126c61421c565b602002013581526020018a61010001600360016112899190614248565b600981106112995761129961421c565b602090810291909101359091526000838152600f825260408082208e8352835281208351815592909101516001909201919091556003829055806112f56112ee866000805160206144cb83398151915261425b565b60006118ad565b91509150611309600554600654848461241f565b600655600555600780548690039055604080518d8152602081018790527f262a9a1794440b6af993000f5805d7f51b5a19d4c32fcb10a1c5216beb0616f4910160405180910390a1611359612a99565b5060019b9a5050505050505050505050565b33600090815260126020526040812054606090820361139d5760405163aba4733960e01b815260040160405180910390fd5b6001600054146113c0576040516321c4e35760e21b815260040160405180910390fd5b600254600160a01b900460ff16156113eb576040516306d39fcd60e41b815260040160405180910390fd5b600254600160a81b900460ff161561141657604051633ee5aeb560e01b815260040160405180910390fd5b6002805460ff60a81b1916600160a81b1790558288146114495760405163023f995760e61b815260040160405180910390fd5b6006881461146a576040516354fb304560e01b815260040160405180910390fd5b6000888152601460205260409020546001600160a01b0316806114a057604051633896c50b60e21b815260040160405180910390fd5b806001600160a01b03163b6000036114cb576040516362d4176d60e11b815260040160405180910390fd5b6000816001600160a01b0316896040516024016114e89190614096565b60408051601f198184030181529181526020820180516001600160e01b0316630f948af160e01b1790525161151d91906140a5565b600060405180830381855afa9150503d8060008114611558576040519150601f19603f3d011682016040523d82523d6000602084013e61155d565b606091505b505090508061157f576040516309bde33960e01b815260040160405180910390fd5b611590896101000187878e8e612570565b61159d89610100016127e9565b6115aa8961010001612816565b6115ba88886107408c0135612aa1565b6115c68b8b8888612876565b6115d08b8b612983565b60006115dc8989612afc565b6002805460ff60a81b1916905560019d909c509a5050505050505050505050565b60008061160983612ce8565b91509150915091565b6001546000906001600160a01b03163314611640576040516330cd747160e01b815260040160405180910390fd5b6000805160206144cb833981519152821061166e5760405163bb22c5a960e01b815260040160405180910390fd5b60085460408051918252602082018490527fb404cac19fb1cbeff98d325795b08886e3cd8fe8cb1a2f193aac66f13fb239c3910160405180910390a150600855600190565b3360009081526012602052604081205481036116e25760405163aba4733960e01b815260040160405180910390fd5b600160005414611705576040516321c4e35760e21b815260040160405180910390fd5b600254600160a01b900460ff1615611730576040516306d39fcd60e41b815260040160405180910390fd5b61173986612d81565b61174a866101000186868b8b612e93565b6117578661010001613101565b611764866101000161310c565b60085461074087013590811461178d576040516358d620b360e01b815260040160405180910390fd5b61179681613116565b6117a28989888861319a565b60405133907fe85c8c79cebe1b6656a265affa1c69c79539e5ae9a9c9229f5b5d8961978108090600090a2336001600160a01b03167f1d5f56c1b8cdd9a3aa6495672f7e6917c2f5f1e98a43abd3e16e2e997c1160e1858560405161180892919061426e565b60405180910390a250600198975050505050505050565b6002546000906001600160a01b0316331461184d57604051630614e5c760e21b815260040160405180910390fd5b600180546001600160a01b0319808216339081179093556002805490911690556040516001600160a01b03909116919082907f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e090600090a3600191505090565b6000806000806118bc866115fd565b915091506000806118cc876120f1565b915091506118dc8484848461241f565b95509550505050505b9250929050565b6001546000906001600160a01b0316331461191a576040516330cd747160e01b815260040160405180910390fd5b60016000540361193c5760405162dc149f60e41b815260040160405180910390fd5b5060016000818155600555600681905590565b6001546000906001600160a01b0316331461197d576040516330cd747160e01b815260040160405180910390fd5b6001600054146119a0576040516321c4e35760e21b815260040160405180910390fd5b600254600160a01b900460ff16156119cb576040516306d39fcd60e41b815260040160405180910390fd5b600087815260106020526040902054156119f857604051630ea075bf60e21b815260040160405180910390fd5b86600003611a1957604051630d57928360e21b815260040160405180910390fd5b85600003611a3a5760405163145a1fdd60e31b815260040160405180910390fd5b8115801590611a4b57506104a08214155b15611a695760405163759f482960e11b815260040160405180910390fd5b611a738585613270565b611a905760405163ecd2690d60e01b815260040160405180910390fd5b600087815260106020908152604080832089905560119091529020611ab68385836142ff565b506001600160a01b03881660009081526012602090815260408083208a9055805180820182528881528083018881526003548552600f84528285208c865290935292209151825551600190910155600554600654611b169190878761241f565b6006556005556004805460010190556040518781526001600160a01b038916907fefd1ddef00b1051abc144c2e895de70a10dbbc3ad8985118c74c15e40e3d391f906020015b60405180910390a2506001979650505050505050565b6001546000906001600160a01b03163314611ba0576040516330cd747160e01b815260040160405180910390fd5b6002805460ff60a01b1916600160a01b1790556040513381527f62e78cea01bee320cd4e420270b5ea74000d11b0c9f74754ebdbfc544b05a258906020016109f6565b6001546000906001600160a01b03163314611c11576040516330cd747160e01b815260040160405180910390fd5b600160005414611c34576040516321c4e35760e21b815260040160405180910390fd5b600254600160a01b900460ff1615611c5f576040516306d39fcd60e41b815260040160405180910390fd5b611c698383613270565b611c865760405163ecd2690d60e01b815260040160405180910390fd5b611c96600554600654858561241f565b6006556005556007805486019055611cad846129ee565b6003546000908152600f602090815260408083208784529091528120805460018201549192918291611ce091888861241f565b915091506000611cee612a62565b60408051808201825285815260208082018681526000858152600f83528481208e825283528490209251835551600190920191909155600383905581518c81529081018b905291925082917feae287c62f1ff4911334dee03f631d5dded5284b1b03ea7bc1d6282916c7249f910160405180910390a2611d6c612a99565b50600198975050505050505050565b6001546000906001600160a01b03163314611da9576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b038216611dd05760405163d92e233d60e01b815260040160405180910390fd5b816001600160a01b03163b600003611dfb576040516362d4176d60e11b815260040160405180910390fd5b600660005260136020527f709d0e3cf89777a1e1f9c99632e4494f29b0327befd0df15e277a12d9482579580546001600160a01b0384166001600160a01b03199182168117909255600980549091168217905560045460405160008051602061448b833981519152916108739190815260200190565b6000611e7b61331a565b905090565b606080826001600160401b03811115611e9b57611e9b61429d565b604051908082528060200260200182016040528015611ee057816020015b6040805180820190915260008082526020820152815260200190600190039081611eb95790505b509150826001600160401b03811115611efb57611efb61429d565b604051908082528060200260200182016040528015611f24578160200160208202803683370190505b50905060005b83811015611fb857611f3b8161093b565b848381518110611f4d57611f4d61421c565b6020026020010151600001858481518110611f6a57611f6a61421c565b6020026020010151602001828152508281525050506010600082815260200190815260200160002054828281518110611fa557611fa561421c565b6020908102919091010152600101611f2a565b50915091565b336000908152601260205260408120548103611fed5760405163aba4733960e01b815260040160405180910390fd5b600160005414612010576040516321c4e35760e21b815260040160405180910390fd5b600254600160a01b900460ff161561203b576040516306d39fcd60e41b815260040160405180910390fd5b6120458688613390565b612056866101000186868b8b6134a5565b61206586610100018686613713565b612072866101000161383a565b61207f8661010001613845565b61208b8888878761319a565b60405133907fe85c8c79cebe1b6656a265affa1c69c79539e5ae9a9c9229f5b5d8961978108090600090a2336001600160a01b03167f1d5f56c1b8cdd9a3aa6495672f7e6917c2f5f1e98a43abd3e16e2e997c1160e18484604051611b5c92919061426e565b6000806116098361384f565b6001546000906001600160a01b0316331461212b576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b0382166121525760405163d92e233d60e01b815260040160405180910390fd5b816001600160a01b03163b60000361217d576040516362d4176d60e11b815260040160405180910390fd5b600e80546001600160a01b0319166001600160a01b03841690811790915560045460405190815260008051602061448b83398151915290602001610873565b6001546000906001600160a01b031633146121ea576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b03821661221157604051633a247dd760e11b815260040160405180910390fd5b600280546001600160a01b0319166001600160a01b03848116918217909255600154604051919216907f38d16b8cac22d99fc7c124b9cd0de2d3fa1faef420bfe791d8c362d765e2270090600090a3506001919050565b6001546000906001600160a01b03163314612296576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b0382166122bd5760405163d92e233d60e01b815260040160405180910390fd5b600660005260166020527fc6a239f207aea309a1b4879cb5411b8facfc2bc1e4cc4717ff2598676c399b5f80546001600160a01b0384166001600160a01b03199182168117909255600c80549091168217905560045460405160008051602061448b833981519152916108739190815260200190565b6001546000906001600160a01b03163314612361576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b0383166123885760405163d92e233d60e01b815260040160405180910390fd5b826001600160a01b03163b6000036123b3576040516362d4176d60e11b815260040160405180910390fd5b6000828152601460205260409081902080546001600160a01b0386166001600160a01b03199182168117909255600a8054909116821790556004549151909160008051602061448b8339815191529161240e91815260200190565b60405180910390a250600192915050565b600080851580156124305750846001145b1561243f575082905081610932565b8315801561244d5750826001145b1561245c575084905083610932565b60006000805160206144ab833981519152858809905060006000805160206144ab833981519152858809905060006000805160206144ab83398151915280838509620292f809905060006000805160206144ab83398151915280898b096000805160206144ab833981519152898d0908905060006124fd846000805160206144ab83398151915287620292fc096000805160206144ab8339815191526138db565b90506000805160206144ab8339815191526125296000805160206144ab83398151915285600108613916565b830996506000805160206144ab83398151915261255e6125596001866000805160206144ab8339815191526138db565b613916565b82099550505050505094509492505050565b304660a01b176106608601351461259a576040516375893cc160e11b815260040160405180910390fd5b6000806125af60045460016106639190614248565b90925090508460005b818110156127de5760008888838181106125d4576125d461421c565b9050602002013590508381815181106125ef576125ef61421c565b60200260200101516000036126175760405163c669128160e01b815260040160405180910390fd5b8381815181106126295761262961421c565b60200260200101518a83600661263f9190614248565b6034811061264f5761264f61421c565b602002013514612672576040516319dcebfb60e21b815260040160405180910390fd5b6000612683600184901b600c614248565b90508582815181106126975761269761421c565b6020026020010151600001518b82603481106126b5576126b561421c565b602002013514158061270657508582815181106126d4576126d461421c565b6020026020010151602001518b8260016126ee9190614248565b603481106126fe576126fe61421c565b602002013514155b15612724576040516319dcebfb60e21b815260040160405180910390fd5b6000612735600185901b6018614248565b90508b81603481106127495761274961421c565b60200201358989868181106127605761276061421c565b905060400201600001351415806127b257508b61277e826001614248565b6034811061278e5761278e61421c565b60200201358989868181106127a5576127a561421c565b9050604002016020013514155b156127d0576040516319dcebfb60e21b815260040160405180910390fd5b8360010193505050506125b8565b505050505050505050565b6003548160245b60200201351461281357604051631391e11b60e21b815260040160405180910390fd5b50565b60008160315b602090810291909101356000818152601790925260409091205490915060ff161561285a57604051636569570160e11b815260040160405180910390fd5b6000908152601760205260409020805460ff1916600117905550565b808381146128975760405163023f995760e61b815260040160405180910390fd5b6128a160006129ee565b60006128ab612a62565b90506000805b838110156129775760008686838181106128cd576128cd61421c565b9050602002013590508281116128f65760405163f170f72d60e01b815260040160405180910390fd5b6000848152600f6020908152604080832084845290915281208054600182015493955085939192918291612961918e8e898181106129365761293661421c565b905060400201600001358f8f8a8181106129525761295261421c565b9050604002016020013561241f565b90845560019384015550509190910190506128b1565b50506003555050505050565b60006001815b838110156129d1576129c483838787858181106129a8576129a861421c565b905060400201600001358888868181106129525761295261421c565b9093509150600101612989565b506129e2600554600654848461241f565b60065560055550505050565b60006129f8612a62565b60045490915060015b818111612a5c57612a1181613949565b838114612a54576003546000908152600f602081815260408084208585528252808420878552928252808420858552909152909120815481556001918201549101555b600101612a01565b50505050565b60007f0000000000000000000000000000000000000000000000000000000000000000612a8f81436143d4565b611e7b91906143f6565b61281361331a565b6000805b83811015612adc57848482818110612abf57612abf61421c565b612ad29260609091020135905083614248565b9150600101612aa5565b50818114612a5c576040516202aef760e91b815260040160405180910390fd5b600c546060906001600160a01b0316826000816001600160401b03811115612b2657612b2661429d565b604051908082528060200260200182016040528015612b4f578160200160208202803683370190505b50905060005b82811015612cde57604080516002808252606082018352600092602083019080368337019050509050878783818110612b9057612b9061421c565b9050606002016000013581600081518110612bad57612bad61421c565b602002602001018181525050878783818110612bcb57612bcb61421c565b9050606002016040013581600181518110612be857612be861421c565b602002602001018181525050600080866001600160a01b03166383bf2edd846040518263ffffffff1660e01b8152600401612c23919061440d565b60408051808303816000875af1158015612c41573d6000803e3d6000fd5b505050506040513d601f19601f82011682018060405250810190612c659190614420565b9150915081612c875760405163068fdd5760e41b815260040160405180910390fd5b80858581518110612c9a57612c9a61421c565b602090810291909101015260405181907fef61e988d9804d573b4fc504760f55d3507094e4168fddc9245ac56fbfc419e490600090a2836001019350505050612b55565b5095945050505050565b600080827f1b46f45118b90335391ae7f66ffe16bdf117c9b9be79e1802d987d2347fcb12b7f21a94082e95c6df187baa045c30f4cb20e13eeb4b9ef2ef96bc2659242d50cda8360015b8415612d74576001851615612d5357612d4d8282868661241f565b90925090505b612d5d8484613984565b9094509250612d6d6002866143d4565b9450612d32565b9097909650945050505050565b600d546001600160a01b0316612daa57604051633896c50b60e21b815260040160405180910390fd5b600d546001600160a01b03163b600003612dd7576040516362d4176d60e11b815260040160405180910390fd5b600d546040516000916001600160a01b031690612df890849060240161444c565b60408051601f198184030181529181526020820180516001600160e01b031663f6fbf48960e01b17905251612e2d91906140a5565b600060405180830381855afa9150503d8060008114612e68576040519150601f19603f3d011682016040523d82523d6000602084013e612e6d565b606091505b5050905080612e8f576040516309bde33960e01b815260040160405180910390fd5b5050565b304660a01b176106c086013514612ebd576040516375893cc160e11b815260040160405180910390fd5b600080612ed260045460016106639190614248565b90925090508460005b818110156127de576000888883818110612ef757612ef761421c565b905060200201359050838181518110612f1257612f1261421c565b6020026020010151600003612f3a5760405163c669128160e01b815260040160405180910390fd5b838181518110612f4c57612f4c61421c565b60200260200101518a836006612f629190614248565b60378110612f7257612f7261421c565b602002013514612f95576040516319dcebfb60e21b815260040160405180910390fd5b6000612fa6600184901b600c614248565b9050858281518110612fba57612fba61421c565b6020026020010151600001518b8260378110612fd857612fd861421c565b60200201351415806130295750858281518110612ff757612ff761421c565b6020026020010151602001518b8260016130119190614248565b603781106130215761302161421c565b602002013514155b15613047576040516319dcebfb60e21b815260040160405180910390fd5b6000613058600185901b6018614248565b90508b816037811061306c5761306c61421c565b60200201358989868181106130835761308361421c565b905060400201600001351415806130d557508b6130a1826001614248565b603781106130b1576130b161421c565b60200201358989868181106130c8576130c861421c565b9050604002016020013514155b156130f3576040516319dcebfb60e21b815260040160405180910390fd5b836001019350505050612edb565b6003548160246127f0565b600081603161281c565b806000036131215750565b60008061313f6112ee846000805160206144cb83398151915261425b565b91509150613153600554600654848461241f565b6006556005556007805484900390556040518381527fa551808c565cfbf20dfffdbcd44c549f835f9d06a82dcd546c61644b2f5ce7919060200160405180910390a1505050565b808381146131bb5760405163023f995760e61b815260040160405180910390fd5b6131c560006129ee565b60006131cf612a62565b90506000805b838110156129775760008686838181106131f1576131f161421c565b90506020020135905082811161321a5760405163f170f72d60e01b815260040160405180910390fd5b6000848152600f602090815260408083208484529091528120805460018201549395508593919291829161325a918e8e898181106129365761293661421c565b90845560019384015550509190910190506131d5565b6000806000805160206144ab833981519152848509905060006000805160206144ab833981519152848509905060006000805160206144ab833981519152826000805160206144ab83398151915285620292fc0908905060006000805160206144ab83398151915280846000805160206144ab83398151915287620292f80909600108905061330e82826000805160206144ab8339815191526138db565b15979650505050505050565b6000806001805b6004548111613355576000806133368361093b565b915091506133468585848461241f565b90955093505050600101613321565b508160055414158061336957508060065414155b1561338757604051631947c14d60e31b815260040160405180910390fd5b60019250505090565b6000818152601360205260409020546001600160a01b0316806133c657604051633896c50b60e21b815260040160405180910390fd5b806001600160a01b03163b6000036133f1576040516362d4176d60e11b815260040160405180910390fd5b6000816001600160a01b03168460405160240161340e919061446b565b60408051601f198184030181529181526020820180516001600160e01b0316633fdaa96b60e11b1790525161344391906140a5565b600060405180830381855afa9150503d806000811461347e576040519150601f19603f3d011682016040523d82523d6000602084013e613483565b606091505b5050905080612a5c576040516309bde33960e01b815260040160405180910390fd5b304660a01b17610a00860135146134cf576040516375893cc160e11b815260040160405180910390fd5b6000806134e460045460016106639190614248565b90925090508460005b818110156127de5760008888838181106135095761350961421c565b9050602002013590508381815181106135245761352461421c565b602002602001015160000361354c5760405163c669128160e01b815260040160405180910390fd5b83818151811061355e5761355e61421c565b60200260200101518a8360246135749190614248565b605181106135845761358461421c565b6020020135146135a7576040516319dcebfb60e21b815260040160405180910390fd5b60006135b8600184901b602a614248565b90508582815181106135cc576135cc61421c565b6020026020010151600001518b82605181106135ea576135ea61421c565b602002013514158061363b57508582815181106136095761360961421c565b6020026020010151602001518b8260016136239190614248565b605181106136335761363361421c565b602002013514155b15613659576040516319dcebfb60e21b815260040160405180910390fd5b600061366a600185901b6036614248565b90508b816051811061367e5761367e61421c565b60200201358989868181106136955761369561421c565b905060400201600001351415806136e757508b6136b3826001614248565b605181106136c3576136c361421c565b60200201358989868181106136da576136da61421c565b9050604002016020013514155b15613705576040516319dcebfb60e21b815260040160405180910390fd5b8360010193505050506134ed565b8060005b818110156138335760008484838181106137335761373361421c565b90506020020135905060005b83811015613829578083146138215760008686838181106137625761376261421c565b6000868152601a60209081526040808320938202959095013580835292905292909220549192505060ff166137aa576040516310d9346760e21b815260040160405180910390fd5b6000826137b787876143f6565b6137c2906000614248565b6137cc9190614248565b60008581526019602090815260408083208684529091529020549091508982605181106137fb576137fb61421c565b60200201351461381e576040516319dcebfb60e21b815260040160405180910390fd5b50505b60010161373f565b5050600101613717565b5050505050565b6003548160426127f0565b600081604f61281c565b600080827f16546696a66928d34f6be843f8a5afa2063161d92742811279454d60de5322527f109c1c7a758b3e8e54af1ce919fc24e1b986aab09a6b8082600f8694bb3c1b4b8360015b8415612d745760018516156138ba576138b48282868661241f565b90925090505b6138c48484613984565b90945092506138d46002866143d4565b9450613899565b6000838381116138f2576138ef8382614248565b90505b8280613900576139006143be565b600061390c868461425b565b0895945050505050565b6000610c768261393560026000805160206144ab83398151915261425b565b6000805160206144ab83398151915261399e565b6003546000908152600f602090815260408083208484529091529020805415801561397657506001810154155b15612e8f5760019081015550565b6000806139938484868661241f565b915091509250929050565b600060405160208152602080820152602060408201528460608201528360808201528260a082015260208160c08360055afa80801561030c57505051949350505050565b80356001600160a01b038116811461087f57600080fd5b600060208284031215613a0b57600080fd5b613a14826139e2565b9392505050565b600060208284031215613a2d57600080fd5b5035919050565b60005b83811015613a4f578181015183820152602001613a37565b50506000910152565b60008151808452613a70816020860160208601613a34565b601f01601f19169290920160200192915050565b602081526000613a146020830184613a58565b60008060008060808587031215613aad57600080fd5b5050823594602084013594506040840135936060013592509050565b60008060408385031215613adc57600080fd5b50508035926020909101359150565b60008083601f840112613afd57600080fd5b5081356001600160401b03811115613b1457600080fd5b6020830191508360208260061b85010111156118e557600080fd5b60006107808284031215613b4257600080fd5b50919050565b60008083601f840112613b5a57600080fd5b5081356001600160401b03811115613b7157600080fd5b6020830191508360208260051b85010111156118e557600080fd5b6000806000806000806107e08789031215613ba657600080fd5b86356001600160401b03811115613bbc57600080fd5b613bc889828a01613aeb565b9097509550613bdc90508860208901613b2f565b93506107a08701356001600160401b03811115613bf857600080fd5b87016020818a031215613c0a57600080fd5b92506107c08701356001600160401b03811115613c2657600080fd5b613c3289828a01613b48565b979a9699509497509295939492505050565b600080828403610240811215613c5957600080fd5b83359250610220601f1982011215613c7057600080fd5b506020830190509250929050565b60008060008060008060006107e0888a031215613c9a57600080fd5b87356001600160401b03811115613cb057600080fd5b613cbc8a828b01613aeb565b9098509650613cd090508960208a01613b2f565b94506107a08801356001600160401b03811115613cec57600080fd5b8801601f81018a13613cfd57600080fd5b80356001600160401b03811115613d1357600080fd5b8a6020606083028401011115613d2857600080fd5b602091909101945092506107c08801356001600160401b03811115613d4c57600080fd5b613d588a828b01613b48565b989b979a50959850939692959293505050565b600081518084526020840193506020830160005b82811015613d9d578151865260209586019590910190600101613d7f565b5093949350505050565b8215158152604060208201526000613dc26040830184613d6b565b949350505050565b60008083601f840112613ddc57600080fd5b5081356001600160401b03811115613df357600080fd5b6020830191508360208285010111156118e557600080fd5b6000806000806000806000878903610840811215613e2857600080fd5b88356001600160401b03811115613e3e57600080fd5b613e4a8b828c01613aeb565b9099509750506107e0601f1982011215613e6357600080fd5b506020880194506108008801356001600160401b03811115613e8457600080fd5b613e908a828b01613b48565b9095509350506108208801356001600160401b03811115613eb057600080fd5b613d588a828b01613dca565b600080600080600080600060c0888a031215613ed757600080fd5b613ee0886139e2565b96506020880135955060408801359450606088013593506080880135925060a08801356001600160401b03811115613eb057600080fd5b6040808252835190820181905260009060208501906060840190835b81811015613f5d578351805184526020908101518185015290930192604090920191600101613f33565b50508381036020850152613f718186613d6b565b9695505050505050565b6000806000806000806000878903610b80811215613f9857600080fd5b88356001600160401b03811115613fae57600080fd5b613fba8b828c01613aeb565b909950975050610b20601f1982011215613fd357600080fd5b50602088019450610b408801356001600160401b03811115613ff457600080fd5b6140008a828b01613b48565b909550935050610b608801356001600160401b03811115613eb057600080fd5b6000806040838503121561403357600080fd5b61403c836139e2565b946020939093013593505050565b600181811c9082168061405e57607f821691505b602082108103613b4257634e487b7160e01b600052602260045260246000fd5b61010081833761068061010082016101008401375050565b6107808101610c76828461407e565b600082516140b7818460208701613a34565b9190910192915050565b6000823561015e198336030181126140b757600080fd5b81835260006001600160fb1b038311156140f157600080fd5b8260051b80836020870137939093016020019392505050565b602080825282358282015282013560408201526040808301606083013760406080830160a083013761414c60e0820160c0840180358252602090810135910152565b6000610100830135601e1984360301811261416657600080fd5b83016020810190356001600160401b0381111561418257600080fd5b8060051b360382131561419457600080fd5b6101606101208501526141ac610180850182846140d8565b610120860135610140868101919091529095013561016090940193909352509192915050565b8051801515811461087f57600080fd5b6000602082840312156141f457600080fd5b613a14826141d2565b6102208101610100838337610120610100840161010084013792915050565b634e487b7160e01b600052603260045260246000fd5b634e487b7160e01b600052601160045260246000fd5b80820180821115610c7657610c76614232565b81810381811115610c7657610c76614232565b60208152816020820152818360408301376000818301604090810191909152601f909201601f19160101919050565b634e487b7160e01b600052604160045260246000fd5b601f8211156142fa57806000526020600020601f840160051c810160208510156142da5750805b601f840160051c820191505b8181101561383357600081556001016142e6565b505050565b6001600160401b038311156143165761431661429d565b61432a83614324835461404a565b836142b3565b6000601f84116001811461435e57600085156143465750838201355b600019600387901b1c1916600186901b178355613833565b600083815260209020601f19861690835b8281101561438f578685013582556020948501946001909201910161436f565b50868210156143ac5760001960f88860031b161c19848701351681555b505060018560011b0183555050505050565b634e487b7160e01b600052601260045260246000fd5b6000826143f157634e487b7160e01b600052601260045260246000fd5b500490565b8082028115828204841417610c7657610c76614232565b602081526000613a146020830184613d6b565b6000806040838503121561443357600080fd5b61443c836141d2565b9150602083015190509250929050565b6107e081016101008383376106e0610100840161010084013792915050565b610b208101610100838337610a2061010084016101008401379291505056fe983b8264b64c9863a439320eb632213f6e5ca279753b012988656784757d977530644e72e131a029b85045b68181585d2833e84879b9709143e1f593f0000001060c89ce5c263405370a08b6d0302b0bab3eedb83920ee0a677297dc392126f1a2646970667358221220c3fc1eb955c83ba17c051879174209f59a9f9f3757a463aed0e7f2e3e677626f64736f6c634300081b0033",
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

// Transfer is a paid mutator transaction binding the contract method 0xb26720fd.
//
// Solidity: function transfer((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[81]) proof, uint256[] participantIds, string bankTag) returns(bool)
func (_Enygma *EnygmaTransactor) Transfer(opts *bind.TransactOpts, commitmentDeltas []IEnygmaPoint, proof IEnygmaProof, participantIds []*big.Int, bankTag string) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "transfer", commitmentDeltas, proof, participantIds, bankTag)
}

// Transfer is a paid mutator transaction binding the contract method 0xb26720fd.
//
// Solidity: function transfer((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[81]) proof, uint256[] participantIds, string bankTag) returns(bool)
func (_Enygma *EnygmaSession) Transfer(commitmentDeltas []IEnygmaPoint, proof IEnygmaProof, participantIds []*big.Int, bankTag string) (*types.Transaction, error) {
	return _Enygma.Contract.Transfer(&_Enygma.TransactOpts, commitmentDeltas, proof, participantIds, bankTag)
}

// Transfer is a paid mutator transaction binding the contract method 0xb26720fd.
//
// Solidity: function transfer((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[81]) proof, uint256[] participantIds, string bankTag) returns(bool)
func (_Enygma *EnygmaTransactorSession) Transfer(commitmentDeltas []IEnygmaPoint, proof IEnygmaProof, participantIds []*big.Int, bankTag string) (*types.Transaction, error) {
	return _Enygma.Contract.Transfer(&_Enygma.TransactOpts, commitmentDeltas, proof, participantIds, bankTag)
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
