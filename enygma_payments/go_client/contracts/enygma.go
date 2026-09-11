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

// IEnygmaDepositParams is an auto generated low-level Go binding around an user-defined struct.
type IEnygmaDepositParams struct {
	Amount      *big.Int
	Erc20Adress common.Address
	PublicKey   *big.Int
}

// IEnygmaDepositProof is an auto generated low-level Go binding around an user-defined struct.
type IEnygmaDepositProof struct {
	Proof        [8]*big.Int
	PublicSignal [50]*big.Int
}

// IEnygmaFeeProof is an auto generated low-level Go binding around an user-defined struct.
type IEnygmaFeeProof struct {
	Proof        [8]*big.Int
	PublicSignal [54]*big.Int
}

// IEnygmaPoint is an auto generated low-level Go binding around an user-defined struct.
type IEnygmaPoint struct {
	C1 *big.Int
	C2 *big.Int
}

// IEnygmaProof is an auto generated low-level Go binding around an user-defined struct.
type IEnygmaProof struct {
	Proof        [8]*big.Int
	PublicSignal [80]*big.Int
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
	PublicSignal [50]*big.Int
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
	ABI: "[{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_epochInterval\",\"type\":\"uint256\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[],\"name\":\"AlreadyInitialized\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"BalanceMismatch\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"BurnExceedsModulus\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidBlockNumber\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidFeeAmount\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidProof\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidPublicInputs\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NotInitialized\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NotOwner\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NotRegistered\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NullifierAlreadyUsed\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ReentrantCall\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"UsdrBindingMismatch\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"VerifierNotFound\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ZeroAddress\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ZkDvpOperationFailed\",\"type\":\"error\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"addedBank\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"totalRegisteredParties\",\"type\":\"uint256\"}],\"name\":\"AccountRegistered\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"bankIndex\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"burnValue\",\"type\":\"uint256\"}],\"name\":\"BurnSuccessful\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"commitment\",\"type\":\"uint256\"}],\"name\":\"Commitment\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"lastblockNum\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"to\",\"type\":\"uint256\"}],\"name\":\"SupplyMinted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"maxBankCount\",\"type\":\"uint256\"}],\"name\":\"TokenInitialized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"senderAddress\",\"type\":\"address\"}],\"name\":\"TransactionSuccessful\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"verifierAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"totalRegisteredVerifiers\",\"type\":\"uint256\"}],\"name\":\"VerifierRegistered\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"DepositVerifierAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"GetBlckHash\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"Name\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"Symbol\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"TotalRegisteredBanks\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"TotalSupply\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"VerifierAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"WithdrawVerifierAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"ZkdvpAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"}],\"name\":\"addDepositVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"}],\"name\":\"addFeeVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"p1x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"p1y\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"p2x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"p2y\",\"type\":\"uint256\"}],\"name\":\"addPedComm\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"}],\"name\":\"addUsdrVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"}],\"name\":\"addVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"splitCount\",\"type\":\"uint256\"}],\"name\":\"addWithdrawVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"zkDvp\",\"type\":\"address\"}],\"name\":\"addZkDvp\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"addressToAccountId\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"balanceCommitments\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"accountId\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"burn\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"check\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"checkUsdr\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"commitmentDeltas\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[50]\",\"name\":\"public_signal\",\"type\":\"uint256[50]\"}],\"internalType\":\"structIEnygma.DepositProof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"components\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"internalType\":\"structIZkDvp.G1Point\",\"name\":\"a\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"uint256[2]\",\"name\":\"x\",\"type\":\"uint256[2]\"},{\"internalType\":\"uint256[2]\",\"name\":\"y\",\"type\":\"uint256[2]\"}],\"internalType\":\"structIZkDvp.G2Point\",\"name\":\"b\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"internalType\":\"structIZkDvp.G1Point\",\"name\":\"c\",\"type\":\"tuple\"}],\"internalType\":\"structIZkDvp.SnarkProof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"internalType\":\"uint256[]\",\"name\":\"statement\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256\",\"name\":\"numberOfInputs\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"numberOfOutputs\",\"type\":\"uint256\"}],\"internalType\":\"structIZkDvp.JoinSplitTransaction\",\"name\":\"transaction\",\"type\":\"tuple\"}],\"internalType\":\"structIEnygma.WithdrawParams\",\"name\":\"withdrawParam\",\"type\":\"tuple\"},{\"internalType\":\"uint256[]\",\"name\":\"participantIds\",\"type\":\"uint256[]\"}],\"name\":\"deposit\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"}],\"name\":\"derivePk\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"randomness\",\"type\":\"uint256\"}],\"name\":\"derivePkH\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"epochInterval\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"accountId\",\"type\":\"uint256\"}],\"name\":\"getBalance\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"count\",\"type\":\"uint256\"}],\"name\":\"getPublicValues\",\"outputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"balances\",\"type\":\"tuple[]\"},{\"internalType\":\"uint256[]\",\"name\":\"keys\",\"type\":\"uint256[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"accountId\",\"type\":\"uint256\"}],\"name\":\"getUsdrBalance\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"count\",\"type\":\"uint256\"}],\"name\":\"getUsdrPublicValues\",\"outputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"balances\",\"type\":\"tuple[]\"},{\"internalType\":\"uint256[]\",\"name\":\"keys\",\"type\":\"uint256[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"initialize\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"accountId\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"randomness\",\"type\":\"uint256\"}],\"name\":\"initializeUsdrBalance\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"lastBlockNum\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"recipientId\",\"type\":\"uint256\"}],\"name\":\"mintSupply\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"recipientId\",\"type\":\"uint256\"}],\"name\":\"mintUsdrSupply\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"randomness\",\"type\":\"uint256\"}],\"name\":\"pedCom\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"publicKeys\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"addr\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"accountId\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"publicKey\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"randomness\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"viewKey\",\"type\":\"bytes\"}],\"name\":\"registerAccount\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"setUsdrFixedFee\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalSupplyAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalSupplyX\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalSupplyY\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"commitmentDeltas\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[80]\",\"name\":\"public_signal\",\"type\":\"uint256[80]\"}],\"internalType\":\"structIEnygma.Proof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"usdrCommitmentDeltas\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[81]\",\"name\":\"public_signal\",\"type\":\"uint256[81]\"}],\"internalType\":\"structIEnygma.UsdrProof\",\"name\":\"usdrProof\",\"type\":\"tuple\"},{\"internalType\":\"uint256[]\",\"name\":\"participantIds\",\"type\":\"uint256[]\"}],\"name\":\"transfer\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"commitmentDeltas\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[54]\",\"name\":\"public_signal\",\"type\":\"uint256[54]\"}],\"internalType\":\"structIEnygma.FeeProof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"internalType\":\"uint256[]\",\"name\":\"participantIds\",\"type\":\"uint256[]\"}],\"name\":\"transferWithFee\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"usdrBalanceCommitments\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"usdrFixedFeeAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"usdrTotalSupplyAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"usdrTotalSupplyX\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"usdrTotalSupplyY\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"viewKeys\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"commitmentDeltas\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[50]\",\"name\":\"public_signal\",\"type\":\"uint256[50]\"}],\"internalType\":\"structIEnygma.WithdrawProof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"erc20Adress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"publicKey\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.DepositParams[]\",\"name\":\"depositParams\",\"type\":\"tuple[]\"},{\"internalType\":\"uint256[]\",\"name\":\"participantIds\",\"type\":\"uint256[]\"}],\"name\":\"withdraw\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"},{\"internalType\":\"uint256[]\",\"name\":\"\",\"type\":\"uint256[]\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
	Bin: "0x60c060405234801561001057600080fd5b5060405161418e38038061418e83398101604081905261002f916100bf565b600081116100835760405162461bcd60e51b815260206004820152601960248201527f65706f6368496e74657276616c206d757374206265203e203000000000000000604482015260640160405180910390fd5b336080526000805560a0819052610099816100a2565b6001555061011f565b6000816100af81436100d8565b6100b991906100fa565b92915050565b6000602082840312156100d157600080fd5b5051919050565b6000826100f557634e487b7160e01b600052601260045260246000fd5b500490565b80820281158282048414176100b957634e487b7160e01b600052601160045260246000fd5b60805160a051613fef61019f6000396000818161029d015261259c01526000818161066801528181610a4401528181610d6801528181610e8f015281816110ce01528181611158015281816112e901528181611446015281816116bd0152818161196401528181611a4101528181611b290152611c070152613fef6000f3fe608060405234801561001057600080fd5b50600436106102465760003560e01c80630197d9421461024b57806307da47ea1461027357806309b1ef26146102985780630cf1839c146102cd57806312dcc88b146102ed578063132ce4d4146103005780631660e58f146103215780631a4e1aa1146103345780631e01043914610345578063249278921461035857806325e74eb31461036b578063272fe53c146103745780632c0457e81461037d5780632e59d0591461038e5780633045aaf3146103af57806336899042146103cd5780634e466c53146103d657806352333894146103e95780635f5377e0146103f157806367511a4d1461040457806367ec46931461040d57806371929e2a1461042e578063723dbbc414610437578063743873b41461044a5780637d894a16146104525780638052474d146104655780638129fc1c1461048757806384aaa2de1461048f578063874ed5b5146104975780638d909dd7146104a85780639000b3d6146104b1578063919840ad146104c45780639edd41ff146104cc578063a276a208146104df578063a44b47f7146104f2578063a61f3ae4146104fa578063a9c58a7e1461050d578063b390c0ab14610520578063c1ab48fc14610533578063c50c9b7314610553578063c680f41014610566578063ce630c1814610586578063ce8cd40014610599578063e52dc188146105cb578063ea0d4573146105d4578063ec4a09d014610606578063f828f50b14610619578063f834443414610622578063fe877fc914610635578063ff98feae14610648575b600080fd5b61025e6102593660046134cf565b61065b565b60405190151581526020015b60405180910390f35b600c546001600160a01b03165b6040516001600160a01b03909116815260200161026a565b6102bf7f000000000000000000000000000000000000000000000000000000000000000081565b60405190815260200161026a565b6102e06102db3660046134ea565b610754565b60405161026a9190613553565b61025e6102fb366004613607565b6107ee565b61031361030e3660046136bf565b610a1a565b60405161026a9291906136f1565b61025e61032f3660046134ea565b610a37565b600d546001600160a01b0316610280565b6103136103533660046134ea565b610a8b565b61025e6103663660046136ff565b610adf565b6102bf60095481565b6102bf60075481565b600b546001600160a01b0316610280565b6103a161039c366004613792565b610bb3565b60405161026a9291906138bb565b60408051808201909152600281526122a760f11b60208201526102e0565b6102bf60015481565b61025e6103e43660046134cf565b610d5b565b61025e610e0c565b61025e6103ff3660046138de565b610e82565b6102bf60045481565b61042061041b3660046134ea565b610f3b565b60405161026a929190613900565b6102bf60035481565b6103136104453660046134ea565b61106d565b6001546102bf565b6103136104603660046138de565b611082565b604080518082019091526006815265456e79676d6160d01b60208201526102e0565b61025e6110c1565b6002546102bf565b600a546001600160a01b0316610280565b6102bf60085481565b61025e6104bf3660046134cf565b61114b565b61025e611233565b6103136104da3660046134ea565b61129d565b61025e6104ed36600461395a565b6112dc565b6005546102bf565b61025e6105083660046138de565b611439565b61042061051b3660046134ea565b611584565b61025e61052e3660046138de565b6116b0565b6102bf6105413660046134cf565b60136020526000908152604090205481565b61025e6105613660046139fa565b611829565b6102bf6105743660046134ea565b60116020526000908152604090205481565b6103136105943660046134ea565b61194b565b6103136105a73660046138de565b60106020908152600092835260408084209091529082529020805460019091015482565b6102bf60065481565b6103136105e23660046138de565b600f6020908152600092835260408084209091529082529020805460019091015482565b61025e6106143660046134cf565b611957565b6102bf60055481565b61025e6106303660046134cf565b611a34565b61025e610643366004613adb565b611b1c565b61025e6106563660046138de565b611bfa565b6000336001600160a01b037f000000000000000000000000000000000000000000000000000000000000000016146106a6576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b0382166106cd5760405163d92e233d60e01b815260040160405180910390fd5b600660005260176020527fdba639bd458ad7aa817aa5e381ad7ed79ea3aeac2e3d9b2d7c51df9a9229709d80546001600160a01b0384166001600160a01b03199182168117909255600c805490911682179055600254604051600080516020613f3a833981519152916107439190815260200190565b60405180910390a25060015b919050565b6012602052600090815260409020805461076d90613b05565b80601f016020809104026020016040519081016040528092919081815260200182805461079990613b05565b80156107e65780601f106107bb576101008083540402835291602001916107e6565b820191906000526020600020905b8154815290600101906020018083116107c957829003601f168201915b505050505081565b33600090815260136020526040812054810361081d5760405163aba4733960e01b815260040160405180910390fd5b600160005414610840576040516321c4e35760e21b815260040160405180910390fd5b610848611d45565b6000868152601760205260409020546001600160a01b03168061087e57604051633896c50b60e21b815260040160405180910390fd5b6000816001600160a01b03168760405160240161089b9190613b51565b60408051601f198184030181529181526020820180516001600160e01b03166318e2c03f60e01b179052516108d09190613b60565b600060405180830381855afa9150503d806000811461090b576040519150601f19603f3d011682016040523d82523d6000602084013e610910565b606091505b5050905080610932576040516309bde33960e01b815260040160405180910390fd5b610943876101000186868c8c611d78565b6109508761010001611f8d565b61095d8761010001611fba565b600d546001600160a01b031680634ac058ed6109798980613b7c565b6040518263ffffffff1660e01b81526004016109959190613bc5565b6020604051808303816000875af11580156109b4573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906109d89190613c9d565b6109f55760405163068fdd5760e41b815260040160405180910390fd5b610a018a8a888861201a565b60019350505050610a10612119565b9695505050505050565b600080610a2986868686612125565b915091505b94509492505050565b6000336001600160a01b037f00000000000000000000000000000000000000000000000000000000000000001614610a82576040516330cd747160e01b815260040160405180910390fd5b50600955600190565b6001546000908152600f6020908152604080832084845290915281208054829190158015610abb57506001810154155b15610acd575060009360019350915050565b80546001909101549094909350915050565b336000908152601360205260408120548103610b0e5760405163aba4733960e01b815260040160405180910390fd5b600160005414610b31576040516321c4e35760e21b815260040160405180910390fd5b610b3a84612276565b610b4b846101000184848989612357565b610b588461010001612561565b610b65846101000161258e565b6000610b6f612598565b9050610b7e81888887876125d4565b610b886000612720565b60018190556040513390600080516020613f1a83398151915290600090a25060019695505050505050565b336000908152601360205260408120546060908203610be55760405163aba4733960e01b815260040160405180910390fd5b600160005414610c08576040516321c4e35760e21b815260040160405180910390fd5b610c10611d45565b6000858152601660205260409020546001600160a01b031680610c4657604051633896c50b60e21b815260040160405180910390fd5b6000816001600160a01b031689604051602401610c639190613b51565b60408051601f198184030181529181526020820180516001600160e01b03166318e2c03f60e01b17905251610c989190613b60565b600060405180830381855afa9150503d8060008114610cd3576040519150601f19603f3d011682016040523d82523d6000602084013e610cd8565b606091505b5050905080610cfa576040516309bde33960e01b815260040160405180910390fd5b610d0b896101000187878e8e611d78565b610d188961010001611f8d565b610d258961010001611fba565b6000610d318989612793565b9050610d3f8c8c898961201a565b6001945092505050610d4f612119565b97509795505050505050565b6000336001600160a01b037f00000000000000000000000000000000000000000000000000000000000000001614610da6576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b038216610dcd5760405163d92e233d60e01b815260040160405180910390fd5b600e80546001600160a01b0319166001600160a01b038416908117909155600254604051908152600080516020613f3a83398151915290602001610743565b6000806001805b6002548111610e4757600080610e288361129d565b91509150610e3885858484612125565b90955093505050600101610e13565b5081600654141580610e5b57508060075414155b15610e7957604051631947c14d60e31b815260040160405180910390fd5b60019250505090565b6000336001600160a01b037f00000000000000000000000000000000000000000000000000000000000000001614610ecd576040516330cd747160e01b815260040160405180910390fd5b600080610edb600085611082565b6040805180820182528381526020808201848152600180546000908152601084528581208d8252909352939091209151825551910155600654600754929450909250610f28918484612125565b6007556006555060019150505b92915050565b606080826001600160401b03811115610f5657610f56613cb8565b604051908082528060200260200182016040528015610f8f57816020015b610f7c61349e565b815260200190600190039081610f745790505b509150826001600160401b03811115610faa57610faa613cb8565b604051908082528060200260200182016040528015610fd3578160200160208202803683370190505b50905060005b8381101561106757610fea8161129d565b848381518110610ffc57610ffc613cce565b602002602001015160000185848151811061101957611019613cce565b602002602001015160200182815250828152505050601160008281526020019081526020016000205482828151811061105457611054613cce565b6020908102919091010152600101610fd9565b50915091565b6000806110798361297f565b91509150915091565b6000806000806110918661106d565b915091506000806110a18761194b565b915091506110b184848484612125565b95509550505050505b9250929050565b6000336001600160a01b037f0000000000000000000000000000000000000000000000000000000000000000161461110c576040516330cd747160e01b815260040160405180910390fd5b60016000540361112e5760405162dc149f60e41b815260040160405180910390fd5b506001600081815560038190556004829055600655600781905590565b6000336001600160a01b037f00000000000000000000000000000000000000000000000000000000000000001614611196576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b0382166111bd5760405163d92e233d60e01b815260040160405180910390fd5b600660005260146020527fe1f6b6a5fb7e47dad87547d4b0671e7e995a1dae22fbe5b3b5d10e2a77ed7aff80546001600160a01b0384166001600160a01b03199182168117909255600a805490911682179055600254604051600080516020613f3a833981519152916107439190815260200190565b6000806001805b600254811161126e5760008061124f83610a8b565b9150915061125f85858484612125565b9095509350505060010161123a565b5081600354141580610e5b57508060045414610e7957604051631947c14d60e31b815260040160405180910390fd5b600154600090815260106020908152604080832084845290915281208054829190158015610abb57506001810154610acd575060009360019350915050565b6000336001600160a01b037f00000000000000000000000000000000000000000000000000000000000000001614611327576040516330cd747160e01b815260040160405180910390fd5b60008681526011602090815260408083208890556012909152902061134d838583613d33565b506001600160a01b0387166000908152601360205260408120879055806113748187611082565b91509150604051806040016040528083815260200182815250600f6000600154815260200190815260200160002060008a815260200190815260200160002060008201518160000155602082015181600101559050506113da6003546004548484612125565b60045560035560028054600101908190556040519081526001600160a01b038a16907fefd1ddef00b1051abc144c2e895de70a10dbbc3ad8985118c74c15e40e3d391f906020015b60405180910390a250600198975050505050505050565b6000336001600160a01b037f00000000000000000000000000000000000000000000000000000000000000001614611484576040516330cd747160e01b815260040160405180910390fd5b6001600054146114a7576040516321c4e35760e21b815260040160405180910390fd5b6000806114b38561106d565b915091506114c76006546007548484612125565b60075560065560088054860190556114de84612720565b6114e86000612a18565b60018054600090815260106020908152604080832088845290915281208054928101549092829161151b91908787612125565b915091506000611529612598565b60408051808201825285815260208082018681526000858152601083528481208e8252909252908390209151825551600191820155829055519091508190600080516020613f5a83398151915290611422908c908c906136f1565b606080826001600160401b0381111561159f5761159f613cb8565b6040519080825280602002602001820160405280156115d857816020015b6115c561349e565b8152602001906001900390816115bd5790505b509150826001600160401b038111156115f3576115f3613cb8565b60405190808252806020026020018201604052801561161c578160200160208202803683370190505b50905060005b838110156110675761163381610a8b565b84838151811061164557611645613cce565b602002602001015160000185848151811061166257611662613cce565b602002602001015160200182815250828152505050601160008281526020019081526020016000205482828151811061169d5761169d613cce565b6020908102919091010152600101611622565b6000336001600160a01b037f000000000000000000000000000000000000000000000000000000000000000016146116fb576040516330cd747160e01b815260040160405180910390fd5b600080516020613f9a83398151915282111561172a576040516304b4b91960e11b815260040160405180910390fd5b60008061174f61174885600080516020613f9a833981519152613e08565b6000611082565b9150915061175c85612a18565b6117666000612720565b600180546000908152600f6020908152604080832089845290915281208054928101549092829161179991908787612125565b9150915060006117a7612598565b60408051808201825285815260208082018681526000858152600f83528481208f8252909252908390209151825551600191820155829055519091507f262a9a1794440b6af993000f5805d7f51b5a19d4c32fcb10a1c5216beb0616f490611812908b908b906136f1565b60405180910390a150600198975050505050505050565b3360009081526013602052604081205481036118585760405163aba4733960e01b815260040160405180910390fd5b60016000541461187b576040516321c4e35760e21b815260040160405180910390fd5b6118858789612a85565b61188f8486612b6f565b6118a187610100018561010001612bf7565b6118b2876101000184848c8c612cf6565b6118c3846101000184848989612f00565b6118d08761010001613130565b6118dd8461010001613130565b6118ea876101000161313b565b6118f7846101000161313b565b6000611901612598565b9050611910818b8b87876125d4565b61191d8188888787613145565b60018190556040513390600080516020613f1a83398151915290600090a25060019998505050505050505050565b60008061107983613286565b6000336001600160a01b037f000000000000000000000000000000000000000000000000000000000000000016146119a2576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b0382166119c95760405163d92e233d60e01b815260040160405180910390fd5b600660005260156020527f25847c9ccf691da811a9f934d6b3b92e6062ef92feb71bf4cb08cbb4fad8d65280546001600160a01b0384166001600160a01b03199091168117909155600254604051600080516020613f3a833981519152916107439190815260200190565b6000336001600160a01b037f00000000000000000000000000000000000000000000000000000000000000001614611a7f576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b038216611aa65760405163d92e233d60e01b815260040160405180910390fd5b600660005260186020527f33d69b83f8d9644c8360a50d7275bf12a50019a4cd4e17926d0b315da648c58d80546001600160a01b0384166001600160a01b03199182168117909255600d805490911682179055600254604051600080516020613f3a833981519152916107439190815260200190565b6000336001600160a01b037f00000000000000000000000000000000000000000000000000000000000000001614611b67576040516330cd747160e01b815260040160405180910390fd5b6001600160a01b038316611b8e5760405163d92e233d60e01b815260040160405180910390fd5b6000828152601660205260409081902080546001600160a01b0386166001600160a01b03199182168117909255600b80549091168217905560025491519091600080516020613f3a83398151915291611be991815260200190565b60405180910390a250600192915050565b6000336001600160a01b037f00000000000000000000000000000000000000000000000000000000000000001614611c45576040516330cd747160e01b815260040160405180910390fd5b600160005414611c68576040516321c4e35760e21b815260040160405180910390fd5b600080611c748561106d565b91509150611c886003546004548484612125565b6004556003556005805486019055611c9f84612a18565b611ca96000612720565b600180546000908152600f60209081526040808320888452909152812080549281015490928291611cdc91908787612125565b915091506000611cea612598565b60408051808201825285815260208082018681526000858152600f83528481208e8252909252908390209151825551600191820155829055519091508190600080516020613f5a83398151915290611422908c908c906136f1565b601a5460ff1615611d69576040516306fda65d60e31b815260040160405180910390fd5b601a805460ff19166001179055565b600080611d8d600254600161051b9190613e1b565b90925090508460005b81811015611f82576000888883818110611db257611db2613cce565b905060200201359050838181518110611dcd57611dcd613cce565b60200260200101518a836006611de39190613e1b565b60328110611df357611df3613cce565b602002013514611e16576040516319dcebfb60e21b815260040160405180910390fd5b6000611e27600184901b600c613e1b565b9050858281518110611e3b57611e3b613cce565b6020026020010151600001518b8260328110611e5957611e59613cce565b6020020135141580611eaa5750858281518110611e7857611e78613cce565b6020026020010151602001518b826001611e929190613e1b565b60328110611ea257611ea2613cce565b602002013514155b15611ec8576040516319dcebfb60e21b815260040160405180910390fd5b6000611ed9600185901b6018613e1b565b90508b8160328110611eed57611eed613cce565b6020020135898986818110611f0457611f04613cce565b90506040020160000135141580611f5657508b611f22826001613e1b565b60328110611f3257611f32613cce565b6020020135898986818110611f4957611f49613cce565b9050604002016020013514155b15611f74576040516319dcebfb60e21b815260040160405180910390fd5b836001019350505050611d96565b505050505050505050565b600154610480820135908114611fb657604051631391e11b60e21b815260040160405180910390fd5b5050565b60008160315b602090810291909101356000818152601990925260409091205490915060ff1615611ffe57604051636569570160e11b815260040160405180910390fd5b6000908152601960205260409020805460ff1916600117905550565b6000612024612598565b90508360005b8181101561210457600085858381811061204657612046613cce565b9050602002013590506000600f60006001548152602001908152602001600020600083815260200190815260200160002090506000806120c6836000015484600101548d8d8981811061209b5761209b613cce565b905060400201600001358e8e8a8181106120b7576120b7613cce565b90506040020160200135612125565b604080518082018252928352602080840192835260008b8152600f82528281209881529790529095209051815593516001948501555050500161202a565b5061210f6000612720565b5060015550505050565b601a805460ff19169055565b600080851580156121365750846001145b15612145575082905081610a2e565b831580156121535750826001145b15612162575084905083610a2e565b6000600080516020613f7a83398151915285880990506000600080516020613f7a83398151915285880990506000600080516020613f7a83398151915280838509620292f80990506000600080516020613f7a83398151915280898b09600080516020613f7a833981519152898d09089050600061220384600080516020613f7a83398151915287620292fc09600080516020613f7a833981519152613312565b9050600080516020613f7a83398151915261222f600080516020613f7a8339815191528560010861334f565b83099650600080516020613f7a83398151915261226461225f600186600080516020613f7a833981519152613312565b61334f565b82099550505050505094509492505050565b600e546001600160a01b031661229f57604051633896c50b60e21b815260040160405180910390fd5b600e546040516000916001600160a01b0316906122c0908490602401613e44565b60408051601f198184030181529181526020820180516001600160e01b031663fb336d8b60e01b179052516122f59190613b60565b600060405180830381855afa9150503d8060008114612330576040519150601f19603f3d011682016040523d82523d6000602084013e612335565b606091505b5050905080611fb6576040516309bde33960e01b815260040160405180910390fd5b60008061236c600254600161051b9190613e1b565b90925090508460005b81811015611f8257600088888381811061239157612391613cce565b9050602002013590508381815181106123ac576123ac613cce565b60200260200101518a8360066123c29190613e1b565b603681106123d2576123d2613cce565b6020020135146123f5576040516319dcebfb60e21b815260040160405180910390fd5b6000612406600184901b600c613e1b565b905085828151811061241a5761241a613cce565b6020026020010151600001518b826036811061243857612438613cce565b6020020135141580612489575085828151811061245757612457613cce565b6020026020010151602001518b8260016124719190613e1b565b6036811061248157612481613cce565b602002013514155b156124a7576040516319dcebfb60e21b815260040160405180910390fd5b60006124b8600185901b6018613e1b565b90508b81603681106124cc576124cc613cce565b60200201358989868181106124e3576124e3613cce565b9050604002016000013514158061253557508b612501826001613e1b565b6036811061251157612511613cce565b602002013589898681811061252857612528613cce565b9050604002016020013514155b15612553576040516319dcebfb60e21b815260040160405180910390fd5b836001019350505050612375565b6001548160245b60200201351461258b57604051631391e11b60e21b815260040160405180910390fd5b50565b6000816031611fc0565b60007f00000000000000000000000000000000000000000000000000000000000000006125c58143613e63565b6125cf9190613e85565b905090565b60025460005b8181101561263d576125eb81613382565b6125f68484836133bd565b61263557600180546000908152600f6020818152604080842086855282528084208c855292825280842086855290915290912081548155908201549101555b6001016125da565b508360005b8181101561271657600085858381811061265e5761265e613cce565b9050602002013590506000600f60006001548152602001908152602001600020600083815260200190815260200160002090506000806126b3836000015484600101548d8d8981811061209b5761209b613cce565b91509150604051806040016040528083815260200182815250600f60008e81526020019081526020016000206000868152602001908152602001600020600082015181600001556020820151816001015590505084600101945050505050612642565b5050505050505050565b600061272a612598565b60025490915060015b81811161278d5761274381613408565b83811461278557600180546000908152601060208181526040808420868552825280842088855292825280842086855290915290912081548155908201549101555b600101612733565b50505050565b600d546060906001600160a01b0316826000816001600160401b038111156127bd576127bd613cb8565b6040519080825280602002602001820160405280156127e6578160200160208202803683370190505b50905060005b828110156129755760408051600280825260608201835260009260208301908036833701905050905087878381811061282757612827613cce565b905060600201600001358160008151811061284457612844613cce565b60200260200101818152505087878381811061286257612862613cce565b905060600201604001358160018151811061287f5761287f613cce565b602002602001018181525050600080866001600160a01b03166383bf2edd846040518263ffffffff1660e01b81526004016128ba9190613e9c565b60408051808303816000875af11580156128d8573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906128fc9190613eaf565b915091508161291e5760405163068fdd5760e41b815260040160405180910390fd5b8085858151811061293157612931613cce565b602090810291909101015260405181907fef61e988d9804d573b4fc504760f55d3507094e4168fddc9245ac56fbfc419e490600090a28360010193505050506127ec565b5095945050505050565b600080827f2491aba8d3a191a76e35bc47bd9afe6cc88fee14d607cbe779f2349047d5c1577f2e07297f8d3c3d7818dbddfd24c35583f9a9d4ed0cb0c1d1348dd8f7f99152d78360015b8415612a0b5760018516156129ea576129e482828686612125565b90925090505b6129f48484613440565b9094509250612a04600286613e63565b94506129c9565b9097909650945050505050565b6000612a22612598565b60025490915060015b81811161278d57612a3b81613382565b838114612a7d57600180546000908152600f60208181526040808420868552825280842088855292825280842086855290915290912081548155908201549101555b600101612a2b565b6000818152601460205260409020546001600160a01b031680612abb57604051633896c50b60e21b815260040160405180910390fd5b6000816001600160a01b031684604051602401612ad89190613edb565b60408051601f198184030181529181526020820180516001600160e01b0316633172abe960e21b17905251612b0d9190613b60565b600060405180830381855afa9150503d8060008114612b48576040519150601f19603f3d011682016040523d82523d6000602084013e612b4d565b606091505b505090508061278d576040516309bde33960e01b815260040160405180910390fd5b6000818152601560205260409020546001600160a01b031680612ba557604051633896c50b60e21b815260040160405180910390fd5b6000816001600160a01b031684604051602401612bc29190613efa565b60408051601f198184030181529181526020820180516001600160e01b0316633fdaa96b60e11b17905251612b0d9190613b60565b60245b612c0660066024613e1b565b811015612c6157818160518110612c1f57612c1f613cce565b6020020135838260508110612c3657612c36613cce565b602002013514612c5957604051630663ff5760e11b815260040160405180910390fd5b600101612bfa565b5060435b612c7160066043613e1b565b811015612ccc57818160518110612c8a57612c8a613cce565b6020020135838260508110612ca157612ca1613cce565b602002013514612cc457604051630663ff5760e11b815260040160405180910390fd5b600101612c65565b50610840828101359082013514611fb657604051630663ff5760e11b815260040160405180910390fd5b600080612d0b600254600161051b9190613e1b565b90925090508460005b81811015611f82576000888883818110612d3057612d30613cce565b905060200201359050838181518110612d4b57612d4b613cce565b60200260200101518a836024612d619190613e1b565b60508110612d7157612d71613cce565b602002013514612d94576040516319dcebfb60e21b815260040160405180910390fd5b6000612da5600184901b602a613e1b565b9050858281518110612db957612db9613cce565b6020026020010151600001518b8260508110612dd757612dd7613cce565b6020020135141580612e285750858281518110612df657612df6613cce565b6020026020010151602001518b826001612e109190613e1b565b60508110612e2057612e20613cce565b602002013514155b15612e46576040516319dcebfb60e21b815260040160405180910390fd5b6000612e57600185901b6036613e1b565b90508b8160508110612e6b57612e6b613cce565b6020020135898986818110612e8257612e82613cce565b90506040020160000135141580612ed457508b612ea0826001613e1b565b60508110612eb057612eb0613cce565b6020020135898986818110612ec757612ec7613cce565b9050604002016020013514155b15612ef2576040516319dcebfb60e21b815260040160405180910390fd5b836001019350505050612d14565b600954610a0086013514612f265760405162a4671960e71b815260040160405180910390fd5b600080612f3b600254600161041b9190613e1b565b90925090508460005b81811015611f82576000888883818110612f6057612f60613cce565b905060200201359050838181518110612f7b57612f7b613cce565b60200260200101518a836024612f919190613e1b565b60518110612fa157612fa1613cce565b602002013514612fc4576040516319dcebfb60e21b815260040160405180910390fd5b6000612fd5600184901b602a613e1b565b9050858281518110612fe957612fe9613cce565b6020026020010151600001518b826051811061300757613007613cce565b6020020135141580613058575085828151811061302657613026613cce565b6020026020010151602001518b8260016130409190613e1b565b6051811061305057613050613cce565b602002013514155b15613076576040516319dcebfb60e21b815260040160405180910390fd5b6000613087600185901b6036613e1b565b90508b816051811061309b5761309b613cce565b60200201358989868181106130b2576130b2613cce565b9050604002016000013514158061310457508b6130d0826001613e1b565b605181106130e0576130e0613cce565b60200201358989868181106130f7576130f7613cce565b9050604002016020013514155b15613122576040516319dcebfb60e21b815260040160405180910390fd5b836001019350505050612f44565b600154816042612568565b600081604f611fc0565b60025460015b8181116131ad5761315b81613408565b6131668484836133bd565b6131a55760018054600090815260106020818152604080842086855282528084208c855292825280842086855290915290912081548155908201549101555b60010161314b565b508360005b818110156127165760008585838181106131ce576131ce613cce565b905060200201359050600060106000600154815260200190815260200160002060008381526020019081526020016000209050600080613223836000015484600101548d8d8981811061209b5761209b613cce565b91509150604051806040016040528083815260200182815250601060008e815260200190815260200160002060008681526020019081526020016000206000820151816000015560208201518160010155905050846001019450505050506131b2565b600080827f16546696a66928d34f6be843f8a5afa2063161d92742811279454d60de5322527f109c1c7a758b3e8e54af1ce919fc24e1b986aab09a6b8082600f8694bb3c1b4b8360015b8415612a0b5760018516156132f1576132eb82828686612125565b90925090505b6132fb8484613440565b909450925061330b600286613e63565b94506132d0565b600083838111613329576133268382613e1b565b90505b828061333757613337613e2e565b60006133438684613e08565b089150505b9392505050565b6000610f358261336e6002600080516020613f7a833981519152613e08565b600080516020613f7a83398151915261345a565b6001546000908152600f60209081526040808320848452909152902080541580156133af57506001810154155b15611fb65760019081015550565b600082815b818110156133fc57838686838181106133dd576133dd613cce565b90506020020135036133f457600192505050613348565b6001016133c2565b50600095945050505050565b6001546000908152601060209081526040808320848452909152902080541580156133af57506001810154611fb65760019081015550565b60008061344f84848686612125565b915091509250929050565b600060405160208152602080820152602060408201528460608201528360808201528260a082015260208160c08360055afa80801561024657505051949350505050565b604051806040016040528060008152602001600081525090565b80356001600160a01b038116811461074f57600080fd5b6000602082840312156134e157600080fd5b613348826134b8565b6000602082840312156134fc57600080fd5b5035919050565b60005b8381101561351e578181015183820152602001613506565b50506000910152565b6000815180845261353f816020860160208601613503565b601f01601f19169290920160200192915050565b6020815260006133486020830184613527565b60008083601f84011261357857600080fd5b5081356001600160401b0381111561358f57600080fd5b6020830191508360208260061b85010111156110ba57600080fd5b600061074082840312156135bd57600080fd5b50919050565b60008083601f8401126135d557600080fd5b5081356001600160401b038111156135ec57600080fd5b6020830191508360208260051b85010111156110ba57600080fd5b6000806000806000806107a0878903121561362157600080fd5b86356001600160401b0381111561363757600080fd5b61364389828a01613566565b9097509550613657905088602089016135aa565b93506107608701356001600160401b0381111561367357600080fd5b87016020818a03121561368557600080fd5b92506107808701356001600160401b038111156136a157600080fd5b6136ad89828a016135c3565b979a9699509497509295939492505050565b600080600080608085870312156136d557600080fd5b5050823594602084013594506040840135936060013592509050565b918252602082015260400190565b600080600080600085870361080081121561371957600080fd5b86356001600160401b0381111561372f57600080fd5b61373b89828a01613566565b9097509550506107c0601f198201121561375457600080fd5b506020860192506107e08601356001600160401b0381111561377557600080fd5b613781888289016135c3565b969995985093965092949392505050565b60008060008060008060006107a0888a0312156137ae57600080fd5b87356001600160401b038111156137c457600080fd5b6137d08a828b01613566565b90985096506137e490508960208a016135aa565b94506107608801356001600160401b0381111561380057600080fd5b8801601f81018a1361381157600080fd5b80356001600160401b0381111561382757600080fd5b8a602060608302840101111561383c57600080fd5b602091909101945092506107808801356001600160401b0381111561386057600080fd5b61386c8a828b016135c3565b989b979a50959850939692959293505050565b600081518084526020840193506020830160005b828110156138b1578151865260209586019590910190600101613893565b5093949350505050565b82151581526040602082015260006138d6604083018461387f565b949350505050565b600080604083850312156138f157600080fd5b50508035926020909101359150565b6040808252835190820181905260009060208501906060840190835b8181101561394657835180518452602090810151818501529093019260409092019160010161391c565b50508381036020850152610a10818661387f565b60008060008060008060a0878903121561397357600080fd5b61397c876134b8565b955060208701359450604087013593506060870135925060808701356001600160401b038111156139ac57600080fd5b8701601f810189136139bd57600080fd5b80356001600160401b038111156139d357600080fd5b8960208284010111156139e557600080fd5b60208201935080925050509295509295509295565b600080600080600080600080888a03611680811215613a1857600080fd5b89356001600160401b03811115613a2e57600080fd5b613a3a8c828d01613566565b909a50985050610b00601f1982011215613a5357600080fd5b60208a019650610b208a01356001600160401b03811115613a7357600080fd5b613a7f8c828d01613566565b909750955050610b20610b3f1982011215613a9957600080fd5b50610b40890192506116608901356001600160401b03811115613abb57600080fd5b613ac78b828c016135c3565b999c989b5096995094979396929594505050565b60008060408385031215613aee57600080fd5b613af7836134b8565b946020939093013593505050565b600181811c90821680613b1957607f821691505b6020821081036135bd57634e487b7160e01b600052602260045260246000fd5b61010081833761064061010082016101008401375050565b6107408101610f358284613b39565b60008251613b72818460208701613503565b9190910192915050565b6000823561015e19833603018112613b7257600080fd5b81835260006001600160fb1b03831115613bac57600080fd5b8260051b80836020870137939093016020019392505050565b602080825282358282015282013560408201526040808301606083013760406080830160a0830137613c0760e0820160c0840180358252602090810135910152565b6000610100830135601e19843603018112613c2157600080fd5b83016020810190356001600160401b03811115613c3d57600080fd5b8060051b3603821315613c4f57600080fd5b610160610120850152613c6761018085018284613b93565b610120860135610140868101919091529095013561016090940193909352509192915050565b8051801515811461074f57600080fd5b600060208284031215613caf57600080fd5b61334882613c8d565b634e487b7160e01b600052604160045260246000fd5b634e487b7160e01b600052603260045260246000fd5b601f821115613d2e57806000526020600020601f840160051c81016020851015613d0b5750805b601f840160051c820191505b81811015613d2b5760008155600101613d17565b50505b505050565b6001600160401b03831115613d4a57613d4a613cb8565b613d5e83613d588354613b05565b83613ce4565b6000601f841160018114613d925760008515613d7a5750838201355b600019600387901b1c1916600186901b178355613d2b565b600083815260209020601f19861690835b82811015613dc35786850135825560209485019460019092019101613da3565b5086821015613de05760001960f88860031b161c19848701351681555b505060018560011b0183555050505050565b634e487b7160e01b600052601160045260246000fd5b81810381811115610f3557610f35613df2565b80820180821115610f3557610f35613df2565b634e487b7160e01b600052601260045260246000fd5b6107c081016101008383376106c0610100840161010084013792915050565b600082613e8057634e487b7160e01b600052601260045260246000fd5b500490565b8082028115828204841417610f3557610f35613df2565b602081526000613348602083018461387f565b60008060408385031215613ec257600080fd5b613ecb83613c8d565b9150602083015190509250929050565b610b008101610100838337610a00610100840161010084013792915050565b610b208101610100838337610a2061010084016101008401379291505056fee85c8c79cebe1b6656a265affa1c69c79539e5ae9a9c9229f5b5d89619781080983b8264b64c9863a439320eb632213f6e5ca279753b012988656784757d9775eae287c62f1ff4911334dee03f631d5dded5284b1b03ea7bc1d6282916c7249f30644e72e131a029b85045b68181585d2833e84879b9709143e1f593f0000001060c89ce5c263405370a08b6d0302b0bab3eedb83920ee0a677297dc392126f1a26469706673582212204f136822f6df43268c97ca811027671ee45b29f0338fe9560d1c79541ca4a96b64736f6c634300081b0033",
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

// Burn is a paid mutator transaction binding the contract method 0xb390c0ab.
//
// Solidity: function burn(uint256 accountId, uint256 amount) returns(bool)
func (_Enygma *EnygmaTransactor) Burn(opts *bind.TransactOpts, accountId *big.Int, amount *big.Int) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "burn", accountId, amount)
}

