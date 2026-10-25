package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ScutumNodeSpec defines the desired state of ScutumNode.
type ScutumNodeSpec struct {
	// HubRef references the ScutumHub in the same namespace that this node joins.
	HubRef corev1.LocalObjectReference `json:"hubRef"`

	// NodeName is the display name registered with the hub mesh.
	NodeName string `json:"nodeName"`

	// NodeType is the install type for this node: "remote" or "combined".
	// +kubebuilder:default="remote"
	NodeType string `json:"nodeType,omitempty"`

	// Image is the container image to use for the edge node.
	// +kubebuilder:default="ghcr.io/sovforge/scutum:latest"
	Image string `json:"image,omitempty"`

	// Storage configures persistent volumes for data and secrets.
	// +optional
	Storage ScutumNodeStorage `json:"storage,omitempty"`

	// Resources sets compute resource requirements for the node container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// TLSSkipVerify skips TLS verification when calling the hub API.
	// Appropriate when the hub uses a self-signed certificate.
	// +kubebuilder:default=true
	TLSSkipVerify bool `json:"tlsSkipVerify,omitempty"`

	// MeshAddress is the desired WireGuard mesh IP for this node, e.g. "10.0.0.2/32".
	// Optional — if empty the hub assigns one automatically.
	// +optional
	MeshAddress string `json:"meshAddress,omitempty"`
}

// ScutumNodeStorage holds PVC size configuration for an edge node.
type ScutumNodeStorage struct {
	// DataSize is the size of the data PVC.
	// +kubebuilder:default="2Gi"
	DataSize string `json:"dataSize,omitempty"`

	// SecretsSize is the size of the secrets PVC.
	// +kubebuilder:default="256Mi"
	SecretsSize string `json:"secretsSize,omitempty"`

	// StorageClass is the storage class for both PVCs. Defaults to cluster default.
	// +optional
	StorageClass string `json:"storageClass,omitempty"`
}

// ScutumNodeStatus defines the observed state of ScutumNode.
type ScutumNodeStatus struct {
	// Ready is true when the node is fully enrolled and its StatefulSet has a ready replica.
	Ready bool `json:"ready,omitempty"`

	// Phase is the current lifecycle phase: Pending, Enrolling, Configuring, Running, or Error.
	Phase string `json:"phase,omitempty"`

	// NodeID is the UUID assigned by the hub after enrollment.
	NodeID string `json:"nodeID,omitempty"`

	// MeshAddress is the WireGuard mesh IP assigned to this node.
	MeshAddress string `json:"meshAddress,omitempty"`

	// Conditions contains standard metav1 conditions for the node.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation last processed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.ready`
// +kubebuilder:printcolumn:name="NodeID",type=string,JSONPath=`.status.nodeID`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ScutumNode is the Schema for the scutumnodes API.
type ScutumNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScutumNodeSpec   `json:"spec,omitempty"`
	Status ScutumNodeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ScutumNodeList contains a list of ScutumNode.
type ScutumNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScutumNode `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ScutumNode{}, &ScutumNodeList{})
}
