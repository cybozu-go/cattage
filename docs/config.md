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
  # The mode of validation for Application resources.
  # If true is set, this does not deny Application resources but issues a warning.
  permissiveValidation: true

  organization: "cybozu-go"

  appProjectTemplate: |
    apiVersion: argoproj.io/v1alpha1
    kind: AppProject
    spec:
      namespaceResourceBlacklist:
        - group: ""
          kind: ResourceQuota
        - group: ""
          kind: LimitRange
        - group: networking.k8s.io
          kind: NetworkPolicy
      orphanedResources:
        warn: false
      sourceRepos:
        - '*'

teleport:
  # The name of namespace where Teleport Nodes are deployed
  namespace: teleport
  # The name of Teleport container image
  image: quay.io/cybozu/teleport-node:latest
  # The name of secret resource contains a license key for Teleport Enterprise
  licenseSecretName: teleport-general-secret-20210310
```
