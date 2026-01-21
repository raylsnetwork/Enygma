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


func SetupAuctionBid(config templates.AuctionBidCircuitConfig, circuitName string){

	fmt.Print("Initializing Setup Process")
	fmt.Print("\n")
	circuitAuctionBid :=templates.AuctionBidCircuit{
		Config: config,
		StTreeNumber:    			make([]frontend.Variable, config.TmNInputs),
		StMerkleRoot:    			make([]frontend.Variable, config.TmNInputs),
		StNullifier:  				make([]frontend.Variable, config.TmNInputs),
		StCommitmentsOuts:  		make([]frontend.Variable, config.TmNInputs),
		WtPrivateKeysIn: 			make([]frontend.Variable, config.TmNInputs),
		WtPathElements: 			make([][]frontend.Variable, config.TmNInputs),
		WtPathIndices: 				make([]frontend.Variable, config.TmNInputs),
		WtPublicKeysOut:			make([]frontend.Variable, config.TmMOutputs),
		WtAssetGroupPathElements:	make([]frontend.Variable, config.TmDepthMerkle),
		WtIdParamsIn:    			make([][]frontend.Variable, config.TmNInputs),
        WtIdParamsOut:   			make([][]frontend.Variable, config.TmMOutputs),
	}
 	for i := range circuitAuctionBid.WtPathElements {

        circuitAuctionBid.WtPathElements[i] = make([]frontend.Variable, config.TmDepthMerkle)
    }

	for i := range circuitAuctionBid.WtIdParamsIn {

        circuitAuctionBid.WtIdParamsIn[i] = make([]frontend.Variable, config.TmNumOfIdParams)
    }

	for i := range circuitAuctionBid.WtIdParamsOut {

        circuitAuctionBid.WtIdParamsOut[i] = make([]frontend.Variable, config.TmNumOfIdParams)
    }

	printable:= fmt.Sprintf("Generating Proving Key and Veryfing key for %s",circuitName)
	fmt.Println(printable)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitAuctionBid)
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
	

}

func SetupAuctionBidAuditor(config templates.TmAuctionBidAuditorCircuitConfig, circuitName string){
	fmt.Print("Initializing Setup Process")
	fmt.Print("\n")

	plainLength:= config.TmNumOfIdParms * (config.TmInputs+config.TmOutputs) + 4

	decLength:=plainLength
	for decLength%3 != 0 {
		decLength++
	}
	encLength := decLength + 1

	// Auctioneer encryption: 3 values (bidAmount, bidRandom, 0) -> encLength = 4
	auctioneerEncLength := 4

	circuitAuctionBidAuditor := templates.TmAuctionBidAuditorCircuit{
		Config 					 :config,
		StTreeNumbers   		 :make([]frontend.Variable, config.TmInputs),
		StMerkleRoots   		 :make([]frontend.Variable, config.TmInputs),
		StNullifiers		 	 :make([]frontend.Variable, config.TmInputs),
		StCommitmentsOuts	     :make([]frontend.Variable, config.TmOutputs),
		StAuctioneeEncryptedValues: make([]frontend.Variable, auctioneerEncLength),
		StAuditorEncryptedValues : make([]frontend.Variable,encLength),

		WtPrivateKeysIn			 :make([]frontend.Variable, config.TmInputs),
		WtPathElements			 :make([][]frontend.Variable, config.TmInputs),
		WtPathIndices			 :make([]frontend.Variable, config.TmInputs),

		WtPublicKeysOut			  :make([]frontend.Variable, config.TmOutputs),

		WtAssetGroupPathElements :make([]frontend.Variable, config.TmMerkleTreeDepth),

		WtIdParamsIn            :make([][]frontend.Variable, config.TmInputs),
		WtIdParamsOut           :make([][]frontend.Variable, config.TmOutputs),
	}

	for i := range circuitAuctionBidAuditor.WtPathElements {

        circuitAuctionBidAuditor.WtPathElements[i] = make([]frontend.Variable, config.TmMerkleTreeDepth)
    }

	for i := range circuitAuctionBidAuditor.WtIdParamsIn {

        circuitAuctionBidAuditor.WtIdParamsIn[i] = make([]frontend.Variable, config.TmNumOfIdParms)
    }

	for i := range circuitAuctionBidAuditor.WtIdParamsOut {

        circuitAuctionBidAuditor.WtIdParamsOut[i] = make([]frontend.Variable, config.TmNumOfIdParms)
    }
	
	printable:= fmt.Sprintf("Generating Proving Key and Veryfing key for %s",circuitName)
	fmt.Println(printable)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitAuctionBidAuditor)
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

}


