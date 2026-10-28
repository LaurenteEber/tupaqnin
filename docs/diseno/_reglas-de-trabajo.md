---
titulo: Reglas de trabajo y diseño de tupaqnin (vigentes desde el 26-oct-2026)
para: todo chat o agente que trabaje en el diseño de tupaqnin
copia: doc del proyecto «tupaqnin» en claude.ai (claude/reglas-de-diseno-tupaqnin.md); mismo contenido, y si difieren rige la más reciente
---

# Reglas de trabajo y diseño de tupaqnin

*Nacen de un diagnóstico: el diseño del bloque 1 cayó en la deuda cognitiva que tupaqnin quiere resolver. El módulo 1 llegó a unas 12 mil palabras, con 183 hallazgos (ninguno rechazado en la ronda 2), más de 40 estados y unos 67 códigos de auditoría dentro del documento. Eber aprobaba lo que se le recomendaba sin poder explicarlo sin mirar. Estas reglas evitan repetirlo.*

**Para empezar en un chat nuevo,** lee este documento y después, en este orden:
1. `docs/diseno/bloque-1/bloque-1.md`, el ciclo vigente;
2. `docs/diseno/bloque-1/bloque-1-decisiones.md`, las decisiones de Eber con su contraste;
3. si vas a probar el ciclo, `docs/casos-prueba/simulacion-v4.1/resultado.md` y `docs/casos-prueba/regresion-v4.2/resultado.md`.

## 1. Qué espera Eber del bloque 1 (literal, 26-oct)

> El bloque 1 debe abordar el diseño completo del flujo de funcionamiento de tupaqnin, toda la lógica de trabajo entre el humano y agentes, pero sin abordar lo que se ve en el bloque 2 y 3. Este bloque 1 debe plantear los pasos, etapas, ciclos, etc del trabajo. Este diseño debe plantearse siempre de modo simple y entendible, priorizar el uso de gráficas y esquemas para sintetizar. Cómo se inicia, que hace y ve el humano, qué hace el agente principal, cómo realiza los encargos a los subagentes (por ejemplo, acá es muy importante que el principal provea un contexto idóneo), etc. Y de modo transversal se debe mostrar cómo tupaqnin aborda la deuda cognitiva, de cómo se usan las capacidades potentes del agentes IA para construir en conjunto con el humano.

Bloques: 1 Núcleo (flujo humano-agentes) · 2 Memoria del orquestador · 3 Plataforma y archivos.

## 2. Reglas para los documentos de diseño

1. **Máximo 2 páginas por módulo**, con al menos un esquema. Primero el esquema y después el texto. Comprueba que el esquema se renderiza antes de entregarlo.
2. **Tres cosas separadas:**
   - **idea:** qué y por qué; es lo que Eber lee;
   - **especificación:** campos, umbrales, listas; va a código y pruebas;
   - **proceso:** hallazgos, códigos, parches, simulaciones; va a su propio archivo, nunca al documento.
3. **Lo que se puede probar construyendo no se especifica.** Queda como hipótesis de una línea, con su forma de probarla (L-13).
4. **Presupuesto de complejidad:** cada cambio declara cuántos estados y señales agrega. Si hay muchos hallazgos, primero se simplifica el mecanismo; no se parchea uno por uno.
5. **Proporcionalidad:** antes de agregar un control, nombra qué protege: un criterio, un riesgo concreto o un compromiso. Si no protege nada concreto, no bloquea. Ante señales de sobreingeniería, primero se quita o se fusiona.
6. **Primero la respuesta.** Todo lo que se muestra a Eber, y lo que el producto muestra al humano, empieza por la decisión o el resultado; el detalle va después.
7. **Sin nombres internos** en lo que ve el usuario, y en el diseño solo los necesarios.
8. **Toda decisión se registra con su contraste** («en vez de», «elegiría la otra si…») en `bloque-1-decisiones.md`, con la respuesta de Eber en literal. Al compactar, el contraste no se pierde.

## 3. Reglas para trabajar con Eber

1. **Eber primero.** Antes de que los agentes analicen un tema, Eber puede escribir o dibujar su idea en 5–10 líneas. Los agentes contrastan esa idea; no la generan.
2. **Toda propuesta del agente lleva contraste:** «A en vez de B, porque…; elegiría B si…».
3. **Eber adjudica.** Las propuestas van como filas en `bloque-1-decisiones.md`, con la columna «Decisión de Eber» vacía. Eber responde en el archivo. Que no se rechace ninguna es una alarma.
4. **Prueba de cierre:** Eber explica el ciclo sin mirar (3 minutos, voz o texto). Lo que no puede explicar todavía no está cerrado.
5. **Encargos a subagentes:** contexto completo y preciso, armado como plantilla:
   - pedido literal;
   - criterios y decisiones en literal, nunca parafraseados;
   - pregunta que responde;
   - alcance;
   - archivos y para qué sirve cada uno;
   - reglas de evidencia (cita literal, `[inferencia]`);
   - formato de retorno fijo y con tope.

   El contexto laxo ya costó trabajo perdido.
