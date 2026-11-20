// Command verify_deployed_verifier checks a reviewed Hardhat artifact against
// its Groth16 verifying key and optionally compares the deployed runtime
// byte-for-byte against that artifact.
package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	utils "enygma_payments/gnark-server/utils"

	"github.com/consensys/gnark-crypto/ecc"
)

var constantRe = regexp.MustCompile(`(?m)^\s*uint256\s+constant\s+(\w+)\s*=\s*(\d+)\s*;`)

func main() {
	vkPath := flag.String("vk", "", "path to the verifying key file to check against (required)")
	artifactPath := flag.String("artifact", "", "reviewed Hardhat artifact JSON (required)")
	rpcURL := flag.String("rpc", "", "JSON-RPC URL of a chain to fetch deployed bytecode from (use with -address)")
	address := flag.String("address", "", "deployed verifier contract address (use with -rpc)")
	flag.Parse()

	if *vkPath == "" || *artifactPath == "" || (*rpcURL == "") != (*address == "") {
		fmt.Fprintln(os.Stderr, "error: -vk and -artifact are required; pass -rpc and -address together to verify a deployment")
		flag.Usage()
		os.Exit(2)
	}
	if err := verifyRuntime(*vkPath, *artifactPath, *rpcURL, *address); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("PASS: reviewed verifier artifact checked against the verifying key")
	if *rpcURL != "" {
		fmt.Println("PASS: deployed runtime exactly matches the reviewed artifact")
	}
}

func verifyRuntime(vkPath, artifactPath, rpcURL, address string) error {
	expected, err := expectedConstants(vkPath)
	if err != nil {
		return err
	}
	artifactHex, err := bytecodeFromArtifact(artifactPath)
	if err != nil {
		return err
	}
	reviewed, err := decodeRuntime(artifactHex)
	if err != nil {
		return fmt.Errorf("reviewed artifact: %w", err)
	}
	if missing := findMissing(expected, hex.EncodeToString(reviewed)); len(missing) != 0 {
		return fmt.Errorf("reviewed artifact is missing verifying-key constants: %s", strings.Join(missing, ", "))
	}
	if rpcURL == "" {
		return nil
	}
	actualHex, err := bytecodeFromRPC(rpcURL, address)
	if err != nil {
		return err
	}
	actual, err := decodeRuntime(actualHex)
	if err != nil {
		return fmt.Errorf("deployed verifier: %w", err)
	}
	if !bytes.Equal(reviewed, actual) {
		return fmt.Errorf("deployed verifier runtime differs from the reviewed artifact")
	}
	return nil
}

func decodeRuntime(raw string) ([]byte, error) {
	code, err := hex.DecodeString(strings.TrimPrefix(raw, "0x"))
	if err != nil {
		return nil, fmt.Errorf("decode runtime bytecode: %w", err)
	}
	if len(code) == 0 {
		return nil, fmt.Errorf("empty runtime bytecode")
	}
	return code, nil
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
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(rpcURL, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("eth_getCode request to %s: %w", rpcURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("eth_getCode returned HTTP %d", resp.StatusCode)
	}

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
