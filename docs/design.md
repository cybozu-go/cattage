# Design notes

## Overview

Cattage is a Kubernetes controller that enhances the multi-tenancy of [Argo CD][] with [Accurate][].

## Motivation

There is a known limitation for Argo CD to implement app-of-apps pattern in a multi-tenancy environment.

https://github.com/argoproj/argo-cd/issues/2785

We have developed the following mechanism to resolve the problem.

https://blog.kintone.io/entry/production-grade-delivery-workflow-using-argocd#Multi-tenancy

However, the mechanism still has the following problems:

- When a SubNamespace is created in [HNC][] or [Accurate][], an administrator needs to add it to the destinations of the Application resource.
  (Argo CD supports specifying wildcards in destinations, but that is not enough for us.)

We need to build a better solution.

## Goals

- Develop a Kubernetes custom controller to automate the configuration for multi-tenancy of Argo CD with Accurate.
- Automates the creation of root-namespaces and AppProject for each tenant.
- Allow tenant users to create Application resources in any namespace.
- Perform strict validation of Application resources for security.

## User stories

### Adding a tenant

An administrator only need to create one custom resource to add a tenant to a Kubernetes cluster.
No more manual operations to add Applications, AppProjects and Namespaces.

Tenant users can create sub-namespaces within their tenant.

### Adding an app-of-apps Application

Tenant users can create Application resources within their sub-namespaces.
Application resources are strictly validated.
No more deploying to another tenant's namespace by mistake.

### Changing ownership

There are cases where you want to move ownership of an application between tenants.
Accurate supports `kubectl accurate sub move` command to change the parent of a sub-namespace.

https://cybozu-go.github.io/accurate/subnamespaces.html#changing-the-parent-of-a-sub-namespace

An administrators can use this command to move the sub-namespace to another tenant.
The permission of AppProjects, Applications and Namespaces will be updated automatically.

## Alternatives

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
