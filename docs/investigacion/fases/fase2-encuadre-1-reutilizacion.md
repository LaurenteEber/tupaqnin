# Encuadre 1: reutilización máxima de gentle-ai instalado

*hamutay v1 como capa de proyecto encima de gentle-ai v4 (sin fork) · 7 oct 2026*

**Idea central:** no peleamos con ODD; lo **reinterpretamos** para documentos. El documento de feature de ODD (`S#/T#/Log`) pasa a ser el **encargo**. Los criterios y las decisiones viven en un expediente Markdown que los `S#` citan por ID. Todo lo propio va en `<proyecto>/`, con prefijo `hamutay-`. Nada se instala en los marcadores `gentle-ai:`.

---

## 1. Principios

1. **Glosario, no contraorden.** El `CLAUDE.md` del proyecto no dice "ignora ODD". Redefine sus términos: "test" significa lente de documento, "test-first" significa criterio primero, y la línea `Risk:` usa ítems de documento. Dos reglas MANDATORY en conflicto dependen del modelo para resolverse; un glosario no entra en conflicto (gentle-ai-tecnico §9.2).
2. **El expediente es la fuente de verdad y Engram es el índice.** Cada criterio y cada decisión es un archivo en git; Engram solo guarda un puntero con `topic_key` igual al ID (engram §7.2).
3. **El humano escribe primero en los momentos que definen el trabajo.** La regla de ODD y de la persona ("una pregunta y detenerse") encaja con M12: la pregunta es "escribe tu encuadre". El agente no propone nada antes.
4. **Reusar antes de construir.** Usamos Judgment Day, Lossless Blocking Prompts, el traspaso por referencia, el veredicto por `S#`, el espejo y la reanudación con Engram, y `cognitive-doc-design`. Solo construimos los huecos: el criterio vivo, la comprensión y la propagación.
5. **Proporcionalidad de ODD.** Una edición pequeña no crea expediente. Las compuertas se activan por riesgo, no por fase.

---

## 2. Componentes

### 2.1 Instrucción: `<proyecto>/CLAUDE.md`, fuera de los marcadores

Tiene unas 60 líneas en seis bloques:

- **Idioma:** "Todos los artefactos de este repo se escriben en español, registro profesional." Es el pedido explícito que exige el Language Contract.
- **Glosario ODD → documentos:**

| ODD | En este repo |
|---|---|
| Feature document `odd/tasks/<f>.md` | Encargo de un documento |
| `S#` | Requisito literal del humano o `Cumple C-###` |
| `T#` | Sección o entregable |
| Test / RED→GREEN | Lente aplicada / veredicto por `S#` |
| `## Verification` | Lentes a correr (L1–L4) |
| Commit de unidad | Commit por sección, en la rama `doc/<f>` |

- **Riesgo de documento** (reemplaza los seis ítems de código y conserva el formato `Risk: item N`):
  1. Cambia una decisión o un criterio confirmado.
  2. Introduce cifras, normas o citas.
  3. Toca una sección que citan otros documentos.
  4. Hay fragmentos redactados en paralelo.
  5. Es una versión que sale a terceros.
  6. Tiene afirmaciones sin fuente verificable.
- **Rutas:** la tabla de §2.2 y §2.3, con los disparadores de cada skill.
- **Excepción de persona:** en exploración y en decisiones clave, presentar opciones es una "bifurcación real" y se permite un menú. El detalle va a archivos; el chat sigue corto.
- **Protocolo Engram propio** (§5).

### 2.2 Skills: `<proyecto>/.claude/skills/hamutay-*/SKILL.md`

Siguen la guía de gentle-ai: cuerpo de 180–450 tokens y detalle en `references/`. El prefijo evita colisiones con skills incluidas y con `sdd-*` (§3.3).

