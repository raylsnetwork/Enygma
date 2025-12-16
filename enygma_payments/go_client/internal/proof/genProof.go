package proof

import (
	"math/big"
	"strconv"
	"encoding/json"
	"io/ioutil"
	"bytes"
	enygma "enygma/contracts"
	"net/http"
	"enygma/internal/types"
	"enygma/config"
)



func GenerateProof(args *types.TransactionArgs, nullifier *big.Int, 
				blockHash *big.Int,  publicKey []*big.Int, 
				previousCommit []enygma.IEnygmaPoint, txCommit []enygma.IEnygmaPoint, 
				txValue []*big.Int, txRandom []*big.Int, secrets [][]*big.Int, 
				k_index []*big.Int, hashArray [][]*big.Int, tagMessage []*big.Int,cfg *config.Config) *types.Response {
	

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
	kIndexString := convertBigIntsToStrings(k_index)
	tagMessageString:=convertBigIntsToStrings(tagMessage)
	hashArrayString:= convertBigIntsToStrings2D(hashArray)



	jsonInfo := types.Proof{
		hashArrayString,//ArrayHashSecret		
		pkFinal, // publicKey - change
		refBalFinal, // previous Commit
		blockHash.String(), //block number
		kIndexString, //KIndex
		strconv.FormatInt(int64(args.SenderId), 10), //senderId
		secretsString, //secret
		tagMessageString, 
		args.Sk.String(),
		args.PreviousV.String(),
		args.PreviousR.String(),
		commFinal, //tx Commit
		txValueString,
		txRandomString,
		args.Value.String(),
		nullifier.String(),
		
	}

	jsonMar, _ := json.Marshal(jsonInfo)
	var jsonData = []byte(jsonMar)

	request, error := http.NewRequest("POST", cfg.ProofServerURL, bytes.NewBuffer(jsonData))
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

	var result types.Response
	
	e := json.Unmarshal(body, &result)
	// panic on error
	if e != nil {
		panic(e)
	}

	return &result

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