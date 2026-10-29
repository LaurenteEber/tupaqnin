# gentle-ai v4.0.0 (HEAD eb81278): revisión técnica profunda para un harness de documentos

*7 de octubre de 2026 · Revisión estática de solo lectura (no se ejecutaron scripts, builds ni instaladores) · Complementa la revisión general y `acervo-mecanismos-cuello-de-botella.md`*

Rutas relativas a la raíz del repo `gentle-ai/`. "No encontrado" significa que se buscó con grep y no apareció, no que sea imposible.

---

## 0. Resumen para el diseño

| Tema | Hallazgo clave | Implicación para el harness de documentos |
|---|---|---|
| Modelo/esfuerzo | Asignación **estática por rol**, elegida con presets en la TUI y guardada en `~/.gentle-ai/state.json`. No hay selección dinámica por complejidad. | Hay que definir los criterios de modelo por fase nosotros. Para nuestros propios subagentes de Claude basta con escribir el frontmatter `model:`/`effort:`. |
| Claude: roles ODD | Los roles `odd-explorer/worker/verify` y `orchestrator` se guardan, pero **no encontré ningún consumidor en Claude Code**. Solo los subagentes `jd-*` y `review-*` reciben `model:` y `effort:`. | En Claude, la delegación ODD usa el modelo por defecto del subagente nativo. Nuestros subagentes deben fijar su modelo ellos mismos. |
| Orquestación | Por defecto el agente trabaja inline. Solo delega ante disparadores con nombre (mapa, escritor en paralelo, backstop de contexto, verificador de alto riesgo). Pasa el contexto por referencia (`Spec: odd/tasks/<f>.md, T2, S3-S4`). | El patrón de traspaso por referencia y el verificador que emite un veredicto por `S#` se pueden reutilizar casi tal cual. |
| Skills | Se instalan `SKILL.md` en `~/.claude/skills/<id>/`. Un registro por proyecto (`.atl/skill-registry.md`) indexa skills del proyecto y globales, y el del proyecto gana. | **Se pueden agregar skills propias sin fork**. Sobreviven a `sync` siempre que su nombre no choque con uno incluido ni con `sdd-*`, `_shared` o `skill-registry`. |
| ODD | Se inyecta **incondicionalmente** y se declara MANDATORY "on every request". No encontré un interruptor para apagarlo. | Sustituirlo sin fork no es posible. Convivir exige sobrescribirlo de forma explícita a nivel de proyecto, o instalar gentle-ai solo con alcance workspace en los repos de código. |
| RDD | Está ligado a git y a código. Un `.md` "pasivo" recibe **0 lentes**: solo lectura estructural. | RDD no aporta nada a los documentos. Lo reutilizable es **Judgment Day** (dos jueces ciegos más un corrector, con presupuesto acotado) y el esquema de ledger. |
| Conflicto fuerte | El contrato de idioma pide que los artefactos generados vayan **en inglés por defecto** salvo pedido explícito. | Nuestro harness tiene que declarar el idioma del artefacto de forma explícita y persistente. |

---

## 1. Modelos y esfuerzo por agente y subagente

### 1.1 Dónde se declara

| Capa | Archivo o lugar | Qué guarda |
|---|---|---|
| Elección del usuario | TUI **Configure Models** (`internal/tui/screens/claude_model_picker.go`, `codex_model_picker.go`, `model_picker.go`) | Preset o asignación Custom por rol |
| Persistencia | `~/.gentle-ai/state.json`, campos `ClaudeModelAssignments` (legado, solo modelo) y `ClaudePhaseAssignments` (modelo + esfuerzo) (`internal/cli/run.go:315-322`, `internal/app/app.go:663-674`) | Mapa rol → {modelo, esfuerzo} |
| Proyección en Claude Code | Frontmatter de `~/.claude/agents/<rol>.md`: `model: {{CLAUDE_MODEL}}` y `{{CLAUDE_EFFORT_FRONTMATTER}}` (`internal/assets/claude/agents/*.md`, líneas 3-8) | Lo resuelve `renderNativeAgent` (`internal/components/reviewassets/install.go:238-265`) |
| Proyección en Codex | `~/.codex/config.toml`, con `model` y `model_reasoning_effort` de primer nivel para el orquestador (`internal/components/engram/inject.go:889-893`), más una tabla de clases de worker inyectada en `AGENTS.md` (`internal/components/agentguidance/inject.go:114-117`) | El orquestador pasa `model` y `reasoning_effort` a `spawn_agent` con `fork_turns: "none"` (`internal/assets/codex/orchestrator.md:167,228`) |
| Proyección en OpenCode | `opencode.json` → `agent.<nombre>.model` y `agent.<nombre>.variant` (`internal/cli/sync.go:1157-1180`) | `variant` es el "esfuerzo" que anuncia el proveedor (`internal/opencode/models.go:79-86`) |
| Revisores RDD en Claude | Proceso aparte `claude --print --model X --effort Y --setting-sources "" --tools ""` (`internal/reviewerprovider/claude_adapter.go:38-73`) | Lee el rol (`risk`, `readability`, `reliability`, `resilience`, `refuter`, `validator`) de `state.json` (`internal/cli/review_provider_runtime.go:84-110`). El commit HEAD justamente corrige "honor assigned Claude reviewer effort". |

### 1.2 Valores posibles (Claude)

- **Modelos**: `fable` > `opus` > `sonnet` > `haiku` (`internal/model/claude_model.go:12-29`). El comentario del código asigna roles a cada uno: fable para "most demanding architectural and review work"; opus para arquitectura y orquestación; sonnet para la mayoría de las fases; haiku para tareas mecánicas.
- **Esfuerzo**: `""` (hereda), `low`, `medium`, `high`, `xhigh`, `max`. Depende del modelo: fable y opus admiten todos; sonnet todos menos `xhigh`; **haiku solo el default** (`claude_model.go:70-83`). Si la combinación no es válida, no se escribe `effort:` (`install.go:257-265`).

### 1.3 Roles configurables y presets

