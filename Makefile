.PHONY: run
run:
	@ cerbos run --set=storage.disk.directory=cerbos/policies -- go run main.go

.PHONY: build
build:
	@ goreleaser --config=.goreleaser.yml --snapshot --skip-publish --rm-dist





