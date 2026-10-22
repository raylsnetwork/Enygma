package script

import (
	"fmt"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"

	"enygma_retail_payments/gnark_circuits/templates"
)

func SetupPayment(config templates.PaymentCircuitConfig, circuitName string) {
	fmt.Println("Initializing Setup Process")

	circuit := templates.NewPaymentCircuit(config)

	fmt.Printf("Generating Proving Key and Verifying Key for %s\n", circuitName)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		panic(err)
	}

	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		panic(err)
	}

	pkPath := fmt.Sprintf("scripts/keys/%sPK.key", circuitName)
	vkPath := fmt.Sprintf("scripts/keys/%sVK.key", circuitName)
	SavingFiles(pkPath, vkPath, pk, vk)
}

func SetupPaymentFee(config templates.PaymentCircuitConfig, circuitName string) {
	fmt.Println("Initializing Setup Process")

	circuit := templates.NewPaymentFeeCircuit(config)

	fmt.Printf("Generating Proving Key and Verifying Key for %s\n", circuitName)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		panic(err)
	}

	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		panic(err)
	}

	pkPath := fmt.Sprintf("scripts/keys/%sPK.key", circuitName)
	vkPath := fmt.Sprintf("scripts/keys/%sVK.key", circuitName)
	SavingFiles(pkPath, vkPath, pk, vk)
}

func SetupUsdrFee(config templates.PaymentCircuitConfig, circuitName string) {
	fmt.Println("Initializing Setup Process")

	circuit := templates.NewUsdrFeeCircuit(config)

	fmt.Printf("Generating Proving Key and Verifying Key for %s\n", circuitName)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		panic(err)
	}

	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		panic(err)
	}

	pkPath := fmt.Sprintf("scripts/keys/%sPK.key", circuitName)
	vkPath := fmt.Sprintf("scripts/keys/%sVK.key", circuitName)
	SavingFiles(pkPath, vkPath, pk, vk)
}

func SetupPaymentRelayerFeePublic(config templates.PaymentCircuitConfig, circuitName string) {
	fmt.Println("Initializing Setup Process")

	circuit := templates.NewPaymentRelayerFeePublicCircuit(config)

	fmt.Printf("Generating Proving Key and Verifying Key for %s\n", circuitName)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		panic(err)
	}

	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		panic(err)
	}

	pkPath := fmt.Sprintf("scripts/keys/%sPK.key", circuitName)
	vkPath := fmt.Sprintf("scripts/keys/%sVK.key", circuitName)
	SavingFiles(pkPath, vkPath, pk, vk)
}

func SetupPrivateMint(circuitName string) {
	fmt.Println("Initializing Setup Process")

	circuit := templates.PrivateMintCircuit{
		Config: templates.PrivateMintConfig{
			TmRange: frontend.Variable("1000000000000000000000000000000000000"),
		},
	}

	fmt.Printf("Generating Proving Key and Verifying Key for %s\n", circuitName)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		panic(err)
	}

	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		panic(err)
	}

	pkPath := fmt.Sprintf("scripts/keys/%sPK.key", circuitName)
	vkPath := fmt.Sprintf("scripts/keys/%sVK.key", circuitName)
	SavingFiles(pkPath, vkPath, pk, vk)
}
