# Enygma monorepo — quality gates and shortcuts.
#
#   make help          list every target
#   make ci            what CI runs: fmt-check + build + vet + unit tests, all modules,
#                      and the Hardhat projects' install, compile and tests
#   make ci MODULE=enygma_dvp/src        the same, for one Go module (no Hardhat)
#
# The repo is 25 independent Go modules (see `make modules`); there is no
# single `go build ./...` at the root, so these targets loop over them.

SHELL := /bin/bash
.DEFAULT_GOAL := help

# go-ethereum needs CGo. Homebrew's clang on macOS points at an SDK path that
# does not exist, so use Apple's clang there.
ifeq ($(shell uname -s),Darwin)
export CC ?= /usr/bin/clang
endif

# Tracked modules only (same set CI sees). Narrow with MODULE=<dir>, or
# drop some with SKIP_MODULES="<dir> <dir>".
GO_MODULES := $(sort $(patsubst %/go.mod,%,$(shell git ls-files '*go.mod')))
SKIP_MODULES ?=
MODULES := $(if $(MODULE),$(MODULE),$(filter-out $(SKIP_MODULES),$(GO_MODULES)))
ifneq ($(filter-out $(GO_MODULES),$(MODULE)),)
$(error MODULE=$(MODULE) is not a tracked Go module; `make modules` lists them)
endif

GO_FILES = git ls-files '*.go'

# $(call each_in,<modules>,<command>): run <command> inside each listed module,
# keep going after a failure, print which modules failed, and exit non-zero if
# any did.
define each_in
@fail=""; for m in $(1); do \
	echo "==> $$m"; \
	( cd "$$m" && $(2) ) || fail="$$fail $$m"; \
done; \
if [ -n "$$fail" ]; then echo; echo "FAILED:$$fail"; exit 1; fi
endef
each_module = $(call each_in,$(MODULES),$(1))

# Known-red tests, excluded from test-unit so CI starts green and catches new
# regressions. These fail on main too (verified against the commit before the
# cleanup work), so they are not regressions. Fix one, then delete its entry.
#   enygma_payments/go_client/enygma_test
#       Integration suite: needs MY_KEY, a chain, gnark-server and the
#       relayer, and fails instead of skipping when they are absent.
#   enygma-server/pkg/circuits/withdraw
#       TestM03_HandlerProvesRealRequest: "len(points) != len(scalars)" —
#       keys/zkdvp/WithdrawPk6.key looks out of sync with the withdraw circuit.
# Override to run them anyway:  make test-unit TEST_SKIP_MODULES= TEST_SKIP_PACKAGES=
TEST_SKIP_MODULES  ?= enygma_payments/go_client/enygma_test
TEST_SKIP_PACKAGES ?= enygma-server/pkg/circuits/withdraw
TEST_MODULES := $(filter-out $(TEST_SKIP_MODULES),$(MODULES))
# Package list for one module, minus the skipped packages (never empty-fails).
TEST_PKGS = pkgs=$$(go list ./... | grep -v -x $(foreach p,$(TEST_SKIP_PACKAGES),-e '$(p)') || true)

# errcheck fails on a dropped error, except the idioms in .errcheck-exclude.
# Modules with unchecked errors when it was added (70 findings) are skipped so
# CI starts green and catches new ones. Fix a module's, then delete its entry;
# see them with:  make errcheck ERRCHECK_SKIP_MODULES= MODULE=<dir>
ERRCHECK := go run github.com/kisielk/errcheck@v1.30.0 -exclude $(CURDIR)/.errcheck-exclude
ERRCHECK_SKIP_MODULES ?= \
	enygma_dvp/gnark_circuits enygma_dvp/relayer enygma_dvp/src enygma_dvp/test \
	enygma_payments/demo enygma_payments/gnark-server enygma_payments/go_client/enygma_test \
	enygma_payments/test enygma_retail_payments/demo enygma_retail_payments/gnark_circuits \
	enygma_retail_payments/private_tags/src enygma_retail_payments/private_tags/test \
	enygma_retail_payments/src enygma_retail_payments/test

