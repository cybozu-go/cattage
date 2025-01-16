# Overview

Cattage is a Kubernetes controller that enhances the multi-tenancy of [Argo CD][] with [Accurate][].

## Features

- Management of root-namespaces for tenants

    When an administrator creates a [tenant resource](crd_tenant.md), root-namespaces for the tenant will be created.
    A RoleBinding resource will be created in the namespace so that the namespace can only be accessed by the tenant's users.
    Tenant users can create sub-namespaces in those root-namespaces.

- Automatic update of Argo CD AppProject resources

    An AppProject resource can control namespaces where users can deploy manifests.
    When a tenant user creates a sub-namespace, the AppProject will be automatically updated accordingly.
    Tenant users will be able to deploy applications with Argo CD to the namespaces.

- The ownership of sub-namespaces can be changed between tenants

    Sometimes users may want to move the ownership of an application to another tenant.
    When the parent of a sub-namespace is changed, Cattage will automatically update the permissions.

- Sharding application-controller instances

    Cattage can shard application-controller instances by the tenant.
    This feature is useful when you have a large number of tenants and want to avoid a single application-controller instance from being overloaded.

[Accurate]: https://github.com/cybozu-go/accurate
[Argo CD]: https://argo-cd.readthedocs.io/en/stable/
