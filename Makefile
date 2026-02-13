.PHONY: build build-mocks

build:
	go build -o .gemini/skills/graphdb/scripts/graphdb ./cmd/graphdb

build-mocks:
	go build -tags test_mocks -o .gemini/skills/graphdb/scripts/graphdb_test ./cmd/graphdb
