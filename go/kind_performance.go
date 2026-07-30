package main

import (
	"fmt"

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
		WithRow(kindClusterAtAGlanceRow(t)).
		WithRow(ocpOVNRow(t)).
		WithRow(ocpMonitoringStackRow(t)).
		WithRow(ocpClusterKubeletRow(t)).
		WithRow(ocpClusterDetailsRow(t, false))
	withKindMasterNodeDetailRow(dbBuilder, t)
	withKindWorkerNodeDetailRow(dbBuilder, t)
	return dbBuilder
}

// nodeRoleViaNodeInfo joins node_exporter metrics to kube_node_role through
// kube_node_info, bridging internal_ip → instance for kind clusters where
// instance=IP:9100 instead of hostname.
func nodeRoleViaNodeInfo(role mg.NodeRole) *mg.Query {
	return mg.Q(mg.MetricKubeNodeInfo, "").
		MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByNode},
			mg.Q(mg.MetricKubeNodeRole, fmt.Sprintf(`role="%s"`, role))).
		LabelReplace("instance", "$1:9100", "internal_ip", "(.+)")
}

func kindClusterAtAGlanceRow(t panelTracker) *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("Cluster-at-a-Glance").
		Collapsed(true).
		WithPanel(genericLegendTimeSeries("Workers CPU Usage", "percent",
			12, 8,
			t.track("nodeCPUWorker",
				mg.Q(mg.MetricNodeCPU, `mode != "idle"`).
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						nodeRoleViaNodeInfo(mg.RoleWorker)).
					RateSubquery(intervalVar).
					Agg(mg.AggSum, mg.GroupByInstance).
					Multiply("100"),
				"{{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("Control Plane CPU Usage", "percent",
			12, 8,
			t.track("nodeCPUControlPlane",
				mg.Q(mg.MetricNodeCPU, `mode != "idle"`).
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						nodeRoleViaNodeInfo("control-plane")).
					RateSubquery(intervalVar).
					Agg(mg.AggSum, mg.GroupByInstance).
					Multiply("100"),
				"{{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("Workers Load1", "short",
			12, 8,
			t.track("nodeLoad1Worker",
				mg.Raw("node_load1").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						nodeRoleViaNodeInfo(mg.RoleWorker)),
				"{{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("Control Plane Load1", "short",
			12, 8,
			t.track("nodeLoad1ControlPlane",
				mg.Raw("node_load1").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						nodeRoleViaNodeInfo("control-plane")),
				"{{node}}"),
		)).
		WithPanel(genericLegendCounterSumRightHandTimeSeries("Workers Memory Available", "bytes",
			12, 8,
			t.track("nodeMemoryAvailableWorker",
				mg.Q(mg.MetricNodeMemoryAvailable, "").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						nodeRoleViaNodeInfo(mg.RoleWorker)),
				"{{node}}"),
			t.track("nodeMemoryAvailableWorkerSum",
				mg.Q(mg.MetricNodeMemoryAvailable, "").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						nodeRoleViaNodeInfo(mg.RoleWorker)).
					Agg(mg.AggSum),
				"sum"),
		)).
		WithPanel(genericLegendCounterSumRightHandTimeSeries("Control Plane Memory Available", "bytes",
			12, 8,
			t.track("nodeMemoryAvailableControlPlane",
				mg.Q(mg.MetricNodeMemoryAvailable, "").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						nodeRoleViaNodeInfo("control-plane")),
				"{{node}}"),
			t.track("nodeMemoryAvailableControlPlaneSum",
				mg.Q(mg.MetricNodeMemoryAvailable, "").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						nodeRoleViaNodeInfo("control-plane")).
					Agg(mg.AggSum),
				"sum"),
		)).
		WithPanel(genericLegendTimeSeries("Workers CGroup CPU Rate", "percent",
			12, 8,
			t.track("cgroupCPUWorker",
				mg.Q(mg.MetricContainerCPU, cgroupIDFilter).
					Rate(intervalVar).
					Multiply("100").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByNode},
						mg.NodeRoleFilter(mg.RoleWorker)).
					Agg(mg.AggSum, mg.GroupByID),
				"{{instance}}"),
		)).
		WithPanel(genericLegendTimeSeries("Control Plane CGroup CPU Rate", "percent",
			12, 8,
			t.track("cgroupCPUControlPlane",
				mg.Q(mg.MetricContainerCPU, cgroupIDFilter).
					Rate(intervalVar).
					Multiply("100").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByNode},
						mg.NodeRoleFilter("control-plane")).
					Agg(mg.AggSum, mg.GroupByID),
				"{{instance}}"),
		)).
		WithPanel(genericLegendCounterTimeSeries("Workers CGroup Memory RSS", "bytes",
			12, 8,
			t.track("cgroupMemoryRSSWorker",
				mg.Q(mg.MetricContainerMemoryRSS, cgroupIDFilter).
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByNode},
						mg.NodeRoleFilter(mg.RoleWorker)).
					Agg(mg.AggSum, mg.GroupByID),
				"{{instance}}"),
		)).
		WithPanel(genericLegendCounterTimeSeries("Control Plane CGroup Memory RSS", "bytes",
			12, 8,
			t.track("cgroupMemoryRSSControlPlane",
				mg.Q(mg.MetricContainerMemoryRSS, cgroupIDFilter).
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByNode},
						mg.NodeRoleFilter("control-plane")).
					Agg(mg.AggSum, mg.GroupByID),
				"{{instance}}"),
		)).
		WithPanel(genericLegendCounterTimeSeries("Workers Container Threads", "short",
			12, 8,
			t.track("containerThreadsWorker",
				mg.Raw(`container_threads{container!=""}`).
					Agg(mg.AggSum, mg.GroupByNode).
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByNode},
						mg.NodeRoleFilter(mg.RoleWorker)),
				"{{instance}}"),
		)).
		WithPanel(genericLegendCounterTimeSeries("Control Plane Container Threads", "short",
			12, 8,
			t.track("containerThreadsControlPlane",
				mg.Raw(`container_threads{container!=""}`).
					Agg(mg.AggSum, mg.GroupByNode).
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByNode},
						mg.NodeRoleFilter("control-plane")),
				"{{instance}}"),
		)).
		WithPanel(genericLegendTimeSeries("Workers Disk IOPS", "short",
			12, 8,
			t.track("nodeDiskReadsWorker",
				mg.Raw("node_disk_reads_completed_total").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						nodeRoleViaNodeInfo(mg.RoleWorker)).
					RateSubquery(intervalVar),
				"{{node}} - {{ device }} - read"),
			t.track("nodeDiskWritesWorker",
				mg.Raw("node_disk_writes_completed_total").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						nodeRoleViaNodeInfo(mg.RoleWorker)).
					RateSubquery(intervalVar),
				"{{node}} - {{ device }} - write"),
		)).
		WithPanel(genericLegendTimeSeries("Control Plane Disk IOPS", "short",
			12, 8,
			t.track("nodeDiskReadsControlPlane",
				mg.Raw("node_disk_reads_completed_total").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						nodeRoleViaNodeInfo("control-plane")).
					RateSubquery(intervalVar),
				"{{node}} - {{ device }} - read"),
			t.track("nodeDiskWritesControlPlane",
				mg.Raw("node_disk_writes_completed_total").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						nodeRoleViaNodeInfo("control-plane")).
					RateSubquery(intervalVar),
				"{{node}} - {{ device }} - write"),
		))
}

