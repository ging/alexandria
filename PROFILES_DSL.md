# Lenguaje de perfiles `eunomia.profiles/v1`: implementación y revisión

Un espacio de datos es una **pila de especificaciones** (ODRL, DCAT, DSP, DCP). Cada una deja libertad, y cada libertad es un **slot** que el espacio acota con un **binding**: qué perfil manda, en qué versión, con qué obligación y en qué gate se comprueba. La autoridad escribe esa pila en YAML (`OdrlProfile` + `SpaceProfile`), y los artefactos de validación se **generan al vuelo**: nunca se editan a mano.

Este documento describe lo implementado y recoge la revisión de la especificación (borrador, revisión 2). `ConformanceSuite` y la Parte IV (conformidad) quedan fuera, a petición.

## 1. Cómo se usa

```bash
go build -o bin/alexandriac ./cmd/alexandriac
bin/alexandriac validate internal/dsl/testdata/colors.profile.yaml
bin/alexandriac compile -o out internal/dsl/testdata/colors.profile.yaml
bin/alexandriac compile -o out -vendor vendor/ -previous release-1.0.0.yaml bundle.yaml
```

- **`validate`** imprime los hallazgos (`severidad CÓDIGO documento#n:línea ruta mensaje`) y sale con 1 si hay errores.
- **`compile`** además escribe `profiles/odrl/<token>/<versión>/…`, los ficheros vendorizados y, si hay `SpaceProfile`, `space/prof.ttl`, `space/shacl-union.ttl`, `space/spec.html` y `space/space-lock.json`.
- **`-previous`** son las fuentes de la release anterior, para N4.
- **`-vendor`** es el directorio de copias locales: `<slot>/<versión>/vendor.json`, más los ficheros que lista.

En el hub, `server.json` admite `"compiler": "dsl"` (por defecto `"poc"`) y `"vendor_dir"`. Un borrador pasa a ser un **stream YAML**: un `SpaceProfile` y los `OdrlProfile` que enlaza, separados por `---`. Al proponer, el hub compila, y el paquete lleva los artefactos de cada slot con los gates de su binding, más `sources/draft.yaml`. La release siguiente se compara (N4) con esa fuente. El escenario F6 de extremo a extremo (`conformance/f6/drafts/*.yaml`) ya funciona así con los binarios reales.

## 2. Qué se genera

| Artefacto | Rol en el paquete | Origen |
| --- | --- | --- |
| `vocab.ttl` | `vocabulary` | Ontología, colecciones SKOS, términos en su forma §8.5, firmas reificadas §9.3, enforcement en `eu:` |
| `context.jsonld` | `mapping` (con `url`, D-PVP-1) | Un alias por término |
| `shapes.ttl` | `shacl` | §13.3: listas cerradas, avisos de `deprecated`, una shape por firma |
| `schema.json` | `json-schema` | Draft 2020-12 autocontenido sobre la `Offer` compacta: perfil declarado, acciones, restricciones lógicas y atómicas, una condición por firma |
| `cedar-mapping.json`, `cedar.schema` | `cedar-mapping`, `cedar-schema` | §13.4, solo si algún término tiene `enforcement` |
| `prof.ttl`, `spec.html` | `prof`, `specification` | Manifiesto PROF y especificación legible |
| `space/space-lock.json` | (fuera del manifiesto) | Por binding: versión resuelta, obligación, artefactos con SHA-256 y gates; además `lifecycle` y `profiles`, que lee alexandria-client |
| `space/shacl-union.ttl` | `shacl-union` | SHACL de los bindings `mandatory`, por gate |

La salida es determinista: las mismas fuentes producen los mismos bytes.

## 3. Validación

| Nivel | Implementado |
| --- | --- |
| N1 | YAML restringido al modelo JSON (sin anclas, alias ni etiquetas; claves duplicadas rechazadas) y el JSON Schema normativo (`internal/dsl/schema.json`), con línea de origen en cada hallazgo |
| N2 | IDs únicos, colisiones con adoptados y de alias, prefijos declarados, id del perfil ≠ namespace, tabla §9.4 operador–tipo, `enforcement` ⇒ firma, un binding por slot, `mandatory` ⇒ gate, `via` sin ciclos, invariantes `LIFECYCLE_*` |
| N3 | Adoptados contra una copia local de ODRL 2.2 o contra perfiles del bundle; resolución de rangos SemVer (`=`, `^`, `~`); vendorizados desde copia local; artefacto de cada `conformance` presente |
| N4 | Diff contra la release anterior con los bumps de §14 (perfil y espacio); `TERM_REMOVED`, `TERM_RETIRED_WITHOUT_DEPRECATION`, `NAMESPACE_CHANGED`, versión publicada inmutable |
| N5 | JSON válido, `schema.json` compila, `context.jsonld` se procesa con json-gold sin red y cada alias se expande al IRI de su término, Turtle bien formado (comprobación estructural, ver Límites en §4) |

