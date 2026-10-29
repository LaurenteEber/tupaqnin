# Encuadre 3: Humano primero (cognitivo/HCI)

*Fase 2 · Propuesta de arquitectura v1 de `hamutay`. La técnica está al servicio de lo que el humano necesita para co-generar, entender, decidir y defender.*

---

## 1. Principios

1. **Generar antes de ver.** En encuadre, ideación y criterios, el humano escribe primero y el agente expande o critica después. Usar el LLM desde el inicio reduce la originalidad y la autoría (*Timing Matters*, CHI 2025; Doshi & Hauser 2024 → M12). Las dimensiones que propone el modelo anclan el pensamiento (límite de M10 y M11).
2. **La atención es un presupuesto, no un recurso libre.** Cada interrupción tiene un tipo y un costo. Si se aprueba casi todo, la aprobación ya no supervisa nada (93 % de permisos aprobados, M33). Por eso **no hay compuertas de "¿apruebas?"**: cada contacto exige un acto distinto (elegir, predecir, editar o explicar).
3. **Los criterios se reconocen al reaccionar, no se dictan al inicio.** La deriva de criterios (EvalGen, Shankar et al. 2024 → M04), el feedback convertido en principios (ConstitutionMaker → M05) y la elicitación activa (GATE → M01) muestran que el criterio aparece al ver resultados.
4. **Señales contrarias antes que narrativa.** Las explicaciones elevan la confianza en lo correcto y en lo incorrecto. Las fuentes, las inconsistencias y la incertidumbre en primera persona sí calibran (Kim et al., CHI 2025 y FAccT 2024 → M43). Las trazas mejoradas no mejoran la precisión (Grunde-McLaughlin 2026).
5. **La comprensión es participación activa mínima, medida por actos y no por lecturas.** "Generar y luego comprender" preserva el aprendizaje (Anthropic, N=52 → M23). Editar y hacer prompts iterativos recuperan parte de la propiedad (Pennanen 2026). Una mayor confianza en la IA reduce el pensamiento crítico (Lee et al., CHI 2025).

---

## 2. Componentes

### 2.1 Puntos de contacto y presupuesto de atención

| Tipo | Acto humano | Costo | Tope por sesión | Cuándo |
|---|---|---|---|---|
| **G Generar** | Escribir el encuadre, las ideas o una explicación propia | Alto | 1 bloque | F0, F1 y cierre |
| **D Decidir** | Elegir entre 2 y 4 opciones contrastadas; la recomendación del agente se muestra **después** de elegir | Medio | 1 lote de ≤3 decisiones | Fin de F2/F4 |
| **R Reaccionar** | Marcar +/–/? en tarjetas, con una razón opcional de ≤1 línea | Bajo | Sin tope, pero asíncrono | F2 y F4 |
| **C Confirmar criterio** | Aceptar, editar o retirar candidatos | Bajo | 1 lote de ≤5 | Cierre de sesión |
| **P Predecir/explicar** | 1 o 2 preguntas de comprensión | Medio | ≤2 | Cierre de etapa o reentrada |
| **A Alerta** | Responder a una interrupción en caliente | Muy alto | ≤1, solo con disparador | Ver regla abajo |

**Reglas:**
- **Interrupción en caliente**, solo con uno de tres disparadores:
  - contradicción con un criterio o decisión **confirmados**;
  - cambio de premisa que afecta a más de una sección;
  - acción irreversible o salida a terceros.
  
  Todo lo demás se encola en el expediente.
- **Agrupar siempre.** Las decisiones y los criterios llegan en lotes al final de un tramo, no dispersos (M17, M37).
- **Anti-fatiga.** `atencion.md` lleva la **tasa de aceptación sin cambios** de las últimas 10 respuestas D/C.
  - Si supera el 85 %, el siguiente lote D se presenta a ciegas: el humano predice o elige antes de ver la recomendación (forcing function, Buçinca → M13).
  - Además, el agente propone **bajar** de nivel los tipos de ítem que el humano nunca cambia, para que no consuman atención (M33).
- **Omitir es válido y se registra.** Saltar una P no bloquea; queda como deuda de comprensión visible, sin castigo.

### 2.2 Instrucción

