# hamutay: arquitectura v1.1 (tras auditorías A y B)

*Corrige la v1 con `auditoria-A-ceremonia.md` y `auditoria-B-tecnica.md`. Separa el **MVP**, que basta para probar la tesis, de lo que pasa a **v2**.*

## Cambios principales respecto de v1

| # | Cambio | Origen |
|---|---|---|
| 1 | MVP recortado: 3 skills, 2 subagentes y 2 tipos de registro. El resto pasa a v2 | Auditoría A: unos 50 conceptos repetían el error de SDD |
| 2 | **Presupuesto de lectura del documento:** por sección, el agente marca qué párrafos hay que leer | Auditoría A: el registro se sumaba a la lectura en vez de reemplazarla |
| 3 | Las señales que gobiernan al humano (lo crítico, las confirmaciones, el resultado del teach-back) no las decide el agente | Auditoría A: la métrica era circular |
| 4 | **Cola de desborde** visible: lo que supera un tope no se pierde ni se rebaja | Auditoría A |
| 5 | Se elimina la regla del 85 % | Auditoría A: sin sentido estadístico con lotes de 3 |
| 6 | Los jueces no exigen consenso: el lector externo reporta y lo que los jueces no comparten va a la cola | Auditoría A: la intersección de lentes distintas quedaba casi vacía |
| 7 | **Las reglas duras se cumplen con hooks, no con instrucciones** | Auditoría B: `tools` no limita rutas; los hooks sí bloquean |
| 8 | Aislamiento de ODD mediante `claudeMdExcludes` en el repo de documentos. Los subagentes llevan `omitClaudeMd: true` y las reglas precargadas con `skills:` | Auditoría B |
| 9 | Sale Engram del MVP | Auditoría B: es redundante con `decision`/`policy` y su reloj no se reinicia |
| 10 | Se empaqueta como **plugin** de Claude Code | Auditoría B |
| 11 | El registro empieza en el delta (etiqueta `v6-base`), sin reconstruir lo anterior | Auditoría A |

## 1. MVP

### 1.1 Skills

| Skill | Qué hace |
|---|---|
| `hamutay-explorar` | **G1:** pide el encuadre y las ideas propias, que quedan en `ideas/*-humano.md`, inmutables. Diverge en tarjetas (`opciones.yaml`, con estado vigente o descartada y su razón). Registra las reacciones +/–/? con cita literal y destila los CRI propuestos |
| `hamutay-construir` | Redacta por sección citando CRI y DEC y propone DEC nuevas con su Contrapeso. Ejecuta `hm impacto` ante cualquier cambio de CRI o DEC. Entrega el **presupuesto de lectura** |
| `hamutay-retomar` | Hace una pregunta de recuperación antes de leer; luego genera `reentrada.md` (lo que decidiste, qué cambió, la cola, la próxima decisión) y una **prueba de defensa** de 1–2 preguntas, que califica el humano |

### 1.2 Subagentes

Llevan `omitClaudeMd: true` y las reglas de hamutay precargadas con `skills:`.

| Subagente | Modelo/esfuerzo | Por qué |
|---|---|---|
| `hm-divergente` | sonnet/medium | Generar opciones contrastantes sin ver la recomendación del orquestador |
| `hm-lector` | opus/high (o un modelo de otro proveedor en OpenCode) | Lector externo ciego: solo ve la sección, no las decisiones ni la conversación. Es el sustituto más barato del revisor externo (C12) |

El orquestador es la sesión principal (opus/medium); en ella se redacta. Se escala de modelo pasando `model` en la invocación, que está verificado.

### 1.3 Registros

| Registro | Campos |
|---|---|
| `CRI-###` | `version`, `estado`, `cita_humana`, `origen`, `ejemplo`, `contraejemplo`, `aplica_a` |
| `DEC-###` | `pregunta`, `elegida`, `descartes` con razón, `supuestos` (texto), `critico`, `generada_por`, `contrapeso`, `estado` |
| `encargo.md` | `S#` literales, `T#` y Log, en formato ODD |
| `doc/` | El documento. Cada sección declara en su frontmatter `depende_de: [DEC-, CRI-@v]` |

