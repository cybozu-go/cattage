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
$ kubeclt get rolebinding -n your-root
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

```console
$ kubectl accurate sub create your-sub your-root
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

```console
$ kubectl apply -f application.yaml
```

Make sure that the Application resource are synchronized.

```console
$ kubectl get application -n your-sub
NAME        SYNC STATUS   HEALTH STATUS
testhttpd   Synced        Healthy
```

```console
$ kubectl get application -n argocd
NAME        SYNC STATUS   HEALTH STATUS
testhttpd   Synced        Healthy
```

Get the result of synchronization as events.

```console
$ kubectl get events -n your-sub
LAST SEEN   TYPE     REASON              OBJECT                  MESSAGE
45s         Normal   ApplicationSynced   application/testhttpd   Sync application spec succeeded
34s         Normal   StatusSynced        application/testhttpd   Sync application status succeeded
```

## How to manage resources that already exist

Cattage can manage resources that have existed before with Tenant and Application.

You can make an existing namespace belong to Tenant.
However, the namespace must be root or not managed by accurate.

A RoleBinding resource named `<tenant-name>-admin` will be created on a namespace belonging to a tenant.
If a resource with the same name already exists, it will be overwritten.

A AppProject resource with the same name as a tenant will be created in argocd namespace.
If a resource with the same name already exists, it will be overwritten.

When you create an Application on a sub-namespace, an Application resource with the same name will be created in argocd namespace.
If that Application in argocd namespace exists, the Application will be overwritten only if the `spec.project` filed matches.
If not, then the creation of the Application resource will be rejected.

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

```console
$ kubectl accurate sub move your-sub new-root
```

As a result, `application/testhttpd` in your-sub will be out of sync with `application/testhttpd` in argocd.
You can see it as an event resource.

```console
$ kubectl get events -n your-sub
LAST SEEN   TYPE      REASON         OBJECT                  MESSAGE
10s         Warning   CannotSync     application/testhttpd   project 'your-team' of the application 'your-sub/testhttpd' does not match the tenant name 'new-team'
```

Please change the project of `application/testhttpd` correctly.

```console
$ kubectl patch app testhttpd -n your-sub --type='json' -p '[{ "op": "replace", "path": "/spec/project", "value": "new-team"}]'
```

The application will sync again.

## Remove resources

When a tenant user delete an Application resource on the tenant's namespace, an Application resource on argocd namespace will be deleted as well.
If `resources-finalizer.argocd.argoproj.io` is annotated, resources deployed by the Application will be deleted.

When an administrator deleted a tenant resource:
- Root-namespaces and sub-namespaces for the tenant will remain
- RoleBinding on the namespaces will be deleted
- Applications on the namespaces will be deleted
- AppProject for the tenant will be deleted
