package main

import (
	"regexp"
	"strings"

	"github.com/grafana/grafana-foundation-sdk/go/prometheus"
	mg "github.com/kube-burner/metrics-generator/pkg/metrics"
)

// profileInterval is the default rate interval used in metrics profile queries.
const profileInterval = mg.Rate2m

// grafanaVarRe matches label matchers whose value contains a Grafana template variable ($...).
// These are dashboard-only scoping — stripped before registering in the metrics profile generator.
var grafanaVarRe = regexp.MustCompile(`,?\s*\w+[=!~]+=?"[^"]*\$[^"]*"`)

func stripGrafanaVars(expr string) string {
	clean := grafanaVarRe.ReplaceAllString(expr, "")
	clean = strings.ReplaceAll(clean, "{}", "")
	clean = strings.ReplaceAll(clean, "$interval", string(profileInterval))
	return clean
}

type queryTracker struct{ g *mg.Generator }

// track registers the query in the generator (Grafana vars stripped) and returns the panel target.
func (t *queryTracker) track(name string, query *mg.Query, legend string) *prometheus.DataqueryBuilder {
	if t.g != nil {
		t.g.Add(name, stripGrafanaVars(query.String()))
	}
	return promQuery(query.String(), legend)
}

// trackRaw is like track but for raw string expressions (mg.Raw / compound).
func (t *queryTracker) trackRaw(name string, expr string, legend string) *prometheus.DataqueryBuilder {
	if t.g != nil {
		t.g.Add(name, stripGrafanaVars(expr))
	}
	return promQuery(expr, legend)
}
