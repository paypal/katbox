# CSI Katbox Driver

Katbox is an inline ephemeral volume manager with delayed deletion for Kubernetes.

It is inspired by the "Sandbox" functionality provided by
[Apache Mesos](http://mesos.apache.org/documentation/latest/sandbox/).

## Pre-requisite
- Kubernetes cluster
- Running version 1.18 or later
- Access to terminal with `kubectl` installed

## Deployment
Deployment varies depending on the Kubernetes version your cluster is running:
- [Deployment for Kubernetes 1.18 and later](docs/deploy-1.18-and-later.md)

## Examples
The following examples assume that the CSI Katbox driver has been deployed and validated:
- [Inline ephemeral volumes](docs/example-ephemeral.md)

## Building the binaries
If you want to build the driver yourself, you can do so with the following command from the root directory:

```shell
make
```

## Building a docker image
If you want to make a docker image to be used on a kubernetes cluster you may do so with the following command:

```shell
docker build . -t <image name>
```

## Documentation
A high level overview of how katbox works can be found [here](docs/overview.md)

A closer look at how the creation and deletion of volumes works can be found [here](docs/create-delete-flow.md)

# Credits
This project was initially a fork of
[CSI Hostpath driver](https://github.com/kubernetes-csi/csi-driver-host-path).