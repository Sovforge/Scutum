package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HttpRequestsTotal tracks the total number of HTTP requests
	HttpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "scutum_http_requests_total",
		Help: "Total number of HTTP requests by method and status code",
	}, []string{"method", "path", "status"})

	// MeshNodesTotal tracks the total number of nodes in the mesh
	MeshNodesTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "scutum_mesh_nodes_total",
		Help: "Total number of nodes registered in the mesh",
	})

	// MeshNodesHealthy tracks the number of healthy nodes
	MeshNodesHealthy = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "scutum_mesh_nodes_healthy",
		Help: "Number of nodes currently reporting healthy",
	})

	// MeshSyncLatency tracks the latency of configuration pushes to edges
	MeshSyncLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "scutum_mesh_sync_latency_seconds",
		Help:    "Latency of mesh configuration synchronization",
		Buckets: prometheus.DefBuckets,
	}, []string{"node_id", "status"})

	// HealerCheckTotal tracks the number of health checks performed by the healer
	HealerCheckTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "scutum_healer_checks_total",
		Help: "Total number of health checks performed by the healer",
	}, []string{"result"})
)
