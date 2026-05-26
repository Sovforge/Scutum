package controller

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	scutumv1alpha1 "github.com/sovforge/scutum-operator/api/v1alpha1"
	"github.com/sovforge/scutum-operator/internal/hubclient"
)

// ScutumNodeReconciler reconciles ScutumNode objects.
type ScutumNodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=scutum.io,resources=scutumnodes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=scutum.io,resources=scutumnodes/status,verbs=update;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps;secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is the main reconciliation loop for ScutumNode.
func (r *ScutumNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	node := &scutumv1alpha1.ScutumNode{}
	if err := r.Get(ctx, req.NamespacedName, node); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger.Info("reconciling ScutumNode", "name", node.Name, "namespace", node.Namespace)

	// 1. Look up referenced ScutumHub
	hub := &scutumv1alpha1.ScutumHub{}
	if err := r.Get(ctx, types.NamespacedName{Name: node.Spec.HubRef.Name, Namespace: node.Namespace}, hub); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("ScutumHub not found, requeuing", "hub", node.Spec.HubRef.Name)
			return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	if !hub.Status.Ready {
		logger.Info("ScutumHub not ready, requeuing", "hub", hub.Name)
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	// 2. Enrollment phase — only when NodeID is not yet known
	if node.Status.NodeID == "" {
		result, err := r.enroll(ctx, node, hub)
		if err != nil {
			return r.setNodeError(ctx, node, "Enrollment", err)
		}
		if result.RequeueAfter > 0 {
			return result, nil
		}
	}

	image := node.Spec.Image
	if image == "" {
		image = "ghcr.io/sovforge/scutum:latest"
	}

	// 3. ConfigMap for edge node
	if err := r.reconcileNodeConfigMap(ctx, node); err != nil {
		return r.setNodeError(ctx, node, "ConfigMap", err)
	}

	// 4. ClusterIP Service for edge node API
	if err := r.reconcileNodeService(ctx, node); err != nil {
		return r.setNodeError(ctx, node, "Service", err)
	}

	// 5. StatefulSet
	if err := r.reconcileNodeStatefulSet(ctx, node, image); err != nil {
		return r.setNodeError(ctx, node, "StatefulSet", err)
	}

	// 6. Check StatefulSet readiness
	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: node.Name, Namespace: node.Namespace}, sts); err != nil {
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	stsReady := sts.Status.ReadyReplicas > 0

	// 7. If StatefulSet is ready and we are in Enrolling phase, perform setup call
	if stsReady && node.Status.Phase == "Enrolling" {
		if err := r.setupEdgeNode(ctx, node, hub); err != nil {
			// Setup failed — log and requeue; don't error-out permanently
			logger.Error(err, "edge setup call failed, will retry")
			return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
		}
	}

	// 8. Update status
	phase := node.Status.Phase
	ready := false
	if phase == "Running" {
		ready = true
	} else if stsReady && phase == "Configuring" {
		// Poll hub to see if node address is no longer "pending"
		if r.checkNodeConnected(ctx, node, hub) {
			phase = "Running"
			ready = true
		}
	}

	patch := client.MergeFrom(node.DeepCopy())
	node.Status.Ready = ready
	node.Status.Phase = phase
	node.Status.ObservedGeneration = node.Generation
	if err := r.Status().Patch(ctx, node, patch); err != nil {
		return ctrl.Result{}, err
	}

	if !ready {
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

// SetupWithManager registers the controller with the manager.
func (r *ScutumNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&scutumv1alpha1.ScutumNode{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}

// enroll registers the node with the hub and creates the bootstrap secret.
func (r *ScutumNodeReconciler) enroll(ctx context.Context, node *scutumv1alpha1.ScutumNode, hub *scutumv1alpha1.ScutumHub) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Read admin credentials from hub.Spec.AdminSecret (in hub's namespace)
	adminSecret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      hub.Spec.AdminSecret,
		Namespace: hub.Namespace,
	}, adminSecret); err != nil {
		return ctrl.Result{}, fmt.Errorf("get admin secret %s: %w", hub.Spec.AdminSecret, err)
	}
	username := string(adminSecret.Data["username"])
	password := string(adminSecret.Data["password"])
	if username == "" || password == "" {
		return ctrl.Result{}, fmt.Errorf("admin secret %s missing username or password keys", hub.Spec.AdminSecret)
	}

	hubClient := hubclient.New(true) // always skip verify for operator-internal calls
	apiBase := hub.Status.APIEndpoint

	// Login
	token, err := hubClient.Login(ctx, apiBase, username, password)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("hub login: %w", err)
	}

	// Check if node already exists (idempotent re-enrollment)
	existingNodes, err := hubClient.GetNodes(ctx, apiBase, token)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("get nodes: %w", err)
	}
	nodeID := ""
	for _, n := range existingNodes {
		if n.Name == node.Spec.NodeName {
			nodeID = n.ID
			logger.Info("found existing hub node, reusing", "nodeID", nodeID)
			break
		}
	}

	if nodeID == "" {
		meshAddr := node.Spec.MeshAddress
		if meshAddr == "" {
			meshAddr = "pending"
		}
		nodeType := node.Spec.NodeType
		if nodeType == "" {
			nodeType = "remote"
		}
		created, err := hubClient.CreateNode(ctx, apiBase, token, hubclient.CreateNodeRequest{
			Name:      node.Spec.NodeName,
			Type:      nodeType,
			Address:   meshAddr,
			PublicKey: "pending",
		})
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("create node: %w", err)
		}
		nodeID = created.ID
		logger.Info("registered new hub node", "nodeID", nodeID)
	}

	// Generate random 32-byte edge sync token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return ctrl.Result{}, fmt.Errorf("generate edge token: %w", err)
	}
	edgeToken := hex.EncodeToString(tokenBytes)

	// RegisterEdge — sync URL is empty; the operator doesn't know the edge URL yet
	if err := hubClient.RegisterEdge(ctx, apiBase, token, nodeID, "", edgeToken); err != nil {
		return ctrl.Result{}, fmt.Errorf("register edge: %w", err)
	}

	// GetBootstrap
	bootstrap, err := hubClient.GetBootstrap(ctx, apiBase, token)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("get bootstrap: %w", err)
	}

	tlsSkipVerify := "true"
	if !node.Spec.TLSSkipVerify {
		tlsSkipVerify = "false"
	}

	// Create bootstrap secret
	secretName := node.Name + "-bootstrap"
	bootstrapSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: node.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, bootstrapSecret, func() error {
		bootstrapSecret.StringData = map[string]string{
			"hub_api_base":        apiBase,
			"hub_wg_public_key":   bootstrap.HubWGPublicKey,
			"hub_wg_port":         fmt.Sprintf("%d", bootstrap.HubWGPort),
			"hub_hmac_key":        bootstrap.HubHMACKey,
			"hub_mesh_cidr":       bootstrap.HubMeshCIDR,
			"hub_tls_skip_verify": tlsSkipVerify,
			"edge_sync_token":     edgeToken,
		}
		return controllerutil.SetControllerReference(node, bootstrapSecret, r.Scheme)
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("create bootstrap secret: %w", err)
	}

	// Update status
	patch := client.MergeFrom(node.DeepCopy())
	node.Status.NodeID = nodeID
	node.Status.Phase = "Enrolling"
	if err := r.Status().Patch(ctx, node, patch); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// setupEdgeNode calls POST /api/setup on the edge node to configure it.
