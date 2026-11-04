package engine

import (
	"context"
	"fmt"
	"math/big"
	"regexp"
)

// stepDef is one step a run plays: its id and label, the message it starts
// with, and what it does. In a walkthrough, pre is the action the whole step
// is released as, and actions are the actions inside it, each released on
// its own (see Gate).
type stepDef struct {
	id, label, msg string
	run            func(*Engine, context.Context, *step) error
	pre            string
	actions        []string
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

// StepActions are, for each step a run with opts announces, the actions its
// gate releases, in the order they first come; none outside a walkthrough.
func StepActions(opts Options) map[string][]string {
	out := map[string][]string{}
	for _, s := range plan(opts) {
		var as []string
		if s.pre != "" {
			as = append(as, s.pre)
		}
		if as = append(as, s.actions...); len(as) > 0 {
			out[s.id] = as
		}
	}
	return out
}

// plan is the steps a run with opts plays, in order.
func plan(opts Options) []stepDef {
	if opts.Walkthrough {
		return walkthroughPlan(opts.effective())
	}
	return fullPlan(opts)
}

// effective is opts as a run plays them: a walkthrough has no USDr fee, no
// tags and no scripted transfers.
func (o Options) effective() Options {
	if o.Walkthrough {
		o.UsdrFee, o.Tags, o.Script = false, false, nil
	}
	return o
}

// gating is how a walkthrough step's gate releases it: as a whole (pre),
// or each of its actions on its own.
type gating struct {
	pre     string
	actions []string
}

// walkGates are the actions of each walkthrough step, by step id, or by
// step kind for the fan-out's later payments: keygen, register, the first
// payment (one hop per click: proof request, proof back, relay, settle) and
// discover release each action inside them on its own; the
// other steps release once, as a whole. A step with none is not walked.
var walkGates = map[string]gating{
	"keygen":     {actions: []string{"auditor", "keys"}},
	"register":   {actions: []string{"register"}},
	"audit_keys": {pre: "recover"},
	"shield":     {pre: "shield"},
	"fanout_1":   {actions: []string{"prove", "proof", "relay", "settle"}},
	"fanout":     {pre: "payment"},
	"discover":   {actions: []string{"scan"}},
	"audit":      {pre: "audit"},
}

// stepKind is a step id without its _N suffix.
var stepKind = regexp.MustCompile(`_\d+$`)

// walkthroughPlan is the Sender's path through the full plan of opts:
// keygen, register, auditor keys, shield, the fan-out, discover and audit.
// It drops prereqs, folding its connection to the services into keygen,
// which reads but sends nothing.
func walkthroughPlan(opts Options) []stepDef {
	var steps []stepDef
	for _, s := range fullPlan(opts) {
		g, ok := walkGates[s.id]
		if !ok {
			g, ok = walkGates[stepKind.ReplaceAllString(s.id, "")]
		}
		if !ok {
			continue
		}
		s.pre, s.actions = g.pre, g.actions
		if s.id == "keygen" {
			s.msg = "The auditor, the issuer, the sender and the participants make their keys, one per click…"
			s.run = func(e *Engine, ctx context.Context, st *step) error {
				if err := e.connect(ctx, st); err != nil {
					return err
				}
				return e.generateKeys(ctx, st)
			}
		}
		steps = append(steps, s)
	}
	return steps
}

// fullPlan is the steps a full run with opts plays, in order.
func fullPlan(opts Options) []stepDef {
	steps := []stepDef{
		{id: "prereqs", label: "Check prerequisites", msg: "Probing chain · gnark server · relayer…", run: (*Engine).checkPrereqs},
		{id: "keygen", label: "Generate keys", msg: "Issuer, sender and participants generate spend and view keys…", run: (*Engine).generateKeys},
		{id: "register", label: "Register in UserRegistry", msg: "Sender and participants publish their keys…", run: (*Engine).register},
		{id: "audit_keys", label: "Auditor keys", msg: "Auditor opens each registered user's audit ciphertexts in the UserRegistry…", run: (*Engine).auditorKeysStep},
	}
	if opts.Tags {
		steps = append(steps, stepDef{id: "channels", label: "Tag channels", msg: "Each payer opens a private-tag channel to everyone they will pay, through the relayer…", run: (*Engine).openChannels})
	}
	steps = append(steps, []stepDef{
		{id: "usdr", label: "USDr fee credit", msg: fmt.Sprintf("Issuer shields a %d USDr note to the sender and each participant…", UsdrGrant), run: (*Engine).grantUsdr},
		{id: "shield", label: "Shield", msg: fmt.Sprintf("Sender gets %d public RaylsERC20 and shields it into one private note…", opts.ShieldTotal),
			run: func(e *Engine, ctx context.Context, s *step) error { return e.shield(ctx, s, opts) }},
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
			id: fmt.Sprintf("fanout_%d", n), label: "Pay " + name,
			msg: fmt.Sprintf("Fan-out %d/%d: sender pays %s privately, change back to the sender (%s)…", n, K, name, route),
			run: func(e *Engine, ctx context.Context, s *step) error {
				return e.payFanOut(ctx, s, n, e.participants()[i], amount, opts.UsdrFee)
			},
		})
	}
	discoverMsg := "Participants scan the chain with their view keys…"
	if opts.Tags {
		discoverMsg = "Participants scan the tag registry for their tags…"
	}
	steps = append(steps, stepDef{id: "discover", label: "Discover", msg: discoverMsg, run: (*Engine).discover})
	for i, t := range opts.Script {
		steps = append(steps, stepDef{
			id: fmt.Sprintf("transfer_%d", i+1), label: fmt.Sprintf("%s → %s", t.From, t.To),
			msg: fmt.Sprintf("Transfer %d/%d: %s pays %s %d privately (%s)…", i+1, len(opts.Script), t.From, t.To, t.Amount, route),
			run: func(e *Engine, ctx context.Context, s *step) error { return e.payByName(ctx, s, t) },
		})
	}
	return append(steps,
		stepDef{id: "audit", label: "Audit", msg: "Auditor reconciles every user's balances from the transactions it decrypted…", run: (*Engine).reconcile},
		stepDef{id: "totals", label: "Totals", msg: "Adding up the run's transactions, gas, proving and relay time…", run: (*Engine).showTotals},
	)
}
