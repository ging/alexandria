# Receta de Despliegue con Docker Compose (Caddy + PostgreSQL + Keycloak)

Esta receta despliega Alexandria sobre infraestructura real:

- **Caddy (`caddy:2.9-alpine`):** Reverse proxy terminando TLS delante del Hub y de Keycloak (`*.127.0.0.1.nip.io` con CA local por defecto; adaptable a cualquier dominio real con Let's Encrypt automático).
- **PostgreSQL (`postgres:17-alpine`):** Persistencia transaccional con tres bases de datos aisladas (`keycloak`, `alexandria_hub` con triggers de inmutabilidad en tablas append-only, y `alexandria_client`).
- **Keycloak (`quay.io/keycloak/keycloak:26.2`):** Proveedor OAuth 2.0 / OIDC real con el realm `alexandria` auto-importado, emisión de access tokens RFC 9068 (`typ: at+jwt`), audiencia `alexandria-vhub`, mapeo del claim `roles` (`vhub-admin`), PKCE (`S256`) para la consola web y clientes CLI/M2M.
- **`alexandria-server`:** Vocabulary Hub compilado con la consola web embebida (`/ui/`).
- **Perfil opcional `demo` (`--profile demo`):** Añade **`alexandria-client`** (respaldado por PostgreSQL y descubriendo automáticamente el `hub_did`) + **`fake-connector`**.

---

## 1. Arranque rápido (local con `*.127.0.0.1.nip.io`)

Desde la raíz del repositorio (con `task` o `make`, o directamente con `docker compose`):

```bash
# Opcional: personalizar variables copiando .env.example
cp deploy/docker/.env.example deploy/docker/.env

# Levantar Hub + Caddy + PostgreSQL + Keycloak:
task docker:up
# (o con el sidecar alexandria-client + fake-connector incluidos:)
task docker:up:demo
```

Equivalente directo con Docker Compose:

```bash
docker compose -f deploy/docker/docker-compose.yaml up -d --build
# Con el perfil demo:
docker compose -f deploy/docker/docker-compose.yaml --profile demo up -d --build
```

### Confiar en la CA local de Caddy (una sola vez en tu máquina)

Cuando `CADDY_TLS=internal` (por defecto), Caddy firma los certificados de `hub.127.0.0.1.nip.io` y `auth.127.0.0.1.nip.io` con su CA local. Para que el navegador (necesario en el flujo OIDC de `/ui/`) y `curl` confíen en ella sin avisos:

```bash
# Exporta la CA raíz de Caddy a deploy/docker/caddy-root.crt
task docker:ca

# macOS (añadir al llavero del sistema):
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain deploy/docker/caddy-root.crt

# Linux (Debian/Ubuntu):
sudo cp deploy/docker/caddy-root.crt /usr/local/share/ca-certificates/alexandria-caddy.crt && sudo update-ca-certificates
```

---

## 2. Endpoints y credenciales preconfiguradas

| Servicio | URL | Credenciales / Notas |
|:---|:---|:---|
| **Hub API & Publicación** | `https://hub.127.0.0.1.nip.io:8443` (`/v1`, `/pub`) | Valida JWT `at+jwt` emitidos por Keycloak |
| **Consola Web del Hub** | `https://hub.127.0.0.1.nip.io:8443/ui/` | Login OIDC + PKCE con cliente `alexandria-console` |
| **Keycloak (OIDC Issuer)** | `https://auth.127.0.0.1.nip.io:8443/realms/alexandria` | Realm `alexandria` auto-importado |
| **Keycloak Admin Console** | `https://auth.127.0.0.1.nip.io:8443/admin/` | `admin` / `admin` (`KEYCLOAK_ADMIN` / `KEYCLOAK_ADMIN_PASSWORD`) |
| **Client Admin & Consola** *(perfil `demo`)* | `http://127.0.0.1:9090/ui/` | Estado, reglas dinámicas, `:explain` y `POST /sync` |
| **Client DSP Ingress** *(perfil `demo`)* | `http://127.0.0.1:9080` | Proxy L7 delante de `fake-connector` |

### Usuarios del realm `alexandria`

| Usuario | Contraseña | Rol en Hub | `sub` fijo en Keycloak |
|:---|:---|:---|:---|
| `ana` | `ana123` | Administradora (`vhub-admin`) | `11111111-1111-4111-8111-111111111111` |
| `sergio` | `sergio123` | Steward de namespaces | `22222222-2222-4222-8222-222222222222` |

*(También existe el cliente confidencial `alexandria-m2m` con `secret: alexandria-m2m-secret` y `client_credentials` para pipelines CI/CD).*

---

## 3. Prueba rápida desde CLI (`curl`)

```bash
# 1. Exportar la CA local de Caddy para usarla con curl
task docker:ca
export CURL_CA_BUNDLE="$PWD/deploy/docker/caddy-root.crt"

# 2. Obtener un access token RFC 9068 (at+jwt) real desde Keycloak como administradora (ana)
ANA_TOKEN=$(curl -fsSL -X POST \
  "https://auth.127.0.0.1.nip.io:8443/realms/alexandria/protocol/openid-connect/token" \
  -d "client_id=alexandria-cli" \
  -d "grant_type=password" \
  -d "username=ana" \
  -d "password=ana123" | jq -r .access_token)

# 3. Crear el namespace "mobility" en alexandria-server (persistido en PostgreSQL)
curl -i -X POST "https://hub.127.0.0.1.nip.io:8443/v1/namespaces" \
  -H "Authorization: Bearer $ANA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"slug":"mobility","displayName":"GreenCity Mobility"}'

# 4. Asignar a sergio como steward de "mobility"
curl -i -X POST "https://hub.127.0.0.1.nip.io:8443/v1/namespaces/mobility/stewards" \
  -H "Authorization: Bearer $ANA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "iss": "https://auth.127.0.0.1.nip.io:8443/realms/alexandria",
    "sub": "22222222-2222-4222-8222-222222222222",
    "label": "sergio"
  }'
```

---

## 4. Pasar a dominio real con Let's Encrypt

1. Apunta los registros DNS `A`/`AAAA` de tus subdominios (p. ej. `hub.ejemplo.es` y `auth.ejemplo.es`) al host Docker.
2. En `deploy/docker/.env`, configura:
   ```dotenv
   HUB_DOMAIN=hub.ejemplo.es
   AUTH_DOMAIN=auth.ejemplo.es
   HTTPS_PORT=443
   CADDY_TLS=ops@ejemplo.es
   POSTGRES_PASSWORD=<secreto-fuerte>
   KEYCLOAK_ADMIN_PASSWORD=<secreto-fuerte>
   ```
3. Añade `"https://hub.ejemplo.es/ui/*"` en `redirectUris` del cliente `alexandria-console` (en `deploy/docker/keycloak/alexandria-realm.json` antes del primer arranque, o desde la consola de administración de Keycloak) y cambia las contraseñas iniciales de prueba.
4. Ejecuta `docker compose -f deploy/docker/docker-compose.yaml up -d --build`. Caddy emitirá y renovará automáticamente los certificados TLS vía Let's Encrypt (TLS-ALPN-01 en `:443`).
