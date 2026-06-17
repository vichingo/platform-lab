# Fase 5 — CI/CD pipeline (GitHub Actions)

**Status:** Niet gestart
**Vereiste:** Fase 1–4 afgerond
**MAP-equivalent:** GitHub Actions → ECR → ArgoCD image updater

---

## Doel

Een GitHub Actions workflow bouwen die bij elke push naar `main` automatisch een Docker image bouwt, pusht naar een registry (GitHub Container Registry of Docker Hub), en ArgoCD triggert om de nieuwe versie te deployen. Image tags worden via Kustomize bijgehouden in Git.

## Wat je leert

- GitHub Actions: jobs, steps, secrets, environments
- Container registry workflow (GHCR of Docker Hub)
- Hoe ArgoCD Image Updater een nieuwe tag detecteert en een commit maakt
- Hoe het "push image → update Git → ArgoCD sync"-patroon werkt (het hart van MAP's deployment flow)
- Semantic versioning via Git tags

---

## Vereisten

- Fase 1–4 afgerond
- GitHub account + deze repo is gepusht naar GitHub
- Een simpele applicatie om te bouwen (bijv. de echo-service of een eigen webhook-receiver)

---

## Stappen

### 1. Dockerfile aanmaken voor de webhook-service

```dockerfile
# services/webhook-receiver/Dockerfile
FROM node:20-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY src/ ./src/
EXPOSE 3000
CMD ["node", "src/index.js"]
```

Minimale `src/index.js`:
```js
const http = require('http');

const server = http.createServer((req, res) => {
  let body = '';
  req.on('data', chunk => body += chunk);
  req.on('end', () => {
    console.log(`${req.method} ${req.url}`, body);
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ received: true, path: req.url }));
  });
});

server.listen(3000, () => console.log('webhook-receiver listening on :3000'));
```

### 2. GitHub Actions workflow

```yaml
# .github/workflows/build-push.yaml
name: Build and push

on:
  push:
    branches: [main]
    paths:
      - 'services/webhook-receiver/**'

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}/webhook-receiver

jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write

    steps:
      - uses: actions/checkout@v4

      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Extract metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
          tags: |
            type=sha,prefix=sha-
            type=raw,value=latest,enable={{is_default_branch}}

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: services/webhook-receiver
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
```

### 3. ArgoCD Image Updater installeren

```yaml
# infra-k8s/base/argocd-image-updater/application.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: argocd-image-updater
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://argoproj.github.io/argo-helm
    chart: argocd-image-updater
    targetRevision: "0.x.x"
  destination:
    server: https://kubernetes.default.svc
    namespace: argocd
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

### 4. Deployment annoteren voor automatische image updates

```yaml
# infra-k8s/base/webhook-receiver/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: webhook-receiver
  annotations:
    argocd-image-updater.argoproj.io/image-list: receiver=ghcr.io/<username>/platform-lab/webhook-receiver
    argocd-image-updater.argoproj.io/receiver.update-strategy: latest
    argocd-image-updater.argoproj.io/write-back-method: git
spec:
  replicas: 1
  selector:
    matchLabels:
      app: webhook-receiver
  template:
    metadata:
      labels:
        app: webhook-receiver
    spec:
      containers:
        - name: receiver
          image: ghcr.io/<username>/platform-lab/webhook-receiver:latest
          ports:
            - containerPort: 3000
```

### 5. Flow testen

1. Maak een kleine change in `src/index.js`
2. Commit + push naar `main`
3. Volg de Actions tab op GitHub
4. Na succesvol builden: ArgoCD Image Updater detecteert nieuwe tag
5. Image Updater maakt een commit in Git (update naar nieuwe SHA-tag)
6. ArgoCD detecteert de Git-wijziging en synct
7. `kubectl rollout status deployment/webhook-receiver -n dev`

---

## Definitie van klaar

- [ ] GitHub Actions workflow bouwt en pusht een image bij push naar `main`
- [ ] Image is zichtbaar in GHCR (packages tab)
- [ ] ArgoCD Image Updater draait en detecteert nieuwe images
- [ ] Een code-change → commit → nieuwe pod draait zonder handmatige actie
- [ ] Rollback getest: oudere image-tag in Git zetten → ArgoCD synct terug

---

## Referenties

- [GitHub Actions docs](https://docs.github.com/en/actions)
- [ArgoCD Image Updater](https://argocd-image-updater.readthedocs.io/)
- [GitHub Container Registry](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
