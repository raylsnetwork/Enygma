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

// IEnygmaRelayerProof is an auto generated low-level Go binding around an user-defined struct.
type IEnygmaRelayerProof struct {
	Proof         [8]*big.Int
	Commitments   [2]*big.Int
	CommitmentPok [2]*big.Int
	PublicSignal  [80]*big.Int
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
	ABI: "[{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_epochInterval\",\"type\":\"uint256\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[],\"name\":\"AlreadyInitialized\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"BalanceMismatch\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"BurnExceedsModulus\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidBlockNumber\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidProof\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidPublicInputs\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NotInitialized\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NotOwner\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NotRegistered\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NullifierAlreadyUsed\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ReentrantCall\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"RelayerBindingMismatch\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"VerifierNotFound\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ZeroAddress\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ZkDvpOperationFailed\",\"type\":\"error\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"addedBank\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"totalRegisteredParties\",\"type\":\"uint256\"}],\"name\":\"AccountRegistered\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"bankIndex\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"burnValue\",\"type\":\"uint256\"}],\"name\":\"BurnSuccessful\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"commitment\",\"type\":\"uint256\"}],\"name\":\"Commitment\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"lastblockNum\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"to\",\"type\":\"uint256\"}],\"name\":\"SupplyMinted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"maxBankCount\",\"type\":\"uint256\"}],\"name\":\"TokenInitialized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"senderAddress\",\"type\":\"address\"}],\"name\":\"TransactionSuccessful\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"verifierAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"totalRegisteredVerifiers\",\"type\":\"uint256\"}],\"name\":\"VerifierRegistered\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"DepositVerifierAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"GetBlckHash\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"Name\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"Symbol\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"TotalRegisteredBanks\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"TotalSupply\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"VerifierAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"WithdrawVerifierAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"ZkdvpAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"}],\"name\":\"addDepositVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"}],\"name\":\"addFeeVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"p1x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"p1y\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"p2x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"p2y\",\"type\":\"uint256\"}],\"name\":\"addPedComm\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"}],\"name\":\"addRelayerVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"}],\"name\":\"addVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"splitCount\",\"type\":\"uint256\"}],\"name\":\"addWithdrawVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"zkDvp\",\"type\":\"address\"}],\"name\":\"addZkDvp\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"addressToAccountId\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"balanceCommitments\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"accountId\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"burn\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"check\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"commitmentDeltas\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[50]\",\"name\":\"public_signal\",\"type\":\"uint256[50]\"}],\"internalType\":\"structIEnygma.DepositProof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"components\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"internalType\":\"structIZkDvp.G1Point\",\"name\":\"a\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"uint256[2]\",\"name\":\"x\",\"type\":\"uint256[2]\"},{\"internalType\":\"uint256[2]\",\"name\":\"y\",\"type\":\"uint256[2]\"}],\"internalType\":\"structIZkDvp.G2Point\",\"name\":\"b\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"internalType\":\"structIZkDvp.G1Point\",\"name\":\"c\",\"type\":\"tuple\"}],\"internalType\":\"structIZkDvp.SnarkProof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"internalType\":\"uint256[]\",\"name\":\"statement\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256\",\"name\":\"numberOfInputs\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"numberOfOutputs\",\"type\":\"uint256\"}],\"internalType\":\"structIZkDvp.JoinSplitTransaction\",\"name\":\"transaction\",\"type\":\"tuple\"}],\"internalType\":\"structIEnygma.WithdrawParams\",\"name\":\"withdrawParam\",\"type\":\"tuple\"},{\"internalType\":\"uint256[]\",\"name\":\"participantIds\",\"type\":\"uint256[]\"}],\"name\":\"deposit\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"}],\"name\":\"derivePk\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"randomness\",\"type\":\"uint256\"}],\"name\":\"derivePkH\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"epochInterval\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"accountId\",\"type\":\"uint256\"}],\"name\":\"getBalance\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"count\",\"type\":\"uint256\"}],\"name\":\"getPublicValues\",\"outputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"balances\",\"type\":\"tuple[]\"},{\"internalType\":\"uint256[]\",\"name\":\"keys\",\"type\":\"uint256[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"initialize\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"lastBlockNum\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"recipientId\",\"type\":\"uint256\"}],\"name\":\"mintSupply\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"randomness\",\"type\":\"uint256\"}],\"name\":\"pedCom\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"publicKeys\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"addr\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"accountId\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"publicKey\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"randomness\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"viewKey\",\"type\":\"bytes\"}],\"name\":\"registerAccount\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalSupplyAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalSupplyX\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalSupplyY\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"commitmentDeltas\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[80]\",\"name\":\"public_signal\",\"type\":\"uint256[80]\"}],\"internalType\":\"structIEnygma.Proof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[2]\",\"name\":\"commitments\",\"type\":\"uint256[2]\"},{\"internalType\":\"uint256[2]\",\"name\":\"commitmentPok\",\"type\":\"uint256[2]\"},{\"internalType\":\"uint256[80]\",\"name\":\"public_signal\",\"type\":\"uint256[80]\"}],\"internalType\":\"structIEnygma.RelayerProof\",\"name\":\"relayerProof\",\"type\":\"tuple\"},{\"internalType\":\"uint256[]\",\"name\":\"participantIds\",\"type\":\"uint256[]\"}],\"name\":\"transfer\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"commitmentDeltas\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[54]\",\"name\":\"public_signal\",\"type\":\"uint256[54]\"}],\"internalType\":\"structIEnygma.FeeProof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"internalType\":\"uint256[]\",\"name\":\"participantIds\",\"type\":\"uint256[]\"}],\"name\":\"transferWithFee\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"viewKeys\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"commitmentDeltas\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[50]\",\"name\":\"public_signal\",\"type\":\"uint256[50]\"}],\"internalType\":\"structIEnygma.WithdrawProof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"erc20Adress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"publicKey\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.DepositParams[]\",\"name\":\"depositParams\",\"type\":\"tuple[]\"},{\"internalType\":\"uint256[]\",\"name\":\"participantIds\",\"type\":\"uint256[]\"}],\"name\":\"withdraw\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"},{\"internalType\":\"uint256[]\",\"name\":\"\",\"type\":\"uint256[]\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
	Bin: "0x60c060405234801561001057600080fd5b506040516160053803806160058339818101604052810190610032919061012e565b60008111610075576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040161006c906101b8565b60405180910390fd5b3373ffffffffffffffffffffffffffffffffffffffff1660808173ffffffffffffffffffffffffffffffffffffffff1681525050600080819055508060a081815250506100c7816100d360201b60201c565b600181905550506102a9565b60008182436100e29190610236565b6100ec9190610267565b9050919050565b600080fd5b6000819050919050565b61010b816100f8565b811461011657600080fd5b50565b60008151905061012881610102565b92915050565b600060208284031215610144576101436100f3565b5b600061015284828501610119565b91505092915050565b600082825260208201905092915050565b7f65706f6368496e74657276616c206d757374206265203e203000000000000000600082015250565b60006101a260198361015b565b91506101ad8261016c565b602082019050919050565b600060208201905081810360008301526101d181610195565b9050919050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601260045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b6000610241826100f8565b915061024c836100f8565b92508261025c5761025b6101d8565b5b828204905092915050565b6000610272826100f8565b915061027d836100f8565b925082820261028b816100f8565b915082820484148315176102a2576102a1610207565b5b5092915050565b60805160a051615cea61031b60003960008181610a8001528181613bbd0152613bde01526000818161087e0152818161146601528181611693015281816117a901528181611a2101528181611d6b0152818161216901528181612343015281816124dc01526126b60152615cea6000f3fe608060405234801561001057600080fd5b506004361061023d5760003560e01c80638052474d1161013b578063c1ab48fc116100b8578063f828f50b1161007c578063f828f50b1461079c578063f8344434146107ba578063fe1e3e9e146107ea578063fe877fc91461081a578063ff98feae1461084a5761023d565b8063c1ab48fc146106aa578063c3d842bd146106da578063c680f4101461070a578063ce630c181461073a578063ea0d45731461076b5761023d565b8063919840ad116100ff578063919840ad146105dd578063a276a208146105fb578063a44b47f71461062b578063a9c58a7e14610649578063b390c0ab1461067a5761023d565b80638052474d146105355780638129fc1c1461055357806384aaa2de14610571578063874ed5b51461058f5780639000b3d6146105ad5761023d565b80632c0457e8116101c957806367511a4d1161018d57806367511a4d1461047957806371929e2a14610497578063723dbbc4146104b5578063743873b4146104e65780637d894a16146105045761023d565b80632c0457e8146103be5780632e59d059146103dc5780633045aaf31461040d578063368990421461042b5780634e466c53146104495761023d565b806312dcc88b1161021057806312dcc88b146102de578063132ce4d41461030e5780631a4e1aa11461033f5780631e0104391461035d578063249278921461038e5761023d565b80630197d9421461024257806307da47ea1461027257806309b1ef26146102905780630cf1839c146102ae575b600080fd5b61025c600480360381019061025791906146b0565b61087a565b60405161026991906146f8565b60405180910390f35b61027a610a54565b6040516102879190614722565b60405180910390f35b610298610a7e565b6040516102a59190614756565b60405180910390f35b6102c860048036038101906102c3919061479d565b610aa2565b6040516102d5919061485a565b60405180910390f35b6102f860048036038101906102f3919061497b565b610b42565b60405161030591906146f8565b60405180910390f35b61032860048036038101906103239190614a41565b610f08565b604051610336929190614aa8565b60405180910390f35b610347610f24565b6040516103549190614722565b60405180910390f35b6103776004803603810190610372919061479d565b610f4e565b604051610385929190614aa8565b60405180910390f35b6103a860048036038101906103a39190614af1565b610fbb565b6040516103b591906146f8565b60405180910390f35b6103c6611101565b6040516103d39190614722565b60405180910390f35b6103f660048036038101906103f19190614bfe565b61112b565b604051610404929190614d88565b60405180910390f35b61041561141f565b6040516104229190614e0d565b60405180910390f35b61043361145c565b6040516104409190614756565b60405180910390f35b610463600480360381019061045e91906146b0565b611462565b60405161047091906146f8565b60405180910390f35b6104816115e9565b60405161048e9190614756565b60405180910390f35b61049f6115ef565b6040516104ac9190614756565b60405180910390f35b6104cf60048036038101906104ca919061479d565b6115f5565b6040516104dd929190614aa8565b60405180910390f35b6104ee61160a565b6040516104fb9190614756565b60405180910390f35b61051e60048036038101906105199190614e2f565b611614565b60405161052c929190614aa8565b60405180910390f35b61053d611652565b60405161054a9190614e0d565b60405180910390f35b61055b61168f565b60405161056891906146f8565b60405180910390f35b610579611771565b6040516105869190614756565b60405180910390f35b61059761177b565b6040516105a49190614722565b60405180910390f35b6105c760048036038101906105c291906146b0565b6117a5565b6040516105d491906146f8565b60405180910390f35b6105e561197f565b6040516105f291906146f8565b60405180910390f35b61061560048036038101906106109190614ec5565b611a1d565b60405161062291906146f8565b60405180910390f35b610633611c1b565b6040516106409190614756565b60405180910390f35b610663600480360381019061065e919061479d565b611c25565b60405161067192919061503d565b60405180910390f35b610694600480360381019061068f9190614e2f565b611d67565b6040516106a191906146f8565b60405180910390f35b6106c460048036038101906106bf91906146b0565b611f7f565b6040516106d19190614756565b60405180910390f35b6106f460048036038101906106ef91906150b4565b611f97565b60405161070191906146f8565b60405180910390f35b610724600480360381019061071f919061479d565b612101565b6040516107319190614756565b60405180910390f35b610754600480360381019061074f919061479d565b612119565b604051610762929190614aa8565b60405180910390f35b61078560048036038101906107809190614e2f565b61212e565b604051610793929190614aa8565b60405180910390f35b6107a461215f565b6040516107b19190614756565b60405180910390f35b6107d460048036038101906107cf91906146b0565b612165565b6040516107e191906146f8565b60405180910390f35b61080460048036038101906107ff91906146b0565b61233f565b60405161081191906146f8565b60405180910390f35b610834600480360381019061082f919061515e565b6124d8565b60405161084191906146f8565b60405180910390f35b610864600480360381019061085f9190614e2f565b6126b2565b60405161087191906146f8565b60405180910390f35b60007f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614610901576040517f30cd747100000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b600073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff1603610967576040517fd92e233d00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b81601260006006815260200190815260200160002060006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555081600860006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055508173ffffffffffffffffffffffffffffffffffffffff167f983b8264b64c9863a439320eb632213f6e5ca279753b012988656784757d9775600254604051610a439190614756565b60405180910390a260019050919050565b6000600860009054906101000a900473ffffffffffffffffffffffffffffffffffffffff16905090565b7f000000000000000000000000000000000000000000000000000000000000000081565b600d6020528060005260406000206000915090508054610ac1906151cd565b80601f0160208091040260200160405190810160405280929190818152602001828054610aed906151cd565b8015610b3a5780601f10610b0f57610100808354040283529160200191610b3a565b820191906000526020600020905b815481529060010190602001808311610b1d57829003601f168201915b505050505081565b600080600e60003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205403610bbc576040517faba4733900000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b600160005414610bf8576040517f87138d5c00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b610c006128b6565b60006012600089899050815260200190815260200160002060009054906101000a900473ffffffffffffffffffffffffffffffffffffffff169050600073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff1603610ca1576040517fe25b142c00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b60008173ffffffffffffffffffffffffffffffffffffffff1687604051602401610ccb9190615281565b6040516020818303038152906040527f18e2c03f000000000000000000000000000000000000000000000000000000007bffffffffffffffffffffffffffffffffffffffffffffffffffffffff19166020820180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff8381831617835250505050604051610d5591906152d9565b600060405180830381855afa9150503d8060008114610d90576040519150601f19603f3d011682016040523d82523d6000602084013e610d95565b606091505b5050905080610dd0576040517f09bde33900000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b610de1876101000186868c8c61291a565b610dee8761010001612b96565b610dfb8761010001612bf2565b6000600960009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1690508073ffffffffffffffffffffffffffffffffffffffff16634ac058ed888060000190610e4e91906152f5565b6040518263ffffffff1660e01b8152600401610e6a91906155ba565b6020604051808303816000875af1158015610e89573d6000803e3d6000fd5b505050506040513d601f19601f82011682018060405250810190610ead9190615608565b610ee3576040517f68fdd57000000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b610eef8a8a8888612c97565b60019350505050610efe612dc0565b9695505050505050565b600080610f1786868686612ddd565b9150915094509492505050565b6000600960009054906101000a900473ffffffffffffffffffffffffffffffffffffffff16905090565b6000806000600b600060015481526020019081526020016000206000858152602001908152602001600020905060008160000154148015610f93575060008160010154145b15610fa657600060019250925050610fb6565b8060000154816001015492509250505b915091565b600080600e60003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205403611035576040517faba4733900000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b600160005414611071576040517f87138d5c00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b61107a846130d5565b61108b8461010001848489896132b2565b611098846101000161352e565b6110a58461010001613584565b6110b186868585613629565b3373ffffffffffffffffffffffffffffffffffffffff167fe85c8c79cebe1b6656a265affa1c69c79539e5ae9a9c9229f5b5d8961978108060405160405180910390a26001905095945050505050565b6000600760009054906101000a900473ffffffffffffffffffffffffffffffffffffffff16905090565b600060606000600e60003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054036111a8576040517faba4733900000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b6001600054146111e4576040517f87138d5c00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b6111ec6128b6565b60006011600088889050815260200190815260200160002060009054906101000a900473ffffffffffffffffffffffffffffffffffffffff169050600073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff160361128d576040517fe25b142c00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b60008173ffffffffffffffffffffffffffffffffffffffff16896040516024016112b79190615677565b6040516020818303038152906040527f18e2c03f000000000000000000000000000000000000000000000000000000007bffffffffffffffffffffffffffffffffffffffffffffffffffffffff19166020820180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff838183161783525050505060405161134191906152d9565b600060405180830381855afa9150503d806000811461137c576040519150601f19603f3d011682016040523d82523d6000602084013e611381565b606091505b50509050806113bc576040517f09bde33900000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b6113cd896101000187878e8e61291a565b6113da8961010001612b96565b6113e78961010001612bf2565b60006113f389896137eb565b90506114018c8c8989612c97565b60018194509450505050611413612dc0565b97509795505050505050565b60606040518060400160405280600281526020017f454e000000000000000000000000000000000000000000000000000000000000815250905090565b60015481565b60007f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16146114e9576040517f30cd747100000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b600073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff160361154f576040517fd92e233d00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b81600a60006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055508173ffffffffffffffffffffffffffffffffffffffff167f983b8264b64c9863a439320eb632213f6e5ca279753b012988656784757d97756002546040516115d89190614756565b60405180910390a260019050919050565b60045481565b60035481565b60008061160183613a5d565b91509150915091565b6000600154905090565b600080600080611623866115f5565b9150915060008061163387612119565b9150915061164384848484612ddd565b95509550505050509250929050565b60606040518060400160405280600681526020017f456e79676d610000000000000000000000000000000000000000000000000000815250905090565b60007f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614611716576040517f30cd747100000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b600160005403611752576040517f0dc149f000000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b6001600081905550600060038190555060016004819055506001905090565b6000600254905090565b6000600660009054906101000a900473ffffffffffffffffffffffffffffffffffffffff16905090565b60007f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff161461182c576040517f30cd747100000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b600073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff1603611892576040517fd92e233d00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b81600f60006006815260200190815260200160002060006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555081600660006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055508173ffffffffffffffffffffffffffffffffffffffff167f983b8264b64c9863a439320eb632213f6e5ca279753b012988656784757d977560025460405161196e9190614756565b60405180910390a260019050919050565b6000806000600190506000600190505b60025481116119c9576000806119a483610f4e565b915091506119b485858484612ddd565b8095508196505050826001019250505061198f565b50816003541415806119dd57508060045414155b15611a14576040517fca3e0a6800000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b60019250505090565b60007f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614611aa4576040517f30cd747100000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b84600c6000888152602001908152602001600020819055508282600d60008981526020019081526020016000209182611ade929190615879565b5085600e60008973ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002081905550600080611b31600087611614565b91509150604051806040016040528083815260200182815250600b6000600154815260200190815260200160002060008a81526020019081526020016000206000820151816000015560208201518160010155905050611b976003546004548484612ddd565b6003600060046000849190505583919050555050600260008154600101919050819055508873ffffffffffffffffffffffffffffffffffffffff167fefd1ddef00b1051abc144c2e895de70a10dbbc3ad8985118c74c15e40e3d391f600254604051611c039190614756565b60405180910390a26001925050509695505050505050565b6000600554905090565b6060808267ffffffffffffffff811115611c4257611c4161569e565b5b604051908082528060200260200182016040528015611c7b57816020015b611c6861462e565b815260200190600190039081611c605790505b5091508267ffffffffffffffff811115611c9857611c9761569e565b5b604051908082528060200260200182016040528015611cc65781602001602082028036833780820191505090505b50905060005b83811015611d6157611cdd81610f4e565b848381518110611cf057611cef615949565b5b6020026020010151600001858481518110611d0e57611d0d615949565b5b602002602001015160200182815250828152505050600c600082815260200190815260200160002054828281518110611d4a57611d49615949565b5b602002602001018181525050806001019050611ccc565b50915091565b60007f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614611dee576040517f30cd747100000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b7f060c89ce5c263405370a08b6d0302b0bab3eedb83920ee0a677297dc392126f1821115611e48576040517f0969723200000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b600080611e81847f060c89ce5c263405370a08b6d0302b0bab3eedb83920ee0a677297dc392126f1611e7a91906159a7565b6000611614565b91509150611e8e85613b15565b6000600b6000600154815260200190815260200160002060008781526020019081526020016000209050600080611ecf836000015484600101548787612ddd565b915091506000611edd613bb9565b9050604051806040016040528084815260200183815250600b600083815260200190815260200160002060008b81526020019081526020016000206000820151816000015560208201518160010155905050806001819055507f262a9a1794440b6af993000f5805d7f51b5a19d4c32fcb10a1c5216beb0616f48989604051611f67929190614aa8565b60405180910390a16001965050505050505092915050565b600e6020528060005260406000206000915090505481565b600080600e60003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205403612011576040517faba4733900000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b60016000541461204d576040517f87138d5c00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b61205a8588889050613c17565b6120678488889050613dea565b61207985610100018561018001613fbd565b61208a856101000184848a8a614040565b61209785610100016142bc565b6120a48561010001614312565b6120b087878585613629565b3373ffffffffffffffffffffffffffffffffffffffff167fe85c8c79cebe1b6656a265affa1c69c79539e5ae9a9c9229f5b5d8961978108060405160405180910390a2600190509695505050505050565b600c6020528060005260406000206000915090505481565b600080612125836143b7565b91509150915091565b600b602052816000526040600020602052806000526040600020600091509150508060000154908060010154905082565b60055481565b60007f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16146121ec576040517f30cd747100000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b600073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff1603612252576040517fd92e233d00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b81601360006006815260200190815260200160002060006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555081600960006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055508173ffffffffffffffffffffffffffffffffffffffff167f983b8264b64c9863a439320eb632213f6e5ca279753b012988656784757d977560025460405161232e9190614756565b60405180910390a260019050919050565b60007f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16146123c6576040517f30cd747100000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b600073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff160361242c576040517fd92e233d00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b81601060006006815260200190815260200160002060006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055508173ffffffffffffffffffffffffffffffffffffffff167f983b8264b64c9863a439320eb632213f6e5ca279753b012988656784757d97756002546040516124c79190614756565b60405180910390a260019050919050565b60007f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff161461255f576040517f30cd747100000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b600073ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff16036125c5576040517fd92e233d00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b826011600084815260200190815260200160002060006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555082600760006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055508273ffffffffffffffffffffffffffffffffffffffff167f983b8264b64c9863a439320eb632213f6e5ca279753b012988656784757d97756002546040516126a09190614756565b60405180910390a26001905092915050565b60007f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614612739576040517f30cd747100000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b600160005414612775576040517f87138d5c00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b600080612781856115f5565b915091506127956003546004548484612ddd565b6003600060046000849190505583919050555050846005600082825401925050819055506127c284613b15565b6000600b6000600154815260200190815260200160002060008681526020019081526020016000209050600080612803836000015484600101548787612ddd565b915091506000612811613bb9565b9050604051806040016040528084815260200183815250600b600083815260200190815260200160002060008a81526020019081526020016000206000820151816000015560208201518160010155905050806001819055506001547feae287c62f1ff4911334dee03f631d5dded5284b1b03ea7bc1d6282916c7249f8a8a60405161289e929190614aa8565b60405180910390a26001965050505050505092915050565b601560009054906101000a900460ff16156128fd576040517f37ed32e800000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b6001601560006101000a81548160ff021916908315150217905550565b600080612934600160025461292f91906159db565b611c25565b91509150600086869050905060005b81811015612b8b5760008888838181106129605761295f615949565b5b90506020020135905083818151811061297c5761297b615949565b5b60200260200101518a83600661299291906159db565b603281106129a3576129a2615949565b5b6020020135146129df576040517f6773afec00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b6000600183901b600c6129f291906159db565b9050858281518110612a0757612a06615949565b5b6020026020010151600001518b8260328110612a2657612a25615949565b5b6020020135141580612a795750858281518110612a4657612a45615949565b5b6020026020010151602001518b600183612a6091906159db565b60328110612a7157612a70615949565b5b602002013514155b15612ab0576040517f6773afec00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b6000600184901b6018612ac391906159db565b90508b8160328110612ad857612ad7615949565b5b6020020135898986818110612af057612aef615949565b5b90506040020160000135141580612b4657508b600182612b1091906159db565b60328110612b2157612b20615949565b5b6020020135898986818110612b3957612b38615949565b5b9050604002016020013514155b15612b7d576040517f6773afec00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b836001019350505050612943565b505050505050505050565b600081602460328110612bac57612bab615949565b5b602002013590506001548114612bee576040517f4e47846c00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b5050565b600081603160328110612c0857612c07615949565b5b602002013590506014600082815260200190815260200160002060009054906101000a900460ff1615612c67576040517fcad2ae0200000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b60016014600083815260200190815260200160002060006101000a81548160ff0219169083151502179055505050565b6000612ca1613bb9565b9050600085859050905060005b81811015612db0576000858583818110612ccb57612cca615949565b5b9050602002013590506000600b6000600154815260200190815260200160002060008381526020019081526020016000209050600080612d4d836000015484600101548d8d89818110612d2157612d20615949565b5b905060400201600001358e8e8a818110612d3e57612d3d615949565b5b90506040020160200135612ddd565b91509150604051806040016040528083815260200182815250600b60008981526020019081526020016000206000868152602001908152602001600020600082015181600001556020820151816001015590505084600101945050505050612cae565b5081600181905550505050505050565b6000601560006101000a81548160ff021916908315150217905550565b600080600086148015612df05750600185145b15612e00578383915091506130cc565b600084148015612e105750600183145b15612e20578585915091506130cc565b60007f30644e72e131a029b85045b68181585d2833e84879b9709143e1f593f000000180612e5157612e50615a0f565b5b858809905060007f30644e72e131a029b85045b68181585d2833e84879b9709143e1f593f000000180612e8757612e86615a0f565b5b858809905060007f30644e72e131a029b85045b68181585d2833e84879b9709143e1f593f000000180612ebd57612ebc615a0f565b5b7f30644e72e131a029b85045b68181585d2833e84879b9709143e1f593f000000180612eec57612eeb615a0f565b5b838509620292f809905060007f30644e72e131a029b85045b68181585d2833e84879b9709143e1f593f000000180612f2757612f26615a0f565b5b7f30644e72e131a029b85045b68181585d2833e84879b9709143e1f593f000000180612f5657612f55615a0f565b5b898b097f30644e72e131a029b85045b68181585d2833e84879b9709143e1f593f000000180612f8857612f87615a0f565b5b898d090890506000612fef847f30644e72e131a029b85045b68181585d2833e84879b9709143e1f593f000000180612fc357612fc2615a0f565b5b87620292fc097f30644e72e131a029b85045b68181585d2833e84879b9709143e1f593f000000161446f565b90507f30644e72e131a029b85045b68181585d2833e84879b9709143e1f593f0000001806130205761301f615a0f565b5b61305b7f30644e72e131a029b85045b68181585d2833e84879b9709143e1f593f00000018061305257613051615a0f565b5b856001086144b3565b830996507f30644e72e131a029b85045b68181585d2833e84879b9709143e1f593f00000018061308e5761308d615a0f565b5b6130c26130bd6001867f30644e72e131a029b85045b68181585d2833e84879b9709143e1f593f000000161446f565b6144b3565b8209955050505050505b94509492505050565b600073ffffffffffffffffffffffffffffffffffffffff16600a60009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff160361315d576040517fe25b142c00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b6000600a60009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16826040516024016131a99190615a9c565b6040516020818303038152906040527ffb336d8b000000000000000000000000000000000000000000000000000000007bffffffffffffffffffffffffffffffffffffffffffffffffffffffff19166020820180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff838183161783525050505060405161323391906152d9565b600060405180830381855afa9150503d806000811461326e576040519150601f19603f3d011682016040523d82523d6000602084013e613273565b606091505b50509050806132ae576040517f09bde33900000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b5050565b6000806132cc60016002546132c791906159db565b611c25565b91509150600086869050905060005b818110156135235760008888838181106132f8576132f7615949565b5b90506020020135905083818151811061331457613313615949565b5b60200260200101518a83600661332a91906159db565b6036811061333b5761333a615949565b5b602002013514613377576040517f6773afec00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b6000600183901b600c61338a91906159db565b905085828151811061339f5761339e615949565b5b6020026020010151600001518b82603681106133be576133bd615949565b5b602002013514158061341157508582815181106133de576133dd615949565b5b6020026020010151602001518b6001836133f891906159db565b6036811061340957613408615949565b5b602002013514155b15613448576040517f6773afec00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b6000600184901b601861345b91906159db565b90508b81603681106134705761346f615949565b5b602002013589898681811061348857613487615949565b5b905060400201600001351415806134de57508b6001826134a891906159db565b603681106134b9576134b8615949565b5b60200201358989868181106134d1576134d0615949565b5b9050604002016020013514155b15613515576040517f6773afec00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b8360010193505050506132db565b505050505050505050565b6001548160246036811061354557613544615949565b5b602002013514613581576040517f4e47846c00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b50565b60008160316036811061359a57613599615949565b5b602002013590506014600082815260200190815260200160002060009054906101000a900460ff16156135f9576040517fcad2ae0200000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b60016014600083815260200190815260200160002060006101000a81548160ff0219169083151502179055505050565b6000613633613bb9565b90506000600254905060005b818110156136cc5761365081614513565b61365b858583614569565b6136c157600b600060015481526020019081526020016000206000828152602001908152602001600020600b6000858152602001908152602001600020600083815260200190815260200160002060008201548160000155600182015481600101559050505b80600101905061363f565b50600086869050905060005b818110156137da5760008686838181106136f5576136f4615949565b5b9050602002013590506000600b6000600154815260200190815260200160002060008381526020019081526020016000209050600080613777836000015484600101548e8e8981811061374b5761374a615949565b5b905060400201600001358f8f8a81811061376857613767615949565b5b90506040020160200135612ddd565b91509150604051806040016040528083815260200182815250600b60008a815260200190815260200160002060008681526020019081526020016000206000820151816000015560208201518160010155905050846001019450505050506136d8565b508260018190555050505050505050565b60606000600960009054906101000a900473ffffffffffffffffffffffffffffffffffffffff169050600084849050905060008167ffffffffffffffff8111156138385761383761569e565b5b6040519080825280602002602001820160405280156138665781602001602082028036833780820191505090505b50905060005b82811015613a50576000600267ffffffffffffffff8111156138915761389061569e565b5b6040519080825280602002602001820160405280156138bf5781602001602082028036833780820191505090505b5090508787838181106138d5576138d4615949565b5b90506060020160000135816000815181106138f3576138f2615949565b5b60200260200101818152505087878381811061391257613911615949565b5b90506060020160400135816001815181106139305761392f615949565b5b6020026020010181815250506000808673ffffffffffffffffffffffffffffffffffffffff166383bf2edd846040518263ffffffff1660e01b81526004016139789190615ab8565b60408051808303816000875af1158015613996573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906139ba9190615aef565b91509150816139f5576040517f68fdd57000000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b80858581518110613a0957613a08615949565b5b602002602001018181525050807fef61e988d9804d573b4fc504760f55d3507094e4168fddc9245ac56fbfc419e460405160405180910390a283600101935050505061386c565b5080935050505092915050565b600080600083905060007f2491aba8d3a191a76e35bc47bd9afe6cc88fee14d607cbe779f2349047d5c157905060007f2e07297f8d3c3d7818dbddfd24c35583f9a9d4ed0cb0c1d1348dd8f7f99152d79050600080600190505b60008514613b055760006001861614613adf57613ad682828686612ddd565b80925081935050505b613ae984846145c1565b8094508195505050600285613afe9190615b2f565b9450613ab7565b8196508095505050505050915091565b6000613b1f613bb9565b9050600060025490506000600190505b818111613bb357613b3f81614513565b838114613ba857600b600060015481526020019081526020016000206000828152602001908152602001600020600b6000858152602001908152602001600020600083815260200190815260200160002060008201548160000155600182015481600101559050505b806001019050613b2f565b50505050565b60007f00000000000000000000000000000000000000000000000000000000000000007f000000000000000000000000000000000000000000000000000000000000000043613c089190615b2f565b613c129190615b60565b905090565b6000600f600083815260200190815260200160002060009054906101000a900473ffffffffffffffffffffffffffffffffffffffff169050600073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff1603613cb5576040517fe25b142c00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b60008173ffffffffffffffffffffffffffffffffffffffff1684604051602401613cdf9190615c00565b6040516020818303038152906040527fc5caafa4000000000000000000000000000000000000000000000000000000007bffffffffffffffffffffffffffffffffffffffffffffffffffffffff19166020820180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff8381831617835250505050604051613d6991906152d9565b600060405180830381855afa9150503d8060008114613da4576040519150601f19603f3d011682016040523d82523d6000602084013e613da9565b606091505b5050905080613de4576040517f09bde33900000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b50505050565b60006010600083815260200190815260200160002060009054906101000a900473ffffffffffffffffffffffffffffffffffffffff169050600073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff1603613e88576040517fe25b142c00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b60008173ffffffffffffffffffffffffffffffffffffffff1684604051602401613eb29190615c98565b6040516020818303038152906040527fb52b706e000000000000000000000000000000000000000000000000000000007bffffffffffffffffffffffffffffffffffffffffffffffffffffffff19166020820180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff8381831617835250505050604051613f3c91906152d9565b600060405180830381855afa9150503d8060008114613f77576040519150601f19603f3d011682016040523d82523d6000602084013e613f7c565b606091505b5050905080613fb7576040517f09bde33900000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b50505050565b60005b605081101561403b57818160508110613fdc57613fdb615949565b5b6020020135838260508110613ff457613ff3615949565b5b602002013514614030576040517f5d5c932500000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b806001019050613fc0565b505050565b60008061405a600160025461405591906159db565b611c25565b91509150600086869050905060005b818110156142b157600088888381811061408657614085615949565b5b9050602002013590508381815181106140a2576140a1615949565b5b60200260200101518a8360246140b891906159db565b605081106140c9576140c8615949565b5b602002013514614105576040517f6773afec00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b6000600183901b602a61411891906159db565b905085828151811061412d5761412c615949565b5b6020026020010151600001518b826050811061414c5761414b615949565b5b602002013514158061419f575085828151811061416c5761416b615949565b5b6020026020010151602001518b60018361418691906159db565b6050811061419757614196615949565b5b602002013514155b156141d6576040517f6773afec00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b6000600184901b60366141e991906159db565b90508b81605081106141fe576141fd615949565b5b602002013589898681811061421657614215615949565b5b9050604002016000013514158061426c57508b60018261423691906159db565b6050811061424757614246615949565b5b602002013589898681811061425f5761425e615949565b5b9050604002016020013514155b156142a3576040517f6773afec00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b836001019350505050614069565b505050505050505050565b600154816042605081106142d3576142d2615949565b5b60200201351461430f576040517f4e47846c00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b50565b600081604f6050811061432857614327615949565b5b602002013590506014600082815260200190815260200160002060009054906101000a900460ff1615614387576040517fcad2ae0200000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b60016014600083815260200190815260200160002060006101000a81548160ff0219169083151502179055505050565b600080600083905060007f16546696a66928d34f6be843f8a5afa2063161d92742811279454d60de532252905060007f109c1c7a758b3e8e54af1ce919fc24e1b986aab09a6b8082600f8694bb3c1b4b9050600080600190505b6000851461445f57600060018616146144395761443082828686612ddd565b80925081935050505b61444384846145c1565b80945081955050506002856144589190615b2f565b9450614411565b8196508095505050505050915091565b60008084905083851161448b57828161448891906159db565b90505b828061449a57614499615a0f565b5b600085836144a891906159a7565b089150509392505050565b600061450c8260027f30644e72e131a029b85045b68181585d2833e84879b9709143e1f593f00000016144e691906159a7565b7f30644e72e131a029b85045b68181585d2833e84879b9709143e1f593f00000016145db565b9050919050565b6000600b600060015481526020019081526020016000206000838152602001908152602001600020905060008160000154148015614555575060008160010154145b1561456557600181600101819055505b5050565b60008084849050905060005b818110156145b3578386868381811061459157614590615949565b5b90506020020135036145a8576001925050506145ba565b806001019050614575565b5060009150505b9392505050565b6000806145d084848686612ddd565b915091509250929050565b600060405160208152602080820152602060408201528460608201528360808201528260a082015260208160c08360055afa806000811461461f5782519350614624565b600080fd5b5050509392505050565b604051806040016040528060008152602001600081525090565b600080fd5b600080fd5b600073ffffffffffffffffffffffffffffffffffffffff82169050919050565b600061467d82614652565b9050919050565b61468d81614672565b811461469857600080fd5b50565b6000813590506146aa81614684565b92915050565b6000602082840312156146c6576146c5614648565b5b60006146d48482850161469b565b91505092915050565b60008115159050919050565b6146f2816146dd565b82525050565b600060208201905061470d60008301846146e9565b92915050565b61471c81614672565b82525050565b60006020820190506147376000830184614713565b92915050565b6000819050919050565b6147508161473d565b82525050565b600060208201905061476b6000830184614747565b92915050565b61477a8161473d565b811461478557600080fd5b50565b60008135905061479781614771565b92915050565b6000602082840312156147b3576147b2614648565b5b60006147c184828501614788565b91505092915050565b600081519050919050565b600082825260208201905092915050565b60005b838110156148045780820151818401526020810190506147e9565b60008484015250505050565b6000601f19601f8301169050919050565b600061482c826147ca565b61483681856147d5565b93506148468185602086016147e6565b61484f81614810565b840191505092915050565b600060208201905081810360008301526148748184614821565b905092915050565b600080fd5b600080fd5b600080fd5b60008083601f8401126148a1576148a061487c565b5b8235905067ffffffffffffffff8111156148be576148bd614881565b5b6020830191508360408202830111156148da576148d9614886565b5b9250929050565b600080fd5b600061074082840312156148fd576148fc6148e1565b5b81905092915050565b60006020828403121561491c5761491b6148e1565b5b81905092915050565b60008083601f84011261493b5761493a61487c565b5b8235905067ffffffffffffffff81111561495857614957614881565b5b60208301915083602082028301111561497457614973614886565b5b9250929050565b6000806000806000806107a0878903121561499957614998614648565b5b600087013567ffffffffffffffff8111156149b7576149b661464d565b5b6149c389828a0161488b565b965096505060206149d689828a016148e6565b94505061076087013567ffffffffffffffff8111156149f8576149f761464d565b5b614a0489828a01614906565b93505061078087013567ffffffffffffffff811115614a2657614a2561464d565b5b614a3289828a01614925565b92509250509295509295509295565b60008060008060808587031215614a5b57614a5a614648565b5b6000614a6987828801614788565b9450506020614a7a87828801614788565b9350506040614a8b87828801614788565b9250506060614a9c87828801614788565b91505092959194509250565b6000604082019050614abd6000830185614747565b614aca6020830184614747565b9392505050565b60006107c08284031215614ae857614ae76148e1565b5b81905092915050565b60008060008060006108008688031215614b0e57614b0d614648565b5b600086013567ffffffffffffffff811115614b2c57614b2b61464d565b5b614b388882890161488b565b95509550506020614b4b88828901614ad1565b9350506107e086013567ffffffffffffffff811115614b6d57614b6c61464d565b5b614b7988828901614925565b92509250509295509295909350565b60006107408284031215614b9f57614b9e6148e1565b5b81905092915050565b60008083601f840112614bbe57614bbd61487c565b5b8235905067ffffffffffffffff811115614bdb57614bda614881565b5b602083019150836060820283011115614bf757614bf6614886565b5b9250929050565b60008060008060008060006107a0888a031215614c1e57614c1d614648565b5b600088013567ffffffffffffffff811115614c3c57614c3b61464d565b5b614c488a828b0161488b565b97509750506020614c5b8a828b01614b88565b95505061076088013567ffffffffffffffff811115614c7d57614c7c61464d565b5b614c898a828b01614ba8565b945094505061078088013567ffffffffffffffff811115614cad57614cac61464d565b5b614cb98a828b01614925565b925092505092959891949750929550565b600081519050919050565b600082825260208201905092915050565b6000819050602082019050919050565b614cff8161473d565b82525050565b6000614d118383614cf6565b60208301905092915050565b6000602082019050919050565b6000614d3582614cca565b614d3f8185614cd5565b9350614d4a83614ce6565b8060005b83811015614d7b578151614d628882614d05565b9750614d6d83614d1d565b925050600181019050614d4e565b5085935050505092915050565b6000604082019050614d9d60008301856146e9565b8181036020830152614daf8184614d2a565b90509392505050565b600081519050919050565b600082825260208201905092915050565b6000614ddf82614db8565b614de98185614dc3565b9350614df98185602086016147e6565b614e0281614810565b840191505092915050565b60006020820190508181036000830152614e278184614dd4565b905092915050565b60008060408385031215614e4657614e45614648565b5b6000614e5485828601614788565b9250506020614e6585828601614788565b9150509250929050565b60008083601f840112614e8557614e8461487c565b5b8235905067ffffffffffffffff811115614ea257614ea1614881565b5b602083019150836001820283011115614ebe57614ebd614886565b5b9250929050565b60008060008060008060a08789031215614ee257614ee1614648565b5b6000614ef089828a0161469b565b9650506020614f0189828a01614788565b9550506040614f1289828a01614788565b9450506060614f2389828a01614788565b935050608087013567ffffffffffffffff811115614f4457614f4361464d565b5b614f5089828a01614e6f565b92509250509295509295509295565b600081519050919050565b600082825260208201905092915050565b6000819050602082019050919050565b604082016000820151614fa16000850182614cf6565b506020820151614fb46020850182614cf6565b50505050565b6000614fc68383614f8b565b60408301905092915050565b6000602082019050919050565b6000614fea82614f5f565b614ff48185614f6a565b9350614fff83614f7b565b8060005b838110156150305781516150178882614fba565b975061502283614fd2565b925050600181019050615003565b5085935050505092915050565b600060408201905081810360008301526150578185614fdf565b9050818103602083015261506b8184614d2a565b90509392505050565b6000610b00828403121561508b5761508a6148e1565b5b81905092915050565b6000610b8082840312156150ab576150aa6148e1565b5b81905092915050565b6000806000806000806116c087890312156150d2576150d1614648565b5b600087013567ffffffffffffffff8111156150f0576150ef61464d565b5b6150fc89828a0161488b565b9650965050602061510f89828a01615074565b945050610b2061512189828a01615094565b9350506116a087013567ffffffffffffffff8111156151435761514261464d565b5b61514f89828a01614925565b92509250509295509295509295565b6000806040838503121561517557615174614648565b5b60006151838582860161469b565b925050602061519485828601614788565b9150509250929050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052602260045260246000fd5b600060028204905060018216806151e557607f821691505b6020821081036151f8576151f761519e565b5b50919050565b600082905092915050565b82818337505050565b61521f6101008383615209565b5050565b600082905092915050565b61523b6106408383615209565b5050565b610740820161525160008301836151fe565b61525e6000850182615212565b5061526d610100830183615223565b61527b61010085018261522e565b50505050565b600061074082019050615297600083018461523f565b92915050565b600081905092915050565b60006152b3826147ca565b6152bd818561529d565b93506152cd8185602086016147e6565b80840191505092915050565b60006152e582846152a8565b915081905092915050565b600080fd5b60008235600161016003833603038112615312576153116152f0565b5b80830191505092915050565b600082905092915050565b600082905092915050565b60006153436020840184614788565b905092915050565b6040820161535c6000830183615334565b6153696000850182614cf6565b506153776020830183615334565b6153846020850182614cf6565b50505050565b600082905092915050565b600082905092915050565b6153ac60408383615209565b5050565b608082016153c16000830183615395565b6153ce60008501826153a0565b506153dc6040830183615395565b6153e960408501826153a0565b50505050565b61010082016154016000830183615329565b61540e600085018261534b565b5061541c604083018361538a565b61542960408501826153b0565b5061543760c0830183615329565b61544460c085018261534b565b50505050565b600080fd5b600080fd5b600080fd5b6000808335600160200384360303811261547657615475615454565b5b83810192508235915060208301925067ffffffffffffffff82111561549e5761549d61544a565b5b6020820236038313156154b4576154b361544f565b5b509250929050565b600082825260208201905092915050565b600080fd5b60006154de83856154bc565b93507f07ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff831115615511576155106154cd565b5b602083029250615522838584615209565b82840190509392505050565b60006101608301615542600084018461531e565b61554f60008601826153ef565b5061555e610100840184615459565b8583036101008701526155728382846154d2565b92505050615584610120840184615334565b615592610120860182614cf6565b506155a1610140840184615334565b6155af610140860182614cf6565b508091505092915050565b600060208201905081810360008301526155d4818461552e565b905092915050565b6155e5816146dd565b81146155f057600080fd5b50565b600081519050615602816155dc565b92915050565b60006020828403121561561e5761561d614648565b5b600061562c848285016155f3565b91505092915050565b610740820161564760008301836151fe565b6156546000850182615212565b50615663610100830183615223565b61567161010085018261522e565b50505050565b60006107408201905061568d6000830184615635565b92915050565b600082905092915050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052604160045260246000fd5b60008190508160005260206000209050919050565b60006020601f8301049050919050565b600082821b905092915050565b60006008830261572f7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff826156f2565b61573986836156f2565b95508019841693508086168417925050509392505050565b6000819050919050565b600061577661577161576c8461473d565b615751565b61473d565b9050919050565b6000819050919050565b6157908361575b565b6157a461579c8261577d565b8484546156ff565b825550505050565b600090565b6157b96157ac565b6157c4818484615787565b505050565b5b818110156157e8576157dd6000826157b1565b6001810190506157ca565b5050565b601f82111561582d576157fe816156cd565b615807846156e2565b81016020851015615816578190505b61582a615822856156e2565b8301826157c9565b50505b505050565b600082821c905092915050565b600061585060001984600802615832565b1980831691505092915050565b6000615869838361583f565b9150826002028217905092915050565b6158838383615693565b67ffffffffffffffff81111561589c5761589b61569e565b5b6158a682546151cd565b6158b18282856157ec565b6000601f8311600181146158e057600084156158ce578287013590505b6158d8858261585d565b865550615940565b601f1984166158ee866156cd565b60005b82811015615916578489013582556001820191506020850194506020810190506158f1565b86831015615933578489013561592f601f89168261583f565b8355505b6001600288020188555050505b50505050505050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052603260045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b60006159b28261473d565b91506159bd8361473d565b92508282039050818111156159d5576159d4615978565b5b92915050565b60006159e68261473d565b91506159f18361473d565b9250828201905080821115615a0957615a08615978565b5b92915050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601260045260246000fd5b600082905092915050565b615a566106c08383615209565b5050565b6107c08201615a6c60008301836151fe565b615a796000850182615212565b50615a88610100830183615a3e565b615a96610100850182615a49565b50505050565b60006107c082019050615ab26000830184615a5a565b92915050565b60006020820190508181036000830152615ad28184614d2a565b905092915050565b600081519050615ae981614771565b92915050565b60008060408385031215615b0657615b05614648565b5b6000615b14858286016155f3565b9250506020615b2585828601615ada565b9150509250929050565b6000615b3a8261473d565b9150615b458361473d565b925082615b5557615b54615a0f565b5b828204905092915050565b6000615b6b8261473d565b9150615b768361473d565b9250828202615b848161473d565b91508282048414831517615b9b57615b9a615978565b5b5092915050565b600082905092915050565b615bba610a008383615209565b5050565b610b008201615bd060008301836151fe565b615bdd6000850182615212565b50615bec610100830183615ba2565b615bfa610100850182615bad565b50505050565b6000610b0082019050615c166000830184615bbe565b92915050565b610b808201615c2e60008301836151fe565b615c3b6000850182615212565b50615c4a610100830183615395565b615c586101008501826153a0565b50615c67610140830183615395565b615c756101408501826153a0565b50615c84610180830183615ba2565b615c92610180850182615bad565b50505050565b6000610b8082019050615cae6000830184615c1c565b9291505056fea26469706673582212200dac74342b9713d7f03a0320b2ec33eeb049af9ee7444e76ca8a0528e5b7f76a64736f6c634300081b0033",
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

// AddRelayerVerifier is a paid mutator transaction binding the contract method 0xfe1e3e9e.
//
// Solidity: function addRelayerVerifier(address verifier) returns(bool)
func (_Enygma *EnygmaTransactor) AddRelayerVerifier(opts *bind.TransactOpts, verifier common.Address) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "addRelayerVerifier", verifier)
}

