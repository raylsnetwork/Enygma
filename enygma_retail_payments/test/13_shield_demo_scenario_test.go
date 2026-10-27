package tests

// TestShieldDemoScenario drives the shield demo's scenario engine (the same
// engine the port-9092 web UI drives) against the live stack, with a sink
// that captures its events, and checks only what the chain and the actors'
// scans show.
//
// Prerequisites — a fresh deployment, as shield_demo/run.sh brings up:
//
//	Hardhat node (8545), retail deploy + init, gnark server (8082), relayer (8090)
//
// Run:
//
//	cd test && CC=/usr/bin/clang go test -run TestShieldDemoScenario -v -timeout 900s

import (
	"bytes"
	"context"
	"math/big"
	"strings"
	"sync"
	"testing"

	"enygma_retail_payments/shield_demo/engine"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
)

// capturingSink records every event the engine emits.
type capturingSink struct {
	mu     sync.Mutex
	events []engine.Event
}

func (s *capturingSink) Emit(e engine.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
}

// failures returns the messages of the steps that failed, for diagnostics.
func (s *capturingSink) failures() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, e := range s.events {
		if e.Kind == engine.StepFailed {
			out = append(out, e.Step+": "+e.Msg)
		}
	}
	return strings.Join(out, "; ")
}

func TestShieldDemoScenario(t *testing.T) {
	cfg := engine.DefaultConfig("..")
	if missing := engine.Unreachable(cfg); len(missing) > 0 {
		t.Skipf("services not reachable: %s", strings.Join(missing, ", "))
	}

	sink := &capturingSink{}
	e := engine.New(cfg, sink)
	defer e.Close()
	opts := engine.DefaultOptions()
	if err := e.Run(context.Background(), opts); err != nil {
		t.Fatalf("Run: %v (failed steps: %s)", err, sink.failures())
	}

	client := mustDial(t)
	defer client.Close()
	registryAddr := common.HexToAddress(loadOnchainReceipts(t)["UserRegistry"].ContractAddress)

	t.Run("sender and participants are registered", func(t *testing.T) {
		registered := append([]engine.Actor{e.Sender()}, e.Participants()...)
		if len(registered) != 1+engine.K {
			t.Fatalf("got %d registrable actors, want %d", len(registered), 1+engine.K)
		}
		for _, a := range registered {
			keys, err := rpcore.LookupKeys(client, registryAddr, a.Address)
			if err != nil {
				t.Fatalf("%s: LookupKeys: %v", a.Name, err)
			}
			if keys.SpendKey.Cmp(a.Spend.PublicKey) != 0 {
				t.Errorf("%s: registered pk_spend %s, want %s", a.Name, keys.SpendKey, a.Spend.PublicKey)
			}
			if !bytes.Equal(keys.ViewKey, a.View.EncapsKey) {
				t.Errorf("%s: registered pk_view differs from the actor's view key", a.Name)
			}
			mlKemCt, aesCt, err := rpcore.LookupAuditCiphertexts(client, registryAddr, a.Address)
			if err != nil {
				t.Fatalf("%s: LookupAuditCiphertexts: %v", a.Name, err)
			}
			recovered, err := rpcore.AuditorDecryptViewKey(e.Auditor().DecapsKey, mlKemCt, aesCt)
			if err != nil {
				t.Fatalf("%s: auditor cannot open the audit ciphertexts: %v", a.Name, err)
			}
			if !bytes.Equal(recovered.EncapsulationKey().Bytes(), a.View.EncapsKey) {
				t.Errorf("%s: auditor recovered a different view key", a.Name)
			}
		}
	})

	t.Run("sender shields the whole public balance", func(t *testing.T) {
		erc20 := bindContract(t, client, common.HexToAddress(loadOnchainReceipts(t)["ERC20"].ContractAddress), loadOnchainABI(t, "RaylsERC20"))
		var out []any
		if err := erc20.Call(&bind.CallOpts{}, &out, "balanceOf", e.Sender().Address); err != nil {
			t.Fatalf("balanceOf: %v", err)
		}
		if public := out[0].(*big.Int); public.Sign() != 0 {
			t.Errorf("sender still holds %s public RaylsERC20, want 0", public)
		}
	})

	t.Run("each participant scans the amount the sender assigned", func(t *testing.T) {
		for i, p := range e.Participants() {
			if got, want := e.Balance(p), big.NewInt(opts.Payouts[i]); got.Cmp(want) != 0 {
				t.Errorf("%s: scanned balance %s, want %s", p.Name, got, want)
			}
		}
	})

	t.Run("sender keeps the shielded total minus the payouts", func(t *testing.T) {
		// DefaultOptions: 1000 shielded, 120+90+150+60+200+80 = 700 paid out.
		if got, want := e.Balance(e.Sender()), big.NewInt(300); got.Cmp(want) != 0 {
			t.Errorf("sender's private balance %s, want %s", got, want)
		}
	})
}
