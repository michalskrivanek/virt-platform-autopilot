apiVersion: machineconfiguration.openshift.io/v1
kind: MachineConfig
metadata:
  labels:
    machineconfiguration.openshift.io/role: worker
  name: 96-worker-kubelet-cpu-manager
spec:
  config:
    ignition:
      version: 3.5.0
    storage:
      files:
      - path: /etc/openshift/kubelet.conf.d/96-cpu-manager.conf
        overwrite: true
        contents:
          source: data:text/plain;charset=utf-8;base64,{{ readAsset "machine-config/07-kubelet-cpu-manager/kubelet-96-cpu-manager.conf" | b64enc }}
        mode: 420
