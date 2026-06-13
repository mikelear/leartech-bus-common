NAME := mqube-go-common
BINARY_NAME := mqube-go-common
BUILD_TARGET = build
ORG := spring-financial-group
ORG_REPO := $(ORG)/$(NAME)
ROOT_PACKAGE := github.com/$(ORG_REPO)
MAIN_SRC_FILE=./...
GO := GO111MODULE=on go
GO_NOMOD :=GO111MODULE=off go
REV := $(shell git rev-parse --short HEAD 2> /dev/null || echo 'unknown')
RELEASE_ORG_REPO := $(ORG_REPO)
GO_VERSION := 1.24.5
GO_DEPENDENCIES := $(call rwildcard,pkg/,*.go) $(call rwildcard,cmd/,*.go)


BRANCH     := $(shell git rev-parse --abbrev-ref HEAD 2> /dev/null  || echo 'unknown')
BUILD_DATE := $(shell date +%Y%m%d-%H:%M:%S)
CGO_ENABLED = 0


REPORTS_DIR=$(BUILD_TARGET)/reports

GOTEST := $(GO) test

# set dev version unless VERSION is explicitly set via environment
VERSION ?= $(shell echo "$$(git for-each-ref refs/tags/ --count=1 --sort=-version:refname --format='%(refname:short)' 2>/dev/null)-dev+$(REV)" | sed 's/^v//')

# Build flags for setting build-specific configuration at build time - defaults to empty
#BUILD_TIME_CONFIG_FLAGS ?= ""

# Full build flags used when building binaries. Not used for test compilation/execution.
BUILDFLAGS :=  -ldflags \
  " -X $(ROOT_PACKAGE)/pkg/cmd/version.Version=$(VERSION)\
		-X $(ROOT_PACKAGE)/pkg/cmd/version.Version=$(VERSION)\
		-X $(ROOT_PACKAGE)/pkg/cmd/version.Revision='$(REV)'\
		-X $(ROOT_PACKAGE)/pkg/cmd/version.Branch='$(BRANCH)'\
		-X $(ROOT_PACKAGE)/pkg/cmd/version.BuildDate='$(BUILD_DATE)'\
		-X $(ROOT_PACKAGE)/pkg/cmd/version.GoVersion='$(GO_VERSION)'\
		$(BUILD_TIME_CONFIG_FLAGS)"

# Some tests expect default values for version.*, so just use the config package values there.
TEST_BUILDFLAGS :=  -ldflags "$(BUILD_TIME_CONFIG_FLAGS)"

ifdef DEBUG
BUILDFLAGS := -gcflags "all=-N -l" $(BUILDFLAGS)
endif

ifdef PARALLEL_BUILDS
BUILDFLAGS += -p $(PARALLEL_BUILDS)
GOTEST += -p $(PARALLEL_BUILDS)
else
# -p 4 seems to work well for people
GOTEST += -p 4
endif

ifdef DISABLE_TEST_CACHING
GOTEST += -count=1
endif

TEST_PACKAGE ?= ./...
COVER_OUT:=$(REPORTS_DIR)/cover.out
COVERFLAGS=-coverprofile=$(COVER_OUT) --covermode=count --coverpkg=./...

.PHONY: list
list: ## List all make targets
	@$(MAKE) -pRrn : -f $(MAKEFILE_LIST) 2>/dev/null | awk -v RS= -F: '/^# File/,/^# Finished Make data base/ {if ($$1 !~ "^[#.]") {print $$1}}' | egrep -v -e '^[^[:alnum:]]' -e '^$@$$' | sort

.PHONY: help
.DEFAULT_GOAL := help
help:
	@grep -h -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

full: check ## Build and run the tests
check: build test ## Build and run the tests
get-test-deps: ## Install test dependencies
	$(GO_NOMOD) get github.com/axw/gocov/gocov
	$(GO_NOMOD) get -u gopkg.in/matm/v1/gocov-html

print-version: ## Print version
	@echo $(VERSION)

build: build-app ## Builds the application

PKG_NAMES := $(shell find pkg -type f -name '*.go' -exec dirname {} \; | sort -u | sed 's|^pkg/||')

