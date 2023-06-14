all:
	go build

gen-changelog:
	git cliff --unreleased --tag 1.0.1 --prepend CHANGELOG.md	
