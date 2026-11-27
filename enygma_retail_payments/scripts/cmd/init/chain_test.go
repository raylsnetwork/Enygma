package main

// Runs init on go-ethereum's simulated chain against the contracts the
// deployment ships: contracts/abis, which enygma_dvp's RetailArtifacts.test.js
// checks are enygma_dvp's build.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
)

// The config's deployer key (projectRoot's), and the governance account the
// handover tests hand the roles to: a contract, as init requires, whose one
// byte of code does nothing.
const testDeployerKey = "34d091c661db4c814d65c8ae9277b7055c0dde5a752ce5a3fdfd4ea11a8f7154"

var testGovernance = common.HexToAddress("0x6000000000000000000000000000000000000006")

// The addresses init only stores: nothing calls them during init.
var (
	testG16Verifier = common.HexToAddress("0x1000000000000000000000000000000000000001")
	testToken       = common.HexToAddress("0x3000000000000000000000000000000000000003")
	testUsdrToken   = common.HexToAddress("0x4000000000000000000000000000000000000004")
)

// The refusal tests' addresses: one set where init expects its own backend
// or verifier, and an account with no code.
var (
	testOtherContract = common.HexToAddress("0x5000000000000000000000000000000000000005")
	testNoCode        = common.HexToAddress("0x7000000000000000000000000000000000000007")
)

// shippedArtifact reads the shipped artifact of contract.
func shippedArtifact(t *testing.T, contract string) (raw []byte, parsed abi.ABI, bytecode string, links map[string]map[string][]struct{ Start, Length int }) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "abis", contract+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var a struct {
		ABI            json.RawMessage                                     `json:"abi"`
		Bytecode       string                                              `json:"bytecode"`
		LinkReferences map[string]map[string][]struct{ Start, Length int } `json:"linkReferences"`
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatal(err)
	}
	if parsed, err = abi.JSON(strings.NewReader(string(a.ABI))); err != nil {
		t.Fatal(err)
	}
	return raw, parsed, a.Bytecode, a.LinkReferences
}

// stopHow is how the send a simChain stops at fails.
type stopHow int

const (
	stopBeforeSending stopHow = iota // the node never gets the transaction
	stopUnmined                      // the node holds it, not yet mined
	stopMined                        // the node took and mined it
)

func (h stopHow) String() string {
	return [...]string{"not sent", "sent, not mined", "mined"}[h]
}

// simChain is a simulated chain that mines each transaction as it is sent,
// as init expects of a node. With stopAt >= 0, the send that many sends
// later fails, as if init had died there, after the node took the
// transaction as stopHow says. answer, if set, answers in the chain's place
// the view calls it returns ok for.
type simChain struct {
	simulated.Client
	backend *simulated.Backend
	stopAt  int
	stopHow stopHow
	sent    []string               // the method each sent transaction calls
	methods map[[4]byte]abi.Method // the deployed contracts' methods
	answer  func(method string, args []interface{}) (results []interface{}, ok bool)
}

func (c *simChain) CallContract(ctx context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	if len(msg.Data) < 4 || c.answer == nil {
		return c.Client.CallContract(ctx, msg, block)
	}
	if m, known := c.methods[[4]byte(msg.Data)]; known {
		args, err := m.Inputs.Unpack(msg.Data[4:])
		if err != nil {
			return nil, err
		}
		if results, ok := c.answer(m.Name, args); ok {
			return m.Outputs.Pack(results...)
		}
	}
	return c.Client.CallContract(ctx, msg, block)
}

func (c *simChain) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	if c.stopAt == 0 {
		c.stopAt = -1
		if c.stopHow == stopBeforeSending {
			return errors.New("init stopped before sending")
		}
		if err := c.Client.SendTransaction(ctx, tx); err != nil {
			return err
		}
		if c.stopHow == stopUnmined {
			return errors.New("init stopped after sending, before the node mined it")
		}
		c.backend.Commit()
		return errors.New("init stopped after sending")
	}
	if c.stopAt > 0 {
		c.stopAt--
	}
	if err := c.Client.SendTransaction(ctx, tx); err != nil {
		return err
	}
	c.backend.Commit()
	name := "deploy"
	if tx.To() != nil && len(tx.Data()) >= 4 {
		name = c.methods[[4]byte(tx.Data())].Name
	}
	c.sent = append(c.sent, name)
	return nil
}

