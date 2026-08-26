// Command verify_deployed_verifier implements L-04 part 3 (also cited
// inside H-12 as "the control L-04 identifies as absent"): a deploy-time
// assertion binding a deployed Solidity verifier contract to the specific
// verifying key it should have been generated from.
//
// The problem it closes: nothing anywhere in this repository previously
// checked that a registered verifier address (addVerifier,
// addWithdrawVerifier, addBurnVerifier, ...) was actually compiled from
// the current key in use. A verifier compiled from a stale key, from
// someone else's key, or swapped after the fact, would be accepted
// silently — that absence is exactly what let H-16's mismatched zkdvp
// verifiers go undetected until manual git-history archaeology found them.
//
// How it works: gnark's own vk.ExportSolidity bakes the verifying key's
// elliptic-curve constants (alpha, beta, gamma, delta, and one point per
// public input) into the generated contract as `uint256 constant NAME =
// <value>;` declarations. Solidity inlines `constant` values directly into
// bytecode as literals wherever they're referenced — there is no SLOAD,
// so the values are recoverable straight from deployed bytecode without
// needing the contract's source or ABI. This tool regenerates the
// *expected* constant set from a VK file and checks that every single one
// of them appears, as its 32-byte big-endian encoding, somewhere in the
// bytecode being checked. A verifier compiled from a different key is
// exceedingly unlikely to share even one 254-bit curve coordinate by
// chance, so a single missing constant is already a conclusive mismatch.
//
// Usage — check a compiled-but-not-yet-deployed artifact (pre-deploy CI gate):
//
//	go run ./cmd/verify_deployed_verifier \
//	  -vk keys/EnygmaVk.key \
//	  -artifact ../contracts/enygma/artifacts/contracts/EnygmaVerifier.sol/Verifier.json
//
// Usage — check an already-deployed contract on a live chain:
//
//	go run ./cmd/verify_deployed_verifier \
//	  -vk keys/EnygmaVk.key \
//	  -rpc http://127.0.0.1:8545 \
//	  -address 0x1234...
//
// Exit code is 0 iff every constant from the VK was found in the bytecode;
// non-zero (with a listing of what's missing) otherwise. Intended to be
// wired into deployment/CI as a hard gate, the same way the audit's
// remediation asks for.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"regexp"
	"strings"

	utils "enygma-server/utils"

	"github.com/consensys/gnark-crypto/ecc"
)

var constantRe = regexp.MustCompile(`(?m)^\s*uint256\s+constant\s+(\w+)\s*=\s*(\d+)\s*;`)