func SetupAuctionInit(config templates.AuctionInitCircuitConfig, circuitName string){

	fmt.Print("Initializing Setup Process")
	fmt.Print("\n")
	circuitAuctionInit :=templates.AuctionInitCircuit{
		Config: config,
		WtPathElements:    			make([]frontend.Variable, config.TmMerkleTreeDepth),
		WtIdParams:					make([]frontend.Variable, config.TmNumOfIdParms),
		WtAssetGroupPathElements:   make([]frontend.Variable, config.TmGroupMerkleTreeDepth),
		
	}
 	
	printable:= fmt.Sprintf("Generating Proving Key and Veryfing key for %s",circuitName)
	fmt.Println(printable)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitAuctionInit)
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
	

}

func SetupAuctionInitAuditor(config templates.TmAuctionInitAuditorCircuitConfig, circuitName string){
	fmt.Print("Initializing Setup Process")
	fmt.Print("\n")
	
	plainLength:= config.TmNumOfIdParms  + 1

	decLength:=plainLength
	for decLength%3 != 0 {
		decLength++
	}
	encLength := decLength + 1

	circuitInitAuditor := templates.TmAuctionInitAuditorCircuit{
		Config: config,
		StAuditorEncryptedValues: make([]frontend.Variable, encLength),
		WtPathElements: 		  make([]frontend.Variable, config.TmMerkleTreeDepth),
		WtIdParams:				  make([]frontend.Variable, config.TmNumOfIdParms),
		WtAssetGroupPathElements: make([]frontend.Variable, config.TmAssetGroupMerkleTreeDepth),
	}

	printable:= fmt.Sprintf("Generating Proving Key and Veryfing key for %s",circuitName)
	fmt.Println(printable)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitInitAuditor)
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
}

func SetupAuctionNotWinning(config templates.AuctionNotWinningCircuitConfig, circuitName string){

	fmt.Print("Initializing Setup Process")
	fmt.Print("\n")
	
 	 circuitAuctionNotWinning:=templates.AuctionNotWinningCircuit{
		
	 }
	printable:= fmt.Sprintf("Generating Proving Key and Veryfing key for %s",circuitName)
	fmt.Println(printable)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitAuctionNotWinning)
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
	

}


func SetupAuctionPrivateOpenning(config templates.AuctionPrivateOpeningCircuitConfig, circuitName string){

	fmt.Print("Initializing Setup Process")
	fmt.Print("\n")
	
 	 circuitAuctionNotWinning:=templates.AuctionPrivateOpeningCircuit{
		
	 }
	printable:= fmt.Sprintf("Generating Proving Key and Veryfing key for %s",circuitName)
	fmt.Println(printable)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitAuctionNotWinning)
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
	

}


func SetupBrokerRegistration(config templates.BrokerageRegistrationConfig, circuitName string){
	
	fmt.Print("Initializing Setup Process")
	fmt.Print("\n")
	
 	 circuitBrokerRegistration:=templates.BrokerageRegistrationCircuit{
		Config: config,
		StDelegatorTreeNumbers:   make([]frontend.Variable, config.TmNumOfInputs),
		StDelegatorMerkleRoots:   make([]frontend.Variable, config.TmNumOfInputs),
		StDelegatorNullifier:     make([]frontend.Variable, config.TmNumOfInputs),
		WtDelegatorPrivatekeys:   make([]frontend.Variable, config.TmNumOfInputs),
		WtDelegatorPathElements:  make([][]frontend.Variable, config.TmNumOfInputs),
		WtDelegatorPathIndices:   make([]frontend.Variable, config.TmNumOfInputs),
		WtDelegatorIdParams:	  make([][5]frontend.Variable, config.TmNumOfInputs),
		WtAssetGroupPathElements: make([]frontend.Variable, config.TmGroupMerkleTreeDepth),

	 }

	 for i := range circuitBrokerRegistration.WtDelegatorPathElements {

        circuitBrokerRegistration.WtDelegatorPathElements[i] = make([]frontend.Variable, config.TmMerkleTreeDepth)
    }

	printable:= fmt.Sprintf("Generating Proving Key and Veryfing key for %s",circuitName)
	fmt.Println(printable)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitBrokerRegistration)
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
	

}



