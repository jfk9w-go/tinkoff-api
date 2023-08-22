MODULE := $(shell head -1 go.mod | cut -d ' ' -f2)

build:
	go build -v ./...

test:
	go test -v ./...

tools:
	go install golang.org/x/tools/cmd/goimports@latest

fmt: tools
	goimports -local $(MODULE) -l -w $(shell find . -type f -name '*.go' -not -path "./vendor/*")
