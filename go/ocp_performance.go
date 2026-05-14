package main

import (
	"github.com/grafana/grafana-foundation-sdk/go/cog"
	"github.com/grafana/grafana-foundation-sdk/go/dashboard"
	mg "github.com/kube-burner/metrics-generator/pkg/metrics"
)

const (
	intervalVar = mg.RateInterval("$interval")

	cgroupIDFilter = `job=~".*", id =~"/system.slice|/system.slice/kubelet.service|/.*/ovs-vswitchd.service|/system.slice/crio.service|/system.slice/systemd-journald.service|/.*/ovsdb-server.service|/system.slice/systemd-udevd.service|/kubepods.slice"`
	cgroupIDFilterWithJournald = `job=~".*", id =~"/system.slice|/system.slice/kubelet.service|/.*/ovs-vswitchd.service|/system.slice/crio.service|/system.slice/systemd-journald.service|/system.slice/.*.service|/system.slice/systemd-udevd.service|/kubepods.slice"`

	fsWriteFilter = `device!~".+dm.+"`
	fsReadFilter  = `device!~".+dm.+"`
	cgroupFSIDFilter = `device!~".+dm.+", id =~"/system.slice/kubelet.service|/.*/ovs-vswitchd.service|/system.slice/crio.service|/system.slice/systemd-journald.service|/.*/ovsdb-server.service|/system.slice/systemd-udevd.service|/kubepods.slice"`
)

func q(metric mg.Metric, filters string) string {
	return mg.Q(metric, filters).String()
}

func buildOCPPerformanceDashboard() *dashboard.DashboardBuilder {
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
			Query(dashboard.StringOrMap{String: cog.ToPtr(q(mg.MetricKubePodInfo, `namespace!="(cluster-density.*|node-density-.*)"`) + ",namespace)")}).
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
		// Row: Cluster-at-a-Glance
		WithRow(ocpClusterAtAGlanceRow()).
		// Row: OVN
		WithRow(ocpOVNRow()).
		// Row: Monitoring stack
		WithRow(ocpMonitoringStackRow()).
		// Row: Cluster Kubelet
		WithRow(ocpClusterKubeletRow()).
		// Row: Cluster Details
		WithRow(ocpClusterDetailsRow()).
		// Row: Cluster Operators Details
		WithRow(ocpClusterOperatorsDetailsRow()).
		// Row: Master
		WithRow(ocpMasterRow()).
		// Row: Worker
		WithRow(ocpWorkerRow()).
		// Row: Infra
		WithRow(ocpInfraRow()).
		// Row: Stackrox
		WithRow(ocpStackroxRow())
}


