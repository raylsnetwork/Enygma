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
        PublicSignal [2]*big.Int
}

// IEnygmaPoint is an auto generated low-level Go binding around an user-defined struct.
type IEnygmaPoint struct {
        C1 *big.Int
        C2 *big.Int
}

// IEnygmaProof is an auto generated low-level Go binding around an user-defined struct.
type IEnygmaProof struct {
        Proof        [8]*big.Int
        PublicSignal [31]*big.Int
}

// IEnygmaWithdrawParams is an auto generated low-level Go binding around an user-defined struct.
type IEnygmaWithdrawParams struct {
        Transaction IZkDvpJoinSplitTransaction
}

// IEnygmaWithdrawProof is an auto generated low-level Go binding around an user-defined struct.
type IEnygmaWithdrawProof struct {
        Proof        [8]*big.Int
        PublicSignal [1]*big.Int
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
        ABI: "[{\"inputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"addedBank\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"totalRegisteredParties\",\"type\":\"uint256\"}],\"name\":\"AccountRegistered\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"bankIndex\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"burnValue\",\"type\":\"uint256\"}],\"name\":\"BurnSuccessful\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"commitment\",\"type\":\"uint256\"}],\"name\":\"Commitment\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"lastblockNum\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"to\",\"type\":\"uint256\"}],\"name\":\"SupplyMinted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"maxBankCount\",\"type\":\"uint256\"}],\"name\":\"TokenInitialized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"senderAddress\",\"type\":\"address\"}],\"name\":\"TransactionSuccessful\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"verifierAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"totalRegisteredVerifiers\",\"type\":\"uint256\"}],\"name\":\"VerifierRegistered\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"DepositVerifierAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"Name\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"Symbol\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"TotalRegisteredBanks\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"TotalSupply\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"VerifierAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"WithdrawVerifierAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"ZkdvpAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"_totalSupply\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"accounts\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"depositVerifier\",\"type\":\"address\"}],\"name\":\"addDepositVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"p1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"p2\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"x2\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y2\",\"type\":\"uint256\"}],\"name\":\"addPedComm\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"verifier\",\"type\":\"address\"}],\"name\":\"addVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"withdrawVerifier\",\"type\":\"address\"}],\"name\":\"addWithdrawVerifier\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"zkdvpAddress\",\"type\":\"address\"}],\"name\":\"addZkDvp\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"bankIndex\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"burnValue\",\"type\":\"uint256\"}],\"name\":\"burn\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"check\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"commitments\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[2]\",\"name\":\"public_signal\",\"type\":\"uint256[2]\"}],\"internalType\":\"structIEnygma.DepositProof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"components\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"internalType\":\"structIZkDvp.G1Point\",\"name\":\"a\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"uint256[2]\",\"name\":\"x\",\"type\":\"uint256[2]\"},{\"internalType\":\"uint256[2]\",\"name\":\"y\",\"type\":\"uint256[2]\"}],\"internalType\":\"structIZkDvp.G2Point\",\"name\":\"b\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"internalType\":\"structIZkDvp.G1Point\",\"name\":\"c\",\"type\":\"tuple\"}],\"internalType\":\"structIZkDvp.SnarkProof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"internalType\":\"uint256[]\",\"name\":\"statement\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256\",\"name\":\"numberOfInputs\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"numberOfOutputs\",\"type\":\"uint256\"}],\"internalType\":\"structIZkDvp.JoinSplitTransaction\",\"name\":\"transaction\",\"type\":\"tuple\"}],\"internalType\":\"structIEnygma.WithdrawParams\",\"name\":\"withdrawParam\",\"type\":\"tuple\"},{\"internalType\":\"uint256[]\",\"name\":\"kIndex\",\"type\":\"uint256[]\"}],\"name\":\"deposit\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"depositVerifiers\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"v\",\"type\":\"uint256\"}],\"name\":\"derivePk\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"x2\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y2\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"r\",\"type\":\"uint256\"}],\"name\":\"derivePkH\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"x2\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y2\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"account\",\"type\":\"uint256\"}],\"name\":\"getBalance\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"x\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"y\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"size\",\"type\":\"uint256\"}],\"name\":\"getPublicValues\",\"outputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"\",\"type\":\"tuple[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"initialize\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"lastblockNum\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"to\",\"type\":\"uint256\"}],\"name\":\"mintSupply\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"v\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"r\",\"type\":\"uint256\"}],\"name\":\"pedCom\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"pubKeys\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"referenceBalance\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"addr\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"accountNum\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"k1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"k2\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"r\",\"type\":\"uint256\"}],\"name\":\"registerAccount\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalSupplyX\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalSupplyY\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"commitments\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[31]\",\"name\":\"public_signal\",\"type\":\"uint256[31]\"}],\"internalType\":\"structIEnygma.Proof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"internalType\":\"uint256[]\",\"name\":\"kIndex\",\"type\":\"uint256[]\"}],\"name\":\"transfer\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"verifiers\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"c1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"c2\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.Point[]\",\"name\":\"commitments\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"uint256[8]\",\"name\":\"proof\",\"type\":\"uint256[8]\"},{\"internalType\":\"uint256[1]\",\"name\":\"public_signal\",\"type\":\"uint256[1]\"}],\"internalType\":\"structIEnygma.WithdrawProof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"erc20Adress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"publicKey\",\"type\":\"uint256\"}],\"internalType\":\"structIEnygma.DepositParams[]\",\"name\":\"deposistParam\",\"type\":\"tuple[]\"},{\"internalType\":\"uint256[]\",\"name\":\"kIndex\",\"type\":\"uint256[]\"}],\"name\":\"withdraw\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"},{\"internalType\":\"uint256[]\",\"name\":\"\",\"type\":\"uint256[]\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"withdrawVerifiers\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"zkdvps\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
}

