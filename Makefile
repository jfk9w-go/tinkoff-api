MODULE := $(shell head -1 go.mod | cut -d ' ' -f2)
GOIMPORTS := $(shell go env GOPATH)/bin/goimports

$(GOIMPORTS):
	go install golang.org/x/tools/cmd/goimports@latest

fmt: $(GOIMPORTS)
	$(GOIMPORTS) -local $(MODULE) -l -w $(shell find . -type f -name '*.go' -not -path "./vendor/*")

test:
	go test -v ./...

build:
	go build -v ./...