// Row: Cluster-at-a-Glance
func ocpClusterAtAGlanceRow() *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("Cluster-at-a-Glance").
		Collapsed(true).
		GridPos(dashboard.GridPos{X: 0, Y: 0, W: 24, H: 1}).
		WithPanel(genericLegendTimeSeries("Workers CPU Usage", "percent",
			dashboard.GridPos{X: 0, Y: 2, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricNodeCPU, `mode != "idle"`).
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						mg.NodeRoleLabelReplace(mg.RoleWorker)).
					Rate(intervalVar).
					Agg(mg.AggSum, mg.GroupByInstance).
					Multiply("100").String(),
				"{{instance}}"),
		)).
		WithPanel(genericLegendTimeSeries("Control Plane CPU Usage", "percent",
			dashboard.GridPos{X: 12, Y: 2, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricNodeCPU, `mode != "idle"`).
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						mg.NodeRoleLabelReplace("control-plane")).
					Rate(intervalVar).
					Agg(mg.AggSum, mg.GroupByInstance).
					Multiply("100").String(),
				"{{instance}}"),
		)).
		WithPanel(genericLegendTimeSeries("Workers Load1", "short",
			dashboard.GridPos{X: 0, Y: 9, W: 12, H: 8},
			promQuery(
				mg.Raw("node_load1").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						mg.NodeRoleLabelReplace(mg.RoleWorker)).String(),
				"{{instance}}"),
		)).
		WithPanel(genericLegendTimeSeries("Control Plane Load1", "short",
			dashboard.GridPos{X: 12, Y: 9, W: 12, H: 8},
			promQuery(
				mg.Raw("node_load1").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						mg.NodeRoleLabelReplace("control-plane")).String(),
				"{{instance}}"),
		)).
		WithPanel(genericLegendCounterSumRightHandTimeSeries("Workers Memory Available", "bytes",
			dashboard.GridPos{X: 0, Y: 17, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricNodeMemoryAvailable, "").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						mg.NodeRoleLabelReplace(mg.RoleWorker)).String(),
				"{{instance}}"),
			promQuery(
				mg.Q(mg.MetricNodeMemoryAvailable, "").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						mg.NodeRoleLabelReplace(mg.RoleWorker)).
					Agg(mg.AggSum).String(),
				"sum"),
		)).
		WithPanel(genericLegendCounterSumRightHandTimeSeries("Control Plane Memory Available", "bytes",
			dashboard.GridPos{X: 12, Y: 17, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricNodeMemoryAvailable, "").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						mg.NodeRoleLabelReplace("control-plane")).String(),
				"{{instance}}"),
			promQuery(
				mg.Q(mg.MetricNodeMemoryAvailable, "").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						mg.NodeRoleLabelReplace("control-plane")).
					Agg(mg.AggSum).String(),
				"sum"),
		)).
		WithPanel(genericLegendTimeSeries("Workers CGroup CPU Rate", "percent",
			dashboard.GridPos{X: 0, Y: 25, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerCPU, cgroupIDFilter).
					Rate(intervalVar).
					Multiply("100").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByNode},
						mg.NodeRoleFilter(mg.RoleWorker)).
					Agg(mg.AggSum, mg.GroupByID).String(),
				"{{instance}}"),
		)).
		WithPanel(genericLegendTimeSeries("Control Plane CGroup CPU Rate", "percent",
			dashboard.GridPos{X: 12, Y: 25, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerCPU, cgroupIDFilter).
					Rate(intervalVar).
					Multiply("100").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByNode},
						mg.NodeRoleFilter("control-plane")).
					Agg(mg.AggSum, mg.GroupByID).String(),
				"{{instance}}"),
		)).
		WithPanel(genericLegendCounterTimeSeries("Workers CGroup Memory RSS", "bytes",
			dashboard.GridPos{X: 0, Y: 33, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerMemoryRSS, cgroupIDFilter).
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByNode},
						mg.NodeRoleFilter(mg.RoleWorker)).
					Agg(mg.AggSum, mg.GroupByID).String(),
				"{{instance}}"),
		)).
		WithPanel(genericLegendCounterTimeSeries("Control Plane CGroup Memory RSS", "bytes",
			dashboard.GridPos{X: 12, Y: 33, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerMemoryRSS, cgroupIDFilter).
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByNode},
						mg.NodeRoleFilter("control-plane")).
					Agg(mg.AggSum, mg.GroupByID).String(),
				"{{instance}}"),
		)).
		WithPanel(genericLegendCounterTimeSeries("Workers Container Threads", "short",
			dashboard.GridPos{X: 0, Y: 41, W: 12, H: 8},
			promQuery(
				mg.Raw(`container_threads{container!=""}`).
					Agg(mg.AggSum, mg.GroupByNode).
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByNode},
						mg.NodeRoleFilter(mg.RoleWorker)).String(),
				"{{instance}}"),
		)).
		WithPanel(genericLegendCounterTimeSeries("Control Plane Container Threads", "short",
			dashboard.GridPos{X: 12, Y: 41, W: 12, H: 8},
			promQuery(
				mg.Raw(`container_threads{container!=""}`).
					Agg(mg.AggSum, mg.GroupByNode).
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByNode},
						mg.NodeRoleFilter("control-plane")).String(),
				"{{instance}}"),
		)).
		WithPanel(genericLegendTimeSeries("Workers Disk IOPS", "short",
			dashboard.GridPos{X: 0, Y: 49, W: 12, H: 8},
			promQuery(
				mg.Raw("node_disk_reads_completed_total").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						mg.NodeRoleLabelReplace(mg.RoleWorker)).
					Rate(intervalVar).String(),
				"{{instance}} - {{ device }} - read"),
			promQuery(
				mg.Raw("node_disk_writes_completed_total").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						mg.NodeRoleLabelReplace(mg.RoleWorker)).
					Rate(intervalVar).String(),
				"{{instance}} - {{ device }} - write"),
		)).
		WithPanel(genericLegendTimeSeries("Control Plane Disk IOPS", "short",
			dashboard.GridPos{X: 12, Y: 49, W: 12, H: 8},
			promQuery(
				mg.Raw("node_disk_reads_completed_total").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						mg.NodeRoleLabelReplace("control-plane")).
					Rate(intervalVar).String(),
				"{{instance}} - {{ device }} - read"),
			promQuery(
				mg.Raw("node_disk_writes_completed_total").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByInstance},
						mg.NodeRoleLabelReplace("control-plane")).
					Rate(intervalVar).String(),
				"{{instance}} - {{ device }} - write"),
		))
}

