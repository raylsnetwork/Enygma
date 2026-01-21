package primitives 

import(
	"github.com/consensys/gnark/frontend"
	 pos "gnark_server/poseidon"
)

func Erc1155UniqueId(api frontend.API, erc1155ContractAddress frontend.Variable,erc1155TokenId frontend.Variable,amount frontend.Variable)frontend.Variable{

	hasher1:= pos.Poseidon(api, []frontend.Variable{erc1155ContractAddress,erc1155TokenId})	
	hasherout1,_ := api.NewHint(ModHint, 2,hasher1)

	hasher2 := pos.Poseidon(api, []frontend.Variable{frontend.Variable(hasherout1[0]),amount})
	hasherout2,_ := api.NewHint(ModHint, 2,hasher2)

	return hasherout2[0]
}

func Erc1155UniqueId2(api frontend.API, erc1155ContractAddress frontend.Variable,erc1155TokenId frontend.Variable,amount frontend.Variable)frontend.Variable{

	Id,_ := api.NewHint(ERC155UniqueIdNative, 1, erc1155ContractAddress,erc1155TokenId,amount)


	return Id[0]
}