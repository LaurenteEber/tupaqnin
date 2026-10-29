# Encuadre 2: independiente y portable

*Fase 2 · 7-oct-2026 · Arquitectura v1 de `hamutay` sin depender de gentle-ai instalado*

## 1. Principios

1. **El expediente es el producto; el harness es solo texto.** Todo es Markdown con frontmatter YAML en git. No hay binario, servidor ni base de datos obligatoria (engram.md §7.2; M15, M16).
2. **Una fuente y varias proyecciones.** `AGENTS.md` es la instrucción canónica: OpenCode lo lee de forma nativa y Claude Code lo recibe mediante `@AGENTS.md`. Las skills viven en `.claude/skills/`, que ambos hosts descubren. Los subagentes se declaran una vez en YAML y se proyectan a cada host.
3. **Patrones, no dependencias.** Tomamos de gentle-ai cuatro patrones, reescritos con texto propio:
   - de ODD, el formato `S#/T#/Log` y "reescribir solo el `S#` afectado";
   - el traspaso por referencia;
   - de Judgment Day, los jueces ciegos con consenso y dos rondas;
   - los Lossless Blocking Prompts.

   No heredamos sus supuestos de código: tests, commit por tarea ni `git diff` como prueba de reanudación.
4. **Ceremonia proporcional.** Hay tres tamaños: edición, sección y versión. Solo una versión recorre todas las fases.
5. **Funciona con o sin gentle-ai.** Si gentle-ai está instalado, el proyecto declara precedencia; si no, no falta nada.

## 2. Componentes

### Instrucciones

| Archivo | Contenido |
|---|---|
| `AGENTS.md` (≤150 líneas) | Idioma de los artefactos (es-PE, explícito), quién genera en cada fase, reglas del expediente, compuertas, niveles de riesgo y bloque de precedencia (§5) |
| `CLAUDE.md` (≤20 líneas) | `@AGENTS.md` y nada más que lo propio de Claude |

`CLAUDE.md` hace falta porque, si existe uno, Claude Code deja de leer `AGENTS.md` por sí solo. La importación evita duplicar contenido.

### Skills

Cada skill está en `.claude/skills/<n>/SKILL.md`, con cuerpo de 180 a 450 tokens y los detalles en `references/`. El `name` va en minúsculas e igual al nombre del directorio, como exige OpenCode. El prefijo `hmt-` evita colisiones con las skills de gentle-ai, que `sync` sobrescribe.

| Skill | Qué hace | Mecanismos |
|---|---|---|
| `hmt-encuadre` | El humano escribe primero 5 líneas: problema, propósito, destinatario, qué no es y qué decisión debe habilitar el documento. Luego el agente lo entrevista con casos límite y escribe `encargo.md` | M12, M02, M01 |
| `hmt-explorar` | No hay variantes hasta que exista `ideas-humano.md`. Después: dimensiones, variantes divergentes, una matriz de una página y el registro de reacciones | M12, M10, M11, M09 |
| `hmt-registrar` | Convierte reacciones, ediciones o decisiones en criterios (`C-`), decisiones (`D-`) o descartes (`X-`). Reescribe los `S#` afectados y anota en el `Log` | M04, M05, M06, M15 |
| `hmt-juicio` | Revisión por nivel de riesgo, con lentes de documento, jueces ciegos y ledger | M31, M27, M43 |
| `hmt-impacto` | Diff semántico entre etiquetas: qué premisa cambió y qué `S#`, `C-`, secciones y documentos hermanos toca | M19, M26 |
| `hmt-reentrada` | Briefing de una página: decisiones y criterios cambiados desde tu última visita, lo que nunca explicaste y la próxima compuerta | M21, M44 |
| `hmt-defensa` | Teach-back con 3 a 5 preguntas sobre las decisiones de más peso, respondidas sin ver el registro. Actualiza `comprension.md` | M23, M13 |

### Subagentes

La sesión principal orquesta con el modelo que elija el humano; se sugiere opus.

