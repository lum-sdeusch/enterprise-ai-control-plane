SHELL := /bin/bash
.DEFAULT_GOAL := help

# Tools are looked up on PATH first, then in ./bin (populated by `make tools`).
BIN     := $(CURDIR)/bin
export PATH := $(BIN):$(PATH)
PY      ?= python3
OPA     ?= opa
HELM    ?= helm
BUF     ?= buf
PROFILE ?= laptop
REVISION ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
OPA_VERSION  ?= v1.22.2
HELM_VERSION ?= v3.19.0
KUBE_VERSION ?= 1.31.0

TOOL_MANIFESTS  := $(wildcard tools/*/manifest.yaml)
AGENT_MANIFESTS := $(wildcard agents/*/manifest.yaml)
TEAM_MANIFESTS  := $(wildcard teams/*/manifest.yaml)

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# ---------------------------------------------------------------------------
# Toolchain
# ---------------------------------------------------------------------------
.PHONY: tools
tools: ## Install opa, helm and buf into ./bin via the Go toolchain (no curl-pipe-sh)
	@mkdir -p $(BIN)
	@command -v opa  >/dev/null || GOBIN=$(BIN) go install github.com/open-policy-agent/opa@$(OPA_VERSION)
	@command -v helm >/dev/null || GOBIN=$(BIN) go install helm.sh/helm/v3/cmd/helm@$(HELM_VERSION)
	@command -v buf  >/dev/null || GOBIN=$(BIN) go install github.com/bufbuild/buf/cmd/buf@latest
	@GOBIN=$(BIN) go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@GOBIN=$(BIN) go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "tools ready in $(BIN)"

# ---------------------------------------------------------------------------
# Manifests and policy data
# ---------------------------------------------------------------------------
.PHONY: validate
validate: ## Validate every tool, agent and team manifest against its JSON schema
	$(PY) scripts/validate-manifests.py

.PHONY: compile
compile: validate ## Compile manifests into OPA data, the Keycloak realm import and the chart schema
	$(PY) scripts/compile-manifests.py
	$(PY) scripts/sync-chart-schema.py

.PHONY: policy-fmt
policy-fmt: ## Format Rego in place
	$(OPA) fmt --write policies/

.PHONY: policy-test
policy-test: compile ## Run OPA policy checks and unit tests against the fixtures
	$(OPA) fmt --fail --list policies/
	$(OPA) check --strict policies/rego policies/tests
	$(OPA) test --explain fails policies/ -v

.PHONY: policy-bundle
policy-bundle: policy-test ## Build the signed-ready OPA bundle (dist/eacp-bundle.tar.gz)
	scripts/build-bundle.sh $(REVISION)

# ---------------------------------------------------------------------------
# Helm chart
# ---------------------------------------------------------------------------
.PHONY: chart-lint
chart-lint: ## Lint the mcp-tool chart
	$(HELM) lint charts/mcp-tool

.PHONY: chart-template
chart-template: chart-lint ## Render the mcp-tool chart for every tool manifest into dist/rendered/
	@mkdir -p dist/rendered
	@for m in $(TOOL_MANIFESTS); do \
	  n=$$(basename $$(dirname $$m)); \
	  echo "rendering $$n"; \
	  $(HELM) template $$n charts/mcp-tool --kube-version $(KUBE_VERSION) -f platform/values/$(PROFILE).yaml -f $$m > dist/rendered/$$n.yaml || exit 1; \
	done

# ---------------------------------------------------------------------------
# Services
# ---------------------------------------------------------------------------
.PHONY: proto
proto: ## Regenerate Go code from the vendored agentgateway ExtMCP proto
	cd adapter && $(BUF) generate

.PHONY: adapter
adapter: ## Build and test the Go adapter
	cd adapter && go vet ./... && go build ./... && go test ./...

.PHONY: guardrails
guardrails: ## Test the Python guardrail service (uses the active python3)
	cd guardrails && $(PY) -m pytest -q

# ---------------------------------------------------------------------------
# Docs
# ---------------------------------------------------------------------------
.PHONY: adrs
adrs: ## Regenerate docs/adr/*.md from docs/adr/decisions.yaml
	$(PY) scripts/gen-adrs.py

# ---------------------------------------------------------------------------
# Aggregate
# ---------------------------------------------------------------------------
.PHONY: test
test: validate compile policy-test chart-template adapter guardrails adrs ## Run every local check
	@echo "all checks passed"

# ---------------------------------------------------------------------------
# Cluster
# ---------------------------------------------------------------------------
.PHONY: up
up: ## Create the kind cluster, install Argo CD, seed OpenBao and wait for the platform (PROFILE, EACP_REVISION)
	PROFILE=$(PROFILE) scripts/up.sh

.PHONY: down
down: ## Delete the kind cluster
	kind delete cluster --name eacp || true

.PHONY: e2e
e2e: ## Run the six demo scenarios against a running cluster (skeleton)
	PROFILE=$(PROFILE) scripts/e2e.sh

.PHONY: smoke
smoke: ## Gateway and identity smoke test: tickets via its own route and the aggregate, OAuth audiences
	PROFILE=$(PROFILE) scripts/e2e.sh --smoke

.PHONY: pathway-cluster
pathway-cluster: ## Adapter-in-the-pathway checks on the running cluster: listings, denials, audit rows, trace, fail-closed
	PROFILE=$(PROFILE) scripts/e2e.sh --pathway

.PHONY: diagnose
diagnose: ## Collect cluster state (Applications, pods, events, logs) into .eacp/diagnose.txt
	scripts/diagnose.sh

.PHONY: demo-images
demo-images: ## Build the demo MCP servers and load them into the kind cluster (rerun make up to deploy)
	scripts/local-images.sh tools

.PHONY: policy-load
policy-load: ## Laptop: rebuild the policy bundle from this checkout and load it into the running cluster
	PROFILE=$(PROFILE) scripts/up.sh policy

.PHONY: pathway
pathway: ## The adapter behind a real agentgateway, without Kubernetes (needs Docker or EACP_AGENTGATEWAY)
	EACP_PYTHON=$(PY) test/pathway/run.sh

.PHONY: pathway-probe
pathway-probe: ## Print exactly what agentgateway sends to the policy hook on each phase
	EACP_PYTHON=$(PY) test/pathway/run.sh --probe

.PHONY: demo
demo: e2e ## Alias for e2e

.PHONY: clean
clean: ## Remove build artefacts
	rm -rf dist adapter/bin guardrails/.pytest_cache
