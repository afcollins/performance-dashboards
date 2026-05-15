package main

import (
	"github.com/grafana/grafana-foundation-sdk/go/cog"
	"github.com/grafana/grafana-foundation-sdk/go/common"
	"github.com/grafana/grafana-foundation-sdk/go/dashboard"
	"github.com/grafana/grafana-foundation-sdk/go/prometheus"
	mg "github.com/kube-burner/metrics-generator/pkg/metrics"
)

const (
	intervalVar = mg.RateInterval("$interval")

	cgroupIDFilter             = `job=~".*", id =~"/system.slice|/system.slice/kubelet.service|/.*/ovs-vswitchd.service|/system.slice/crio.service|/system.slice/systemd-journald.service|/.*/ovsdb-server.service|/system.slice/systemd-udevd.service|/kubepods.slice"`
	cgroupIDFilterWithJournald = `job=~".*", id =~"/system.slice|/system.slice/kubelet.service|/.*/ovs-vswitchd.service|/system.slice/crio.service|/system.slice/systemd-journald.service|/system.slice/.*.service|/system.slice/systemd-udevd.service|/kubepods.slice"`

	fsWriteFilter    = `device!~".+dm.+"`
	fsReadFilter     = `device!~".+dm.+"`
	cgroupFSIDFilter = `device!~".+dm.+", id =~"/system.slice/kubelet.service|/.*/ovs-vswitchd.service|/system.slice/crio.service|/system.slice/systemd-journald.service|/.*/ovsdb-server.service|/system.slice/systemd-udevd.service|/kubepods.slice"`
)

func q(metric mg.Metric, filters string) string {
	return mg.Q(metric, filters).String()
}

func buildOCPPerformanceDashboard() *dashboard.DashboardBuilder {
	return buildOCPDashboard(&queryTracker{})
}

func buildOCPMetricsProfile() *mg.Generator {
	g := &mg.Generator{}
	buildOCPDashboard(&queryTracker{g})
	return g
}

func buildOCPDashboard(t *queryTracker) *dashboard.DashboardBuilder {
	return dashboard.NewDashboardBuilder("Openshift Performance").
		Description("Performance dashboard for Red Hat Openshift\n").
		Tags([]string{}).
		Time("now-1h", "now").
		Timezone("utc").
		Timepicker(dashboard.NewTimePickerBuilder().
			RefreshIntervals([]string{"5s", "10s", "30s", "1m", "5m", "15m", "30m", "1h", "2h", "1d"}),
		).
		Refresh("30s").
		Tooltip(dashboard.DashboardCursorSyncCrosshair).
		WithVariable(dashboard.NewDatasourceVariableBuilder("Datasource").
			Type("prometheus").
			Label("Datasource"),
		).
		WithVariable(dashboard.NewQueryVariableBuilder("_master_node").
			Label("Master").
			Query(dashboard.StringOrMap{String: cog.ToPtr(`label_values(kube_node_role{role="master"}, node)`)}).
			Datasource(promDatasourceRef()).
			Refresh(dashboard.VariableRefreshOnTimeRangeChanged).
			Multi(true).
			IncludeAll(false),
		).
		WithVariable(dashboard.NewQueryVariableBuilder("_worker_node").
			Label("Worker").
			Query(dashboard.StringOrMap{String: cog.ToPtr(`label_values(kube_node_role{role=~"worker"}, node)`)}).
			Datasource(promDatasourceRef()).
			Refresh(dashboard.VariableRefreshOnTimeRangeChanged).
			Multi(true).
			IncludeAll(false),
		).
		WithVariable(dashboard.NewQueryVariableBuilder("_infra_node").
			Label("Infra").
			Query(dashboard.StringOrMap{String: cog.ToPtr(`label_values(kube_node_role{role="infra"}, node)`)}).
			Datasource(promDatasourceRef()).
			Refresh(dashboard.VariableRefreshOnTimeRangeChanged).
			Multi(true).
			IncludeAll(false),
		).
		WithVariable(dashboard.NewQueryVariableBuilder("namespace").
			Label("Namespace").
			Query(dashboard.StringOrMap{String: cog.ToPtr(`label_values(` + q(mg.MetricKubePodInfo, mg.Filters(mg.NSNotRegex("cluster-density.*|node-density-.*"))) + ",namespace)")}).
			Datasource(promDatasourceRef()).
			Refresh(dashboard.VariableRefreshOnTimeRangeChanged).
			Regex("").
			Multi(false).
			IncludeAll(true),
		).
		WithVariable(dashboard.NewQueryVariableBuilder("block_device").
			Label("Block device").
			Query(dashboard.StringOrMap{String: cog.ToPtr(`label_values(node_disk_written_bytes_total, device)`)}).
			Datasource(promDatasourceRef()).
			Refresh(dashboard.VariableRefreshOnTimeRangeChanged).
			Regex(`/^(?:(?!dm|rb).)*$/`).
			Multi(true).
			IncludeAll(true),
		).
		WithVariable(dashboard.NewQueryVariableBuilder("net_device").
			Label("Network device").
			Query(dashboard.StringOrMap{String: cog.ToPtr(`label_values(node_network_receive_bytes_total, device)`)}).
			Datasource(promDatasourceRef()).
			Refresh(dashboard.VariableRefreshOnTimeRangeChanged).
			Regex(`/^((br|en|et).*)$/`).
			Multi(true).
			IncludeAll(true),
		).
		WithVariable(dashboard.NewIntervalVariableBuilder("interval").
			Label("interval").
			Values(dashboard.StringOrMap{String: cog.ToPtr("2m,3m,4m,5m")}).
			Current(intervalOption("2m")).
			Options([]dashboard.VariableOption{
				intervalOption("2m"),
				intervalOption("3m"),
				intervalOption("4m"),
				intervalOption("5m"),
			}),
		).
		WithRow(ocpSnrNhcRow(t)).
		// Row: Cluster-at-a-Glance
		WithRow(ocpClusterAtAGlanceRow(t)).
		// Row: OVN
		WithRow(ocpOVNRow(t)).
		// Row: Monitoring stack
		WithRow(ocpMonitoringStackRow(t)).
		// Row: Cluster Kubelet
		WithRow(ocpClusterKubeletRow(t)).
		// Row: Cluster Details
		WithRow(ocpClusterDetailsRow(t)).
		// Row: Cluster Operators Details
		WithRow(ocpClusterOperatorsDetailsRow(t)).
		// Row: Master
		WithRow(ocpMasterRow(t)).
		// Row: Worker
		WithRow(ocpWorkerRow(t)).
		// Row: Infra
		WithRow(ocpInfraRow(t)).
		// Row: Stackrox
		WithRow(ocpStackroxRow(t))
}

func ocpSnrNhcRow(t *queryTracker) cog.Builder[dashboard.RowPanel] {
	return dashboard.NewRowBuilder("SNR / NHC Panels").
		Collapsed(true).
		GridPos(dashboard.GridPos{X: 0, Y: 0, W: 24, H: 1}).
		WithPanel(genericLegendTimeSeries("openshift-workload-availability CPU stats", "percent",
			dashboard.GridPos{X: 0, Y: 2, W: 12, H: 8},
			containerCPUForNamespace(t, mg.AggAvg, "openshift-workload-availability"),
			containerCPUForNamespace(t, mg.AggMax, "openshift-workload-availability"),
		)).
		WithPanel(genericLegendTimeSeries("openshift-workload-availability Mem stats", "bytes",
			dashboard.GridPos{X: 12, Y: 1, W: 12, H: 8},
			wrkldAvailMem(t, mg.AggAvg),
			wrkldAvailMem(t, mg.AggMax),
		)).
		WithPanel(genericLegendTimeSeries("openshift-workload-availability CPU sum", "percent",
			dashboard.GridPos{X: 0, Y: 9, W: 12, H: 8},
			containerCPUForNamespace(t, mg.AggSum, "openshift-workload-availability"),
		)).
		WithPanel(genericLegendTimeSeries("openshift-workload-availability Mem sum", "bytes",
			dashboard.GridPos{X: 12, Y: 9, W: 12, H: 8},
			wrkldAvailMem(t, mg.AggSum),
		)).
		WithPanel(genericLegendTimeSeries("openshift-insights CPU sum", "percent",
			dashboard.GridPos{X: 0, Y: 17, W: 12, H: 8},
			containerCPUForNamespace(t, mg.AggSum, "openshift-insights"),
		)).
		WithPanel(genericLegendTimeSeries("taints and unschedulable", "short",
			dashboard.GridPos{X: 12, Y: 17, W: 12, H: 8},
			t.track("kubeNodeSpecTaintCount", mg.Q("kube_node_spec_taint", `key=~'medik8s.io/remediation|node.kubernetes.io/unreachable|node.kubernetes.io/unschedulable'`).Agg(mg.AggCount, mg.GroupByKey), "taint - {{key}}"),
			promQuery(mg.Q("kube_node_spec_taint", `key=~'medik8s.io/remediation|node.kubernetes.io/unreachable|node.kubernetes.io/unschedulable'`).String(), "taint - {{key}} {{node}}"),
			t.track("kubeNodeSpecUnschedulableCount", mg.Q("kube_node_spec_unschedulable", "").Gt("0").Agg(mg.AggCount), "unschedulable"),
			promQuery(mg.Q("kube_node_spec_unschedulable", "").Gt("0").String(), "unschedulable - {{node}}"),
			t.track("kubeNodeStatusNotReady", mg.Q("kube_node_status_condition", `condition="Ready",status=~"false|unknown"`).Gt("0"), "status false or unknown {{node}}"),
			t.track("kubePodPending", mg.Q(mg.MetricKubePodStatusPhase, mg.Filters(`phase="Pending"`, mg.NSNotRegex("preload.*"))).Gt("0").Agg(mg.AggCount, mg.GroupByNamespace), "pods pending {{namespace}}"),
		).Stacking(common.NewStackingConfigBuilder().Mode(common.StackingModeNormal)))
}