// Row: OVN
func ocpOVNRow() *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("OVN").
		Collapsed(true).
		GridPos(dashboard.GridPos{X: 0, Y: 0, W: 24, H: 1}).
		WithPanel(genericLegendTimeSeries("Top 10 ovnkube-controller CPU Usage", "percent",
			dashboard.GridPos{X: 0, Y: 1, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerCPU, `pod=~"ovnkube-.*",namespace="openshift-ovn-kubernetes",container="ovnkube-controller"`).
					IRate(intervalVar).Multiply("100").
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByNode).
					TopK(10).String(),
				"{{pod}} - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 ovnkube-controller Memory Usage", "bytes",
			dashboard.GridPos{X: 12, Y: 1, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerMemoryRSS, `pod=~"ovnkube-node-.*",namespace="openshift-ovn-kubernetes",container="ovnkube-controller"`).
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByNode).
					TopK(10).String(),
				"{{pod}} - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 ovn-controller CPU Usage", "percent",
			dashboard.GridPos{X: 0, Y: 8, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerCPU, `pod=~"ovnkube-.*",namespace="openshift-ovn-kubernetes",container="ovn-controller"`).
					IRate(intervalVar).Multiply("100").
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByNode).
					TopK(10).String(),
				"{{pod}} - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 ovn-controller Memory Usage", "bytes",
			dashboard.GridPos{X: 12, Y: 8, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerMemoryRSS, `pod=~"ovnkube-node-.*",namespace="openshift-ovn-kubernetes",container="ovn-controller"`).
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByNode).
					TopK(10).String(),
				"{{pod}} - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 nbdb CPU Usage", "percent",
			dashboard.GridPos{X: 0, Y: 16, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerCPU, `pod=~"ovnkube-.*",namespace="openshift-ovn-kubernetes",container="nbdb"`).
					IRate(intervalVar).Multiply("100").
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByNode).
					TopK(10).String(),
				"{{pod}} - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 nbdb Memory Usage", "bytes",
			dashboard.GridPos{X: 12, Y: 16, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerMemoryRSS, `pod=~"ovnkube-node-.*",namespace="openshift-ovn-kubernetes",container="nbdb"`).
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByNode).
					TopK(10).String(),
				"{{pod}} - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 northd CPU Usage", "percent",
			dashboard.GridPos{X: 0, Y: 24, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerCPU, `pod=~"ovnkube-.*",namespace="openshift-ovn-kubernetes",container="northd"`).
					IRate(intervalVar).Multiply("100").
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByNode).
					TopK(10).String(),
				"{{pod}} - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 northd Memory Usage", "bytes",
			dashboard.GridPos{X: 12, Y: 24, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerMemoryRSS, `pod=~"ovnkube-node-.*",namespace="openshift-ovn-kubernetes",container="northd"`).
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByNode).
					TopK(10).String(),
				"{{pod}} - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 sbdb CPU Usage", "percent",
			dashboard.GridPos{X: 0, Y: 32, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerCPU, `pod=~"ovnkube-.*",namespace="openshift-ovn-kubernetes",container="sbdb"`).
					IRate(intervalVar).Multiply("100").
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByNode).
					TopK(10).String(),
				"{{pod}} - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 sbdb Memory Usage", "bytes",
			dashboard.GridPos{X: 12, Y: 32, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerMemoryRSS, `pod=~"ovnkube-node-.*",namespace="openshift-ovn-kubernetes",container="sbdb"`).
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByNode).
					TopK(10).String(),
				"{{pod}} - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("ovs-master CPU Usage", "percent",
			dashboard.GridPos{X: 0, Y: 40, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerCPU, `id=~"/.*/ovs-vswitchd.service", node=~"$_master_node"`).
					IRate(intervalVar).Multiply("100").String(),
				"OVS CPU - {{ node }}"),
			promQuery(
				mg.Q(mg.MetricContainerCPU, `id=~"/.*/ovsdb-server.service", node=~"$_master_node"`).
					IRate(intervalVar).Multiply("100").String(),
				"OVS DB CPU - {{ node }}"),
		)).
		WithPanel(genericLegendTimeSeries("ovs-master Memory Usage", "bytes",
			dashboard.GridPos{X: 12, Y: 40, W: 12, H: 8},
			promQuery(q(mg.MetricContainerMemoryRSS, `id=~"/.*/ovs-vswitchd.service", node=~"$_master_node"`), "OVS Memory - {{ node }}"),
			promQuery(q(mg.MetricContainerMemoryRSS, `id=~"/.*/ovsdb-server.service", node=~"$_master_node"`), "OVS DB Memory - {{ node }}"),
		)).
		WithPanel(genericLegendTimeSeries("ovs-worker CPU Usage", "percent",
			dashboard.GridPos{X: 0, Y: 48, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerCPU, `id=~"/.*/ovs-vswitchd.service", node=~"$_worker_node"`).
					IRate(intervalVar).Multiply("100").String(),
				"OVS CPU - {{ node }}"),
			promQuery(
				mg.Q(mg.MetricContainerCPU, `id=~"/.*/ovsdb-server.service", node=~"$_worker_node"`).
					IRate(intervalVar).Multiply("100").String(),
				"OVS DB CPU - {{ node }}"),
		)).
		WithPanel(genericLegendTimeSeries("ovs-worker Memory Usage", "bytes",
			dashboard.GridPos{X: 12, Y: 48, W: 12, H: 8},
			promQuery(q(mg.MetricContainerMemoryRSS, `id=~"/.*/ovs-vswitchd.service", node=~"$_worker_node"`), "OVS Memory - {{ node }}"),
			promQuery(q(mg.MetricContainerMemoryRSS, `id=~"/.*/ovsdb-server.service", node=~"$_worker_node"`), "OVS DB Memory - {{ node }}"),
		)).
		WithPanel(genericLegendTimeSeries("99% Pod Annotation Latency", "s",
			dashboard.GridPos{X: 0, Y: 56, W: 8, H: 8},
			promQuery(
				mg.Raw("ovnkube_controller_pod_creation_latency_seconds").
					BucketRate(intervalVar).
					Agg(mg.AggSum, mg.GroupByInstance, mg.GroupByPod, mg.GroupByLE).
					HistogramQuantile(mg.P99).
					Gt("0").String(),
				"{{ pod }} - {{ instance }}"),
		)).
		WithPanel(genericLegendTimeSeries("99% CNI Request ADD Latency", "s",
			dashboard.GridPos{X: 8, Y: 56, W: 8, H: 8},
			promQuery(
				mg.Raw(`ovnkube_node_cni_request_duration_seconds{command="ADD"}`).
					BucketRate(intervalVar).
					Agg(mg.AggSum, mg.GroupByInstance, mg.GroupByPod, mg.GroupByLE).
					HistogramQuantile(mg.P99).
					Gt("0").String(),
				"{{ pod }} - {{ instance }}"),
		)).
		WithPanel(genericLegendTimeSeries("99% CNI Request DEL Latency", "s",
			dashboard.GridPos{X: 16, Y: 56, W: 8, H: 8},
			promQuery(
				mg.Raw(`ovnkube_node_cni_request_duration_seconds{command="DEL"}`).
					BucketRate(intervalVar).
					Agg(mg.AggSum, mg.GroupByInstance, mg.GroupByPod, mg.GroupByLE).
					HistogramQuantile(mg.P99).
					Gt("0").String(),
				"{{ pod }} - {{ instance }}"),
		)).
		WithPanel(genericLegendTimeSeries("ovnkube-control-plane CPU Usage", "percent",
			dashboard.GridPos{X: 0, Y: 64, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerCPU, `pod=~"(ovnkube-master|ovnkube-control-plane).+",namespace="openshift-ovn-kubernetes",container!~"POD|"`).
					IRate(intervalVar).Multiply("100").
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByNode).String(),
				"{{pod}} - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("ovnkube-control-plane Memory Usage", "bytes",
			dashboard.GridPos{X: 12, Y: 64, W: 12, H: 8},
			promQuery(q(mg.MetricContainerMemoryRSS, `pod=~"(ovnkube-master|ovnkube-control-plane).+",namespace="openshift-ovn-kubernetes",container!~"POD|"`), "{{pod}} - {{node}}"),
		))
}

