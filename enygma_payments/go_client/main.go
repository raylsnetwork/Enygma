package main

import (
	"fmt"
	"math/big"
	"os"
	"strconv"
	"github.com/iden3/go-iden3-crypto/poseidon"
	"encoding/json"
	"io/ioutil"
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
	// These hardcoded values must be stored somewhere and retrieved to generate details of the transaction
	// previousV, _ = new(big.Int).SetString("0", 10)
	// previousR, _ = new(big.Int).SetString("1868501308730589599347486446800155493731984071794351926997077137692500338410", 10)
	// blockHash, _ = new(big.Int).SetString("4129789127591820896172587", 10)
	
	k0 = big.NewInt(0) 
	k1 = big.NewInt(1)
	k2 = big.NewInt(2)
	k3 = big.NewInt(3)
	k4 = big.NewInt(4)
	k5 = big.NewInt(5)

	
	kIndex        = []*big.Int{k0, k1, k2,k3,k4,k5}

	s0 = big.NewInt(0) 
	s1 = big.NewInt(54142)
	s2 = big.NewInt(814712)
	s3 = big.NewInt(250912012)
	s4 = big.NewInt(12312512)
	s5 = big.NewInt(12312512)



	// s0 = big.NewInt(58912784912894) 
	// s1 = big.NewInt(0) 
	// s2 = big.NewInt(741274128)
	// s3 = big.NewInt(4817481284)
	// s4 = big.NewInt(74128491240)
	secrets = []*big.Int{
		s0,s1,s2,s3,s4,s5,
	}

	address          = readJsonFile()
	commitChainURL   = "http://127.0.0.1:8545"
	httpposturl      = "http://127.0.0.1:8080/proof/enygma"
	privateKeyString = "34d091c661db4c814d65c8ae9277b7055c0dde5a752ce5a3fdfd4ea11a8f7154"
)

func main() {
	readJsonFile()
	// Example code (The bank IDs represent its position in the array), there are 5 banks transacting here
	// Bank A has ID 1, Bank B is 2, Bank C is 3 and Bank D is 4

	// ** Reserve the position 0 to always be empty

	// We will transfer 60 to Bank B and 40 to Bank D, the sender of 100 is Bank A.

	// 1. Retrieve the from account stored values
	if len(os.Args) < 7 {
        fmt.Println("Usage: go run . <qtyBank> <value> <senderId> <sk> <previousV> <previousR> <blockHash>")
        return
    }
	qtyBanks,_ := strconv.Atoi(os.Args[1])
	value      := os.Args[2]
	senderId,_     := strconv.Atoi(os.Args[3])
	sk         	    := os.Args[4]
	previousVTemp  := os.Args[5]
	previousRTemp  := os.Args[6]
	blockHashTemp  := os.Args[7]
	// 2. Generate the nullifier that provides an identification of account without revealing the identity to prevent double spending
	// and other banks from accidently subtracting from another banks balance
	// Provides an account mechanism protection

	previousV, _ := new(big.Int).SetString(previousVTemp, 10)
	previousR, _ := new(big.Int).SetString(previousRTemp, 10)
	blockHash, _ := new(big.Int).SetString(blockHashTemp, 10)
	skBigInt, _ := new(big.Int).SetString(sk, 10)

	inputs := []*big.Int{skBigInt, blockHash}
	nullifier, _ := poseidon.Hash(inputs)
	

	// How much will be subtracted from the sending account
	v, _         := new(big.Int).SetString(value, 10)
	amountToSend := v

	// Create the Transaction values. Notice the positions of the values
	// 60 is Bank A in position 0 of the array
	// 40 is Bank B in position 1 of the array
	// -100 is Bank C in position 2 of the array
	// The remaining positions we add 0, as no balance change will happen

	v0 , _ := new(big.Int).SetString("2736030358979909402780800718157159386076813972158567259200215660948447372941", 10)
	txValues := []*big.Int{v0,big.NewInt(60), big.NewInt(40), big.NewInt(0), big.NewInt(0), big.NewInt(0) }
	//txValues := []*big.Int{big.NewInt(20),new(big.Int).Neg(amountToSend), big.NewInt(20),  big.NewInt(0),big.NewInt(0) }

	// The commitments are generated to send from one account to multiple accounts
	commit, txValue, txRandom, secrets := makeCommitment(qtyBanks,amountToSend,senderId, txValues,blockHash,kIndex)
	
	// The referenceBalance and Public Keys stored on the smart contract are queried and the proofs are generated based on them
	referenceBalance, publicKey := getDataFromSmartContract(qtyBanks)

	// Generate the proofs from the commitments
	proof := generateProof(qtyBanks, value,senderId,nullifier, blockHash, sk, publicKey, referenceBalance, commit, txValue, txRandom, secrets, previousV, previousR,kIndex)


	
	// Send the transaction to the ZkToken.sol Transfer function

	sendTransaction(proof, commit,kIndex, publicKey,referenceBalance,blockHash)

	

}