func SetupBatchERC1155(config templates.ERC1155NonFungibleCircuitConfig, circuitName string){
	
	fmt.Print("Initializing Setup Process")
	fmt.Print("\n")
	circuitBatchERC1155:=templates.ERC1155NonFungibleCircuit{
		Config: config,
		StTreeNumbers:    			make([]frontend.Variable, config.TmNumOfTokens),
		StMerkleRoots:    			make([]frontend.Variable, config.TmNumOfTokens),
		StNullifiers:  				make([]frontend.Variable, config.TmNumOfTokens),
		StCommitmentOut:  			make([]frontend.Variable, config.TmNumOfTokens),
		StAssetGroupTreeNumber: 	make([]frontend.Variable, config.TmNumOfTokens),
		StAssetGroupMerkleRoot: 	make([]frontend.Variable, config.TmNumOfTokens),
		WtPrivateKeysIn: 			make([]frontend.Variable, config.TmNumOfTokens),
		WtValues: 					make([]frontend.Variable, config.TmNumOfTokens),
		WtPathElements:				make([][]frontend.Variable, config.TmNumOfTokens),
		WtPathIndices:				make([]frontend.Variable, config.TmNumOfTokens),
		WtErc1155TokenId:			make([]frontend.Variable, config.TmNumOfTokens),
		WtPublicKeysOut:    		make([]frontend.Variable, config.TmNumOfTokens),
        WtAssetGroupPathElements:   make([][]frontend.Variable, config.TmNumOfTokens),
		WtAssetGroupPathIndices:    make([]frontend.Variable, config.TmNumOfTokens),
	}

	for i := range circuitBatchERC1155.WtPathElements {

        circuitBatchERC1155.WtPathElements[i] = make([]frontend.Variable, config.TmMerkleTreeDepth)
    }

	for i := range circuitBatchERC1155.WtAssetGroupPathElements {

        circuitBatchERC1155.WtAssetGroupPathElements[i] = make([]frontend.Variable, config.TmAssetGroupMerkleTreeDepth)
    }
	
	printable:= fmt.Sprintf("Generating Proving Key and Veryfing key for %s",circuitName)
	fmt.Println(printable)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitBatchERC1155)
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

}

func SetupBatchErc1155WithAuditor(config templates.ERC1155NonFungibleWithAuditorCircuitConfig, circuitName string){

	fmt.Print("Initializing Setup Process")
	fmt.Print("\n")
	plainLength:= config.TmNumOfTokens*2+1

	decLength:=plainLength
	for decLength%3 != 0 {
		decLength++
	}
	encLength := decLength + 1
	

	circuitBatchERC1155WithAuditor:=templates.ERC1155NonFungibleWithAuditorCircuit{

		Config: 				 config,
		StTreeNumbers: 		     make([]frontend.Variable, config.TmNumOfTokens),
		StMerkleRoots:		     make([]frontend.Variable, config.TmNumOfTokens),     		
		StNullifiers:  			 make([]frontend.Variable, config.TmNumOfTokens),   
		StCommitmentOut:   		 make([]frontend.Variable, config.TmNumOfTokens),   
		
		StAuditorEncryptedValues: make([]frontend.Variable, encLength),
		
		StAssetGroupTreeNumber:  make([]frontend.Variable, config.TmNumOfTokens),
		StAssetGroupMerkleRoot:  make([]frontend.Variable, config.TmNumOfTokens),

		WtPrivateKeysIn:		 make([]frontend.Variable, config.TmNumOfTokens),  
		WtValues:    		 	 make([]frontend.Variable, config.TmNumOfTokens),  
		WtPathElements:   		 make([][]frontend.Variable, config.TmNumOfTokens),
		WtPathIndices:     		 make([]frontend.Variable, config.TmNumOfTokens),
		WtErc1155TokenIds:		 make([]frontend.Variable, config.TmNumOfTokens),
		WtPublicKeysOut: 		 make([]frontend.Variable, config.TmNumOfTokens),

		WtAssetGroupPathElements: make([][]frontend.Variable, config.TmNumOfTokens),
		WtAssetGroupPathIndices: make([]frontend.Variable, config.TmNumOfTokens),
	}


	for i := range circuitBatchERC1155WithAuditor.WtPathElements {

        circuitBatchERC1155WithAuditor.WtPathElements[i] = make([]frontend.Variable, config.TmMerkleTreeDepth)
    }

	for i := range circuitBatchERC1155WithAuditor.WtAssetGroupPathElements {

        circuitBatchERC1155WithAuditor.WtAssetGroupPathElements[i] = make([]frontend.Variable, config.TmAssetGroupMerkleTree)
    }

	printable:= fmt.Sprintf("Generating Proving Key and Veryfing key for %s",circuitName)
	fmt.Println(printable)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitBatchERC1155WithAuditor)
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


}

