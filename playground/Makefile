# Playground — Docker test harness. See README.md.
# Run from here (`make full`) or via the root shortcuts (`make playground-full`).
.PHONY: fresh gnomcp full sim e2e e2e-external

DOCKER         ?= docker
PLAYGROUND_IMG ?= gnomcp-playground
# Build context is the repo root (..) — the Dockerfile COPYs go.mod, cmd/, internal/,
# skills/, agents/, .claude-plugin/, entrypoint.sh from there.
DOCKER_BUILD = $(DOCKER) build -f Dockerfile
DOCKER_RUN   = $(DOCKER) run --rm -it --env-file .env

# bash + pipefail: the e2e recipes pipe the driver through progress.sh; a
# driver crash must fail the target, not be masked by the filter's exit 0.
SHELL := /bin/bash
.SHELLFLAGS := -o pipefail -ec

# Guard: error (not build) when the token file is absent. The order-only prereq
# (| .env) means "must exist, timestamp irrelevant, never built to satisfy".
.env:
	@echo "Missing playground/.env. Create it:" >&2
	@echo "  cp playground/.env.example playground/.env" >&2
	@echo "  # then paste the output of: claude setup-token" >&2
	@exit 1

fresh: | .env
	$(DOCKER_BUILD) --target l1-fresh -t $(PLAYGROUND_IMG):fresh ..
	$(DOCKER_RUN) $(PLAYGROUND_IMG):fresh

gnomcp: | .env
	$(DOCKER_BUILD) --target l2-gnomcp -t $(PLAYGROUND_IMG):gnomcp ..
	$(DOCKER_RUN) $(PLAYGROUND_IMG):gnomcp

full: | .env
	$(DOCKER_BUILD) --target l3-full -t $(PLAYGROUND_IMG):full ..
	$(DOCKER_RUN) $(PLAYGROUND_IMG):full

# Interactive shell with the simulated testnet already serving (the sim image
# stage starts simnet from its init entrypoint), gnoweb published to the host.
# Override the host port: `make sim GNOWEB_PORT=9999`.
GNOWEB_PORT ?= 8688
sim: | .env
	$(DOCKER_BUILD) --target sim -t $(PLAYGROUND_IMG):sim ..
	$(DOCKER_RUN) -p $(GNOWEB_PORT):8688 --add-host testnet.gnomcp.sim:127.0.0.1 $(PLAYGROUND_IMG):sim

# ---- e2e harness (the playground-driver agent owns scenario selection + lifecycle;
# make is a thin passthrough + verdict gate). Selection flags go to the driver via ARGS:
#   make e2e                                   # whole local tier
#   make e2e ARGS="--scenario chain-overview"  # one scenario
#   make e2e ARGS="--category reads"           # one feature set
# := is load-bearing: a recursive `=` re-runs `date` per expansion, so the driver and
# the jq gate could disagree on the report directory.
ARGS        ?= --all
REPORT_BASE := e2e/reports/$(shell date -u +%Y-%m-%d_%H-%M-%S)
e2e:          REPORT_DIR := $(REPORT_BASE)-local
e2e-external: REPORT_DIR := $(REPORT_BASE)-external
# The batch driver runs UNATTENDED WITH PERMISSIONS BYPASSED ON THE HOST (mutating
# Bash/Write are otherwise denied headless). Developer tooling only — see README.
E2E_CLAUDE = claude -p --agent playground-driver --dangerously-skip-permissions \
	  --output-format stream-json --verbose

e2e: | .env
	printf '%s' "$(ARGS) --tier local --report $(REPORT_DIR)" | $(E2E_CLAUDE) | e2e/scripts/progress.sh
	jq -e '[.scenarios[].verdict] | length > 0 and all(. == "pass")' \
	  $(REPORT_DIR)/results.json > /dev/null \
	  || { echo "E2E FAILED — see $(REPORT_DIR)/report.md" >&2; exit 1; }
	@echo "E2E GREEN — $(REPORT_DIR)/report.md"

e2e-external: | .env
	printf '%s' "$(ARGS) --tier external --report $(REPORT_DIR)" | $(E2E_CLAUDE) | e2e/scripts/progress.sh
# no length guard: empty external tier is a legitimate pass (spec); pipefail above catches driver crashes
	jq -e '[.scenarios[].verdict] | any(. == "fail") | not' \
	  $(REPORT_DIR)/results.json > /dev/null \
	  || { echo "E2E-EXTERNAL FAILED (hard fail, not blocked) — see $(REPORT_DIR)/report.md" >&2; exit 1; }
	@echo "E2E-EXTERNAL DONE (blocked tolerated) — $(REPORT_DIR)/report.md"
