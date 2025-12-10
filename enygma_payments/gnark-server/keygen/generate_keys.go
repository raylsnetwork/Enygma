package main

import (
	"fmt"
	"log"
	"os"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend/cs/r1cs"

	enygma "enygma-server/pkg/circuits/enygma" 
	deposit "enygma-server/pkg/circuits/deposit" 
	withdraw "enygma-server/pkg/circuits/withdraw" 
	utils "enygma-server/utils"
)

const splitSize = 6

// Generic key generation function to reduce code duplication
func generateKeys(circuit frontend.Circuit, pkPath, vkPath string) error {
	fmt.Printf("Generating keys for: %s\n", pkPath)
	
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		return fmt.Errorf("compile failed for %s: %w", pkPath, err)
	}

	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		return fmt.Errorf("setup failed for %s: %w", pkPath, err)
	}

	if err := utils.SavingFiles(pkPath, vkPath, pk, vk); err != nil {
		return fmt.Errorf("saving files failed for %s: %w", pkPath, err)
	}

	fmt.Printf("✓ Keys generated successfully: %s  and %s\n", pkPath,vkPath)
	return nil
}

func generateKeysEnygma() error {
	config:= enygma.EnygmaCircuitConfig{
		BitWith:256,
		NCommitment:6,
	}
	enygmaCircuit:= enygma.EnygmaCircuit{
		Config:config,
		ArrayHashSecret:make([][]frontend.Variable, config.NCommitment),
		PublicKey:      make([]frontend.Variable,config.NCommitment),
		PreviousCommit: make([][2]frontend.Variable, config.NCommitment),
		KIndex:  		make([]frontend.Variable,config.NCommitment),
		Secrets:		make([][]frontend.Variable, config.NCommitment),
		TagMessage: 	make([]frontend.Variable,config.NCommitment),
		TxCommit: 		make([][2]frontend.Variable, config.NCommitment),
		TxValue: 		make([]frontend.Variable,config.NCommitment),
		TxRandom: 		make([]frontend.Variable,config.NCommitment),

	}

	for i := range config.NCommitment {

        enygmaCircuit.ArrayHashSecret[i] = make([]frontend.Variable, config.NCommitment)
		enygmaCircuit.Secrets[i] = make([]frontend.Variable, config.NCommitment)
    }

	
	return generateKeys(
		&enygmaCircuit,
		"keys/EnygmaPk.key",
		"keys/EnygmaVk.key",
	)
}

func generateKeysZkDvpDeposit() error {
	var depositCircuit deposit.DepositEnygmaCircuit
	return generateKeys(
		&depositCircuit,
		"keys/zkdvp/DepositPk.key",
		"keys/zkdvp/DepositVk.key",
	)
}

func generateKeysZkDvpWithdraw() error {
	for i := 1; i <= splitSize; i++ {
		config := withdraw.WithdrawEnygmaCircuitConfig{
			NSplit: i,
		}
		
		withdrawCircuit := withdraw.WithdrawEnygmaCircuit{
			Config:    config,
			HashArray: make([]frontend.Variable, config.NSplit),
			VArray:    make([]frontend.Variable, config.NSplit),
			Pk:        make([]frontend.Variable, config.NSplit),
		}
		
		pkPath := fmt.Sprintf("keys/zkdvp/WithdrawPk%d.key", i)
		vkPath := fmt.Sprintf("keys/zkdvp/WithdrawVk%d.key", i)
		
		if err := generateKeys(&withdrawCircuit, pkPath, vkPath); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	fmt.Println("Starting key generation...")
	
	// Sequential execution with error handling
	if err := generateKeysEnygma(); err != nil {
		fmt.Printf("Error generating Enygma keys: %v\n", err)
		return
	}
	
	// if err := generateKeysZkDvpDeposit(); err != nil {
	// 	fmt.Printf("Error generating Deposit keys: %v\n", err)
	// 	return
	// }
	
	// if err := generateKeysZkDvpWithdraw(); err != nil {
	// 	fmt.Printf("Error generating Withdraw keys: %v\n", err)
	// 	return
	// }
	
	fmt.Println("✓ All keys generated successfully!")
}