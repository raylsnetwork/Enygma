package relayer

import (
	"fmt"
	"log"
	"math/big"
	"net/http"

	enygma "enygma-server/pkg/circuits/enygma"
	utils "enygma-server/utils"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend"
	"github.com/consensys/gnark/backend/groth16"
	groth16_bn254 "github.com/consensys/gnark/backend/groth16/bn254"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/std/algebra/emulated/sw_bn254"
	"github.com/consensys/gnark/std/math/emulated"
	stdgroth16 "github.com/consensys/gnark/std/recursion/groth16"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/sha3"
)

// enygmaNCommitment must match the deployed transfer circuit's
// EnygmaCircuitConfig.NCommitment (see keygen/generate_keys.go's splitSize
// and enygma/handler.go's config) — fixed at 6, giving 80 public signals.
const enygmaNCommitment = 6

// enygmaCircuitTemplate builds an empty *enygma.EnygmaCircuit purely for
// frontend.Compile — to learn the transfer circuit's exact public-input
// shape, not to touch its keys. Mirrors enygma/handler.go's
// createCircuitTemplate (unexported there, so duplicated here rather than
// exported cross-package for a one-shot shape query).
func enygmaCircuitTemplate() *enygma.EnygmaCircuit {
	config := enygma.EnygmaCircuitConfig{NCommitment: enygmaNCommitment}
	fp := make([][]frontend.Variable, config.NCommitment)
	for i := range fp {
		fp[i] = make([]frontend.Variable, config.NCommitment)
	}
	return &enygma.EnygmaCircuit{
		Config:                     config,
		FingerPrintofSharedSecrets: fp,
		PublicKey:                  make([]frontend.Variable, config.NCommitment),
		PreviousCommit:             make([][2]frontend.Variable, config.NCommitment),
		TxCommit:                   make([][2]frontend.Variable, config.NCommitment),
		AnonymitySet:               make([]frontend.Variable, config.NCommitment),
		SharedSecrets:              make([]frontend.Variable, config.NCommitment),
		MessageTags:                make([]frontend.Variable, config.NCommitment),
		TxValues:                   make([]frontend.Variable, config.NCommitment),
		TxRandomValues:             make([]frontend.Variable, config.NCommitment),
	}
}

// RelayerRequest carries exactly the two fields the relayer already receives
// unmodified from the sender ({Proof, PublicSignal} in RelayTransferRequest
// on the relayer side) — no private/secret data ever appears here.
type RelayerRequest struct {
	Proof        [8]string `json:"proof" binding:"required"`
	PublicSignal []string  `json:"publicSignal" binding:"required,len=80"`
}

// RelayerOutput is the relayer's own recursive proof, plus the public
// signal it attests to — identical to the input PublicSignal, since this
// handler re-attests to the same values rather than transforming them.
//
// Proof has 12 elements, not the usual 8: [Ax, Ay, BX11, BX01, BY11, BY01,
// Cx, Cy] in the same gnark-solidity remix every other circuit in this
// server uses, followed by [CommitmentX, CommitmentY, PokX, PokY]. The
// extra 4 are NOT a design choice here — gnark's std/math/emulated library
// internally batches this circuit's very large number of range-checks
// (from the recursive pairing check's field emulation) via one BSB22
// Pedersen commitment, purely as an efficiency mechanism, independent of
// anything this circuit's Define() does explicitly. RelayerVerifier.sol's
// generated verifyProof therefore takes uint256[8] proof, uint256[2]
// commitments, uint256[2] commitmentPok, uint256[80] input — not the plain
// 2-arg form EnygmaVerifier.sol uses. Confirmed via ccs.GetCommitments()
// (exactly 1) and the generated .sol file itself.
type RelayerOutput struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}