// Row: Monitoring stack
func ocpMonitoringStackRow() *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("Monitoring stack").
		Collapsed(true).
		GridPos(dashboard.GridPos{X: 0, Y: 0, W: 24, H: 1}).
		WithPanel(genericLegendTimeSeries("Prometheus Replica CPU", "percent",
			dashboard.GridPos{X: 0, Y: 2, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerCPU, `pod=~"prometheus-k8s-0",namespace!="",name!="",container="prometheus"`).
					IRate(intervalVar).
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByContainer, mg.GroupByNode).
					Multiply("100").String(),
				"{{pod}} - {{node}}"),
			promQuery(
				mg.Q(mg.MetricContainerCPU, `pod=~"prometheus-k8s-1",namespace!="",name!="",container="prometheus"`).
					IRate(intervalVar).
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByContainer, mg.GroupByNode).
					Multiply("100").String(),
				"{{pod}} - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("Prometheus Replica RSS", "bytes",
			dashboard.GridPos{X: 12, Y: 2, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerMemoryRSS, `pod="prometheus-k8s-1",namespace!="",name!="",container="prometheus"`).
					Agg(mg.AggSum, mg.GroupByPod).String(),
				"{{pod}}"),
			promQuery(
				mg.Q(mg.MetricContainerMemoryRSS, `pod="prometheus-k8s-0",namespace!="",name!="",container="prometheus"`).
					Agg(mg.AggSum, mg.GroupByPod).String(),
				"{{pod}}"),
		)).
		WithPanel(genericLegendTimeSeries("metrics-server/prom-adapter CPU", "percent",
			dashboard.GridPos{X: 0, Y: 10, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerCPU, `pod=~"metrics-server-.*",namespace!="",name!=""`).
					IRate(intervalVar).
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByContainer).
					Multiply("100").String(),
				"{{pod}}"),
			promQuery(
				mg.Q(mg.MetricContainerCPU, `pod=~"prometheus-adapter-.*",namespace="openshift-monitoring",name!=""`).
					IRate(intervalVar).
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByContainer).
					Multiply("100").String(),
				"{{pod}}"),
		)).
		WithPanel(genericLegendTimeSeries("metrics-server/prom-adapter RSS", "bytes",
			dashboard.GridPos{X: 12, Y: 10, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerMemoryRSS, `pod=~"metrics-server-.*",namespace!="",name!=""`).
					Agg(mg.AggSum, mg.GroupByPod).String(),
				"{{pod}}"),
			promQuery(
				mg.Q(mg.MetricContainerMemoryRSS, `pod=~"prometheus-adapter-.*",namespace="openshift-monitoring",name!=""`).
					Agg(mg.AggSum, mg.GroupByPod).String(),
				"{{pod}}"),
		))
}