## 4. Revisión de la especificación

Todos los hallazgos se han comprobado al implementar. Están ordenados por gravedad.

### Bloqueantes: el texto no compila tal cual

- **R1. El ejemplo `demo.space.yaml` (§30) no se puede compilar.**
  - `dcp` con `source: internal` es imposible, porque el lenguaje solo tiene `OdrlProfile`: no hay documento para un perfil de credenciales.
  - Los bindings `vendored` de DCAT-AP y DSP no resuelven sin copias locales, y la especificación no dice dónde ni cómo se vendorizan.
  - El compilador lo reporta como `BINDING_BASE_MISMATCH` y `BINDING_UNRESOLVED`.
  - *Propuesta:* restringir `internal` al slot `odrl`, o añadir un `kind` para DCP, y definir el formato de vendorizado. Aquí: `<slot>/<versión>/vendor.json`.
- **R2. El Apéndice A es de la revisión 1 y no acepta la revisión 2.**
  - No tiene `status` (§8.5 rev. 2), sino `deprecated: bool`.
  - No tiene `lifecycle` en `SpaceProfile`, y con `additionalProperties: false` el ejemplo §30 falla en N1.
  - El `dsl` del Problema 2 es `example.profiles/v1`, y el del esquema es `eunomia.profiles/v1`.
  - *Aplicado:* el esquema embebido añade `status`, `lifecycle` (incluidos los deltas de vhub de §2.10) y fija `eunomia.profiles/v1`.
- **R3. `lifecycle` está en dos sitios.** La tabla §11.1 lo pone dentro de `space`, y el bloque §11.6 y el ejemplo §30 en la raíz del documento. *Aplicado:* en la raíz, como en el YAML de ejemplo.
- **R4. El `schema.json` del §6.4 no sirve como gate.**
  - Hace `$ref` remoto al schema de DSP, lo que contradice "nada se resuelve contra la red".
  - Solo admite restricciones atómicas, así que rechaza el `and` de C1 y del suite.
  - No valida `dateTime`, por lo que no reproduce la fila 4 del §33.
  - *Aplicado:* schema autocontenido con restricciones lógicas y condiciones por firma. El test `TestGeneratedSchemaReproducesSection33` reproduce las filas del §6.6 y el §33.

### Ambigüedades que obligan a decidir

- **R5. Cuál es el "namespace de términos"** cuando `namespaces` declara varios prefijos. *Aplicado:* el primero declarado en el YAML.
- **R6. `scheme` no es implementable.**
  - Ningún campo de término declara la pertenencia a un `ConceptScheme`.
  - SHACL sobre la política no ve `skos:inScheme` si el gate no carga `vocab.ttl` en el grafo de datos.
  - *Aplicado:* se compila como pide §13.3 y se emite un aviso.
- **R7. `values` con literales.** §9.2 deduce `iri` "cuando son CURIEs", pero no dice nada de una enumeración de literales. *Aplicado:* `datatype` o `value_kind: literal` hacen la enumeración literal.
- **R8. La tabla de bumps de espacio (§14) no cubre:**
  - cambios de `lifecycle`, que aquí son MINOR;
  - añadir un gate a un `mandatory`, que aquí es MAJOR;
  - retirar una adopción, que aquí es MAJOR;
  - cambiar `enforcement`, que aquí es MAJOR al cambiar y MINOR al añadir.
