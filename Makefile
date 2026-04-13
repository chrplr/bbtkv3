VERSION   ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
BUILD_DIR := _build
DIST_DIR  := binaries

CMDS := bbtk-capture bbtk-detect-port bbtk-adjust-thresholds \
        bbtk-get-thresholds bbtk-set-thresholds bbtk-set-smoothing \
        get-serial-port-list ibbtk events-stats bbtk-send-command \
        bbtk-event-marking

PLATFORMS ?= darwin linux windows
ARCHS     ?= amd64 arm64

GIT_HASH   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GO_SOURCES := $(shell find . -name '*.go' -not -path './$(BUILD_DIR)/*' -not -path './$(DIST_DIR)/*') go.mod go.sum

LDFLAGS := -ldflags "\
  -X github.com/chrplr/bbtkv3.Version=$(VERSION) \
  -X github.com/chrplr/bbtkv3.Build=$(GIT_HASH) \
  -X main.Version=$(VERSION) \
  -X main.Build=$(GIT_HASH)"

# ── Local build ───────────────────────────────────────────────────────────────

all: build test

build: $(addprefix $(BUILD_DIR)/, $(CMDS))

$(BUILD_DIR)/%: $(GO_SOURCES)
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $@ ./cmd/$*

test:
	@echo "Testing..."
	@go test ./... -v

clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR) $(DIST_DIR)

# ── Cross-platform distribution ───────────────────────────────────────────────

# Windows binaries need a .exe suffix; the other platforms get nothing.
EXE_windows := .exe

PLAT_ARCHS := $(foreach p,$(PLATFORMS),$(foreach a,$(ARCHS),$(p)-$(a)))
ZIPS       := $(patsubst %,$(DIST_DIR)/bbtkv3-%-$(VERSION).zip,$(PLAT_ARCHS))

dist: $(ZIPS)

# Generate one rule per platform/arch pair.
# $(1) = OS  (darwin | linux | windows)
# $(2) = ARCH (amd64 | arm64)
define build_zip
$(DIST_DIR)/bbtkv3-$(1)-$(2)-$(VERSION).zip: $(GO_SOURCES)
	@mkdir -p $(DIST_DIR)
	@echo "  Building $(1)/$(2)..."
	@$(foreach cmd,$(CMDS), \
	    GOOS=$(1) GOARCH=$(2) go build $(LDFLAGS) \
	        -o $(DIST_DIR)/$(cmd)-$(1)-$(2)-$(VERSION)$(EXE_$(1)) \
	        ./cmd/$(cmd);)
	@cd $(DIST_DIR) && zip -q bbtkv3-$(1)-$(2)-$(VERSION).zip \
	    $(foreach cmd,$(CMDS),$(cmd)-$(1)-$(2)-$(VERSION)$(EXE_$(1)))
	@rm -f $(foreach cmd,$(CMDS),$(DIST_DIR)/$(cmd)-$(1)-$(2)-$(VERSION)$(EXE_$(1)))
	@echo "  Created $$@"
endef

$(foreach p,$(PLATFORMS),$(foreach a,$(ARCHS),$(eval $(call build_zip,$(p),$(a)))))

# ── GitHub release ────────────────────────────────────────────────────────────

release: dist
	gh release create v$(VERSION) $(ZIPS) \
	    --title "v$(VERSION)" \
	    --generate-notes

.PHONY: all build test clean dist release
