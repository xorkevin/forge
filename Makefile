.PHONY: all install

all: install

install:
	go install -trimpath -ldflags "-w -s" .

.PHONY: test coverage cover

TEST_ARGS?=
TEST_PACKAGE?=./...
COVERAGE?=cover.out

test:
	go test -race -trimpath -ldflags "-w -s" -cover -covermode atomic -coverprofile $(COVERAGE) $(TEST_ARGS) $(TEST_PACKAGE)


coverage:
	go tool cover -html $(COVERAGE)

cover: test coverage

.PHONY: fmt vet prepare

fmt:
	goimports -w .

vet:
	go vet ./...

prepare: fmt vet
