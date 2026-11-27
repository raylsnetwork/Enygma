package tests

// 36_v2_vault_tree_depth_test.go
//
// Init registers every vault at a tree depth, and a vault's notes can be
// spent only by proofs made at that depth. Both enygma_dvp deployments
// register the vaults proofs spend from at the circuits' depth
// (layout.MerkleTreeDepth), and the DvP deployment registers its
// EnygmaErc20CoinVault at its own, layout.EnygmaErc20CoinVaultTreeDepth:
// no proof spends from that vault, and Enygma pays for each of its levels
// on every depositThroughEnygma. A vault's tree 0 starts with the empty
// root of its depth (Merkle.sol's initializeMerkle), which isValidRoot
// keeps, so that root shows the depth init registered it at.
// Needs a live chain with both deployments (make dvp-stack).

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/raylsnetwork/enygma_dvp/src/core"

	"enygma_dvp/layout"
)

func TestV2VaultTreeDepth_EachVaultAtTheDepthInitRegistersItAt(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping on-chain test")
	}
	client, err := ethclient.Dial(hardhatRPC)
	if err != nil {
		t.Fatalf("ethclient.Dial: %v", err)
	}
	defer client.Close()
	merkleABI := loadOnchainABI(t, "core/contracts/vaults/Merkle.sol/Merkle.json")
	dvp, payment := loadOnchainReceipts(t), loadPaymentReceipts(t)

	for _, v := range []struct {
		deployment, vault string
		addr              string
		depth             int
	}{
		{"DvP", "Erc20CoinVault", dvp["Erc20CoinVault"].ContractAddress, layout.MerkleTreeDepth},
		{"DvP", "Erc721CoinVault", dvp["Erc721CoinVault"].ContractAddress, layout.MerkleTreeDepth},
		{"DvP", "Erc1155CoinVault", dvp["Erc1155CoinVault"].ContractAddress, layout.MerkleTreeDepth},
		{"DvP", "EnygmaErc20CoinVault", dvp["EnygmaErc20CoinVault"].ContractAddress, layout.EnygmaErc20CoinVaultTreeDepth},
		{"Payment", "Erc20CoinVault", payment["Erc20CoinVault"].ContractAddress, layout.MerkleTreeDepth},
		{"Payment", "UsdrCoinVault", payment["UsdrCoinVault"].ContractAddress, layout.MerkleTreeDepth},
	} {
		if !common.IsHexAddress(v.addr) {
			t.Errorf("%s deployment: no %s in its receipts", v.deployment, v.vault)
			continue
		}
		addr := common.HexToAddress(v.addr)
		if !tree0HadEmptyRoot(t, client, merkleABI, addr, v.depth) {
			var others []int
			for d := 1; d <= 32; d++ {
				if d != v.depth && tree0HadEmptyRoot(t, client, merkleABI, addr, d) {
					others = append(others, d)
				}
			}
			t.Errorf("%s deployment: %s's tree is not depth %d (its tree 0 started empty at depth %v)", v.deployment, v.vault, v.depth, others)
		}
	}
}

// tree0HadEmptyRoot reports whether the vault at addr accepts, for its tree
// 0, the root of an empty tree of the given depth: the root its tree starts
// with when registered at that depth.
func tree0HadEmptyRoot(t *testing.T, client *ethclient.Client, merkleABI abi.ABI, addr common.Address, depth int) bool {
	t.Helper()
	data, err := merkleABI.Pack("isValidRoot", big.NewInt(0), core.NewMerkleTree(depth).Root())
	if err != nil {
		t.Fatalf("pack isValidRoot: %v", err)
	}
	out, err := client.CallContract(context.Background(), ethereum.CallMsg{To: &addr, Data: data}, nil)
	if err != nil {
		t.Fatalf("isValidRoot on %s: %v", addr.Hex(), err)
	}
	res, err := merkleABI.Unpack("isValidRoot", out)
	if err != nil {
		t.Fatalf("unpack isValidRoot: %v", err)
	}
	return res[0].(bool)
}
