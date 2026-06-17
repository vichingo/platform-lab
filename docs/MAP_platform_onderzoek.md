# Managed API Platform (MAP) — Onderzoek

> Adapcare | adapcare.atlassian.net | Jira board: MAP | Sprint 8 actief (3 jun – 17 jun 2026)

---

## Use cases (prioriteit)

| Use case | Focus | Mode | Richting |
|----------|-------|------|----------|
| **API** — extern systeem raadpleegt ECD data | **Nu actief** | Synchroon | Eenrichtingsverkeer |
| **BI** — klant exporteert grote hoeveelheden ECD data naar BI-tooling | Toekomst | Asynchroon | Eenrichtingsverkeer |
| **App dev** — Carebeat gebruikt eigen ECD data als dataset voor interne apps | Toekomst | Synchroon | Bidirectioneel (multi-master?) |
| **Subscriptions** — externe app ontvangt webhook bij datawijziging | Toekomst | Event-driven | Eenrichtingsverkeer |

---

## Wat is het MAP?

Het Managed API Platform is een **middleware-platform** dat staat tussen Pluriform Zorg (PZ) — het interne ECD/EPD-systeem van Adapcare — en externe partijen (zorgorganisaties, applicaties, partners). Het platform biedt:

- Externe partijen beveiligde API-toegang tot PZ-data via tokens + certificaten
- Een sync-mechanisme waarmee PZ-wijzigingen asynchroon worden bijgehouden in een Externe Database (EDB)
- Een Self-Service Portal voor klanten om koppelingen zelf te beheren
- Standaard authenticatie, certificaatbeheer en monitoring voor alle integraties

**Eerste klant in productie: CACAO** (care platform, koppeling via FHIR + EntraID SSO)

---

## Architectuur — hoog over

```
Externe Partij
    │
    ▼ (API call + token/cert)
Traefik Gateway  ──── Keycloak (token validatie)
    │                 OpenBao PKI (cert validatie)
    ▼
EventBridge (Node.js/TS)
    │
    ├──► CscBridge (op PZ Windows server, .NET Windows service)
    │         │
    │         └──► Pluriform Zorg (via CareConnector)
    │
    └──► Externe Database (EDB)  ◄── Transformatie Service ◄── PZ
              │
              └──► Semantische Service ──► Externe Partij (response)
```

**Architecturele beslissing — GENOMEN (17 maart 2026, Gerhart/Roel/Paul + Bart/Ruud):**
Beide opties worden **parallel** uitgevoerd:
- **Option A (EventBus — huidig):** Queue-mechanisme naar MAP brengen. Acceptabele performance voor nu. Klanten worden later gemigreerd.
- **Option B (Sync Service — eindoplossing):** Async PZ→EDB sync parallel ontwikkelen (Transformatie Service, Schema Service, EDB). Zodra klaar: klanten omzetten.

Reden: klantcommitments zo klein mogelijk houden, performance van Option A is bewust geaccepteerd als tijdelijk.

---

## Technische stack — vastgesteld

| Laag | Tool/Platform | Status |
|------|--------------|--------|
| Cloud | AWS (EKS, ECR, ALB, RDS PostgreSQL, Route53, VPC, S3) | Live |
| Kubernetes | Amazon EKS (`ac-eks-nonprod`, `ac-eks-prd`) | Live |
| GitOps/CD | ArgoCD + Kustomize (base + overlays per env) | Live |
| Ingress/Gateway | Traefik (IngressRoute CRDs) | Live — gekozen boven Kong/KrakenD (ADR.001) |
| Auth/IdP | Keycloak (OAuth2/OIDC, M2M, realm per env) | Live |
| Secrets + PKI | OpenBao (open-source Vault fork, certificate CA) | Live |
| IaC AWS-kant | Terraform/OpenTofu + Terragrunt (door Cambrian) | Live |
| IaC K8s-kant | Kustomize overlays in `infra-k8s` repo | Live |
| Node-autoscaling | Karpenter | **Live** (door Cambrian opgezet) |
| CI/CD | GitHub Actions → ECR → ArgoCD sync | Live |
| Container registry | AWS ECR | Live |
| Backend services | TypeScript / Node.js (monorepo: `carebeat-services-catalog`) | Live |
| On-prem connector | CscBridge (.NET Windows service op PZ-servers) | Live |
| Frontend | Vite + React | Live |
| Database | RDS PostgreSQL (voor Configuration Service) | In aanleg |
| File service | SFTPGo (open source, S3 backend, OIDC plugin) — ADR.002 | Gepland |

