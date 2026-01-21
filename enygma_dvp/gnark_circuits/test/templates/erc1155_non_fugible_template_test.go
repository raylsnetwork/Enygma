package test

import (
	"testing"
	"fmt"
	"log"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
    "github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark-crypto/ecc"
	"gnark_server/templates"
	"gnark_server/primitives"
)

func TestEr1155NonFungibleCircuit(t *testing.T) {
	
	
	var circuit templates.ERC1155NonFungibleCircuit
	solver.RegisterHint(primitives.ModHint)
	solver.RegisterHint(primitives.ERC155UniqueIdNative)
	solver.RegisterHint(primitives.PoseidonNative)
	
    // 2) Compile it to R1CS
    ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
    if err != nil {
        t.Fatalf("circuit compilation failed: %v", err)
    }

	pk, vk, err := groth16.Setup(ccs)
    if err != nil {
        t.Fatalf("setup error: %v", err)
    }

	var witness templates.ERC1155NonFungibleCircuit
	witness.StMessage = frontend.Variable(0)
	witness.StTreeNumbers[0] = frontend.Variable(0)
	
	witness.StMerkleRoots[0] = frontend.Variable("5015379001191728684065191435716069739588755884767248042575705416979445370516")

	witness.StNullifiers[0] = frontend.Variable("8590843529303629213672805585652518591845625662985804194539596638001698137329")
	
	witness.StCommitmentOut[0] = frontend.Variable("3363647828104204740961831826391265015974935355629795328851804968816036188888")
	
	witness.StAssetGroupMerkleRoot[0] = frontend.Variable("19513357275602891559563293403649553644928068741036343117680935662240406981597")

	witness.StAssetGroupTreeNumber[0] = frontend.Variable("19513357275602891559563293403649553644928068741036343117680935662240406981597")


	witness.WtPrivateKeys[0] = frontend.Variable("3018446416600974899307163627201125457451016079685814234364164808825378975065")
	
	witness.WtValues[0] = frontend.Variable(1)

	witness.WtPathElements[0][0] = frontend.Variable("21786163348386038604960770631540184405176086071424084281943739401806009032442")
	witness.WtPathElements[0][1] = frontend.Variable("7747158917768839779257019139610198751020281645444268631809010087516198759109")
	witness.WtPathElements[0][2] = frontend.Variable("6835016751643004685399531239412290004916600334737160799018280508881893131965")
	witness.WtPathElements[0][3] = frontend.Variable("20304752715056542815811329636342056767286663221745207495535784435548485430561")
	witness.WtPathElements[0][4] = frontend.Variable("10123741241923738392878827949878021844799105553604491080266583922031272507954")
	witness.WtPathElements[0][5] = frontend.Variable("7763070937946622828545897559398364952701415676360141127081877575944863661924")
	witness.WtPathElements[0][6] = frontend.Variable("4336110172767843874572170947831826800762554433417721132971126558487413457970")
	witness.WtPathElements[0][7] = frontend.Variable("17793516955206672117877101086466671169038346493482525475927531680188516206475")
	
	witness.WtPathIndices[0] = frontend.Variable(6)

	witness.WtErc1155TokenId[0] = frontend.Variable("2023559577")

	witness.WtOutPublicKeys[0] = frontend.Variable("902270300019012918568897505096485064485894695815")

	witness.WtErc1155ContractAddress = frontend.Variable("738117306821290338319128355827600052815533846299")
	
	witness.WtAssetGroupPathElements[0][0] = frontend.Variable("21786163348386038604960770631540184405176086071424084281943739401806009032442")
	witness.WtAssetGroupPathElements[0][1] = frontend.Variable("18548196279604799461048000334259916142758522801444715662239164855577399482970")
	witness.WtAssetGroupPathElements[0][2] = frontend.Variable("6394967702556731931349597360857231577818344086247308420600702098348151392332")
	witness.WtAssetGroupPathElements[0][3] = frontend.Variable("20304752715056542815811329636342056767286663221745207495535784435548485430561")
	witness.WtAssetGroupPathElements[0][4] = frontend.Variable("10123741241923738392878827949878021844799105553604491080266583922031272507954")
	witness.WtAssetGroupPathElements[0][5] = frontend.Variable("7763070937946622828545897559398364952701415676360141127081877575944863661924")
	witness.WtAssetGroupPathElements[0][6] = frontend.Variable("4336110172767843874572170947831826800762554433417721132971126558487413457970")
	witness.WtAssetGroupPathElements[0][7] = frontend.Variable("17793516955206672117877101086466671169038346493482525475927531680188516206475")

	witness.WtAssetGroupPathIndices[0] = frontend.Variable(0)
	

	witnessFull, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField())
		if err != nil {
			log.Fatal(err)
		}
	proof, err := groth16.Prove(ccs, pk, witnessFull)

	witnessPublic, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		log.Fatal(err)
	}

	
	err = groth16.Verify(proof, vk, witnessPublic)
	if err != nil {
		panic(err)
	}

	fmt.Println("Proof verified successfully!")


}