`<repo-doc>/CLAUDE.md`, fuera de los marcadores de gentle-ai:
- precedencia del flujo hamutay sobre los pasos 4 a 7 de ODD en este repo;
- idioma de los artefactos: español;
- el presupuesto de atención anterior;
- "no generes ideas ni dimensiones antes de leer `ideas/*-humano.md`";
- todo mensaje con propuesta lleva un bloque `Contrapeso` (§2.5).

### 2.3 Skills (`<repo-doc>/.claude/skills/`)

| Skill | Hace | Contacto |
|---|---|---|
| `hamutay-encuadre` | Pide 5 líneas del humano (problema, audiencia, qué tendrá que defender, ante quién, qué sería un fracaso). Luego entrevista con casos límite (M01, M02) | G |
| `hamutay-idear` | Exige ≥3 ideas humanas en `ideas/`; luego lanza `hm-divergente` para expandir y criticar, **marcando qué ideas son del humano** (M20) | G → R |
| `hamutay-reaccionar` | Presenta tarjetas contrastadas en tabla (M09, M11) y registra reacciones; `hm-destilador` propone candidatos a criterio | R → C |
| `hamutay-decidir` | Arma el lote D en Lossless Blocking Prompt: opciones, criterios en juego y contrapeso; revela la recomendación después | D |
| `hamutay-comprender` | Teach-back, predicción o simulacro de defensa; actualiza `comprension.md` | P |
| `hamutay-retomar` | Genera `reentrada.md` con pregunta de recuperación previa | P → lectura |
| `hamutay-cerrar` | Lote C + ≤2 P + resumen de deuda | C, P |

### 2.4 Subagentes (`<repo-doc>/.claude/agents/`, modelo y esfuerzo fijados en el frontmatter)

| Subagente | Rol | `model` / `effort` | Herramientas |
|---|---|---|---|
| `hm-divergente` | Variantes y dimensiones **después** de las ideas humanas | sonnet / high | Read, Write en `exploracion/` |
| `hm-destilador` | Reacciones → candidatos a criterio, con citas textuales del humano como evidencia | sonnet / medium | Read, Write en `criterios/` (solo `estado: propuesto`) |
| `hm-abogado` | Contrapeso: inconsistencias contra criterios y decisiones confirmados, alternativas descartadas y preguntas del simulacro de defensa; contexto limpio (M31) | opus / high | Read |
| `hm-examinador` | Compara el teach-back con el registro y señala discrepancias, **no califica** | sonnet / high | Read, Write en `comprension.md` |
| `hm-reentrada` | Arma `reentrada.md` desde `git log` del expediente y `comprension.md` | sonnet / medium | Read, Bash (git de solo lectura) |
| `hm-juez-a` / `hm-juez-b` | Judgment Day con lentes de documento, solo en cierre de riesgo alto | opus / high · haiku / (default) | Read |

`hm-juez-b` usa otro modelo a propósito: los errores de modelos parecidos se correlacionan (Goel et al., ICML 2025). Aun así, se declara que no es una revisión independiente.

### 2.5 Plantilla `Contrapeso` (obligatoria en propuestas, máx. 6 líneas)

```
Contrapeso
- No estoy seguro de: <afirmación> (porque <razón>)            ← 1ª persona, M43
- Choca con: C-007 v2 / D-012 (<cita>)                        ← inconsistencia visible
- Descarté: <alternativa> por <criterio>; la retomaría si <condición>
- No verificado: <qué>  ·  Fuentes: <ruta o URL>
```

El bloque no lleva explicaciones largas del razonamiento (Kim et al., CHI 2025). Si no hay nada que señalar, se escribe "Contrapeso: ninguno detectado", nunca se omite.

### 2.6 Hooks (mínimos)

- **`SessionStart`:** si pasaron más de 3 días desde la última sesión humana, imprime una línea: "Deuda: 4 decisiones de peso alto sin explicar · 2 criterios propuestos · usa /hamutay-retomar".
- **`Stop`:** suma los contactos del turno en `atencion.md` y avisa si se superó el tope. No bloquea.

### 2.7 Memoria

- **Expediente Markdown en git:** fuente de verdad.
- **Engram:** índice, con `topic_key` igual al ID (`criterio/C-007`). Ante un `judgment_required` del tipo `conflicts_with` o `supersedes` sobre un criterio o una decisión, se **encola como alerta A o lote C**; nunca se resuelve en silencio (engram.md §7.2).

