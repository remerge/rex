PACKAGE := github.com/remerge/rex

# http://stackoverflow.com/questions/322936/common-gnu-makefile-directory-path#comment11704496_324782
TOP := $(dir $(CURDIR)/$(word $(words $(MAKEFILE_LIST)),$(MAKEFILE_LIST)))

GOOP=goop
GO=$(GOOP) exec go
GOFMT=gofmt -w

GOFILES=$(shell git ls-files | grep '\.go$$')
SRCS=$(wildcard main/*.go)
OBJS=$(patsubst main/%.go,%,$(SRCS))

.PHONY: build clean test fmt dep

all: build

build: fmt
	$(GO) build $(SRCS)

clean:
	$(GO) clean
	rm -f $(OBJS)

test:
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