func (r *ScutumNodeReconciler) setupEdgeNode(ctx context.Context, node *scutumv1alpha1.ScutumNode, hub *scutumv1alpha1.ScutumHub) error {
	logger := log.FromContext(ctx)

	// Read bootstrap secret
	secretName := node.Name + "-bootstrap"
	bootstrapSecret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: node.Namespace}, bootstrapSecret); err != nil {
		return fmt.Errorf("get bootstrap secret: %w", err)
	}

	hubAPIBase := string(bootstrapSecret.Data["hub_api_base"])
	hubWGPublicKey := string(bootstrapSecret.Data["hub_wg_public_key"])
	hubHMACKey := string(bootstrapSecret.Data["hub_hmac_key"])
	hubMeshCIDR := string(bootstrapSecret.Data["hub_mesh_cidr"])
	hubWGEndpoint := hub.Status.WireGuardEndpoint

	if hubWGEndpoint == "" {
		return fmt.Errorf("hub WireGuard endpoint not yet assigned")
	}

	// Edge node service URL — use the in-cluster headless pod DNS
	edgeSvcName := node.Name
	edgeURL := fmt.Sprintf("http://%s-0.%s.%s.svc.cluster.local:8080",
		node.Name, edgeSvcName, node.Namespace)

	// First check: is the edge node's API up?
	if !r.isEdgeHealthy(ctx, edgeURL) {
		logger.Info("edge node API not yet healthy, will retry", "url", edgeURL)
		return fmt.Errorf("edge API not ready")
	}

	// Check if setup is already done
	if r.isEdgeSetupDone(ctx, edgeURL) {
		logger.Info("edge node setup already complete")
		patch := client.MergeFrom(node.DeepCopy())
		node.Status.Phase = "Configuring"
		_ = r.Status().Patch(ctx, node, patch)
		return nil
	}

	nodeType := node.Spec.NodeType
	if nodeType == "" {
		nodeType = "remote"
	}

	setupPayload := map[string]interface{}{
		"install_type": nodeType,
		"wireguard": map[string]interface{}{
			"hub_endpoint":    hubWGEndpoint,
			"hub_public_key":  hubWGPublicKey,
			"hub_allowed_ips": hubMeshCIDR,
			"hub_hmac_key":    hubHMACKey,
			"address":         node.Spec.MeshAddress,
		},
		"hub_api_base":         hubAPIBase,
		"hub_tls_skip_verify":  node.Spec.TLSSkipVerify,
	}

	payloadBytes, _ := json.Marshal(setupPayload)

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Post(edgeURL+"/api/setup", "application/json",
		strings.NewReader(string(payloadBytes)))
	if err != nil {
		return fmt.Errorf("setup POST: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("setup returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	logger.Info("edge node setup call succeeded", "status", resp.StatusCode)

	patch := client.MergeFrom(node.DeepCopy())
	node.Status.Phase = "Configuring"
	return r.Status().Patch(ctx, node, patch)
}

// isEdgeHealthy returns true when GET /api/health returns 200.
func (r *ScutumNodeReconciler) isEdgeHealthy(ctx context.Context, edgeURL string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, edgeURL+"/api/health", nil)
	if err != nil {
		return false
	}
	c := &http.Client{Timeout: 5 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// isEdgeSetupDone returns true when the edge reports setup is complete.
func (r *ScutumNodeReconciler) isEdgeSetupDone(ctx context.Context, edgeURL string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, edgeURL+"/api/setup/status", nil)
	if err != nil {
		return false
	}
	c := &http.Client{Timeout: 5 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var result struct {
		Complete bool `json:"complete"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}
	return result.Complete
}

// checkNodeConnected polls the hub to see if the node's address is no longer "pending".
func (r *ScutumNodeReconciler) checkNodeConnected(ctx context.Context, node *scutumv1alpha1.ScutumNode, hub *scutumv1alpha1.ScutumHub) bool {
	adminSecret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      hub.Spec.AdminSecret,
		Namespace: hub.Namespace,
	}, adminSecret); err != nil {
		return false
	}
	username := string(adminSecret.Data["username"])
	password := string(adminSecret.Data["password"])

	hubCl := hubclient.New(true)
	token, err := hubCl.Login(ctx, hub.Status.APIEndpoint, username, password)
	if err != nil {
		return false
	}
	nodes, err := hubCl.GetNodes(ctx, hub.Status.APIEndpoint, token)
	if err != nil {
		return false
	}
	for _, n := range nodes {
		if n.ID == node.Status.NodeID {
			return n.Address != "pending" && n.Address != ""
		}
	}
	return false
}

// --- sub-reconcilers for edge node resources ---

func (r *ScutumNodeReconciler) reconcileNodeConfigMap(ctx context.Context, node *scutumv1alpha1.ScutumNode) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      node.Name,
			Namespace: node.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
		cm.Data = map[string]string{
			"DATA_DIR":    "/data",
			"SECRETS_DIR": "/secrets",
			"PORT":        "8080",
			"LOG_LEVEL":   "info",
		}
		return controllerutil.SetControllerReference(node, cm, r.Scheme)
	})
	return err
}

func (r *ScutumNodeReconciler) reconcileNodeService(ctx context.Context, node *scutumv1alpha1.ScutumNode) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      node.Name,
			Namespace: node.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Spec.Type = corev1.ServiceTypeClusterIP
		svc.Spec.ClusterIP = corev1.ClusterIPNone // headless for stable DNS
		svc.Spec.Selector = map[string]string{"app": node.Name}
		svc.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "api",
				Port:       8080,
				TargetPort: intstr.FromInt(8080),
				Protocol:   corev1.ProtocolTCP,
			},
		}
		return controllerutil.SetControllerReference(node, svc, r.Scheme)
	})
	return err
}

func (r *ScutumNodeReconciler) reconcileNodeStatefulSet(ctx context.Context, node *scutumv1alpha1.ScutumNode, image string) error {
	replicas := int32(1)

	privEsc := true
	secCtx := &corev1.SecurityContext{
		RunAsUser: int64Ptr(0),
		Capabilities: &corev1.Capabilities{
			Add:  []corev1.Capability{"NET_ADMIN"},
			Drop: []corev1.Capability{"ALL"},
		},
		AllowPrivilegeEscalation: &privEsc,
	}

	envFrom := []corev1.EnvFromSource{
		{
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: node.Name},
			},
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "data", MountPath: "/data"},
		{Name: "secrets", MountPath: "/secrets"},
		{Name: "dev-net-tun", MountPath: "/dev/net/tun"},
	}

	volumes := []corev1.Volume{
		{
			Name: "dev-net-tun",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/dev/net/tun",
					Type: hostPathTypePtr(corev1.HostPathCharDev),
				},
			},
		},
	}

	// TLS init container
	initContainers := []corev1.Container{
		{
			Name:  "tls-init",
			Image: "alpine:3.21",
			Command: []string{"sh", "-c", fmt.Sprintf(`
if [ -f /secrets/server.crt ] && [ -f /secrets/server.key ]; then
  echo "TLS cert already present, skipping generation."
  exit 0
fi
apk add --no-cache openssl >/dev/null 2>&1
openssl req -x509 -newkey rsa:4096 \
  -keyout /secrets/server.key \
  -out /secrets/server.crt \
  -days 3650 -nodes \
  -subj "/CN=%s" \
  -addext "subjectAltName=DNS:%s,DNS:%s.%s.svc.cluster.local,IP:127.0.0.1"
echo "TLS cert generated."
`, node.Name, node.Name, node.Name, node.Namespace)},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "secrets", MountPath: "/secrets"},
			},
		},
	}

	// Probes
	livenessProbe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/api/health",
				Port:   intstr.FromString("api"),
				Scheme: corev1.URISchemeHTTP,
			},
		},
		InitialDelaySeconds: 15,
		PeriodSeconds:       30,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
	readinessProbe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/api/health",
				Port:   intstr.FromString("api"),
				Scheme: corev1.URISchemeHTTP,
			},
		},
		InitialDelaySeconds: 10,
		PeriodSeconds:       10,
		TimeoutSeconds:      3,
		FailureThreshold:    3,
	}

	dataSize := node.Spec.Storage.DataSize
	if dataSize == "" {
		dataSize = "2Gi"
	}
	secretsSize := node.Spec.Storage.SecretsSize
	if secretsSize == "" {
		secretsSize = "256Mi"
	}

	dataPVC := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "data"},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(dataSize),
				},
			},
		},
	}
	secretsPVC := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "secrets"},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(secretsSize),
				},
			},
		},
	}
	if node.Spec.Storage.StorageClass != "" {
		dataPVC.Spec.StorageClassName = &node.Spec.Storage.StorageClass
		secretsPVC.Spec.StorageClassName = &node.Spec.Storage.StorageClass
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      node.Name,
			Namespace: node.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, sts, func() error {
		sts.Spec.Replicas = &replicas
		sts.Spec.ServiceName = node.Name
		sts.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": node.Name},
		}
		sts.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": node.Name},
			},
			Spec: corev1.PodSpec{
				SecurityContext: &corev1.PodSecurityContext{
					RunAsUser:  int64Ptr(0),
					RunAsGroup: int64Ptr(0),
					FSGroup:    int64Ptr(0),
				},
				InitContainers: initContainers,
				Containers: []corev1.Container{
					{
						Name:            "scutum",
						Image:           image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Ports: []corev1.ContainerPort{
							{Name: "api", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
						},
						EnvFrom:         envFrom,
						VolumeMounts:    volumeMounts,
						SecurityContext: secCtx,
						LivenessProbe:   livenessProbe,
						ReadinessProbe:  readinessProbe,
						Resources:       node.Spec.Resources,
					},
				},
				Volumes: volumes,
			},
		}
		sts.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{dataPVC, secretsPVC}
		return controllerutil.SetControllerReference(node, sts, r.Scheme)
	})
	return err
}

// setNodeError updates the node status to Error and returns a requeue result.
func (r *ScutumNodeReconciler) setNodeError(ctx context.Context, node *scutumv1alpha1.ScutumNode, component string, err error) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Error(err, "reconcile error", "component", component)
	patch := client.MergeFrom(node.DeepCopy())
	node.Status.Phase = "Error"
	_ = r.Status().Patch(ctx, node, patch)
	return ctrl.Result{RequeueAfter: 15 * time.Second}, err
}
