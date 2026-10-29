# Engram: revisión técnica para un harness de trabajo con documentos

*7 de octubre de 2026 · Fase 1 · Fuentes: repos clonados (solo lectura, sin ejecutar).*
*Abreviaturas de rutas:* `E/` = `scratchpad/repos/engram` (commit `8e525e7`, clon superficial, tag más reciente `v3.2.1`); `G/` = `scratchpad/repos/gentle-ai` (commit `eb81278`).
*Contexto:* huecos H1–H8 de `/home/claude/acervo-mecanismos-cuello-de-botella.md` (§ huecos, líneas 416–425).

---

## 0. Veredicto corto

- Engram es **memoria del agente, no expediente del humano**. Su filosofía declarada es que el humano no lo mire: "If you're thinking about engram while working, something went wrong" (`E/docs/intended-usage.md:33`) y "Manually inspect or edit engram's storage" aparece en la columna *Don't* (`:41`).
- Sí aporta piezas útiles: **tipos libres**, **topic_key con upsert e historial de versiones**, **relaciones juzgadas** (supersedes / conflicts_with) con razón y evidencia, **estado `needs_review` por caducidad**, FTS5, API HTTP y export JSON.
- No sirve como **registro auditable de criterios y decisiones para el humano** sin una capa encima: no tiene esquema por tipo, ni autoría humana de las decisiones, ni vista legible editable, ni historial exportable, y el vault de Obsidian es de solo salida.
- **Recomendación:** expediente Markdown en git como fuente de verdad del humano (criterios, decisiones, descartes, fuentes, comprensión); Engram como índice y memoria operativa del agente que **apunta** al expediente, con topic_keys alineados a sus IDs.

---

## 1. Modelo de datos

### 1.1 Almacenamiento y ubicación
| Aspecto | Evidencia |
|---|---|
| Un binario Go + SQLite + FTS5 | `E/README.md:90-104` |
| BD por defecto `~/.engram/engram.db` | `E/internal/store/store.go:824`; override `ENGRAM_DATA_DIR` (`E/cmd/engram/main.go:763`), debe ser ruta absoluta (`store.go:1026`) |
| SQLite: WAL, busy timeout 5 s, FK activas | `E/DOCS.md:141-146` |
| Rechaza NFS/SMB como directorio de datos | `E/DOCS.md:148-164` |
| FTS5 con tokenizador trigram sobre title, content, tool_name, type, project, topic_key | `E/DOCS.md:130`; `store.go:1341` (`tokenize='trigram'`) |
| Límite de contenido 50 000 bytes por observación; ventana de dedupe 15 min | `store.go:825,828` |
| Columnas `embedding*` existen pero están **reservadas, sin uso** (no hay búsqueda semántica vectorial) | `store.go:1601-1612`; sin otros usos en el código |

### 1.2 Tablas relevantes (`E/DOCS.md:128-139`, fuente de autoridad: `Store.migrate`)
- **sessions**: `id`, `project`, `ownership_mode`, `directory`, `started_at`, `ended_at`, `summary`.
- **observations**: `id`, `sync_id`, `session_id`, `type`, `title`, `content`, `tool_name`, `project`, `scope`, `topic_key`, `normalized_hash`, `revision_count`, `duplicate_count`, `last_seen_at`, `pinned`, `review_after`, `expires_at`, `embedding*`, `created_at`, `updated_at`, `deleted_at`.
- **observation_versions**: `observation_id`, `title`, `content`, `version`, `created_at` (`store.go:1346-1354`). Guarda la versión **anterior** antes de cada upsert por topic_key o `mem_update` que cambie contenido (`store.go:11777-11785`).
- **memory_relations**: `source_id`, `target_id`, `relation`, `reason`, `evidence`, `confidence`, `judgment_status`, `marked_by_actor/kind/model`, `session_id`, `superseded_at`, `superseded_by_relation_id` (`store.go:1625-1644`).
- **user_prompts** (+FTS): lo que pidió el usuario (`DOCS.md:131`).
- Tablas de sincronización (`sync_*`, tombstones, deferred).

**No hay** columna de metadatos libres (JSON), ni etiquetas, ni autor humano, ni enlace a archivo/fragmento de documento, ni enlace observación→prompt ("there is no explicit observation-ID link", `DOCS.md:1639`).

