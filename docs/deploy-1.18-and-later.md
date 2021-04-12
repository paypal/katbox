## Cluster setup
Kubernetes 1.16+ required due to the Volume context `csi.storage.k8s.io/ephemeral` not existing before this version.

### Create CSIDriver object

Using kubectl, create a CSIDriver object for katbox
```yaml
apiVersion: storage.k8s.io/v1
kind: CSIDriver
metadata:
  name: katbox.csi.paypal.com
spec:
  # Supports persistent and ephemeral inline volumes.
  volumeLifecycleModes:
  - Ephemeral
  # To determine at runtime which mode a volume uses, pod info and its
  # "csi.storage.k8s.io/ephemeral" entry are needed.
  podInfoOnMount: true
  attachRequired: false
```
### Deploy DaemonSet on to cluster

#### Create a namespace (optional)

It makes it easier for all katbox pods to run in a different namespace.

A dedicated namespace can be created by using kubectl to apply the following configuration:
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: csi-plugins
```

#### Creating the DaemonSet
Deploy the DaemonSet to run katbox (preferably in a namespace that is not used by default)

```shell
$ kubectl apply --namespace csi-plugins -f csi-katbox-plugin.yaml
```
```yaml
kind: DaemonSet
apiVersion: apps/v1
metadata:
  name: csi-katboxplugin
spec:
  selector:
    matchLabels:
      app: csi-katboxplugin
  template:
    metadata:
      labels:
        app: csi-katboxplugin
    spec:
      hostNetwork: true
      tolerations:
        - operator: "Exists"
      containers:
        - name: node-driver-registrar
          image: quay.io/k8scsi/csi-node-driver-registrar:v1.3.0
          args:
            - --v=5
            - --csi-address=/csi/csi.sock
            - --kubelet-registration-path=/var/lib/kubelet/plugins/csi-katbox/csi.sock
          securityContext:
            # This is necessary only for systems with SELinux, where
            # non-privileged sidecar containers cannot access unix domain socket
            # created by privileged CSI driver container.
            privileged: true
          env:
            - name: KUBE_NODE_NAME
              valueFrom:
                fieldRef:
                  apiVersion: v1
                  fieldPath: spec.nodeName
          volumeMounts:
          - mountPath: /csi
            name: socket-dir
          - mountPath: /registration
            name: registration-dir
          - mountPath: /csi-data-dir
            name: csi-data-dir

        - name: katbox
          image: quay.io/katbox/katboxplugin:latest
          args:
            - "--drivername=katbox.csi.paypal.com"
            - "--v=1"
            - "--endpoint=$(CSI_ENDPOINT)"
            - "--nodeid=$(KUBE_NODE_NAME)"
            - "--afterlifespan=3h"
          env:
            - name: CSI_ENDPOINT
              value: unix:///csi/csi.sock
            - name: KUBE_NODE_NAME
              valueFrom:
                fieldRef:
                  apiVersion: v1
                  fieldPath: spec.nodeName
          securityContext:
            privileged: true
          ports:
          - containerPort: 9898
            name: healthz
            protocol: TCP
          livenessProbe:
            failureThreshold: 5
            httpGet:
              path: /healthz
              port: healthz
            initialDelaySeconds: 10
            timeoutSeconds: 3
            periodSeconds: 2
          volumeMounts:
            - mountPath: /csi
              name: socket-dir
            - mountPath: /var/lib/kubelet/pods
              mountPropagation: Bidirectional
              name: mountpoint-dir
            - mountPath: /var/lib/kubelet/plugins
              mountPropagation: Bidirectional
              name: plugins-dir
            - mountPath: /csi-data-dir
              name: csi-data-dir
        - name: liveness-probe
          volumeMounts:
          - mountPath: /csi
            name: socket-dir
          image: quay.io/k8scsi/livenessprobe:v1.1.0
          args:
          - --csi-address=/csi/csi.sock
          - --health-port=9898

      volumes:
        - hostPath:
            path: /var/lib/kubelet/plugins/csi-katbox
            type: DirectoryOrCreate
          name: socket-dir
        - hostPath:
            path: /var/lib/kubelet/pods
            type: DirectoryOrCreate
          name: mountpoint-dir
        - hostPath:
            path: /var/lib/kubelet/plugins_registry
            type: Directory
          name: registration-dir
        - hostPath:
            path: /var/lib/kubelet/plugins
            type: Directory
          name: plugins-dir
        - hostPath:
            path: /var/lib/csi-katbox-data/
            type: DirectoryOrCreate
          name: csi-data-dir
```

### Run example application and validate

Next, validate the deployment.  First, ensure all expected pods are running properly:

```shell
$ kubectl get pods
NAME                     READY   STATUS    RESTARTS   AGE
csi-katboxplugin-298f5   3/3     Running   0          43h
csi-katboxplugin-2qd8d   3/3     Running   0          43h
csi-katboxplugin-hvkjf   3/3     Running   0          43h
csi-katboxplugin-n62fm   3/3     Running   0          43h
csi-katboxplugin-x824j   3/3     Running   3          43h
csi-katboxplugin-zjr6g   3/3     Running   0          43h
```

There should be exactly one katbox pod per node able to schedule work.

From the [examples directory](../examples), run `csi-app-inline.yaml`
```yaml
kind: Pod
apiVersion: v1
metadata:
  name: my-csi-app-inline
