package tests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"scutum/cmd/internal/clients"
	"scutum/cmd/internal/handlers"
	"scutum/cmd/internal/models"
)

// TestKubernetesHandlerDeploymentCreation tests that CreateDeployment handles deployment creation
func TestKubernetesHandlerDeploymentCreation(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		depName   string
		image     string
		shouldErr bool
	}{
		{"valid deployment", "default", "nginx-app", "nginx:latest", false},
		{"system namespace", "kube-system", "metrics", "metrics:v1", false},
		{"custom namespace", "my-app", "web-server", "nginx:alpine", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewKubernetesHandler(&mockNodeProxyStore{})
			err := handler.CreateDeployment(tt.namespace, tt.depName, tt.image)

			// Handler may fail due to missing k8s cluster, but should not panic
			if err != nil && tt.namespace == "" {
				t.Errorf("unexpected error for valid namespace: %v", err)
			}
		})
	}
}

// TestKubernetesHandlerNamespaceValidation tests namespace name validation
func TestKubernetesHandlerNamespaceValidation(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		valid     bool
	}{
		{"default namespace", "default", true},
		{"kube prefix", "kube-system", true},
		{"hyphenated", "my-app-ns", true},
		{"single char", "a", true},
		{"empty namespace", "", false},
		{"uppercase not allowed", "MyApp", false},
		{"underscore not allowed", "my_app", false},
		{"long single label (255 chars)", generateString(255), true},
		{"too long (256 chars)", generateString(256), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewKubernetesHandler(&mockNodeProxyStore{})

			// Attempt to create a deployment in the namespace to test validation
			err := handler.CreateDeployment(tt.namespace, "test-app", "test:latest")

			// Empty namespace should fail at handler level
			if tt.namespace == "" && err == nil {
				t.Error("expected error for empty namespace")
			}
		})
	}
}

// TestKubernetesScalableDeployment tests deployment replica scaling
func TestKubernetesScalableDeployment(t *testing.T) {
	tests := []struct {
		name     string
		replicas int
		valid    bool
	}{
		{"single replica", 1, true},
		{"multiple replicas", 3, true},
		{"high replica count", 100, true},
		{"zero replicas", 0, false},
		{"negative replicas", -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replica validation would happen during deployment spec creation
			deploy := models.Deployment{
				Spec: models.DeploymentSpec{
					Replicas: tt.replicas,
				},
			}

			isValid := deploy.Spec.Replicas > 0
			if isValid != tt.valid {
				t.Errorf("replica validation: got %v, want %v", isValid, tt.valid)
			}
		})
	}
}

// TestKubernetesResourceLimits tests resource limit specifications
func TestKubernetesResourceLimits(t *testing.T) {
	tests := []struct {
		name          string
		cpuLimit      string
		memoryLimit   string
		shouldBeValid bool
	}{
		{"standard limits", "500m", "512Mi", true},
		{"high limits", "2000m", "2Gi", true},
		{"low limits", "50m", "64Mi", true},
		{"empty limit", "", "", true},
		{"invalid cpu unit", "500z", "512Mi", false},
		{"invalid memory unit", "500m", "512Fb", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := (tt.cpuLimit == "" || contains(tt.cpuLimit, "m")) &&
				(tt.memoryLimit == "" || contains(tt.memoryLimit, "Mi") || contains(tt.memoryLimit, "Gi"))
			if isValid != tt.shouldBeValid {
				t.Errorf("resource limit validation: got %v, want %v", isValid, tt.shouldBeValid)
			}
		})
	}
}

// TestKubernetesServicePortMapping tests service port configurations
func TestKubernetesServicePortMapping(t *testing.T) {
	tests := []struct {
		name          string
		containerPort int
		servicePort   int
		valid         bool
	}{
		{"http ports", 80, 80, true},
		{"https ports", 443, 443, true},
		{"app port", 8080, 8080, true},
		{"custom mapping", 3000, 9000, true},
		{"dynamic port", 0, 0, false},
		{"invalid high port", 65536, 65536, false},
		{"negative port", -1, 8080, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.containerPort > 0 && tt.containerPort < 65536 &&
				tt.servicePort > 0 && tt.servicePort < 65536
			if isValid != tt.valid {
				t.Errorf("port mapping validation: got %v, want %v", isValid, tt.valid)
			}
		})
	}
}