---

## Technische stack — gepland / nog niet live

| Laag | Tool | Staat |
|------|------|-------|
| Message broker | NATS + JetStream (vs SQS / Redpanda — beslissing open) | Gepland |
| Metrics | Prometheus + Thanos → S3 | Gepland |
| Logs | Loki → S3 | Gepland |
| Tracing | Tempo (OpenTelemetry) | Gepland |
| Dashboards | Grafana | MAP-151 open, hoge prio |
| Observability centraal | Datadog (metadata + langetermijn) | Gepland |
| Event-driven autoscaling | KEDA (NATS queue-based) | Gepland |
| Backup | Velero (cluster state + PVCs) | Gepland |
| Security scanning | Trivy / Snyk (in CI/CD) | Gepland |
| Service mesh (optioneel) | Linkerd / Istio (mTLS intern) | Toekomst |
| Node provisioning | Karpenter → (later) geavanceerder | Basis live |

---

## Omgevingen

| Omgeving | Cluster | Namespace | URL-patroon |
|----------|---------|-----------|------------|
| Development | ac-eks-nonprod | dev | `*.dev.connector.adapcare.nl` |
| Acceptatie | ac-eks-nonprod | acc | `*.nonprod.connector.adapcare.nl` |
| Productie | ac-eks-prd | prd | `*.connector.adapcare.nl` |

OpenBao staat op een **shared EC2-instance** (buiten de K8s cluster), met eigen PKI-structuur: root CA → intermediate CA → leaf certs per namespace/omgeving.

---

## Wat is er klaar (Sprints 1–7, t/m jun 2026)

### Release 1.0 (Sprint 5, 13 mei 2026)
De initiële release bevatte:

**Infrastructuur (Cambrian + Bart + Jeroen)**
- AWS account aangemaakt, IAM-rollen, VPC, EKS nonprod cluster
- S3 voor Terraform state, ECR repositories
- Karpenter voor node-autoscaling
- VPC peering (MAP-95), ALB + Ingress (MAP-79), Load Balancer (MAP-76)
- OpenBao ingericht: Root CA, intermediate CA, namespaces dev/acc
- ArgoCD opgezet met GitOps workflow, dev/acc namespaces

**Platform services**
- Traefik als ingress controller met IngressRoute CRDs
- Keycloak: M2M realm, client credentials, ForwardAuth middleware
- Certificaat-gebaseerde authenticatie (mTLS) via OpenBao PKI
- Webhook service endpoint (`webhook.dev.connector.adapcare.nl`)

**Applicaties (Ewout, Youri, Jasper)**
- EventBridge: routing van externe calls naar CscBridge
- CscBridge: .NET Windows service, healthcheck, version endpoint (`/info`)
- Tenant-isolatie op gateway-niveau (`x-tenant-id` header)
- Interne + externe documentatieportal (`docs.dev.connector.adapcare.nl`)

### Sprints 6–7 (mei–jun 2026)
- Self-Service Portal: frontend (React/Vite) + API backend (Node.js), ArgoCD deployed
- CACAO als eerste klant: tenant allowed in EventBridge, SSO via Microsoft EntraID (MAP-148)
- Interne gateway voor PZ-interne communicatie (MAP-141)
- POST/PUT/DELETE implementatie op EventBridge (MAP-126)
- CSC Bridge healthcheck (MAP-125)
- Productie namespace ingericht (MAP-31)
- Deploy scripts voor self-service portal

---

## Wat is er nog NIET klaar (Sprint 8 + backlog)

### Sprint 8 — actief op dit moment