func (c *simChain) Close() {}

// testDeployment is a retail deployment on a simulated chain, with a project
// root init runs from.
type testDeployment struct {
	chain                                         *simChain
	dvp, verifier, privateMint, vault0, vaultUsdr common.Address
	dvpABI, verifierABI                           abi.ABI
	deployer                                      common.Address
	root                                          string
}

// deployForInit deploys EnygmaDvp, the Verifier, the PrivateMintVerifier,
// the Poseidon contracts and both vaults as cmd/deploy does, and writes
// projectRoot's project root, with owner as its config's "owner" object, the
// committed config's tree depth, and the deployed addresses as its receipts.
func deployForInit(t *testing.T, owner map[string]any) *testDeployment {
	t.Helper()
	key, err := crypto.HexToECDSA(testDeployerKey)
	if err != nil {
		t.Fatal(err)
	}
	d := &testDeployment{deployer: crypto.PubkeyToAddress(key.PublicKey)}
	backend := simulated.NewBackend(types.GenesisAlloc{
		d.deployer:     {Balance: new(big.Int).Lsh(big.NewInt(1), 100)},
		testGovernance: {Code: []byte{0x00}, Balance: new(big.Int)},
	}, simulated.WithBlockGasLimit(100_000_000))
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Errorf("close the simulated chain: %v", err)
		}
	})
	d.chain = &simChain{Client: backend.Client(), backend: backend, stopAt: -1, methods: map[[4]byte]abi.Method{}}
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(1337))
	if err != nil {
		t.Fatal(err)
	}
	deploy := func(contract string, libs map[string]common.Address, args ...interface{}) (common.Address, abi.ABI) {
		t.Helper()
		_, parsed, bytecode, links := shippedArtifact(t, contract)
		code := strings.TrimPrefix(bytecode, "0x")
		for _, byLib := range links {
			for lib, positions := range byLib {
				for _, p := range positions {
					code = code[:2*p.Start] + strings.ToLower(libs[lib].Hex()[2:]) + code[2*(p.Start+p.Length):]
				}
			}
		}
		addr, tx, _, err := bind.DeployContract(auth, parsed, common.FromHex(code), d.chain, args...)
		if err != nil {
			t.Fatalf("deploy %s: %v", contract, err)
		}
		if r, err := bind.WaitMined(context.Background(), d.chain, tx); err != nil || r.Status != types.ReceiptStatusSuccessful {
			t.Fatalf("deploy %s: %v, %+v", contract, err, r)
		}
		for _, m := range parsed.Methods {
			d.chain.methods[[4]byte(m.ID)] = m
		}
		return addr, parsed
	}
	t3, _ := deploy("PoseidonT3", nil)
	t5, _ := deploy("PoseidonT5", nil)
	wrapper, _ := deploy("PoseidonWrapper", map[string]common.Address{"PoseidonT3": t3, "PoseidonT5": t5})
	d.verifier, d.verifierABI = deploy("Verifier", nil)
	d.dvp, d.dvpABI = deploy("EnygmaDvp", nil, wrapper, testG16Verifier)
	d.privateMint, _ = deploy("PrivateMintVerifier", nil)
	d.vault0, _ = deploy("Erc20CoinVault", nil, d.dvp)
	d.vaultUsdr, _ = deploy("Erc20CoinVault", nil, d.dvp)
	d.chain.sent = nil

	d.root = projectRoot(t, func([]map[string]any) {})
	writeJSON(t, filepath.Join(d.root, "build", "receipts.json"), map[string]any{
		"EnygmaDvp":           map[string]string{"contractAddress": d.dvp.Hex()},
		"Verifier":            map[string]string{"contractAddress": d.verifier.Hex()},
		"G16Verifier":         map[string]string{"contractAddress": testG16Verifier.Hex()},
		"PrivateMintVerifier": map[string]string{"contractAddress": d.privateMint.Hex()},
		"Erc20CoinVault":      map[string]string{"contractAddress": d.vault0.Hex()},
		"ERC20":               map[string]string{"contractAddress": testToken.Hex()},
		"UsdrCoinVault":       map[string]string{"contractAddress": d.vaultUsdr.Hex()},
		"UsdrERC20":           map[string]string{"contractAddress": testUsdrToken.Hex()},
	})
	editConfig(t, d.root, func(cfg map[string]any) {
		cfg["owner"] = owner
		cfg["circom"].(map[string]any)["meta-parameters"] = map[string]any{"tree-depth": committedTreeDepth(t)}
	})

	saved := initProjectRoot
	initProjectRoot = d.root
	t.Cleanup(func() { initProjectRoot = saved })
	dial := dialInitChain
	dialInitChain = func(string) (initChain, error) { return d.chain, nil }
	t.Cleanup(func() { dialInitChain = dial })
	return d
}