---

## 3. Flujo proporcional

| Tamaño | Fases | Contactos |
|---|---|---|
| **Pequeño** (ajuste de redacción, una sección) | F4 directo | Solo `Contrapeso`; R opcional |
| **Mediano** (rehacer un capítulo, integrar una decisión) | F3 → F4 → F5 | 1 lote D, 1 lote C |
| **Grande** (nueva versión, por ejemplo GEE v6 → v7) | F0 a F6 | Todos |

| Fase | Quién genera | Compuerta |
|---|---|---|
| F0 Encuadre | **Humano** (5 líneas), luego entrevista | **G1:** `encuadre.md` con texto del humano. El agente no lo redacta, solo lo pule si se le pide |
| F1 Ideación | **Humano** ≥3 ideas y sus dimensiones; luego el agente expande | — |
| F2 Exploración | Agente diverge; humano reacciona (R) | — |
| F3 Criterios | `hm-destilador` propone; humano confirma | **G2:** criterios v1 confirmados (lote C) |
| F4 Construcción | Agente redacta; interrumpe solo con disparador A | Lote D al final de cada tramo |
| F5 Revisión | `hm-abogado`, más JD si el riesgo es alto | — |
| F6 Cierre y defensa | **Humano** responde un simulacro de 3 preguntas | **G3:** simulacro respondido o deuda aceptada de forma explícita |

**Criterios desde reacciones (H1/H3).** Reacciones R con cita textual → con ≥2 coherentes, `hm-destilador` propone enunciado + ejemplo + contraejemplo tomados de ellas (M08) → el humano acepta o reescribe en sus palabras en el lote C (M05, M06) → `confirmado vN` alimenta el `S#` afectado y la rúbrica de `hm-abogado` y los jueces. Cambiarlo crea `vN+1` con `motivo` y la reacción que lo provocó; `hm-abogado` marca las secciones que aplicaron la versión anterior (H4).

**Comprensión sin burocracia (H2).** Solo sobre decisiones `peso: alta`, ≤2 por etapa, rotando formato:
- **Teach-back:** "en 2 frases, ¿por qué D-012 y no la descartada?"; `hm-examinador` muestra discrepancias con el registro (M23).
- **Predicción:** "¿qué esperas que cambie en §3 al aplicar D-014?" antes de mostrarlo; fallar revela deuda.
- **Simulacro de defensa** (cierre): 3 preguntas de revisor externo hostil (C6, C12); las respuestas quedan como evidencia.

**Deuda de comprensión** = decisiones de peso alta con `explicado: nunca` o explicadas antes de su última modificación. Conteo, no puntaje.

**Reentrada (H5).** `hamutay-retomar`: primero una **pregunta de recuperación antes de leer** ("¿cuál era la premisa y qué decidiste?"; aplica "generar y luego comprender", M23, como hipótesis). Luego `reentrada.md` de una página: (a) lo que decidiste tú, citando tus explicaciones; (b) qué cambió en criterios y decisiones (diff git del expediente, no de la prosa: M21 centrado en decisiones); (c) lo que probablemente olvidaste (peso alto sin explicar o explicado hace >14 días); (d) la próxima decisión pendiente. Detalle atenuado, no resumido (M44).

---

## 4. Artefactos

```
<repo-doc>/
  CLAUDE.md                      # instrucción hamutay (fuera de marcadores)
  .claude/skills/hamutay-*/      .claude/agents/hm-*.md
  doc/metodologia-gee-v7.md      # el documento
  expediente/
    encuadre.md                  # texto humano; origen: humano
    ideas/2026-10-07-humano.md   # inmutable; nunca lo edita el agente
    exploracion/                 # salidas de hm-divergente
    reacciones.md                # R-### | fecha | tarjeta | +/–/? | cita
    criterios/C-007-coherencia-indicador-meta.md
    decisiones/D-012-unidad-de-analisis.md
    comprension.md               # tabla por D-### (ver abajo)
    atencion.md                  # contactos por sesión, tasa de aceptación
    reentrada.md                 # generado; no es fuente de verdad
```

