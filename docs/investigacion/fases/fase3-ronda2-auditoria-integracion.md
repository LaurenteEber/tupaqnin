# Ronda 2 · Auditoría de integración y carga

*7 oct 2026 · Insumos: `arquitectura-v1.1.md`, las tres propuestas de la ronda 1 y `auditoria-A-ceremonia.md`. Criterio único: ¿la idea **reduce la carga humana neta** una vez sumada a las demás? Si una idea solo sirve sumada a otras, se fusiona. Si duplica a otra, se descarta.*

## 1. Conflictos y sinergias

| # | Cruce | Diagnóstico | Resolución |
|---|---|---|---|
| C1 | **Lazy consensus (T1.2)** vs. hook "`confirmado` exige `cita_humana`" | Conflicto: el contador "aplicado y leído sin objeción" lo llevaría el agente; vuelve la autoevaluación circular (auditoría A §3.3) | Sin contador. El CRI nace **`provisional` y se aplica**; solo los **críticos** necesitan un "sí" con cita. Los no críticos pueden quedar provisionales y se listan en la reentrada. El hook no cambia; la reacción +/– ya es cita (T1.1) |
| C2 | **Explicar antes de ver (T2.2) + recordar primero (T3.2) + pregunta antes de leer (T2.7) + defensa por proyección (T2.1)** | **Misma primitiva** (producir sin mirar y contrastar). Juntas dan 3–4 "exámenes" al día: ceremonia y Goodhart | Una primitiva, **"explicar sin mirar → contraste"**, en dos momentos: reentrada y sección con DEC crítica nueva. La proyección (T2.1) es su variante para pausas largas o entrega. T2.7 se descarta |
| C3 | **Read-back selectivo (T2.3) + muestreo (T2.4) + presupuesto de lectura (v1.1)** | T2.3 *es* el presupuesto renombrado; su "frase propia por párrafo" daría ~24 frases al día | Presupuesto **derivado de la hoja de afirmaciones**, no del redactor; se lee sin frase por párrafo (la producción ya la cubre C2). Más **1 párrafo de muestra al azar** por sección: un error ahí vuelve obligatoria toda la sección |
| C4 | **3 lectores con información distinta (T5.2)** vs. MVP con un solo `hm-lector` | El ciego no ve contradicciones con DEC; pero 3 lectores triplican los reportes | **2 lectores**: el ciego y **`hm-premisas`** (CRI/DEC + sección + fuentes), que absorbe al de fuentes y **llena las marcas** para que el redactor no se califique (T5.3). Al humano solo llegan `✗`/`?` críticos; el resto, a la cola |
| C5 | **Hoja de afirmaciones (T5.1) + esqueleto (T6.3) + enlaces sospechosos (T6.1)** | Mismo objeto. Riesgo: dependencias por afirmación multiplican la ceremonia de declarar | **Un archivo, `arg/<sec>.yaml`**: unidades (tesis, razón, supuesto, cifra, conclusión, recomendación) con `marca` y `usa` **opcional**. La sospecha es **por sección** (vía `depende_de`); `usa` solo **acota qué se relee**. El diff de esqueleto es el diff de este archivo |
| C6 | **Nota de reanudación (T3.1) + recordar primero (T3.2) + I-PASS (T3.3) + graduación (T3.4) + claves (T3.5)** | Cabe en `hamutay-retomar`; la síntesis del receptor de I-PASS duplica la nota | **Una skill**: nota al cerrar, sin síntesis, 4 bloques mostrados según la pausa |
| C7 | **Pre-mortem humano (T4.6) vs. disenso anclado en descartes (T5.5) vs. pre-mortem del lector (T5.7) vs. Contrapeso** | Tres señales contrarias para la misma DEC; el pre-mortem humano suma 1 contacto por DEC crítica | Contrapeso **=** mejor caso del descarte propio (T5.5); el pre-mortem lo hace el lector (T5.7). El humano, a v2 |
| C8 | **Triage en 3 carriles (T6.4) + banderas de tratamiento (T6.2) + enlaces sospechosos** | Sinergia: el triage limpia la sospecha y la bandera es su razón | Un solo flujo de `hm impacto` |
| C9 | **Dimensiones nombradas por el humano (T4.5) ↔ CRI semilla provisional (T1.2)** | Sinergia: la respuesta de G1 siembra ejes y CRI sin un contacto extra | Se adopta |
| C10 | **Hook `Stop` = `hm check` bloqueante (T6.1)** | `Stop` corre **en cada turno**: bloquear ahí interrumpe todas las respuestas | `Stop` solo **informa**; el bloqueo va a `PreToolUse` al escribir `estado: lista`. La nota la propone la skill (`SessionEnd` no interactúa); `PreCompact`/`SessionEnd` guardan `ultimo.yaml` |

