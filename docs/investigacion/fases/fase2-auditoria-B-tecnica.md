# Auditoría B: verificación técnica de arquitectura-v1

*Fuentes revisadas el 2026-10-07: docs oficiales de Claude Code (CLI local v2.1.293), docs de OpenCode, gentle-ai @eb81278 y engram @8e525e7, ambos en solo lectura. Las rutas de los repos son relativas a `scratchpad/repos/`.*

## 1. Supuestos marcados [por probar]

| # | Supuesto | Veredicto | Fuente | Implicación |
|---|---|---|---|---|
| 1 | `omitClaudeMd` o algo equivalente aísla a los subagentes | **VERIFICADO** | code.claude.com/docs/en/sub-agents (frontmatter): `omitClaudeMd: true` lanza el subagente "without the user, project, and local CLAUDE.md files". Los CLAUDE.md gestionados sí cargan. Requiere v2.1.271 o posterior; la local es 2.1.293. | Sirve en los dos modos, pero **quita también el CLAUDE.md del proyecto y AGENTS.md** ("subagents that skip project instructions skip these files too", memory#agents-md). Un `hm-*` con `omitClaudeMd` pierde las reglas de hamutay, así que hay que precargarlas con `skills:` o ponerlas en el cuerpo del agente. |
| 2 | `model` se puede pasar al invocar | **VERIFICADO** | sub-agents, "Model resolution order": 1) parámetro `model` de la invocación, 2) frontmatter, 3) `CLAUDE_CODE_SUBAGENT_MODEL`, 4) modelo principal. Con `CLAUDE_CODE_SUBAGENT_MODEL_FORCE=1` la variable de entorno gana a todo. | La escalada por riesgo funciona. Hay dos riesgos: si el usuario tiene `..._FORCE=1`, los presets dejan de funcionar sin avisar, y antes de v2.1.251 la variable de entorno ganaba. Conviene fijar una versión mínima. La escalada se puede auditar con la regla de permisos `Agent(model:opus)` (permissions). |
| 3 | El esfuerzo solo cambia mediante una variante de archivo | **VERIFICADO** (no hay parámetro `effort` en la invocación) | sub-agents: `effort` existe solo en el frontmatter y "overrides the session effort level, but not `CLAUDE_CODE_EFFORT_LEVEL`". | Las variantes de archivo están bien. Hay que documentar que `CLAUDE_CODE_EFFORT_LEVEL` las anula todas. |
| 4 | Haiku acepta el esfuerzo | **VERIFICADO para Haiku 5.5**, FALSO para Haiku anteriores | model-config: "Opus 5.5, Sonnet 5.5, Haiku 5.5 … low…max"; "Models not listed here do not support effort". El alias `haiku` resuelve a 5.5 en la API de Anthropic, pero a un modelo 4.5 en Microsoft Foundry. | `hm-explorador` en modo económico (haiku/low) funciona en la API de Anthropic. Con otros proveedores, el esfuerzo se ignora o se degrada. Si un nivel no está soportado, Claude Code baja al mayor nivel soportado. |
| 5 | El glosario del proyecto se impone al ODD obligatorio | **NO VERIFICABLE por documentación**, y la evidencia es desfavorable | memory: el CLAUDE.md de usuario y el de proyecto cargan juntos y "Neither set overrides the other: … Claude may follow either one" (sección rules, mismo principio). gentle-ai inyecta ODD en `~/.claude/CLAUDE.md` (`internal/agents/claude/adapter.go:145`) entre marcadores `<!-- gentle-ai:… -->` (`internal/components/filemerge/section.go:23`). | La prueba de humo sigue siendo necesaria, pero **no hace falta apostar por la precedencia**: `claudeMdExcludes` (ver 2.1) puede quitar el CLAUDE.md global en el repo de documentos. El modo acoplado queda opcional, no necesario. |

## 2. Otras afirmaciones técnicas

### 2.1 Memoria e instrucciones (Claude Code)

