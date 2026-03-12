# Kind node-role join — remaining work

## Done
- `kindClusterAtAGlanceRow` — joins node_* metrics through `kube_node_info` instead of `NodeRoleLabelReplace`
- `nodeRow` refactored with `nodeInstanceStrategy` — shared between OCP and kind, no panel duplication
- Legends use `{{node}}` instead of `{{instance}}` on kind dashboard
- OCP output unchanged (zero diff)

## TODO
- Verify kind dashboard end-to-end on a running kind cluster with kube-state-metrics + node-exporter
- Audit whether `kindClusterAtAGlanceRow` cgroup/container panels need legend fixes too (currently use `{{instance}}` for tracked queries)
- Consider whether kind needs a collected dashboard variant (promQuery secondary series currently reference OCP recording rule names)
- Clean up `spicy-sparking-lake.md` plan file from go/ directory