### 1.3 Tipos de observación
- El campo `type` es **texto libre**: `mem_save` lo documenta como "decision, architecture, bugfix, pattern, config, discovery, learning (default: manual)" (`E/internal/mcp/mcp.go:527-529`), pero el handler solo pone `manual` si viene vacío (`mcp.go:1535-1537`) y `AddObservation` no valida contra lista (`store.go:3899+`).
- Tipos con semántica real en el código:
  - `decision` (revisión a 6 meses), `policy` (12), `preference` (3) → `review_after` (`store.go:386-411`, `4044-4053`).
  - `session_summary` (lo crea `mem_session_summary`, `mcp.go:2479`).
  - `inferTopicFamily` mapea sinónimos a familias para topic_key (`store.go:12775+`).
- El protocolo de gentle-ai usa `bugfix | decision | architecture | discovery | pattern | config | preference` (`G/internal/assets/engram/protocol.md:55`).

### 1.4 Proyectos y ámbitos
- **Proyecto**: se resuelve en cada llamada: `.engram/config.json` → git remote → git root → hijo git único → ambiguo → basename del cwd (`E/DOCS.md:1077-1090`). Nombres normalizados (NFC, minúsculas, etc.) (`DOCS.md:1516-1518`). `engram init <nombre>` fija la identidad en carpetas sin git (`DOCS.md:1094-1098`) — relevante para carpetas de documentos.
- **Scope**: `project` (defecto) | `personal` | `global`; cualquier otro valor se convierte en `project` (`store.go:12717-12725`). **No es frontera de privacidad**: la sync exporta ambos (`E/docs/TEAM-USAGE.md`, sección "What scope does NOT do today").

### 1.5 Sesiones
- Registradas por el runtime/hook (en Claude Code, `session-start.sh` hace `POST /sessions`; `E/plugin/claude-code/scripts/session-start.sh`). Una sesión cerrada no se reabre; existe continuación `<id>:resume:N` vía HTTP (`E/docs/ARCHITECTURE.md:60`).
- El resumen de sesión se guarda como observación tipo `session_summary` con estructura fija Goal / Instructions / Discoveries / Accomplished / Next Steps / Relevant Files (`mcp.go:840-870`).

### 1.6 Topic keys, versionado y supersesión
- `topic_key` convierte `mem_save` en **upsert** sobre `project + scope + topic_key`, incrementa `revision_count` (`E/docs/ARCHITECTURE.md:101-105`, `186-195`). Formato recomendado `familia/descripcion`, máx. 2 niveles; no hay jerarquía real en el store (`:172-180`).
- `mem_suggest_topic_key`: heurística por tipo + título, sin LLM (`store.go:12746-12772`).
- Historial: `mem_get_observation(include_history=true)` devuelve versiones (página de 50) (`DOCS.md:1296-1309`).
- Dos mecanismos distintos de "reemplazo":
  1. **Upsert por topic_key**: misma fila, contenido sobrescrito, versión previa a `observation_versions`.
  2. **Relación `supersedes`** entre dos observaciones distintas (vía `mem_judge`/`mem_compare`). En búsqueda se **anota** (`supersedes:`, `superseded_by:`, `conflicts:`) (`mcp.go:1367-1413`); **no encontré** código que oculte o degrade lo reemplazado en búsqueda o contexto.
- Borrado: soft-delete por defecto (`deleted_at`), hard opcional (`ARCHITECTURE.md:94`, `184`).

### 1.7 Estados
- `state` es **virtual**: `active` o `needs_review` según `review_after` vencido (`store.go:182-200`). Solo existen esos dos.
- `judgment_status` de relaciones: `pending | judged | orphaned | ignored` (`DOCS.md:138`).
- `pinned`: local, no se sincroniza (`DOCS.md:1235-1253`). `mark_reviewed` también es solo local (`DOCS.md:1233`).

---

## 2. Herramientas MCP (23; `E/DOCS.md:1176-1399`, registro en `E/internal/mcp/mcp.go:445-1160`)

gentle-ai las monta con `engram mcp --tools=agent` (`G/internal/components/engram/inject.go:91`), es decir, el perfil `ProfileAgent` (`mcp.go:226-246`): todas menos `mem_delete`, `mem_stats`, `mem_timeline`, `mem_merge_projects` (`ProfileAdmin`, `mcp.go:250-255`).

