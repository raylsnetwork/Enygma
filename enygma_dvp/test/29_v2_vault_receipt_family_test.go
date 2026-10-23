package tests

// 29_v2_vault_receipt_family_test.go
//
// A coin vault checks a DvP leg apart from a payment receipt (ADR 0001):
// checkDvpReceiptConditions takes only the vault's own leg kind, read by
// name and never by numberOfInputs/numberOfOutputs, and Erc20CoinVault's
// checkReceiptConditions takes only Payment-family receipts, choosing the
// circuit by (inputs, outputs) and statement length before reading anything.
// This pins the revert each malformed shape gets. The vaults are deployed
// bare (no verifier, empty trees) on go-ethereum's simulated backend, so a
// receipt that passes its shape checks stops at InvalidMerkleRoot; needs
// `npx hardhat compile`.

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/rpc"

	"enygma_dvp/layout"
)

// bareVault is a vault deployed with no initialization.
type bareVault struct {
	t    *testing.T
	abi  abi.ABI
	addr common.Address
	from common.Address
	call func(ethereum.CallMsg) ([]byte, error)
}

func deployBareVaults(t *testing.T, artifacts ...string) []*bareVault {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	from := crypto.PubkeyToAddress(key.PublicKey)
	backend := simulated.NewBackend(types.GenesisAlloc{from: {Balance: new(big.Int).Lsh(big.NewInt(1), 100)}})
	t.Cleanup(func() { backend.Close() })
	client := backend.Client()
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(1337))
	if err != nil {
		t.Fatal(err)
	}
	auth.GasLimit = 12_000_000
	var vaults []*bareVault
	for _, rel := range artifacts {
		vaultABI, vaultBin := loadArtifact(t, rel)
		addr, _, _, err := bind.DeployContract(auth, vaultABI, vaultBin, client, common.HexToAddress("0x00000000000000000000000000000000000000d1"))
		if err != nil {
			t.Fatalf("deploy %s: %v", rel, err)
		}
		backend.Commit()
		vaults = append(vaults, &bareVault{t: t, abi: vaultABI, addr: addr, from: from, call: func(m ethereum.CallMsg) ([]byte, error) {
			return client.CallContract(context.Background(), m, nil)
		}})
	}
	return vaults
}

// revert calls method with receipt and returns the revert's selector, or ""
// if the call succeeded.
func (v *bareVault) revert(method string, receipt onchainProofReceipt) string {
	v.t.Helper()
	data, err := v.abi.Pack(method, receipt)
	if err != nil {
		v.t.Fatalf("pack %s: %v", method, err)
	}
	_, err = v.call(ethereum.CallMsg{From: v.from, To: &v.addr, Gas: 12_000_000, Data: data})
	if err == nil {
		return ""
	}
	var de rpc.DataError
	if errors.As(err, &de) {
		if s, ok := de.ErrorData().(string); ok && len(s) >= 10 {
			return strings.TrimPrefix(s, "0x")[:8]
		}
	}
	return err.Error()
}

func selectorOf(signature string) string {
	return common.Bytes2Hex(crypto.Keccak256([]byte(signature))[:4])
}

var (
	selInvalidStatmentSize          = selectorOf("InvalidStatmentSize()")
	selInvalidNumberOfInputs        = selectorOf("InvalidNumberOfInputs()")
	selInvalidNumberOfOutputs       = selectorOf("InvalidNumberOfOutputs()")
	selJoinSplitWithSameCommitments = selectorOf("JoinSplitWithSameCommitments()")
	selInvalidMerkleRoot            = selectorOf("InvalidMerkleRoot()")
	selDvpLegNotSupported           = selectorOf("DvpLegNotSupported()")
	selErrorString                  = selectorOf("Error(string)")
)

// fakeReceipt has an empty proof and a statement of length n whose signals
// are distinct, non-zero and name no vault.
func fakeReceipt(n, inputs, outputs int) onchainProofReceipt {
	zero := big.NewInt(0)
	st := make([]*big.Int, n)
	for i := range st {
		st[i] = big.NewInt(int64(5 + i))
	}
	return onchainProofReceipt{
		Proof: onchainSnarkProof{
			A: onchainG1Point{X: zero, Y: zero},
			B: onchainG2Point{X: [2]*big.Int{zero, zero}, Y: [2]*big.Int{zero, zero}},
			C: onchainG1Point{X: zero, Y: zero},
		},
		Statement:       st,
		NumberOfInputs:  big.NewInt(int64(inputs)),
		NumberOfOutputs: big.NewInt(int64(outputs)),
	}
}

