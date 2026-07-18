package main

import (
	mg "github.com/afcollins/metrics-generator/pkg/metrics"
	"github.com/grafana/grafana-foundation-sdk/go/dashboard"
)

// TODO Need a better way to template out all these things without copying so much boilerplate each time. But we are getting close with being able to add rows on top of the existing dashboard without having to modify it.
func buildOCPPerformanceCUDNsDashboard() *dashboard.DashboardBuilder {
	return buildOCPCUDNsDashboard(&queryTracker{})
}

func buildOCPPerformanceCUDNsCollectedDashboard() *dashboard.DashboardBuilder {
	return buildOCPCUDNsDashboard(&queryTracker{useMetricNames: true})
}

func buildOCPCUDNsDashboard(t panelTracker) *dashboard.DashboardBuilder {
	return ocpBase(t, "Openshift Performance - CUDNs").
		// Row: CUDNs
		WithRow(ocpCUDNRow(t))
}

func buildOCPCUDNsProfiles() []namedProfile {
	agg := &mg.Generator{}
	buildOCPCUDNsDashboard(&queryTracker{g: agg})

	raw := &mg.Generator{}
	buildOCPCUDNsDashboard(&rawTracker{g: raw, seen: map[string]struct{}{}})

	return []namedProfile{
		{"-metrics", agg},
		{"-raw-metrics", raw},
	}
}

// Row: OVNk-at-a-Glance
func ocpCUDNRow(t panelTracker) *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("CUDNs").
		Collapsed(true).
		WithPanel(genericLegendTimeSeries("goroutines", "short",
			12, 6,
			t.trackRaw("go_goroutines_sum_by_namespace",
				`sum(go_goroutines) by (namespace)`,
				"{{namespace}}"),
		)).
		WithPanel(genericLegendTimeSeries("ovnkube_clustermanager_cluster_user_defined_networks", "short",
			12, 6,
			t.trackRaw("ovnkube_clustermanager_cluster_user_defined_networks",
				`ovnkube_clustermanager_cluster_user_defined_networks`,
				"ovnkube_clustermanager_cluster_user_defined_networks"),
		)).
		WithPanel(genericLegendTimeSeries("ovs_vswitchd_xlate_actions", "short",
			12, 6,
			t.trackRaw("ovs_vswitchd_xlate_actions_rate",
				`rate(ovs_vswitchd_xlate_actions[$interval])`,
				"ovs_vswitchd_xlate_actions - {{pod}} - {{instance}}"),
		)).
		WithPanel(genericLegendTimeSeries("ovs_vswitchd_dp_flows_lookup_hit", "short",
			12, 6,
			t.trackRaw("ovs_vswitchd_dp_flows_lookup_hit_rate",
				`rate(ovs_vswitchd_dp_flows_lookup_hit[$interval])`,
				"ovs_vswitchd_dp_flows_lookup_hit - {{pod}} - {{instance}}"),
		)).
		WithPanel(genericLegendTimeSeries("ovs_vswitchd_dpif_port_add", "short",
			12, 6,
			t.trackRaw("ovs_vswitchd_dpif_port_add_rate",
				`rate(ovs_vswitchd_dpif_port_add[$interval])`,
				"ovs_vswitchd_dpif_port_add - {{pod}}"),
		)).
		WithPanel(genericLegendTimeSeries("ovs_vswitchd_interface_resets_total", "short",
			12, 6,
			t.trackRaw("ovs_vswitchd_interface_resets_total_rate",
				`rate(ovs_vswitchd_interface_resets_total[$interval])`,
				"ovs_vswitchd_interface_resets_total - {{pod}}"),
		)).
		WithPanel(genericLegendTimeSeries("ovs_vswitchd_dpif_flow_get", "short",
			12, 6,
			t.trackRaw("ovs_vswitchd_dpif_flow_get_rate",
				`rate(ovs_vswitchd_dpif_flow_get[$interval])`,
				"ovs_vswitchd_dpif_flow_get - {{pod}}"),
		))
}
