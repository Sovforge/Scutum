package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ScutumHubSpec defines the desired state of ScutumHub.
type ScutumHubSpec struct {
	// Image is the container image to use for the hub.
	// +kubebuilder:default="ghcr.io/sovforge/scutum:latest"
	Image string `json:"image,omitempty"`

	// Replicas is the number of hub replicas. Requires an external database if > 1.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// AdminSecret is the name of a K8s Secret in the same namespace containing
	// keys "username" and "password" used by the operator to authenticate to the hub API.
	AdminSecret string `json:"adminSecret"`

	// Database configures an external database for the hub.
	// +optional
	Database ScutumHubDatabase `json:"database,omitempty"`

	// TLS configures TLS certificate provisioning.
	// +optional
	TLS ScutumHubTLS `json:"tls,omitempty"`

	// WireGuard configures the WireGuard mesh overlay.
	// +optional
	WireGuard ScutumHubWireGuard `json:"wireGuard,omitempty"`

	// Storage configures persistent volumes for data and secrets.
	// +optional
	Storage ScutumHubStorage `json:"storage,omitempty"`

	// Resources sets compute resource requirements for the hub container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// LogLevel sets the application log level (debug, info, warn, error).
	// +kubebuilder:default="info"
	LogLevel string `json:"logLevel,omitempty"`

	// AuditEnabled enables CRA-compliant audit logging.
	// +kubebuilder:default=true
	AuditEnabled bool `json:"auditEnabled,omitempty"`
}

// ScutumHubDatabase holds external database configuration.
type ScutumHubDatabase struct {
	// ExistingSecret is the name of a Secret containing the database URL.
	// +optional
	ExistingSecret string `json:"existingSecret,omitempty"`

	// ExistingSecretKey is the key within ExistingSecret that holds the DATABASE_URL.
	// +kubebuilder:default="DATABASE_URL"
	ExistingSecretKey string `json:"existingSecretKey,omitempty"`
}

// ScutumHubTLS holds TLS certificate configuration.
type ScutumHubTLS struct {
	// AutoGenerate generates a self-signed certificate via an init container.
	// +kubebuilder:default=true
	AutoGenerate bool `json:"autoGenerate,omitempty"`

	// ExistingSecret is the name of a kubernetes.io/tls Secret with keys tls.crt and tls.key.
	// When set, AutoGenerate is ignored.
	// +optional
	ExistingSecret string `json:"existingSecret,omitempty"`
}

// ScutumHubWireGuard holds WireGuard configuration for the hub.
type ScutumHubWireGuard struct {
	// Enabled controls whether WireGuard is configured on this hub.
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// Port is the UDP listen port for WireGuard.
	// +kubebuilder:default=51820
	Port int32 `json:"port,omitempty"`

	// ServiceType is the Kubernetes Service type for the WireGuard endpoint.
	// +kubebuilder:default="LoadBalancer"
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`
}

// ScutumHubStorage holds PVC size configuration.
type ScutumHubStorage struct {
	// DataSize is the size of the data PVC (SQLite database).
	// +kubebuilder:default="5Gi"
	DataSize string `json:"dataSize,omitempty"`

	// SecretsSize is the size of the secrets PVC.
	// +kubebuilder:default="256Mi"
	SecretsSize string `json:"secretsSize,omitempty"`

	// StorageClass is the storage class for both PVCs. Defaults to cluster default.
	// +optional
	StorageClass string `json:"storageClass,omitempty"`
}

// ScutumHubStatus defines the observed state of ScutumHub.
type ScutumHubStatus struct {
	// Ready is true when the hub StatefulSet has at least one ready replica.
	Ready bool `json:"ready,omitempty"`

	// Phase is the current lifecycle phase: Pending, Running, or Error.
	Phase string `json:"phase,omitempty"`

	// APIEndpoint is the in-cluster HTTPS URL of the hub API.
	APIEndpoint string `json:"apiEndpoint,omitempty"`

	// WireGuardEndpoint is the external IP:port assigned to the WireGuard LoadBalancer Service.
	// Populated once the cloud provider assigns an external IP.
	WireGuardEndpoint string `json:"wireGuardEndpoint,omitempty"`

	// Conditions contains standard metav1 conditions for the hub.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation last processed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.ready`
// +kubebuilder:printcolumn:name="API Endpoint",type=string,JSONPath=`.status.apiEndpoint`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ScutumHub is the Schema for the scutumhubs API.
type ScutumHub struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScutumHubSpec   `json:"spec,omitempty"`
	Status ScutumHubStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ScutumHubList contains a list of ScutumHub.
type ScutumHubList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScutumHub `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ScutumHub{}, &ScutumHubList{})
}