// EnygmaABI is the input ABI used to generate the binding from.
// Deprecated: Use EnygmaMetaData.ABI instead.
var EnygmaABI = EnygmaMetaData.ABI

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

// Name is a free data retrieval call binding the contract method 0x8052474d.
//
// Solidity: function Name() view returns(string)
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
// Solidity: function Name() view returns(string)
func (_Enygma *EnygmaSession) Name() (string, error) {
        return _Enygma.Contract.Name(&_Enygma.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x8052474d.
//
// Solidity: function Name() view returns(string)
func (_Enygma *EnygmaCallerSession) Name() (string, error) {
        return _Enygma.Contract.Name(&_Enygma.CallOpts)
}

// Symbol is a free data retrieval call binding the contract method 0x3045aaf3.
//
// Solidity: function Symbol() view returns(string)
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
// Solidity: function Symbol() view returns(string)
func (_Enygma *EnygmaSession) Symbol() (string, error) {
        return _Enygma.Contract.Symbol(&_Enygma.CallOpts)
}

// Symbol is a free data retrieval call binding the contract method 0x3045aaf3.
//
// Solidity: function Symbol() view returns(string)
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

// TotalSupply2 is a free data retrieval call binding the contract method 0x3eaaf86b.
//
// Solidity: function _totalSupply() view returns(uint256)
func (_Enygma *EnygmaCaller) TotalSupply2(opts *bind.CallOpts) (*big.Int, error) {
        var out []interface{}
        err := _Enygma.contract.Call(opts, &out, "_totalSupply")

        if err != nil {
                return *new(*big.Int), err
        }

        out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

        return out0, err

}

// TotalSupply2 is a free data retrieval call binding the contract method 0x3eaaf86b.
//
// Solidity: function _totalSupply() view returns(uint256)
func (_Enygma *EnygmaSession) TotalSupply2() (*big.Int, error) {
        return _Enygma.Contract.TotalSupply2(&_Enygma.CallOpts)
}

// TotalSupply2 is a free data retrieval call binding the contract method 0x3eaaf86b.
//
// Solidity: function _totalSupply() view returns(uint256)
func (_Enygma *EnygmaCallerSession) TotalSupply2() (*big.Int, error) {
        return _Enygma.Contract.TotalSupply2(&_Enygma.CallOpts)
}

// Accounts is a free data retrieval call binding the contract method 0x5e5c06e2.
//
// Solidity: function accounts(address ) view returns(uint256)
func (_Enygma *EnygmaCaller) Accounts(opts *bind.CallOpts, arg0 common.Address) (*big.Int, error) {
        var out []interface{}
        err := _Enygma.contract.Call(opts, &out, "accounts", arg0)

        if err != nil {
                return *new(*big.Int), err
        }

        out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

        return out0, err

}

// Accounts is a free data retrieval call binding the contract method 0x5e5c06e2.
//
// Solidity: function accounts(address ) view returns(uint256)
func (_Enygma *EnygmaSession) Accounts(arg0 common.Address) (*big.Int, error) {
        return _Enygma.Contract.Accounts(&_Enygma.CallOpts, arg0)
}

// Accounts is a free data retrieval call binding the contract method 0x5e5c06e2.
//
// Solidity: function accounts(address ) view returns(uint256)
func (_Enygma *EnygmaCallerSession) Accounts(arg0 common.Address) (*big.Int, error) {
        return _Enygma.Contract.Accounts(&_Enygma.CallOpts, arg0)
}

// AddPedComm is a free data retrieval call binding the contract method 0x132ce4d4.
//
// Solidity: function addPedComm(uint256 p1, uint256 p2, uint256 x2, uint256 y2) view returns(uint256, uint256)
func (_Enygma *EnygmaCaller) AddPedComm(opts *bind.CallOpts, p1 *big.Int, p2 *big.Int, x2 *big.Int, y2 *big.Int) (*big.Int, *big.Int, error) {
        var out []interface{}
        err := _Enygma.contract.Call(opts, &out, "addPedComm", p1, p2, x2, y2)

        if err != nil {
                return *new(*big.Int), *new(*big.Int), err
        }

        out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
        out1 := *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)

        return out0, out1, err

}

// AddPedComm is a free data retrieval call binding the contract method 0x132ce4d4.
//
// Solidity: function addPedComm(uint256 p1, uint256 p2, uint256 x2, uint256 y2) view returns(uint256, uint256)
func (_Enygma *EnygmaSession) AddPedComm(p1 *big.Int, p2 *big.Int, x2 *big.Int, y2 *big.Int) (*big.Int, *big.Int, error) {
        return _Enygma.Contract.AddPedComm(&_Enygma.CallOpts, p1, p2, x2, y2)
}

// AddPedComm is a free data retrieval call binding the contract method 0x132ce4d4.
//
// Solidity: function addPedComm(uint256 p1, uint256 p2, uint256 x2, uint256 y2) view returns(uint256, uint256)
func (_Enygma *EnygmaCallerSession) AddPedComm(p1 *big.Int, p2 *big.Int, x2 *big.Int, y2 *big.Int) (*big.Int, *big.Int, error) {
        return _Enygma.Contract.AddPedComm(&_Enygma.CallOpts, p1, p2, x2, y2)
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

// DepositVerifiers is a free data retrieval call binding the contract method 0x02e9b07b.
//
// Solidity: function depositVerifiers(uint256 ) view returns(address)
func (_Enygma *EnygmaCaller) DepositVerifiers(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
        var out []interface{}
        err := _Enygma.contract.Call(opts, &out, "depositVerifiers", arg0)

        if err != nil {
                return *new(common.Address), err
        }

        out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

        return out0, err

}

// DepositVerifiers is a free data retrieval call binding the contract method 0x02e9b07b.
//
// Solidity: function depositVerifiers(uint256 ) view returns(address)
func (_Enygma *EnygmaSession) DepositVerifiers(arg0 *big.Int) (common.Address, error) {
        return _Enygma.Contract.DepositVerifiers(&_Enygma.CallOpts, arg0)
}

// DepositVerifiers is a free data retrieval call binding the contract method 0x02e9b07b.
//
// Solidity: function depositVerifiers(uint256 ) view returns(address)
func (_Enygma *EnygmaCallerSession) DepositVerifiers(arg0 *big.Int) (common.Address, error) {
        return _Enygma.Contract.DepositVerifiers(&_Enygma.CallOpts, arg0)
}

// DerivePk is a free data retrieval call binding the contract method 0x723dbbc4.
//
// Solidity: function derivePk(uint256 v) view returns(uint256 x2, uint256 y2)
func (_Enygma *EnygmaCaller) DerivePk(opts *bind.CallOpts, v *big.Int) (struct {
        X2 *big.Int
        Y2 *big.Int
}, error) {
        var out []interface{}
        err := _Enygma.contract.Call(opts, &out, "derivePk", v)

        outstruct := new(struct {
                X2 *big.Int
                Y2 *big.Int
        })
        if err != nil {
                return *outstruct, err
        }

        outstruct.X2 = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
        outstruct.Y2 = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)

        return *outstruct, err

}

// DerivePk is a free data retrieval call binding the contract method 0x723dbbc4.
//
// Solidity: function derivePk(uint256 v) view returns(uint256 x2, uint256 y2)
func (_Enygma *EnygmaSession) DerivePk(v *big.Int) (struct {
        X2 *big.Int
        Y2 *big.Int
}, error) {
        return _Enygma.Contract.DerivePk(&_Enygma.CallOpts, v)
}

// DerivePk is a free data retrieval call binding the contract method 0x723dbbc4.
//
// Solidity: function derivePk(uint256 v) view returns(uint256 x2, uint256 y2)
func (_Enygma *EnygmaCallerSession) DerivePk(v *big.Int) (struct {
        X2 *big.Int
        Y2 *big.Int
}, error) {
        return _Enygma.Contract.DerivePk(&_Enygma.CallOpts, v)
}

// DerivePkH is a free data retrieval call binding the contract method 0xce630c18.
//
// Solidity: function derivePkH(uint256 r) view returns(uint256 x2, uint256 y2)
func (_Enygma *EnygmaCaller) DerivePkH(opts *bind.CallOpts, r *big.Int) (struct {
        X2 *big.Int
        Y2 *big.Int
}, error) {
        var out []interface{}
        err := _Enygma.contract.Call(opts, &out, "derivePkH", r)

        outstruct := new(struct {
                X2 *big.Int
                Y2 *big.Int
        })
        if err != nil {
                return *outstruct, err
        }

        outstruct.X2 = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
        outstruct.Y2 = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)

        return *outstruct, err

}

