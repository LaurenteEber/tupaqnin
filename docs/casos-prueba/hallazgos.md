---
titulo: Casos de prueba · registro de auditoría
fecha: 2026-10-26
relacion: qué se hizo con cada hallazgo al construir casos-bloque-1.md
---

# Casos de prueba · registro de auditoría

## Proceso

1. **Redacción:** tres analistas trabajaron en paralelo (casos 1–3, 4–7 y 8–10), con un brief común. El brief traía el pedido de Eber en literal, DEC-002 y DEC-003, el estado de cada regla y un formato fijo.
2. **Auditoría adversarial en dos flancos:**
   - *Fidelidad a las reglas:* 38 hallazgos (3 críticos, 9 altos, 18 medios y 8 bajos).
   - *Problema y jurado:* 20 hallazgos (6 altos y ninguno crítico), más el análisis de plausibilidad, de variedad y un ranking.
   - Los críticos y los altos se verificaron contra el texto, y todos se sostuvieron.
3. **Disposiciones transversales (D1–D14):** fijaron, entre otras cosas, quién quita un marcador (rige B3 §8), la predicción oculta, la ausencia de jerga interna, una sola tarjeta por conflicto y la variedad de actos. También cambiaron casos de la matriz:
   - en el caso 4 conduce el ingeniero y el agente audita;
   - el caso 5 incluye una ronda que no avanza;
   - el caso 10 incluye el camino feliz de la puerta (unos 2 minutos).
4. **Edición:** tres editores corrigieron los casos. Cada uno verificó los hallazgos antes de aplicarlos. Rechazos:
   - P-15, en parte: se conserva `[POR ACLARAR]`, porque es lo que muestra el producto;
   - la variante de P-05 en que edita un colega: el diseño no contempla varios usuarios;
   - las tarjetas de dos desenlaces de P-11 y A-31: D8 las reserva para los casos 2 y 4.
5. **Verificación independiente de la v2:**
   - de los 3 críticos y 15 altos, todos quedaron resueltos salvo P-04, que queda parcial (el bloque 1 exige la segunda tarjeta en el caso 6);
   - encontró 1 problema nuevo alto (V-01: el caso 3 decía que la tabla se firmaba estando en borrador), 4 medios y 7 bajos;
   - se aplicaron V-01, V-02, V-03, V-04, V-08, V-09, V-10, V-11 y V-12;
   - V-05 (tarjeta de los hallazgos del juez), V-06 (acto o decisión en B3 §8) y V-07 (etiqueta del marcador) pasaron a «Lo que destapan los casos», porque los resuelve el diseño, no los casos.
6. **Nombres:** los editores repitieron «Inés» en los casos 1, 5 y 9. Ahora son Inés (1), Paula (5) y Natalia (9).

**Ingrediente débil que queda:** en el caso 7, la comprensión de las 4 decisiones provisionales al volver depende del bucle de comprensión, que está por diseñar.

---

## Registro de cambios (casos 1–3)