// Row: Stackrox
func ocpStackroxRow() *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("Stackrox").
		Collapsed(true).
		GridPos(dashboard.GridPos{X: 0, Y: 0, W: 24, H: 1}).
		WithPanel(genericLegendTimeSeries("Top 25 stackrox container RSS bytes", "bytes",
			dashboard.GridPos{X: 0, Y: 2, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerMemoryRSS, `container!="POD",name!="",namespace!="",namespace=~"stackrox"`).
					TopK(25).String(),
				"{{ pod }}: {{ container }}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 25 stackrox container CPU percent", "percent",
			dashboard.GridPos{X: 12, Y: 2, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerCPU, `container!="POD",name!="",namespace!="",namespace=~"stackrox"`).
					IRate(intervalVar).
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByContainer, mg.GroupByNamespace, mg.GroupByName, "service").
					Multiply("100").
					TopK(25).String(),
				"{{ pod }}: {{ container }}"),
		))
}

// Row: Cluster Kubelet
func ocpClusterKubeletRow() *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("Cluster Kubelet").
		Collapsed(true).
		GridPos(dashboard.GridPos{X: 0, Y: 0, W: 24, H: 1}).
		WithPanel(genericLegendTimeSeries("Top 10 Kubelet CPU usage", "percent",
			dashboard.GridPos{X: 0, Y: 3, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricProcessCPU, `service="kubelet",job="kubelet"`).
					IRate(intervalVar).Multiply("100").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByNode},
						mg.NodeRoleFilter(mg.RoleWorker)).
					TopK(10).String(),
				"kubelet - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 crio CPU usage", "percent",
			dashboard.GridPos{X: 12, Y: 3, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricProcessCPU, `service="kubelet",job="crio"`).
					IRate(intervalVar).Multiply("100").
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByNode},
						mg.NodeRoleFilter(mg.RoleWorker)).
					TopK(10).String(),
				"crio - {{node}}"),
		)).
		WithPanel(genericLegendCounterTimeSeries("Top 10 Kubelet memory usage", "bytes",
			dashboard.GridPos{X: 0, Y: 11, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricProcessMemory, `service="kubelet",job="kubelet"`).
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByNode},
						mg.NodeRoleFilter(mg.RoleWorker)).
					TopK(10).String(),
				"kubelet - {{node}}"),
		)).
		WithPanel(genericLegendCounterTimeSeries("Top 10 crio memory usage", "bytes",
			dashboard.GridPos{X: 12, Y: 11, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricProcessMemory, `service="kubelet",job="crio"`).
					MultiplyOnGroupLeft([]mg.GroupBy{mg.GroupByNode},
						mg.NodeRoleFilter(mg.RoleWorker)).
					TopK(10).String(),
				"crio - {{node}}"),
		)).
		WithPanel(genericLegendTimeSeries("inodes usage in /run", "percent",
			dashboard.GridPos{X: 0, Y: 19, W: 12, H: 8},
			promQuery(`(1 - node_filesystem_files_free{fstype!="",mountpoint="/run"} / node_filesystem_files{fstype!="",mountpoint="/run"}) * 100`, "{{instance}}"),
		)).
		WithPanel(genericLegendCounterSumRightHandTimeSeries("inodes count in /run", "none",
			dashboard.GridPos{X: 12, Y: 19, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricNodeFsFiles, `fstype!="",mountpoint="/run"`).
					Sub(mg.Q(mg.MetricNodeFsFilesFree, `fstype!="",mountpoint="/run"`)).String(),
				"{{instance}}"),
			promQuery(
				mg.Q(mg.MetricNodeFsFiles, `fstype!="",mountpoint="/run"`).
					Sub(mg.Q(mg.MetricNodeFsFilesFree, `fstype!="",mountpoint="/run"`)).
					Agg(mg.AggSum).String(),
				"sum"),
		))
}

