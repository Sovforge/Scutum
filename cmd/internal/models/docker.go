package models

// DeployRequest is what YOUR API receives from the user/frontend.
type DeployRequest struct {
	Repo        string   `json:"repo"`         // e.g., "nginx:latest"
	Name        string   `json:"name"`         // Unique container name
	Cmd         []string `json:"cmd"`          // Optional command override
	Port        int      `json:"port"`         // Internal port (e.g., 80)
	HostPort    int      `json:"host_port"`    // External port (e.g., 8080)
	Env         []string `json:"env"`          // e.g., ["DEBUG=true"]
	Volumes     []string `json:"volumes"`      // e.g., ["/host:/container:rw"]
	MemoryLimit int64    `json:"memory_limit"` // In bytes
	CPULimit    float64  `json:"cpu_limit"`    // e.g., 0.5 for half a core
	Restart     string   `json:"restart"`      // "always", "no", etc.
}

// ContainerCreateConfig is what YOU send to the DOCKER API.
type ContainerCreateConfig struct {
	Hostname     string              `json:"Hostname,omitempty"`
	Domainname   string              `json:"Domainname,omitempty"`
	User         string              `json:"User,omitempty"`
	Tty          bool                `json:"Tty,omitempty"`
	Env          []string            `json:"Env,omitempty"`
	Cmd          []string            `json:"Cmd,omitempty"`
	Image        string              `json:"Image"`
	Labels       map[string]string   `json:"Labels,omitempty"`
	ExposedPorts map[string]struct{} `json:"ExposedPorts,omitempty"`
	HostConfig   HostConfig          `json:"HostConfig,omitempty"`
}

type HostConfig struct {
	Binds         []string                 `json:"Binds,omitempty"`
	Memory        int64                    `json:"Memory,omitempty"`
	NanoCpus      int64                    `json:"NanoCpus,omitempty"`
	PortBindings  map[string][]PortBinding `json:"PortBindings,omitempty"`
	RestartPolicy RestartPolicy            `json:"RestartPolicy,omitempty"`
	AutoRemove    bool                     `json:"AutoRemove,omitempty"`
	NetworkMode   string                   `json:"NetworkMode,omitempty"`
	Privileged    bool                     `json:"Privileged,omitempty"`
}

type PortBinding struct {
	HostPort string `json:"HostPort"`
}

type RestartPolicy struct {
	Name              string `json:"Name"`
	MaximumRetryCount int    `json:"MaximumRetryCount"`
}

type DeployResponse struct {
	ID      string `json:"id,omitempty"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}