| Herramienta | Parámetros clave | Semántica |
|---|---|---|
| `mem_save` | `title`*, `content` (What/Why/Where/Learned), `type`, `scope`, `topic_key`, `session_id`, `project`, `capture_prompt` | Guarda; dedupe 15 min; upsert si hay topic_key; tras guardar busca **candidatos de conflicto** y devuelve `judgment_required`, `candidates[]`, `judgment_id` (`mcp.go:485-557`, `1625-1681`) |
| `mem_search` | `query`*, `type`, `project`, `all_projects`, `scope`, `match_mode` (all/any), `response_format` (compact), `limit` ≤20 | FTS5 con pesos título 5, contenido 1, topic_key 3 + leves impulsos por pin/recencia (`store.go:393-401`); anota relaciones y `needs_review` |
| `mem_context` | `project`, `scope`, `max_bytes` (16 KiB def., 64 KiB máx.), `compact` | Sesiones, prompts y observaciones recientes + hasta 20 fijadas (`DOCS.md:1268-1276`) |
| `mem_get_observation` | `id`*, `project`, `include_history`, `history_cursor` | Contenido completo + historial de versiones |
| `mem_timeline` (admin) | `observation_id`, antes/después | Vecindad cronológica en la sesión |
| `mem_update` | `id`*, `expected_project`*, `title`, `content` o `find`/`replace`, `type`, `scope`, `topic_key` | Edición parcial; guarda versión previa |
| `mem_delete` (admin) | `id`*, `expected_project`*, `hard_delete` | Soft/hard delete |
| `mem_suggest_topic_key` | `type`, `title`, `content` | Heurística de familia |
| `mem_review` | `action` = list / mark_reviewed, `project`, `limit`, `observation_id` | Lista vencidas; reinicia ciclo |
| `mem_pin` / `mem_unpin` | `id` | Prioridad en contexto (local) |
| `mem_session_summary` | `content` (estructura fija), `session_id`, `project` | Resumen de cierre |
| `mem_session_start` / `mem_session_end` | id, `directory` / resumen | Ciclo de sesión (gentle-ai prohíbe al agente llamar `start`, `protocol.md:15`) |
| `mem_save_prompt` | `content` | Guarda el prompt del usuario |
| `mem_capture_passive` | `content` | Extrae ítems de una sección `## Key Learnings:` como observaciones |
| `mem_judge` | `judgment_id`*, `relation`*, `reason`, `evidence`, `confidence`, `session_id` | Resuelve candidato pendiente (ver §4) |
| `mem_compare` | `memory_id_a/b`*, `relation`*, `confidence`*, `reasoning`* (≤200 car.), `model` | Crea relación directamente entre dos memorias |
| `mem_current_project`, `mem_list_projects`, `mem_stats`, `mem_doctor`, `mem_merge_projects` | — | Resolución, inventario, diagnóstico, fusión |

---

## 3. Protocolo que inyecta gentle-ai

**Origen y forma de instalación:** un único archivo `G/internal/assets/engram/protocol.md` con secciones `full`, `slim`, `passive-capture`, `compact` (`G/internal/components/engram/protocol.go:13-74`). Claude Code recibe `slim` en CLAUDE.md si el binario es ≥1.4.0, porque el texto completo también llega por las *instructions* del servidor MCP y el hook SessionStart (`protocol.go:76-89`; `G/internal/cli/run.go:3170-3195`). Codex recibe `full` + `passive-capture` y un prompt de compactación propio (`protocol.go:63-74`). Además `engram setup <agente> --protocol=full|slim` instala hooks (Claude Code: SessionStart, UserPromptSubmit, SubagentStop, post-compaction, session-end; `E/plugin/claude-code/hooks/hooks.json`).

| Momento | Regla (`G/internal/assets/engram/protocol.md`) |
|---|---|
| Inicio | `mem_current_project` antes de leer; si ambiguo, preguntar al humano; nunca inventar proyecto ni session_id (`:7-21`) |
| Cuándo guardar | Inmediatamente, sin pedirlo: decisiones de arquitectura, convenciones, cambios de flujo, elección con tradeoffs, bugfix, hallazgos no obvios, preferencias del usuario, etc. (`:23-39`) |
| Formato | title "Verbo + qué", type, scope, topic_key, content What/Why/Where/Learned (`:52-63`) |
| Evolución | mismo tema → mismo topic_key; no sobrescribir temas distintos (`:73-77`) |
| Caducidad | `mem_review list` al inicio; `needs_review` = "stale context, not trusted facts"; `mark_reviewed` solo con confirmación del usuario (`:79-85`) |
| Búsqueda | `mem_context` → `mem_search` → `mem_get_observation`; proactiva si el primer mensaje alude a trabajo previo (`:94-106`) |
| Entrega | Guardar no es responder; la respuesta final va al final, sin herramientas después (`:41-50`) |
| Cierre | `mem_session_summary` obligatorio antes de decir "done" (`:108-130`) |
| Compactación | El prompt de compactación antepone "FIRST ACTION REQUIRED…"; tras compactar: `mem_session_summary` con el resumen, luego `mem_context` (`:132-139`, `:198-215`) |
| Captura pasiva | Sección `## Key Learnings:` al final de cada tarea (`:184-196`) |

