PACKAGE := github.com/remerge/rex

# http://stackoverflow.com/questions/322936/common-gnu-makefile-directory-path#comment11704496_324782
TOP := $(dir $(CURDIR)/$(word $(words $(MAKEFILE_LIST)),$(MAKEFILE_LIST)))

GOOP=goop
GO=$(GOOP) exec go
GOFMT=gofmt -w -s

GOFILES=$(shell git ls-files | grep '\.go$$')
MAINGO=$(wildcard main/*.go)
MAIN=$(patsubst main/%.go,%,$(MAINGO))

.PHONY: build run watch clean test fmt dep

all: build

build: fmt
	$(GO) build $(MAINGO)

build-log: fmt
	$(GO) build -gcflags=-m $(MAINGO) 2>&1 | tee build.log
	grep escapes build.log | sort > escape.log

install: build
	$(GO) install $(PACKAGE)

run: build
	./$(MAIN)

watch:
	go get github.com/cespare/reflex
	reflex -t10s -r '\.go$$' -s -- sh -c 'make build test && ./$(MAIN)'

clean:
	$(GO) clean
	rm -f $(MAIN)

lint: install
	go get github.com/alecthomas/gometalinter
	gometalinter --install
	$(GOOP) exec gometalinter -D golint -D gocyclo -D errcheck -D dupl

test: build lint
	go get github.com/smartystreets/goconvey
	$(GO) test
	$(GO) test -v $(PACKAGE)/rand
	$(GO) test -v $(PACKAGE)/rollbar

bench:
	$(GO) test -bench=. -cpu 4

fmt:
	$(GOFMT) $(GOFILES)

dep:
	go get github.com/nitrous-io/goop
	goop install
	mkdir -p $(dir $(TOP)/.vendor/src/$(PACKAGE))
	ln -nfs $(TOP) $(TOP)/.vendor/src/$(PACKAGE)
