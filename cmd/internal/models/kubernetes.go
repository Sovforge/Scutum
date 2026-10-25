package models

type KubernetesResurce struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   Meta   `json:"metadata"`
}

type Meta struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	// ResourceVersion round-trips through GET-modify-PUT flows (scale,
	// restart, secret rotate). Without it, the API server rejects the PUT
	// as a conflicting update on a real cluster.
	ResourceVersion string `json:"resourceVersion,omitempty"`
}

type Pod struct {
	KubernetesResurce
	Spec PodSpec `json:"spec"`
}

type PodSpec struct {
	Containers    []Container `json:"containers"`
	RestartPolicy string      `json:"restartPolicy,omitempty"`
}

type Container struct {
	Name  string          `json:"name"`
	Image string          `json:"image"`
	Env   []EnvVar        `json:"env,omitempty"`
	Ports []ContainerPort `json:"ports,omitempty"`
}

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ContainerPort struct {
	ContainerPort int `json:"containerPort"`
}

type Deployment struct {
	KubernetesResurce
	Spec DeploymentSpec `json:"spec"`
}

type DeploymentSpec struct {
	Replicas int           `json:"replicas"`
	Selector LabelSelector `json:"selector"`
	Template PodTemplate   `json:"template"`
}

type LabelSelector struct {
	MatchLabels map[string]string `json:"matchLabels"`
}

type Secret struct {
	KubernetesResurce
	Type string `json:"type,omitempty"`
	// Data holds base64-encoded values (what the API returns on GET).
	// StringData holds plaintext values and is only ever sent on
	// create/update — the API server base64-encodes it into Data itself.
	Data       map[string]string `json:"data,omitempty"`
	StringData map[string]string `json:"stringData,omitempty"`
}

type PodTemplate struct {
	Metadata Meta    `json:"metadata"`
	Spec     PodSpec `json:"spec"`
}