- **`@AGENTS.md` en CLAUDE.md: VERIFICADO.** Funciona por import, con rutas relativas al archivo que importa y profundidad máxima de 4 saltos (memory, "Import additional files"). Matiz: desde v2.1.277, Claude Code lee `AGENTS.md` de forma nativa solo si **no** hay CLAUDE.md. Si existe CLAUDE.md, se lee únicamente CLAUDE.md, salvo que este importe AGENTS.md (memory, tabla AGENTS.md). El diseño (CLAUDE.md = `@AGENTS.md` más el glosario) es correcto. La alternativa más limpia es no tener CLAUDE.md y dejar solo AGENTS.md, pero entonces no hay dónde poner el glosario del modo acoplado.
- **Excluir el CLAUDE.md global: VERIFICADO, no se contempló en el diseño.** `claudeMdExcludes` acepta globs sobre rutas absolutas, "at any settings layer: user, project, local". Solo los CLAUDE.md gestionados no se pueden excluir. Un `.claude/settings.json` del repo de documentos con `"claudeMdExcludes": ["**/.claude/CLAUDE.md"]` (o la ruta absoluta del home) elimina el ODD de gentle-ai **en toda la sesión, incluidos los subagentes**, sin desinstalar nada. Nota: la documentación lo ilustra con monorepos; aplicarlo a `~/.claude/CLAUDE.md` se deduce de "Patterns are matched against absolute file paths" y queda por confirmar en una prueba de 1 minuto con `/memory`.
- **Selector «Project instructions»** (`claude-md-and-agents-md`, `managed-only`…): existe, pero solo se lee desde `~/.claude/settings.json` o desde configuración gestionada, no desde la del proyecto. No sirve para un ajuste por repo.

### 2.2 Subagentes

- **Campos `model`, `effort` y `tools`: VERIFICADOS.** También existen `skills` (precarga el contenido completo), `hooks`, `memory`, `permissionMode`, `maxTurns`, `disallowedTools` e `isolation: worktree`.
- **"Herramientas: lectura y escritura en `expediente/`" o "en `doc/`": FALSO tal como está escrito.** `tools` solo acepta nombres de herramienta, no rutas (sub-agents). Una regla `Write(path)` "is accepted but never consulted"; hay que usar `Edit(path)` (permissions:343). Las reglas de permisos se definen por settings, no por subagente. **Cambio:** restringir rutas con un hook `PreToolUse` en el frontmatter del subagente, con `matcher: "Write|Edit"` y un script que salga con código 2 si la ruta está fuera de `expediente/` o `doc/`.
- **Anidamiento:** por defecto un subagente puede lanzar otros, hasta 3 niveles. Para los jueces conviene quitar `Agent` de `tools` y conservar así la ceguera y el costo acotado.
- **`hm-juez-b` "no ve la conversación": VERIFICADO en parte.** Ningún subagente sin fork ve el historial. Sí recibe CLAUDE.md, el snapshot de git status y la lista de agentes hermanos. Para que de verdad no vea las decisiones, necesita `omitClaudeMd: true` y no debe tener `Read` sobre `expediente/`: se puede poner un deny `Read(./expediente/**)` mediante un hook, porque el permiso no es por agente.

### 2.3 Skills

- **`.claude/skills/<nombre>/SKILL.md`: VERIFICADO.** En Claude Code ningún campo es obligatorio y `description` es el recomendado (docs/skills). **En OpenCode, `name` y `description` sí son obligatorios**, y `name` debe coincidir con el directorio y cumplir `^[a-z0-9]+(-[a-z0-9]+)*$`. `hamutay-encuadre` lo cumple. OpenCode también lee `.claude/skills/` (opencode.ai/docs/skills), así que el núcleo portable es viable si cada skill declara `name`.
- Campos útiles que no se aprovechan: `context: fork` con `agent: hm-…`, para que una skill corra dentro de un subagente concreto; `disable-model-invocation: true`, para que `hamutay-cerrar` y `hamutay-retomar` solo arranquen cuando el humano las invoca; y `hooks` por skill.

### 2.4 OpenCode