func containerCPUForNamespace(t *queryTracker, agg mg.AggFunc, namespace string) *prometheus.DataqueryBuilder {
	return t.track(
		"containerCPU"+string(agg)+"_"+namespace,
		mg.Q(mg.MetricContainerCPU, mg.Filters(mg.NSIn(namespace), `container!="POD",name!=""`)).
			Rate(intervalVar).Multiply("100").
			Agg(agg, mg.GroupByContainer),
		"{{container}} - "+string(agg))
}

func wrkldAvailMem(t *queryTracker, agg mg.AggFunc) *prometheus.DataqueryBuilder {
	return t.track(
		"containerMemoryRSS"+string(agg)+"_openshift-workload-availability",
		mg.Q(mg.MetricContainerMemoryRSS, mg.Filters(mg.NSIn("openshift-workload-availability"), `container!="POD",name!=""`)).
			Agg(agg, mg.GroupByContainer),
		"{{container}} - "+string(agg))
}

// Row: Cluster-at-a-Glance
func ocpClusterAtAGlanceRow(t *queryTracker) *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("Cluster-at-a-Glance").
		Collapsed(true).
		GridPos(dashboard.GridPos{X: 0, Y: 0, W: 24, H: 1}).
		WithPanel(genericLegendTimeSeries("Workers CPU Usage", "percent",
			dashboard.GridPos{X: 0, Y: 2, W: 12, H: 8},
			t.track("nodeCPUWorker",
				mg.Q(mg.MetricNodeCPU, `mode != "idle"`).
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						mg.NodeRoleLabelReplace(mg.RoleWorker)).
					RateSubquery(intervalVar).
					Agg(mg.AggSum, mg.GroupByInstance).
					Multiply("100"),
				"{{instance}}"),
		)).
		WithPanel(genericLegendTimeSeries("Control Plane CPU Usage", "percent",
			dashboard.GridPos{X: 12, Y: 2, W: 12, H: 8},
			t.track("nodeCPUControlPlane",
				mg.Q(mg.MetricNodeCPU, `mode != "idle"`).
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						mg.NodeRoleLabelReplace("control-plane")).
					RateSubquery(intervalVar).
					Agg(mg.AggSum, mg.GroupByInstance).
					Multiply("100"),
				"{{instance}}"),
		)).
		WithPanel(genericLegendTimeSeries("Workers Load1", "short",
			dashboard.GridPos{X: 0, Y: 9, W: 12, H: 8},
			t.track("nodeLoad1Worker",
				mg.Raw("node_load1").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						mg.NodeRoleLabelReplace(mg.RoleWorker)),
				"{{instance}}"),
		)).
		WithPanel(genericLegendTimeSeries("Control Plane Load1", "short",
			dashboard.GridPos{X: 12, Y: 9, W: 12, H: 8},
			t.track("nodeLoad1ControlPlane",
				mg.Raw("node_load1").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						mg.NodeRoleLabelReplace("control-plane")),
				"{{instance}}"),
		)).
		WithPanel(genericLegendCounterSumRightHandTimeSeries("Workers Memory Available", "bytes",
			dashboard.GridPos{X: 0, Y: 17, W: 12, H: 8},
			t.track("nodeMemoryAvailableWorker",
				mg.Q(mg.MetricNodeMemoryAvailable, "").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						mg.NodeRoleLabelReplace(mg.RoleWorker)),
				"{{instance}}"),
			t.track("nodeMemoryAvailableWorkerSum",
				mg.Q(mg.MetricNodeMemoryAvailable, "").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						mg.NodeRoleLabelReplace(mg.RoleWorker)).
					Agg(mg.AggSum),
				"sum"),
		)).
		WithPanel(genericLegendCounterSumRightHandTimeSeries("Control Plane Memory Available", "bytes",
			dashboard.GridPos{X: 12, Y: 17, W: 12, H: 8},
			t.track("nodeMemoryAvailableControlPlane",
				mg.Q(mg.MetricNodeMemoryAvailable, "").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						mg.NodeRoleLabelReplace("control-plane")),
				"{{instance}}"),
			t.track("nodeMemoryAvailableControlPlaneSum",
				mg.Q(mg.MetricNodeMemoryAvailable, "").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						mg.NodeRoleLabelReplace("control-plane")).
					Agg(mg.AggSum),
				"sum"),
		)).
		WithPanel(genericLegendTimeSeries("Workers CGroup CPU Rate", "percent",
			dashboard.GridPos{X: 0, Y: 25, W: 12, H: 8},
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
			dashboard.GridPos{X: 12, Y: 25, W: 12, H: 8},
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
			dashboard.GridPos{X: 0, Y: 33, W: 12, H: 8},
			t.track("cgroupMemoryRSSWorker",
				mg.Q(mg.MetricContainerMemoryRSS, cgroupIDFilter).
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByNode},
						mg.NodeRoleFilter(mg.RoleWorker)).
					Agg(mg.AggSum, mg.GroupByID),
				"{{instance}}"),
		)).
		WithPanel(genericLegendCounterTimeSeries("Control Plane CGroup Memory RSS", "bytes",
			dashboard.GridPos{X: 12, Y: 33, W: 12, H: 8},
			t.track("cgroupMemoryRSSControlPlane",
				mg.Q(mg.MetricContainerMemoryRSS, cgroupIDFilter).
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByNode},
						mg.NodeRoleFilter("control-plane")).
					Agg(mg.AggSum, mg.GroupByID),
				"{{instance}}"),
		)).
		WithPanel(genericLegendCounterTimeSeries("Workers Container Threads", "short",
			dashboard.GridPos{X: 0, Y: 41, W: 12, H: 8},
			t.track("containerThreadsWorker",
				mg.Raw(`container_threads{container!=""}`).
					Agg(mg.AggSum, mg.GroupByNode).
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByNode},
						mg.NodeRoleFilter(mg.RoleWorker)),
				"{{instance}}"),
		)).
		WithPanel(genericLegendCounterTimeSeries("Control Plane Container Threads", "short",
			dashboard.GridPos{X: 12, Y: 41, W: 12, H: 8},
			t.track("containerThreadsControlPlane",
				mg.Raw(`container_threads{container!=""}`).
					Agg(mg.AggSum, mg.GroupByNode).
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByNode},
						mg.NodeRoleFilter("control-plane")),
				"{{instance}}"),
		)).
		WithPanel(genericLegendTimeSeries("Workers Disk IOPS", "short",
			dashboard.GridPos{X: 0, Y: 49, W: 12, H: 8},
			t.track("nodeDiskReadsWorker",
				mg.Raw("node_disk_reads_completed_total").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						mg.NodeRoleLabelReplace(mg.RoleWorker)).
					RateSubquery(intervalVar),
				"{{instance}} - {{ device }} - read"),
			t.track("nodeDiskWritesWorker",
				mg.Raw("node_disk_writes_completed_total").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						mg.NodeRoleLabelReplace(mg.RoleWorker)).
					RateSubquery(intervalVar),
				"{{instance}} - {{ device }} - write"),
		)).
		WithPanel(genericLegendTimeSeries("Control Plane Disk IOPS", "short",
			dashboard.GridPos{X: 12, Y: 49, W: 12, H: 8},
			t.track("nodeDiskReadsControlPlane",
				mg.Raw("node_disk_reads_completed_total").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						mg.NodeRoleLabelReplace("control-plane")).
					RateSubquery(intervalVar),
				"{{instance}} - {{ device }} - read"),
			t.track("nodeDiskWritesControlPlane",
				mg.Raw("node_disk_writes_completed_total").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						mg.NodeRoleLabelReplace("control-plane")).
					RateSubquery(intervalVar),
				"{{instance}} - {{ device }} - write"),
		))
}

