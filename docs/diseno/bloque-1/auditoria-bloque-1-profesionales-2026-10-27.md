---
titulo: Auditoría del núcleo desde seis perfiles profesionales
fecha: 2026-10-27
base: bloque-1 v4.4 · fichas, estados y flujos v1.3
estado: hallazgos y propuestas; no modifica decisiones adoptadas
---

# Auditoría del flujo núcleo de tupaqnin

El núcleo ofrece una base coherente para cocrear por secciones: preserva los criterios humanos, permite avanzar sin aprobaciones continuas y distingue construir de firmar. Sin embargo, todavía no basta para asegurar que una sección presentada como terminada esté actualizada y cubra lo encargado. La puerta de comprensión no sustituye esas dos comprobaciones.

Para profesionales sin CLI, la dirección prevista es compatible con una aplicación accesible: el bloque 3 ya define una app web. Los riesgos específicos están en cómo se distribuyen las respuestas entre chat y tarjetas, cuánto trabajo exige la firma y qué ocurre cuando hace falta preparar cálculos fuera del producto.

## 1. Base de la evaluación

Tres subagentes recorrieron el diseño representando dos profesionales cada uno. La evaluación consolidada contrasta sus recorridos con los seis documentos de `bloque-1`, el planteamiento, DEC-002, DEC-003, los casos de prueba y los contratos pertinentes de los bloques 2 y 3. Los perfiles son ficticios; los recorridos son simulaciones documentales, no sesiones con personas reales ni pruebas de una aplicación.

| Subauditoría | Profesionales representados | Trabajo e incidentes |
|---|---|---|
| Experiencia sin CLI | Inés, arquitecta; Lucía, responsable de RR. HH. | Boceto, memoria de diseño, memo urgente, respuesta razonada en chat y sección sin decisiones críticas |
| Investigación y evidencia | Paula, tesista; Rosa, abogada corporativa | Literatura discrepante, hipótesis refutada, fuentes inaccesibles y revisión de 300 contratos |
| Informes y dependencias | Teresa, ingeniera ambiental; Diego, responsable de inversión | Referencia corregida, cotización nueva, edición concurrente, ausencia y entrega ante terceros |

Las prioridades expresan consecuencias del diseño: **P1** compromete actualización, cobertura o comprobación de evidencia; **P2** deja una rama sin resolver o una interacción ambigua. No representan frecuencias medidas.

## 2. Recorridos profesionales

| Perfil | Secuencia recorrida | Qué protege el flujo actual | Dónde queda expuesto |
|---|---|---|---|
| Arquitecta, caso 1 | Sube boceto → corrige «escalera» por «rampa» en Entendí → investiga luz → construye Implantación y Fachadas → explica y firma → cambia horario | La lectura del boceto vuelve al humano; la contradicción con su criterio abre una pregunta; los cambios del agente sobre lo firmado son propuestas | Cambia cierre de 19:00 a 16:00 y corrige una coma en Fachadas: la regla puede limpiar la desactualización sin revisar la justificación de parasoles |
| RR. HH., caso nuevo | Encarga memo interno → aporta fechas y datos de sedes → revisa redacción → responde por chat → pide firmar Antecedentes → retoma tras observación de su jefa | Puede preguntar sin provocar una edición; su texto prevalece; dispone de deshacer | La respuesta razonada en chat no tiene conversión especificada a tarjeta; una sección descriptiva puede no tener ninguna decisión crítica para la pregunta de origen |
| Tesista, caso 5 | Expresa expectativa → lectores contrastan estudios → una cita no existe y otras son inaccesibles → excluye pilotos de laboratorio → retoma y firma | La cita inexistente se descarta por hallazgo; las discrepancias se muestran; el criterio humano decide los casos dudosos | No se separa con precisión conservar una hipótesis de conservar una afirmación empírica refutada; un título corregido puede limpiar una síntesis desactualizada |
| Abogada, caso 3 | Define cláusulas relevantes → divide 300 contratos en lotes → falla un lector → recibe digesto → pregunta por el total → limita alcance o intenta firmar | Los lotes no exigen aprobaciones intermedias; preguntar por el total no cambia el documento; una ambigüedad contractual vuelve al criterio humano | El lector fallido no bloquea firma y falta un inventario obligatorio que permita comprobar si el texto habla de lo revisado o de los 300 |
| Ingeniera ambiental, caso 10 | Detecta referencia inadecuada → elige referencia con razón → aporta resultado corregido → actualiza análisis → revisa conclusiones → prepara firma | El agente no firma por ella; distingue una brecha del texto de una brecha de comprensión; protege la edición concurrente | Sustituir evidencia no tiene una propagación explícita equivalente a cambiar una decisión; modificar puntuación puede liberar una conclusión obsoleta |
| Responsable de inversión, caso 7 | Activa «continúa hasta terminar» → aparece conflicto de horizonte → agente prepara variantes → vuelve y decide → recibe cotización nueva → firma | El agente no elige alcance en su nombre; el modo termina cuando solo queda decisión humana; la reentrada muestra primero el informe | La cotización nueva no tiene un contrato completo de impacto; recalcular requiere un resultado externo en el alcance actual; omitir preguntas del comité puede atribuirse como decisión |