| Hallazgo | Aplicado / ajustado / rechazado | Qué cambió o por qué no |
|---|---|---|
| A-01 | Aplicado | Caso 1, paso 2: la lectura del boceto vuelve con un caso de respuesta libre, sin «¿Es así?», y la predicción se revela después. Tipo de acto marcado «por diseñar» |
| A-02 | Aplicado | Caso 1, paso 3: dos hechos puntuales en línea (doc 12 §6.2, §5.2 D1), sin tarjeta de ronda ni investigadores |
| A-03 | Aplicado | Caso 1, paso 4: se quitan doc 12 §2.3 y §9.3; se cita §1.6 (predicción guardada antes de mostrar) como `[inferencia]` y el formato queda por diseñar |
| A-04 | Ajustado | El hallazgo se sostiene (3 frente a 7 palabras), pero la tarjeta de dos desenlaces de los parasoles sale por D8: la decisión la escribe Inés con sus palabras (§1.3) |
| A-05 | Aplicado | Resultado 1 del caso 1: el choque se marca `[inferencia]`, y se agrega la salida «criterio del agente sin confirmar», que bloquea (§1.7) |
| A-06 | Aplicado | Caso 2: el «en vez de» de los supuestos es «extender el horario en todas las cunas», alineado con el segundo desenlace. El supuesto de turno contra horas extra queda en la explicación, y su falta de red va como pregunta abierta nueva |
| A-07 | Aplicado | Caso 2, «En el hackathon»: se quita «toda cifra conserva su acto (doc 12 §4.1)»; queda «las cifras con vínculo sí entran (§1.10)» |
| A-08 | Aplicado | Caso 3, paso 1: el plan previo se marca `[inferencia]` y módulo 6 |
| A-09 | Aplicado | Caso 3, paso 2: «¿Cuenta con tu regla?» pasa a «¿Cómo lo juzgas?», con respuesta libre |
| A-10 | Aplicado | Caso 3, paso 2 y resultado 5: «no se pudo comprobar» aplicado a una lectura, marcado `[inferencia]` |
| A-11 | Aplicado | Caso 3, paso 3: se marca `[inferencia]` que podría nacer como propuesta; pregunta abierta nueva sobre sus efectos distintos |
| A-12 | Aplicado | Caso 3: las 80 páginas son solo lo que daría un chat. Paso 6: ≤ 2k por encargo, partido si excede. P3.6 se reescribe |
| A-16 (formato) | Aplicado como referencia | Los teach-back de los casos 1 y 3 siguen ocultando la predicción, según D2 |
| A-38 | Ajustado | Su corrección es la del caso 7 (A-23). En el caso 2 se conserva la pregunta del predecir sin el ejemplo de 3 veces |
| C-1 | Aplicado | Pregunta abierta del caso 1 ampliada: también deja sin vía los hechos puntuales en línea |
| C-3 | Aplicado | Pregunta abierta del caso 2: los tres cambios al guion de la demo |
| C-4 / D1 | Aplicado | Caso 3, paso 4 y resultado 3: rige B3 §8 (la app levanta el marcador; el vínculo resuelto lo define §1.8). La pregunta del choque B1 §1.8 frente a B3 §8 no se repite aquí: va una sola vez, donde más se nota (casos 6, 7 o 10) |
| P1.1 | Conservada | Hueco real |
| P1.2 | Quitada | Error del autor: la corrección la escribe el agente, con la cita de Inés (§1.9). Pasa al resultado 5 |
| P1.3 | Reescrita | Subir no basta (§1.6); queda solo el choque entre fuentes: doc 12 §4.1 frente a §1.6, sin reemplazo listado en §1.14 |
| P1.4 | Reescrita | Se reduce a lo que falta en §1.2: el formato y el ocultamiento de la predicción |
| P1.5 / P3.7 | Movidas (D7) | Al caso 4; aquí, una línea que remite a él |
| P1.6, P1.7 | Conservadas | Huecos reales |
| P2.1 | Reescrita | Sin el ejemplo de 3 veces (ver A-38) |
| P2.2 / D6 | Reescrita | Pregunta común de D6, con el matiz de P2.2: el botón «No estoy de acuerdo» rechaza sin razón |
| P2.3, P2.4 | Conservadas | P2.3 se amplía con C-3 |
| P2.5 | Reescrita | Se acota a qué son las «secciones afectadas» de un predecir |
| P3.1, P3.2, P3.4, P3.5 | Conservadas | Huecos reales; P3.2 ahora la destapa el contrato testigo |
| P3.3 | Reescrita | Error parcial del autor: la respuesta queda en el acto (A1.2), no como ejemplo. Lo real: el matiz no llega a los lectores |
| P3.6 | Reescrita | Se quita el error de 40k (se parte) y se agrega el costo de muchos encargos partidos |
| P-09 | Ajustado | La lectura equivocada es del criterio, no de la física. La que propone la auditoría («en horario de clase») chocaba con la respuesta de Inés; se usó «evitar el sol que calienta» |
| P-10 / D6 | Ajustado | La narración no cambia (D6); va la pregunta común en el caso 2 |
| P-13 / D11 | Aplicado | Ficha del caso 2: «cunas = guarderías públicas», S/ y US$ |
| P-14 / D11 | Aplicado | Ficha del caso 3: «cambio de control» y «due diligence» explicados |
| P-17 / D7 | Aplicado | Caso 1, paso 6: las medidas normativas salen del estado visible |
| P-19 / D10 | Aplicado | Lucía (caso 1) pasa a Inés. El caso 5 conserva Lucía |
| Auditoría jurado §1 (caso 1: tono de examen, cuatro actos, «teach-back» en la narración, celdas largas) | Aplicado | El paso 6 es una pregunta de defensa ante la municipalidad; sale la tarjeta de parasoles y el «¿Es así?»; nada de «teach-back» fuera de la columna de regla; celdas ≤ 50 palabras (D13) |
| Auditoría jurado §1 (caso 3: pasos densos) | Aplicado | Se reparte en 6 pasos, con el detalle en «Cómo se asegura» |
| §3 caso 1, sol | Aplicado | «De marzo a septiembre… antes de las 8:00 y después de las 16:30» pasa a «en verano, temprano y al final de la tarde» |
| §3 caso 2, jornada | Aplicado | «Una jornada regular de 12 horas no es viable para el personal de cuidado», sin entrar en el régimen laboral |
| §3 caso 3, US$ 2 M | Aplicado | «Al que le quedan 5 años» |
| §3 caso 3, euros | Aplicado | La conversión queda en la decisión del tipo de cambio, sin la frase ambigua |
| §4 caso 2 | Ajustado | «Ficticio» queda en la ficha. No se puede comprobar desde aquí que no se reuse material del curso de evaluación de políticas: queda para Eber |
| §5 / D9, caso 3 | Aplicado | Contrato testigo como aporte de Rosa (§1.6, doc 12 §4.1): es lo que destapa la interpretación del agente |
| D2 | Aplicado | Casos 1 y 3: «Antes de seguir, quiero comprobar que entendí bien tu regla»; el caso va primero, con respuesta libre; la predicción se revela después |
| D3 | Verificado | Caso 2: 12 frente a 120 es orden de magnitud, así que la pregunta «¿quién se equivoca?» se mantiene |
| D4 | Aplicado | Estados internos solo en la columna de regla o entre paréntesis; «Confirmado» sale de lo que ve el humano |
| D8 | Aplicado | La tarjeta de dos desenlaces queda solo en el caso 2. En el caso 1: decisión escrita por Inés y pregunta de defensa. En el caso 3: aporte y caso de comprobación de opción múltiple |
| D13 | Aplicado | Toda celda «Qué ve el humano» tiene 50 palabras o menos |
| D14 | Aplicado | Caso 2, paso 5: «Costos · lista para firmar» antes de la puerta |
| Rechazados | — | Ninguno: todos los hallazgos que tocan los casos 1–3 se sostienen contra el texto. A-04 y P-09 se ajustaron en vez de aplicarse tal cual |

