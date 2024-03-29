OBJS = $(shell find cmd -mindepth 1 -type d -execdir printf '%s\n' {} +)
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
COMMIT := $(shell git log -1 --format='%H')
BASEPKG = github.com/emerishq/emeris-rpcwatcher
EXTRAFLAGS :=

.PHONY: $(OBJS) clean

all: $(OBJS)

clean:
	@rm -rf build

test:
	go test -v -race ./... -cover

integration-test:
	go test -timeout 15m -v ${BASEPKG}/integration

lint:
	golangci-lint run ./...

$(OBJS):
	go build -o build/$@ -ldflags='-X main.Version=${BRANCH}-${COMMIT}' ${EXTRAFLAGS} ${BASEPKG}/cmd/$@
