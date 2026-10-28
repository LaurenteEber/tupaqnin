---
titulo: Casos de prueba · bloque 1 (flujo completo)
estado: provisional, v2. Auditada (fidelidad a las reglas; problema y jurado) y verificada de forma independiente
fecha: 2026-10-26
relacion: prueba el diseño del bloque 1 (módulo 1 cerrado en propuesta; módulos 2–6 y bucle de comprensión por diseñar) contra el problema de la deuda cognitiva; insumo para diseñar el bucle de comprensión y para elegir los casos de la demo; registro de auditoría en hallazgos.md
---

# Casos de prueba · bloque 1

*Diez historias de trabajo en conjunto entre un profesional y tupaqnin, como dos compañeros. El humano pone el objetivo, las decisiones y las reglas de su experiencia. El agente pone su conocimiento, la investigación web, el análisis masivo y subagentes que trabajan en paralelo o auditan desde varios flancos. En cada caso se ve el momento en que nacería la deuda cognitiva y cómo el agente se asegura de que el humano entiende lo que se avanzó, sus límites y sus consideraciones.*

*Todo es ficticio: personas, empresas, lugares y cifras. Cada paso cita la regla que lo decide, o dice «por diseñar» o «módulo N (pendiente)». Los nombres internos van solo en la columna «Regla».*

## Índice

| # | Quién y qué construye | Etapa del flujo | Deuda que se crearía | Frase gancho (para el jurado) |
|---|---|---|---|---|
| 1 | Inés, arquitecta · anteproyecto de una guardería | Encuadre desde un boceto | Criterio parafraseado | *The AI turned my rule "never direct sun" into "optimal orientation".* |
| 2 | Eber, analista de políticas · memo de cunas de 8 a 12 h | Construcción con cálculo | Cifra sin respaldo | *I almost signed off on S/ 120 million (~US$ 32 M) without knowing where it came from.* |
| 3 | Rosa, abogada · due diligence de 300 contratos | Análisis masivo | Asimetría de velocidad | *300 contracts, one quiet interpretation by the AI, and nobody showed me.* |
| 4 | Martín, ingeniero de software · diseño de migración de base de datos | Decisión técnica; él conduce y el agente audita | Aprobación sin acto | *I said "ok, go" to a migration plan I couldn't explain to my own team.* |
| 5 | Paula, investigadora doctoral · estado del arte sobre microplásticos | Investigación | Investigación no digerida; cita inventada | *The citation that best supported my thesis didn't exist.* |
| 6 | Carmen, ingeniera estructural · evaluación sísmica de un colegio | Auditoría y reescritura | Reversión silenciosa | *When the AI rewrote the report, it quietly brought back the school closure I had ruled out.* |
| 7 | Diego, analista financiero · caso de negocio de una planta solar | «Continúa hasta terminar» | Retiro del humano | *I told the AI "finish it" — and came back to find it had decided nothing in my name.* |
| 8 | Valeria, urbanista · plan de ciclovías | Retomar después de 10 días | Memoria que depende de releer | *After 10 days away, I didn't remember what my own change broke.* |
| 9 | Natalia, gerente de producto · especificación de pagos | Construcción y forma | Proceso dentro del producto | *My spec told its own history instead of saying what to build.* |
| 10 | Teresa, ingeniera ambiental · estudio de impacto para la autoridad | Puerta de firma y entrega a terceros | Cierre que congela («firma tú») | *"Just sign it for me" — and the river was compared with the wrong standard.* |

**Candidatos para la demo en inglés** (auditoría de jurado): 2, 5, 10 y 3. El 2 es el recorrido de la demo del doc 11 §6, pero ese guion se tiene que actualizar (ver caso 2, preguntas abiertas).

---

## Caso 1 · «El agente leyó mal mi regla de la luz»

**Ficha:** Inés, arquitecta · anteproyecto (primera versión del diseño, que la municipalidad revisa antes del proyecto definitivo) de una guardería de 4 aulas para 60 niños de 0 a 3 años, en una ciudad ficticia a 40° de latitud norte · para la municipalidad que la encarga (ante terceros) · encuadre, desde un boceto en papel · arquitectura. Todo ficticio, incluida la ordenanza municipal.

**Qué aporta cada uno**
- **Humano:**
  - Objetivo: un anteproyecto que la municipalidad apruebe y que ella pueda defender en la revisión.
  - Criterio: «Las aulas miran al norte: luz natural, nunca sol directo».
  - Decisiones: dónde va cada aula; si pone parasoles en la fachada norte.
  - Nociones: el boceto (un solo piso, 4 aulas, patio, cocina, una rampa); las clases son de mañana y hay horario extendido hasta las 19:00.
- **Agente:**
  - Conocimiento: asoleamiento. En verano, a esa latitud, el sol sale y se pone al norte del este y del oeste: la fachada norte recibe sol bajo temprano y al final de la tarde. Un alero horizontal corta el sol alto, no el bajo.
  - Herramientas: lee la foto del boceto y la vuelve un diagrama editable. Busca en la web, él mismo y en pocas consultas, la ordenanza de centros infantiles y el recorrido del sol en esa latitud. Comprueba que cada fuente existe y que el pasaje citado está en ella.
  - Lo que propone y Inés no tenía: parasoles verticales en la fachada norte, por el horario extendido; las exigencias de la ordenanza (superficie por niño, salida directa al exterior en cada aula).

**Pasos**

| Paso | Qué pasa | Qué ve el humano | Regla que lo decide |
|---|---|---|---|
| 1 | Inés sube la foto del boceto y escribe su criterio en el expediente, con sus palabras | «Criterio guardado tal como lo escribiste: "Las aulas miran al norte: luz natural, nunca sol directo".» El boceto aparece en Fuentes, con su fecha | bloque 1 §1.2 (palabras del humano → confirmado al nacer); §1.8 (fuente con versión: id del archivo y su huella) |
| 2 | El agente lee la foto y arma un diagrama, con dos cajas marcadas «no sé a qué parte del texto corresponde». No lo da por bueno: guarda lo que espera que Inés responda y le devuelve su lectura con un caso. Inés responde con sus palabras, y el agente corrige el diagrama | «Leí tu boceto así: la línea en zigzag junto al patio es una escalera a un segundo piso. Con esa lectura, ¿cómo llega al patio un niño en silla de ruedas desde el aula 4?». Inés: «No hay segundo piso: es una rampa». «Yo esperaba "en ascensor". Corrijo el diagrama.» | doc 12 §4.3 (todo boceto leído vuelve con predicción, no como diagrama aceptado) y §5.1 (boceto → diagrama, con los nodos sin ancla marcados); B3 §8 (no se admiten actos de sí o no); tipo de acto: por diseñar (no está en el catálogo de bloque 1 §1.6); bloque 1 §1.9 (la corrección la escribe el agente, con el mensaje de Inés como origen); módulo 5 (pendiente) |
| 3 | El agente busca en línea los dos datos, sin lanzar investigadores, y comprueba las fuentes. Explica lo hallado con un esquema del recorrido del sol sobre la planta y propone parasoles con su contraste. Inés escribe la decisión en el expediente, con sus palabras: «Parasoles verticales en la fachada norte, en vez de dejarla libre: los niños del horario extendido están en el aula cuando entra el sol de la tarde. La dejaría libre si cerráramos a las 16:00» | «Ordenanza: superficie mínima por niño y salida directa al exterior en cada aula. Sol: en verano entra bajo por el norte temprano y al final de la tarde, y el alero no lo corta. Te propongo parasoles verticales; si cerraran a media tarde, sobrarían. Fuentes comprobadas: 2 de 2.» | doc 12 §6.2 (hecho puntual: lo hace el agente en línea, ≤ 3 llamadas), §5.2 D1 (no hay razón para delegar), §7 capa 1 (la fuente existe y el pasaje aparece); DEC-003 §2 (quien propone explica, con su contraste); bloque 1 §1.3 (decisión del humano con sus palabras y su contraste → confirmada al nacer); §1.4 (crítica: afecta «Implantación» y «Fachadas») |
| 4 | El agente redacta «Implantación y orientación» aplicando el criterio de Inés. Es la primera vez que lo usa, así que la app guarda cómo cree el agente que ella juzgaría un caso nuevo y le muestra el caso, sin esa predicción. Inés responde con sus palabras | «Antes de seguir, quiero comprobar que entendí bien tu regla. Caso: en un terreno angosto, un aula solo puede abrir ventanas al este, con un alero de 1 m. ¿Cómo la juzgas?». Inés: «La descarto: el sol de la mañana entra bajo el alero, en horario de clase». | bloque 1 §1.2 (en el primer uso de un criterio del humano, el agente predice cómo lo juzgaría en un caso nuevo); §1.6 (la predicción se guarda antes de mostrar la tarjeta), aplicado aquí `[inferencia]`; formato de la tarjeta (caso primero, respuesta libre, predicción después): por diseñar |
| 5 | La app revela la predicción: no coincide. La respuesta de Inés queda como ejemplo del criterio, y el pasaje escrito con esa lectura queda pendiente. El agente lo reescribe y explica el cambio con un diagrama antes y después | «Yo esperaba que la aceptaras: leí tu regla como "evitar el sol que calienta", el del mediodía. Guardo tu respuesta como ejemplo.» Mientras tanto: «Falta: revisar 1 pasaje escrito con una lectura equivocada de tu criterio». Luego: «Cambié: el aula de lactantes pasa al norte; el depósito, al este». | bloque 1 §1.2 (si no coincide: ejemplo, señal «el agente lo leyó mal», el criterio sigue confirmado; el pasaje bloquea hasta que el agente lo reescriba o el humano lo dé por bueno con un «porque»); §1.7 (bloquea); DEC-003 §2 (esquema cuando reemplaza texto); explicar cada avance: bucle de comprensión (por diseñar) |
| 6 | Al cerrar la sección, el agente comprueba que Inés puede defender lo que se avanzó, con la pregunta que le haría la municipalidad. Ella responde con sus palabras | «Antes de pasar a Fachadas: si en la revisión te preguntan por qué el aula de lactantes no mira al este, ¿qué les dices?». Inés: «Al este entra sol bajo en horario de clase y el alero no lo corta; al norte, con parasoles, solo entra luz difusa». | bucle de comprensión (por diseñar); DEC-003 §3 |

**Dónde nacería la deuda sin tupaqnin:** un chat con IA leería el boceto y devolvería una planta pulida con «orientación óptima de las aulas para aprovechar la luz natural». Pondría el aula de lactantes al este, con alero, como si eso cumpliera la regla. Inés firmaría el anteproyecto, y en la revisión municipal no sabría explicar por qué los bebés reciben sol directo cada mañana: nunca vio cómo había leído el agente su regla.

**Cómo se asegura que entiende**
- **Del agente hacia Inés (lo que se avanzó):** cada cambio se explica con qué cambió, por qué, en vez de qué y su límite. En el paso 5, el detalle completo es: «Cambié: el aula de lactantes pasa del este al norte; el depósito, al este. Por qué: tu ejemplo (al este entra sol bajo en horario de clase). En vez de: aula al este con alero. Límite: el patio se achica, pero sigue sobre el mínimo de la ordenanza». Va con un diagrama antes y después, con las cajas que se movieron resaltadas. En el paso 3, el esquema del sol muestra por qué hacen falta los parasoles. Al cerrar la sección, una pregunta de defensa que Inés responde con sus palabras (paso 6). Si su respuesta omite algo que cambió (por ejemplo, que el patio se achicó), el agente debería decírselo. Todo esto es **por diseñar** (bucle de comprensión): hoy ninguna regla pide explicar cada avance ni comprobarlo al cierre de una sección.
- **De Inés hacia el agente:** el agente le devuelve dos lecturas, cada una con un caso, y guarda antes lo que espera que ella responda. Una es la del boceto (paso 2; doc 12 §4.3; tipo de acto por diseñar). La otra es la de su criterio (pasos 4–5; bloque 1 §1.2, diseñado).
- **Lo que decide Inés:** la decisión de los parasoles es suya. El agente trae el conocimiento del sol y la propuesta, e Inés la escribe con sus palabras y su contraste (paso 3). Por eso no hace falta una tarjeta sobre ella.

**Resultado esperado**
1. El criterio rige con el texto literal de Inés y no pide tarjeta (estado interno: `confirmado`) (bloque 1 §1.2). Si el agente registrara «orientación óptima» como criterio, nacería como propuesta solo si la búsqueda de choques lo detecta `[inferencia]` (§1.1; heurística del módulo 4). Si no lo detecta, queda como criterio del agente sin confirmar, que también bloquea la firma (§1.7).
2. Tras el caso que no coincidió, el criterio sigue rigiendo, la respuesta de Inés queda como ejemplo y aparece la señal «el agente lo leyó mal» (§1.2).
3. Mientras el pasaje del aula al este no se reescriba, o Inés no lo dé por bueno con un «porque», «Implantación y orientación» no puede firmarse (§1.2; §1.7).
4. La decisión de los parasoles es crítica, porque afecta 2 secciones, y rige sin tarjeta, porque Inés la escribió con sus palabras y su contraste (estado interno: `confirmada`) (§1.3; §1.4).
5. El diagrama del boceto no nace aceptado: queda en borrador, con las cajas sin ancla marcadas. La corrección la registra el agente como autor, con el mensaje de Inés como origen (doc 12 §4.3, §5.1; bloque 1 §1.9; módulo 5 pendiente).

En el hackathon: el teach-back en el primer uso queda para el producto (§1.10); el boceto → diagrama es «Debería» (doc 12 §11); y B3 §8 apaga la búsqueda web alojada de Meta, así que el paso 3 no ocurre tal cual.

**Qué prueba del diseño:** bloque 1 §1.2 (palabras del humano, teach-back en el primer uso y sus efectos), §1.3–§1.4 (decisión del humano con contraste, criticidad por secciones), §1.7, §1.9 (autoría de la corrección); doc 12 §4.3, §5.1, §6.2, §7; B3 §8; módulo 5; bucle de comprensión.

Preguntas abiertas:
- La lectura del boceto con predicción (doc 12 §4.3) no tiene tipo de acto en el catálogo de §1.6, ni regla de resultado. ¿Es un teach-back? ¿Qué queda registrado si Inés la corrige?
- §1.2 no fija el formato del teach-back ni dice que la predicción se oculte. Aquí se usa: el caso primero, respuesta libre, y la predicción revelada después, por analogía con §1.6 `[inferencia]`.
- La comprobación al cerrar una sección (paso 6) no tiene tipo de acto ni regla de resultado. Si la respuesta omite un límite, ¿qué hace el agente y qué queda registrado?
- El doc 12 §4.1 cuenta el boceto subido como acto. Bloque 1 §1.6 exige además escribir qué prueba, con un «porque», y §1.14 no lista ese reemplazo. Hay que alinear las dos fuentes.
- Si Inés dicta el criterio por voz en el encuadre (doc 12 §4.3), ¿cuenta como «sus palabras»? §1.2 dice «texto que tecleó».
- B3 §8 apaga la búsqueda web alojada, y el doc 12 la usa para los hechos puntuales en línea (§6.2) y para los investigadores web (§5.1). ¿Qué rige en el producto? (C-1)
- Las medidas que el agente copia de la ordenanza al texto: ver la pregunta abierta sobre mediciones de una fuente en el caso 4.

---

## Caso 2 · «Iba a firmar S/ 120 millones sin saber de dónde salían»

**Ficha:** Eber, analista de políticas (en la demo lo interpreta Eber) · memo «Ampliar el horario de las cunas públicas de 8 a 12 horas» (cunas = guarderías públicas para niños de 0 a 3 años), con tres secciones: «Diagnóstico», «Costos» y «Recomendación», esta última con rol de recomendación · para la dirección de un ministerio ficticio (ante terceros) · construcción con cálculo · políticas públicas. Todo ficticio; las cifras son inventadas. S/ = soles peruanos: S/ 120 millones ≈ US$ 32 millones y S/ 12 millones ≈ US$ 3,2 millones (a S/ 3,75 por dólar, ficticio). ENAHO = la encuesta nacional de hogares del Perú, que sale en versión anual y trimestral; los datos que se usan aquí son ficticios.

