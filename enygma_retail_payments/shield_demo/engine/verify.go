package engine

import (
	"encoding/json"
	"fmt"
	"math/big"

	"enygma_retail_payments/layout"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// PaymentVKSlot is the Verifier slot of the Payment circuit's verifying
// key: init registers it first, and Erc20CoinVault checks a 1-in Payment
// proof against it (VK_ID_ERC20_JOINSPLIT).
const PaymentVKSlot = 0

// VerifyProofField is the protocol field a walkthrough sends once payment 1
// settles: a VerifyCall, as JSON, that checks its proof on chain.
const VerifyProofField = "verifyProof"

// Verifier is the on-chain verifier a page checks a proof against: the
// node's JSON-RPC URL, the deployment's Verifier contract and the Payment
// verifying key's slot in it.
type Verifier struct {
	RPCURL    string `json:"rpcUrl"`
	Address   string `json:"verifier"`
	PaymentVK int    `json:"paymentVk"`
}

// DeployedVerifier is cfg's deployment's Verifier, from build/receipts.json.
func DeployedVerifier(cfg Config) (Verifier, error) {
	receipts, err := loadReceipts(cfg.RetailRoot)
	if err != nil {
		return Verifier{}, err
	}
	r, ok := receipts["Verifier"]
	if !ok || r.ContractAddress == "" {
		return Verifier{}, fmt.Errorf("Verifier missing from build/receipts.json (run deploy+init)")
	}
	return Verifier{RPCURL: cfg.RPCURL, Address: common.HexToAddress(r.ContractAddress).Hex(), PaymentVK: PaymentVKSlot}, nil
}

// VerifyCall is a Payment proof's Verifier.verifyProof(vkIndex, proof,
// inputs) call: the slot, the proof's 8 elements and its public inputs, and
// the call's ABI-encoded data. Tamper is the same call with one output
// commitment changed.
type VerifyCall struct {
	Circuit string        `json:"circuit"`
	VK      int           `json:"vk"`
	Proof   [8]string     `json:"proof"`
	Inputs  []PublicInput `json:"inputs"`
	Data    string        `json:"data"`
	Tamper  Tamper        `json:"tamper"`
}

// Tamper is a VerifyCall with public input Index (named Name) changed from
// From to To, a different field element, and its data.
type Tamper struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
	From  string `json:"from"`
	To    string `json:"to"`
	Data  string `json:"data"`
}

// fieldModulus is BN254's scalar field order: public inputs are below it.
var fieldModulus, _ = new(big.Int).SetString("21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)

// snarkProof is IEnygmaDvp.SnarkProof as the ABI packs it. The relayer
// fills it from the gnark proof in order: A, B.X, B.Y, C.
type snarkProof struct {
	A struct{ X, Y *big.Int }
	B struct{ X, Y [2]*big.Int }
	C struct{ X, Y *big.Int }
}

// verifyCall is the VerifyCall of a Payment proof (8 decimal elements) with
// statement stmt, packed with the Verifier's ABI. Tamper adds 1 to the
// recipient's output commitment.
func verifyCall(verifierABI abi.ABI, proof []string, stmt []*big.Int) (VerifyCall, error) {
	inputs, err := publicInputs("Payment", NodeVault, 0, stmt)
	if err != nil {
		return VerifyCall{}, err
	}
	if len(proof) != 8 {
		return VerifyCall{}, fmt.Errorf("payment proof has %d elements, want 8", len(proof))
	}
	call := VerifyCall{Circuit: "Payment", VK: PaymentVKSlot, Inputs: inputs.Inputs}
	var p [8]*big.Int
	for i, s := range proof {
		v, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return VerifyCall{}, fmt.Errorf("proof[%d]: %q is not a decimal integer", i, s)
		}
		p[i], call.Proof[i] = v, fieldHex(v)
	}
	var sp snarkProof
	sp.A.X, sp.A.Y = p[0], p[1]
	sp.B.X, sp.B.Y = [2]*big.Int{p[2], p[3]}, [2]*big.Int{p[4], p[5]}
	sp.C.X, sp.C.Y = p[6], p[7]
	pack := func(stmt []*big.Int) (string, error) {
		data, err := verifierABI.Pack("verifyProof", big.NewInt(PaymentVKSlot), sp, stmt)
		if err != nil {
			return "", fmt.Errorf("pack verifyProof: %w", err)
		}
		return hexutil.Encode(data), nil
	}
	if call.Data, err = pack(stmt); err != nil {
		return VerifyCall{}, err
	}

	i := layout.PaymentCommitmentsOutOffset
	tampered := append([]*big.Int(nil), stmt...)
	tampered[i] = new(big.Int).Add(stmt[i], big.NewInt(1))
	tampered[i].Mod(tampered[i], fieldModulus)
	in := inputs.Inputs[i]
	call.Tamper = Tamper{Index: i, Name: fmt.Sprintf("%s[%d]", in.Name, in.Index), From: fieldHex(stmt[i]), To: fieldHex(tampered[i])}
	if call.Tamper.Data, err = pack(tampered); err != nil {
		return VerifyCall{}, err
	}
	return call, nil
}

// field is the call as the VerifyProofField protocol field carries it.
func (c VerifyCall) field() string {
	b, _ := json.Marshal(c)
	return string(b)
}

// sendVerifyCall sends the VerifyProofField of t, a settled 1-in Payment,
// so the page can check its proof on chain itself.
func (e *Engine) sendVerifyCall(s *step, t *sent) error {
	verifierABI, err := loadABI(e.cfg.RetailRoot, "Verifier")
	if err != nil {
		return err
	}
	call, err := verifyCall(verifierABI, t.proof, t.stmt)
	if err != nil {
		return fmt.Errorf("payment 1's verifyProof call: %w", err)
	}
	s.proto(VerifyProofField, call.field())
	return nil
}
