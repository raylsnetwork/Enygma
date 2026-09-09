package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"

	enygma "enygma-server/pkg/circuits/enygma"
	enygma_fee "enygma-server/pkg/circuits/enygma_fee"
	deposit "enygma-server/pkg/circuits/deposit"
	relayer "enygma-server/pkg/circuits/relayer"
	withdraw "enygma-server/pkg/circuits/withdraw"
	utils "enygma-server/utils"

	"github.com/consensys/gnark/std/algebra/emulated/sw_bn254"
	stdgroth16 "github.com/consensys/gnark/std/recursion/groth16"
)

const splitSize = 6

// Generic key generation function to reduce code duplication
func generateKeys(circuit frontend.Circuit, pkPath, vkPath, solPath string) error {
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

	fSol, err := os.Create(solPath)
	if err != nil {
		return fmt.Errorf("could not create verifier sol %s: %w", solPath, err)
	}
	defer fSol.Close()
	if err := vk.ExportSolidity(fSol); err != nil {
		return fmt.Errorf("export solidity failed for %s: %w", solPath, err)
	}

	fmt.Printf("✓ Keys generated successfully: %s, %s, %s\n", pkPath, vkPath, solPath)
	return nil
}

func generateKeysEnygma() error {
	config := enygma.EnygmaCircuitConfig{
		NCommitment: 6,
	}

	fp := make([][]frontend.Variable, config.NCommitment)
	for i := range fp {
		fp[i] = make([]frontend.Variable, config.NCommitment)
	}
	enygmaCircuit := enygma.EnygmaCircuit{
		Config:                     config,
		FingerPrintofSharedSecrets: fp,
		PublicKey:                  make([]frontend.Variable, config.NCommitment),
		PreviousCommit:             make([][2]frontend.Variable, config.NCommitment),
		TxCommit:                   make([][2]frontend.Variable, config.NCommitment),
		AnonymitySet:               make([]frontend.Variable, config.NCommitment),
		SharedSecrets:              make([]frontend.Variable, config.NCommitment),
		MessageTags:                make([]frontend.Variable, config.NCommitment),
		TxValues:                   make([]frontend.Variable, config.NCommitment),
		TxRandomValues:             make([]frontend.Variable, config.NCommitment),
	}

	return generateKeys(
		&enygmaCircuit,
		"keys/EnygmaPk.key",
		"keys/EnygmaVk.key",
		"keys/EnygmaVerifier.sol",
	)
}

// generateKeysRelayer builds the relayer's recursive-verification circuit
// keys. It requires keys/EnygmaVk.key to already exist — the relayer
// circuit embeds the transfer circuit's verifying key as a compile-time
// constant, so "enygma" must be generated before "relayer" (enforced by job
// ordering in main(), below).
func generateKeysRelayer() error {
	enygmaVk, err := utils.LoadVerifyingKey(ecc.BN254, "keys/EnygmaVk.key")
	if err != nil {
		return fmt.Errorf("generateKeysRelayer: failed to load keys/EnygmaVk.key (generate the enygma circuit's keys first): %w", err)
	}

	// Recompile the transfer circuit only to learn its exact public-input
	// count, needed to correctly size the placeholder inner witness below
	// (stdgroth16.PlaceholderWitness sizes off ccs.GetNbPublicVariables()).
	// This does not touch or regenerate the enygma circuit's own keys.
	config := enygma.EnygmaCircuitConfig{NCommitment: splitSize}
	fp := make([][]frontend.Variable, config.NCommitment)
	for i := range fp {
		fp[i] = make([]frontend.Variable, config.NCommitment)
	}
	enygmaTemplate := &enygma.EnygmaCircuit{
		Config:                     config,
		FingerPrintofSharedSecrets: fp,
		PublicKey:                  make([]frontend.Variable, config.NCommitment),
		PreviousCommit:             make([][2]frontend.Variable, config.NCommitment),
		TxCommit:                   make([][2]frontend.Variable, config.NCommitment),
		AnonymitySet:               make([]frontend.Variable, config.NCommitment),
		SharedSecrets:              make([]frontend.Variable, config.NCommitment),
		MessageTags:                make([]frontend.Variable, config.NCommitment),
		TxValues:                   make([]frontend.Variable, config.NCommitment),
		TxRandomValues:             make([]frontend.Variable, config.NCommitment),
	}
	enygmaCcs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, enygmaTemplate)
	if err != nil {
		return fmt.Errorf("generateKeysRelayer: failed to compile enygma circuit: %w", err)
	}

	fixedVk, err := stdgroth16.ValueOfVerifyingKeyFixed[sw_bn254.G1Affine, sw_bn254.G2Affine, sw_bn254.GTEl](enygmaVk)
	if err != nil {
		return fmt.Errorf("generateKeysRelayer: failed to build fixed verifying key: %w", err)
	}

	relayerCircuit := relayer.NewCircuit(fixedVk)
	relayerCircuit.InnerWitness = stdgroth16.PlaceholderWitness[sw_bn254.ScalarField](enygmaCcs)

	return generateKeys(
		relayerCircuit,
		"keys/RelayerPk.key",
		"keys/RelayerVk.key",
		"keys/RelayerVerifier.sol",
	)
}