func SetupJoinSplitERC20(config templates.Erc20CircuitConfig, circuitName string){
	
	fmt.Print("Initializing Setup Process")
	fmt.Print("\n")
	circuitERC20:=templates.Erc20Circuit{
		Config: config,
		StTreeNumber:    				make([]frontend.Variable, config.TmNInputs),
		StMerkleRoots:    				make([]frontend.Variable, config.TmNInputs),
		StNullifiers:  					make([]frontend.Variable, config.TmNInputs),
		StCommitmentOut:  				make([]frontend.Variable, config.TmMOutputs),
		WtPrivateKeysIn: 				make([]frontend.Variable, config.TmNInputs),
		WtValuesIn: 					make([]frontend.Variable, config.TmNInputs),
		WtPathIndices:					make([]frontend.Variable, config.TmNInputs),
		WtPathElements:					make([][]frontend.Variable, config.TmNInputs),
		WtPublicKeysOut:				make([]frontend.Variable, config.TmMOutputs),
		WtValuesOut:    				make([]frontend.Variable, config.TmMOutputs),
    
	}

	for i := range circuitERC20.WtPathElements {

        circuitERC20.WtPathElements[i] = make([]frontend.Variable, config.TmMerkleTreeDepth)
    }

	printable:= fmt.Sprintf("Generating Proving Key and Veryfing key for %s",circuitName)
	fmt.Println(printable)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitERC20)
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

}


func SetupJoinSplitERC20WithAuditor(config templates.Erc20WithAuditorConfig, circuitName string){
	fmt.Print("Initializing Setup Process")
	fmt.Print("\n")

	fmt.Print("Initializing Setup Process")
	fmt.Print("\n")
	
	plainLength:= config.TmNInputs+config.TmMOutputs+1
	decLength:=plainLength
	for decLength%3 != 0 {
		decLength++
	}
	encLength := decLength + 1

	
	circuitERC20WithAuditor:=templates.Erc20WithAuditorCircuit{
		Config: config,
		
		StTreeNumber:    				make([]frontend.Variable, config.TmNInputs),
		StMerkleRoots:    				make([]frontend.Variable, config.TmNInputs),
		StNullifiers:  					make([]frontend.Variable, config.TmNInputs),
		StCommitmentOut:  				make([]frontend.Variable, config.TmMOutputs),
		WtPrivateKeysIn: 				make([]frontend.Variable, config.TmNInputs),
		WtValuesIn: 					make([]frontend.Variable, config.TmNInputs),
		WtPathIndices:					make([]frontend.Variable, config.TmNInputs),
		WtPathElements:					make([][]frontend.Variable, config.TmNInputs),
		WtPublicKeysOut:				make([]frontend.Variable, config.TmMOutputs),
		WtValuesOut:    				make([]frontend.Variable, config.TmMOutputs),

		StAuditorEncryptedValues:		make([]frontend.Variable,encLength ),
    
	}


	for i := range circuitERC20WithAuditor.WtPathElements {

        circuitERC20WithAuditor.WtPathElements[i] = make([]frontend.Variable, config.TmMerkleTreeDepth)
    }

	printable:= fmt.Sprintf("Generating Proving Key and Veryfing key for %s",circuitName)
	fmt.Println(printable)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitERC20WithAuditor)
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

}

