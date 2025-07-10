# SyncWindow

## Overview

Argo CD has a feature called Sync Windows.
This is a functionality to restrict application synchronization during specific time periods.  
https://argo-cd.readthedocs.io/en/stable/user-guide/sync_windows/

However, to configure Sync Windows, you need to modify the `AppProject` resource.
Modifying the `AppProject` resource is essentially equivalent to having administrator privileges.
Therefore, when operating Argo CD in a multi-tenant environment, tenant users cannot freely configure Sync Windows.

Similar concerns have been raised in Argo CD Issues as well.  
https://github.com/argoproj/argo-cd/issues/11755

Therefore, Cattage provides a [`SyncWindow` custom resource](crd_syncwindow.yaml) that allows tenant users to create it freely.
Cattage identifies the tenant to which the namespace where the `SyncWindow` resource is created belongs, and configures syncWindows field in the `AppProject` resource associated with that tenant.

When multiple `SyncWindow` resources are created within the same tenant, their contents are merged and reflected in the `AppProject` resource.

## How to use

Create a `SyncWindow` resource as follows:

```yaml
apiVersion: cattage.cybozu.io/v1beta1
kind: SyncWindow
metadata:
  name: syncwindow-sample
  namespace: sub-1
spec:
  syncWindows:
  - kind: allow
    schedule: '10 1 * * *'
    duration: 1h
    applications:
    - '*-prod'
    manualSync: true
  - kind: deny
    schedule: '0 22 * * *'
    timeZone: "Europe/Amsterdam"
    duration: 1h
    namespaces:
    - default
```

`SYNCED` status will be `True` as shown below:

```console
$ kubectl get syncwindow -n sub-1
NAME                 SYNCED
syncwindow-sample    True
```

Then, `syncWindows` field will be reflected in the `AppProject`:

```console
$ kubectl get appproject -n argocd a-team -o yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  labels:
    cattage.cybozu.io/tenant: a-team
  name: a-team
  namespace: argocd
spec:
  destinations:
  - namespace: app-a
    server: '*'
  - namespace: sub-1
    server: '*'
  sourceNamespaces:
  - app-a
  - sub-1
  syncWindows:
  - kind: allow
    schedule: '10 1 * * *'
    duration: 1h
    applications:
    - '*-prod'
    manualSync: true
  - kind: deny
    schedule: '0 22 * * *'
    timeZone: "Europe/Amsterdam"
    duration: 1h
    namespaces:
    - default
```