| Subagente | Rol | Herramientas | Modelo / esfuerzo |
|---|---|---|---|
| `hmt-explorador` | Variantes y matriz | Read, Glob, Grep, Write (solo `exploracion/`) | sonnet / medium |
| `hmt-redactor` | Redacta una sección acotada por `S#` y `C-`; corrige los hallazgos confirmados | Read, Edit, Write (solo las superficies permitidas) | sonnet / high |
| `hmt-juez-a` | Juez ciego | Read, Glob, Grep | opus / high |
| `hmt-juez-b` | Juez ciego | Read, Glob, Grep | sonnet / high, o un modelo de otro proveedor en OpenCode |
| `hmt-verificador` | Verifica cada afirmación contra `fuentes.md` | Read, Grep, WebFetch | sonnet / medium |
| `hmt-impacto` | Propaga cambios | Read, Grep, Bash (`git log/diff`) | sonnet / high |

Para el juez B no se usa haiku, a diferencia del preset *diversity* de gentle-ai. Juzgar prosa argumentativa exige razonamiento, y los errores de modelos parecidos se correlacionan (acervo §5.3). Por eso conviene diversificar el proveedor más que el tamaño del modelo.

### Modelo y esfuerzo por host

**Fuente única:** `hamutay/agentes.yaml`. Por cada rol guarda descripción, herramientas, modelo y esfuerzo por host, y ruta del prompt. El script `scripts/hmt-proyectar.py` (~80 líneas) genera las dos proyecciones. Ambas se commitean, así que el script no es una dependencia en tiempo de ejecución.

**Claude Code** (`.claude/agents/hmt-juez-a.md`):
```yaml
---
name: hmt-juez-a
description: Juez ciego de documentos. Solo desde la skill hmt-juicio.
tools: Read, Glob, Grep
model: opus      # alias, ID completo o "inherit"
effort: high     # low|medium|high|xhigh|max, según el modelo
---
```
- **Precedencia:** primero el parámetro de invocación, luego el frontmatter, después `CLAUDE_CODE_SUBAGENT_MODEL` y al final el modelo de la sesión.
- **Regla en `AGENTS.md`:** no pasar `model` al invocar un `hmt-*`, para que mande el frontmatter.
- **Validación en el script:** haiku no admite `effort` y sonnet no admite `xhigh` (misma lógica que `claude_model.go:70-83`).

**OpenCode** (`opencode.json` del proyecto):
```json
{ "agent": {
  "hamutay":    { "mode": "primary", "prompt": "{file:./hamutay/prompts/orquestador.md}" },
  "hmt-juez-a": { "mode": "subagent", "model": "anthropic/<opus>", "variant": "high",
                  "prompt": "{file:./hamutay/prompts/juez.md}",
                  "permission": { "edit": "deny", "bash": "deny" } } } }
```
- **Esfuerzo:** gentle-ai lo expresa con `variant` (`sync.go:1157-1180`). La documentación pública de OpenCode solo muestra opciones del proveedor como `reasoningEffort`, así que hay que verificarlo.
- **Herencia:** un subagente sin `model` hereda el del agente primario, por lo que todos los `hmt-*` lo fijan.

### Hooks y memoria

- **Hook:** un `pre-commit` de git (`scripts/hmt-check.py`), portable a cualquier host. Solo avisa, no bloquea. Revisa que:
  - los IDs sean únicos;
  - cada `S#` cite criterios que existen;
  - todo `C-` confirmado tenga `confirmado_por`.
- **Plantillas:** en `hamutay/plantillas/`.
- **Memoria:** el expediente.

## 3. Flujo proporcional

| Tamaño | Cuándo | Recorrido |
|---|---|---|
| **Edición** | Forma, sin cambio de argumento | Edición directa. Si la reacción revela un criterio, se registra como `propuesto` |
| **Sección** | Cambia el argumento de una sección | Redactor por `S#` → juicio según el riesgo → `Log` |
| **Versión** | v6 → v7, documento nuevo o cambio de premisa | Las siete fases del acervo §2, con cuatro compuertas |

### Cuándo genera el humano

1. **Encuadre:** escribe sus 5 líneas antes de la entrevista.
2. **Ideación:** sus ideas van antes de las variantes (M12, contra el anclaje de M10 y M11).
3. **Criterios:** el agente propone y solo el humano confirma.
4. **Decisiones:** toda decisión que cambia un `S#` es humana.
5. **Defensa:** responde antes de ver el registro.

