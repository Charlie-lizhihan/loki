package manifests

import (
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
)

const (
	containerImage = "docker.io/grafana/loki:2.1.0"
	gossipPort     = 7946
	httpPort       = 3100
	grpcPort       = 9095
)

func commonLabels(stackName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":     "loki",
		"app.kubernetes.io/provider": "openshift",
		"loki.grafana.com/name":      stackName,
	}
}

func ComponentLabels(component, stackName string) labels.Set {
	return labels.Merge(commonLabels(stackName), map[string]string{
		"loki.grafana.com/component": component,
	})
}

func GossipLabels() map[string]string {
	return map[string]string{
		"loki.grafana.com/gossip": "true",
	}
}

func serviceNameQuerierHTTP(stackName string) string {
	return fmt.Sprintf("loki-querier-http-%s", stackName)
}

func serviceNameQuerierGRPC(stackName string) string {
	return fmt.Sprintf("loki-querier-grpc-%s", stackName)
}

func serviceNameIngesterGRPC(stackName string) string {
	return fmt.Sprintf("loki-ingester-grpc-%s", stackName)
}

func serviceNameIngesterHTTP(stackName string) string {
	return fmt.Sprintf("loki-ingester-http-%s", stackName)
}

func serviceNameDistributorGRPC(stackName string) string {
	return fmt.Sprintf("loki-distributor-grpc-%s", stackName)
}

func serviceNameDistributorHTTP(stackName string) string {
	return fmt.Sprintf("loki-distributor-http-%s", stackName)
}

func fqdn(serviceName, namespace string) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace)
}
