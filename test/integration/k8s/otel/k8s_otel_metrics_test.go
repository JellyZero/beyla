//go:build integration

package otel

import (
	"testing"

	k8s "github.com/grafana/beyla/test/integration/k8s/common"
)

// Run it alphabetically first (AA-prefix), with a longer timeout, to wait until all the components are up and
// traces/metrics are flowing normally
func TestOTEL_MetricsDecoration_AA_HTTP_ExternalToPod(t *testing.T) {
	k8s.DoTestHTTPMetricsDecorationExternalToPod(t)
}

func TestOTEL_MetricsDecoration_HTTP_Pod2Service(t *testing.T) {
	cluster.TestEnv().Test(t, k8s.FeatureHTTPDecorationPod2Service())
}

func TestOTEL_MetricsDecoration_HTTPClient_Pod2Pod(t *testing.T) {
	cluster.TestEnv().Test(t, k8s.FeatureHTTPClientMetricsDecorationPod2Pod())
}

func TestOTEL_MetricsDecoration_HTTP_Pod2External(t *testing.T) {
	cluster.TestEnv().Test(t, k8s.FeatureHTTPMetricsDecorationPod2External())
}

func TestOTEL_MetricsDecoration_GRPC_Pod2Service(t *testing.T) {
	cluster.TestEnv().Test(t, k8s.FeatureGRPCMetricsDecorationPod2Service())
}

func TestOTEL_MetricsDecoration_GRPC_Pod2Pod(t *testing.T) {
	cluster.TestEnv().Test(t, k8s.FeatureGRPCMetricsDecorationPod2Pod())
}
