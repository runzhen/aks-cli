GO ?= go

.PHONY: build run test test-race vet fmt check clean help

build:
	$(GO) build -o aks-cli .

run: build
	./aks-cli

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w main.go internal

check: test vet build

clean:
	rm -f ./aks-cli

help:
	@printf '%s\n' \
		'make            Build ./aks-cli (default)' \
		'make run        Build and launch the terminal UI' \
		'make test       Run all tests' \
		'make test-race  Run tests with the race detector' \
		'make vet        Run go vet' \
		'make fmt        Format Go source files' \
		'make check      Run tests, vet, and build' \
		'make clean      Remove only the built executable'