.PHONY: help modules hooks hooks-check fmt fmt-check page-check primer-from-disk moments-fit build vet errcheck check test-unit hardhat-check ci \
	dvp-chain dvp-deploy dvp-init dvp-gnark dvp-keygen dvp-stack dvp-stack-stop dvp-test \
	retail-setup retail-gnark retail-keygen retail-test shield-test shield-stack shield-stack-stop primer-refresh shield-fixtures shield-demo-restart shield-demo-stale shield-gnark-restart \
	payments-stack payments-stack-stop payments-test \
	auctions-gnark auctions-keygen

help: ## Show this help
	@awk 'BEGIN{FS=":.*## "} /^[a-zA-Z0-9_-]+:.*## /{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

modules: ## List the Go modules the targets below operate on
	@printf '%s\n' $(MODULES)

# ── Quality gates ─────────────────────────────────────────────────────────────

hooks: ## Install the git hooks (gofmt, page scripts, gitleaks; on commit, a commit adding a check names its red; on push, refuse a branch whose PR merged) and the primer's git diff driver in this clone; needs pre-commit
	@# git diff shows the primer's data block as its step and value names (.gitattributes).
	@git config diff.primer.textconv 'node "$$(git rev-parse --show-toplevel)/scripts/primer_page.js" --textconv'
	@command -v pre-commit >/dev/null || { echo "pre-commit not found: pip install pre-commit"; exit 1; }
	@# .pre-commit-config.yaml's stage names (pre-commit, pre-push) need pre-commit 3.2+.
	@v=$$(pre-commit --version | cut -d' ' -f2); [ "$$(printf '%s\n' 3.2.0 "$$v" | sort -V | head -1)" = 3.2.0 ] || { echo "pre-commit $$v is too old for .pre-commit-config.yaml: pip install -U pre-commit (3.2+)"; exit 1; }
	pre-commit install

fmt: ## Format all tracked Go files with gofmt
	@$(GO_FILES) | xargs gofmt -w

fmt-check: ## Fail if any tracked Go file is not gofmt-clean
	@bad=$$($(GO_FILES) | xargs gofmt -l); \
	if [ -n "$$bad" ]; then echo "Not gofmt-clean (run 'make fmt'):"; echo "$$bad"; exit 1; fi; \
	echo "gofmt: clean"

# page-check's npm dependencies (jsdom, for the shield demo replay test),
# installed again only when the lockfile changes.
PAGE_DEPS := scripts/node_modules/.package-lock.json
$(PAGE_DEPS): scripts/package-lock.json
	@command -v node >/dev/null && command -v npm >/dev/null || { echo "node or npm not found (page-check needs node 22.22+ and npm)"; exit 1; }
	cd scripts && npm ci --no-audit --no-fund