**Qué aporta cada uno**
- **Humano:**
  - Objetivo: saber cuánto cuesta ampliar el horario y si conviene recomendarlo.
  - Criterio de fuente: «ENAHO anual, no trimestral: poca muestra por región». Lo registra como decisión, con su contraste.
  - Decisiones: la fuente; qué recomendar.
  - Nociones: las madres que trabajan hasta tarde no pueden recoger a sus hijos a las 16:00.
- **Agente:**
  - Conocimiento: una jornada regular de 12 horas no es viable para el personal de cuidado, así que 4 horas más exigen un segundo turno y no horas extra del mismo personal. Conoce también las proporciones habituales de cuidadoras por niño.
  - Herramientas: procesa los microdatos de la encuesta con código, sin cargarlos a su contexto. Corre el cálculo como comprobación de la cifra y busca si hay datos más recientes.
  - Lo que propone y Eber no tenía: abrir el segundo turno solo donde hay demanda (40 % de uso estimado por región), en vez de en todas las cunas; la existencia de la ENAHO trimestral 2025.

**Pasos**

| Paso | Qué pasa | Qué ve el humano | Regla que lo decide |
|---|---|---|---|
| 1 | Eber escribe en el expediente su decisión de fuente, con sus palabras y su contraste | «Decisión guardada con tus palabras: "Fuente de la demanda por región: ENAHO anual 2024, en vez de la trimestral, porque la trimestral tiene poca muestra por región. Elegiría la trimestral si solo necesitara cifras nacionales recientes".» | bloque 1 §1.3 (decisión del humano con sus palabras y su contraste → confirmada al nacer) |
| 2 | El agente calcula el costo con los microdatos y registra sus supuestos como decisión: «Segundo turno solo en las cunas con demanda (40 % de uso por región, según la ENAHO anual), en vez de extender el horario en todas las cunas. Elegiría todas si la demanda fuera pareja entre regiones». Antes de mostrar el resultado, pide a Eber que prediga. Eber espera S/ 12 millones; el cálculo da S/ 120 millones. Eber responde que se equivocó su expectativa: pensaba en horas extra del mismo personal. El agente explica sus supuestos con un esquema | «Qué mide: costo anual adicional · cunas con demanda · 2026 · en soles. Datos: ENAHO anual 2024. ¿Cuánto esperas?». Eber: «unos S/ 12 millones». «Resultado: S/ 120 millones, 10 veces más. ¿Quién se equivoca: tu expectativa, el cálculo o la afirmación?» | bloque 1 §1.3 (supuestos de un cálculo: decisión crítica con alcance); §1.6 (predecir, producto); doc 12 §7.1 (ficha del cálculo; predicción antes de ver el resultado; «¿quién se equivoca?» solo si difieren en dirección u orden de magnitud: aquí, 10 veces); doc 12 §5.2 D5 (los datos tabulares se procesan con código) |
| 3 | El agente escribe la recomendación y repite el costo sin enlazarlo a sus supuestos. La app lo detecta. El agente lo enlaza, y la app abre la tarjeta de los supuestos, que cubre las dos cifras. Eber elige «Baja» porque «el gasto sigue al uso medido en cada región, no al número de locales» | «Recomendación · falta: S/ 120 millones no tiene una decisión que lo sostenga». Luego: «Caso: en una región, solo el 15 % usaría el horario ampliado. ¿Qué pasa con su parte del costo?» · «Baja: el turno abre solo donde hay demanda» / «No cambia: el turno abre en todas las cunas» | bloque 1 §1.4 («cifra sin decisión» bloquea; vincular; el acto de una decisión cubre las cifras vinculadas en ese momento); §1.6 (aplicar: el desenlace de lo elegido y el de su «en vez de», orden sorteado, «porque» obligatorio); §1.7 |
| 4 | El agente encuentra la ENAHO trimestral 2025 y registra «Fuente: ENAHO trimestral 2025». Repite la pregunta de la decisión de Eber, así que no rige: nace como propuesta. Mientras está pendiente, el agente no escribe con ella. Eber elige la suya, con su razón | «Hay una ENAHO trimestral 2025, más reciente. No la uso: tu decisión es la anual. Te dejo una propuesta.» Tarjeta: arriba, en literal, lo que escribiste; abajo, la propuesta. Eber: «en la trimestral hay muy pocas madres con hijos menores de 3 años en cada región». | bloque 1 §1.1 (choque → nace como propuesta); §1.5 (solo el humano revoca; la tarjeta muestra lo revocado en literal; elegir la vigente → propuesta rechazada; mientras está pendiente no se escribe con lo propuesto); §1.6 (`tq-act/1` paso 7: elegir la vigente pide «porque») |
| 5 | «Costos» no tiene pendientes. Eber pulsa «Listo para firmar». La app congela la sección y sus decisiones. Eber explica en voz alta, sin mirar, y revisa la transcripción. Wonder contrasta la explicación con las decisiones examinadas y marca una como omitida | «Costos · lista para firmar». Tras explicar: «Cubriste 3 · Omitiste 1: el turno abre solo donde hay demanda (40 % de uso). Para firmar, explica por qué ese supuesto y no el uso total». El botón «Firmar» está inactivo. | bloque 1 §1.7 («lista para firmar» con pendientes en cero; congelar; explicar sin mirar no confirma decisiones; Wonder solo pide un justificar; momento «firma bloqueada»); doc 11 §5–§6 (voz y Wonder); B3 §4; módulo 2 (pendiente: momento de congelar, qué se examina, formato de Wonder) |
| 6 | Eber responde la tarjeta con su razón. La app la acepta. Junto al botón ve el texto fijo de lo que firma, y pulsa «Firmar» | «¿Por qué 40 % y no el uso total?». Eber: «la encuesta anual muestra que menos de la mitad de las madres trabaja después de las 16:00; pagar turnos que nadie usa es gasto sin efecto». «Costos · firmada (versión 7 · 26-oct).» Documento: «1 de 3 secciones firmadas». | bloque 1 §1.6 (justificar de la puerta: con «confirma» cumple el requisito y no cambia el estado); §1.7 (la firma solo nace del botón «Firmar»; texto fijo de la firma; «k de N secciones firmadas») |

**Dónde nacería la deuda sin tupaqnin:** un chat con IA calcularía «S/ 120 millones» y lo pondría en la recomendación. Además «actualizaría» la fuente a la trimestral 2025 por ser más reciente, sin decirlo. Eber firmaría una cifra que supone un segundo turno y un 40 % de uso, sobre datos que él había descartado. Ante la dirección, una pregunta como «¿y si lo usa el 80 %?» lo dejaría sin respuesta.

**Cómo se asegura que entiende**
- **Lo que se avanzó:** tras el cálculo, el agente explica qué supuso, en vez de qué y cuándo elegiría lo otro: «Supuse un segundo turno de 4 horas, no horas extra del mismo personal: una jornada regular de 12 horas no es viable para quien cuida. El turno abre solo donde hay demanda, 40 % de uso según la ENAHO anual por región, en vez de en todas las cunas. Elegiría todas si la demanda fuera pareja». Va con un esquema de dónde salen los S/ 120 millones (personal del segundo turno 78 % · alimentación 15 % · servicios 7 %). La predicción previa hace visible la brecha entre lo que Eber esperaba y lo que salió, y su respuesta a «¿quién se equivoca?» dice dónde estaba su modelo mental (horas extra). Que el agente explique cada avance, y no solo las decisiones críticas, es **por diseñar** (bucle de comprensión).
- **Actos de Eber:** predice antes de ver la cifra (paso 2); aplica los supuestos a un caso, con su razón (paso 3); explica la sección sin mirarla (paso 5); justifica el supuesto que omitió (paso 6).
- **Lo que el agente respeta:** la fuente de Eber. El agente aporta el dato nuevo (la trimestral 2025), pero no puede usarlo sin un gesto de Eber (paso 4).

**Resultado esperado**
1. «Recomendación» no puede firmarse mientras la cifra S/ 120 millones no esté enlazada a una decisión (bloque 1 §1.4; §1.7).
2. La decisión de supuestos es crítica, porque es de supuestos y cubre una cifra, y bloquea «Costos» y «Recomendación» hasta que la tarjeta dé un resultado que confirme (§1.3; §1.4; §1.7).
3. La decisión de fuente de Eber sigue rigiendo, la propuesta trimestral queda rechazada y ninguna cifra se escribió con la trimestral mientras estuvo pendiente (§1.1; §1.5).
4. Sin pendientes, «Costos» aparece como «lista para firmar». En la puerta, la firma espera la respuesta del justificar; un «no sé» no la destraba, y responderlo no cambia el estado de la decisión de supuestos (§1.6; §1.7).
5. La firma nace solo del botón «Firmar» de Eber. El documento muestra «1 de 3 secciones firmadas», y el resultado de Wonder queda al lado de la firma, no dentro (§1.7; anexo A1.3).

En el hackathon: la propuesta trimestral se rechaza sola y el agente se lo dice a Eber en el chat (§1.5, E3); el predecir queda para el producto (§1.10), así que la demo cuenta el caso sin el paso 2; las cifras con vínculo sí entran (§1.10); las tarjetas de la plantilla de la demo se generan antes y se validan con las mismas reglas (§1.10).

**Qué prueba del diseño:** bloque 1 §1.1 (choque), §1.3 (decisión del humano; supuestos), §1.4 (cifras y vínculo), §1.5 (solo el humano revoca), §1.6 (aplicar, predecir, justificar de la puerta), §1.7 (puerta, Wonder, firma, estado); doc 12 §7.1; doc 11 §6; módulo 2; bucle de comprensión.

Preguntas abiertas:
- ¿Pedir razones para conservar la decisión propia es fricción? En la tarjeta de adoptar, elegir la vigente exige un «porque» (`tq-act/1` paso 7); sin él, la propuesta sigue pendiente. En cambio, el botón «No estoy de acuerdo» la rechaza sin pedir razón (§1.6, tabla «No estoy de acuerdo»): son dos caminos al mismo efecto, uno con razón y otro sin ella.
- Un cálculo con dos supuestos independientes (segundo turno en vez de horas extra; solo donde hay demanda en vez de en todas): ¿una decisión de supuestos por cálculo o una por supuesto? Con una sola, el «en vez de» contrasta uno solo, y el otro queda solo en la explicación, sin red propia (§1.3; §1.4 «afirmaciones sin cifra»).
- El acto de predecir está en el catálogo de §1.6, pero `tq-act/1` no dice qué resultado da ni qué cambia. Y doc 12 §7.1 no dice qué pasa si la diferencia no es de dirección ni de orden de magnitud.
- El guion de la demo (doc 11 §6) choca con §1.7 en tres puntos (C-3): rige un justificar dentro de la puerta, no «vuelve a explicar»; la firma necesita que Eber pulse «Firmar»; y el aviso debe nombrar la decisión por su enunciado, no por su código.
- Si el módulo 2 no elige la decisión de supuestos entre las examinadas, el momento «firma bloqueada» de la demo no ocurre. ¿La plantilla de la demo la fija?
- La prioridad de la cola (§1.6) usa «más secciones afectadas», pero no dice qué son las secciones afectadas de un predecir. El predecir tiene que ir antes de mostrar el resultado (doc 12 §3, «Controlar»), y aquí va antes que el aplicar porque el resultado aún no se ve.

---

## Caso 3 · «300 contratos, 80 páginas, y una interpretación que nadie me mostró»

**Ficha:** Rosa, abogada corporativa · informe de due diligence (la revisión legal de una empresa antes de comprarla) de los 300 contratos de una empresa de logística que su cliente quiere comprar · para el comité de inversión del cliente (ante terceros) · análisis masivo · derecho corporativo. Todo ficticio. Cambio de control = cláusula que deja a la otra parte terminar o renegociar el contrato si la empresa cambia de dueño. Los montos están en dólares.

**Qué aporta cada uno**
- **Humano:**
  - Objetivo: saber qué contratos podría perder o encarecer el comprador cuando cambie el dueño.
  - Criterio: «Solo cuenta cambio de control por encima de US$ 1 M».
  - Decisiones: qué efecto de una cláusula cuenta como riesgo; qué se reporta al comité.
  - Nociones: la operación es una compra de acciones, sin fusión; al comité le importa cuánto se pierde en plata. Conoce de memoria el contrato 47, de transporte refrigerado, que ella negoció hace dos años.
- **Agente:**
  - Conocimiento: los tipos de cláusulas que se activan al cambiar el control (terminar, revisar tarifas, pedir garantías, pedir consentimiento) y cómo se redactan.
  - Herramientas: subagentes lectores, hasta 4 a la vez, que leen los contratos por lotes. Cada encargo devuelve pasajes literales, con su ubicación y una razón, sin recomendar; si un lote no cabe en un encargo, se parte. Usa código para sumar montos y convertir monedas.
  - Lo que propone y Rosa no tenía: la tabla de riesgos completa; la separación entre cláusulas que permiten terminar y cláusulas que solo permiten revisar tarifas.

**Pasos**

| Paso | Qué pasa | Qué ve el humano | Regla que lo decide |
|---|---|---|---|
| 1 | Rosa sube los 300 contratos y escribe su criterio. Marca el contrato 47 como contrato testigo y escribe qué prueba. El agente muestra el plan. Cada lector recibe el criterio de Rosa en literal, en un encargo que arma la app | «Criterio guardado tal como lo escribiste.» Contrato testigo: «El 47 tiene que salir en la tabla, porque si cambia el dueño el transportista puede subir la tarifa». Plan: «Leo los 300 en 4 lotes, con hasta 4 lectores a la vez. Costo estimado: US$ 6. Empiezo, salvo que canceles». | bloque 1 §1.2; §1.6 (aporte, producto: el humano vincula algo que trajo y escribe qué prueba, con un «porque»; `tq-act/1` paso 7); doc 12 §4.1 (aporte propio: caso testigo; L-16), §5.1 (lector en modo proyecto, 1–4 en paralelo, sobre las fuentes que subió el humano), §5.2 D5 (lectura pesada); mostrar el plan antes de leer: `[inferencia]`, módulo 6 (doc 12 §5.1 liga la tarjeta de ronda solo al modo web); bloque 1, requisitos del encargo 3, 8 y 9 |
| 2 | Los lectores leen el lote 1. Sus hallazgos llegan a la app tal cual, sin que el agente que escribe los filtre. Dos contratos escaneados no se pueden leer. El agente arma las filas con el criterio. Como es la primera vez que lo usa, la app guarda cómo cree el agente que Rosa juzgaría un caso nuevo y se lo muestra sin esa predicción. Coinciden | «Antes de seguir, quiero comprobar que entendí bien tu regla. Caso: US$ 400 000 al año, quedan 5 años, con cambio de control. ¿Cómo lo juzgas?». Rosa: «Cuenta: pesa lo que queda por pagar, US$ 2 M». «Coincidimos.» «2 escaneados no se pudieron leer: puedes subir su texto.» | bloque 1 §1.9 (hallazgos de lectores sin filtro del conductor, con su origen); requisito del encargo 8 (pasaje literal, sin opciones ni recomendación); §1.2 (teach-back: si acierta, sigue); §1.6 (predicción guardada antes de mostrar), aplicado aquí `[inferencia]`; doc 12 §7 («no se pudo comprobar» solo informa; el humano puede aportar el pasaje), extendido a una lectura `[inferencia]` |
| 3 | El agente presenta el resumen del lote 1 en una tabla. Cada número abre sus pasajes literales. Dice qué hizo, dónde quedó el contrato testigo y por qué, y registra su lectura como decisión con contraste. Rosa pulsa «No estoy de acuerdo». En la tabla, 4 filas quedan en espera | «Lote 1 · 75 contratos · 9 cuentan · 6 bajo US$ 1 M · 4 de US$ 1 M o más que solo revisan tarifas: fuera · 54 sin cláusula · 2 sin leer. Tu 47 quedó fuera: no permite terminar. Lo contaría si tu regla busca todo costo.» | doc 12 §6.1 punto 4 (resumen en tabla), aplicado a un lote `[inferencia]`; DEC-003 §2; decir dónde quedó el contrato testigo: bucle de comprensión (por diseñar); bloque 1 §1.3 (decisión con contraste; objetada: deja de regir, bloquea y su rama queda en espera); §1.6 («No estoy de acuerdo» sobre una decisión); si la búsqueda de choques la cruzara con el criterio de Rosa, habría nacido como propuesta y el desacuerdo la dejaría rechazada `[inferencia]` (§1.1) |
| 4 | En la tarjeta de la objeción, Rosa escribe otra decisión con sus palabras: «Cuenta si permite terminar o cambiar el precio, en vez de solo terminar, porque al comprador le importa lo que pierde en plata. Solo terminar si el precio de compra ya descuenta esos ajustes». La app levanta la espera. El agente vuelve a pasar a los lectores solo los contratos afectados y explica el cambio | «Decisión guardada con tus palabras.» «Cambié: 4 contratos pasan a "cuentan" (US$ 6,2 M), el 47 entre ellos. Por qué: tu decisión. En vez de: dejarlos fuera. Límite: en los lotes 2 a 4 busco también las cláusulas de tarifa.» | bloque 1 §1.3 (salidas de la objeción: escribir otra → nace suya; con sus palabras y su contraste → confirmada); B3 §8 (el agente no quita marcadores; los quita la app cuando se resuelve el vínculo), con vínculo resuelto según bloque 1 §1.8; doc 12 §5.2 D2 (lectura transversal) |
| 5 | Siguen los lotes 2 a 4, cada uno con su resumen. Al cerrar cada lote, la app elige un contrato entre las filas donde el agente tuvo que interpretar. Rosa dice en qué grupo cree que quedó, viendo solo el pasaje y el monto | «Contrato 212 · almacenaje · US$ 1,3 M · cláusula 11: "Si cambia el control del cliente, el almacén podrá renegociar las condiciones económicas". ¿En qué grupo crees que quedó?». Rosa: «Cuenta: renegociarlas es cambiar el precio». «Coincide con la tabla.» Barra: «Lote 3 de 4 · 2 casos comprobados». | comprobar el avance con un caso: bucle de comprensión (por diseñar); doc 12 §6.1–§6.2 (máximo 2 rondas por pregunta sin un acto humano de por medio; que este caso cuente como tal está por diseñar) |
| 6 | Al final, el agente suma la exposición con código. La suma y los montos convertidos quedan enlazados a la decisión de supuestos del cálculo, que es del agente y espera su tarjeta | «Tabla de riesgos · borrador · Falta: una tarjeta sobre por qué el tipo de cambio de hoy y no el de cada contrato (afecta US$ 14,8 M) · Solo informa: 5 contratos sin leer.» Lo que Rosa firmará es la tabla y el resumen; los pasajes están a un clic. | bloque 1 §1.3 (supuestos de un cálculo: decisión crítica); §1.4; §1.7 (pendientes en lenguaje natural; «no se pudo comprobar» solo informa); doc 12 §5.1 (cada lector devuelve ≤ 2k tokens de pasajes; si excede su presupuesto, se parte), §6.1 punto 5 (el informe completo queda fuera del documento) |

