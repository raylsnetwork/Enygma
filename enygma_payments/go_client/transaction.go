package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"strconv"

	"net/http"
	enygma "enygma/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

var (
	// Generator point of Baby Jub jub curve
	gx, _ = new(big.Int).SetString("16540640123574156134436876038791482806971768689494387082833631921987005038935", 10)
	gy, _ = new(big.Int).SetString("20819045374670962167435360035096875258406992893633759881276124905556507972311", 10)
	G     = &babyjub.Point{X: gx, Y: gy}
	//Another Baby Jubjub point (randomnly generated)
	hx, _ = new(big.Int).SetString("10100005861917718053548237064487763771145251762383025193119768015180892676690", 10)
	hy, _ = new(big.Int).SetString("7512830269827713629724023825249861327768672768516116945507944076335453576011", 10)
	H     = &babyjub.Point{X: hx, Y: hy}

	//subgroup order of BabyJubJub Eliptic curve
	P, _  = new(big.Int).SetString("2736030358979909402780800718157159386076813972158567259200215660948447373041", 10)
	
)

type Response struct {
	Message        string     `json:"message"`
	Proof          []*big.Int   `json:"proof"`
	PublicSignal   []*big.Int   `json:"publicSignal"`
}
type ProofZk struct {
	PiA          [2]*big.Int
	PiB          [2][2]*big.Int
	PiC          [2]*big.Int
	PublicSignal [2]*big.Int
}

type Proof struct {
	
	ArrayHashSecret [][]string `json:"arrayHashSecret`
	PublicKeys      []string   `json:"publicKey"`
	PreviousCommit [][]string `json:"previousCommit"`
	BlockNumber     string     `json:"blockNumber"`
	K               []string   `json:"kIndex"`

	SenderId        string     `json:"senderId"`
	Secrets         [][]string `json:"secrets"`
	TagMessage		[]string   `json:"tagMessage"`
	Sk              string     `json:"sk"`
	PreviousV       string     `json:"previousV"`
	PreviousR       string     `json:"previousR"`
	TxCommit        [][]string `json:"txCommit"`
	TxValue         []string   `json:"txValue"`
	TxRandom        []string   `json:"txRandom"`
	V               string     `json:"v"`
	Nullifier       string     `json:"nullifier"`
	
}



func GetPK(v *big.Int) *babyjub.Point {
	rG := babyjub.NewPoint().Mul(v, G)
	return rG
}

func GetH(v *big.Int) *babyjub.Point {
	rG := babyjub.NewPoint().Mul(v, H)
	return rG
}

func pedersenCommitment(v *big.Int, r *big.Int) *babyjub.Point {

	vG := GetPK(v)
	vH := GetH(r)

	return addPks(vG, vH)
}

func addPks(pk1 *babyjub.Point, pk2 *babyjub.Point) *babyjub.Point {
	return babyjub.NewPoint().Projective().Add(pk1.Projective(), pk2.Projective()).Affine()
}

func getNegative(x *big.Int) *big.Int {

	inverse := big.NewInt(0).Sub(P, x)

	return inverse
}

func getRValues( senderId int, s [][]*big.Int, block_hash *big.Int, k_index []*big.Int) []*big.Int {
	var rValues []*big.Int
	randomInt:= big.NewInt(21)
	HashRandom,_:= poseidon.Hash([]*big.Int{randomInt})

	for i := 0; i < len(s[senderId]); i++ {
		if containsBigInt(k_index, i){

			inputs := []*big.Int{ HashRandom,s[senderId][i],block_hash}
			block_hash.Mod(block_hash, P)
			PoseidonHash, _ := poseidon.Hash(inputs)
			PoseidonHash.Mod(PoseidonHash, P)
			rValues = append(rValues, PoseidonHash)}

	}
	return rValues
}


func hashArrayGen(senderId int, s [][]*big.Int, block_hash *big.Int, k_index []*big.Int) [][]*big.Int {
    // Initialize 2D slice with proper dimensions
    hashArray := make([][]*big.Int, len(k_index))
    
    for i := 0; i < len(k_index); i++ {
        // Initialize inner slice
        hashArray[i] = make([]*big.Int, len(k_index))
        
        for j := 0; j < len(k_index); j++ {

            inputs := []*big.Int{s[i][j], s[i][j]}
            poseidonHash, err := poseidon.Hash(inputs)
            if err != nil {
                panic(err) // or return error
            }
            
            // Reduce modulo P
            poseidonHash.Mod(poseidonHash, P)
            hashArray[i][j] = poseidonHash
        }
    }
    
    return hashArray
}

