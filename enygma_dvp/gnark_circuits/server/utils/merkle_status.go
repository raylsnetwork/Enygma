package utils

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// maxRPCResponseBytes caps how much of an RPC response doRPC will buffer in
// memory. 64 MiB is generous for a full-history eth_getLogs scan (see
// ethGetLogs) while still bounding what an allowed-but-hostile target can
// force this process to hold in memory per request.
const maxRPCResponseBytes = 64 << 20 // 64 MiB

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
	// newSafeRPCClient's job is blocking disallowed *targets*; it says
	// nothing about how much an allowed-but-hostile target is permitted to
	// send back. Without a cap here, decode buffers the entire body in
	// memory regardless of size — bounded only by rpcClientTimeout, not by
	// bytes. Read up to the limit +1 so an oversized body is detected
	// (rather than silently truncated and fed to the JSON decoder).
	limited := io.LimitReader(resp.Body, maxRPCResponseBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return jsonRPCResponse{}, err
	}
	if len(data) > maxRPCResponseBytes {
		return jsonRPCResponse{}, fmt.Errorf("rpc response exceeds %d byte limit", maxRPCResponseBytes)
	}
	var result jsonRPCResponse
	if err := json.Unmarshal(data, &result); err != nil {
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
	// big.Int.SetString accepts a leading '-' even in base 16, so a
	// malformed/adversarial response like {"result":"0x-1"} would otherwise
	// parse "successfully" into a negative value. Every caller of this
	// function eventually narrows the result via Uint64(), which silently
	// returns the two's-complement magnitude for a negative big.Int instead
	// of erroring — turning "0x-1" into a plausible-looking 1 instead of a
	// rejected response. Reject negative values here, at the parse site,
	// rather than relying on every downstream Uint64() call to notice.
	if v.Sign() < 0 {
		return nil, fmt.Errorf("eth_call: result %q is negative, expected an unsigned uint256", hexStr)
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

// RequireLoopbackBind panics if bindAddr's host is not a loopback address.
// resolveToSafeIP's rpcUrl SSRF check deliberately allows loopback as a
// *target* address, on the assumption that this server itself is only ever
// reachable via loopback — an attacker able to reach it at all already has
// loopback access, so allowing loopback as a target grants nothing new.
// That assumption previously lived only in a comment; call this from
// main() with the exact address passed to router.Run so a future change to
// the bind address (e.g. containerizing onto 0.0.0.0) fails loudly at
// startup instead of silently reopening loopback as an SSRF target for any
// remote caller.
func RequireLoopbackBind(bindAddr string) {
	host, _, err := net.SplitHostPort(bindAddr)
	if err != nil {
		host = bindAddr // bindAddr may be host-only, e.g. "127.0.0.1"
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		panic(fmt.Sprintf("utils.RequireLoopbackBind: bind address %q is not loopback-only — "+
			"resolveToSafeIP's rpcUrl SSRF check allows loopback targets on the assumption this "+
			"server itself is loopback-bound; update both together", bindAddr))
	}
}

// rpcClientOrAbort builds a safe RPC client for rpcUrl, or writes the 400
// JSON error response itself and returns ok=false if rpcUrl is invalid or
// disallowed. Shared by MerkleStatusHandler and MerkleVaultHandler, which
// previously duplicated this exact four-line block, so a future change to
// how a bad rpcUrl is reported only has to be made in one place.
func rpcClientOrAbort(c *gin.Context, rpcUrl string) (*http.Client, bool) {
	client, err := newSafeRPCClient(rpcUrl)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return nil, false
	}
	return client, true
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
//     allowed: this handler is itself bound to 127.0.0.1 only, and
//     RequireLoopbackBind (called from main.go at startup) turns a future
//     change to that bind address into a startup panic instead of silently
//     reopening loopback as an SSRF target — so it grants an attacker no
//     new reach beyond what they'd already need to have to call this
//     endpoint at all, and 127.0.0.1 is this tool's own documented,
//     primary use case (a local dev node).
//  2. TOCTOU / DNS rebinding. Validating the hostname and then handing the
//     same hostname to http.Client for a second, independent DNS lookup at
//     connect time leaves a window where the attacker's DNS server answers
//     safely for the check and unsafely for the real connection. The
//     returned client's Transport.DialContext ignores the addr it's given
//     and always dials the IP validated here — the connection cannot land
//     anywhere else, no matter what a second lookup would have returned.
//  3. Redirects. CheckRedirect refuses any redirect to a different host
//     (comparing the URL's hostname, not a re-resolution of it) — a
//     same-address response that then 302s to a disallowed target is
//     exactly the bypass this blocks. A redirect that stays on the same
//     already-validated host (e.g. a same-host http->https upgrade some
//     RPC gateways perform) is allowed and only updates the pinned port —
//     the hostname is still never re-resolved, so this carries none of the
//     rebinding risk (2) exists to close.
//
// rpcClientTimeout bounds newSafeRPCClient's http.Client.Timeout — see the
// comment where it's used for why this can't just be the previous 15s.
const rpcClientTimeout = 120 * time.Second

// idleConnTimeout bounds how long this client's Transport keeps an idle
// connection open. Each call to newSafeRPCClient builds a brand-new
// *http.Transport (rather than sharing one across requests), so without a
// finite value here (the zero value means "no limit") its pooled idle
// connections and the goroutines backing them would never be reclaimed by
// anything once the handler that created this client returns.
const idleConnTimeout = 30 * time.Second

// maxRPCRedirects bounds the same-host redirect chain CheckRedirect allows
// below — plain hygiene against a pathological or hostile redirect loop,
// not itself a security boundary (host equality is what does that).
const maxRPCRedirects = 3

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

	// pinned is the address DialContext always dials, regardless of what
	// http.Transport computed from the request URL — this is what closes
	// the rebinding gap (2) above. Its host half is fixed forever at the
	// single resolution safeIP already did; only its port half can change,
	// and only via the same-host redirect case in CheckRedirect below.
	var pinnedMu sync.Mutex
	pinned := net.JoinHostPort(safeIP.String(), port)

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			pinnedMu.Lock()
			addr := pinned
			pinnedMu.Unlock()
			return dialer.DialContext(ctx, network, addr)
		},
		IdleConnTimeout: idleConnTimeout,
	}

	return &http.Client{
		Transport: transport,
		// checkVault's eth_getLogs call always scans fromBlock=0x0..latest
		// (an unbounded full-history query, see ethGetLogs) — against a
		// real/loaded RPC node backing a vault with a long Commitment-event
		// history this can legitimately take well over 15s. The old
		// http.DefaultClient this replaced had Timeout=0 (unlimited); pick
		// a generous-but-bounded value instead of reintroducing "no cap at
		// all" — bounded matters for the SSRF-adjacent goal of not letting
		// an allowed-but-slow/hostile target hold a connection open
		// forever, but it must not be so tight it turns legitimate slow
		// scans into failures that worked before this fix shipped.
		Timeout: rpcClientTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRPCRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRPCRedirects)
			}
			// Hostname equality only — never re-resolved, so a redirect
			// can't be used to smuggle a rebound or disallowed address
			// past the one resolution safeIP came from.
			if req.URL.Hostname() != host {
				return fmt.Errorf("refusing to follow redirect to a different host %q", req.URL.Hostname())
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("refusing to follow redirect to scheme %q", req.URL.Scheme)
			}
			newPort := req.URL.Port()
			if newPort == "" {
				if req.URL.Scheme == "https" {
					newPort = "443"
				} else {
					newPort = "80"
				}
			}
			pinnedMu.Lock()
			pinned = net.JoinHostPort(safeIP.String(), newPort)
			pinnedMu.Unlock()
			return nil
		},
	}, nil
}

