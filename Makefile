.PHONY: build test test-integration lint fmt run test-e2e dev bump \
	playground-fresh playground-gnomcp playground-full playground-sim \
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

bump:
	@test -n "$(VERSION)" || { echo "usage: make bump VERSION=x.y.z" >&2; exit 2; }
	./scripts/bump-version.sh "$(VERSION)"

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

# Scenario selection goes to the driver via ARGS, which propagates to the
# sub-make as a command-line variable: `make playground-e2e ARGS="--scenario chain-overview"`.
playground-e2e:
	$(MAKE) -C playground e2e

playground-e2e-external:
	$(MAKE) -C playground e2e-external
