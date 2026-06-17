# Fase 7 — Observability (Prometheus + Loki + Grafana)

**Status:** Niet gestart
**Vereiste:** Fase 6 afgerond
**MAP-equivalent:** Observability stack (volledig open, hoge prio — MAP-151)

---

## Doel

Een volledige observability stack opzetten: metrics (Prometheus), logs (Loki), traces (Tempo) en dashboards (Grafana). Dit is precies wat MAP-151 vraagt — en wat jij als eerste bij MAP kunt leveren als je dit al in de vingers hebt.

## Wat je leert

- De drie pijlers van observability: metrics / logs / traces
- Prometheus scraping, PromQL, alerting rules
- Loki log aggregatie (push-model via Promtail/Alloy)
- Tempo distributed tracing via OpenTelemetry
- Grafana: datasources koppelen, dashboards bouwen, alerting
- kube-state-metrics en node-exporter voor cluster metrics
- Hoe je NATS, Keycloak en Traefik metrics scrapt

---

## Vereisten

- Fase 1–6 afgerond (alle services draaien)

---

## Stappen

### 1. kube-prometheus-stack deployen

Dit is de snelste manier: één Helm chart met Prometheus, Alertmanager, Grafana, kube-state-metrics en node-exporter.

```yaml
# infra-k8s/base/monitoring/application.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: kube-prometheus-stack
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://prometheus-community.github.io/helm-charts
    chart: kube-prometheus-stack
    targetRevision: "58.x.x"
    helm:
      values: |
        grafana:
          enabled: true
          adminPassword: "changeme"
          ingress:
            enabled: false
        prometheus:
          prometheusSpec:
            retention: 7d
            storageSpec:
              volumeClaimTemplate:
                spec:
                  accessModes: ["ReadWriteOnce"]
                  resources:
                    requests:
                      storage: 10Gi
        alertmanager:
          enabled: true
  destination:
    server: https://kubernetes.default.svc
    namespace: monitoring
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

IngressRoute voor Grafana:
```yaml
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: grafana
  namespace: monitoring
spec:
  entryPoints:
    - web
  routes:
    - match: Host(`grafana.localhost`)
      kind: Rule
      services:
        - name: kube-prometheus-stack-grafana
          port: 80
```

Voeg toe aan `/etc/hosts`: `127.0.0.1 grafana.localhost`

### 2. Loki deployen

```yaml
# infra-k8s/base/loki/application.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: loki-stack
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://grafana.github.io/helm-charts
    chart: loki-stack
    targetRevision: "2.x.x"
    helm:
      values: |
        loki:
          enabled: true
          persistence:
            enabled: true
            size: 10Gi
        promtail:
          enabled: true
        grafana:
          enabled: false   # al gedeployed via kube-prometheus-stack
  destination:
    server: https://kubernetes.default.svc
    namespace: monitoring
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

Loki toevoegen als datasource in Grafana:
- URL: `http://loki:3100`
- Type: Loki

### 3. Tempo deployen (distributed tracing)

```yaml
# infra-k8s/base/tempo/application.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: tempo
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://grafana.github.io/helm-charts
    chart: tempo
    targetRevision: "1.x.x"
    helm:
      values: |
        tempo:
          retention: 48h
        persistence:
          enabled: true
          size: 5Gi
  destination:
    server: https://kubernetes.default.svc
    namespace: monitoring
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

OpenTelemetry toevoegen aan de webhook-receiver:
```js
const { NodeTracerProvider } = require('@opentelemetry/sdk-node');
const { OTLPTraceExporter } = require('@opentelemetry/exporter-trace-otlp-http');

const provider = new NodeTracerProvider();
provider.register();
// traces gaan naar http://tempo:4318/v1/traces
```

### 4. ServiceMonitors voor eigen services

Prometheus haalt automatisch metrics op van services met een ServiceMonitor:

```yaml
# infra-k8s/base/webhook-receiver/servicemonitor.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: webhook-receiver
  labels:
    release: kube-prometheus-stack
spec:
  selector:
    matchLabels:
      app: webhook-receiver
  endpoints:
    - port: metrics
      interval: 15s
      path: /metrics
```

Voeg `/metrics` endpoint toe aan de webhook-receiver (via `prom-client`):
```js
const client = require('prom-client');
const register = new client.Registry();
client.collectDefaultMetrics({ register });

const webhooksReceived = new client.Counter({
  name: 'webhooks_received_total',
  help: 'Total webhooks received',
  labelNames: ['source'],
  registers: [register],
});
```

### 5. Alerts aanmaken

```yaml
# infra-k8s/base/monitoring/alerts.yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: platform-lab-alerts
  labels:
    release: kube-prometheus-stack
spec:
  groups:
    - name: platform-lab
      rules:
        - alert: WebhookProcessorDown
          expr: up{job="webhook-processor"} == 0
          for: 2m
          labels:
            severity: critical
          annotations:
            summary: "Webhook processor is down"

        - alert: NATSQueueLagHigh
          expr: nats_consumer_num_pending > 1000
          for: 5m
          labels:
            severity: warning
          annotations:
            summary: "NATS queue lag is high ({{ $value }} messages)"
```

### 6. Grafana dashboards

Importeer community dashboards:
- **Kubernetes cluster**: ID `15760`
- **Traefik**: ID `17347`
- **NATS**: ID `2279`
- **Node exporter**: ID `1860`

Bouw zelf een dashboard met:
- Webhooks ontvangen per minuut (PromQL: `rate(webhooks_received_total[5m])`)
- NATS queue diepte over tijd
- P99 latency van de webhook-receiver
- Log panel: laatste errors uit Loki

---

## Definitie van klaar

- [ ] Grafana bereikbaar op `http://grafana.localhost`
- [ ] Prometheus scrapt alle services (webhook-receiver, NATS, Traefik, Keycloak)
- [ ] Loki ontvangt logs van alle pods (Promtail draait)
- [ ] Tempo ontvangt traces van de webhook-receiver
- [ ] Grafana Explore: logs en traces zijn doorzoekbaar
- [ ] Eigen dashboard met minimaal 4 panels
- [ ] Alert vuurt wanneer webhook-processor down is
- [ ] Alles deployed via ArgoCD vanuit Git

---

## Referenties

- [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack)
- [Loki](https://grafana.com/docs/loki/latest/)
- [Tempo](https://grafana.com/docs/tempo/latest/)
- [PromQL cheatsheet](https://promlabs.com/promql-cheat-sheet/)
- [OpenTelemetry Node.js](https://opentelemetry.io/docs/languages/js/)