**Frontmatter de un criterio:**
```yaml
id: C-007
version: 2
estado: propuesto | confirmado | retirado
enunciado: "..."
origen: [R-031, R-034]
cita_humana: "no me sirve un indicador que no puedas medir anualmente"
ejemplo: ...
contraejemplo: ...
aplica_a: [doc/metodologia-gee-v7.md#3.2]
confirmado: {fecha: 2026-10-07, forma: editado | aceptado}
reemplaza: C-007@v1
motivo_cambio: "..."
```

**Frontmatter de una decisión:** `id`, `pregunta`, `opciones` (cada una con `descartada_por`), `elegida`, `generada_por: humano | agente | conjunta`, `eleccion: ciega | vista_recomendacion`, `criterios: [C-007@v2]`, `peso: alta | media | baja`, `incertidumbre`, `contrapeso`, `afecta: [secciones]`.

**`comprension.md`:** `D-### | peso | generada_por | explicado (fecha) | prediccion (ok/fallo/omitida) | simulacro | vigente (sí/no tras cambio)`.

---

## 5. Relación con gentle-ai y Engram

| Pieza | Decisión |
|---|---|
| ODD | **Convive.** El `CLAUDE.md` del proyecto declara la precedencia. El encargo se mantiene como `S#` (citas textuales del humano) y el resto vive en el expediente. Hay que probar si el modelo respeta la precedencia (gentle-ai §11) |
| Persona | `--persona custom` en el repo de documentos. La regla de "una pregunta" choca con los lotes D y C; los lotes viajan como Lossless Blocking Prompts |
| Contrato de idioma | Se sobrescribe: los artefactos van en español |
| RDD | `review mode disable --scope clone`; no revisa prosa |
| Judgment Day | Se reutiliza como `hm-juez-a/b` con lentes de documento: coherencia con criterios confirmados, trazabilidad a fuentes y consistencia con decisiones. Se usa solo en cierres de riesgo alto |
| Engram | Índice. Protocolo propio: escribir primero en el expediente y luego `mem_save` con un puntero. Los conflictos se escalan al humano |

---

## 6. Cobertura

| Hueco o necesidad | Cómo |
|---|---|
| **H1** rúbrica desde criterios emergentes | Reacciones → `hm-destilador` → lote C → `C-### vN` |
| **H3** criterio tácito auditable | `cita_humana`, `origen`, `C-###@vN` en cada decisión y `aplica_a` |
| **H4** spec viva | Un criterio confirmado reescribe solo el `S#` afectado (regla ODD) y marca las secciones con versiones anteriores |
| **H2** deuda de comprensión | `comprension.md` más un conteo de deuda; teach-back, predicción y simulacro |
| **H5** reentrada | Recuperación previa más `reentrada.md` centrado en decisiones y en lo olvidado |
| Co-generación | G1, `ideas/*-humano.md` inmutable, `generada_por` |
| C1, C6/C12, fatiga | Tarjetas contrastadas (M09, M11); simulacro y `Contrapeso` archivados; topes, lotes y elección a ciegas |

No cubre H6, H7 ni H8, ni el diff semántico entre documentos (C2); quedan para otro encuadre.

---

## 7. Riesgos, costos y decisiones abiertas

**Riesgos:**
1. **Rechazo de la fricción**: las forcing functions funcionan pero se valoran peor (Buçinca). Se activan solo con aceptación >85 % y se pueden omitir con registro.
2. **Teach-back como examen**: el examinador contrasta, no califica; ≤2 preguntas, solo peso alto.
3. **Goodhart**: responder por responder baja el conteo de deuda sin comprensión; el simulacro hostil es el único control parcial.
4. **Evidencia de laboratorio y tareas cortas** (informe §3): los umbrales (85 %, 14 días, ≤3) son conjeturas que calibra el piloto GEE v6 → v7.
5. **El agente asigna el `peso`** y con eso decide qué se pregunta; el humano puede subirlo en el lote D.
6. **Precedencia sobre ODD MANDATORY** sin verificar.

**Costo:** opus/high solo en `hm-abogado` y `hm-juez-a` (F5, F6); el resto en sonnet. Ideación humana primero alarga el arranque, aceptable solo en tamaño grande.

**Decisiones abiertas:** ¿tasa de aceptación o tiempo de respuesta como señal de complacencia? ¿Simulacro oral (dictado)? ¿Lote C por sesión o por tramo? ¿`comprension.md` visible siempre o solo en reentrada, para evitar sensación de vigilancia?
