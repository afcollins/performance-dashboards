package main

import (
	"github.com/grafana/grafana-foundation-sdk/go/cog"
	"github.com/grafana/grafana-foundation-sdk/go/dashboard"
	mg "github.com/afcollins/metrics-generator/pkg/metrics"
)

func buildEtcdWorkloadsDashboard() *dashboard.DashboardBuilder {
	return buildEtcdWorkloadsDash(&queryTracker{})
}

func buildEtcdWorkloadsProfiles() []namedProfile {
	agg := &mg.Generator{}
	buildEtcdWorkloadsDash(&queryTracker{g: agg})

	raw := &mg.Generator{}
	buildEtcdWorkloadsDash(&rawTracker{g: raw, seen: map[string]struct{}{}})

	return []namedProfile{
		{"-metrics", agg},
		{"-raw-metrics", raw},
	}
}

func buildEtcdWorkloadsDash(t panelTracker) *dashboard.DashboardBuilder {
	return dashboard.NewDashboardBuilder("etcd-density-hcp dashboard").
		Time("now-1h", "now").
		Timezone("utc").
		Timepicker(dashboard.NewTimePickerBuilder().
			RefreshIntervals([]string{"5s", "10s", "30s", "1m", "5m", "15m", "30m", "1h", "2h", "1d"}),
		).
		Refresh("").
		Readonly().
		Tooltip(dashboard.DashboardCursorSyncCrosshair).
		WithVariable(dashboard.NewDatasourceVariableBuilder("Datasource").
			Type("prometheus").
			Regex("").
			Label("Datasource"),
		).
		WithVariable(dashboard.NewCustomVariableBuilder("namespace").
			Label("HCP Namespace").
			Values(dashboard.StringOrMap{String: cog.ToPtr("openshift-etcd")}).
			Current(dashboard.VariableOption{
				Selected: boolRef(true),
				Text: dashboard.StringOrArrayOfString{String: strRef("")},
				Value: dashboard.StringOrArrayOfString{String: strRef("")},
			}),
		).
		WithRow(hcpEtcdDBSizeRow(t)).
		WithRow(hcpEtcdDiskIORow(t)).
		WithRow(hcpEtcdRaftRow(t)).
		WithRow(hcpEtcdEventRatesRow(t)).
		WithRow(hcpEtcdStorageObjectsRow(t)).
		WithRow(hcpAPIServerRow(t)).
		WithRow(hcpContainerResourcesRow(t)).
		WithRow(hcpSchedulerRow(t)).
		WithRow(hcpNodeResourceUsageRow(t)).
		WithRow(hcpSnapshotsRow(t))
}

func strRef(s string) *string { return &s }
func boolRef(b bool) *bool    { return &b }

// Row: Etcd DB Size
func hcpEtcdDBSizeRow(t panelTracker) *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("Etcd DB Size").
		Collapsed(true).
		WithPanel(etcdGeneralUsageAgg("Etcd DB Total Size (avg by pod)", "bytes",
			12, 8,
			t.trackRaw("etcdDBTotalSize", `avg by (pod) (etcd_mvcc_db_total_size_in_bytes{namespace="$namespace"})`, "{{ pod }}"),
		)).
		WithPanel(etcdGeneralUsageAgg("Etcd DB Size In Use (avg by pod)", "bytes",
			12, 8,
			t.trackRaw("etcdDBSizeInUse", `avg by (pod) (etcd_mvcc_db_total_size_in_use_in_bytes{namespace="$namespace"})`, "{{ pod }}"),
		)).
		WithPanel(etcdGeneralUsageAgg("Etcd DB Fragmentation", "bytes",
			12, 8,
			t.trackRaw("etcdDBFragmentationBytes", `avg by (pod) (etcd_mvcc_db_total_size_in_bytes{namespace="$namespace"}) - avg by (pod) (etcd_mvcc_db_total_size_in_use_in_bytes{namespace="$namespace"})`, "{{ pod }}"),
		)).
		WithPanel(etcdGeneralUsageAgg("Etcd DB Total Size Per Instance", "bytes",
			12, 8,
			t.trackRaw("etcdDBTotalSizePerInstance", `etcd_mvcc_db_total_size_in_bytes{namespace="$namespace"}`, "{{ pod }}"),
		)).
		WithPanel(etcdGeneralUsageAgg("Etcd Quota Backend Bytes", "bytes",
			12, 8,
			t.trackRaw("etcdQuotaBackendBytes", `etcd_server_quota_backend_bytes{namespace="$namespace"}`, "{{ pod }}"),
		)).
		WithPanel(etcdGeneralUsageAgg("Etcd DB Size In Use Per Instance", "bytes",
			12, 8,
			t.trackRaw("etcdDBSizeInUsePerInstance", `etcd_mvcc_db_total_size_in_use_in_bytes{namespace="$namespace"}`, "{{ pod }}"),
		))
}