// Row: Cluster Details
func ocpClusterDetailsRow() *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("Cluster Details").
		Collapsed(true).
		GridPos(dashboard.GridPos{X: 0, Y: 0, W: 24, H: 1}).
		WithPanel(genericStat("Current Node Count",
			dashboard.GridPos{X: 0, Y: 4, W: 8, H: 3},
			promQuery(mg.Raw("kube_node_info{}").Agg(mg.AggSum).String(), "Number of nodes"),
			promQuery(
				mg.Q(mg.MetricKubeNodeStatusCondition, `status="true"`).
					Agg(mg.AggSum, mg.GroupByCondition).
					Gt("0").String(),
				"Node: {{ condition }}"),
		)).
		WithPanel(genericStat("Current Namespace Count",
			dashboard.GridPos{X: 8, Y: 4, W: 8, H: 3},
			promQuery(
				mg.Q(mg.MetricKubeNamespacePhase, "").
					Agg(mg.AggSum, mg.GroupByPhase).String(),
				"{{ phase }}"),
		)).
		WithPanel(genericStat("Current Pod Count",
			dashboard.GridPos{X: 16, Y: 4, W: 8, H: 3},
			promQuery(
				mg.Q(mg.MetricKubePodStatusPhase, "").
					Agg(mg.AggSum, mg.GroupByPhase).
					Gt("0").String(),
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
			promQuery(mg.Q(mg.MetricKubeSecretInfo, "").Agg(mg.AggCount).String(), "secrets"),
			promQuery(mg.Q(mg.MetricKubeConfigmapInfo, "").Agg(mg.AggCount).String(), "Configmaps"),
		)).
		WithPanel(genericTimeSeries("Deployment count", "none",
			dashboard.GridPos{X: 8, Y: 20, W: 8, H: 8},
			promQuery(mg.Raw("kube_deployment_spec_replicas{}").Agg(mg.AggCount).String(), "Deployments"),
		)).
		WithPanel(genericTimeSeries("Services count", "none",
			dashboard.GridPos{X: 16, Y: 20, W: 8, H: 8},
			promQuery(mg.Q(mg.MetricKubeServiceInfo, "").Agg(mg.AggCount).String(), "Services"),
		)).
		WithPanel(genericTimeSeries("Routes count", "none",
			dashboard.GridPos{X: 0, Y: 20, W: 8, H: 8},
			promQuery(mg.Raw("openshift_route_info{}").Agg(mg.AggCount).String(), "Routes"),
		)).
		WithPanel(genericTimeSeries("Alerts", "none",
			dashboard.GridPos{X: 8, Y: 20, W: 8, H: 8},
			promQuery(`topk(10,sum(ALERTS{severity!="none"}) by (alertname, severity))`, "{{severity}}: {{alertname}}"),
		)).
		WithPanel(genericLegendTimeSeries("Pod Distribution", "none",
			dashboard.GridPos{X: 16, Y: 20, W: 8, H: 8},
			promQuery(
				mg.Q(mg.MetricKubePodInfo, "").
					Agg(mg.AggCount, mg.GroupByNode).String(),
				"{{ node }}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 container CPU", "percent",
			dashboard.GridPos{X: 0, Y: 28, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerCPU, `namespace!="",container!="POD",name!=""`).
					IRate(intervalVar).Multiply("100").
					TopK(10).String(),
				"{{ namespace }} - {{ name }} - {{ node }}"),
		)).
		WithPanel(genericLegendTimeSeries("Top 10 container RSS", "bytes",
			dashboard.GridPos{X: 12, Y: 28, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerMemoryRSS, `namespace!="",container!="POD",name!=""`).
					TopK(10).String(),
				"{{ namespace }} - {{ name }} - {{ node }}"),
		)).
		WithPanel(genericLegendTimeSeries("container RSS system.slice", "bytes",
			dashboard.GridPos{X: 12, Y: 36, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerMemoryRSS, `id="/system.slice"`).
					Agg(mg.AggSum, mg.GroupByNode).String(),
				"system.slice - {{ node }}"),
		)).
		WithPanel(genericTimeSeries("Goroutines count", "none",
			dashboard.GridPos{X: 0, Y: 36, W: 12, H: 8},
			promQuery(`topk(10, sum(go_goroutines{}) by (job,instance))`, "{{ job }} - {{ instance }}"),
		))
}