// cgnatBlock is RFC 6598 shared address space (100.64.0.0/10), used for
// carrier-grade NAT and, notably, as the range several cloud providers bind
// their instance-metadata endpoint to (e.g. Alibaba Cloud ECS's metadata
// service at 100.100.100.200). Go's net.IP.IsPrivate() only covers RFC1918
// (and RFC4193 for IPv6) and does not include this range, so it has to be
// checked separately or it slips through as "safe".
var cgnatBlock = func() *net.IPNet {
	_, block, err := net.ParseCIDR("100.64.0.0/10")
	if err != nil {
		panic(err) // static literal; cannot fail
	}
	return block
}()

// dnsLookupTimeout bounds resolveToSafeIP's DNS lookup. net.LookupIP takes
// no context/deadline of its own, so without this, an unresponsive
// resolver could block the calling handler goroutine indefinitely — this
// is bounded separately from (and tighter than) rpcClientTimeout, since
// that timeout doesn't exist yet at the point this eager check runs.
const dnsLookupTimeout = 5 * time.Second

// resolveToSafeIP resolves host (an IP literal or a DNS name) and returns
// one address that is not link-local, private-use, or CGNAT space — see
// newSafeRPCClient for why loopback is the one exception.
func resolveToSafeIP(host string) (net.IP, error) {
	var candidates []net.IP
	if ip := net.ParseIP(host); ip != nil {
		candidates = []net.IP{ip}
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), dnsLookupTimeout)
		defer cancel()
		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("resolve rpcUrl host %q: %v", host, err)
		}
		candidates = make([]net.IP, len(addrs))
		for i, a := range addrs {
			candidates[i] = a.IP
		}
	}

	for _, ip := range candidates {
		// Two IPv6 forms carry an IPv4 address in their low 32 bits without
		// being unwrapped by To4() the way the modern IPv4-mapped form
		// (::ffff:a.b.c.d) is, so none of the checks below would otherwise
		// fire even when the embedded address is disallowed. Substitute the
		// embedded IPv4 address before classifying — see embeddedIPv4.
		if v4 := embeddedIPv4(ip); v4 != nil {
			ip = v4
		}
		if ip.IsLoopback() {
			return ip, nil
		}
		// IsMulticast() alone covers IsLinkLocalMulticast() too (the latter
		// is a strict subset in net.IP) — listed once, not as two clauses
		// that would otherwise look like distinct exclusions.
		if ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
			ip.IsUnspecified() || ip.IsMulticast() || cgnatBlock.Contains(ip) {
			continue
		}
		return ip, nil
	}
	return nil, fmt.Errorf("rpcUrl host %q resolves only to disallowed addresses (private, link-local, CGNAT, or unspecified) — cloud metadata and internal-network targets are blocked; point rpcUrl at a public or loopback RPC endpoint", host)
}

