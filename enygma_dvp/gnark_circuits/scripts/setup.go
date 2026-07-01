package script

import(
	"fmt"
	"os"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/frontend"
	
	"gnark_server/templates"

)


func SetupPrivateMint(config templates.PrivateMintConfig, circuitName string){
	fmt.Print("Initializing Setup Process")
	fmt.Print("\n")
	circuit := templates.PrivateMintCircuit{
		Config: config,
	}

	printable:= fmt.Sprintf("Generating Proving Key and Veryfing key for %s",circuitName)
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

	SavingFiles(pkPath,vkPath, pk, vk)

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

