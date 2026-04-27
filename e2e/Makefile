KUBERNETES_VERSION = v1.34.3@sha256:08497ee19eace7b4b5348db5c6a1591d7752b164530a36f855cb0f2bdcbadd48 # renovate: kindest/node

CERT_MANAGER_VERSION := v1.20.2
CERT_MANAGER_SHA256 := 1ce11cae912adecc69e6bb623435fafc9ed21505f9efff98bd71d7b80f01db1f
CERT_MANAGER_URL := https://github.com/cert-manager/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml

RELOADER_CHART_VERSION := 2.2.11
RELOADER_CHART_SHA256 := 3c587e013f37369a67e2c2e6e1f2f7a6efe00cc3d38063eeb567c426529ddc92

ACCURATE_CHART_VERSION := 0.7.12
ACCURATE_CHART_SHA256 := 7004356f40d8dcbc80709c199d6fed285d525e1a32dc56f76c6fd654d4bc5527

TMP_DIR := $(abspath $(CURDIR)/tmp)

KIND_CONFIG = kind-config.yaml
export KUBECONFIG

$(TMP_DIR):
	mkdir -p $(TMP_DIR)

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

$(TMP_DIR)/cert-manager-$(CERT_MANAGER_VERSION).yaml: | $(TMP_DIR)
	curl -sSLf -o $@ $(CERT_MANAGER_URL)

$(TMP_DIR)/cert-manager.yaml: $(TMP_DIR)/cert-manager-$(CERT_MANAGER_VERSION).yaml
	echo "$(CERT_MANAGER_SHA256)  $<" | sha256sum -c -
	ln -sf "$<" "$@"

.PHONY: setup-cert-manager
setup-cert-manager: $(TMP_DIR)/cert-manager.yaml
	kubectl apply -f "$<"
	kubectl -n cert-manager wait --for=condition=available --timeout=180s --all deployments

$(TMP_DIR)/reloader-$(RELOADER_CHART_VERSION).tgz: | $(TMP_DIR)
	helm pull reloader \
		--repo https://stakater.github.io/stakater-charts \
		--version $(RELOADER_CHART_VERSION) \
		--destination $(TMP_DIR)

$(TMP_DIR)/reloader.tgz: $(TMP_DIR)/reloader-$(RELOADER_CHART_VERSION).tgz
	echo "$(RELOADER_CHART_SHA256)  $<" | sha256sum -c -
	ln -sf "$<" "$@"

.PHONY: setup-reloader
setup-reloader: $(TMP_DIR)/reloader.tgz
	helm upgrade --install --create-namespace --namespace reloader reloader \
		-f manifests/reloader-values.yaml "$<"
	kubectl -n reloader wait --for=condition=available --timeout=180s --all deployments

$(TMP_DIR)/accurate-$(ACCURATE_CHART_VERSION).tgz: | $(TMP_DIR)
	helm pull accurate \
		--repo https://cybozu-go.github.io/accurate \
		--version $(ACCURATE_CHART_VERSION) \
		--destination $(TMP_DIR)

$(TMP_DIR)/accurate.tgz: $(TMP_DIR)/accurate-$(ACCURATE_CHART_VERSION).tgz
	echo "$(ACCURATE_CHART_SHA256)  $<" | sha256sum -c -
	ln -sf "$<" "$@"

.PHONY: setup-accurate
setup-accurate: $(TMP_DIR)/accurate.tgz
	helm upgrade --install --create-namespace --namespace accurate accurate \
		-f manifests/accurate-values.yaml "$<"
	kubectl -n accurate wait --for=condition=available --timeout=180s --all deployments

.PHONY: prepare
prepare: setup-cert-manager setup-reloader setup-accurate
	# Setup Argo CD
	kubectl get ns argocd || kubectl create namespace argocd
	kustomize build --enable-helm ./manifests | kubectl apply -f -
	kubectl -n argocd wait --for=condition=available --timeout=180s --all deployments

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
