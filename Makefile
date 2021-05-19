.PHONY: build
build:
	@ goreleaser --config=.goreleaser.yml --snapshot --skip-publish --rm-dist
