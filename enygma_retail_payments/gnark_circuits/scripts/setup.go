package script

import(
	"fmt"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/frontend"
	
	"gnark_server/templates"

)



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