// DerivePkH is a free data retrieval call binding the contract method 0xce630c18.
//
// Solidity: function derivePkH(uint256 r) view returns(uint256 x2, uint256 y2)
func (_Enygma *EnygmaSession) DerivePkH(r *big.Int) (struct {
        X2 *big.Int
        Y2 *big.Int
}, error) {
        return _Enygma.Contract.DerivePkH(&_Enygma.CallOpts, r)
}

// DerivePkH is a free data retrieval call binding the contract method 0xce630c18.
//
// Solidity: function derivePkH(uint256 r) view returns(uint256 x2, uint256 y2)
func (_Enygma *EnygmaCallerSession) DerivePkH(r *big.Int) (struct {
        X2 *big.Int
        Y2 *big.Int
}, error) {
        return _Enygma.Contract.DerivePkH(&_Enygma.CallOpts, r)
}

// GetBalance is a free data retrieval call binding the contract method 0x1e010439.
//
// Solidity: function getBalance(uint256 account) view returns(uint256 x, uint256 y)
func (_Enygma *EnygmaCaller) GetBalance(opts *bind.CallOpts, account *big.Int) (struct {
        X *big.Int
        Y *big.Int
}, error) {
        var out []interface{}
        err := _Enygma.contract.Call(opts, &out, "getBalance", account)

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
// Solidity: function getBalance(uint256 account) view returns(uint256 x, uint256 y)
func (_Enygma *EnygmaSession) GetBalance(account *big.Int) (struct {
        X *big.Int
        Y *big.Int
}, error) {
        return _Enygma.Contract.GetBalance(&_Enygma.CallOpts, account)
}

// GetBalance is a free data retrieval call binding the contract method 0x1e010439.
//
// Solidity: function getBalance(uint256 account) view returns(uint256 x, uint256 y)
func (_Enygma *EnygmaCallerSession) GetBalance(account *big.Int) (struct {
        X *big.Int
        Y *big.Int
}, error) {
        return _Enygma.Contract.GetBalance(&_Enygma.CallOpts, account)
}

// GetPublicValues is a free data retrieval call binding the contract method 0xa9c58a7e.
//
// Solidity: function getPublicValues(uint256 size) view returns((uint256,uint256)[], (uint256,uint256)[])
func (_Enygma *EnygmaCaller) GetPublicValues(opts *bind.CallOpts, size *big.Int) ([]IEnygmaPoint, []IEnygmaPoint, error) {
        var out []interface{}
        err := _Enygma.contract.Call(opts, &out, "getPublicValues", size)

        if err != nil {
                return *new([]IEnygmaPoint), *new([]IEnygmaPoint), err
        }

        out0 := *abi.ConvertType(out[0], new([]IEnygmaPoint)).(*[]IEnygmaPoint)
        out1 := *abi.ConvertType(out[1], new([]IEnygmaPoint)).(*[]IEnygmaPoint)

        return out0, out1, err

}

// GetPublicValues is a free data retrieval call binding the contract method 0xa9c58a7e.
//
// Solidity: function getPublicValues(uint256 size) view returns((uint256,uint256)[], (uint256,uint256)[])
func (_Enygma *EnygmaSession) GetPublicValues(size *big.Int) ([]IEnygmaPoint, []IEnygmaPoint, error) {
        return _Enygma.Contract.GetPublicValues(&_Enygma.CallOpts, size)
}

// GetPublicValues is a free data retrieval call binding the contract method 0xa9c58a7e.
//
// Solidity: function getPublicValues(uint256 size) view returns((uint256,uint256)[], (uint256,uint256)[])
func (_Enygma *EnygmaCallerSession) GetPublicValues(size *big.Int) ([]IEnygmaPoint, []IEnygmaPoint, error) {
        return _Enygma.Contract.GetPublicValues(&_Enygma.CallOpts, size)
}

// LastblockNum is a free data retrieval call binding the contract method 0xa79d55e6.
//
// Solidity: function lastblockNum() view returns(uint256)
func (_Enygma *EnygmaCaller) LastblockNum(opts *bind.CallOpts) (*big.Int, error) {
        var out []interface{}
        err := _Enygma.contract.Call(opts, &out, "lastblockNum")

        if err != nil {
                return *new(*big.Int), err
        }

        out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

        return out0, err

}

// LastblockNum is a free data retrieval call binding the contract method 0xa79d55e6.
//
// Solidity: function lastblockNum() view returns(uint256)
func (_Enygma *EnygmaSession) LastblockNum() (*big.Int, error) {
        return _Enygma.Contract.LastblockNum(&_Enygma.CallOpts)
}

// LastblockNum is a free data retrieval call binding the contract method 0xa79d55e6.
//
// Solidity: function lastblockNum() view returns(uint256)
func (_Enygma *EnygmaCallerSession) LastblockNum() (*big.Int, error) {
        return _Enygma.Contract.LastblockNum(&_Enygma.CallOpts)
}

// PedCom is a free data retrieval call binding the contract method 0x7d894a16.
//
// Solidity: function pedCom(uint256 v, uint256 r) view returns(uint256, uint256)
func (_Enygma *EnygmaCaller) PedCom(opts *bind.CallOpts, v *big.Int, r *big.Int) (*big.Int, *big.Int, error) {
        var out []interface{}
        err := _Enygma.contract.Call(opts, &out, "pedCom", v, r)

        if err != nil {
                return *new(*big.Int), *new(*big.Int), err
        }

        out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
        out1 := *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)

        return out0, out1, err

}