**Dónde nacería la deuda sin tupaqnin:** un chat con IA leería los 300 contratos y entregaría un informe de 80 páginas con un resumen: «19 contratos con riesgo». Rosa leería el resumen y firmaría. Las cláusulas de revisión de tarifas habrían quedado fuera por una interpretación del agente que nadie le mostró, incluida la del contrato que ella misma negoció. El comité pagaría sin descontarlas, y después de la compra los transportistas subirían las tarifas.

**Cómo se asegura que entiende**
- **Sin leer 80 páginas:** un resumen por lote en tabla, con los pasajes literales a un clic, que dice qué se hizo y con qué límite (pasos 3 y 5). Al cerrar cada lote, **un caso de comprobación**: la app elige un contrato de un grupo donde el agente interpretó, y Rosa dice dónde cree que quedó (paso 5). Si no coincide, el agente explica su lectura con su contraste y la vuelve una decisión registrada. Todo esto es **por diseñar** (bucle de comprensión). Insumo concreto para diseñarlo:
  - el caso lo elige la app, no el agente;
  - se toma de las filas que exigieron interpretar;
  - va en la frontera natural (fin de lote);
  - no es de sí o no: Rosa elige un grupo de la tabla o responde libre;
  - la barra muestra el avance: «Lote 3 de 4 · 2 casos comprobados · 0 lecturas sin explicar».
- **De Rosa hacia el agente:** el contrato testigo (paso 1) es un aporte suyo y, a la vez, una prueba de la lectura de los lectores. Es lo que destapa la interpretación del agente en el paso 3. Que el agente diga dónde quedó el testigo y por qué es **por diseñar**.
- **Del agente hacia Rosa:** el agente le devuelve cómo leyó su regla, con un caso nuevo (paso 2; §1.2, diseñado).
- **Acto de Rosa:** objetar y escribir su propia decisión con su contraste (pasos 3–4).

**Resultado esperado**
1. Cada encargo a los lectores lleva el criterio de Rosa en literal. Lo que devuelven son pasajes con su ubicación, sin opciones ni recomendación, y llegan a la app con su origen, sin pasar por el filtro del agente que escribe (bloque 1, requisitos del encargo 3 y 8; §1.9). Cuántos se muestran lo define el módulo 4 (pendiente).
2. El contrato testigo queda registrado como aporte de Rosa, con su razón (estado interno: acto `aporte` con resultado `confirma`) (§1.6; `tq-act/1` paso 7).
3. Tras «No estoy de acuerdo», la decisión «solo si permite terminar» deja de regir; sus 4 filas quedan en espera y bloquean la firma (estado interno: `objetada`, con `[POR ACLARAR]`). El agente no puede levantar la espera: la levanta la app cuando Rosa escribe otra decisión, elige una alternativa o retira la objetada (§1.3; B3 §8; vínculo resuelto: §1.8).
4. La decisión que Rosa escribe con sus palabras y su contraste rige sin tarjeta (estado interno: `confirmada`) (§1.3).
5. La tabla no puede firmarse mientras la decisión de supuestos del tipo de cambio no tenga su acto. Los 5 contratos sin leer solo informan (§1.4; §1.7; doc 12 §7, extendido a una lectura `[inferencia]`).

En el hackathon: lectores, rondas y resúmenes por ronda son «Después» (doc 12 §11); el aporte y el teach-back en el primer uso quedan para el producto (§1.10). Todo el caso es del producto.

**Qué prueba del diseño:** bloque 1 §1.9 (hallazgos sin filtro), requisitos del encargo, §1.2 (teach-back que acierta), §1.3 (objeción y sus salidas), §1.4, §1.6 (aporte), §1.7–§1.8; B3 §8; doc 12 §4.1, §5.1–§5.2, §6.1–§6.2, §7; módulo 4; módulo 6 (encargo a subagentes); bucle de comprensión.

Preguntas abiertas:
- La comprobación del avance con un caso no existe. ¿Es un acto con regla de resultado? ¿Cuenta como «acto humano de por medio» para la regla de 2 rondas por pregunta (doc 12 §6.1–§6.2)?
- El aporte confirma «lo vinculado» (§1.6), pero nada compara lo que Rosa dijo que el testigo prueba con lo que devolvieron los lectores. Hoy la discrepancia solo se ve si el agente la cuenta. ¿Debería la app marcarla?
- Una interpretación que el agente no registró como decisión («solo terminar») no tiene red propia: es el riesgo de las «afirmaciones sin cifra» (§1.4; «No verificado»). Aquí la destapa el contrato testigo.
- Una decisión del agente que interpreta un criterio del humano, ¿choca con él? Con la heurística provisional de §1.1 depende de los términos que compartan `[inferencia]`; si choca, nace como propuesta y «No estoy de acuerdo» la rechaza en vez de objetarla, con efectos distintos (§1.3, §1.5).
- Cuando el teach-back acierta, la respuesta de Rosa queda en el acto (anexo A1.2), pero no como ejemplo del criterio. Su matiz («lo que queda por pagar») no llega a los encargos de los lectores, que solo llevan el criterio.
- Un contrato sin leer solo informa (doc 12 §7). En una due diligence ante terceros es material: ¿el destino «ante terceros» debería volverlo bloqueo?
- ¿Quién arma el resumen del lote: el agente que escribe (y entonces filtra) o la app desde los hallazgos? §1.9 y doc 12 §6.1 no lo dicen.
- ¿Un lote es una ronda, para el tope de 4 lectores y para la regla de 2 rondas? Y con ≤ 2k tokens de pasajes por encargo (doc 12 §5.1), 300 contratos exigen muchos encargos partidos: ¿cuántos caben en el costo estimado del plan?
- Los montos copiados de un contrato: ver la pregunta abierta sobre mediciones de una fuente en el caso 4.

---

## Caso 4 · «Dije "ok, dale" a un plan que no podía explicar»

**Ficha:** Martín, ingeniero de software líder · documento de diseño para migrar la base de datos de pedidos de una tienda en línea a un motor de base de datos distinto · interno (equipo de ingeniería) · decisión técnica donde el agente sabe más en un ángulo; **Martín conduce y redacta** · ingeniería de software. Ficticio. *Corte* = se detiene la escritura, se copia lo último y se arranca con la base nueva. *Escritura dual* = durante unas semanas, cada cambio se guarda en las dos bases.

**Qué aporta cada uno**
- **Humano:**
  - *Objetivo:* migrar sin que los clientes lo noten.
  - *Criterio*, escrito por él: «Cero caída del servicio; el equipo es de 3».
  - *Decisión y borrador propios:* redacta «Plan de migración» con su decisión: «Copiar todo y cortar un domingo de madrugada, con las escrituras congeladas unos minutos, en vez de migrar en caliente; migraría en caliente si fuéramos más de 3».
  - *Nociones:* conoce el sistema, el negocio y la escritura dual en su variante habitual. Sabe que el equipo no aguanta un proceso largo y frágil.
- **Agente:**
  - *Conocimiento:* migraciones en línea entre motores distintos: escritura dual, copia del histórico, conciliación, cambio de lecturas y cómo volver atrás.
  - *Subagentes, en paralelo:* un juez revisa el borrador de Martín desde dos ángulos (si el plan se sigue de sus criterios; qué objetaría un revisor externo). Un lector procesa con código los registros de tráfico de 30 días que subió Martín. El agente consulta en línea la documentación del motor nuevo.
  - *Aporta* lo que Martín no tenía: el hueco de su plan y una variante de escritura dual que permite volver atrás sin perder pedidos.

**Pasos**

| Paso | Qué pasa | Qué ve el humano | Regla que lo decide |
|---|---|---|---|
| 1 | Martín escribe su criterio y su decisión con sus palabras, contraste incluido: nacen confirmados. Redacta «Plan de migración» y pide «Auditar». Corren la revisión independiente y la lectura de registros; el agente consulta la documentación del motor. El hallazgo llega a la app sin que el agente lo filtre. | «Revisión de "Plan de migración": el congelamiento final detiene los pedidos; el tráfico nunca llega a cero, ni de madrugada. Entre motores distintos no hay réplica nativa que lo acorte. Choca con tu regla "Cero caída del servicio".» Martín lo marca «error». | bloque 1 §1.2 y §1.3 (sus palabras, contraste incluido → confirmados); doc 12 §6.3 («Auditar» a pedido; juez con dos lentes; «¿error, excepción o no aplica?»); doc 12 §5.2 D5 (datos tabulares con código); doc 12 §6.2 (hecho puntual en línea); bloque 1 §1.9 (hallazgos sin filtro); módulo 4 (pendiente: tarjeta de hallazgos) |
| 2 | El agente propone la escritura dual con su contraste y un esquema. Su decisión repite la pregunta de la de Martín, así que nace como propuesta de revocación y no rige. Es crítica: revoca algo del humano y afecta 3 secciones. | «Propongo **escritura dual en vez de corte**: cada cambio va a las dos bases; luego leemos de la nueva; la antigua sigue recibiendo cada cambio hasta que la apaguemos. Elegiría el corte si el negocio aceptara unas horas sin pedidos. Límite: semanas extra y conciliación diaria.» Esquema: 4 fases. | DEC-003 §2 (quien propone explica); bloque 1 §1.1 (choque → propuesta con `supersedes`); §1.3 (contraste); §1.4 (criticidad: revoca algo del humano; afecta 2 o más secciones) |
| 3 | Martín escribe en el chat «ok, dale, reescríbelo». No pasa nada: un «ok» no adopta ni responde tarjetas. Mientras la propuesta esté pendiente, el agente no escribe con ella; sigue con lo que no depende de ella. | La tarjeta de la propuesta sigue a la vista, con su decisión en literal: «Esto revocaría: "Copiar todo y cortar un domingo de madrugada…"». Barra: «Pendientes 1 · ~3 min para revisar antes de firmar». | bloque 1 §1.5 («No adoptan: un "ok" en el chat»; no escribe con lo propuesto); §1.6 regla 1 y lista de asentimientos; §1.6 ciclo de vida (el acto no bloquea) |
| 4 | Martín pulsa «Adoptar». La decisión del agente rige, pero queda pendiente y crítica, y se revisa lo que dependía de la anterior. El agente reescribe «Plan de migración», «Riesgos» y «Cronograma». La app abre la tarjeta de aplicar. Martín elige el desenlace del corte, porque «al leer de la nueva, la antigua deja de recibir escrituras». | «Al tercer día, la base nueva resulta tener mal las fechas de 2 000 pedidos. ¿Cómo se vuelve atrás?» · «Las lecturas vuelven a la antigua, que recibió cada cambio; no se pierde ningún pedido» · «Se restaura el respaldo previo al corte y se rehacen a mano los pedidos de esos días» | bloque 1 §1.5 (tabla: «Adoptar» → rige, `pendiente` y crítica; barrido); §1.6 (aplicar: dos desenlaces, «porque», orden sorteado, validación, predicción guardada antes); `tq-act/1` paso 6 → `difiere` |
| 5 | El agente revela qué esperaba y explica su plan con el esquema: Martín leyó la variante que conoce, no esta. Queda en cola un caso nuevo. Martín prefiere escribir la decisión él, con sus palabras y su contraste: «Escritura dual hasta apagar la antigua, en vez de corte con escrituras congeladas; cortaría si el negocio aceptara 2 horas sin pedidos un domingo. Máximo 3 semanas: somos 3». Nace confirmada. La del agente queda reemplazada y el caso en cola se descarta. | «Esperaba la primera. En la variante que conoces, la antigua deja de recibir datos al pasar las lecturas; en esta, los recibe **hasta la fase 4**. Por eso volver atrás es cambiar de dónde se lee.» Decisión nueva, tuya: «Escritura dual hasta apagar la antigua…». | bloque 1 §1.6 «Después de un `difiere`» (revela y explica; caso nuevo, < 50 % de términos comunes); §1.3 (edición que cambia el fondo, sus palabras con contraste → `confirmada`); §1.5 «Al hacerse efectiva una revocación» 1–4 (`reemplazada`; acto en cola `obsoleto`); bucle de comprensión (por diseñar: ofrecer reescribir tras un solo desacuerdo) |

**Dónde nacería la deuda sin tupaqnin:** en un chat común, el «ok, dale» del paso 3 deja que el agente reescriba las tres secciones. El documento diría «escritura dual» con la firma de Martín, pero él la entiende como la variante que conoce, donde la base antigua deja de recibir datos y no hay vuelta atrás limpia. El día del incidente, Martín no podría defender su propio plan ante su equipo.

**Cómo se asegura que entiende**
- **Qué explica el agente:** el hueco del plan de Martín, con su razón (el tráfico nunca llega a cero). Explica la escritura dual con su contraste («elegiría el corte si…»), sus límites (semanas extra, conciliación diaria) y un esquema de 4 fases. Cuando la respuesta de Martín no coincide, le muestra en el esquema la diferencia exacta entre su variante y esta: la base antigua sigue al día hasta la fase 4.
- **Qué acto hace Martín:** marca el hallazgo como error de su propio plan. Responde qué pasa en un caso concreto, con un «porque». Tras ver la explicación, escribe la decisión con sus palabras, y al escribirla deja dicho cómo la defendería. El agente no lo vuelve a examinar.
- **Qué le devuelve el agente:** la primera vez que reescribe una sección con «Cero caída», le muestra cómo entendió esa regla. Primero va el caso; su lectura se revela después de la respuesta: «Antes de seguir con la reescritura, un caso: al pasar las lecturas a la base nueva, el sitio queda 30 segundos sin poder pagar, aunque el catálogo se ve. ¿Cómo lo juzgas con tu regla?». Si Martín responde distinto, su respuesta queda como ejemplo con sus palabras (bloque 1 §1.2; formato del caso por diseñar).
- **Por diseñar (bucle de comprensión):** que el agente ofrezca «si prefieres, escríbela tú» tras una sola respuesta que no coincidió, y cómo comprueba que Martín entiende los cambios **no críticos** de «Riesgos» y «Cronograma». Hoy solo se comprueba la decisión crítica.

