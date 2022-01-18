load('ext://restart_process', 'docker_build_with_restart')
load('ext://cert_manager', 'deploy_cert_manager')

def kubebuilder():

    DOCKERFILE = '''FROM golang:alpine
    WORKDIR /
    COPY ./bin/cattage-controller /
    CMD ["/cattage-controller"]
    '''

    def manifests():
        return './bin/controller-gen crd rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases;'

    def generate():
        return './bin/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./...";'

    def binary():
        return 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -o bin/cattage-controller cmd/cattage-controller/main.go'

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

    docker_build_with_restart('cattage:dev', '.',
     dockerfile_contents=DOCKERFILE,
     entrypoint=['/cattage-controller', '--zap-devel=true'],
     only=['./bin/cattage-controller'],
     live_update=[
           sync('./bin/cattage-controller', '/cattage-controller'),
       ]
    )

# deploy_cert_manager(version="v1.6.1")
kubebuilder()