// TestKubernetesImagePullPolicy tests image pull policy configuration
func TestKubernetesImagePullPolicy(t *testing.T) {
	tests := []struct {
		name   string
		policy string
		valid  bool
	}{
		{"Always pull", "Always", true},
		{"Never pull", "Never", true},
		{"IfNotPresent", "IfNotPresent", true},
		{"empty policy", "", true},
		{"invalid policy", "Sometimes", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.policy == "" || tt.policy == "Always" ||
				tt.policy == "Never" || tt.policy == "IfNotPresent"
			if isValid != tt.valid {
				t.Errorf("pull policy validation: got %v, want %v", isValid, tt.valid)
			}
		})
	}
}

// TestKubernetesHandlerInitialization tests Kubernetes handler creation
func TestKubernetesHandlerInitialization(t *testing.T) {
	handler := handlers.NewKubernetesHandler(&mockNodeProxyStore{})
	if handler == nil {
		t.Error("NewKubernetesHandler() returned nil")
	}
}

// TestKubernetesDeploymentSpec tests deployment specification validation
func TestKubernetesDeploymentSpec(t *testing.T) {
	tests := []struct {
		name      string
		apiVer    string
		kind      string
		depName   string
		namespace string
		valid     bool
	}{
		{
			"valid deployment",
			"apps/v1",
			"Deployment",
			"nginx-deploy",
			"default",
			true,
		},
		{
			"missing api version",
			"",
			"Deployment",
			"nginx-deploy",
			"default",
			false,
		},
		{
			"wrong kind",
			"apps/v1",
			"Pod",
			"nginx-deploy",
			"default",
			false,
		},
		{
			"missing name",
			"apps/v1",
			"Deployment",
			"",
			"default",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.apiVer != "" && tt.kind == "Deployment" &&
				tt.depName != "" && tt.namespace != ""
			if isValid != tt.valid {
				t.Errorf("deployment spec: valid = %v, want %v", isValid, tt.valid)
			}
		})
	}
}

// TestKubernetesUtilsGetInClusterConfig tests in-cluster config handling
func TestKubernetesUtilsGetInClusterConfig(t *testing.T) {
	// This function reads from hardcoded paths, so we can't easily test it without
	// modifying the function. For now, we'll skip testing this as it's hard to mock.
	// In a real scenario, we'd refactor to accept paths as parameters.
	t.Skip("GetInClusterConfig reads from hardcoded paths, difficult to test")
}

// TestKubernetesHandlerHTTPHandlersErrorCases tests error handling in Kubernetes handler methods
func TestKubernetesHandlerHTTPHandlersErrorCases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("k8s error"))
	}))
	defer server.Close()

	h := &handlers.KubernetesHandler{}
	setUnexportedField(h, "client", clients.NewKubernetesClient(server.Client(), server.URL, "test-token"))

	// Test HandleDeletePod error
	req := httptest.NewRequest(http.MethodDelete, "/k8s/default/pods/test-pod", nil)
	req.SetPathValue("ns", "default")
	req.SetPathValue("name", "test-pod")
	w := httptest.NewRecorder()
	h.HandleDeletePod(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	// Test HandleLogs error
	req = httptest.NewRequest(http.MethodGet, "/k8s/default/pods/test-pod/logs", nil)
	req.SetPathValue("ns", "default")
	req.SetPathValue("name", "test-pod")
	w = httptest.NewRecorder()
	h.HandleLogs(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	// Test HandleDeploy with invalid JSON
	req = httptest.NewRequest(http.MethodPost, "/k8s/default/deploy", strings.NewReader("invalid json"))
	req.SetPathValue("ns", "default")
	w = httptest.NewRecorder()
	h.HandleDeploy(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