// Row: OVN
func ocpOVNRow(t *queryTracker) *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("OVN").
		Collapsed(true).
		GridPos(dashboard.GridPos{X: 0, Y: 0, W: 24, H: 1}).
		WithPanel(genericLegendTimeSeries("Top 10 ovnkube-controller CPU Usage", "percent",
			dashboard.GridPos{X: 0, Y: 1, W: 12, H: 8},
			t.track("ovnkubeControllerCPU",
				mg.Q(mg.MetricContainerCPU, `pod=~"ovnkube-.*",namespace="openshift-ovn-kubernetes",container="ovnkube-controller"`).
					IRate(intervalVar).Multiply("100").
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByNode).
					TopK(10),
				"{{pod}} - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 ovnkube-controller Memory Usage", "bytes",
			dashboard.GridPos{X: 12, Y: 1, W: 12, H: 8},
			t.track("ovnkubeControllerMemory",
				mg.Q(mg.MetricContainerMemoryRSS, `pod=~"ovnkube-node-.*",namespace="openshift-ovn-kubernetes",container="ovnkube-controller"`).
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByNode).
					TopK(10),
				"{{pod}} - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 ovn-controller CPU Usage", "percent",
			dashboard.GridPos{X: 0, Y: 8, W: 12, H: 8},
			t.track("ovnControllerCPU",
				mg.Q(mg.MetricContainerCPU, `pod=~"ovnkube-.*",namespace="openshift-ovn-kubernetes",container="ovn-controller"`).
					IRate(intervalVar).Multiply("100").
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByNode).
					TopK(10),
				"{{pod}} - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 ovn-controller Memory Usage", "bytes",
			dashboard.GridPos{X: 12, Y: 8, W: 12, H: 8},
			t.track("ovnControllerMemory",
				mg.Q(mg.MetricContainerMemoryRSS, `pod=~"ovnkube-node-.*",namespace="openshift-ovn-kubernetes",container="ovn-controller"`).
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByNode).
					TopK(10),
				"{{pod}} - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 nbdb CPU Usage", "percent",
			dashboard.GridPos{X: 0, Y: 16, W: 12, H: 8},
			t.track("ovnNbdbCPU",
				mg.Q(mg.MetricContainerCPU, `pod=~"ovnkube-.*",namespace="openshift-ovn-kubernetes",container="nbdb"`).
					IRate(intervalVar).Multiply("100").
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByNode).
					TopK(10),
				"{{pod}} - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 nbdb Memory Usage", "bytes",
			dashboard.GridPos{X: 12, Y: 16, W: 12, H: 8},
			t.track("ovnNbdbMemory",
				mg.Q(mg.MetricContainerMemoryRSS, `pod=~"ovnkube-node-.*",namespace="openshift-ovn-kubernetes",container="nbdb"`).
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByNode).
					TopK(10),
				"{{pod}} - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 northd CPU Usage", "percent",
			dashboard.GridPos{X: 0, Y: 24, W: 12, H: 8},
			t.track("ovnNorthdCPU",
				mg.Q(mg.MetricContainerCPU, `pod=~"ovnkube-.*",namespace="openshift-ovn-kubernetes",container="northd"`).
					IRate(intervalVar).Multiply("100").
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByNode).
					TopK(10),
				"{{pod}} - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 northd Memory Usage", "bytes",
			dashboard.GridPos{X: 12, Y: 24, W: 12, H: 8},
			t.track("ovnNorthdMemory",
				mg.Q(mg.MetricContainerMemoryRSS, `pod=~"ovnkube-node-.*",namespace="openshift-ovn-kubernetes",container="northd"`).
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByNode).
					TopK(10),
				"{{pod}} - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 sbdb CPU Usage", "percent",
			dashboard.GridPos{X: 0, Y: 32, W: 12, H: 8},
			t.track("ovnSbdbCPU",
				mg.Q(mg.MetricContainerCPU, `pod=~"ovnkube-.*",namespace="openshift-ovn-kubernetes",container="sbdb"`).
					IRate(intervalVar).Multiply("100").
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByNode).
					TopK(10),
				"{{pod}} - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 sbdb Memory Usage", "bytes",
			dashboard.GridPos{X: 12, Y: 32, W: 12, H: 8},
			t.track("ovnSbdbMemory",
				mg.Q(mg.MetricContainerMemoryRSS, `pod=~"ovnkube-node-.*",namespace="openshift-ovn-kubernetes",container="sbdb"`).
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByNode).
					TopK(10),
				"{{pod}} - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("ovs-master CPU Usage", "percent",
			dashboard.GridPos{X: 0, Y: 40, W: 12, H: 8},
			t.track("ovsMasterVswitchdCPU",
				mg.Q(mg.MetricContainerCPU, `id=~"/.*/ovs-vswitchd.service", node=~"$_master_node"`).
					IRate(intervalVar).Multiply("100"),
				"OVS CPU - {{ node }}"),
			t.track("ovsMasterOvsdbCPU",
				mg.Q(mg.MetricContainerCPU, `id=~"/.*/ovsdb-server.service", node=~"$_master_node"`).
					IRate(intervalVar).Multiply("100"),
				"OVS DB CPU - {{ node }}"),
		)).
		WithPanel(genericLegendTimeSeries("ovs-master Memory Usage", "bytes",
			dashboard.GridPos{X: 12, Y: 40, W: 12, H: 8},
			t.track("ovsMasterVswitchdMemory", mg.Q(mg.MetricContainerMemoryRSS, `id=~"/.*/ovs-vswitchd.service", node=~"$_master_node"`), "OVS Memory - {{ node }}"),
			t.track("ovsMasterOvsdbMemory", mg.Q(mg.MetricContainerMemoryRSS, `id=~"/.*/ovsdb-server.service", node=~"$_master_node"`), "OVS DB Memory - {{ node }}"),
		)).
		WithPanel(genericLegendTimeSeries("ovs-worker CPU Usage", "percent",
			dashboard.GridPos{X: 0, Y: 48, W: 12, H: 8},
			t.track("ovsWorkerVswitchdCPU",
				mg.Q(mg.MetricContainerCPU, `id=~"/.*/ovs-vswitchd.service", node=~"$_worker_node"`).
					IRate(intervalVar).Multiply("100"),
				"OVS CPU - {{ node }}"),
			t.track("ovsWorkerOvsdbCPU",
				mg.Q(mg.MetricContainerCPU, `id=~"/.*/ovsdb-server.service", node=~"$_worker_node"`).
					IRate(intervalVar).Multiply("100"),
				"OVS DB CPU - {{ node }}"),
		)).
		WithPanel(genericLegendTimeSeries("ovs-worker Memory Usage", "bytes",
			dashboard.GridPos{X: 12, Y: 48, W: 12, H: 8},
			t.track("ovsWorkerVswitchdMemory", mg.Q(mg.MetricContainerMemoryRSS, `id=~"/.*/ovs-vswitchd.service", node=~"$_worker_node"`), "OVS Memory - {{ node }}"),
			t.track("ovsWorkerOvsdbMemory", mg.Q(mg.MetricContainerMemoryRSS, `id=~"/.*/ovsdb-server.service", node=~"$_worker_node"`), "OVS DB Memory - {{ node }}"),
		)).
		WithPanel(genericLegendTimeSeries("99% Pod Annotation Latency", "s",
			dashboard.GridPos{X: 0, Y: 56, W: 8, H: 8},
			t.track("ovnPodAnnotationLatencyP99",
				mg.Raw("ovnkube_controller_pod_creation_latency_seconds").
					BucketRate(intervalVar).
					Agg(mg.AggSum, mg.GroupByInstance, mg.GroupByPod, mg.GroupByLE).
					HistogramQuantile(mg.P99).
					Gt("0"),
				"{{ pod }} - {{ instance }}"),
		)).
		WithPanel(genericLegendTimeSeries("99% CNI Request ADD Latency", "s",
			dashboard.GridPos{X: 8, Y: 56, W: 8, H: 8},
			t.track("ovnCNIAddLatencyP99",
				mg.Raw(`ovnkube_node_cni_request_duration_seconds{command="ADD"}`).
					BucketRate(intervalVar).
					Agg(mg.AggSum, mg.GroupByInstance, mg.GroupByPod, mg.GroupByLE).
					HistogramQuantile(mg.P99).
					Gt("0"),
				"{{ pod }} - {{ instance }}"),
		)).
		WithPanel(genericLegendTimeSeries("99% CNI Request DEL Latency", "s",
			dashboard.GridPos{X: 16, Y: 56, W: 8, H: 8},
			t.track("ovnCNIDelLatencyP99",
				mg.Raw(`ovnkube_node_cni_request_duration_seconds{command="DEL"}`).
					BucketRate(intervalVar).
					Agg(mg.AggSum, mg.GroupByInstance, mg.GroupByPod, mg.GroupByLE).
					HistogramQuantile(mg.P99).
					Gt("0"),
				"{{ pod }} - {{ instance }}"),
		)).
		WithPanel(genericLegendTimeSeries("ovnkube-control-plane CPU Usage", "percent",
			dashboard.GridPos{X: 0, Y: 64, W: 12, H: 8},
			t.track("ovnkubeControlPlaneCPU",
				mg.Q(mg.MetricContainerCPU, `pod=~"(ovnkube-master|ovnkube-control-plane).+",namespace="openshift-ovn-kubernetes",container!~"POD|"`).
					IRate(intervalVar).Multiply("100").
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByNode),
				"{{pod}} - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("ovnkube-control-plane Memory Usage", "bytes",
			dashboard.GridPos{X: 12, Y: 64, W: 12, H: 8},
			t.track("ovnkubeControlPlaneMemory",
				mg.Q(mg.MetricContainerMemoryRSS, `pod=~"(ovnkube-master|ovnkube-control-plane).+",namespace="openshift-ovn-kubernetes",container!~"POD|"`),
				"{{pod}} - {{node}}"),
		))
}

