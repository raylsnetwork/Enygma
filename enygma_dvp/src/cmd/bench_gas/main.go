// bench_gas measures the on-chain gas cost of GenericGroth16Verifier.verify()
// for each active circuit using a go-ethereum simulated backend.
//
// No Hardhat node or gnark server required. The contract is deployed from the
// artifact bytecode in artifacts/. The VK's G2 points are taken from the real
// PrivateMint proving key, so all curve points are valid — verify() returns
// false (pairing check fails with dummy proof) but consumes the same gas as a
// valid proof, since the precompile runs unconditionally.
//
// Run from enygma_dvp/ (repo root):
//
//	CC=/usr/bin/clang go build -o /tmp/bench_gas ./src/cmd/bench_gas && /tmp/bench_gas
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func mustBig(s string) *big.Int {
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic("invalid big.Int: " + s)
	}
	return n
}

// — ABI-compatible structs — //

type G1Point struct {
	X *big.Int `abi:"x"`
	Y *big.Int `abi:"y"`
}

type G2Point struct {
	X [2]*big.Int `abi:"x"`
	Y [2]*big.Int `abi:"y"`
}

type VerifyingKey struct {
	Alpha1 G1Point   `abi:"alpha1"`
	Beta2  G2Point   `abi:"beta2"`
	Gamma2 G2Point   `abi:"gamma2"`
	Delta2 G2Point   `abi:"delta2"`
	Ic     []G1Point `abi:"ic"`
}

type SnarkProof struct {
	A G1Point `abi:"a"`
	B G2Point `abi:"b"`
	C G1Point `abi:"c"`
}

// — Real VK values from PrivateMintVK.key (exported via gnark-crypto).
// G2 convention: x[0] = imaginary (A1), x[1] = real (A0).
// These are valid BN254 G2 curve points so the pairing precompile won't revert.
// —

var (
	// PrivateMint alpha1 (G1 point from VK)
	vkAlpha1 = G1Point{
		X: mustBig("15048447158655805652807104086039656509374918613321514519308628737773337751940"),
		Y: mustBig("12131950576689362872180698683490322491638777142694112473044054620240647743084"),
	}

	vkBeta2 = G2Point{
		X: [2]*big.Int{
			mustBig("20648198355859620042284446754064708303131901134134937454726343182717307111575"),
			mustBig("20528208130197338462380113270651004711155708324912775272227749004636988012491"),
		},
		Y: [2]*big.Int{
			mustBig("15589915852716027158041427900592090894350717572264417712038497890025300576016"),
			mustBig("6763402187969383355539237712350452724284604809514224391300825004030204675961"),
		},
	}

	vkGamma2 = G2Point{
		X: [2]*big.Int{
			mustBig("2108526014916061969160057142396304685372542228716165178212408576441525872639"),
			mustBig("21026054852774688826352672664599699504591916168273302458615077890896052063714"),
		},
		Y: [2]*big.Int{
			mustBig("7908839535380465352845657960059825877590790153942925242830548596412117427636"),
			mustBig("14054366319694178402162252444185553539473940138021998405429429406378954377197"),
		},
	}

	vkDelta2 = G2Point{
		X: [2]*big.Int{
			mustBig("9553527959326072480417273268196598605682622112984849764609764349174302433776"),
			mustBig("1310456564780679344202573868462904942710836776207946893167930705444619921416"),
		},
		Y: [2]*big.Int{
			mustBig("4127445072588616074848416379824976278229829582341135776640584654713843099301"),
			mustBig("15438800841642147343535992948246105391959909046328754846549702858925596573401"),
		},
	}

	// IC[0] from PrivateMintVK.key — used as IC[i] for all i.
	vkIC0 = G1Point{
		X: mustBig("144691320275242522811476388514689292665483723904206469744458863343737194363"),
		Y: mustBig("1791209404015863598368349783247910502580280197424943876558343268736391119692"),
	}

	// proof.a must be a valid G1 point (negate() checks curve membership).
	// The PrivateMint alpha1 is a valid G1 point we can reuse.
	dummyG1 = vkAlpha1

	// proof.b — any valid G2 point.
	dummyG2 = vkBeta2
)

