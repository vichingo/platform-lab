# Managed API Platform (MAP) — Onderzoek

Persoonlijke notities over een managed-API-platform-architectuur die ik professioneel ben tegengekomen, bijgehouden over een aantal maanden.

## Mijn persoonlijke tijdlijn

| Periode | Wat ik meemaakte |
|---------|------------------|
| Maart 2026 | Voor het eerst goed kennisgemaakt met de architectuur — zag een belangrijke architectuurbeslissing (EventBus vs. Sync Service) langskomen |
| Mei 2026 | Eerste live release meegemaakt; begon structureel notities bij te houden |
| Eind mei – half juni 2026 | Vervolgstappen gezien (self-service portal, eerste externe klant aangesloten) |
| 17 juni 2026 | Notities afgerond — besloten om zelf een mini-versie te bouwen om alles hands-on te leren. Dit repo is het resultaat |

---

## Use cases (prioriteit)

| Use case | Focus | Mode | Richting |
|----------|-------|------|----------|
| **API** — extern systeem raadpleegt ECD data | **Nu actief** | Synchroon | Eenrichtingsverkeer |
| **BI** — klant exporteert grote hoeveelheden ECD data naar BI-tooling | Toekomst | Asynchroon | Eenrichtingsverkeer |
| **App dev** — intern app-team gebruikt eigen ECD data als dataset voor interne apps | Toekomst | Synchroon | Bidirectioneel (multi-master?) |
| **Subscriptions** — externe app ontvangt webhook bij datawijziging | Toekomst | Event-driven | Eenrichtingsverkeer |

---

## Wat is het MAP?

Het Managed API Platform is een **middleware-platform** dat staat tussen het interne ECD/EPD-systeem en externe partijen (zorgorganisaties, applicaties, partners). Het platform biedt:

- Externe partijen beveiligde API-toegang tot ECD-data via tokens + certificaten
- Een sync-mechanisme waarmee wijzigingen in het ECD-systeem asynchroon worden bijgehouden in een Externe Database (EDB)
- Een Self-Service Portal voor klanten om koppelingen zelf te beheren
- Standaard authenticatie, certificaatbeheer en monitoring voor alle integraties

**Eerste klant in productie: Klant A** (care platform, koppeling via FHIR + EntraID SSO)

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
    ├──► OnPremBridge (op ECD Windows server, .NET Windows service)
    │         │
    │         └──► ECD-systeem (via interne connector)
    │
    └──► Externe Database (EDB)  ◄── Transformatie Service ◄── ECD-systeem
              │
              └──► Semantische Service ──► Externe Partij (response)