.PHONY: build-pkgs
build-pkgs:
	@for pkg in $(PKG_NAMES); do \
  		echo "Building package $$pkg"; \
		CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(BUILDFLAGS) -o build/$$pkg ./pkg/$$pkg; \
	done

build-app: $(GO_DEPENDENCIES) ## Build mqube-go-common API binary for current OS
	CGO_ENABLED=$(CGO_ENABLED) $(GO) $(BUILD_TARGET) $(BUILDFLAGS) -o build/$(BINARY_NAME) $(MAIN_SRC_FILE)

build-all: $(GO_DEPENDENCIES) build make-reports-dir ## Build all files - runtime, all tests etc.
	CGO_ENABLED=$(CGO_ENABLED) $(GOTEST) -run=nope -tags=integration -failfast -short ./... $(BUILDFLAGS)

tidy-deps: ## Cleans up dependencies
	$(GO) mod tidy
	# mod tidy only takes compile dependencies into account, let's make sure we capture tooling dependencies as well
	@$(MAKE) install-generate-deps

.PHONY: make-reports-dir
make-reports-dir:
	mkdir -p $(REPORTS_DIR)

mocks: ## Generates mock implementations from interfaces
	mockery --all --dir internal/domain/ --output internal/domain/mocks

test: ## Run tests with the "unit" build tag
	KUBECONFIG=/cluster/connections/not/allowed CGO_ENABLED=$(CGO_ENABLED) $(GOTEST) --tags=unit -failfast -short ./... $(TEST_BUILDFLAGS)

test-coverage : make-reports-dir ## Run tests and coverage for all tests with the "unit" build tag
	CGO_ENABLED=$(CGO_ENABLED) $(GOTEST) --tags=unit $(COVERFLAGS) -failfast -short ./... $(TEST_BUILDFLAGS)

test-report: make-reports-dir get-test-deps test-coverage ## Create the test report
	@gocov convert $(COVER_OUT) | gocov report

test-report-html: make-reports-dir get-test-deps test-coverage ## Create the test report in HTML format
	@gocov convert $(COVER_OUT) | gocov-html > $(REPORTS_DIR)/cover.html && open $(REPORTS_DIR)/cover.html

install: $(GO_DEPENDENCIES) ## Install the binary
	GOBIN=${GOPATH}/bin $(GO) install $(BUILDFLAGS) $(MAIN_SRC_FILE)
	mv ${GOPATH}/bin/main ${GOPATH}/bin/$(BINARY_NAME)

linux: ## Build for Linux
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 $(GO) $(BUILD_TARGET) $(BUILDFLAGS) -o build/linux/$(BINARY_NAME) $(MAIN_SRC_FILE)
	chmod +x build/linux/$(BINARY_NAME)

arm: ## Build for ARM
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=arm $(GO) $(BUILD_TARGET) $(BUILDFLAGS) -o build/arm/$(BINARY_NAME) $(MAIN_SRC_FILE)
	chmod +x build/arm/$(BINARY_NAME)

android: ## Build for ARM
	CGO_ENABLED=$(CGO_ENABLED) GOOS=android GOARCH=arm $(GO) $(BUILD_TARGET) $(BUILDFLAGS) -o build/android/$(BINARY_NAME) $(MAIN_SRC_FILE)
	chmod +x build/android/$(BINARY_NAME)

win: ## Build for Windows
	CGO_ENABLED=$(CGO_ENABLED) GOOS=windows GOARCH=amd64 $(GO) $(BUILD_TARGET) $(BUILDFLAGS) -o build/win/$(BINARY_NAME)-windows-amd64.exe $(MAIN_SRC_FILE)

darwin: ## Build for OSX
	CGO_ENABLED=$(CGO_ENABLED) GOOS=darwin GOARCH=amd64 $(GO) $(BUILD_TARGET) $(BUILDFLAGS) -o build/darwin/$(BINARY_NAME) $(MAIN_SRC_FILE)
	chmod +x build/darwin/$(BINARY_NAME)

.PHONY: release
release: clean linux test

release-all: release linux win darwin

