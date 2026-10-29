# hamutay: arquitectura v1 (borrador consolidado para auditoría)

*Consolida los encuadres 1 a 5 de `/home/claude/proyecto/fase2/`. Las decisiones del orquestador van marcadas como **[decisión]**; lo que falta probar, como **[por probar]**.*

## 0. Principios

1. **El humano genera en los momentos que definen el trabajo y el agente en el resto.** Encuadre, ideas iniciales, criterios y decisiones de peso son humanos (M12, M17).
2. **Flujo proporcional, sin ceremonia.** Lección del retiro de SDD: un pedido pequeño se atiende directo; el expediente solo se abre cuando el trabajo lo exige.
3. **El expediente en git es la fuente de verdad para el humano.** Engram es solo índice y memoria del agente.
4. **Evidencia con señales contrarias.** Toda propuesta lleva un bloque *Contrapeso*: incertidumbre, choques con criterios, descartes y lo no verificado (M43).
5. **El humano tiene un presupuesto de atención.** Las interrupciones son pocas, se agrupan en lotes y cada una tiene un tipo; no hay compuertas de "¿apruebas?".
6. **Núcleo portable.** Funciona con o sin gentle-ai, en Claude Code y en OpenCode.

## 1. Relación con gentle-ai **[decisión]**

El núcleo de hamutay no depende de gentle-ai, y hay dos modos de convivencia:

| Modo | Cómo | Cuándo |
|---|---|---|
| **Acoplado** | Un `CLAUDE.md` de proyecto, fuera de los marcadores de gentle-ai, define un glosario que reinterpreta ODD para documentos: el documento de feature es el *encargo*, "test" es una *lente de documento* y `Risk:` usa ítems de documento. Se reutilizan Judgment Day, Lossless Blocking Prompts y Engram. Se apaga RDD por repo. La persona se queda, pero con menús permitidos en exploración y decisiones. | Si la prueba de humo confirma que el modelo respeta el glosario frente al ODD obligatorio |
| **Aislado** | gentle-ai se instala solo en repos de código (`--scope workspace`). En el repo de documentos rigen `AGENTS.md` y las skills de hamutay, y los subagentes no heredan el CLAUDE.md global. | Si la prueba de humo falla, o para el repo público |

**De gentle-ai se toman siempre estos patrones:**
- el encargo con `S#` literales, `T#` y Log;
- "solo se toca lo afectado";
- el encargo por referencia;
- el veredicto por `S#`;
- los cuatro estados públicos (*Working, Checking, Ready, Needs your decision*);
- una sola ronda de corrección y luego se escala al humano;
- los dos jueces ciegos.

**[por probar] Prueba de humo:** 5 pedidos de documentos con gentle-ai activo para ver si gana el glosario o el ODD.

## 2. Componentes

### 2.1 Instrucciones

- **`AGENTS.md` es la instrucción canónica.** El `CLAUDE.md` del proyecto solo contiene `@AGENTS.md` y, en modo acoplado, el glosario ODD.
- **El idioma de los artefactos es el español**, de forma explícita.

### 2.2 Skills (`.claude/skills/hamutay-*`)

| Skill | Qué hace | Casuísticas y huecos |
|---|---|---|
| `hamutay-encuadre` | Pide el encuadre humano (5 líneas) y las ideas propias antes de cualquier generación; abre el expediente si el trabajo es grande | C10, co-generación |
| `hamutay-explorar` | Diverge en tarjetas comparables y registra las reacciones +/–/? con cita literal | C1 |
| `hamutay-criterio` | Destila los criterios desde las reacciones; el humano los confirma o reescribe; se versionan | H1, H3, H4, C8 |
| `hamutay-construir` | Redacta por sección, citando criterios y decisiones, y anota las decisiones nuevas como propuestas | C2 |
| `hamutay-juicio` | Jueces ciegos con lentes de documento; solo se corrige lo que ambos confirman | C0 |
| `hamutay-impacto` | Propagación y diff semántico, apoyado en `hm.py` | C2, C7 |
| `hamutay-cerrar` | Cierre de etapa: teach-back breve y actualización de la deuda de comprensión | H2, C6 |
| `hamutay-retomar` | Reentrada humana: primero una pregunta de recuperación, luego `reentrada.md` | H5, C11 |

### 2.3 Subagentes (`.claude/agents/hm-*.md`)

Cada uno fija modelo y esfuerzo en su frontmatter. El orquestador es la sesión principal (opus/medium).

