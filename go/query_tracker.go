package main

import (
	"regexp"
	"strings"

	mg "github.com/afcollins/metrics-generator/pkg/metrics"
	"github.com/grafana/grafana-foundation-sdk/go/cog"
	"github.com/grafana/grafana-foundation-sdk/go/dashboard"
	"github.com/grafana/grafana-foundation-sdk/go/prometheus"
)

// profileInterval is substituted for $interval in metrics profile queries.
const profileInterval = mg.Rate2m

var grafanaVarRe = regexp.MustCompile(`,?\s*\w+[=!~]+=?"[^"]*\$[^"]*"`)
var labelValuesRe = regexp.MustCompile(`^label_values\((.+),\s*(\w+)\)$`)
var grafanaVarNameRe = regexp.MustCompile(`\$(?:\{[a-zA-Z_][a-zA-Z0-9_]*\}|[a-zA-Z_][a-zA-Z0-9_]*)`)

// recordingRuleName replaces Grafana template variables in an internal
// recording-rule name with a stable descriptive suffix. A repeated panel is
// rendered once by Grafana for each variable value, so its rule must be one
// stable metric filtered by that variable rather than a metric whose name
// contains the template expression. Keeping the variable name in the suffix
// also avoids colliding with a different, non-repeated rule (for example,
// nodeCPU and nodeCPUByClusterRole).
func recordingRuleName(name string) string {
	return grafanaVarNameRe.ReplaceAllStringFunc(name, func(variable string) string {
		variable = strings.TrimSuffix(strings.TrimPrefix(variable, "$"), "}")
		variable = strings.TrimPrefix(variable, "{")
		parts := strings.FieldsFunc(variable, func(r rune) bool { return r == '_' })
		for i, part := range parts {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
		return "By" + strings.Join(parts, "")
	})
}

func stripGrafanaVars(expr string) string {
	clean := grafanaVarRe.ReplaceAllString(expr, "")
	clean = strings.ReplaceAll(clean, "{,", "{")
	clean = strings.ReplaceAll(clean, "{}", "")
	clean = strings.ReplaceAll(clean, "$interval", string(profileInterval))
	return clean
}

func extractGrafanaVarFilters(expr string) string {
	matches := grafanaVarRe.FindAllString(expr, -1)
	if len(matches) == 0 {
		return ""
	}
	seen := map[string]struct{}{}
	var filters []string
	for _, m := range matches {
		f := strings.TrimLeft(m, ", ")
		if _, dup := seen[f]; dup {
			continue
		}
		seen[f] = struct{}{}
		filters = append(filters, f)
	}
	return "{" + strings.Join(filters, ",") + "}"
}

// queryTracker implements panelTracker. Registers aggregated PromQL queries in a Generator.
// When useMetricNames is true, panel targets use the registered metric name as the query
// expression — for dashboards that visualize already-collected metrics.
type queryTracker struct {
	g              *mg.Generator
	useMetricNames bool
	seen           map[string]struct{} // dedup by stripped query; prevents role-variant tripling
}

func (t *queryTracker) add(name, stripped string) {
	if t.g == nil {
		return
	}
	if t.seen == nil {
		t.seen = map[string]struct{}{}
	}
	if _, dup := t.seen[stripped]; dup {
		return
	}
	t.seen[stripped] = struct{}{}
	t.g.Add(name, stripped)
}

func (t *queryTracker) track(name string, query *mg.Query, legend string) *prometheus.DataqueryBuilder {
	expr := query.String()
	recordName := recordingRuleName(name)
	t.add(recordName, stripGrafanaVars(expr))
	if t.useMetricNames {
		return promQuery(recordName+extractGrafanaVarFilters(expr), legend)
	}
	return promQuery(expr, legend)
}

func (t *queryTracker) trackVarQuery(name string, expr string) dashboard.StringOrMap {
	recordName := recordingRuleName(name)
	if m := labelValuesRe.FindStringSubmatch(expr); m != nil {
		metricExpr, label := strings.TrimSpace(m[1]), m[2]
		t.add(recordName, "count("+stripGrafanaVars(metricExpr)+") by ("+label+")")
		if t.useMetricNames {
			rewritten := "label_values(" + recordName + ", " + label + ")"
			return dashboard.StringOrMap{String: cog.ToPtr(rewritten)}
		}
	}
	return dashboard.StringOrMap{String: cog.ToPtr(expr)}
}

func (t *queryTracker) trackRaw(name string, expr string, legend string) *prometheus.DataqueryBuilder {
	recordName := recordingRuleName(name)
	t.add(recordName, stripGrafanaVars(expr))
	if t.useMetricNames {
		return promQuery(recordName+extractGrafanaVarFilters(expr), legend)
	}
	return promQuery(expr, legend)
}
