package main

import (
	"testing"

	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func TestStorageClassCacheExemption(t *testing.T) {
	exemptions := cacheByObjectExemptions(
		&unstructured.Unstructured{}, &unstructured.Unstructured{},
		&unstructured.Unstructured{}, &unstructured.Unstructured{}, &unstructured.Unstructured{}, false, false, false,
	)
	for object, options := range exemptions {
		if _, ok := object.(*storagev1.StorageClass); !ok {
			continue
		}
		if options.Label == nil || !options.Label.Matches(labels.Set{}) {
			t.Fatal("StorageClasses must be cached without requiring the managed-by label")
		}
		return
	}
	t.Fatal("StorageClass cache exemption is missing")
}

func TestMachineConfigurationCacheExemption(t *testing.T) {
	machineConfiguration := &unstructured.Unstructured{}
	machineConfiguration.SetAPIVersion("operator.openshift.io/v1")
	machineConfiguration.SetKind("MachineConfiguration")
	exemptions := cacheByObjectExemptions(
		&unstructured.Unstructured{}, &unstructured.Unstructured{},
		&unstructured.Unstructured{}, machineConfiguration, &unstructured.Unstructured{}, false, false, true,
	)
	options, found := exemptions[machineConfiguration]
	if !found || options.Label == nil || !options.Label.Matches(labels.Set{}) {
		t.Fatal("MachineConfiguration must be cached without requiring the managed-by label")
	}
}
