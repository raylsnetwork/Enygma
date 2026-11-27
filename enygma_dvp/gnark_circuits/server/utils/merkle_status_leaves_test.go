package utils

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// leafLogJSON is the eth_getLogs entry of a LeafInserted event placing leaf
// at (tree, index).
func leafLogJSON(tree, index int, leaf int64) string {
	return fmt.Sprintf(`{"topics":["%s","0x%064x","0x%064x"],"data":"0x%064x%064x"}`,
		keccak32Hex("LeafInserted(uint256,uint256,uint256,uint256)"), 1, leaf, tree, index)
}

// fakeVaultRPC answers eth_getLogs with logs and the vault's treeNumber()
// and currentRoot() with tree and root.
func fakeVaultRPC(t *testing.T, logs []string, tree int, root *big.Int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		switch req.Method {
		case "eth_getLogs":
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":[%s]}`, strings.Join(logs, ","))
		case "eth_call":
			var call struct {
				Data string `json:"data"`
			}
			if err := json.Unmarshal(req.Params[0], &call); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			v := root
			if call.Data == "0x"+keccak4("treeNumber()") {
				v = big.NewInt(int64(tree))
			}
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":"0x%064x"}`, v)
		default:
			http.Error(w, "unexpected RPC method", http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// boundaryLogs is a depth-3 vault's LeafInserted logs where one transaction
// made two inserts: leaf 100 took index 5 of tree 0, then leaves 101 and
// 102 started tree 1, the vault's current one, whose root is boundaryRoot.
var boundaryLogs = []string{
	leafLogJSON(0, 0, 1), leafLogJSON(0, 1, 2), leafLogJSON(0, 2, 3), leafLogJSON(0, 3, 4), leafLogJSON(0, 4, 5),
	leafLogJSON(0, 5, 100), leafLogJSON(1, 0, 101), leafLogJSON(1, 1, 102),
}

// boundaryRoot is tree 1's root holding leaves 101 and 102, hashed here
// level by level rather than by localMerkleTree.
func boundaryRoot() *big.Int {
	z := buildZeroLevels(3)
	return localPoseidon2(localPoseidon2(localPoseidon2(big.NewInt(101), big.NewInt(102)), z[1]), z[2])
}

const testVaultAddr = "0x1111111111111111111111111111111111111111"

func checkTestVault(srv *httptest.Server) VaultMerkleStatus {
	return checkVault(srv.Client(), srv.URL, "Erc20CoinVault", testVaultAddr, 3,
		keccak32Hex("LeafInserted(uint256,uint256,uint256,uint256)"), keccak4("currentRoot()"), keccak4("treeNumber()"))
}

// Each leaf goes where its LeafInserted event says: tree 1 holds the second
// insert's two leaves, and its number and root match the vault's.
func TestCheckVaultRebuildsTreesFromLeafPositions(t *testing.T) {
	root := boundaryRoot()
	s := checkTestVault(fakeVaultRPC(t, boundaryLogs, 1, root))
	if s.Error != "" || !s.Match || s.LocalTreeNumber != 1 || s.LocalRoot != root.String() || s.LeafCount != 8 {
		t.Fatalf("status %+v, want a match on tree 1, root %s, from 8 leaves", s, root)
	}
	if got := s.Tree.Levels[0]; len(got) != 2 || got[0] != "101" || got[1] != "102" {
		t.Errorf("the current tree's leaves are %v, want [101 102]", got)
	}
}

// The same root on another tree number is no match.
func TestCheckVaultComparesTheTreeNumber(t *testing.T) {
	s := checkTestVault(fakeVaultRPC(t, boundaryLogs, 2, boundaryRoot()))
	if s.Error != "" || s.Match || s.TreeNumber != 2 || s.LocalTreeNumber != 1 {
		t.Fatalf("status %+v, want no match: the vault is on tree 2, the rebuild on tree 1", s)
	}
}

// A leaf out of sequence (a gap a missing event leaves, a skipped tree, or
// a new tree that does not start at index 0), one past the tree's
// capacity, or a log of another shape stops the rebuild with an error, and
// no local root.
func TestCheckVaultRefusesLeavesOutOfSequence(t *testing.T) {
	gap := append(append([]string{}, boundaryLogs[:2]...), boundaryLogs[3:]...)
	var full []string
	for i := range 9 {
		full = append(full, leafLogJSON(0, i, int64(i+1)))
	}
	// then is boundaryLogs' first two logs and log.
	then := func(log string) []string { return append(append([]string{}, boundaryLogs[:2]...), log) }
	for name, tc := range map[string]struct {
		logs []string
		want string
	}{
		"a gap":                       {gap, "log[2]: leaf at tree 0 index 3, want tree 0 index 2 or tree 1 index 0"},
		"a tree skipped":              {then(leafLogJSON(2, 0, 9)), "log[2]: leaf at tree 2 index 0, want tree 0 index 2 or tree 1 index 0"},
		"the next tree past index 0":  {then(leafLogJSON(1, 1, 9)), "log[2]: leaf at tree 1 index 1, want tree 0 index 2 or tree 1 index 0"},
		"past the capacity":           {full, "log[8]: leaf at tree 0 index 8: a tree of depth 3 holds 8 leaves"},
		"no data":                     {then(`{"topics":["0x01","0x02","0x03"],"data":"0x"}`), "log[2]: data \"0x\" is not two 32-byte words"},
		"two topics":                  {then(`{"topics":["0x01","0x02"],"data":"0x"}`), "log[2]: expected 3 topics, got 2"},
		"a commitment that isn't hex": {then(`{"topics":["0x01","0x02","0xzz"],"data":"0x"}`), "log[2]: cannot parse commitment \"0xzz\""},
		"a tree number past 2^31":     {then(fmt.Sprintf(`{"topics":["0x01","0x02","0x03"],"data":"0x%064x%064x"}`, int64(1)<<40, 0)), "log[2]: tree 1099511627776 index 0 out of range"},
	} {
		t.Run(name, func(t *testing.T) {
			s := checkTestVault(fakeVaultRPC(t, tc.logs, 1, boundaryRoot()))
			if s.Error != tc.want || s.LocalRoot != "" || s.Match {
				t.Fatalf("status %+v, want error %q and no local root", s, tc.want)
			}
		})
	}
}