### Compuertas

Funcionan como Lossless Blocking Prompts: opciones numeradas, sin resumir. En este repo los menús están permitidos.

| Compuerta | Condición |
|---|---|
| **G1 Encargo** | `encargo.md` aprobado |
| **G2 Criterios** | Los `C-` de la sección, confirmados antes de redactar |
| **G3 Juicio** | Veredicto `APROBADO` o `ESCALADO`, en dos rondas como máximo |
| **G4 Defensa** | Ninguna decisión de riesgo alto en estado `no-explicada` |

### Verificación sin tests

| Riesgo | Qué abarca | Revisión |
|---|---|---|
| Bajo | Forma | Sin juez |
| Medio | Argumento de una sección | Un juez con la lente de consistencia |
| Alto | Definiciones, metodología, cifras y recomendaciones | Dos jueces ciegos, verificador y todas las lentes |

**Lentes de documento:**
- coherencia argumental;
- trazabilidad de cada afirmación a una fuente;
- consistencia con los `C-` y `D-` vigentes;
- supuestos no declarados y señales contrarias (M43);
- legibilidad para el destinatario.

Solo se corrigen los BLOCKER y CRITICAL que confirman ambos jueces. Lo que marca un solo juez queda como *sospechoso* y lo revisa el humano.

## 4. Artefactos

```
<repo>/ AGENTS.md · CLAUDE.md · opencode.json
  .claude/skills/hmt-*/   .claude/agents/hmt-*.md (generados)
  doc/                    ← capítulos del documento
  hamutay/
    estado.md        ≤60 líneas: versión, fase, compuerta siguiente, pendientes
    encargo.md       # Specs (S# citan C-/D-) · # Tasks · # Log (cita textual y fecha)
    registro/        C-012-umbral-cobertura.md · D-007-….md · X-003-….md
    exploracion/E-02-tipologia/  ideas-humano · variantes · matriz · reacciones
    revisiones/R-2026-10-07-cap3.md   ledger: id, lente, ubicación, severidad, criterio_aplicado, evidencia, juez, estado
    comprension.md   objeto | peso | no-vista/leída/explicada | fecha | pregunta fallada
    fuentes.md · agentes.yaml · prompts/ · plantillas/
  scripts/hmt-proyectar.py · hmt-check.py
```

**Frontmatter de un criterio:**
```yaml
id: C-012
estado: propuesto | confirmado | retirado | reemplazado
origen: reaccion | edicion | decision | externo
evidencia: [exploracion/E-02/reacciones.md#r4]
afecta: [S3, doc/cap3.md#umbrales]
reemplaza: C-007
confirmado_por: Eber
fecha: 2026-10-07
señales_contrarias: ["umbral fijo penaliza entidades pequeñas"]
```

**Reglas del expediente:**
- Un objeto confirmado no se edita: se reemplaza.
- Los commits del agente llevan `Co-Authored-By` y las confirmaciones humanas llevan el trailer `Confirmado-por:`. Así `git log` separa lo que hizo la IA de lo que hizo el humano (M20, M42).
- Las versiones se marcan con etiquetas de git (`v7.0`).

## 5. Relación con gentle-ai y Engram

### Trade-off

| Eje | Montarse sobre gentle-ai | Independiente |
|---|---|---|
| Mantenimiento | Cada `upgrade` o `sync` puede cambiar el comportamiento; la v4.0 retiró SDD y `--strict-tdd` | Mantener 7 skills, 6 prompts y 2 scripts, pero los cambios los decidimos nosotros |
| ODD | Obligatorio en cada pedido, sin interruptor y pensado para código | Solo tomamos su formato y su regla de corrección |
| Persona | Una pregunta y ningún menú, lo que choca con M01, M02, M10 y M11. Además prohíbe `Co-Authored-By`, que nosotros usamos para la procedencia | Menús en las compuertas y entrevistas ricas |
| Idioma | Artefactos en inglés por defecto | Español declarado |
| Modelo por rol en Claude | Los roles ODD no se proyectan (§1.5 del informe técnico) | Lo fijamos en nuestro frontmatter |
| Revisión | RDD aplica cero lentes a los `.md` | `hmt-juicio` con lentes de documento |
| Repo público | Exige instalar un binario de terceros, con telemetría opt-out | `git clone` y funciona |
| Licencia y marcas | Dependencia de sus prompts y nombres | Reescribir, no copiar. Hay que verificar la LICENSE antes de citar. Solo atribución ("inspirado en ODD y Judgment Day"), sin nombres ajenos en skills ni en el README |

