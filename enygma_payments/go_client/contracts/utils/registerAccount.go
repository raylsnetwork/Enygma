package main

import (

	"os"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"

	zktoken "zk_circom/contracts"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type Address struct {
    Address string `json:"address"`
}

func readJsonFile()string{
	jsonFile, _ := os.Open("./address.json")
    

    // Read the file into a byte slice
    byteValue, _ := ioutil.ReadAll(jsonFile)
    

    // Define a variable of type Address to hold the unmarshalled data
    var address Address

    json.Unmarshal(byteValue, &address)
	
	return address.Address
}
var (
	privateKeyString = "34d091c661db4c814d65c8ae9277b7055c0dde5a752ce5a3fdfd4ea11a8f7154"
	commitChainURL ="http://commitchain-dev.parfin.corp:8545"
)

func registerBank() {
	
	

	address:=readJsonFile()
	contractAddr := common.HexToAddress(address)

	client, err := ethclient.Dial(commitChainURL)
	if err != nil {
		fmt.Println("Error connecting to client", err)
	}
	privateKey, err := crypto.HexToECDSA(privateKeyString)
	if err != nil {
		fmt.Println("Error private key")
	}
	publicKey := privateKey.Public()

	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("error casting public key to ECDSA")
	}
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		log.Fatal(err)
	}

	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	auth, _ := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(149401))
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)
	auth.GasLimit = uint64(1000000000)
	auth.GasPrice = gasPrice

	instance, err := zktoken.NewZktoken(contractAddr, client)
	if err != nil {
		log.Fatal(err)
	}

	
	addressUser := common.HexToAddress("0xE41d6D443B7998ffF234c4328Ae61F273DDa0c39")
	accountNum1,_:=new(big.Int).SetString("016132353802239767858145943504008947476432404911738104163298818256277804241484", 10)
	accountNum2,_:=new(big.Int).SetString("6628477271988031251695526275163398093384206135519811020907741221006371008986", 10)
	randomFactor,_:=new(big.Int).SetString("0", 10)
	accountNumber,_:=new(big.Int).SetString("5", 10)

	register, err := instance.RegisterAccount(auth, addressUser,accountNumber,accountNum1,accountNum2,randomFactor)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(register)
	return;
}

func main() {

	

	registerBank()

}