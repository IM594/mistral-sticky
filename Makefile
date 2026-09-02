.PHONY: test cover build

test:
	go test ./... -count=1

cover:
	go test ./... -count=1 -coverprofile=coverage.out
	go tool cover -func=coverage.out

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/mistral-sticky ./cmd/mistral-sticky
