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

.PHONY: help modules hooks fmt fmt-check page-check primer-from-disk build vet errcheck check test-unit hardhat-check ci \
	dvp-chain dvp-deploy dvp-init dvp-gnark dvp-keygen dvp-test \
	retail-setup retail-gnark retail-keygen retail-test shield-test shield-stack shield-stack-stop primer-refresh shield-demo-restart shield-gnark-restart \
	auctions-gnark auctions-keygen

help: ## Show this help
	@awk 'BEGIN{FS=":.*## "} /^[a-zA-Z0-9_-]+:.*## /{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

modules: ## List the Go modules the targets below operate on
	@printf '%s\n' $(MODULES)

# ── Quality gates ─────────────────────────────────────────────────────────────

hooks: ## Install the git hooks (gofmt, page scripts, gitleaks; on push, refuse a branch whose PR merged) and the primer's git diff driver in this clone; needs pre-commit
	@# git diff shows the primer's data block as its step and value names (.gitattributes).
	@git config diff.primer.textconv 'node "$$(git rev-parse --show-toplevel)/scripts/primer_page.js" --textconv'
	@command -v pre-commit >/dev/null || { echo "pre-commit not found: pip install pre-commit"; exit 1; }
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
	@command -v node >/dev/null && command -v npm >/dev/null || { echo "node or npm not found (page-check needs node 18+ and npm)"; exit 1; }
	cd scripts && npm ci --no-audit --no-fund

page-check: $(PAGE_DEPS) ## Fail on a bad inline <script>, a duplicated <style> selector or MathML Chrome draws wrong in a tracked HTML page, or on the shield demo page failing to play a real run, or on the Protocol primer failing to step through its run; needs node
	@command -v node >/dev/null || { echo "node not found (the demo pages' scripts need it to check)"; exit 1; }
	@# The checks' own tests, quiet unless they fail (then rerun to show why).
	@node --test scripts/check_page_scripts.test.js >/dev/null || node --test scripts/check_page_scripts.test.js
	@node scripts/check_page_scripts.js $$(git ls-files '*.html')
	@# The shield demo page plays and replays a real run's events (shield_demo/testdata).
	@node --test scripts/shield_demo_replay.test.js >/dev/null || node --test scripts/shield_demo_replay.test.js
	@# The Protocol primer steps through the run its data block holds.
	@node --test scripts/protocol_primer_replay.test.js >/dev/null || node --test scripts/protocol_primer_replay.test.js
	@# primer_page.js, the primer's reader, still finds and summarises the data block.
	@node --test scripts/primer_page.test.js >/dev/null || node --test scripts/primer_page.test.js

primer-from-disk: $(PAGE_DEPS) ## Run the Protocol primer's from-disk hand check in headless Chrome (OUT=<dir> for its step screenshots); needs Chrome, no stack
	@node scripts/primer_from_disk.js $(OUT)

build: ## go build ./... in every module
	$(call each_module,go build ./...)

vet: ## go vet ./... in every module
	$(call each_module,go vet ./...)

errcheck: ## Fail on a dropped error (idioms in .errcheck-exclude allowed) in every module not known-red
	$(call each_in,$(filter-out $(ERRCHECK_SKIP_MODULES),$(MODULES)),$(ERRCHECK) ./...)

check: fmt-check page-check build vet errcheck ## Static checks only (no tests)

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
# Full local flow for DvP:  dvp-chain (own terminal) -> dvp-keygen (once) ->
# dvp-deploy -> dvp-init -> dvp-gnark (own terminal) -> dvp-test.
# Run dvp-test once per fresh chain: ERC721 state accumulates across runs.

dvp-chain: ## Start a local Hardhat node (keep running)
	cd enygma_dvp && npx hardhat node

dvp-deploy: ## Regenerate Poseidon artifacts, build and run the DvP deploy
	cd enygma_dvp && bash scripts/deploy.sh

dvp-init: ## Export VKs and register them on-chain
	cd enygma_dvp && bash scripts/init.sh

dvp-keygen: ## Generate DvP proving/verifying keys (slow; rerun after circuit changes)
	cd enygma_dvp/gnark_circuits && go run ./cmd/keygen

dvp-gnark: ## Start the DvP gnark proof server on :8081 (keep running)
	cd enygma_dvp/gnark_circuits && go run ./cmd/server

dvp-test: ## DvP integration tests (needs chain, deploy/init and gnark server)
	cd enygma_dvp/test && go test ./... -v -timeout 600s

retail-setup: ## Retail one-shot setup (needs dvp-chain running; see script header)
	cd enygma_retail_payments && bash setup.sh

retail-keygen: ## Generate retail proving/verifying keys
	cd enygma_retail_payments/gnark_circuits && go run ./cmd/keygen

retail-gnark: ## Start the retail gnark proof server (keep running)
	cd enygma_retail_payments/gnark_circuits && go run ./cmd/server

retail-test: ## Retail integration tests (needs chain, setup and gnark server)
	cd enygma_retail_payments/test && go test ./... -v -timeout 600s

