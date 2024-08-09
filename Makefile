all: lint test demo

GO_BUILD_TAGS:=no_net,no_json,no_pprof

demo:
	go run -tags $(GO_BUILD_TAGS) ./example/ -loglevel debug -only-valid

tinygo-demo:
	# No luck on mac https://github.com/tinygo-org/tinygo/issues/4395
	# on linux, after doesn't work it seems.
	CGO_ENABLED=0 tinygo build -tags $(GO_BUILD_TAGS) -o example-tinygo ./example/
	./example-tinygo -loglevel debug

test:
	CGO_ENABLED=0 go test -tags $(GO_BUILD_TAGS) ./...
	(echo -e "help\rafter 1s hi\r"; sleep 2) | go run -tags $(GO_BUILD_TAGS) ./example # check non terminal input

lint: .golangci.yml
	CGO_ENABLED=0 golangci-lint run --build-tags $(GO_BUILD_TAGS)

.golangci.yml: Makefile
	curl -fsS -o .golangci.yml https://raw.githubusercontent.com/fortio/workflows/main/golangci.yml


.PHONY: all lint test demo tinygo-demo
