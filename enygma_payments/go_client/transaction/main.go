package main


import (
	"fmt"
	"math/big"
	"os"
	"strconv"
	"log"

	"github.com/iden3/go-iden3-crypto/poseidon"
	//"encoding/json"
	//"io/ioutil"
	"enygma/config"
	"enygma/internal/curve"
	"enygma/internal/contract"
	"enygma/internal/randomness"
	"enygma/internal/types"
	"enygma/internal/proof"

)



type Address struct {
    Address string `json:"address"`
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run() error {
	// Parse command-line arguments
	args, err := parseArguments()
	if err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}

	// Load configuration
	cfg, err := config.Load("./config/address.json")
	
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize secrets (in production, load from secure storage)
	secrets := initializeSecrets()

	// Execute transaction
	return executeTransaction(cfg, args, secrets)
}

func parseArguments() (*types.TransactionArgs, error) {
	if len(os.Args) < 7 {
		return nil, fmt.Errorf("usage: %s <qtyBank> <value> <senderId> <sk> <previousV> <previousR> ", os.Args[0])
	}

	qtyBanks, err := strconv.Atoi(os.Args[1])
	if err != nil {
		return nil, fmt.Errorf("invalid qtyBank: %w", err)
	}

	value := new(big.Int)
	if _, ok := value.SetString(os.Args[2], 10); !ok {
		return nil, fmt.Errorf("invalid value")
	}

	senderId, err := strconv.Atoi(os.Args[3])
	if err != nil {
		return nil, fmt.Errorf("invalid senderId: %w", err)
	}

	previousV := new(big.Int)
	if _, ok := previousV.SetString(os.Args[5], 10); !ok {
		return nil, fmt.Errorf("invalid previousV")
	}

	sk := new(big.Int)
	if _, ok := sk.SetString(os.Args[4], 10); !ok {
		return nil, fmt.Errorf("invalid sk")
	}


	previousR := new(big.Int)
	if _, ok := previousR.SetString(os.Args[6], 10); !ok {
		return nil, fmt.Errorf("invalid previousR")
	}


	return &types.TransactionArgs{
		QtyBanks:  qtyBanks,
		Value:     value,
		SenderId:  senderId,
		Sk:        sk,
		PreviousV: previousV,
		PreviousR: previousR,
	}, nil
}

func executeTransaction(cfg *config.Config, args *types.TransactionArgs, secrets [][]*big.Int) error {
	// Initialize contract client
	contractClient, err := contract.NewClient(cfg.CommitChainURL, cfg.ContractAddress, cfg.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to create contract client: %w", err)
	}

	// Get data from smart contract
	ReferenceBalance, PublicKeys, err1 := contractClient.GetPublicValues(args.QtyBanks)
	
	
	if err1 != nil {
		return fmt.Errorf("failed to get public values: %w", err)
	}

	BlockHash, err:=contractClient.GetBlockHash()

	// Build transaction
	kIndex := generateKIndex()
	HashArray:= randomness.HashArrayGen(args.SenderId,secrets,BlockHash, kIndex)
	TagMessage:= randomness.TagMessageGen(args.SenderId,secrets,BlockHash, kIndex, args.PreviousV)
	TxValue:= GenerateTxValues(args.Value)
	Nullifier, _ := poseidon.Hash([]*big.Int{HashArray[args.SenderId][args.SenderId], BlockHash})
	TxCommit,TxRandom:=randomness.GenCommitmentAndRandom(args.QtyBanks, args.Value,args.SenderId,TxValue, BlockHash, kIndex, secrets)


	fmt.Println("TxRandom",TxRandom)

	// Generate proof
	proofResponse := proof.GenerateProof(args, 
		Nullifier,
		BlockHash,
		PublicKeys,
		ReferenceBalance,
		TxCommit,
		TxValue,
		TxRandom,
		secrets,
		kIndex,
		HashArray,
		TagMessage,
		cfg,
	)

	// Send transaction
	if err := contractClient.SendTransaction(
		TxCommit,
		proofResponse,
		kIndex,
	); err != nil {
		return fmt.Errorf("failed to send transaction: %w", err)
	}

	log.Println("Transaction successful!")
	return nil
}

/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////// 
/////////// DEMO PURPOSE ONLY /////////////////////////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func GenerateTxValues(value *big.Int) []*big.Int {
	// TODO: This should be configurable based on actual recipients
	vNegate := curve.GetNegative(value)
	return []*big.Int{
		vNegate,
		big.NewInt(60),
		big.NewInt(40),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
	}
}

func generateKIndex() []*big.Int {
	k0:= big.NewInt(0) 
	k1:= big.NewInt(1)
	k2:= big.NewInt(2)
	k3:= big.NewInt(3)
	k4:= big.NewInt(4)
	k5:= big.NewInt(5)

	
	kIndex := []*big.Int{k0, k1, k2,k3,k4,k5}
	return kIndex
}

func initializeSecrets() [][]*big.Int {
	return [][]*big.Int{
		{
			mustParseBigInt("1057552177615391071371517357831905644488869902410624499760761745282922661721"),
			big.NewInt(54142),
			big.NewInt(814712),
			big.NewInt(250912012),
			big.NewInt(12312512),
			big.NewInt(12312512),
		},
		{
			big.NewInt(412321),
			big.NewInt(634609235),
			big.NewInt(8352331231),
			big.NewInt(289412412),
			big.NewInt(8932589237),
			big.NewInt(423423523),
		},
		{
			big.NewInt(3634),
			big.NewInt(967865),
			big.NewInt(364352),
			big.NewInt(25324754),
			big.NewInt(636434),
			big.NewInt(5674564),
		},
		{
			big.NewInt(41241226),
			big.NewInt(4365343563),
			big.NewInt(235234135),
			big.NewInt(2321634634),
			big.NewInt(3451212),
			big.NewInt(34634634),
		},
		{
			big.NewInt(5151341231),
			big.NewInt(64363452),
			big.NewInt(2141623423),
			big.NewInt(52355347345),
			big.NewInt(352112412),
			big.NewInt(514125123125),
		},
		{
			big.NewInt(5141264353),
			big.NewInt(46345346346),
			big.NewInt(63463442331),
			big.NewInt(2412624534),
			big.NewInt(346423423),
			big.NewInt(2412634634534),
		},
	}
}

func mustParseBigInt(s string) *big.Int {
	n := new(big.Int)
	if _, ok := n.SetString(s, 10); !ok {
		panic(fmt.Sprintf("invalid big int: %s", s))
	}
	return n
}

/////////////////////////////////////////////////////////////////////////////////