| Skill | Disparador | Hace | Mecanismos |
|---|---|---|---|
| `hamutay-encuadre` | documento nuevo o versión mayor | Pide al humano 5–10 líneas de encuadre e ideas, sin mostrar nada antes. Luego entrevista con casos límite, una pregunta por turno. Crea `odd/tasks/<f>.md`: `L1` es el encuadre literal y los `S#` salen de él. | M12, M02, M01 |
| `hamutay-explorar` | pregunta abierta nombrada en un `S#` | El humano lista primero sus dimensiones; el agente agrega las suyas, marcadas. Delega variantes a `hamutay-explorador` y arma una tabla comparativa de opciones × dimensiones. Pide reacciones breves por opción. | M10, M11, M09, M04 |
| `hamutay-criterio` | reacción, edición humana, hallazgo de juicio, o "no me gusta" | Propone `C-###` con la cita literal. El humano confirma en lote (G2). Después: Log literal → reescribe solo el `S#` afectado → reabre su `T#`. Lo mismo para `D-###` (decisión/descarte). | M05, M06, M15; **H1, H3, H4** |
| `hamutay-juicio` | riesgo con ítem 1, 2 o 5, o pedido explícito | Judgment Day para documentos (ver §5). Ledger en `expediente/juicios/`. | M31, M32, M36 |
| `hamutay-propagar` | cambio de un `C-###`/`D-###` confirmado o del ítem 3 | Delega a `hamutay-propagador`. Produce un diff semántico ("la premisa X cambió → secciones Y, Z y otro documento W") y un `T#` reabierto por sección afectada. | M26, M19; **C2, H6** |
| `hamutay-retomar` | sesión nueva o "¿en qué estábamos?" | Arma la reentrada desde git y el expediente (no desde Engram). Al cerrar un hito, hace teach-back de las decisiones cambiadas. | M21, M15, M23; **H2, H5** |

### 2.3 Subagentes: `<proyecto>/.claude/agents/`, con modelo y esfuerzo propios

El orquestador de gentle-ai no los conoce. Las skills los nombran explícitamente y les pasan el contexto por referencia (`Spec: odd/tasks/<f>.md, T3, S2-S5`).

| Agente | `model` / `effort` | `tools` | Rol |
|---|---|---|---|
| `hamutay-explorador` | `sonnet` / `medium` | Read, Glob, Grep, WebSearch, Write (solo `exploracion/`) | 3–5 variantes divergentes por dimensión, ≤300 palabras cada una |
| `hamutay-redactor` | `sonnet` / `high` | Read, Edit, Write (superficies permitidas) | Redacta un `T#`; carga `cognitive-doc-design`; marca incertidumbre (M43) e informa qué `S#` cubrió |
| `hamutay-verificador` | `opus` / `high` | Read, Glob, Grep, WebFetch | Veredicto por `S#` y por afirmación (M27): `ok`, `sin_fuente` o `contradice C-###/D-###`; nunca edita |
| `hamutay-propagador` | `sonnet` / `high` | Read, Glob, Grep | Mapa de impacto de un cambio de premisa; devuelve `archivo#sección` y el motivo |
| `hamutay-examinador` | `sonnet` / `low` | Read | Genera 3–5 preguntas de defensa sobre los `D-###`/`C-###` cambiados; corrige las respuestas del humano |

**Reusados sin cambios:** `jd-judge-a`, `jd-judge-b` y `jd-fix-agent`, con el modelo que asigna gentle-ai. Recomendación: en la TUI, modo Custom, `jd-judge-a = opus/high` y `jd-judge-b = sonnet/high`. El preset *diversity* pone haiku, que es débil para juzgar prosa.

### 2.4 Hook (opcional, en `<proyecto>/.claude/settings.json`)

Un solo hook `SessionStart` que imprime las 10 líneas de `expediente/estado.md`: última visita, decisiones pendientes y deuda de comprensión. gentle-ai no toca el `settings.json` del proyecto con alcance global.

### 2.5 Plantillas

`hamutay-*/assets/`: `criterio.md`, `decision.md`, `juicio.md`, `encargo.md` (el esqueleto ODD con el glosario).

---

## 3. Flujo proporcional

- **Pequeño:** una edición puntual que puede retomarse desde `git diff`. Va en línea, según ODD. Solo si emerge un criterio se llama a `hamutay-criterio`; un criterio es un archivo de 10 líneas.
- **Grande:** un documento o una versión. La prueba de reanudación de ODD lo marca como grande casi siempre.

| Fase | Quién genera | Compuerta |
|---|---|---|
| 1. Encuadre | **Humano** escribe; el agente entrevista | **G1**: no hay `S#` sin `L1` escrito por el humano |
| 2. Exploración (solo si un `S#` dice "abierto") | El humano da sus dimensiones; el agente diverge; el humano reacciona | — |
| 3. Criterios | Emergen de las reacciones; el agente propone | **G2**: confirmación en lote de `C-###` (envelope lossless, multiselección) |
| 4. Construcción por `T#` | El agente redacta; el humano decide los `D-###` clave | **G3**: decisión clave como menú M11 (opciones × criterios) |
| 5. Verificación según el riesgo | Verificador; `hamutay-juicio` si hay ítem 1, 2 o 5 | **G4**: solo ante ESCALATED o contradicción entre jueces |
| 6. Cierre | El **humano** explica (teach-back) | **G5**: bloquea solo con ítem 5 (sale a terceros); si no, solo registra la deuda |

