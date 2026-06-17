# Fase 2 — Ingress + routing (Traefik)

**Status:** Niet gestart
**Vereiste:** Fase 1 afgerond
**MAP-equivalent:** Traefik gateway setup, IngressRoute CRDs, middleware (ADR.001)

---

## Doel

Traefik deployen als ingress controller via ArgoCD. Alle inkomend verkeer gaat via Traefik. Twee services deployen met aparte routes, inclusief middleware voor rate limiting en header-injectie.

## Wat je leert

- Verschil tussen Ingress (standaard K8s) en IngressRoute (Traefik CRD)
- Hoe Traefik auto-discovery werkt op basis van labels/annotations
- Middleware ketenen (rate limit → auth → service)
- Waarom MAP Traefik koos boven Kong en KrakenD (ADR.001)
- Hoe `x-tenant-id` headers worden geïnjecteerd (straks gekoppeld aan Keycloak)

---

## Vereisten

- Fase 1 afgerond (k3d cluster + ArgoCD)
- Poorten 80 en 443 zijn doorgestuurd via k3d (gedaan in fase 1)

---

## Stappen

### 1. Traefik deployen via Helm + ArgoCD

Voeg een Traefik Application toe aan je `infra-k8s/`:

```yaml
# infra-k8s/base/traefik/application.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: traefik
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://helm.traefik.io/traefik
    chart: traefik
    targetRevision: "28.x.x"
    helm:
      values: |
        ports:
          web:
            port: 8000
            exposedPort: 80
          websecure:
            port: 8443
            exposedPort: 443
        ingressRoute:
          dashboard:
            enabled: true
  destination:
    server: https://kubernetes.default.svc
    namespace: dev
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

### 2. Eerste service deployen

Een minimale echo-service om routing te testen:

```yaml
# infra-k8s/base/echo/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: echo
spec:
  replicas: 1
  selector:
    matchLabels:
      app: echo
  template:
    metadata:
      labels:
        app: echo
    spec:
      containers:
        - name: echo
          image: ealen/echo-server:latest
          ports:
            - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: echo
spec:
  selector:
    app: echo
  ports:
    - port: 80
```

### 3. IngressRoute aanmaken

```yaml
# infra-k8s/base/echo/ingressroute.yaml
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: echo
spec:
  entryPoints:
    - web
  routes:
    - match: Host(`echo.localhost`)
      kind: Rule
      services:
        - name: echo
          port: 80
```

Test: voeg `127.0.0.1 echo.localhost` toe aan `/etc/hosts`, dan `curl http://echo.localhost`.

### 4. Middleware: rate limiting

```yaml
# infra-k8s/base/echo/middleware.yaml
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: rate-limit
spec:
  rateLimit:
    average: 10
    burst: 20
```

Koppel aan de IngressRoute:
```yaml
routes:
  - match: Host(`echo.localhost`)
    middlewares:
      - name: rate-limit
```

### 5. Traefik dashboard

```bash
kubectl port-forward -n dev svc/traefik 9000:9000
```

Open: http://localhost:9000/dashboard/

---

## Definitie van klaar

- [ ] Traefik draait en is deployed via ArgoCD
- [ ] `http://echo.localhost` geeft een response
- [ ] Traefik dashboard is bereikbaar
- [ ] Rate-limit middleware is actief (test met `ab` of `hey`)
- [ ] Een tweede service draait op een ander hostname (bijv. `api.localhost`)
- [ ] Alles staat in Git, ArgoCD synct automatisch

---

## Referenties

- [Traefik docs](https://doc.traefik.io/traefik/)
- [IngressRoute CRD](https://doc.traefik.io/traefik/routing/providers/kubernetes-crd/)
- [Traefik Helm chart](https://github.com/traefik/traefik-helm-chart)