// Burn is a paid mutator transaction binding the contract method 0xb390c0ab.
//
// Solidity: function burn(uint256 accountId, uint256 amount) returns(bool)
func (_Enygma *EnygmaSession) Burn(accountId *big.Int, amount *big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.Burn(&_Enygma.TransactOpts, accountId, amount)
}

// Burn is a paid mutator transaction binding the contract method 0xb390c0ab.
//
// Solidity: function burn(uint256 accountId, uint256 amount) returns(bool)
func (_Enygma *EnygmaTransactorSession) Burn(accountId *big.Int, amount *big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.Burn(&_Enygma.TransactOpts, accountId, amount)
}

// Deposit is a paid mutator transaction binding the contract method 0x12dcc88b.
//
// Solidity: function deposit((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[50]) proof, ((((uint256,uint256),(uint256[2],uint256[2]),(uint256,uint256)),uint256[],uint256,uint256)) withdrawParam, uint256[] participantIds) returns(bool)
func (_Enygma *EnygmaTransactor) Deposit(opts *bind.TransactOpts, commitmentDeltas []IEnygmaPoint, proof IEnygmaDepositProof, withdrawParam IEnygmaWithdrawParams, participantIds []*big.Int) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "deposit", commitmentDeltas, proof, withdrawParam, participantIds)
}

