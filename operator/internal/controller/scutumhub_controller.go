// Package controller implements the ScutumHub and ScutumNode reconcilers.
package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
)

// ScutumHubReconciler reconciles ScutumHub objects.
type ScutumHubReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=scutum.io,resources=scutumhubs,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=scutum.io,resources=scutumhubs/status,verbs=update;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps;serviceaccounts;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete

// Reconcile is the main reconciliation loop for ScutumHub.
func (r *ScutumHubReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	hub := &scutumv1alpha1.ScutumHub{}
	if err := r.Get(ctx, req.NamespacedName, hub); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Set initial phase on fresh objects.
	if hub.Status.Phase == "" {
		hub.Status.Phase = "Pending"
		if err := r.Status().Update(ctx, hub); err != nil {
			return ctrl.Result{}, err
		}
	}

	logger.Info("reconciling ScutumHub", "name", hub.Name, "namespace", hub.Namespace)

	image := hub.Spec.Image
	if image == "" {
		image = "ghcr.io/sovforge/scutum:latest"
	}

	// 1. ServiceAccount
	if err := r.reconcileServiceAccount(ctx, hub); err != nil {
		return r.setError(ctx, hub, "ServiceAccount", err)
	}

	// 2. ClusterRole + ClusterRoleBinding
	if err := r.reconcileClusterRole(ctx, hub); err != nil {
		return r.setError(ctx, hub, "ClusterRole", err)
	}

	// 3. ConfigMap
	if err := r.reconcileConfigMap(ctx, hub); err != nil {
		return r.setError(ctx, hub, "ConfigMap", err)
	}

	// 4. API Service (ClusterIP)
	if err := r.reconcileAPIService(ctx, hub); err != nil {
		return r.setError(ctx, hub, "APIService", err)
	}

	// 5. WireGuard Service (LoadBalancer) — only if enabled
	var wgSvc *corev1.Service
	if hub.Spec.WireGuard.Enabled {
		svc, err := r.reconcileWireGuardService(ctx, hub)
		if err != nil {
			return r.setError(ctx, hub, "WireGuardService", err)
		}
		wgSvc = svc
	}

	// 6. StatefulSet
	if err := r.reconcileStatefulSet(ctx, hub, image); err != nil {
		return r.setError(ctx, hub, "StatefulSet", err)
	}

	// 7. Check readiness
	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: hub.Name, Namespace: hub.Namespace}, sts); err != nil {
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	ready := sts.Status.ReadyReplicas > 0

	// 8. Derive endpoints
	apiEndpoint := fmt.Sprintf("https://%s.%s.svc.cluster.local:8080", hub.Name, hub.Namespace)
	wgEndpoint := ""
	if wgSvc != nil {
		for _, ing := range wgSvc.Status.LoadBalancer.Ingress {
			ip := ing.IP
			if ip == "" {
				ip = ing.Hostname
			}
			if ip != "" {
				wgPort := hub.Spec.WireGuard.Port
				if wgPort == 0 {
					wgPort = 51820
				}
				wgEndpoint = fmt.Sprintf("%s:%d", ip, wgPort)
				break
			}
		}
		if wgEndpoint == "" {
			// LB not yet assigned; requeue to poll
			logger.Info("waiting for WireGuard LB IP", "svc", wgSvc.Name)
		}
	}

	// 9. Update status
	phase := "Pending"
	if ready {
		phase = "Running"
	}

	patch := client.MergeFrom(hub.DeepCopy())
	hub.Status.Ready = ready
	hub.Status.Phase = phase
	hub.Status.APIEndpoint = apiEndpoint
	hub.Status.WireGuardEndpoint = wgEndpoint
	hub.Status.ObservedGeneration = hub.Generation
	if err := r.Status().Patch(ctx, hub, patch); err != nil {
		return ctrl.Result{}, err
	}

	if !ready || (hub.Spec.WireGuard.Enabled && wgEndpoint == "") {
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager registers the controller with the manager.
func (r *ScutumHubReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&scutumv1alpha1.ScutumHub{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.ServiceAccount{}).
		Complete(r)
}

// setError updates the hub status to Error and returns a requeue result.
func (r *ScutumHubReconciler) setError(ctx context.Context, hub *scutumv1alpha1.ScutumHub, component string, err error) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Error(err, "reconcile error", "component", component)
	patch := client.MergeFrom(hub.DeepCopy())
	hub.Status.Phase = "Error"
	_ = r.Status().Patch(ctx, hub, patch)
	return ctrl.Result{RequeueAfter: 15 * time.Second}, err
}

// --- sub-reconcilers ---

func (r *ScutumHubReconciler) reconcileServiceAccount(ctx context.Context, hub *scutumv1alpha1.ScutumHub) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hub.Name,
			Namespace: hub.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, sa, func() error {
		return controllerutil.SetControllerReference(hub, sa, r.Scheme)
	})
	return err
}

