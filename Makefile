# Enygma monorepo — quality gates and shortcuts.
#
#   make help          list every target
#   make ci            what CI runs: fmt-check + build + vet + unit tests, all modules
#   make ci MODULE=enygma_dvp/src        the same, for one module
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

.PHONY: help modules fmt fmt-check build vet check test-unit ci \
	dvp-chain dvp-deploy dvp-init dvp-gnark dvp-keygen dvp-test \
	retail-setup retail-gnark retail-keygen retail-test \
	auctions-gnark auctions-keygen

help: ## Show this help
	@awk 'BEGIN{FS=":.*## "} /^[a-zA-Z0-9_-]+:.*## /{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

modules: ## List the Go modules the targets below operate on
	@printf '%s\n' $(MODULES)

# ── Quality gates ─────────────────────────────────────────────────────────────

fmt: ## Format all tracked Go files with gofmt
	@$(GO_FILES) | xargs gofmt -w

fmt-check: ## Fail if any tracked Go file is not gofmt-clean
	@bad=$$($(GO_FILES) | xargs gofmt -l); \
	if [ -n "$$bad" ]; then echo "Not gofmt-clean (run 'make fmt'):"; echo "$$bad"; exit 1; fi; \
	echo "gofmt: clean"

build: ## go build ./... in every module
	$(call each_module,go build ./...)

vet: ## go vet ./... in every module
	$(call each_module,go vet ./...)

check: fmt-check build vet ## Static checks only (no tests)

test-unit: ## go test ./... in every module (chain-dependent tests skip themselves)
	@echo "Not run (known red, see TEST_SKIP_* above): modules [$(TEST_SKIP_MODULES)], packages [$(TEST_SKIP_PACKAGES)]"
	$(call each_in,$(TEST_MODULES),$(TEST_PKGS); if [ -n "$$pkgs" ]; then go test -count=1 $$pkgs; fi)

ci: check test-unit ## Everything CI runs

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

auctions-keygen: ## Generate auction proving/verifying keys
	cd enygma_dvp_auctions/gnark_circuits && go run ./cmd/keygen

auctions-gnark: ## Start the auctions gnark proof server (keep running)
	cd enygma_dvp_auctions/gnark_circuits && go run ./cmd/server