func generateKeysEnygmaFee() error {
	config := enygma_fee.EnygmaFeeCircuitConfig{NCommitment: 6}
	circuit := enygma_fee.EnygmaFeeCircuit{
		Config:              config,
		HashedSharedSecrets: make([]frontend.Variable, config.NCommitment),
		PublicKey:           make([]frontend.Variable, config.NCommitment),
		PreviousCommit:      make([][2]frontend.Variable, config.NCommitment),
		TxCommit:            make([][2]frontend.Variable, config.NCommitment),
		AnonymitySet:        make([]frontend.Variable, config.NCommitment),
		SharedSecrets:       make([]frontend.Variable, config.NCommitment),
		MessageTags:         make([]frontend.Variable, config.NCommitment),
		TxValues:            make([]frontend.Variable, config.NCommitment),
		TxRandomValues:      make([]frontend.Variable, config.NCommitment),
	}
	return generateKeys(
		&circuit,
		"keys/EnygmaFeePk.key",
		"keys/EnygmaFeeVk.key",
		"keys/EnygmaFeeVerifier.sol",
	)
}

func generateKeysZkDvpDeposit() error {
	config := deposit.DepositEnygmaCircuitConfig{
		NCommitment: 6,
	}
	depositCircuit := deposit.DepositEnygmaCircuit{
		Config:              config,
		HashedSharedSecrets: make([]frontend.Variable, config.NCommitment),
		PublicKey:           make([]frontend.Variable, config.NCommitment),
		PreviousCommit:      make([][2]frontend.Variable, config.NCommitment),
		TxCommit:            make([][2]frontend.Variable, config.NCommitment),
		AnonymitySet:        make([]frontend.Variable, config.NCommitment),
		SharedSecrets:       make([]frontend.Variable, config.NCommitment),
		MessageTags:         make([]frontend.Variable, config.NCommitment),
		TxValues:            make([]frontend.Variable, config.NCommitment),
		TxRandomValues:      make([]frontend.Variable, config.NCommitment),
	}
	return generateKeys(
		&depositCircuit,
		"keys/zkdvp/DepositPk.key",
		"keys/zkdvp/DepositVk.key",
		"keys/zkdvp/DepositVerifier.sol",
	)
}

func generateKeysZkDvpWithdraw() error {
	for i := 1; i <= splitSize; i++ {
		config := withdraw.WithdrawEnygmaCircuitConfig{
			NCommitment: 6,
		}
		
		withdrawCircuit := withdraw.WithdrawEnygmaCircuit{
			Config:              config,
			HashedSharedSecrets: make([]frontend.Variable, config.NCommitment),
			PublicKey:           make([]frontend.Variable, config.NCommitment),
			PreviousCommit:      make([][2]frontend.Variable, config.NCommitment),
			TxCommit:            make([][2]frontend.Variable, config.NCommitment),
			AnonymitySet:        make([]frontend.Variable, config.NCommitment),
			SharedSecrets:       make([]frontend.Variable, config.NCommitment),
			MessageTags:         make([]frontend.Variable, config.NCommitment),
			TxValues:            make([]frontend.Variable, config.NCommitment),
			TxRandomValues:      make([]frontend.Variable, config.NCommitment),
		}
		
		pkPath := fmt.Sprintf("keys/zkdvp/WithdrawPk%d.key", i)
		vkPath := fmt.Sprintf("keys/zkdvp/WithdrawVk%d.key", i)
		solPath := fmt.Sprintf("keys/zkdvp/WithdrawVerifier%d.sol", i)

		if err := generateKeys(&withdrawCircuit, pkPath, vkPath, solPath); err != nil {
			return err
		}
	}
	return nil
}

// main runs key generation.
//
// Usage (MUST run from the gnark-server/ directory, not from keygen/):
//
//	cd enygma_payments/gnark-server
//	go run ./keygen/generate_keys.go              # regenerate ALL keys
//	go run ./keygen/generate_keys.go -circuit enygma_fee  # only fee keys
//
// Available -circuit values: all, enygma, relayer, enygma_fee, deposit, withdraw
//
// NOTE: "relayer" embeds the "enygma" circuit's verifying key as a
// compile-time constant, so it must be generated after "enygma" — the job
// ordering below (relayer immediately follows enygma) enforces this for
// -circuit all; regenerating just "relayer" on its own requires
// keys/EnygmaVk.key to already exist from a prior run.
func main() {
	circuit := flag.String("circuit", "all",
		"which circuit keys to generate: all | enygma | relayer | enygma_fee | deposit | withdraw")
	flag.Parse()

	type job struct {
		name string
		fn   func() error
	}

	all := []job{
		{"enygma", generateKeysEnygma},
		{"relayer", generateKeysRelayer},
		{"enygma_fee", generateKeysEnygmaFee},
		{"deposit", generateKeysZkDvpDeposit},
		{"withdraw", generateKeysZkDvpWithdraw},
	}

	var jobs []job
	if *circuit == "all" {
		jobs = all
	} else {
		for _, j := range all {
			if j.name == *circuit {
				jobs = append(jobs, j)
				break
			}
		}
		if len(jobs) == 0 {
			fmt.Printf("unknown -circuit %q — valid values: all, enygma, relayer, enygma_fee, deposit, withdraw\n", *circuit)
			os.Exit(1)
		}
	}

	fmt.Printf("Starting key generation (circuit=%s)…\n", *circuit)
	for _, j := range jobs {
		if err := j.fn(); err != nil {
			fmt.Printf("Error generating %s keys: %v\n", j.name, err)
			os.Exit(1)
		}
	}
	fmt.Println("✓ Keys generated successfully!")
}