func tagMessageGen(senderId int, s [][]*big.Int, block_hash *big.Int, k_index []*big.Int, previousV *big.Int)[]*big.Int {
	var tagMesssage []*big.Int
	tag:= big.NewInt(12)
	HashTag,_:= poseidon.Hash([]*big.Int{tag})
	block_hash.Mod(block_hash, P)
	for i := 0; i < len(s[senderId]); i++ {
		if containsBigInt(k_index, i){

			
			inputs := []*big.Int{ HashTag,s[senderId][i],block_hash}
			PoseidonHash, _ := poseidon.Hash(inputs)
			PoseidonHash.Mod(PoseidonHash, P)
			tagMesssage = append(tagMesssage, PoseidonHash)}

	}
	inputSender:=[]*big.Int{ HashTag,previousV,block_hash}
	PoseidonHashSender, _ := poseidon.Hash(inputSender)
	PoseidonHashSender.Mod(PoseidonHashSender, P)
	tagMesssage[senderId] = PoseidonHashSender

	return tagMesssage
}

func getRSum(senderId int,s [][]*big.Int,  block_hash *big.Int,k_index []*big.Int) *big.Int {
	sum := big.NewInt(0)
	randomInt:= big.NewInt(21)
	HashRandom,_:= poseidon.Hash([]*big.Int{randomInt})

	for i := 0; i < len(s[senderId]); i++ {
		if senderId != i {
			if containsBigInt(k_index, i){
			inputs := []*big.Int{HashRandom,s[senderId][i],block_hash }
			PoseidonHash, _ := poseidon.Hash(inputs)
			PoseidonHash.Mod(PoseidonHash, P)
			sum.Add(sum, PoseidonHash)
			sum.Mod(sum, P)}

		}
	}
	return sum
}

func containsBigInt( s []*big.Int, val int) bool {
	valBig := big.NewInt(int64(val))
	for _, v := range s {
		if v.Cmp(valBig) == 0 {
			return true
		}
	}
	return false
}


func makeCommitment(qtyBanks int,v *big.Int,senderId int,txValues []*big.Int, blockHash *big.Int, kIndex []*big.Int) ( []enygma.IEnygmaPoint, []*big.Int, []*big.Int, [][]*big.Int) {
	
	
	txRandom := getRValues(senderId,secrets, blockHash, kIndex)
	rSum := getRSum(senderId,secrets, blockHash,kIndex)
	txRandom[senderId] = rSum
	var txCommit []*babyjub.Point

	for i := 0; i < len(kIndex); i++ {
		if i == senderId {
			txCommit = append(txCommit, pedersenCommitment(getNegative(v), txRandom[i]))
		} else {
			txCommit = append(txCommit, pedersenCommitment(txValues[i], getNegative(txRandom[i])))
		}
	}

	commitments := make([]enygma.IEnygmaPoint, len(kIndex))

	for i := 0; i < len(kIndex); i++ {
		commit := enygma.IEnygmaPoint{C1: txCommit[i].X, C2: txCommit[i].Y}
		commitments[i] = commit
	}

	return commitments, txValues, txRandom, secrets
}

func generateProof( qtyBanks int, value string,senderId int,nullifier *big.Int, 
				blockHash *big.Int, sk string, publicKey []*big.Int, 
				previousCommit []enygma.IEnygmaPoint, txCommit []enygma.IEnygmaPoint, 
				txValue []*big.Int, txRandom []*big.Int, secrets [][]*big.Int, previousV *big.Int,
				previousR *big.Int, k_index []*big.Int, hashArray [][]*big.Int, tagMessage []*big.Int) Response {
	

	var pkFinal []string
	var refBalFinal [][]string
	var commFinal [][]string

	for _, pkVal := range publicKey {
		
		pkFinal = append(pkFinal, pkVal.String())
	}

	for _, value := range previousCommit {
		var refBalFinal1 []string

		refBalFinal1 = append(refBalFinal1, value.C1.String())
		refBalFinal1 = append(refBalFinal1, value.C2.String())

		refBalFinal = append(refBalFinal, refBalFinal1)
	}

	for _, commVal := range txCommit {
		var commObj []string

		commObj = append(commObj, commVal.C1.String())
		commObj = append(commObj, commVal.C2.String())

		commFinal = append(commFinal, commObj)
	}

	txValueString := convertBigIntsToStrings(txValue)
	txRandomString := convertBigIntsToStrings(txRandom)
	secretsString := convertBigIntsToStrings2D(secrets)
	kIndexString := convertBigIntsToStrings(kIndex)
	tagMessageString:=convertBigIntsToStrings(tagMessage)
	hashArrayString:= convertBigIntsToStrings2D(hashArray)



	jsonInfo := Proof{
		hashArrayString,//ArrayHashSecret		
		pkFinal, // publicKey - change
		refBalFinal, // previous Commit
		blockHash.String(), //block number
		kIndexString, //KIndex
		strconv.FormatInt(int64(senderId), 10), //senderId
		secretsString, //secret
		tagMessageString, 
		sk,
		previousV.String(),
		previousR.String(),
		commFinal, //tx Commit
		txValueString,
		txRandomString,
		value,
		nullifier.String(),
		
	}

	fmt.Println("txRandomString",txRandomString)
	

	jsonMar, _ := json.Marshal(jsonInfo)
	var jsonData = []byte(jsonMar)

	request, error := http.NewRequest("POST", httpposturl, bytes.NewBuffer(jsonData))
	if error != nil {
		panic(error)
	}
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")

	clientPost := &http.Client{}
	response, error := clientPost.Do(request)
	if error != nil {
		panic(error)
	}
	defer response.Body.Close()

	
	body, _ := ioutil.ReadAll(response.Body)

	var result Response
	
	e := json.Unmarshal(body, &result)
	// panic on error
	if e != nil {
		panic(e)
	}

	return result

}