// NewHandler wires the relayer's recursive-verification endpoint.
//   - pkPath/vkPath: this circuit's own proving/verifying keys
//     (keys/RelayerPk.key, keys/RelayerVk.key — from generateKeysRelayer).
//   - enygmaVkPath: the transfer circuit's verifying key
//     (keys/EnygmaVk.key), embedded as a compile-time-fixed constant — must
//     be the exact same file used by keygen's generateKeysRelayer, or the
//     compiled circuit here won't match the loaded pk/vk.
func NewHandler(pkPath, vkPath, enygmaVkPath string) gin.HandlerFunc {
	curveID := ecc.BN254

	pk, err := utils.LoadProvingKey(curveID, pkPath)
	if err != nil {
		log.Fatalf("failed to load relayer proving key %s: %v", pkPath, err)
	}

	enygmaVk, err := utils.LoadVerifyingKey(curveID, enygmaVkPath)
	if err != nil {
		log.Fatalf("failed to load enygma verifying key %s: %v", enygmaVkPath, err)
	}

	// Recompile the transfer circuit only to size the placeholder inner
	// witness (PlaceholderWitness sizes off ccs.GetNbPublicVariables()).
	enygmaCcs, err := frontend.Compile(curveID.ScalarField(), r1cs.NewBuilder, enygmaCircuitTemplate())
	if err != nil {
		log.Fatalf("failed to compile enygma circuit template: %v", err)
	}
	if got := enygmaCcs.GetNbPublicVariables() - 1; got != NPublicSignals {
		log.Fatalf("enygma circuit has %d public signals, relayer circuit expects %d", got, NPublicSignals)
	}

	fixedVk, err := stdgroth16.ValueOfVerifyingKeyFixed[sw_bn254.G1Affine, sw_bn254.G2Affine, sw_bn254.GTEl](enygmaVk)
	if err != nil {
		log.Fatalf("failed to build fixed verifying key: %v", err)
	}

	circuitTemplate := NewCircuit(fixedVk)
	circuitTemplate.InnerWitness = stdgroth16.PlaceholderWitness[sw_bn254.ScalarField](enygmaCcs)

	ccs, err := frontend.Compile(curveID.ScalarField(), r1cs.NewBuilder, circuitTemplate)
	if err != nil {
		log.Fatalf("failed to compile relayer circuit: %v", err)
	}

	return func(c *gin.Context) {
		var request RelayerRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		nativeProof, err := parseInnerProof(request.Proof)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("proof: %v", err)})
			return
		}

		var publicValues [NPublicSignals]*big.Int
		for i, s := range request.PublicSignal {
			n, ok := new(big.Int).SetString(s, 10)
			if !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("publicSignal[%d]: invalid decimal %q", i, s)})
				return
			}
			publicValues[i] = n
		}

		circuitProof, err := stdgroth16.ValueOfProof[sw_bn254.G1Affine, sw_bn254.G2Affine](nativeProof)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid inner proof: %v", err)})
			return
		}

		assignment := NewCircuit(fixedVk)
		assignment.Proof = circuitProof
		assignment.InnerWitness = stdgroth16.PlaceholderWitness[sw_bn254.ScalarField](enygmaCcs)
		for i, v := range publicValues {
			assignment.InnerWitness.Public[i] = emulated.ValueOf[sw_bn254.ScalarField](v)
			assignment.PublicSignal[i] = frontend.Variable(v)
		}

		witnessFull, err := frontend.NewWitness(assignment, curveID.ScalarField())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid witness: %v", err)})
			return
		}

		// WithProverHashToFieldFunction(keccak256): this circuit has one
		// BSB22 Pedersen commitment (see NPublicSignals doc comment in
		// circuit.go). groth16.Prove()'s default hash-to-field for
		// commitments does NOT match what vk.ExportSolidity() assumed when
		// generating RelayerVerifier.sol (keccak256, gnark's own default
		// there) — without this, the commitment itself verifies fine but
		// the main pairing check fails on-chain with ProofInvalid, since
		// prover and verifier derived different "public commitment" values.
		proof, err := groth16.Prove(ccs, pk, witnessFull, backend.WithProverHashToFieldFunction(sha3.NewLegacyKeccak256()))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("proof generation failed: %v", err)})
			return
		}

		p := proof.(*groth16_bn254.Proof)
		if len(p.Commitments) != 1 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("relayer proof has %d commitments, expected exactly 1 (circuit/key mismatch?)", len(p.Commitments))})
			return
		}
		A_x1 := new(big.Int)
		p.Ar.X.BigInt(A_x1)
		A_y1 := new(big.Int)
		p.Ar.Y.BigInt(A_y1)
		C_x1 := new(big.Int)
		p.Krs.X.BigInt(C_x1)
		C_y1 := new(big.Int)
		p.Krs.Y.BigInt(C_y1)
		BX01 := new(big.Int)
		p.Bs.X.A0.BigInt(BX01)
		BX11 := new(big.Int)
		p.Bs.X.A1.BigInt(BX11)
		BY01 := new(big.Int)
		p.Bs.Y.A0.BigInt(BY01)
		BY11 := new(big.Int)
		p.Bs.Y.A1.BigInt(BY11)
		commitX := new(big.Int)
		p.Commitments[0].X.BigInt(commitX)
		commitY := new(big.Int)
		p.Commitments[0].Y.BigInt(commitY)
		pokX := new(big.Int)
		p.CommitmentPok.X.BigInt(pokX)
		pokY := new(big.Int)
		p.CommitmentPok.Y.BigInt(pokY)

		proofRemix := []*big.Int{
			A_x1, A_y1,
			BX11, BX01,
			BY11, BY01,
			C_x1, C_y1,
			commitX, commitY,
			pokX, pokY,
		}

		c.JSON(http.StatusOK, RelayerOutput{
			Proof:        proofRemix,
			PublicSignal: publicValues[:],
		})
	}
}

// parseInnerProof inverts the gnark-solidity 8-element remix used by every
// handler in this server ([Ax, Ay, B_A1_1, B_A0_1, B_A1_0... — see
// enygma/handler.go's proofRemix) back into a native *groth16_bn254.Proof.
// Element order: [A.X, A.Y, B.X.A1, B.X.A0, B.Y.A1, B.Y.A0, C.X, C.Y].
func parseInnerProof(raw [8]string) (groth16.Proof, error) {
	vals := make([]*big.Int, 8)
	for i, s := range raw {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return nil, fmt.Errorf("[%d]: invalid decimal %q", i, s)
		}
		vals[i] = n
	}

	var proof groth16_bn254.Proof
	proof.Ar.X.SetBigInt(vals[0])
	proof.Ar.Y.SetBigInt(vals[1])
	proof.Bs.X.A1.SetBigInt(vals[2])
	proof.Bs.X.A0.SetBigInt(vals[3])
	proof.Bs.Y.A1.SetBigInt(vals[4])
	proof.Bs.Y.A0.SetBigInt(vals[5])
	proof.Krs.X.SetBigInt(vals[6])
	proof.Krs.Y.SetBigInt(vals[7])

	return &proof, nil
}