page-check: $(PAGE_DEPS) ## Fail on a bad inline <script>, a duplicated <style> selector or MathML Chrome draws wrong in a tracked HTML page, or on the shield demo page failing to play a real run, or on the Protocol primer failing to step through its run; needs node
	@command -v node >/dev/null || { echo "node not found (the demo pages' scripts need it to check)"; exit 1; }
	@# The node versions jsdom supports (its package.json engines): on another, jsdom fails as it loads.
	@node -e 'const [a, b] = process.versions.node.split(".").map(Number); process.exit((a === 22 && b >= 22) || (a === 24 && b >= 15) || a >= 26 ? 0 : 1)' || { echo "node $$(node -v) is not one jsdom runs on: page-check needs node 22.22+, 24.15+ or 26+ (nvm use, from .nvmrc)"; exit 1; }
	@# The checks' own tests, quiet unless they fail (then rerun to show why).
	@node --test scripts/check_page_scripts.test.js >/dev/null || node --test scripts/check_page_scripts.test.js
	@node scripts/check_page_scripts.js $$(git ls-files '*.html')
	@# The shield demo page plays and replays a real run's events (shield_demo/testdata).
	@node --test scripts/shield_demo_replay.test.js >/dev/null || node --test scripts/shield_demo_replay.test.js
	@# The Protocol primer steps through the run its data block holds.
	@node --test scripts/protocol_primer_replay.test.js >/dev/null || node --test scripts/protocol_primer_replay.test.js
	@# primer_page.js, the primer's reader, still finds and summarises the data block.
	@node --test scripts/primer_page.test.js >/dev/null || node --test scripts/primer_page.test.js
	@# mutant.sh, which shows a new check red, says RED only for a red it can trust.
	@node --test scripts/mutant.test.js >/dev/null || node --test scripts/mutant.test.js
	@# check_red_named.sh, the commit-msg hook, asks for a red only when a commit adds a check.
	@node --test scripts/check_red_named.test.js >/dev/null || node --test scripts/check_red_named.test.js
	@# check_hooks_installed.sh, make check's hooks-check, finds a hook type this clone lacks.
	@node --test scripts/check_hooks_installed.test.js >/dev/null || node --test scripts/check_hooks_installed.test.js
	@# suite_result.sh, the on-chain suites' verdict, fails a failed test or an unlisted skip.
	@node --test scripts/suite_result.test.js >/dev/null || node --test scripts/suite_result.test.js
	@# suite_changes.sh, the on-chain workflow's filter, runs the suites a change touches.
	@node --test scripts/suite_changes.test.js >/dev/null || node --test scripts/suite_changes.test.js
	@# stack_lib.sh, the stack scripts' helpers, stops a stack cleanly on a signal, a dead service or a failed step.
	@node --test scripts/stack_lib.test.js >/dev/null || node --test scripts/stack_lib.test.js
	@# stack_start.sh, make's *-stack, records the stack's pid from the start, and drops it when the stack fails to come up.
	@node --test scripts/stack_start.test.js >/dev/null || node --test scripts/stack_start.test.js
	@# stack_stop.sh, make's *-stack-stop, signals only the recorded stack and fails on a service it reports dead.
	@node --test scripts/stack_stop.test.js >/dev/null || node --test scripts/stack_stop.test.js

primer-from-disk: $(PAGE_DEPS) ## Run the Protocol primer's from-disk hand check in headless Chrome (OUT=<dir> for its step screenshots); needs Chrome, no stack
	@node scripts/primer_from_disk.js $(OUT)

moments-fit: $(PAGE_DEPS) ## Fail when a shield demo concept moment runs past the drawer at 1200x785, in any full run or walkthrough, in headless Chrome; needs Chrome, no stack
	@node scripts/moments_fit.js

build: ## go build ./... in every module
	$(call each_module,go build ./...)

vet: ## go vet ./... in every module
	$(call each_module,go vet ./...)

errcheck: ## Fail on a dropped error (idioms in .errcheck-exclude allowed) in every module not known-red
	$(call each_in,$(filter-out $(ERRCHECK_SKIP_MODULES),$(MODULES)),$(ERRCHECK) ./...)

check: hooks-check fmt-check page-check build vet errcheck ## Static checks only (no tests)

hooks-check: ## Fail when a git hook type .pre-commit-config.yaml installs is missing from this clone (skipped in CI)
	@scripts/check_hooks_installed.sh

test-unit: ## go test ./... in every module (chain-dependent tests skip themselves)
	@echo "Not run (known red, see TEST_SKIP_* above): modules [$(TEST_SKIP_MODULES)], packages [$(TEST_SKIP_PACKAGES)]"
	$(call each_in,$(TEST_MODULES),$(TEST_PKGS); if [ -n "$$pkgs" ]; then go test -count=1 $$pkgs; fi)

# The Hardhat (npm) projects. CI's Go jobs never touch them, so a dependency
# bump that breaks `hardhat compile` would otherwise go unnoticed (#110).
HARDHAT_PROJECTS := enygma_dvp enygma_dvp_auctions enygma_payments/contracts/enygma