## 2. Presupuesto de contactos (simulación)

Supuestos: una **pausa de 5 días**; en el delta GEE v6→v7 se redactan o ajustan 3 secciones de ~1.500 palabras (el cambio del indicador 3.2), con 6 DEC, de las cuales 4 son críticas, y 3 pedidos pequeños. Contacto = cada vez que el humano tiene que responder o decidir algo.

### Día GEE v6→v7

| Momento | MVP v1.1 | Combinación ingenua (todos los top-3) | Selección integrada (§3) |
|---|---|---|---|
| Retomar | Recuperación + `reentrada.md` + defensa: **3** · 600 p · 8 min | Recuerdo + I-PASS + síntesis + proyección + marca de impacto: **4** · 700 p · 10 min | Recuerdo libre + reentrada de 4 bloques (lo olvidado primero, diff de esqueleto): **2** · 350 p · 5 min |
| Exploración (indicador 3.2) | 5 tarjetas: **1** · 600 p · 6 min | Tabla de contraste: **1** · 250 p · 4 min | Igual: **1** · 250 p · 4 min |
| Confirmar CRI | ≤5: **1** · 500 p · 5 min | Solo críticos: **1** · 200 p · 2 min | Va dentro del lote G3: **0** · 150 p · 1 min |
| Leer secciones | Presupuesto (~40 %): **3** · 1.800 p · 15 min | Explicar antes ×3 + read-back con frase por párrafo (~24) + muestra + hoja: **~30** · 2.400 p · 25 min | Presupuesto derivado de la hoja (~20 %) + 1 muestra por sección + explicar sin mirar solo en la sección con DEC crítica nueva: **4** · 1.300 p · 14 min |
| DEC críticas | 2 lotes: **2** · 1.800 p · 15 min | 2 lotes + Contrapeso de descartes + pre-mortem humano ×4: **6** · 1.500 p · 16 min | 2 lotes con Contrapeso de descartes: **2** · 1.100 p · 11 min |
| Dudas de lectores | Va a la cola, se lee en la reentrada | 3 lectores × 3 secciones: **1** · 600 p · 5 min | Solo `✗`/`?` críticos: va dentro de la lectura · 150 p |
| Propagación y alerta | **1** · 200 p · 2 min | Sospechosas + diff: **1** · 200 p · 2 min | Igual (el carril 3 va a G3): **1** · 150 p · 2 min |
| Pedidos pequeños | **3** · 900 p · 6 min | **3** · 900 p · 6 min | Carriles 1–2 sin contacto; resultado: **3** · 600 p · 5 min |
| Cierre | — | Nota: **1** · 0,5 min | Nota: **1** · 0,5 min |
| **Total** | **~14 · ~6.400 p · ~57 min** | **~48 · ~6.750 p · ~70 min** | **~14 · ~4.050 p · ~43 min** |

### Día de exploración de nichos

| | MVP v1.1 | Combinación ingenua | Selección |
|---|---|---|---|
| Total | 6 contactos (G1, 3 lotes de 15 tarjetas, confirmar, retomar) · ~2.500 p · 35 min | 8 (+ pre-mortem, + explicar antes de ver, + pares T4.8) · ~1.300 p · 35 min | **5** (G1 con dimensiones, 2 tablas de contraste, 0–1 CRI crítico, nota) · **~800 p · 25 min** |

### Dónde se excede la combinación ingenua

1. **Read-back con frase propia por párrafo:** ~24 microcontactos, es el mayor exceso.
2. **Tres pruebas de comprensión en un día:** recuerdo, explicar antes de ver y proyección, más la pregunta previa.
3. **Pre-mortem humano** en cada DEC crítica.
4. **Síntesis I-PASS** duplicada con la nota de reanudación.
5. **Reportes de 3 lectores** por sección.

En palabras leídas la combinación ingenua apenas cambia frente a v1.1, pero **triplica los contactos**. La selección conserva los contactos y baja las lecturas **~37 %** en GEE y **~68 %** en la exploración. *Estas cifras son estimaciones y hay que calibrarlas en el piloto.*

## 3. Selección integrada

### Mecanismos nuevos del MVP (8)

