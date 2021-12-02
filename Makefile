OBJS = $(shell find cmd -mindepth 1 -type d -execdir printf '%s\n' {} +)
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
COMMIT := $(shell git log -1 --format='%H')
BASEPKG = github.com/allinbits/emeris-rpcwatcher
EXTRAFLAGS :=

.PHONY: $(OBJS) clean

all: $(OBJS)

clean:
	@rm -rf build

test:
	go test -v -race ./...

$(OBJS):
	golangci-lint run ./...
	go build -o build/$@ -ldflags='-X main.Version=${BRANCH}-${COMMIT}' ${EXTRAFLAGS} ${BASEPKG}/cmd/$@
