# Maintenance

## How to update supported Kubernetes

Cattage supports the three latest Kubernetes versions.
If a new Kubernetes is released, please update the following files.

- Update Kubernetes version in `e2e/Makefile`, `.github/workflows/ci.yaml` and `cluster.yaml`.
- Update kubectl version in `aqua.yaml`.
- Update `k8s.io/*` and `sigs.k8s.io/controller-runtime` packages version in `go.mod`.

If Kubernetes or controller-runtime API has changed, please fix the relevant source code.

## How to update supported Argo CD

Cattage supports one Argo CD version.
If a new Argo CD is released, please update the following files.

- Update Argo CD Version in `aqua.yaml` and `Makefile`.
- Run `make crds`.

If Argo CD API has changed, please fix the relevant source code.

## How to update dependencies

Renovate will create PRs that update dependencies once a week.
However, Argo CD is not subject to Renovate. Also, Kubernetes is only updated with patched versions.