| Issue | Omschrijving | Wie | Status |
|-------|-------------|-----|--------|
| MAP-164 | **Configuration Service** opzetten (REST API voor Org > Service > Env > Destination config, PostgreSQL backend) | Youri | In Progress |
| MAP-173 | Config service maken (code) | Youri | Ready for Testing |
| MAP-185 | Database structuur implementeren | Youri | Ready for Testing |
| MAP-186 | Workflow bestanden maken | Youri | In Progress |
| MAP-176 | **Infra bestanden aanmaken** (K8s manifests voor config-service) | Jeroen | To Do |
| MAP-177 | **Argo applicatie aanmaken** (config-service in ArgoCD) | Jeroen | To Do |
| MAP-178 | **Deployment naar Acceptatie** | Jeroen | To Do |
| MAP-172 | Database aanmaken (RDS) | Jeroen | To Do |
| MAP-149 | **Productie deployen** | Bart | In Progress |
| MAP-151 | **Grafana + Datadog dashboards** inrichten | Jeroen | To Do — Hoge prio |
| MAP-171 | Documentatie endpoints updaten | Youri | In Progress |
| MAP-174 | POC: opzet voor testen sync | Jasper | In Progress |
| MAP-132 | Authentication — Sessiebeheer (session timeout, refresh) | Ewout | Ready for Testing |
| MAP-119 | Werkruimte aanmaken vanuit PF → CACAO (FHIR endpoint) | Ewout | Ready for Testing |
| MAP-107 | Bug: EventBridge geeft 200 terug i.p.v. 500 bij PZ-fout | Ewout | Ready for Testing |
| MAP-120 | Documentatie opmaak valideren | Ruud | In Progress |

### Grote open punten (buiten Sprint 8)

| Onderdeel | Toelichting |
|-----------|------------|
| **Observability stack** | Prometheus, Loki, Tempo, Grafana, Datadog — volledig nog op te bouwen |
| **NATS message broker** | Keuze tussen NATS / SQS / Redpanda nog open. Kern van toekomstige event-driven architectuur |
| **Sync Service (Option B)** | De architectuurkeuze voor async PZ→EDB sync ligt bij management. Als goedgekeurd: Transformatie Service, Schema Service, Database Service moeten gebouwd worden |
| **KEDA** | Event-driven autoscaling (op NATS queue-diepte) — gepland maar nog niet gestart |
| **Velero backup** | Cluster backup — gepland |
| **24/7 dienstverlening** | Cambrian moet nog helpen met strategie (open taak op Cambrian-lijst) |
| **IaC-overdracht** | Cambrian moet nog kennis overdragen aan Jeroen over de Terraform/OpenTofu code |
| **Productie hardening** | PEN test was doel van Sprint 5/6, productie-deployment nog in gang |

---

## De Self-Service Portal (kernproduct)

Dit is het klantvacerende product van het MAP. Klanten beheren hier zelf hun koppelingen:

**Functionaliteit (in scope):**
- Koppelvlakken aanvragen, uitbreiden, inactiveren
- Credentials (client ID + secret) beheren en intrekken
- Certificaten bekijken en vernieuwing aanvragen
- Endpoint-autorisaties beheren (scope per koppeling)
- Live health monitor per koppeling

**Actoren:**
- Klant beheerder (functioneel beheer van de zorgorganisatie)
- Interne consultant (Carebeat/customer care)
- Certificaat manager (keurt certificaataanvragen goed)
- PM + Security Officer (keuren nieuwe koppelingen goed via Jira workflow)

**Status:** Basis portal live (login/logout, sessies), Config Service (backend voor koppelingsdata) nu in aanleg (Sprint 8).

---

## Sync-architectuur (Option B) — updatebericht schema

Wanneer een object wijzigt in PZ wordt een gestandaardiseerd updatebericht gestuurd naar de Transformatie Service.

**Berichtstructuur (JSON, versie 1.0.0):**

```json
{
  "header": {
    "messageId": "uuid-v4",
    "timestamp": "2025-01-15T10:32:00.000Z",
    "schemaVersion": "1.0.0",
    "sourceApplication": "PluriFormZorg",
    "syncType": "incremental | full",
    "mutationType": "create | update | delete"
  },
  "object": {
    "objectType": "Cliënt",
    "ak": "PZ-CLIENT-00123"
  },
  "changes": {
    "fields": [
      { "ak": "PZ-FIELD-GEBOORTEDATUM", "newValue": "1980-05-14" }
    ]
  }
}
```