**Roles que muestra la TUI de Claude** (`claude_model_picker.go:41-72`): `odd-explorer`, `odd-worker`, `odd-verify`, `jd-judge-a`, `jd-judge-b`, `jd-fix-agent`, `risk`, `readability`, `reliability`, `resilience`, `refuter`, `validator` y `default` ("General delegation"). `orchestrator` se persiste pero no aparece como fila editable (`:165-169`).

**Presets de Claude** (`claude_model.go:132-209`, descripciones en `claude_model_picker.go:23-30`):

| Rol | balanced (default) | performance | economy | diversity |
|---|---|---|---|---|
| orchestrator | opus | opus | sonnet | opus |
| odd-explorer/worker/verify (forzados en la TUI) | sonnet | sonnet | sonnet | sonnet |
| jd-judge-a | sonnet | opus | haiku | **opus** |
| jd-judge-b | sonnet | opus | haiku | **haiku** |
| jd-fix-agent | sonnet | opus | haiku | sonnet |
| default | sonnet | sonnet | sonnet | sonnet |

Los presets no fijan esfuerzo: siempre queda en default y solo se cambia en Custom (`ClaudePhaseAssignmentsFromModelPreset`, `claude_model.go:126-130`). La TUI fuerza `odd-*` a sonnet en todos los presets (`claude_model_picker.go:173-186`).

**Presets de Codex** (`internal/model/codex_model.go:295-325`):

| Preset | Orquestador | odd-explorer | odd-worker | odd-verify |
|---|---|---|---|---|
| low-cost | luna / medium | luna / high | luna / medium | sol / medium |
| recommended | sol / medium | luna / high | luna / high | sol / medium |
| powerful | astra / medium | luna / high | sol / high | astra / xhigh |

El orquestador va siempre en **medium**. El código lo justifica así: planifica, enruta y adjudica, y con low era "the weakest link" (`codex_model.go:401-405`). Además existe un `service_tier` (Fast) separado del esfuerzo (`docs/usage.md:329-333`).

**Kiro** tiene sus propios alias (Auto, Sonnet, Opus, Qwen, DeepSeek, MiniMax, GLM) con presets análogos (`internal/model/kiro_model.go:55-121`).

### 1.4 ¿Por agente host, por fase o por tipo de tarea?

- **Por host**: sí. Cada runtime tiene su propio picker, sus presets y su mecanismo de proyección (frontmatter, `config.toml`, `opencode.json`). `docs/agents.md:47` lo dice así: "Model assignment is client-specific… not a formal development-phase matrix".
- **Por tipo de tarea**: sí, mediante roles: explorar, implementar, verificar, juzgar, corregir y las cuatro lentes de revisión más refuter y validator. No hay asignación por *fase* de ODD; las fases de SDD se retiraron en v4.0.0.
- **Por complejidad**: **no encontré selección dinámica**. Las únicas reglas que dependen del modelo usado son estas:
  - **Sesgo de modelo pequeño**: si el escritor corrió con esfuerzo low o con un modelo mini, sube un nivel el tier de verificación y exige un verificador aparte incluso en riesgo medio (`internal/assets/skills/_shared/odd-orchestrator-sections.md:16,30,33`).
  - **Variante de prompt** `capable`/`small` del orquestador genérico. Selecciona *texto*, no modelo (`internal/components/agentguidance/inject.go:67-69`; `internal/assets/generic/orchestrator.md:1,123`).

### 1.5 Hallazgo importante: en Claude, los roles ODD no tienen efecto observable

- El instalador de agentes nativos de Claude solo escribe `jd-fix-agent`, `jd-judge-a`, `jd-judge-b` y `review-{readability,refuter,reliability,resilience,risk}` (`internal/components/reviewassets/install.go:27`).
- **No existe** un `gentle-ai-worker` ni un explorador o verificador para Claude. El orquestador de Claude manda usar "Claude Code's native Agent/Task mechanism" (`internal/assets/claude/orchestrator.md:75`) y no dice qué modelo usar.
- Busqué consumidores de `odd-worker`/`odd-explorer`/`odd-verify` fuera de `internal/model` y de la TUI. Solo aparecen en Codex (tabla para `spawn_agent`) y en Kiro (asignaciones), además de tests. Tampoco encontré consumidor de `"orchestrator"` para Claude.
- **Conclusión**: en Claude Code, explorador, escritor y verificador corren con el modelo que Claude Code asigne por defecto a sus subagentes. Lo que se elige en la TUI para esos roles se guarda pero no se proyecta.
- En OpenCode, en cambio, sí se proyecta: `gentle-ai-explore`, `gentle-ai-verify` y `gentle-ai-worker` son agentes con nombre que reciben `model` y `variant` (`internal/opencode/models.go:118-126`; `internal/cli/sync.go:1157-1180`).

### 1.6 Ejemplo concreto (Claude, preset diversity)

`~/.claude/agents/jd-judge-a.md` queda con `model: opus` y sin línea `effort:`; `jd-judge-b.md` queda con `model: haiku`. Si en Custom se elige `jd-judge-a = opus/high`, se escribe `effort: high` (está cubierto por un test en `internal/components/reviewassets/install_options_test.go:93`).

---

## 2. Orquestación y delegación

### 2.1 Directo o delegado

La fuente canónica es `internal/components/agentguidance/routing.go:24-161`, que se renderiza como `## Implementation Routing` en cada runtime. Las secciones compartidas están en `internal/assets/skills/_shared/odd-orchestrator-sections.md`.