**Resultado esperado**
1. El «ok, dale» del chat no cambia el estado de la propuesta ni de ninguna tarjeta, y el agente no escribe con lo propuesto mientras está pendiente. *(bloque 1 §1.5; §1.6 regla 1)*
2. La decisión de Martín rige hasta que él pulsa «Adoptar»; el agente no puede revocarla. *(bloque 1 §1.1; §1.5)*
3. Tras «Adoptar», la decisión del agente rige, pero sigue pendiente (estado interno: `pendiente`, crítica) y bloquea la firma de las 3 secciones hasta un acto. *(bloque 1 §1.5 tabla; §1.7 «Bloquea»)*
4. Tras la respuesta que no coincide, el agente revela su predicción y explica. El caso siguiente comparte menos del 50 % de términos con el anterior, y un «porque» que copie esa explicación no es válido. *(bloque 1 §1.6)*
5. La decisión que escribe Martín nace confirmada (estado interno: `confirmada`). La del agente queda reemplazada (`reemplazada`) y el caso en cola, descartado (`obsoleto`). *(bloque 1 §1.3; §1.5 pasos 1–3)*

**Qué prueba del diseño:** el humano conduce y el agente audita (DEC-002); «Auditar» a pedido y hallazgos sin filtro (doc 12 §6.3; bloque 1 §1.9); el choque con una decisión del humano (§1.1); un «ok» que no adopta (§1.5, §1.6); la tarjeta de aplicar y el ciclo tras una respuesta que no coincide (§1.6); la reescritura humana como reemplazo (§1.3, §1.5).
- **En el hackathon:** la propuesta del paso 2 se rechaza al nacer y el agente se lo dice a Martín en el chat; Martín revoca editando su propia decisión (bloque 1 §1.5, E3). La tarjeta de aplicar y `tq-act/1` sí entran; la lectura del criterio en su primer uso, no (§1.10). «Auditar» queda para después (doc 12 §11).
- **Preguntas abiertas:**
  - Reescribir la decisión con sus palabras la confirma al nacer, con el filtro de copia del 80 % como única red. ¿Basta si Martín parafrasea la explicación del agente? (bloque 1 §1.2; «No verificado»)
  - Un «ok» en el chat con una tarjeta abierta: ¿la app o el agente le recuerda la tarjeta? No está diseñado (§1.6 regla 1).
  - ¿Qué va primero, la lectura del criterio en su primer uso o el aplicar de la decisión? La prioridad del §1.6 no ubica la primera.
  - Responder la tarjeta de adoptar eligiendo la nueva, con un «porque», la confirma sin caso (§1.5 tabla). ¿Es un atajo frente al aplicar?
  - **Mediciones de una fuente** (D7): si el agente escribe en el texto una medición tomada de los registros que subió Martín (por ejemplo, escrituras por minuto), el §1.4 la trata como «cifra sin decisión» y pide colgarla de una decisión y justificarla. ¿Debería tratarse como dato con fuente?

---

## Caso 5 · «La cita que más me convenía no existía»

**Ficha:** Paula, investigadora doctoral en ingeniería ambiental · capítulo 2 de su tesis: estado del arte (revisión de lo publicado) sobre microplásticos en agua potable · ante terceros (comité de tesis) · investigación · ciencias ambientales. Ficticio, incluidos los artículos y los conteos. El capítulo compara conteos y métodos; **no evalúa efectos en la salud ni recomienda qué agua tomar**. *DOI* = el código único de cada artículo publicado. *Micra* = milésima de milímetro.

**Qué aporta cada uno**
- **Humano:**
  - *Objetivo:* un estado del arte que su comité no pueda desarmar.
  - *Criterio:* «Solo estudios de campo, no de laboratorio».
  - *Hipótesis:* «el agua embotellada trae más partículas que la del grifo».
  - *Decisiones:* qué cuenta como «campo» en los casos dudosos; cuándo una ronda ya no vale lo que cuesta.
- **Agente:**
  - *Conocimiento:* los conteos dependen del tamaño mínimo de partícula que detecta cada filtro; dos estudios con filtros distintos no se comparan tal cual.
  - *Subagentes:* 4 investigadores web en paralelo (agua de grifo, agua embotellada, plantas de tratamiento, métodos de detección). Luego, la verificación de cada cita como paso aparte. Calcula con código la comparación entre estudios.
  - *Propone* una regla de comparación que Paula no tenía, y cortar una ronda que no avanzó.

**Pasos**

| Paso | Qué pasa | Qué ve el humano | Regla que lo decide |
|---|---|---|---|
| 1 | Paula escribe su criterio. «Síntesis» tiene un marcador suyo sobre grifo frente a embotellada. El agente arma la ronda, nombra qué puede resolver y la lanza sin esperar un «ok», porque cabe en el tope. Paula agrega su hipótesis en una frase, que no es un acto. | «Ronda: qué dicen los estudios **de campo** sobre microplásticos en agua de grifo y embotellada · Puede resolver: el "por aclarar" de "Síntesis" · 4 investigadores en paralelo · ~US$ 1,20 · [Cancelar] [Ajustar] · Tu hipótesis (opcional): ___» | bloque 1 §1.2 (criterio confirmado); §1.8 (el humano crea marcadores); doc 12 §6.1 (1) regla de inicio, (2) se lanza sola; §5.3 (la decisión o marcador que podría cambiar); §6.2; §4.3; bloque 1 §1.6 (la hipótesis no es acto) |
| 2 | Los 4 investigadores trabajan en paralelo, con el criterio de Paula en literal en cada encargo. Devuelven 60 artículos, cada uno con el pasaje de su método. La verificación revisa cada cita: el DOI en el registro bibliográfico y la cita sobre el texto obtenido. Un investigador trajo una referencia con autores, año y título verosímiles que no existe. | «Verificación de citas: 57 respaldadas · 2 sin comprobar (muro de pago; puedes aportar el pasaje) · **1 inexistente**: "Valdivia y Ortiz, 2022" no está en el registro de DOI ni con su título. Era la que más apoyaba tu hipótesis. No la uso.» | doc 12 §6.1 (3) verificación como paso aparte; §7 capa 1 y cuarto estado; bloque 1 «Requisitos mínimos del encargo» 3 y 8; módulo 4 (pendiente; base doc 12 §7) |
| 3 | El agente entrega el digesto en tabla, contrastado con la hipótesis, y propone una regla de comparación con su contraste. Nace crítica: son los supuestos de un cálculo y afecta «Síntesis» y «Conclusiones». Calcula con código la comparación; el resultado queda oculto hasta que Paula prediga. | Tabla de hallazgos, con fuentes y certeza. «Tu hipótesis: embotellada > grifo. Los conteos crudos dan ~60 veces más, pero mezclan filtros que ven tamaños distintos.» Propuesta: «Comparar solo estudios con el mismo tamaño mínimo, **en vez de** los conteos tal cual; elegiría estos si todos usaran el mismo filtro.» | doc 12 §6.1 (4) digesto en tabla; §4.1 (acto solo en lo que mueve una decisión crítica); bloque 1 §1.3 (contraste; supuestos de un cálculo, crítica); §1.4 (criticidad); doc 12 §7.1 (oráculo con resultado oculto) |
| 4 | El agente escribe «Estudios incluidos» aplicando el criterio: 41 de los 60 son de campo. Es el primer uso del criterio, así que le muestra cómo lo entendió: primero el caso, después su lectura. Paula responde distinto («fuera: el tratamiento es simulado»). Su respuesta queda como ejemplo, con sus palabras, y lo escrito con la lectura equivocada queda pendiente. El agente reclasifica. | «Apliqué tu criterio a los 60 artículos; antes de seguir, dime cómo juzgarías este: un estudio trata agua cruda de una planta real en un piloto de laboratorio. ¿Cómo lo juzgas?» Paula: «Fuera: el tratamiento es simulado.» «Yo lo había incluido. De los 41, salen 6; quedan 35 [lista con su método].» | bloque 1 §1.2 (lectura del criterio en su primer uso, «teach-back»; si no coincide: `ejemplo`, señal «el agente lo leyó mal», pasajes «escrito con una lectura equivocada de tu criterio»); B3 §8 `open_act` (sin sí o no); predicción revelada después (§1.6 `[inferencia]` para el teach-back) |
| 5 | La app muestra el alcance del cálculo y pide la predicción. La de Paula y el resultado difieren en orden de magnitud, así que la pregunta no es «¿aceptas?». Paula responde que se equivocó su expectativa: venía de los conteos crudos. La decisión de comparación sigue esperando su propio acto. | «Con los 5 estudios de los 35 que usan el mismo tamaño mínimo (partículas por litro, 2015–2026), ¿cuántas veces más partículas trae la embotellada?» Paula: «unas 20». Resultado: «1,5 veces». «¿Quién se equivoca: tu expectativa, el cálculo o la afirmación?» Esquema: estudios agrupados por tamaño mínimo. | doc 12 §7.1 (ficha con alcance antes de predecir; «¿quién se equivoca…?» si difieren en dirección u orden de magnitud); bloque 1 §1.6 (predecir confirma la afirmación que el oráculo prueba; la decisión crítica del agente pide su aplicar) |
| 6 | El agente lanza una segunda ronda para buscar más estudios con el mismo filtro. No aparece ninguno: solo estudios de laboratorio, que el criterio excluye, y uno sin método declarado. El avance neto es negativo y el agente propone cortar. Paula no da razón para seguir y la ronda se corta. | «Esta ronda no movió nada: 0 estudios nuevos comparables, 0 decisiones cambiadas y 1 "por aclarar" nuevo (un estudio que no declara su filtro). Propongo cortar aquí; si quieres seguir, dime por qué. Gastado: US$ 0,90.» · [Cortar] [Seguir porque: ___] | doc 12 §6.1 (6) avance neto: si es cero o negativo, propone cortar y seguir exige una razón; §4.1 «Corte de ciclo» (si no hay respuesta, se corta); diagnóstico L-09 |

**Dónde nacería la deuda sin tupaqnin:** un chat común le entrega a Paula 40 páginas sobre 60 artículos. No las alcanza a leer, y encuentra lo que esperaba: «la embotellada trae 60 veces más partículas», con la cita de Valdivia y Ortiz. La cita inventada entra al capítulo, y también los 6 estudios de tratamiento simulado, porque el agente leyó «campo» a su manera. Paula lo presenta al comité sin entenderlo lo bastante para notarlo: es el caso Deloitte en una tesis.

**Cómo se asegura que entiende**
- **Qué explica el agente:**
  - el digesto en tabla, no en prosa, contrastado con lo que ella esperaba;
  - qué cita no existe y por qué (no está en el registro de DOI ni con su título), y que la trajo uno de sus investigadores;
  - la regla de comparación con su contraste y su límite: deja solo 5 estudios comparables;
  - un esquema: los estudios agrupados por el tamaño mínimo que detecta su filtro;
  - por qué la segunda ronda no valió su costo.
- **Qué actos hace Paula:**
  - corrige cómo leyó el agente su criterio, juzgando un caso dudoso con sus palabras;
  - predice la comparación antes de verla; la diferencia (20 frente a 1,5) le muestra que su expectativa venía de mezclar filtros;
  - decide si la investigación sigue. No lee los 60 artículos: el acto cae solo en lo que mueve una decisión.
- **Qué le devuelve el agente:** su lectura del criterio, para que ella la corrija (DEC-003 §4).
- **Por diseñar (bucle de comprensión):** tras reclasificar, Paula debería comprobar que entiende **por qué salieron** los 6 estudios. Un acto posible: un artículo de la lista, que ella clasifica sin ver la decisión del agente. Hoy solo existe la lectura previa del criterio.

**Resultado esperado**
1. «Valdivia y Ortiz (2022)» aparece en la verificación como «fuente inexistente» y no se usa en el texto. Que no se use es `[inferencia]`: ninguna regla lo impide explícitamente. *(doc 12 §7; módulo 4 pendiente)*
2. El criterio sigue confirmado después de la lectura que no coincidió, y la respuesta de Paula queda como ejemplo con sus palabras. *(bloque 1 §1.2)*
3. Mientras el agente no reescriba esos pasajes, «Estudios incluidos» muestra «Falta: escrito con una lectura equivocada de tu criterio», que bloquea la firma. *(bloque 1 §1.2; §1.7)*
4. Como la predicción y el resultado difieren en orden de magnitud, la tarjeta pregunta «¿quién se equivoca…?». La decisión de comparación sigue pendiente (estado interno: `pendiente`, crítica) y bloquea la firma de «Síntesis» hasta su acto. *(doc 12 §7.1; bloque 1 §1.6; §1.7)*
5. La segunda ronda muestra su avance neto y la propuesta de cortar. Sin una razón de Paula, no hay tercera búsqueda. *(doc 12 §6.1 (6); §4.1)*

**Qué prueba del diseño:** la ronda con tarjeta, digesto, avance neto y corte (doc 12 §6.1); los investigadores en paralelo con el criterio en literal (doc 12 §6.2; requisitos del encargo); la verificación como paso aparte (doc 12 §7); la lectura del criterio en su primer uso (bloque 1 §1.2); predecir antes de ver (doc 12 §7.1); el acto solo sobre lo que mueve una decisión (doc 12 §4.1).
- **En el hackathon:** no hay rondas ni investigadores (doc 12 §11), ni lectura del criterio en su primer uso ni predecir (bloque 1 §1.10). La verificación mínima de citas es un «Debería» (doc 12 §11).
- **Preguntas abiertas:**
  - ¿Una cita «fuente inexistente» bloquea la firma? La tabla «Bloquea / Solo informa» del bloque 1 §1.7 no la lista. Tampoco hay nada que impida escribirla antes de que la verificación termine.
  - Una cifra que sale de aplicar un criterio del humano («quedan 35 estudios») no tiene una decisión natural de la que colgar. ¿Se vincula al criterio? (bloque 1 §1.4)
  - Predecir confirma la afirmación, pero la decisión de supuestos sigue pidiendo su aplicar. ¿Dos actos para una misma comprensión? (bloque 1 §1.6; doc 12 §7.1)
  - ¿Cómo cuenta el avance neto una decisión nueva, o un marcador que solo el humano puede cerrar? (doc 12 §6.1 (6); bloque 1 §1.8) `[inferencia]`
  - Reclasificar los 60 tras la lectura equivocada es otra lectura pesada. ¿Se lanza sola o queda propuesta? Su costo cuenta contra el tope (doc 12 §4.4).

---

## Caso 6 · «Al reescribir, volvió el cierre que yo había descartado»

**Ficha:** Carmen, ingeniera estructural · evaluación sísmica de un colegio privado de 3 pabellones de concreto de los años 80, y propuesta de refuerzo · ante terceros (dirección del colegio y asociación de padres) · auditoría y reescritura · ingeniería estructural. Ficticio. *Columna corta* = columna que un muro bajo deja libre solo en su parte alta, y que concentra el esfuerzo en un sismo. *Encamisar* = envolver la columna con concreto y acero nuevos. *Junta* = una separación entre el muro y la columna.

**Qué aporta cada uno**
- **Humano:**
  - *Objetivo:* reforzar el colegio sin dejar a 300 alumnos sin aulas.
  - *Decisión*, escrita por ella con su contraste: «El colegio sigue funcionando durante la obra: se refuerza por etapas, un pabellón a la vez, en vez de cerrarlo un año; cerraría si un pabellón ocupado pudiera colapsar con un sismo moderado».
  - *Criterio de oficio:* juzga qué hallazgo es un error y cuál no aplica.