func SetupJoinSplitERC20WithBroker(config templates.Erc20BrokerConfig, circuitName string){

	fmt.Print("Initializing Setup Process")
	fmt.Print("\n")
	circuitERC20WithBroker:=templates.Erc20WithBrokerCircuit{
		Config: config,
		StTreeNumber:    			make([]frontend.Variable, config.TmNInputs),
		StMerkleRoots:    			make([]frontend.Variable, config.TmNInputs),
		StNullifiers:  				make([]frontend.Variable, config.TmNInputs),
		StCommitmentOut:  			make([]frontend.Variable, config.TmMOutputs),
	
		WtPrivateKeys: 				make([]frontend.Variable, config.TmNInputs),
		WtPathElements:				make([][]frontend.Variable, config.TmNInputs),
		WtValuesIn:					make([]frontend.Variable, config.TmNInputs),
		WtPathIndices:				make([]frontend.Variable, config.TmNInputs),
		WtPublicKeyOut:    			make([]frontend.Variable, config.TmMOutputs),
		WtValuesOut:    			make([]frontend.Variable, config.TmMOutputs),

		
    
	}

	for i := range circuitERC20WithBroker.WtPathElements {

        circuitERC20WithBroker.WtPathElements[i] = make([]frontend.Variable, config.TmMerkleTree)
    }

	printable:= fmt.Sprintf("Generating Proving Key and Veryfing key for %s",circuitName)
	fmt.Println(printable)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitERC20WithBroker)
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

}


func SetupJoinSplitERC1155(config templates.ERC1155FungibleCircuitConfig, circuitName string){

	fmt.Print("Initializing Setup Process")
	fmt.Print("\n")
	circuitJoinSplitERC1155:=templates.Erc1155FungibleCircuit{
		Config: config,
		StTreeNumbers:    			make([]frontend.Variable, config.TmNInputs),
		StMerkleRoots:    			make([]frontend.Variable, config.TmNInputs),
		StNullifiers:  				make([]frontend.Variable, config.TmNInputs),
		StCommitmentOut:  			make([]frontend.Variable, config.TmMOutputs),
	
		WtPrivateKeysIn: 			make([]frontend.Variable, config.TmNInputs),
		WtValuesIn:					make([]frontend.Variable, config.TmNInputs),
		WtPathElements:				make([][]frontend.Variable, config.TmNInputs),
		
		WtPathIndices:				make([]frontend.Variable, config.TmNInputs),
		WtPublicKeysOut:    		make([]frontend.Variable, config.TmMOutputs),
		WtValuesOut:    			make([]frontend.Variable, config.TmMOutputs),
		WtAssetGroupPathElements: 	make([]frontend.Variable, config.TmAssetGroupMerkleTree),
	}

	for i := range circuitJoinSplitERC1155.WtPathElements {

        circuitJoinSplitERC1155.WtPathElements[i] = make([]frontend.Variable, config.TmMerkleTreeDepth)
    }

	printable:= fmt.Sprintf("Generating Proving Key and Veryfing key for %s",circuitName)
	fmt.Println(printable)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitJoinSplitERC1155)
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

}


func SetupJoinSplitERC1155Auditor(config templates.ERC1155FungibleWithAuditorCircuitConfig, circuitName string){
	fmt.Print("Initializing Setup Process")
	fmt.Print("\n")

	plainLength:= config.TmNInputs  + config.TmMOutputs+2

	decLength:=plainLength
	for decLength%3 != 0 {
		decLength++
	}
	encLength := decLength + 1

	circuitJoinSplitERC1155Auditor:=templates.Erc1155FungibleWithAuditorCircuit{
		Config: config,
		StTreeNumbers:    			make([]frontend.Variable, config.TmNInputs),
		StMerkleRoots:    			make([]frontend.Variable, config.TmNInputs),
		StNullifiers:  				make([]frontend.Variable, config.TmNInputs),
		StCommitmentOut:  			make([]frontend.Variable, config.TmMOutputs),
	
		WtPrivateKeysIn: 			make([]frontend.Variable, config.TmNInputs),
		WtValuesIn:					make([]frontend.Variable, config.TmNInputs),
		WtPathElements:				make([][]frontend.Variable, config.TmNInputs),
		WtPathIndices:				make([]frontend.Variable, config.TmNInputs),

		StAuditorEncryptedValues:   make([]frontend.Variable,encLength),
		
		WtPublicKeysOut:    		make([]frontend.Variable, config.TmMOutputs),
		WtValuesOut:    			make([]frontend.Variable, config.TmMOutputs),
		WtAssetGroupPathElements: 	make([]frontend.Variable, config.TmAssetGroupMerkleTree),
	}

	for i := range circuitJoinSplitERC1155Auditor.WtPathElements {

        circuitJoinSplitERC1155Auditor.WtPathElements[i] = make([]frontend.Variable, config.TmMerkleTreeDepth)
    }

	printable:= fmt.Sprintf("Generating Proving Key and Veryfing key for %s",circuitName)
	fmt.Println(printable)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitJoinSplitERC1155Auditor)
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


}

