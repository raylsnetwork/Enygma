package contract

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"

	"enygma/internal/types"
	enygma "enygma/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Client interacts with Enygma smart contract
type Client struct {
	client      *ethclient.Client
	instance    *enygma.Enygma
	privateKey  *ecdsa.PrivateKey
	fromAddress common.Address
}


// NewClient creates a new contract client
func NewClient(rpcURL, contractAddress, privateKeyHex string) (*Client, error) {
	// Connect to Ethereum client
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to client: %w", err)
	}

	// Parse contract address
	contractAddr := common.HexToAddress(contractAddress)

	// Create contract instance
	instance, err := enygma.NewEnygma(contractAddr, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create contract instance: %w", err)
	}

	// Parse private key
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Derive from address
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("failed to cast public key to ECDSA")
	}
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	return &Client{
		client:      client,
		instance:    instance,
		privateKey:  privateKey,
		fromAddress: fromAddress,
	}, nil
}

// GetPublicValues retrieves public values from contract
func (c *Client) GetPublicValues(size int) ([]enygma.IEnygmaPoint, []*big.Int, error) {
	sizeBig := big.NewInt(int64(size))
	data, err := c.instance.GetPublicValues(&bind.CallOpts{}, sizeBig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get public values: %w", err)
	}

	return data.Balances, data.Keys, nil
}


// GetBlockHash retrieves block hash from contract
func (c*Client) GetBlockHash()(*big.Int, error){

	blockHash, err := c.instance.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		return nil,  fmt.Errorf("failed to get block hash: %w", err)
	}
	
	return blockHash, nil

}
// SendTransaction sends a confidential transfer transaction
func (c *Client) SendTransaction(
	commitments []enygma.IEnygmaPoint,
	proofResp *types.Response,
	kIndex []*big.Int,
) error {
	// Get nonce
	nonce, err := c.client.PendingNonceAt(context.Background(), c.fromAddress)
	if err != nil {
		return fmt.Errorf("failed to get nonce: %w", err)
	}

	// Get gas price
	gasPrice, err := c.client.SuggestGasPrice(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get gas price: %w", err)
	}

	// Create transaction options
	auth, err := bind.NewKeyedTransactorWithChainID(c.privateKey, big.NewInt(1337))
	if err != nil {
		return fmt.Errorf("failed to create transactor: %w", err)
	}

	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)
	auth.GasLimit = uint64(400000000)
	auth.GasPrice = gasPrice

	// Convert proof to contract format
	proof := c.convertProof(proofResp)

	// Send transaction

	tx, err := c.instance.Transfer(auth, commitments, proof, kIndex)
	if err != nil {
		return fmt.Errorf("failed to send transfer: %w", err)
	}

	// Wait for transaction to be mined
	receipt, err := bind.WaitMined(context.Background(), c.client, tx)
	if err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}

	if receipt.Status == 1 {
		log.Println("✓ Transfer was successful")
		log.Printf("Transaction hash: %s", tx.Hash().Hex())
	} else {
		return fmt.Errorf("transaction failed with status %d", receipt.Status)
	}

	return nil
}

func (c *Client) convertProof(resp *types.Response) enygma.IEnygmaProof {
	var proofArray [8]*big.Int
	for i := 0; i < 8 && i < len(resp.Proof); i++ {
		proofArray[i] = resp.Proof[i]
	}

	var publicSignalArray [62]*big.Int
	for i := 0; i < 62 && i < len(resp.PublicSignal); i++ {
		publicSignalArray[i] = resp.PublicSignal[i]
	}

	return enygma.IEnygmaProof{
		Proof:        proofArray,
		PublicSignal: publicSignalArray,
	}
}