| Subagente | Rol | Modelo/esfuerzo | Herramientas | Recibe |
|---|---|---|---|---|
| `hm-explorador` | Buscar y leer fuentes, extraer evidencia | sonnet/low (haiku en económico) | lectura, web | Pregunta y alcance |
| `hm-divergente` | Generar opciones contrastantes en dimensiones explícitas | sonnet/medium | lectura | Encuadre e ideas humanas (inmutables) |
| `hm-destilador` | Proponer criterios con ejemplo y contraejemplo a partir de las reacciones | opus/high | lectura y escritura en `expediente/` | `reacciones.md` |
| `hm-redactor` | Redactar una sección | sonnet/high; variante `hm-redactor-nucleo` en opus/high para secciones R3 | lectura y escritura en `doc/` | `SEC-` más los CRI y DEC referidos |
| `hm-verificador` | Verificar afirmaciones contra fuentes | sonnet/medium | lectura, web | Afirmaciones marcadas R2 y R3 |
| `hm-coherencia` | Coherencia entre documentos y propagación de cambios | opus/high | lectura y `hm.py` | Ref de cambio y grafo |
| `hm-juez-a` | Lente de rúbrica: ¿cumple los CRI confirmados? | opus/high | lectura | Sección y CRI |
| `hm-juez-b` | Lector externo: no ve las decisiones ni la conversación | sonnet/high (otro proveedor si se usa OpenCode) | lectura | Solo la sección |
| `hm-tutor` | Preparar la reentrada y el teach-back | sonnet/medium | lectura | Expediente y última sesión |

**Selección por riesgo [decisión].** gentle-ai no elige modelo de forma dinámica, así que se emula. Cada pieza del encargo declara un riesgo:
- **R0:** forma.
- **R1:** prosa.
- **R2:** afirmación con fuente.
- **R3:** conclusión, cifra o cambio de premisa.

Una tabla rol × riesgo define qué subagente, qué variante y cuántos jueces se usan. Para subir de modelo se pasa el parámetro `model` en la invocación **[por probar]**; para subir de esfuerzo se usa una variante de archivo. Se escala una sola vez y luego decide el humano.

**Presets:** económico, equilibrado y riguroso. Un script reescribe el frontmatter según el preset elegido.

### 2.4 Herramienta `hm.py`

Usa solo la biblioteca estándar de Python.

| Comando | Qué hace |
|---|---|
| `hm impacto <ID>` | Lista lo que depende del ID |
| `hm cambios <ref>` | Diff semántico desde el frontmatter |
| `hm vistas` | Genera `vistas/tablero-criterios.md`, `mapa-decisiones.md`, `reentrada.md` y `deuda.md` |
| `hm check` | Valida IDs, enlaces y estados (también como hook `pre-commit` que avisa) |

### 2.5 Memoria (Engram, opcional)

- Por cada CRI o DEC confirmado se guarda un puntero con `topic_key = hm/<ID>`. Los DEC se guardan como `decision` y los CRI como `policy`, para que Engram los marque como "por revisar" con el tiempo.
- **Regla propia:** todo `supersedes` o `conflicts_with` que toque un CRI o DEC se lleva al humano. Esta regla va fuera de la sección que gentle-ai reescribe.
- Si Engram y el archivo no coinciden, gana el archivo.

## 3. Flujo proporcional y contactos humanos

```
Pedido ─► ¿pequeño? (se puede terminar con el pedido + el diff del documento) ─► directo, con Contrapeso
        └► grande ─► G1 Encuadre ─► Explorar ─► G2 Reacciones/criterios ─► Construir ─► Verificar ─► G3 Decisiones ─► G4 Cierre de etapa
                                      ▲                                       │
                                      └──────── criterio cambia ◄─────────────┘
Retomar (al volver) ─► pregunta de recuperación ─► reentrada.md ─► siguiente decisión
```

### Presupuesto de atención

| Tipo de contacto | Ejemplo | Tope |
|---|---|---|
| **G1 Generar** | Encuadre de 5 líneas e ideas propias, que quedan inmutables antes de que actúe `hm-divergente` | 1 por trabajo |
| **G2 Reaccionar y confirmar criterios** | +/–/? sobre tarjetas; aceptar o reescribir criterios | Lotes de ≤5 |
| **G3 Decidir** | Decisiones de peso alto, con opciones, descartes y Contrapeso | Lotes de ≤3 |
| **G4 Explicar o predecir** | Teach-back sobre decisiones de peso alto | ≤2 preguntas por etapa |
| **Alerta en caliente** | Choca con un criterio o decisión confirmados, cambia una premisa que toca varias secciones, o es irreversible | ≤1 por sesión |