// send makes the deployer call method on the contract at addr, before init
// runs.
func (d *testDeployment) send(t *testing.T, contractABI abi.ABI, addr common.Address, method string, args ...interface{}) {
	t.Helper()
	key, err := crypto.HexToECDSA(testDeployerKey)
	if err != nil {
		t.Fatal(err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(1337))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := bind.NewBoundContract(addr, contractABI, d.chain, d.chain, d.chain).Transact(auth, method, args...)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	if r, err := bind.WaitMined(context.Background(), d.chain, tx); err != nil || r.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("%s: %v, %+v", method, err, r)
	}
	d.chain.sent = nil
}

// view calls a view with one result on d's contract at addr.
func (d *testDeployment) view(t *testing.T, contractABI abi.ABI, addr common.Address, method string, args ...interface{}) interface{} {
	t.Helper()
	var out []interface{}
	if err := bind.NewBoundContract(addr, contractABI, d.chain, d.chain, d.chain).Call(nil, &out, method, args...); err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	return out[0]
}

// vaults returns the vaults d's EnygmaDvp holds as vault ids 0 and 1, the
// two init registers.
func (d *testDeployment) vaults(t *testing.T) [2]common.Address {
	t.Helper()
	var v [2]common.Address
	for id := range v {
		v[id] = d.view(t, d.dvpABI, d.dvp, "vaultById", big.NewInt(int64(id))).(common.Address)
	}
	return v
}

// checkNoVaultBeforeTheFreeze fails when d has a vault registered while its
// Verifier is not frozen: a vault takes the Verifier for good, so init
// freezes the Verifier's keys first.
func (d *testDeployment) checkNoVaultBeforeTheFreeze(t *testing.T) {
	t.Helper()
	if d.view(t, d.verifierABI, d.verifier, "frozen") == true {
		return
	}
	if v := d.vaults(t); v != ([2]common.Address{}) {
		t.Fatalf("vault ids 0 and 1 are %v while the Verifier is not frozen", v)
	}
}

