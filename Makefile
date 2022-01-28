PINT_BIN     := pint
PINT_GO_DIRS := cmd internal
PINT_SRC     := $(shell find $(PINT_GO_DIRS) -type f -name '*.go')
PINT_VERSION ?= $(shell git describe --tags --always --dirty='-dev')
PINT_COMMIT  ?= $(shell git rev-parse HEAD)

GOBIN := $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN = $(shell go env GOPATH)/bin
endif

COVER_DIR     = .cover
COVER_PROFILE = $(COVER_DIR)/coverage.out

.PHONY: build
build: $(PINT_BIN)

$(PINT_BIN): $(PINT_SRC) go.mod go.sum
	CGO_ENABLED=0 go build -trimpath -ldflags='-X main.version=$(PINT_VERSION) -X main.commit=$(PINT_COMMIT) -s -w' ./cmd/pint

$(GOBIN)/golangci-lint: tools/golangci-lint/go.mod tools/golangci-lint/go.sum
	go install -modfile=tools/golangci-lint/go.mod github.com/golangci/golangci-lint/cmd/golangci-lint
.PHONY: lint
lint: $(GOBIN)/golangci-lint
	$(GOBIN)/golangci-lint run -E staticcheck,misspell,promlinter,revive,tenv,errorlint,exportloopref,predeclared,bodyclose

$(GOBIN)/gofumpt: tools/gofumpt/go.mod tools/gofumpt/go.sum
	go install -modfile=tools/gofumpt/go.mod mvdan.cc/gofumpt
$(GOBIN)/goimports: tools/goimports/go.mod tools/goimports/go.sum
	go install -modfile=tools/goimports/go.mod golang.org/x/tools/cmd/goimports
.PHONY: format
format: $(GOBIN)/gofumpt $(GOBIN)/goimports
	$(GOBIN)/gofumpt -l -w .
	$(GOBIN)/goimports -local github.com/cloudflare/pint -w .


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

.PHONY: debug-testscript
debug-testscript:
	for I in ./cmd/pint/tests/*.txt ; do T=`basename "$${I}" | cut -d. -f1`; echo ">>> $${T}" ; go test -count=1 -timeout=30s -v -run=TestScript/$${T} ./cmd/pint ; done

.PHONY: cover
cover: test
	go tool cover -func=$(COVER_PROFILE)

.PHONY: coverhtml
coverhtml: test
	go tool cover -html=$(COVER_PROFILE)

.PHONY: benchmark
benchmark:
	go test \
		-v \
		-count=10 \
		-run=none \
		-bench=. \
		-benchmem \
		-cpuprofile cpu.prof \
		-memprofile mem.prof \
		./cmd/pint
