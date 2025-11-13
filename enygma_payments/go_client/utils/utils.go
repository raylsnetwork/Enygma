package utils

import (
	"os"
	"fmt"
	"bytes"
	"strings"
	"math/big"
	"net/http"
	"io/ioutil"
	"encoding/json"

	enygma "enygma/contracts"
	
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/iden3/go-iden3-crypto/poseidon"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	
)

// Configuration and Constants
var (
	CommitChainURL     = "http://127.0.0.1:8545"
	HttpPostURL        = "http://127.0.0.1:3000/generateProof"
	ZkdvpAddLeaveURL   = "http://127.0.0.1:3001/insertMerkleTree"
	ZkdvpGetMerkleTree = "http://127.0.0.1:3001/getMerkleTree"
	ZkdvpGetProof      = "http://127.0.0.1:3001/generateJoinEnygmaProof"
	WithdrawProofURL   = "http://127.0.0.1:8080/proof/withdraw"
	DepositProofURL   = "http://127.0.0.1:8080/proof/deposit"
	PrivateKeyString   = "" // Please insert private key of your walllet
	Address            = readJSONFile()
	P, _               = new(big.Int).SetString("2736030358979909402780800718157159386076813972158567259200215660948447373041", 10)
	G                  = initPoint("16540640123574156134436876038791482806971768689494387082833631921987005038935", "20819045374670962167435360035096875258406992893633759881276124905556507972311")
	H                  = initPoint("10100005861917718053548237064487763771145251762383025193119768015180892676690", "7512830269827713629724023825249861327768672768516116945507944076335453576011")
)




// Utility Functions
func initPoint(xStr, yStr string) *babyjub.Point {
	x, _ := new(big.Int).SetString(xStr, 10)
	y, _ := new(big.Int).SetString(yStr, 10)
	return &babyjub.Point{X: x, Y: y}
}

func readJSONFile() string {
	jsonFile, _ := os.Open("./address.json")
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	var address struct {
		Address string `json:"address"`
	}
	json.Unmarshal(byteValue, &address)
	return address.Address
}

func pedersenCommitment(v, r *big.Int) *babyjub.Point {
	vG := babyjub.NewPoint().Mul(v, G)
	rH := babyjub.NewPoint().Mul(r, H)
	return babyjub.NewPoint().Projective().Add(vG.Projective(), rH.Projective()).Affine()
}

func getNegative(x *big.Int) *big.Int {
	if x.Cmp(big.NewInt(0)) == 0 {
		return big.NewInt(0)
	}
	return new(big.Int).Sub(P, x)
}

func ConvertBigIntsToStrings(bigInts []*big.Int) []string {
	strs := make([]string, len(bigInts))
	for i, bi := range bigInts {
		strs[i] = bi.String()
	}
	return strs
}

func PostJSON(url string, payload interface{}) ([]byte, error) {
	jsonData, _ := json.Marshal(payload)
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return nil, fmt.Errorf("failed to connect to %s: connection refused", url)
		}
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return  nil,fmt.Errorf("failed to read response body: %v", err)
	}

	return body,nil
}

func GetJSON(url string) ([]byte, error) {
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating GET request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return nil, fmt.Errorf("failed to connect to %s: connection refused", url)
		}
		return nil, fmt.Errorf("error making GET request: %v", err)
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	return body, nil
}

func GenerateWithdrawCommitments(v *big.Int, senderId int, blockNumber *big.Int, kIndex []int, secrets []*big.Int) ([]enygma.IEnygmaPoint, []*big.Int) {
	txRandom := getTxRandomAndRValues(secrets, blockNumber, senderId, kIndex)
	var commitments []*babyjub.Point

	mappingIndex := make(map[int]int)
	for i, val := range kIndex {
		mappingIndex[i] = val
	}

	for i, _ := range kIndex {
		var c *babyjub.Point
		if mappingIndex[i] == senderId {
			c = pedersenCommitment(getNegative(v), txRandom[i])
		} else {
			c = pedersenCommitment(big.NewInt(0), txRandom[i])
		}
		commitments = append(commitments, c)
	}
	commitmentsEnygma := make([]enygma.IEnygmaPoint, 6)

	for i := 0; i < len(kIndex); i++ {
		commit := enygma.IEnygmaPoint{C1: commitments[i].X, C2: commitments[i].Y}
		commitmentsEnygma[i] = commit
	}

	return commitmentsEnygma, txRandom
}