Observaciones para nuestro caso: todos los disparadores están escritos para **código** (bugfix, config, archivos). Ninguno menciona criterio emergente, descarte, fuente o comprensión del humano. El protocolo de `mem_judge` (cuándo preguntar al usuario) vive en la descripción de la herramienta, no en protocol.md.

---

## 4. `mem_judge` y detección de conflictos

**Detección (sin LLM, heurística léxica):**
1. Tras cada `mem_save` exitoso, `FindCandidates` lanza una consulta FTS5 con el texto de la observación guardada contra el mismo `project` y `scope`, excluye pares ya juzgados y borrados, filtra por umbral BM25 y devuelve **máx. 3** (`E/internal/store/relations.go:74`, `358-512`, `1081-1120`).
2. Inserta filas `pending` en `memory_relations` y responde `judgment_required: true` con los candidatos (`mcp.go:1641-1681`). El fallo de detección nunca hace fallar el guardado (`mcp.go:1627-1628`).
3. Por ser léxico, dos criterios contradictorios con vocabulario distinto **no** se detectan.

**Juicio (lo hace el agente, no Engram):** el agente que llamó lee los candidatos y llama `mem_judge` con uno de 6 veredictos: `related`, `compatible`, `scoped`, `conflicts_with`, `supersedes`, `not_conflict` (`mcp.go:1057-1108`). Reglas embebidas en la descripción: preguntar al usuario si `confidence < 0.7` o si es `supersedes`/`conflicts_with` sobre tipos `architecture|policy|decision`; en otro caso "resolve silently". Re-juzgar sobrescribe. Procedencia registrada como `marked_by_actor="agent"`, `kind="agent"` (`mcp.go:2674-2686`): **no distingue si el humano confirmó**.

**LLM opcional (solo CLI/HTTP admin):** `engram conflicts scan --semantic` toma los pares que ya encontró FTS5 y los pasa a un CLI externo (`claude` u `opencode`, variable `ENGRAM_AGENT_CLI`) con un prompt fijo que devuelve `{Relation, Confidence, Reasoning}` (`E/internal/llm/prompt.go`, `factory.go:28-40`; `E/cmd/engram/conflicts.go:436-470`; `E/DOCS.md:558-590`). Pide confirmación de coste; tope 100 llamadas. `mem_compare` permite al agente persistir un veredicto propio sobre cualquier par.

**Efecto:** solo anotaciones en `mem_search`. La cadena de supersesión (`superseded_at`, `superseded_by_relation_id`) no viaja en la sync ("Phase 3", `store.go:644-649`).

---

## 5. Exportación y visualización

| Superficie | Qué hace | Límites |
|---|---|---|
| **TUI** (`engram tui`) | Dashboard, búsqueda, recientes, detalle, timeline, sesiones; copiar con `c` (`E/DOCS.md:1643-1676`) | Solo lectura navegable; no muestra relaciones ni historial como vista propia (no hallado) |
| **Obsidian (beta)** `engram obsidian-export --vault` | Una nota `.md` por observación con frontmatter (id, type, project, scope, topic_key, session, fechas, revision_count, tags, alias) + wikilinks a hubs de sesión y de tema (`E/internal/obsidian/markdown.go:13-91`); modo watcher | **Unidireccional**: "Engram writes to the vault, never reads from it"; reexportar sobrescribe ediciones (`E/docs/beta/obsidian-brain.md:126`). No exporta relaciones como enlaces ni historial de versiones |
| Plugin Obsidian "Engram Brain" | Lee `GET /export` del servidor local (`E/plugin/obsidian/src/sync.ts:59,103-111`) | Igual, de solo lectura |
| **Web** | Solo en Engram Cloud: `/dashboard/*` para "humans/operators" (`E/docs/engram-cloud/README.md:58,78`) | Requiere servidor cloud (Docker + Postgres); no hay UI web local |
| **HTTP API** (`engram serve`, puerto 7437) | CRUD de observaciones, búsqueda, contexto, review, export/import, conflictos (`E/DOCS.md:215-700`) | Algunos endpoints admin requieren token |
| **Export/Import JSON** | Backup versionado 0.2.0: sesiones, observaciones, prompts, pins, relaciones (`DOCS.md:1588-1594`; struct `store.go:712-720`) | `ExportData` **no incluye `observation_versions`**: el historial de un topic_key se pierde al exportar |
| **Git sync** `engram sync` | Chunks JSONL gzip inmutables + manifest en `.engram/` del repo (`DOCS.md:1596-1624`) | Binario comprimido: **no legible ni revisable en un diff** |
| **Cloud** | Replicación por proyecto, autosync, dashboard | Infraestructura extra; scope no filtra |

