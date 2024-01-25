LDFLAGS = $(if $(DEBUGGER),,-s -w) $(shell ./hack/version.sh)

GOVER_MAJOR := $(shell go version | sed -E -e "s/.*go([0-9]+)[.]([0-9]+).*/\1/")
GOVER_MINOR := $(shell go version | sed -E -e "s/.*go([0-9]+)[.]([0-9]+).*/\2/")
GO111 := $(shell [ $(GOVER_MAJOR) -gt 1 ] || [ $(GOVER_MAJOR) -eq 1 ] && [ $(GOVER_MINOR) -ge 11 ]; echo $$?)
ifeq ($(GO111), 1)
$(error Please upgrade your Go compiler to 1.11 or higher version)
endif

# Enable GO111MODULE=on explicitly, disable it with GO111MODULE=off when necessary.
export GO111MODULE := on
GOOS := $(if $(GOOS),$(GOOS),linux)
GOARCH := $(if $(GOARCH),$(GOARCH),amd64)
CGO_ENABLED := $(if $(CGO_ENABLED),$(CGO_ENABLED),0)
GOENV  := CGO_ENABLED=$(CGO_ENABLED) GO15VENDOREXPERIMENT="1" GOOS=$(GOOS) GOARCH=$(GOARCH)
GO     := $(GOENV) go build
GOTEST := CGO_ENABLED=0 go test -v -cover

PACKAGE_LIST := $$(go list ./... | grep -vE "cmd|deprecated|generated")
PACKAGE_DIRECTORIES := $(PACKAGE_LIST) | sed 's|lite.io/liteio/||'
COVERPKG := $(shell echo $(PACKAGE_LIST) | tr " " ",")
FILES := $$(find $$($(PACKAGE_DIRECTORIES)) -name "*.go")
FAIL_ON_STDOUT := awk '{ print } END { if (NR > 0) { exit 1 } }'

IMAGE_NAME := reg.docker.alibaba-inc.com/dbplatform/node-disk-controller:test

CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
CLIENT_GEN = $(shell pwd)/bin/client-gen
LISTER_GEN = $(shell pwd)/bin/lister-gen
INFORMER_GEN = $(shell pwd)/bin/informer-gen
DEEPCOPY_GEN = $(shell pwd)/bin/deepcopy-gen
#CRD_OPTIONS ?= "crd:trivialVersions=true,preserveUnknownFields=false" # controller-gen@v0.4.1 has these options
CRD_OPTIONS ?= "crd:ignoreUnexportedFields=true"

.PHONY: dep
dep:
	go mod download
	go mod tidy

.PHONY: test
test:
	@echo "Run unit tests"
	@go test -v -cover -coverprofile=coverage.txt -covermode=atomic -coverpkg=$(COVERPKG) $(PACKAGE_LIST) && echo "\nUnit tests run successfully!"
	#go test -v -cover -coverprofile=coverage.txt -covermode=atomic $(PACKAGE_LIST) && echo "\nUnit tests run successfully!"
	@go tool cover -func=coverage.txt -o totalCoverage.md

.PHONY: gen-mock
gen-mock:
	@echo "generating mock files"
	mockery --dir pkg/util/osutil --name ShellExec --output pkg/generated/mocks/util --outpkg utilmock
	mockery --dir pkg/util/lvm --all --output pkg/generated/mocks/lvm --outpkg lvmmock
	mockery --dir pkg/spdk/jsonrpc/client --name SPDKClientIface --output pkg/generated/mocks/spdk --outpkg spdkmock

.PHONY: build
build: controller

.PHONY: controller
controller:
	$(GO) -ldflags '$(LDFLAGS)' -o _build/node-disk-controller cmd/controller/main.go

.PHONY: scheduler
scheduler:
	$(GO) -ldflags '$(LDFLAGS)' -o _build/scheduler-plugin cmd/scheduler/main.go

.PHONY: clean
clean:
	@rm coverage.txt totalCoverage.md
	@rm -rf _build

.PHONY: build-image
build-image:
	cp -a _build hack/docker
	cd hack/docker; docker build -t $(IMAGE_NAME) .
	rm -rf hack/docker/_build

.PHONY: controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.12.1)
	$(call go-get-tool,$(CLIENT_GEN),k8s.io/code-generator/cmd/client-gen@v0.28.0)
	$(call go-get-tool,$(LISTER_GEN),k8s.io/code-generator/cmd/lister-gen@v0.28.0)
	$(call go-get-tool,$(DEEPCOPY_GEN),k8s.io/code-generator/cmd/deepcopy-gen@v0.28.0)
	$(call go-get-tool,$(INFORMER_GEN),k8s.io/code-generator/cmd/informer-gen@v0.28.0)

## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./pkg/api/..."

generate-client:
	cd hack && ./update-codegen.sh

## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
manifests: controller-gen
	$(CONTROLLER_GEN) webhook $(CRD_OPTIONS) rbac:roleName=manager-role paths="./pkg/api/..." output:crd:artifacts:config=manifests/crd/bases output:webhook:artifacts:config=manifests/webhook

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go install $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef


#-----------------------------------------------------------------------------
# Target: docker.buildx.agent
#-----------------------------------------------------------------------------
include Makefile.buildx.mk