// checkInitialized fails unless d is initialized as init leaves a
// deployment: the Verifier's backend and keys in place and frozen, both
// vaults registered, the USDr settings and fees set, and the roles with
// governance if not nil, else with the deployer.
func (d *testDeployment) checkInitialized(t *testing.T, relayerFee, usdrFee int64, governance *common.Address) {
	t.Helper()
	want := map[string][2]interface{}{
		"Verifier.groth16Verifier":          {d.view(t, d.verifierABI, d.verifier, "groth16Verifier"), testG16Verifier},
		"Verifier.frozen":                   {d.view(t, d.verifierABI, d.verifier, "frozen"), true},
		"EnygmaDvp.verifierContractAddress": {d.view(t, d.dvpABI, d.dvp, "verifierContractAddress"), d.verifier},
		"EnygmaDvp.vaultById(0)":            {d.view(t, d.dvpABI, d.dvp, "vaultById", big.NewInt(0)), d.vault0},
		"EnygmaDvp.vaultById(1)":            {d.view(t, d.dvpABI, d.dvp, "vaultById", big.NewInt(1)), d.vaultUsdr},
		"EnygmaDvp.vaultById(2)":            {d.view(t, d.dvpABI, d.dvp, "vaultById", big.NewInt(2)), common.Address{}},
		"EnygmaDvp.usdrFeeVaultSet":         {d.view(t, d.dvpABI, d.dvp, "usdrFeeVaultSet"), true},
		"EnygmaDvp.usdrFeeVaultId":          {d.view(t, d.dvpABI, d.dvp, "usdrFeeVaultId").(*big.Int).Int64(), int64(1)},
		"EnygmaDvp.usdrTokenId":             {d.view(t, d.dvpABI, d.dvp, "usdrTokenId").(*big.Int).Int64(), int64(0)},
		"EnygmaDvp.relayerFixedFeeAmount":   {d.view(t, d.dvpABI, d.dvp, "relayerFixedFeeAmount").(*big.Int).Int64(), relayerFee},
		"EnygmaDvp.usdrFixedFeeAmount":      {d.view(t, d.dvpABI, d.dvp, "usdrFixedFeeAmount").(*big.Int).Int64(), usdrFee},
		"Verifier: EnygmaDvp's owner role":  {d.view(t, d.verifierABI, d.verifier, "hasRole", ownerRole, d.dvp), false},
	}
	holder, other := d.deployer, testGovernance
	if governance != nil {
		holder, other = *governance, d.deployer
	}
	for _, c := range []struct {
		name string
		abi  abi.ABI
		addr common.Address
	}{{"EnygmaDvp", d.dvpABI, d.dvp}, {"Verifier", d.verifierABI, d.verifier}} {
		for roleName, role := range map[string]common.Hash{"owner": ownerRole, "admin": adminRole} {
			want[fmt.Sprintf("%s: %s's %s role", c.name, holder.Hex(), roleName)] = [2]interface{}{d.view(t, c.abi, c.addr, "hasRole", role, holder), true}
			want[fmt.Sprintf("%s: %s's %s role", c.name, other.Hex(), roleName)] = [2]interface{}{d.view(t, c.abi, c.addr, "hasRole", role, other), false}
		}
	}
	for what, gw := range want {
		if gw[0] != gw[1] {
			t.Errorf("%s = %v, want %v", what, gw[0], gw[1])
		}
	}
	// The keys, in their slots: the two projectRoot's manifest pins.
	vkeys, err := getVerificationKeys([]struct {
		ID       int      `json:"id"`
		Filename string   `json:"filename"`
		Tags     []string `json:"tags"`
	}{{Filename: "Payment"}, {Filename: "PaymentFee"}})
	if err != nil {
		t.Fatal(err)
	}
	if count := d.view(t, d.verifierABI, d.verifier, "verificationKeyCount").(*big.Int); count.Int64() != int64(len(vkeys)) {
		t.Errorf("verificationKeyCount = %s, want %d", count, len(vkeys))
	}
	for i, vk := range vkeys {
		got := common.Hash(d.view(t, d.verifierABI, d.verifier, "verificationKeyHash", big.NewInt(int64(i))).([32]byte)).Hex()
		if got != vk.slotKey().Hash() {
			t.Errorf("slot %d holds vk hash %s, want %s", i, got, vk.slotKey().Hash())
		}
	}
}

// committedTreeDepth is the tree depth the committed enygmapayment.config.json gives,
// which the vaults init registers here take.
func committedTreeDepth(t *testing.T) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "enygmapayment.config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg InitConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Circom.MetaParameters.TreeDepth == 0 {
		t.Fatal("the committed enygmapayment.config.json gives no tree depth")
	}
	return cfg.Circom.MetaParameters.TreeDepth
}

