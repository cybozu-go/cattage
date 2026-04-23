KUBERNETES_VERSION = v1.34.3@sha256:08497ee19eace7b4b5348db5c6a1591d7752b164530a36f855cb0f2bdcbadd48 # renovate: kindest/node

CERT_MANAGER_VERSION := v1.20.2
CERT_MANAGER_SHA256 := 1ce11cae912adecc69e6bb623435fafc9ed21505f9efff98bd71d7b80f01db1f
CERT_MANAGER_URL := https://github.com/cert-manager/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml

RELOADER_CHART_VERSION := 2.2.11
RELOADER_CHART_SHA256 := 3c587e013f37369a67e2c2e6e1f2f7a6efe00cc3d38063eeb567c426529ddc92

TMP_DIR := $(abspath $(CURDIR)/tmp)

KIND_CONFIG = kind-config.yaml
export KUBECONFIG

.PHONY: help
help:
	@echo "Choose one of the following target"
	@echo
	@echo "start      Start kind cluster and install accurate"
	@echo "test       Run e2e tests"
	@echo "logs       Save logs as logs.tar.gz"
	@echo "stop       Stop the kind cluster"

.PHONY: start
start:
	kind create cluster --name=cattage --config=$(KIND_CONFIG) --image=kindest/node:$(KUBERNETES_VERSION) --wait 1m
	cd ..; CGO_ENABLED=0 go build -o cattage-controller ./cmd/cattage-controller/main.go
	cd ..; docker build --no-cache -t cattage:dev .
	kind load docker-image cattage:dev --name=cattage
	$(MAKE) prepare
	kubectl apply -k ../config/default
	kubectl -n cattage wait --for=condition=available --timeout=180s --all deployments

.PHONY: setup-cert-manager
setup-cert-manager:
	mkdir -p $(TMP_DIR)
	curl -sSLf -o $(TMP_DIR)/cert-manager.yaml $(CERT_MANAGER_URL)
	echo "$(CERT_MANAGER_SHA256)  $(TMP_DIR)/cert-manager.yaml" | sha256sum -c -
	kubectl apply -f $(TMP_DIR)/cert-manager.yaml
	kubectl -n cert-manager wait --for=condition=available --timeout=180s --all deployments

.PHONY: setup-reloader
setup-reloader:
	mkdir -p $(TMP_DIR)
	helm repo add stakater https://stakater.github.io/stakater-charts
	helm repo update
	helm pull stakater/reloader --version $(RELOADER_CHART_VERSION) --destination $(TMP_DIR)
	echo "$(RELOADER_CHART_SHA256)  $(TMP_DIR)/reloader-$(RELOADER_CHART_VERSION).tgz" | sha256sum -c -
	helm upgrade --install --create-namespace --namespace reloader reloader \
		-f manifests/reloader-values.yaml \
		$(TMP_DIR)/reloader-$(RELOADER_CHART_VERSION).tgz
	kubectl -n reloader wait --for=condition=available --timeout=180s --all deployments

.PHONY: prepare
prepare: setup-cert-manager setup-reloader
	# Setup Argo CD
	kubectl create namespace argocd
	kustomize build --enable-helm ./manifests | kubectl apply -f -
	kubectl -n argocd wait --for=condition=available --timeout=180s --all deployments

	# Setup accurate
	helm repo add accurate https://cybozu-go.github.io/accurate
	helm repo update
	helm install --create-namespace --namespace accurate accurate -f manifests/accurate-values.yaml accurate/accurate
	kubectl -n accurate wait --for=condition=available --timeout=180s --all deployments

.PHONY: generate-second-controller
generate-second-controller:
	kustomize build --enable-helm ./manifests | yq ea '. as $$i ireduce ([]; . + $$i) | .[] | select(.kind=="StatefulSet") | .metadata.name="second-application-controller"' > ./manifests/second-application-controller.yaml

.PHONY: test
test:
	env RUN_E2E=1 \
		go test -v -race . -ginkgo.v -ginkgo.fail-fast

.PHONY: logs
logs:
	rm -rf logs.tar.gz logs
	kind export logs --name=cattage ./logs
	tar czf logs.tar.gz logs
	rm -rf logs

.PHONY: stop
stop:
	kind delete cluster --name=cattage
	-docker image rm cattage:dev
	-docker image prune -f

.PHONY: clean
clean:
	rm -rf $(TMP_DIR)
