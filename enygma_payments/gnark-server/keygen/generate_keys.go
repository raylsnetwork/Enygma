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
	withdraw "enygma-server/pkg/circuits/withdraw"
	burn "enygma-server/pkg/circuits/burn"
	utils "enygma-server/utils"
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

// Fix M-16: this used to loop i := 1..splitSize, generating SIX
// independent groth16.Setup trusted setups for the byte-for-byte
// identical constraint system (the loop variable i was used only in the
// three output filenames — config.NCommitment is hardcoded to 6
// regardless of i, and "the split count" the naming implies exists only
// as a commented-out `// const nSplit = 6` in the circuit itself). Two
// consequences, both closed by generating exactly one key now:
//  1. Enygma.sol's withdraw() selects its verifier by
//     commitmentDeltas.length, which the function itself forces to equal
//     DEFAULT_SIZE (6) — so _withdrawVerifiers[6] was already the ONLY
//     slot ever reachable through the real call path; the other five
//     keys were pure dead weight (and most of the audit's measured
//     "~30s cold start" — five of the six withdraw setups being
//     redundant).
//  2. Six separate setups meant six independent trapdoors for one
//     constraint system — anyone holding the toxic waste from any ONE of
//     the five now-removed setups could have forged proofs accepted by
//     that (dead but still deployed, if ever wired to a live slot other
//     than 6) verifier. Down to exactly one setup means exactly one
//     trapdoor, matching what's actually used.
func generateKeysZkDvpWithdraw() error {
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

	// splitSize (6) names the one reachable slot — matches DEFAULT_SIZE
	// in Enygma.sol and _withdrawVerifiers[6]'s lookup key.
	pkPath := fmt.Sprintf("keys/zkdvp/WithdrawPk%d.key", splitSize)
	vkPath := fmt.Sprintf("keys/zkdvp/WithdrawVk%d.key", splitSize)
	solPath := fmt.Sprintf("keys/zkdvp/WithdrawVerifier%d.sol", splitSize)

	return generateKeys(&withdrawCircuit, pkPath, vkPath, solPath)
}

func generateKeysBurn() error {
	circuit := burn.BurnCircuit{}
	return generateKeys(
		&circuit,
		"keys/BurnPk.key",
		"keys/BurnVk.key",
		"keys/BurnVerifier.sol",
	)
}

// main runs key generation.
//
// Usage (MUST run from the gnark-server/ directory, not from keygen/):
//
//	cd enygma_payments/gnark-server
//	go run ./keygen/generate_keys.go              # regenerate ALL keys
//	go run ./keygen/generate_keys.go -circuit enygma_fee  # only fee keys
//
// Available -circuit values: all, enygma, enygma_fee, deposit, withdraw, burn
//
// H-13: burn is a brand new circuit added by this fix, not a re-key of an
// existing one — see gnark-server/pkg/circuits/burn. Like every other
// circuit-touching fix on this branch (C-01/C-02/C-03/C-08/H-11/H-01/H-02),
// this job exists so it can be included in the single batched trusted-setup
// ceremony (H-12) rather than triggering its own one-off `groth16.Setup`
// call — do not run this job in production ahead of that ceremony.
func main() {
	circuit := flag.String("circuit", "all",
		"which circuit keys to generate: all | enygma | enygma_fee | deposit | withdraw | burn")
	flag.Parse()

	type job struct {
		name string
		fn   func() error
	}

	all := []job{
		{"enygma", generateKeysEnygma},
		{"enygma_fee", generateKeysEnygmaFee},
		{"deposit", generateKeysZkDvpDeposit},
		{"withdraw", generateKeysZkDvpWithdraw},
		{"burn", generateKeysBurn},
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
			fmt.Printf("unknown -circuit %q — valid values: all, enygma, enygma_fee, deposit, withdraw, burn\n", *circuit)
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