// Deposit is a paid mutator transaction binding the contract method 0x12dcc88b.
//
// Solidity: function deposit((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[50]) proof, ((((uint256,uint256),(uint256[2],uint256[2]),(uint256,uint256)),uint256[],uint256,uint256)) withdrawParam, uint256[] participantIds) returns(bool)
func (_Enygma *EnygmaSession) Deposit(commitmentDeltas []IEnygmaPoint, proof IEnygmaDepositProof, withdrawParam IEnygmaWithdrawParams, participantIds []*big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.Deposit(&_Enygma.TransactOpts, commitmentDeltas, proof, withdrawParam, participantIds)
}

// Deposit is a paid mutator transaction binding the contract method 0x12dcc88b.
//
// Solidity: function deposit((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[50]) proof, ((((uint256,uint256),(uint256[2],uint256[2]),(uint256,uint256)),uint256[],uint256,uint256)) withdrawParam, uint256[] participantIds) returns(bool)
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

// MintSupply is a paid mutator transaction binding the contract method 0xff98feae.
//
// Solidity: function mintSupply(uint256 amount, uint256 recipientId) returns(bool)
func (_Enygma *EnygmaTransactor) MintSupply(opts *bind.TransactOpts, amount *big.Int, recipientId *big.Int) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "mintSupply", amount, recipientId)
}

