oc create configmap istio-grafana-configuration-dashboards-istio-workload-dashboard --from-file=istio-workload-dashboard.json --dry-run -o yaml | oc replace -n istio-system -f -
oc delete pod -l app=grafana -n istio-system

#cat /var/lib/grafana/dashboards/istio/istio-workload-dashboard.json | grep span