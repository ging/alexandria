# Cuestiones Abiertas y Cobertura del Catálogo (`OPEN_QUESTIONS.md`)

Este documento registra el estado en la PoC de las cuestiones abiertas de F7 (`Q1`–`Q17`), los huecos de diseño de F8 (`G1`–`G5`) y la justificación del único código de F5 no alcanzable en la PoC (DoD punto 3).

## 1. Códigos del catálogo F5 no alcanzables en la PoC (DoD punto 3)

De los **64 códigos** del catálogo cerrado F5 (`internal/core/codes/catalog.tsv`), **63 códigos** cuentan con su caso negativo automatizado `N-<CÓDIGO>` en `conformance/negative_hub_test.go` y `conformance/negative_client_test.go`.

El único código no alcanzable en la PoC es:

| Código | Emisor | Canal | Severidad | Requisito | Motivo por el que no es alcanzable en la PoC |
| --- | --- | --- | --- | --- | --- |
| `VERDICT_SHACL_WARNING` | C-CLI | clasificación | `info` | `REQ-CLI-011` | Por **DP-7**, el adaptador `evaluator_poc` evalúa únicamente artefactos `json-schema` (cuyos fallos tienen siempre severidad `Violation` según §4.7 paso 3) y omite la evaluación de grafos `shacl` emitiendo el código informativo `POC_EVAL_UNSUPPORTED_ROLE`. Como los hallazgos con severidad `Warning` solo provienen de shapes SHACL (`sh:Warning`), la rama `r.shacl_warnings → VERDICT_SHACL_WARNING` de `internal/client/domain/classify.go` está implementada en el dominio puro pero `evaluator_poc` nunca produce hallazgos `shacl` de severidad `Warning`. |

---

## 2. Estado de las cuestiones abiertas de F7 (`Q1` a `Q17`)

| ID | Cuestión en F7 | ¿Bloquea v1? | Tratamiento en esta PoC |
| --- | --- | --- | --- |
| **Q1** | Formato de firma `{kid, alg, sig}` sobre JCS frente a JWS (RFC 7515) o Data Integrity (VC) | No | Se implementa el sobre normativo `{signed, signature: {kid, alg, sig}}` con `EdDSA` (Ed25519) sobre `JCS(signed)` en `internal/core/sign`. |
| **Q2** | Formato y verificación del recibo de transparencia (estilo SCITT) | Sí | **DP-8**: recibo JSON simulado `{digest, registered_at, log_id, sig}` firmado con la clave del hub en `scitt_fake` y verificado en `receipt_scitt_fake` tras los puertos `TransparencyLog` y `ReceiptVerifier`. |
| **Q3** | Revisar la tabla de rutas por defecto contra DSP 2025-1 y extensiones EDC/FIWARE | Sí | **DP-10**: se cargan las 16 rutas del Anexo de rutas DSP en modo `observe` cuando `routes` está vacío (`internal/client/domain/routes.go`, `config/routes.default.json`). |
| **Q4** | Campos obligatorios de `ContractNegotiationError` y `TransferError` cuando la petición inicial no trae `providerPid` | Sí | **DP-11**: cuando el cuerpo de entrada no contiene `providerPid` o `consumerPid`, se rellena con el centinela `"urn:uuid:00000000-0000-0000-0000-000000000000"` (`internal/client/domain/verdict.go`). |
| **Q5** | Auditoría activa: identidad con la que el `vhub-client` pide el catálogo de su propio conector | No | Solo se implementa la auditoría pasiva de §4.10 (`internal/client/app/audit.go`, `REQ-CLI-023`). |
| **Q6** | Agreements anteriores al despliegue (`import` frente a versión ficticia `pre-hub`) | No | Se implementa `POST /agreements:import` exigiendo que la versión exista en los paquetes conocidos (`REQ-CLI-026`). |
| **Q7** | Compromiso de la clave del hub y ausencia de raíz sobre el DID document (`did:web`) | No | Se verifica en el momento de recepción (S2, S9, V1–V4) y nunca se reverifican documentos ya aceptados (`REQ-CLI-024`). |
| **Q8** | Incorporar a la spec del lenguaje los parámetros delta de §2.10, `LIFECYCLE_*` y D-PVP-1 (`url` en contextos) | Sí | **DP-6**: `compiler_poc` valida los parámetros delta, emite los 7 códigos `LIFECYCLE_*` y exige `url` en cada entrada `application/ld+json`. |
| **Q9** | Alinear F0 con F2–F5 (`new_content_from`, `enforced_from`, `revoked_from`, `Revoked`, fechas en registro) | Sí | **DP-13**: todo el código y los schemas usan exclusivamente la nomenclatura de F2–F5. |
| **Q10** | Presupuesto de latencia por mensaje y coste de evaluar varias versiones | No | `Classify` evalúa primero la cabeza y solo evalúa versiones anteriores de `V_t` si la cabeza falla (`internal/client/domain/classify.go`), exponiendo la métrica `vhub_client_request_duration_seconds`. |
| **Q11** | Recolección de basura de paquetes retenidos y caducidad de agreements | No | `PackageStore.Retain` conserva todos los paquetes de `versions[]` más los referenciados por agreements registrados (`REQ-CLI-017`). |
| **Q12** | Alinear Gobernanza rev. 2 §16–§19 y `vhub_spec.html` con R10 (una sola clave en el DID document, sin `roles.json`) | Sí | **DP-13**: no existen `roles.json` ni `current.json` en ningún punto del código. |
| **Q13** | `/pub` público frente a restringido a miembros | No | **DP-15**: `/pub` se sirve sin autenticación con cabeceras `immutable` y `ETag` SHA-256 (`REQ-HUB-011`). |
| **Q14** | Métodos de DID distintos de `did:web` (`did:key`, `did:ebsi`) | No | El schema A.3 y `internal/core/did` restringen el DID del hub a `did:web`; el puerto `DIDResolver` permite inyectar otros métodos desde `internal/ssi-auth`. |
| **Q15** | Quién sirve el DID document del hub y en qué ruta (`G1`) | No | **DP-4**: `vhub-server` sirve `GET /.well-known/did.json` generado desde `Signer.PublishedKeys()` (`internal/server/adapters/http_did`). |
| **Q16** | Configuración del `vhub-server` (`G2`) | No | **DP-12**: `config/server.json` validado con `config/server.schema.json` (`internal/server/config`). |
| **Q17** | Interfaz de `TransparencyLog` y `ReceiptVerifier` (`G5`) | Sí | **DP-8**: puertos definidos en `internal/server/app/ports.go` e `internal/client/app/ports.go`. |

---

## 3. Especificación del lenguaje de perfiles

La revisión del borrador rev. 2 está en [`PROFILES_DSL.md`](PROFILES_DSL.md) §4 (R1–R15). Los puntos que bloquean el ejemplo §30 son R1 (binding `internal` de `dcp` imposible; formato de vendorizado sin definir) y R2–R3 (Apéndice A de la rev. 1, `lifecycle` en dos sitios). R13–R15 son incoherencias con la spec de vhub 0.2.1.