// Row: Monitoring stack
func ocpMonitoringStackRow(t *queryTracker) *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("Monitoring stack").
		Collapsed(true).
		GridPos(dashboard.GridPos{X: 0, Y: 0, W: 24, H: 1}).
		WithPanel(genericLegendTimeSeries("Prometheus Replica CPU", "percent",
			dashboard.GridPos{X: 0, Y: 2, W: 12, H: 8},
			t.track("prometheusCPU",
				mg.Q(mg.MetricContainerCPU, `pod=~"prometheus-k8s-[01]",namespace!="",name!="",container="prometheus"`).
					IRate(intervalVar).
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByContainer, mg.GroupByNode).
					Multiply("100"),
				"{{pod}} - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("Prometheus Replica RSS", "bytes",
			dashboard.GridPos{X: 12, Y: 2, W: 12, H: 8},
			t.track("prometheusMemory",
				mg.Q(mg.MetricContainerMemoryRSS, `pod=~"prometheus-k8s-[01]",namespace!="",name!="",container="prometheus"`).
					Agg(mg.AggSum, mg.GroupByPod),
				"{{pod}}"),
		)).
		WithPanel(genericLegendTimeSeries("metrics-server/prom-adapter CPU", "percent",
			dashboard.GridPos{X: 0, Y: 10, W: 12, H: 8},
			t.track("metricsServerCPU",
				mg.Q(mg.MetricContainerCPU, `pod=~"metrics-server-.*",namespace!="",name!=""`).
					IRate(intervalVar).
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByContainer).
					Multiply("100"),
				"{{pod}}"),
			t.track("promAdapterCPU",
				mg.Q(mg.MetricContainerCPU, `pod=~"prometheus-adapter-.*",namespace="openshift-monitoring",name!=""`).
					IRate(intervalVar).
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByContainer).
					Multiply("100"),
				"{{pod}}"),
		)).
		WithPanel(genericLegendTimeSeries("metrics-server/prom-adapter RSS", "bytes",
			dashboard.GridPos{X: 12, Y: 10, W: 12, H: 8},
			t.track("metricsServerMemory",
				mg.Q(mg.MetricContainerMemoryRSS, `pod=~"metrics-server-.*",namespace!="",name!=""`).
					Agg(mg.AggSum, mg.GroupByPod),
				"{{pod}}"),
			t.track("promAdapterMemory",
				mg.Q(mg.MetricContainerMemoryRSS, `pod=~"prometheus-adapter-.*",namespace="openshift-monitoring",name!=""`).
					Agg(mg.AggSum, mg.GroupByPod),
				"{{pod}}"),
		))
}

// Row: Stackrox
func ocpStackroxRow(t *queryTracker) *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("Stackrox").
		Collapsed(true).
		GridPos(dashboard.GridPos{X: 0, Y: 0, W: 24, H: 1}).
		WithPanel(genericLegendTimeSeries("Top 25 stackrox container RSS bytes", "bytes",
			dashboard.GridPos{X: 0, Y: 2, W: 12, H: 8},
			t.track("stackroxContainerMemoryRSS",
				mg.Q(mg.MetricContainerMemoryRSS, `container!="POD",name!="",namespace!="",namespace=~"stackrox"`).
					TopK(25),
				"{{ pod }}: {{ container }}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 25 stackrox container CPU percent", "percent",
			dashboard.GridPos{X: 12, Y: 2, W: 12, H: 8},
			t.track("stackroxContainerCPU",
				mg.Q(mg.MetricContainerCPU, `container!="POD",name!="",namespace!="",namespace=~"stackrox"`).
					IRate(intervalVar).
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByContainer, mg.GroupByNamespace, mg.GroupByName, "service").
					Multiply("100").
					TopK(25),
				"{{ pod }}: {{ container }}"),
		))
}

// Row: Cluster Kubelet
func ocpClusterKubeletRow(t *queryTracker) *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("Cluster Kubelet").
		Collapsed(true).
		GridPos(dashboard.GridPos{X: 0, Y: 0, W: 24, H: 1}).
		WithPanel(genericLegendTimeSeries("Top 10 Kubelet CPU usage", "percent",
			dashboard.GridPos{X: 0, Y: 3, W: 12, H: 8},
			t.track("kubeletCPU",
				mg.Q(mg.MetricProcessCPU, `service="kubelet",job="kubelet"`).
					IRate(intervalVar).Multiply("100").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByNode},
						mg.NodeRoleFilter(mg.RoleWorker)).
					TopK(10),
				"kubelet - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 crio CPU usage", "percent",
			dashboard.GridPos{X: 12, Y: 3, W: 12, H: 8},
			t.track("crioCPU",
				mg.Q(mg.MetricProcessCPU, `service="kubelet",job="crio"`).
					IRate(intervalVar).Multiply("100").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByNode},
						mg.NodeRoleFilter(mg.RoleWorker)).
					TopK(10),
				"crio - {{node}}"),
		)).
		WithPanel(genericLegendCounterTimeSeries("Top 10 Kubelet memory usage", "bytes",
			dashboard.GridPos{X: 0, Y: 11, W: 12, H: 8},
			t.track("kubeletMemory",
				mg.Q(mg.MetricProcessMemory, `service="kubelet",job="kubelet"`).
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByNode},
						mg.NodeRoleFilter(mg.RoleWorker)).
					TopK(10),
				"kubelet - {{node}}"),
		)).
		WithPanel(genericLegendCounterTimeSeries("Top 10 crio memory usage", "bytes",
			dashboard.GridPos{X: 12, Y: 11, W: 12, H: 8},
			t.track("crioMemory",
				mg.Q(mg.MetricProcessMemory, `service="kubelet",job="crio"`).
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByNode},
						mg.NodeRoleFilter(mg.RoleWorker)).
					TopK(10),
				"crio - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("inodes usage in /run", "percent",
			dashboard.GridPos{X: 0, Y: 19, W: 12, H: 8},
			t.trackRaw("nodeInodesUsageRun", `(1 - node_filesystem_files_free{fstype!="",mountpoint="/run"} / node_filesystem_files{fstype!="",mountpoint="/run"}) * 100`, "{{instance}}"),
		)).
		WithPanel(genericLegendCounterSumRightHandTimeSeries("inodes count in /run", "none",
			dashboard.GridPos{X: 12, Y: 19, W: 12, H: 8},
			t.track("nodeInodesCountRun",
				mg.Q(mg.MetricNodeFsFiles, `fstype!="",mountpoint="/run"`).
					Sub(mg.Q(mg.MetricNodeFsFilesFree, `fstype!="",mountpoint="/run"`)),
				"{{instance}}"),
			promQuery(
				mg.Q(mg.MetricNodeFsFiles, `fstype!="",mountpoint="/run"`).
					Sub(mg.Q(mg.MetricNodeFsFilesFree, `fstype!="",mountpoint="/run"`)).
					Agg(mg.AggSum).String(),
				"sum"),
		))
}

