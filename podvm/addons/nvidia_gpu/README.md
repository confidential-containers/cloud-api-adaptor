## Introduction

This addon enables nvidia GPU support in the podvm image.

You need to specify the GPU instance types in the cloud-api-adaptor configMap (peer-pods-cm).

Here is an example. Replace it as appropriate depending on the specific provider and region

```
# For AWS 
PODVM_INSTANCE_TYPES: "t3.small,c5.xlarge,p3.2xlarge"

# For Azure
AZURE_INSTANCE_SIZES: "Standard_D8as_v5,Standard_D4as_v5,Standard_NC6s_v3,Standard_NC4as_T4_v3"

```

Example pod definition:
```
apiVersion: v1
kind: Pod
metadata:
  name: gpu-test
  labels:
    app: test
  annotations:
    io.katacontainers.config.hypervisor.machine_type: Standard_NC4as_T4_v3
    io.containerd.cri.runtime-handler: kata-remote
spec:
  runtimeClassName: kata-remote
  containers:
    - name: ubuntu
      image: ubuntu
      command: ["sleep"]
      args: ["infinity"]
      env:
        - name: NVIDIA_VISIBLE_DEVICES
          value: "all"
```

You can verify the GPU devices by execing a shell in the pod as shown below:

```
$ kubectl exec -it gpu-test -- bash
root@gpu-test:/# nvidia-smi
Thu Nov 23 17:30:58 2023
+---------------------------------------------------------------------------------------+
| NVIDIA-SMI 535.129.03             Driver Version: 535.129.03   CUDA Version: 12.2     |
|-----------------------------------------+----------------------+----------------------+
| GPU  Name                 Persistence-M | Bus-Id        Disp.A | Volatile Uncorr. ECC |
| Fan  Temp   Perf          Pwr:Usage/Cap |         Memory-Usage | GPU-Util  Compute M. |
|                                         |                      |               MIG M. |
|=========================================+======================+======================|
|   0  Tesla T4                       Off | 00000001:00:00.0 Off |                  Off |
| N/A   36C    P8               9W /  70W |      2MiB / 16384MiB |      0%      Default |
|                                         |                      |                  N/A |
+-----------------------------------------+----------------------+----------------------+

+---------------------------------------------------------------------------------------+
| Processes:                                                                            |
|  GPU   GI   CI        PID   Type   Process name                            GPU Memory |
|        ID   ID                                                             Usage      |
|=======================================================================================|
|  No running processes found                                                           |
+---------------------------------------------------------------------------------------+

root@gpu-test:/# nvidia-smi -L
GPU 0: Tesla T4 (UUID: GPU-2b9a9945-a56c-fcf3-7156-8e380cf1d0cc)

root@gpu-test:/# ls -l /dev/nvidia*
crw-rw-rw- 1 root root 235,   0 Nov 23 17:27 /dev/nvidia-uvm
crw-rw-rw- 1 root root 235,   1 Nov 23 17:27 /dev/nvidia-uvm-tools
crw-rw-rw- 1 root root 195,   0 Nov 23 17:27 /dev/nvidia0
crw-rw-rw- 1 root root 195, 255 Nov 23 17:27 /dev/nvidiactl

```
