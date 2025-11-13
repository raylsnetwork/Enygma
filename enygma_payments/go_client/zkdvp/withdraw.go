package main

import (
	"fmt"
	"log"
	"time"
	"context"
	"strconv"
	"math/big"
	"crypto/ecdsa"
	"encoding/json"

	utils "enygma/utils"
	enygma "enygma/contracts"
	interfacezkdvp "enygma/interfacezkdvp"
	
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iden3/go-iden3-crypto/poseidon"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
)
/*

Script for integration Engyma and ZkDVP integration

Withdraw from Enygma --> Deposit to ZkDvp

*/


func withdraw(senderId int, amount *big.Int, vSplit []*big.Int, kIndex []int, depositKey []*big.Int, secrets []*big.Int, block_number string) error {
	blockNumber, _ := new(big.Int).SetString(block_number, 10)

	// Generate commitments and randomness
	commitments, txRandom := utils.GenerateWithdrawCommitments(amount, senderId, blockNumber, kIndex, secrets)

	// Generate proof
	addressBigInt, _ := new(big.Int).SetString(utils.Address[2:], 16)
	

	var hashArray []*big.Int
	var pkArray []*big.Int
	for i:=0; i< len(vSplit); i++{

		inputs := []*big.Int{addressBigInt,vSplit[i]}
		poseidonHash, _ := poseidon.Hash(inputs)
		pk:= utils.GetPkZkDvP(depositKey[i])
		inputsWithPubKey := []*big.Int{poseidonHash, pk}
		hash, _ := poseidon.Hash(inputsWithPubKey)
		
		hashArray = append(hashArray, hash)
		pkArray = append(pkArray, pk)
	}
	
	var commFinal [][]string

	for _, commVal := range commitments {
		var commObj []string
		commObj = append(commObj, commVal.C1.String())
		commObj = append(commObj, commVal.C2.String())
		commFinal = append(commFinal, commObj)
	}

	var hashArrayString []string
	for _, hashVal := range hashArray{
		hashArrayString = append(hashArrayString, hashVal.String())
	}

	var valueArray []string
	for _, value_i := range vSplit{
		valueArray = append(valueArray,value_i.String())
	}

	var pkArrayString []string
	for _, pk_i := range pkArray{
		pkArrayString = append(pkArrayString,pk_i.String())
	}

	proofData := interfacezkdvp.SnarkWithdraw{

		SenderID:      strconv.FormatInt(int64(senderId), 10),
		Address: 	   addressBigInt.String(),
		HashArray:     hashArrayString,
		VArray:		   valueArray,
		V:             amount.String(),
		Pk:		       pkArrayString,
		TxCommit:      commFinal,
		TxRandom:      utils.ConvertBigIntsToStrings(txRandom),
	}

	url := utils.WithdrawProofURL +"/"+strconv.Itoa(len(vSplit))
	body, err := utils.PostJSON(url, proofData)
	if err != nil {
		return fmt.Errorf("failed to generate proof: %v", err)
	}
	
	var response interfacezkdvp.SnarkRespose
	if err = json.Unmarshal(body, &response); err != nil {
		fmt.Errorf("failed to parse response JSON: %v", err)
	}

	//Proof parsing
	var ProofToSend  [8]*big.Int
	for i := 0; i < len(response.Proof); i++ {
		ProofToSend[i] =  response.Proof[i]
	}	
	//Public signal parsing	
	var PublicSignalToSend  [1]*big.Int
	for i := 0; i < len(response.PublicSignal); i++ {
		PublicSignalToSend[i] =  response.PublicSignal[i]
	}

	// Prepare the proof with the dynamic number of public signals
	proof := enygma.IEnygmaWithdrawProof{
		Proof:ProofToSend,
		PublicSignal: PublicSignalToSend,
	}

	
	contractAddr := common.HexToAddress(utils.Address)
	client,err := ethclient.Dial(utils.CommitChainURL)
	if err != nil {
		fmt.Println("Error connecting to client", err)
		return fmt.Errorf("error connecting to client: %v", err) // Added error handling
	}
	privateKey, err := crypto.HexToECDSA(utils.PrivateKeyString)
	if err != nil {
		fmt.Println("Error private key")
		return fmt.Errorf("error with private key: %v", err) // Added error handling
	}
	publicKey := privateKey.Public()

	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("error casting public key to ECDSA")
		return fmt.Errorf("Error casting public key to ECDSA") // Added error handling
	}
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		log.Fatal(err)
		return fmt.Errorf("Error getting nonce: %v", err) // Added error handling
	}

	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		log.Fatal(err)
		return fmt.Errorf("Error suggesting gas price: %v", err) // Added error handling
	}
	// Rayls ChainId 149401
	// 
	auth, _ := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(1337))
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)         // In Wei
	auth.GasLimit = uint64(30000000) // In units
	auth.GasPrice = gasPrice

	instance, err := enygma.NewEnygma(contractAddr, client)
	if err != nil {
		log.Fatal(err)
		return fmt.Errorf("Error creating Enygma contract instance: %v", err) // Added error handling
	}

	// Create a context with a 20-second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel() // Always call cancel to release resources

	contractAddrW := common.HexToAddress(utils.Address)

	var depositArrayParams []enygma.IEnygmaDepositParams

	for i:=0 ; i< len(vSplit); i++{

		deposit := enygma.IEnygmaDepositParams{
			Amount :vSplit[i],
			Erc20Adress:contractAddrW,
			PublicKey:pkArray[i],
		}
		depositArrayParams = append(depositArrayParams, deposit)
	}

	kIndexBigInt  := make([]*big.Int, len(kIndex))
	for i, num := range kIndex {
		kIndexBigInt[i] = big.NewInt(int64(num)) // Convert each int to *big.Int
	}
	
	transfer, err := instance.Withdraw(auth, commitments, proof,depositArrayParams,kIndexBigInt )
	
	if err != nil {
		fmt.Println("Transfer error")
		fmt.Println(err)
		return fmt.Errorf("transfer error: %v", err) // Added error handling
	}

	// Wait for transaction to be mined
	_, err2 := bind.WaitMined(ctx, client, transfer)


	if err2 != nil {
		fmt.Println("Transfer error")
		fmt.Println(err)
		return fmt.Errorf("transfer error: %v", err) // Added error handling
	}


	
	for _, hash_i := range hashArray{
		utils.InsertLeaves(hash_i.String())
	}

	return nil

}

func hasDuplicates(slice []*big.Int) bool {
    for i := 0; i < len(slice); i++ {
        for j := i + 1; j < len(slice); j++ {
            if slice[i].Cmp(slice[j]) == 0 {
                return true
            }
        }
    }
    return false
}



func main() {
	senderId := 0
	amount := big.NewInt(50)
	kIndex := []int{0, 1, 2, 3, 4, 5}
	vSplit := []*big.Int{
		big.NewInt(25),
		big.NewInt(7),
		big.NewInt(4),
		big.NewInt(8),
		big.NewInt(1),
		big.NewInt(5),
	}
	
	secrets := []*big.Int{
		big.NewInt(1234567890),
		big.NewInt(1241251),
		big.NewInt(112233445566),
		big.NewInt(1234567890),
		big.NewInt(1234567890),
		big.NewInt(41241261412),
	}
	block_number:= "1294012084012789"

	depositKey := []*big.Int{
		big.NewInt(99),
		big.NewInt(98),
		big.NewInt(97),
		big.NewInt(96),
		big.NewInt(95),
		big.NewInt(94),
	}

	if err := withdraw(senderId, amount,vSplit, kIndex, depositKey,secrets,block_number); err != nil {
		fmt.Println("Withdraw error: ", err)
	}
}
