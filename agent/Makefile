BINARY=clawpeteer-agent
VERSION=1.0.0

.PHONY: build build-all clean test

build:
	go build -ldflags "-X main.Version=$(VERSION)" -o $(BINARY) .

build-all:
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main.Version=$(VERSION)" -o dist/$(BINARY)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -ldflags "-X main.Version=$(VERSION)" -o dist/$(BINARY)-linux-arm64 .
	GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.Version=$(VERSION)" -o dist/$(BINARY)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.Version=$(VERSION)" -o dist/$(BINARY)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -ldflags "-X main.Version=$(VERSION)" -o dist/$(BINARY)-windows-amd64.exe .

clean:
	rm -rf dist/ $(BINARY)

test:
	go test ./... -v
