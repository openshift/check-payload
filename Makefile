GO ?= go

all:
	CGO_ENABLED=0 $(GO) build -ldflags="-X main.Commit=$$(git describe --tags --abbrev=8 --dirty --always --long)"

gen-changelog:
	git cliff --latest --unreleased --prepend CHANGELOG.md	
