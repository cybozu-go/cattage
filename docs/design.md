# Design notes

## Overview

`neco-tenant-controller` is a custom controller that uses [Accurate][] and [Argo CD][] to provide multi-tenant environment for Kubernetes cluster.

## Motivation

We have developed the following mechanism for multi-tenant [Argo CD][].

https://blog.kintone.io/entry/production-grade-delivery-workflow-using-argocd#Multi-tenancy

However, the above mechanism has the following problems:

- Tenant users cannot create app-of-apps Application resources. They need to ask an administrator for that.
- Application resources are not strictly validated. Tenant users can specify Project for other tenants, and can also specify duplicate names.
- When a SubNamespace is created in [HNC][] or [Accurate][], an administrator needs to add it to the destinations of Application resources.
  (Argo CD supports specifying wildcards in destinations, but that is not enough for us.)

## Goals

- Develop a custom controller to automate the configuration for multi-tenancy.
- Automates the creation of root-namespaces and AppProject for administrators.
- Allow tenant users to create Application resources in any namespace.
- Perform strict validation of Application resources for security.

## User stories

### Adding a team

Administrators only need to create one custom resource to add a team to a Kubernetes cluster.
No more manual operations to add Namespaces and Applications.

### Adding an app-of-apps Application

### Changing ownership


## Upgrade Strategy


## Alternatives

### Argo CD: Multi-tenancy improvements

Argo CD will improve multi-tenancy in the future.

https://argo-cd.readthedocs.io/en/stable/roadmap/#multi-tenancy-improvements

The above proposal would allow us to create Application resources in any namespace.

If this feature is supported, we will migrate to it immediately.
So we need to design our controller to be easy to migrate to.

### ApplicationSet

ApplicationSet is one of the features of Argo CD which generates Application resources based on user input.

https://argo-cd.readthedocs.io/en/stable/user-guide/application-set/

However, this feature does not give tenant users enough flexibility in their settings.

### AppSource Controller

AppSource controller is similar to our proposal.

https://github.com/argoproj-labs/appsource

But AppSource is still not production-ready.
Also, it does not solve our some problems.

### Multiple Argo CD instance (argocd-operator)

We considered having an Argo CD instance for each tenant team, but it turned out to be a permissions problem.

### Other Continuous Delivery tools

Other Continuous Delivery tools support multi-tenancy.

- https://github.com/fluxcd/flux2
- https://github.com/pipe-cd/pipe

However, we love Argo CD (the many features and the useful UI).
We already have a lot of manifests managed by Argo CD. It's hard to switch to another tool now.

[Argo CD]: https://argo-cd.readthedocs.io/
[HNC]: https://github.com/kubernetes-sigs/hierarchical-namespaces
[Accurate]: https://cybozu-go.github.io/accurate/
