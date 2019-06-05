.PHONY: all install

all: install

install:
	go install .

.PHONY: test coverage bench fmt vet prepare

COVERAGE=cover.out
COVERAGE_ARGS=-covermode count -coverprofile $(COVERAGE)

test:
	go test -v -cover $(COVERAGE_ARGS) ./...

coverage:
	go tool cover -html $(COVERAGE)

BENCHMARK_ARGS=-benchtime 5s -benchmem

bench:
	go test -bench . $(BENCHMARK_ARGS)

fmt:
	go fmt ./...

vet:
	go vet ./...

prepare: fmt vet
