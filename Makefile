.PHONY: build test install

build:
	go build ./...

test:
	go test ./...

install:
	mkdir -p $(HOME)/.local/bin
	go build -o $(HOME)/.local/bin/autotune ./cmd/autotune
