GO_FILES      = $(shell find . -path ./vendor -prune -o -type f -name "*.go" -print)
IMPORT_PATH   = $(shell pwd | sed "s|^$(GOPATH)/src/||g")
GIT_HASH      = $(shell git rev-parse HEAD)
LDFLAGS       = -w -X $(IMPORT_PATH)/version.PreRelease=$(PRE_RELEASE)

build: clean bindata
	@go build -ldflags '$(LDFLAGS)'

clean:
	@rm -f archive-backend

install:
	@godep restore

test: bindata
	@go test $(shell go list ./... | grep -v /vendor/)

lint:
	@golint $(GO_FILES) || true

fmt:
	@gofmt -w $(shell find . -type f -name '*.go' -not -path "./vendor/*")

bindata:
	@go-bindata data/... && sed -i 's/package main/package bindata/' bindata.go && mv bindata.go ./bindata

bindata_debug:
	@go-bindata -debug data/... && sed -i 's/package main/package bindata/' bindata.go && mv bindata.go ./bindata

.PHONY: all clean test lint fmt bindata bindata_debug