| # | Mecanismo | Fusiona | Por qué entra |
|---|---|---|---|
| M1 | **`arg/<sec>.yaml`**: unidades argumentales con marca `✓ ≈ ? ✗`, quién marcó y `usa` opcional | T5.1 + T6.3 | Es el objeto del que salen la lectura, el diff, la propagación y el expediente (C6/C12) |
| M2 | **Lectura presupuestada derivada + 1 muestra al azar** | T2.3 + T2.4 (sustituye la regla de v1.1) | Es lo único que *reduce* la lectura del documento, y la muestra pone a prueba el presupuesto |
| M3 | **Enlaces sospechosos con triage en 3 carriles y bandera de tratamiento** | T6.1 + T6.4 + T6.2 | La propagación pasa a ser un estado verificable. Solo el carril 3 llega al humano |
| M4 | **CRI provisional aplicado; "sí" con cita solo si es crítico** | T1.2 sin contador | Es lo que más contactos quita (G2) sin autoevaluación |
| M5 | **Distinguir o revocar** como única pregunta de versionado | T1.4 | Le da sentido a `@v` y a `hm impacto`. Usa la alerta ya presupuestada |
| M6 | **Explicar sin mirar → contraste** (lo omitido entra al presupuesto); variante de proyección | T2.2 + T3.2 + T2.1 | Tiene la mejor evidencia (g≈0,55), es una sola primitiva y no lleva nota |
| M7 | **Reentrada graduada:** nota de reanudación con cita, claves contextuales y 4 bloques fijos | T3.1 + T3.4 + T3.5 + T3.3 sin síntesis | Cuesta 30 s al cerrar y es proporcional a la pausa |
| M8 | **G1 ampliado + tabla de contraste**: ideas y dimensiones del humano primero, con un hook que lo impone | T4.1 + T4.5 + T4.4 | Combate el anclaje y sustituye leer N tarjetas por leer una tabla |

### Ajustes de prompt o plantilla (no son mecanismos)

- Gesto → CRI (T1.1).
- Preguntas rojas a la cola (T1.3).
- Expansión desde la idea humana y eje obligatorio por tarjeta (T4.2, T4.3).
- Contrapeso = mejor caso del descarte (T5.5).
- Pre-mortem en el prompt del lector (T5.7).
- El redactor no marca `✓` en lo suyo, y el informe declara el proveedor (T5.3).
- Aviso léxico de dependencias no declaradas en `hm check` (T6.5, versión léxica).

### Protocolo del piloto, una sola vez

- Errores sembrados en una copia de 2 secciones para medir el recall de los lectores (T5.4).

### Pasan a v2

Deriva contada (T1.5) y anclas (T1.6), si el piloto supera ~10 CRI · encargo desde CRI (T1.7) · atenuar texto (T2.5; el MVP usa `▶ leer`) · `comprension.yaml` (T2.6) y repaso espaciado (T3.6) · pre-mortem humano (T4.6) · diseño por conjuntos (T4.7) y pares (T4.8) · lector de fuentes separado (T5.2) · GRADE (T5.6) y ACH (T5.8) · dependencias semánticas (T6.5) · TMS (T6.6, requiere SUP) · cruce de secciones (T6.7, si hay >5 secciones).

### Se descartan

Contador "aplicado y leído sin objeción" (circular) · read-back con frase por párrafo (~24 contactos/día) · pregunta antes de leer T2.7 (duplica M6) · síntesis I-PASS (duplica la nota) · panel fijo de 3 lectores (9 jueces ≈ 2 votos, Apple).

**Recuento:** 3 skills, **3 subagentes**. El tercero, `hm-premisas`, se justifica porque sin él el redactor pondría las marcas de M1 sobre su propio texto (autopreferencia). Además reemplaza al lector de fuentes y al de coherencia que v1.1 dejaba para v2.

## 4. Especificación compacta del MVP v1.2

### Skills

| Skill | Flujo |
|---|---|
| `hamutay-explorar` | G1: 3–7 ideas en viñetas (≤10 min) + "¿qué distingue una buena opción?" → `ideas/*-humano.md` inmutable; da ejes y CRI semilla `provisional`. `hm-divergente`: variantes de cada idea humana (`deriva_de`), luego propias, con eje obligatorio. **Tabla de contraste** (celdas ≤8 p.) → +/–/? → CRI provisional con ejemplo/contraejemplo. Descartes en la tarjeta |
| `hamutay-construir` | Redacta citando CRI/DEC; escribe `arg/<sec>.yaml` en `?`. Invoca `hm-premisas` y `hm-lector`. Entrega `hm leer <sec>` (`?`/`✗`, `≈` críticas, lo que cambió, 1 muestra). Con DEC crítica nueva: explicar sin mirar → contraste; lo omitido entra al presupuesto. DEC y CRI críticos en lotes G3 ≤3 con Contrapeso del descarte. Contradicción con CRI → "¿distingue o revoca?". Cambio → `hm impacto` + triage |
| `hamutay-retomar` | **<3 días**: `proxima.md` + última cita. **3–14**: + recuerdo libre + 4 bloques (estado · lo no recordado · lo que cambió, diff de esqueleto · cola y próxima decisión). **>14 o entrega**: + 1 pregunta de proyección contrastada con `hm impacto`, calificada por el humano. Al cerrar: nota "cuando vuelva, primero…" |

