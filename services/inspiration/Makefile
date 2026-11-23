.PHONY: build build-linux test lint docker-build clean

BIN := bin/inspiration
BIN_LINUX := bin/inspiration-linux
VERSION ?= 0.1.0
LDFLAGS := -s -w -X main.version=$(VERSION)
GOARCH ?= amd64

build:
	@mkdir -p $(dir $(BIN))
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/inspiration

build-linux:
	@mkdir -p $(dir $(BIN_LINUX))
	CGO_ENABLED=0 GOOS=linux GOARCH=$(GOARCH) go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_LINUX) ./cmd/inspiration

test:
	go test ./...

lint:
	go vet ./...

docker-build: build-linux
	docker build --platform linux/$(GOARCH) --provenance=false -t omnimux-inspiration:local .

clean:
	rm -rf $(BIN) $(BIN_LINUX)
