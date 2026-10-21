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

package engine

import (
	"context"
	"testing"
	"testing/fstest"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kubevirt/virt-platform-autopilot/pkg/assets"
	pkgcontext "github.com/kubevirt/virt-platform-autopilot/pkg/context"
)

func TestReconcileTombstonesMatching(t *testing.T) {
	loader := assets.NewLoaderFromFS(fstest.MapFS{
		"tombstones/test.yaml": &fstest.MapFile{Data: []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  namespace: default
  labels:
    platform.kubevirt.io/managed-by: virt-platform-autopilot
`)},
	})
	live := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      "test",
			"namespace": "default",
			"labels":    map[string]any{assets.TombstoneLabel: assets.TombstoneLabelValue},
		},
	}}
	c := fake.NewClientBuilder().WithObjects(live).Build()
	r := NewTombstoneReconciler(c, loader)
	hco := pkgcontext.NewMockHCO("kubevirt-hyperconverged", "openshift-cnv")

	deleted, err := r.ReconcileTombstonesMatching(context.Background(), hco, func(assets.TombstoneMetadata) bool { return false })
	if err != nil || deleted != 0 {
		t.Fatalf("filtered tombstone cleanup: deleted=%d err=%v", deleted, err)
	}
	check := live.DeepCopy()
	if err := c.Get(context.Background(), client.ObjectKey{Name: "test", Namespace: "default"}, check); err != nil {
		t.Fatalf("filtered tombstone was deleted: %v", err)
	}

	deleted, err = r.ReconcileTombstonesMatching(context.Background(), hco, func(assets.TombstoneMetadata) bool { return true })
	if err != nil || deleted != 1 {
		t.Fatalf("matching tombstone cleanup: deleted=%d err=%v", deleted, err)
	}
	if err := c.Get(context.Background(), client.ObjectKey{Name: "test", Namespace: "default"}, check); !apierrors.IsNotFound(err) {
		t.Fatalf("matching tombstone remains: %v", err)
	}
}

func TestMachineConfigTombstonesHaveEffectiveNodeDisruptionPolicy(t *testing.T) {
	loader := assets.NewLoaderFromFS(fstest.MapFS{
		"tombstones/machineconfig.yaml": &fstest.MapFile{Data: []byte(`apiVersion: machineconfiguration.openshift.io/v1
kind: MachineConfig
metadata:
  name: retired
  labels:
    platform.kubevirt.io/managed-by: virt-platform-autopilot
`)},
	})
	live := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "machineconfiguration.openshift.io/v1",
		"kind":       "MachineConfig",
		"metadata": map[string]any{
			"name":   "retired",
			"labels": map[string]any{assets.TombstoneLabel: assets.TombstoneLabelValue},
		},
		"spec": map[string]any{"config": map[string]any{
			"storage": map[string]any{"files": []any{map[string]any{"path": "/usr/local/bin/tune"}}},
			"systemd": map[string]any{"units": []any{map[string]any{"name": "tune.service"}}},
		}},
	}}
	policy := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "operator.openshift.io/v1", "kind": "MachineConfiguration",
		"metadata": map[string]any{"name": "cluster"},
		"status": map[string]any{"nodeDisruptionPolicyStatus": map[string]any{"clusterPolicies": map[string]any{
			"files": []any{map[string]any{"path": "/usr/local/bin/tune"}},
			"units": []any{map[string]any{"name": "other.service"}},
		}}},
	}}
	r := NewTombstoneReconciler(fake.NewClientBuilder().WithObjects(live, policy).Build(), loader)
	blocked, err := r.MachineConfigTombstonesHaveEffectiveNodeDisruptionPolicy(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !blocked {
		t.Fatal("effective action for a tombstoned MachineConfig file must block deletion")
	}

	_ = unstructured.SetNestedField(policy.Object, []any{map[string]any{"path": "/other"}}, "status", "nodeDisruptionPolicyStatus", "clusterPolicies", "files")
	r = NewTombstoneReconciler(fake.NewClientBuilder().WithObjects(live, policy).Build(), loader)
	blocked, err = r.MachineConfigTombstonesHaveEffectiveNodeDisruptionPolicy(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if blocked {
		t.Fatal("unrelated effective actions must not block deletion")
	}
}