spec:
  containers:
    - name: my-frontend
      image: busybox
      volumeMounts:
      - mountPath: "/data"
        name: my-csi-volume
      command: ["sh", "-c", "while true; do echo hello >> /data/test; sleep 100; done"]
  volumes:
    - name: my-csi-volume
      csi:
        driver: katbox.csi.paypal.com
```

Finally, inspect the application pod `my-csi-app`  which mounts a katbox volume:

```shell
$ kubectl describe pods/my-csi-app
Name:         my-csi-app-inline
Namespace:    default
Priority:     0
Node:         k8s-test-node-4/10.180.73.244
Start Time:   Thu, 16 Jul 2020 13:44:53 -0700
Labels:       <none>
Annotations:  Status:  Running
IP:           10.180.96.189
IPs:
  IP:  10.180.96.189
Containers:
  my-frontend:
    Container ID:  docker://f777b8c44d0d146241d73bbc2663b85274dca2e954c19d23ff504e81ffc0e875
    Image:         busybox
    Image ID:      docker-pullable://busybox@sha256:9ddee63a712cea977267342e8750ecbc60d3aab25f04ceacfa795e6fce341793
    Port:          <none>
    Host Port:     <none>
    Command:
      sh
      -c
      while true; do echo hello >> /data/test; sleep 100; done
    State:          Running
      Started:      Thu, 16 Jul 2020 13:44:59 -0700
    Ready:          True
    Restart Count:  0
    Environment:    <none>
    Mounts:
      /data from my-csi-volume (rw)
      /var/run/secrets/kubernetes.io/serviceaccount from default-token-wrfhf (ro)
Conditions:
  Type              Status
  Initialized       True
  Ready             True
  ContainersReady   True
  PodScheduled      True
Volumes:
  my-csi-volume:
    Type:              CSI (a Container Storage Interface (CSI) volume source)
    Driver:            katbox.csi.paypal.com
    FSType:
    ReadOnly:          false
    VolumeAttributes:  <none>
  default-token-wrfhf:
    Type:        Secret (a volume populated by a Secret)
    SecretName:  default-token-wrfhf
    Optional:    false
QoS Class:       BestEffort
Node-Selectors:  <none>
Tolerations:     node.kubernetes.io/not-ready:NoExecute for 300s
                 node.kubernetes.io/unreachable:NoExecute for 300s
Events:
  Type     Reason            Age                From                          Message
  ----     ------            ----               ----                          -------
  Normal   Scheduled         <unknown>          default-scheduler             Successfully assigned default/my-csi-app-inline to k8s-test-node-4
  Normal   Pulling           32s                kubelet, k8s-test-node-4  Pulling image "busybox"
  Normal   Pulled            28s                kubelet, k8s-test-node-4  Successfully pulled image "busybox"
  Normal   Created           28s                kubelet, k8s-test-node-4  Created container my-frontend
  Normal   Started           27s                kubelet, k8s-test-node-4  Started container my-frontend
```

## Confirm the katbox driver works
The katpox driver is configured to create new volumes under `/csi-data-dir` inside the katbox container that is specified in the plugin DaemonSet previously deployed.

A file written in a properly mounted katbox volume inside an application should show up inside the katbox container.  The following steps confirms that katbox is working properly.  First, create a file from the application pod as shown:

```shell
$ kubectl exec -it my-csi-app-inline -- /bin/sh
/ # touch /data/hello-world
/ # exit
```

Find the node in which the sample app is running in by running:
```shell
$ kubectl get pods my-csi-app-inline -o wide
NAME                READY   STATUS    RESTARTS   AGE     IP              NODE                  NOMINATED NODE   READINESS GATES
my-csi-app-inline   1/1     Running   0          7m57s   10.180.96.189   k8s-test-node-4   <none>           <none>
```

Next, find the katbox driver for the node on which the sample application is running on:
```shell
$ kubectl get pods -n csi-plugins -o wide --field-selector spec.nodeName=k8s-test-node-4
NAME                     READY   STATUS    RESTARTS   AGE   IP              NODE                  NOMINATED NODE   READINESS GATES
csi-katboxplugin-n62fm   3/3     Running   0          43h   10.180.73.244   k8s-test-node-4   <none>           <none>
```

Next, ssh into the katbox container and verify that the file shows up there:
```shell
$ kubectl exec -it csi-katboxplugin-n62fm -n csi-plugin -c katbox -- /bin/sh
```
Then, use the following command to locate the file. If everything works OK you should get a result similar to the following:

```shell
/ # find / -name hello-world
/csi-data-dir/csi-69121cc2ba7624a259442664bc942c00811cf4495faefccdd11efc2e79d1127c/hello-world
/var/lib/kubelet/pods/32a784c5-88a3-4585-8827-989d2c79dbfe/volumes/kubernetes.io~csi/my-csi-volume/mount/hello-world
/ #
```


