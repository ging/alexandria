# Tutorial Interactivo en Jupyter Notebooks — Alexandria (`alexandria`, `alexandria-server`, `alexandria-client`, `alexandriac`)

Esta carpeta contiene **6 Jupyter Notebooks incrementales** (`.ipynb`) que recorren de extremo a extremo todos los flujos de **Alexandria** (`alexandria`, `alexandria-server`, `alexandria-client` y `alexandriac`) realizando **peticiones HTTP reales** contra los binarios Go compilados localmente.

Todos los notebooks ya incluyen las **salidas de ejecución pre-capturadas** (`stdout`, widgets HTML interactivos, códigos HTTP, cabeceras `VHub-Verdict`, manifiestos firmados JCS + Ed25519 y eventos JSONL), por lo que puedes leerlos directamente o volver a ejecutar celda por celda.

---

## Contenido de la Serie Incremental

| # | Notebook | Flujos y Peticiones HTTP Cubiertos |
| :- | :--- | :--- |
| **01** | [`01_dsl_y_compilacion_alexandriac.ipynb`](./01_dsl_y_compilacion_alexandriac.ipynb) | Lenguaje declarativo **`eunomia.profiles/v1`** (`OdrlProfile` y `SpaceProfile`), validación en 5 niveles (`N1`–`N5`), compilación determinista de los 8 artefactos con `bin/alexandriac` y evolución de términos (`active -> deprecated -> retired`). |
| **02** | [`02_alexandria_server_gobernanza_y_pub.ipynb`](./02_alexandria_server_gobernanza_y_pub.ipynb) | API de Gobernanza (`/v1/*`) y Distribución Inmutable (`/pub/*`) en **`alexandria-server`**: identidad `GET /.well-known/did.json`, autenticación Bearer y matriz de roles (`401`/`403`), borradores (`POST/PUT /v1/drafts`) con `ETag`/`If-Match` (`428`/`412`), propuesta de candidatos (`:propose`), aprobación firmada (`:approve`), listado `GET /v1/versions`, redirección `303 See Other` en `/pub/demo` y auditoría en `GET /v1/events`. |
| **03** | [`03_alexandria_client_sincronizacion_y_proxy_dsp.ipynb`](./03_alexandria_client_sincronizacion_y_proxy_dsp.ipynb) | Sidecar **`alexandria-client`**: protocolo de sincronización verificada **`S1`–`S13`**, API de administración (`GET /status`, `GET /versions`, `GET /metrics`), validación en caliente L7 de mensajes DSP (`ContractRequestMessage`, `ContractOfferMessage`), comparación entre modos `enforce`, `shadow` y `observe`, bloqueo de `@context` externos (`CONTEXT_NOT_IN_PACKAGE`) y emisión de eventos `vhub.verdict`. |
| **04** | [`04_ciclo_de_vida_f6_transiciones_acuerdos_y_revocacion.ipynb`](./04_ciclo_de_vida_f6_transiciones_acuerdos_y_revocacion.ipynb) | **Escenario Normativo F6 completo** con viaje en el tiempo (`clock_file`, `DP-9`) de septiembre a noviembre de 2026: transición `1.0.0 -> 2.0.0`, ventana de propagación (`Announced`), ventana de gracia (`Transition`) con cabecera RFC 9651 `VHub-Verdict: warning;...;sunset=@1793613600`, captura del acuerdo `urn:uuid:A1` en `dsp-egress`, margen de contrato (`lease_margin`), continuidad de acuerdos tras retirada (`Retired`) y bloqueo tras revocación (`AGREEMENT_REVOCATION_SCHEDULED` -> `AGREEMENT_VERSION_REVOKED`). |
| **05** | [`05_resiliencia_seguridad_y_casos_negativos.ipynb`](./05_resiliencia_seguridad_y_casos_negativos.ipynb) | Catálogo cerrado de errores **F5 (`N-<CÓDIGO>`)**: invariantes de la tabla de 9 filas de aprobación (`DATES_GRACE_ON_COMPATIBLE`, `DATES_EMERGENCY_ON_COMPATIBLE`, `DATES_ZERO_GRACE_WITHOUT_EMERGENCY`, `DATES_JUSTIFICATION_REQUIRED`, `DATES_EMERGENCY_WITH_GRACE`), invariantes de revocación (`REV-1` a `REV-4`) y máquina de estados del cliente (`Active -> Degraded -> FailClosed` con `CLIENT_FAIL_CLOSED`) ante caída prolongada del Hub. |
| **06** | [`06_alexandria_wallets_fafnir_e_identityhub.ipynb`](./06_alexandria_wallets_fafnir_e_identityhub.ipynb) | Gateway de Identidad Soberana **`cmd/alexandria`** (`/api/v1/wallet/*`): enlace de wallet (`/is-linked`, `/link`, `/info`), ciclo de vida de claves asimétricas (`/keys`, `:rotate`), gestión de DIDs y Service Endpoints (`/did/:id/endpoints`, `:publish`), manejo de errores estructurados con `originalResponse` de IdentityHub (`409 Conflict`), participantes (`/participants`), credenciales verificables (`/credential`, `/vcs?type=...`) y protocolo DCP (`/dcp/request`). |

