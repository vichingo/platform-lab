# Fase 4 — Secrets + PKI (OpenBao)

**Status:** Niet gestart
**Vereiste:** Fase 3 afgerond
**MAP-equivalent:** OpenBao op shared EC2, root CA → intermediate CA → leaf certs, VaultStaticSecret

---

## Doel

OpenBao (open-source Vault fork) draaien buiten de cluster (op host of aparte container), PKI-hiërarchie opzetten (root CA → intermediate CA → leaf), en K8s secrets injecteren via de Vault Secrets Operator. Daarna mTLS aanzetten op een service.

## Wat je leert

- Hoe een PKI-hiërarchie werkt: root CA / intermediate CA / leaf certs
- OpenBao Raft storage (hoe MAP het ook doet)
- Certificate lifecycle: TTL, rotatie, intrekking
- VaultStaticSecret: secrets vanuit OpenBao automatisch als K8s Secret
- Waarom PKI buiten de cluster staat (availability bij cluster-issues)
- mTLS: beide kanten presenteren een certificaat

---

## Vereisten

- Fase 1–3 afgerond
- `bao` CLI (OpenBao) of `vault` CLI (compatibel)

```bash
brew install openbao   # of: brew install vault
```

---

## Stappen

### 1. OpenBao draaien (buiten cluster)

In dev: draai OpenBao in dev-mode als Docker container op de host:

```bash
docker run --rm -d \
  --name openbao \
  -p 8200:8200 \
  -e VAULT_DEV_ROOT_TOKEN_ID=root \
  -e VAULT_DEV_LISTEN_ADDRESS=0.0.0.0:8200 \
  openbao/openbao:latest server -dev
```

Exporteer:
```bash
export VAULT_ADDR='http://localhost:8200'
export VAULT_TOKEN='root'
bao status
```

Later (productie-achtig): Raft storage opzetten:
```bash
docker run -d --name openbao \
  -p 8200:8200 \
  -v openbao-data:/vault/data \
  openbao/openbao:latest server \
  -config=/vault/config/config.hcl
```

### 2. PKI hiërarchie opzetten

```bash
# Root CA
bao secrets enable -path=pki pki
bao secrets tune -max-lease-ttl=87600h pki
bao write pki/root/generate/internal \
  common_name="Platform Lab Root CA" \
  ttl=87600h

# Intermediate CA
bao secrets enable -path=pki_int pki
bao secrets tune -max-lease-ttl=43800h pki_int

bao write pki_int/intermediate/generate/internal \
  common_name="Platform Lab Intermediate CA"

# Laat root de intermediate CA ondertekenen
bao write pki/root/sign-intermediate \
  csr=@intermediate.csr \
  ttl=43800h

# PKI rol voor leaf certs
bao write pki_int/roles/platform-lab \
  allowed_domains="localhost,platform-lab.local" \
  allow_subdomains=true \
  max_ttl=720h
```

### 3. Vault Secrets Operator installeren

```yaml
# infra-k8s/base/vault-operator/application.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: vault-secrets-operator
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://helm.releases.hashicorp.com
    chart: vault-secrets-operator
    targetRevision: "0.x.x"
    helm:
      values: |
        defaultVaultConnection:
          enabled: true
          address: "http://host.k3d.internal:8200"
  destination:
    server: https://kubernetes.default.svc
    namespace: dev
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

### 4. VaultStaticSecret: een secret in K8s injecteren

```yaml
# infra-k8s/base/echo/vault-secret.yaml
apiVersion: secrets.hashicorp.com/v1beta1
kind: VaultStaticSecret
metadata:
  name: echo-api-key
spec:
  type: kv-v2
  mount: secret
  path: platform-lab/echo/config
  destination:
    name: echo-api-key
    create: true
  refreshAfter: 60s
```

Zet eerst de secret in OpenBao:
```bash
bao kv put secret/platform-lab/echo/config api_key="supersecret123"
```

Controleer of het K8s Secret aangemaakt is:
```bash
kubectl get secret echo-api-key -n dev -o jsonpath='{.data.api_key}' | base64 -d
```

### 5. TLS certificaat genereren voor een service

```bash
bao write pki_int/issue/platform-lab \
  common_name="echo.localhost" \
  ttl=72h
```

Mount het certificaat als K8s Secret via VaultStaticSecret, en gebruik het in een Traefik IngressRoute:

```yaml
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: echo-tls
spec:
  entryPoints:
    - websecure
  routes:
    - match: Host(`echo.localhost`)
      services:
        - name: echo
          port: 80
  tls:
    secretName: echo-tls-cert
```

---

## Definitie van klaar

- [ ] OpenBao draait en `bao status` geeft Initialized+Unsealed
- [ ] Root CA + Intermediate CA zijn opgezet
- [ ] Leaf certificaat aangevraagd voor `echo.localhost`
- [ ] VaultStaticSecret synchroniseert een OpenBao secret als K8s Secret
- [ ] `https://echo.localhost` werkt met het OpenBao-certificaat
- [ ] Certificaat rotatie getest (nieuw cert aanvragen, K8s Secret update gecontroleerd)

---

## Referenties

- [OpenBao documentatie](https://openbao.org/docs/)
- [Vault PKI secrets engine](https://developer.hashicorp.com/vault/docs/secrets/pki)
- [Vault Secrets Operator](https://developer.hashicorp.com/vault/docs/platform/k8s/vso)