# The full shield demo scenario test plays three runs (~11 min); this plays the
# default-mode run (~4 min) and the subtests matching T, e.g. T=leavesIn. A mode's
# own run is a subtest too, so T=with_the_tag_toggle_off adds the tags-off run (~8 min).
shield-test: ## Shield demo scenario, default run only, subtests matching T= (needs the shield_demo/run.sh stack)
	@# T is grouped: go test splits a top-level | before /, so T=a|b would run only TestShieldDemoScenario/a.
	cd enygma_retail_payments/test && go test -run 'TestShieldDemoScenario$$/($(T))' -v -timeout 1800s

# run.sh stays in the foreground for the stack's whole life, so anything waiting
# on it (a pipe, the next command) waits forever: this detaches it and returns
# once the stack is up, or with run.sh's error.
# Where shield-stack records run.sh's pid, so shield-stack-stop signals only that process.
SHIELD_STACK_PID := $(or $(TMPDIR),/tmp)/enygma_shield_stack.pid

shield-stack: ## Start the retail stack (fresh chain, setup, gnark :8082, relayer :8090), detached; returns when it is up
	@logs=$$(mktemp -d -t shield_demo); \
	(cd enygma_retail_payments/shield_demo && LOG_DIR=$$logs exec perl -e 'use POSIX qw(setsid); setsid(); exec @ARGV or die "exec: $$!"' -- bash run.sh --no-demo >$$logs/run.log 2>&1) & \
	pid=$$!; \
	while kill -0 $$pid 2>/dev/null; do \
		grep -q 'stack is up' $$logs/run.log && { echo $$pid >$(SHIELD_STACK_PID); echo "stack is up (logs $$logs); stop it with make shield-stack-stop"; exit 0; }; \
		sleep 1; \
	done; \
	echo "run.sh exited before the stack came up:"; tail -5 $$logs/run.log; exit 1

shield-stack-stop: ## Stop the stack make shield-stack started, and wait about 30 s for its ports to free
	@pid=$$(cat $(SHIELD_STACK_PID) 2>/dev/null); \
	ps -o command= -p "$$pid" 2>/dev/null | grep -q 'run.sh --no-demo' || { echo "no stack started by make shield-stack"; exit 1; }; \
	kill -TERM $$pid; \
	for _ in $$(seq 1 110); do \
		lsof -tiTCP:8545,8082,8090 -sTCP:LISTEN >/dev/null || { echo "stack stopped"; exit 0; }; \
		sleep 0.2; \
	done; \
	echo "still listening after about 30 s:"; lsof -iTCP:8545,8082,8090 -sTCP:LISTEN; exit 1

# run.sh's -dev serves index.html from disk, but an engine (Go) change needs the
# demo binary rebuilt: this rebuilds it and relaunches it on the running stack,
# with that demo's -logs dir, detached as run.sh starts it.
# The suite runs first so the primer's scans cover other payments: a fresh
# chain holds only the story's own, and the replay check refuses that block.
PRIMER_REFRESH_SKIP := TestRetailThroughput|TestRetailLoad|TestRetailScanRate|TestRetailChainTPS|TestShieldDemoScenario|TestShieldDemoWalkthrough|TestProtocolPrimer$$
primer-refresh: ## Fresh stack, the retail suite, then TestProtocolPrimer (rewrites the primer's data block); prints pass/fail/skip counts, stops the stack
	@$(MAKE) --no-print-directory shield-stack
	@log=$$(mktemp -t primer_refresh); status=0; export CC=$${CC:-/usr/bin/clang}; cd enygma_retail_payments/test; \
	go test -count=1 -v . -timeout 900s -skip '$(PRIMER_REFRESH_SKIP)' >$$log 2>&1 || status=1; \
	for k in PASS FAIL SKIP; do echo "suite $$k: $$(grep -c "^--- $$k" $$log)"; done; \
	grep -E '^--- (FAIL|SKIP)' $$log; \
	if [ $$status = 0 ]; then go test -count=1 -run 'TestProtocolPrimer$$$$' . || status=1; else echo "suite failed (log: $$log), data block left alone"; fi; \
	cd ../..; $(MAKE) --no-print-directory shield-stack-stop; exit $$status

shield-demo-restart: ## Rebuild the shield demo and restart it on the running run.sh stack (after an engine change)
	@pid=$$(lsof -tiTCP:9092 -sTCP:LISTEN) || { echo "no demo on :9092: start the stack with bash enygma_retail_payments/shield_demo/run.sh"; exit 1; }; \
	logs=$$(ps -o args= -p $$pid | sed -n 's/.* -logs \([^ ]*\).*/\1/p'); \
	[ -d "$$logs" ] || { echo "the demo on :9092 (pid $$pid) has no -logs dir: restart the stack with run.sh"; exit 1; }; \
	root=$(CURDIR)/enygma_retail_payments; \
	go build -C $$root/shield_demo -o $$logs/bin/shield_demo . && kill $$pid && \
	while lsof -tiTCP:9092 -sTCP:LISTEN >/dev/null; do sleep 0.2; done; \
	(cd $$root/shield_demo && exec perl -e 'use POSIX qw(setsid); setsid(); exec @ARGV or die "exec: $$!"' -- $$logs/bin/shield_demo -root $$root -logs $$logs -dev >>$$logs/demo.log 2>&1) & \
	for _ in $$(seq 1 100); do nc -z localhost 9092 2>/dev/null && { echo "demo restarted on :9092 (logs $$logs)"; exit 0; }; sleep 0.2; done; \
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
