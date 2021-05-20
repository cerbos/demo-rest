.PHONY: build
build:
	@ goreleaser --config=.goreleaser-dev.yml --snapshot --skip-publish --rm-dist