**`critico` se decide por reglas mecánicas, no por juicio del agente.** Una DEC es crítica si cumple alguna de estas condiciones, y el humano siempre puede subirla:
- cambia o contradice un CRI;
- afecta a dos o más secciones;
- toca una cifra, una conclusión o una recomendación.

### 1.4 Hooks: las reglas duras

| Hook | Regla |
|---|---|
| `PreToolUse` (Write/Edit) | Bloquea editar `ideas/*-humano.md`. Bloquea escribir `estado: confirmado` en un CRI o DEC si `cita_humana` no aparece en el transcript **[por probar]** |
| `SessionStart` | Si hubo pausa de N días o hay cola pendiente, propone `hamutay-retomar` |
| `Stop` | Ejecuta `hm check`: IDs, enlaces y secciones que dependen de versiones viejas |
| `PreCompact` | Guarda el estado de la sesión en `sesiones/` |

### 1.5 Herramienta `hm.py`

Usa solo la biblioteca estándar de Python.

| Comando | Qué hace |
|---|---|
| `hm impacto <ID>` | Lista lo que depende del ID |
| `hm check` | Valida IDs, enlaces y versiones |
| `hm reentrada` | Genera la vista de reentrada |

### 1.6 Contactos humanos

| Tipo | Tope | Desborde |
|---|---|---|
| G1 Generar (encuadre e ideas) | 1 por trabajo | — |
| G2 Reaccionar y confirmar criterios | ≤5 por lote | A la cola |
| G3 Decidir (DEC críticas) | ≤3 por lote | A la cola |
| Defensa al retomar o cerrar | 1–2 preguntas | — |
| Alerta en caliente | ≤1 por sesión | — |
| **Lectura presupuestada** | Solo los párrafos marcados: dependen de un CRI o DEC crítico, o llevan `[no verificado]` | El resto queda como lectura opcional |

### 1.7 Aislamiento frente a gentle-ai

- En `.claude/settings.json` del repo de documentos: `claudeMdExcludes` sobre el `~/.claude/CLAUDE.md` global. **[por probar]** Verificar con `/memory` en un minuto.
- Si funciona, gentle-ai sigue intacto en los repos de código.
- **Patrones que se adoptan sin instalar nada:** encargo `S#/T#/Log`, encargo por referencia, cuatro estados públicos, una corrección y luego escalar al humano.
- El modo acoplado queda como alternativa documentada, no como camino principal.

## 2. v2 (después del piloto)

- **Registros adicionales:** SUP, AFI y FUE separados, y `comprension.yaml` con métrica de deuda.
- **Subagentes adicionales:** `hm-destilador` (opus/high), `hm-verificador`, `hm-coherencia`, `hm-juez-a` de rúbrica.
- **Operación:** riesgo R0–R3 con tabla rol × riesgo, presets y `corridas.md`.
- **Memoria:** Engram como índice, o `memory: project` en los subagentes.
- **Vistas y trazabilidad:** tablero de criterios, mapa de decisiones y trailers de git.
- **Hooks de validación:** `SubagentStop`, y hooks tipo `prompt` para revisar el formato de salida.

## 3. Prueba de la tesis (piloto GEE v6 → v7)

- **Diseño:** el mismo capítulo con y sin hamutay.
- **Medidas:**
  1. palabras leídas por el humano;
  2. número de interacciones;
  3. prueba de defensa a la semana;
  4. tiempo de reentrada tras una pausa.
- **Éxito:** se lee menos y la defensa es igual o mejor.

## 4. Abiertos

- **[por probar]** `claudeMdExcludes` sobre el CLAUDE.md global.
- **[por probar]** Validar citas contra el transcript desde un hook.
- **Calibración** de los topes en el piloto.
- **Diversidad del lector:** usar otro proveedor o no.
- **Organización del repositorio:** un repo por entregable o un monorepo.