// nat64WellKnownPrefix is 64:ff9b::/96 (RFC 6052) — the IANA-assigned
// prefix NAT64/DNS64 gateways use to synthesize an IPv6 address from an
// IPv4 one, regardless of local network config. e.g. 64:ff9b::a9fe:a9fe
// embeds 169.254.169.254, the single highest-value SSRF target this whole
// check exists to block.
var nat64WellKnownPrefix = [12]byte{0x00, 0x64, 0xff, 0x9b}

// embeddedIPv4 returns the IPv4 address embedded in ip if ip is in one of
// two IPv6 forms that carry an IPv4 address in their low 32 bits but, unlike
// the modern IPv4-mapped form (::ffff:a.b.c.d), aren't unwrapped by
// net.IP.To4() — so none of resolveToSafeIP's checks would otherwise fire
// even when the embedded address is disallowed:
//
//  1. The deprecated "IPv4-compatible" form (RFC 4291 §2.5.5.1, ::a.b.c.d).
//  2. The NAT64 Well-Known Prefix (RFC 6052, 64:ff9b::/96) — see
//     nat64WellKnownPrefix.
//
// Returns nil if ip is in neither form. ::1 (loopback) and :: (unspecified)
// are excluded even though they share the same "12 zero bytes" prefix as
// (1), since those are their own distinct, already-correctly-handled IPv6
// addresses, not instances of this deprecated notation.
func embeddedIPv4(ip net.IP) net.IP {
	if ip.To4() != nil || ip.IsLoopback() || ip.IsUnspecified() {
		return nil
	}
	ip16 := ip.To16()
	if ip16 == nil {
		return nil
	}
	var prefix [12]byte
	copy(prefix[:], ip16[:12])
	if prefix != ([12]byte{}) && prefix != nat64WellKnownPrefix {
		return nil
	}
	return net.IPv4(ip16[12], ip16[13], ip16[14], ip16[15])
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

		rpcClient, ok := rpcClientOrAbort(c, req.RpcUrl)
		if !ok {
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

		rpcClient, ok := rpcClientOrAbort(c, req.RpcUrl)
		if !ok {
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
