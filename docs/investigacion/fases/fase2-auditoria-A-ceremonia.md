# Auditoría A: ceremonia y cuello de botella en hamutay v1

*Auditor adversarial · 7 oct 2026 · Lee `arquitectura-v1.md` con `encuadre-5-artefactos.md` y el acervo §9.*

**Veredicto:** la v1 tiene buena tesis y buenas piezas, pero, en su tamaño actual, reproduce el error de SDD. Sustituye "releer el documento" por "leer el registro" y el humano **sigue teniendo que leer el documento**, porque firma (C6). Se puede probar la tesis con cerca de un tercio de la maquinaria.

## 1. Inventario de ceremonia

| Pieza | Cantidad en v1 |
|---|---|
| Tipos de registro | 9 con ID (CRI, DEC, SUP, SEC, OPC, FUE, AFI, COM, SES), más `reacciones.md`, `corridas.md` e `ideas/`; DES aparece en el encuadre 5 |
| Vistas generadas | 4 (tablero, mapa, reentrada, deuda) |
| Skills | 8 |
| Subagentes | 9, más la variante `hm-redactor-nucleo` |
| Tipos de contacto humano | 5 (G1–G4 y alerta), con 4 topes numéricos y una regla del 85 % |
| Niveles de riesgo | 4 (R0–R3), una tabla rol × riesgo y 3 presets con un script que reescribe el frontmatter |
| Modos de convivencia con gentle-ai | 2, que dependen de una prueba de humo |
| Convenciones git | 3 *trailers*, 9 tipos de commit y una etiqueta base |
| Comandos de `hm.py` | 4, más un hook |
| Reglas de Engram | 3 |

Son unos 50 conceptos con nombre que el usuario debe conocer para entender por qué el agente le pregunta lo que le pregunta. La sección 6 de la propia arquitectura ya lo reconoce ("hay que vigilar si sobran"). La respuesta es que sí sobran.

### MVP que todavía prueba la tesis

La tesis tiene tres patas: (a) co-generación en los momentos clave, (b) criterio y decisión como evidencia versionada con señal contraria y (c) decidir y defender sin releer.

| Conservar (núcleo) | Fusionar | Posponer a v2 |
|---|---|---|
| `ideas/*-humano.md` inmutable y el encuadre de 5 líneas (a) | SUP y la lista de descartes **dentro de DEC** | AFI, FUE y `hm-verificador` como registro: en v1 se marca `[no verificado]` en línea y el Contrapeso lo enumera |
| `CRI` con `cita`, `version`, `contraejemplo` y `estado` (b) | OPC y reacciones en **un solo** `exploracion.md` (tarjetas más +/–/? con cita) | COM y `deuda.md`: el teach-back queda como una línea `defendida: v2` dentro de DEC/CRI |
| `DEC` con opciones, descartes, supuestos y Contrapeso (b) | SES y `corridas.md` → `git log` más una línea por sesión en `bitacora.md` | Presets, R0–R3 y la tabla rol × riesgo: basta una sola marca, `critico: sí/no` |
| `depende_de` en el frontmatter de cada sección de `doc/` y `hm impacto` / `hm check` (c) | `hamutay-cerrar` dentro de `hamutay-retomar`: la pregunta de recuperación **es** el teach-back | Modo acoplado con gentle-ai y la integración con Engram |
| `reentrada.md` ordenado por decisiones (c) | `hm-destilador` y `hm-tutor` → la sesión principal, que tiene el contexto de la conversación | `hm-coherencia`, `hm-juez-a`, `hm-redactor-nucleo`, la regla del 85 % y los *trailers* git |

El MVP queda así:
- **3 skills:** `explorar` (encuadre, ideas, tarjetas, reacciones y criterios), `construir` (redactar con Contrapeso y proponer DEC) y `retomar`.
- **2 subagentes:** `hm-divergente` (sonnet) y `hm-lector-externo` (juez B, ciego; de otro proveedor cuando sea posible).
- **2 tipos de registro:** CRI y DEC.
- **2 comandos:** `hm impacto` y `hm check`.
- **Modo aislado** con respecto a gentle-ai.

Justificación: cada pieza que se conserva corresponde a uno de los tres "construir" del acervo §9 (criterio vivo, comprensión y propagación). Lo demás es un mecanismo "adoptar ya" que no necesita infraestructura, o es optimización de costo prematura para un único usuario.

## 2. ¿Reduce o traslada el cuello de botella?

### Día GEE v6→v7 con la v1 completa