---

## 6. Extensibilidad

| Pregunta | Respuesta | Evidencia |
|---|---|---|
| ¿Tipos propios (criterio, decision, descarte, fuente, comprension)? | **Sí, de facto**: `type` es texto libre sin validación. Pero no tienen comportamiento propio: ni esquema, ni caducidad (solo decision/policy/preference), ni familia de topic_key (caen en heurística por palabras, `store.go:12775+`) | `mcp.go:1535`; `store.go:407-411` |
| ¿Metadatos libres? | **No.** Solo columnas fijas. Lo estructurado hay que meterlo en `content` (Markdown) o codificarlo en `topic_key`/`title` | `DOCS.md:129` |
| ¿Consulta por tipo / tema? | Por tipo: sí (`type` en `mem_search`, `GET /search`). Por tema: el topic_key está indexado en FTS (búsqueda por subcadena); no hay filtro exacto por prefijo de topic_key en `mem_search` (no hallado) | `mcp.go:456`; `ARCHITECTURE.md:122` |
| ¿Enlaces explícitos entre memorias? | Solo los 6 verbos de relación; vocabulario cerrado (`validRelationVerbs`) | `relations.go:60-63` |
| ¿API además de MCP? | Sí: CLI completa y HTTP local; SQLite legible directamente con cualquier cliente | `DOCS.md:168-215` |
| ¿Cambiar el protocolo? | gentle-ai lo reescribe en cada instalación (sección marcada `engram-protocol` en CLAUDE.md, `inject.go:937`); Engram desaconseja protocolos propios (`intended-usage.md:43`). Se puede añadir un protocolo propio fuera de esa sección | — |

---

## 7. Evaluación para nuestro caso

### 7.1 ¿Sirve como registro de criterios y decisiones auditable por el humano?

| Necesidad (hueco) | Qué da Engram | Qué falta |
|---|---|---|
| **H1/H4 criterio vivo** que nace, se confirma, se versiona y alimenta la spec | Tipo libre `criterio` + topic_key `criterio/...` con upsert e historial; `needs_review` si se le da tipo `policy` | Estado de confirmación humana (propuesto → confirmado → retirado); vínculo con la spec; historial no exportable; nada empuja el criterio a un documento |
| **H3 criterio tácito auditable** | `user_prompts` guarda lo que pidió el humano; relaciones con `reason` y `evidence` | No liga observación ↔ prompt; `marked_by` siempre "agent"; un humano no puede ver "qué criterio se aplicó en qué párrafo" |
| **H2 comprensión** | Nada específico | No hay objeto "lo que el humano sabe/ha leído"; todo está pensado para el agente |
| **H5 reentrada centrada en el humano** | `mem_session_summary` (Goal/Accomplished/Next Steps), `mem_context`, TUI | El resumen lo escribe el agente para el agente; no hay vista de "decisiones desde tu última visita" ni de "lo que olvidaste" |
| **H6 contradicciones** | Detección léxica de conflictos + `mem_judge`; scan semántico opcional por CLI | Léxico: falla con paráfrasis; el agente resuelve "silently" casi siempre; supersedes no oculta lo viejo |
| Auditoría | Soft-delete, versiones, procedencia de relaciones | Base binaria; Obsidian de solo salida; git sync ilegible; edición humana desaconsejada |

**Conclusión:** útil como memoria operativa y como índice consultable por el agente entre sesiones; **insuficiente como expediente del humano**. Su diseño asume explícitamente que el humano no lo lee ni lo edita.

