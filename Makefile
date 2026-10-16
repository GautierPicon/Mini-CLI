BINARY=mini
PKG=./cmd/mini

.PHONY: build test vet run-mock clean

build:
	go build -o $(BINARY) $(PKG)

test:
	go test ./...
	go vet ./...

vet:
	go vet ./...

run-mock:
	go run $(PKG) run --mock "Hello"

clean:
	rm -f $(BINARY)

# cross-compile smoke (no python needed for build)
build-all:
	GOOS=linux GOARCH=amd64 go build -o dist/mini-linux-amd64 $(PKG)
	GOOS=darwin GOARCH=arm64 go build -o dist/mini-darwin-arm64 $(PKG)
	GOOS=windows GOARCH=amd64 go build -o dist/mini-windows-amd64.exe $(PKG)