// Row: Etcd Disk I/O Latency
func hcpEtcdDiskIORow(t panelTracker) *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("Etcd Disk I/O Latency").
		Collapsed(true).
		WithPanel(etcdGeneralUsageAgg("99th WAL Fsync Duration", "s",
			12, 8,
			t.trackRaw("99thEtcdDiskWalFsyncDurationSeconds", `histogram_quantile(0.99, rate(etcd_disk_wal_fsync_duration_seconds_bucket{namespace="$namespace"}[2m]))`, "{{ pod }}"),
		)).
		WithPanel(etcdGeneralUsageAgg("99th Backend Commit Duration", "s",
			12, 8,
			t.trackRaw("99thEtcdDiskBackendCommitDurationSeconds", `histogram_quantile(0.99, rate(etcd_disk_backend_commit_duration_seconds_bucket{namespace="$namespace"}[2m]))`, "{{ pod }}"),
		)).
		WithPanel(etcdGeneralUsageAgg("99th gRPC Round Trip Time", "s",
			12, 8,
			t.trackRaw("99thEtcdGRPCRoundTripTimeSeconds", `histogram_quantile(0.99, sum(rate(grpc_server_handling_seconds_bucket{namespace="$namespace", grpc_service="etcdserverpb.KV"}[2m])) by (pod, le))`, "{{ pod }}"),
		)).
		WithPanel(etcdGeneralUsageAgg("Compaction Duration", "ms",
			12, 8,
			t.trackRaw("99thEtcdCompaction", `delta(etcd_debugging_mvcc_db_compaction_total_duration_milliseconds_sum{namespace="$namespace"}[1m:30s])/2 > 0`, "{{ pod }}"),
		)).
		WithPanel(etcdGeneralUsageAgg("Defrag Duration", "s",
			12, 8,
			t.trackRaw("99thEtcdDefrag", `delta(etcd_disk_backend_defrag_duration_seconds_sum{namespace="$namespace"}[1m:30s])/2 > 0`, "{{ pod }}"),
		))
}

// Row: Etcd Raft Write Pressure
func hcpEtcdRaftRow(t panelTracker) *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("Etcd Raft Write Pressure").
		Collapsed(true).
		WithPanel(etcdGeneralUsageAgg("Proposals Committed Rate", "ops",
			12, 8,
			t.trackRaw("etcdProposalsCommittedRate", `rate(etcd_server_proposals_committed_total{namespace="$namespace"}[2m])`, "{{ pod }}"),
		)).
		WithPanel(etcdGeneralUsageAgg("Proposals Failed Rate", "ops",
			12, 8,
			t.trackRaw("etcdProposalsFailedRate", `rate(etcd_server_proposals_failed_total{namespace="$namespace"}[2m])`, "{{ pod }}"),
		))
}

// Row: Etcd Event Rates
func hcpEtcdEventRatesRow(t panelTracker) *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("Etcd Event Rates").
		Collapsed(true).
		WithPanel(etcdGeneralUsageAgg("Event Write Rate", "ops",
			8, 8,
			t.trackRaw("etcdEventWriteRate", `sum(rate(apiserver_request_total{namespace="$namespace", job="kube-apiserver", resource="events", verb=~"POST|PUT|DELETE|PATCH"}[2m]))`, "writes/s"),
			t.trackRaw("etcdEventWriteRate", `sum(rate(apiserver_request_total{job=~"api|apiserver", resource="events", verb=~"POST|PUT|DELETE|PATCH"}[2m]))`, "writes/s"),
		)).
		WithPanel(etcdGeneralUsageAgg("Event Read Rate", "ops",
			8, 8,
			t.trackRaw("etcdEventReadRate", `sum(rate(apiserver_request_total{namespace="$namespace", job="kube-apiserver", resource="events", verb=~"LIST|GET"}[2m]))`, "reads/s"),
			t.trackRaw("etcdEventReadRate", `sum(rate(apiserver_request_total{job=~"api|apiserver", resource="events", verb=~"LIST|GET"}[2m]))`, "reads/s"),
		)).
		WithPanel(etcdGeneralUsageAgg("Cluster Event Count", "short",
			8, 8,
			t.trackRaw("clusterEventCount", `cluster:usage:resources:sum{resource="events"}`, "events"),
		))
}