```

**Architecturele beslissing — GENOMEN (17 maart 2026):**
Beide opties worden **parallel** uitgevoerd:
- **Option A (EventBus — huidig):** Queue-mechanisme naar MAP brengen. Acceptabele performance voor nu. Klanten worden later gemigreerd.
- **Option B (Sync Service — eindoplossing):** Async ECD→EDB sync parallel ontwikkelen (Transformatie Service, Schema Service, EDB). Zodra klaar: klanten omzetten.

Reden: klantcommitments zo klein mogelijk houden, performance van Option A is bewust geaccepteerd als tijdelijk.

---

## Technische stack — vastgesteld

| Laag | Tool/Platform | Status |
|------|--------------|--------|
| Cloud | AWS (EKS, ECR, ALB, RDS PostgreSQL, Route53, VPC, S3) | Live |
| Kubernetes | Amazon EKS (nonprod / prd clusters) | Live |
| GitOps/CD | ArgoCD + Kustomize (base + overlays per env) | Live |
| Ingress/Gateway | Traefik (IngressRoute CRDs) | Live — gekozen boven Kong/KrakenD (ADR.001) |
| Auth/IdP | Keycloak (OAuth2/OIDC, M2M, realm per env) | Live |
| Secrets + PKI | OpenBao (open-source Vault fork, certificate CA) | Live |
| IaC AWS-kant | Terraform/OpenTofu + Terragrunt (door externe infra-partner) | Live |
| IaC K8s-kant | Kustomize overlays in `infra-k8s` repo | Live |
| Node-autoscaling | Karpenter | **Live** (door externe infra-partner opgezet) |
| CI/CD | GitHub Actions → ECR → ArgoCD sync | Live |
| Container registry | AWS ECR | Live |
| Backend services | TypeScript / Node.js (monorepo: `services-catalog`) | Live |
| On-prem connector | OnPremBridge (.NET Windows service op ECD-servers) | Live |
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
| Dashboards | Grafana | Open, hoge prio |
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
| Development | eks-nonprod | dev | `*.dev.connector.example.com` |
| Acceptatie | eks-nonprod | acc | `*.nonprod.connector.example.com` |
| Productie | eks-prd | prd | `*.connector.example.com` |

OpenBao staat op een **shared EC2-instance** (buiten de K8s cluster), met eigen PKI-structuur: root CA → intermediate CA → leaf certs per namespace/omgeving.

---

## Wat ik heb zien groeien (voorjaar–vroege zomer 2026)

### Eerste live release die ik meemaakte (13 mei 2026)
De initiële release bevatte:

**Infrastructuur (externe infra-partner + interne infra-team)**
- AWS account aangemaakt, IAM-rollen, VPC, EKS nonprod cluster
- S3 voor Terraform state, ECR repositories
- Karpenter voor node-autoscaling
- VPC peering, ALB + Ingress, Load Balancer
- OpenBao ingericht: Root CA, intermediate CA, namespaces dev/acc
- ArgoCD opgezet met GitOps workflow, dev/acc namespaces

**Platform services**
- Traefik als ingress controller met IngressRoute CRDs
- Keycloak: M2M realm, client credentials, ForwardAuth middleware
- Certificaat-gebaseerde authenticatie (mTLS) via OpenBao PKI
- Webhook service endpoint (`webhook.dev.connector.example.com`)

**Applicaties (backend-team)**
- EventBridge: routing van externe calls naar OnPremBridge
- OnPremBridge: .NET Windows service, healthcheck, version endpoint (`/info`)
- Tenant-isolatie op gateway-niveau (`x-tenant-id` header)
- Interne + externe documentatieportal (`docs.dev.connector.example.com`)

### Vervolgstappen die ik zag (eind mei – half juni 2026)
- Self-Service Portal: frontend (React/Vite) + API backend (Node.js), ArgoCD deployed
- Klant A als eerste klant: tenant allowed in EventBridge, SSO via Microsoft EntraID
- Interne gateway voor ECD-interne communicatie
- POST/PUT/DELETE implementatie op EventBridge
- OnPremBridge healthcheck
- Productie namespace ingericht
- Deploy scripts voor self-service portal

---

## Wat nog openstond toen ik stopte met observeren (medio juni 2026 + backlog)

### Laatste openstaande punten die ik zag (medio juni 2026)

| Omschrijving | Rol | Status |
|-------------|-----|--------|
| **Configuration Service** opzetten (REST API voor Org > Service > Env > Destination config, PostgreSQL backend) | Backend | In Progress |
| Config service maken (code) | Backend | Ready for Testing |
| Database structuur implementeren | Backend | Ready for Testing |
| Workflow bestanden maken | Backend | In Progress |
| **Infra bestanden aanmaken** (K8s manifests voor config-service) | Infra | To Do |
| **Argo applicatie aanmaken** (config-service in ArgoCD) | Infra | To Do |
| **Deployment naar Acceptatie** | Infra | To Do |
| Database aanmaken (RDS) | Infra | To Do |
| **Productie deployen** | PO/Architect | In Progress |
| **Grafana + Datadog dashboards** inrichten | Infra | To Do — Hoge prio |
| Documentatie endpoints updaten | Backend | In Progress |
| POC: opzet voor testen sync | Developer | In Progress |
| Authentication — Sessiebeheer (session timeout, refresh) | Backend | Ready for Testing |
| Werkruimte aanmaken vanuit intern systeem → Klant A (FHIR endpoint) | Backend | Ready for Testing |
| Bug: EventBridge geeft 200 terug i.p.v. 500 bij ECD-fout | Backend | Ready for Testing |
| Documentatie opmaak valideren | Documentatie/QA | In Progress |

### Grote open punten (verder in de backlog)

| Onderdeel | Toelichting |
|-----------|------------|
| **Observability stack** | Prometheus, Loki, Tempo, Grafana, Datadog — volledig nog op te bouwen |
| **NATS message broker** | Keuze tussen NATS / SQS / Redpanda nog open. Kern van toekomstige event-driven architectuur |
| **Sync Service (Option B)** | De architectuurkeuze voor async ECD→EDB sync ligt bij management. Als goedgekeurd: Transformatie Service, Schema Service, Database Service moeten gebouwd worden |
| **KEDA** | Event-driven autoscaling (op NATS queue-diepte) — gepland maar nog niet gestart |
| **Velero backup** | Cluster backup — gepland |
| **24/7 dienstverlening** | Externe infra-partner moet nog helpen met strategie |
| **IaC-overdracht** | Externe infra-partner moet nog kennis overdragen aan intern infra-team over de Terraform/OpenTofu code |
| **Productie hardening** | PEN test was doel van een eerdere fase, productie-deployment nog in gang |

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
- Interne consultant (customer care)
- Certificaat manager (keurt certificaataanvragen goed)
- PM + Security Officer (keuren nieuwe koppelingen goed via issue-tracker workflow)

**Status:** Basis portal live (login/logout, sessies), Config Service (backend voor koppelingsdata) nu in aanleg (laatste fase die ik zag).

---

## Sync-architectuur (Option B) — updatebericht schema

Wanneer een object wijzigt in het ECD-systeem wordt een gestandaardiseerd updatebericht gestuurd naar de Transformatie Service.

**Berichtstructuur (JSON, versie 1.0.0):**

```json
{
  "header": {
    "messageId": "uuid-v4",
    "timestamp": "2025-01-15T10:32:00.000Z",
    "schemaVersion": "1.0.0",
    "sourceApplication": "ECDSysteem",
    "syncType": "incremental | full",
    "mutationType": "create | update | delete"
  },
  "object": {
    "objectType": "Cliënt",
    "ak": "ECD-CLIENT-00123"
  },
  "changes": {
    "fields": [
      { "ak": "ECD-FIELD-GEBOORTEDATUM", "newValue": "1980-05-14" }
    ]
  }
}
```

**Kernprincipes:**
- Identificatie via **Application Key (AK)** — stabiel bij hernoeming van velden/objecten
- Standaard **incrementeel** (alleen gewijzigde velden); **full sync** handmatig triggerbaar per objecttype
- **Dead references** worden expliciet ondersteund: transformatieservice mag niet afhankelijk zijn van volgorde van ontvangst
- **Retry** met behoud van originele `timestamp` (idempotentie)
- Per objecttype configureerbaar in/uitschakelen (zonder code-deployment)
- Conflict resolutie: **last-write-wins**
- `delete`-berichten bevatten geen `changes`-blok — alleen het `object.ak` is voldoende

**Schema Service (gepland):**
Centrale service die objectstructuren beschikbaar stelt op basis van bronapp + schemaversie + objecttype. Het ECD-systeem levert Swagger/JSON Schema via een interne connector. Historische versies worden bewaard (ook fixpacks). Voedt ook de documentatieportal.

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
3. **Self-service portal login:** Keycloak OIDC → sessietokens, refresh via Keycloak, session timeout waarschuwing
4. **SSO externe partijen:** Klant A gebruikt Microsoft EntraID → Keycloak identity brokering

---

## OnPremBridge — on-prem connector

De OnPremBridge is een .NET Windows Service die draait op dezelfde servers als het ECD-systeem:
- Communiceert via **TCP** met EventBridge (in AWS)
- Communiceert via **HTTP** met de interne connector (lokaal)
- Queue-mechanisme: `GET /queue` + `PUT /queue` (correlation ID gebaseerd)
- Geïnstalleerd via installatiescript op Windows servers

**Debugging:** lokaal via ngrok (bridge exposed via ngrok, URL tijdelijk in het ECD-systeem instellen).

---

## EventBridge — kernservice

Node.js/TS service binnen het MAP-cluster:
- Ontvangt externe API-calls via Traefik
- Routeert naar connectoren (momenteel: connector → OnPremBridge)
- Handelt failover af: 503 als OnPremBridge onbereikbaar, 504 bij timeout op antwoord
- `x-tenant-id` header wordt door Keycloak/Traefik geïnjecteerd en gevalideerd

---

## Team

| Rol |
|---------|
| PO / Architect |
| Backend developer (Node.js/TS) |
| Infra / DevOps (K8s, ArgoCD, AWS) |
| Backend developer (EventBridge, OnPremBridge, integraties) |
| Developer (sync, testing) |
| Documentatie / QA |
| Externe infra-partner (AWS advisory, Terraform, initiële infra setup) |

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

## Open issues (bekende knelpunten)

| # | Issue | Status | Toelichting |
|---|-------|--------|------------|
| 1 | Applicatiebeheer/ops bemensing | Open | Inzicht geven in type werkzaamheden; wie neemt dit over vanuit dev-kant? |
| 2 | Accountinfo meegeven bij API calls | Open | Zonder account-context kan een arts journaalregels van een fysiotherapeut lezen — autorisatie-lek bij rolgebaseerde toegang in het ECD-systeem |
| 3 | SSO Klant A kost te veel tijd | Open | EntraID + Keycloak in productie kost disproportioneel veel tijd |
| 4 | Issue-tracker workflow inrichten | Bezig | Nieuwe flow voor aanvragen van API-koppelingen, certificaten, etc. |

---

## Relevantie voor de K8s platform-rol

Twee rollen zijn expliciet relevant: **Kubernetes Administrator** en **Platform Engineer**. De externe infra-partner doet alleen de AWS-kant (Terraform, VPC, EKS provisioning). Alles ín de cluster valt onder de interne verantwoordelijkheid.

**Wat de rol nu concreet vraagt:**

1. **ArgoCD beheer:** Elke fase landen er nieuwe services (config-service nu, NATS straks). Manifesten omzetten naar werkende K8s-configuratie in `infra-k8s`.
2. **Observability stack bouwen:** Prometheus + Loki + Tempo + Grafana + Datadog-integratie — volledig open, hoge prioriteit.
3. **NATS cluster opzetten:** Zodra de message broker-beslissing valt, moet de hele NATS + JetStream cluster uitgerold worden incl. KEDA scalers.
4. **Keycloak realm-beheer:** Bij elke nieuwe klant: realm, client, scopes, identity brokering configureren.
5. **OpenBao PKI:** Certificate lifecycle, rotatie, TTLs, PKI rollen per namespace.
6. **Traefik middleware:** IngressRoutes, ForwardAuth, rate limiting, circuit breakers configureren per service.
7. **Karpenter:** Node-scaling al live, maar tuning + uitbreiding bij groei.
8. **Vertaalslag dev → infra:** Developers geven env-variabelen en service-configuratie, jij maakt de K8s manifests, ArgoCD applicatie en secrets in OpenBao.

**Huidig knelpunt:** Het infra-team doet dit nu alleen en loopt achter — de config-service infra-taken staan allemaal nog open terwijl de code al bijna klaar is. Hier zit de directe toegevoegde waarde.