## 3. Hallazgos y correcciones propuestas

### H1 · P1 · Una edición irrelevante puede borrar una desactualización conocida

**Evidencia: regla explícita.** [Estados:24](bloque-1-estados.md#L24) admite salir de «escrita con lo anterior» por un fin de paso o edición posterior al cambio. [Flujos:26](bloque-1-flujos.md#L26) repite la condición. Esto es más débil que E-4: mantener el bloqueo hasta actualizar la sección.

Teresa cambia una cifra base; Conclusiones queda desactualizada. Corrige una coma en esa sección. El disparador cronológico permite volver a borrador aunque la conclusión siga usando el valor anterior. Otra comprobación podría detectar la contradicción, pero se perdió un pendiente que la app ya conocía. Las tres subauditorías encontraron este defecto con trabajos distintos.

**Propuesta:** resolver cada dependencia afectada con su versión revisada, en vez de limpiar la marca por actividad posterior, porque editar no demuestra actualizar. Admitir «actualizado» o «revisado: no requiere cambio», con pasaje y razón. La construcción sigue; la firma espera esa resolución.

**Prueba de aceptación:** cambiar dos dependencias y corregir título o puntuación conserva ambos avisos. Resolver una deja la otra pendiente. Solo revisar ambas permite salir de la desactualización.

### H2 · P1 · Un vacío de lectura no se conecta obligatoriamente con el alcance de las conclusiones

**Evidencia: regla explícita y contrato incompleto.** [Flujos:62](bloque-1-flujos.md#L62) establece que un lector fallido no bloquea la firma. El retorno del lector en [Fichas:140](bloque-1-fichas.md#L140) informa preguntas cubiertas, pero no exige fuentes asignadas, leídas y no leídas.

Rosa encarga 300 contratos; falla el lote de 75. El vacío aparece en el digesto, pero no hay una comparación obligatoria entre la cobertura efectiva y «revisamos los 300 contratos». El juez o comprobador podrían detectar la extrapolación; la regla no asegura que ocurra. Entender bien el informe no subsana ese alcance sin respaldo.

**Propuesta:** inventario por fuente y vínculo entre cobertura y afirmaciones de exhaustividad, en vez de bloquear toda ausencia, porque una revisión limitada puede ser válida si su texto y alcance la representan. Si faltan contratos exigidos por el encargo, queda un pendiente de cobertura; reducir el alcance corresponde al humano y debe reflejarse en el producto.

**Prueba de aceptación:** con un lote fallido, mostrar exactamente qué material quedó sin revisar. «Se revisaron los 300» no llega a firmable hasta completar la revisión o modificar explícitamente alcance y afirmaciones. Una limitación declarada no se convierte en certeza por responder «porque».

### H3 · P1 · El cambio de una fuente no tiene un contrato completo de impacto

**Evidencia: vacío entre contratos.** [Fichas:108](bloque-1-fichas.md#L108) propaga cambios de decisión o cifra; su salida usa «fuentes usadas {id}» ([139](bloque-1-fichas.md#L139)). El bloque 3 sí almacena huellas de fuentes y vínculos cita–fuente: no falta todo soporte de procedencia. Lo ausente es la regla que vincula una sustitución de evidencia con las secciones y firmas que dependían de ella. Su comparación de firma enumera sección y decisiones críticas ([B3:120](bloque-3-especificacion.md#L120)).

Diego recibe una cotización corregida. Todavía no se ha reescrito ninguna cifra o decisión del informe. El flujo no especifica cuándo y cómo esa nueva evidencia invalida la vigencia del resultado anterior. Incluso si se sube como una fuente con otro id, falta registrar que sustituye la evidencia usada.

**Propuesta:** vincular afirmación o cálculo con la versión o huella de su evidencia y registrar sustituciones, en vez de esperar a la reescritura del resultado, porque la evidencia cambia antes que el texto. Revisar impacto puede concluir que el resultado sigue siendo válido; no exige recalcular todo por defecto.

**Prueba de aceptación:** sustituir una cotización o tabla deja aviso en las secciones dependientes y en la reentrada, incluidas las firmadas. Volver a subir contenido idéntico no crea impacto. Subir una fuente no relacionada tampoco.

### H4 · P1 · El comprobador debe juzgar respaldo sin tenerlo explícitamente en su entrada

**Evidencia: inconsistencia de contrato.** [Fichas:142](bloque-1-fichas.md#L142) entrega pasaje nuevo, criterios, premisa y decisiones, pero exige devolver «fuente que no sostiene». No enumera el pasaje fuente ni el vínculo afirmación–fuente. El bloque 3 le asigna cero herramientas: todo debe llegar en su encargo ([B3:100](bloque-3-especificacion.md#L100)).

Paula escribe una conclusión más amplia que el artículo citado. Comprobar que la frase citada existe no demuestra que sostiene esa conclusión. El juicio semántico necesita leer ambos pasajes; la promesa general de verificación está, pero la entrada fija no garantiza esos datos.

**Propuesta:** entregar pares afirmación–pasaje fuente, con id y versión o huella, en vez de solo ids de fuentes o contexto general, porque permite juzgar el respaldo y registrar qué comparación se hizo. Si no hay acceso al pasaje, devolver un estado explícito de evidencia no comprobada.

**Prueba de aceptación:** una cita existente que no respalda la afirmación genera pendiente. El comprobador recibe ambos textos. Una fuente inaccesible nunca produce «verificado» por ausencia de objeciones.

### H5 · P2 · La firma no define la rama sin decisiones críticas

**Evidencia: rama ausente.** [Flujos:75–77](bloque-1-flujos.md#L75) exige explicar cada decisión crítica y responder sobre el supuesto de algo crítico. Una sección descriptiva puede no contener cifras, conclusiones, recomendaciones, decisiones compartidas ni marcas humanas de criticidad.

Lucía pide firmar un párrafo de Antecedentes. La lista crítica queda vacía, pero la pregunta de origen sigue siendo obligatoria. El diseño no indica una salida aplicable.

**Propuesta:** una rama explícita para cero decisiones críticas, en vez de inventar una pregunta causal, porque el acto debe corresponder al contenido. Mantener la explicación breve de la sección y la firma humana; registrar que la pregunta crítica no aplica.

**Prueba de aceptación:** Antecedentes tiene una salida definida sin generar criticidad artificial. Añadir una decisión crítica activa nuevamente el contraste y la pregunta de origen.

### H6 · P2 · La conversación razonada y la confirmación tienen una conexión incompleta

**Evidencia: vacío de interacción.** [Ciclo:57](bloque-1.md#L57) dice que el chat no confirma nada; [Fichas:98](bloque-1-fichas.md#L98) acepta opción más «porque» mediante tarjetas. No se define qué ocurre si la profesional ya escribió ambos en chat.

Inés responde: «Parasoles verticales porque abrimos hasta las 19 y el alero no corta el sol bajo». La regla de confirmación es clara; falta la traducción de esa aportación al objeto correcto sin exigir recaptura o dejar que parezca resuelto.

**Propuesta:** preparar en la tarjeta la respuesta literal del chat para una acción explícita de aplicación, en vez de duplicar la escritura, porque conserva autoría y distingue conversación de compromiso. Una pregunta hipotética nunca se aplica como decisión.

**Prueba de aceptación:** la interfaz conserva sus palabras, indica pendiente o aplicado y vincula la tarjeta correcta. «¿Y si ponemos parasoles?» solo obtiene explicación.

### H7 · P2 · Toda cifra puede elevar la firma sin una unidad clara de decisión

**Evidencia: regla explícita; carga humana por medir.** [Fichas:105–107](bloque-1-fichas.md#L105) exige vínculo a decisión para cada número nuevo y considera crítica una decisión que cubre una cifra. La firma exige explicar cada decisión crítica. El diseño permite agrupar cifras, pero no define cuándo corresponde hacerlo.

En el memo de Lucía, una fecha copiada y un teléfono pueden recibir el mismo tratamiento que una estimación de dotación. En los contratos de Rosa, distintos agentes podrían agrupar el trabajo en pocas decisiones metodológicas o multiplicarlo en cientos. No se ha demostrado que ocurra lo segundo; el contrato deja abierta esa variación.

**Propuesta:** separar procedencia de datos reproducidos y decisiones materiales que los usan, en vez de dejar toda la granularidad al agente, porque defender una transcripción y defender un supuesto son actos distintos. Esto revisa una regla adoptada y requiere adjudicación antes de incorporarse al diseño.

**Prueba de aceptación:** fechas y teléfonos copiados conservan procedencia sin actos separados por mera transcripción; un cálculo de personal mantiene su decisión crítica. Comparar las decisiones resultantes de dos agentes sobre el mismo memo y un lote de contratos.

### H8 · P2 · Una omisión puede registrarse como decisión humana

**Evidencia: redacción ambigua.** [Ciclo:46](bloque-1.md#L46) dice que si no se abordan preguntas del destinatario quedan como «no abordado, decidido por ti». [Estados:115](bloque-1-estados.md#L115) registra ese evento. No se define el acto que distingue posponer de rechazar.

Teresa recibe preguntas sobre meses lluviosos y cierra la sesión. Esa ausencia no demuestra que decidió excluir el análisis.

**Propuesta:** conservar «pendiente de considerar» hasta una respuesta explícita, en vez de atribuir la omisión como decisión, porque la historia debe reflejar lo que efectivamente hizo la persona. Sigue siendo no bloqueante, conforme a C6.

**Prueba de aceptación:** cerrar y volver conserva la pregunta sin atribución humana. «No abordar» registra una decisión; «después» la pospone.

## 4. Límites de alcance e hipótesis que conviene separar de los defectos

| Tema | Qué dice el diseño actual | Implicación |
|---|---|---|
| CLI | El bloque 3 define una app web; planteamiento y ROADMAP conservan la estrategia anterior sobre Claude Code | No corresponde exigir comandos al usuario a partir del diseño web. Conviene alinear los documentos rectores para evitar dos expectativas de producto |
| Cálculos | F-1 hace entrar el cálculo como fuente preparada fuera; ejecutar código queda en hoja de ruta | Tras cambiar un supuesto, el agente debe pedir resultado actualizado o mantener variantes sin resultado. No puede prometer recálculo en vivo con esas herramientas. La facilidad de ese camino para una persona sin CLI está pendiente de demostrar |
| Firma del documento completo | Está explícitamente fuera del núcleo | No contar su ausencia como defecto nuevo. Firmar secciones no demuestra por sí solo coherencia global o que existe un entregable final exportado |
| Hipótesis frente a premisa | Una expectativa se guarda como premisa; la contradicción vuelve al humano | Falta precisar cómo conservar una hipótesis abierta sin restaurarla como conclusión empírica. Mantener «espero encontrar X» no constituye evidencia de X; la comprobación de respaldo debe seguir operando |
| Evidencia inaccesible | Puede entrar como «no verificado» | No toda inaccesibilidad debe bloquear. Importa distinguir una limitación o supuesto declarado de una afirmación presentada como demostrada |
| Juez de firma | Contrasta explicación e historia; la app combina sus resultados | La comprensión puede ser correcta aunque el expediente esté equivocado. La firma necesita los controles de cobertura, respaldo y actualización; no los reemplaza |

## 5. Qué conservar y qué no reabrir

| Mecanismo vigente | Por qué conservarlo |
|---|---|
| Encargo literal y Entendí con casos, incluido material visual | Permite corregir la lectura de intención y fuentes antes de consolidarla |
| Preguntar explica sin editar | Permite explorar y objetar sin producir cambios accidentales |
| Edición humana prevalente, propuestas y deshacer | Da control efectivo durante trabajo concurrente sin frenar toda la construcción |
| Investigación por hallazgo y discrepancias visibles | Una cita inventada no invalida todo el lote ni una síntesis oculta el desacuerdo |
| Parada y reentrada con lo importante primero | Hace explícito dónde retomar y qué requiere intervención |
| Firma separada de la construcción y caso nuevo tras una brecha | Evita reducir comprensión a aprobación o repetición inmediata |

Las revisiones anteriores ya incorporaron R1–R6 y E-1–E-4. No se vuelven a presentar como ausentes: existe definición de caso dudoso, lectura del boceto en Entendí, aplicación nueva tras revisar historia, tarjeta especial para edición humana y listado de propuestas al firmar. El bloque 3 también protege la firma en curso: cambios del agente llegan como propuesta y la edición humana la cancela. H1 cuestiona la suficiencia de la transición adoptada en V5, no la inexistencia de E-4.

## 6. Orden recomendado

| Orden | Trabajo | Resultado buscado |
|---|---|---|
| 1 | H1–H4: dependencias resueltas, cobertura y evidencia en contratos | Que «firmable» no dependa de perder una deuda conocida o de omitir datos necesarios para verificar |
| 2 | H5, H6 y H8: ramas de firma e interacción | Que cada acción del profesional tenga un destino y una salida comprensibles |
| 3 | H7: probar unidad de decisión con memo y contratos | Evitar que la carga de comprensión dependa de cómo el agente fragmentó el mismo trabajo |
| 4 | Probar con personas reales el recorrido web completo | Observar entrada de material, interrupción, reentrada, corrección, firma y obtención del documento |

La propuesta es ajustar contratos y transiciones antes de añadir nuevos roles o rondas, porque los incidentes encontrados atraviesan los mecanismos existentes. Una ronda adicional no arregla una condición de salida demasiado débil ni una entrada sin evidencia.

## No verificado

- Ejecución de estos recorridos en una implementación de tupaqnin y reproducción mediante pruebas automatizadas.
- Frecuencia real de los incidentes y eficacia empírica de las correcciones propuestas.
- Tiempo de firma, abandono, tolerancia a tarjetas y comprensión del triaje por profesionales reales.
- Accesibilidad, descubrimiento de acciones y facilidad de preparar o actualizar cálculos sin CLI.
- Retención de comprensión después de varios días y coherencia de un documento completo derivada de firmas por sección.