- **Agentes:** `.opencode/agents/` o `opencode.json`, con `model`, `mode: subagent`, `permission` y `prompt`. `reasoningEffort` se pasa tal cual al proveedor y su efecto depende del proveedor (opencode.ai/docs/agents). Las variantes de modelo existen (Anthropic: `high` y `max`, docs/models), pero **la documentación no recoge un campo `variant` por agente**. gentle-ai sí lo escribe en las entradas de agente (`internal/components/opencodeagents/agents.go:122`), lo que indica soporte de facto. Queda **NO VERIFICABLE en fuente oficial**. **No hay `model` al invocar** en OpenCode: la escalada por riesgo necesita agentes duplicados (`hm-redactor-opus`).
- **AGENTS.md:** es lo primero que se lee. `CLAUDE.md` solo se usa como respaldo si no hay AGENTS.md, y `~/.claude/CLAUDE.md` como respaldo global salvo que se fije `OPENCODE_DISABLE_CLAUDE_CODE_PROMPT` (docs/rules). En OpenCode, el aislamiento frente al ODD de Claude se consigue con esa variable de entorno.

### 2.5 gentle-ai

- **Inyección de ODD:** el orquestador va en `~/.claude/CLAUDE.md` con secciones gestionadas `{{GENTLE_AI_ODD_SECTION:…}}` (`internal/assets/claude/orchestrator.md:55-125`). **No existe exclusión de ODD por proyecto** en el código: no hay referencias a `claudeMdExcludes` ni opt-out por repo. La única vía es `claudeMdExcludes`, del lado de Claude Code.
- **`--scope workspace`: VERIFICADO con una salvedad importante.** Escribe los archivos del agente en la raíz del proyecto (`internal/cli/scope.go:50`; docs/usage.md:386). Pero **un install global previo sigue vivo**: workspace no limpia `~/.claude/CLAUDE.md`. Además, para Claude Code, "a workspace sync delivers no routing guidance" (`internal/cli/sync.go:989-997`, `run.go:1052`). El modo aislado exige que **no haya** install global, o que se use `claudeMdExcludes`.
- **`review mode disable --scope clone`: VERIFICADO** (`internal/cli/review_mode.go:27,61`; `review_narration.go:104`). Es local al clon, requiere `--cwd` en un repo git y "a clone-local override can only disable" (`review_mode.go:509`). Omitir `--scope` lo apaga en toda la máquina (`review_narration.go:111`).
- **`--persona custom`: VERIFICADO.** No instala persona ("Custom persona does nothing — user keeps their own config", `internal/components/persona/inject.go:124-126`; `model/presets.go:38`). El ODD y el ruteo siguen inyectándose porque son independientes de la persona.
- **Judgment Day reutilizable: VERIFICADO** como skill independiente, sin la maquinaria de recibos (`internal/assets/skills/judgment-day/SKILL.md`). **Diferencias con hamutay:** sus criterios de juicio son de código (correctness, security…, `references/prompts-and-formats.md`), los jueces solo tienen `Read, Glob, Grep` más engram (`internal/assets/claude/agents/jd-judge-a.md:7-9`) y permite **hasta 2 rondas de corrección**, no una sola. Para reutilizarlo hay que sustituir las lentes y aceptar ese tope, o copiar el patrón en lugar del skill. Los dos jueces comparten modelo configurable (`{{CLAUDE_MODEL}}`).
- **Lossless Blocking Prompts: VERIFICADO** (`internal/assets/claude/orchestrator.md:13-19`). Solo existe dentro del orquestador global, no como skill aparte: para usarlo en modo aislado hay que copiar el texto.
- **Los cuatro estados públicos y "una ronda y escalar": VERIFICADOS** (`docs/trigger-rules.md:21`; `_shared/odd-orchestrator-sections.md:20,45`). Matiz: ODD permite una **segunda** corrección si el mismo bloqueo persiste.

### 2.6 Engram

