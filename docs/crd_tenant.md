
### Custom Resources

* [Tenant](#tenant)

### Sub Resources

* [ArgoCDApplicationSpec](#argocdapplicationspec)
* [ArgoCDSpec](#argocdspec)
* [NamespaceSpec](#namespacespec)
* [TeleportApplicationSpec](#teleportapplicationspec)
* [TeleportNodeSpec](#teleportnodespec)
* [TeleportSpec](#teleportspec)
* [TenantList](#tenantlist)
* [TenantSpec](#tenantspec)

#### ArgoCDApplicationSpec

ArgoCDApplicationSpec defines the desired state of Application

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| name | Name is the name of Application resource. | string | true |
| path | Path is a directory path within the Git repository, and is only valid for applications sourced from Git. | string | true |
| repoURL | RepoURL is the URL to the repository (Git or Helm) that contains the application manifests. | string | true |
| targetRevision | TargetRevision defines the revision of the source to sync the application to. In case of Git, this can be commit, tag, or branch. If omitted, will equal to HEAD. In case of Helm, this is a semver tag for the Chart's version. | string | true |

[Back to Custom Resources](#custom-resources)

#### ArgoCDSpec

ArgoCDSpec defines the desired state of the settings for Argo CD

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| applications | Applications are the list of Application resources managed by the tenant team. | [][ArgoCDApplicationSpec](#argocdapplicationspec) | false |
| repositories | Repositories are the list of repositories used by the tenant team. | []string | false |
| extraAdmins | ExtraAdmins are the names of the team to add to the AppProject user. Specify this if you want other tenant teams to be able to use your AppProject. | []string | false |

[Back to Custom Resources](#custom-resources)

#### NamespaceSpec

NamespaceSpec defines the desired state of Namespace

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| name | Name is the name of namespace to be generated | string | true |
| labels | Labels are the labels to add to the namespace | map[string]string | false |
| annotations | Annotations are the annotations to add to the namespace | map[string]string | false |
| extraAdmins | ExtraAdmins are the names of the team to add to the namespace administrator. Specify this if you want other tenant teams to be able to use your namespace. | []string | false |

[Back to Custom Resources](#custom-resources)

#### TeleportApplicationSpec

TeleportApplicationSpec defines the desired state of Teleport Application.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| name | Name is the name of the application to proxy. | string | true |
| url | URL is the internal address of the application to proxy. | string | true |
| extraArgs | ExtraArgs are the list of additional arguments to be specified for Teleport Application Pod. | []string | false |

[Back to Custom Resources](#custom-resources)

#### TeleportNodeSpec



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| replicas | Replicas is the number of Teleport Node Pods. | int | true |
| extraArgs | ExtraArgs are the list of additional arguments to be specified for Teleport Node Pod. | []string | false |

[Back to Custom Resources](#custom-resources)

#### TeleportSpec

TeleportSpec defines the desired state of the settings for Teleport

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| node | Node is the settings of Teleport Node for the tenant team. | *[TeleportNodeSpec](#teleportnodespec) | false |
| applications | Applications are the list of applications to be used by the tenant team. | [][TeleportApplicationSpec](#teleportapplicationspec) | false |

[Back to Custom Resources](#custom-resources)

#### Tenant

Tenant is the Schema for the tenants API

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata |  | metav1.ObjectMeta | false |
| spec |  | [TenantSpec](#tenantspec) | false |
| status |  | [TenantStatus](#tenantstatus) | false |

[Back to Custom Resources](#custom-resources)

#### TenantList

TenantList contains a list of Tenant

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata |  | metav1.ListMeta | false |
| items |  | [][Tenant](#tenant) | true |

[Back to Custom Resources](#custom-resources)

#### TenantSpec

TenantSpec defines the desired state of Tenant

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| namespaces |  | [][NamespaceSpec](#namespacespec) | false |
| argocd |  | *[ArgoCDSpec](#argocdspec) | false |
| teleport |  | *[TeleportSpec](#teleportspec) | false |

[Back to Custom Resources](#custom-resources)
