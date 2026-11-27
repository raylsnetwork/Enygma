package main

// Init's owner settings, and the steps that let a second run of init carry
// on where an earlier one stopped: each reads the chain first, and sends
// nothing when its change is already made.

import (
	"context"
	"fmt"
	"math/big"

	"enygma_vkslots/vkslots"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// initPaymentOwnerConfig is the config's "owner" object. The two fixed fees
// are required, in the token's base units ("0" for none): init sets them.
// governance is optional: an account (a Safe, or a TimelockController) that
// init hands EnygmaDvp's and the Verifier's roles to at the end, renouncing
// the deployer's. Left empty, the deployer keeps them.
type initPaymentOwnerConfig struct {
	RelayerFixedFee *string `json:"relayer-fixed-fee"`
	UsdrFixedFee    *string `json:"usdr-fixed-fee"`
	Governance      string  `json:"governance"`
}

// paymentOwnerSettings is initPaymentOwnerConfig read and checked.
type paymentOwnerSettings struct {
	relayerFixedFee *big.Int
	usdrFixedFee    *big.Int
	// governance receives the deployer's roles; nil keeps them with the
	// deployer.
	governance *common.Address
}

// paymentMaxFixedFee is EnygmaDvp's bound on a fixed fee, MAX_FEE_AMOUNT
// (10^36): every fee must be below it.
var paymentMaxFixedFee = new(big.Int).Exp(big.NewInt(10), big.NewInt(36), nil)

// settings reads and checks the owner settings, before init sends anything.
// deployer is the account init sends from.
func (c initPaymentOwnerConfig) settings(deployer common.Address) (paymentOwnerSettings, error) {
	var s paymentOwnerSettings
	var err error
	if s.relayerFixedFee, err = parsePaymentFixedFee("relayer-fixed-fee", c.RelayerFixedFee); err != nil {
		return s, err
	}
	if s.usdrFixedFee, err = parsePaymentFixedFee("usdr-fixed-fee", c.UsdrFixedFee); err != nil {
		return s, err
	}
	if c.Governance == "" {
		return s, nil
	}
	if !common.IsHexAddress(c.Governance) {
		return s, fmt.Errorf("owner.governance %q is not an address", c.Governance)
	}
	gov := common.HexToAddress(c.Governance)
	if gov == (common.Address{}) {
		return s, fmt.Errorf("owner.governance is the zero address: leave it empty to keep the roles with the deployer")
	}
	if gov == deployer {
		return s, fmt.Errorf("owner.governance %s is the deployer's own address: leave it empty to keep the roles with the deployer", gov.Hex())
	}
	s.governance = &gov
	return s, nil
}

// parsePaymentFixedFee reads the fee the config gives as owner.<name>, which
// it must give.
func parsePaymentFixedFee(name string, value *string) (*big.Int, error) {
	if value == nil {
		return nil, fmt.Errorf(`the config sets no owner.%s: give the fee in the token's base units, "0" for none`, name)
	}
	fee, ok := new(big.Int).SetString(*value, 10)
	if !ok || fee.Sign() < 0 {
		return nil, fmt.Errorf("owner.%s %q is not a whole number of base units", name, *value)
	}
	if fee.Cmp(paymentMaxFixedFee) >= 0 {
		return nil, fmt.Errorf("owner.%s %s is not below 10^36, the circuits' bound on a note value, so EnygmaDvp refuses it", name, fee)
	}
	return fee, nil
}

// paymentChain is the node init talks to: an ethclient.Client, or a
// simulated chain in the tests.
type paymentChain interface {
	bind.ContractBackend
	bind.DeployBackend
	StorageAt(ctx context.Context, account common.Address, key common.Hash, blockNumber *big.Int) ([]byte, error)
	Close()
}

// The roles init grants and renounces: OpenZeppelin's DEFAULT_ADMIN_ROLE,
// and DEFAULT_OWNER_ROLE as EnygmaDvp and the Verifier define it.
var (
	paymentAdminRole = common.Hash{}
	paymentOwnerRole = crypto.Keccak256Hash([]byte("ownerRole"))
)

// viewPayment calls method, a view with one result, on contract.
func viewPayment[T any](client paymentChain, contractABI abi.ABI, contract common.Address, method string, args ...interface{}) (T, error) {
	var zero T
	var out []interface{}
	bound := bind.NewBoundContract(contract, contractABI, client, client, client)
	if err := bound.Call(&bind.CallOpts{Context: context.Background()}, &out, method, args...); err != nil {
		return zero, fmt.Errorf("call %s: %w", method, err)
	}
	if len(out) != 1 {
		return zero, fmt.Errorf("call %s: %d results, want 1", method, len(out))
	}
	v, ok := out[0].(T)
	if !ok {
		return zero, fmt.Errorf("call %s: result %T, want %T", method, out[0], zero)
	}
	return v, nil
}

// paymentStepDone says, under the step's own line, that init skips it.
func paymentStepDone() {
	fmt.Println("  already done")
}

// hasPaymentRole says whether account holds role on contract.
func hasPaymentRole(client paymentChain, contractABI abi.ABI, contract common.Address, role common.Hash, account common.Address) (bool, error) {
	return viewPayment[bool](client, contractABI, contract, "hasRole", role, account)
}

// ensurePaymentRole grants role on contract to account, unless it holds it.
func ensurePaymentRole(client paymentChain, auth *bind.TransactOpts, contractABI abi.ABI, contract common.Address, role common.Hash, account common.Address) error {
	held, err := hasPaymentRole(client, contractABI, contract, role, account)
	if err != nil {
		return err
	}
	if held {
		paymentStepDone()
		return nil
	}
	_, err = callPaymentInitMethod(client, auth, contractABI, contract, "grantRole", role, account)
	return err
}

// ensurePaymentBackend has the Verifier use backend, unless it already does.
// A Verifier set to another backend is refused: it takes one only once. So
// is one frozen with none, which can never take one.
func ensurePaymentBackend(client paymentChain, auth *bind.TransactOpts, verifierABI abi.ABI, verifier, backend common.Address) error {
	current, err := viewPayment[common.Address](client, verifierABI, verifier, "groth16Verifier")
	if err != nil {
		return err
	}
	switch current {
	case backend:
		paymentStepDone()
		return nil
	case common.Address{}:
		frozen, err := viewPayment[bool](client, verifierABI, verifier, "frozen")
		if err != nil {
			return err
		}
		if frozen {
			return fmt.Errorf("the Verifier is frozen with no Groth16 backend: deploy a new Verifier")
		}
		_, err = callPaymentInitMethod(client, auth, verifierABI, verifier, "initializeVerifier", backend)
		return err
	default:
		return fmt.Errorf("the Verifier already uses the Groth16 backend %s, not the deployment's %s", current.Hex(), backend.Hex())
	}
}

// ensurePaymentDvpVerifier points EnygmaDvp at verifier, unless it already
// points there.
func ensurePaymentDvpVerifier(client paymentChain, auth *bind.TransactOpts, dvpABI abi.ABI, dvp, verifier common.Address) error {
	current, err := viewPayment[common.Address](client, dvpABI, dvp, "verifierContractAddress")
	if err != nil {
		return err
	}
	if current == verifier {
		paymentStepDone()
		return nil
	}
	_, err = callPaymentInitMethod(client, auth, dvpABI, dvp, "initializeDvp", verifier)
	return err
}

// paymentKeysToRegister returns held, the slot of the first key init still
// has to register, unless the Verifier is frozen with keys still to
// register: then no run can finish it.
func paymentKeysToRegister(client paymentChain, verifierABI abi.ABI, verifier common.Address, held, all int) (int, error) {
	if held == all {
		return held, nil
	}
	frozen, err := viewPayment[bool](client, verifierABI, verifier, "frozen")
	if err != nil {
		return 0, err
	}
	if frozen {
		return 0, fmt.Errorf("the Verifier is frozen with %d of the %d keys init registers: deploy a new Verifier", held, all)
	}
	return held, nil
}

// checkPaymentKeyHashes checks the Verifier's own record of its keys,
// verificationKeyCount and verificationKeyHash, against keys: keys[i] is the
// key init registered in slot i.
func checkPaymentKeyHashes(client paymentChain, verifierABI abi.ABI, verifier common.Address, keys []vkslots.Key) error {
	count, err := viewPayment[*big.Int](client, verifierABI, verifier, "verificationKeyCount")
	if err != nil {
		return err
	}
	if count.Cmp(big.NewInt(int64(len(keys)))) != 0 {
		return fmt.Errorf("verificationKeyCount is %s, but init registered %d keys", count, len(keys))
	}
	for i, k := range keys {
		hash, err := viewPayment[[32]byte](client, verifierABI, verifier, "verificationKeyHash", big.NewInt(int64(i)))
		if err != nil {
			return err
		}
		if got := common.Hash(hash).Hex(); got != k.Hash() {
			return fmt.Errorf("slot %d: verificationKeyHash is %s, but init registered %s", i, got, k.Hash())
		}
	}
	return nil
}

// ensurePaymentFrozen freezes the Verifier's keys, unless they are frozen.
func ensurePaymentFrozen(client paymentChain, auth *bind.TransactOpts, verifierABI abi.ABI, verifier common.Address) error {
	frozen, err := viewPayment[bool](client, verifierABI, verifier, "frozen")
	if err != nil {
		return err
	}
	if frozen {
		paymentStepDone()
		return nil
	}
	_, err = callPaymentInitMethod(client, auth, verifierABI, verifier, "freeze")
	return err
}

// ensurePaymentVault registers vault as vault id, unless it is registered
// there. registerVault numbers vaults in order, so id must be the next one;
// another vault under id is refused.
func ensurePaymentVault(client paymentChain, auth *bind.TransactOpts, dvpABI abi.ABI, dvp common.Address, id int64, vault, asset common.Address, treeDepth *big.Int) error {
	current, err := viewPayment[common.Address](client, dvpABI, dvp, "vaultById", big.NewInt(id))
	if err != nil {
		return err
	}
	switch current {
	case vault:
		paymentStepDone()
		return nil
	case common.Address{}:
		_, err = callPaymentInitMethod(client, auth, dvpABI, dvp, "registerVault", vault, asset, big.NewInt(1), treeDepth)
		return err
	default:
		return fmt.Errorf("vault id %d is %s, not %s", id, current.Hex(), vault.Hex())
	}
}

// ensurePaymentValue calls setter(want) on contract, unless getter already
// returns want.
func ensurePaymentValue(client paymentChain, auth *bind.TransactOpts, contractABI abi.ABI, contract common.Address, getter, setter string, want *big.Int) error {
	current, err := viewPayment[*big.Int](client, contractABI, contract, getter)
	if err != nil {
		return err
	}
	if current.Cmp(want) == 0 {
		paymentStepDone()
		return nil
	}
	_, err = callPaymentInitMethod(client, auth, contractABI, contract, setter, want)
	return err
}

// ensurePaymentUsdrVault pins vault id as the USDr fee vault, unless it is
// pinned there.
func ensurePaymentUsdrVault(client paymentChain, auth *bind.TransactOpts, dvpABI abi.ABI, dvp common.Address, id *big.Int) error {
	set, err := viewPayment[bool](client, dvpABI, dvp, "usdrFeeVaultSet")
	if err != nil {
		return err
	}
	current, err := viewPayment[*big.Int](client, dvpABI, dvp, "usdrFeeVaultId")
	if err != nil {
		return err
	}
	if set && current.Cmp(id) == 0 {
		paymentStepDone()
		return nil
	}
	_, err = callPaymentInitMethod(client, auth, dvpABI, dvp, "setUsdrFeeVaultId", id)
	return err
}

// paymentRoleHolder is a contract whose roles init hands over.
type paymentRoleHolder struct {
	name     string
	abi      abi.ABI
	contract common.Address
}

// handOverPayment gives governance DEFAULT_OWNER_ROLE and DEFAULT_ADMIN_ROLE
// on each holder and checks it holds both; then the deployer renounces its
// own, the admin role last, and init checks it holds neither. A grant or
// renounce already made is skipped.
func handOverPayment(client paymentChain, auth *bind.TransactOpts, governance common.Address, holders ...paymentRoleHolder) error {
	roles := []struct {
		name string
		id   common.Hash
	}{{"DEFAULT_OWNER_ROLE", paymentOwnerRole}, {"DEFAULT_ADMIN_ROLE", paymentAdminRole}}
	for _, h := range holders {
		for _, r := range roles {
			fmt.Printf("Granting %s on %s to %s...\n", r.name, h.name, governance.Hex())
			if err := ensurePaymentRole(client, auth, h.abi, h.contract, r.id, governance); err != nil {
				return fmt.Errorf("failed to grant %s on %s to %s: %w", r.name, h.name, governance.Hex(), err)
			}
		}
		for _, r := range roles {
			held, err := hasPaymentRole(client, h.abi, h.contract, r.id, governance)
			if err != nil {
				return err
			}
			if !held {
				return fmt.Errorf("%s does not hold %s on %s after the grant, so the deployer keeps its roles", governance.Hex(), r.name, h.name)
			}
		}
		for _, r := range roles {
			fmt.Printf("Renouncing the deployer's %s on %s...\n", r.name, h.name)
			held, err := hasPaymentRole(client, h.abi, h.contract, r.id, auth.From)
			if err != nil {
				return err
			}
			if !held {
				paymentStepDone()
				continue
			}
			if _, err := callPaymentInitMethod(client, auth, h.abi, h.contract, "renounceRole", r.id, auth.From); err != nil {
				return fmt.Errorf("failed to renounce the deployer's %s on %s: %w", r.name, h.name, err)
			}
		}
		for _, r := range roles {
			held, err := hasPaymentRole(client, h.abi, h.contract, r.id, auth.From)
			if err != nil {
				return err
			}
			if held {
				return fmt.Errorf("the deployer still holds %s on %s after renouncing it", r.name, h.name)
			}
		}
	}
	return nil
}
