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

func TestAuctionBid(t *testing.T) {
	var circuit templates.AuctionBidCircuit 

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

	var witness templates.AuctionBidCircuit
	witness.StAuctionId= frontend.Variable("2")
	witness.StBlindedBid = frontend.Variable("13938781916021849685930694519753619193386545311916835371959921258369505045219")
	witness.StVaultId = frontend.Variable(2)

	witness.StMerkleRoot[0] = frontend.Variable("14310674493864022265829313778933834999702450930001666568497311098884452449428")
	witness.StMerkleRoot[1] = frontend.Variable("16194567313546770126186274166276494330236476698432548304990089015550965414840")

	witness.StNullifier[0] = frontend.Variable("1409303702192228628191321225690934166891765188528230257537690755934887060013")
	witness.StNullifier[1] = frontend.Variable("13828406772296960317800834249004647600438194532372248044721318948880379678548")

	witness.StTreeNumber[0] = 0
	witness.StTreeNumber[1] = 0 

	witness.StCommitmentsOuts[0] = frontend.Variable("17126509831822827654967316299075628069408666615285747490756981669262034522075")
	witness.StCommitmentsOuts[1] = frontend.Variable("2113666042252201701571238878782567195510583100450293751147164976831931296356")

	witness.StAssetGroupMerkleRoot = frontend.Variable("15897691944733955197825233234057069730619987108623431897804310311285544863326")

	witness.WtBidAmount = 3451394
	witness.WtBidRandom = frontend.Variable("144094844587005612452027919768918333013")

	witness.WtPrivateKeys[0] = frontend.Variable("11149310982444166737893114186140640894114323591469490224677341038575754329142")
	witness.WtPrivateKeys[1] = frontend.Variable("4735560674998652638198902691178142414008102609783103003775253872585243416152")

	witness.WtValuesIn[0] = 1887665
	witness.WtValuesIn[1] = 1887665

	witness.WtPathElements[0][0] = frontend.Variable("3259869345957070909000408826385578070231059233388388996195005613669298855503")
	witness.WtPathElements[0][1] = frontend.Variable("17672200604108779160026896084352749017573902697409584610000656563318641924588")
	witness.WtPathElements[0][2] = frontend.Variable("6394967702556731931349597360857231577818344086247308420600702098348151392332")
	witness.WtPathElements[0][3] = frontend.Variable("20304752715056542815811329636342056767286663221745207495535784435548485430561")
	witness.WtPathElements[0][4] = frontend.Variable("10123741241923738392878827949878021844799105553604491080266583922031272507954")
	witness.WtPathElements[0][5] = frontend.Variable("7763070937946622828545897559398364952701415676360141127081877575944863661924")
	witness.WtPathElements[0][6] = frontend.Variable("4336110172767843874572170947831826800762554433417721132971126558487413457970")
	witness.WtPathElements[0][7] = frontend.Variable("17793516955206672117877101086466671169038346493482525475927531680188516206475")

	witness.WtPathElements[1][0] = frontend.Variable("21786163348386038604960770631540184405176086071424084281943739401806009032442")
	witness.WtPathElements[1][1] = frontend.Variable("18548196279604799461048000334259916142758522801444715662239164855577399482970")
	witness.WtPathElements[1][2] = frontend.Variable("12885933609056696298216482078554174466465889262661674792819405819876420746858")
	witness.WtPathElements[1][3] = frontend.Variable("20304752715056542815811329636342056767286663221745207495535784435548485430561")
	witness.WtPathElements[1][4] = frontend.Variable("10123741241923738392878827949878021844799105553604491080266583922031272507954")
	witness.WtPathElements[1][5] = frontend.Variable("7763070937946622828545897559398364952701415676360141127081877575944863661924")
	witness.WtPathElements[1][6] = frontend.Variable("4336110172767843874572170947831826800762554433417721132971126558487413457970")
	witness.WtPathElements[1][7] = frontend.Variable("17793516955206672117877101086466671169038346493482525475927531680188516206475")

	witness.WtPathIndices[0] = 3
	witness.WtPathIndices[1] = 4

	witness.WtContractAddress = frontend.Variable("829047437456771662439155153226700331605172030834")

	witness.WtRecipientPK[0] = frontend.Variable("8323164986175492715058413183752555089302808616238321257552168632933024066734")
	witness.WtRecipientPK[1] = frontend.Variable("10176086556679386529835807335560934840814924152025527128558348523910415409491")
	
	witness.WtValuesOut[0] = 3451394
	witness.WtValuesOut[1] = 323936

	witness.WtAssetGroupPathElements[0] = frontend.Variable("21786163348386038604960770631540184405176086071424084281943739401806009032442")
	witness.WtAssetGroupPathElements[1] = frontend.Variable("18548196279604799461048000334259916142758522801444715662239164855577399482970")
	witness.WtAssetGroupPathElements[2] = frontend.Variable("6394967702556731931349597360857231577818344086247308420600702098348151392332")
	witness.WtAssetGroupPathElements[3] = frontend.Variable("20304752715056542815811329636342056767286663221745207495535784435548485430561")
	witness.WtAssetGroupPathElements[4] = frontend.Variable("10123741241923738392878827949878021844799105553604491080266583922031272507954")
	witness.WtAssetGroupPathElements[5] = frontend.Variable("7763070937946622828545897559398364952701415676360141127081877575944863661924")
	witness.WtAssetGroupPathElements[6] = frontend.Variable("4336110172767843874572170947831826800762554433417721132971126558487413457970")
	witness.WtAssetGroupPathElements[7] = frontend.Variable("17793516955206672117877101086466671169038346493482525475927531680188516206475")

	witness.WtAssetGroupPathIndices = 0

	witness.WtIdParamsIn[0][0] = frontend.Variable("1887665")
	witness.WtIdParamsIn[0][1] = frontend.Variable("166969940")
	witness.WtIdParamsIn[0][2] = frontend.Variable("0")
	witness.WtIdParamsIn[0][3] = frontend.Variable("0")
	witness.WtIdParamsIn[0][4] = frontend.Variable("0")
	
	witness.WtIdParamsIn[1][0] = frontend.Variable("1887665")
	witness.WtIdParamsIn[1][1] = frontend.Variable("1887665")
	witness.WtIdParamsIn[1][2] = frontend.Variable("0")
	witness.WtIdParamsIn[1][3] = frontend.Variable("0")
	witness.WtIdParamsIn[1][4] = frontend.Variable("0")
	
	witness.WtIdParamsOut[0][0] = frontend.Variable("3451394")
	witness.WtIdParamsOut[0][1] = frontend.Variable("166969940")
	witness.WtIdParamsOut[0][2] = frontend.Variable("0")
	witness.WtIdParamsOut[0][3] = frontend.Variable("0")
	witness.WtIdParamsOut[0][4] = frontend.Variable("0")

	witness.WtIdParamsOut[1][0] = frontend.Variable("323936")
	witness.WtIdParamsOut[1][1] = frontend.Variable("166969940")
	witness.WtIdParamsOut[1][2] = frontend.Variable("0")
	witness.WtIdParamsOut[1][3] = frontend.Variable("0")
	witness.WtIdParamsOut[1][4] = frontend.Variable("0")



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

	fmt.Println("Proof verified successfully for Auction Bid Circuit")
	
}