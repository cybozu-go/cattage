# Usage

## Create a Tenant resource

Administrators can create the following tenant resource for a tenant team.

```yaml
apiVersion: cattage.cybozu.io/v1beta1
kind: Tenant
metadata:
  name: your-team
spec:
  rootNamespaces:
    - name: your-root
```

The name of the tenant resource must match the name of the group in Kubernetes and Argo CD.
The namespaces specified in `spec.rootNamespaces` will be created automatically.

```console
$ kubectl get ns your-root
NAME        STATUS   AGE
your-root   Active   1m
```

RoleBinding and AppProject resource are also automatically created.

```console
$ kubectl get rolebinding -n your-root
NAME              ROLE                AGE
your-team-admin   ClusterRole/admin   2m
```

```console
$ kubectl get appproject -n argocd your-team
NAME        AGE
your-team   2m
```

## Create an Application resource

Tenant users can create a SubNamespace on their namespaces.

```sh
kubectl accurate sub create your-sub your-root
```

Tenant users can create an Application resource in the sub-namespace.

Prepare an Application resource as follows:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: testhttpd
  namespace: your-sub
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: your-team
  source:
    repoURL: https://github.com/cybozu-go/cattage.git
    targetRevision: main
    path: samples/testhttpd
  destination:
    server: https://kubernetes.default.svc
    namespace: your-sub
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

Apply the resource:

```sh
kubectl apply -f application.yaml
```

Make sure that the Application resource is synchronized.

```console
$ kubectl get application -n your-sub
NAME        SYNC STATUS   HEALTH STATUS
testhttpd   Synced        Healthy
```

## How to manage resources that already exist

Cattage can manage resources that have existed before with Tenant and Application.

You can make an existing namespace belong to Tenant.
However, the namespace must be root or not managed by accurate.

A RoleBinding resource named `<tenant-name>-admin` will be created on a namespace belonging to a tenant.
If a resource with the same name already exists, it will be overwritten.

An AppProject resource with the same name as a tenant will be created in argocd namespace.
If a resource with the same name already exists, it will be overwritten.

## How to change ownership

The ownership of sub-namespace can be transferred to other tenant.

Prepare a new tenant:

```yaml
apiVersion: cattage.cybozu.io/v1beta1
kind: Tenant
metadata:
  name: new-team
spec:
  rootNamespaces:
    - name: new-root
```

Use `kubectl accurate sub move` command to change the parent of your-sub namespace to new-root.

```bash
kubectl accurate sub move your-sub new-root
```

As a result, `application/testhttpd` in your-sub will be out of sync.
Please change the project of `application/testhttpd` correctly.

```bash
kubectl patch app testhttpd -n your-sub --type='json' -p '[{ "op": "replace", "path": "/spec/project", "value": "new-team"}]'
```

The application will be synced again.

## Remove resources

When an administrator deleted a tenant resource:

- Root-namespaces and sub-namespaces for the tenant will remain
- RoleBinding on the namespaces will be deleted
- Applications on the namespaces will be deleted
- AppProject for the tenant will be deleted