- **R9. `extra_turtle` "DEBE parsear"** obliga a tener un parser RDF en el compilador y rompe el mundo cerrado de las salidas. *Aplicado:* se inyecta y se avisa. *Propuesta:* quitarlo de v1.
- **R10. Las URLs de los artefactos no llevan versión** (`…/colors/context.jsonld`, §6.1), pero su contenido cambia entre versiones. Eso choca con la inmutabilidad de §17 y REQ-SYS-005. *Aplicado:* el contexto conserva la URL sin versión, porque es la que citan los mensajes, y `/pub` sirve rutas versionadas.
- **R11. Contexto y términos protegidos de DSP.** Redefinir `odrl` o `use` en el contexto del perfil (ejemplo §17) puede fallar con el contexto protegido de DSP. *Aplicado:* no se redefinen `use` ni los operadores, y los adoptados de ODRL se escriben con su IRI completo.
- **R12. Los alias de rightOperand (`red`, …) no sirven de nada**, porque un valor `@id` no se resuelve como término (§6.5 lo explica). Se emiten porque §13.1 lo pide, pero conviene quitarlos.

### Incoherencias con vhub (spec 0.2.1)

- **R13. La Parte III de la rev. 2 está superada por vhub.** `current.json`, las claves por rol (§19) y el PVP con `change`/`previous` (§16) contradicen R3, R10 y R11 de vhub. Además, las fechas de §26 (`enforced_from = published_at + ventana`) contradicen vhub §3.3 (`new_content_from = P + pw`). *Propuesta:* que la especificación del lenguaje remita a vhub para distribución y ciclo de vida (equivale a Q9 y Q12).
- **R14. Gates distintos.** El lenguaje tiene `pdp_load` y `credential_presentation`. El esquema A.3 de alexandria-client solo admite `dsp_message`, `policy_intake`, `catalog_entry` y `agreement_admission`. Los bindings en esos dos gates compilan, pero ningún alexandria-client los aplica.
- **R15. `space-lock.json` tiene dos formatos.** El del lenguaje (§13.2) va por binding, y el de vhub (5.2) apunta al manifiesto. *Aplicado:* un solo documento que contiene ambos.

### Límites de esta implementación

- **JSON-LD:** se usa `json-gold`, la implementación de referencia en Go, que el módulo ya tenía a través del fork de trustbloc. Su `DocumentLoader` solo sirve los contextos que declara el manifiesto, así que la expansión nunca sale a la red. La usan el N5 del compilador y `evaluator_poc`, que ahora expande el mensaje completo y detecta también los contextos anidados.
- **Turtle y SHACL siguen con la comprobación estructural**, por decisión pendiente: en Go no hay librería estándar con licencia limpia.
  - `knakk/rdf`, la más usada, tiene por licencia una sola frase ("Do what thou wilt shall be the whole of the Law."), que no es una licencia OSI. Descartada.
  - Las referencias reales son pySHACL/RDFLib y Apache Jena. goRDFlib (BSD-3) es la opción en Go, pero es de un solo autor y reciente.
  - Hasta decidir, no se ejecuta la semántica SHACL (las filas SHACL del §33), ni en el compilador ni en alexandria-client (DP-7).
- **La copia de ODRL 2.2** (`internal/dsl/odrlvocab.go`) está escrita a mano. Hay que contrastarla con el TTL oficial.
- **El tipo `datetime` de Cedar** depende de la versión de Cedar.

## 5. Decisiones del agente

1. `gopkg.in/yaml.v3`, ya dependencia directa del módulo, entra en la lista de DP-1 para `internal/dsl`. El test de arquitectura impide que el paquete importe server, client, clock o testkit.
2. Códigos del lenguaje propios: `YAML`, `ALIAS_COLLISION`, `SIG_VALUE_KIND`, `ENFORCEMENT_WITHOUT_SIGNATURE`, `VIA_CYCLE`, `PROFILE_VERSION_CHANGED`, `NAMESPACE_CHANGED`, entre otros (`internal/dsl/findings.go`). En el adaptador: `BUNDLE_WITHOUT_SPACE` y `SPACE_MISMATCH`. Ninguno entra en el catálogo cerrado F5 de vhub.
3. La versión de vhub es la del `SpaceProfile`. Un perfil ODRL intermedio (p. ej. 1.1.0, que depreca un término) viaja en el bundle de la release del espacio aunque no esté enlazado, para que N4 pueda exigir *deprecated → retired*.
4. Los mensajes F6 declaran `profile` en la `Offer` y en el `Agreement` (regla de gobernanza del §5), y la ruta valida `$.offer`. En F6, el binding pone `json-schema` en `policy_intake`, porque alexandria-client solo ejecuta JSON Schema.