func SetupJoinSplitERC1155WithBroker(config templates.ERC1155FungibleWithBrokerCircuitConfig, circuitName string){

	fmt.Print("Initializing Setup Process")
	fmt.Print("\n")
	circuitJoinSplitERC1155WithBroker:=templates.Erc1155FungibleWithBrokerCircuit{
		Config: config,
		StTreeNumbers:    			make([]frontend.Variable, config.NInputs),
		StMerkleRoots:    			make([]frontend.Variable, config.NInputs),
		StNullifiers:  				make([]frontend.Variable, config.NInputs),
		StCommitmentOut:  			make([]frontend.Variable, config.MOutputs),
	
		WtPrivateKeys: 				make([]frontend.Variable, config.NInputs),
		WtValuesIn:					make([]frontend.Variable, config.NInputs),
		WtPathElements:				make([][]frontend.Variable, config.NInputs),
		
		WtPathIndices:				make([]frontend.Variable, config.NInputs),
		WtRecipientPk:    			make([]frontend.Variable, config.MOutputs),
		WtValuesOut:    			make([]frontend.Variable, config.MOutputs),
		WtAssetGroupPathElements: 	make([]frontend.Variable, config.AssetGroupMerkleTreeDepth),
	}

	for i := range circuitJoinSplitERC1155WithBroker.WtPathElements {

        circuitJoinSplitERC1155WithBroker.WtPathElements[i] = make([]frontend.Variable, config.MerkleTreeDepth)
    }

	printable:= fmt.Sprintf("Generating Proving Key and Veryfing key for %s",circuitName)
	fmt.Println(printable)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitJoinSplitERC1155WithBroker)
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


}


func SetupLegitBroker(config templates.LegitBrokerCircuitConfig, circuitName string){
	fmt.Print("Initializing Setup Process")
	fmt.Print("\n")
	circuitLegitBroker:=templates.LegitBrokerCircuit{
		
	}

	printable:= fmt.Sprintf("Generating Proving Key and Veryfing key for %s",circuitName)
	fmt.Println(printable)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitLegitBroker)
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

}

func SetupOwnershipERC721(config templates.Erc721CircuitConfig, circuitName string){
	fmt.Print("Initializing Setup Process")
	fmt.Print("\n")
	circuitOwnershipERC721:=templates.Erc721Circuit{
		Config: config,
		StTreeNumbers:			    make([]frontend.Variable,config.TmNumOfTokens),
		StNullifiers:				make([]frontend.Variable, config.TmNumOfTokens),
		StMerkleRoots:				make([]frontend.Variable, config.TmNumOfTokens),
		StCommitmentOut:			make([]frontend.Variable, config.TmNumOfTokens),
		WtPrivateKeysIn:			make([]frontend.Variable, config.TmNumOfTokens),
		WtValues:					make([]frontend.Variable, config.TmNumOfTokens),
		
		WtPathElements:             make([][]frontend.Variable, config.TmNumOfTokens),
		WtPathIndices:				make([]frontend.Variable, config.TmNumOfTokens),
		WtPublicKeysOut:			make([]frontend.Variable, config.TmNumOfTokens),
		
	}
	for i := range circuitOwnershipERC721.WtPathElements {

        circuitOwnershipERC721.WtPathElements[i] = make([]frontend.Variable, config.TmMerkleTreeDepth)
    }
	printable:= fmt.Sprintf("Generating Proving Key and Veryfing key for %s",circuitName)
	fmt.Println(printable)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitOwnershipERC721)
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
}