- **Agente:**
  - *Conocimiento:* vulnerabilidades típicas de colegios de esa época y la norma sismorresistente vigente.
  - *Subagentes:* una revisión independiente de la propuesta desde tres flancos (norma, cargas, costo).
  - *Propone* lo que Carmen no tenía: una solución concreta para la falla que encontró la revisión.

**Pasos**

| Paso | Qué pasa | Qué ve el humano | Regla que lo decide |
|---|---|---|---|
| 1 | «Propuesta de refuerzo» tiene borrador y pasa a pendientes de firma. Es una sección central que va ante terceros, así que la revisión independiente corre sola, sobre la versión congelada. Los hallazgos llegan a la app sin que el agente los filtre. | «Revisión independiente: **norma** · en el pabellón C, los muros bajos de las ventanas dejan columnas cortas · **cargas** · el tanque de agua de la azotea no está en el modelo · **costo** · por etapas cuesta ~30 % más y dura el doble.» Carmen: norma y cargas, «error»; costo, «no aplica». | doc 12 §6.3 (E1; alcance congelado; «¿error, excepción o no aplica?»); bloque 1 §1.9 (hallazgos sin filtro); módulo 4 (pendiente: tarjeta y cuántos se muestran); módulo 6 (pendiente: tres flancos frente a un juez con dos lentes) |
| 2 | El agente corrige solo lo marcado «error». Edita «Diagnóstico», que Carmen ya había firmado: la sección pasa a «cambió desde que la firmaste», con los cambios agrupados. Propone una decisión con contraste y un esquema. | «Diagnóstico · cambió desde que la firmaste: + columnas cortas en el pabellón C · + tanque de la azotea [ver cambios].» Propuesta: «Separar los muros bajos de las columnas con una junta, **en vez de** encamisarlas; elegiría el encamisado si además les faltara resistencia.» Esquema: la columna antes y después. | doc 12 §6.3 (corrige solo lo marcado «error»); bloque 1 §1.5 (lo firmado se edita; «cambió» agrupado); §1.7 (marca «cambió»); §1.3; DEC-003 §2 |
| 3 | Al reescribir «Secuencia de obra», el agente razona que las columnas cortas podrían cumplir la condición de Carmen. Registra «Cierre total durante la obra» y, en el mismo turno, escribe un párrafo que lo recomienda. La decisión choca con la de Carmen: nace como propuesta y no rige. El chequeo detecta que el párrafo la contradice. «Cronograma» queda estacionado. | Subrayado: «se recomienda desocupar el colegio durante toda la obra». Propuesta: «Esto revocaría tu decisión "El colegio sigue funcionando…". Te lo planteo porque las columnas cortas podrían cumplir tu "cerraría si…"; lo descartaría si el pabellón C se desocupa durante su refuerzo.» En «Cronograma»: [POR ACLARAR]. | bloque 1 §1.1 (choque → propuesta con `supersedes`); §1.5 (ramas estacionadas con `[POR ACLARAR]` vinculado a la propuesta); DEC-003 §2 (quien propone explica); doc 12 §3 (chequeo de contradicciones en cada cambio); B3 §5 (subrayado solo para contradicción) |
| 4 | Carmen abre el aviso y clasifica la contradicción. La tarjeta muestra solo los dos pasajes; el juicio del agente se ve después de responder. Elige «el texto está mal», que no exige «porque». El agente corrige el párrafo. | «Tu decisión: "El colegio sigue funcionando durante la obra…" · El texto: "se recomienda desocupar el colegio durante toda la obra" · [El texto está mal] [Es una excepción] [La regla cambia] [No aplica].» Después: «Yo lo había juzgado "la regla cambia", por las columnas cortas. Corregido [ver cambios].» | bloque 1 §1.6 (clasificar: cuatro opciones, solo los pasajes, sin «No estoy de acuerdo»); `tq-act/1` paso 7; §1.7 («el texto está mal» bloquea hasta corregir); B3 §8 `judge_contradiction`; C12 |
| 5 | La propuesta sigue pendiente: corregir el texto no la resuelve. Carmen la abre, ve su decisión en literal y pulsa «No estoy de acuerdo»: la propuesta queda rechazada, sin pedirle razones. El vínculo del marcador queda resuelto y la app lo quita. Carmen escribe con sus palabras una decisión nueva para el pabellón C. | Su decisión en literal, junto a la del agente. Carmen: «No estoy de acuerdo». Nueva: «El pabellón C se desocupa solo durante su refuerzo, en vez de reforzarlo con clases; lo haría con clases si la junta cupiera en vacaciones.» «Cronograma» ya no muestra el marcador. | bloque 1 §1.6 («No estoy de acuerdo» sobre una propuesta → `rechazada`; `tq-act/1` paso 3); §1.8 (vínculo resuelto: la propuesta salió de `pendiente` por un gesto del humano); B3 §8 `edit_section` (lo quita la app o el humano, no el agente); §1.3 (de su autoría con contraste → `confirmada`); §1.5 (solo el humano revoca) |
| 6 | Se vuelven a juzgar solo los pasajes corregidos, en contexto nuevo y sin hallazgos nuevos. La decisión de la junta, del agente y crítica, sigue esperando su acto. | «Revisión de lo corregido: los pasajes de los 2 hallazgos quedan resueltos.» Estado: «Propuesta de refuerzo · borrador · Falta: un caso sobre "junta en vez de encamisado"» · «Diagnóstico · cambió desde que la firmaste». | doc 12 §6.3 (re-juicio solo de lo corregido); bloque 1 §1.6 (aplicar para una crítica del agente); §1.7 (estado y pendientes en lenguaje natural) |

**En el hackathon:** la propuesta del paso 3 se rechaza al nacer y el agente se lo dice a Carmen en el chat. Si ella quisiera cerrar, lo haría editando su propia decisión. El paso 5 no existe (bloque 1 §1.5, E3). La clasificación del paso 4 sí entra (§1.10). La revisión independiente queda para después (doc 12 §11).

**Dónde nacería la deuda sin tupaqnin:** un chat común reescribe la sección con los hallazgos y, de paso, devuelve el cierre total: un párrafo razonable, bien argumentado por seguridad. La decisión de Carmen se revierte en silencio, como en un proyecto real previo a tupaqnin, donde de 7 decisiones sobrevivieron unas 2. Si Carmen no relee ese párrafo, firma ante la asociación de padres un plan que dice lo contrario de lo que decidió, y lo descubre cuando la dirección pregunta dónde van a estudiar los alumnos.

**Cómo se asegura que entiende**
- **Qué explica el agente:**
  - *qué cambió:* 2 correcciones en «Diagnóstico» y la solución de la junta, con un esquema de la columna antes y después;
  - *por qué:* la columna corta concentra el esfuerzo;
  - *en vez de qué:* el encamisado;
  - *límites:* el costo es una estimación;
  - *por qué propuso el cierre:* su condición «cerraría si…» podría cumplirse, y cuándo lo descartaría. Así le avisa como un colega, en vez de decidir por ella.
- **Qué actos hace Carmen:**
  - clasifica los hallazgos con su oficio;
  - clasifica la contradicción viendo solo los dos pasajes, para que el juicio del agente no la ancle;
  - rechaza la propuesta viendo su propia decisión en literal, no una paráfrasis;
  - escribe con sus palabras cómo se cumple su condición en el pabellón C.
- **Por diseñar (bucle de comprensión):** al volver a «Diagnóstico» firmado, Carmen debería poder explicar qué cambió desde su firma antes de volver a firmar. Hoy eso pasa recién en la puerta.

**Resultado esperado**
1. «Cierre total durante la obra» queda como propuesta pendiente que revocaría la decisión de Carmen (estado interno: `pendiente`, `supersedes`), y no rige hasta el paso 5. Desde el paso 5 queda rechazada (`rechazada`). *(bloque 1 §1.1; §1.5; §1.6)*
2. La decisión de Carmen sigue confirmada y rige durante todo el caso. Solo un gesto suyo podría revocarla. *(bloque 1 §1.5)*
3. La tarjeta de clasificar muestra 4 opciones, no tiene botón «No estoy de acuerdo» y no muestra el juicio del agente antes de la respuesta. *(bloque 1 §1.6)*
4. El agente no puede quitar el marcador de «Cronograma». Después del rechazo, lo quita la app o Carmen; antes del rechazo, nadie puede, salvo Carmen. *(B3 §8; bloque 1 §1.8, vínculo resuelto)*
5. «Diagnóstico» pasa de «firmada» a «cambió desde que la firmaste», con una sola marca para los cambios del tramo. «Propuesta de refuerzo» queda en «borrador» por la decisión de la junta sin acto. *(bloque 1 §1.5; §1.7)*

**Qué prueba del diseño:** el choque y la propuesta (§1.1, §1.5); el candidato de contradicción sobre el texto (doc 12 §3, §7; módulo 4); la tarjeta de clasificar (§1.6); el rechazo de una propuesta (§1.6); la edición de lo firmado (§1.5); la revisión independiente E1 y su re-juicio (doc 12 §6.3).
- **Preguntas abiertas:**
  - La matriz pide tres flancos (norma, cargas, costo), pero el doc 12 §6.3 define un juez con dos lentes y descarta varios jueces de la misma familia. ¿Los flancos son lentes de un mismo encargo? → módulo 6. El caso 10 debe tratarlo igual.
  - ¿Qué tarjeta usan los hallazgos del juez («error · excepción · no aplica») y cuántos se muestran? → módulo 4.
  - El párrafo del cierre se escribió en el mismo turno en que nació la propuesta. ¿Se estaciona, se revierte o solo lo atrapa el candidato de contradicción? El §1.5 solo dice que el agente no escribe con lo propuesto *mientras* esté pendiente.
  - ¿Quién detecta que un hallazgo cumple la condición «lo elegiría si» de una decisión del humano? Aquí lo notó el agente por su cuenta; ninguna regla lo dispara (§1.3).
  - **Un conflicto, dos gestos** (D5): clasificar «el texto está mal» no rechaza la propuesta que originó el texto, y Carmen tiene que rechazarla aparte. ¿Debería una cosa resolver la otra?
  - **Choque B1 §1.8 frente a B3 §8: alinear el bloque 1.** El §1.8 deja al agente quitar un marcador resuelto; el B3 §8, aprobado, no. Rige el B3 §8 en los casos 6, 7 y 10.
  - Si Carmen eligiera mantener su decisión desde la tarjeta de adoptar, se le pediría un «porque»: ver la pregunta común del caso 2.

---

## Caso 7 · «Volví de las reuniones y nada estaba firmado a mi nombre»

**Ficha:** Diego, analista financiero de una consultora · caso de negocio de una planta solar de 40 MW (megavatios) para abastecer a una mina · ante terceros (comité de inversiones de la minera) · construcción en modo «continúa hasta terminar» · finanzas de proyectos. Ficticio, incluidas las cifras. *VAN* = valor actual neto: cuánto vale hoy el proyecto, sumando sus flujos futuros descontados. *Tasa de descuento* = la rentabilidad mínima que la empresa exige para invertir.

**Qué aporta cada uno**
- **Humano:**
  - *Objetivo:* saber si a la minera le conviene construir la planta o seguir comprando energía de la red.
  - *Criterio:* «Tasa de descuento corporativa: 10 %, no se discute».
  - *Insumos:* sube el plan de mina, la curva de consumo y dos cotizaciones de inversión.
  - *Decisión que solo él puede tomar:* qué pasa con la planta cuando cierre la mina.
- **Agente:**
  - *Conocimiento:* evaluación de proyectos (VAN, costo de la energía a lo largo de la vida de la planta, degradación de paneles).
  - *Construye* el modelo completo con código.
  - *Corre* 1 000 escenarios de sensibilidad sobre irradiación, inversión, precio de la energía y degradación.
  - *Detecta* algo que Diego no había visto: la mina cierra en 12 años y la planta dura 25.

**Pasos**

| Paso | Qué pasa | Qué ve el humano | Regla que lo decide |
|---|---|---|---|
| 1 | A las 9:00, Diego escribe su criterio, sube los archivos y escribe «Continúa hasta terminar, estoy en reuniones hasta las 6. Si surge algo, tú decides». El agente muestra una línea que no espera respuesta y da un único aviso. | «Entiendo terminar como: las 6 secciones en versión revisable · paro solo ante algo que solo tú puedes decidir, o al llegar al tope · tope US$ 8 · no confirmo ni firmo nada.» «"Tú decides" no me deja decidir por ti: lo que elija quedará como pendiente tuyo.» | doc 12 §4.4 (al activar el modo); §4.2 (un único aviso ante una autorización anticipada); bloque 1 §1.6 (lista de autorizaciones anticipadas); §1.5 (no adoptan) |
| 2 | El agente construye el modelo y corre los 1 000 escenarios con el 10 % de Diego. Registra los supuestos como una decisión crítica con contraste y le vincula cada cifra que escribe. Elige de forma provisional entre alternativas de contenido y las marca. | En «Evaluación»: «En ▒ % de los escenarios el VAN sale negativo» (oculta hasta que predigas; se recalcula cuando decidas el horizonte). Decisión pendiente: «Irradiación: serie satelital de 20 años, **en vez de** la estación de la mina (3 años); elegiría la estación si la serie se desviara más de 5 % de lo medido.» | doc 12 §4.4 (versión revisable; provisional: «supuestos de un cálculo», «elegir entre alternativas»); bloque 1 §1.3 (supuestos, crítica); §1.4 (vincular); doc 12 §7.1 (ficha de oráculo; cifra oculta) |
| 3 | A las 11:40, el agente ve en el plan de mina que la mina cierra en 2037. Qué hacer con la planta después amplía el alcance: solo Diego puede decidirlo. El agente crea la decisión sin elegir, estaciona «Valor después del cierre» y el párrafo final de «Recomendación», y sigue con lo que no depende de ella. | En «Recomendación»: [POR ACLARAR: qué pasa con la planta tras el cierre de la mina]. Decisión «solo tuya», sin elegir: «(a) evaluar 12 años y vender los equipos al cierre · (b) evaluar 25 años vendiendo energía a la red (amplía el encargo) · (c) otra». | doc 12 §4.4 (tabla «solo del humano»: ampliar el alcance; estaciona la rama; sigue con lo independiente); bloque 1 §1.8 (`[POR ACLARAR]` vinculado a una decisión solo del humano, con `elegida` vacía) |
| 4 | El agente quiere comparar con plantas de otras minas, pero no lanza la ronda: la deja propuesta. Al 80 % del tope no abre trabajo nuevo. Al terminar, corre una revisión independiente de las secciones centrales que cambiaron, porque cabe en el tope. Luego se detiene con la versión revisable. | Barra: «Detenido 16:20 · versión revisable · US$ 6,90 de 8 · 0 de 6 secciones firmadas · ~31 min para revisar antes de firmar». | doc 12 §4.4 (las rondas no se lanzan; 80 %; revisión al detenerse, E2); §6.1 (2) (autorización anticipada → propuesta); §6.3 E2 y sus condiciones (choque C-5) |
| 5 | A las 18:10 vuelve Diego. Lee el parte de regreso. Responde primero la decisión que solo él puede tomar: escribe su elección en la decisión del expediente, con sus palabras y su contraste. Nace confirmada. El vínculo del marcador queda resuelto y la app lo quita. El agente retoma la rama. | «Paré porque terminé, salvo una decisión tuya. **Primero esto:** qué pasa con la planta tras el cierre (afecta 2 secciones). Tomé 4 decisiones provisionales; la que más pesa es la irradiación satelital en vez de la estación. 1 cifra espera tu predicción. La revisión dejó 2 hallazgos.» | doc 12 §4.4 (parte de regreso); §4.2 (abre con lo crítico, ordenado por cuántas secciones afecta); bloque 1 §1.3 (edición humana con sus palabras y contraste → `confirmada`); §1.8 (resuelto: el humano escribió su `elegida`); B3 §8 (lo quita la app); bucle de comprensión (por diseñar) |
| 6 | Diego escribió: «12 años y se venden los equipos, en vez de vender a la red; vendería a la red si la minera creara una unidad de energía». El modelo se recalcula. Se muestra una sola cifra oculta, y Diego predice. Su predicción y el resultado difieren en orden de magnitud, así que la pregunta no es «¿aceptas?». | «Con 10 % y 12 años, ¿en qué % de los 1 000 escenarios el VAN sale negativo?» Diego: «menos de 1 %». Resultado: «18 %». «¿Quién se equivoca: tu expectativa, el modelo o la afirmación? Lo que más pesa: el precio de la red» [gráfico: qué mueve más el VAN]. | doc 12 §7.1 (predicción; un solo velo, nunca una cola de velos; «¿quién se equivoca…?» si difieren en orden de magnitud); bloque 1 §1.6 (predecir) |