func kindNodeInfoFilter(nodeVar string) *mg.Query {
	return mg.Q(mg.MetricKubeNodeInfo, `node=~"$`+nodeVar+`"`).
		LabelReplace("instance", "$1:9100", "internal_ip", "(.+)")
}

func kindNodeInstanceStrategy(nodeVar string) nodeInstanceStrategy {
	nif := kindNodeInfoFilter(nodeVar)
	return nodeInstanceStrategy{
		applyFilter: func(q *mg.Query) *mg.Query {
			return q.Paren().MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance}, nif)
		},
		legend: "{{node}}",
	}
}

func withKindMasterNodeDetailRow(builder *dashboard.DashboardBuilder, t panelTracker) *dashboard.DashboardBuilder {
	return builder.WithRow(nodeRow(t, "_master_node", mg.RoleMaster, kindNodeInstanceStrategy("_master_node"))).
		WithVariable(dashboard.NewQueryVariableBuilder("_master_node").
			Label("Master").
			Query(t.trackVarQuery("masterNodes", `label_values(kube_node_role{role=~"master|control-plane"}, node)`)).
			Datasource(promDatasourceRef()).
			Refresh(dashboard.VariableRefreshOnTimeRangeChanged).
			Multi(true).
			IncludeAll(false),
		)
}

func withKindWorkerNodeDetailRow(builder *dashboard.DashboardBuilder, t panelTracker) *dashboard.DashboardBuilder {
	return builder.WithRow(nodeRow(t, "_worker_node", mg.RoleWorker, kindNodeInstanceStrategy("_worker_node"))).
		WithVariable(dashboard.NewQueryVariableBuilder("_worker_node").
			Label("Worker").
			Query(t.trackVarQuery("workerNodes", `label_values(kube_node_role{role=~"worker"}, node)`)).
			Datasource(promDatasourceRef()).
			Refresh(dashboard.VariableRefreshOnTimeRangeChanged).
			Multi(true).
			IncludeAll(false),
		)
}