// Row: Cluster Details
func ocpClusterDetailsRow(t *queryTracker) *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("Cluster Details").
		Collapsed(true).
		GridPos(dashboard.GridPos{X: 0, Y: 0, W: 24, H: 1}).
		WithPanel(genericStat("Current Node Count",
			dashboard.GridPos{X: 0, Y: 4, W: 8, H: 3},
			t.track("kubeNodeInfo", mg.Raw("kube_node_info{}").Agg(mg.AggSum), "Number of nodes"),
			t.track("kubeNodeStatusCondition",
				mg.Q(mg.MetricKubeNodeStatusCondition, `status="true"`).
					Agg(mg.AggSum, mg.GroupByCondition).
					Gt("0"),
				"Node: {{ condition }}"),
		)).
		WithPanel(genericStat("Current Namespace Count",
			dashboard.GridPos{X: 8, Y: 4, W: 8, H: 3},
			t.track("kubeNamespacePhase",
				mg.Q(mg.MetricKubeNamespacePhase, "").
					Agg(mg.AggSum, mg.GroupByPhase),
				"{{ phase }}"),
		)).
		WithPanel(genericStat("Current Pod Count",
			dashboard.GridPos{X: 16, Y: 4, W: 8, H: 3},
			t.track("kubePodStatusPhase",
				mg.Q(mg.MetricKubePodStatusPhase, "").
					Agg(mg.AggSum, mg.GroupByPhase).
					Gt("0"),
				"{{ phase}} Pods"),
		)).
		WithPanel(genericTimeSeries("Number of nodes", "none",
			dashboard.GridPos{X: 0, Y: 12, W: 8, H: 8},
			promQuery(mg.Raw("kube_node_info{}").Agg(mg.AggSum).String(), "Number of nodes"),
			promQuery(
				mg.Q(mg.MetricKubeNodeStatusCondition, `status="true"`).
					Agg(mg.AggSum, mg.GroupByCondition).
					Gt("0").String(),
				"Node: {{ condition }}"),
		)).
		WithPanel(genericTimeSeries("Namespace count", "none",
			dashboard.GridPos{X: 8, Y: 12, W: 8, H: 8},
			promQuery(
				mg.Q(mg.MetricKubeNamespacePhase, "").
					Agg(mg.AggSum, mg.GroupByPhase).
					Gt("0").String(),
				"{{ phase }} namespaces"),
		)).
		WithPanel(genericTimeSeries("Pod count", "none",
			dashboard.GridPos{X: 16, Y: 12, W: 8, H: 8},
			promQuery(
				mg.Q(mg.MetricKubePodStatusPhase, "").
					Agg(mg.AggSum, mg.GroupByPhase).String(),
				"{{phase}} pods"),
		)).
		WithPanel(genericTimeSeries("Secret & configmap count", "none",
			dashboard.GridPos{X: 0, Y: 20, W: 8, H: 8},
			t.track("kubeSecretInfo", mg.Q(mg.MetricKubeSecretInfo, "").Agg(mg.AggCount), "secrets"),
			t.track("kubeConfigmapInfo", mg.Q(mg.MetricKubeConfigmapInfo, "").Agg(mg.AggCount), "Configmaps"),
		)).
		WithPanel(genericTimeSeries("Deployment count", "none",
			dashboard.GridPos{X: 8, Y: 20, W: 8, H: 8},
			t.track("kubeDeploymentReplicas", mg.Raw("kube_deployment_spec_replicas{}").Agg(mg.AggCount), "Deployments"),
		)).
		WithPanel(genericTimeSeries("Services count", "none",
			dashboard.GridPos{X: 16, Y: 20, W: 8, H: 8},
			t.track("kubeServiceInfo", mg.Q(mg.MetricKubeServiceInfo, "").Agg(mg.AggCount), "Services"),
		)).
		WithPanel(genericTimeSeries("Routes count", "none",
			dashboard.GridPos{X: 0, Y: 20, W: 8, H: 8},
			t.track("openshiftRouteInfo", mg.Raw("openshift_route_info{}").Agg(mg.AggCount), "Routes"),
		)).
		WithPanel(genericTimeSeries("Alerts", "none",
			dashboard.GridPos{X: 8, Y: 20, W: 8, H: 8},
			t.trackRaw("alerts", `topk(10,sum(ALERTS{severity!="none"}) by (alertname, severity))`, "{{severity}}: {{alertname}}"),
		)).
		WithPanel(genericLegendTimeSeries("Pod Distribution", "none",
			dashboard.GridPos{X: 16, Y: 20, W: 8, H: 8},
			t.track("kubePodDistribution",
				mg.Q(mg.MetricKubePodInfo, "").
					Agg(mg.AggCount, mg.GroupByNode),
				"{{ node }}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 container CPU", "percent",
			dashboard.GridPos{X: 0, Y: 28, W: 12, H: 8},
			t.track("containerCPUTop10",
				mg.Q(mg.MetricContainerCPU, `namespace!="",container!="POD",name!=""`).
					IRate(intervalVar).Multiply("100").
					TopK(10),
				"{{ namespace }} - {{ name }} - {{ node }}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 container RSS", "bytes",
			dashboard.GridPos{X: 12, Y: 28, W: 12, H: 8},
			t.track("containerMemoryRSSTop10",
				mg.Q(mg.MetricContainerMemoryRSS, `namespace!="",container!="POD",name!=""`).
					TopK(10),
				"{{ namespace }} - {{ name }} - {{ node }}"),
		)).
		WithPanel(genericLegendTimeSeries("container RSS system.slice", "bytes",
			dashboard.GridPos{X: 12, Y: 36, W: 12, H: 8},
			t.track("containerMemoryRSSSystemSlice",
				mg.Q(mg.MetricContainerMemoryRSS, `id="/system.slice"`).
					Agg(mg.AggSum, mg.GroupByNode),
				"system.slice - {{ node }}"),
		)).
		WithPanel(genericTimeSeries("Goroutines count", "none",
			dashboard.GridPos{X: 0, Y: 36, W: 12, H: 8},
			t.trackRaw("goGoroutines", `topk(10, sum(go_goroutines{}) by (job,instance))`, "{{ job }} - {{ instance }}"),
		))
}

// Row: Cluster Operators Details
func ocpClusterOperatorsDetailsRow(t *queryTracker) *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("Cluster Operators Details").
		Collapsed(true).
		GridPos(dashboard.GridPos{X: 0, Y: 0, W: 24, H: 1}).
		WithPanel(genericStat("Cluster operators overview",
			dashboard.GridPos{X: 0, Y: 4, W: 24, H: 3},
			t.track("clusterOperatorConditions",
				mg.Q(mg.MetricClusterOperatorConditions, `condition!=""`).
					Agg(mg.AggSum, mg.GroupByCondition),
				"{{ condition }}"),
		)).
		WithPanel(genericLegendTimeSeries("Cluster operators information", "none",
			dashboard.GridPos{X: 0, Y: 4, W: 8, H: 8},
			t.track("clusterOperatorInfo", mg.Q(mg.MetricClusterOperatorConditions, `name!="",reason!=""`), "{{name}} - {{reason}}"),
		)).
		WithPanel(genericLegendTimeSeries("Cluster operators degraded", "none",
			dashboard.GridPos{X: 8, Y: 4, W: 8, H: 8},
			t.track("clusterOperatorDegraded", mg.Q(mg.MetricClusterOperatorConditions, `condition="Degraded",name!="",reason!=""`), "{{name}} - {{reason}}"),
		))
}

