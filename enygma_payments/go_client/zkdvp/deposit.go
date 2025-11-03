package main

import (
	"fmt"
	"log"
	"time"
	"strconv"
	"context"
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
func GenerateProofAtZkDvP(pk *big.Int, amount *big.Int, withdrawKey interfacezkdvp.Key, enygmaAddress *big.Int, merkleTree interfacezkdvp.MerkleTree) (interfacezkdvp.ZkDvpProofToWithdrawResponse, error) {
	
	jsonInfo := interfacezkdvp.ZkDvpProofToWithdrawRequest{
		PK:            pk.String(),
		Amount:        amount.String(),
		WithdrawKey:   withdrawKey,
		EnygmaAddress: enygmaAddress.String(),
		MerkleTree:    merkleTree,
	}
	fmt.Println("jsoninfo", jsonInfo)
	response, err := utils.PostJSON(utils.ZkdvpGetProof, jsonInfo)
		
	if err != nil {
		return interfacezkdvp.ZkDvpProofToWithdrawResponse{}, fmt.Errorf("failed to generate ZkDvP proof: %v", err)
	}

	var result interfacezkdvp.ZkDvpProofToWithdrawResponse
	if err = json.Unmarshal(response, &result); err != nil {
		fmt.Println("err", err)
		return interfacezkdvp.ZkDvpProofToWithdrawResponse{}, fmt.Errorf("failed to parse ZkDvP proof JSON: %v", err)
	}
	
	

	return result, nil
}

func getMerkleTreeZkDvp() (interfacezkdvp.MerkleTreeResponse, error) {
	response, err := utils.GetJSON(utils.ZkdvpGetMerkleTree)
	if err != nil {
		return interfacezkdvp.MerkleTreeResponse{}, fmt.Errorf("failed to retrieve Merkle tree: %v", err)
	}

	var result interfacezkdvp.MerkleTreeResponse
	if err = json.Unmarshal(response, &result); err != nil {
		return interfacezkdvp.MerkleTreeResponse{}, fmt.Errorf("failed to parse Merkle tree JSON: %v", err)
	}
	
	return result, nil
	
}

func InsertLeveMerkleTreeZkDvp(commitment *big.Int) (interfacezkdvp.InsertLeafResponse, error) {

	jsonInfo := interfacezkdvp.InsertLeaf{
		Commitment: commitment.String(),
	}
	
	response, err := utils.PostJSON(utils.ZkdvpAddLeaveURL,jsonInfo)
	if err != nil {
		return interfacezkdvp.InsertLeafResponse{}, fmt.Errorf("failed to retrieve Merkle tree: %v", err)
	}

	var result interfacezkdvp.InsertLeafResponse
	if err = json.Unmarshal(response, &result); err != nil {
		return interfacezkdvp.InsertLeafResponse{}, fmt.Errorf("failed to parse Merkle tree JSON: %v", err)
	}
	
	return result, nil
	
}


func ProccessGenerateProofAtZkDvp( amount *big.Int,amountToDeposit *big.Int ,pk *big.Int,secretDeposit string , enygmaAddress *big.Int )(interfacezkdvp.ZkDvpProofToWithdrawResponse,error){
	withdrawSecretKey,_ := new(big.Int).SetString(secretDeposit,10)
	withdrawPublicKey :=utils.GetPkZkDvP(withdrawSecretKey)

	var withdrawKey = interfacezkdvp.Key{
		PublicKey:  withdrawPublicKey.String(),
    	PrivateKey: withdrawSecretKey.String(),
	}

	
	merkleTree,_:= getMerkleTreeZkDvp()
	
	tx,errorM := GenerateProofAtZkDvP(pk,amountToDeposit,withdrawKey,enygmaAddress,merkleTree.MerkleTree)
	
	
	return tx,errorM
}

func typeConversionForZkDvpProof( txResponse  interfacezkdvp.ZkDvpProofToWithdrawResponse) (enygma.IZkDvpJoinSplitTransaction){

	// Proof construction

	proofResponse:= txResponse.ProofWithdraw
	
	PiA0, _ := new(big.Int).SetString(proofResponse.Proof.A[0], 10)
	PiA1, _ := new(big.Int).SetString(proofResponse.Proof.A[1], 10)

	PiB00, _ := new(big.Int).SetString(proofResponse.Proof.B[0][0], 10)
	PiB01, _ := new(big.Int).SetString(proofResponse.Proof.B[0][1], 10)
	PiB10, _ := new(big.Int).SetString(proofResponse.Proof.B[1][0], 10)
	PiB11, _ := new(big.Int).SetString(proofResponse.Proof.B[1][1], 10)

	PiC0, _ := new(big.Int).SetString(proofResponse.Proof.C[0], 10)
	PiC1, _ := new(big.Int).SetString(proofResponse.Proof.C[1], 10)

	proof := enygma.IZkDvpSnarkProof{
		A:          enygma.IZkDvpG1Point{X:PiA0, Y:PiA1},
		B:          enygma.IZkDvpG2Point{X:[2]*big.Int{PiB00, PiB01}, Y:[2]*big.Int{PiB10, PiB11}},
		C:          enygma.IZkDvpG1Point{X:PiC0, Y:PiC1},
	}

	NumberOfInputs:= big.NewInt(int64(txResponse.ProofWithdraw.NumberOfInputs))
	NumberOfOutputs:=  big.NewInt(int64(txResponse.ProofWithdraw.NumberOfOutputs))
	Statement:= utils.ConvertStringToBigInt(txResponse.ProofWithdraw.Statement)
	
	var transactionConverted= enygma.IZkDvpJoinSplitTransaction{
		Proof: proof,		
		Statement: Statement,
		NumberOfInputs:NumberOfInputs,
		NumberOfOutputs:NumberOfOutputs,
	}
		
	return transactionConverted
}


func deposit(senderId int, amount *big.Int, amountToDeposit *big.Int ,kIndex []int, secrets []*big.Int,depositSecret string, block_number string) error {
	blockNumber, _ := new(big.Int).SetString(block_number, 10)

	// Generate commitments and randomness
	commitments, txRandom := utils.GenerateDepositCommitments(amountToDeposit, senderId, blockNumber, kIndex, secrets)

	addressBigInt, _ := new(big.Int).SetString(utils.Address[2:], 16)
	depositSecretBigInt, _ := new(big.Int).SetString(depositSecret,10)
	pk:= utils.GetPkZkDvP(depositSecretBigInt)

	// Generate proof
	
	inputs := []*big.Int{addressBigInt,amountToDeposit }
	poseidonHash, _ := poseidon.Hash(inputs)
	inputsWithPubKey := []*big.Int{poseidonHash, pk}
	hash, _ := poseidon.Hash(inputsWithPubKey)

	var commFinal [][]string
	for _, commVal := range commitments {
		var commObj []string
		commObj = append(commObj, commVal.C1.String())
		commObj = append(commObj, commVal.C2.String())
		commFinal = append(commFinal, commObj)
	}

	
	proofData := struct {
		SenderID      string     `json:"senderId"`
		Address       string     `json:"address"`
		Hash     	  string     `json:"hash"`
		VInit         string   	 `json:"vInit"`		
		VDeposit      string     `json:"vDeposit"`
		Secret        string     `json:"secret"`
		Pk			  string     `json:"pk"`	
		Commitments   [][]string `json:"txCommit"`
		TxRandom      []string   `json:"txRandom"`

	}{
		
		SenderID:      strconv.FormatInt(int64(senderId), 10),
		Address:       addressBigInt.String(),
		Hash:     	   hash.String(),
		VInit:         amount.String(),
		VDeposit:      amountToDeposit.String(),
		Secret:        depositSecretBigInt.String(),  
		Pk:            pk.String(),
		Commitments:   commFinal,
		TxRandom:      utils.ConvertBigIntsToStrings(txRandom),	
	}

	body, err := utils.PostJSON(utils.DepositProofURL, proofData)
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
	var PublicSignalToSend  [2]*big.Int
	for i := 0; i < len(response.PublicSignal); i++ {
		PublicSignalToSend[i] =  response.PublicSignal[i]

	}

	proof := enygma.IEnygmaDepositProof{
		Proof:ProofToSend,
		PublicSignal: PublicSignalToSend,
	}

	contractAddr := common.HexToAddress(utils.Address)
	client, err := ethclient.Dial(utils.CommitChainURL)
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
	auth.GasLimit = uint64(300000000) // In units
	auth.GasPrice = gasPrice

	instance, err := enygma.NewEnygma(contractAddr, client)
	if err != nil {
		log.Fatal(err)
		return fmt.Errorf("Error creating Enygma contract instance: %v", err) // Added error handling
	}

	// Create a context with a 20-second timeout
	ctx, _ := context.WithTimeout(context.Background(), 20*time.Second)

	kIndexBigInt  := make([]*big.Int, len(kIndex))
	for i, num := range kIndex {
		kIndexBigInt[i] = big.NewInt(int64(num)) // Convert each int to *big.Int
	}

	transaction,_ := ProccessGenerateProofAtZkDvp(amount,amountToDeposit,pk,depositSecret,addressBigInt)
	
	transactionFormat := typeConversionForZkDvpProof(transaction)

	
	withdraw := enygma.IEnygmaWithdrawParams{ // Change struct
		// Amount :amount,
		// Erc20Adress:contractAddr,
		// PublicKey: pkString,
		Transaction: transactionFormat,
	}

	transfer, err := instance.Deposit(auth, commitments, proof,withdraw,kIndexBigInt )
	if err != nil {
		fmt.Println("Transfer error")
		fmt.Println(err)
		return fmt.Errorf("transfer error: %v", err) // Added error handling
	}

	_, err2 := bind.WaitMined(ctx, client, transfer)

	if err2 != nil {
		fmt.Println("Transfer error")
		fmt.Println(err)
		return fmt.Errorf("transfer error: %v", err) // Added error handling
	}
	fmt.Println("Transfer", transfer)
	
	fmt.Printf("Deposit Successfully")

	return nil

	}


func main() {
	senderId := 0
	originalAmount := big.NewInt(50)
	amountToDeposit:= big.NewInt(5)
	kIndex := []int{0, 1, 2, 3, 4, 5}
	secrets := []*big.Int{
		big.NewInt(1234567890),
		big.NewInt(9876543210),
		big.NewInt(112233445566),
		big.NewInt(1234567890),
		big.NewInt(1234567890),
		big.NewInt(41241261412),
	}
	block_number:= "1294712897901284091"
	
	depositSecret := "94"

	
	if err := deposit(senderId, originalAmount, amountToDeposit,kIndex, secrets,depositSecret,block_number); err != nil {
		fmt.Println("Deposit error: ", err)
	}
}
