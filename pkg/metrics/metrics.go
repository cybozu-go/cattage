package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	k8smetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	metricsNameSpace = "cattage"
	tenantSubsystem  = "tenant"
)

var (
	HealthyVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNameSpace,
		Subsystem: tenantSubsystem,
		Name:      "healthy",
		Help:      "The tenant status about healthy condition",
	}, []string{"name", "namespace"})

	UnhealthyVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNameSpace,
		Subsystem: tenantSubsystem,
		Name:      "unhealthy",
		Help:      "The tenant status about unhealthy condition",
	}, []string{"name", "namespace"})
)

func init() {
	k8smetrics.Registry.MustRegister(HealthyVec, UnhealthyVec)
}