- **Principio**: "does this inflate the parent context without need?" (`claude/orchestrator.md:61`). Por defecto, inline.
- **Presupuesto de evidencia inline**: 1 lote paralelo, como máximo 3 llamadas y unos 10k tokens (`internal/agents/capabilitymanifest/manifest.go:217-219`; `routing.go:53,66-72`).
- **Disparadores obligatorios** (`routing.go:89-101`):
  - **Ask**: una decisión de producto abierta lleva una sola pregunta, y luego se detiene.
  - **Mapping**: más de unas 5 búsquedas secuenciales o más del presupuesto inline lleva un explorador de solo lectura. El handoff es de unos 2k tokens con evidencia `path:line` y admite 1 verificación puntual del padre (`manifest.go:227-231`).
  - **Verification**: un cambio de alto riesgo lleva un verificador independiente.
  - **Tracking**: una tarea grande lleva documento de feature, espejo en Engram y commits por unidad.
  - **Writer**: se delega un escritor solo con una razón nombrada, sea paralelismo (unidades independientes con superficies disjuntas) o el backstop de contexto. "Never… for size… file count".
  - **Long-session backstop**: a unos 150k tokens de contexto del padre, delegar la siguiente unidad. Es un consejo y no está instrumentado (`routing.go:99`).
  - **Route declaration**: en tareas grandes, registrar la ruta elegida por tarea en el documento (`routing.go:100`).
- Si el runtime no puede delegar, hay que seguir su ruta de degradación o detenerse y explicar (`odd-orchestrator-sections.md:100`). Codex tiene un "Graceful Degradation Path" que **no** permite seguir inline (`codex/orchestrator.md`, sección homónima).

### 2.2 Catálogo de subagentes

| Subagente | Runtime | Rol | Herramientas | Modelo | Cuándo |
|---|---|---|---|---|---|
| Agent/Task nativo (general) | Claude | explorador, escritor y verificador ODD | las del subagente nativo | default de Claude Code (ver §1.5) | según los disparadores de §2.1 |
| `gentle-ai-explore` | OpenCode | explorador de solo lectura; prioriza `codegraph_explore` | deniega write, edit, bash y task (`internal/components/opencodeagents/agents.go:20`) | `opencode.json` | Mapping trigger |
| `gentle-ai-worker` | OpenCode | escritor acotado; nunca hace commit ni delega | deniega task (`agents.go:22`) | ídem | Writer trigger |
| `gentle-ai-verify` | OpenCode | verificador técnico de solo lectura; da veredicto por `S#`; sus probes van en una copia scratch creada con `mktemp -d` | deniega write, edit y task (`agents.go:21`) | ídem | riesgo alto, o medio con un modelo pequeño |
| `odd-explorer/worker/verify` | Codex | clases de worker (no son agentes instalados) | las de `spawn_agent` | tabla en AGENTS.md | ídem |
| `jd-judge-a`, `jd-judge-b` | Claude, Kiro, OpenCode | jueces ciegos | Read, Glob, Grep y lectura de Engram (`claude/agents/jd-judge-a.md:8`) | `jd-judge-a`/`jd-judge-b` | Judgment Day |
| `jd-fix-agent` | ídem | corrector quirúrgico | Read, Edit, Write, Glob, Grep, Bash y Engram (`jd-fix-agent.md:7`) | `jd-fix-agent` | tras la confirmación de ambos jueces |
| `review-risk/readability/reliability/resilience` | Claude | lentes R1 a R4 de RDD | `tools: []` (se ejecutan en proceso, aislados) | roles `risk`… | RDD con riesgo medio o alto |
| `review-refuter` | Claude | refutador de hallazgos severos | `tools: []` | `refuter` | una vez por transacción |

### 2.3 Paso de contexto y de resultados

- **Por referencia, nunca por paráfrasis** (`routing.go:43`): "Spec: odd/tasks/<feature>.md, T2, S3-S4". El worker lee hasta `## Log` e informa qué `S#` cubrió. Si no hay documento, se pasa el pedido del usuario textualmente.
- **Superficies de edición permitidas**: el padre las deriva y las pasa bajo el encabezado `## Allowed edit surfaces`. El escritor se niega a escribir fuera de ellas. Si no se pueden derivar, se ofrece una lista candidata para aprobar o rechazar; nunca una pregunta abierta (`odd-orchestrator-sections.md:136-150`).
- **Skills**: el padre pasa las rutas exactas bajo `## Skills to load before work`. El subagente informa `skill_resolution` (`paths-injected`, `fallback-registry`, `fallback-path` o `none`) (`odd-orchestrator-sections.md:196-215`).
- **Cierre `## Key Learnings`**: 1 a 5 hechos reutilizables que Engram captura de forma pasiva (`odd-orchestrator-sections.md:154-156`).
- **Retorno**: handoff comprimido de unos 2k tokens con comandos ejecutados, resultados observados y evidencia `path:line` (`opencode/agents/gentle-ai-verify.md`, al final).
- **Engram**: en OpenCode y Codex el padre busca en memoria y pasa el contexto; el subagente no busca por su cuenta (`opencode/orchestrator.md`, "Sub-Agent Context Protocol").

### 2.4 Paralelismo

- **Escritores en paralelo**: superficies disjuntas en el mismo árbol, el padre es dueño de git y hay un seam check al final. Si dos unidades necesitan el mismo archivo, usan worktrees aislados. El trabajo en otro repositorio va en un worktree nuevo (`routing.go:96`).
- **Codex**: `features.multi_agent=true`, `agents.max_threads=4`, `agents.max_depth=2` (`engram/inject.go:884-886`). Todo spawn se espera con `wait_agent` antes de continuar.
- **OpenCode**: background subagents opcionales (`--opencode-background-subagents`). No son durables, no aíslan el sistema de archivos y no deben usarse para escritores en el mismo árbol (`docs/opencode-profiles.md`).
- **Judgment Day**: los dos jueces se lanzan en paralelo (`skills/judgment-day/SKILL.md:17,40`).
- **Deduplicación de lanzamientos** en OpenCode con pares `(phase, fingerprint)` (`opencode/orchestrator.md`, sección "Sub-Agent Launch Deduplication").

---

## 3. Skills

### 3.1 Formato

- `SKILL.md` con frontmatter `name`, `description` (una sola línea entre comillas, con el disparador al principio y como máximo 250 caracteres), `license` y `metadata.author/version` (`docs/skill-style-guide.md:21-27`).
- **Orden del cuerpo**: Activation Contract → Hard Rules → Decision Gates → Execution Steps → Output Contract → References. El cuerpo debería tener entre 180 y 450 tokens, con un máximo duro de 1000. Los detalles van en `references/` o `assets/` (`skill-style-guide.md:9-35`).
- Ejemplo canónico: `internal/assets/skills/judgment-day/SKILL.md`.

