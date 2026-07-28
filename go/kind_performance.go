package main

import (
	mg "github.com/afcollins/metrics-generator/pkg/metrics"
	"github.com/grafana/grafana-foundation-sdk/go/dashboard"
)

func buildKindPerformanceDashboard() *dashboard.DashboardBuilder {
	return buildKindDashboard(&queryTracker{})
}

func buildKindProfiles() []namedProfile {
	agg := &mg.Generator{}
	buildKindDashboard(&queryTracker{g: agg})

	raw := &mg.Generator{}
	buildKindDashboard(&rawTracker{g: raw, seen: map[string]struct{}{}})

	return []namedProfile{
		{"-metrics", agg},
		{"-raw-metrics", raw},
	}
}

func buildKindDashboard(t panelTracker) *dashboard.DashboardBuilder {
	dbBuilder := perfBase(t, "Kubernetes Performance", "Performance dashboard for Kubernetes\n").
		WithRow(ocpClusterAtAGlanceRow(t)).
		WithRow(ocpOVNRow(t)).
		WithRow(ocpMonitoringStackRow(t)).
		WithRow(ocpClusterKubeletRow(t)).
		WithRow(ocpClusterDetailsRow(t, false))
	withMasterNodeDetailRow(dbBuilder, t)
	withWorkerNodeDetailRow(dbBuilder, t)
	return dbBuilder
}
