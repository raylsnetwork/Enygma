package tests

// 29_v2_vault_zero_commitments_test.go
//
// A vault skips zero commitments wherever they sit: registerCoins and
// insertCommitmentsFromReceipt insert (and emit Commitment for) exactly the
// non-zero entries, in order, even when a zero comes before a real one.
// Both ways a vault reads a receipt's outputs are covered:
//   - Erc20CoinVault reads them through its Payment-family layout
//     (_paymentCircuit), so its receipt has a real PaymentRelayerFee shape;
//   - EnygmaErc20CoinVault keeps AbstractCoinVault's legacy walker, which
//     reads them by the receipt's numberOfInputs and numberOfOutputs.
// Runs on go-ethereum's simulated backend with the echidna harness's
// MockPoseidonWrapper; needs `npx hardhat compile`.

import (
	"math/big"
	"testing"

	"enygma_dvp/layout"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
)

func TestV2Vault_SkipsZeroCommitments(t *testing.T) {
	n := func(v int64) *big.Int { return big.NewInt(v) }
	receiptOf := func(statement []*big.Int, inputs, outputs int64) onchainProofReceipt {
		return onchainProofReceipt{
			Proof: onchainSnarkProof{
				A: onchainG1Point{X: n(0), Y: n(0)},
				B: onchainG2Point{X: [2]*big.Int{n(0), n(0)}, Y: [2]*big.Int{n(0), n(0)}},
				C: onchainG1Point{X: n(0), Y: n(0)},
			},
			Statement:       statement,
			NumberOfInputs:  n(inputs),
			NumberOfOutputs: n(outputs),
		}
	}

	t.Run("Erc20CoinVault", func(t *testing.T) {
		send := deployZeroCommitmentVault(t, "core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json")
		assertCommitments(t, "registerCoins",
			send("registerCoins", []*big.Int{n(0), n(5), n(0), n(7)}), n(5), n(7))

		// A PaymentRelayerFee receipt (one input, three outputs): its outputs
		// sit where the layout says, the first of them zero.
		statement := make([]*big.Int, layout.PaymentRelayerFeeLen)
		for i := range statement {
			statement[i] = n(int64(100 + i))
		}
		outs := statement[layout.PaymentRelayerFeeCommitmentsOutOffset:][:layout.PaymentRelayerFeeCommitmentsOutSize]
		outs[0], outs[1], outs[2] = n(0), n(9), n(11)
		assertCommitments(t, "insertCommitmentsFromReceipt",
			send("insertCommitmentsFromReceipt", receiptOf(statement,
				layout.PaymentRelayerFeeNullifiersSize, layout.PaymentRelayerFeeCommitmentsOutSize)),
			n(9), n(11))
	})

	t.Run("EnygmaErc20CoinVault (legacy walker)", func(t *testing.T) {
		send := deployZeroCommitmentVault(t, "core/contracts/vaults/EnygmaErc20CoinVault.sol/EnygmaErc20CoinVault.json")
		assertCommitments(t, "registerCoins",
			send("registerCoins", []*big.Int{n(0), n(5), n(0), n(7)}), n(5), n(7))

		// One input, three outputs: the walker reads outputs from statement
		// index 1 + 2*inputs + inputs = 4 on; slots 0-3 are not read here.
		statement := []*big.Int{n(1), n(2), n(3), n(4), n(0), n(9), n(11)}
		assertCommitments(t, "insertCommitmentsFromReceipt",
			send("insertCommitmentsFromReceipt", receiptOf(statement, 1, 3)), n(9), n(11))
	})
}

// deployZeroCommitmentVault deploys and initializes the vault in artifact on a
// fresh simulated chain, the deployer acting as its DvP (so it may initialize
// and insert), and returns send: it mines one call and returns the
// commitments its Commitment events carry.
func deployZeroCommitmentVault(t *testing.T, artifact string) func(method string, args ...interface{}) []*big.Int {
	t.Helper()
	vaultABI, vaultBin := loadArtifact(t, artifact)
	poseidonABI, poseidonBin := loadArtifact(t, "echidna/EchidnaVaultHarness.sol/MockPoseidonWrapper.json")

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
	auth.GasLimit = 10_000_000

	poseidonAddr, _, _, err := bind.DeployContract(auth, poseidonABI, poseidonBin, client)
	if err != nil {
		t.Fatal(err)
	}
	_, _, vault, err := bind.DeployContract(auth, vaultABI, vaultBin, client, from)
	if err != nil {
		t.Fatal(err)
	}
	backend.Commit()

	send := func(method string, args ...interface{}) []*big.Int {
		t.Helper()
		tx, err := vault.Transact(auth, method, args...)
		if err != nil {
			t.Fatalf("%s: %v", method, err)
		}
		backend.Commit()
		r, err := bind.WaitMined(t.Context(), client, tx)
		if err != nil || r.Status != types.ReceiptStatusSuccessful {
			t.Fatalf("%s: mined %v, status %v", method, err, r.Status)
		}
		return commitmentEvents(vaultABI, r)
	}
	send("initializeVault", big.NewInt(1), big.NewInt(1), common.HexToAddress("0x1"), big.NewInt(8),
		poseidonAddr, common.HexToAddress("0xdead"), common.HexToAddress("0xbeef"))
	return send
}

// commitmentEvents returns the commitment of every Commitment event in r, in order.
func commitmentEvents(vaultABI abi.ABI, r *types.Receipt) []*big.Int {
	id := vaultABI.Events["Commitment"].ID
	var out []*big.Int
	for _, l := range r.Logs {
		if len(l.Topics) == 3 && l.Topics[0] == id {
			out = append(out, l.Topics[2].Big())
		}
	}
	return out
}

// assertCommitments fails unless got is exactly want, in order.
func assertCommitments(t *testing.T, what string, got []*big.Int, want ...*big.Int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: inserted %v, want %v", what, got, want)
	}
	for i := range want {
		if got[i].Cmp(want[i]) != 0 {
			t.Fatalf("%s: inserted %v, want %v", what, got, want)
		}
	}
}