### 3.2 Instalación, registro y descubrimiento

- **Instalación**: las skills incluidas se embeben en el binario y se escriben en `adapter.SkillsDir()` (para Claude, `~/.claude/skills/<id>/…`), junto con `_shared/` (`internal/components/skills/inject.go`; `internal/agents/claude/adapter.go`, `SkillsDir`). Solo se escriben los IDs seleccionados (`directoryAssets`).
- **Registro**: `gentle-ai skill-registry refresh` recorre primero las raíces del proyecto (`skills/`, `.claude/skills/`, `.opencode/skills/`, `.github/skills/`, `.agents/skills/` y otras) y después las globales (unas 20 raíces, entre ellas `~/.claude/skills`) (`internal/skillregistry/registry.go:61-110`). Deduplica por nombre (gana el proyecto) y escribe `.atl/skill-registry.md` con `Skill | Trigger/description | Scope | Path`, más una caché con fingerprint (`docs/skill-registry.md:35-64`).
- **Exclusiones silenciosas**: `_shared`, `skill-registry` y todo lo que empiece por `sdd-` (`registry.go:26-40`).
- **Refresco automático** (§4): un hook `UserPromptSubmit` en Claude, `SessionStart` en Codex y el plugin `skill-registry.ts` en OpenCode.
- **Carga por el orquestador**: lee `.atl/skill-registry.md`, empareja por descripción y por archivos, pasa rutas a los subagentes y prefiere la skill de proyecto sobre la global (`odd-orchestrator-sections.md:162-185`).
- **Carga por la persona** (Claude): "Self-check BEFORE every response" para invocar la skill con el tool `Skill` (`internal/assets/claude/persona-gentleman.md`, sección *Contextual Skill Loading*).

### 3.3 Skills propias sin forkear

- **Sí se puede.** Basta con poner `<nombre>/SKILL.md` en `~/.claude/skills/` (global) o en `<proyecto>/.claude/skills/` o `<proyecto>/skills/` (proyecto). El registro las indexa y el orquestador las ofrece.
- **Sobreviven a `sync`** si su nombre no coincide con una skill incluida, porque el instalador solo escribe los directorios de los IDs seleccionados. *Inferencia*: una skill de usuario con el mismo nombre que una incluida (por ejemplo `judgment-day`) sería sobrescrita, porque la escritura es atómica y no hay ledger de propiedad para skills. No encontré protección contra eso.
- **Agent Builder** (TUI): genera un `SKILL.md` a partir de una descripción en lenguaje natural y lo instala en todos los agentes. Lleva su propio registro en `~/.config/gentle-ai/custom-agents.json` (`internal/tui/model.go:4647`) y detecta conflictos con las skills incluidas (`internal/agentbuilder/registry.go:77-78`). Según el PRD es una "draft" (`PRD-AGENT-BUILDER.md`).
- Skills incluidas que pueden servir para documentos: `cognitive-doc-design` (estructura que reduce la carga cognitiva) y `comment-writer`. Están en `docs/components.md:73-91`.

---

## 4. Hooks por agente host

| Host | Archivo | Evento | Comando o acción | Detalle |
|---|---|---|---|---|
| Claude Code | `~/.claude/settings.json` | `Stop` | `gentle-ai review stop-hook --agent claude-code` (timeout 60) | Recuerda al final del turno los candidatos RDD que produjo esa sesión; **no inicia revisiones** (`internal/components/agenthooks/claude.go:46-56`; `docs/review-integration.md:31`) |
| Claude Code | ídem | `SessionStart` (matcher `startup\|resume\|clear\|compact`) | el mismo stop-hook | Registra el candidato base de la sesión |
| Claude Code | ídem | `SubagentStop` y `Stop` | `gentle-ai telemetry runtime claude --json` (async, 5 s) | Telemetría de runtime |
| Claude Code | ídem | `UserPromptSubmit` | `gentle-ai skill-registry refresh --quiet --no-gitignore --cwd …` | Refresca el registro en cada prompt; la caché lo abarata (`agenthooks/skill_registry.go:83-86`) |
| Codex | `~/.codex/hooks.json` | `SessionStart` | refresco del registro de skills | `skill_registry.go:79-82` |
| Codex | ídem | `SubagentStop` y `Stop` | telemetría (async) | `agenthooks/codex_telemetry.go` |
| OpenCode | `~/.config/opencode/plugins/*.ts` | setup | `skill-registry.ts`: refresco si detecta un proyecto | `internal/assets/opencode/plugins-v2/skill-registry.ts` |
| OpenCode | ídem | `session.step.*` | `telemetry-runtime.ts` | — |
| OpenCode | ídem | `model.updated` y `provider.updated` | `model-variants.ts`: refresca el catálogo de variantes | — |
| OpenCode | ídem | `tool execute.before/after`, `shell create.before`, `session.deleted` | `opencode-review-transport.ts`: transporte de RDD | `:219-281` |

- **Idempotencia**: antes de agregar un hook se comprueba que el comando no exista (`claudeHookListContains`). Los hooks del usuario se conservan.
- **JSONC**: los escritores de hooks solo reescriben el valor `hooks`. Otros escritores (persona, permisos, output style) sí normalizan el archivo (`docs/review-integration.md:33`).

---

## 5. Persona y estilo

- **Opciones**: `gentleman`, `neutral` o `custom`. `custom` significa no inyectar persona y dejar la del usuario (`docs/usage.md:101-109`; `internal/components/persona/inject.go:124-126`).
- **Claude**: un bloque `<!-- gentle-ai:persona -->` en `~/.claude/CLAUDE.md`, el archivo de estilo en `~/.claude/output-styles/` y `"outputStyle": "Gentleman"|"Neutral"` en `settings.json` (`persona/inject.go:67-70`). El output style lleva `keep-coding-instructions: true`.
- **Reglas de la persona** que chocan con trabajo de documentos (`claude/persona-gentleman.md`):
  - respuestas cortas por defecto;
  - una pregunta por vez y luego detenerse;
  - nada de menús de opciones salvo una bifurcación real;
  - "Never use cat/grep/find/sed/ls" (pide bat/rg/fd);
  - "Never add Co-Authored-By";
  - pericia declarada en arquitectura de software.