const (
	erc20VaultArtifact   = "core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json"
	erc721VaultArtifact  = "core/contracts/vaults/Erc721CoinVault.sol/Erc721CoinVault.json"
	erc1155VaultArtifact = "core/contracts/vaults/Erc1155CoinVault.sol/Erc1155CoinVault.json"
)

func TestV2Erc20Vault_PaymentReceiptShapes(t *testing.T) {
	vault := deployBareVaults(t, erc20VaultArtifact)[0]
	for _, c := range []struct {
		name               string
		inputs, outputs, n int
		want               string
	}{
		{"3 inputs", 3, 2, layout.PaymentLen, selInvalidNumberOfInputs},
		{"0 inputs", 0, 2, layout.PaymentLen, selInvalidNumberOfInputs},
		{"Payment2in with 3 outputs", 2, 3, layout.Payment2inLen, selInvalidNumberOfOutputs},
		{"1 input, 1 output (a DvpInitiator leg)", 1, 1, layout.DvpInitiatorLen, selInvalidNumberOfOutputs},
		{"1 input, 4 outputs", 1, 4, layout.PaymentRelayerFeeLen, selInvalidNumberOfOutputs},
		{"(1,2) at length 6", 1, 2, 6, selInvalidStatmentSize},
		{"(1,2) at length 10", 1, 2, 10, selInvalidStatmentSize},
		{"(1,3) at length 8", 1, 3, 8, selInvalidStatmentSize},
		{"(2,2) at length 7", 2, 2, 7, selInvalidStatmentSize},
		{"empty statement", 1, 2, 0, selInvalidStatmentSize},
		// A well-shaped receipt that names no vault.
		{"Payment not bound to the vault", 1, 2, layout.PaymentLen, selErrorString},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := vault.revert("checkReceiptConditions", fakeReceipt(c.n, c.inputs, c.outputs)); got != c.want {
				t.Errorf("checkReceiptConditions reverted %q, want %q", got, c.want)
			}
		})
	}

	// Bound to the vault at its own circuit's ContractAddress offset, each row
	// gets past the binding and the distinct-outputs check to the root check:
	// the fields are read at that circuit's offsets.
	vaultSignal := vaultInt(vault.addr)
	for _, c := range []struct {
		name                         string
		inputs, outputs, n, contract int
	}{
		{"Payment", 1, 2, layout.PaymentLen, layout.PaymentContractAddressOffset},
		{"PaymentFee", 1, 2, layout.PaymentFeeLen, layout.PaymentFeeContractAddressOffset},
		{"UsdrFee", 1, 2, layout.UsdrFeeLen, layout.UsdrFeeContractAddressOffset},
		{"PaymentRelayerFee", 1, 3, layout.PaymentRelayerFeeLen, layout.PaymentRelayerFeeContractAddressOffset},
		{"Payment2in", 2, 2, layout.Payment2inLen, layout.Payment2inContractAddressOffset},
	} {
		t.Run(c.name+" bound to the vault", func(t *testing.T) {
			rc := fakeReceipt(c.n, c.inputs, c.outputs)
			rc.Statement[c.contract] = vaultSignal
			if got := vault.revert("checkReceiptConditions", rc); got != selInvalidMerkleRoot {
				t.Errorf("checkReceiptConditions reverted %q, want InvalidMerkleRoot (%s)", got, selInvalidMerkleRoot)
			}
		})
	}

	t.Run("Payment2in with equal outputs", func(t *testing.T) {
		rc := fakeReceipt(layout.Payment2inLen, 2, 2)
		rc.Statement[layout.Payment2inContractAddressOffset] = vaultSignal
		rc.Statement[layout.Payment2inCommitmentsOutOffset+1] = rc.Statement[layout.Payment2inCommitmentsOutOffset]
		if got := vault.revert("checkReceiptConditions", rc); got != selJoinSplitWithSameCommitments {
			t.Errorf("checkReceiptConditions reverted %q, want JoinSplitWithSameCommitments (%s)", got, selJoinSplitWithSameCommitments)
		}
	})
}

