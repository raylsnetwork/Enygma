package script

import (
	"fmt"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"os"

	"gnark_server/templates"
)

func SetupPrivateMint(config templates.PrivateMintConfig, circuitName string) {
	fmt.Print("Initializing Setup Process")
	fmt.Print("\n")
	circuit := templates.PrivateMintCircuit{
		Config: config,
	}

	printable := fmt.Sprintf("Generating Proving Key and Veryfing key for %s", circuitName)
	fmt.Println(printable)
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

	solidityFile, _ := os.Create("scripts/verifier/verifier_privateMint.sol")
	defer solidityFile.Close()

	err = vk.ExportSolidity(solidityFile)
	if err != nil {
		panic(err)
	}

}

func SetupDvPInitiator(config templates.DvPInitiatorCircuitConfig, circuitName string) {
	fmt.Println("Initializing Setup Process")

	circuit := templates.DvPInitiatorCircuit{
		Config:         config,
		WtPathElements: make([]frontend.Variable, config.TmMerkleTreeDepth),
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

func SetupDvPDestination(config templates.DvPDestinationCircuitConfig, circuitName string) {
	fmt.Println("Initializing Setup Process")

	circuit := templates.DvPDestinationCircuit{
		Config:         config,
		WtPathElements: make([]frontend.Variable, config.TmMerkleTreeDepth),
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

// ── Payment-family circuits — dedicated Payment deployment, ported from
// enygma_retail_payments/gnark_circuits/scripts/setup.go ──────────────────────

// SetupPayment also serves Payment2in — same templates.PaymentCircuit, just a
// different TmNInputs in the config (1 vs 2), matching retail's convention.
func SetupPayment(config templates.PaymentCircuitConfig, circuitName string) {
	fmt.Println("Initializing Setup Process")

	circuit := templates.PaymentCircuit{
		Config:               config,
		StTreeNumbers:        make([]frontend.Variable, config.TmNInputs),
		StMerkleRoots:        make([]frontend.Variable, config.TmNInputs),
		StNullifiers:         make([]frontend.Variable, config.TmNInputs),
		StCommitmentsOut:     make([]frontend.Variable, config.TmMOutputs),
		WtPrivateKeysIn:      make([]frontend.Variable, config.TmNInputs),
		WtValuesIn:           make([]frontend.Variable, config.TmNInputs),
		WtSaltsIn:            make([]frontend.Variable, config.TmNInputs),
		WtPathIndices:        make([]frontend.Variable, config.TmNInputs),
		WtPathElements:       make([][]frontend.Variable, config.TmNInputs),
		WtSpendPublicKeysOut: make([]frontend.Variable, config.TmMOutputs),
		WtValuesOut:          make([]frontend.Variable, config.TmMOutputs),
		WtSaltsOut:           make([]frontend.Variable, config.TmMOutputs),
	}
	for i := range circuit.WtPathElements {
		circuit.WtPathElements[i] = make([]frontend.Variable, config.TmMerkleTreeDepth)
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

func SetupPaymentFee(config templates.PaymentCircuitConfig, circuitName string) {
	fmt.Println("Initializing Setup Process")

	circuit := templates.PaymentFeeCircuit{
		Config:               config,
		StTreeNumbers:        make([]frontend.Variable, config.TmNInputs),
		StMerkleRoots:        make([]frontend.Variable, config.TmNInputs),
		StNullifiers:         make([]frontend.Variable, config.TmNInputs),
		StCommitmentsOut:     make([]frontend.Variable, config.TmMOutputs),
		WtPrivateKeysIn:      make([]frontend.Variable, config.TmNInputs),
		WtValuesIn:           make([]frontend.Variable, config.TmNInputs),
		WtSaltsIn:            make([]frontend.Variable, config.TmNInputs),
		WtPathIndices:        make([]frontend.Variable, config.TmNInputs),
		WtPathElements:       make([][]frontend.Variable, config.TmNInputs),
		WtSpendPublicKeysOut: make([]frontend.Variable, config.TmMOutputs),
		WtValuesOut:          make([]frontend.Variable, config.TmMOutputs),
		WtSaltsOut:           make([]frontend.Variable, config.TmMOutputs),
	}
	for i := range circuit.WtPathElements {
		circuit.WtPathElements[i] = make([]frontend.Variable, config.TmMerkleTreeDepth)
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

func SetupPaymentRelayerFeePublic(config templates.PaymentCircuitConfig, circuitName string) {
	fmt.Println("Initializing Setup Process")

	circuit := templates.PaymentRelayerFeePublicCircuit{
		Config:               config,
		StTreeNumbers:        make([]frontend.Variable, config.TmNInputs),
		StMerkleRoots:        make([]frontend.Variable, config.TmNInputs),
		StNullifiers:         make([]frontend.Variable, config.TmNInputs),
		StCommitmentsOut:     make([]frontend.Variable, config.TmMOutputs),
		WtPrivateKeysIn:      make([]frontend.Variable, config.TmNInputs),
		WtValuesIn:           make([]frontend.Variable, config.TmNInputs),
		WtSaltsIn:            make([]frontend.Variable, config.TmNInputs),
		WtPathIndices:        make([]frontend.Variable, config.TmNInputs),
		WtPathElements:       make([][]frontend.Variable, config.TmNInputs),
		WtSpendPublicKeysOut: make([]frontend.Variable, config.TmMOutputs),
		WtValuesOut:          make([]frontend.Variable, config.TmMOutputs),
		WtSaltsOut:           make([]frontend.Variable, config.TmMOutputs),
	}
	for i := range circuit.WtPathElements {
		circuit.WtPathElements[i] = make([]frontend.Variable, config.TmMerkleTreeDepth)
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

func SetupUsdrFee(config templates.PaymentCircuitConfig, circuitName string) {
	fmt.Println("Initializing Setup Process")

	circuit := templates.UsdrFeeCircuit{
		Config:               config,
		StTreeNumbers:        make([]frontend.Variable, config.TmNInputs),
		StMerkleRoots:        make([]frontend.Variable, config.TmNInputs),
		StNullifiers:         make([]frontend.Variable, config.TmNInputs),
		StCommitmentsOut:     make([]frontend.Variable, config.TmMOutputs),
		WtPrivateKeysIn:      make([]frontend.Variable, config.TmNInputs),
		WtValuesIn:           make([]frontend.Variable, config.TmNInputs),
		WtSaltsIn:            make([]frontend.Variable, config.TmNInputs),
		WtPathIndices:        make([]frontend.Variable, config.TmNInputs),
		WtPathElements:       make([][]frontend.Variable, config.TmNInputs),
		WtSpendPublicKeysOut: make([]frontend.Variable, config.TmMOutputs),
		WtValuesOut:          make([]frontend.Variable, config.TmMOutputs),
		WtSaltsOut:           make([]frontend.Variable, config.TmMOutputs),
	}
	for i := range circuit.WtPathElements {
		circuit.WtPathElements[i] = make([]frontend.Variable, config.TmMerkleTreeDepth)
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