// writeJSON writes v to path as JSON.
func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// editConfig rewrites root's config with edit applied.
func editConfig(t *testing.T, root string, edit func(map[string]any)) {
	t.Helper()
	path := filepath.Join(root, "enygmapayment.config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	edit(cfg)
	writeJSON(t, path, cfg)
}

func fees(relayer, usdr string) map[string]any {
	return map[string]any{"relayer-fixed-fee": relayer, "usdr-fixed-fee": usdr}
}

func withGovernance(owner map[string]any) map[string]any {
	return withGovernanceAt(testGovernance, owner)
}

func withGovernanceAt(governance common.Address, owner map[string]any) map[string]any {
	owner["governance"] = governance.Hex()
	return owner
}

func TestInitInitializesADeploymentAndKeepsTheRolesWithoutGovernance(t *testing.T) {
	d := deployForInit(t, fees("7", "3"))
	if err := initializePayment(); err != nil {
		t.Fatalf("initializePayment = %v", err)
	}
	d.checkInitialized(t, 7, 3, nil)
}

func TestInitHandsTheRolesToGovernance(t *testing.T) {
	d := deployForInit(t, withGovernance(fees("7", "3")))
	if err := initializePayment(); err != nil {
		t.Fatalf("initializePayment = %v", err)
	}
	d.checkInitialized(t, 7, 3, &testGovernance)
}

// A run on a deployment init finished sends nothing it already did: only
// registerPrivateMintVerifier, which sets the same address again (EnygmaDvp
// has no getter for it), and after a handover nothing at all.
func TestInitRunAgainSendsOnlyWhatItCannotRead(t *testing.T) {
	for name, tc := range map[string]struct {
		owner map[string]any
		gov   *common.Address
		want  []string
	}{
		"without governance": {fees("7", "3"), nil, []string{"registerPrivateMintVerifier"}},
		"after a handover":   {withGovernance(fees("7", "3")), &testGovernance, nil},
	} {
		t.Run(name, func(t *testing.T) {
			d := deployForInit(t, tc.owner)
			if err := initializePayment(); err != nil {
				t.Fatalf("first initializePayment = %v", err)
			}
			d.chain.sent = nil
			if err := initializePayment(); err != nil {
				t.Fatalf("second initializePayment = %v", err)
			}
			if fmt.Sprint(d.chain.sent) != fmt.Sprint(tc.want) {
				t.Fatalf("the second run sent %v, want %v", d.chain.sent, tc.want)
			}
			d.checkInitialized(t, 7, 3, tc.gov)
		})
	}
}

// A run that stops at any transaction is finished by a later run: every key
// in its slot once, each vault once, the handover complete. A stopped run
// has registered no vault on a Verifier it has not frozen. While the node
// holds the stopped run's transaction unmined, the next run refuses to start
// and sends nothing; once the node mines it, a run finishes.
func TestInitCarriesOnAfterARunThatStopped(t *testing.T) {
	d := deployForInit(t, withGovernance(fees("7", "3")))
	if err := initializePayment(); err != nil {
		t.Fatalf("initializePayment = %v", err)
	}
	clean := d.chain.sent
	for stop := range clean {
		for _, how := range []stopHow{stopBeforeSending, stopUnmined, stopMined} {
			t.Run(fmt.Sprintf("stopped at %d %s, %s", stop+1, clean[stop], how), func(t *testing.T) {
				d := deployForInit(t, withGovernance(fees("7", "3")))
				d.chain.stopAt, d.chain.stopHow = stop, how
				if err := initializePayment(); err == nil || !strings.Contains(err.Error(), "init stopped") {
					t.Fatalf("first initializePayment = %v, want it stopped", err)
				}
				d.checkNoVaultBeforeTheFreeze(t)
				if how == stopUnmined {
					d.chain.sent = nil
					if err := initializePayment(); err == nil || !strings.Contains(err.Error(), "that it has not mined") {
						t.Fatalf("initializePayment with a transaction unmined = %v, want a refusal", err)
					}
					if len(d.chain.sent) != 0 {
						t.Fatalf("init sent %v with a transaction unmined", d.chain.sent)
					}
					d.chain.backend.Commit()
				}
				if err := initializePayment(); err != nil {
					t.Fatalf("second initializePayment = %v", err)
				}
				d.checkInitialized(t, 7, 3, &testGovernance)
			})
		}
	}
}

func TestInitRefusesOwnerSettingsBeforeSendingAnything(t *testing.T) {
	deployer := "0x0F1013e0e46B97144b25b3131668EF99858BD8D0"
	for name, tc := range map[string]struct {
		owner map[string]any
		want  string
	}{
		"no relayer fee":       {map[string]any{"usdr-fixed-fee": "0"}, "sets no owner.relayer-fixed-fee"},
		"no USDr fee":          {map[string]any{"relayer-fixed-fee": "0"}, "sets no owner.usdr-fixed-fee"},
		"a fee of 10^36":       {fees("1000000000000000000000000000000000000", "0"), "owner.relayer-fixed-fee 1000000000000000000000000000000000000 is not below 10^36"},
		"a negative fee":       {fees("0", "-1"), `owner.usdr-fixed-fee "-1" is not a whole number`},
		"a fee in hex":         {fees("0x10", "0"), `owner.relayer-fixed-fee "0x10" is not a whole number`},
		"governance not hex":   {map[string]any{"relayer-fixed-fee": "0", "usdr-fixed-fee": "0", "governance": "safe.eth"}, `owner.governance "safe.eth" is not an address`},
		"governance zero":      {map[string]any{"relayer-fixed-fee": "0", "usdr-fixed-fee": "0", "governance": common.Address{}.Hex()}, "owner.governance is the zero address"},
		"governance, deployer": {map[string]any{"relayer-fixed-fee": "0", "usdr-fixed-fee": "0", "governance": deployer}, "is the deployer's own address"},
		// EIP-55's own example, 0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed,
		// with its second letter's case changed.
		"governance mistyped": {map[string]any{"relayer-fixed-fee": "0", "usdr-fixed-fee": "0", "governance": "0x5AAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"}, `owner.governance "0x5AAeb6053F3E94C9b9A09f33669435E7Ef1BeAed" fails its EIP-55 checksum`},
		"governance no code":  {withGovernanceAt(testNoCode, fees("0", "0")), "owner.governance " + testNoCode.Hex() + " has no code"},
	} {
		t.Run(name, func(t *testing.T) {
			d := deployForInit(t, tc.owner)
			err := initializePayment()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("initializePayment = %v, want a refusal containing %q", err, tc.want)
			}
			if len(d.chain.sent) != 0 {
				t.Fatalf("init sent %v before refusing", d.chain.sent)
			}
		})
	}
}

