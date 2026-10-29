# Encuadre 4 · Operación: roles, modelos, esfuerzo, contexto y costo

*7 oct 2026 · Arquitectura v1 de `hamutay` vista desde cómo se asigna trabajo a agentes.*

**Hechos verificados en docs oficiales (7 oct 2026):**
- **Subagentes de Claude Code** ([code.claude.com/docs/en/sub-agents](https://code.claude.com/docs/en/sub-agents)):
  - el frontmatter admite `model`, `effort`, `tools`, `disallowedTools`, `omitClaudeMd`, `maxTurns`, `isolation`, `background`;
  - el padre **puede pasar `model` en cada invocación**, y ese valor gana sobre el frontmatter;
  - **no existe un parámetro de esfuerzo por invocación**: el esfuerzo solo se fija en el frontmatter;
  - `omitClaudeMd: true` arranca el subagente sin los CLAUDE.md de usuario y proyecto (requiere v2.1.271 o posterior);
  - el límite por defecto es 20 subagentes concurrentes.
- **Precios por MTok, entrada/salida** ([pricing](https://platform.claude.com/docs/en/about-claude/pricing)): Fable 5.1 $10/$50 · Opus 5.5 $4/$20 · Sonnet 5.5 $2/$10 · Haiku 5.5 $0,10/$0,50 (hasta 100k de prompt; por encima, $0,50/$2,50). Todos tienen 1M de contexto.
- **Esfuerzo** ([effort](https://platform.claude.com/docs/en/build-with-claude/effort)):
  - los cuatro modelos admiten low…max;
  - el default es `medium` en Opus 5.5 y Haiku 5.5, y `high` en Sonnet 5.5 y Fable 5.1.
- **Discrepancia con gentle-ai:** `claude_model.go:70-83` dice que Haiku solo admite el default. La doc ya no lo dice. *No verificado*: si Claude Code acepta `effort` con Haiku en el frontmatter; hay que probarlo.
- **Costo con suscripción:** si se usa Claude Code con suscripción, el costo se mide en límites de uso, no en dólares. *No verificado* para el plan de Eber.

---

## 1. Principios

1. **Asignar el modelo según el costo del error, no según el tamaño de la tarea.**
   - El recurso más caro es la atención humana (tesis).
   - Se gastan tokens donde un error obligaría al humano a releer o terminaría en el documento firmado (C6). Se ahorran en lo mecánico.
   - Es la misma lógica de gentle-ai: "never for size… file count" (`routing.go`).
2. **Rol fijo y tier explícito.** gentle-ai no tiene selección dinámica, así que se emula así:
   - **variantes de archivo** por rol cuando cambia el esfuerzo;
   - el parámetro **`model` por invocación** cuando solo cambia el modelo;
   - una **tabla de asignación** que el orquestador consulta según el riesgo.
   - Es determinista y auditable, no "criterio del modelo".
3. **Lo generativo humano no se delega.** Encuadre, ideación inicial, confirmación de criterios y decisiones clave no tienen subagente (M12, M13). Los subagentes trabajan *después* de que el humano generó y reciben sus ideas como insumo.
4. **Contexto por referencia y aislado.**
   - Cada `hm-*` lleva `omitClaudeMd: true`. Así no hereda ODD, la persona ni el contrato de "artefactos en inglés" de gentle-ai.
   - Recibe punteros (`criterios.md C2,C5@v3`), no paráfrasis (`routing.go:43`).
5. **La independencia hay que construirla.**
   - Quien escribe no verifica (M36).
   - Los jueces difieren en modelo, lente *y* contexto.
   - El consenso entre IA es corroboración, no verdad: los errores de modelos parecidos se correlacionan (acervo §5.3).

---

## 2. Componentes

### 2.1 Catálogo de subagentes (`<proyecto>/.claude/agents/hm-*.md`)

Preset **equilibrado**. Todos llevan `omitClaudeMd: true` y la línea "Idioma de artefactos: es-PE".

| Subagente | Modelo / esfuerzo | Herramientas | Contexto que recibe (por ref.) | Devuelve | Por qué |
|---|---|---|---|---|---|
| `hm-explorador` | haiku / medium | Read, Glob, Grep, WebSearch, WebFetch, Write solo en `trabajo/fichas/` | pregunta de `encargo.md §E#`; lista de fuentes | fichas con `ruta:línea` o URL más cita textual, ≤1,5k tokens | Lectura y extracción masivas. Las citas textuales permiten verificar después; el error es barato porque se verifica aguas abajo. |
| `hm-divergente` | sonnet / high | Read | `encargo.md §Encuadre`; `ideas-humanas.md` (obligatorio); dimensiones confirmadas | N opciones por dimensión, cada una con supuesto y riesgo (M10) | Requiere variedad, no profundidad. Se corre en 2 instancias paralelas con distinta postura, y en riguroso una con `model: opus`. |
| `hm-comparador` | sonnet / medium | Read | opciones en `trabajo/<run>/`; `criterios.md` | matriz opciones × criterios con similitudes y diferencias (M09, M11) | Síntesis estructurada; el humano lee la matriz, no las opciones. |
| `hm-elicitador` | **opus / high** | Read | `reacciones.md` (las reacciones textuales del humano); `criterios.md` | criterios candidatos `C#` con la reacción que los origina y un contraejemplo (M04, M05, M08) | Inferir criterio tácito es la tarea de más juicio. Un error contamina toda la verificación posterior (H1, H3, H4). No escribe: propone, y el humano confirma. |
| `hm-redactor` | sonnet / high | Read, Edit, Write solo en superficies permitidas | `encargo.md §S#`; `criterios.md C#@vN`; `decisiones.md D#`; fichas | sección más una lista de afirmaciones `A#` con su fuente | Producción de prosa. Sonnet a high es el punto de equilibrio entre costo y calidad. |
| `hm-redactor-nucleo` | **opus / high** | ídem | ídem | ídem | Para secciones R2 y R3: marco, decisiones metodológicas, indicadores. Es una variante aparte porque el esfuerzo no se puede pasar por invocación. |
| `hm-verif-afirmaciones` | sonnet / medium | Read, Grep, WebFetch | lista `A#`; fichas; fuentes | veredicto por `A#`: sostenida, parcial, sin fuente o contradicha, con evidencia (M27, M43) | Comparar una afirmación con su fuente es acotado. Las afirmaciones R3 se reenvían con `model: opus`. |
| `hm-coherencia` | **opus / high** | Read, Grep, Glob | diff de la sección; `dependencias.md`; `decisiones.md` | diff semántico ("cambia la premisa X") y secciones o documentos afectados con su motivo (C2, H6) | Razonar sobre interdependencias es la segunda tarea de más juicio. También es el *seam check* después de los redactores paralelos. |
| `hm-juez-a` | **opus / high** | Read, Glob, Grep | documento congelado; `criterios.md@vN` | ledger estilo JD con un campo `criterio: C#` (H3) | Lente "rúbrica": ¿cumple cada `C#`? |
| `hm-juez-b` | sonnet / high | Read, Glob, Grep | documento congelado **sin** `decisiones.md` ni log | ledger con la lente "lector externo escéptico" (C12) | Difiere en modelo, lente y contexto. Al no ver el porqué, detecta lo que el texto no defiende por sí solo. |
| `hm-corrector` | sonnet / medium | Read, Edit | solo los hallazgos BLOCKER o CRITICAL en los que coinciden A y B | parche más el delta | Correcciones quirúrgicas y acotadas (JD). |
| `hm-reentrada` | sonnet / high | Read | `decisiones.md` desde la última fecha registrada en `comprension.md`; `corridas.md` | brief centrado en decisiones más 3–5 preguntas de teach-back; en una 2.ª invocación, califica las respuestas (M21, M23, H2, H5) | Hay que seleccionar qué importa y redactar preguntas que discriminen. Haiku produciría preguntas triviales. |
| `hm-archivista` | haiku / low | Read, Write solo en `expediente/indice.md` y `corridas.md`; Engram (`mem_save`) | resultados de corridas | índice actualizado y espejo en Engram | Tarea mecánica. |

**Orquestador:** es la sesión principal, no un subagente. Recomendado: opus 5.5 / medium, el default oficial. El mismo razonamiento usa Codex en gentle-ai: el orquestador con low era "the weakest link" (`codex_model.go:401-405`).

### 2.2 Otras piezas

- **Skills** (`.claude/skills/`):
  - `hm-ciclo`: el flujo; nombra al agente exacto en cada paso.
  - `hm-juicio`: JD adaptado a documentos, con agentes propios. No reutiliza `jd-judge-*`, cuyo modelo controla la TUI de gentle-ai y cuyos criterios son de código.
  - `hm-reentrada`.
- **Plantilla** `hamutay/asignacion.md`: la tabla rol × riesgo (§3).
- **Presets** `hamutay/presets/{economico,equilibrado,riguroso}.yaml`, con un script `hamutay preset <nombre>` que reescribe las líneas `model:`/`effort:` de los `hm-*.md`, igual que hace gentle-ai con su TUI.
- **Hook `SubagentStop`**: agrega una fila a `expediente/corridas.md`. *No verificado* qué metadatos (modelo, tokens) entrega el hook; en el peor caso, la fila la escribe el orquestador.

---

## 3. Reglas de asignación (emular selección dinámica)

### 3.1 Riesgo por pieza

Se declara en `encargo.md`, campo `riesgo:` por `S#`; el valor por defecto es R1.

| Nivel | Qué es | Ejemplo GEE |
|---|---|---|
| R0 | Forma | formato, numeración |
| R1 | Descriptivo | antecedentes |
| R2 | Argumento núcleo | marco conceptual, criterios metodológicos |
| R3 | Sustenta decisiones o se firma ante terceros | cifras, normas, fórmulas de indicadores, recomendaciones |

### 3.2 Tabla de asignación

| Paso | R0–R1 | R2 | R3 |
|---|---|---|---|
| Redactar | `hm-redactor` | `hm-redactor-nucleo` | `hm-redactor-nucleo` |
| Verificar afirmaciones | muestreo del 20 % | todas | todas, con `model: opus` |
| Coherencia | solo si cambia una decisión | siempre | siempre |
| Juicio | no | `hm-juicio` (A+B) | A+B, más `model: fable` para A en riguroso |
| Humano | no | lee el ledger | lee el ledger y responde el teach-back |

### 3.3 Escalamiento

Es una sola subida de nivel; después decide el humano (M32).

- **Sube un nivel** (cambia el `model` por invocación) si ocurre cualquiera de estos casos:
  - el veredicto llega como "parcial" o "sin fuente" en R2 o más;
  - los jueces se contradicen;
  - el escritor corrió en haiku o en low. Es la regla de sesgo de modelo pequeño de gentle-ai (`odd-orchestrator-sections.md:16`).
- **Si persiste**, pasa a "Necesita tu decisión" con un paquete M38 de 5 líneas.
- **Tope:** 3 escalamientos por ciclo. Si hay más, el encargo está mal especificado y se vuelve a Criterios.

---

## 4. Contexto, delegación y paralelismo

### 4.1 Presupuesto de contexto

- **Inline del orquestador:** ≤3 lecturas o unos 10k tokens. Son las cifras de gentle-ai (`manifest.go:217`).
- **Prompt de delegación:** ≤2k tokens, solo con referencias.
- **Retorno:** ≤1,5k tokens estructurados. El detalle se escribe en `trabajo/<run-id>/` y no entra al contexto del padre.
- **Sesión nueva** cuando el orquestador llega a unos 150k tokens, entrando por `hm-reentrada`.
- El 1M de contexto no cambia la regla. El problema no es la capacidad sino la dilución y el costo por turno.

### 4.2 Cuándo delegar y cuándo trabajar directo

- **Directo:**
  - co-generación humana;
  - ediciones R0–R1 en una sola sección;
  - consultas puntuales.
- **Delegar:**
  - más de 5 fuentes o más de 10k tokens de lectura;
  - toda divergencia, porque el contexto limpio evita que la conversación ancle las opciones;
  - **toda verificación** (M36);
  - secciones paralelas.

### 4.3 Paralelismo

- **Exploradores:** ≤5, uno por grupo de fuentes.
- **Divergentes:** 2–3.
- **Jueces:** siempre 2, a ciegas.
- **Redactores:** solo con superficies disjuntas y `criterios.md` congelado en una versión; después corre obligatoriamente `hm-coherencia` (H6).
- **Techo práctico:** 5 concurrentes, no 20, para que el humano pueda integrar la comprensión (C7).

### 4.4 Errores correlacionados

Hay cuatro ejes de diversidad:
1. **Modelo:** opus frente a sonnet, más fable en riguroso.
2. **Lente:** rúbrica frente a lector externo.
3. **Contexto:** B no ve el porqué.
4. **Postura:** el divergente tiene una instancia "contraria".

Además:
- Un hallazgo que reporta un solo juez queda como *suspect*. Se muestra al humano solo en R3.
- **Muestreo humano:** en cada ciclo el humano revisa 1 afirmación R2 o R3 aprobada, elegida al azar, y se registra el acierto en `corridas.md` (M39). Así se calibra la confianza en los jueces sin leerlo todo.

---

## 5. Presets

| Rol | económico | equilibrado | riguroso |
|---|---|---|---|
| orquestador (sesión) | sonnet / medium | opus / medium | opus / high |
| explorador | haiku / low | haiku / medium | haiku / high |
| divergente ×2 | sonnet / medium ×1 | sonnet / high ×2 | sonnet / high + opus / high |
| elicitador | sonnet / high | opus / high | opus / xhigh |
| redactor / núcleo | sonnet / medium · sonnet / high | sonnet / high · opus / high | opus / medium · opus / high |
| verif. afirmaciones | haiku / high | sonnet / medium | sonnet / high (R3: opus) |
| coherencia | sonnet / high | opus / high | opus / xhigh |
| juez A / B | sonnet / high · haiku / high | opus / high · sonnet / high | fable / high · sonnet / high |
| corrector | sonnet / medium | sonnet / medium | opus / medium |
| reentrada | sonnet / medium | sonnet / high | opus / medium |
| archivista | haiku / low | haiku / low | haiku / low |

**Orden de magnitud por juicio** de un documento de 40k tokens con 4k de salida, a precio de lista y sin contar el pensamiento, que se factura como salida:

| Modelo | Costo aprox. |
|---|---|
| Haiku | $0,006 |
| Sonnet | $0,12 |
| Opus | $0,24 |
| Fable | $0,60 |

Con esfuerzo high o más, la salida puede multiplicarse varias veces. **Lo que encarece es la repetición** (rondas y escalamientos), no el modelo base: de ahí los topes de 2 rondas de JD y 3 escalamientos. *Estimación propia, no medida.*

**Selección:**
- `encargo.md` declara `preset:`.
- Regla por defecto: equilibrado; riguroso si el documento se firma ante terceros (C6); económico para exploración desechable.

---

## 6. Artefactos operativos

```
.claude/agents/hm-*.md          # 13 agentes (2 variantes de esfuerzo)
.claude/skills/hm-ciclo|hm-juicio|hm-reentrada/SKILL.md
hamutay/asignacion.md           # tabla §3, editable por el humano
hamutay/presets/*.yaml
expediente/corridas.md          # run-id | fecha | agente | modelo/esfuerzo | refs de entrada | veredicto | escalado? | muestreo humano ok?
trabajo/<run-id>/               # salidas completas; se puede descartar tras cerrar el ciclo
```

`corridas.md` responde la pregunta de C12: "qué se verificó, con qué criterio (`C#@vN`) y con qué modelo".

---

## 7. Relación con gentle-ai y Engram

| Pieza | Se usa | Se apaga o se ignora | Se reemplaza |
|---|---|---|---|
| Frontmatter `model:`/`effort:` | Sí, en los agentes propios | | |
| Presets de la TUI | | Solo afectan a `jd-*` y `review-*` | `hamutay preset` |
| ODD | En el orquestador, según lo que resuelvan otros encuadres | Los `hm-*` no lo reciben (`omitClaudeMd`) | |
| JD | El patrón: ledger, precision gate, severity floor, 2 rondas | `jd-judge-*` | `hm-juicio` |
| RDD | | `review mode disable` en el repo de documentos | |
| Persona | | `custom` | |
| Engram | Solo el archivista escribe; el padre pasa contexto a los subagentes (protocolo gentle-ai) | Los subagentes no hacen `mem_search` por su cuenta | |

---

## 8. Huecos cubiertos

| Hueco o necesidad | Cómo se cubre |
|---|---|
| H1, H3, H4 | `hm-elicitador` en opus y los jueces que citan `criterio: C#@vN` |
| H2, H5 | `hm-reentrada` en dos invocaciones, más el muestreo humano |
| H6, C2 | `hm-coherencia` como seam check obligatorio |
| C1 | Divergentes con diversidad de postura y modelo, más el comparador |
| C12, C6 | `corridas.md` |
| Verificación sin tests | Niveles R0–R3 que fijan modelo, intensidad y lentes |

---

## 9. Riesgos, costos y decisiones abiertas

### Riesgos

- **Correlación residual:** todos los jueces son de Anthropic. La diversidad de lente y de contexto mitiga, pero no garantiza independencia.
- **El orquestador puede ignorar la tabla.** Es una instrucción, no un control. Mitigaciones: la skill nombra el agente exacto por paso, y `corridas.md` permite auditar si se cumplió.
- **Proliferación de archivos de agente.** Las variantes de esfuerzo se limitan a 1 rol (redactor); el resto sube de tier por `model`.
- **Costo de fable o de xhigh y max** sin evidencia de ganancia en prosa. La doc oficial recomienda subir "only where your evals show a quality gain".
- **Fatiga humana** por escalamientos y *suspects*. Mitigación: los topes de §3.3 y §4.4.
- **Dependencias por verificar:** la versión mínima de `omitClaudeMd`, el esfuerzo de Haiku en Claude Code, los metadatos del hook `SubagentStop`, y los precios, que cambian (fecha de verificación arriba).

### Decisiones abiertas

1. ¿Un juez B no Anthropic (vía Codex o MCP) en riguroso?
2. ¿Fable como orquestador en riguroso, o solo como juez?
3. ¿Muestreo del 20 % en R1, o cero?
4. ¿Mini-evaluación propia (10 secciones GEE v6 con hallazgos conocidos) para calibrar los presets antes de fijarlos?
