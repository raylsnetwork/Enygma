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

func TestAuctionInit(t *testing.T) {
	var circuit templates.AuctionInitCircuit 

	solver.RegisterHint(primitives.ModHint)
	solver.RegisterHint(primitives.ERC155UniqueIdNative)
	solver.RegisterHint(primitives.PoseidonNative)

	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		panic(err)
	}

	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		panic(err)
	}

	var witness templates.AuctionInitCircuit
	witness.StBeacon = frontend.Variable("0")
	witness.StVaultId = frontend.Variable("1")
	witness.StAuctionId = frontend.Variable("16949286317333253226178714925308080471851303629291852942469418296253173155697")
	witness.StTreeNumber = frontend.Variable("0")
	witness.StMerkleRoot = frontend.Variable("12159539653266330931103315208938176844283783006595653895786772864831887770137")
	witness.StNullifier  = frontend.Variable("1276797570318967136699101867816099897464745939570589635933577573952073214593")

	witness.StAssetGroupMerkleRoot = frontend.Variable("0")

	witness.WtCommiment = frontend.Variable("20472840007097634780776593046166761579609736031070593938018800946486941794105")
	witness.WtPathElements[0] = frontend.Variable("21786163348386038604960770631540184405176086071424084281943739401806009032442")
	witness.WtPathElements[1] = frontend.Variable("18548196279604799461048000334259916142758522801444715662239164855577399482970")
	witness.WtPathElements[2] = frontend.Variable("6394967702556731931349597360857231577818344086247308420600702098348151392332")
	witness.WtPathElements[3] = frontend.Variable("20304752715056542815811329636342056767286663221745207495535784435548485430561")
	witness.WtPathElements[4] = frontend.Variable("10123741241923738392878827949878021844799105553604491080266583922031272507954")
	witness.WtPathElements[5] = frontend.Variable("7763070937946622828545897559398364952701415676360141127081877575944863661924")
	witness.WtPathElements[6] = frontend.Variable("4336110172767843874572170947831826800762554433417721132971126558487413457970")
	witness.WtPathElements[7] = frontend.Variable("17793516955206672117877101086466671169038346493482525475927531680188516206475")

	witness.WtPathIndices = frontend.Variable("0")
	witness.WtPrivateKey =  frontend.Variable("19048240736507818962407655404967652069964319187399403651307722600173678778088")

	witness.WtIdParams[0] = frontend.Variable("1201972920")
	witness.WtIdParams[1] = frontend.Variable("0")
	witness.WtIdParams[2] = frontend.Variable("0")
	witness.WtIdParams[3] = frontend.Variable("0")
	witness.WtIdParams[4] = frontend.Variable("0")

	witness.WtContractAddress = frontend.Variable("289377766343063011879541954907895547597630040962")

	witness.WtAssetGroupPathElements[0] = frontend.Variable("0")
	witness.WtAssetGroupPathElements[1] = frontend.Variable("0")
	witness.WtAssetGroupPathElements[2] = frontend.Variable("0")
	witness.WtAssetGroupPathElements[3] = frontend.Variable("0")
	witness.WtAssetGroupPathElements[4] = frontend.Variable("0")
	witness.WtAssetGroupPathElements[5] = frontend.Variable("0")
	witness.WtAssetGroupPathElements[6] = frontend.Variable("0")
	witness.WtAssetGroupPathElements[7] = frontend.Variable("0")

	witness.WtAssetGroupPathIndices = frontend.Variable("0")



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

	fmt.Println("Proof verified successfully!Auction Init Test")

}