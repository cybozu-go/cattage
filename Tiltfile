load('ext://restart_process', 'docker_build_with_restart')
load('ext://cert_manager', 'deploy_cert_manager')

def kubebuilder():

    DOCKERFILE = '''FROM golang:alpine
    WORKDIR /
    COPY ./bin/neco-tenant-controller /
    CMD ["/neco-tenant-controller"]
    '''

    def manifests():
        return 'controller-gen crd rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases;'

    def generate():
        return 'controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./...";'

    def binary():
        return 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -o bin/neco-tenant-controller cmd/neco-tenant-controller/main.go'

    installed = local("which kubebuilder")
    print("kubebuilder is present:", installed)

    DIRNAME = os.path.basename(os. getcwd())

    local(manifests() + generate())

    local_resource('CRD', manifests() + 'kustomize build config/crd | kubectl apply -f -', deps=["api"], ignore=['*/*/zz_generated.deepcopy.go'])

    watch_settings(ignore=['config/crd/bases/', 'config/rbac/role.yaml', 'config/webhook/manifests.yaml'])
    k8s_yaml(kustomize('./config/dev'))

    deps = ['controllers', 'pkg', 'hooks', 'cmd', 'version.go']
    deps.append('api')

    local_resource('Watch&Compile', generate() + binary(), deps=deps, ignore=['*/*/zz_generated.deepcopy.go'])

    local_resource('Sample YAML', 'kubectl apply -f ./config/samples', deps=["./config/samples"], resource_deps=[DIRNAME + "-controller-manager"])

    docker_build_with_restart('neco-tenant-controller:dev', '.',
     dockerfile_contents=DOCKERFILE,
     entrypoint=['/neco-tenant-controller', '--zap-devel=true'],
     only=['./bin/neco-tenant-controller'],
     live_update=[
           sync('./bin/neco-tenant-controller', '/neco-tenant-controller'),
       ]
    )

# deploy_cert_manager(version="v1.6.1")
kubebuilder()
