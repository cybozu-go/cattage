# Overview

neco-tenant-controller is a Kubernetes controller that enhances the multi-tenancy of [Argo CD][] with [Accurate][].

## Features

### For Administrators

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
    When the parent of a sub-namespace is changed, neco-tenant-controller will automatically update the permissions.

### For Tenant Users

- Sync Argo CD Application resources

    Tenant users can create Application resources in their sub-namespaces without `argocd` command.
    neco-tenant-controller will synchronize Application resource between the tenant namespace and argocd namespace.
    It allows for [App Of Apps Pattern][] in multi-tenancy environments.

- Validate Argo CD Application resources

    Tenant users can only specify their own AppProject when creating an application resource.

[Accurate]: https://github.com/cybozu-go/accurate
[Argo CD]: https://argo-cd.readthedocs.io/en/stable/
[App Of Apps Pattern]: https://argo-cd.readthedocs.io/en/stable/operator-manual/cluster-bootstrapping/#app-of-apps-pattern