func ocpNodeRow(t *queryTracker, nodeVar string, role mg.NodeRole) *dashboard.RowBuilder {
	instanceFilter := `instance=~"$` + nodeVar + `"`
	nodeFilter := `node=~"$` + nodeVar + `"`
	roleStr := string(role)

	row := dashboard.NewRowBuilder(roleStr + ": $" + nodeVar).
		Collapsed(true).
		GridPos(dashboard.GridPos{X: 0, Y: 0, W: 24, H: 8}).
		Repeat(nodeVar).
		WithPanel(genericLegendTimeSeries("CPU Basic: $"+nodeVar, "percent",
			dashboard.GridPos{X: 0, Y: 1, W: 12, H: 8},
			t.track("nodeCPU_"+roleStr,
				mg.Q(mg.MetricNodeCPU, instanceFilter+`,job=~".*"`).
					IRate(intervalVar).
					Agg(mg.AggSum, mg.GroupByInstance, mg.GroupByMode).
					Multiply("100"),
				"Busy {{mode}}"),
		)).
		WithPanel(genericLegendTimeSeries("Disk throughput: $"+nodeVar, "Bps",
			dashboard.GridPos{X: 0, Y: 8, W: 12, H: 8},
			t.track("nodeDiskRead_"+roleStr,
				mg.Q(mg.MetricNodeDiskRead, `device=~"$block_device",`+instanceFilter).
					Rate(intervalVar),
				"{{ device }} - read"),
			t.track("nodeDiskWritten_"+roleStr,
				mg.Q(mg.MetricNodeDiskWritten, `device=~"$block_device",`+instanceFilter).
					Rate(intervalVar),
				"{{ device }} - write"),
		)).
		WithPanel(genericLegendTimeSeries("Disk IOPS: $"+nodeVar, "iops",
			dashboard.GridPos{X: 12, Y: 8, W: 12, H: 8},
			t.track("nodeDiskReadsCompleted_"+roleStr,
				mg.Raw("node_disk_reads_completed_total{device=~\"$block_device\","+instanceFilter+"}").
					Rate(intervalVar),
				"{{ device }} - read"),
			t.track("nodeDiskWritesCompleted_"+roleStr,
				mg.Raw("node_disk_writes_completed_total{device=~\"$block_device\","+instanceFilter+"}").
					Rate(intervalVar),
				"{{ device }} - write"),
		)).
		WithPanel(genericLegendTimeSeries("Network Utilization: $"+nodeVar, "bps",
			dashboard.GridPos{X: 0, Y: 16, W: 12, H: 8},
			t.track("nodeNetworkRx_"+roleStr,
				mg.Q(mg.MetricNodeNetworkRx, instanceFilter+`,device=~"$net_device"`).
					Rate(intervalVar).Multiply("8"),
				"{{instance}} - {{device}} - RX"),
			t.track("nodeNetworkTx_"+roleStr,
				mg.Q(mg.MetricNodeNetworkTx, instanceFilter+`,device=~"$net_device"`).
					Rate(intervalVar).Multiply("8"),
				"{{instance}} - {{device}} - TX"),
		)).
		WithPanel(genericLegendTimeSeries("Network Packets: $"+nodeVar, "pps",
			dashboard.GridPos{X: 12, Y: 16, W: 12, H: 8},
			t.track("nodeNetworkRxPackets_"+roleStr,
				mg.Raw("node_network_receive_packets_total{"+instanceFilter+`,device=~"$net_device"}`).
					Rate(intervalVar),
				"{{instance}} - {{device}} - RX"),
			t.track("nodeNetworkTxPackets_"+roleStr,
				mg.Raw("node_network_transmit_packets_total{"+instanceFilter+`,device=~"$net_device"}`).
					Rate(intervalVar),
				"{{instance}} - {{device}} - TX"),
		)).
		WithPanel(genericLegendTimeSeries("Network packets drop: $"+nodeVar, "pps",
			dashboard.GridPos{X: 0, Y: 24, W: 12, H: 8},
			t.track("nodeNetworkRxDrop_"+roleStr,
				mg.Q(mg.MetricNodeNetworkRxDrop, instanceFilter).
					Rate(intervalVar).TopK(10),
				"rx-drop-{{ device }}"),
			t.track("nodeNetworkTxDrop_"+roleStr,
				mg.Raw("node_network_transmit_drop_total{"+instanceFilter+"}").
					Rate(intervalVar).TopK(10),
				"tx-drop-{{ device }}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 container CPU: $"+nodeVar, "percent",
			dashboard.GridPos{X: 0, Y: 24, W: 12, H: 8},
			t.track("containerCPUTop10_"+roleStr,
				mg.Q(mg.MetricContainerCPU, `container!="POD",name!="",`+nodeFilter+`,namespace!="",namespace=~"$namespace"`).
					IRate(intervalVar).
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByContainer, mg.GroupByNamespace, mg.GroupByName, "service").
					Multiply("100").
					TopK(10),
				"{{ pod }}: {{ container }}"),
		))

	if role == "Master" || role == "master" {
		row = row.
			WithPanel(genericLegendCounterTimeSeries("System Memory: $"+nodeVar, "bytes",
				dashboard.GridPos{X: 12, Y: 1, W: 12, H: 8},
				t.track("nodeMemoryActive_"+roleStr, mg.Q(mg.MetricNodeMemoryActive, instanceFilter), "Active"),
				t.track("nodeMemoryTotal_"+roleStr, mg.Q(mg.MetricNodeMemoryTotal, instanceFilter), "Total"),
				promQuery(
					mg.Q(mg.MetricNodeMemoryCached, instanceFilter).
						Sub(mg.Raw("-")).String()+"node_memory_Buffers_bytes{"+instanceFilter+"}",
					"Cached + Buffers"),
				t.track("nodeMemoryAvailable_"+roleStr, mg.Q(mg.MetricNodeMemoryAvailable, instanceFilter), "Available"),
				t.track("nodeMemoryUsed_"+roleStr,
					mg.Q(mg.MetricNodeMemoryTotal, instanceFilter).
						Sub(mg.Q(mg.MetricNodeMemoryFree, instanceFilter).Paren()),
					"Used"),
			))
	}

	return row
}

