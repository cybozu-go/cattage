
### Custom Resources

* [Tenant](#tenant)

### Sub Resources

* [ArgoCDSpec](#argocdspec)
* [Delegate](#delegate)
* [RootNamespaceSpec](#rootnamespacespec)
* [TenantList](#tenantlist)
* [TenantSpec](#tenantspec)
* [TenantStatus](#tenantstatus)

#### ArgoCDSpec

ArgoCDSpec defines the desired state of the settings for Argo CD

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| repositories | Repositories contains list of repository URLs which can be used by the tenant. | []string | false |

[Back to Custom Resources](#custom-resources)

#### Delegate

Delegate defines a tenant that is delegated access to a tenant.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| name | Name is the name of a delegated tenant | string | true |
| roles | Roles is a list of roles that the tenant has | []string | true |

[Back to Custom Resources](#custom-resources)

#### RootNamespaceSpec

RootNamespaceSpec defines the desired state of Namespace

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| name | Name is the name of namespace to be generated | string | true |
| labels | Labels are the labels to add to the namespace | map[string]string | false |
| annotations | Annotations are the annotations to add to the namespace | map[string]string | false |

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
| rootNamespaces | RootNamespaces are the list of root namespaces that belong to this tenant | [][RootNamespaceSpec](#rootnamespacespec) | true |
| argocd | ArgoCD is the settings of Argo CD for this tenant | [ArgoCDSpec](#argocdspec) | false |
| delegates | Delegates is a list of other tenants that are delegated access to this tenant. | [][Delegate](#delegate) | false |

[Back to Custom Resources](#custom-resources)

#### TenantStatus

TenantStatus defines the observed state of Tenant

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| health | Health is the health of Tenant. | TenantHealth | false |
| conditions | Conditions is an array of conditions. | []metav1.Condition | false |

[Back to Custom Resources](#custom-resources)