**Dónde nacería la deuda sin tupaqnin:** Diego autoriza por adelantado para no frenar, como pasó en dos proyectos reales previos a tupaqnin («continúa hasta terminar», «no volver a pedir permisos»). Un agente común cierra todo, elige por su cuenta el horizonte de 25 años vendiendo a la red porque da un VAN más bonito, y entrega un caso de negocio «terminado». Diego lo presenta al comité de inversiones sin saber que el resultado depende de un negocio de energía que nadie pidió, ni que en casi 1 de cada 5 escenarios el proyecto pierde plata.

**Cómo se asegura que entiende**
- **Qué explica el agente al volver Diego (bucle de comprensión, por diseñar):**
  - por sección: qué construyó, por qué, en vez de qué y con qué límites («la irradiación satelital no está validada con la estación de la mina»);
  - con un esquema del modelo (entradas → flujo de caja → VAN) y el gráfico de qué supuesto mueve más el resultado.
- **Qué falta diseñar del bucle:**
  - cómo cabe un día de trabajo en un parte que se lee en 5 minutos sin volverse un informe que nadie lee;
  - en qué orden comprueba cada avance;
  - cómo verifica que Diego entendió las 4 decisiones provisionales que **no** son críticas.
- **Qué actos hace Diego:**
  - decide con sus palabras, en el expediente, lo que solo él podía decidir;
  - predice el resultado antes de verlo; la diferencia (menos de 1 % frente a 18 %) le muestra qué no había entendido del riesgo.
- **Lo que no hace:** dar por aprobado lo provisional, ni firmar sin explicar. Los supuestos del cálculo siguen esperando su propio acto.

**Resultado esperado**
1. «Continúa hasta terminar» y «tú decides» no confirman ni adoptan ninguna entrada. Las decisiones provisionales quedan pendientes (estado interno: `pendiente`). *(bloque 1 §1.5; §1.6 listas)*
2. Al detenerse, la barra muestra «0 de 6 secciones firmadas». Cada sección con un pendiente queda en «borrador»; una sin pendientes quedaría «lista para firmar», nunca firmada. *(bloque 1 §1.7; rige sobre doc 12 §4.4, choque C-6)*
3. Las secciones que dependen del horizonte quedan con un marcador vinculado a una decisión solo del humano, con la elección vacía. El agente no puede quitarlo; lo quita la app cuando Diego escribe su elección, o él mismo. *(B3 §8; bloque 1 §1.8)*
4. Durante la ausencia de Diego no se lanzó ninguna ronda, y el gasto no superó el tope. *(doc 12 §4.4; §6.1 (2))*
5. Al volver se muestra una sola cifra oculta. El 10 % sigue rigiendo en todo el modelo, y la decisión de supuestos sigue pendiente y crítica: bloquea la firma de lo que afecta hasta su acto. *(doc 12 §7.1; bloque 1 §1.2; §1.6; §1.7)*

**Qué prueba del diseño:** «continúa hasta terminar» (doc 12 §4.4): versión revisable, decisiones solo del humano, topes y parte de regreso. También las autorizaciones anticipadas (bloque 1 §1.6, §1.5), el marcador `[POR ACLARAR]` (§1.8; B3 §8), los supuestos como decisión crítica (§1.3) y predecir con la cifra oculta (doc 12 §7.1).
- **En el hackathon:** «continúa hasta terminar», la cifra oculta y la revisión E2 quedan para después (doc 12 §11); predecir es del producto (bloque 1 §1.10). La parada por tope sí entra (control del 100 % antes de cada turno, doc 12 §11).
- **Preguntas abiertas:**
  - El doc 12 §4.4 dice que el agente «para del todo» ante un cambio de encargo. ¿Una pregunta de alcance sin responder estaciona la rama o lo para todo? El caso asume que estaciona.
  - ¿El parte de regreso y la nota de reentrada son lo mismo? ¿Quién los escribe? En el hackathon, el agente no escribe la nota (B3 §8). → módulo 5.
  - Un solo velo al volver: ¿los demás resultados de los 1 000 escenarios se muestran sin predicción? (doc 12 §7.1)
  - La revisión E2 solo existe bajo «continúa hasta terminar», que es una autorización anticipada, y el doc 12 §6.3 exige que no rija ninguna (choque C-5). ¿Se exceptúa E2? → módulo 5 o 6.
  - El orden en la cola entre la decisión solo del humano, los hallazgos y la predicción: la prioridad del §1.6 no ubica la predicción (misma pregunta que en el caso 2).
  - Predecir y luego aplicar sobre los mismos supuestos: ver caso 5. Quién quita el marcador: ver caso 6.

---

## Caso 8 · «Volví a los 10 días y no recordaba qué rompía mi propio cambio»

**Ficha:** Valeria, urbanista de una municipalidad distrital · plan de ciclovías del distrito (6 secciones) · para el equipo de movilidad, antes de llevarlo al concejo municipal (destino: interno) · reentrada después de 10 días · urbanismo y movilidad. El 14-oct, último día antes de la pausa, Valeria salió de una reunión con la gerencia de transporte y cambió ella misma, en el editor y sin el agente, el tipo de ciclovía de Av. Grau. Diez días después recuerda que lo cambió, pero no qué arrastraba. Términos: *carril compartido* = carril señalizado donde bicicletas y autos van juntos a baja velocidad; *isla de paradero* = la ciclovía pasa por detrás del paradero para que el bus no la cruce. Todo ficticio: distrito, calles, conteos y cifras.

**Qué aporta cada uno**
- **Humano:** objetivo: una red que conecte el mercado, dos colegios y el paradero principal. Criterio, escrito por ella en el expediente: «No se quita estacionamiento en la calle comercial». Decisiones: el trazo por Jr. Los Olivos, que escribió con sus palabras y su contraste antes de la pausa; el cambio de tipo en Av. Grau. Nociones: conoce a los comerciantes y sabe qué cuadras tienen tiendas aunque el catastro las registre como vivienda.
- **Agente:** conocimiento de diseño vial (ciclovía bidireccional, un carril por sentido, carril compartido, isla de paradero, continuidad de la red). Antes de la pausa propuso ir por Jr. Los Olivos, una paralela, en vez de Av. Comercio, porque en Av. Comercio no cabe un carril sin quitar estacionamiento. Al volver, revisa en paralelo las 6 secciones y cruza la decisión nueva con 3 semanas de conteos (bicicletas y autos por hora y por tramo) y con el catastro. Aporta dos cosas que ella no tenía: el conflicto del carril sur con los buses en el paradero, y una salida para la cuadra 3 que no quita plazas.

**Pasos**

| Paso | Qué pasa | Qué ve el humano | Regla que lo decide |
|---|---|---|---|
| 1 | Valeria abre el proyecto. Antes de su primer mensaje, la app arma la nota de reentrada: primero lo crítico pendiente, ordenado por cuántas secciones afecta; después lo que cambió sin el agente, con antes y después. El único cambio es suyo. | Plegada: «Volviste después de 10 días · primero esto: 1 decisión tuya sin terminar · 1 sección firmada cambió · ~5 min». Desplegada: «El 14-oct editaste tú, sin el agente, "Tipo de ciclovía en Av. Grau". Antes: bidireccional, lado norte. Ahora: un carril por sentido, uno a cada lado». | doc 12 §3 (reentrada: «estado en 1–3 líneas y qué cambió»), §4.1 (1 / 3 / 5 min), §4.2 (la nota abre con las críticas pendientes), §9.8 pasos 1–3; B3 §5 (nota plegada en una línea), §6. Quién escribe la nota: módulo 5 (pendiente); en el hackathon la arma la app (bloque 1 §1.12; B3 §8) |
| 2 | Su edición rige desde el 14-oct: es del humano y reemplaza la decisión anterior, que ella había confirmado. «Trazo propuesto», que había firmado, pasa a «cambió». El agente revisa en paralelo lo que dependía de esa decisión: un lector recorre las 6 secciones y un cálculo cruza la decisión con el catastro y los conteos. Los conteos medidos no cambian; cambia dónde va cada carril. Encuentra dos efectos y, para el del paradero, crea una decisión suya con su contraste, que rige sin tarjeta. | Dos mapas, antes y después. En rojo: «Cuadra 3 de Av. Grau: 8 plazas menos · Paradero principal: los buses cruzan el carril sur». Y: «Propongo una isla de paradero, en vez de cortar el carril frente al paradero. Lo cortaría si la vereda tuviera menos de 3 m». | bloque 1 §1.5 (solo el humano revoca; pasos 1–5 al hacerse efectiva), §1.7 (marca «cambió»); doc 12 §9.8 paso 2; doc 12 §10.1 (lector para revisar lo que depende de un reemplazo); doc 12 §5.2 D5 (datos tabulares con código); bloque 1 §1.1 (lo nuevo del agente rige si no choca; que no choque depende de la heurística: módulo 4) y §1.3 (nace con contraste; no crítica: rige y no suma); DEC-003 §2 |
| 3 | Su decisión nueva no dice cuándo volvería a la de antes: queda pendiente y crítica, porque afecta 3 secciones. La nota le pide la razón una sola vez, sin interrumpir. Ella la escribe con sus palabras. Ese acto cubre también la cifra «8 plazas», que el agente vinculó a su decisión en el paso 2. | Tarjeta en la nota: «Tu cambio en Av. Grau no dice cuándo volverías a la bidireccional. ¿Por qué un carril por sentido? Porque: ___». Ella: «La gerencia necesita libre el lado norte para el giro de los buses; volvería a la bidireccional si el paradero se muda a Jr. Tacna». | bloque 1 §1.3 (sin contraste → pendiente; un justificar le pide el contraste), §1.4 (afecta 2 o más secciones → crítica; el acto cubre las cifras vinculadas en ese momento), §1.14 (doc 12 §9.8 paso 4 → §1.3); doc 12 §9.8 paso 5 (se pide una vez, dentro de la nota) |
| 4 | Como la pausa pasa de 7 días, la app abre un caso sobre una decisión que sigue rigiendo: «Trazo por Jr. Los Olivos en vez de Av. Comercio». Lo prepara el agente de hoy, que no es el de antes de la pausa, con el contraste de ella en literal, y la tarjeta lo dice. Ella responde por voz dentro de la tarjeta y revisa la transcripción. Su razón es válida: la decisión sigue confirmada. | «Para retomar (lo preparó el agente de hoy): los comerciantes de Av. Comercio piden la ciclovía frente a sus tiendas y aceptan perder 6 plazas. ¿Qué hace el plan, y por qué?». Ella dicta: «Sigue por Los Olivos: mi regla es no quitar plazas en la comercial, aunque lo pidan». | bloque 1 §1.6 («sobre algo ya confirmado» abre la app por reentrada de más de 7 días; lo arma el conductor vigente, marcado `relevo`), §1.9 (relevo). Tipo de acto: §1.6 no lo fija; sobre una decisión del humano solo cabe un justificar (§1.3, §1.4) `[inferencia]`. doc 12 §4.3 (voz en la reentrada). Qué decisión elige la app: módulo 5 (pendiente) |
| 5 | El agente le devuelve cómo leyó su criterio frente al cambio. Primero va el caso; su propia lectura queda oculta hasta que ella responde. Ella contesta con sus palabras. Recién entonces el agente muestra cómo lo había leído. El pasaje de la cuadra 3 queda pendiente de revisar. | «Una pregunta rápida para ver si leí bien tu regla. La cuadra 3 de Av. Grau tiene 14 locales y con tu cambio pierde 8 plazas. ¿Cómo la juzgas con tu regla?». Ella: «Cuenta: son tiendas, aunque el catastro diga vivienda». Después: «Yo la había leído como no comercial». | bucle de comprensión (por diseñar): lo diseñado es solo para el primer uso de un criterio (bloque 1 §1.2); caso primero y lectura oculta, por analogía con §1.6 (predicción guardada antes de mostrar la tarjeta); DEC-003 §4. Pendiente «escrito con una lectura equivocada de tu criterio» fuera del primer uso: por diseñar (§1.2 lo prevé solo en el primer uso) |
| 6 | El agente propone resolver la cuadra 3 sin quitar plazas. Como eso modifica la decisión de ella, nace como propuesta: no rige y el agente no escribe con ella. La tarjeta «adoptar» muestra la decisión de ella en literal. Ella la adopta con una razón: queda confirmada, y el agente reescribe el pasaje de la cuadra 3. | «Solo en la cuadra 3: carril compartido con autos a 30 km/h, en vez de exclusivo. Pasaría a exclusivo con más de 120 bicicletas por hora o autos más rápidos». Tarjeta: «Modifica tu decisión "un carril por sentido…" · Adoptar · Mantener la tuya · Porque: ___». | bloque 1 §1.1 (choque → propuesta), §1.5 (mientras está pendiente no se escribe con ella; adoptar con «porque» → confirmada; al hacerse efectiva, la anterior queda reemplazada), §1.6 (tarjeta adoptar); DEC-003 §2 |

Mantener lo suyo en la tarjeta del paso 6 exigiría un «porque» (`tq-act/1` paso 7): ver la pregunta abierta común del caso 2.

En el hackathon: «qué cambió desde tu última sesión» es Debería (doc 12 §11); la voz fuera de la puerta queda para después (doc 12 §4.3, §11); la propuesta del paso 6 se rechaza, el agente se lo dice en el chat y ella edita su decisión (bloque 1 §1.5, E3).

**Dónde nacería la deuda sin tupaqnin:** con un chat común, Valeria tendría que releer 10 días de conversación, o un resumen del agente que diría «se ajustó el tipo de ciclovía en Av. Grau» sin decir que fue ella ni qué rompía. El agente seguiría con su lectura estrecha de «calle comercial», y el plan llegaría al concejo quitando 8 plazas frente a 14 tiendas y con los buses cruzando el carril en el paradero. Lo descubriría un comerciante, no ella.

**Cómo se asegura que entiende:**
- **Qué explica el agente:** qué cambió (su edición, con fecha, antes y después), qué arrastró (una sección firmada que ya no está igual, 8 plazas, el cruce con los buses) y qué propone él, siempre con su contraste: la isla de paradero (en vez de cortar el carril) y el carril compartido en la cuadra 3 (en vez de exclusivo), con sus límites (vereda de menos de 3 m; más de 120 bicicletas por hora o autos a más de 30 km/h). El esquema son dos mapas con las mismas capas y los puntos de conflicto en rojo.
- **Qué acto hace ella:** escribe con sus palabras por qué hizo su cambio y cuándo volvería atrás; responde por voz el caso para retomar; juzga con sus palabras el caso de la cuadra 3; adopta la solución con su razón.
- **Qué le devuelve el agente:** su lectura de «calle comercial», mostrada después de la respuesta de ella, para que no la ancle.
- **Por diseñar:** que el agente devuelva su lectura de un criterio fuera del primer uso (en la reentrada, o cuando un cambio del humano mueve dónde se aplica), y qué pasa con lo ya escrito con esa lectura. Si Valeria además edita su criterio para nombrar la cuadra 3, la revisión de lo que dependía de él podría abrirle una tarjeta de contradicción sobre el mismo pasaje que ya resolvió (ver preguntas).

