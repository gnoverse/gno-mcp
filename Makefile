.PHONY: build test lint fmt run

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
