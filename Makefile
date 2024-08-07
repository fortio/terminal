all: lint test demo

GO_BUILD_TAGS:=no_net,no_json,no_pprof

demo:
	go run ./example/ -loglevel debug

test:
	CGO_ENABLED=0 go test -tags $(GO_BUILD_TAGS) ./...
	echo "" | go run ./example # check non terminal input

lint: .golangci.yml
	CGO_ENABLED=0 golangci-lint run --build-tags $(GO_BUILD_TAGS)

.golangci.yml: Makefile
	curl -fsS -o .golangci.yml https://raw.githubusercontent.com/fortio/workflows/main/golangci.yml

.PHONY: all lint test demo
