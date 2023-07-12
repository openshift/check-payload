GO ?= go
GOLANGCI_LINT_CACHE ?= /tmp/golangci-cache

.PHONY: all
all:
	CGO_ENABLED=0 $(GO) build -ldflags="-X main.Commit=$$(git describe --tags --abbrev=8 --dirty --always --long)"

.PHONY: verify
verify: verify-space verify-golangci

.PHONY: verify-golangci
verify-golangci:
	GOLANGCI_LINT_CACHE=${GOLANGCI_LINT_CACHE} golangci-lint run

.PHONY: verify-space
verify-space: ## Ensure no whitespace at EOL
	@if git -P grep -I -n '\s$$'; then \
		echo "^^^^ extra whitespace at EOL, please fix"; \
		exit 1; \
	fi
