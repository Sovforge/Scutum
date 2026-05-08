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

type PodTemplate struct {
	Metadata Meta    `json:"metadata"`
	Spec     PodSpec `json:"spec"`
}
