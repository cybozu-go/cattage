# Setup

## Kubernetes cluster

Cattage is a controller that runs in a soft multi-tenancy Kubernetes cluster.
Namespaces must be isolated for each tenant.

There are many ways to achieve Namespace isolation.
In EKS and GKE, you can integrate RBAC with IAM.
For on-premises, [Teleport](https://goteleport.com) and [Loft](https://loft.sh) may help you.

## Argo CD

Install Argo CD as shown in the following page:

https://argo-cd.readthedocs.io/en/stable/getting_started/

Cattage isolates AppProject resource for each tenant.

So, please refer to the following page to enable user management.
Argo CD supports a lot of authentication methods.

https://argo-cd.readthedocs.io/en/stable/operator-manual/user-management/

Cattage expects tenant users to be able to create Application resources.
Apply the following manifest:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: application-admin
  labels:
    rbac.authorization.k8s.io/aggregate-to-admin: "true"
rules:
- apiGroups:
  - argoproj.io
  resources:
  - applications
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
  - deletecollection
```

Cattage requires Argo CD's [Applications in any namespace](https://argo-cd.readthedocs.io/en/stable/operator-manual/app-any-namespace/) is enabled.
In order to enable the feature, add `--application-namespace="*"` parameter to `argocd-server` and `argocd-application-controller`.

## cert-manager

Cattage and Accurate depend on [cert-manager][] to issue TLS certificate for admission webhooks.
If cert-manager is not installed on your cluster, install it as follows:

```console
$ curl -fsLO https://github.com/jetstack/cert-manager/releases/latest/download/cert-manager.yaml
$ kubectl apply -f cert-manager.yaml
```

## Accurate

Cattage depends on Accurate.
It expects `cattage.cybozu.io/tenant` labels and RoleBinding resources to be propagated.

Include the following settings in your values.yaml:

```yaml
controller:
  config:
    labelKeys:
      - cattage.cybozu.io/tenant
    watches:
      - group: rbac.authorization.k8s.io
        version: v1
        kind: RoleBinding
```

Install Accurate with the values.yaml as follows:

```console
$ helm install --create-namespace --namespace accurate accurate -f values.yaml accurate/accurate
```

For more information, see the following page:

https://cybozu-go.github.io/accurate/helm.html

## Cattage

Prepare values.yaml as follows:

```yaml
controller:
  config:
    namespace:
      roleBindingTemplate: |
        apiVersion: rbac.authorization.k8s.io/v1
        kind: RoleBinding
        roleRef:
          apiGroup: rbac.authorization.k8s.io
          kind: ClusterRole
          name: admin
        subjects:
          - apiGroup: rbac.authorization.k8s.io
            kind: Group
            name: {{ .Name }}
          {{- range .Roles.admin }}
          - apiGroup: rbac.authorization.k8s.io
            kind: Group
            name: {{ . }}
          {{- end }}
    argocd:
      namespace: argocd
      appProjectTemplate: |
        apiVersion: argoproj.io/v1alpha1
        kind: AppProject
        spec:
          destinations:
          {{- range .Namespaces }}
            - namespace: {{ . }}
              server: '*'
          {{- end }}
          roles:
            - groups:
                - {{ .Name }}
                {{- range .Roles.admin }}
                - {{ . }}
                {{- end }}
              name: admin
              policies:
                - p, proj:{{ .Name }}:admin, applications, *, {{ .Name }}/*, allow
          sourceNamespaces:
            {{- range .Namespaces }}
            - {{ . }}
            {{- end }}
          sourceRepos:
            {{- range .Repositories }}
            - '{{ . }}'
            {{- else }}
            - '*'
            {{- end }}
```

`appProjectTemplate` and `roleBindingTemplate` should be configured appropriately for your multi-tenancy environment.
Read [Configurations](config.md) for details.

Setup Helm repository:

 ```console
 $ helm repo add cattage https://cybozu-go.github.io/cattage
 $ helm repo update
   ```

Install the Helm chart with your values.yaml:

```console
$ helm install --create-namespace --namespace cattage cattage cattage/cattage -f values.yaml
```