- **Contrato de idioma** (`odd-orchestrator-sections.md:5-11`, inyectado también en cada subagente nativo como `agent-language-contract`, `reviewassets/install.go:273`):
  - la persona controla solo la conversación;
  - los **artefactos técnicos van en inglés por defecto**;
  - en otro idioma solo si se pide explícitamente, con registro neutral.
- **No hay editor de persona**: `custom` es una decisión de propiedad, no una forma de personalizar (`docs/usage.md:109`).

---

## 6. Configuración global: qué escribe y cómo convive

### 6.1 Archivos en el sistema del usuario (Claude Code, alcance global)

| Ruta | Contenido gestionado | Mecanismo |
|---|---|---|
| `~/.claude/CLAUDE.md` | Secciones `<!-- gentle-ai:orchestrator -->`, `<!-- gentle-ai:agent-routing -->` (con `remote-authorization` anidado), `persona` y `engram-protocol` | Secciones marcadas; lo que esté fuera de los marcadores se conserva tal cual (`internal/components/agentguidance/inject.go:97-98`; `filemerge/section.go:23-25`) |
| `~/.claude/settings.json` | `hooks`, `outputStyle`, `permissions.deny` (rutas sensibles) | Merge JSON (`internal/components/permissions/inject.go`) |
| `~/.claude/output-styles/` | Gentleman y Neutral | archivo |
| `~/.claude/agents/*.md` | `jd-*` y `review-*`, más un ledger de propiedad | Solo sobrescribe si el hash coincide con el ledger; si no, el archivo se salta (`reviewassets/install.go:139-160`) |
| `~/.claude/skills/<id>/`, `~/.claude/skills/_shared/` | skills incluidas y referencias | escritura atómica |
| `~/.claude.json` | MCP de usuario (Context7), con permisos 0600; también guarda la sesión OAuth | Merge con reintentos; no lo toca si el JSON no es válido (`internal/agents/claude/adapter.go:86-128`; `components/mcp/inject.go:284-289`) |
| `~/.claude/gentle-ai/orchestrator/` | Módulos bajo demanda (piloto, opt-in con `--claude-orchestrator-modules`) | ledger propio (`docs/rollback.md:96-108`) |
| `~/.gentle-ai/state.json` | agentes instalados, asignaciones de modelo, preferencias | — |
| `~/.gentle-ai/backups/` | snapshots tar.gz antes de install, sync y upgrade; conserva 5; se pueden fijar | `docs/rollback.md` |
| `~/.gentle-ai/review-stop-hook/v1`, `~/.gentle-ai/review-context.key`, `~/.gentle-ai/bin/` | estado de hooks, clave AES de RDD, lanzadores de OpenCode | — |
| `<proyecto>/.atl/skill-registry.md` y caché | registro de skills | se agrega `.atl/` a `.gitignore` salvo con `--no-gitignore` |

**Otros hosts**:
- **Codex**: `~/.codex/AGENTS.md`, `config.toml` (multi_agent, max_threads, modelo y esfuerzo del orquestador, MCP de Engram, `experimental_compact_prompt_file`), `hooks.json` y `engram-compact-prompt.md`.
- **OpenCode**: `~/.config/opencode/opencode.json` (el agente `gentle-orchestrator`, cuyo prompt lleva la guía, porque OpenCode no la lee desde un archivo global; `agentguidance/inject.go:297-299`), `plugins/` y `skills/`.

### 6.2 Idempotencia, actualización y convivencia

- **`sync` es idempotente**: la segunda corrida no cambia nada (`docs/usage.md:232`). Actualiza solo los agentes registrados en `state.json` y hace backup antes.
- **Actualizar**: `gentle-ai upgrade` y luego `gentle-ai sync`. Los assets dependen de la versión del binario.
- **CLAUDE.md propio**: se conserva lo que esté fuera de los marcadores. **Lo que esté dentro se pierde en cada `sync`.**
- **Alcance**: `--scope workspace` escribe en el proyecto (por ejemplo `<proyecto>/CLAUDE.md` y `.claude/`); los ajustes que solo existen a nivel global siguen siendo globales (`docs/agents.md:30`; `internal/cli/scope.go`). El commit a18513a ajusta los hooks de Claude para que respeten ese alcance.
- **Desinstalar**: quita solo lo gestionado y respeta los archivos que no reconoce como suyos (`docs/usage.md:250-271`).
- **No hay interruptor para ODD**: "ODD… is shared routing guidance, not a separate component" (`docs/components.md:21`). El enrutamiento es "always-installed… unconditional" (`agentguidance/routing.go:1-3`). `--strict-tdd` se retiró en v4.0.0 (`docs/usage.md:406`).

---

## 7. Revisión: RDD y Judgment Day

### 7.1 RDD (Receipt-Driven Development)

- **Interruptor**: activado por defecto, opt-out global o por clon con `gentle-ai review mode enable|disable|status` (`docs/review-integration.md:9-11`). El orquestador debe obedecer `disable` sin discutir (`routing.go:152-159`).
- **Flujo**:
  1. Tras cada commit de unidad de trabajo, `gentle-ai review assess` devuelve `risk` y `review_due`.
  2. Si es alto, STATUS → START (congela el candidato en git) → consentimiento (`consent/v3`) → lentes → refuter → **una** corrección acotada → validator dirigido → `approved` → acknowledgement que "quema" la autoridad.
  3. La revisión es informativa y **nunca autoriza la entrega** (`docs/review-integration.md:14-27,63-76,120-160`).
