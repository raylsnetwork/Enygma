package engine

import (
	"context"
	"fmt"
	"math/big"
)

// stepDef is one step a run plays: its id and label, the message it starts
// with, and what it does.
type stepDef struct {
	id, label, msg string
	run            func(*Engine, context.Context, *step) error
}

// StepIDs are the ids of the steps a run with opts announces, in order.
func StepIDs(opts Options) []string {
	steps := plan(opts)
	ids := make([]string, len(steps))
	for i, s := range steps {
		ids[i] = s.id
	}
	return ids
}

// plan is the steps a run with opts plays, in order.
func plan(opts Options) []stepDef {
	steps := []stepDef{
		{"prereqs", "Check prerequisites", "Probing chain · gnark server · relayer…", (*Engine).checkPrereqs},
		{"keygen", "Generate keys", "Issuer, sender and participants generate spend and view keys…", (*Engine).generateKeys},
		{"register", "Register in UserRegistry", "Sender and participants publish their keys…", (*Engine).register},
		{"audit_keys", "Auditor keys", "Auditor opens each registered user's audit ciphertexts in the UserRegistry…", (*Engine).auditorKeysStep},
	}
	if opts.Tags {
		steps = append(steps, stepDef{"channels", "Tag channels", "Each payer opens a private-tag channel to everyone they will pay, through the relayer…", (*Engine).openChannels})
	}
	steps = append(steps, []stepDef{
		{"usdr", "USDr fee credit", fmt.Sprintf("Issuer shields a %d USDr note to the sender and each participant…", UsdrGrant), (*Engine).grantUsdr},
		{"shield", "Shield", fmt.Sprintf("Sender gets %d public RaylsERC20 and shields it into one private note…", opts.ShieldTotal),
			func(e *Engine, ctx context.Context, s *step) error { return e.shield(ctx, s, opts) }},
	}...)
	route := "no fee"
	if opts.UsdrFee {
		route = "USDr fee to the relayer"
	}
	if opts.Tags {
		route += ", private tag"
	}
	for i, name := range participantNames {
		n, amount := i+1, big.NewInt(opts.Payouts[i])
		steps = append(steps, stepDef{
			fmt.Sprintf("fanout_%d", n), "Pay " + name,
			fmt.Sprintf("Fan-out %d/%d: sender pays %s privately, change back to the sender (%s)…", n, K, name, route),
			func(e *Engine, ctx context.Context, s *step) error {
				return e.payFanOut(ctx, s, n, e.participants()[i], amount, opts.UsdrFee)
			},
		})
	}
	discoverMsg := "Participants scan the chain with their view keys…"
	if opts.Tags {
		discoverMsg = "Participants scan the tag registry for their tags…"
	}
	steps = append(steps, stepDef{"discover", "Discover", discoverMsg, (*Engine).discover})
	for i, t := range opts.Script {
		steps = append(steps, stepDef{
			fmt.Sprintf("transfer_%d", i+1), fmt.Sprintf("%s → %s", t.From, t.To),
			fmt.Sprintf("Transfer %d/%d: %s pays %s %d privately (%s)…", i+1, len(opts.Script), t.From, t.To, t.Amount, route),
			func(e *Engine, ctx context.Context, s *step) error { return e.payByName(ctx, s, t) },
		})
	}
	return append(steps,
		stepDef{"audit", "Audit", "Auditor reconciles every user's balances from the transactions it decrypted…", (*Engine).reconcile},
		stepDef{"totals", "Totals", "Adding up the run's transactions, gas, proving and relay time…", (*Engine).showTotals},
	)
}