### Subagentes

Los tres llevan `omitClaudeMd: true` y reglas cargadas con `skills:`. Ninguno ve la conversación.

| Subagente | Modelo | Ve | Produce |
|---|---|---|---|
| `hm-divergente` | sonnet/medium | Encuadre, ideas, ejes | Tarjetas |
| `hm-lector` | opus/high, o de otro proveedor | Solo la sección | Dudas, con cobertura explícita, y pre-mortem. Informa su proveedor |
| `hm-premisas` | opus/high | Sección, `arg`, CRI/DEC vigentes, fuentes | Marcas `✓≈?✗` con `marcado_por`, contradicciones con DEC, revisión de que el esqueleto coincida con el texto |

### Registros

| Registro | Contenido |
|---|---|
| `CRI-###` | `regla`, `ejemplos`, `contraejemplo`, `preguntas`, `no_aplica_si`, `estado: provisional\|confirmado\|reemplazado`, `reemplazado_por`, `cita_humana`, `version`, `aplica_a` |
| `DEC-###` | Igual que en v1.1. `contrapeso` = mejor caso del descarte + evidencia nueva, o "nada nuevo" |
| `doc/<sec>.md` | Frontmatter con `depende_de` y `estado: borrador\|sospechosa@ID\|lista` |
| `arg/<sec>.yaml` | Unidades (`id`, `tipo`, `texto`≤25 p, `parrafo`, `marca`, `marcado_por`, `usa`?) |
| `opciones.yaml` | Con `ejes`, `deriva_de`, `descartada_por` |
| `encargo.md` | En formato ODD |
| `sesiones/` | `proxima.md` (con cita) y `ultimo.yaml` (sección, DEC en curso, última reacción literal) |

### Hooks

| Hook | Regla |
|---|---|
| `PreToolUse` (Write/Edit) | Bloquea editar `ideas/*-humano.md`. Bloquea escribir `opciones.yaml` si no existe el archivo humano del trabajo. Bloquea `confirmado` sin cita en el transcript **[por probar]**. Bloquea `estado: lista` si `hm check` encuentra sospechosas, `?`/`✗` en unidades críticas, un CRI crítico todavía provisional o `marcado_por` igual al redactor |
| `SessionStart` | Calcula la pausa y propone `hamutay-retomar` en el nivel que corresponde |
| `Stop` | `hm check` **solo informativo**: IDs, versiones, sospechosas y aviso léxico de dependencias no declaradas |
| `PreCompact` y `SessionEnd` | Guardan `ultimo.yaml` sin interacción |

### `hm.py`

Usa solo la biblioteca estándar.

| Comando | Qué hace |
|---|---|
| `hm impacto <ID>` | Marca sospechosas y lista el carril (1–3) y la bandera de tratamiento (`aplica`/`distingue`/`contradice`) |
| `hm check` | Valida IDs, enlaces, versiones y los estados que bloquean |
| `hm leer <sec>` | Devuelve el presupuesto de lectura con la muestra |
| `hm diff <sec> vA vB` | Diff de `arg/` emparejado con `difflib` |
| `hm reentrada` | Genera la vista de reentrada |

### Métricas que se añaden al piloto

- Recall de los lectores con errores sembrados.
- Errores encontrados en la muestra (si es mayor que 0 de forma repetida, el presupuesto está mal calibrado).
- Contactos y palabras por día frente a la tabla de §2.
- Proyección y recuerdo separados en la defensa a la semana.

### Riesgos abiertos

- La extracción de `arg/` puede omitir unidades. Mitigación: la revisión de esqueleto contra texto de `hm-premisas` y la muestra al azar.
- El cruce del recuerdo libre contra las DEC puede fallar. Mitigación: el humano corrige el cruce.
- Un CRI provisional puede quedar olvidado. Mitigación: aparece listado en la reentrada.
