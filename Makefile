# Tool versions
CTRL_RUNTIME_VERSION := $(shell awk '/sigs.k8s.io\/controller-runtime/ {print substr($$2, 2)}' go.mod)
ARGOCD_VERSION = 2.13.2

# Test tools
BIN_DIR := $(shell pwd)/bin
STATICCHECK := $(BIN_DIR)/staticcheck
NILERR := $(BIN_DIR)/nilerr
SUDO = sudo

# Set the shell used to bash for better error handling.
SHELL = /bin/bash
.SHELLFLAGS = -e -o pipefail -c

CRD_OPTIONS = "crd:crdVersions=v1"

# for Go
GOOS = $(shell go env GOOS)
GOARCH = $(shell go env GOARCH)
SUFFIX =

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	controller-gen $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	kustomize build config/helm/crds | yq e "." - > charts/cattage/crds/tenant.yaml
	kustomize build config/helm/templates | yq e "." - > charts/cattage/templates/generated.yaml


.PHONY: generate
generate: ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: install
install: manifests ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	kustomize build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	kustomize build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: apidoc
apidoc: $(wildcard api/*/*_types.go)
	crd-to-markdown --links docs/links.csv -f api/v1beta1/tenant_types.go -n Tenant > docs/crd_tenant.md

.PHONY: book
book:
	rm -rf docs/book
	cd docs; mdbook build

.PHONY: check-generate
check-generate:
	$(MAKE) manifests generate apidoc
	git diff --exit-code --name-only

.PHONY: crds
crds:
	mkdir -p test/crd/
	curl -fsL -o test/crd/application.yaml https://raw.githubusercontent.com/argoproj/argo-cd/v$(ARGOCD_VERSION)/manifests/crds/application-crd.yaml
	curl -fsL -o test/crd/appproject.yaml https://raw.githubusercontent.com/argoproj/argo-cd/v$(ARGOCD_VERSION)/manifests/crds/appproject-crd.yaml

.PHONY: envtest
envtest: setup-envtest crds
	source <($(SETUP_ENVTEST) use -p env); \
		go test -v -count 1 -race ./internal/controller -ginkgo.v -ginkgo.fail-fast
	source <($(SETUP_ENVTEST) use -p env); \
		go test -v -count 1 -race ./internal/hooks -ginkgo.v -ginkgo.fail-fast

.PHONY: test
test: test-tools
	go test -v -count 1 -race ./internal/config
	go install ./...
	go vet ./...
	test -z $$(gofmt -s -l . | tee /dev/stderr)
	$(STATICCHECK) ./...

.PHONY: container-structure-test
container-structure-test:
	container-structure-test test --image ghcr.io/cybozu-go/cattage:$(shell git describe --tags --abbrev=0 --match "v*" || echo v0.0.0)-next-amd64 --config cst.yaml

##@ Build

.PHONY: build
build: generate
	mkdir -p bin
	GOBIN=$(shell pwd)/bin CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go install ./cmd/...

##@ Development

.PHONY: dev
dev:
	ctlptl apply -f ./cluster.yaml
	$(MAKE) -C ./e2e/ prepare

.PHONY: stop-dev
stop-dev:
	ctlptl delete -f ./cluster.yaml

.PHONY: port-forward-argocd
port-forward-argocd:
	mkdir -p ./tmp/
	kubectl port-forward -n argocd service/argocd-server 8080:80 > /dev/null 2>&1 & jobs -p > ./tmp/port-forward-argocd.pid

.PHONY: stop-port-forward-argocd
stop-port-forward-argocd:
	echo "kill `cat ./tmp/port-forward-argocd.pid`" && kill `cat ./tmp/port-forward-argocd.pid`
	rm ./tmp/port-forward-argocd.pid

.PHONY: argocd-password
argocd-password: ## Show admin password for ArgoCD
	@kubectl get secret -n argocd argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d

.PHONY: login-argocd
login-argocd:
	argocd login localhost:8080 --insecure --username admin --password $$(kubectl get secret -n argocd argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d)

##@ Tools

SETUP_ENVTEST := $(BIN_DIR)/setup-envtest
.PHONY: setup-envtest
setup-envtest: ## Download setup-envtest locally if necessary
	# see https://github.com/kubernetes-sigs/controller-runtime/tree/master/tools/setup-envtest
	GOBIN=$(BIN_DIR) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: test-tools
test-tools: $(STATICCHECK)

$(STATICCHECK):
	mkdir -p $(BIN_DIR)
	GOBIN=$(BIN_DIR) go install honnef.co/go/tools/cmd/staticcheck@latest
