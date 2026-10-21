/*
Copyright 2026 The Virt Platform Autopilot Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/virt-platform-autopilot/pkg/assets"
	"github.com/kubevirt/virt-platform-autopilot/pkg/engine"
)

var _ = Describe("MachineConfiguration node disruption policy SSA", func() {
	It("prunes only Autopilot entries after the final dependent is disabled", func() {
		Expect(InstallCRDs(ctx, k8sClient, CRDSetOpenShift)).To(Succeed())
		DeferCleanup(func() { _ = UninstallCRDs(ctx, k8sClient, CRDSetOpenShift) })

		const name = "cluster"
		policy := func(files []any) *unstructured.Unstructured {
			return &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "operator.openshift.io/v1", "kind": "MachineConfiguration",
				"metadata": map[string]any{"name": name},
				"spec": map[string]any{"nodeDisruptionPolicy": map[string]any{
					"files": files, "units": []any{},
				}},
			}}
		}
		shared := map[string]any{"path": "/usr/local/bin/shared", "actions": []any{map[string]any{"type": "None"}}}
		foreign := map[string]any{"path": "/etc/admin-owned", "actions": []any{map[string]any{"type": "None"}}}
		applier := engine.NewApplier(k8sClient, apiReader)
		_, err := applier.Apply(ctx, policy([]any{shared}), true)
		Expect(err).NotTo(HaveOccurred())

		admin := policy([]any{foreign})
		admin.SetName(name)
		Expect(k8sClient.Apply(ctx, client.ApplyConfigurationFromUnstructured(admin), client.FieldOwner("cluster-admin"))).To(Succeed())

		// One dependent still references the shared entry, so the aggregate keeps it.
		_, err = applier.Apply(ctx, policy([]any{shared}), true)
		Expect(err).NotTo(HaveOccurred())
		// After the final dependent is disabled, the real NDP reconciler applies
		// its empty aggregate. This must release only Autopilot-owned entries.
		patcher := engine.NewPatcher(k8sClient, apiReader, assets.NewLoader())
		managed, err := patcher.ReconcileNodeDisruptionPolicies(ctx, []assets.AssetMetadata{{
			Name: "disabled-dependent",
			NodeDisruptionPolicy: &assets.NodeDisruptionPolicyDependency{
				Asset: "unused-while-inactive",
			},
		}}, map[string]bool{}, false, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(managed).To(BeTrue())

		live := &unstructured.Unstructured{}
		live.SetAPIVersion("operator.openshift.io/v1")
		live.SetKind("MachineConfiguration")
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: name}, live)).To(Succeed())
		files, found, err := unstructured.NestedSlice(live.Object, "spec", "nodeDisruptionPolicy", "files")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(files).To(HaveLen(1))
		path, _, err := unstructured.NestedString(files[0].(map[string]any), "path")
		Expect(err).NotTo(HaveOccurred())
		Expect(path).To(Equal("/etc/admin-owned"), fmt.Sprintf("files: %#v", files))
	})
})
