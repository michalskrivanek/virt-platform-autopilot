/*
Copyright 2026 The KubeVirt Authors.

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
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubevirt/virt-platform-autopilot/pkg/assets"
	pkgcontext "github.com/kubevirt/virt-platform-autopilot/pkg/context"
)

func renderObservabilityAsset(t *testing.T, assetName string, annotations map[string]string) *unstructured.Unstructured {
	t.Helper()

	loader := assets.NewLoader()
	registry, err := assets.NewRegistry(loader)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	renderer := NewRenderer(loader)

	hco := &unstructured.Unstructured{}
	hco.SetAPIVersion("hco.kubevirt.io/v1")
	hco.SetKind("HyperConverged")
	hco.SetName("kubevirt-hyperconverged")
	hco.SetNamespace("openshift-cnv")
	if annotations != nil {
		hco.SetAnnotations(annotations)
	}

	renderCtx := &pkgcontext.RenderContext{
		HCO: hco,
	}

	asset, err := registry.GetAsset(assetName)
	if err != nil {
		t.Fatalf("Failed to get asset %q: %v", assetName, err)
	}

	rendered, err := renderer.RenderAsset(asset, renderCtx)
	if err != nil {
		t.Fatalf("Failed to render asset %q: %v", assetName, err)
	}

	if rendered == nil {
		t.Fatalf("Rendered asset %q is nil", assetName)
	}

	return rendered
}

func TestMonitoringUIPluginWithoutIncidents(t *testing.T) {
	rendered := renderObservabilityAsset(t, "monitoring-ui-plugin", nil)

	if rendered.GetKind() != "UIPlugin" {
		t.Errorf("Kind = %q, want UIPlugin", rendered.GetKind())
	}
	if rendered.GetName() != "monitoring" {
		t.Errorf("Name = %q, want monitoring", rendered.GetName())
	}

	pluginType, _, _ := unstructured.NestedString(rendered.Object, "spec", "type")
	if pluginType != "Monitoring" {
		t.Errorf("spec.type = %q, want Monitoring", pluginType)
	}

	persesEnabled, found, _ := unstructured.NestedBool(rendered.Object, "spec", "monitoring", "perses", "enabled")
	if !found {
		t.Fatal("spec.monitoring.perses.enabled not found")
	}
	if !persesEnabled {
		t.Error("spec.monitoring.perses.enabled should be true")
	}

	_, found, _ = unstructured.NestedBool(rendered.Object, "spec", "monitoring", "incidents", "enabled")
	if found {
		t.Error("spec.monitoring.incidents should NOT be present without annotation")
	}
}

func TestMonitoringUIPluginWithIncidents(t *testing.T) {
	annotations := map[string]string{
		"platform.kubevirt.io/enable-incident-detection": "true",
	}

	// Render base asset (always applied)
	baseRendered := renderObservabilityAsset(t, "monitoring-ui-plugin", annotations)

	persesEnabled, found, _ := unstructured.NestedBool(baseRendered.Object, "spec", "monitoring", "perses", "enabled")
	if !found {
		t.Fatal("base asset: spec.monitoring.perses.enabled not found")
	}
	if !persesEnabled {
		t.Error("base asset: spec.monitoring.perses.enabled should be true")
	}

	// Base asset should not have incidents field
	_, found, _ = unstructured.NestedBool(baseRendered.Object, "spec", "monitoring", "incidents", "enabled")
	if found {
		t.Error("base asset: spec.monitoring.incidents should NOT be present")
	}

	// Render incidents asset (opt-in, applied after base)
	incidentsRendered := renderObservabilityAsset(t, "monitoring-ui-plugin-incidents", annotations)

	// Incidents asset should have both perses and incidents
	persesEnabled, found, _ = unstructured.NestedBool(incidentsRendered.Object, "spec", "monitoring", "perses", "enabled")
	if !found {
		t.Fatal("incidents asset: spec.monitoring.perses.enabled not found")
	}
	if !persesEnabled {
		t.Error("incidents asset: spec.monitoring.perses.enabled should be true")
	}

	incidentsEnabled, found, _ := unstructured.NestedBool(incidentsRendered.Object, "spec", "monitoring", "incidents", "enabled")
	if !found {
		t.Fatal("incidents asset: spec.monitoring.incidents.enabled not found when annotation is set")
	}
	if !incidentsEnabled {
		t.Error("incidents asset: spec.monitoring.incidents.enabled should be true when annotation is set")
	}

	// Both should target the same resource
	if baseRendered.GetName() != incidentsRendered.GetName() {
		t.Errorf("assets target different resources: base=%s, incidents=%s", baseRendered.GetName(), incidentsRendered.GetName())
	}
}

func TestMonitoringUIPluginIncidentsFalseAnnotation(t *testing.T) {
	annotations := map[string]string{
		"platform.kubevirt.io/enable-incident-detection": "false",
	}

	// Base asset should always render
	baseRendered := renderObservabilityAsset(t, "monitoring-ui-plugin", annotations)

	persesEnabled, found, _ := unstructured.NestedBool(baseRendered.Object, "spec", "monitoring", "perses", "enabled")
	if !found {
		t.Fatal("base asset: spec.monitoring.perses.enabled not found")
	}
	if !persesEnabled {
		t.Error("base asset: spec.monitoring.perses.enabled should be true")
	}

	// Base asset should not have incidents
	_, found, _ = unstructured.NestedBool(baseRendered.Object, "spec", "monitoring", "incidents", "enabled")
	if found {
		t.Error("base asset: spec.monitoring.incidents should NOT be present when annotation is 'false'")
	}

	// Incidents asset should not render when annotation is "false"
	// (In real reconciliation, this asset would be skipped due to conditions not met)
	// Here we're just verifying the template doesn't conditionally hide it
	incidentsRendered := renderObservabilityAsset(t, "monitoring-ui-plugin-incidents", annotations)

	// The incidents asset template is unconditional, so it always includes incidents
	// The filtering happens at the asset selection level (conditions in metadata.yaml)
	incidentsEnabled, found, _ := unstructured.NestedBool(incidentsRendered.Object, "spec", "monitoring", "incidents", "enabled")
	if !found {
		t.Error("incidents asset template should always include incidents field (filtering happens at reconcile level)")
	}
	if !incidentsEnabled {
		t.Error("incidents asset template should always set incidents.enabled to true")
	}
}

func TestTroubleshootingPanelUIPlugin(t *testing.T) {
	rendered := renderObservabilityAsset(t, "troubleshooting-panel-ui-plugin", nil)

	if rendered.GetKind() != "UIPlugin" {
		t.Errorf("Kind = %q, want UIPlugin", rendered.GetKind())
	}
	if rendered.GetName() != "troubleshooting-panel" {
		t.Errorf("Name = %q, want troubleshooting-panel", rendered.GetName())
	}

	pluginType, _, _ := unstructured.NestedString(rendered.Object, "spec", "type")
	if pluginType != "TroubleshootingPanel" {
		t.Errorf("spec.type = %q, want TroubleshootingPanel", pluginType)
	}
}