// MintSupply is a paid mutator transaction binding the contract method 0xff98feae.
//
// Solidity: function mintSupply(uint256 amount, uint256 recipientId) returns(bool)
func (_Enygma *EnygmaSession) MintSupply(amount *big.Int, recipientId *big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.MintSupply(&_Enygma.TransactOpts, amount, recipientId)
}

// MintSupply is a paid mutator transaction binding the contract method 0xff98feae.
//
// Solidity: function mintSupply(uint256 amount, uint256 recipientId) returns(bool)
func (_Enygma *EnygmaTransactorSession) MintSupply(amount *big.Int, recipientId *big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.MintSupply(&_Enygma.TransactOpts, amount, recipientId)
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

// RegisterAccount is a paid mutator transaction binding the contract method 0xa276a208.
//
// Solidity: function registerAccount(address addr, uint256 accountId, uint256 publicKey, uint256 randomness, bytes viewKey) returns(bool)
func (_Enygma *EnygmaTransactor) RegisterAccount(opts *bind.TransactOpts, addr common.Address, accountId *big.Int, publicKey *big.Int, randomness *big.Int, viewKey []byte) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "registerAccount", addr, accountId, publicKey, randomness, viewKey)
}

// RegisterAccount is a paid mutator transaction binding the contract method 0xa276a208.
//
// Solidity: function registerAccount(address addr, uint256 accountId, uint256 publicKey, uint256 randomness, bytes viewKey) returns(bool)
func (_Enygma *EnygmaSession) RegisterAccount(addr common.Address, accountId *big.Int, publicKey *big.Int, randomness *big.Int, viewKey []byte) (*types.Transaction, error) {
	return _Enygma.Contract.RegisterAccount(&_Enygma.TransactOpts, addr, accountId, publicKey, randomness, viewKey)
}

