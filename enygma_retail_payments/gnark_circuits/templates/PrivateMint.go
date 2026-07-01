package templates

import(
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/cmp"
	"gnark_server/primitives"
	pos "gnark_server/poseidon"
)



type PrivateMintConfig struct{
	TmRange frontend.Variable
}

type PrivateMintCircuit struct {
	Config PrivateMintConfig `gnark:"-"`

	// --- public inputs (statement) ---
	Commitment      frontend.Variable `gnark:",public"` // Poseidon(pk_spend, salt, amount, tokenId) — inserted into the Merkle tree on-chain
	ContractAddress frontend.Variable `gnark:",public"` // address of the EnygmaDvp contract — binds the proof to a specific deployment
	TokenId         frontend.Variable `gnark:",public"` // ERC20 token being minted
	CipherText      frontend.Variable `gnark:",public"` // Poseidon(pk_spend, salt, contractAddress) — note tag bound to this deployment

	// --- private witnesses ---
	Salt      frontend.Variable // saltB — random blinding factor for the commitment and note tag
	Amount    frontend.Variable // amount of tokens being privately minted
	PublicKey frontend.Variable // pk_spend of the recipient (Alice's spend public key)
}


func (circuit *PrivateMintCircuit) Define(api frontend.API) error{

	
	api.AssertIsEqual(cmp.IsLess(api, circuit.Amount, circuit.Config.TmRange), 1)
    api.AssertIsEqual(cmp.IsLessOrEqual(api, 0, circuit.Amount), 1)
	
	calculatedCommitment := primitives.Erc20CommitmentV2(api, circuit.PublicKey, circuit.Salt, circuit.Amount, circuit.TokenId)
	api.AssertIsEqual(calculatedCommitment, circuit.Commitment)

	
	calculatedCipherText := pos.Poseidon(api, []frontend.Variable{circuit.PublicKey, circuit.Salt, circuit.ContractAddress})
	api.AssertIsEqual(calculatedCipherText, circuit.CipherText)

	return nil
}