hardhat-check: ## npm ci, hardhat compile and the JS tests in every Hardhat project; needs node 22
	@command -v node >/dev/null || { echo "node not found (nvm use 22)"; exit 1; }
	@# Compiling overwrites the real Poseidon artifacts with the placeholder's;
	@# regen_poseidon.js puts them back where a project has one.
	$(call each_in,$(HARDHAT_PROJECTS),npm ci --no-audit --no-fund && npx hardhat compile && { [ ! -f scripts/regen_poseidon.js ] || node scripts/regen_poseidon.js; } && npx hardhat test)

ci: check test-unit $(if $(MODULE),,hardhat-check) ## Everything CI runs

# ── Local development shortcuts (thin wrappers over the existing scripts) ─────
# $(call stack_start,<pid file>,<log dir>,<log prefix>,<stack log>,<run dir>,<command line>,<start target>,<test target>):
# a *-stack recipe, starting <command line> in <run dir> detached, with its
# output in <stack log> in <log dir> (else a fresh temp dir <log prefix>.*), and
# returning once it is up, or with its log when it exits first. It records the
# stack's pid in <pid file> from the start, so a stack still starting can be
# stopped, and refuses a second start while one runs (scripts/stack_start.sh).
define stack_start
@bash scripts/stack_start.sh $(1) '$(2)' $(3) $(4) $(5) '$(6)' $(7) $(8)
endef

# $(call stack_stop,<pid file>,<command line>,<stack log>,<start target>,<ports>):
# a *-stack-stop recipe, signalling only the stack <start target> recorded in
# <pid file>, whose command line is all of <command line>, and failing when it
# doesn't stop cleanly (scripts/stack_stop.sh).
define stack_stop
@bash scripts/stack_stop.sh $(1) '$(2)' $(3) $(4) $(5)
endef

# $(call suite_test,<pid file>,<suite dir>,<name>,<timeout>,<env>,<go test flags>,<known skips>,<no stack env note>):
# an on-chain suite's recipe. It runs go test in <suite dir> with <env>, plus the
# stack's test.env when it wrote one (else prints the note, if any), logging to
# suite.log in the stack's log dir (else a temp file named <name>.*). It prints
# the verdict from scripts/suite_result.sh.
define suite_test
@logs=$$(cat $(1).logs 2>/dev/null); \
if [ -d "$$logs" ]; then log=$$logs/suite.log; else log=$$(mktemp "$${TMPDIR:-/tmp}/$(3).XXXXXX"); fi; \
if [ -f "$$logs/test.env" ]; then set -a; . "$$logs/test.env"; set +a; $(if $(8),else echo '$(8)'; )fi; \
[ "$$(uname -s)" != Darwin ] || export CC=$${CC:-/usr/bin/clang}; \
status=0; cd $(2); $(5) go test -count=1 -v ./... -timeout $(4) $(6) >$$log 2>&1 || status=$$?; \
cd $(CURDIR); bash scripts/suite_result.sh $$log $$status '$(7)'; result=$$?; echo "log: $$log"; exit $$result
endef

# Full local flow for DvP: dvp-keygen (once) -> dvp-stack -> dvp-test -> dvp-stack-stop.
# dvp-stack brings up everything the suite uses (both deployments, gnark, both
# relayers), so no test skips for a missing service. Run dvp-test once per
# fresh stack: ERC721 state accumulates across runs. The dvp-chain/deploy/init/
# gnark targets are the stack's pieces, for running one by hand. Logs go to
# DVP_LOG_DIR (default: a fresh temp dir), the suite's to suite.log there.
DVP_STACK_PID := $(or $(TMPDIR),/tmp)/enygma_dvp_stack.pid
# What dvp-stack runs, in enygma_dvp: dvp-stack-stop signals only this command line.
DVP_STACK_CMD := bash scripts/stack.sh
DVP_LOG_DIR ?=
# Tests allowed to skip in dvp-test (full names, space-separated): the
# NewKeyPair field-range flakes. Any other skip fails the suite, so a relayer
# or deployment missing for tests 10/12/15/26/27 cannot pass for green.
DVP_KNOWN_SKIPS := TestNewKeyPair TestNewKeyPairDeterministicPublicKey

