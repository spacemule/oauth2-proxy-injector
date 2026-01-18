package mutation

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PatchOperation represents a single JSON Patch operation (RFC 6902)
// Used to describe mutations to the Pod spec
type PatchOperation struct {
	// Op is the operation type: "add", "remove", "replace", "move", "copy", "test"
	Op string `json:"op"`

	// Path is the JSON Pointer to the location in the document
	// Example: "/spec/containers/-" (append to containers array)
	Path string `json:"path"`

	// Value is the value for add/replace operations
	// Omitted for remove operations
	Value interface{} `json:"value,omitempty"`
}

// PatchBuilder provides a fluent interface for building JSON patch operations
// This makes it easier to construct patches without worrying about JSON pointer syntax
type PatchBuilder interface {
	// AddContainer appends a container to the pod's containers list
	AddContainer(container interface{}) PatchBuilder

	// AddInitContainer appends an init container
	AddInitContainer(container interface{}) PatchBuilder

	// AddVolume appends a volume to the pod's volumes list
	AddVolume(volume interface{}) PatchBuilder

	// AddVolumeMount adds a volume mount to a specific container by index
	AddVolumeMount(containerIndex int, mount interface{}) PatchBuilder

	// AddAnnotation adds or updates an annotation
	AddAnnotation(key, value string) PatchBuilder

	// AddLabel adds or updates a label
	AddLabel(key, value string) PatchBuilder

	// RemovePort removes a port from a container
	RemovePort(containerIndex, portIndex int) PatchBuilder

	// ReplaceProbePort replaces a probe's port (from name to number)
	// probeType is one of: "livenessProbe", "readinessProbe", "startupProbe"
	// handlerType is one of: "httpGet", "tcpSocket"
	ReplaceProbePort(containerIndex int, probeType, handlerType string, port int32) PatchBuilder

	// ReplaceEnvVarValue replaces an environment variable's value in a container
	// Used for Knative support to redirect queue-proxy's USER_PORT
	ReplaceEnvVarValue(containerIndex, envIndex int, newValue string) PatchBuilder

	// Build returns the accumulated patch operations
	Build() []PatchOperation
}

// JSONPatchBuilder implements PatchBuilder
type JSONPatchBuilder struct {
	operations []PatchOperation
	// hasAnnotations tracks if the pod already has annotations
	// (needed to decide between "add" object vs "add" key)
	hasAnnotations bool
	// hasLabels tracks if the pod already has labels
	hasLabels bool
	// hasVolumes tracks if the pod already has volumes
	hasVolumes bool
}

func NewPatchBuilder(hasAnnotations, hasLabels, hasVolumes bool) *JSONPatchBuilder {
	return &JSONPatchBuilder{
		hasAnnotations: hasAnnotations,
		hasLabels: hasLabels,
		hasVolumes: hasVolumes,
	}
}

func (b *JSONPatchBuilder) AddContainer(container interface{}) PatchBuilder {
	b.operations = append(b.operations, PatchOperation{
		Op: "add",
		Path: "/spec/containers/-",
		Value: container,
	})
	return b
}

func (b *JSONPatchBuilder) AddInitContainer(container interface{}) PatchBuilder {
	b.operations = append(b.operations, PatchOperation{
		Op: "add",
		Path: "/spec/initContainers/-",
		Value: container,
	})
	return b
}

// AddVolume appends to /spec/volumes/-
func (b *JSONPatchBuilder) AddVolume(volume interface{}) PatchBuilder {
	if !b.hasVolumes {
		b.operations = append(b.operations, PatchOperation{
			Op: "add",
			Path: "/spec/volumes",
			Value: []interface{}{},
		})
		b.hasVolumes = true
	}
	
	b.operations = append(b.operations, PatchOperation{
		Op: "add",
		Path: "/spec/volumes/-",
		Value: volume,
	})
	
	return b
}

func (b *JSONPatchBuilder) AddVolumeMountsArray(containerIndex int) PatchBuilder {
	b.operations = append(b.operations, PatchOperation{
		Op: "add",
		Path: fmt.Sprintf("/spec/containers/%d/volumeMounts", containerIndex),
		Value: []interface{}{},
		
	})
	return b
}

func (b *JSONPatchBuilder) AddVolumeMount(containerIndex int, mount interface{}) PatchBuilder {
	b.operations = append(b.operations, PatchOperation{
		Op: "add",
		Path: fmt.Sprintf("/spec/containers/%d/volumeMounts/-", containerIndex),
		Value: mount,
	})
	return b
}

// AddAnnotation adds or updates an annotation
func (b *JSONPatchBuilder) AddAnnotation(key, value string) PatchBuilder {
	if !b.hasAnnotations {
		b.operations = append(b.operations, PatchOperation{
			Op: "add",
			Path: "/metadata/annotations",
			Value: map[string]string{},
		})

		b.hasAnnotations = true
	}
	b.operations = append(b.operations, PatchOperation{
		Op: "add",
		Path: "/metadata/annotations/" + escapeJSONPointer(key),
		Value: value,
	})

	return b
}

// AddLabel adds or updates a label
func (b *JSONPatchBuilder) AddLabel(key, value string) PatchBuilder {
	if !b.hasLabels {
		b.operations = append(b.operations, PatchOperation{
			Op: "add",
			Path: "/metadata/labels",
			Value: map[string]string{},
		})

		b.hasLabels = true
	}
	b.operations = append(b.operations, PatchOperation{
		Op: "add",
		Path: "/metadata/labels/" + escapeJSONPointer(key),
		Value: value,
	})

	return b
}

func (b *JSONPatchBuilder) RemovePort(containerIndex, portIndex int) PatchBuilder {
	b.operations = append(b.operations, PatchOperation{
		Op: "remove",
		Path: fmt.Sprintf("/spec/containers/%d/ports/%d", containerIndex, portIndex),
	})

	return b
}

// ReplaceProbePort replaces a probe's port from a name to a number
func (b *JSONPatchBuilder) ReplaceProbePort(containerIndex int, probeType, handlerType string, port int32) PatchBuilder {
	b.operations = append(b.operations, PatchOperation{
		Op: "replace",
		Path: fmt.Sprintf("/spec/containers/%d/%s/%s/port", containerIndex, probeType, handlerType),
		Value: port,
	})

	return b
}

// ReplaceEnvVarValue replaces an environment variable's value in a container
func (b *JSONPatchBuilder) ReplaceEnvVarValue(containerIndex, envIndex int, newValue string) PatchBuilder {
	b.operations = append(b.operations, PatchOperation{
		Op: "replace",
		Path: fmt.Sprintf("/spec/containers/%d/env/%d/value", containerIndex, envIndex),
		Value: newValue,
	})

	return b
}

// Build returns the accumulated patch operations
func (b *JSONPatchBuilder) Build() []PatchOperation {
	ret := make([]PatchOperation, len(b.operations))
	copy(ret, b.operations)
	return ret
}

func escapeJSONPointer(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	return strings.ReplaceAll(s, "/", "~1")
}

// MarshalPatches converts patch operations to JSON bytes
// It's only here to keep encoding/json imports out of other packages
func MarshalPatches(patches []PatchOperation) ([]byte, error) {
	return json.Marshal(patches)
}