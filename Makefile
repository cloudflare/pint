PINT_BIN     := pint
PINT_GO_DIRS := cmd internal
PINT_SRC     := $(shell find $(PINT_GO_DIRS) -type f -name '*.go')
PINT_VERSION ?= $(shell git describe --tags --always --dirty='-dev')
PINT_COMMIT  ?= $(shell git rev-parse HEAD)

GOFLAGS := -tags stringlabels

COVER_DIR     = .cover
COVER_PROFILE = $(COVER_DIR)/coverage.out

.PHONY: build
build: $(PINT_BIN)

$(PINT_BIN): $(PINT_SRC) go.mod go.sum
	CGO_ENABLED=0 go build \
	    $(GOFLAGS) \
		-trimpath \
		-ldflags='-X main.version=$(PINT_VERSION) -X main.commit=$(PINT_COMMIT) -s -w' \
		./cmd/pint

.PHONY: lint
lint:
	go tool -modfile=tools/golangci-lint/go.mod golangci-lint run

.PHONY: format
format:
	go tool -modfile=tools/betteralign/go.mod betteralign -test_files -apply ./...
	go tool -modfile=tools/golangci-lint/go.mod golangci-lint fmt

.PHONY: test
test:
	mkdir -p $(COVER_DIR)
	echo 'mode: atomic' > $(COVER_PROFILE)
	go test \
		$(GOFLAGS) \
		-covermode=atomic \
		-coverprofile=$(COVER_PROFILE) \
		-coverpkg=./... \
		-race \
		-count=3 \
		-timeout=15m \
		./...

.PHONY: debug-testscript
debug-testscript:
	for I in ./cmd/pint/tests/*.txt ; do T=`basename "$${I}" | cut -d. -f1`; echo ">>> $${T}" ; go test -count=1 -timeout=30s -v -run=TestScript/$${T} ./cmd/pint || exit 1 ; done

.PHONY: update-snapshots
update-snapshots:
	UPDATE_SNAPS=true UPDATE_SNAPSHOTS=1 go test -count=1  ./...
	$(MAKE) test

.PHONY: cover
cover: test
	go tool cover -func=$(COVER_PROFILE)

.PHONY: coverhtml
coverhtml: test
	go tool cover -html=$(COVER_PROFILE)

.PHONY: benchmark
benchmark:
	go test \
	    $(GOFLAGS) \
		-timeout=15m \
		-count=5 \
		-run=none \
		-bench=. \
		-benchmem \
		./...

.PHONY: benchmark-diff
benchmark-diff:
	echo "Benchmark diff:" | tee benchstat.txt
	echo "" | tee -a benchstat.txt
	echo '```' | tee -a benchstat.txt
	go tool -modfile=tools/benchstat/go.mod benchstat old.txt new.txt | tee -a benchstat.txt
	echo '```' | tee -a benchstat.txt
