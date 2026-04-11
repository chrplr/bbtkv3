VERSION ?= dev
BUILD_DIR := _build

CMDS := bbtk-capture bbtk-detect-port bbtk-adjust-thresholds \
        bbtk-get-thresholds bbtk-set-thresholds bbtk-set-smoothing \
        get-serial-port-list ibbtk events-stats

GIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "\
  -X github.com/chrplr/bbtkv3.Version=$(VERSION) \
  -X github.com/chrplr/bbtkv3.Build=$(GIT_HASH) \
  -X main.Version=$(VERSION) \
  -X main.Build=$(GIT_HASH)"

all: build test

build: $(addprefix $(BUILD_DIR)/, $(CMDS))

$(BUILD_DIR)/%:
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $@ ./cmd/$*

test:
	@echo "Testing..."
	@go test ./... -v

clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)

.PHONY: all build test clean