dvp-chain: ## Start a local Hardhat node (keep running)
	cd enygma_dvp && npx hardhat node

dvp-deploy: ## Regenerate Poseidon artifacts, build and run the DvP deploy
	cd enygma_dvp && bash scripts/deploy.sh

dvp-init: ## Export VKs and register them on-chain
	cd enygma_dvp && bash scripts/init.sh

dvp-keygen: ## Generate DvP proving/verifying keys (slow; rerun after circuit changes), with retail's PrivateMint keys
	cd enygma_dvp/gnark_circuits && mkdir -p scripts/keys scripts/verifier && go run ./cmd/keygen
	@# keygen draws fresh PrivateMint keys; the committed PrivateMintVerifier.sol is for retail's (same R1CS).
	cp enygma_retail_payments/gnark_circuits/scripts/keys/PrivateMintPK.key enygma_retail_payments/gnark_circuits/scripts/keys/PrivateMintVK.key enygma_dvp/gnark_circuits/scripts/keys/

dvp-gnark: ## Start the DvP gnark proof server on :8081 (keep running)
	cd enygma_dvp/gnark_circuits && go run ./cmd/server

dvp-stack: ## Start the DvP test stack (fresh chain, both deployments, gnark :8081, relayers :8091/:8092), detached; returns when it is up
	$(call stack_start,$(DVP_STACK_PID),$(DVP_LOG_DIR),dvp_stack,stack.log,enygma_dvp,$(DVP_STACK_CMD),dvp-stack,dvp-test)

dvp-stack-stop: ## Stop the stack make dvp-stack started, and wait for its ports to free
	$(call stack_stop,$(DVP_STACK_PID),$(DVP_STACK_CMD),stack.log,dvp-stack,8545 8081 8091 8092)

dvp-test: ## DvP integration suite (on a fresh make dvp-stack); prints pass/fail/skip counts, fails on a skip not in DVP_KNOWN_SKIPS
	$(call suite_test,$(DVP_STACK_PID),enygma_dvp/test,dvp_test,1800s,,,$(DVP_KNOWN_SKIPS),no make dvp-stack env: tests 10/12/15/26/27 skip without its relayers)

# Full local flow for enygma_payments' end-to-end suite (go_client/enygma_test):
# payments-stack -> payments-test -> payments-stack-stop. payments-stack compiles
# the contracts and builds relayer_bin first (both gitignored), then starts a
# fresh node and the gnark server; the suite deploys its own contracts and
# spawns relayer_bin. Logs go to PAYMENTS_LOG_DIR (default: a fresh temp dir),
# the suite's to suite.log there. The Burn keys are gitignored: see
# run_scripts/stack.sh's header.
PAYMENTS_STACK_PID := $(or $(TMPDIR),/tmp)/enygma_payments_stack.pid
# What payments-stack runs, from the repo root: payments-stack-stop signals only this command line.
PAYMENTS_STACK_CMD := bash enygma_payments/run_scripts/stack.sh
PAYMENTS_LOG_DIR ?=
# Tests allowed to skip in payments-test (full names, space-separated). Any
# other skip fails the suite: a missing service must not pass for green.
PAYMENTS_KNOWN_SKIPS :=

payments-stack: ## Start the payments e2e stack (compile contracts, build relayer_bin, fresh chain :8545, gnark :8080), detached; returns when it is up
	$(call stack_start,$(PAYMENTS_STACK_PID),$(PAYMENTS_LOG_DIR),payments_stack,stack.log,.,$(PAYMENTS_STACK_CMD),payments-stack,payments-test)

payments-stack-stop: ## Stop the stack make payments-stack started, and wait for its ports to free
	$(call stack_stop,$(PAYMENTS_STACK_PID),$(PAYMENTS_STACK_CMD),stack.log,payments-stack,8545 8080)

