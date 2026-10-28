# Brief · Armador del arranque (regresión de relevo, bloque 2 v2.1)

Carpeta de trabajo: `/home/claude/relevo-b2-v2/` (todas las rutas son relativas a ella).

## 1. Qué es tupaqnin y qué se prueba

tupaqnin es un harness para que un humano y agentes de IA cocreen documentos, y el humano entienda, firme y defienda lo creado. El ciclo está en `base/bloque-1.md`; sus contratos en `base/bloque-1-fichas.md` (§7 contratos, §9 grafo del proyecto) y sus estados en `base/bloque-1-estados.md`.

El **bloque 2** (`base/bloque-2-v2.1.md`) es la memoria del agente principal. Pedido de su autor, en literal:

> «En todo momento le da al agente el contexto apropiado para continuar con el trabajo, y este no dependa de lo que recuerde; ser robusto a los casos donde se cambia de modelo, o cuando se continua el trabajo en varias sesiones de días diferentes. Asimismo, cuando la capacidad de contexto del agente llegue a un umbral […] el nuevo agente no tenga que leer todos los documentos sino solo Engram/su_equivalente para ponerse al tanto y continuar ágilmente con el trabajo.»

Decisiones vigentes que debes aplicar al pie de la letra (en literal, de `base/bloque-2-v2.1.md`): la memoria es «una consulta del grafo del proyecto (bloque 1, fichas §9), más los eventos de la historia que no son nodos del grafo (modo, nota del agente, último triaje, encargos abiertos, tus mensajes e informe de parada). La app la recalcula en cada fin de paso; el agente no decide qué guardar. Copia los estados del bloque 1 y no define estados propios.» Las 3 capas y el arranque están en §2 y §3 de ese archivo.

**Esta es una regresión.** Una prueba anterior (v1, con otro diseño) usó estos mismos guiones y cortes. Ahora se prueban tres hipótesis que la v1 no pudo probar:
1. La memoria calculada desde el grafo basta para continuar.
2. La primera línea basta como entrada del índice.
3. La nota del agente aporta algo que la memoria no tiene.

## 2. Tu papel: hacer de la app

Lees un guion simulado (`guiones/caso-NN-B.md`) y su caso (`casos/caso-NN.md`). Cortas en la **fila indicada en tu encargo** (la misma de la v1) y armas, **como lo haría la app por código**, el arranque que recibe el principal nuevo según `base/bloque-2-v2.1.md` §2 y §3.

**Regla central:** la app solo sabe lo que el ciclo registró hasta la fila de corte: el encargo y los criterios en literal, lo que escribió el humano, las tarjetas y sus respuestas, los «por aclarar», los contrastes, los digestos, los encargos a subagentes y sus retornos, las versiones de las secciones, las ediciones y los mensajes del humano. **No sabe** lo que el agente pensó y no escribió, ni nada de filas posteriores al corte.

**Dos excepciones nuevas, porque el diseño v2.1 las exige y los guiones no las traen:**
- **Primera línea.** Por la regla «primero la respuesta», cada sección, digesto o decisión empieza con una línea que ya dice lo esencial. Si el guion no la muestra, escríbela tú como la habría escrito el principal original, **solo con lo que el guion muestra hasta el corte**, y márcala `[1.ª línea reconstruida]`. Si no hay contenido suficiente, escribe `[sin primera línea]`.
- **Nota del agente.** Es parte de la salida de cada paso (fichas §7: «nota corta para el bloque 2»). La escribe el principal original, que sí sabía lo que intentó. Escríbela tú **haciendo del principal original en el fin de paso de la fila de corte**, en máximo 80 palabras, con tres partes: qué intentó · qué descartó · qué piensa hacer después. Básate solo en la columna de tupaqnin y lo que el agente dijo o hizo hasta el corte; no uses filas posteriores. Va marcada «del agente».

## 3. Archivos que escribes (NN = tu caso)

1. **`estados/caso-NN.md`** · el arranque, en este orden (el orden aprovecha la caché por prefijo, §3 del bloque 2):
   - `## Capa 1 · Lo esencial, en literal`: todo lo que lista la fila «1» de la tabla de §2 del bloque 2 (pedido y destinatario · criterios vigentes con sus casos y estado · premisa o «exploratorio» · rol de cada sección · decisiones críticas vigentes con estado, contraste, ids que mueven y aristas · **lo que te toca**: tarjetas abiertas, «por aclarar», propuestas, casos sin confirmar, preguntas del destinatario abiertas, firma en pausa, informe sin ver y mensajes del humano sin respuesta). Nunca resumida.
   - `## Capa 2 · Índice`: una línea `id · versión · estado · primera línea` por cada sección (uno de sus 5 estados del bloque 1 más el número de pendientes), decisión no crítica, digesto, retorno de subagente y fuente, incluidos los cálculos. Una sección prevista sin texto es una sección en borrador.
   - `## Capa 3 · En vuelo`: modo y el mensaje del humano al activarlo · sección y paso actuales · último triaje · pendientes del agente · cambios sin cerrar por sección · inventario de lo no revisado · encargos abiertos · y **una línea que diga exactamente `<<NOTA>>`** donde iría la nota del agente (la app la inserta después).
   - `## Tarea en turno`: la sección actual y lo que usa, según el grafo (ids).
   - `## Lo que cambió desde lo último entregado` (vacío si no aplica).
   - `## Tus últimos mensajes y sus respuestas, en literal` (los últimos 3 intercambios del humano, como máximo).
2. **`nota/caso-NN.md`** · solo la nota del agente (ver §2), sin encabezados extra.
3. **`lecturas/caso-NN/<id>.md`** · un archivo por cada cosa del índice, con el texto que `leer(id)` devolvería (empieza por su primera línea). Si el guion no muestra el texto, escribe lo que se sabe y `[texto no mostrado en el guion]`. **Caso 03:** si hay muchos contratos indistinguibles, crea solo los que el guion distingue y uno `K-resto.md` que diga cuáles son y que no tienen nada registrado; en el índice basta una línea para el grupo, si cumple la regla de una línea por cosa que la app distinga.
4. **`armado/caso-NN.md`** (solo para el evaluador, máximo 300 palabras):
   - fila de corte;
   - lo registrado que no supiste dónde poner, con `[EL BLOQUE 2 NO LO DICE: …]`;
   - lo que estaba en el guion antes del corte pero no entra porque no está registrado;
   - qué primeras líneas reconstruiste.

Las entradas del humano ya están en `entradas/caso-NN.md`: no las toques.

## 4. Reglas

- No inventes cifras ni hechos. Copia en literal.
- Capa 1 no se resume nunca. Capa 2 es una línea por cosa.
- **Prohibido** leer `oculta/`, `v1/`, `relevos/`, otros guiones u otros casos.

Tu respuesta final debe ser solo: «Arranque armado para caso NN, corte en fila k», el tamaño de `estados/caso-NN.md` en caracteres (`wc -c`) y el número de marcas.