6. **Delegar solo con una razón nombrada:** leer más de lo que cabe en una pasada, partes independientes, o un riesgo alto o una auditoría que pides (un contexto largo ya no es razón: B2-4). El tamaño nunca es razón.
7. **Auditorías:** un solo auditor independiente, sin las conclusiones del chat principal, con un máximo de 6 hallazgos y correcciones mínimas. Prohibido rellenar. Hay una corrección y una sola reverificación; lo que queda abierto lo decide Eber.
8. **Antes de entregar un hallazgo a Eber,** el chat principal lo verifica contra el texto o el guion que cita.
9. **Commit al cerrar cada sesión de trabajo.** Ya se perdió una versión por no tenerla en git. Desde el chat no se borra nada en la carpeta de Eber: se mueve o se le pide a él.
10. **Salidas concisas, estructuradas y prácticas.** En español, con frases cortas.
11. **Un solo escritor por archivo (28-oct).** Dos programas que escriben el mismo archivo se pisan sin aviso (Obsidian guarda su copia en memoria; la herramienta del chat puede entregar una copia vieja).
    - Antes de editar, el chat avisa qué archivos tocará y Eber cierra esas pestañas en Obsidian; al terminar, el chat avisa «listo».
    - El chat lee el archivo del disco justo antes de editarlo y edita sobre esa versión; si cambió, vuelve a leer y fusiona.
    - El chat verifica unos 30 s después de escribir que su cambio sigue ahí; si no, lo dice de inmediato.
    - Si Eber responde en un archivo, lo avisa en el chat; si responde en el chat, lo escribe el chat.
    - Commit antes de cada tanda larga de cambios del chat.

## 4. Cómo se prueba el ciclo: simulación con subagentes

Se usa para validar el ciclo contra los casos de `docs/casos-prueba/`. Los briefs completos y un ejemplo de cada pieza están en `docs/casos-prueba/simulacion-v4.1/`.

1. **Preparar.** Cada caso se reduce a su núcleo, unas 500 palabras: persona, objetivo, lo que trae el humano (con una creencia equivocada si la hay), lo que sabe y encontrará el agente (incluidos sus errores) y dos perfiles. **Ciego:** el caso no dice dónde nace la deuda. Eso va en una hoja aparte que solo ve el evaluador.
2. **Simular en paralelo.** Un subagente por caso y perfil, con dos perfiles: A diligente y B apurado. El simulador aplica el ciclo al pie de la letra, no inventa reglas y marca solo lo que cambia el resultado (máximo 6 marcas), con tres tipos:
   - `[EL CICLO NO LO DICE]`;
   - `[AMBIGUO]`;
   - `[CHOCAN]`, solo entre dos reglas del ciclo.

   No evalúa.
3. **Evaluar con otros agentes.** Los evaluadores no simularon nada y conocen la hoja oculta. Para cada guion dicen si hubo protección y en qué paso, si el humano podría defenderse ante un tercero, y dan como máximo 2 hallazgos tipificados.
4. **Consolidar.** El chat principal verifica contra los guiones y propone como máximo 6 cambios, que Eber adjudica. Pesa más lo que se repite entre casos y perfiles.
5. **Regresión.** Después de cambiar el ciclo, se vuelven a simular los guiones donde falló.

**Límite:** prueba la lógica del ciclo, no el comportamiento real de los modelos ni la interfaz. El perfil diligente simulado es optimista.

## 5. Principios del producto que no cambian

DEC-002 (codirección y comprensión compartida), DEC-003 (nadie bloquea; quien propone explica; la comprensión se comprueba con actos; una sola puerta: la firma) y las lecciones L-01 a L-16 (`docs/casos/diagnostico-y-aprendizajes.md`).

## 6. Estado (26-oct, noche)

- **Ciclo vigente:** `docs/diseno/bloque-1/bloque-1.md` v4.3. Integra D1–D22, P1, P3–P6, P2 ampliada, C1–C6, R1–R6 (R3 reemplazada por R3′: en la firma, una pregunta de aplicación con un caso nuevo) y la decisión sobre sensibilidad, que no es objetivo del ciclo (la cubre C6). La v4.2 se validó y pasó una regresión en los casos 1, 8 y 9.
- **Carpetas (27-oct):** el bloque 1 vive en `docs/diseno/bloque-1/`. Lo retirado está en `docs/diseno/_por-borrar/`, con su evaluación en `INDICE.md`; lo borra Eber. En la raíz de `docs/diseno/` quedan dos punteros temporales (`bloque-1.md` y `bloque-1-decisiones.md`) hasta que el bloque 2 actualice sus enlaces.
- **Fichas por etapa y estados** (anexo de implementación, v1 borrador): `docs/diseno/bloque-1/bloque-1-fichas.md`. Dice qué componente hace cada cosa (app, principal, lector, juez, comprobador, Wonder), qué calcula la app frente a qué juzga un modelo, los estados de los objetos y qué entrega el bloque 1 a los bloques 2 y 3.
- **Pruebas hechas:**
  - `docs/casos-prueba/piloto-v4/`;
  - `docs/casos-prueba/simulacion-v4.1/`;
  - `docs/casos-prueba/regresion-v4.2/`.
- **Bloques en paralelo:** el bloque 2 avanza en otro chat (`docs/diseno/bloque-2/`) y usa los términos del bloque 1. Si cambia un término del bloque 1 (paso, parada con informe fijo, triaje, tarjetas, «por aclarar», premisa, historia, encargo = §4, C1, C3), hay que avisarlo. El bloque 3 se retoma después.
- **Siguiente paso:**
  1. prueba de cierre de Eber;
  2. anexo de implementación del bloque 1: qué calcula la app y qué juzga un modelo, roles con su modelo y esfuerzo, objetos mínimos y valores iniciales;
  3. recorte del hackathon, caso de demo y Gantt;
  4. revisar el bloque 3 contra la v4.2;
  5. esqueleto de la app.
- **No se retiran:** DEC-002, DEC-003, `casos/`, la investigación 11–13, el encuadre y el bloque 3.