// Row: Etcd Storage Objects
func hcpEtcdStorageObjectsRow(t panelTracker) *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("Etcd Storage Objects").
		Collapsed(true).
		WithPanel(etcdGeneralUsageAgg("Storage Objects (top 20)", "short",
			24, 8,
			promQuery(`topk(20, max by (resource) (apiserver_storage_objects{namespace="$namespace"}))`, "{{ resource }}"),
			promQuery(`topk(20, max by (resource) (apiserver_storage_objects{job=~"api|apiserver"}))`, "{{ resource }}"),
		))
}

// Row: API Server
func hcpAPIServerRow(t panelTracker) *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("API Server").
		Collapsed(true).
		WithPanel(etcdGeneralUsageAgg("API Request Rate", "ops",
			12, 8,
			t.trackRaw("APIRequestRate", `sum(irate(apiserver_request_total{namespace="$namespace", job="kube-apiserver", verb!="WATCH"}[2m])) by (verb, resource, code) > 0`, "{{ verb }} {{ resource }} {{ code }}"),
			t.trackRaw("APIRequestRate", `sum(irate(apiserver_request_total{job=~"api|apiserver", verb!="WATCH"}[2m])) by (verb, resource, code) > 0`, "{{ verb }} {{ resource }} {{ code }}"),
		)).
		WithPanel(etcdGeneralUsageAgg("API Server Errors (5xx)", "ops",
			12, 8,
			t.trackRaw("APIServerErrors", `sum(irate(apiserver_request_total{namespace=~"$namespace|openshift-kube-apiserver", job="kube-apiserver", verb!="WATCH", code=~"5.."}[2m])) by (verb, resource, code) > 0`, "{{ verb }} {{ resource }} {{ code }}"),
			t.trackRaw("APIServerErrors", `sum(irate(apiserver_request_total{job=~"api|apiserver", verb!="WATCH", code=~"5.."}[2m])) by (verb, resource, code) > 0`, "{{ verb }} {{ resource }} {{ code }}"),
		))
}

// Row: Control Plane Container Resources
func hcpContainerResourcesRow(t panelTracker) *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("Control Plane Container Resources").
		Collapsed(true).
		WithPanel(etcdGeneralUsageAgg("Container CPU", "percent",
			12, 8,
			t.trackRaw("containerCPU", `(sum(irate(container_cpu_usage_seconds_total{name!="", container!~"POD|", namespace="$namespace", container=~"etcd|etcd-events|kube-apiserver"}[2m]) * 100) by (container, pod, namespace)) > 0`, "{{ container }} {{ pod }}"),
		)).
		WithPanel(etcdGeneralUsageAgg("Container Memory", "bytes",
			12, 8,
			t.trackRaw("containerMemory", `sum(container_memory_rss{name!="", container!~"POD|", namespace="$namespace", container=~"etcd|etcd-events|kube-apiserver"}) by (container, pod, namespace)`, "{{ container }} {{ pod }}"),
		))
}

// Row: Scheduler
func hcpSchedulerRow(t panelTracker) *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("Scheduler").
		Collapsed(true).
		WithPanel(etcdGeneralUsageAgg("Scheduler Throughput", "ops",
			12, 8,
			t.trackRaw("schedulerThroughput", `sum(irate(scheduler_schedule_attempts_total{namespace=~"$namespace|openshift-kube-scheduler", result="scheduled"}[2m]))`, "scheduled/s"),
		)).
		WithPanel(etcdGeneralUsageAgg("99th Scheduler E2E Latency", "s",
			12, 8,
			t.trackRaw("99thSchedulerE2ELatency", `histogram_quantile(0.99, sum(rate(scheduler_pod_scheduling_sli_duration_seconds_bucket{namespace=~"$namespace|openshift-kube-scheduler"}[2m])) by (le))`, "p99"),
		))
}