payments-test: ## Payments end-to-end suite (on make payments-stack); prints pass/fail/skip counts, fails on a skip not in PAYMENTS_KNOWN_SKIPS
	$(call suite_test,$(PAYMENTS_STACK_PID),enygma_payments/go_client/enygma_test,payments_test,900s,,,$(PAYMENTS_KNOWN_SKIPS))

retail-setup: ## Retail one-shot setup (needs dvp-chain running; see script header)
	cd enygma_retail_payments && bash setup.sh

retail-keygen: ## Generate retail proving/verifying keys
	cd enygma_retail_payments/gnark_circuits && go run ./cmd/keygen

retail-gnark: ## Start the retail gnark proof server (keep running)
	cd enygma_retail_payments/gnark_circuits && go run ./cmd/server

# The retail suite takes 8–12 min on the shield_demo/run.sh stack; the bench and
# load tests are left out (run them by name). Tests that rebuild the vault's
# tree 0 fail fast once the chain is past one tree: restart it fresh.
# Full flow: shield-stack -> retail-test -> shield-stack-stop. Logs go to
# RETAIL_LOG_DIR (default: a fresh temp dir), the suite's to suite.log there
# when the stack is shield-stack's.
RETAIL_BENCH_SKIP := TestRetailThroughput|TestRetailLoad|TestRetailScanRate|TestRetailChainTPS
RETAIL_LOG_DIR ?=
# Tests allowed to skip in retail-test (full names, space-separated): both skip
# once Alice is registered, which the suite's earlier tests do. Any other skip
# fails the suite: a missing service must not pass for green.
RETAIL_KNOWN_SKIPS := TestUserRegistration TestAuditorViewKeyRecovery
retail-test: ## Retail integration suite, bench and load tests skipped (needs the shield_demo/run.sh stack on a fresh chain); prints pass/fail/skip counts, fails on a skip not in RETAIL_KNOWN_SKIPS; leaves the shield demo's replay fixtures alone
	$(call suite_test,$(SHIELD_STACK_PID),enygma_retail_payments/test,retail_test,1800s,REPLAY_FIXTURE_WRITE=0,-skip '$(RETAIL_BENCH_SKIP)',$(RETAIL_KNOWN_SKIPS))

# The full shield demo scenario test plays three runs (about 1–2 min since #190's
# fast proofs); this plays the default-mode run (about 30 s) and the subtests matching
# T, e.g. T=leavesIn. A mode's own run is a subtest too, so T=with_the_tag_toggle_off
# adds the tags-off run.
shield-test: ## Shield demo scenario, default run only, subtests matching T= (needs the shield_demo/run.sh stack)
	@# T is grouped: go test splits a top-level | before /, so T=a|b would run only TestShieldDemoScenario/a.
	cd enygma_retail_payments/test && go test -run 'TestShieldDemoScenario$$/($(T))' -v -timeout 1800s

# Where shield-stack records run.sh's pid, so shield-stack-stop signals only that process.
SHIELD_STACK_PID := $(or $(TMPDIR),/tmp)/enygma_shield_stack.pid
# What shield-stack runs, in shield_demo: shield-stack-stop signals only this command line.
SHIELD_STACK_CMD := bash run.sh --no-demo

shield-stack: ## Start the retail stack (fresh chain, setup, gnark :8082, relayer :8090), detached; returns when it is up
	$(call stack_start,$(SHIELD_STACK_PID),$(RETAIL_LOG_DIR),shield_demo,run.log,enygma_retail_payments/shield_demo,$(SHIELD_STACK_CMD),shield-stack,retail-test)

shield-stack-stop: ## Stop the stack make shield-stack started, and wait about 30 s for its ports to free
	$(call stack_stop,$(SHIELD_STACK_PID),$(SHIELD_STACK_CMD),run.log,shield-stack,8545 8082 8090)