// RegisterAccount is a paid mutator transaction binding the contract method 0xa276a208.
//
// Solidity: function registerAccount(address addr, uint256 accountId, uint256 publicKey, uint256 randomness, bytes viewKey) returns(bool)
func (_Enygma *EnygmaTransactorSession) RegisterAccount(addr common.Address, accountId *big.Int, publicKey *big.Int, randomness *big.Int, viewKey []byte) (*types.Transaction, error) {
	return _Enygma.Contract.RegisterAccount(&_Enygma.TransactOpts, addr, accountId, publicKey, randomness, viewKey)
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

// Transfer is a paid mutator transaction binding the contract method 0xc50c9b73.
//
// Solidity: function transfer((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[80]) proof, (uint256,uint256)[] usdrCommitmentDeltas, (uint256[8],uint256[81]) usdrProof, uint256[] participantIds) returns(bool)
func (_Enygma *EnygmaTransactor) Transfer(opts *bind.TransactOpts, commitmentDeltas []IEnygmaPoint, proof IEnygmaProof, usdrCommitmentDeltas []IEnygmaPoint, usdrProof IEnygmaUsdrProof, participantIds []*big.Int) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "transfer", commitmentDeltas, proof, usdrCommitmentDeltas, usdrProof, participantIds)
}

// Transfer is a paid mutator transaction binding the contract method 0xc50c9b73.
//
// Solidity: function transfer((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[80]) proof, (uint256,uint256)[] usdrCommitmentDeltas, (uint256[8],uint256[81]) usdrProof, uint256[] participantIds) returns(bool)
func (_Enygma *EnygmaSession) Transfer(commitmentDeltas []IEnygmaPoint, proof IEnygmaProof, usdrCommitmentDeltas []IEnygmaPoint, usdrProof IEnygmaUsdrProof, participantIds []*big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.Transfer(&_Enygma.TransactOpts, commitmentDeltas, proof, usdrCommitmentDeltas, usdrProof, participantIds)
}