func TestV2Vaults_DvpLegChecks(t *testing.T) {
	vaults := deployBareVaults(t, erc20VaultArtifact, erc721VaultArtifact, erc1155VaultArtifact)
	erc20, erc721, erc1155 := vaults[0], vaults[1], vaults[2]
	// numberOfInputs/numberOfOutputs are never read for a leg: give them
	// values no circuit has.
	initiator := fakeReceipt(layout.DvpInitiatorLen, 0, 7)
	destination := fakeReceipt(layout.DvpDestinationLen, 0, 7)

	for _, c := range []struct {
		name    string
		vault   *bareVault
		receipt onchainProofReceipt
		want    string
	}{
		{"Erc20 takes an initiator leg", erc20, initiator, selInvalidMerkleRoot},
		{"Erc20 refuses a destination leg", erc20, destination, selInvalidStatmentSize},
		{"Erc20 refuses a Payment receipt", erc20, fakeReceipt(layout.PaymentLen, 1, 2), selInvalidStatmentSize},
		{"Erc721 takes a destination leg", erc721, destination, selInvalidMerkleRoot},
		{"Erc721 refuses an initiator leg", erc721, initiator, selInvalidStatmentSize},
		{"Erc1155 refuses an initiator leg", erc1155, initiator, selDvpLegNotSupported},
		{"Erc1155 refuses a destination leg", erc1155, destination, selDvpLegNotSupported},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := c.vault.revert("checkDvpReceiptConditions", c.receipt); got != c.want {
				t.Errorf("checkDvpReceiptConditions reverted %q, want %q", got, c.want)
			}
		})
	}

	t.Run("Erc20 refuses an initiator leg whose CommitB is its CommitA", func(t *testing.T) {
		rc := fakeReceipt(layout.DvpInitiatorLen, 0, 7)
		rc.Statement[layout.DvpInitiatorCommitBOffset] = rc.Statement[layout.DvpInitiatorCommitAOffset]
		if got := erc20.revert("checkDvpReceiptConditions", rc); got != selJoinSplitWithSameCommitments {
			t.Errorf("checkDvpReceiptConditions reverted %q, want JoinSplitWithSameCommitments (%s)", got, selJoinSplitWithSameCommitments)
		}
	})

	// A leg is not a payment receipt, whatever counts it is sent with.
	t.Run("Erc20 checkReceiptConditions refuses an initiator leg", func(t *testing.T) {
		for _, c := range []struct {
			outputs int
			want    string
		}{
			{1, selInvalidNumberOfOutputs},
			// Read as PaymentFee (same length): its ContractAddress slot holds
			// the leg's RevertCommitA, never this vault.
			{2, selErrorString},
			{3, selInvalidStatmentSize},
		} {
			if got := erc20.revert("checkReceiptConditions", fakeReceipt(layout.DvpInitiatorLen, 1, c.outputs)); got != c.want {
				t.Errorf("numberOfOutputs %d: checkReceiptConditions reverted %q, want %q", c.outputs, got, c.want)
			}
		}
	})

	// transfer() refuses a leg in checkReceiptConditions, not only in its
	// StMessage guard: with StMessage 0 and three outputs the guard passes.
	t.Run("Erc20 transfer refuses an initiator leg past the message guard", func(t *testing.T) {
		rc := fakeReceipt(layout.DvpInitiatorLen, 1, 3)
		rc.Statement[layout.DvpInitiatorMessageOffset] = big.NewInt(0)
		if got := erc20.revert("transfer", rc); got != selInvalidStatmentSize {
			t.Errorf("transfer reverted %q, want InvalidStatmentSize (%s)", got, selInvalidStatmentSize)
		}
	})
}

func TestV2Erc721Vault_LegacyOwnershipReceiptShapes(t *testing.T) {
	vault := deployBareVaults(t, erc721VaultArtifact)[0]
	for _, c := range []struct {
		name string
		n    int
		want string
	}{
		{"empty statement", 0, selInvalidStatmentSize},
		{"short statement", 3, selInvalidStatmentSize},
		{"a destination leg", layout.DvpDestinationLen, selInvalidStatmentSize},
		{"legacy ownership", 5, selInvalidMerkleRoot},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := vault.revert("checkReceiptConditions", fakeReceipt(c.n, 1, 1)); got != c.want {
				t.Errorf("checkReceiptConditions reverted %q, want %q", got, c.want)
			}
		})
	}
}