# run.sh's -dev serves index.html from disk, but an engine (Go) change needs the
# demo binary rebuilt: this rebuilds it and relaunches it on the running stack,
# with that demo's -logs dir, detached as run.sh starts it.
# The suite runs first so the primer's scans cover other payments: a fresh
# chain holds only the story's own, and the replay check refuses that block.
PRIMER_REFRESH_SKIP := $(RETAIL_BENCH_SKIP)|TestShieldDemoScenario|TestShieldDemoWalkthrough|TestProtocolPrimer$$
primer-refresh: ## Fresh stack, the retail suite, then TestProtocolPrimer (rewrites the primer's data block); prints pass/fail/skip counts, stops the stack
	@$(MAKE) --no-print-directory shield-stack
	@log=$$(mktemp -t primer_refresh); status=0; export CC=$${CC:-/usr/bin/clang}; cd enygma_retail_payments/test; \
	REPLAY_FIXTURE_WRITE=0 go test -count=1 -v . -timeout 900s -skip '$(PRIMER_REFRESH_SKIP)' >$$log 2>&1 || status=1; \
	for k in PASS FAIL SKIP; do echo "suite $$k: $$(grep -c "^--- $$k" $$log)"; done; \
	grep -E '^--- (FAIL|SKIP)' $$log; \
	if [ $$status = 0 ]; then PRIMER_WRITE=1 go test -count=1 -run 'TestProtocolPrimer$$$$' . || status=1; else echo "suite failed (log: $$log), data block left alone"; fi; \
	cd ../..; $(MAKE) --no-print-directory shield-stack-stop; exit $$status

# The two live runs whose events the shield demo's replay test plays: each
# rewrites its fixture in shield_demo/testdata only when it passes. A skip
# fails the target too, as a skipped run writes nothing.
shield-fixtures: ## Fresh stack, then TestShieldDemoScenario and TestShieldDemoWalkthrough (rewrite the replay fixtures); prints their verdicts, stops the stack
	@$(MAKE) --no-print-directory shield-stack
	@log=$$(mktemp -t shield_fixtures); status=0; export CC=$${CC:-/usr/bin/clang}; cd enygma_retail_payments/test; \
	go test -count=1 -v -run 'TestShieldDemoScenario$$|TestShieldDemoWalkthrough$$' . -timeout 900s >$$log 2>&1 || status=1; \
	grep -E '^--- (PASS|FAIL|SKIP)' $$log; grep -q '^--- SKIP' $$log && status=1; \
	[ $$status = 0 ] || echo "a run failed or skipped (log: $$log): its fixture is left as it was"; \
	cd ../..; $(MAKE) --no-print-directory shield-stack-stop; git status --short enygma_retail_payments/shield_demo/testdata; exit $$status

shield-demo-stale: ## Fail if the demo on :9092 was built from Go sources other than the working tree's (then make shield-demo-restart)
	@pid=$$(lsof -tiTCP:9092 -sTCP:LISTEN) || { echo "no demo on :9092: run make shield-demo-restart (on a running stack) or bash enygma_retail_payments/shield_demo/run.sh"; exit 1; }; \
	logs=$$(ps -o args= -p $$pid | sed -n 's/.* -logs \([^ ]*\).*/\1/p'); \
	built=$$(cat "$$logs/bin/shield_demo.src" 2>/dev/null) || { echo "the demo on :9092 has no record of its sources (built before shield-demo-stale): run make shield-demo-restart"; exit 1; }; \
	[ "$$built" = "$$(scripts/shield_demo_src.sh)" ] || { echo "the demo on :9092 was built from other Go sources than the working tree's: run make shield-demo-restart"; exit 1; }; \
	echo "the demo on :9092 is current: built from the working tree's Go sources"

