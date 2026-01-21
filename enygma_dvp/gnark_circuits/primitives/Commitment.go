package primitives 

import(
	"github.com/consensys/gnark/frontend"
	 pos "gnark_server/poseidon"
)

func Commitment(api frontend.API, uniqueId frontend.Variable,publicKey frontend.Variable)frontend.Variable{

	
	commit:= pos.Poseidon(api, []frontend.Variable{uniqueId,publicKey})

	commitout,_ := api.NewHint(ModHint, 2,commit)
	
	commitmentVar:=commitout[0]
	return commitmentVar

}

func CommitmentNative(api frontend.API, uniqueId frontend.Variable,publicKey frontend.Variable)frontend.Variable{

	commit,_:= api.NewHint(PoseidonNative, 1, uniqueId,publicKey)
		
	return commit[0]

}