#!/bin/bash
# Lint check to prevent KubeletConfig usage in assets.
#
# KubeletConfig resources cannot be safely merged: each one completely
# overwrites /etc/kubernetes/kubelet.conf, even though MCO reports success
# for all of them. This causes unpredictable kubelet configuration.
#
# The modular solution is to use MachineConfig to drop files under
# /etc/openshift/kubelet.conf.d as documented in:
# - https://kubernetes.io/blog/2025/12/22/kubernetes-v1-35-kubelet-config-drop-in-directory-ga/
# - https://kubernetes.io/docs/tasks/administer-cluster/kubelet-config-file/#kubelet-conf-d
#
# All files are merged in lexicographical order, making configuration
# predictable and allowing incremental updates without requiring node drains.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "Checking for prohibited KubeletConfig usage..."

# Check asset YAML files for KubeletConfig CRD (not KubeletConfiguration format files)
# We want to block the KubeletConfig CustomResource, which has API version
# machineconfiguration.openshift.io/v1, not the KubeletConfiguration format
# used in drop-in files (apiVersion: kubelet.config.k8s.io/v1beta1).
# Exclude tombstones directory - those are resources marked for deletion.
if grep -r "kind: KubeletConfig" "${REPO_ROOT}/assets/" --include="*.yaml" --include="*.yaml.tpl" --exclude-dir=tombstones | \
   grep -v "KubeletConfiguration" | \
   grep "kind: KubeletConfig" 2>/dev/null; then
    echo ""
    echo "ERROR: KubeletConfig CRD found in assets!"
    echo ""
    echo "KubeletConfig resources cannot be safely merged - each one completely"
    echo "overwrites /etc/kubernetes/kubelet.conf, causing unpredictable behavior."
    echo ""
    echo "Use MachineConfig to drop files to /etc/openshift/kubelet.conf.d instead."
    echo "See hack/lint-no-kubeletconfig.sh for details and references."
    exit 1
fi

# Check metadata.yaml for KubeletConfig component
if grep -E '^\s+component:\s+KubeletConfig' "${REPO_ROOT}/assets/active/metadata.yaml" 2>/dev/null; then
    echo ""
    echo "ERROR: component: KubeletConfig found in metadata.yaml!"
    echo ""
    echo "Change component to MachineConfig and drop kubelet config files to"
    echo "/etc/openshift/kubelet.conf.d instead."
    exit 1
fi

echo "✓ No KubeletConfig usage found"
