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
	
	
	var circuit templates.Erc1155FungibleWithBrokerCircuit
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

	var witness templates.Erc1155FungibleWithBrokerCircuit
	witness.StMessage = frontend.Variable("14224204531633748113239524117745463080781248824731809551264901514107490038299")
	
	witness.StTreeNumbers[0] = frontend.Variable("0")
	witness.StTreeNumbers[1] = frontend.Variable("0")

	witness.StMerkleRoots[0] =frontend.Variable("17041743510802630780807153352717945988088686134670808027156671359003479011893")
	witness.StMerkleRoots[1] =frontend.Variable("2840646995331295496538901865493626612403575032208215835748677716605669345963")

	witness.StNullifiers[0] = frontend.Variable("20036356465621671022558827772261213646374461834687890691044028157812474019270")
	witness.StNullifiers[1] = frontend.Variable("21570489458024801665647543602262131370307380768027379604187023529758617210909")

	witness.StCommitmentOut[0] = frontend.Variable("9331768188683508734827095841947239603283711501934577599013787778437423183818")
	witness.StCommitmentOut[1] = frontend.Variable("2227480536213927800575618578983222326349865592352403744663507364688475160580")
	witness.StCommitmentOut[2] = frontend.Variable("7420888091092721160596360673490298440004990732331499941582319282242141989512")

	witness.StBrokerBlindedPublicKey = frontend.Variable("780768083355181460873287065555761908957469797924822923322029106513795805308")
	witness.StBrokerCommisionRate = frontend.Variable("5")
	witness.StAssetGroupTreeNumber = frontend.Variable("0")
	witness.StAssetGroupMerkleRoot =  frontend.Variable("17681840246716908734304101562283117497374825045602246162429417215588297189127")

	witness.WtPrivateKeys[0] = frontend.Variable("17504077082868048612278727674764106842746337475464993426518817894454158338557")
	witness.WtPrivateKeys[1] = frontend.Variable("14096809894907481794134823610503509911664903512788441211581709559050195584153")

	witness.WtValuesIn[0] = frontend.Variable("2556395178")
	witness.WtValuesIn[1] = frontend.Variable("2556395178")

	witness.WtPathElements[0][0] =frontend.Variable("15331289870862493622120787487199514098148927430786397392452253726934703087341")
	witness.WtPathElements[0][1] =frontend.Variable("18548196279604799461048000334259916142758522801444715662239164855577399482970")
	witness.WtPathElements[0][2] =frontend.Variable("6394967702556731931349597360857231577818344086247308420600702098348151392332")
	witness.WtPathElements[0][3] =frontend.Variable("20304752715056542815811329636342056767286663221745207495535784435548485430561")
	witness.WtPathElements[0][4] =frontend.Variable("10123741241923738392878827949878021844799105553604491080266583922031272507954")
	witness.WtPathElements[0][5] =frontend.Variable("7763070937946622828545897559398364952701415676360141127081877575944863661924")
	witness.WtPathElements[0][6] =frontend.Variable("4336110172767843874572170947831826800762554433417721132971126558487413457970")
	witness.WtPathElements[0][7] =frontend.Variable("17793516955206672117877101086466671169038346493482525475927531680188516206475")

	witness.WtPathElements[1][0] =frontend.Variable("21786163348386038604960770631540184405176086071424084281943739401806009032442")
	witness.WtPathElements[1][1] =frontend.Variable("13655835165160433502924211944877349426779538173842426987443293960541355445354")
	witness.WtPathElements[1][2] =frontend.Variable("6394967702556731931349597360857231577818344086247308420600702098348151392332")
	witness.WtPathElements[1][3] =frontend.Variable("20304752715056542815811329636342056767286663221745207495535784435548485430561")
	witness.WtPathElements[1][4] =frontend.Variable("10123741241923738392878827949878021844799105553604491080266583922031272507954")
	witness.WtPathElements[1][5] =frontend.Variable("7763070937946622828545897559398364952701415676360141127081877575944863661924")
	witness.WtPathElements[1][6] =frontend.Variable("4336110172767843874572170947831826800762554433417721132971126558487413457970")
	witness.WtPathElements[1][7] =frontend.Variable("17793516955206672117877101086466671169038346493482525475927531680188516206475")

	witness.WtPathIndices[0] = frontend.Variable(1)
	witness.WtPathIndices[1] = frontend.Variable(2)

	witness.WtErc1155ContractAddress = frontend.Variable("1079907384650745142447910825889693660297716822884")
	witness.WtErc1155TokenId = frontend.Variable("4056762596")

	witness.WtRecipientPk[0] = frontend.Variable("5014914988458975766248955872934842959817597703346395130498362191901064416483")
	witness.WtRecipientPk[1] = frontend.Variable("7764857453038298708337351713853659510596467523505427596026559600179900159486")
	witness.WtRecipientPk[2] = frontend.Variable("1102937444340507149501478111631055580692804912800779201013079306297978294619")

	witness.WtValuesOut[0] = frontend.Variable("100")
	witness.WtValuesOut[1] = frontend.Variable("5112790251")
	witness.WtValuesOut[2] = frontend.Variable("5")

	witness.WtAssetGroupPathElements[0] = frontend.Variable("21786163348386038604960770631540184405176086071424084281943739401806009032442")
	witness.WtAssetGroupPathElements[1] = frontend.Variable("18548196279604799461048000334259916142758522801444715662239164855577399482970")
	witness.WtAssetGroupPathElements[2] = frontend.Variable("6394967702556731931349597360857231577818344086247308420600702098348151392332")
	witness.WtAssetGroupPathElements[3] = frontend.Variable("20304752715056542815811329636342056767286663221745207495535784435548485430561")
	witness.WtAssetGroupPathElements[4] = frontend.Variable("10123741241923738392878827949878021844799105553604491080266583922031272507954")
	witness.WtAssetGroupPathElements[5] = frontend.Variable("7763070937946622828545897559398364952701415676360141127081877575944863661924")
	witness.WtAssetGroupPathElements[6] = frontend.Variable("4336110172767843874572170947831826800762554433417721132971126558487413457970")
	witness.WtAssetGroupPathElements[7] = frontend.Variable("17793516955206672117877101086466671169038346493482525475927531680188516206475")

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

	fmt.Println("ERC1155 Fungible with Broker Circuit - Proof verified successfully!")


}