package main

import (
	mg "github.com/afcollins/metrics-generator/pkg/metrics"
	"github.com/grafana/grafana-foundation-sdk/go/prometheus"
)

var (
	summaryAggs        = []mg.AggFunc{mg.AggMin, mg.AggMax, mg.AggAvg}
	summaryPercentiles = []mg.Percentile{mg.P10, mg.P25, mg.P50, mg.P75, mg.P90}
)

func summaryStatsQueries(t panelTracker, baseName string, queryFactory func() *mg.Query, labelPrefix ...string) []*prometheus.DataqueryBuilder {
	prefix := ""
	if len(labelPrefix) > 0 {
		prefix = labelPrefix[0] + " - "
	}
	var queries []*prometheus.DataqueryBuilder
	for _, agg := range summaryAggs {
		queries = append(queries,
			t.track(baseName+"_"+string(agg), queryFactory().Agg(agg), prefix+string(agg)))
	}
	for _, p := range summaryPercentiles {
		queries = append(queries,
			t.track(baseName+"_"+p.Label, queryFactory().Quantile(p), prefix+p.Label))
	}
	return queries
}