// AddRelayerVerifier is a paid mutator transaction binding the contract method 0xfe1e3e9e.
//
// Solidity: function addRelayerVerifier(address verifier) returns(bool)
func (_Enygma *EnygmaSession) AddRelayerVerifier(verifier common.Address) (*types.Transaction, error) {
	return _Enygma.Contract.AddRelayerVerifier(&_Enygma.TransactOpts, verifier)
}

// AddRelayerVerifier is a paid mutator transaction binding the contract method 0xfe1e3e9e.
//
// Solidity: function addRelayerVerifier(address verifier) returns(bool)
func (_Enygma *EnygmaTransactorSession) AddRelayerVerifier(verifier common.Address) (*types.Transaction, error) {
	return _Enygma.Contract.AddRelayerVerifier(&_Enygma.TransactOpts, verifier)
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

// Transfer is a paid mutator transaction binding the contract method 0xc3d842bd.
//
// Solidity: function transfer((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[80]) proof, (uint256[8],uint256[2],uint256[2],uint256[80]) relayerProof, uint256[] participantIds) returns(bool)
func (_Enygma *EnygmaTransactor) Transfer(opts *bind.TransactOpts, commitmentDeltas []IEnygmaPoint, proof IEnygmaProof, relayerProof IEnygmaRelayerProof, participantIds []*big.Int) (*types.Transaction, error) {
	return _Enygma.contract.Transact(opts, "transfer", commitmentDeltas, proof, relayerProof, participantIds)
}

// Transfer is a paid mutator transaction binding the contract method 0xc3d842bd.
//
// Solidity: function transfer((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[80]) proof, (uint256[8],uint256[2],uint256[2],uint256[80]) relayerProof, uint256[] participantIds) returns(bool)
func (_Enygma *EnygmaSession) Transfer(commitmentDeltas []IEnygmaPoint, proof IEnygmaProof, relayerProof IEnygmaRelayerProof, participantIds []*big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.Transfer(&_Enygma.TransactOpts, commitmentDeltas, proof, relayerProof, participantIds)
}

// Transfer is a paid mutator transaction binding the contract method 0xc3d842bd.
//
// Solidity: function transfer((uint256,uint256)[] commitmentDeltas, (uint256[8],uint256[80]) proof, (uint256[8],uint256[2],uint256[2],uint256[80]) relayerProof, uint256[] participantIds) returns(bool)
func (_Enygma *EnygmaTransactorSession) Transfer(commitmentDeltas []IEnygmaPoint, proof IEnygmaProof, relayerProof IEnygmaRelayerProof, participantIds []*big.Int) (*types.Transaction, error) {
	return _Enygma.Contract.Transfer(&_Enygma.TransactOpts, commitmentDeltas, proof, relayerProof, participantIds)
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