func main() {
	vkPath := flag.String("vk", "", "path to the verifying key file to check against (required)")
	artifactPath := flag.String("artifact", "", "path to a Hardhat artifact JSON with a deployedBytecode field")
	rpcURL := flag.String("rpc", "", "JSON-RPC URL of a chain to fetch deployed bytecode from (use with -address)")
	address := flag.String("address", "", "deployed verifier contract address (use with -rpc)")
	flag.Parse()

	if *vkPath == "" {
		fmt.Fprintln(os.Stderr, "error: -vk is required")
		flag.Usage()
		os.Exit(2)
	}
	if (*artifactPath == "") == (*rpcURL == "" || *address == "") {
		fmt.Fprintln(os.Stderr, "error: pass exactly one of -artifact, or -rpc together with -address")
		flag.Usage()
		os.Exit(2)
	}

	expected, err := expectedConstants(*vkPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("loaded %s: %d embedded constants expected from vk.ExportSolidity\n", *vkPath, len(expected))

	var bytecodeHex string
	if *artifactPath != "" {
		bytecodeHex, err = bytecodeFromArtifact(*artifactPath)
	} else {
		bytecodeHex, err = bytecodeFromRPC(*rpcURL, *address)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	bytecodeHex = strings.ToLower(strings.TrimPrefix(bytecodeHex, "0x"))
	if bytecodeHex == "" {
		fmt.Fprintln(os.Stderr, "error: bytecode is empty — no contract code at that address/artifact")
		os.Exit(1)
	}

	missing := findMissing(expected, bytecodeHex)

	if len(missing) == 0 {
		fmt.Printf("PASS: all %d constants from %s are present in the checked bytecode — this verifier corresponds to this key.\n", len(expected), *vkPath)
		return
	}

	fmt.Printf("FAIL: %d of %d expected constants were NOT found in the checked bytecode:\n", len(missing), len(expected))
	for _, name := range missing {
		fmt.Printf("  - %s\n", name)
	}
	fmt.Println("This verifier does NOT correspond to the given verifying key — do not register it.")
	os.Exit(1)
}

// expectedConstants loads vkPath and regenerates its Solidity export,
// returning every `uint256 constant NAME = value;` declaration it contains.
func expectedConstants(vkPath string) (map[string]*big.Int, error) {
	vk, err := utils.LoadVerifyingKey(ecc.BN254, vkPath)
	if err != nil {
		return nil, fmt.Errorf("load verifying key %q: %w", vkPath, err)
	}

	var buf bytes.Buffer
	if err := vk.ExportSolidity(&buf); err != nil {
		return nil, fmt.Errorf("export solidity from %q: %w", vkPath, err)
	}

	matches := constantRe.FindAllStringSubmatch(buf.String(), -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("found zero `uint256 constant` declarations in the exported Solidity for %q — gnark's ExportSolidity output format may have changed", vkPath)
	}

	out := make(map[string]*big.Int, len(matches))
	for _, m := range matches {
		name, decimal := m[1], m[2]
		v, ok := new(big.Int).SetString(decimal, 10)
		if !ok {
			return nil, fmt.Errorf("could not parse constant %s = %s as a decimal integer", name, decimal)
		}
		out[name] = v
	}
	return out, nil
}

// bytecodeFromArtifact reads a Hardhat-style compiled artifact JSON and
// returns its deployedBytecode field.
func bytecodeFromArtifact(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read artifact %q: %w", path, err)
	}
	var artifact struct {
		DeployedBytecode string `json:"deployedBytecode"`
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		return "", fmt.Errorf("parse artifact %q: %w", path, err)
	}
	if artifact.DeployedBytecode == "" {
		return "", fmt.Errorf("artifact %q has no deployedBytecode field", path)
	}
	return artifact.DeployedBytecode, nil
}

// bytecodeFromRPC fetches the live deployed bytecode at address via a
// plain eth_getCode JSON-RPC call. Deliberately uses only net/http and
// encoding/json (both stdlib) rather than pulling in go-ethereum, which
// gnark-server does not otherwise depend on.
func bytecodeFromRPC(rpcURL, address string) (string, error) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "eth_getCode",
		"params":  []string{address, "latest"},
	})
	resp, err := http.Post(rpcURL, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("eth_getCode request to %s: %w", rpcURL, err)
	}
	defer resp.Body.Close()

	var rpcResp struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return "", fmt.Errorf("decode eth_getCode response: %w", err)
	}
	if rpcResp.Error != nil {
		return "", fmt.Errorf("eth_getCode RPC error: %s", rpcResp.Error.Message)
	}
	return rpcResp.Result, nil
}

// fmtHex is the minimal (unpadded) lowercase big-endian hex encoding of a
// constant's value — what Solidity actually emits as a PUSH immediate,
// since it picks the narrowest PUSH opcode that fits (PUSH1..PUSH32), not
// always PUSH32.
func fmtHex(v *big.Int) string {
	return strings.ToLower(v.Text(16))
}

// findMissing reports which of the expected constants do not appear,
// in their minimal big-endian hex encoding, anywhere in bytecodeHex.
//
// A first version of this assumed every constant needs the full 32-byte
// (PUSH32) encoding and reported four false-positive mismatches against
// this repo's own genuinely-matching keys/EnygmaVk.key /
// EnygmaVerifier.sol pair — four of EnygmaVk's 176 constants happen to
// have a leading zero byte, so solc emits them one byte narrower with no
// zero padding. See main_test.go's TestMatching_HandlesNarrowerPushWidths.
func findMissing(expected map[string]*big.Int, bytecodeHex string) []string {
	bytecodeHex = strings.ToLower(bytecodeHex)
	var missing []string
	for name, val := range expected {
		if !strings.Contains(bytecodeHex, fmtHex(val)) {
			missing = append(missing, name)
		}
	}
	return missing
}
