PROJECT := rex
PACKAGE := github.com/remerge/$(PROJECT)

# http://stackoverflow.com/questions/322936/common-gnu-makefile-directory-path#comment11704496_324782
TOP := $(dir $(CURDIR)/$(word $(words $(MAKEFILE_LIST)),$(MAKEFILE_LIST)))

GOFMT=gofmt -w -s

GOSRCDIR=$(GOPATH)/src/$(PACKAGE)
GOPATHS=$(shell glide novendor | grep -v /main/)
GOFILES=$(shell git ls-files | grep '\.go$$')

CODE_VERSION=$(TRAVIS_COMMIT)
ifeq ($(CODE_VERSION),)
	CODE_VERSION=$(shell git rev-parse --short HEAD)-dev
endif

CODE_BUILD=$(TRAVIS_REPO_SLUG)\#$(TRAVIS_JOB_NUMBER)
ifeq ($(CODE_BUILD),\#)
	CODE_BUILD=$(PACKAGE)\#$(shell whoami)
endif

LDFLAGS=-X github.com/remerge/rex.CodeVersion=$(CODE_VERSION) -X github.com/remerge/rex.CodeBuild=$(CODE_BUILD)@$(shell date -u +%FT%TZ)

.PHONY: build clean lint test bench fmt dep init up gen release deploy

all: fmt

clean:
	go clean
	rm -rf $(TOP)/vendor/

lint:
	cd $(GOSRCDIR) && \
		gometalinter --vendor --errors --fast --deadline=60s -D gotype $(GOPATHS)

test: lint
	cd $(GOSRCDIR) && \
		go test -timeout 60s -v $(GOPATHS)

bench:
	cd $(GOSRCDIR) && \
		go test -bench=. -cpu 4 $(GOPATHS)

fmt:
	$(GOFMT) $(GOFILES)

dep:
	go get -u github.com/Masterminds/glide
	go get -u github.com/alecthomas/gometalinter
	gometalinter --install
	cd $(GOSRCDIR) && glide install

init:
	cd $(GOSRCDIR) && \
		glide init

up:
	cd $(GOSRCDIR) && \
		glide update

gen:
	cd $(GOSRCDIR) && \
		go generate $(GOPATHS)
	$(GOFMT) $(GOFILES)

release:
	git push origin master master:production