## Registro de cambios (casos 4–7)

| Hallazgo | Aplicado / ajustado / rechazado | Qué cambió o por qué no |
|---|---|---|
| D9 (caso 4) · P-02 | Aplicado | Martín redacta su plan y su decisión; pide «Auditar»; el juez encuentra el hueco (el congelamiento detiene pedidos). El agente propone la escritura dual, que choca con la decisión de Martín y nace como propuesta. Un solo desacuerdo: el agente enseña con el esquema y Martín reescribe. Su error ya no es ignorar la escritura dual, sino leer la variante que conoce. Se quitó «No te voy a poner otro caso» |
| D9 (caso 5) · A-15 · L-09 | Aplicado | Paso 6 nuevo: segunda ronda con avance neto negativo; el agente propone cortar y seguir exige una razón (doc 12 §6.1 (6); §4.1). Se quitó la línea «1 decisión nueva, 0 marcadores resueltos» de la primera ronda |
| D7 · P-03 | Aplicado | La cifra de escrituras por minuto sale de los pasos visibles; ya no se pide justificar una medición. Queda como pregunta abierta en el caso 4 |
| D8 | Aplicado | La tarjeta de dos desenlaces queda solo en el caso 4. Caso 5: predecir y corte de ronda en vez del aplicar. Caso 6: el aplicar de la junta queda como pendiente, sin mostrarse. Caso 7: predecir y decisión escrita por Diego; el aplicar de supuestos queda pendiente. Ninguna regla cambió: la regla sigue pidiendo esos aplicar (pregunta abierta en el caso 5) |
| D10 · P-19 | Aplicado | La investigadora del caso 5 pasa a llamarse Paula. Títulos ya en primera persona; el del caso 4 se ajustó al nuevo conflicto |
| D11 | Aplicado | Media línea en cada ficha: corte y escritura dual; estado del arte, DOI y micra; columna corta, encamisar y junta; VAN, tasa de descuento y MW |
| D13 | Aplicado | Celdas «Qué ve el humano» recortadas a 50 palabras o menos; lo demás pasó a «Cómo se asegura» |
| D14 · A-24 · C-6 | Aplicado | Caso 7, resultado 2: una sección sin pendientes estaría «lista para firmar»; se quitó «ninguna lista para firmar» sin condición |
| D1 · A-20 · A-37 · C-4 | Aplicado | Casos 6 y 7: el agente no quita marcadores; los quita la app o el humano (B3 §8). La pregunta del choque B1 §1.8 frente a B3 §8 va una sola vez, en el caso 6 |
| D2 · A-09 (caso 4) · A-16 | Aplicado | La lectura del criterio va con el caso primero, respuesta libre, y la lectura del agente se revela después. Se quitó «¿violan "cero caída"?» (sí o no) |
| D3 · A-23 · A-38 · P-08 | Aplicado | Caso 7: Diego predice «menos de 1 %» frente a 18 % (×18). Caso 5: «unas 20» frente a 1,5 (×13). En los dos, «¿quién se equivoca?» se sostiene |
| D4 · P-01 · P-15 · P-18 | Aplicado | «~N min para revisar antes de firmar»; sin «teach-back», «barrido», «velo», «lector de proyecto», «pieza núcleo» ni `aplica` en narración ni pantallas; estados internos entre paréntesis en el resultado; «diff» → «ver cambios»; sin «v6» |
| P-15 («[POR ACLARAR]» como jerga) | Rechazado | Es el marcador que el producto muestra en la sección (bloque 1 §1.8; B3 §8). Cambiar su forma visible sería inventar interfaz. Se explica solo, con su texto entre los corchetes |
| D5 · P-04 | Ajustado | Caso 6: la segunda tarjeta la exige el bloque 1 (la propuesta queda pendiente y su marcador bloquea), así que no se escondió. Se abarató a un clic («No estoy de acuerdo», sin «porque», `tq-act/1` paso 3) y se anotó como pregunta abierta |
| D6 · P-10 | Aplicado | Caso 6: Carmen ya no elige la vigente con «porque»; una línea remite a la pregunta común del caso 2 |
| D12 | Aplicado | Las líneas «En el hackathon» citan el bloque 1 (§1.5 E3, §1.10) y el doc 12 §11, que el §1.14 no reemplazó |
| A-13 | Ajustado | Con un solo desacuerdo hay un caso nuevo en cola, que sí queda descartado al reemplazar (bloque 1 §1.5 paso 3). La premisa del hallazgo, dos desacuerdos sin acto abierto, ya no existe |
| P4.4 | Aplicado (quitada) | Error del autor: el bloque 1 §1.4 deja «sin acto» las cifras heredadas a propósito. Además, ya no hay cifras visibles en el caso 4 |
| A-14 | Aplicado | La tarjeta de ronda nombra qué puede resolver: el marcador de «Síntesis» (doc 12 §5.3) |
| P-07 | Aplicado | «41 de campo; salen 6; quedan 35» |
| §3 plausibilidad (caso 5) · §4 anti-proyectos | Aplicado | Se dice de dónde salió la cita inexistente (un investigador). La ficha aclara que no se evalúa la salud ni se recomienda qué agua tomar; ninguna pantalla dice «más segura» ni «mejor» |
| §3 plausibilidad (caso 4) | Aplicado sin cambio | Volver atrás no arregla las fechas en la base nueva, pero el desenlace sigue siendo correcto: no se pierde ningún pedido |
| P5.2 | Aplicado | Fusionada con P5.1 (el efecto es lo que falta) |
| P5.4 | Ajustado | Se quitó «¿cuenta contra el tope?»: todo cuenta (doc 12 §4.4). Queda si se lanza sola |
| A-17 | Ajustado | El cierre ya no sale del hallazgo de costo, que Carmen marcó «no aplica» y el agente respeta. Sale de corregir las columnas cortas, marcadas «error» |
| P-16 · P6.4 | Aplicado | El agente explica que su propuesta nace porque la condición «cerraría si…» podría cumplirse (DEC-003 §2). La pregunta de quién lo detecta sigue abierta |
| A-18 | Aplicado | «los pasajes de los 2 hallazgos» |
| A-19 | Aplicado | Resultado 1 del caso 6: pendiente hasta el paso 5, rechazada desde entonces |
| A-36 · C-2 · P6.1 | Aplicado | Tres flancos marcados como módulo 6 y la tarjeta de hallazgos como módulo 4; nota para que el caso 10 los trate igual |
| D7 (caso 6) | Aplicado | Se quitó «tanque de 10 m³» de lo que ve el humano |
| A-21 · C-5 | Aplicado | Caso 7: el choque interno del doc 12 queda citado en la regla del paso 4 y como pregunta abierta |
| A-22 | Aplicado | Diego escribe su elección en la decisión del expediente, no en el chat |
| Auditoría §1 (caso 7: repite el «dale») | Aplicado | Se quitó «se ve bien, dale» del caso 7; el «ok» sin efecto queda solo en el caso 4 |
| P7.4 | Aplicado | Se dejó como remisión a la misma pregunta del caso 2 |