func GenerateDepositCommitments(v *big.Int, senderId int, blockNumber *big.Int, kIndex []int, secrets []*big.Int) ([]enygma.IEnygmaPoint, []*big.Int) {
	txRandom := getTxRandomAndRValues(secrets, blockNumber, senderId, kIndex)
	var commitments []*babyjub.Point

	mappingIndex := make(map[int]int)
	for i, val := range kIndex {
		mappingIndex[i] = val
	}

	for i, _ := range kIndex {
		var c *babyjub.Point
		if mappingIndex[i] == senderId {
			c = pedersenCommitment(v, txRandom[i])
		} else {
			c = pedersenCommitment(big.NewInt(0), txRandom[i])
		}
		commitments = append(commitments, c)
	}
	commitmentsEnygma := make([]enygma.IEnygmaPoint, 6)

	for i := 0; i < len(kIndex); i++ {
		commit := enygma.IEnygmaPoint{C1: commitments[i].X, C2: commitments[i].Y}
		commitmentsEnygma[i] = commit
	}

	return commitmentsEnygma, txRandom
}

func getTxRandomAndRValues(s []*big.Int, block_number *big.Int, senderId int, kIndex []int) []*big.Int {
	var rValues []*big.Int = make([]*big.Int, len(kIndex)) // Initialize rValues to the length of kIndex
	sum := big.NewInt(0)                                   // Initialize the sum as zero

	// Iterate over the kIndex list to calculate rValues
	for i, idx := range kIndex {
		// Reduce s[idx] and block_number modulo P before hashing
		modBlockNumber := new(big.Int).Mod(block_number, P)
		modS := new(big.Int).Mod(s[idx], P)

		// Prepare inputs for Poseidon hash
		inputs := []*big.Int{modBlockNumber, modS}

		// Calculate Poseidon hash and reduce it modulo P
		PoseidonHash, _ := poseidon.Hash(inputs)
		PoseidonHash.Mod(PoseidonHash, P)
		// Assign PoseidonHash to rValues
		rValues[i] = getNegative(PoseidonHash)

		// Check if the current index corresponds to senderId
		if idx != senderId {
			// Sum rValues of non-sender participants modulo P
			sum.Add(sum, PoseidonHash)
			sum.Mod(sum, P)
		}
	}

	// Assign the sum of all other rValues to the sender's position in rValues
	for i, idx := range kIndex {
		if idx == senderId {
			rValues[i] = sum // Assign sum to the sender's rValue
			break
		}
	}

	return rValues
}

func ConnectToSmartContract() (*ethclient.Client, *bind.TransactOpts, error) {
	client, err := ethclient.Dial(CommitChainURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to Ethereum client: %v", err)
	}

	privateKey, err := crypto.HexToECDSA(PrivateKeyString)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse private key: %v", err)
	}

	auth, _ := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(1337))
	auth.Value = big.NewInt(0)
	auth.GasLimit = uint64(3000000)
	auth.GasPrice, _ = client.SuggestGasPrice(nil)

	return client, auth, nil
}

func InsertLeaves(commitment string) error {
	jsonInfo := struct {
		Commitment string `json:"commitment"`
	}{
		Commitment: commitment,
	}

	response, err := PostJSON(ZkdvpAddLeaveURL, jsonInfo)
	if err != nil {
		return err
	}

	fmt.Println("InsertLeaves Response: ", string(response))
	return nil
}

func GetPkZkDvP(secret *big.Int) *big.Int{
	inputs := []*big.Int{secret}
	PoseidonHash, _ := poseidon.Hash(inputs)

	return PoseidonHash

}


func ConvertStringToBigInt(arrayString []string) []*big.Int {
	var bigIntArray []*big.Int 
	
	for i := 0; i < len(arrayString); i++{
		bigInt0,_ :=new(big.Int).SetString(arrayString[i], 10)
		bigIntArray = append(bigIntArray, bigInt0)
	}

	return bigIntArray
}