// Transfer is a paid mutator transaction binding the contract method 0xc50c9b73.
//
// Solidity: function transfer((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[80]) proof, (uint256,uint256)[] usdrCommitmentDeltas, (uint256[8],uint256[81]) usdrProof, uint256[] participantIds) returns(bool)
func (_Enygma *EnygmaTransactorSession) Transfer(commitmentDeltas []IEnygmaPoint, proof IEnygmaProof, usdrCommitmentDeltas []IEnygmaPoint, usdrProof IEnygmaUsdrProof, participantIds []*big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.Transfer(&_Enygma.TransactOpts, commitmentDeltas, proof, usdrCommitmentDeltas, usdrProof, participantIds)
}

// TransferWithFee is a paid mutator transaction binding the contract method 0x24927892.
//
// Solidity: function transferWithFee((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[54]) proof, uint256[] participantIds) returns(bool)
func (_Enygma *EnygmaTransactor) TransferWithFee(opts *bind.TransactOpts, commitmentDeltas []IEnygmaPoint, proof IEnygmaFeeProof, participantIds []*big.Int) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "transferWithFee", commitmentDeltas, proof, participantIds)
}

// TransferWithFee is a paid mutator transaction binding the contract method 0x24927892.
//
// Solidity: function transferWithFee((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[54]) proof, uint256[] participantIds) returns(bool)
func (_Enygma *EnygmaSession) TransferWithFee(commitmentDeltas []IEnygmaPoint, proof IEnygmaFeeProof, participantIds []*big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.TransferWithFee(&_Enygma.TransactOpts, commitmentDeltas, proof, participantIds)
}