func SetupOwnershipERC721Auditor(config templates.Erc721WithAuditorCircuitConfig, circuitName string){
	fmt.Print("Initializing Setup Process")
	fmt.Print("\n")

	plainLength:= config.TmNumOfTokens

	decLength:=plainLength
	for decLength%3 != 0 {
		decLength++
	}
	encLength := decLength + 1


	circuitOwnershipERC721:=templates.Erc721WithAuditorCircuit{
		Config: config,
		StTreeNumbers:			    make([]frontend.Variable,config.TmNumOfTokens),
		StNullifiers:				make([]frontend.Variable, config.TmNumOfTokens),
		StMerkleRoots:				make([]frontend.Variable, config.TmNumOfTokens),
		StCommitmentOut:			make([]frontend.Variable, config.TmNumOfTokens),
		WtPrivateKeysIn:			make([]frontend.Variable, config.TmNumOfTokens),
		WtValues:					make([]frontend.Variable, config.TmNumOfTokens),
		StAuditorEncryptedValues:   make([]frontend.Variable,encLength),
		WtPathElements:             make([][]frontend.Variable, config.TmNumOfTokens),
		WtPathIndices:				make([]frontend.Variable, config.TmNumOfTokens),
		WtPrivateKeysOut:			make([]frontend.Variable, config.TmNumOfTokens),
		
	}
	for i := range circuitOwnershipERC721.WtPathElements {

        circuitOwnershipERC721.WtPathElements[i] = make([]frontend.Variable, config.TmMerkleTreeDepth)
    }
	printable:= fmt.Sprintf("Generating Proving Key and Veryfing key for %s",circuitName)
	fmt.Println(printable)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitOwnershipERC721)
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
}

func SetupOwnershipERC1155Fungible(config templates.ERC1155FungibleCircuitConfig, circuitName string){
	fmt.Print("Initializing Setup Process")
	fmt.Print("\n")
	circuitOwnershipERC1155fFungible:=templates.Erc1155FungibleCircuit{
		Config: config,
		StTreeNumbers:					make([]frontend.Variable, config.TmNInputs),
		StMerkleRoots:					make([]frontend.Variable, config.TmNInputs),
		StNullifiers:					make([]frontend.Variable, config.TmNInputs),
		StCommitmentOut:				make([]frontend.Variable, config.TmMOutputs),
		
		WtPrivateKeysIn:             		make([]frontend.Variable, config.TmNInputs),
		WtValuesIn:						make([]frontend.Variable, config.TmNInputs),
		WtPathElements:					make([][]frontend.Variable, config.TmNInputs),
		WtPathIndices:					make([]frontend.Variable, config.TmNInputs),

		WtPublicKeysOut: 				make([]frontend.Variable, config.TmMOutputs),
		WtValuesOut: 				make([]frontend.Variable, config.TmMOutputs),
		WtAssetGroupPathElements: 	make([]frontend.Variable, config.TmAssetGroupMerkleTree),

	}

	for i := range circuitOwnershipERC1155fFungible.WtPathElements {

        circuitOwnershipERC1155fFungible.WtPathElements[i] = make([]frontend.Variable, config.TmMerkleTreeDepth)
    }

	printable:= fmt.Sprintf("Generating Proving Key and Veryfing key for %s",circuitName)
	fmt.Println(printable)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitOwnershipERC1155fFungible)
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
}