- **`decision` y `policy` con `review_after`: VERIFICADO.** El decaimiento es de 6 meses para `decision`, 12 para `policy` y 3 para `preference` (`internal/store/store.go:387-411`). El estado `needs_review` se deriva de ahí (`store.go:185-200`). **Salvedad:** `review_after` solo se fija **en inserciones nuevas, no en revisiones por `topic_key`** (`store.go:4044-4047`). Así, un CRI que se actualiza con el mismo `topic_key` conserva el reloj del original. Además `policy` no aparece en la lista de tipos de `mem_save` (`internal/mcp/mcp.go:528`), aunque el store la acepta.
- **`topic_key` hace upsert: VERIFICADO** ("Reuses and updates the latest observation in same project+scope", `mcp.go:536-537`). El formato `hm/<ID>` sirve.
- **Regla de llevar al humano `supersedes` y `conflicts_with`:** ya es el comportamiento por defecto de engram para los tipos `architecture`, `policy` y `decision` (`docs/AGENT-SETUP.md:891`). La regla propia es redundante, aunque no hace daño.

## 3. Capacidades oficiales que el diseño no aprovecha

1. **Hooks como compuertas deterministas.** Hoy todo depende de instrucciones.
   - `PreToolUse` (bloquea con exit 2 o `permissionDecision: deny`): impide escribir en `ideas/*-humano.md`, que debe ser inmutable de verdad; impide que el agente escriba `estado: confirmado` o `confirmado_por` en CRI y DEC. Esto **cierra el riesgo "confirmaciones que el agente puede escribir solo"** del §6. También restringe rutas por subagente.
   - `SubagentStop` (puede bloquear y recibe `last_assistant_message`): valida que la salida de `hm-redactor` o de los jueces tenga el formato exigido (veredicto por `S#`, bloque Contrapeso). Si falla, la devuelve.
   - `Stop`: ejecuta `hm check` y bloquea el cierre si hay IDs o enlaces rotos.
   - `SessionStart` (inyecta `additionalContext`; `source` = `resume`, `compact`…): dispara `hamutay-retomar` y `hm vistas` de forma automática.
   - `UserPromptSubmit`: registra la tasa de aceptación para la regla del 85 %.
   - `PreCompact`: guarda el estado de la sesión en `sesiones/SES-*.md`.
   - Fuente: code.claude.com/docs/en/hooks.
2. **Hooks tipo `prompt` o `agent`:** un modelo puede decidir sí o no sin código propio, por ejemplo "¿esta sección cita un CRI confirmado?".
3. **Plugin de Claude Code:** puede empaquetar skills, agentes, hooks y output style en una unidad instalable. Resuelve el "costo de mantenimiento" y la distribución al repo público.
4. **Output style personalizado sin `keep-coding-instructions`:** retira las instrucciones de ingeniería de software del prompt de sistema del orquestador, que es pertinente para trabajo documental. No llega a subagentes que no sean forks (output-styles).
5. **`memory: project` en subagentes:** `hm-destilador` y `hm-coherencia` pueden aprender entre sesiones sin depender de Engram.
6. **OpenCode:** los plugins con `tool.execute.before` (bloqueo por excepción) y `experimental.session.compacting` (opencode.ai/docs/plugins) son el equivalente parcial de los hooks.

## 4. Cambios que exige el diseño

1. Sustituir las "herramientas con ruta" por hooks `PreToolUse` y llevar al hook la inmutabilidad de `ideas/` y de las confirmaciones.
2. Añadir `claudeMdExcludes` como mecanismo principal del modo aislado. Advertir que `--scope workspace` no basta si existe un install global.
3. Para los subagentes con `omitClaudeMd`, precargar las reglas de hamutay con `skills:`, porque también pierden AGENTS.md.
4. Escalada de modelo: en Claude Code, por parámetro; en OpenCode, con agentes duplicados. Advertir sobre `CLAUDE_CODE_SUBAGENT_MODEL_FORCE` y `CLAUDE_CODE_EFFORT_LEVEL`.
5. Declarar `name` en cada SKILL.md para que funcione en OpenCode.
6. Judgment Day: decidir si se acepta su tope de 2 rondas y lentes de código, o si se copia solo el patrón.
7. Engram: al cambiar la versión de un CRI, hay que llamar a `mem_review` o insertarlo de nuevo para reiniciar `review_after`.
