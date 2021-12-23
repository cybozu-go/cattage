# Overview

neco-tenant-controller is a Kubernetes controller to manage tenant team resources.

## Features

### For Administrator

Cluster administrator can creates a [Tenant custom resource](./crd_tenant.md) for each tenant team.
neco-tenant-controller will automatically apply the following resources needed by the tenant team based on the Tenant resource.

- Manage root namespaces

    Tenant users can use [Accurate][] to create a SubNamespace.
    Because of that, users need a root namespace.
    neco-tenant-controller creates multiple root namespaces for each tenant team.
    Those namespaces are permission-controlled so that only the tenant users can access them.

- Manage Argo CD AppProject resources

    Tenant users need an AppProject resource to deploy their manifests with [Argo CD][Argo CD].
    The AppProject can control namespaces where tenant users can deploy manifests.
    neco-tenant-controller dynamically rewrites the AppProject resource each time tenant users creates a SubNamespace.


### For Tenant Users

- Manage Argo CD Application resources

    Tenant users need Application resources to deploy their manifests with Argo CD.
    neco-tenant-controller creates Application resources for each tenant team.

- Validate Argo CD Application resources

    Tenant users can specify any repository for an Application resource.
    However, that's not appropriate from a security perspective.
    Admission webhook will deny the creation of Application that contains an unauthorized repository.

[Teleport]: https://goteleport.com
