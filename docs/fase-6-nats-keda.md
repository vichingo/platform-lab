# Fase 6 — Message broker + autoscaling (NATS + KEDA)

**Status:** Niet gestart
**Vereiste:** Fase 5 afgerond
**MAP-equivalent:** NATS + JetStream (gepland), KEDA scalers (gepland) — dit is wat MAP nog NIET heeft

---

## Doel

NATS met JetStream deployen als message broker. Binnenkomende webhooks worden op een NATS queue gezet. Een aparte consumer-service verwerkt de berichten. KEDA schaalt de consumer automatisch op basis van de queue-diepte. Dit mirrors de geplande sync-architectuur van MAP (Option B).

## Wat je leert

- NATS JetStream: streams, consumers, acknowledgements, replay
- Verschil tussen push-consumers en pull-consumers
- KEDA: ScaledObject op basis van NATS queue-diepte
- Waarom MAP NATS overweegt boven SQS (geen vendor lock-in) en Redpanda (Kafka-compatible maar zwaarder)
- Horizontaal schalen op queue-diepte: 0 pods bij lege queue, meerdere pods onder load
- Dead letter queues en retry-logica

---

## Vereisten

- Fase 1–5 afgerond
- `nats` CLI

```bash
brew install nats-io/nats-tools/nats
```

---

## Stappen

### 1. NATS deployen via Helm + ArgoCD

```yaml
# infra-k8s/base/nats/application.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: nats
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://nats-io.github.io/k8s/helm/charts/
    chart: nats
    targetRevision: "1.x.x"
    helm:
      values: |
        config:
          jetstream:
            enabled: true
            memStorage:
              enabled: true
              size: 1Gi
            fileStorage:
              enabled: true
              size: 5Gi
        natsBox:
          enabled: true
  destination:
    server: https://kubernetes.default.svc
    namespace: dev
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

### 2. Stream aanmaken

```bash
kubectl port-forward svc/nats 4222:4222 -n dev &

nats stream add WEBHOOKS \
  --subjects "webhooks.>" \
  --storage file \
  --replicas 1 \
  --retention limits \
  --max-age 24h \
  --max-msgs 100000
```

Test publiceren:
```bash
nats pub webhooks.github '{"event": "push", "repo": "platform-lab"}'
nats stream info WEBHOOKS
```

### 3. Webhook-receiver publiceert naar NATS

Update de webhook-receiver service om berichten naar NATS te sturen:

```js
// services/webhook-receiver/src/index.js
const { connect, StringCodec } = require('nats');

let nc;
const sc = StringCodec();

async function main() {
  nc = await connect({ servers: process.env.NATS_URL || 'nats://nats:4222' });
  const js = nc.jetstream();

  const server = http.createServer(async (req, res) => {
    let body = '';
    req.on('data', chunk => body += chunk);
    req.on('end', async () => {
      const subject = `webhooks.${req.url.replace('/', '').replace('/', '.')}`;
      await js.publish(subject, sc.encode(body));
      res.writeHead(202);
      res.end(JSON.stringify({ queued: true }));
    });
  });

  server.listen(3000);
}

main();
```

### 4. Consumer-service schrijven

```js
// services/webhook-processor/src/index.js
const { connect, StringCodec, AckPolicy } = require('nats');

const sc = StringCodec();

async function main() {
  const nc = await connect({ servers: process.env.NATS_URL || 'nats://nats:4222' });
  const js = nc.jetstream();

  const consumer = await js.consumers.get('WEBHOOKS', 'processor');

  console.log('Waiting for messages...');
  const messages = await consumer.consume();

  for await (const msg of messages) {
    const data = JSON.parse(sc.decode(msg.data));
    console.log('Processing:', data);
    // transformatie logica hier
    msg.ack();
  }
}

main();
```

Consumer aanmaken:
```bash
nats consumer add WEBHOOKS processor \
  --pull \
  --ack explicit \
  --deliver all \
  --max-deliver 3 \
  --filter "webhooks.>"
```

### 5. KEDA installeren

```yaml
# infra-k8s/base/keda/application.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: keda
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://kedacore.github.io/charts
    chart: keda
    targetRevision: "2.x.x"
  destination:
    server: https://kubernetes.default.svc
    namespace: keda
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

### 6. ScaledObject aanmaken

```yaml
# infra-k8s/base/webhook-processor/scaledobject.yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: webhook-processor-scaler
spec:
  scaleTargetRef:
    name: webhook-processor
  minReplicaCount: 0
  maxReplicaCount: 10
  triggers:
    - type: nats-jetstream
      metadata:
        natsServerMonitoringEndpoint: nats.dev.svc.cluster.local:8222
        account: "$G"
        stream: WEBHOOKS
        consumer: processor
        lagThreshold: "10"
```

Test schaling:
```bash
# Publish 100 berichten snel
for i in $(seq 1 100); do
  nats pub webhooks.test '{"n": '$i'}'
done

# Bekijk pods schalen
kubectl get pods -n dev -w -l app=webhook-processor
```

---

## Definitie van klaar

- [ ] NATS + JetStream draait, stream `WEBHOOKS` bestaat
- [ ] Webhook-receiver publiceert inkomende HTTP requests naar NATS
- [ ] Consumer-service verwerkt berichten (log zichtbaar)
- [ ] KEDA schaalt consumer van 0 → N pods bij berichten op de queue
- [ ] Bij lege queue: 0 pods (scale-to-zero werkt)
- [ ] `nats stream info WEBHOOKS` toont consumerlag

---

## Referenties

- [NATS JetStream](https://docs.nats.io/nats-concepts/jetstream)
- [KEDA NATS JetStream scaler](https://keda.sh/docs/scalers/nats-jetstream/)
- [nats.js client](https://github.com/nats-io/nats.js)