// The largest fee EnygmaDvp takes, 10^36 - 1, goes through.
func TestInitSetsTheLargestFixedFee(t *testing.T) {
	max := "999999999999999999999999999999999999"
	d := deployForInit(t, fees(max, "0"))
	if err := initializePayment(); err != nil {
		t.Fatalf("initializePayment = %v", err)
	}
	if got := d.view(t, d.dvpABI, d.dvp, "relayerFixedFeeAmount").(*big.Int).String(); got != max {
		t.Fatalf("relayerFixedFeeAmount = %s, want %s", got, max)
	}
}

// A deployment that init cannot finish is refused before init registers a
// vault, the step that binds a vault to the Verifier for good. Init may
// have sent earlier steps by then.
func TestInitRefusesADeploymentItCannotFinish(t *testing.T) {
	for name, tc := range map[string]struct {
		setUp func(t *testing.T, d *testDeployment)
		want  func(d *testDeployment) string
	}{
		"a Verifier frozen with fewer keys": {func(t *testing.T, d *testDeployment) {
			vkeys, err := getVerificationKeys([]struct {
				ID       int      `json:"id"`
				Filename string   `json:"filename"`
				Tags     []string `json:"tags"`
			}{{Filename: "Payment"}})
			if err != nil {
				t.Fatal(err)
			}
			d.send(t, d.verifierABI, d.verifier, "initializeVerifier", testG16Verifier)
			d.send(t, d.verifierABI, d.verifier, "addVerificationKey", vkeys[0])
			d.send(t, d.verifierABI, d.verifier, "freeze")
		}, func(*testDeployment) string { return "the Verifier is frozen with 1 of the 2 keys init registers" }},
		"a Verifier frozen with no backend": {func(t *testing.T, d *testDeployment) {
			d.send(t, d.verifierABI, d.verifier, "freeze")
		}, func(*testDeployment) string { return "the Verifier is frozen with no Groth16 backend" }},
		"a Verifier on another backend": {func(t *testing.T, d *testDeployment) {
			d.send(t, d.verifierABI, d.verifier, "initializeVerifier", testOtherContract)
		}, func(*testDeployment) string {
			return "the Verifier already uses the Groth16 backend " + testOtherContract.Hex()
		}},
		"EnygmaDvp on another verifier": {func(t *testing.T, d *testDeployment) {
			d.send(t, d.dvpABI, d.dvp, "initializeDvp", testOtherContract)
		}, func(d *testDeployment) string {
			return fmt.Sprintf("EnygmaDvp already points at the verifier %s, not the deployment's %s", testOtherContract.Hex(), d.verifier.Hex())
		}},
		"another vault as vault 0": {func(t *testing.T, d *testDeployment) {
			d.send(t, d.dvpABI, d.dvp, "initializeDvp", d.verifier)
			d.send(t, d.dvpABI, d.dvp, "registerVault", d.vaultUsdr, testUsdrToken, big.NewInt(1), big.NewInt(int64(committedTreeDepth(t))))
		}, func(d *testDeployment) string {
			return fmt.Sprintf("vault id 0 is %s, not %s", d.vaultUsdr.Hex(), d.vault0.Hex())
		}},
	} {
		t.Run(name, func(t *testing.T) {
			d := deployForInit(t, fees("7", "3"))
			tc.setUp(t, d)
			before := d.vaults(t)
			err := initializePayment()
			if want := tc.want(d); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("initializePayment = %v, want a refusal containing %q", err, want)
			}
			if after := d.vaults(t); after != before {
				t.Fatalf("init registered a vault before refusing: vault ids 0 and 1 went from %v to %v", before, after)
			}
		})
	}
}