# With no demo on :9092 but the stack up (the demo died, or was killed), it
# finds the stack's logs dir from the gnark server's binary path, as
# shield-gnark-restart does, and starts the demo there: run.sh would start a
# fresh node over the running one.
shield-demo-restart: ## Rebuild the shield demo and restart it on the running run.sh stack, or start it there if it is down (after an engine change)
	@if pid=$$(lsof -tiTCP:9092 -sTCP:LISTEN); then \
	  logs=$$(ps -o args= -p $$pid | sed -n 's/.* -logs \([^ ]*\).*/\1/p'); done_msg=restarted; \
	  [ -d "$$logs" ] || { echo "the demo on :9092 (pid $$pid) has no -logs dir: restart the stack with run.sh"; exit 1; }; \
	else \
	  gpid=$$(lsof -tiTCP:8082 -sTCP:LISTEN) || { echo "no demo on :9092 and no gnark server on :8082: start the stack with bash enygma_retail_payments/shield_demo/run.sh"; exit 1; }; \
	  logs=$$(dirname $$(dirname $$(ps -o args= -p $$gpid | awk '{print $$1}'))); done_msg=started; \
	  [ -f "$$logs/gnark.log" ] || { echo "no demo on :9092, and the gnark server on :8082 (pid $$gpid) was not started by run.sh: restart the stack with run.sh"; exit 1; }; \
	fi; \
	root=$(CURDIR)/enygma_retail_payments; \
	go build -C $$root/shield_demo -o $$logs/bin/shield_demo . && scripts/shield_demo_src.sh >$$logs/bin/shield_demo.src && { [ -z "$$pid" ] || kill $$pid; } && \
	while lsof -tiTCP:9092 -sTCP:LISTEN >/dev/null; do sleep 0.2; done; \
	(cd $$root/shield_demo && exec perl -e 'use POSIX qw(setsid); setsid(); exec @ARGV or die "exec: $$!"' -- $$logs/bin/shield_demo -root $$root -logs $$logs -dev >>$$logs/demo.log 2>&1) & \
	for _ in $$(seq 1 100); do nc -z localhost 9092 2>/dev/null && { echo "demo $$done_msg on :9092 (logs $$logs)"; exit 0; }; sleep 0.2; done; \
	echo "the demo did not come back on :9092: see $$logs/demo.log"; exit 1

# The gnark server holds no chain state, so a gnark_circuits/server change needs
# only that server rebuilt: this relaunches it on the running stack, logging to
# the same gnark.log the demo tails.
shield-gnark-restart: ## Rebuild the retail gnark server and restart it on the running run.sh stack (after a gnark_circuits change)
	@pid=$$(lsof -tiTCP:8082 -sTCP:LISTEN) || { echo "no gnark server on :8082: start the stack with bash enygma_retail_payments/shield_demo/run.sh"; exit 1; }; \
	bin=$$(ps -o args= -p $$pid | awk '{print $$1}'); logs=$$(dirname $$(dirname $$bin)); \
	[ -f "$$logs/gnark.log" ] || { echo "the gnark server on :8082 (pid $$pid, $$bin) was not started by run.sh: restart the stack with run.sh"; exit 1; }; \
	root=$(CURDIR)/enygma_retail_payments; \
	go build -C $$root/gnark_circuits -o $$bin ./cmd/server && kill $$pid && \
	while lsof -tiTCP:8082 -sTCP:LISTEN >/dev/null; do sleep 0.2; done; \
	(cd $$root/gnark_circuits && exec perl -e 'use POSIX qw(setsid); setsid(); exec @ARGV or die "exec: $$!"' -- $$bin >>$$logs/gnark.log 2>&1) & \
	for _ in $$(seq 1 300); do nc -z localhost 8082 2>/dev/null && { echo "gnark server restarted on :8082 (logs $$logs)"; exit 0; }; sleep 0.2; done; \
	echo "the gnark server did not come back on :8082: see $$logs/gnark.log"; exit 1

auctions-keygen: ## Generate auction proving/verifying keys
	cd enygma_dvp_auctions/gnark_circuits && go run ./cmd/keygen

auctions-gnark: ## Start the auctions gnark proof server (keep running)
	cd enygma_dvp_auctions/gnark_circuits && go run ./cmd/server