Las flechas de retorno del acervo (§2) se implementan con la regla de corrección de ODD: un cambio de criterio agrega una entrada al Log, reescribe un `S#`, reabre un `T#` y dispara `hamutay-propagar`.

---

## 4. Artefactos

```
<proyecto>/
├── CLAUDE.md                     # capa hamutay (fuera de marcadores gentle-ai)
├── .claude/{skills,agents}/hamutay-*
├── .engram/config.json           # engram init <proyecto>
├── odd/tasks/<doc>.md            # ENCARGO en formato ODD nativo
├── docs/<doc>.md                 # el producto (p. ej. metodologia-syee-v7.md)
├── exploracion/<tema>/{variantes.md, comparacion.md}
└── expediente/
    ├── estado.md                 # reentrada: ultima_visita, pendientes, deuda
    ├── criterios/C-007-umbral-cobertura.md
    ├── decisiones/D-012-tipologia-indicadores.md   # incluye descartes
    ├── juicios/J-2026-10-07-doc-T4.md               # ledger JD
    └── comprension.md
```

**Encargo (ODD):**
```
## Specs
S1. "La metodología debe poder aplicarla un sectorista sin capacitación" (L1)
S4. Cumple C-007 (expediente/criterios/C-007-…md, v2)
## Tasks
- [x] T3 (S1,S4) delegado hamutay-redactor · docs/met-v7.md#3 · L1,L2 · a1b2c3
## Log
L1 2026-10-07 user (verbatim): > …
L7 2026-10-09 criterio C-007 v1→v2: > "no, 80 % es demasiado para gobiernos locales"
```

**Frontmatter de `C-###`:** `id`, `version`, `estado` (`propuesto | confirmado | retirado | reemplazado`), `origen` (`encuadre | reaccion | edicion | juicio`), `cita_humana` (literal), `confirmado` (fecha + commit del humano), `reemplaza`, `specs: [S4]`, `aplica_a: [docs/x.md#3]`, `contraejemplo`, `senal_contraria` (qué haría revisarlo).

**`D-###`:** igual que `C-###`, más `opciones`, `elegida`, `descartes` (cada uno con su razón) y `autor` (`humano | agente-propuesto`).

**`comprension.md`:** una tabla con `id | version_explicada | resultado (ok/parcial/no) | fecha`. La **deuda** es el número de `C-###`/`D-###` cuya versión actual es mayor que la explicada (H2). Es medible con un `grep`.

---

## 5. Relación con gentle-ai y Engram

| Pieza | Qué hacemos |
|---|---|
| **ODD** | **Se usa.** El documento de feature es el encargo; la regla del `S#` afectado nos da H4 sin costo; se conservan los estados públicos (Working, Checking, Ready, Needs your decision). Se reinterpreta: test, Verification, Risk y la rama. |
| **Judgment Day** | **Se reusan sus agentes y su protocolo**, con skill propia (`hamutay-juicio`, no `judgment-day`, para que `sync` no la sobrescriba). El prompt delegado reemplaza los criterios de código por lentes de documento: **L1** coherencia argumental, **L2** trazabilidad a fuentes (M27), **L3** consistencia con `C-###`/`D-###` confirmados, **L4** legibilidad para el lector destino (C12). `location` = `archivo#sección:línea`. Se agrega el campo `criterio: C-###` al ledger (H3). Las reglas se conservan: solo se corrige lo que confirman ambos jueces, máximo dos rondas y veredicto APPROVED o ESCALATED. El fix-agent recibe "test: N/A; evidencia = cita de la fuente". |
| **Lossless Blocking Prompts** | **Se usan tal cual** en G2 a G4. El examinador y los jueces devuelven envelopes y el orquestador los releva sin resumir. |
| **Traspaso por referencia, `Allowed edit surfaces`, `Key Learnings`** | Se usan tal cual. |
| **`cognitive-doc-design`** | La carga el redactor. |
| **Persona** | Se queda *gentleman* (afecta también al trabajo de código), con la excepción de §2.1. Plan B: `--persona custom` si la prueba falla. |
| **RDD** | **Se apaga** con `gentle-ai review mode disable --scope clone` en el repo de documentos. Además los `.claude/` serían "operativos" y dispararían consentimientos. |
| **Telemetría** | Opcional: `DO_NOT_TRACK=1` para documentos sensibles. |
| **Engram** | **Índice.** Protocolo propio en `CLAUDE.md`: (1) primero el archivo, después `mem_save` con `topic_key = hamutay/criterio/C-007`, `type = policy` (revisión a 12 meses) o `decision` (6 meses), y `content` = 2 líneas + ruta. (2) Ante un `judgment_required` de tipo `supersedes` o `conflicts_with` sobre `policy` o `decision`: **siempre** se presenta un envelope al humano y el resultado se escribe en el expediente; nunca se resuelve en silencio. (3) Se mantiene el espejo nativo `odd/<doc>/tasks`. (4) No se usa para la comprensión. (5) Va `<private>` en los fragmentos sensibles. |

