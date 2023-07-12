GO ?= go

.PHONY: all
all:
	CGO_ENABLED=0 $(GO) build -ldflags="-X main.Commit=$$(git describe --tags --abbrev=8 --dirty --always --long)"

.PHONY: verify
verify: verify-install verify-space verify-golangci

.PHONY: verify-install
verify-install:
	@command golangci-lint &> /dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.53.3

.PHONY: verify-golangci
verify-golangci:
	golangci-lint run

.PHONY: verify-space
verify-space: ## Ensure no whitespace at EOL
	@if git -P grep -I -n '\s$$'; then \
		echo "^^^^ extra whitespace at EOL, please fix"; \
		exit 1; \
	fi