// Row: Master (repeats on _master_node)
// TODO: convert remaining promQuery calls to t.track so these panels are included in the metrics profile
func ocpMasterRow(t *queryTracker) *dashboard.RowBuilder {
	nodeVar := "_master_node"
	instanceFilter := `instance=~"$` + nodeVar + `"`
	nodeFilter := `node=~"$` + nodeVar + `"`

	return dashboard.NewRowBuilder("Master: $_master_node").
		Collapsed(true).
		GridPos(dashboard.GridPos{X: 0, Y: 0, W: 24, H: 8}).
		Repeat("_master_node").
		WithPanel(genericLegendTimeSeries("CPU Basic: $_master_node", "percent",
			dashboard.GridPos{X: 0, Y: 1, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricNodeCPU, instanceFilter+`,job=~".*"`).
					IRate(intervalVar).
					Agg(mg.AggSum, mg.GroupByInstance, mg.GroupByMode).
					Multiply("100").String(),
				"Busy {{mode}}"),
		)).
		WithPanel(genericLegendCounterTimeSeries("System Memory: $_master_node", "bytes",
			dashboard.GridPos{X: 12, Y: 1, W: 12, H: 8},
			promQuery(q(mg.MetricNodeMemoryActive, instanceFilter), "Active"),
			promQuery(q(mg.MetricNodeMemoryTotal, instanceFilter), "Total"),
			promQuery(`node_memory_Cached_bytes{`+instanceFilter+`} + node_memory_Buffers_bytes{`+instanceFilter+`}`, "Cached + Buffers"),
			promQuery(q(mg.MetricNodeMemoryAvailable, instanceFilter), "Available"),
			promQuery(`(node_memory_MemTotal_bytes{`+instanceFilter+`} - (node_memory_MemFree_bytes{`+instanceFilter+`} + node_memory_Buffers_bytes{`+instanceFilter+`} +  node_memory_Cached_bytes{`+instanceFilter+`}))`, "Used"),
		)).
		WithPanel(genericLegendTimeSeries("Disk throughput: $_master_node", "Bps",
			dashboard.GridPos{X: 0, Y: 8, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricNodeDiskRead, `device=~"$block_device",`+instanceFilter).
					Rate(intervalVar).String(),
				"{{ device }} - read"),
			promQuery(
				mg.Q(mg.MetricNodeDiskWritten, `device=~"$block_device",`+instanceFilter).
					Rate(intervalVar).String(),
				"{{ device }} - write"),
		)).
		WithPanel(genericLegendTimeSeries("Disk IOPS: $_master_node", "iops",
			dashboard.GridPos{X: 12, Y: 8, W: 12, H: 8},
			promQuery(
				mg.Raw("node_disk_reads_completed_total{device=~\"$block_device\","+instanceFilter+"}").
					Rate(intervalVar).String(),
				"{{ device }} - read"),
			promQuery(
				mg.Raw("node_disk_writes_completed_total{device=~\"$block_device\","+instanceFilter+"}").
					Rate(intervalVar).String(),
				"{{ device }} - write"),
		)).
		WithPanel(genericLegendTimeSeries("Network Utilization: $_master_node", "bps",
			dashboard.GridPos{X: 0, Y: 16, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricNodeNetworkRx, instanceFilter+`,device=~"$net_device"`).
					Rate(intervalVar).Multiply("8").String(),
				"{{instance}} - {{device}} - RX"),
			promQuery(
				mg.Q(mg.MetricNodeNetworkTx, instanceFilter+`,device=~"$net_device"`).
					Rate(intervalVar).Multiply("8").String(),
				"{{instance}} - {{device}} - TX"),
		)).
		WithPanel(genericLegendTimeSeries("Network Packets: $_master_node", "pps",
			dashboard.GridPos{X: 12, Y: 16, W: 12, H: 8},
			promQuery(
				mg.Raw("node_network_receive_packets_total{"+instanceFilter+`,device=~"$net_device"}`).
					Rate(intervalVar).String(),
				"{{instance}} - {{device}} - RX"),
			promQuery(
				mg.Raw("node_network_transmit_packets_total{"+instanceFilter+`,device=~"$net_device"}`).
					Rate(intervalVar).String(),
				"{{instance}} - {{device}} - TX"),
		)).
		WithPanel(genericLegendTimeSeries("Network packets drop: $_master_node", "pps",
			dashboard.GridPos{X: 0, Y: 24, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricNodeNetworkRxDrop, instanceFilter).
					Rate(intervalVar).TopK(10).String(),
				"rx-drop-{{ device }}"),
			promQuery(
				mg.Raw("node_network_transmit_drop_total{"+instanceFilter+"}").
					Rate(intervalVar).TopK(10).String(),
				"tx-drop-{{ device }}"),
		)).
		WithPanel(genericLegendCounterTimeSeries("Conntrack stats: $_master_node", "",
			dashboard.GridPos{X: 12, Y: 24, W: 12, H: 8},
			promQuery(`node_nf_conntrack_entries{`+instanceFilter+`}`, "conntrack_entries"),
			promQuery(`node_nf_conntrack_entries_limit{`+instanceFilter+`}`, "conntrack_limit"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 container CPU: $_master_node", "percent",
			dashboard.GridPos{X: 0, Y: 24, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerCPU, `container!="POD",name!="",`+nodeFilter+`,namespace!="",namespace=~"$namespace"`).
					IRate(intervalVar).
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByContainer, mg.GroupByNamespace, mg.GroupByName, "service").
					Multiply("100").
					TopK(10).String(),
				"{{ pod }}: {{ container }}"),
		)).
		WithPanel(genericLegendCounterTimeSeries("Top 10 container RSS: $_master_node", "bytes",
			dashboard.GridPos{X: 12, Y: 24, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerMemoryRSS, `container!="POD",name!="",`+nodeFilter+`,namespace!="",namespace=~"$namespace"`).
					TopK(10).String(),
				"{{ pod }}: {{ container }}"),
		)).
		WithPanel(genericLegendTimeSeries("cgroup CPU: $_master_node", "percent",
			dashboard.GridPos{X: 0, Y: 32, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerCPU, cgroupIDFilter+", "+nodeFilter).
					Rate(intervalVar).
					Agg(mg.AggSum, mg.GroupByID).
					Multiply("100").String(),
				"{{ id }}"),
		)).
		WithPanel(genericLegendCounterTimeSeries("cgroup RSS: $_master_node", "bytes",
			dashboard.GridPos{X: 12, Y: 32, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerMemoryRSS, cgroupIDFilterWithJournald+", "+nodeFilter).
					Agg(mg.AggSum, mg.GroupByID).String(),
				"{{ id }}"),
		)).
		WithPanel(genericLegendTimeSeries("Pod fs rw rate: $_master_node", "Bps",
			dashboard.GridPos{X: 0, Y: 32, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerFSWrites, fsWriteFilter+", "+nodeFilter+`, pod!=""`).
					Rate(intervalVar).
					Agg(mg.AggSum, mg.GroupByDevice, mg.GroupByPod).String(),
				"{{ pod }}: {{ device }} - write"),
			promQuery(
				mg.Raw(`container_fs_reads_bytes_total{`+fsReadFilter+", "+nodeFilter+`, pod!=""}`).
					Rate(intervalVar).
					Agg(mg.AggSum, mg.GroupByDevice, mg.GroupByPod).String(),
				"{{ pod }}: {{ device }} - read"),
		)).
		WithPanel(genericLegendTimeSeries("cgroup fs rw rate: $_master_node", "Bps",
			dashboard.GridPos{X: 12, Y: 32, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerFSWrites, cgroupFSIDFilter+", "+nodeFilter).
					Rate(intervalVar).
					Agg(mg.AggSum, mg.GroupByDevice, mg.GroupByID).String(),
				"{{ id }}: {{ device }} - write"),
			promQuery(
				mg.Raw(`container_fs_reads_bytes_total{`+cgroupFSIDFilter+", "+nodeFilter+"}").
					Rate(intervalVar).
					Agg(mg.AggSum, mg.GroupByDevice, mg.GroupByID).String(),
				"{{ id }}: {{ device }} - read"),
		))
}

// Row: Worker (repeats on _worker_node)
// TODO: convert remaining promQuery calls to t.track so these panels are included in the metrics profile
func ocpWorkerRow(t *queryTracker) *dashboard.RowBuilder {
	nodeVar := "_worker_node"
	instanceFilter := `instance=~"$` + nodeVar + `"`
	nodeFilter := `node=~"$` + nodeVar + `"`

	return dashboard.NewRowBuilder("Worker: $_worker_node").
		Collapsed(true).
		GridPos(dashboard.GridPos{X: 0, Y: 0, W: 24, H: 8}).
		Repeat("_worker_node").
		WithPanel(genericLegendTimeSeries("CPU Basic: $_worker_node", "percent",
			dashboard.GridPos{X: 0, Y: 1, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricNodeCPU, instanceFilter+`,job=~".*"`).
					IRate(intervalVar).
					Agg(mg.AggSum, mg.GroupByInstance, mg.GroupByMode).
					Multiply("100").String(),
				"Busy {{mode}}"),
		)).
		WithPanel(genericLegendCounterTimeSeries("System Memory: $_worker_node", "bytes",
			dashboard.GridPos{X: 12, Y: 1, W: 12, H: 8},
			promQuery(q(mg.MetricNodeMemoryActive, instanceFilter), "Active"),
			promQuery(q(mg.MetricNodeMemoryTotal, instanceFilter), "Total"),
			promQuery(`node_memory_Cached_bytes{`+instanceFilter+`} + node_memory_Buffers_bytes{`+instanceFilter+`}`, "Cached + Buffers"),
			promQuery(q(mg.MetricNodeMemoryAvailable, instanceFilter), "Available"),
			promQuery(`(node_memory_MemTotal_bytes{`+instanceFilter+`} - (node_memory_MemFree_bytes{`+instanceFilter+`} + node_memory_Buffers_bytes{`+instanceFilter+`} +  node_memory_Cached_bytes{`+instanceFilter+`}))`, "Used"),
		)).
		WithPanel(genericLegendTimeSeries("Disk throughput: $_worker_node", "Bps",
			dashboard.GridPos{X: 0, Y: 8, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricNodeDiskRead, `device=~"$block_device",`+instanceFilter).
					Rate(intervalVar).String(),
				"{{ device }} - read"),
			promQuery(
				mg.Q(mg.MetricNodeDiskWritten, `device=~"$block_device",`+instanceFilter).
					Rate(intervalVar).String(),
				"{{ device }} - write"),
		)).
		WithPanel(genericLegendTimeSeries("Disk IOPS: $_worker_node", "iops",
			dashboard.GridPos{X: 12, Y: 8, W: 12, H: 8},
			promQuery(
				mg.Raw("node_disk_reads_completed_total{device=~\"$block_device\","+instanceFilter+"}").
					Rate(intervalVar).String(),
				"{{ device }} - read"),
			promQuery(
				mg.Raw("node_disk_writes_completed_total{device=~\"$block_device\","+instanceFilter+"}").
					Rate(intervalVar).String(),
				"{{ device }} - write"),
		)).
		WithPanel(genericLegendTimeSeries("Network Utilization: $_worker_node", "bps",
			dashboard.GridPos{X: 0, Y: 16, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricNodeNetworkRx, instanceFilter+`,device=~"$net_device"`).
					Rate(intervalVar).Multiply("8").String(),
				"{{instance}} - {{device}} - RX"),
			promQuery(
				mg.Q(mg.MetricNodeNetworkTx, instanceFilter+`,device=~"$net_device"`).
					Rate(intervalVar).Multiply("8").String(),
				"{{instance}} - {{device}} - TX"),
		)).
		WithPanel(genericLegendTimeSeries("Network Packets: $_worker_node", "pps",
			dashboard.GridPos{X: 12, Y: 16, W: 12, H: 8},
			promQuery(
				mg.Raw("node_network_receive_packets_total{"+instanceFilter+`,device=~"$net_device"}`).
					Rate(intervalVar).String(),
				"{{instance}} - {{device}} - RX"),
			promQuery(
				mg.Raw("node_network_transmit_packets_total{"+instanceFilter+`,device=~"$net_device"}`).
					Rate(intervalVar).String(),
				"{{instance}} - {{device}} - TX"),
		)).
		WithPanel(genericLegendTimeSeries("Network packets drop: $_worker_node", "pps",
			dashboard.GridPos{X: 0, Y: 24, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricNodeNetworkRxDrop, instanceFilter).
					Rate(intervalVar).TopK(10).String(),
				"rx-drop-{{ device }}"),
			promQuery(
				mg.Raw("node_network_transmit_drop_total{"+instanceFilter+"}").
					Rate(intervalVar).TopK(10).String(),
				"tx-drop-{{ device }}"),
		)).
		WithPanel(genericLegendCounterTimeSeries("Conntrack stats: $_worker_node", "",
			dashboard.GridPos{X: 12, Y: 24, W: 12, H: 8},
			promQuery(`node_nf_conntrack_entries{`+instanceFilter+`}`, "conntrack_entries"),
			promQuery(`node_nf_conntrack_entries_limit{`+instanceFilter+`}`, "conntrack_limit"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 container CPU: $_worker_node", "percent",
			dashboard.GridPos{X: 0, Y: 32, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerCPU, `container!="POD",name!="",`+nodeFilter+`,namespace!="",namespace=~"$namespace"`).
					IRate(intervalVar).
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByContainer, mg.GroupByNamespace, mg.GroupByName, "service").
					Multiply("100").
					TopK(10).String(),
				"{{ pod }}: {{ container }}"),
		)).
		WithPanel(genericLegendCounterTimeSeries("Top 10 container RSS: $_worker_node", "bytes",
			dashboard.GridPos{X: 12, Y: 32, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerMemoryRSS, `container!="POD",name!="",`+nodeFilter+`,namespace!="",namespace=~"$namespace"`).
					TopK(10).String(),
				"{{ pod }}: {{ container }}"),
		)).
		WithPanel(genericLegendTimeSeries("cgroup CPU: $_worker_node", "percent",
			dashboard.GridPos{X: 0, Y: 40, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerCPU, cgroupIDFilter+", "+nodeFilter).
					Rate(intervalVar).
					Agg(mg.AggSum, mg.GroupByID).
					Multiply("100").String(),
				"{{ id }}"),
		)).
		WithPanel(genericLegendCounterTimeSeries("cgroup RSS: $_worker_node", "bytes",
			dashboard.GridPos{X: 12, Y: 40, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerMemoryRSS, cgroupIDFilterWithJournald+", "+nodeFilter).
					Agg(mg.AggSum, mg.GroupByID).String(),
				"{{ id }}"),
		))
}

