#!make
-include .env
export $(shell [ -f ".env" ] && sed 's/=.*//' .env)

export BIN_DIR=./bin
export DIST_DIR=./dist
export PROJ_PATH=github.com/alexj212/dumpr
export DOCKER_APP_NAME=dumpr


export DATE := $(shell date +%Y.%m.%d-%H%M)
export BUILT_ON_IP := $(shell [ $$(uname) = Linux ] && hostname -i || hostname )
export RUNTIME_VER := $(shell go version)
export BUILT_ON_OS=$(shell uname -a)

export LATEST_COMMIT := $(shell git rev-parse HEAD 2> /dev/null)
export COMMIT_CNT := $(shell git rev-list --all 2> /dev/null | wc -l | sed 's/ //g' )
export BRANCH := $(shell git branch  2> /dev/null |grep -v "no branch"| grep \*|cut -d ' ' -f2)
export GIT_REPO := $(shell git config --get remote.origin.url  2> /dev/null)
export COMMIT_DATE := $(shell git log -1 --format=%cd  2> /dev/null)

export BUILT_BY := $(shell whoami  2> /dev/null)
export VERSION=v0.0.0


ifeq ($(BRANCH),)
BRANCH := master
endif

ifeq ($(COMMIT_CNT),)
COMMIT_CNT := 0
endif

export BUILD_NUMBER := ${BRANCH}-${COMMIT_CNT}


export COMPILE_LDFLAGS=-s -X "main.BuildDate=${DATE}" \
                          -X "main.GitRepo=${GIT_REPO}" \
                          -X "main.BuiltBy=${BUILT_BY}" \
                          -X "main.CommitDate=${COMMIT_DATE}" \
                          -X "main.LatestCommit=${LATEST_COMMIT}" \
                          -X "main.Branch=${BRANCH}" \
						  -X "main.Version=${VERSION}"


build_info: check_prereq ## Build the container
	@echo ''
	@echo '---------------------------------------------------------'
	@echo 'DATE              $(DATE)'
	@echo 'LATEST_COMMIT     $(LATEST_COMMIT)'
	@echo 'BUILD_NUMBER      $(BUILD_NUMBER)'
	@echo 'BUILT_ON_IP       $(BUILT_ON_IP)'
	@echo 'BUILT_ON_OS       $(BUILT_ON_OS)'
	@echo 'RUNTIME_VER       $(RUNTIME_VER)'

	@echo 'BRANCH            $(BRANCH)'
	@echo 'COMMIT_CNT        $(COMMIT_CNT)'
	@echo 'COMPILE_LDFLAGS   $(COMPILE_LDFLAGS)'

	@echo 'PATH              $(PATH)'
	@echo '---------------------------------------------------------'
	@echo ''


####################################################################################################################
##
## help for each task - https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
##
####################################################################################################################
.PHONY: help

help: ## This help.
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help



####################################################################################################################
##
## Build of binaries
##
####################################################################################################################
all: dumpr test reportcard ## build example and run tests

binaries: dumpr ## build binaries in bin dir

create_dir:
	@mkdir -p $(BIN_DIR)

check_prereq: create_dir

build_app: create_dir
	export CGO_ENABLED=0
	go build -o $(BIN_DIR)/$(BIN_NAME) -a -ldflags '$(COMPILE_LDFLAGS)' $(APP_PATH)


dumpr: ## build_info ## build example binary in bin dir
	@echo "build dumpr server"
	make BIN_NAME=dumpr APP_PATH=$(PROJ_PATH) build_app
	@echo ''
	@echo ''



####################################################################################################################
##
## Cleanup of binaries
##
####################################################################################################################

clean_binary: ## clean binary in bin dir
	rm -f $(BIN_DIR)/$(BIN_NAME)

clean_dumpr: ## clean dumpr
	make BIN_NAME=dumpr clean_binary

clean: clean_dumpr ## clean all
	rm -rf $(DIST_DIR)

test: ## run tests
	go test -v $(PROJ_PATH)

fmt: ## run fmt on project
	#go fmt $(PROJ_PATH)/...
	gofmt -s -d -w -l .

doc: ## launch godoc on port 6060
	godoc -http=:6060

deps: ## display deps for project
	go list -f '{{ join .Deps  "\n"}}' . |grep "/" | grep -v $(PROJ_PATH)| grep "\." | sort |uniq

lint: ## run lint on the project
	golint ./...

staticcheck: ## run staticcheck on the project
	staticcheck -ignore "$(shell cat .checkignore)" .

vet: ## run go vet on the project
	go vet .

reportcard: ## run goreportcard-cli
	goreportcard-cli -v

release: clean # create a release
	goreleaser release --rm-dist

release-snapshot: # create release snapshot
	goreleaser release --snapshot --skip-publish --rm-dist

release-debug: # create release snapshot
	goreleaser release --snapshot --skip-publish --rm-dist --debug


tools: ## install dependent tools for code analysis
	go install github.com/gordonklaus/ineffassign@latest
	go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
	go install golang.org/x/lint/golint@latest
	go install github.com/gojp/goreportcard/cmd/goreportcard-cli@latest
	go install github.com/goreleaser/goreleaser@latest


