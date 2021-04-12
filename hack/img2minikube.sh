#!/bin/bash

TMPDIR=$(mktemp -d)

pushd "$TMPDIR" || exit

docker save -o image.dockerimg $1

scp -i ~/.minikube/machines/minikube/id_rsa image.dockerimg docker@"$(minikube ip)":~/image.dockerimg

ssh -i  ~/.minikube/machines/minikube/id_rsa docker@"$(minikube ip)" 'docker load -i ~/image.dockerimg'

popd || exit