- **Niveles de riesgo** (`internal/reviewtransaction/risk.go:220-238`):
  - **high** si hay señal de alto riesgo, ruta caliente o escalamiento del agente (ítems 1 a 6);
  - **low/passive** si solo cambió contenido pasivo probado por sus bytes;
  - **medium** en cualquier otro caso.
  - `review_due` solo es verdadero en `high` (`docs/review-integration.md:159`).
- **Política de lentes 0/1/4**: passive → 0 lentes; medium → 1 (reliability por defecto); high → R1 a R4. El agente puede elegir con `--lenses` y `--lenses-reason` (`risk.go:134-185`).
- **Documentos pasivos**: `.md` no operativo, `.mdx` estático, `.rst`, `.adoc` e imágenes con firma válida.
  - Cuenta como `.md` **operativo**, y por lo tanto no pasivo: `AGENTS.md`, `CLAUDE.md`, `SKILL.md`, rutas con `.claude/`, `.agents/` y similares, y nombres con tokens como agent, skill, prompt, instruction, orchestrator o workflow (`risk.go:1240-1280`).
- **Jueces y modelos**: las lentes se ejecutan en un proceso Claude aislado, sin herramientas, sin ajustes del usuario ni CLAUDE.md, con el modelo y esfuerzo de cada rol (`reviewerprovider/claude_adapter.go:29-73`). Las reglas de cada lente son de código; por ejemplo R1 cita fuentes OWASP (`claude/agents/review-risk.md:10-22`).
- **Para documentos**: un repo de prosa `.md` queda casi siempre en *passive*, es decir, **cero revisores**. Si los documentos tienen nombres como `workflow-*.md`, pasan a "operativos" y se clasifican como medium con una lente de código. **RDD no sirve para revisar documentos.**

### 7.2 Judgment Day (JD)

Skill independiente; no necesita RDD ni git (`skills/judgment-day/SKILL.md:12`).

- **Pasos**:
  1. Se congela un objetivo inmutable.
  2. Se lanzan en paralelo `jd-judge-a` y `jd-judge-b`, ciegos, de solo lectura y con los mismos criterios.
  3. Se unen los hallazgos en un ledger congelado.
  4. Se pregunta al usuario antes de la primera corrección.
  5. `jd-fix-agent` corrige **solo** los hallazgos severos que **ambos** jueces confirmaron.
  6. Re-juicio acotado: solo el ledger más el delta de la corrección.
  7. **Máximo dos rondas**. Veredicto `APPROVED` o `ESCALATED` (`SKILL.md:14-48`).
- **Compuertas**: lo que reporta un solo juez queda como *suspect* y no se corrige automáticamente. Si los jueces se contradicen, se escala al humano (`SKILL.md:26-35`).
- **Ledger** (`claude/agents/jd-judge-a.md`, sección *Review ledger contract*):
  - campos `id`, `lens`, `location`, `severity` (BLOCKER/CRITICAL/WARNING/SUGGESTION), `status` y `evidence`;
  - *precision gate*: "when in doubt, stay silent";
  - *severity floor*: solo BLOCKER y CRITICAL entran al ciclo de corrección;
  - el ledger persiste en Engram en `review/{slug}/ledger`.
- **Modelos**: roles propios. El preset diversity usa opus para A y haiku para B, buscando diversidad de perspectiva (`claude_model.go:199-209`).
- **Criterios del juez** en el prompt: "correctness, edge cases, error handling, performance, security, and project conventions" (`skills/judgment-day/references/prompts-and-formats.md`). La salida es JSON con `location`, `severity`, `claim`, `evidence_class`, `causal_disposition` y `proof_refs`.

### 7.3 Qué se puede reutilizar para documentos

| Pieza | Reutilizable | Ajuste necesario |
|---|---|---|
| JD: dos jueces ciegos, consenso como corroboración, corrector acotado, dos rondas, APPROVED/ESCALATED | **Sí, casi completa** | Criterios de documento en lugar de código (por ejemplo coherencia argumental, trazabilidad a fuentes, consistencia con decisiones). `location` como `archivo:sección` o `archivo:línea`. Hecho como skill propia, sin tocar la incluida. |
| Esquema de ledger, precision gate, severity floor, presupuesto de convergencia | Sí | Agregar campos para el criterio aplicado (relevante para H3: criterio auditable). |
| Verificador por `S#` (`gentle-ai-verify`) | Sí, como patrón | Veredicto por requisito del encargo y evidencia citada, en lugar de probes ejecutables. |
| Refuter con voto 2 de 3 (full-4R) | Sí, como patrón | — |
| Binario `gentle-ai review` (RDD) | No | Ligado a git y a código; clasifica los documentos como passive. |

---

## 8. ODD: flujo, tamaño, estados, documento y correcciones

### 8.1 Protocolo (MANDATORY, en orden, en cada pedido)

Fuente: `routing.go:35-50`.

1. **Authorize**. Investigar, explicar, revisar, auditar, comparar o proponer es de solo lectura. Si la intención es ambigua, una aclaración, y se mantiene en solo lectura.
2. **Explore**, en proporción al pedido.
3. **Resolve uncertainty**. Investigación opcional solo ante una incertidumbre nombrada; una pregunta por cada decisión de producto real; como máximo un desafío a un supuesto de alto impacto.
4. **Classify** (ver §8.2).
5. **Track before the first write**. Si es grande, se crea `odd/tasks/<feature>.md` y el espejo en Engram `odd/<feature>/tasks` sin pedir permiso, y se informa en una línea.
6. **Implement task by task**, con traspaso por referencia, test-first cuando aplica, y un commit de unidad de trabajo por tarea en una rama de feature.
7. **Close**, con línea de riesgo, checks fallidos o pendientes y siguiente paso.
- **Resume**: `mem_context` → `mem_search` → `mem_get_observation` → archivo de tareas; reconciliar antes de seguir (`routing.go:50,134-135`).

### 8.2 Reglas de tamaño (`routing.go:51-63`)

- **Pequeña**, si se cumplen las tres:
  - **Entendida**: está demostrado dentro de un lote de lectura (≤3 llamadas, unos 10k tokens).
  - **Riesgo contenido**.
  - **Retomable desde el diff**: alguien podría terminarla con el pedido original y `git diff`.