func (r *ScutumHubReconciler) reconcileClusterRole(ctx context.Context, hub *scutumv1alpha1.ScutumHub) error {
	crName := "scutum-hub-" + hub.Namespace + "-" + hub.Name
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: crName,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, cr, func() error {
		cr.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{
					"namespaces", "nodes", "pods", "pods/log", "pods/exec",
					"services", "endpoints", "persistentvolumeclaims",
					"configmaps", "secrets", "serviceaccounts",
				},
				Verbs: []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "statefulsets", "daemonsets", "replicasets"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
			{
				APIGroups: []string{"batch"},
				Resources: []string{"jobs", "cronjobs"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
			{
				APIGroups: []string{"networking.k8s.io"},
				Resources: []string{"ingresses"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
			{
				APIGroups: []string{"metrics.k8s.io"},
				Resources: []string{"pods", "nodes"},
				Verbs:     []string{"get", "list"},
			},
		}
		return nil
	})
	if err != nil {
		return err
	}

	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: crName,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, crb, func() error {
		crb.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     crName,
		}
		crb.Subjects = []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      hub.Name,
				Namespace: hub.Namespace,
			},
		}
		return nil
	})
	return err
}

func (r *ScutumHubReconciler) reconcileConfigMap(ctx context.Context, hub *scutumv1alpha1.ScutumHub) error {
	logLevel := hub.Spec.LogLevel
	if logLevel == "" {
		logLevel = "info"
	}
	auditEnabled := "true"
	if !hub.Spec.AuditEnabled {
		auditEnabled = "false"
	}

	certFile := "/secrets/server.crt"
	keyFile := "/secrets/server.key"
	if hub.Spec.TLS.ExistingSecret != "" {
		certFile = "/secrets/tls.crt"
		keyFile = "/secrets/tls.key"
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hub.Name,
			Namespace: hub.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
		cm.Data = map[string]string{
			"PORT":                 "8080",
			"LOG_LEVEL":            logLevel,
			"AUDIT_ENABLED":        auditEnabled,
			"AUDIT_RETENTION_DAYS": "365",
			"HEALER_INTERVAL":      "30s",
			"DATA_DIR":             "/data",
			"SECRETS_DIR":          "/secrets",
			"CERT_FILE":            certFile,
			"KEY_FILE":             keyFile,
		}
		return controllerutil.SetControllerReference(hub, cm, r.Scheme)
	})
	return err
}

func (r *ScutumHubReconciler) reconcileAPIService(ctx context.Context, hub *scutumv1alpha1.ScutumHub) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hub.Name,
			Namespace: hub.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Spec.Type = corev1.ServiceTypeClusterIP
		svc.Spec.Selector = map[string]string{"app": hub.Name}
		svc.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "api",
				Port:       8080,
				TargetPort: intstr.FromInt(8080),
				Protocol:   corev1.ProtocolTCP,
			},
		}
		return controllerutil.SetControllerReference(hub, svc, r.Scheme)
	})
	return err
}

func (r *ScutumHubReconciler) reconcileWireGuardService(ctx context.Context, hub *scutumv1alpha1.ScutumHub) (*corev1.Service, error) {
	wgPort := hub.Spec.WireGuard.Port
	if wgPort == 0 {
		wgPort = 51820
	}
	svcType := hub.Spec.WireGuard.ServiceType
	if svcType == "" {
		svcType = corev1.ServiceTypeLoadBalancer
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hub.Name + "-wireguard",
			Namespace: hub.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Spec.Type = svcType
		svc.Spec.Selector = map[string]string{"app": hub.Name}
		svc.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "wireguard",
				Port:       wgPort,
				TargetPort: intstr.FromInt(int(wgPort)),
				Protocol:   corev1.ProtocolUDP,
			},
		}
		return controllerutil.SetControllerReference(hub, svc, r.Scheme)
	})
	if err != nil {
		return nil, err
	}

	// Re-fetch to get status (LB ingress)
	fetched := &corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, fetched); err != nil {
		return nil, err
	}
	return fetched, nil
}

