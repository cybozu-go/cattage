load('ext://restart_process', 'docker_build_with_restart')

DOCKERFILE = '''FROM golang:alpine
WORKDIR /
COPY ./bin/cattage-controller /
CMD ["/cattage-controller"]
'''


def manifests():
    return './bin/controller-gen crd rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases;'


def generate():
    return './bin/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./...";'


def apidoc():
    return 'make apidoc;'


def binary():
    return 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -o bin/cattage-controller cmd/cattage-controller/main.go'


# Don't watch generated files
watch_settings(ignore=['config/crd/bases/', 'config/rbac/role.yaml', 'config/webhook/manifests.yaml'])

# Generate manifests and go files
local(manifests() + generate() + apidoc())

# Deploy CRD
local_resource(
    'CRD', manifests() + 'kustomize build config/crd | kubectl apply -f -', deps=["api"],
    ignore=['*/*/zz_generated.deepcopy.go'], labels=['cattage'])

# Deploy Cattage
k8s_yaml(kustomize('./config/dev'))
k8s_resource(new_name='Cattage Resources', objects=[
    'cattage:namespace',
    'tenants.cattage.cybozu.io:customresourcedefinition',
    'cattage-mutating-webhook-configuration:mutatingwebhookconfiguration',
    'cattage-controller-manager:serviceaccount',
    'cattage-leader-election-role:role',
    'cattage-manager-role:clusterrole',
    'cattage-leader-election-rolebinding:rolebinding',
    'cattage-manager-rolebinding:clusterrolebinding',
    'cattage-controller-config:configmap',
    'cattage-manager-config:configmap',
    'cattage-serving-cert:certificate',
    'cattage-selfsigned-issuer:issuer',
    'cattage-validating-webhook-configuration:validatingwebhookconfiguration'
], labels=['cattage'])

k8s_resource(workload='cattage-controller-manager', labels=['cattage'])
local_resource(
    'Watch & Compile', generate() + binary(), deps=['controllers', 'pkg', 'hooks', 'cmd', 'version.go', 'api'],
    ignore=['*/*/zz_generated.deepcopy.go'],
    labels=['cattage'])

docker_build_with_restart(
    'cattage:dev', '.',
    dockerfile_contents=DOCKERFILE,
    entrypoint=['/cattage-controller', '--zap-devel=true'],
    only=['./bin/cattage-controller'],
    live_update=[
        sync('./bin/cattage-controller', '/cattage-controller'),
    ]
)

# Sample
local_resource(
    'Sample: Template', 'kubectl apply -f ./config/samples/template.yaml',
    deps=["./config/samples/template.yaml"], labels=['sample'])
local_resource(
    'Sample: Tenant', 'kubectl apply -f ./config/samples/tenant.yaml', deps=["./config/samples/tenant.yaml"],
    resource_deps=["cattage-controller-manager", "Sample: Template"], labels=['sample'])
local_resource(
    'Sample: SubNamespace', 'kubectl apply -f ./config/samples/subnamespace.yaml',
    deps=["./config/samples/subnamespace.yaml"], resource_deps=["Sample: Tenant"], labels=['sample'])
local_resource(
    'Wait for SubNamespace',
    'kubectl wait namespace/sub-1 --for=jsonpath="{.status.phase}"=Active --timeout=10s',
    resource_deps=["Sample: SubNamespace"], labels=['sample'])
local_resource(
    'Sample: Application', 'kubectl apply -f ./config/samples/application.yaml',
    deps=["./config/samples/application.yaml"], resource_deps=["Wait for SubNamespace"], labels=['sample'])