- **Grande**, solo si falla esa prueba de reanudación (varias sesiones, espera de algo externo, entregables separados o requisitos que no sobrevivirían una compactación).
- **Nunca deciden el tamaño**: el número de archivos, comandos, tests o líneas, ni que el usuario pida una lista de tareas.
- **Unas 400 líneas por tarea**: es una heurística de planificación, no un tope (`routing.go:125`). Sí gatilla la estrategia de entrega en PRs encadenados (`routing.go:142`).
- **Alto riesgo**, seis ítems: datos o efectos irreversibles, seguridad, contratos que otros consumen, concurrencia, entrega o entorno, y cambios sin test que detecte una regresión. Toda tarea cierra con `Risk: item N (motivo)` o `Risk: none` (`routing.go:58-63`).

### 8.3 Estados públicos

Son cuatro: **Working**, **Checking**, **Ready** y **Needs your decision** (`docs/trigger-rules.md:21-22,48-56`; `README.md:159`).
- Una verificación agotada o no disponible se convierte en **un único** "Needs your decision" que nombra los bloqueos (`odd-orchestrator-sections.md:45`).

### 8.4 Formato del documento de feature

Fuente: `routing.go:42,126`. Ejemplo real: `odd/tasks/odd-spec-by-reference.md`.

```
# <título>
<2–3 líneas: objetivo, rama, estrategia de entrega, runner de tests>

## Specs
S1. … cita textual del usuario (strings, errores, ejemplos) en su idioma; sin resumir;
    sin requisitos no pedidos; supuestos marcados como supuestos; criterios de aceptación y checks
## Tasks
- [x] T1 (S2-S5) inline · <superficie> · RED→GREEN · <commit>
## Log
L1 <fecha> user (verbatim): > <pedido original>
L2 … correcciones del usuario textuales con fecha; evidencia; decisiones; siguiente paso
```

- El orden es fijo: lo estable arriba y el log, que crece, al final. Así los workers leen "hasta `## Log`".
- No hay un "diario de decisiones exhaustivo"; las correcciones de rutina van breves.
- El espejo en Engram contiene el documento completo y no es atómico con el archivo: hay que releer ambas escrituras (`routing.go:133`).

### 8.5 Reglas de corrección

- **Cambio de requisito**: se agrega la entrada textual al `## Log`, **se reescribe solo el `S#` afectado y se reabre solo su tarea**. Se preserva el trabajo completo y el no relacionado (`routing.go:42,127`).
- **Los hallazgos no autorizan** ampliar el alcance ni aceptar automáticamente. Un cambio de alcance de negocio requiere autorización del usuario. Marcar una casilla requiere un resultado observado y no equivale a aprobación.
- **Bloqueos del verificador**: un lote de corrección que arregla todos, luego un rechequeo limitado a esos bloqueos. Una segunda corrección solo si el mismo bloqueo persiste; un hallazgo nuevo no da derecho a otra. Lo que quede abierto pasa a "Needs your decision" (`odd-orchestrator-sections.md:20,34`).
- **Severidad del verificador**: es bloqueo un defecto causado por el cambio o un `S#` incumplido que se reproduce con entradas realistas y que no existía en la línea base. Lo preexistente es una advertencia (`gentle-ai-verify.md`, sección *Severity*).

---

## 9. Puntos de extensión para una capa de documentos encima de gentle-ai (sin fork)

### 9.1 Qué se puede añadir

| Mecanismo | Cómo | ¿Lo usa el orquestador? | ¿Sobrevive a sync? |
|---|---|---|---|
| **Skills propias** (por ejemplo `doc-encargo`, `doc-criterios`, `doc-juicio`) | `~/.claude/skills/<n>/SKILL.md` o `<proyecto>/.claude/skills/` | Sí: el registro las indexa y el orquestador las empareja por descripción y pasa rutas; la persona fuerza el self-check con el tool `Skill` | Sí, salvo colisión de nombre (§3.3) |
| **Subagentes propios de Claude** | `~/.claude/agents/<n>.md` o `<proyecto>/.claude/agents/`, con su propio `model:`, `effort:` y `tools:` | Claude Code los expone a su tool Agent. El orquestador de gentle-ai **no los conoce**: hay que referirse a ellos desde una skill o desde CLAUDE.md fuera de los marcadores | Sí: el instalador solo toca los nombres de su manifiesto y archivos cuyo hash está en su ledger |
| **Agentes propios en OpenCode** | entradas `agent.<n>` en `opencode.json` | Aparecen en el picker de modelos (`opencodedefault/custom_agents.go:13-45`; `sync.go:1167-1169`) | Sí (son reservados para el usuario) |
| **Instrucciones de proyecto** | `<proyecto>/CLAUDE.md` o `AGENTS.md`, fuera de los marcadores `gentle-ai:` | Se cargan junto con la guía global | Sí |
| **Persona `custom`** | `--persona custom` | Quita persona y output style gestionados | — |
| **RDD apagado** | `gentle-ai review mode disable` (global o `--scope clone`) | Elimina consentimientos y recordatorios del stop-hook | Sí (preferencia guardada) |
| **Telemetría apagada** | `DO_NOT_TRACK=1`, `GENTLE_AI_TELEMETRY=0` o `gentle-ai telemetry disable` (`docs/telemetry.md:553-558`) | — | — |

### 9.2 ¿Sustituir ODD o convivir?

