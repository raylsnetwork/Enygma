package tests

// 22_v2_erc1155_standalone_leg_test.go
//
// Erc1155CoinVault.transfer() is open to any caller. A receipt with a non-zero
// StMessage is one leg of a DvP swap, which must only settle through
// EnygmaDvp.submitPartialSettlement with the counterparty's leg; settled alone
// it hands the owner's token over for nothing. Erc20CoinVault and
// Erc721CoinVault already refuse such a receipt; this checks the ERC1155 vault
// does too, before it looks at the proof. Runs on go-ethereum's simulated
// backend; needs `npx hardhat compile`.

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/rpc"
)

func TestV2Erc1155Vault_RejectsSwapLegInTransfer(t *testing.T) {
	vaultABI, vaultBin := loadArtifact(t, "core/contracts/vaults/Erc1155CoinVault.sol/Erc1155CoinVault.json")
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	from := crypto.PubkeyToAddress(key.PublicKey)
	backend := simulated.NewBackend(types.GenesisAlloc{from: {Balance: new(big.Int).Lsh(big.NewInt(1), 100)}})
	defer backend.Close()
	client := backend.Client()
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(1337))
	if err != nil {
		t.Fatal(err)
	}
	auth.GasLimit = 12_000_000
	addr, _, _, err := bind.DeployContract(auth, vaultABI, vaultBin, client, common.HexToAddress("0x00000000000000000000000000000000000000d1"))
	if err != nil {
		t.Fatal(err)
	}
	backend.Commit()

	// A 1-in/1-out receipt shaped like a swap leg: statement
	// [message, treeNumber, root, nullifier, commitment].
	receipt := func(message int64) interface{} {
		type g1 struct{ X, Y *big.Int }
		type g2 struct{ X, Y [2]*big.Int }
		zero := big.NewInt(0)
		return struct {
			Proof struct {
				A g1
				B g2
				C g1
			}
			Statement       []*big.Int
			NumberOfInputs  *big.Int
			NumberOfOutputs *big.Int
		}{
			Proof: struct {
				A g1
				B g2
				C g1
			}{g1{zero, zero}, g2{[2]*big.Int{zero, zero}, [2]*big.Int{zero, zero}}, g1{zero, zero}},
			Statement:       []*big.Int{big.NewInt(message), zero, big.NewInt(5), big.NewInt(6), big.NewInt(7)},
			NumberOfInputs:  big.NewInt(1),
			NumberOfOutputs: big.NewInt(1),
		}
	}
	revertOf := func(message int64) string {
		t.Helper()
		data, err := vaultABI.Pack("transfer", receipt(message))
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.CallContract(context.Background(), ethereum.CallMsg{From: from, To: &addr, Gas: auth.GasLimit, Data: data}, nil)
		if err == nil {
			return ""
		}
		// Return the revert data's selector when there is one.
		var de rpc.DataError
		if errors.As(err, &de) {
			if s, ok := de.ErrorData().(string); ok && len(s) >= 10 {
				return strings.TrimPrefix(s, "0x")[:8]
			}
		}
		return err.Error()
	}

	wantSel := common.Bytes2Hex(crypto.Keccak256([]byte("InvalidPaymentMessage()"))[:4])
	got := revertOf(42)
	if got != wantSel {
		t.Fatalf("VULNERABLE: transfer() did not refuse a swap-leg receipt (StMessage != 0) up front: got %q, want InvalidPaymentMessage (0x%s)", got, wantSel)
	}

	// A standalone receipt (StMessage == 0) is not refused by this guard; it
	// goes on to the proof checks (and fails there, as this proof is empty).
	if got := revertOf(0); got == wantSel {
		t.Fatalf("a standalone receipt (StMessage == 0) was refused as a swap leg: %q", got)
	}
}