// The handover checks what the chain says after each grant and renounce, so
// a chain that does not show the change stops init: hasRole is made to
// answer false for governance, or true for the deployer, whatever was sent.
func TestInitChecksTheHandover(t *testing.T) {
	for name, tc := range map[string]struct {
		held func(d *testDeployment, account common.Address) (held, ok bool)
		want string
	}{
		"governance without the role after the grant": {func(d *testDeployment, a common.Address) (bool, bool) {
			return false, a == testGovernance
		}, "does not hold DEFAULT_OWNER_ROLE on EnygmaDvp after the grant"},
		"the deployer with the role after renouncing it": {func(d *testDeployment, a common.Address) (bool, bool) {
			return true, a == d.deployer
		}, "the deployer still holds DEFAULT_OWNER_ROLE on EnygmaDvp after renouncing it"},
	} {
		t.Run(name, func(t *testing.T) {
			d := deployForInit(t, withGovernance(fees("7", "3")))
			d.chain.answer = func(method string, args []interface{}) ([]interface{}, bool) {
				if method != "hasRole" {
					return nil, false
				}
				held, ok := tc.held(d, args[1].(common.Address))
				return []interface{}{held}, ok
			}
			err := initializePayment()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("initializePayment = %v, want a refusal containing %q", err, tc.want)
			}
		})
	}
}

// Before the vaults, init checks the Verifier's own record of its keys too:
// a Verifier that reports another count or another key hash than the keys
// its storage holds stops init, before it freezes the Verifier or registers
// a vault.
func TestInitChecksTheVerifiersRecordOfItsKeys(t *testing.T) {
	for name, tc := range map[string]struct {
		method  string
		results []interface{}
		want    string
	}{
		"another key count": {"verificationKeyCount", []interface{}{big.NewInt(3)}, "verificationKeyCount is 3, but init registered 2 keys"},
		"another key hash":  {"verificationKeyHash", []interface{}{[32]byte{31: 1}}, "slot 0: verificationKeyHash is 0x0000000000000000000000000000000000000000000000000000000000000001"},
	} {
		t.Run(name, func(t *testing.T) {
			d := deployForInit(t, fees("7", "3"))
			d.chain.answer = func(method string, _ []interface{}) ([]interface{}, bool) {
				return tc.results, method == tc.method
			}
			err := initializePayment()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("initializePayment = %v, want a refusal containing %q", err, tc.want)
			}
			if frozen := d.view(t, d.verifierABI, d.verifier, "frozen"); frozen != false {
				t.Fatalf("init froze the Verifier after the refusal")
			}
			if v := d.vaults(t); v != ([2]common.Address{}) {
				t.Fatalf("init registered vault ids 0 and 1 as %v before the refusal", v)
			}
		})
	}
}
