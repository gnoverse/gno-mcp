.PHONY: build test test-integration lint fmt run test-e2e dev

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

run: build
	./bin/gnomcp

# Launch Claude Code with the gnomcp skill + agents loaded live from this repo
# (visible top-level skills/ and agents/, no .claude/ symlinks). Iterate with
# /reload-skills and /reload-plugins; no restart needed.
dev:
	claude --plugin-dir .

test-e2e:
	@echo "Manual e2e protocol — see test/e2e/PROTOCOL.md"
	@echo "1. Run: ./test/e2e/setup.sh"
	@echo "2. In another terminal: ./bin/gnomcp --config test/e2e/profiles.toml"
	@echo "3. Walk through test/e2e/PROTOCOL.md by hand."
	@echo "4. Tear down: ./test/e2e/teardown.sh"
