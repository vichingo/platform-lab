# Platform Lab

Persoonlijk leerproject gebaseerd op de MAP (Managed API Platform) stack van Adapcare.
Doel: alle technologieën die in MAP gebruikt worden hands-on leren door een werkende mini-versie te bouwen.

## Context

Zie [docs/MAP_platform_onderzoek.md](docs/MAP_platform_onderzoek.md) voor het volledige onderzoek van het MAP-project.

## Stack

| Component | Tool | MAP equivalent |
|-----------|------|---------------|
| Kubernetes | k3d (lokaal) | Amazon EKS |
| GitOps | ArgoCD + Kustomize | ArgoCD + Kustomize |
| Ingress/Gateway | Traefik | Traefik |
| Auth/IdP | Keycloak | Keycloak |
| Secrets + PKI | OpenBao | OpenBao |
| CI/CD | GitHub Actions | GitHub Actions |
| Message broker | NATS + JetStream | NATS + JetStream (gepland) |
| Autoscaling | KEDA | KEDA (gepland) |
| Observability | Prometheus + Loki + Grafana | Gepland (MAP-151) |

## Use case

Een **webhook relay + event bus**: externe services sturen events (GitHub webhooks, HTTP triggers, etc.) naar het platform. Het platform routeert, transformeert en stuurt door naar andere endpoints. Mirrors precies de EventBridge → CscBridge-flow van MAP.

## Fases

| # | Fase | Status |
|---|------|--------|
| 1 | [Cluster + GitOps fundament](docs/fase-1-cluster-gitops.md) | Niet gestart |
| 2 | [Ingress + routing (Traefik)](docs/fase-2-ingress-traefik.md) | Niet gestart |
| 3 | [Auth stack (Keycloak)](docs/fase-3-auth-keycloak.md) | Niet gestart |
| 4 | [Secrets + PKI (OpenBao)](docs/fase-4-secrets-openbao.md) | Niet gestart |
| 5 | [CI/CD pipeline (GitHub Actions)](docs/fase-5-cicd.md) | Niet gestart |
| 6 | [Message broker + autoscaling (NATS + KEDA)](docs/fase-6-nats-keda.md) | Niet gestart |
| 7 | [Observability (Prometheus + Loki + Grafana)](docs/fase-7-observability.md) | Niet gestart |

## Repo structuur

```
platform-lab/
├── docs/                        # Documentatie en fase-gidsen
│   ├── MAP_platform_onderzoek.md
│   ├── fase-1-cluster-gitops.md
│   ├── fase-2-ingress-traefik.md
│   ├── fase-3-auth-keycloak.md
│   ├── fase-4-secrets-openbao.md
│   ├── fase-5-cicd.md
│   ├── fase-6-nats-keda.md
│   └── fase-7-observability.md
└── infra-k8s/                   # Kubernetes manifests (gebouwd per fase)
    ├── base/                    # Gedeelde base manifests
    └── overlays/
        ├── dev/
        └── prod/
```