// Row: HCP Node Resource Usage
func hcpNodeResourceUsageRow(t panelTracker) *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("HCP Control Plane Node Resources").
		Collapsed(true).
		WithPanel(etcdGeneralUsageAgg("Node CPU (Control Plane Hosts)", "percent",
			12, 8,
			t.trackRaw("nodeCPU-ControlPlaneHosts", `(sum(irate(container_cpu_usage_seconds_total{name!="", namespace="$namespace"}[2m]) * 100) by (node)) > 0`, "{{ node }}"),
		)).
		WithPanel(etcdGeneralUsageAgg("Node Memory (Control Plane Hosts)", "bytes",
			12, 8,
			t.trackRaw("nodeMemory-ControlPlaneHosts", `sum(container_memory_rss{name!="", namespace="$namespace"}) by (node)`, "{{ node }}"),
		)).
		WithPanel(etcdGeneralUsageAgg("CPU % (Control Plane Containers)", "percent",
			12, 8,
			t.trackRaw("cpuPercent-ControlPlane", `(sum(irate(container_cpu_usage_seconds_total{name!="", container!~"POD|", namespace="$namespace"}[2m]) * 100) by (container, node, pod)) > 0`, "{{ container }} {{ node }} {{ pod }}"),
		)).
		WithPanel(etcdGeneralUsageAgg("Memory RSS (Control Plane Containers)", "bytes",
			12, 8,
			t.trackRaw("memoryRSS-ControlPlane", `sum(container_memory_rss{name!="", container!~"POD|", namespace="$namespace"}) by (container, node, pod)`, "{{ container }} {{ node }} {{ pod }}"),
		))
}

// Row: Snapshots (instant queries rendered as time series panels)
func hcpSnapshotsRow(t panelTracker) *dashboard.RowBuilder {
	return dashboard.NewRowBuilder("Snapshots").
		Collapsed(true).
		WithPanel(etcdGeneralUsageAgg("Etcd DB Total Size (snapshot)", "bytes",
			12, 8,
			t.trackRaw("etcdDBTotalSizeSnapshot", `etcd_mvcc_db_total_size_in_bytes{namespace="$namespace"}`, "{{ pod }}"),
		)).
		WithPanel(etcdGeneralUsageAgg("Etcd Proposals Committed", "short",
			12, 8,
			t.trackRaw("etcdProposalsCommitted", `etcd_server_proposals_committed_total{namespace="$namespace"}`, "{{ pod }}"),
		)).
		WithPanel(etcdGeneralUsageAgg("Etcd Slow Apply Total", "short",
			12, 8,
			t.trackRaw("etcdSlowApplyTotal", `etcd_server_slow_apply_total{namespace="$namespace"}`, "{{ pod }}"),
		)).
		WithPanel(etcdGeneralUsageAgg("Etcd Leader Changes", "short",
			12, 8,
			t.trackRaw("etcdLeaderChanges", `etcd_server_leader_changes_seen_total{namespace="$namespace"}`, "{{ pod }}"),
		)).
		WithPanel(etcdGeneralUsageAgg("Compaction Duration (raw)", "ms",
			12, 8,
			t.trackRaw("99thEtcdCompaction-raw", `etcd_debugging_mvcc_db_compaction_total_duration_milliseconds_sum{namespace="$namespace"}`, "{{ pod }}"),
		)).
		WithPanel(etcdGeneralUsageAgg("Defrag Duration (raw)", "s",
			12, 8,
			t.trackRaw("99thEtcdDefrag-raw", `etcd_disk_backend_defrag_duration_seconds_sum{namespace="$namespace"}`, "{{ pod }}"),
		)).
		WithPanel(etcdGeneralUsageAgg("Storage Objects (snapshot)", "short",
			12, 8,
			promQuery(`topk(20, max by (resource) (apiserver_storage_objects{namespace="$namespace"}))`, "{{ resource }}"),
		)).
		WithPanel(etcdGeneralUsageAgg("Etcd Version", "short",
			12, 8,
			t.trackRaw("etcdVersion", `sum by (cluster_version)(etcd_cluster_version{namespace="$namespace"})`, "{{ cluster_version }}"),
		)).
		WithPanel(etcdGeneralUsageAgg("CPU Seconds (Control Plane)", "s",
			12, 8,
			t.trackRaw("cpuSeconds-ControlPlane", `sum(container_cpu_usage_seconds_total{namespace="$namespace"}) by (container, node, pod)`, "{{ container }} {{ node }} {{ pod }}"),
		)).
		WithPanel(etcdGeneralUsageAgg("Memory RSS (Control Plane)", "bytes",
			12, 8,
			t.trackRaw("memoryRSS-ControlPlane", `sum(container_memory_rss{namespace="$namespace"}) by (container, node, pod)`, "{{ container }} {{ node }} {{ pod }}"),
		))
}
