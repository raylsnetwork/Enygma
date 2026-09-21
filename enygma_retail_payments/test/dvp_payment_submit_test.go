package tests

// Minimal ABI-shaped types and a submit helper for calling
// EnygmaDvp.payment() directly (used by 01_payment_test.go).
//
// Moved here from enygma_dvp/src/core/endpoints, which was otherwise unused.

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// These must match the IEnygmaDvp ABI tuple definitions exactly so that
// go-ethereum's reflection-based ABI encoder produces the correct calldata.

// G1Point matches Solidity: struct IEnygmaDvp.G1Point { uint256 x; uint256 y; }
type G1Point struct {
	X *big.Int `abi:"x"`
	Y *big.Int `abi:"y"`
}

// G2Point matches Solidity: struct IEnygmaDvp.G2Point { uint256[2] x; uint256[2] y; }
type G2Point struct {
	X [2]*big.Int `abi:"x"`
	Y [2]*big.Int `abi:"y"`
}

// SnarkProof matches Solidity: struct IEnygmaDvp.SnarkProof { G1Point a; G2Point b; G1Point c; }
type SnarkProof struct {
	A G1Point `abi:"a"`
	B G2Point `abi:"b"`
	C G1Point `abi:"c"`
}

//	ProofReceipt matches Solidity: struct IEnygmaDvp.ProofReceipt {
//	  SnarkProof proof; uint256[] statement; uint256 numberOfInputs; uint256 numberOfOutputs;
//	}
type ProofReceipt struct {
	Proof           SnarkProof `abi:"proof"`
	Statement       []*big.Int `abi:"statement"`
	NumberOfInputs  *big.Int   `abi:"numberOfInputs"`
	NumberOfOutputs *big.Int   `abi:"numberOfOutputs"`
}

// submitPayment submits a Payment circuit proof to EnygmaDvp.payment() and
// waits for the receipt. ctxt and encTxData are Bob's ML-KEM capsule and
// AES-GCM ciphertext (output 0 only); Alice's change ciphertext is not
// published — she holds saltA locally.
func submitPayment(
	client *ethclient.Client,
	auth *bind.TransactOpts,
	contractABI abi.ABI,
	contractAddr common.Address,
	receipt ProofReceipt,
	vaultId *big.Int,
	ctxt []byte,
	encTxData []byte,
) (*types.Receipt, error) {
	contract := bind.NewBoundContract(contractAddr, contractABI, client, client, client)
	tx, err := contract.Transact(auth, "payment", receipt, vaultId, ctxt, encTxData)
	if err != nil {
		return nil, fmt.Errorf("payment failed: %w", err)
	}
	mined, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		return nil, fmt.Errorf("waiting for payment receipt failed: %w", err)
	}
	return mined, nil
}
