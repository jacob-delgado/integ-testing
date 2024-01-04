This is a fork of the Istio test framework. There are many good features of the Istio integration testing framework that I've liked over the years I've had using it, however, it is tied into the istio repository. The goal of this project is to create a fork of the code that allows you do to testing on k8s clusters easily.

This could help people test their projects on K8s, and if it uses Istio could also deploy Istio for their testing purposes.

---
This page introduces the Istio test framework. For an overview of the architecture as well as how
to extend the framework, see the [Developer Guide](https://github.com/istio/istio/wiki/Preparing-for-Development).

- [Introduction](#introduction)
- [Writing Tests](#writing-tests)
- [Running the Tests](#running-the-tests)
    - [Locally with KinD](#locally-with-kind)
    - [Kubernetes](#kubernetes)

## Introduction

Writing tests for cloud-based microservices is hard. Getting the tests to run quickly and reliably is difficult in and of itself. However, supporting
multiple cloud platforms is another thing altogether.

The Istio test framework attempts to address these problems. Some of the objectives for the framework are:

- **Writing Tests**:
  - **Platform Agnostic**: The API abstracts away the details of the underlying platform. This allows the developer to focus on testing the Istio logic rather than plumbing the infrastructure.
  - **Reuseable Tests**: Suites of tests can be written which will run against any platform that supports Istio. This effectively makes conformance testing built-in.

- **Running Tests**:
  - **Standard Tools**: Built on [Go](https://golang.org/)'s testing infrastructure and run with standard commands (e.g. `go test`) .
  - **Easy**: Few or no flags are required to run tests out of the box. While many flags are available, they are provided reasonable defaults where possible.
  - **Fast**: With the ability to run processes natively on the host machine, running tests are orders of magnitude faster.
  - **Reliable**: Running tests natively are inherently more reliable than in-cluster. However, the components for each platform are written to be reliable (e.g. retries).

## Writing Tests

The following examples show some barebones examples of how to use the test framwork. For a full guide on how to write
well-structured, reliable, and fast running tests please read [Writing Good Integration Tests](https://github.com/istio/istio/wiki/Writing-Good-Integration-Tests).

#### Suite

To begin using the test framework, you'll need to write a `TestMain` that simply calls `framework.NewSuite`:

```golang
func TestMain(m *testing.M) {
    framework.
        NewSuite(m).
        Run()
}
```

You can customize your suite to only run in certain environments, or add additional setup by methods on `NewSuite`.
Check the docs for [Suite](https://godoc.org/istio.io/istio/pkg/test/framework#Suite) to see a full list of capabilities.

```golang
func TestMain(m *testing.M) {
    framework.NewTest("my_test", m).
        RequireEnvironmentVersion("1.19"). // Post-submit jobs use older k8s version that may not support features you're testing
        Setup(istio.Setup(nil, setupIstioConfig)). // You probably want to deploy Istio to the cluster(s), to be used by the whole suite.
        Setup(setup). // Perform additional suite-level setup.
        Run()
}

func setupIstioConfig(cfg *istio.Config) {
    // equivalent to `--set your-feature-enabled=true` in the `istioctl install` command.
    // in real tests, consider if your feature can be enabled by default for all integration tests.
    cfg.Values["your-feature-enabled"] = "true"
}

func setup(ctx resource.Context) error {
  // deploy apps, configuration, or other components under test
}

```

The call to `framework.NewSuite` does the following:

1. Checks the current environment to make sure the test is compatible.
2. Runs setup functions in the order they're given.
3. Run all tests in the current package. This is the standard Go behavior for `TestMain`.
4. Stops the environment.

Then, in the same package as `TestMain`, define your tests:

```go
func TestHTTP(t *testing.T) {
    framework.NewTest(t). // 1
        Features("security.egress.mtls.sds"). // 2
        Run(func(ctx framework.TestContext) {
            ctx.Config().ApplyYAMLOrFail(...) // 3
            ctx.WhenDone(func() error {
                ctx.Config().DeleteYAMLOrFail(...)
            })
            a.CallOrFail(ctx, echo.CallOptions{Target: b[0], Count: 2*len(b)}). // 4
                CheckOKOrFail(ctx) // 5
        })
}
```

Every test will follow the pattern in the example above:

1. Calls `framework.NewTest`. This allows the test framwork to cleanup any config/resources that are part of `ctx`, and dump logs/cluster state for individual test failures.
2. Sets a [feature label](https://github.com/istio/istio/blob/master/pkg/test/framework/features/README.md)) to give the project maintainers a high-level indication of what we have covered.
3. Sets up any configuration under test. Always make sure to clean up using `ctx.WhenDone` (never `defer`, or the test framework can't dump the cluster-state while your configuration-under-test is active).
4. Performs the action of the test, usually calling a service.
5. Check results.

## Running the tests

### Locally With KinD

[KinD](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) 0.9.0+ is required to run the tests as described below.


> NOTE: I have been successful running (at least some) tests locally on a Mac if I do everything inside a build container, using `make shell`. I run the `./prow/integ-suite-kind.sh` command to create the kind clusters and then use `--istio.test.kube.topology` on the test invocation. The topology file is a copy of the multicluster.json updated with pointer to the kubeconfig metadata. For example this is added for the `primary` and similarly for the others:
```
    "network": "network-1",
    "meta": {
      "kubeconfig": "/work/artifacts/kubeconfig/primary"
    }  
```
> ~~NOTE: The following instructions don't work on Mac. Multicluster tests need the kubeconfigs to contain an address that is reachable from both the test runner and from within each cluster container. Due to [the lack of docker0 on Docker for Mac](https://docs.docker.com/docker-for-mac/networking/#there-is-no-docker0-bridge-on-macos) we cannot satisfy this requirement.~~

Our CI infrastructure uses [Kubernetes-in-Docker](https://kind.sigs.k8s.io/) to run one or more clusters to test against.
We install [MetalLB](https://metallb.universe.tf/) and manually allocate IPs to the clusters to enable the use of
[LoadBalancer](https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer) type services in the cluster.

We can leverage KinD to test locally as well, using the same script as CI to start our clusters and/or tests: [/prow/integ-suite-kind.sh](https://github.com/istio/istio/blob/015e9cccb1a9a735b00ce9fde4a3c62d0258cb37/prow/integ-suite-kind.sh) can spin up KinD cluster(s) on your local machine as well as trigger tests.

To spin up a single cluster without running any tests or building the code:

```bash
./prow/integ-suite-kind.sh --skip-build --skip-cleanup
```

To spin up multiple clusters (using the default [topology](https://github.com/istio/istio/tree/9c7842669edde9fc697158ce6645e519a0904759/prow/config/topology)):

```bash
mkdir artifacts # create a directory from kubeconfigs and logs
ARTIFACTS=$PWD/artifacts ./prow/integ-suite-kind.sh --topology MULTICLUSTER --skip-build --skip-cleanup
```

Then to run tests, trigger the desired suite(s) with `go test` via the command-line or your IDE:

```bash
ARTIFACTS=artifacts HUB=yourname TAG=yourimage go test -tags integ ./tests/integration/pilot/... \
  --istio.test.kube.config=$PWD/artifacts/kubeconfig/primary,$PWD/artifacts/kubeconfig/remote,$PWD/artifacts/kubeconfig/cross-network-primary  \
  --istio.test.kube.topology=$PWD/prow/config/topology/multicluster.json
```

This script accepts a few flags to customize the kind installation:

| FLAG | Default | Description |
| ---- | ------- | ----------- |
| --node-image | gcr.io/istio-testing/kindest/node:v1.19.1 (subject to change, we try to use the latest Kubernetes version) | Customizes what Kubernetes version KinD runs. |
| --kind-config | prow/config/default.yaml | Points to a file with Kubernetes configuration and feature flags. |
| --skip-setup | "" | Skips creation of KinD clusters. Useful if you want to run make targets using this script. |
| --skip-build | "" | Skips building and pushing images for Istio and test containers. |
| --skip-cleanup | "" | Skips tearing down KinD clusters created by this script. Does NOT skip cleanup inside of the test framework (e.g. cleaning up Istio deployments). |
| --manual | "" | Waits for user input before tearing down KinD clusters creatd by this script. |
| --topology | SINGLE_CLUSTER | Chooses whether to use single or multi-cluster setup. Must be either `SINGLE_CLUSTER` or `MULTICLUSTER` |
| --topology-config | prow/config/topology/multicluster.json | Defines how many clusters to setup, what network they're on, which control plane they use, and more. Only relevant when using `--topology MULTICLUSTER`. |

The above example includes `controlPlaneTopology` to tell the test framework which clusters to use for
[primary and remote clusters](https://istio.io/latest/docs/ops/deployment/deployment-models/). The
`networkTopology` is controlled by the KinD topology config, but we must inform the test framework of which network
each cluster is on, to allow it to install Istio correctly. The test framework itself has many flags to customize how
to run the tests, described in the [Flags section](#flags) below.

### Kubernetes

By default, Istio will be deployed using the configuration in `~/.kube/config`.

You can also run tests against a multi-cluster setup by specifying multiple comma-separated kubeconfig files to `--istio.test.kube.config`.
Each cluster will be treated as a [Primary cluster](https://istio.io/latest/docs/reference/glossary/#primary-cluster), unless `istio.test.kube.controlPlaneTopology`.
is specified. See the Flags section below for more information on testing multi-cluster and multi-network topologies.

To split your `~/.kube/config` into separate files, you can use the following script.

<details>

<summary>splitclusters.sh</summary>

```bash
for ctx in $(kubectl config get-contexts -o name); do
  FILE="$ctx.yaml"
  SHORTNAME=$(echo $ctx | cut -d'_' -f4)
  if [ -n $SHORTNAME ]; then
    FILE=$SHORTNAME.yaml
  fi
  kubectl config view --minify --flatten --context=$ctx > $FILE
done
```

</details>

```bash
mkdir artifacts
ARTIFACTS=artifacts HUB=yourname TAG=yourimage go test -tags integ tests/integration/pilot/... \
  --istio.test.kube.config=cluster1.yaml,cluster2.yaml,cluster3.yaml
```

If you want to test more complex topologies (primary remote, multi-network) and don't want to write long winded flags, consider using topology file instead:

```yaml

- kind: Kubernetes
  clusterName: primary
  network: network-1
  meta:
    kubeconfig: ~/kubeconf/primary
- kind: Kubernetes
  clusterName: remote
  network: network-1
  primaryClusterName: primary
  meta:
    kubeconfig: ~/kubeconf/remote
    # doesn't make sense to have VMs on a remote
    fakeVM: false
- kind: Kubernetes
  clusterName: cross-network-primary
  network: network-2
  meta:
    kubeconfig: ~/kubeconf/cross-network-primary

```

Giving the path to this file via `--istio.test.kube.topology` replaces the need for `istio.test.kube.controlPlaneTopology` and `istio.test.kube.networkTopology` and is much easier to edit. It may be useful to save a copy of this outside the source tree and edit it as you spin up testing environments. 

### Flags

Several flags are available to customize the behavior, such as:

| Flag | Default | Description |
| --- | --- | --- |
| istio.test.env | `native` | Specify the environment to run the tests against. Allowed values are: `kube`, `native`. Defaults to `native`. |
| istio.test.work_dir | '' | Local working directory for creating logs/temp files. If left empty, os.TempDir() is used. |
| istio.test.hub | '' | Container registry hub to use. If not specified, `HUB` environment value will be used. |
| istio.test.tag | '' | Common container tag to use when deploying container images. If not specified `TAG` environment value will be used. |
| istio.test.pullpolicy | `Always` | Common image pull policy to use when deploying container images. If not specified `PULL_POLICY` environment value will be used. Defaults to `Always` |
| istio.test.nocleanup | `false` | Do not cleanup resources after test completion. |
| istio.test.ci | `false` | Enable CI Mode. Additional logging and state dumping will be enabled. |
| istio.test.kube.config | `~/.kube/config` | List of locations of kube config files to be used. If more than one kubeconfig is supplied, the tests are run in multicluster mode. |
| istio.test.kube.minikube | `false` | If `true` access to the ingress will be via nodeport. Should be set to `true` if running on Minikube or KinD without MetalLB. |
| istio.test.kube.loadbalancer | `true` | If `false` access to the ingress will be via nodeport. Should be set to `true` if running on Minikube or KinD without MetalLB. Overrides/replaces `istio.test.kube.minikube` |
| istio.test.kube.systemNamespace | `istio-system` | Deprecated, namespace for Istio deployment. If '', the namespace is generated with the prefix "istio-system-". |
| istio.test.kube.istioNamespace | `istio-system` | Namespace in which Istio ca and cert provisioning components are deployed. |
| istio.test.kube.configNamespace | `istio-system` | Namespace in which config, discovery and auto-injector are deployed. |
| istio.test.kube.telemetryNamespace | `istio-system` | Namespace in which mixer, kiali, tracing providers, graphana, prometheus are deployed. |
| istio.test.kube.policyNamespace | `istio-system` | Namespace in which istio policy checker is deployed. |
| istio.test.kube.ingressNamespace | `istio-system` | Namespace in which istio ingressgateway is deployed. |
| istio.test.kube.egressNamespace | `istio-system` | Namespace in which istio ingressgateway is deployed. |
| istio.test.kube.deploy | `true` | If `true`, the components should be deployed to the cluster. Otherwise, it is assumed that the components have already deployed. |
| istio.test.kube.helm.chartDir | `$(ISTIO)/install/kubernetes/helm/istio` | |
| istio.test.kube.helm.valuesFile | `values-e2e.yaml` | The name of a file (relative to `istio.test.kube.helm.chartDir`) to provide Helm values. |
| istio.test.kube.helm.values | `''` | A comma-separated list of helm values that will override those provided by `istio.test.kube.helm.valuesFile`. These are overlaid on top of a map containing the following: `global.hub=${HUB}`, `global.tag=${TAG}`, `global.proxy.enableCoreDump=true`, `global.mtls.enabled=true`,`galley.enabled=true`. |
| istio.test.kube.controlPlaneTopology | `''` | Specifies the mapping for each cluster to the cluster hosting its control plane. The value is a comma-separated list of the form \<clusterIndex>:\<controlPlaneClusterIndex>, where the indexes refer to the order in which a given cluster appears in the 'istio.test.kube.config' flag. This topology also determines where control planes should be deployed. If not specified, the default is to deploy a control plane per cluster (i.e. `replicated control planes') and map every cluster to itself (e.g. 0:0,1:1,...). |
| istio.test.kube.networkTopology | `''` | Specifies the mapping for each cluster to it's network name, for multi-network scenarios. The value is a comma-separated list of the form \<clusterIndex>:\<networkName>, where the indexes refer to the order in which a given cluster appears in the 'istio.test.kube.config' flag. If not specified, network name will be left unset |
| istio.test.kube.topology | `''` | The path to a JSON file that defines control plane, network, and config cluster topology. The JSON document should be an array of objects that contain the keys "control_plane_index","network_id" and "config_index" with all integer values. If control_plane_index is omitted, the index of the array item is used. If network_id is omitted, 0 will be used. If config_index is omitted, control_plane_index will be used. |

### MacOS problems

1. If you see the following error message
```bash
# runtime/cgo
_cgo_export.c:3:10: fatal error: 'stdlib.h' file not found
FAIL	command-line-arguments [build failed]
FAIL
```
You are missing `include` directory in the standard place. You can specify the sysroot by exporting `CGO_CFLAGS`.

```
export CGO_CFLAGS="-g -O2 -isysroot /Library/Developer/CommandLineTools/SDKs/MacOSX.sdk"
```