---

## Arquitectura del Laboratorio Local ([`lab.py`](./lab.py))

El módulo auxiliar [`lab.py`](./lab.py) está escrito **100 % en librería estándar de Python 3** (`urllib.request`, `subprocess`, `socket`, `json`, `tempfile`), por lo que **no requiere instalar paquetes externos** (`requests`, etc.) para ejecutar las celdas de código:

- **`ensure_binaries()`**: Compila automáticamente `bin/alexandriac`, `bin/alexandria-server`, `bin/alexandria-client` y `bin/fake-connector` si aún no existen.
- **`AlexandriaLab`**: Orquesta procesos reales de `alexandria-server`, `alexandria-client` y `fake-connector` en puertos locales libres, compartiendo un archivo de reloj (`clock_file`, `DP-9`) que permite avanzar el tiempo de forma determinista con `lab.set_clock("2026-10-10T12:00:00Z")`.
- **`http_request(method, url, ...)`**: Ejecuta peticiones HTTP reales (incluyendo soporte de forward proxy para `dsp-egress`) y renderiza widgets HTML interactivos con el método, cabeceras relevantes (`ETag`, `Cache-Control`, `Location`, `VHub-Verdict`, `WWW-Authenticate`) y el cuerpo JSON formateado.

---

## Cómo Ejecutar los Notebooks

### 1. Levantar JupyterLab con un solo comando (ejecutable [`./tutorial/run`](./run))
Compila automáticamente los binarios Go (`bin/alexandriac`, `bin/alexandria-server`, `bin/alexandria-client`, `bin/fake-connector`), crea el entorno virtual `tutorial/.venv` (si no existe), instala `jupyterlab` y abre el navegador directamente en `tutorial/`:

```bash
./tutorial/run
# o desde la raíz con Make o Task:
make tutorial
task tutorial
```

### 2. Ejecutar y verificar todos los notebooks en consola (modo headless)
Ejecuta las 35 celdas de código de los 6 notebooks contra los binarios Go reales y actualiza las salidas dentro de cada `.ipynb`:

```bash
make tutorial-exec
# o con Task:
task tutorial:exec
# o filtrando un notebook específico (ej. el 04):
./tutorial/run.py exec -f 04 -v
```

### 3. Directamente en VS Code / Cursor / GoLand / JetBrains
1. Abre cualquiera de los archivos `.ipynb` de esta carpeta `tutorial/`.
2. Selecciona cualquier intérprete **Python 3.10+** del sistema (o `tutorial/.venv/bin/python`).
3. Pulsa **Run All**.