### Compatibilidad: tres modos

1. **Solo:** sin gentle-ai, todo funciona.
2. **Convivencia:** `AGENTS.md` incluye `## Precedencia en este repositorio`, que establece cuatro cosas:
   - `hamutay` reemplaza los pasos 4 a 7 de ODD;
   - no se usan `odd/tasks/`, ramas por feature ni test-first;
   - artefactos en español y menús permitidos;
   - Engram guarda solo punteros.

   Además se ejecuta `gentle-ai review mode disable --scope clone` y conviene usar `--persona custom`. En OpenCode el aislamiento es limpio, porque el agente primario `hamutay` tiene su propio prompt y ODD vive en el de `gentle-orchestrator`. En Claude, `~/.claude/CLAUDE.md` siempre se carga; hay que probar si `claudeMdExcludes` lo excluye.
3. **Aislado (recomendado):** gentle-ai se instala con `--scope workspace` solo en los repos de código.

### Engram

Es opcional y sirve solo como índice. Cada objeto se guarda con `topic_key = criterio/C-012-…` y un `content` que cita la ruta del archivo. Los conflictos que detecte sobre `C-` o `D-` se escalan siempre al humano.

## 6. Cobertura

| Hueco o necesidad | Cómo se cubre |
|---|---|
| **H1, H3, H4** | `C-` nace de una reacción con evidencia, el humano lo confirma, se versiona por reemplazo y alimenta el `S#` mediante `afecta`. El ledger registra `criterio_aplicado`, así que es auditable |
| **H2** | Deuda de comprensión = % de `C-` y `D-` de peso alto no explicados. Se mide con `hmt-defensa` y la exige G4 |
| **H5** | `hmt-reentrada` filtra los objetos humanos desde la última fecha registrada en `comprension.md` |
| **C1** | Ideas humanas primero, luego matriz de una página. Las reacciones se convierten en `C-` |
| **C2** | `hmt-impacto` recorre `afecta` y `reemplaza`, y los documentos hermanos declarados en `estado.md` |
| **Documentos sin tests** | Riesgo, lentes, dos jueces y verificación por afirmación |
| **H6, H7** | Parcial: lente de consistencia y párrafo de supuestos por cada cifra derivada |
| **H8** | Fuera de alcance |

## 7. Riesgos, costos y decisiones abiertas

### Riesgos

- **Instrucción sin garantía:** sin hooks del host, el agente puede saltarse compuertas. Se mitiga con `hmt-check` y `estado.md`; más adelante, con un hook `PreToolUse`.
- **Dos instrucciones "MANDATORY" compitiendo** en el modo convivencia. Hay que probarlo; si falla, pasar al modo aislado.
- **Deriva entre hosts:** semántica distinta de `variant`. Se mitiga con la fuente única `agentes.yaml`.
- **Correlación entre jueces de IA:** diversificar el proveedor y escalar lo que marque un solo juez.
- **Fricción de las forcing functions:** válvula "salto", siempre registrada en el `Log`.
- **Autoengaño en la comprensión:** `explicada` solo vale tras `hmt-defensa`.

**Costo:** de 2 a 3 días para la v1, más los tokens de opus como juez en riesgo alto.

### Decisiones abiertas

1. ¿Adoptar el modo aislado?
2. ¿Excluir `~/.claude/CLAUDE.md` con `claudeMdExcludes`?
3. ¿Cómo funcionan `variant` y `reasoningEffort` en la versión instalada de OpenCode?
4. ¿Juez B de otro proveedor?
5. ¿Documento y `hamutay/` en el mismo repo?

**Piloto:** GEE v6 → v7.
