package utils

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"enygma_dvp/gnark_circuits/templates"
)

// registeredDepths is the depth init registers each vault's tree at: the
// circuits' depth, and EnygmaErc20CoinVault's own.
var registeredDepths = map[string]int{
	"Erc20CoinVault":       templates.MerkleTreeDepth,
	"Erc721CoinVault":      templates.MerkleTreeDepth,
	"Erc1155CoinVault":     templates.MerkleTreeDepth,
	"EnygmaErc20CoinVault": templates.EnygmaErc20CoinVaultTreeDepth,
}

// emptyVaultsChain serves a deployment whose vaults are all empty, each
// registered at its registeredDepths entry: currentRoot() is the empty
// root at that depth, treeNumber() is 0 and there are no logs. It writes
// the deployment's receipts.json into a fresh directory, makes that the
// working directory, and returns the RPC URL.
func emptyVaultsChain(t *testing.T) string {
	t.Helper()
	currentRootSel, treeNumberSel, vaultByIDSel := keccak4("currentRoot()"), keccak4("treeNumber()"), keccak4("vaultById(uint256)")
	receipts := map[string]contractReceipt{"EnygmaDvp": {ContractAddress: fmt.Sprintf("0x%040x", 0xd0)}}
	roots := map[string]*big.Int{}
	for i, name := range vaultNames {
		addr := fmt.Sprintf("0x%040x", i+1)
		receipts[name] = contractReceipt{ContractAddress: addr}
		roots[addr], _ = new(big.Int).SetString(newLocalMerkleTree(registeredDepths[name]).root(), 10)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Params) == 0 {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		result := `[]`
		if req.Method == "eth_call" {
			var call struct{ To, Data string }
			if err := json.Unmarshal(req.Params[0], &call); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			switch sel := strings.TrimPrefix(call.Data, "0x")[:8]; {
			case sel == currentRootSel && roots[call.To] != nil:
				result = fmt.Sprintf(`"0x%064x"`, roots[call.To])
			case sel == treeNumberSel, sel == vaultByIDSel:
				result = fmt.Sprintf(`"0x%064x"`, 0)
			default:
				http.Error(w, "unexpected eth_call", http.StatusBadRequest)
				return
			}
		}
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":%s}`, result)
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	data, err := json.Marshal(receipts)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "receipts.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	return srv.URL
}

// post runs handler on a POST of body and returns its status code and
// response body.
func post(t *testing.T, handler gin.HandlerFunc, body string) (int, []byte) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	handler(c)
	return w.Code, w.Body.Bytes()
}

// checkRebuiltAt fails unless s is the vault's tree rebuilt at the depth
// init registered it at, matching its onchain root.
func checkRebuiltAt(t *testing.T, handler string, s VaultMerkleStatus) {
	t.Helper()
	want, ok := registeredDepths[s.Name]
	if !ok {
		t.Fatalf("%s: a status for an unknown vault %q", handler, s.Name)
	}
	if s.Error != "" || s.Tree.Depth != want || !s.Match {
		t.Errorf("%s: %s rebuilt at depth %d (match %v, error %q); init registers it at depth %d",
			handler, s.Name, s.Tree.Depth, s.Match, s.Error, want)
	}
}

// TestMerkleHandlersRebuildEachVaultAtItsRegisteredDepth: init registers
// EnygmaErc20CoinVault at templates.EnygmaErc20CoinVaultTreeDepth and the
// other vaults at the circuits' depth, so merkleStatus and merkleVault
// rebuild each vault's tree at the depth it was registered at; at any
// other, an empty vault's rebuilt root would not match its onchain root.
func TestMerkleHandlersRebuildEachVaultAtItsRegisteredDepth(t *testing.T) {
	rpcURL := emptyVaultsChain(t)

	code, body := post(t, MerkleStatusHandler(), fmt.Sprintf(`{"rpcUrl":%q,"receiptsPath":"receipts.json"}`, rpcURL))
	var all MerkleStatusResponse
	if err := json.Unmarshal(body, &all); code != http.StatusOK || err != nil {
		t.Fatalf("merkleStatus: %d %s (%v)", code, body, err)
	}
	if len(all.Vaults) != len(registeredDepths) {
		t.Fatalf("merkleStatus: %d vaults, want %d", len(all.Vaults), len(registeredDepths))
	}
	for _, s := range all.Vaults {
		checkRebuiltAt(t, "merkleStatus", s)
	}

	for _, name := range vaultNames {
		code, body := post(t, MerkleVaultHandler(), fmt.Sprintf(`{"vault":%q,"rpcUrl":%q,"receiptsPath":"receipts.json"}`, name, rpcURL))
		var s VaultMerkleStatus
		if err := json.Unmarshal(body, &s); code != http.StatusOK || err != nil {
			t.Fatalf("merkleVault %s: %d %s (%v)", name, code, body, err)
		}
		checkRebuiltAt(t, "merkleVault", s)
	}
}
