# Function to execute a command.
# Accepts command to execute as first parameter.
define exec-command
$(1)

endef

GOCMD?= go

# SRC_ROOT is the top of the source tree.
SRC_ROOT := $(shell git rev-parse --show-toplevel)

TOOLS_MOD_DIR   := $(SRC_ROOT)/internal/tools
TOOLS_BIN_DIR   := $(SRC_ROOT)/.tools
TOOLS_MOD_REGEX := "\s+_\s+\".*\""
TOOLS_PKG_NAMES := $(shell grep -E $(TOOLS_MOD_REGEX) < $(TOOLS_MOD_DIR)/tools.go | tr -d " _\"" | grep -vE '/v[0-9]+$$')
TOOLS_BIN_NAMES := $(addprefix $(TOOLS_BIN_DIR)/, $(notdir $(shell echo $(TOOLS_PKG_NAMES))))

GOFUMPT      := $(TOOLS_BIN_DIR)/gofumpt
GOIMPORTS    := $(TOOLS_BIN_DIR)/goimports
GOACC	     := $(TOOLS_BIN_DIR)/go-acc

# Find all .proto files.
BASELINE_PROTO_FILES := $(wildcard internal/opamp-spec/proto/*.proto)

all: test build-examples

.PHONY: test
test:
	$(GOCMD) test -race ./...
	cd internal/examples && $(GOCMD) test -race ./...

.PHONY: test-with-cover
test-with-cover: $(GOACC)
	$(GOACC) --output=coverage.out --ignore=protobufs ./...

show-coverage: test-with-cover
	# Show coverage as HTML in the default browser.
	$(GOCMD) tool cover -html=coverage.out

.PHONY: build-examples
build-examples: build-example-agent build-example-supervisor build-example-server

build-example-agent:
	cd internal/examples && $(GOCMD) build -o agent/bin/agent agent/main.go

build-example-supervisor:
	cd internal/examples && $(GOCMD) build -o supervisor/bin/supervisor supervisor/main.go

build-example-server:
	cd internal/examples && $(GOCMD) build -o server/bin/server server/main.go

run-examples: build-examples
	cd internal/examples/server && ./bin/server &
	@echo Server UI is running at http://localhost:4321/
	cd internal/examples/agent && ./bin/agent

OTEL_DOCKER_PROTOBUF ?= otel/build-protobuf:0.14.0

# Generate Protobuf Go files.
.PHONY: gen-proto
gen-proto:
	mkdir -p ${PWD}/internal/proto/

	@if [ ! -d "${PWD}/internal/opamp-spec/proto" ]; then \
		echo "${PWD}/internal/opamp-spec/proto does not exist."; \
		echo "Run \`git submodule update --init\` to fetch the submodule"; \
		exit 1; \
	fi

	$(foreach file,$(BASELINE_PROTO_FILES),$(call exec-command,docker run --rm -v${PWD}:${PWD} \
        -w${PWD} $(OTEL_DOCKER_PROTOBUF) --proto_path=${PWD}/internal/opamp-spec/proto/ \
        --go_out=${PWD}/internal/proto/ -I${PWD}/internal/proto/ ${PWD}/$(file)))

	cp -R internal/proto/github.com/open-telemetry/opamp-go/protobufs/* protobufs/
	rm -rf internal/proto/github.com/
	$(MAKE) fmt

.PHONY: gomoddownload
gomoddownload:
	$(GOCMD) mod download

.PHONY: install-tools
install-tools: $(TOOLS_BIN_NAMES)

$(TOOLS_BIN_DIR):
	mkdir -p $@

$(TOOLS_BIN_NAMES): $(TOOLS_BIN_DIR) $(TOOLS_MOD_DIR)/go.mod
	cd $(TOOLS_MOD_DIR) && $(GOCMD) build -o $@ -trimpath $(filter %/$(notdir $@),$(TOOLS_PKG_NAMES))

.PHONY: tidy
tidy:
	rm -fr go.sum
	$(GOCMD) mod tidy
	cd internal/examples && rm -fr go.sum && $(GOCMD) mod tidy
	cd $(TOOLS_MOD_DIR) && rm -fr go.sum && $(GOCMD) mod tidy

.PHONY: fmt
fmt: $(GOIMPORTS) $(GOFUMPT)
	gofmt -w -s ./
	$(GOIMPORTS) -w  ./
	$(GOFUMPT) -l -w .
