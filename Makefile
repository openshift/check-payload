GO ?= go

.PHONY: all
all:
	CGO_ENABLED=0 $(GO) build -ldflags="-X main.Commit=$$(git describe --tags --abbrev=8 --dirty --always --long)"

.PHONY: verify
verify: verify-space verify-golangci

.PHONY: test
test:
	go test -v ./...

.PHONY: verify-golangci
verify-golangci:
	golangci-lint run

.PHONY: verify-space
verify-space: ## Ensure no whitespace at EOL
	@if git -P grep -I -n '\s$$' -- ':(exclude)vendor'; then \
		echo "^^^^ extra whitespace at EOL, please fix"; \
		exit 1; \
	fi