### 7.2 Complementar con expediente Markdown en git: división de responsabilidades

| Responsabilidad | Expediente (Markdown + git) | Engram |
|---|---|---|
| Fuente de verdad de criterios, decisiones, descartes, fuentes | **Sí**: un archivo por objeto con frontmatter (`id`, `estado`, `confirmado_por`, `fecha`, `reemplaza`, `fuentes`) | Solo un puntero/resumen con `topic_key` = id del archivo (p. ej. `criterio/c-012-umbral-cobertura`) |
| Historial y diff legible | `git log`/`git diff` por criterio | Historial interno como respaldo |
| Confirmación humana | Commit o cambio de `estado` hecho por el humano | No |
| Recordar contexto entre sesiones del agente | No (o vía índice) | **Sí**: `mem_context`, `mem_search`, resúmenes, compactación |
| Hallazgos menores, gotchas, preferencias de trabajo | Opcional | **Sí** |
| Detección de choques entre criterios | Revisión humana + verificador propio | Señal temprana (candidatos FTS) que el agente debe **escalar al humano** en vez de resolver en silencio |
| Reentrada del humano (H5) | Vista generada desde git: "decisiones y criterios cambiados desde la fecha X", con los "por qué" | Insumo: resúmenes de sesión y prompts |
| Comprensión (H2) | Registro propio (qué decisiones revisó/explicó el humano) | No aplica |

Reglas prácticas para el harness:
1. Protocolo propio adicional (fuera de la sección `engram-protocol` que gentle-ai reescribe): "toda decisión o criterio se escribe primero en el expediente; luego `mem_save` con `topic_key` = id y `content` que cite la ruta".
2. Tipos propios fijos: `criterio`, `decision`, `descarte`, `fuente`, `comprension`; usar `decision` (y quizá `policy` para criterios) para heredar la caducidad `needs_review`.
3. Ante `judgment_required` con `supersedes`/`conflicts_with` sobre criterio o decisión: **siempre** preguntar al humano y reflejar el resultado en el expediente.
4. Respaldo periódico con `engram export` (sabiendo que no incluye el historial de versiones).

### 7.3 Riesgos

| Riesgo | Detalle | Mitigación |
|---|---|---|
| **Cambios rápidos** | 20 commits del mismo día en el clon superficial; CHANGELOG lleno de cambios de compatibilidad (p. ej. lecturas que dejan de ser globales, `E/CHANGELOG.md` "Unreleased"); `mem_update`/`mem_delete` rompieron clientes al exigir `expected_project` (`DOCS.md:1222`); muchas funciones etiquetadas "Phase 1/2/3" | Fijar versión; no depender de funciones beta (Obsidian, cloud); expediente independiente |
| **Lock-in** | Bajo en datos (SQLite + JSON, licencia MIT, `E/LICENSE`); medio en flujo (protocolo y hooks atados a gentle-ai) | La verdad vive en Markdown; Engram es reemplazable |
| **Privacidad** | Guarda prompts del usuario por defecto (`capture_prompt=true`); `scope: personal` no es privado y se sincroniza; solo `<private>…</private>` se redacta (`DOCS.md:1575-1582`) | No activar sync/cloud para documentos sensibles; usar `<private>`; `capture_prompt:false` donde aplique |
| **Mantenimiento** | Superficie grande (store.go ~13 700 líneas, 23 herramientas, cloud, sync, diagnósticos); dependencia de un autor/organización | Usar subconjunto mínimo (save/search/context/summary) |
| **Sesgo al código** | Detección de proyecto por git, formato What/Why/Where, disparadores de bugfix | `engram init` en la carpeta del expediente; protocolo propio para documentos |
| **Juicio silencioso** | El agente resuelve conflictos sin el humano salvo umbrales; procedencia siempre "agent" | Regla 3 de §7.2 |
| **Pérdida de historial** | Upsert sobrescribe; versiones no exportadas; supersedes no viaja en sync | Git como historial canónico |

### 7.4 Lo que no se encontró (no inventar)
- Filtro exacto por prefijo de `topic_key` en `mem_search`.
- Ocultación o degradación de memorias `superseded_by` en búsqueda/contexto.
- UI web local; vista de relaciones o historial en la TUI.
- Importación desde Obsidian o desde Markdown.
- Campo de autor humano o de confirmación.