// PedCom is a free data retrieval call binding the contract method 0x7d894a16.
//
// Solidity: function pedCom(uint256 v, uint256 r) view returns(uint256, uint256)
func (_Enygma *EnygmaSession) PedCom(v *big.Int, r *big.Int) (*big.Int, *big.Int, error) {
        return _Enygma.Contract.PedCom(&_Enygma.CallOpts, v, r)
}

// PedCom is a free data retrieval call binding the contract method 0x7d894a16.
//
// Solidity: function pedCom(uint256 v, uint256 r) view returns(uint256, uint256)
func (_Enygma *EnygmaCallerSession) PedCom(v *big.Int, r *big.Int) (*big.Int, *big.Int, error) {
        return _Enygma.Contract.PedCom(&_Enygma.CallOpts, v, r)
}

// PubKeys is a free data retrieval call binding the contract method 0xde6232d0.
//
// Solidity: function pubKeys(uint256 ) view returns(uint256 c1, uint256 c2)
func (_Enygma *EnygmaCaller) PubKeys(opts *bind.CallOpts, arg0 *big.Int) (struct {
        C1 *big.Int
        C2 *big.Int
}, error) {
        var out []interface{}
        err := _Enygma.contract.Call(opts, &out, "pubKeys", arg0)

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

// PubKeys is a free data retrieval call binding the contract method 0xde6232d0.
//
// Solidity: function pubKeys(uint256 ) view returns(uint256 c1, uint256 c2)
func (_Enygma *EnygmaSession) PubKeys(arg0 *big.Int) (struct {
        C1 *big.Int
        C2 *big.Int
}, error) {
        return _Enygma.Contract.PubKeys(&_Enygma.CallOpts, arg0)
}

// PubKeys is a free data retrieval call binding the contract method 0xde6232d0.
//
// Solidity: function pubKeys(uint256 ) view returns(uint256 c1, uint256 c2)
func (_Enygma *EnygmaCallerSession) PubKeys(arg0 *big.Int) (struct {
        C1 *big.Int
        C2 *big.Int
}, error) {
        return _Enygma.Contract.PubKeys(&_Enygma.CallOpts, arg0)
}

// ReferenceBalance is a free data retrieval call binding the contract method 0xa9bace48.
//
// Solidity: function referenceBalance(uint256 , uint256 ) view returns(uint256 c1, uint256 c2)
func (_Enygma *EnygmaCaller) ReferenceBalance(opts *bind.CallOpts, arg0 *big.Int, arg1 *big.Int) (struct {
        C1 *big.Int
        C2 *big.Int
}, error) {
        var out []interface{}
        err := _Enygma.contract.Call(opts, &out, "referenceBalance", arg0, arg1)

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

// ReferenceBalance is a free data retrieval call binding the contract method 0xa9bace48.
//
// Solidity: function referenceBalance(uint256 , uint256 ) view returns(uint256 c1, uint256 c2)
func (_Enygma *EnygmaSession) ReferenceBalance(arg0 *big.Int, arg1 *big.Int) (struct {
        C1 *big.Int
        C2 *big.Int
}, error) {
        return _Enygma.Contract.ReferenceBalance(&_Enygma.CallOpts, arg0, arg1)
}

// ReferenceBalance is a free data retrieval call binding the contract method 0xa9bace48.
//
// Solidity: function referenceBalance(uint256 , uint256 ) view returns(uint256 c1, uint256 c2)
func (_Enygma *EnygmaCallerSession) ReferenceBalance(arg0 *big.Int, arg1 *big.Int) (struct {
        C1 *big.Int
        C2 *big.Int
}, error) {
        return _Enygma.Contract.ReferenceBalance(&_Enygma.CallOpts, arg0, arg1)
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

// Verifiers is a free data retrieval call binding the contract method 0xac1eff68.
//
// Solidity: function verifiers(uint256 ) view returns(address)
func (_Enygma *EnygmaCaller) Verifiers(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
        var out []interface{}
        err := _Enygma.contract.Call(opts, &out, "verifiers", arg0)

        if err != nil {
                return *new(common.Address), err
        }

        out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

        return out0, err

}

// Verifiers is a free data retrieval call binding the contract method 0xac1eff68.
//
// Solidity: function verifiers(uint256 ) view returns(address)
func (_Enygma *EnygmaSession) Verifiers(arg0 *big.Int) (common.Address, error) {
        return _Enygma.Contract.Verifiers(&_Enygma.CallOpts, arg0)
}

// Verifiers is a free data retrieval call binding the contract method 0xac1eff68.
//
// Solidity: function verifiers(uint256 ) view returns(address)
func (_Enygma *EnygmaCallerSession) Verifiers(arg0 *big.Int) (common.Address, error) {
        return _Enygma.Contract.Verifiers(&_Enygma.CallOpts, arg0)
}

// WithdrawVerifiers is a free data retrieval call binding the contract method 0x15fd8d86.
//
// Solidity: function withdrawVerifiers(uint256 ) view returns(address)
func (_Enygma *EnygmaCaller) WithdrawVerifiers(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
        var out []interface{}
        err := _Enygma.contract.Call(opts, &out, "withdrawVerifiers", arg0)

        if err != nil {
                return *new(common.Address), err
        }

        out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

        return out0, err

}

// WithdrawVerifiers is a free data retrieval call binding the contract method 0x15fd8d86.
//
// Solidity: function withdrawVerifiers(uint256 ) view returns(address)
func (_Enygma *EnygmaSession) WithdrawVerifiers(arg0 *big.Int) (common.Address, error) {
        return _Enygma.Contract.WithdrawVerifiers(&_Enygma.CallOpts, arg0)
}

// WithdrawVerifiers is a free data retrieval call binding the contract method 0x15fd8d86.
//
// Solidity: function withdrawVerifiers(uint256 ) view returns(address)
func (_Enygma *EnygmaCallerSession) WithdrawVerifiers(arg0 *big.Int) (common.Address, error) {
        return _Enygma.Contract.WithdrawVerifiers(&_Enygma.CallOpts, arg0)
}

// Zkdvps is a free data retrieval call binding the contract method 0x46b6e952.
//
// Solidity: function zkdvps(uint256 ) view returns(address)
func (_Enygma *EnygmaCaller) Zkdvps(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
        var out []interface{}
        err := _Enygma.contract.Call(opts, &out, "zkdvps", arg0)

        if err != nil {
                return *new(common.Address), err
        }

        out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

        return out0, err

}

// Zkdvps is a free data retrieval call binding the contract method 0x46b6e952.
//
// Solidity: function zkdvps(uint256 ) view returns(address)
func (_Enygma *EnygmaSession) Zkdvps(arg0 *big.Int) (common.Address, error) {
        return _Enygma.Contract.Zkdvps(&_Enygma.CallOpts, arg0)
}

// Zkdvps is a free data retrieval call binding the contract method 0x46b6e952.
//
// Solidity: function zkdvps(uint256 ) view returns(address)
func (_Enygma *EnygmaCallerSession) Zkdvps(arg0 *big.Int) (common.Address, error) {
        return _Enygma.Contract.Zkdvps(&_Enygma.CallOpts, arg0)
}

// AddDepositVerifier is a paid mutator transaction binding the contract method 0x0197d942.
//
// Solidity: function addDepositVerifier(address depositVerifier) returns(bool)
func (_Enygma *EnygmaTransactor) AddDepositVerifier(opts *bind.TransactOpts, depositVerifier common.Address) (*types.Transaction, error) {
        return _Enygma.contract.Transact(opts, "addDepositVerifier", depositVerifier)
}

// AddDepositVerifier is a paid mutator transaction binding the contract method 0x0197d942.
//
// Solidity: function addDepositVerifier(address depositVerifier) returns(bool)
func (_Enygma *EnygmaSession) AddDepositVerifier(depositVerifier common.Address) (*types.Transaction, error) {
        return _Enygma.Contract.AddDepositVerifier(&_Enygma.TransactOpts, depositVerifier)
}

// AddDepositVerifier is a paid mutator transaction binding the contract method 0x0197d942.
//
// Solidity: function addDepositVerifier(address depositVerifier) returns(bool)
func (_Enygma *EnygmaTransactorSession) AddDepositVerifier(depositVerifier common.Address) (*types.Transaction, error) {
        return _Enygma.Contract.AddDepositVerifier(&_Enygma.TransactOpts, depositVerifier)
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

// AddWithdrawVerifier is a paid mutator transaction binding the contract method 0x67dcb6ae.
//
// Solidity: function addWithdrawVerifier(address withdrawVerifier) returns(bool)
func (_Enygma *EnygmaTransactor) AddWithdrawVerifier(opts *bind.TransactOpts, withdrawVerifier common.Address) (*types.Transaction, error) {
        return _Enygma.contract.Transact(opts, "addWithdrawVerifier", withdrawVerifier)
}

// AddWithdrawVerifier is a paid mutator transaction binding the contract method 0x67dcb6ae.
//
// Solidity: function addWithdrawVerifier(address withdrawVerifier) returns(bool)
func (_Enygma *EnygmaSession) AddWithdrawVerifier(withdrawVerifier common.Address) (*types.Transaction, error) {
        return _Enygma.Contract.AddWithdrawVerifier(&_Enygma.TransactOpts, withdrawVerifier)
}

// AddWithdrawVerifier is a paid mutator transaction binding the contract method 0x67dcb6ae.
//
// Solidity: function addWithdrawVerifier(address withdrawVerifier) returns(bool)
func (_Enygma *EnygmaTransactorSession) AddWithdrawVerifier(withdrawVerifier common.Address) (*types.Transaction, error) {
        return _Enygma.Contract.AddWithdrawVerifier(&_Enygma.TransactOpts, withdrawVerifier)
}

// AddZkDvp is a paid mutator transaction binding the contract method 0xf8344434.
//
// Solidity: function addZkDvp(address zkdvpAddress) returns(bool)
func (_Enygma *EnygmaTransactor) AddZkDvp(opts *bind.TransactOpts, zkdvpAddress common.Address) (*types.Transaction, error) {
        return _Enygma.contract.Transact(opts, "addZkDvp", zkdvpAddress)
}

// AddZkDvp is a paid mutator transaction binding the contract method 0xf8344434.
//
// Solidity: function addZkDvp(address zkdvpAddress) returns(bool)
func (_Enygma *EnygmaSession) AddZkDvp(zkdvpAddress common.Address) (*types.Transaction, error) {
        return _Enygma.Contract.AddZkDvp(&_Enygma.TransactOpts, zkdvpAddress)
}

// AddZkDvp is a paid mutator transaction binding the contract method 0xf8344434.
//
// Solidity: function addZkDvp(address zkdvpAddress) returns(bool)
func (_Enygma *EnygmaTransactorSession) AddZkDvp(zkdvpAddress common.Address) (*types.Transaction, error) {
        return _Enygma.Contract.AddZkDvp(&_Enygma.TransactOpts, zkdvpAddress)
}

// Burn is a paid mutator transaction binding the contract method 0xb390c0ab.
//
// Solidity: function burn(uint256 bankIndex, uint256 burnValue) returns(bool)
func (_Enygma *EnygmaTransactor) Burn(opts *bind.TransactOpts, bankIndex *big.Int, burnValue *big.Int) (*types.Transaction, error) {
        return _Enygma.contract.Transact(opts, "burn", bankIndex, burnValue)
}

// Burn is a paid mutator transaction binding the contract method 0xb390c0ab.
//
// Solidity: function burn(uint256 bankIndex, uint256 burnValue) returns(bool)
func (_Enygma *EnygmaSession) Burn(bankIndex *big.Int, burnValue *big.Int) (*types.Transaction, error) {
        return _Enygma.Contract.Burn(&_Enygma.TransactOpts, bankIndex, burnValue)
}

// Burn is a paid mutator transaction binding the contract method 0xb390c0ab.
//
// Solidity: function burn(uint256 bankIndex, uint256 burnValue) returns(bool)
func (_Enygma *EnygmaTransactorSession) Burn(bankIndex *big.Int, burnValue *big.Int) (*types.Transaction, error) {
        return _Enygma.Contract.Burn(&_Enygma.TransactOpts, bankIndex, burnValue)
}

// Deposit is a paid mutator transaction binding the contract method 0x907f7d55.
//
// Solidity: function deposit((uint256,uint256)[] commitments, (uint256[8],uint256[2]) proof, ((((uint256,uint256),(uint256[2],uint256[2]),(uint256,uint256)),uint256[],uint256,uint256)) withdrawParam, uint256[] kIndex) returns(bool)
func (_Enygma *EnygmaTransactor) Deposit(opts *bind.TransactOpts, commitments []IEnygmaPoint, proof IEnygmaDepositProof, withdrawParam IEnygmaWithdrawParams, kIndex []*big.Int) (*types.Transaction, error) {
        return _Enygma.contract.Transact(opts, "deposit", commitments, proof, withdrawParam, kIndex)
}

// Deposit is a paid mutator transaction binding the contract method 0x907f7d55.
//
// Solidity: function deposit((uint256,uint256)[] commitments, (uint256[8],uint256[2]) proof, ((((uint256,uint256),(uint256[2],uint256[2]),(uint256,uint256)),uint256[],uint256,uint256)) withdrawParam, uint256[] kIndex) returns(bool)
func (_Enygma *EnygmaSession) Deposit(commitments []IEnygmaPoint, proof IEnygmaDepositProof, withdrawParam IEnygmaWithdrawParams, kIndex []*big.Int) (*types.Transaction, error) {
        return _Enygma.Contract.Deposit(&_Enygma.TransactOpts, commitments, proof, withdrawParam, kIndex)
}

// Deposit is a paid mutator transaction binding the contract method 0x907f7d55.
//
// Solidity: function deposit((uint256,uint256)[] commitments, (uint256[8],uint256[2]) proof, ((((uint256,uint256),(uint256[2],uint256[2]),(uint256,uint256)),uint256[],uint256,uint256)) withdrawParam, uint256[] kIndex) returns(bool)
func (_Enygma *EnygmaTransactorSession) Deposit(commitments []IEnygmaPoint, proof IEnygmaDepositProof, withdrawParam IEnygmaWithdrawParams, kIndex []*big.Int) (*types.Transaction, error) {
        return _Enygma.Contract.Deposit(&_Enygma.TransactOpts, commitments, proof, withdrawParam, kIndex)
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
// Solidity: function mintSupply(uint256 amount, uint256 to) returns(bool)
func (_Enygma *EnygmaTransactor) MintSupply(opts *bind.TransactOpts, amount *big.Int, to *big.Int) (*types.Transaction, error) {
        return _Enygma.contract.Transact(opts, "mintSupply", amount, to)
}

// MintSupply is a paid mutator transaction binding the contract method 0xff98feae.
//
// Solidity: function mintSupply(uint256 amount, uint256 to) returns(bool)
func (_Enygma *EnygmaSession) MintSupply(amount *big.Int, to *big.Int) (*types.Transaction, error) {
        return _Enygma.Contract.MintSupply(&_Enygma.TransactOpts, amount, to)
}

// MintSupply is a paid mutator transaction binding the contract method 0xff98feae.
//
// Solidity: function mintSupply(uint256 amount, uint256 to) returns(bool)
func (_Enygma *EnygmaTransactorSession) MintSupply(amount *big.Int, to *big.Int) (*types.Transaction, error) {
        return _Enygma.Contract.MintSupply(&_Enygma.TransactOpts, amount, to)
}

// RegisterAccount is a paid mutator transaction binding the contract method 0x8b3b3168.
//
// Solidity: function registerAccount(address addr, uint256 accountNum, uint256 k1, uint256 k2, uint256 r) returns(bool)
func (_Enygma *EnygmaTransactor) RegisterAccount(opts *bind.TransactOpts, addr common.Address, accountNum *big.Int, k1 *big.Int, k2 *big.Int, r *big.Int) (*types.Transaction, error) {
        return _Enygma.contract.Transact(opts, "registerAccount", addr, accountNum, k1, k2, r)
}

// RegisterAccount is a paid mutator transaction binding the contract method 0x8b3b3168.
//
// Solidity: function registerAccount(address addr, uint256 accountNum, uint256 k1, uint256 k2, uint256 r) returns(bool)
func (_Enygma *EnygmaSession) RegisterAccount(addr common.Address, accountNum *big.Int, k1 *big.Int, k2 *big.Int, r *big.Int) (*types.Transaction, error) {
        return _Enygma.Contract.RegisterAccount(&_Enygma.TransactOpts, addr, accountNum, k1, k2, r)
}

// RegisterAccount is a paid mutator transaction binding the contract method 0x8b3b3168.
//
// Solidity: function registerAccount(address addr, uint256 accountNum, uint256 k1, uint256 k2, uint256 r) returns(bool)
func (_Enygma *EnygmaTransactorSession) RegisterAccount(addr common.Address, accountNum *big.Int, k1 *big.Int, k2 *big.Int, r *big.Int) (*types.Transaction, error) {
        return _Enygma.Contract.RegisterAccount(&_Enygma.TransactOpts, addr, accountNum, k1, k2, r)
}

// Transfer is a paid mutator transaction binding the contract method 0x34c4a97f.
//
// Solidity: function transfer((uint256,uint256)[] commitments, (uint256[8],uint256[31]) proof, uint256[] kIndex) returns(bool)
func (_Enygma *EnygmaTransactor) Transfer(opts *bind.TransactOpts, commitments []IEnygmaPoint, proof IEnygmaProof, kIndex []*big.Int) (*types.Transaction, error) {
        return _Enygma.contract.Transact(opts, "transfer", commitments, proof, kIndex)
}

// Transfer is a paid mutator transaction binding the contract method 0x34c4a97f.
//
// Solidity: function transfer((uint256,uint256)[] commitments, (uint256[8],uint256[31]) proof, uint256[] kIndex) returns(bool)
func (_Enygma *EnygmaSession) Transfer(commitments []IEnygmaPoint, proof IEnygmaProof, kIndex []*big.Int) (*types.Transaction, error) {
        return _Enygma.Contract.Transfer(&_Enygma.TransactOpts, commitments, proof, kIndex)
}

// Transfer is a paid mutator transaction binding the contract method 0x34c4a97f.
//
// Solidity: function transfer((uint256,uint256)[] commitments, (uint256[8],uint256[31]) proof, uint256[] kIndex) returns(bool)
func (_Enygma *EnygmaTransactorSession) Transfer(commitments []IEnygmaPoint, proof IEnygmaProof, kIndex []*big.Int) (*types.Transaction, error) {
        return _Enygma.Contract.Transfer(&_Enygma.TransactOpts, commitments, proof, kIndex)
}

// Withdraw is a paid mutator transaction binding the contract method 0x57e41327.
//
// Solidity: function withdraw((uint256,uint256)[] commitments, (uint256[8],uint256[1]) proof, (uint256,address,uint256)[] deposistParam, uint256[] kIndex) returns(bool, uint256[])
func (_Enygma *EnygmaTransactor) Withdraw(opts *bind.TransactOpts, commitments []IEnygmaPoint, proof IEnygmaWithdrawProof, deposistParam []IEnygmaDepositParams, kIndex []*big.Int) (*types.Transaction, error) {
        return _Enygma.contract.Transact(opts, "withdraw", commitments, proof, deposistParam, kIndex)
}

// Withdraw is a paid mutator transaction binding the contract method 0x57e41327.
//
// Solidity: function withdraw((uint256,uint256)[] commitments, (uint256[8],uint256[1]) proof, (uint256,address,uint256)[] deposistParam, uint256[] kIndex) returns(bool, uint256[])
func (_Enygma *EnygmaSession) Withdraw(commitments []IEnygmaPoint, proof IEnygmaWithdrawProof, deposistParam []IEnygmaDepositParams, kIndex []*big.Int) (*types.Transaction, error) {
        return _Enygma.Contract.Withdraw(&_Enygma.TransactOpts, commitments, proof, deposistParam, kIndex)
}

// Withdraw is a paid mutator transaction binding the contract method 0x57e41327.
//
// Solidity: function withdraw((uint256,uint256)[] commitments, (uint256[8],uint256[1]) proof, (uint256,address,uint256)[] deposistParam, uint256[] kIndex) returns(bool, uint256[])
func (_Enygma *EnygmaTransactorSession) Withdraw(commitments []IEnygmaPoint, proof IEnygmaWithdrawProof, deposistParam []IEnygmaDepositParams, kIndex []*big.Int) (*types.Transaction, error) {
        return _Enygma.Contract.Withdraw(&_Enygma.TransactOpts, commitments, proof, deposistParam, kIndex)
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