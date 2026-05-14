module github.com/cloud-bulldozer/performance-dashboards

go 1.25.8

require (
	github.com/google/uuid v1.6.0
	github.com/grafana/grafana-foundation-sdk/go v0.0.12
	github.com/kube-burner/metrics-generator v0.0.0
)

require gopkg.in/yaml.v3 v3.0.1 // indirect

replace github.com/kube-burner/metrics-generator => /Users/ancollin/go/src/github.com/kube-burner/metrics-generator