**Kernprincipes:**
- Identificatie via **Application Key (AK)** — stabiel bij hernoeming van velden/objecten
- Standaard **incrementeel** (alleen gewijzigde velden); **full sync** handmatig triggerbaar per objecttype vanuit Carebeat
- **Dead references** worden expliciet ondersteund: transformatieservice mag niet afhankelijk zijn van volgorde van ontvangst
- **Retry** met behoud van originele `timestamp` (idempotentie)
- Per objecttype configureerbaar in/uitschakelen (zonder code-deployment)
- Conflict resolutie: **last-write-wins**
- `delete`-berichten bevatten geen `changes`-blok — alleen het `object.ak` is voldoende

**Schema Service (gepland):**
Centrale service die objectstructuren beschikbaar stelt op basis van bronapp + schemaversie + objecttype. PZ levert Swagger/JSON Schema via CareConnector. Historische versies worden bewaard (ook fixpacks). Voedt ook de documentatieportal.

**EDB ontsluiting (gepland):**
- In scope: REST GET/SEARCH, FHIR queries
- Toekomst: POST/PUT/DELETE, subscriptions, bulk export (SQL/ODBC/JDBC/OData voor BI-tooling)
- Auth: certificaten + OAuth conform MAP-standaard
- NEN-7513 logging in overweging
- Multi-tenancy: nog open vraag (row-level of aparte DB per klant)

---

## Authenticatie-aanpak (meerdere lagen)

1. **M2M (machine-to-machine):** Keycloak client credentials → Bearer token → Traefik ForwardAuth middleware valideert. Token TTL max 1 uur. Geen refresh tokens bij M2M. JWKS endpoint voor lokale token-validatie.
2. **Certificaat-authenticatie:** mTLS via OpenBao PKI; `tenant_id` wordt geverifieerd across beide auth-methoden
3. **Self-service portal login:** Keycloak OIDC → sessietokens, refresh via Keycloak, session timeout waarschuwing (MAP-132)
4. **SSO externe partijen:** CACAO gebruikt Microsoft EntraID → Keycloak identity brokering

---

## CscBridge — on-prem connector

De CscBridge is een .NET Windows Service die draait op dezelfde servers als Pluriform Zorg:
- Communiceert via **TCP** met EventBridge (in AWS)
- Communiceert via **HTTP** met Pluriform CareConnector (lokaal)
- Queue-mechanisme: `GET /queue` + `PUT /queue` (correlation ID gebaseerd)
- Geïnstalleerd via installatiescript op Windows servers

**Debugging:** lokaal via ngrok (cscBridge exposed via ngrok, URL tijdelijk in Pluriform instellen).

---

## EventBridge — kernservice

Node.js/TS service binnen het MAP-cluster:
- Ontvangt externe API-calls via Traefik
- Routeert naar connectoren (momenteel: care-connector → CscBridge)
- Handelt failover af: 503 als CscBridge onbereikbaar, 504 bij timeout op antwoord
- `x-tenant-id` header wordt door Keycloak/Traefik geïnjecteerd en gevalideerd

---

## Team

| Persoon | Rol |
|---------|-----|
| Bart Janssen (b.janssen@adapcare.nl) | PO / Architect |
| Youri de Gooijer (y.de.gooijer@adapcare.nl) | Backend developer (Node.js/TS) |
| Jeroen Elshout (j.elshout@adapcare.nl) | Infra / DevOps (K8s, ArgoCD, AWS) |
| Ewout Gort (e.gort@adapcare.nl) | Backend developer (EventBridge, CscBridge, integraties) |
| Jasper Verhaar (j.verhaar@adapcare.nl) | Developer (sync, testing) |
| Ruud Wentink (r.wentink@adapcare.nl) | Documentatie / QA |
| **Cambrian** | AWS advisory, Terraform, initiële infra setup |

---

## AWS IAM-rollen (relevant voor infra-rol)

Twee rollen zijn gedefinieerd voor de infra-kant:

**Infrastructure Engineer** (read + troubleshoot):
`eks:Describe*/List*/AccessKubernetesApi`, `ec2:Describe*`, `cloudwatch:Get*/List*`, `logs:Get*/Describe*`, `iam:Get*/List*`, `route53:Get*/List*`, `elasticloadbalancing:Describe*`

