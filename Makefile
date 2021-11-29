PINT_BIN     := pint
PINT_GO_DIRS := cmd internal
PINT_SRC     := $(shell find $(PINT_GO_DIRS) -type f -name '*.go')

GOBIN := $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN = $(shell go env GOPATH)/bin
endif

COVER_DIR     = .cover
COVER_PROFILE = $(COVER_DIR)/coverage.out

.PHONY: build
build: $(PINT_BIN)

$(PINT_BIN): $(PINT_SRC) go.mod go.sum
	go build -trimpath -ldflags='-s -w' ./cmd/pint

$(GOBIN)/golangci-lint: tools/golangci-lint/go.mod tools/golangci-lint/go.sum
	go install -modfile=tools/golangci-lint/go.mod github.com/golangci/golangci-lint/cmd/golangci-lint
.PHONY: lint
lint: $(GOBIN)/golangci-lint
	$(GOBIN)/golangci-lint run -E staticcheck,misspell,promlinter,revive,tenv,errorlint,exportloopref,predeclared,bodyclose

.PHONY: test
test:
	mkdir -p $(COVER_DIR)
	echo 'mode: atomic' > $(COVER_PROFILE)
	go test \
		-covermode=atomic \
		-coverprofile=$(COVER_PROFILE) \
		-coverpkg=./... \
		-race \
		-count=5 \
		-timeout=5m \
		./...

.PHONY: cover
cover: test
	go tool cover -func=$(COVER_PROFILE)

.PHONY: coverhtml
coverhtml: test
ifndef CI
	go tool cover -html=$(COVER_PROFILE)
else
	go tool cover -html=$(COVER_PROFILE) -o=.cover/all.html
endif

.PHONY: benchmark
benchmark:
	go test \
		-v \
		-count=20 \
		-run=none \
		-bench=. \
		-benchmem \
		-memprofile memprofile.out \
		./cmd/pint