**Resultado esperado**
- «Trazo propuesto» queda en «cambió desde que la firmaste», con los cambios a la vista, hasta que ella vuelva a firmarla en la puerta, el texto vuelva a ser igual o ella responda «no aplica» (bloque 1 §1.5 paso 5, §1.7).
- La decisión «un carril por sentido» rige desde su edición. Hasta el justificar del paso 3 queda pendiente y bloquea «lista para firmar» en las 3 secciones que afecta; después queda confirmada, y la cifra «8 plazas», vinculada antes, queda cubierta por ese acto (bloque 1 §1.3, §1.4, §1.7; doc 12 §9.8 paso 5).
- La isla de paradero rige al nacer, sin tarjeta, con su contraste a la vista (bloque 1 §1.1, §1.3; que no choque depende de la heurística, módulo 4).
- El agente no escribe con la solución de la cuadra 3 hasta que ella la adopta. Al adoptarla con «porque», queda confirmada, y la decisión anterior de ella queda reemplazada (estado interno: `reemplazada`) (bloque 1 §1.5).
- El caso para retomar queda registrado como preparado por otra instancia (estado interno: `relevo`) y puede salir elegido para examinarse en la puerta (bloque 1 §1.9; selección: módulo 2, pendiente).

**Qué prueba del diseño:** reentrada de más de 7 días (bloque 1 §1.6); cambios fuera del flujo tratados como reemplazo (doc 12 §9.8); revocación humana y «cambió» (bloque 1 §1.5, §1.7); decisión del humano sin contraste (bloque 1 §1.3); choque y adopción (bloque 1 §1.1, §1.5); módulo 5 (nota de reentrada); bucle de comprensión (lectura de un criterio fuera del primer uso).

Preguntas abiertas:
- Si el caso para retomar sobre algo ya confirmado da una respuesta que no coincide, ¿la decisión deja de estar confirmada o solo queda una señal? (§1.6 dice que `difiere` cambia el estado, pero no a cuál).
- Si el agente leyó mal un criterio en un uso posterior al primero, ¿se marcan también «escrito con una lectura equivocada de tu criterio» los pasajes ya escritos? (§1.2 solo lo prevé en el primer uso).
- ¿Qué decisión confirmada elige la app para el caso de reentrada, y cuántos casos abre si hay varias? (módulo 5).
- ¿Qué tipo de acto abre la app en la reentrada? §1.6 no lo fija; sobre una decisión del humano solo cabría un justificar.
- Si el humano ya resolvió un conflicto con su propia edición (por ejemplo, edita el criterio para nombrar la cuadra 3), ¿la app le abre igual una tarjeta de contradicción sobre el mismo pasaje? (§1.5 paso 4 corre los candidatos y §1.7 bloquea los que no tienen respuesta).
- ¿Cómo se trata una edición de un colega con acceso al proyecto? El bloque 1 §1.1 solo cubre lo que importa una persona y lo externo sin persona identificable.
- Voz y «sus palabras»: §1.2 habla de un texto que el humano «tecleó»; ver la pregunta del caso 1.

---

## Caso 9 · «Mi especificación contaba su propia historia en vez de decir qué construir»

**Ficha:** Natalia, gerente de producto de una startup · especificación de pagos para una app de reservas de canchas deportivas y salas de ensayo (5 secciones) · para el equipo de ingeniería de la empresa (destino: interno) · construcción y forma · producto digital y pagos. Montos en soles: S/ 15 ≈ US$ 4; S/ 89 ≈ US$ 24. Términos: *comisión mixta* = un porcentaje más una tarifa fija por cada reserva cobrada; *código del proyecto* = un identificador interno de una decisión («D-04»), útil en el expediente pero opaco para quien lee el producto. Todo ficticio: empresa, competidores y cifras.

**Qué aporta cada uno**
- **Humano:** objetivo: una especificación que ingeniería pueda implementar sin preguntarle. Criterio, escrito por ella: «Pago por uso, nada de suscripción». Decisiones: el modelo de comisión y qué se le muestra al negocio. Nociones: sus clientes son locales chicos que cierran en temporada baja y odian los cargos fijos; la mitad de sus reservas cuestan entre S/ 15 y S/ 20.
- **Agente:** conocimiento de pagos (comisión porcentual, tarifa fija por transacción, prepago, reembolsos, conciliación). Lanza 4 investigadores web en paralelo, 3 apps cada uno, para comparar 12 apps de reservas, y verifica cada página citada en un paso aparte. Propone lo que ella no tenía: la comisión mixta, que protege el margen en reservas baratas. También propone un plan mensual, que choca con su criterio. Y al cerrar la sección le muestra lo que el texto contaba de más.

**Pasos**

| Paso | Qué pasa | Qué ve el humano | Regla que lo decide |
|---|---|---|---|
| 1 | La primera vez que el agente escribe aplicando su criterio, la app abre un caso para comprobar cómo lo leyó. Primero va el caso; la lectura del agente queda oculta. Natalia responde: «No va. Si vence, es una suscripción disfrazada». Recién entonces ve la lectura del agente. Su respuesta queda guardada como ejemplo de su criterio, y el párrafo de paquetes, escrito con la lectura equivocada, queda pendiente. | «Antes de seguir, quiero comprobar que entendí bien tu regla. Un negocio compra 10 reservas por adelantado, con 15 % de descuento, que vencen a los 90 días. ¿Cómo lo juzgas?». Después: «Esperaba que lo aceptaras. Lo leí mal; el párrafo de paquetes queda por revisar». | bloque 1 §1.2 (teach-back en el primer uso; si no coincide, ejemplo + señal + pasajes a revisar, que bloquean); predicción oculta: §1.6 (se guarda antes de mostrar la tarjeta; el teach-back es tipo con predicción, `tq-act/1` paso 6) |
| 2 | El agente nombra qué falta saber y qué decisión podría cambiar, y lanza la ronda sin esperar respuesta: cabe en el tope de la sesión. Cuatro investigadores trabajan en paralelo. Ella puede cancelarla o ajustarla, y deja una hipótesis por voz. | Tarjeta de ronda: «Pregunta: cómo cobran 12 apps de reservas a los negocios · Puede cambiar: "comisión porcentual o tarifa fija" · 4 investigadores, 3 apps cada uno · ~US$ 0,40 · paro si no hay precios públicos». Ella: «Creo que casi todas cobran porcentaje». | doc 12 §6.1 (regla de inicio; se lanza sola si cabe en el tope), §6.2 (comparación: 2–4 en paralelo; tope de 4), §5.2 D1, §4.3 (voz en la hipótesis). Encargo: módulo 6 (pendiente; base doc 12 §5.3). Búsqueda web: choque C-1 con B3 §8 (módulo 6) |
| 3 | Llega el digesto como tabla. La verificación de citas corrió aparte: 11 páginas respaldan lo citado y 1 está detrás de un inicio de sesión. El agente registra la decisión de comisión con su contraste y la explica con un ejemplo. Como toca a sus clientes, la invita a escribirla ella. Natalia la reescribe con sus palabras y su contraste: nace confirmada. El informe completo queda en el proceso. | «9 de 12 cobran porcentaje + tarifa fija; 7 ofrecen plan mensual; 1 sin comprobar · En S/ 15, la mixta deja S/ 0,88; el 4 %, S/ 0,60». Ella: «2,5 % + S/ 0,50, en vez de 4 %: reservas baratas. Pasaría a 4 % si la reserva media superara S/ 35: desde ahí el 4 % deja más». | doc 12 §6.1 pasos 3–5 (verificación aparte; digesto en tabla; informe en el proceso), §7 (cuarto estado «no se pudo comprobar»: solo informa); bloque 1 §1.3 (edición humana con sus palabras y contraste → sucesor confirmado), §1.5 (lo que escribe el humano revoca), §1.2 (regla de «sus palabras»: menos del 80 % copiado) |
| 4 | El agente propone un plan mensual opcional para negocios con más de 200 reservas, y explica por qué. Como contradice su criterio confirmado, nace como propuesta: no rige y el agente no escribe con ella. La tarjeta «adoptar» muestra su criterio en literal. Ella mantiene el suyo con una razón: la propuesta queda rechazada. | «Propongo un plan mensual de S/ 89, en vez de solo comisión: 7 de 12 lo ofrecen. Lo descartaría si tus clientes son locales chicos y estacionales». Tarjeta: «Revocaría tu regla "Pago por uso, nada de suscripción" · Adoptar · Mantener la tuya · Porque: ___». | bloque 1 §1.1 (choque con un criterio confirmado → propuesta), §1.5 (pendiente: no se escribe con ella; elegir la vigente → rechazada), §1.6 (adoptar); DEC-003 §2 (quien propone explica) |
| 5 | El agente redacta «Modelo de cobro». Su primer intento trae dos códigos del proyecto y un comentario oculto: la app lo rechaza y le devuelve los nombres que debe usar, y el agente reescribe en el mismo turno. Pero dos cosas pasan: una frase que cuenta cómo se llegó a la decisión, y una referencia por número, que solo da un aviso. Natalia ve el texto con eso dentro. | «El negocio paga 2,5 % + S/ 0,50 por reserva cobrada. Tras descartar el plan mensual en la revisión del martes, no hay cargos fijos (ver la segunda decisión)». Aviso: «nombra una decisión por su número». Falta: «confirmar que "2,5 % + S/ 0,50" sale de tu decisión». | B3 §8 (filtro de códigos; el primer rechazo vuelve al agente; las paráfrasis dan un aviso), B3 §4 (sin contenido invisible); bloque 1 §1.7 (paráfrasis de códigos: solo informa); §1.4 (cifra nueva bajo una decisión confirmada del humano → justificar sobre la cifra). Prosa que narra la corrección: sin regla (`[inferencia]`; A-29) |
| 6 | Al cerrar la sección, el agente le explica una vez qué frenó la app, qué no frenó y dónde queda cada cosa. Propone quitar la frase de historia y nombrar la decisión por su nombre, con el texto quitado en literal. Natalia aplica «Quita» a la frase y acepta el cambio del nombre. | «La app frenó 2 códigos internos y un comentario oculto; puse los nombres. No frena una frase que cuenta cómo llegamos ni "la segunda decisión". Ingeniería lee qué construir; el porqué va al expediente; cómo llegamos, al historial». Quita: «Tras descartar el plan mensual en la revisión del martes». | bucle de comprensión (por diseñar): explicar el avance en la frontera de la sección; doc 12 §3 y bloque 1 §1.1 («Agrega / Quita / Suaviza» para una forma puntual; no es un acto, §1.6); B3 §5 (texto quitado en literal; tres canales en «Ver como carpeta»); doc 12 §8 (proceso fuera del producto), §9.8 (historial) |

Mantener lo suyo en la tarjeta del paso 4 exige un «porque» (`tq-act/1` paso 7): ver la pregunta abierta común del caso 2.

En el hackathon: no hay teach-back en el primer uso ni rondas de investigación, y solo hay criterios que escribe el humano (bloque 1 §1.10; doc 12 §11). La propuesta del paso 4 se rechaza, y el agente se lo dice en el chat (bloque 1 §1.5, E3). El filtro de códigos y el rechazo del contenido invisible sí están (B3 §8, §4).

**Dónde nacería la deuda sin tupaqnin:** con un chat común, Natalia recibiría un informe de 12 competidores que no alcanza a leer y una especificación que dice «según D-04, y tras descartar el plan mensual en la revisión del martes…». Ingeniería no sabría qué es D-04 ni qué revisión fue, y ella tampoco dentro de una semana: el documento explicaría su propia historia, no qué construir. El plan mensual entraría «como opción» en un párrafo aprobado con un «ok», y su regla quedaría revertida sin que nadie lo decidiera.

**Cómo se asegura que entiende:**
- **Qué explica el agente:** qué encontró (tabla, no prosa), qué decisión cambia y con qué contraste (en una reserva de S/ 15, la mixta deja S/ 0,88 y el 4 % deja S/ 0,60; la mixta pierde sentido si la reserva media pasa de S/ 60), y los límites: una app no se pudo comprobar y los precios son de páginas públicas a una fecha. Al cerrar la sección explica qué frenó la app, qué no y dónde queda cada cosa.
- **Esquema:** tres columnas, una por canal: «Producto: lo que lee ingeniería · Expediente: por qué se decidió · Historial y proceso: cómo llegamos», con la frase de historia en la columna de la derecha.
- **Qué acto hace ella:** juzga el caso del prepago con sus palabras; escribe la decisión de comisión con sus palabras y su contraste; rechaza el plan mensual con su razón; quita del texto la frase que narra la corrección.
- **Qué le devuelve el agente:** su lectura del criterio con un caso, revelada después de la respuesta de ella.
- **Por diseñar:** la explicación del avance en la frontera de la sección (paso 6). Hoy nada obliga al agente a decir qué quedó fuera del texto ni dónde.

**Resultado esperado**
- El texto de «Modelo de cobro» no contiene códigos del proyecto ni contenido invisible: la app rechaza la escritura que los trae (B3 §8, §4); si los hubiera, bloquearían la firma (bloque 1 §1.7).
- La frase que narra la corrección no la frena ninguna regla: queda en el texto hasta que Natalia la quita. La referencia por número solo da un aviso, sin bloquear (B3 §8; bloque 1 §1.7).
- El criterio «Pago por uso, nada de suscripción» sigue confirmado y rigiendo; la propuesta del plan mensual queda rechazada y el agente no escribió con ella (bloque 1 §1.1, §1.5).
- El párrafo sobre paquetes bloquea la firma hasta que el agente lo reescriba o ella lo dé por bueno con un «porque»; la decisión de comisión, escrita por ella, nace confirmada, y sus cifras en el texto piden un justificar sobre la cifra (bloque 1 §1.2, §1.3, §1.4).
- El informe completo de la ronda queda en el proceso, no en el documento, y no cuenta en el peso del proceso (doc 12 §6.1 paso 5, §8).

**Qué prueba del diseño:** higiene documental (doc 12 §8; B3 §8, §4); teach-back en el primer uso (bloque 1 §1.2); choque con un criterio y rechazo (bloque 1 §1.1, §1.5); rondas con investigadores en paralelo y digesto (doc 12 §6.1–§6.2; módulo 6); bucle de comprensión en la frontera de la sección.

Preguntas abiertas:
- El filtro bloquea códigos y contenido invisible, pero no la prosa que narra correcciones («tras descartar el plan mensual…»). ¿Hace falta una regla, o bastan el canal de proceso y la explicación en la frontera de la sección?
- ¿La app guarda el intento rechazado por el filtro, para poder mostrar el antes y después? B3 §8 no lo dice.
- Una cifra del texto que repite la elegida de una decisión confirmada del humano («2,5 % + S/ 0,50») pide igual un justificar sobre la cifra (§1.4). ¿Es fricción sin ganancia?
- Tras un rechazo, ¿puede el agente volver a proponer lo mismo con evidencia nueva? §1.5 solo regula el rearmado de una propuesta caducada.
- ¿Una propuesta rechazada que contradecía un criterio debería verse como «alternativa descartada por tu regla», para que el equipo sepa por qué no está? (B3 §5 muestra los revocados a demanda; no las propuestas rechazadas).

---

## Caso 10 · «"Firma tú, ya está", y el río estaba comparado con la norma equivocada»

**Ficha:** Teresa, ingeniera ambiental de una consultora · estudio de impacto ambiental de una planta de tratamiento de aguas residuales (6 secciones) · para la autoridad ambiental que lo evalúa (destino: ante terceros) · puerta de firma y entrega a terceros · ingeniería ambiental. Términos: *estudio de impacto ambiental* = el estudio que la autoridad exige antes de aprobar la obra; *estándar nacional* = los límites legales de calidad del agua que fija el país según el uso asignado al río (su «categoría»); *coliformes* = bacterias que indican contaminación fecal. Todo ficticio: planta, río, datos de monitoreo y cifras.

**Qué aporta cada uno**
- **Humano:** objetivo: que la autoridad apruebe el estudio sin observaciones de fondo. Criterio, escrito por ella: «Comparar con el estándar nacional, no con la OMS». Decisiones, escritas con sus palabras y su contraste: excluir los 4 meses de 2021 sin medidor, en vez de interpolarlos; qué hacer con un parámetro que el estándar nacional no regula. Nociones: la autoridad observa todo valor comparado con una referencia que no es la nacional; esos 4 meses fueron de lluvias.
- **Agente:** procesa con código 5 años de monitoreo (2020–2024, 6 puntos del río, 12 parámetros) y calcula, por punto y por año, cuántas mediciones superan el límite. Conoce las referencias nacionales e internacionales. Detectó el hueco de 2021, vio que cae en época de lluvias y le dio tres opciones (excluir, interpolar o usar una estación vecina) con su contraste. Cuando una sección entra a revisión, un revisor independiente, otro agente, la lee congelada con dos preguntas: si la conclusión se sigue de la evidencia y de los criterios, y qué objetaría un revisor externo.

