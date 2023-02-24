# Change Log

All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/).

## [Unreleased]

### **Breaking Changes**

- Support Argo CD's [Applications in any namespace](https://argo-cd.readthedocs.io/en/stable/operator-manual/app-any-namespace/)
  - Need Argo CD 2.5 or higher required.
  - Synced Applications in argocd namespace will be removed.
  - You have to add `--application-namespaces="*"` parameter to `argocd-application-controller` and `argocd-server`.
  - You have to add `sourceNamespaces` field in `appProjectTemplate`.

## [0.1.4] - 2022-12-15

### Changed

- Upgrade Argo CD to v2.4 ([#23](https://github.com/cybozu-go/cattage/pull/23))

## [0.1.3] - 2022-11-16

### Changed

- Support Kubernetes 1.24 ([#19](https://github.com/cybozu-go/cattage/pull/19))
    - Build with go 1.19
    - Update dependencies

## [0.1.2] - 2022-04-06

### Fixed
- Sync application resource, when an application resource on argocd namespace is deleted. ([#15](https://github.com/cybozu-go/cattage/pull/15))

## [0.1.1] - 2022-03-10

### Fixed
- Fix an application created with argocd cli is given an unnecessary finalizer ([#11](https://github.com/cybozu-go/cattage/pull/11))

## [0.1.0] - 2022-02-10

This is the first public release.

[Unreleased]: https://github.com/cybozu-go/cattage/compare/v0.1.4...HEAD
[0.1.4]: https://github.com/cybozu-go/cattage/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/cybozu-go/cattage/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/cybozu-go/cattage/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/cybozu-go/cattage/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/cybozu-go/cattage/compare/60bcea7b1cf9d79e5e439d0fa7dbb4629c9f1125...v0.1.0
