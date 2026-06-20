.PHONY: build test test-integration lint fmt run test-e2e dev bump bump.gnomcp \
	playground-fresh playground-gnomcp playground-full playground-sim playground-sim-cla \
	playground-e2e playground-e2e-external

build:
	go build -o bin/gnomcp ./cmd/gnomcp

test:
	go test ./...

test-integration:
	go test -tags=integration ./test/integration/...

lint:
	go vet ./...
	gofmt -l . | tee /dev/stderr | (! read)

fmt:
	gofmt -w .

# gnomcp ships as a plugin, so its version lives in committed manifests
# (package.json, plugin.json, …) that this rewrites before a gnomcp release.
# agentfaucet is a Docker image with no manifest — it is versioned by its release
# tag alone, so it has no bump target. See docs/releasing.md.
bump.gnomcp:
	@test -n "$(VERSION)" || { echo "usage: make bump.gnomcp VERSION=x.y.z" >&2; exit 2; }
	./scripts/bump-version.sh "$(VERSION)"

# Back-compat alias: gnomcp is the only component with bumpable manifests.
bump: bump.gnomcp

run: build
	./bin/gnomcp

# Launch Claude Code with the gnomcp skill + agents loaded live from this repo
# (visible top-level skills/ and agents/, no .claude/ symlinks). Iterate with
# /reload-skills and /reload-plugins; no restart needed.
dev:
	claude --plugin-dir .

test-e2e:
	@echo "There is no standalone manual e2e protocol — see test/README.md."
	@echo "Automated agent e2e:   make playground-e2e"
	@echo "Manual / exploratory:  make playground-sim  (then run claude)"

# ---- Playground (Docker test harness — see playground/Makefile + playground/README.md)
playground-fresh:
	$(MAKE) -C playground fresh

playground-gnomcp:
	$(MAKE) -C playground gnomcp

playground-full:
	$(MAKE) -C playground full

# Interactive shell + simulated testnet; gnoweb published to the host.
# Port override propagates: `make playground-sim GNOWEB_PORT=9999`.
playground-sim:
	$(MAKE) -C playground sim

# Like playground-sim, but with the test13 CLA deploy gate seeded (local twin of
# the deploy-gates scenario). Defaults to a 10 GNOT faucet drip, which clears the
# flow at the minimum gas fee. Override with SIM_CLA_DRIP to stress a other grant.
playground-sim-cla:
	$(MAKE) -C playground sim-cla

# Scenario selection goes to the driver via ARGS, which propagates to the
# sub-make as a command-line variable: `make playground-e2e ARGS="--scenario chain-overview"`.
playground-e2e:
	$(MAKE) -C playground e2e

playground-e2e-external:
	$(MAKE) -C playground e2e-external