func makeVK(nPublic int) VerifyingKey {
	ic := make([]G1Point, nPublic+1)
	for i := range ic {
		ic[i] = vkIC0
	}
	return VerifyingKey{
		Alpha1: vkAlpha1,
		Beta2:  vkBeta2,
		Gamma2: vkGamma2,
		Delta2: vkDelta2,
		Ic:     ic,
	}
}

func makeProof() SnarkProof {
	return SnarkProof{A: dummyG1, B: dummyG2, C: dummyG1}
}

func makeInputs(nPublic int) []*big.Int {
	inputs := make([]*big.Int, nPublic)
	for i := range inputs {
		inputs[i] = big.NewInt(1)
	}
	return inputs
}

type artifact struct {
	ABI      json.RawMessage `json:"abi"`
	Bytecode string          `json:"bytecode"`
}

func main() {
	const artifactPath = "artifacts/contracts/core/contracts/GenericGroth16Verifier.sol/GenericGroth16Verifier.json"
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n(run from enygma_dvp/ root)\n", artifactPath, err)
		os.Exit(1)
	}

	var art artifact
	if err := json.Unmarshal(data, &art); err != nil {
		fmt.Fprintf(os.Stderr, "parse artifact: %v\n", err)
		os.Exit(1)
	}

	contractABI, err := abi.JSON(bytes.NewReader(art.ABI))
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse ABI: %v\n", err)
		os.Exit(1)
	}

	// Deploy to simulated chain.
	key, err := crypto.GenerateKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "keygen: %v\n", err)
		os.Exit(1)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(1337))
	if err != nil {
		fmt.Fprintf(os.Stderr, "auth: %v\n", err)
		os.Exit(1)
	}
	auth.GasLimit = 10_000_000

	alloc := types.GenesisAlloc{
		auth.From: {Balance: new(big.Int).Mul(big.NewInt(1e18), big.NewInt(1000))},
	}
	backend := backends.NewSimulatedBackend(alloc, 10_000_000)
	defer backend.Close()

	bytecode := common.FromHex(art.Bytecode)
	_, tx, _, err := bind.DeployContract(auth, contractABI, bytecode, backend)
	if err != nil {
		fmt.Fprintf(os.Stderr, "deploy: %v\n", err)
		os.Exit(1)
	}
	backend.Commit()

	contractAddr := crypto.CreateAddress(auth.From, tx.Nonce())

	fmt.Println("GenericGroth16Verifier.verify() gas cost")
	fmt.Println("Real VK G2 points (PrivateMint) | dummy proof | simulated EVM")
	fmt.Println()
	fmt.Printf("%-44s  %s\n", "Circuit", "Gas")
	fmt.Printf("%-44s  %s\n", "-------", "---")

	circuits := []struct {
		name    string
		nPublic int
	}{
		{"PrivateMint     (4 public inputs)", 4},
		{"DvPInitiator    (7 public inputs)", 7},
		{"DvPDestination  (5 public inputs)", 5},
	}

	for _, c := range circuits {
		vk := makeVK(c.nPublic)
		proof := makeProof()
		inputs := makeInputs(c.nPublic)

		packed, err := contractABI.Pack("verify", vk, proof, inputs)
		if err != nil {
			fmt.Printf("%-44s  pack error: %v\n", c.name, err)
			continue
		}

		gas, err := backend.EstimateGas(context.Background(), ethereum.CallMsg{
			To:   &contractAddr,
			Data: packed,
		})
		if err != nil {
			fmt.Printf("%-44s  estimate error: %v\n", c.name, err)
			continue
		}
		fmt.Printf("%-44s  %d gas\n", c.name, gas)
	}
}
