#!/bin/bash

# Copyright 2020 PayPal.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

GIT_PROJECT_ROOT="$(git rev-parse --show-toplevel)"

pushd "$GIT_PROJECT_ROOT" || exit

# Build latest katbox
docker build . -t quay.io/katbox/katboxplugin:latest

# Load latest katbox on to kind
kind load docker-image quay.io/katbox/katboxplugin:latest

# Pull sidecar containers on to local machine
docker pull quay.io/k8scsi/livenessprobe:v1.1.0
docker pull quay.io/k8scsi/csi-node-driver-registrar:v1.3.0
docker pull busybox:1.32.0

# Load sidecar containers
kind load docker-image quay.io/k8scsi/livenessprobe:v1.1.0
kind load docker-image quay.io/k8scsi/csi-node-driver-registrar:v1.3.0
kind load docker-image busybox:1.32.0

# Deploy katbox onto kind
kubectl apply -f deploy/latest/katbox/csi-katbox-plugin.yaml,deploy/latest/katbox/csi-katbox-driverinfo.yaml

# Run a sample application
kubectl apply -f examples/csi-app-inline.yaml

# Wait for sample app to be ready
kubectl wait --for=condition=Ready pod/my-csi-app-inline

# Confirm that data has been printed on to the correct volume
sleep 10

OUTPUT=$(kubectl exec pod/my-csi-app-inline -- sh -c "cat /data/test")
PODUID=$(kubectl get pod my-csi-app-inline -o jsonpath='{.metadata.uid}')
MOUNTDIR="/var/lib/kubelet/pods/$PODUID/volumes/kubernetes.io~csi/my-csi-volume/"
VOLID=$(kubectl exec --filename deploy/latest/katbox/csi-katbox-plugin.yaml -c katbox -- sh -c "cat $MOUNTDIR/vol_data.json" | jq -r .volumeHandle)

echo "Volume ID: $VOLID"
echo "Mounted at: $MOUNTDIR"

sleep 10

if [ -z "$OUTPUT" ]; then
  echo "FAILURE: No data found in test output";
fi

# Delete example app
kubectl delete -f examples/csi-app-inline.yaml

# Make sure mount point is gone
kubectl exec --filename deploy/latest/katbox/csi-katbox-plugin.yaml -c katbox -- sh -c "if [ -d ""$MOUNTDIR"" ]; then echo 'Error: Mount point still exists'; else echo ''; fi"

# Make sure folder allocated by katbox was deleted after 1 minutes
echo "Sleeping for 60 seconds to test if folder allocated by katbox was deleted"
sleep 60

kubectl exec --filename deploy/latest/katbox/csi-katbox-plugin.yaml -c katbox -- sh -c "if [ -d ""/csi-data-dir/$VOLID"" ]; then echo 'Error: Mount point still exists'; else echo ''; fi"

# Delete katbox daemon set
kubectl delete -f deploy/latest/katbox/csi-katbox-plugin.yaml

popd || exit