**Semi-Administrator** (alles boven, plus):
`eks:*`, `ec2:*`, `iam:CreateRole/DeleteRole/AttachPolicy/PassRole`, `ecr:*`, `autoscaling:*`, `kms:*`, `acm:*`, `route53:*` (scoped), `s3:*` (project buckets)

---

## Architectuurbesluiten (ADRs)

| ADR | Onderwerp | Beslissing | Afgewezen alternatieven |
|-----|-----------|-----------|------------------------|
| ADR.001 | API Gateway voor grote requests (>10MB) | **Traefik** — auto-discovery, community, geen DB, dashboard | KrakenD, Kong (licentiekosten), Express Gateway |
| ADR.002 | File service voor grote bestanden | **SFTPGo** — open source, meerdere S3 buckets, OIDC plugin, per-user virtual dirs | AWS Signed URL (2-step upload, dev overhead), Minio (klein community) |
| ADR.003 | OAuth/IdP | **Keycloak** — open source enterprise, organisaties/tenants, on-premise, JWKS, mTLS | Authentik (enterprise license nodig), AWS Cognito (vendor lock-in), Zitadel (ontbrekende scope grouping) |

---

## Relevante URL's

| Omgeving | URL |
|----------|-----|
| Docs (dev) | `https://docs.dev.connector.adapcare.nl` |
| Auth (nonprod) | `https://auth.nonprod.connector.adapcare.nl` |
| Webhook (dev) | `https://webhook.dev.connector.adapcare.nl` |
| Jira board | `https://adapcare.atlassian.net/jira/software/c/projects/MAP/boards/888` |
| Confluence | `https://adapcare.atlassian.net/wiki/spaces/MAP/overview` |

---

## Open issues (bekende knelpunten)

| # | Issue | Status | Toelichting |
|---|-------|--------|------------|
| 1 | Applicatiebeheer/ops bemensing | Open | Inzicht geven in type werkzaamheden; wie neemt dit over vanuit dev-kant? |
| 7 | Accountinfo meegeven bij API calls | Open | Zonder account-context kan een arts journaalregels van een fysiotherapeut lezen — autorisatie-lek bij rolgebaseerde toegang in PZ |
| 8 | SSO CACAO kost te veel tijd | Open | EntraID + Keycloak in productie kost Ewout disproportioneel veel tijd; bespreken met Gerhart |
| 3 | Jira workflow inrichten | Bezig | Nieuwe flow voor aanvragen van API-koppelingen, certificaten, etc. |

---

## Relevantie voor de K8s platform-rol

De PAD omschrijft expliciet twee rollen volledig voor Adapcare: **Kubernetes Administrator** en **Platform Engineer**. Cambrian doet alleen de AWS-kant (Terraform, VPC, EKS provisioning). Alles ín de cluster is Adapcare's verantwoordelijkheid.

**Wat de rol nu concreet vraagt:**

1. **ArgoCD beheer:** Elke sprint landen er nieuwe services (config-service nu, NATS straks). Manifesten omzetten naar werkende K8s-configuratie in `infra-k8s`.
2. **Observability stack bouwen:** Prometheus + Loki + Tempo + Grafana + Datadog-integratie — volledig open, hoge prioriteit (MAP-151).
3. **NATS cluster opzetten:** Zodra de message broker-beslissing valt, moet de hele NATS + JetStream cluster uitgerold worden incl. KEDA scalers.
4. **Keycloak realm-beheer:** Bij elke nieuwe klant: realm, client, scopes, identity brokering configureren.
5. **OpenBao PKI:** Certificate lifecycle, rotatie, TTLs, PKI rollen per namespace.
6. **Traefik middleware:** IngressRoutes, ForwardAuth, rate limiting, circuit breakers configureren per service.
7. **Karpenter:** Node-scaling al live, maar tuning + uitbreiding bij groei.
8. **Vertaalslag dev → infra:** Developers geven env-variabelen en service-configuratie (zoals MAP-176), jij maakt de K8s manifests, ArgoCD applicatie en secrets in OpenBao.

**Huidig knelpunt:** Jeroen doet dit nu alleen en loopt achter — de config-service infra-taken (MAP-172/176/177/178) staan allemaal nog open terwijl de code al bijna klaar is. Hier zit de directe toegevoegde waarde.
