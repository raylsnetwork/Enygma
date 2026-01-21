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
    var circuit templates.Erc20Circuit
	solver.RegisterHint(primitives.ModHint)

    // 2) Compile it to R1CS
    cc, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
    if err != nil {
        t.Fatalf("circuit compilation failed: %v", err)
    }

	pk, vk, err := groth16.Setup(cc)
    if err != nil {
        t.Fatalf("setup error: %v", err)
    }

	erc20_join_split_10_2 := templates.Erc20CircuitConfig{
		NInputs: 10,
		MOutputs:  2,
		MerkleTreeDepth:8,
		Range: frontend.Variable("1000000000000000000000000000000000000"),
	}

	circuitERC20:=templates.Erc20Circuit{
		Config: erc20_join_split_10_2,
		TreeNumber:    				make([]frontend.Variable, erc20_join_split_10_2.NInputs),
		MerkleRoots:    			make([]frontend.Variable, erc20_join_split_10_2.NInputs),
		Nullifiers:  				make([]frontend.Variable, erc20_join_split_10_2.NInputs),
		CommitmentOut:  			make([]frontend.Variable, erc20_join_split_10_2.MOutputs),
		PrivateKeys: 				make([]frontend.Variable, erc20_join_split_10_2.NInputs),
		ValuesIn: 					make([]frontend.Variable, erc20_join_split_10_2.NInputs),
		PathElements:				make([][]frontend.Variable, erc20_join_split_10_2.NInputs),
		PathIndices:				make([]frontend.Variable, erc20_join_split_10_2.NInputs),
		RecipientPk:				make([]frontend.Variable, erc20_join_split_10_2.MOutputs),
		ValuesOut:    				make([]frontend.Variable, erc20_join_split_10_2.MOutputs),
    
	}

	for i := range circuitERC20.PathElements {

        circuitERC20.PathElements[i] = make([]frontend.Variable, erc20_join_split_10_2.MerkleTreeDepth)
    }

	var witness templates.Erc20Circuit
	
	witness.Config = erc20_join_split_10_2


	witness.Message = frontend.Variable("20635533247367070391952590620243602075566426396544826142892683974695500004640")

	witness.TreeNumber[0] = frontend.Variable(0)
	witness.TreeNumber[1] = frontend.Variable(0)

	witness.MerkleRoots[0] =frontend.Variable("8137076366110092165430068916033398368818046909631492199704323888350028135501")
	witness.MerkleRoots[1] =frontend.Variable("995903248001098894266903273287652263486078326708073038877055460718041375402")

	witness.Nullifiers[0] = frontend.Variable("20421901660213369716964735157781327278383096274953920989306200712886486681037")
	witness.Nullifiers[1] = frontend.Variable("21723333990228820576690980763039738824606149095884478620933008894106962057159")
	
	witness.CommitmentOut[0] = frontend.Variable("14435534265860470383254918255468605211010617354717340815256253817034630979306")
	witness.CommitmentOut[1] = frontend.Variable("6614463975716747063106861606353534848605019763917996811534483789817245807923")

	witness.PrivateKeys[0] = frontend.Variable("6670733758176734740920252165786124275776295656406483869098223479847391045518")
	witness.PrivateKeys[1] = frontend.Variable("14256931271122383862199644892064678211492137173725010279867919926031601593089")

	witness.ValuesIn[0] = frontend.Variable(5638)
	witness.ValuesIn[1] = frontend.Variable(5638)

	witness.PathElements[0][0] =frontend.Variable("21786163348386038604960770631540184405176086071424084281943739401806009032442")
	witness.PathElements[0][1] =frontend.Variable("18548196279604799461048000334259916142758522801444715662239164855577399482970")
	witness.PathElements[0][2] =frontend.Variable("6710186251038589018280388110854968733722859085076546628771924616015152724571")
	witness.PathElements[0][3] =frontend.Variable("20304752715056542815811329636342056767286663221745207495535784435548485430561")
	witness.PathElements[0][4] =frontend.Variable("10123741241923738392878827949878021844799105553604491080266583922031272507954")
	witness.PathElements[0][5] =frontend.Variable("7763070937946622828545897559398364952701415676360141127081877575944863661924")
	witness.PathElements[0][6] =frontend.Variable("4336110172767843874572170947831826800762554433417721132971126558487413457970")
	witness.PathElements[0][7] =frontend.Variable("17793516955206672117877101086466671169038346493482525475927531680188516206475")

	witness.PathElements[1][0] =frontend.Variable("21629651571311636466057656362530975722659392813986155718540248803239315973345")
	witness.PathElements[1][1] =frontend.Variable("18548196279604799461048000334259916142758522801444715662239164855577399482970")
	witness.PathElements[1][2] =frontend.Variable("6710186251038589018280388110854968733722859085076546628771924616015152724571")
	witness.PathElements[1][3] =frontend.Variable("20304752715056542815811329636342056767286663221745207495535784435548485430561")
	witness.PathElements[1][4] =frontend.Variable("10123741241923738392878827949878021844799105553604491080266583922031272507954")
	witness.PathElements[1][5] =frontend.Variable("7763070937946622828545897559398364952701415676360141127081877575944863661924")
	witness.PathElements[1][6] =frontend.Variable("4336110172767843874572170947831826800762554433417721132971126558487413457970")
	witness.PathElements[1][7] =frontend.Variable("17793516955206672117877101086466671169038346493482525475927531680188516206475")

	witness.PathIndices[0] = frontend.Variable(4)
	witness.PathIndices[1] = frontend.Variable(5)

	witness.Erc20ContractAddress = frontend.Variable("144728219537289428489393918839668691461399556295")

	witness.RecipientPk[0] = frontend.Variable("246965489656040821817218942477307543841180726568264988639672502437441029520")
	witness.RecipientPk[1] = frontend.Variable("1347900604386867285715789832454279969870084794118998245766437826212438311288")

	witness.ValuesOut[0] = frontend.Variable("11276")
	witness.ValuesOut[1] = frontend.Variable("0")

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