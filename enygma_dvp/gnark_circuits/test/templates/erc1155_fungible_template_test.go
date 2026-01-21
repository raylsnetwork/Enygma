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


func TestErc1155FungibleCircuit(t *testing.T) {
    // 1) Instantiate an empty circuit
    var circuit templates.Erc1155FungibleCircuit
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

	var witness templates.Erc1155FungibleCircuit
	witness.StMessage = frontend.Variable(0)
	witness.StTreeNumbers[0] = frontend.Variable(0)
	witness.StMerkleRoots[0] = frontend.Variable("18254211460231342150844610803849197619505637760382127497981349963277960519558")
	witness.StNullifiers[0] = frontend.Variable("12710411068038003179331134748551713030011478231924725284707745863847725443687")
	witness.StCommitmentOut[0] = frontend.Variable("20120937903375695250581727741977340959141710193056093819597552151779578675216")
	witness.StAssetGroupMerkleRoot = frontend.Variable("15016459655189866454987152280659455010674610662298983734944531168056568656869")


	witness.WtPrivateKeys[0] = frontend.Variable("19506801210518003060615465271477608163840735387215767224812626336387963355164")
	witness.WtValuesIn[0] = frontend.Variable(100)
	witness.WtPathElements[0][0] = frontend.Variable("7667772301654368511231590734441634728820385355753464717490845307456884280072")
	witness.WtPathElements[0][1] = frontend.Variable("6248971480428681875586872720701903006898164957941161885585232292515327743723")
	witness.WtPathElements[0][2] = frontend.Variable("19842142989411612625912710087914440804720378875685842340101751735648441822748")
	witness.WtPathElements[0][3] = frontend.Variable("20304752715056542815811329636342056767286663221745207495535784435548485430561")
	witness.WtPathElements[0][4] = frontend.Variable("10123741241923738392878827949878021844799105553604491080266583922031272507954")
	witness.WtPathElements[0][5] = frontend.Variable("7763070937946622828545897559398364952701415676360141127081877575944863661924")
	witness.WtPathElements[0][6] = frontend.Variable("4336110172767843874572170947831826800762554433417721132971126558487413457970")
	witness.WtPathElements[0][7] = frontend.Variable("17793516955206672117877101086466671169038346493482525475927531680188516206475")

	witness.WtPathIndices[0]= frontend.Variable(3)
	witness.WtErc1155ContractAddress = frontend.Variable("156785533642590136830176496087798484756809276777")
	witness.WtErc1155TokenId = frontend.Variable(111)

	witness.WtRecipientPk[0] = frontend.Variable("1203252325715224226662903739555242225812271351656")
	witness.WtValuesOut[0] = frontend.Variable(100)

	witness.WtAssetGroupPathElements[0] = frontend.Variable("21786163348386038604960770631540184405176086071424084281943739401806009032442")
	witness.WtAssetGroupPathElements[1] = frontend.Variable("18548196279604799461048000334259916142758522801444715662239164855577399482970")
	witness.WtAssetGroupPathElements[2] = frontend.Variable("6394967702556731931349597360857231577818344086247308420600702098348151392332")
	witness.WtAssetGroupPathElements[3] = frontend.Variable("20304752715056542815811329636342056767286663221745207495535784435548485430561")
	witness.WtAssetGroupPathElements[4] = frontend.Variable("10123741241923738392878827949878021844799105553604491080266583922031272507954")
	witness.WtAssetGroupPathElements[5] = frontend.Variable("7763070937946622828545897559398364952701415676360141127081877575944863661924")
	witness.WtAssetGroupPathElements[6] = frontend.Variable("4336110172767843874572170947831826800762554433417721132971126558487413457970")
	witness.WtAssetGroupPathElements[7] = frontend.Variable("17793516955206672117877101086466671169038346493482525475927531680188516206475")

	witness.WtAssetGroupPathIndices = frontend.Variable(0)

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
	fmt.Println("ERC1155 Fungible Template Proof verified successfully!")


}
