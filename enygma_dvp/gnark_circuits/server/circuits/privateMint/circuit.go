package privateMint


type PrivateMintRequest struct{
	Commitment         string         `json:"commitment" binding:"required"`
	ContractAddress    string 		  `json:"contractAddress" binding:"required"`
	TokenId		 	   string		  `json:"tokenId" binding:"required"`
	Salt      		   string         `json:"salt" binding:"required"`
	Amount        	   string         `json:"amount" binding:"required"`
	PublicKey         string         	  `json:"publicKey" binding:"required"`
	CipherText       string         	  `json:"cipherText" binding:"required"`

}


type PrivateMintOutput struct{
	Proof 			[]string `json:"proof"`
	PublicSignal    []string `json:"publicSignal"`
}


