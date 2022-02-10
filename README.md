[![GitHub release](https://img.shields.io/github/release/cybozu-go/cattage.svg?maxAge=60)](https://github.com/cybozu-go/cattage/releases)
[![CI](https://github.com/cybozu-go/cattage/actions/workflows/ci.yaml/badge.svg)](https://github.com/cybozu-go/cattage/actions/workflows/ci.yaml)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/cybozu-go/cattage?tab=overview)](https://pkg.go.dev/github.com/cybozu-go/cattage?tab=overview)
[![Go Report Card](https://goreportcard.com/badge/github.com/cybozu-go/cattage)](https://goreportcard.com/report/github.com/cybozu-go/cattage)

# Cattage

Cattage is a Kubernetes controller that enhances the multi-tenancy of [Argo CD][] with [Accurate][].

**Project Status**: Initial development

## Features

- Management of root-namespaces for tenants. Tenant users will be able to create sub-namespaces in those root-namespaces.
- When a tenant user creates a sub-namespace, the AppProject will be automatically updated accordingly. Tenant users will be able to deploy applications with Argo CD to the namespaces.
- The ownership of sub-namespaces can be changed between tenants.
- Tenant users can create Application resources in their sub-namespaces without `argocd` command. It allows for [App Of Apps Pattern](https://argo-cd.readthedocs.io/en/stable/operator-manual/cluster-bootstrapping/#app-of-apps-pattern) in multi-tenancy environments.

## Supported Version

Cattage depends on the specification of Argo CD and Accurate.

Cattage supports the following versions:

| Cattage | Argo CD | Accurate |
|---------|---------|----------|
| 0.1.0   | 2.2.x   | 0.3.x    | 

## Documentation

[docs](docs/) directory contains documents about designs and specifications.

[Accurate]: https://github.com/cybozu-go/accurate
[Argo CD]: https://argo-cd.readthedocs.io/en/stable/