// Row: Cluster Operators Details
func ocpClusterOperatorsDetailsRow() *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("Cluster Operators Details").
		Collapsed(true).
		GridPos(dashboard.GridPos{X: 0, Y: 0, W: 24, H: 1}).
		WithPanel(genericStat("Cluster operators overview",
			dashboard.GridPos{X: 0, Y: 4, W: 24, H: 3},
			promQuery(
				mg.Q(mg.MetricClusterOperatorConditions, `condition!=""`).
					Agg(mg.AggSum, mg.GroupByCondition).String(),
				"{{ condition }}"),
		)).
		WithPanel(genericLegendTimeSeries("Cluster operators information", "none",
			dashboard.GridPos{X: 0, Y: 4, W: 8, H: 8},
			promQuery(q(mg.MetricClusterOperatorConditions, `name!="",reason!=""`), "{{name}} - {{reason}}"),
		)).
		WithPanel(genericLegendTimeSeries("Cluster operators degraded", "none",
			dashboard.GridPos{X: 8, Y: 4, W: 8, H: 8},
			promQuery(q(mg.MetricClusterOperatorConditions, `condition="Degraded",name!="",reason!=""`), "{{name}} - {{reason}}"),
		))
}

func ocpNodeRow(nodeVar string, role mg.NodeRole) *dashboard.RowBuilder {
	instanceFilter := `instance=~"$` + nodeVar + `"`
	nodeFilter := `node=~"$` + nodeVar + `"`

	row := dashboard.NewRowBuilder(string(role) + ": $" + nodeVar).
		Collapsed(true).
		GridPos(dashboard.GridPos{X: 0, Y: 0, W: 24, H: 8}).
		Repeat(nodeVar).
		WithPanel(genericLegendTimeSeries("CPU Basic: $"+nodeVar, "percent",
			dashboard.GridPos{X: 0, Y: 1, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricNodeCPU, instanceFilter+`,job=~".*"`).
					IRate(intervalVar).
					Agg(mg.AggSum, mg.GroupByInstance, mg.GroupByMode).
					Multiply("100").String(),
				"Busy {{mode}}"),
		)).
		WithPanel(genericLegendTimeSeries("Disk throughput: $"+nodeVar, "Bps",
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
		WithPanel(genericLegendTimeSeries("Disk IOPS: $"+nodeVar, "iops",
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
		WithPanel(genericLegendTimeSeries("Network Utilization: $"+nodeVar, "bps",
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
		WithPanel(genericLegendTimeSeries("Network Packets: $"+nodeVar, "pps",
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
		WithPanel(genericLegendTimeSeries("Network packets drop: $"+nodeVar, "pps",
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
		WithPanel(genericLegendTimeSeries("Top 10 container CPU: $"+nodeVar, "percent",
			dashboard.GridPos{X: 0, Y: 24, W: 12, H: 8},
			promQuery(
				mg.Q(mg.MetricContainerCPU, `container!="POD",name!="",`+nodeFilter+`,namespace!="",namespace=~"$namespace"`).
					IRate(intervalVar).
					Agg(mg.AggSum, mg.GroupByPod, mg.GroupByContainer, mg.GroupByNamespace, mg.GroupByName, "service").
					Multiply("100").
					TopK(10).String(),
				"{{ pod }}: {{ container }}"),
		))

	if role == "Master" || role == "master" {
		row = row.
			WithPanel(genericLegendCounterTimeSeries("System Memory: $"+nodeVar, "bytes",
				dashboard.GridPos{X: 12, Y: 1, W: 12, H: 8},
				promQuery(q(mg.MetricNodeMemoryActive, instanceFilter), "Active"),
				promQuery(q(mg.MetricNodeMemoryTotal, instanceFilter), "Total"),
				promQuery(
					mg.Q(mg.MetricNodeMemoryCached, instanceFilter).
						Sub(mg.Raw("-")).String()+"node_memory_Buffers_bytes{"+instanceFilter+"}",
					"Cached + Buffers"),
				promQuery(q(mg.MetricNodeMemoryAvailable, instanceFilter), "Available"),
				promQuery(
					mg.Q(mg.MetricNodeMemoryTotal, instanceFilter).
						Sub(
							mg.Q(mg.MetricNodeMemoryFree, instanceFilter).
								Paren()).String(),
					"Used"),
			))
	}

	return row
}

// Row: Master (repeats on _master_node)
func ocpMasterRow() *dashboard.RowBuilder {
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
func ocpWorkerRow() *dashboard.RowBuilder {
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
func ocpInfraRow() *dashboard.RowBuilder {
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
