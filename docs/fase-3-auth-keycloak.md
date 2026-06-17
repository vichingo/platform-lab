# Fase 3 — Auth stack (Keycloak)

**Status:** Niet gestart
**Vereiste:** Fase 2 afgerond
**MAP-equivalent:** Keycloak M2M realm, client credentials, ForwardAuth middleware (ADR.003)

---

## Doel

Keycloak deployen, een M2M realm configureren, en Traefik koppelen via ForwardAuth zodat alle requests naar beschermde routes eerst een Bearer token validatie doorlopen. Daarna een OIDC-flow voor een echte gebruiker (browser-login).

## Wat je leert

- Keycloak realms, clients, scopes en protocol mappers
- OAuth2 client credentials flow (M2M)
- OIDC authorization code flow (gebruiker)
- Hoe Traefik's ForwardAuth middleware werkt
- Hoe `tenant-id` via een custom claim in het token zit
- Token TTL, refresh tokens (en waarom M2M die niet gebruikt)
- Waarom Keycloak boven Authentik/Cognito/Zitadel (ADR.003)

---

## Vereisten

- Fase 1 + 2 afgerond
- `helm` CLI

---

## Stappen

### 1. Keycloak deployen

```yaml
# infra-k8s/base/keycloak/application.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: keycloak
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://charts.bitnami.com/bitnami
    chart: keycloak
    targetRevision: "21.x.x"
    helm:
      values: |
        auth:
          adminUser: admin
          adminPassword: "changeme"
        service:
          type: ClusterIP
        ingress:
          enabled: false
  destination:
    server: https://kubernetes.default.svc
    namespace: dev
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

IngressRoute voor Keycloak:
```yaml
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: keycloak
spec:
  entryPoints:
    - web
  routes:
    - match: Host(`auth.localhost`)
      kind: Rule
      services:
        - name: keycloak
          port: 80
```

Voeg toe aan `/etc/hosts`: `127.0.0.1 auth.localhost`

### 2. M2M realm aanmaken

Via Keycloak admin UI (http://auth.localhost):

1. Nieuwe realm aanmaken: `platform-lab`
2. Realm settings → Tokens:
   - Access token lifespan: **1 uur** (max, conform MAP)
   - Refresh token: **uitgeschakeld** voor M2M
3. Client aanmaken:
   - Client ID: `webhook-service`
   - Client authentication: ON
   - Authorization: OFF
   - Grant types: alleen **Client credentials**
4. Client scope aanmaken: `webhook:write`
5. Protocol mapper toevoegen: custom claim `tenant-id`
   - Mapper type: Hardcoded claim
   - Claim name: `tenant_id`
   - Claim value: `tenant-a`

Token ophalen (test):
```bash
curl -X POST http://auth.localhost/realms/platform-lab/protocol/openid-connect/token \
  -d "grant_type=client_credentials" \
  -d "client_id=webhook-service" \
  -d "client_secret=<secret>"
```

Controleer de `tenant_id` claim in het token:
```bash
# Kopieer de access_token en decodeer:
echo "<access_token>" | cut -d'.' -f2 | base64 -d | jq .
```

### 3. ForwardAuth middleware in Traefik

Keycloak Gatekeeper / OAuth2-Proxy deployen als ForwardAuth sidecar:

```yaml
# infra-k8s/base/auth-proxy/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: oauth2-proxy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: oauth2-proxy
  template:
    metadata:
      labels:
        app: oauth2-proxy
    spec:
      containers:
        - name: oauth2-proxy
          image: quay.io/oauth2-proxy/oauth2-proxy:latest
          args:
            - --provider=keycloak-oidc
            - --client-id=webhook-service
            - --client-secret=<secret>
            - --oidc-issuer-url=http://auth.localhost/realms/platform-lab
            - --upstream=static://200
            - --http-address=0.0.0.0:4180
```

Traefik ForwardAuth middleware:
```yaml
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: keycloak-auth
spec:
  forwardAuth:
    address: http://oauth2-proxy:4180/oauth2/auth
    authResponseHeaders:
      - X-Auth-Request-User
      - X-Auth-Request-Email
```

### 4. Beschermde route testen

Voeg middleware toe aan een IngressRoute:
```yaml
routes:
  - match: Host(`api.localhost`)
    middlewares:
      - name: keycloak-auth
    services:
      - name: echo
        port: 80
```

Test zonder token → 401. Met geldig token → 200.

---

## Definitie van klaar

- [ ] Keycloak draait en is bereikbaar via `http://auth.localhost`
- [ ] Realm `platform-lab` bestaat met M2M client
- [ ] `curl` met client credentials geeft een JWT terug
- [ ] JWT bevat `tenant_id` claim
- [ ] `api.localhost` geeft 401 zonder token, 200 met geldig token
- [ ] Alles deployed via ArgoCD vanuit Git

---

## Referenties

- [Keycloak docs](https://www.keycloak.org/documentation)
- [OAuth2-Proxy](https://oauth2-proxy.github.io/oauth2-proxy/)
- [Keycloak client credentials](https://www.keycloak.org/docs/latest/securing_apps/#_client_credentials)
