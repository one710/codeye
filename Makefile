SHELL := /bin/bash

.PHONY: build test race fmt vet tidy check

build:
	go build ./...

test:
	go test ./...

race:
	go test -race ./...

fmt:
	gofmt -w $$(go list -f '{{.Dir}}' ./...)

vet:
	go vet ./...

tidy:
	go mod tidy

check: fmt vet test