---

## 6. Huecos cubiertos

| Hueco o necesidad | Cómo |
|---|---|
| **H1/H4** criterio → rúbrica → spec | `hamutay-criterio`: la reacción se vuelve `C-###` confirmado, que pasa a un `S#` reescrito según la regla de ODD y que el verificador y la lente L3 aplican |
| **H3** criterio auditable | `cita_humana` + git log por `C-###` + `criterio:` en el ledger de juicio + `aplica_a` |
| **H2** comprensión medible | Deuda calculada en `comprension.md` y teach-back con `hamutay-examinador` |
| **H5** reentrada humana | `hamutay-retomar`: desde `ultima_visita` lista los `D/C` cambiados, quién los cambió, los pendientes de G2/G3 y lo que el humano no ha explicado |
| Co-generación | G1 + dimensiones del humano primero (M12) + autor en `D-###` |
| **C1** exploración | Tabla de opciones × dimensiones; el humano reacciona en lugar de leer todas las variantes |
| Verificación sin tests | Riesgo de documento en 6 ítems → verificador por afirmación + JD con lentes L1–L4 |
| **C2** propagación | `hamutay-propagar` reabre `T#` en este documento y en los demás |
| **H6** contradicciones | Parcial: L3 + propagador; los conflictos léxicos de Engram como señal temprana |
| **H7, H8** | No se cubren |

---

## 7. Riesgos, costos y decisiones abiertas

**Riesgos**
1. **No verificado: precedencia real.** Que el modelo respete el glosario frente a un ODD "MANDATORY" (por ejemplo, que no exija tests ni RED→GREEN) no se probó. *Mitigación:* prueba de humo con 5 pedidos tipo antes de seguir.
2. **Acoplamiento a una base volátil.** gentle-ai retiró SDD en v4; si cambia el formato `S#/T#/Log` o la ruta `odd/tasks/`, se rompe el mapeo. *Mitigación:* fijar la versión; el expediente no depende de `odd/`, así que solo se pierde el encargo.
3. **Agentes `jd-*` con sesgo a código.** Su prompt de sistema dice "Assume the code has bugs". *Plan B barato:* `hamutay-juez-a/b` propios (dos archivos).
4. **Ajustes globales compartidos.** Cambiar el modelo de `jd-*` o la persona afecta también al trabajo de código.
5. **Deriva entre expediente y Engram.** Las escrituras no son atómicas. *Mitigación:* `hamutay-retomar` reconcilia y gana el archivo.
6. **Repo público.** Exige a los usuarios tener gentle-ai, Engram y Claude Code. Esto achica la audiencia; es un costo del encuadre.

**Costos:** unas 60 líneas de `CLAUDE.md`, 6 skills, 5 agentes y 4 plantillas. JD con opus duplica el costo de la revisión, por eso se limita a los ítems de riesgo 1, 2 y 5.

**Decisiones abiertas**
- ¿Basta con la excepción de persona o hace falta `custom`?
- ¿Reusar `jd-*` o tener jueces propios desde el día uno?
- ¿G5 bloqueante solo hacia terceros, o siempre?
- ¿`hamutay-init` (una skill que copia `.claude/` y el `CLAUDE.md` en un proyecto) como mecanismo de distribución?
- Piloto: ¿GEE v6 → v7 entero o un solo capítulo?
