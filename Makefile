all:
	go build -ldflags="-X main.Commit=$$(git describe --tags --abbrev=8 --dirty --always --long)"

gen-changelog:
	git cliff --unreleased --tag 1.0.1 --prepend CHANGELOG.md	
