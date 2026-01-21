package test

import (
	"testing"
	"fmt"
	
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
    "github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark-crypto/ecc"
	"gnark_server/templates"
	"gnark_server/primitives"
)



func TestErc20Circuit(t *testing.T) {
    // 1) Instantiate an empty circuit
    var circuit templates.Erc721Circuit
	solver.RegisterHint(primitives.ModHint)
	solver.RegisterHint(primitives.ERC155UniqueIdNative)
	solver.RegisterHint(primitives.PoseidonNative)

    // 2) Compile it to R1CS
    cc, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
    if err != nil {
        t.Fatalf("circuit compilation failed: %v", err)
    }

	pk, vk, err := groth16.Setup(cc)
    if err != nil {
        t.Fatalf("setup error: %v", err)
    }

	var witness templates.Erc721Circuit

witness.Message = frontend.Variable(0)
	witness.ValuesIn[0] = frontend.Variable("4439357066836141079576128573082253885786048249588142940202752072401914133233")
	witness.MerkleRoots = frontend.Variable("9474206899893398304566266879838327336710863636103042127333418279494585973039")
	witness.PrivateKeys[0] =frontend.Variable("3632415099307080219573419859460348813488014554976588881844159347606981925862")

	witness.PathElements[0][0] = frontend.Variable("5647812400155766518022811634515916803240289455591301803628151118225344751846")
	witness.PathElements[0][1] = frontend.Variable("18548196279604799461048000334259916142758522801444715662239164855577399482970")
	witness.PathElements[0][2] = frontend.Variable("6394967702556731931349597360857231577818344086247308420600702098348151392332")
	witness.PathElements[0][3] = frontend.Variable("20304752715056542815811329636342056767286663221745207495535784435548485430561")
	witness.PathElements[0][4] = frontend.Variable("10123741241923738392878827949878021844799105553604491080266583922031272507954")
	witness.PathElements[0][5] = frontend.Variable("7763070937946622828545897559398364952701415676360141127081877575944863661924")
	witness.PathElements[0][6] = frontend.Variable("4336110172767843874572170947831826800762554433417721132971126558487413457970")
	witness.PathElements[0][7] = frontend.Variable("17793516955206672117877101086466671169038346493482525475927531680188516206475")

	witness.PathIndices[0] = frontend.Variable(1)
	witness.Nullifiers[0] = frontend.Variable("16739510012213295963644267443868891201560496946610070604992251727070104210806")
	
	witness.RecipientPk[0] =frontend.Variable("902270300019012918568897505096485064485894695815")
	
	witness.ValuesOut[0] = frontend.Variable("4439357066836141079576128573082253885786048249588142940202752072401914133233")

	witness.CommitmentOut[0] = frontend.Variable("20264104623186945773152619177943719481728020329718282003411490538711454387477")

	witness.TreeNumber = frontend.Variable("0")

    fullWit, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField())
    if err != nil {
        t.Fatalf("new witness error: %v", err)
    }
    publicWit, err := fullWit.Public()
    if err != nil {
        t.Fatalf("public witness error: %v", err)
    }


    proof, err := groth16.Prove(cc, pk, fullWit)
    if err != nil {
        t.Fatalf("prove error: %v", err)
    }

	  if err := groth16.Verify(proof, vk, publicWit); err != nil {
        t.Fatalf("verify error: %v", err)
    }
	fmt.Println("ERC20 Fungible Template Proof verified successfully!")
}