// TransferWithFee is a paid mutator transaction binding the contract method 0x24927892.
//
// Solidity: function transferWithFee((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[54]) proof, uint256[] participantIds) returns(bool)
func (_Enygma *EnygmaTransactorSession) TransferWithFee(commitmentDeltas []IEnygmaPoint, proof IEnygmaFeeProof, participantIds []*big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.TransferWithFee(&_Enygma.TransactOpts, commitmentDeltas, proof, participantIds)
}

// Withdraw is a paid mutator transaction binding the contract method 0x2e59d059.
//
// Solidity: function withdraw((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[50]) proof, (uint256,address,uint256)[] depositParams, uint256[] participantIds) returns(bool, uint256[])
func (_Enygma *EnygmaTransactor) Withdraw(opts *bind.TransactOpts, commitmentDeltas []IEnygmaPoint, proof IEnygmaWithdrawProof, depositParams []IEnygmaDepositParams, participantIds []*big.Int) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "withdraw", commitmentDeltas, proof, depositParams, participantIds)
}

// Withdraw is a paid mutator transaction binding the contract method 0x2e59d059.
//
// Solidity: function withdraw((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[50]) proof, (uint256,address,uint256)[] depositParams, uint256[] participantIds) returns(bool, uint256[])
func (_Enygma *EnygmaSession) Withdraw(commitmentDeltas []IEnygmaPoint, proof IEnygmaWithdrawProof, depositParams []IEnygmaDepositParams, participantIds []*big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.Withdraw(&_Enygma.TransactOpts, commitmentDeltas, proof, depositParams, participantIds)
}

// Withdraw is a paid mutator transaction binding the contract method 0x2e59d059.
//
// Solidity: function withdraw((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[50]) proof, (uint256,address,uint256)[] depositParams, uint256[] participantIds) returns(bool, uint256[])
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
	AddedBank              common.Address
	TotalRegisteredParties *big.Int
	Raw                    types.Log // Blockchain specific contextual infos
}

// FilterAccountRegistered is a free log retrieval operation binding the contract event 0xefd1ddef00b1051abc144c2e895de70a10dbbc3ad8985118c74c15e40e3d391f.
//
// Solidity: event AccountRegistered(address indexed addedBank, uint256 totalRegisteredParties)
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
// Solidity: event AccountRegistered(address indexed addedBank, uint256 totalRegisteredParties)
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
// Solidity: event AccountRegistered(address indexed addedBank, uint256 totalRegisteredParties)
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