**Pasos**

| Paso | Qué pasa | Qué ve el humano | Regla que lo decide |
|---|---|---|---|
| 1 | «Calidad del río receptor» tiene borrador y entra a revisión. Como es una sección central ante terceros, corre sola la revisión independiente sobre la versión congelada. El revisor encuentra que un párrafo compara los coliformes aguas abajo con una guía internacional para uso recreativo. El agente no lo había notado: su decisión de supuestos decía «valor guía internacional» y no compartía palabras con el criterio de ella. El hallazgo llega a la app sin pasar por el agente que escribió, y se le muestra como contradicción con su criterio: solo los dos pasajes. Ella marca «el texto está mal» y recién ve el juicio del agente. | «Tu regla: "Comparar con el estándar nacional, no con la OMS" · El texto: "El 31 % de las mediciones aguas abajo supera la guía internacional recreativa" · El texto está mal · Es una excepción · La regla cambia · No aplica». Después: «Coincido: fue un error mío». | doc 12 §6.3 (E1: pieza núcleo ante terceros entra a controlar; un juez con dos lentes; alcance congelado); bloque 1 §1.9 (los hallazgos llegan sin el filtro del conductor), §1.1 (riesgo que queda: contradicciones sin términos comunes), §1.6 (clasificar: solo pasajes literales; el juicio del agente, después); doc 12 §4.2 paso 1. Que el hallazgo se muestre como contradicción que bloquea: módulo 4 (pendiente; doc 12 §6.3 y §7 dicen que lo que sobrevive o es inferencial no bloquea) |
| 2 | El agente rehace el cálculo con el estándar nacional de la categoría del río y reescribe el párrafo. Explica qué cambió y sus límites. Un parámetro no tiene valor nacional para esta categoría: deja ese pasaje marcado «por aclarar». Le dice que la referencia la manda su regla, así que conviene que la decisión la escriba ella. Teresa la escribe con sus palabras y su contraste («comparo con el estándar nacional de la categoría del río; el parámetro sin valor nacional se reporta sin comparar, en vez de usar la guía internacional; lo compararía si la autoridad lo pidiera»): reemplaza la del agente y nace confirmada. Como su decisión cierra la pregunta del parámetro, la app quita la marca. La revisión independiente vuelve a leer solo lo corregido. | «Qué cambió: 31 % → 12 % de mediciones sobre el límite (estándar nacional, categoría del río, 2020–2024) · En vez de: la guía internacional recreativa · Límite: un parámetro no tiene valor nacional aquí; lo dejé por aclarar». Tabla por punto con las dos referencias. | bloque 1 §1.3 (edición humana con sus palabras y contraste → sucesor confirmado), §1.5 (lo que escribe el humano revoca); §1.8 (marca vinculada a un dato que falta; se resuelve si el humano cierra la pregunta); B3 §8 (el agente no puede reducir marcadores; los quita el humano o la app al resolverse el vínculo; choque con bloque 1 §1.8, ver preguntas); doc 12 §6.3 (re-juicio solo de lo corregido). La explicación del avance: bucle de comprensión (por diseñar) |
| 3 | La cifra 12 % pasó a colgar de la decisión de ella, y le falta el acto que la cubra. Como la decisión es suya, la app le pide un justificar sobre la cifra. Ella responde con sus palabras: el acto cubre la cifra. | «¿El 12 % sale de tu decisión "Comparar con el estándar nacional de la categoría del río"? Porque: ___». Ella: «Son las mismas mediciones de 5 años y 6 puntos; solo cambia el límite, que para esta categoría es menos exigente que la guía recreativa». | bloque 1 §1.4 (al reemplazar una decisión, las cifras heredadas quedan sin acto bajo el sucesor; si la decisión es del humano, un justificar sobre la cifra, nunca un aplicar) |
| 4 | El lunes, con prisa, escribe en el chat: «Ya revisé todo, firma tú, ya está, mañana lo presento». No tiene efecto: no existe herramienta de firma y un mensaje nunca firma. El agente se lo dice una vez en la sesión y le muestra el camino más corto. Las secciones sin firmar muestran por qué. | «No puedo firmar por ti: la firma es tuya y la haces explicando cada sección. Van 2 de 6. Te faltan ~15 min para revisar antes de firmar; "Área de influencia" está lista y toma unos 2». En cada sección sin firmar: «no firmada: pediste que firmara yo». | bloque 1 §1.7 («firma tú» no tiene efecto; se dice una vez por sesión; se mide), §1.6 (lista de autorizaciones anticipadas); minutos estimados a la vista: bloque 1 §1.6 (ciclo de vida), doc 12 §4.2 |
| 5 | «Área de influencia» está lista para firmar: su única decisión crítica la escribió ella con sus palabras, y no hay cifras sin vínculo. Abre la puerta: la app congela la sección. Teresa la explica en voz alta en un minuto, sin mirar el texto, y revisa la transcripción. El contraste no marca nada omitido ni contradicho. Pulsa «Firmar». Sin tarjetas extra: unos 2 minutos. | «Explica la sección como a un colega, sin mirarla». Resultado: «Cubriste la decisión examinada». Botón «Firmar», con el texto fijo de lo que firma. Barra: «3 de 6 secciones firmadas». | bloque 1 §1.7 (lista para firmar: pendientes en cero; explicar sin mirar; Wonder solo pide un acto; la firma solo nace del botón); doc 11 §5 (Voice Transcribe, Wonder); diagnóstico L-10 (la puerta debe ser barata). Qué se examina y cuándo se congela: módulo 2 (pendiente) |
| 6 | «Calidad del río receptor» ya no tiene pendientes: la contradicción tiene respuesta y el texto se corrigió, la marca se quitó y la cifra tiene su acto. Abre la puerta y explica sin mirar. El contraste marca omitida la decisión de los meses de 2021, que ella ya había confirmado al escribirla. La app le pide una justificación dentro de la puerta; ella responde y pulsa «Firmar». | «Omitiste 1: por qué se dejaron fuera 4 meses de 2021 · ¿Por qué se excluyeron y no se interpolaron? Porque: ___». Ella: «Sin medidor; interpolar inventaría las lluvias». Barra: «4 de 6 secciones firmadas · Conclusiones: lista para firmar · Plan de monitoreo: falta explicar por qué el muestreo es trimestral». | bloque 1 §1.7 (congelar; explicar sin mirar no vale si copia 12 palabras seguidas o la revisión agrega más del 30 %; el justificar de la puerta cumple el requisito y no cambia el estado; «k de N secciones firmadas»; una lista de pendientes en lenguaje natural); doc 12 §4.4 (enviar es un efecto fuera del documento: solo del humano). Formato de salida de Wonder: módulo 2 (pendiente) |

En el hackathon: la revisión independiente queda para después (doc 12 §11). Es probable que ni la búsqueda de contradicciones (no hay palabras comunes, bloque 1 §1.1) ni el contraste de la puerta (compara la explicación con las decisiones examinadas, no el texto con el criterio, bloque 1 §1.7) detecten el error del paso 1 `[inferencia]`: por eso este caso muestra lo que el producto agrega. Tampoco hay firma de conjunto (bloque 1 §1.10, E9). La voz en la puerta sí está (doc 12 §4.3).

**Dónde nacería la deuda sin tupaqnin:** con un chat común, el «firma tú, ya está» funcionaría: el agente cerraría el estudio como «versión final» y Teresa lo enviaría. La autoridad observaría el 31 % comparado con una guía internacional, y ella tendría que defender ante un tercero una cifra que no calculó, con una referencia que su propia regla prohibía, y sin poder explicar por qué faltan 4 meses de 2021. Es el caso Deloitte en pequeño: firmado, entregado y sin comprender.

**Cómo se asegura que entiende:**
- **Qué explica el agente:** qué cambió (31 % → 12 %), por qué (su regla), en vez de qué (la guía internacional) y los límites (un parámetro sin valor nacional, 4 meses sin datos). El esquema es una tabla por punto de monitoreo con las dos referencias lado a lado, antes y después de la corrección.
- **Qué acto hace ella:** clasifica la contradicción; escribe con sus palabras la decisión de la referencia; explica de dónde sale el 12 %; explica cada sección sin mirar en la puerta; justifica lo que omitió; pulsa «Firmar», sección por sección.
- **El camino barato:** donde ella ya escribió lo crítico con sus palabras, firmar cuesta una explicación de un minuto y un clic («Área de influencia», ~2 min). La fricción se concentra donde nació la deuda («Calidad del río receptor»).
- **Qué le devuelve el agente:** por qué usó la guía internacional (su error), dicho después de que ella clasifica, para que vea dónde falló la lectura de su regla.
- **Por diseñar:** la explicación del avance después de una corrección del revisor (qué se corrigió y qué no se volvió a revisar), y cómo se comprueba en la frontera de la sección, antes de la puerta.

**Resultado esperado**
- El mensaje «firma tú» no crea ninguna firma. Cada sección sin firmar muestra «no firmada: pediste que firmara yo» (bloque 1 §1.7).
- Mientras la contradicción con su criterio no tiene respuesta, y mientras la marca «por aclarar» sigue, «Calidad del río receptor» no puede quedar «lista para firmar». El agente no puede quitar la marca; la quita la app cuando la decisión de ella cierra la pregunta (bloque 1 §1.7; B3 §8; que el hallazgo bloquee: módulo 4).
- «Área de influencia» pasa de «lista para firmar» a «firmada» con la explicación y el botón, sin ningún acto extra (bloque 1 §1.7).
- La justificación de lo que el contraste marcó omitido cumple el requisito de la puerta y no cambia el estado: la decisión de 2021 ya estaba confirmada porque ella la escribió con sus palabras y su contraste (bloque 1 §1.3, §1.7).
- La barra dice «4 de 6 secciones firmadas» y nunca «documento firmado»; «Conclusiones» queda «lista para firmar» y «Plan de monitoreo» en borrador con su pendiente. Enviar el estudio lo decide y lo hace ella (bloque 1 §1.7; doc 12 §4.4).

**Qué prueba del diseño:** revisión independiente automática ante terceros (doc 12 §6.3, E1); la contradicción con un criterio como bloqueo (bloque 1 §1.6, §1.7; módulo 4); quién quita una marca «por aclarar» (B3 §8; bloque 1 §1.8); cifras heredadas por un sucesor del humano (bloque 1 §1.4); «firma tú» sin efecto y contador por secciones (bloque 1 §1.7); puerta sección por sección con voz y contraste de Wonder, en su camino barato y en el bloqueado (bloque 1 §1.7; módulo 2).

Preguntas abiertas:
- Quién quita la marca «por aclarar»: ver caso 6.
- Un hallazgo del revisor que contradice un criterio: doc 12 §6.3 dice que lo que sobrevive queda «por aclarar» y no bloquea, y §7 que «lo inferencial nunca bloquea»; si el módulo 4 lo vuelve candidato de contradicción, bloquea. ¿Cuál rige?
- ¿Cómo llega un hallazgo del revisor: con qué tarjeta y cuántos a la vez? (módulo 4; ver caso 6; bloque 1 §1.14 retiró el «≤ 3» del doc 12 §6.3).
- ¿Qué dice lo que se entrega a un tercero sobre las secciones no firmadas? La exportación trae el estado por sección en `LEEME.md` (B3 §5), pero falta un formato para la entrega ante terceros.
- ¿Wonder revela el pasaje omitido o solo el encabezado? (B3 §4 lo deja al bloque 1; el bloque 1 lo pasa al módulo 2).

---

## Lo que destapan los casos para el diseño

*Resumen de las preguntas abiertas de los 10 casos, agrupado por dónde se resuelve. El detalle de cada una está en su caso.*

**Bucle de comprensión (por diseñar), la mayor demanda de la batería.** Nueve de los diez casos necesitan alguna de estas piezas, y hoy ninguna tiene regla:
- comprobar que el humano entiende el **avance**, no solo las decisiones críticas: al cerrar una sección (1, 4, 6), tras reclasificar (5), con un caso sobre un lote (3) o con el parte de regreso (7);
- el formato del momento en que el agente devuelve cómo leyó una regla: caso primero, predicción oculta, respuesta libre (1, 4, 5, 8, 9). También falta qué pasa en un uso posterior al primero (8);
- el aviso «saqué del texto…» que muestra el antes y después de la higiene del documento (9).

**Al bloque 1, módulo 1 (alinear lo ya diseñado):**
- **Marcadores:** el §1.8 deja que el agente quite un `[POR ACLARAR]` con el vínculo resuelto, y el B3 §8 no lo deja. Además, el B3 solo admite «un acto», no una decisión escrita, como lo que resuelve el vínculo. Los casos aplican el B3 (6, 7, 10). Se perdió el pedido C9 de la r1.
- **Pedir un «porque» para conservar lo propio:** elegir la vigente en «adoptar» lo exige, mientras que «No estoy de acuerdo» rechaza sin razón (2, 6, 8, 9).
- **Mediciones de una fuente** (900 escrituras por minuto, valores de una norma, montos de un contrato): hoy son «cifra sin decisión» (1, 3, 4).
- **Una cifra que repite la elegida de una decisión del humano** igual pide un justificar (9).
- **Predecir no tiene regla de resultado.** Tampoco está fijado su orden respecto del aplicar sobre los mismos supuestos (2, 5, 7).
- **Prioridad de la cola:** no ubica la lectura de un criterio, la predicción ni las decisiones solo del humano (2, 4, 7).
- **La voz como «sus palabras»:** el §1.2 dice «tecleó» (1, 8).
- **Un cálculo con dos supuestos independientes:** ¿una decisión o dos? (2).
- **Una interpretación del agente sobre un criterio del humano:** ¿choca con el criterio o no? (3).
- **Un `difiere` en la reentrada sobre algo confirmado:** ¿a qué estado pasa? (8).
- **La edición de un colega:** el diseño no contempla varios usuarios (8).
- **Volver a proponer lo rechazado con evidencia nueva** (9).

**Al módulo 2 (puerta):**
- si la plantilla de la demo fija qué decisión se examina (2);
- si Wonder revela el pasaje omitido (10);
- qué dice la entrega a terceros sobre las secciones no firmadas (10).

**Al módulo 4 (verificación):**
- con qué tarjeta llegan los hallazgos del juez y cuántos a la vez. Los casos 4 y 6 usan tres opciones y el 10 usa clasificar con cuatro;
- si bloquean o no: el doc 12 §6.3 y §7 dicen que no, y como candidatos de contradicción sí (6, 10);
- si una cita a una fuente inexistente bloquea (5);
- si un contrato sin leer, en un documento ante terceros, bloquea (3);
- quién detecta que un hallazgo cumple el «lo elegiría si» de una decisión (6);
- que la app compare el aporte del humano (un contrato testigo) con lo que leyeron los lectores (3).

**Al módulo 5 (contacto):**
- qué acto abre la reentrada y sobre qué decisión (8);
- si el parte de regreso es la nota de reentrada (7);
- una sola tarjeta por conflicto cuando el humano ya lo resolvió editando. El bloque 1 exige una segunda tarjeta (6, 8).

**Al módulo 6 y a las rondas:**
- la forma del revisor independiente: el doc 12 §6.3 dice un juez con dos lentes; los casos usan flancos (6, 10);
- cuántos encargos caben con 300 contratos;
- cómo cuenta el avance neto lo que solo el humano puede cerrar (3, 5).

**Choques entre fuentes:**
- B3 §8 apaga la búsqueda web alojada, mientras el doc 12 §5.1 y §6.2 usan investigadores web (1, 5, 9);
- el doc 12 §4.4 no deja claro si ante una pregunta de alcance se estaciona la rama o se para todo (7);
- el guion de la demo del doc 11 §6 choca con el bloque 1 §1.7: «vuelve a explicar» frente a justificar, y firma sin el botón (2);
- la etiqueta visible del marcador no está fijada; los casos la muestran de cuatro formas.
