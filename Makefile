.PHONY: install test run lint clean e2e

install:
	go install ./cmd/gno-mcp

test:
	go test ./... -race -count=1

run:
	go run ./cmd/gno-mcp

lint:
	go vet ./...
	gofmt -l -d .

clean:
	rm -rf dist/

e2e: install
	bash scripts/e2e.sh
