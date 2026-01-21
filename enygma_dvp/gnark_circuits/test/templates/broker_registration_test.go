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



func TestBrokerRegistration(t *testing.T) {
    var circuit templates.BrokerageRegistrationCircuit 

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

	var witness templates.BrokerageRegistrationCircuit

	witness.StBeacon = frontend.Variable("0")
	witness.StVaultId = frontend.Variable("2")
	witness.StGroupId = frontend.Variable("0")
	witness.StDelegatorTreeNumbers[0] = frontend.Variable("0")
	witness.StDelegatorTreeNumbers[1] = frontend.Variable("0")

	witness.StDelegatorMerkleRoots[0] = frontend.Variable("4844474445669402082918858202915210774479254418149984118887758080066762098480")
	witness.StDelegatorMerkleRoots[1] = frontend.Variable("2937123372151872360879522022381511118922418953029608095055153555843800887803")

	witness.StDelegatorNullifier[0] = frontend.Variable("4206425609803042876678364673878180333496412751583765658285542476066515875026")
	witness.StDelegatorNullifier[1] = frontend.Variable("21539490381351331195750648383988428413840170962647703023649566848190199204357")

	witness.StBrokerBlindedPublicKey = frontend.Variable("14458553889829100103595459833246706819507859713492596632149435690433444584058")

	witness.StBrokerMinComissionRate = frontend.Variable(10)
	witness.StBrokerMaxComissionRate = frontend.Variable(15)

	witness.StAssetGroupTreeNumber = 0
	witness.StAssetGroupMerkleRoot = frontend.Variable("6118879879941741538989599691783850906700716830846545002606152231399919231125")

	witness.WtDelegatorPrivatekeys[0] = frontend.Variable("9427082371021091329543592212977823527003194058106323588984726563557760291713")
	witness.WtDelegatorPrivatekeys[1] = frontend.Variable("16170015143311041101322442727463604021191438070878200552356150235788754017583")

	witness.WtDelegatorPathElements[0][0]= frontend.Variable("9826263710371956255866935459894984360550543031791286583292734454193450095920")
	witness.WtDelegatorPathElements[0][1]= frontend.Variable("18548196279604799461048000334259916142758522801444715662239164855577399482970")
	witness.WtDelegatorPathElements[0][2]= frontend.Variable("6394967702556731931349597360857231577818344086247308420600702098348151392332")
	witness.WtDelegatorPathElements[0][3]= frontend.Variable("20304752715056542815811329636342056767286663221745207495535784435548485430561")
	witness.WtDelegatorPathElements[0][4]= frontend.Variable("10123741241923738392878827949878021844799105553604491080266583922031272507954")
	witness.WtDelegatorPathElements[0][5]= frontend.Variable("7763070937946622828545897559398364952701415676360141127081877575944863661924")
	witness.WtDelegatorPathElements[0][6]= frontend.Variable("4336110172767843874572170947831826800762554433417721132971126558487413457970")
	witness.WtDelegatorPathElements[0][7]= frontend.Variable("17793516955206672117877101086466671169038346493482525475927531680188516206475")

	witness.WtDelegatorPathElements[1][0]= frontend.Variable("21786163348386038604960770631540184405176086071424084281943739401806009032442")
	witness.WtDelegatorPathElements[1][1]= frontend.Variable("1460080798237228150690192265611069992426992955104142373261015275061119008069")
	witness.WtDelegatorPathElements[1][2]= frontend.Variable("6394967702556731931349597360857231577818344086247308420600702098348151392332")
	witness.WtDelegatorPathElements[1][3]= frontend.Variable("20304752715056542815811329636342056767286663221745207495535784435548485430561")
	witness.WtDelegatorPathElements[1][4]= frontend.Variable("10123741241923738392878827949878021844799105553604491080266583922031272507954")
	witness.WtDelegatorPathElements[1][5]= frontend.Variable("7763070937946622828545897559398364952701415676360141127081877575944863661924")
	witness.WtDelegatorPathElements[1][6]= frontend.Variable("4336110172767843874572170947831826800762554433417721132971126558487413457970")
	witness.WtDelegatorPathElements[1][7]= frontend.Variable("17793516955206672117877101086466671169038346493482525475927531680188516206475")
	
	witness.WtDelegatorPathIndices[0] = frontend.Variable(1)
	witness.WtDelegatorPathIndices[1] = frontend.Variable(2)
	
	witness.WtDelegatorIdParams[0][0]= frontend.Variable("41799170")
	witness.WtDelegatorIdParams[0][1]= frontend.Variable("3577490623")
	witness.WtDelegatorIdParams[0][2]= frontend.Variable("0")
	witness.WtDelegatorIdParams[0][3]= frontend.Variable("0")
	witness.WtDelegatorIdParams[0][4]= frontend.Variable("0")

	witness.WtDelegatorIdParams[1][0]= frontend.Variable("41799170")
	witness.WtDelegatorIdParams[1][1]= frontend.Variable("3577490623")
	witness.WtDelegatorIdParams[1][2]= frontend.Variable("0")
	witness.WtDelegatorIdParams[1][3]= frontend.Variable("0")
	witness.WtDelegatorIdParams[1][4]= frontend.Variable("0")
	
	witness.WtContractAddress = frontend.Variable("1209419692645727331000468921240830048052204659044")
	witness.WtBrokerPublickey = frontend.Variable("14548903974447260480131321394461357248994237106806304867503755672864475511090")
	
	witness.WtAssetGroupPathIndices = 0

	witness.WtAssetGroupPathElements[0] = frontend.Variable("21786163348386038604960770631540184405176086071424084281943739401806009032442")
	witness.WtAssetGroupPathElements[1] = frontend.Variable("18548196279604799461048000334259916142758522801444715662239164855577399482970")
	witness.WtAssetGroupPathElements[2] = frontend.Variable("6394967702556731931349597360857231577818344086247308420600702098348151392332")
	witness.WtAssetGroupPathElements[3] = frontend.Variable("20304752715056542815811329636342056767286663221745207495535784435548485430561")
	witness.WtAssetGroupPathElements[4] = frontend.Variable("10123741241923738392878827949878021844799105553604491080266583922031272507954")
	witness.WtAssetGroupPathElements[5] = frontend.Variable("7763070937946622828545897559398364952701415676360141127081877575944863661924")
	witness.WtAssetGroupPathElements[6] = frontend.Variable("4336110172767843874572170947831826800762554433417721132971126558487413457970")
	witness.WtAssetGroupPathElements[7] = frontend.Variable("17793516955206672117877101086466671169038346493482525475927531680188516206475")

	
	
	
	
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

	fmt.Println("Broker Registration Circuit - Proof verified successfully!")

}