// Row: Infra (repeats on _infra_node)
// TODO: convert remaining promQuery calls to t.track so these panels are included in the metrics profile
func ocpInfraRow(t *queryTracker) *dashboard.RowBuilder {
	nodeVar := "_infra_node"
	instanceFilter := `instance=~"$` + nodeVar + `"`
	nodeFilter := `node=~"$` + nodeVar + `"`

	return dashboard.NewRowBuilder("Infra: $_infra_node").
		Collapsed(true).
		GridPos(dashboard.GridPos{X: 0, Y: 0, W: 24, H: 8}).
		Repeat("_infra_node").
		WithPanel(genericLegendTimeSeries("CPU Basic: $_infra_node", "percent",
			dashboard.GridPos{X: 0, Y: 1, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricNodeCPU, instanceFilter+`,job=~".*"`).
					IRate(intervalVar).
					Agg(mg.AggSum, mg.GroupByInstance, mg.GroupByMode).
					Multiply("100").String(),
				"Busy {{mode}}"),
		)).
		WithPanel(genericLegendTimeSeries("System Memory: $_infra_node", "bytes",
			dashboard.GridPos{X: 12, Y: 1, W: 12, H: 8},
			promQuery(q(mg.MetricNodeMemoryActive, instanceFilter), "Active"),
			promQuery(q(mg.MetricNodeMemoryTotal, instanceFilter), "Total"),
			promQuery(`node_memory_Cached_bytes{`+instanceFilter+`} + node_memory_Buffers_bytes{`+instanceFilter+`}`, "Cached + Buffers"),
			promQuery(q(mg.MetricNodeMemoryAvailable, instanceFilter), "Available"),
			promQuery(`(node_memory_MemTotal_bytes{`+instanceFilter+`} - (node_memory_MemFree_bytes{`+instanceFilter+`} + node_memory_Buffers_bytes{`+instanceFilter+`} +  node_memory_Cached_bytes{`+instanceFilter+`}))`, "Used"),
		)).
		WithPanel(genericLegendTimeSeries("Disk throughput: $_infra_node", "Bps",
			dashboard.GridPos{X: 0, Y: 8, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricNodeDiskRead, `device=~"$block_device",`+instanceFilter).
					Rate(intervalVar).String(),
				"{{ device }} - read"),
			promQuery(
				mg.Q(mg.MetricNodeDiskWritten, `device=~"$block_device",`+instanceFilter).
					Rate(intervalVar).String(),
				"{{ device }} - write"),
		)).
		WithPanel(genericLegendTimeSeries("Disk IOPS: $_infra_node", "iops",
			dashboard.GridPos{X: 12, Y: 8, W: 12, H: 8},
			promQuery(
				mg.Raw("node_disk_reads_completed_total{device=~\"$block_device\","+instanceFilter+"}").
					Rate(intervalVar).String(),
				"{{ device }} - read"),
			promQuery(
				mg.Raw("node_disk_writes_completed_total{device=~\"$block_device\","+instanceFilter+"}").
					Rate(intervalVar).String(),
				"{{ device }} - write"),
		)).
		WithPanel(genericLegendTimeSeries("Network Utilization: $_infra_node", "bps",
			dashboard.GridPos{X: 0, Y: 16, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricNodeNetworkRx, instanceFilter+`,device=~"$net_device"`).
					Rate(intervalVar).Multiply("8").String(),
				"{{instance}} - {{device}} - RX"),
			promQuery(
				mg.Q(mg.MetricNodeNetworkTx, instanceFilter+`,device=~"$net_device"`).
					Rate(intervalVar).Multiply("8").String(),
				"{{instance}} - {{device}} - TX"),
		)).
		WithPanel(genericLegendTimeSeries("Network Packets: $_infra_node", "pps",
			dashboard.GridPos{X: 12, Y: 16, W: 12, H: 8},
			promQuery(
				mg.Raw("node_network_receive_packets_total{"+instanceFilter+`,device=~"$net_device"}`).
					Rate(intervalVar).String(),
				"{{instance}} - {{device}} - RX"),
			promQuery(
				mg.Raw("node_network_transmit_packets_total{"+instanceFilter+`,device=~"$net_device"}`).
					Rate(intervalVar).String(),
				"{{instance}} - {{device}} - TX"),
		)).
		WithPanel(genericLegendTimeSeries("Network packets drop: $_infra_node", "pps",
			dashboard.GridPos{X: 0, Y: 24, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricNodeNetworkRxDrop, instanceFilter).
					Rate(intervalVar).TopK(10).String(),
				"rx-drop-{{ device }}"),
			promQuery(
				mg.Raw("node_network_transmit_drop_total{"+instanceFilter+"}").
					Rate(intervalVar).TopK(10).String(),
				"tx-drop-{{ device }}"),
		)).
		WithPanel(genericLegendTimeSeries("Conntrack stats: $_infra_node", "",
			dashboard.GridPos{X: 12, Y: 24, W: 12, H: 8},
			promQuery(`node_nf_conntrack_entries{`+instanceFilter+`}`, "conntrack_entries"),
			promQuery(`node_nf_conntrack_entries_limit{`+instanceFilter+`}`, "conntrack_limit"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 container CPU: $_infra_node", "percent",
			dashboard.GridPos{X: 0, Y: 24, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerCPU, `container!="POD",name!="",`+nodeFilter+`,namespace!="",namespace=~"$namespace"`).
					IRate(intervalVar).
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByContainer, mg.GroupByNamespace, mg.GroupByName, "service").
					Multiply("100").
					TopK(10).String(),
				"{{ pod }}: {{ container }}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 container RSS: $_infra_node", "bytes",
			dashboard.GridPos{X: 12, Y: 24, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerMemoryRSS, `container!="POD",name!="",`+nodeFilter+`,namespace!="",namespace=~"$namespace"`).
					TopK(10).String(),
				"{{ pod }}: {{ container }}"),
		))
}
