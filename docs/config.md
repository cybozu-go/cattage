# Configurations

## Configuration file

`cattage-controller` reads a configuration file on startup. The default location is `/etc/cattage/config.yaml`.
The location can be changed with `--config-file` flag.

The configuration file should be a JSON or YAML file having the following keys:

| Key                                          | Type                | Description                                                                                                                                      |
|----------------------------------------------|---------------------|--------------------------------------------------------------------------------------------------------------------------------------------------|
| `namespace.commonLabels`                     | `map[string]string` | Labels to be added to all namespaces belonging to all tenants. This may be overridden by `rootNamespaces.labels` of a tenant resource.           |
| `namespace.commonAnnotations`                | `map[string]string` | Annotations to be added to all namespaces belonging to all tenants. This may be overridden by `rootNamespaces.annotations` of a tenant resource. |
| `namespace.roleBindingTemplate`              | `string`            | Template for RoleBinding resource that is created on all namespaces belonging to a tenant.                                                       |
| `argocd.namepsace`                           | `string`            | The name of namespace where Argo CD is running.                                                                                                  |
| `argocd.appProjectTemplate`                  | `string`            | Template for AppProject resources that is created for each tenant.                                                                               |
| `argocd.preventAppCreationInArgoCDNamespace` | `bool`              | If true, prevent creating applications in the Argo CD namespace. This is used to enable sharding.                                                |

The repository includes an example as follows:

```yaml
namespace:
  commonLabels:
    accurate.cybozu.com/template: init-template
  commonAnnotations:
    foo: bar
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
        name: {{ .Name }}
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
            # If `GitHubName` is specified as `ExtraParams`, use it, otherwise use `Name`.
            - {{with .ExtraParams.GitHubTeam}}{{ . }}{{else}}{{ .Name }}{{end}}
            {{- range .Roles.admin }}
            - {{with .ExtraParams.GitHubTeam}}{{ . }}{{else}}{{ .Name }}{{end}}
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

`roleBindingTemplate` and `appProjectTemplate` can be written in go-template format.

`roleBindingTemplate` can use the following variables:

| Key           | Type                | Description                                                                      |
|---------------|---------------------|----------------------------------------------------------------------------------|
| `Name`        | `string`            | The name of the tenant.                                                          |
| `Roles`       | `map[string]Role`   | Map of other tenants that are accessible to this tenant. The key is a role name. |
| `ExtraParams` | `map[string]string` | Extra parameters specified per tenant.                                           |

`appProjectTemplate` can use the following variables:

| Key            | Type                | Description                                                                      |
|----------------|---------------------|----------------------------------------------------------------------------------|
| `Name`         | `string`            | The name of the tenant.                                                          |
| `Namespaces`   | `[]string`          | List of namespaces belonging to a tenant (including sub-namespaces).             |
| `Repositories` | `[]string`          | List of repository URLs which can be used by the tenant.                         |
| `Roles`        | `map[string]Role`   | Map of other tenants that are accessible to this tenant. The key is a role name. |
| `ExtraParams`  | `map[string]string` | Extra parameters specified per tenant.                                           |

| Key           | Type                | Description                            |
|---------------|---------------------|----------------------------------------|
| `Name`        | `string`            | The name of the tenant.                |
| `ExtraParams` | `map[string]string` | Extra parameters specified per tenant. |

## Environment variables

| Name            | Required | Description                                    |
|-----------------|----------|------------------------------------------------|
| `POD_NAMESPACE` | Yes      | The namespace name where `cattage` is running. |

## Command-line flags

```
Flags:
      --add_dir_header                   If true, adds the file directory to the header
      --alsologtostderr                  log to standard error as well as files
      --cert-dir string                  webhook certificate directory
      --config-file string               Configuration file path (default "/etc/cattage/config.yaml")
      --health-probe-addr string         Listen address for health probes (default ":8081")
  -h, --help                             help for cattage-controller
      --leader-election-id string        ID for leader election by controller-runtime (default "cattage")
      --log_backtrace_at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                   If non-empty, write log files in this directory
      --log_file string                  If non-empty, use this log file
      --log_file_max_size uint           Defines the maximum size a log file can grow to. Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --logtostderr                      log to standard error instead of files (default true)
      --metrics-addr string              The address the metric endpoint binds to (default ":8080")
      --skip_headers                     If true, avoid header prefixes in the log messages
      --skip_log_headers                 If true, avoid headers when opening log files
      --stderrthreshold severity         logs at or above this threshold go to stderr (default 2)
  -v, --v Level                          number for the log level verbosity
      --version                          version for cattage-controller
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
      --webhook-addr string              Listen address for the webhook endpoint (default ":9443")
      --zap-devel                        Development Mode defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn). Production Mode defaults(encoder=jsonEncoder,logLevel=Info,stackTraceLevel=Error)
      --zap-encoder encoder              Zap log encoding (one of 'json' or 'console')
      --zap-log-level level              Zap Level to configure the verbosity of logging. Can be one of 'debug', 'info', 'error', or any integer value > 0 which corresponds to custom debug levels of increasing verbosity
      --zap-stacktrace-level level       Zap Level at and above which stacktraces are captured (one of 'info', 'error', 'panic').
```
