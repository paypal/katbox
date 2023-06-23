![status: archive](https://github.com/GIScience/badges/raw/master/status/archive.svg)

# Katbox ![Docker Images](https://github.com/paypal/katbox/actions/workflows/create-push-image.yml/badge.svg)

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
Assuming katbox has been successfully deployed, the following example can be run using `kubectl`:

`$ kubectl apply -f https://raw.githubusercontent.com/paypal/katbox/main/examples/csi-app-inline.yaml`

## Building the binaries
To build the driver, run the following command from the root of the repository:

```shell
make
```

## Building a docker image
To build a docker image to be used on a kubernetes cluster, run the following command from the root of the repository:

```shell
docker build . -t <image name>
```

## Documentation
A high level overview of how katbox works can be found [here](docs/overview.md)

A closer look at how the creation and deletion of volumes works can be found [here](docs/create-delete-flow.md)

# Credits
This project was initially a fork of
[CSI Hostpath driver](https://github.com/kubernetes-csi/csi-driver-host-path).
