CONTROLLER_GEN_VERSION ?= v0.17.0
ENVTEST_VERSION ?= latest

GOBIN ?= $(shell go env GOPATH)/bin
CONTROLLER_GEN ?= $(GOBIN)/controller-gen
ENVTEST ?= $(GOBIN)/setup-envtest

.PHONY: all
all: generate manifests build

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: generate
generate:
	rm -f api/v1alpha1/zz_generated.deepcopy.go
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./api/..."

.PHONY: manifests
manifests:
	$(CONTROLLER_GEN) crd \
		rbac:roleName=irrigator-role \
		paths="./..." \
		output:crd:artifacts:config=config/crd/bases

.PHONY: build
build:
	CGO_ENABLED=0 go build -o bin/irrigator ./cmd/controller/

.PHONY: test
test:
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...

.PHONY: lint
lint:
	gofmt -s -l . && go vet ./...

.PHONY: setup-envtest
setup-envtest:
	$(ENVTEST) use --bin-dir ./bin/envtest

.PHONY: install-tools
install-tools:
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)
	go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(ENVTEST_VERSION)

.PHONY: docker-build
docker-build:
	docker build -t ghcr.io/hauke-cloud/irrigator:latest .
