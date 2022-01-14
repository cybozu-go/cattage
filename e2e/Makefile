KUBERNETES_VERSION = 1.23.1
ARGOCD_VERSION = 2.2.2

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
	kind create cluster --name=cattage --config=$(KIND_CONFIG) --image=kindest/node:v$(KUBERNETES_VERSION) --wait 1m
	cd ..; CGO_ENABLED=0 go build -o cattage-controller ./cmd/cattage-controller/main.go
	cd ..; docker build --no-cache -t cattage:dev .
	kind load docker-image cattage:dev --name=cattage
	$(MAKE) prepare
	kubectl apply -k ../config/default
	kubectl -n cattage wait --for=condition=available --timeout=180s --all deployments

.PHONY: prepare
prepare:
	kubectl apply -f https://github.com/jetstack/cert-manager/releases/latest/download/cert-manager.yaml
	kubectl -n cert-manager wait --for=condition=available --timeout=180s --all deployments
	kubectl create ns argocd
	curl -sSLf https://raw.githubusercontent.com/argoproj/argo-cd/v$(ARGOCD_VERSION)/manifests/install.yaml | kubectl -n argocd apply -f -
	kubectl -n argocd wait --for=condition=available --timeout=180s --all deployments
	helm repo add accurate https://cybozu-go.github.io/accurate/
	helm repo update
	helm install --create-namespace --namespace accurate accurate -f accurate-values.yaml accurate/accurate
	kubectl -n accurate wait --for=condition=available --timeout=180s --all deployments

.PHONY: test
test:
	env RUN_E2E=1 \
		go test -v -race . -ginkgo.progress -ginkgo.v -ginkgo.failFast

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
