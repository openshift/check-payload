all:
	go build -ldflags="-X main.Commit=$$(git describe --tags --abbrev=8 --dirty --always --long)"

gen-changelog:
	git cliff --unreleased --prepend CHANGELOG.md	
