# Fase 1 — Cluster + GitOps fundament

**Status:** Niet gestart
**MAP-equivalent:** EKS clusters + ArgoCD setup (Sprints 1–2)

---

## Doel

Een lokale Kubernetes-cluster draaien met ArgoCD als GitOps controller. Alles wat daarna in het cluster komt, wordt via Git gedeployed — nooit meer `kubectl apply` met de hand.

## Wat je leert

- Hoe k3d werkt (lightweight K8s via Docker)
- Kustomize: `base/` + `overlays/` structuur (exact zoals MAP's `infra-k8s` repo)
- ArgoCD App of Apps patroon
- Hoe een change in Git automatisch in de cluster landt
- Namespace-strategie (dev/prod)

---

## Vereisten

- Docker Desktop of Colima
- `kubectl`, `k3d`, `kustomize`, `argocd` CLI tools

```bash
brew install k3d kubectl kustomize argocd
```

---

## Stappen

### 1. Cluster aanmaken

```bash
k3d cluster create platform-lab \
  --api-port 6550 \
  --port "80:80@loadbalancer" \
  --port "443:443@loadbalancer" \
  --agents 2
```

Controleer:
```bash
kubectl get nodes
```

### 2. Namespaces aanmaken

```bash
kubectl create namespace dev
kubectl create namespace prod
kubectl create namespace argocd
```

### 3. ArgoCD installeren

```bash
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
kubectl wait --for=condition=available deployment/argocd-server -n argocd --timeout=120s
```

Port-forward om de UI te bereiken:
```bash
kubectl port-forward svc/argocd-server -n argocd 8080:443
```

Initieel wachtwoord ophalen:
```bash
argocd admin initial-password -n argocd
```

Login: https://localhost:8080

### 4. infra-k8s repo structuur opzetten

Maak de basisstructuur in deze repo aan (in `infra-k8s/`):

```
infra-k8s/
├── base/
│   └── namespaces/
│       └── kustomization.yaml
└── overlays/
    ├── dev/
    │   └── kustomization.yaml
    └── prod/
        └── kustomization.yaml
```

`infra-k8s/base/namespaces/kustomization.yaml`:
```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources: []
```

`infra-k8s/overlays/dev/kustomization.yaml`:
```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
bases:
  - ../../base
namespace: dev
```

### 5. App of Apps in ArgoCD

Maak een ArgoCD Application die naar `infra-k8s/overlays/dev` in jouw Git repo wijst:

```yaml
# infra-k8s/argocd/app-of-apps.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: platform-lab-dev
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/<jouw-username>/platform-lab
    targetRevision: HEAD
    path: infra-k8s/overlays/dev
  destination:
    server: https://kubernetes.default.svc
    namespace: dev
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

```bash
kubectl apply -f infra-k8s/argocd/app-of-apps.yaml
```

---

## Definitie van klaar

- [ ] `kubectl get nodes` toont 3 nodes (1 server + 2 agents)
- [ ] ArgoCD draait en is bereikbaar via browser
- [ ] `infra-k8s/overlays/dev/` is geregistreerd als ArgoCD Application
- [ ] Een dummy ConfigMap via een commit in Git landt automatisch in de cluster
- [ ] `dev` en `prod` namespaces bestaan

---

## Referenties

- [k3d documentatie](https://k3d.io)
- [ArgoCD getting started](https://argo-cd.readthedocs.io/en/stable/getting_started/)
- [Kustomize](https://kustomize.io)