| Momento | Interacción humana | Lectura estimada |
|---|---|---|
| Retomar: pregunta de recuperación y `reentrada.md` | 1–2 | 400–800 palabras |
| G1: encuadre e ideas propias (solo el primer día) | 1 de redacción | — |
| G2: tarjetas para unas 3 preguntas abiertas × 5 → 3 lotes de reacción | 3 | 15 tarjetas × ~120 = ~1.800 |
| G2: confirmar o reescribir criterios (≤5, con ejemplo y contraejemplo) | 1 | ~600 |
| Pedidos pequeños "directos, con Contrapeso" (unos 4 al día) | 4 | Diff más Contrapeso, ~1.200 |
| G3: decisiones en lotes de ≤3 (unas 2 tandas) | 2 | 6 × ~300 = ~1.800 |
| Alerta en caliente | 1 | ~200 |
| G4: teach-back | 1 | ~200 |
| **Prosa de las secciones redactadas que firmará** | **No está presupuestada** | 3 secciones × ~1.500 = 4.500 |

**Total: 13–15 interacciones y unas 10.000 palabras al día.** El registro no reemplaza la lectura del documento: se suma a ella. El diseño nunca dice cuándo lee el humano la prosa ni qué parte puede saltarse con seguridad. Esa es la omisión más grave, porque es justamente lo que la tesis promete ahorrar ("sin releerlo todo").

**Dónde se atasca:**
1. **Los topes sin regla de desborde.** ¿Qué pasa con la cuarta decisión de peso alto, o con la segunda alerta de la sesión? Si se encola, el trabajo se bloquea. Si no se encola, el agente aprende a bajar el `peso` para no superar el tope. El presupuesto de atención pasa a ser un incentivo para ocultar decisiones.
2. **La frontera entre pequeño y grande la juzga el agente.** El caso típico de v7, "cambia el indicador 3.2", parece pequeño pero toca un CRI. No existe un tamaño medio.
3. **El reconstruir v6.** Si se intenta registrar las decisiones de v6 hacia atrás, el arranque cuesta días. Hay que prohibirlo: el registro empieza en el delta v6→v7.
4. **Word.** Las contrapartes de CEPLAN comentan en .docx. Partir el documento en `SEC-*.md` y volver a exportarlo es fricción diaria que no aparece en el diseño.

### Exploración de nichos

En este caso el diseño encaja mejor: 3 ideas propias, 15 tarjetas, reacciones, 2–3 criterios y descartes. Son unas 5 interacciones y unas 2.500 palabras. Sobran el encargo S#/T#, SEC, los jueces, AFI y los presets. Aparece un **hueco**: la arquitectura v1 elimina DES y pone los descartes dentro de DEC, pero en la exploración todavía no hay DEC. Por eso "nueve nichos descartados por CRI-002, reabribles" (encuadre 5, §4.8) no tiene dónde vivir. En el MVP, el estado `descartada_por: CRI-002` va en la tarjeta de `exploracion.md`.

## 3. Coherencia interna

1. **Los jueces se anulan entre sí.** "Solo se corrige lo que ambos confirman" viene de Judgment Day, donde los dos jueces aplican la misma lente. Aquí A juzga la rúbrica y B lee como externo, sin ver criterios. Por diseño ven cosas distintas, así que la intersección queda casi vacía y no se corrige casi nada.
2. **El principio 5 contradice las compuertas.** El principio dice que no hay compuertas de "¿apruebas?", pero G2 ("aceptar o reescribir criterios"), `confirmado_por` y `Hm-Confirma` son aprobaciones con otro nombre. O se acepta que lo son o se reemplazan por una edición humana.
3. **Autoevaluación en cadena.** El agente decide el `peso`, el `riesgo`, si algo es pequeño o grande, si un cambio es de "sentido" o de "objeto" para versionar, el resultado ok/dudó/falló del teach-back, `confirmado_por` y los *trailers*. La deuda de comprensión se mide **solo sobre lo que el agente marcó como alto**. Es una métrica circular.
4. **Ambigüedad en SEC.** La arquitectura tiene `expediente/secciones/SEC-###.md` y además `doc/`. No queda claro si SEC es la sección o su ficha. En el encuadre 5, SEC es el archivo de `doc/`. Hay que elegir una sola opción.
5. **Decisiones construidas sobre supuestos sin probar.** La tabla rol × riesgo depende de pasar `model` en cada invocación, y eso está [por probar]. El modo acoplado depende de la prueba de humo. `reentrada.md` necesita `visitas.yaml` (encuadre 5), que la arquitectura no incluye.
6. **Dos canales de captura del criterio:** `reacciones.md` → destilador (arquitectura) frente a `Log L#` → extractor (encuadre 5). Hay que quedarse con uno.
7. **El regreso a Explorar no está definido.** Cuando el criterio cambia, ¿se vuelve a abrir G1? ¿Cuentan de nuevo los topes?
8. **La regla de Engram rompe el presupuesto.** "Todo `supersedes` se lleva al humano" no está contada en ningún tope.
9. **La regla del 85 % no tiene base estadística.** Con lotes de ≤3, la tasa oscila entre 0, 33, 67 y 100 %. Además, aceptar mucho puede ser legítimo.