- **Sustituir sin fork: no.** Las secciones `orchestrator` y `agent-routing` se reinyectan en cada `install` y `sync`, se declaran MANDATORY en cada pedido y no tienen interruptor (§6.2). Editar dentro de los marcadores se pierde en el siguiente sync.
- **Convivir: sí, con tres estrategias posibles.**
  1. **Precedencia declarada en el proyecto**: un `CLAUDE.md` del repo de documentos dice que, en ese proyecto, el flujo de documentos reemplaza los pasos 4 a 7 de ODD, con su propio formato de encargo, criterios y log. Riesgo: dos instrucciones "MANDATORY" compiten y el resultado depende del modelo. Conviene probarlo.
  2. **Encajar dentro de ODD**: ODD ya separa autorizar, explorar y preguntar, y su documento `S#/T#/Log` se parece mucho a un encargo de documentos. Se podría tratar cada documento grande como una "feature" y llevar los criterios como `S#`. Fricciones: commits por tarea, prueba de reanudación con `git diff`, test-first (que exige justificar la excepción cada vez) y la ruta fija `odd/tasks/`.
  3. **Aislamiento por alcance**: instalar gentle-ai con `--scope workspace` solo en los repos de código y dejar la configuración global de Claude limpia para el trabajo con documentos. *Hay que verificarlo*: no confirmé qué piezas siguen siendo globales en ese modo (`docs/agents.md:30` dice que los ajustes solo globales siguen globales).

### 9.3 Conflictos previsibles

1. **Idioma**: artefactos en inglés por defecto, también en los subagentes nativos (§5). Choca con documentos en español salvo que se pida explícitamente en cada encargo o en las instrucciones del proyecto.
2. **Modelo de los subagentes ODD en Claude**: no se configura (§1.5). Nuestros subagentes deben fijarlo en su frontmatter.
3. **Supuestos de código**: test-first, comandos `## Verification`, `Risk:` con ítems de código, commits por unidad, rama de feature, PR encadenados a partir de unas 400 líneas.
4. **Persona**: respuestas cortas y una pregunta por vez. Esto choca con la elicitación rica (M01, M02) y con mostrar opciones divergentes (M10, M11). La persona prohíbe los menús "salvo bifurcación real".
5. **RDD activado por defecto**: los documentos operativos (nombres con workflow, prompt, agent…) se clasifican como medium y piden consentimiento. Además, el stop-hook se ejecuta en cada turno dentro de repos git.
6. **Engram proactivo**: obliga a `mem_save` tras cada decisión y antes de cerrar (`internal/assets/engram/protocol.md:23-39,108-139`). Puede duplicar nuestro registro de decisiones. Puede convenir o competir.
7. **Registro de skills**: excluye nombres `sdd-*`; la colisión con skills incluidas sobrescribe.
8. **Disparadores de delegación**: (≤3 llamadas o 10k inline, explorador si hay más de 5 búsquedas) están calibrados para código. En documentos largos se disparan siempre. No es un problema, pero la decisión no es nuestra.

---

## 10. Otras capacidades que la revisión anterior no mencionó

| Capacidad | Evidencia | Relevancia |
|---|---|---|
| **Presupuesto de contexto** con cifras explícitas (10k inline, handoff de 2k, backstop de 150k), declarado como consejo y no instrumentado | `capabilitymanifest/manifest.go:217-231`; `routing.go:99` | Las cifras sirven de referencia |
| **Módulos de orquestador bajo demanda** (Claude): un núcleo siempre cargado y módulos que se leen solo cuando un "pointer" lo indica | `agentguidance/orchestrator_modules.go:20-40`; `docs/rollback.md:96-108` | Patrón para reducir el prompt base de nuestro harness |
| **Compactación**: hook `SessionStart` con matcher `compact`; protocolo de Engram post-compactación (`mem_session_summary` inmediato); en Codex, `experimental_compact_prompt_file` | `agenthooks/claude.go:53`; `engram/protocol.md:132-139`; `engram/inject.go:890,1091` | Útil para la reentrada (C11) |
| **Lossless Blocking Prompts**: relevar menús de subagentes sin resumir ni reordenar, con validación de respuestas (ordinales, "opción N") | `claude/orchestrator.md:13-20` | Reutilizable para compuertas humanas (M34) |
| **Reporte de defectos del proveedor** con consentimiento de 3 opciones y escaneo de privacidad | `claude/orchestrator.md:22-50` | Por ahora no aplica |
| **Contrato de autorización remota** (destino, operación y credencial explícitos) | `internal/assets/generic/remote-authorization-contract.md`; `docs/usage.md:236-238` | Para C3 (acción irreversible) |
| **Permisos**: deny de rutas sensibles en Claude; reglas ask/deny en OpenCode | `docs/components.md:18` | — |
| **Backups y restore** automáticos: 5 snapshots, se pueden fijar, `gentle-ai restore` | `docs/rollback.md` | Seguridad al instalar |
| **Telemetría**: anónima, opt-out; registra modelo, esfuerzo seleccionado y efectivo, clase de agente y tokens | `docs/telemetry.md:15-60,520-558` | Desactivarla si interesa la privacidad |
| **Worktrees** para escritores en conflicto o en otros repos; **subagentes en background** en OpenCode (no durables) | `routing.go:96`; `docs/opencode-profiles.md` | C7 (agentes en paralelo) |
| **Agent Builder** (TUI) para generar skills multiagente | `PRD-AGENT-BUILDER.md`; `internal/agentbuilder/` | Alternativa para distribuir nuestras skills |
| **`gentle-ai doctor`** (diagnóstico de solo lectura) y **`sync --dry-run`** | `docs/usage.md:337-350` | Mantenimiento |
| **Canales** stable, beta y nightly | `internal/cli/install.go:30` | — |
| **CodeGraph** (índice estructural de código) para el explorador | `opencode/agents/gentle-ai-explore.md` | No aplica a documentos |
| **Perfiles de proyecto**: no existen como tales. Lo más parecido es `--scope workspace` y los overrides por clon de RDD. | — | No encontrado |

---

## 11. Qué no pude verificar

- El comportamiento real del modelo cuando conviven un CLAUDE.md de proyecto y el ODD global "MANDATORY". Hacer esa prueba exigiría ejecutar Claude Code.
- La lista exacta de archivos que quedan globales con `--scope workspace`.
- Si Claude Code hereda algún modelo por defecto configurable para su subagente general. Esto es comportamiento de Claude Code, no de gentle-ai.
- Todo lo de Pi y Gentle Shell (`gentle-pi`) se maneja fuera de este repositorio.
