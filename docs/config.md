# Configurations

## Configuration file

`neco-tenant-controller` reads its configurations from a configuration file.

The repository includes an example as follows:

```yaml
namespace:
  # Labels to add to all namespaces to be deployed by neco-tenant-controller
  commonLabels:
    accurate.cybozu.com/template: init-template

argocd:
  # The name of namespace where Argo CD is deployed
  namespace: argocd

  appProjectTemplate: |
    apiVersion: argoproj.io/v1alpha1
    kind: AppProject
    spec:
      namespaceResourceBlacklist:
        - group: ""
          kind: ResourceQuota
        - group: ""
          kind: LimitRange
      orphanedResources:
        warn: false
      sourceRepos:
        - '*'
```
