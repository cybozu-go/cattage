KIND_VERSION = 0.11.1
KUBERNETES_VERSION = 1.22.2
ARGOCD_VERSION = 2.1.8
HELM_VERSION = 3.7.1

BIN_DIR := $(dir $(shell pwd))bin
KIND := $(BIN_DIR)/kind
KUBECTL := $(BIN_DIR)/kubectl
HELM := $(BIN_DIR)/helm
ARGOCD = $(BIN_DIR)/argocd
KIND_CONFIG = kind-config.yaml
export KUBECTL KUBECONFIG

.PHONY: help
help:
	@echo "Choose one of the following target"
	@echo
	@echo "start      Start kind cluster and install accurate"
	@echo "test       Run e2e tests"
	@echo "logs       Save logs as logs.tar.gz"
	@echo "stop       Stop the kind cluster"

.PHONY: start
start: $(KIND) $(KUBECTL)
	$(KIND) create cluster --name=neco-tenant-controller --config=$(KIND_CONFIG) --image=kindest/node:v$(KUBERNETES_VERSION) --wait 1m
	cd ..; docker build --no-cache -t neco-tenant-controller:dev .
	$(KIND) load docker-image neco-tenant-controller:dev --name=neco-tenant-controller
	$(MAKE) prepare
	$(KUBECTL) apply -k ../config/default
	$(KUBECTL) -n neco-tenant-controller-system wait --for=condition=available --timeout=180s --all deployments

.PHONY: prepare
prepare: $(KUBECTL) $(HELM)
	$(KUBECTL) apply -f https://github.com/jetstack/cert-manager/releases/latest/download/cert-manager.yaml
	$(KUBECTL) -n cert-manager wait --for=condition=available --timeout=180s --all deployments
	$(KUBECTL) create ns argocd
	curl -sSLf https://raw.githubusercontent.com/argoproj/argo-cd/v$(ARGOCD_VERSION)/manifests/install.yaml | $(KUBECTL) -n argocd apply -f -
	$(KUBECTL) -n argocd wait --for=condition=available --timeout=180s --all deployments
	$(HELM) repo add accurate https://cybozu-go.github.io/accurate/
	$(HELM) repo update
	$(HELM) install --create-namespace --namespace accurate accurate -f accurate-values.yaml accurate/accurate
	$(KUBECTL) -n accurate wait --for=condition=available --timeout=180s --all deployments

.PHONY: test
test:
	env PATH=$$(pwd)/../bin:$$PATH RUN_E2E=1 \
		go test -v -race . -ginkgo.progress -ginkgo.v -ginkgo.failFast

.PHONY: logs
logs:
	rm -rf logs.tar.gz logs
	$(KIND) export logs --name=neco-tenant-controller ./logs
	tar czf logs.tar.gz logs
	rm -rf logs

.PHONY: stop
stop: $(KIND)
	$(KIND) delete cluster --name=neco-tenant-controller
	-docker image rm neco-tenant-controller:dev
	-docker image prune -f

.PHONY: kind
kind: $(KIND)
$(KIND):
	mkdir -p $(BIN_DIR)
	curl -sfL -o $@ https://github.com/kubernetes-sigs/kind/releases/download/v$(KIND_VERSION)/kind-linux-amd64
	chmod a+x $@

$(KUBECTL):
	mkdir -p $(BIN_DIR)
	curl -sfL -o $@ https://dl.k8s.io/release/v$(KUBERNETES_VERSION)/bin/linux/amd64/kubectl
	chmod a+x $@

$(HELM):
	$(MAKE) -C .. helm

.PHONY: argocd
argocd: $(ARGOCD)
$(ARGOCD):
	mkdir -p $(BIN_DIR)
	curl -sSLf https://github.com/argoproj/argo-cd/releases/download/v$(ARGOCD_VERSION)/argocd-linux-amd64 -o $(ARGOCD)
	chmod 755 $(ARGOCD)
