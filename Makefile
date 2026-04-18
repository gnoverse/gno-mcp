.PHONY: install test run lint clean

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
