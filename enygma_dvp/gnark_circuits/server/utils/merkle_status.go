package utils

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	poseidonLib "github.com/iden3/go-iden3-crypto/poseidon"
	"golang.org/x/crypto/sha3"
)

// ─── Local Merkle Tree ────────────────────────────────────────────────────────
// Mirrors src/core/merkle.go so we can reconstruct the tree from on-chain events.

var merkleSnarkField, _ = new(big.Int).SetString(
	"21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)

type localMerkleTree struct {
	depth int
	zeros []*big.Int
	tree  [][]*big.Int
}

func localPoseidon2(left, right *big.Int) *big.Int {
	h, err := poseidonLib.Hash([]*big.Int{left, right})
	if err != nil {
		panic(fmt.Sprintf("poseidon2: %v", err))
	}
	return h
}

func localZeroValue() *big.Int {
	h := sha3.NewLegacyKeccak256()
	h.Write([]byte("ZkDvp"))
	b := h.Sum(nil)
	v := new(big.Int).SetBytes(b)
	v.Mod(v, merkleSnarkField)
	return v
}

func buildZeroLevels(depth int) []*big.Int {
	z := make([]*big.Int, depth)
	z[0] = localZeroValue()
	for i := 1; i < depth; i++ {
		z[i] = localPoseidon2(z[i-1], z[i-1])
	}
	return z
}

func newLocalMerkleTree(depth int) *localMerkleTree {
	mt := &localMerkleTree{depth: depth}
	mt.zeros = buildZeroLevels(depth)
	mt.tree = make([][]*big.Int, depth+1)
	for i := 0; i <= depth; i++ {
		mt.tree[i] = make([]*big.Int, 0)
	}
	mt.tree[depth] = []*big.Int{localPoseidon2(mt.zeros[depth-1], mt.zeros[depth-1])}
	return mt
}

func (mt *localMerkleTree) resetToEmpty() {
	mt.tree = make([][]*big.Int, mt.depth+1)
	for i := 0; i <= mt.depth; i++ {
		mt.tree[i] = make([]*big.Int, 0)
	}
	mt.tree[mt.depth] = []*big.Int{localPoseidon2(mt.zeros[mt.depth-1], mt.zeros[mt.depth-1])}
}

func (mt *localMerkleTree) insertLeaf(leaf *big.Int) {
	maxLeaves := 1 << mt.depth
	if len(mt.tree[0])+1 >= maxLeaves {
		mt.resetToEmpty()
	}
	mt.tree[0] = append(mt.tree[0], leaf)
	mt.rebuildSparse()
}

func (mt *localMerkleTree) rebuildSparse() {
	for level := 0; level < mt.depth; level++ {
		mt.tree[level+1] = mt.tree[level+1][:0]
		for pos := 0; pos < len(mt.tree[level]); pos += 2 {
			right := mt.zeros[level]
			if pos+1 < len(mt.tree[level]) {
				right = mt.tree[level][pos+1]
			}
			mt.tree[level+1] = append(mt.tree[level+1],
				localPoseidon2(mt.tree[level][pos], right))
		}
	}
}

func (mt *localMerkleTree) root() string {
	return mt.tree[mt.depth][0].String()
}

// TreeOutput is the serialisable snapshot of the Merkle tree.
// Levels[0] = leaves, Levels[depth] = [root].
// Each level only contains the explicitly computed nodes; implied zero-filled
// siblings are omitted (use Zeros[level] for the zero value at that level).
type TreeOutput struct {
	Depth  int        `json:"depth"`
	Root   string     `json:"root"`
	Levels [][]string `json:"levels"`
	Zeros  []string   `json:"zeros"`
}

func (mt *localMerkleTree) snapshot() TreeOutput {
	levels := make([][]string, mt.depth+1)
	for i, nodes := range mt.tree {
		levels[i] = make([]string, len(nodes))
		for j, v := range nodes {
			levels[i][j] = v.String()
		}
	}
	zeros := make([]string, mt.depth)
	for i, z := range mt.zeros {
		zeros[i] = z.String()
	}
	return TreeOutput{
		Depth:  mt.depth,
		Root:   mt.root(),
		Levels: levels,
		Zeros:  zeros,
	}
}

// ─── JSON-RPC helpers ─────────────────────────────────────────────────────────

// keccak4 returns the first 4 bytes of the keccak256 hash of sig as a hex string
// (no 0x prefix), suitable for use as an ABI function selector.
func keccak4(sig string) string {
	h := sha3.NewLegacyKeccak256()
	h.Write([]byte(sig))
	return hex.EncodeToString(h.Sum(nil)[:4])
}

// keccak32Hex returns the full 32-byte keccak256 hash of sig as a 0x-prefixed hex string.
func keccak32Hex(sig string) string {
	h := sha3.NewLegacyKeccak256()
	h.Write([]byte(sig))
	return "0x" + hex.EncodeToString(h.Sum(nil))
}

type jsonRPCRequest struct {
	Jsonrpc string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type jsonRPCResponse struct {
	Result interface{}   `json:"result"`
	Error  *jsonRPCError `json:"error"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func doRPC(client *http.Client, rpcURL string, req jsonRPCRequest) (jsonRPCResponse, error) {
	body, _ := json.Marshal(req)
	// client is built by newSafeRPCClient — its DialContext is pinned to the
	// already-validated IP for rpcURL's host, so this is not the SSRF sink
	// gosec would otherwise flag: an attacker-supplied rpcURL can no longer
	// resolve (at connection time) to a different, disallowed address than
	// the one newSafeRPCClient checked.
	resp, err := client.Post(rpcURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return jsonRPCResponse{}, err
	}
	defer resp.Body.Close()
	var result jsonRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return jsonRPCResponse{}, err
	}
	if result.Error != nil {
		return jsonRPCResponse{}, fmt.Errorf("rpc error %d: %s", result.Error.Code, result.Error.Message)
	}
	return result, nil
}

// ethCallUint256 calls a no-argument view function and returns the result as *big.Int.
func ethCallUint256(client *http.Client, rpcURL, contractAddr, selectorHex string) (*big.Int, error) {
	resp, err := doRPC(client, rpcURL, jsonRPCRequest{
		Jsonrpc: "2.0",
		Method:  "eth_call",
		Params: []interface{}{
			map[string]string{"to": contractAddr, "data": "0x" + selectorHex},
			"latest",
		},
		ID: 1,
	})
	if err != nil {
		return nil, err
	}
	hexStr, ok := resp.Result.(string)
	if !ok {
		return nil, fmt.Errorf("eth_call: unexpected result type %T", resp.Result)
	}
	hexStr = strings.TrimPrefix(hexStr, "0x")
	if hexStr == "" {
		return big.NewInt(0), nil
	}
	v, ok2 := new(big.Int).SetString(hexStr, 16)
	if !ok2 {
		return nil, fmt.Errorf("eth_call: cannot parse hex %q", hexStr)
	}
	return v, nil
}

// ethCallAddress calls a function that takes a single uint256 and returns an address.
// selectorHex is 4 bytes (no 0x prefix); arg is ABI-encoded as a 32-byte big-endian word.
func ethCallAddress(client *http.Client, rpcURL, contractAddr, selectorHex string, arg uint64) (string, error) {
	data := "0x" + selectorHex + fmt.Sprintf("%064x", arg)
	resp, err := doRPC(client, rpcURL, jsonRPCRequest{
		Jsonrpc: "2.0",
		Method:  "eth_call",
		Params: []interface{}{
			map[string]string{"to": contractAddr, "data": data},
			"latest",
		},
		ID: 1,
	})
	if err != nil {
		return "", err
	}
	hexStr, ok := resp.Result.(string)
	if !ok {
		return "", fmt.Errorf("eth_call: unexpected result type %T", resp.Result)
	}
	hexStr = strings.TrimPrefix(hexStr, "0x")
	if len(hexStr) < 40 {
		return "", fmt.Errorf("eth_call: result too short to contain an address: %q", hexStr)
	}
	// ABI-encoded address: 32 bytes, address in the last 20 bytes (rightmost 40 hex chars)
	addr := "0x" + hexStr[len(hexStr)-40:]
	return strings.ToLower(addr), nil
}

type logEntry struct {
	Topics []string `json:"topics"`
}

// ethGetLogs fetches all logs for the given contract matching topic0.
func ethGetLogs(client *http.Client, rpcURL, contractAddr, topic0 string) ([]logEntry, error) {
	resp, err := doRPC(client, rpcURL, jsonRPCRequest{
		Jsonrpc: "2.0",
		Method:  "eth_getLogs",
		Params: []interface{}{
			map[string]interface{}{
				"fromBlock": "0x0",
				"toBlock":   "latest",
				"address":   contractAddr,
				"topics":    []interface{}{topic0},
			},
		},
		ID: 1,
	})
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, err
	}
	var logs []logEntry
	if err := json.Unmarshal(raw, &logs); err != nil {
		return nil, err
	}
	return logs, nil
}

// ─── Input validation ─────────────────────────────────────────────────────────

// validateReceiptsPath rejects absolute paths and traversals that escape more
// than one directory level above cwd (e.g. ../../etc/passwd).
// The default "../build/receipts.json" (one level up) is intentionally allowed.
func validateReceiptsPath(p string) error {
	if filepath.IsAbs(p) {
		return fmt.Errorf("receiptsPath must be a relative path")
	}
	clean := filepath.ToSlash(filepath.Clean(p))
	depth := 0
	for _, seg := range strings.Split(clean, "/") {
		if seg == ".." {
			depth++
			if depth > 1 {
				return fmt.Errorf("receiptsPath traverses outside the project directory")
			}
		}
	}
	return nil
}

// newSafeRPCClient validates a caller-supplied rpcUrl and returns an
// *http.Client whose outbound connection is pinned to the specific IP this
// function already checked — closing the two gaps a bare scheme check
// leaves open (this endpoint's own history: the scheme check alone shipped
// first, with a //nolint:gosec on the http.Post call, before this was
// fixed):
//
//  1. SSRF to internal/cloud-metadata targets. rpcUrl is taken directly from
//     the request body and the RPC responses (on-chain data fetched FROM
//     that address) are echoed back to the caller — the exact "attacker
//     controls the URL, and the fetch result comes back" shape that turns
//     an SSRF into a way to steal cloud credentials (e.g. a request for
//     http://169.254.169.254/latest/meta-data/iam/security-credentials/...
//     with the IMDS response reflected in the JSON response body). Resolves
//     the hostname and rejects link-local (169.254.0.0/16 / fe80::/10 —
//     covers every major cloud's metadata endpoint) and private-network
//     (RFC1918 / fc00::/7) addresses. Loopback is deliberately still
//     allowed: this handler is itself bound to 127.0.0.1 only (see
//     main.go), so it grants an attacker no new reach beyond what they'd
//     already need to have to call this endpoint at all, and 127.0.0.1 is
//     this tool's own documented, primary use case (a local dev node).
//  2. TOCTOU / DNS rebinding. Validating the hostname and then handing the
//     same hostname to http.Client for a second, independent DNS lookup at
//     connect time leaves a window where the attacker's DNS server answers
//     safely for the check and unsafely for the real connection. The
//     returned client's Transport.DialContext ignores the addr it's given
//     and always dials the IP validated here — the connection cannot land
//     anywhere else, no matter what a second lookup would have returned.
//  3. Redirects. CheckRedirect refuses to follow any redirect, per this
//     class of finding's own standard remediation — a same-URL response
//     that then 302s to a disallowed target would otherwise bypass all of
//     the above.
func newSafeRPCClient(raw string) (*http.Client, error) {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid rpcUrl: %v", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("rpcUrl must use http or https scheme, got %q", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("rpcUrl has no host")
	}

	safeIP, err := resolveToSafeIP(host)
	if err != nil {
		return nil, err
	}

	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	pinnedAddr := net.JoinHostPort(safeIP.String(), port)

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			// Ignore the addr http.Transport computed from the request URL
			// (a second hostname resolution) and dial the pre-validated IP
			// directly — this is what actually closes the rebinding gap,
			// not just the check above on its own.
			return dialer.DialContext(ctx, network, pinnedAddr)
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return fmt.Errorf("refusing to follow redirect to %s", req.URL)
		},
	}, nil
}

// resolveToSafeIP resolves host (an IP literal or a DNS name) and returns
// one address that is not link-local or private-use — see newSafeRPCClient
// for why loopback is the one exception.
func resolveToSafeIP(host string) (net.IP, error) {
	var candidates []net.IP
	if ip := net.ParseIP(host); ip != nil {
		candidates = []net.IP{ip}
	} else {
		addrs, err := net.LookupIP(host)
		if err != nil {
			return nil, fmt.Errorf("resolve rpcUrl host %q: %v", host, err)
		}
		candidates = addrs
	}

	for _, ip := range candidates {
		if ip.IsLoopback() {
			return ip, nil
		}
		if ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
			ip.IsUnspecified() || ip.IsMulticast() {
			continue
		}
		return ip, nil
	}
	return nil, fmt.Errorf("rpcUrl host %q resolves only to disallowed addresses (private, link-local, or unspecified) — cloud metadata and internal-network targets are blocked; point rpcUrl at a public or loopback RPC endpoint", host)
}

// ─── Receipts ─────────────────────────────────────────────────────────────────

type contractReceipt struct {
	ContractAddress string `json:"contractAddress"`
}

var vaultNames = []string{"Erc20CoinVault", "Erc721CoinVault", "Erc1155CoinVault", "EnygmaErc20CoinVault"}

func loadVaultAddresses(receiptsPath string) (map[string]string, error) {
	data, err := os.ReadFile(receiptsPath)
	if err != nil {
		return nil, err
	}
	var all map[string]contractReceipt
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, err
	}
	out := make(map[string]string)
	for _, name := range vaultNames {
		if r, ok := all[name]; ok {
			out[name] = r.ContractAddress
		}
	}
	return out, nil
}

// ─── Handler ──────────────────────────────────────────────────────────────────

type MerkleStatusRequest struct {
	RpcUrl       string `json:"rpcUrl"`
	ReceiptsPath string `json:"receiptsPath"`
}

type VaultMerkleStatus struct {
	Name        string     `json:"name"`
	Address     string     `json:"address"`
	OnChainRoot string     `json:"onChainRoot"`
	LocalRoot   string     `json:"localRoot"`
	Match       bool       `json:"match"`
	LeafCount   int        `json:"leafCount"`
	TreeNumber  uint64     `json:"treeNumber"`
	Tree        TreeOutput `json:"tree"`
	Error       string     `json:"error,omitempty"`
}

// VaultRegistryEntry is one row of the EnygmaDvP registry cross-check.
type VaultRegistryEntry struct {
	VaultID         uint64 `json:"vaultId"`
	Name            string `json:"name"`
	AddressInDvP    string `json:"addressInDvP"`    // from vaultById(id) on EnygmaDvP
	AddressInReceipts string `json:"addressInReceipts"` // from receipts.json
	Match           bool   `json:"match"`
}

// EnygmaDvPCheck holds the result of comparing receipts.json vault addresses
// against what EnygmaDvP has registered on-chain via vaultById(id).
type EnygmaDvPCheck struct {
	EnygmaDvPAddress string               `json:"enygmaDvpAddress"`
	AllMatch         bool                 `json:"allMatch"`
	Entries          []VaultRegistryEntry `json:"entries"`
	Error            string               `json:"error,omitempty"`
}

type MerkleStatusResponse struct {
	EnygmaDvP EnygmaDvPCheck      `json:"enygmaDvpRegistryCheck"`
	Vaults    []VaultMerkleStatus `json:"vaults"`
}

// MerkleStatusHandler handles POST /util/merkleStatus.
// It reconstructs each vault's Merkle tree locally from on-chain Commitment events
// and compares the computed root against the vault's currentRoot().
//
// Request body (all fields optional):
//
//	{ "rpcUrl": "http://127.0.0.1:8545", "receiptsPath": "../build/receipts.json" }
func MerkleStatusHandler() gin.HandlerFunc {
	const treeDepth = 8

	commitmentTopic := keccak32Hex("Commitment(uint256,uint256)")
	currentRootSel := keccak4("currentRoot()")
	treeNumberSel  := keccak4("treeNumber()")
	vaultByIDSel   := keccak4("vaultById(uint256)")

	return func(c *gin.Context) {
		var req MerkleStatusRequest
		_ = c.ShouldBindJSON(&req) // all fields optional
		if req.RpcUrl == "" {
			req.RpcUrl = "http://127.0.0.1:8545"
		}
		if req.ReceiptsPath == "" {
			req.ReceiptsPath = "../build/receipts.json"
		}

		rpcClient, err := newSafeRPCClient(req.RpcUrl)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := validateReceiptsPath(req.ReceiptsPath); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		vaultAddrs, err := loadVaultAddresses(req.ReceiptsPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError,
				gin.H{"error": fmt.Sprintf("load receipts: %v", err)})
			return
		}

		// Cross-check vault addresses registered in EnygmaDvP vs receipts.json.
		dvpCheck := checkEnygmaDvPRegistry(rpcClient, req.RpcUrl, req.ReceiptsPath, vaultAddrs, vaultByIDSel)

		statuses := make([]VaultMerkleStatus, 0, len(vaultNames))
		for _, name := range vaultNames {
			addr, ok := vaultAddrs[name]
			if !ok {
				statuses = append(statuses, VaultMerkleStatus{
					Name:  name,
					Error: "not found in receipts.json",
				})
				continue
			}
			s := checkVault(rpcClient, req.RpcUrl, name, addr, treeDepth,
				commitmentTopic, currentRootSel, treeNumberSel)
			statuses = append(statuses, s)
		}

		c.JSON(http.StatusOK, MerkleStatusResponse{EnygmaDvP: dvpCheck, Vaults: statuses})
	}
}

// vaultIDByName maps each vault contract name to its on-chain vaultId (position
// in EnygmaDvP._coinVaults[], assigned in registration order by deploy/init).
var vaultIDByName = map[string]uint64{
	"Erc20CoinVault":        0,
	"Erc721CoinVault":       1,
	"Erc1155CoinVault":      2,
	"EnygmaErc20CoinVault":  3,
}

func checkEnygmaDvPRegistry(client *http.Client, rpcURL, receiptsPath string, receiptsAddrs map[string]string, vaultByIDSel string) EnygmaDvPCheck {
	// Load EnygmaDvP address from receipts.
	data, err := os.ReadFile(receiptsPath)
	if err != nil {
		return EnygmaDvPCheck{Error: fmt.Sprintf("read receipts: %v", err)}
	}
	var all map[string]contractReceipt
	if err := json.Unmarshal(data, &all); err != nil {
		return EnygmaDvPCheck{Error: fmt.Sprintf("parse receipts: %v", err)}
	}
	dvpReceipt, ok := all["EnygmaDvp"]
	if !ok {
		return EnygmaDvPCheck{Error: "EnygmaDvp not found in receipts.json"}
	}
	dvpAddr := dvpReceipt.ContractAddress

	entries := make([]VaultRegistryEntry, 0, len(vaultNames))
	allMatch := true

	for _, name := range vaultNames {
		id := vaultIDByName[name]
		onChainAddr, err := ethCallAddress(client, rpcURL, dvpAddr, vaultByIDSel, id)
		if err != nil {
			entries = append(entries, VaultRegistryEntry{
				VaultID: id,
				Name:    name,
				AddressInDvP:      fmt.Sprintf("error: %v", err),
				AddressInReceipts: strings.ToLower(receiptsAddrs[name]),
				Match: false,
			})
			allMatch = false
			continue
		}
		receiptAddr := strings.ToLower(receiptsAddrs[name])
		match := onChainAddr == receiptAddr
		if !match {
			allMatch = false
		}
		entries = append(entries, VaultRegistryEntry{
			VaultID:           id,
			Name:              name,
			AddressInDvP:      onChainAddr,
			AddressInReceipts: receiptAddr,
			Match:             match,
		})
	}

	return EnygmaDvPCheck{
		EnygmaDvPAddress: dvpAddr,
		AllMatch:         allMatch,
		Entries:          entries,
	}
}

func checkVault(client *http.Client, rpcURL, name, addr string, depth int,
	commitmentTopic, currentRootSel, treeNumberSel string,
) VaultMerkleStatus {
	s := VaultMerkleStatus{Name: name, Address: addr}

	// 1. On-chain current root
	onChainRoot, err := ethCallUint256(client, rpcURL, addr, currentRootSel)
	if err != nil {
		s.Error = fmt.Sprintf("currentRoot(): %v", err)
		return s
	}
	s.OnChainRoot = onChainRoot.String()

	// 2. On-chain tree number (how many times the tree has rolled over)
	treeNum, err := ethCallUint256(client, rpcURL, addr, treeNumberSel)
	if err != nil {
		s.Error = fmt.Sprintf("treeNumber(): %v", err)
		return s
	}
	s.TreeNumber = treeNum.Uint64()

	// 3. Collect all Commitment events in order
	//    event Commitment(uint256 indexed vaultId, uint256 indexed commitment)
	//    topics[0] = event signature, topics[1] = vaultId, topics[2] = commitment
	logs, err := ethGetLogs(client, rpcURL, addr, commitmentTopic)
	if err != nil {
		s.Error = fmt.Sprintf("eth_getLogs: %v", err)
		return s
	}
	s.LeafCount = len(logs)

	// 4. Replay insertions into a local Merkle tree and compute root
	mt := newLocalMerkleTree(depth)
	for i, lg := range logs {
		if len(lg.Topics) < 3 {
			s.Error = fmt.Sprintf("log[%d]: expected 3 topics, got %d", i, len(lg.Topics))
			return s
		}
		hexVal := strings.TrimPrefix(lg.Topics[2], "0x")
		leaf, ok := new(big.Int).SetString(hexVal, 16)
		if !ok {
			s.Error = fmt.Sprintf("log[%d]: cannot parse commitment %q", i, lg.Topics[2])
			return s
		}
		mt.insertLeaf(leaf)
	}

	s.LocalRoot = mt.root()
	s.Match = s.LocalRoot == s.OnChainRoot
	s.Tree = mt.snapshot()
	return s
}

// ─── Per-vault handler ────────────────────────────────────────────────────────

// vaultNameByID is the reverse of vaultIDByName.
var vaultNameByID = func() map[uint64]string {
	m := make(map[uint64]string, len(vaultIDByName))
	for name, id := range vaultIDByName {
		m[id] = name
	}
	return m
}()

type MerkleVaultRequest struct {
	// Identify the vault by name OR by id — one is required.
	Vault        string `json:"vault"`        // e.g. "Erc20CoinVault"
	VaultID      *uint64 `json:"vaultId"`     // e.g. 0  (pointer so 0 is distinguishable from absent)
	RpcUrl       string  `json:"rpcUrl"`
	ReceiptsPath string  `json:"receiptsPath"`
}

// MerkleVaultHandler handles POST /util/merkleVault.
// Returns the full Merkle tree status for a single vault identified by name or vaultId.
//
// Request examples:
//
//	{ "vault": "Erc20CoinVault" }
//	{ "vaultId": 0 }
//	{ "vault": "Erc20CoinVault", "rpcUrl": "http://127.0.0.1:8545" }
func MerkleVaultHandler() gin.HandlerFunc {
	const treeDepth = 8

	commitmentTopic := keccak32Hex("Commitment(uint256,uint256)")
	currentRootSel := keccak4("currentRoot()")
	treeNumberSel  := keccak4("treeNumber()")

	return func(c *gin.Context) {
		var req MerkleVaultRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.RpcUrl == "" {
			req.RpcUrl = "http://127.0.0.1:8545"
		}
		if req.ReceiptsPath == "" {
			req.ReceiptsPath = "../build/receipts.json"
		}

		rpcClient, err := newSafeRPCClient(req.RpcUrl)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := validateReceiptsPath(req.ReceiptsPath); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Resolve vault name from either field.
		vaultName := req.Vault
		if vaultName == "" && req.VaultID != nil {
			name, ok := vaultNameByID[*req.VaultID]
			if !ok {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":       fmt.Sprintf("unknown vaultId %d", *req.VaultID),
					"validIds":    []uint64{0, 1, 2, 3},
					"validVaults": vaultNames,
				})
				return
			}
			vaultName = name
		}
		if vaultName == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":       "provide either \"vault\" (name) or \"vaultId\" (0–3)",
				"validVaults": vaultNames,
			})
			return
		}

		// Validate name.
		found := false
		for _, n := range vaultNames {
			if n == vaultName {
				found = true
				break
			}
		}
		if !found {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":       fmt.Sprintf("unknown vault %q", vaultName),
				"validVaults": vaultNames,
			})
			return
		}

		// Load address from receipts.
		vaultAddrs, err := loadVaultAddresses(req.ReceiptsPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError,
				gin.H{"error": fmt.Sprintf("load receipts: %v", err)})
			return
		}
		addr, ok := vaultAddrs[vaultName]
		if !ok {
			c.JSON(http.StatusInternalServerError,
				gin.H{"error": fmt.Sprintf("%s not found in receipts.json", vaultName)})
			return
		}

		status := checkVault(rpcClient, req.RpcUrl, vaultName, addr, treeDepth,
			commitmentTopic, currentRootSel, treeNumberSel)
		c.JSON(http.StatusOK, status)
	}
}
