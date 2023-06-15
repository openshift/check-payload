all:
	CGO_ENABLED=0 go build -ldflags="-X main.Commit=$$(git describe --tags --abbrev=8 --dirty --always --long)"

gen-changelog:
	git cliff --latest --unreleased --prepend CHANGELOG.md	