.PHONY: goreleaser
goreleaser:
	step-go-releaser --organisation=$(ORG) --revision=$(REV) --branch=$(BRANCH) --build-date=$(BUILD_DATE) --go-version=$(GO_VERSION) --root-package=$(ROOT_PACKAGE) --version=$(VERSION)

.PHONY: clean
clean: ## Clean the generated artifacts
	rm -rf build release dist

get-fmt-deps: ## Install test dependencies
	$(GO_NOMOD) get golang.org/x/tools/cmd/goimports

.PHONY: fmt
fmt: importfmt ## Format the code
	$(eval FORMATTED = $(shell $(GO) fmt ./...))
	@if [ "$(FORMATTED)" == "" ]; \
      	then \
      	    echo "All Go files properly formatted"; \
      	else \
      		echo "Fixed formatting for: $(FORMATTED)"; \
      	fi

.PHONY: importfmt
importfmt: get-fmt-deps
	@echo "Formatting the imports..."
	goimports -w $(GO_DEPENDENCIES)

lint: ## Lints the code with golangci-lint
	golangci-lint run

.PHONY: all
all: fmt build test lint

bin/docs:
	go build $(LDFLAGS) -v -o bin/docs cmd/docs/*.go

.PHONY: docs
docs: bin/docs ## update docs
	@echo "Generating docs"
	@./bin/docs --target=./docs/cmd
	@./bin/docs --target=./docs/man/man1 --kind=man
	@rm -f ./bin/docs

MOCKERY_BINARY := $(shell which mockery)

.PHONY: check-mockery-version
check-mockery-version:
	@if [ -z "$(MOCKERY_BINARY)" ]; then \
		echo "Error: mockery is not installed. Please install it by following instructions here (https://vektra.github.io/mockery/latest/installation/)"; \
		exit 1; \
	fi; \
	MOCKERY_VERSION=$$($(MOCKERY_BINARY) version); \
	if ! echo "$$MOCKERY_VERSION" | grep -q '^v3'; then \
		echo "Error: mockery version must be v3.x.x. Installed version: $$MOCKERY_VERSION"; \
		exit 1; \
	fi; \
	echo "Mockery version $$MOCKERY_VERSION is valid."

.PHONY: mocks
mocks: check-mockery-version ## Generates mock implementations from interfaces
	$(MOCKERY_BINARY)

# Pre-commit hooks
PRE_COMMIT_BINARY := $(shell which pre-commit)

.PHONY: check-pre-commit
check-pre-commit:
	@if [ -z "$(PRE_COMMIT_BINARY)" ]; then \
		echo "Error: pre-commit is not installed. Please install it by following instructions here (https://pre-commit.com/#installation). Note can also be installed with brew."; \
		exit 1; \
	fi

pre-commits-run: ## Run pre-commit hooks
	@echo "Running pre-commit hooks..."
	@if $(PRE_COMMIT_BINARY) run --all-files; then \
	  echo "Pre-commit hooks ran successfully."; \
	else \
	  echo "Pre-commit hooks failed. Either the hooks are failing or the install failed. Try rerunning or check output."; \
	  exit 1; \
	fi

.PHONY:
pre-commit-install: check-pre-commit ## Install pre-commit hooks
	@echo "Installing pre-commit hooks..."
	@$(PRE_COMMIT_BINARY) install
	@echo "Making pre-commit executable..."
	chmod +x .git/hooks/pre-commit
	@echo "Pre-commit installed successfully."
	@echo "Making pre-commit cached hooks executable..."
	chmod -R +x $(HOME)/.cache/pre-commit/*
	@echo "Running pre-commit hooks to test install..."
	@if $(PRE_COMMIT_BINARY) run --all-files; then \
      echo "Pre-commit hooks ran successfully."; \
    else \
      echo "Pre-commit hooks failed. Either the hooks are failing or the install failed. Try rerunning or check output."; \
      exit 1; \
    fi

.PHONY: update-pre-commit
pre-commit-update: check-pre-commit ## Update pre-commit hooks
	@echo "Updating pre-commit hooks..."
	@$(PRE_COMMIT_BINARY) autoupdate
	@echo "Pre-commit hooks updated successfully."
	@$(MAKE) pre-commits-run