## 4. Riesgos, ordenados por probabilidad × impacto

| # | Riesgo | P × I | Prueba más barata |
|---|---|---|---|
| 1 | **Abandono por ceremonia:** a las 2 semanas el registro deja de leerse y queda como decoración | Alta × Alta | 5 sesiones reales del delta GEE con el MVP. Medir minutos humanos por sesión, registros creados frente a abiertos por el humano y si el usuario lo usaría sin estar obligado |
| 2 | **Traslado, no reducción:** se lee registro **más** documento | Alta × Alta | Mismo cambio (un capítulo) hecho dos veces: a) revisando el documento a secas; b) con hamutay. Contar palabras leídas y, una semana después, una prueba de defensa de 5 preguntas |
| 3 | **Señales corrompidas por autoevaluación** (peso, confirmado, ok/dudó) | Alta × Media-alta | Auditar 20 registros: ¿cuántos `confirmado` tienen una cita literal en el transcript JSONL de Claude Code? Sembrar 3 decisiones críticas y ver si salen marcadas como alto |
| 4 | **`depende_de` desactualizado:** la propagación da falsos negativos y una sensación de seguridad que no está justificada | Media × Alta | Cambiar CRI-004 y comparar `hm impacto` con una lectura completa del documento hecha por un LLM |
| 5 | **Choque con ODD:** el agente produce artefactos con forma de código | Media × Media | Cuesta cero si se elige el modo aislado. La prueba de humo solo se justifica si después se quiere el acoplado |
| 6 | **Jueces correlacionados o con baja exhaustividad** | Media × Media | 10 secciones de v6 con hallazgos conocidos (encuadre 4): medir la exhaustividad del lector externo frente a la del usuario |
| 7 | **Mantenimiento** de 17 piezas propias frente a los cambios de gentle-ai y Claude Code | Media × Media | Se mitiga con el MVP: 5 piezas |

## 5. Lo que está bien y no debe perderse

- **Ideación humana primero e inmutable (M12).** Es la única mitigación con evidencia contra el anclaje y es barata.
- **Un CRI con cita literal, versión y contraejemplo.** Es el aporte original y el corazón de H1, H3 y H4.
- **El bloque Contrapeso (M43)** en todo, también en lo directo.
- **El archivo gana a Engram.** El expediente en git es la fuente de verdad; los enlaces inversos los calcula `hm.py`, que solo usa la biblioteca estándar.
- **Una reentrada ordenada por decisiones y por lo olvidado**, no por ediciones (H5).
- **El flujo proporcional** como principio y "solo se toca lo que `hm impacto` marca".
- **Una sola ronda de corrección y luego se escala.** Los estados públicos son simples.
- **El lector externo ciego (juez B)**, que es lo más parecido al revisor real de CEPLAN.
- **Etiquetar `v6-base`** para tener diff textual y semántico desde el primer día.
- **El presupuesto de atención como concepto**, siempre que tenga una regla de desborde honesta.

## Recomendaciones de cambio, ordenadas

1. **Recortar al MVP:** 3 skills, 2 subagentes, CRI y DEC, `hm impacto` y `hm check`, modo aislado. Lo demás pasa a v2 y entra solo si el piloto lo pide.
2. **Presupuestar la lectura del documento.** Por sección, el agente entrega un "qué leer": los párrafos que dependen de una DEC o un CRI críticos o que tienen `[no verificado]`. El resto se declara como lectura opcional. Medir si con eso el usuario puede defender el texto.
3. **Quitar la autoevaluación de las señales que gobiernan al humano.** `critico` se determina por reglas mecánicas (toca un CRI confirmado, cambia una cifra o conclusión, o `hm impacto` ≥2 secciones), y el humano puede subirlo. Una confirmación solo es válida si la `cita` aparece en el transcript; lo comprueba `hm check`. El teach-back lo califica el humano, no el agente.
4. **Regla de desborde explícita:** lo que excede el tope pasa a una cola visible en `reentrada.md`, nunca se degrada a normal en silencio. Se elimina la regla del 85 % y solo se registra `vista_recomendacion`.
5. **Arreglar las incoherencias:** el veredicto del juez B se presenta como "dudas del lector" y no se intersecta con A; SEC es el archivo de `doc/`; el descarte va en la tarjeta de exploración; hay un único canal de captura del criterio; y no se reconstruye v6, el registro arranca en el delta v6→v7.
