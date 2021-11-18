KIND_VERSION = 0.11.1
KUBERNETES_VERSION = 1.22.2

KIND := $(dir $(shell pwd))/bin/kind
KUBECTL := $(dir $(shell pwd))/bin/kubectl
KUBECONFIG := $(shell pwd)/.kubeconfig
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
	$(KUBECTL) apply -f https://github.com/jetstack/cert-manager/releases/latest/download/cert-manager.yaml
	$(KUBECTL) -n cert-manager wait --for=condition=available --timeout=180s --all deployments
	$(KUBECTL) apply -k ../config/default
	$(KUBECTL) -n tenant-system wait --for=condition=available --timeout=180s --all deployments

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

$(KIND):
	mkdir -p ../bin
	curl -sfL -o $@ https://github.com/kubernetes-sigs/kind/releases/download/v$(KIND_VERSION)/kind-linux-amd64
	chmod a+x $@

$(KUBECTL):
	mkdir -p ../bin
	curl -sfL -o $@ https://dl.k8s.io/release/v$(KUBERNETES_VERSION)/bin/linux/amd64/kubectl
	chmod a+x $@