func SetupOwnershipERC1155NonFungibleAuditor(config templates.ERC1155NonFungibleWithAuditorCircuitConfig, circuitName string){
	fmt.Print("Initializing Setup Process")
	fmt.Print("\n")
	plainLength:= config.TmNumOfTokens*2+1

	decLength:=plainLength
	for decLength%3 != 0 {
		decLength++
	}
	encLength := decLength + 1


	circuitOwnershipERC1155NonFungible:=templates.ERC1155NonFungibleWithAuditorCircuit{
		Config: config,
		StTreeNumbers:					make([]frontend.Variable, config.TmNumOfTokens),
		StMerkleRoots:					make([]frontend.Variable, config.TmNumOfTokens),
		StNullifiers:					make([]frontend.Variable, config.TmNumOfTokens),
		StCommitmentOut:				make([]frontend.Variable, config.TmNumOfTokens),
		StAssetGroupTreeNumber: 		make([]frontend.Variable, config.TmNumOfTokens),
		StAssetGroupMerkleRoot: 		make([]frontend.Variable, config.TmNumOfTokens),
		
		WtPrivateKeysIn:             		make([]frontend.Variable, config.TmNumOfTokens),
		WtValues:						make([]frontend.Variable, config.TmNumOfTokens),
		WtPathElements:					make([][]frontend.Variable, config.TmNumOfTokens),
		WtPathIndices:					make([]frontend.Variable, config.TmNumOfTokens),
		WtErc1155TokenIds:				make([]frontend.Variable, config.TmNumOfTokens),
		WtPublicKeysOut:				make([]frontend.Variable, config.TmNumOfTokens),
		
		WtAssetGroupPathElements: 		make([][]frontend.Variable, config.TmNumOfTokens),
		WtAssetGroupPathIndices: 		make([]frontend.Variable, config.TmNumOfTokens),

		StAuditorEncryptedValues:       make([]frontend.Variable,encLength),
	}	


	for i := range circuitOwnershipERC1155NonFungible.WtPathElements {

        circuitOwnershipERC1155NonFungible.WtPathElements[i] = make([]frontend.Variable, config.TmMerkleTreeDepth)
    }
	for i := range circuitOwnershipERC1155NonFungible.WtAssetGroupPathElements {

        circuitOwnershipERC1155NonFungible.WtAssetGroupPathElements[i] = make([]frontend.Variable, config.TmAssetGroupMerkleTree)
    }

	printable:= fmt.Sprintf("Generating Proving Key and Veryfing key for %s",circuitName)
	fmt.Println(printable)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitOwnershipERC1155NonFungible)
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
}


func SetupOwnershipERC1155NonFungible(config templates.ERC1155NonFungibleCircuitConfig, circuitName string){
	fmt.Print("Initializing Setup Process")
	fmt.Print("\n")
	
	
	
	circuitOwnershipERC1155NonFungible:=templates.ERC1155NonFungibleCircuit{
		Config: config,
		StTreeNumbers:					make([]frontend.Variable, config.TmNumOfTokens),
		StMerkleRoots:					make([]frontend.Variable, config.TmNumOfTokens),
		StNullifiers:					make([]frontend.Variable, config.TmNumOfTokens),
		StCommitmentOut:				make([]frontend.Variable, config.TmNumOfTokens),
		StAssetGroupTreeNumber: 		make([]frontend.Variable, config.TmNumOfTokens),
		StAssetGroupMerkleRoot: 		make([]frontend.Variable, config.TmNumOfTokens),
		
		WtPrivateKeysIn:             	make([]frontend.Variable, config.TmNumOfTokens),
		WtValues:						make([]frontend.Variable, config.TmNumOfTokens),
		WtPathElements:					make([][]frontend.Variable, config.TmNumOfTokens),
		WtPathIndices:					make([]frontend.Variable, config.TmNumOfTokens),
		WtErc1155TokenId:				make([]frontend.Variable, config.TmNumOfTokens),
		WtPublicKeysOut:				make([]frontend.Variable, config.TmNumOfTokens),
		
		WtAssetGroupPathElements: 		make([][]frontend.Variable, config.TmNumOfTokens),
		WtAssetGroupPathIndices: 		make([]frontend.Variable, config.TmNumOfTokens),

	}


	for i := range circuitOwnershipERC1155NonFungible.WtPathElements {

        circuitOwnershipERC1155NonFungible.WtPathElements[i] = make([]frontend.Variable, config.TmMerkleTreeDepth)
    }
	for i := range circuitOwnershipERC1155NonFungible.WtAssetGroupPathElements {

        circuitOwnershipERC1155NonFungible.WtAssetGroupPathElements[i] = make([]frontend.Variable, config.TmAssetGroupMerkleTreeDepth)
    }

	printable:= fmt.Sprintf("Generating Proving Key and Veryfing key for %s",circuitName)
	fmt.Println(printable)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitOwnershipERC1155NonFungible)
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
}


func SetupPrivateMint(config templates.PrivateMintConfig, circuitName string){
	fmt.Print("Initializing Setup Process")
	fmt.Print("\n")
	circuitLegitBroker:=templates.PrivateMintCircuit{
		
	}

	printable:= fmt.Sprintf("Generating Proving Key and Veryfing key for %s",circuitName)
	fmt.Println(printable)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitLegitBroker)
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