## Registro de cambios (casos 8–10)

| Hallazgo | Aplicado / ajustado / rechazado | Qué cambió o por qué no |
|---|---|---|
| P-05 (caso 8, quién cambió durante la pausa) | Ajustado | La ficha dice que Valeria cambió la decisión el último día, tras una reunión, y lo olvidó. **Rechazada** la variante «un colega fuera de la app»: el diseño no tiene reglas de varios usuarios (bloque 1 §1.1 solo cubre lo importado y lo externo sin persona); queda como pregunta abierta. El paso 2 original se partió en dos (pasos 2 y 3) |
| Jurado §1, caso 8 (a) débil | Aplicado | El agente aporta algo nuevo tras la pausa: el cruce de los buses en el paradero (isla de paradero, decisión suya que rige sin tarjeta) y la salida para la cuadra 3 |
| P-04 (caso 8, dos tarjetas por un conflicto) | Ajustado | Se quitó la tarjeta de clasificar: Valeria ya no edita el criterio; su respuesta al caso basta y el pasaje queda por revisar. El caso en que sí lo edita va a preguntas abiertas (D5) |
| A-09 (caso 8, pregunta de sí o no) | Aplicado | «¿Cómo la juzgas con tu regla?», con respuesta libre, y la lectura del agente oculta hasta después (D2) |
| D8 (caso 8, tarjeta de dos desenlaces) | Aplicado | El caso de reentrada pasa a un justificar con respuesta libre sobre una decisión de ella (tipo no fijado en §1.6: `[inferencia]`, pregunta abierta). Actos del caso: justificar con sus palabras, caso para retomar, juicio libre, adoptar |
| A-25 (caso 8) | Aplicado | «Cambió» sale también si el texto vuelve a ser igual o ella responde «no aplica» |
| A-26 (caso 8) | Aplicado | La decisión queda pendiente solo hasta su justificar (paso 3); después, confirmada |
| A-27 (caso 8) | Aplicado | La cifra «8 plazas» la cubre el justificar de su decisión, no un acto propio |
| P8.4 (caso 8) | Aplicado (quitada) | Error del autor: §1.14 dice que es el justificar de §1.3; queda dicho en la columna de regla |
| Plausibilidad caso 8 (umbral de 120 bicicletas) | Aplicado | Se agrega la velocidad de los autos (30 km/h) |
| P-20, D10 (casos 8–10) | Aplicado | Nombres Valeria, Natalia y Teresa; títulos en primera persona |
| P-06 (caso 9, deuda invisible) | Ajustado | La frase de historia y la referencia por número quedan en pantalla (paso 5), y el agente explica una vez qué frenó la app, qué no y dónde queda cada cosa, con un antes y después (paso 6). No se dice que el intento rechazado esté «en el historial»: ninguna regla lo guarda; va a preguntas abiertas |
| A-29 (caso 9) | Aplicado | Ya no se supone que el agente quitó la frase: queda hasta que Natalia la quita con «Quita» |
| A-16 (caso 9) | Aplicado | Primero el caso; la lectura del agente se revela después de la respuesta |
| A-28 y P-11 (caso 9, tarjeta de memoria) | Ajustado | Se quitó la tarjeta de dos desenlaces (D8). El ejemplo de S/ 0,88 frente a S/ 0,60 que proponía P-11 pasa a la explicación del agente, e Natalia escribe la decisión con sus palabras. **Rechazada** la tarjeta que proponía P-11: D8 la reserva para los casos 2 y 4 |
| Plausibilidad caso 9 («7 de 12») | Aplicado | La cifra está en el digesto |
| D7 (caso 9, «9 de 12») | Aplicado | Ya no se vincula a una decisión ni pide un acto; queda solo en el digesto |
| P-10, D6 (casos 8 y 9) | Aplicado | Una línea que remite a la pregunta común del caso 2 |
| P9.1–P9.3 | Ajustado | Se mantienen y se actualizan; se agregan el intento rechazado y la cifra que repite la elegida |
| D9 (caso 10, camino feliz) | Aplicado | Paso 5: «Área de influencia» se explica y se firma en ~2 min sin tarjetas extra; contrasta con el paso 6 |
| D14, A-34 (caso 10) | Aplicado | «Conclusiones: lista para firmar» |
| D1, A-32, A-37, P-12 (caso 10) | Aplicado | El agente no quita la marca; la quita la app cuando la decisión de ella cierra la pregunta (B3 §8). El choque con B1 §1.8 va como pregunta abierta en el caso 6; aquí remite a él (V-02) |
| A-30 (caso 10) | Aplicado | Que el hallazgo se muestre como contradicción que bloquea queda marcado módulo 4, con la tensión con doc 12 §6.3 y §7 en preguntas |
| A-31 (caso 10) | Ajustado | La decisión del agente se reemplaza y la cifra cuelga del sucesor, como pide el hallazgo, pero el sucesor lo escribe Teresa (su regla manda, y D8 evita la tarjeta de dos desenlaces). Por §1.4, la cifra pide un justificar (paso 3). **Rechazada** la vía «el agente crea el sucesor y pide un aplicar»: chocaría con D8 |
| A-33 (caso 10) | Aplicado | Antes de cada puerta se dice que no hay pendientes. La decisión de 2021 es de ella y está confirmada desde el inicio |
| A-35 (caso 10) | Aplicado | La línea del hackathon dice, como `[inferencia]`, que probablemente nada detecte el error |
| A-36 (caso 10) | Aplicado | Un revisor con dos preguntas (doc 12 §6.3); cómo se muestran los hallazgos: módulo 4 |
| P-01, P-15, P-18, D4 (caso 10 y los demás) | Aplicado | «~15 min para revisar antes de firmar»; sin «pieza núcleo», «juez», «barrido», «teach-back» ni «relevo» fuera de la columna de regla; estados internos entre paréntesis |
| Plausibilidad caso 10 (OMS y coliformes; parámetro con nombre) | Aplicado | «Una guía internacional para uso recreativo»; «un parámetro que el estándar nacional no regula», sin nombrarlo. El criterio de ella conserva «OMS» porque es su cita literal |
| D11 (casos 8–10) | Aplicado | Equivalencias en US$ en el caso 9; media línea para cada término del dominio en las fichas |
| D13 | Aplicado | Las celdas «Qué ve el humano» tienen unas 50 palabras o menos; lo demás pasó a «Qué pasa» o a «Cómo se asegura» |
| P10.4 | Aplicado (quitada) | Error del autor: §1.7 dice «una vez por sesión» |
| P10.2 | Ajustado | Se reformula: la exportación ya trae el estado por sección (B3 §5); falta el formato para terceros |
| P10.1 | Ajustado | Se mantiene corta y remite al caso 6 |