**Contra la complacencia:** si la tasa de aceptación sin cambios supera el 85 %, el siguiente lote de decisiones se responde a ciegas y la recomendación se muestra después (M13). Cada decisión registra `generada_por` y `vista_recomendacion`.

## 4. Expediente: estructura y datos

```
proyecto/
  AGENTS.md  CLAUDE.md  .claude/{skills,agents}/  hamutay/agentes.yaml
  encargo.md            # S# literales, T#, Log (formato ODD; en modo acoplado, odd/tasks/<doc>.md)
  doc/                  # el documento o los documentos
  expediente/
    criterios/CRI-###.md     decisiones/DEC-###.md   supuestos/SUP-###.md
    secciones/SEC-###.md     opciones.yaml (OPC)     fuentes.yaml (FUE)
    afirmaciones.yaml (AFI, solo R2-R3)   reacciones.md   comprension.yaml (COM)
    sesiones/SES-AAAAMMDD.md   corridas.md
  ideas/*-humano.md     # ideación humana, inmutable
  vistas/               # generadas por hm.py
```

### Campos clave (frontmatter YAML)

- **`CRI-`:** `version`, `estado` (propuesto, confirmado o reemplazado), `cita_humana`, `origen` (pedido, reacción o edición), `ejemplo`, `contraejemplo`, `aplica_a: [SEC-]`, `reemplazado_por`.
- **`DEC-`:** `pregunta`, `opciones`, `elegida`, `descartes` (con su razón), `supuestos: [SUP-]`, `peso` (alto o normal), `generada_por` (humano, agente o conjunto), `vista_recomendacion`, `contrapeso`, `estado`.
- **`SEC-`:** `depende_de: [DEC-007, CRI-004@2]`, `riesgo`.

### Reglas de los datos

- Los enlaces solo apuntan hacia lo que sostiene al registro; lo inverso lo calcula `hm.py`.
- Si cambia el sentido de un registro, sube `version` y se anota `cambio:`. Si cambia el objeto, se crea un ID nuevo.
- Los commits llevan las líneas `Hm-Sesion:`, `Hm-Confirma:` e `Hm-Impacto:`.
- **Deuda de comprensión** = conteo de DEC y CRI de peso alto vigentes sin teach-back aprobado en su versión actual.
- **`corridas.md`:** qué se verificó, con qué criterio y con qué modelo (C6, C12).

## 5. Cobertura de huecos

| Hueco | Cómo se cubre |
|---|---|
| H1, H3, H4 Criterio vivo | `reacciones.md` → `hm-destilador` → CRI versionado con cita humana; alimenta `S#`, `hm-juez-a` y `aplica_a` |
| H2 Comprensión | `hamutay-cerrar`, `comprension.yaml` y `deuda.md` |
| H5 Reentrada humana | `hamutay-retomar`: pregunta de recuperación y `reentrada.md` (lo que decidiste, qué cambió, lo que probablemente olvidaste, la próxima decisión) |
| C2 Propagación y diff semántico | `depende_de`, `hm impacto` y `hm cambios`, más `hm-coherencia` |
| C1 Exploración | `hm-divergente` y tarjetas en `opciones.yaml` |
| Co-generación | G1 e `ideas/*-humano.md` inmutable; `generada_por` |
| Verificación sin tests | Riesgo R0–R3 y lentes de documento (evidencia, coherencia, legibilidad, robustez metodológica); jueces A y B |
| **No cubierto en v1** | H6 (contradicciones entre piezas paralelas, salvo `hm-coherencia`), H7 y H8 |

## 6. Riesgos y decisiones abiertas

- **[por probar]** Precedencia del glosario frente al ODD obligatorio.
- **[por probar]** Si `omitClaudeMd` o un mecanismo equivalente existe para aislar a los subagentes.
- **[por probar]** Si `model` se puede pasar por invocación.
- **[por probar]** Si Haiku acepta el parámetro de esfuerzo.
- **Ceremonia de registro.** Son 10 tipos de registro; hay que vigilar si sobran.
- **Confirmaciones que el agente puede escribir solo.** Nada impide hoy que el agente marque `confirmado_por` por su cuenta.
- **El agente asigna el `peso`**, y con eso decide qué se le pregunta al humano.
- **Umbrales conjeturales:** 85 %, ≤3, ≤5 y ≤2 deben calibrarse en el piloto GEE.
- **Errores correlacionados:** todos los jueces son modelos de Anthropic.
- **Costo de mantenimiento** de skills y subagentes propios.
- **Abierta:** un repo por entregable o un monorepo.