func convertBigIntsToStrings(bigInts []*big.Int) []string {
	strings := make([]string, len(bigInts)) // Create a slice of strings with the same length as bigInts.
	for i, bigInt := range bigInts {
		strings[i] = bigInt.String() // Convert each *big.Int to a string using the String() method.
	}
	return strings
}

func convertBigIntsToStrings2D(bigInts [][]*big.Int) [][]string {
    // Create outer slice
    strings := make([][]string, len(bigInts))
    
    for i, row := range bigInts {
        strings[i] = make([]string, len(row))
        for j, bigInt := range row {
            strings[i][j] = bigInt.String()
        }
    }
    
    return strings
}

func getDataFromSmartContract(qtyBanks int) ([]enygma.IEnygmaPoint, []*big.Int) {
	contractAddr := common.HexToAddress(address)

	client, err := ethclient.Dial(commitChainURL)
	if err != nil {
		fmt.Println(err)
	}

	instance, err := enygma.NewEnygma(contractAddr, client)
	if err != nil {
		log.Fatal(err)
	}
	size := big.NewInt(int64(qtyBanks))
	referenceBalance, publicKeys, err := instance.GetPublicValues(&bind.CallOpts{}, size)
	if err != nil {
		log.Fatal(err)
	}
	
	return referenceBalance, publicKeys
}

func sendTransaction(resp Response, commitments []enygma.IEnygmaPoint, kIndex []*big.Int,publicKey []*big.Int,previousCommit []enygma.IEnygmaPoint,blockHash *big.Int) {
	
	contractAddr := common.HexToAddress(address)
	client, err := ethclient.Dial(commitChainURL)
	if err != nil {
		fmt.Println("Error connecting to client", err)
	}
	privateKey, err := crypto.HexToECDSA(privateKeyString)
	if err != nil {
		fmt.Println("Error private key")
	}
	publicKeyAuth := privateKey.Public()

	publicKeyECDSA, ok := publicKeyAuth.(*ecdsa.PublicKey)
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

	auth, _ := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(1337))
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)
	auth.GasLimit = uint64(400000000)
	auth.GasPrice = gasPrice

	instance, err := enygma.NewEnygma(contractAddr, client)
	if err != nil {
		log.Fatal(err)
	}
	

	//Proof parsing
	var ProofToSend  [8]*big.Int
	for i := 0; i < len(resp.Proof); i++ {
		ProofToSend[i] =  resp.Proof[i]
	}
	//Public signal parsing	
	var PublicSignalToSend  [62]*big.Int
	for i := 0; i < len(resp.PublicSignal); i++ {
		PublicSignalToSend[i] =  resp.PublicSignal[i]

	}
	
	proof:= enygma.IEnygmaProof{
		Proof:ProofToSend,
		PublicSignal:PublicSignalToSend,
	}
	fmt.Println("length", len(PublicSignalToSend))

	transfer, err := instance.Transfer(auth,commitments, proof,kIndex)
	// transfer, err := instance.Accounts(&bind.CallOpts{}, fromAddress)
	if err != nil {
		fmt.Println("err")
		fmt.Println(err)
	}
	for _, point := range commitments {
		fmt.Printf("[%s,%s] ", point.C1, point.C2)
	}


	ctx := context.Background()

	receipt, err := bind.WaitMined(ctx, client, transfer)
	if err != nil {
		log.Fatalf("Tx failed: %v", err)
	}
	if receipt.Status == 1 {
		log.Println("Transfer was successful")
	} else {
		log.Println("Transfer failed")
	}

}


func contains(arr []int, num int) bool {
    for _, v := range arr {
        if v == num {
            return true
        }
    }
    return false
}