func (r *ScutumHubReconciler) reconcileStatefulSet(ctx context.Context, hub *scutumv1alpha1.ScutumHub, image string) error {
	replicas := int32(1)
	if hub.Spec.Replicas != nil {
		replicas = *hub.Spec.Replicas
	}

	wgPort := hub.Spec.WireGuard.Port
	if wgPort == 0 {
		wgPort = 51820
	}

	// Security context — must run as root with NET_ADMIN for WireGuard
	privEsc := true
	secCtx := &corev1.SecurityContext{
		RunAsUser: int64Ptr(0),
		Capabilities: &corev1.Capabilities{
			Add:  []corev1.Capability{"NET_ADMIN"},
			Drop: []corev1.Capability{"ALL"},
		},
		AllowPrivilegeEscalation: &privEsc,
	}

	// Container env vars from ConfigMap
	envFrom := []corev1.EnvFromSource{
		{
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: hub.Name},
			},
		},
	}

	// Extra env: DATABASE_URL from secret if configured
	var extraEnv []corev1.EnvVar
	if hub.Spec.Database.ExistingSecret != "" {
		keyName := hub.Spec.Database.ExistingSecretKey
		if keyName == "" {
			keyName = "DATABASE_URL"
		}
		extraEnv = append(extraEnv, corev1.EnvVar{
			Name: "DATABASE_URL",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: hub.Spec.Database.ExistingSecret},
					Key:                  keyName,
				},
			},
		})
	}
	if hub.Spec.TLS.ExistingSecret != "" {
		extraEnv = append(extraEnv,
			corev1.EnvVar{Name: "CERT_FILE", Value: "/secrets/tls.crt"},
			corev1.EnvVar{Name: "KEY_FILE", Value: "/secrets/tls.key"},
		)
	}

	// Volume mounts
	volumeMounts := []corev1.VolumeMount{
		{Name: "data", MountPath: "/data"},
		{Name: "secrets", MountPath: "/secrets"},
	}
	if hub.Spec.WireGuard.Enabled {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "dev-net-tun",
			MountPath: "/dev/net/tun",
		})
	}

	// Ports
	ports := []corev1.ContainerPort{
		{Name: "api", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
	}
	if hub.Spec.WireGuard.Enabled {
		ports = append(ports, corev1.ContainerPort{
			Name:          "wireguard",
			ContainerPort: wgPort,
			Protocol:      corev1.ProtocolUDP,
		})
	}

	// Volumes (non-PVC)
	volumes := []corev1.Volume{}
	if hub.Spec.TLS.ExistingSecret != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "secrets",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{Name: hub.Spec.TLS.ExistingSecret},
								Items: []corev1.KeyToPath{
									{Key: "tls.crt", Path: "tls.crt"},
									{Key: "tls.key", Path: "tls.key"},
								},
							},
						},
					},
				},
			},
		})
	}
	if hub.Spec.WireGuard.Enabled {
		volumes = append(volumes, corev1.Volume{
			Name: "dev-net-tun",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/dev/net/tun",
					Type: hostPathTypePtr(corev1.HostPathCharDev),
				},
			},
		})
	}

	// Init containers
	var initContainers []corev1.Container
	if hub.Spec.TLS.AutoGenerate && hub.Spec.TLS.ExistingSecret == "" {
		initContainers = []corev1.Container{
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
`, hub.Name, hub.Name, hub.Name, hub.Namespace)},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "secrets", MountPath: "/secrets"},
				},
			},
		}
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

	// VolumeClaimTemplates
	dataSize := hub.Spec.Storage.DataSize
	if dataSize == "" {
		dataSize = "5Gi"
	}
	secretsSize := hub.Spec.Storage.SecretsSize
	if secretsSize == "" {
		secretsSize = "256Mi"
	}

	var vcts []corev1.PersistentVolumeClaim

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
	if hub.Spec.Storage.StorageClass != "" {
		dataPVC.Spec.StorageClassName = &hub.Spec.Storage.StorageClass
	}
	vcts = append(vcts, dataPVC)

	// Only add secrets PVC when not using projected (existingSecret)
	if hub.Spec.TLS.ExistingSecret == "" {
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
		if hub.Spec.Storage.StorageClass != "" {
			secretsPVC.Spec.StorageClassName = &hub.Spec.Storage.StorageClass
		}
		vcts = append(vcts, secretsPVC)
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hub.Name,
			Namespace: hub.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, sts, func() error {
		sts.Spec.Replicas = &replicas
		sts.Spec.ServiceName = hub.Name
		sts.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": hub.Name},
		}
		sts.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": hub.Name},
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: hub.Name,
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
						Ports:           ports,
						EnvFrom:         envFrom,
						Env:             extraEnv,
						VolumeMounts:    volumeMounts,
						SecurityContext: secCtx,
						LivenessProbe:   livenessProbe,
						ReadinessProbe:  readinessProbe,
						Resources:       hub.Spec.Resources,
					},
				},
				Volumes: volumes,
			},
		}
		sts.Spec.VolumeClaimTemplates = vcts
		return controllerutil.SetControllerReference(hub, sts, r.Scheme)
	})
	return err
}

// --- helpers ---

func int64Ptr(i int64) *int64 { return &i }

func hostPathTypePtr(t corev1.HostPathType) *corev1.HostPathType { return &t }
