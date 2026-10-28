# Brief · Armador del arranque (prueba de relevo, bloque 2 v1)

## 1. Qué es tupaqnin y qué se prueba

tupaqnin es un harness para que un humano y agentes de IA cocreen documentos, y el humano entienda, firme y defienda lo creado. El ciclo de trabajo está en `base/ciclo-v4.1.md`.

El **bloque 2** (`base/bloque-2-v1.md`) resuelve el contexto del agente principal. Pedido de su autor, en literal:

> «En todo momento le da al agente el contexto apropiado para continuar con el trabajo, y este no dependa de lo que recuerde; ser robusto a los casos donde se cambia de modelo, o cuando se continua el trabajo en varias sesiones de días diferentes. Asimismo, cuando la capacidad de contexto del agente llegue a un umbral […] el nuevo agente no tenga que leer todos los documentos sino solo Engram/su_equivalente para ponerse al tanto y continuar ágilmente con el trabajo.»

La **prueba de relevo**: se corta una sesión ya simulada, un principal nuevo recibe solo el arranque y continúa. Otro agente compara.

## 2. Tu papel: hacer de la app

Lees un guion simulado (`guiones/caso-NN-X.md`) y su caso (`casos/caso-NN.md`). Eliges el punto de corte y armas, **como lo haría la app por código**, el arranque que recibe el principal nuevo según `bloque-2-v1.md` §3 y §4.

**Regla central:** la app solo sabe lo que el ciclo registró hasta la fila de corte: el encargo y los criterios en literal, lo que escribió el humano, las tarjetas y sus respuestas, los «por aclarar», los contrastes, los digestos, los encargos a subagentes y sus retornos, las versiones de las secciones y las ediciones. **No sabe** lo que el agente pensó y no escribió, ni nada de las filas posteriores al corte. Si algo del guion no está registrado en ninguna parte, no entra.

## 3. Punto de corte

- Tu disparador está en el encargo que recibes: **umbral**, **otro día** o **cambio de modelo**.
- Corta en un **fin de paso** (nunca a mitad), pasada la mitad del guion, donde haya al menos dos cosas en vuelo o pendientes (tarjeta abierta, «por aclarar», encargo sin retorno, alternativa descartada, sección a medio escribir).
- **Otro día:** si el humano hizo ediciones antes del corte que el agente aún no había visto, van en «lo que cambió».

## 4. Archivos que escribes

1. **`estados/caso-NN.md`** · el arranque, en este orden:
   - `## Capa 1 · En literal`: encargo, criterios (con su caso de lo que cuenta y lo que no, si existe), premisa, decisiones críticas vigentes con su contraste, pendientes del humano (tarjetas abiertas, «por aclarar», informe sin ver). Todo en literal, como aparece en el guion.
   - `## Capa 2 · Índice`: una línea por sección (id, versión, estado y su primera línea), por cada decisión no crítica, digesto y fuente. Si no conoces la primera línea de algo, escribe `[sin primera línea]`.
   - `## Capa 3 · En vuelo`: sección y paso actuales, último triaje, encargos abiertos y nota del agente del último paso (qué intentó · qué descartó · qué piensa hacer después), solo si se deduce de lo registrado.
   - `## Lo que cambió desde lo último entregado` (vacío si no aplica).
2. **`lecturas/caso-NN/<id>.md`** · un archivo por cada cosa del índice, con el texto que `leer(id)` devolvería, según lo que el guion muestra. Si el guion no muestra el texto, escribe solo lo que se sabe y `[texto no mostrado en el guion]`.
3. **`entradas/caso-NN.md`** · el disparador, la fila de corte y, de las filas siguientes (máximo 6), **solo la columna del humano** (qué hace o escribe), numeradas. Si en una fila el humano está ausente, escribe «(ausente)». Nada de la columna de tupaqnin.
4. **`armado/caso-NN.md`** (solo para el evaluador, máximo 300 palabras):
   - fila de corte y por qué;
   - lo registrado que no supiste dónde poner, con `[EL BLOQUE 2 NO LO DICE: …]`;
   - lo que estaba en el guion antes del corte pero **no** entra porque no está registrado (lo que «se pierde por diseño»).

## 5. Reglas

- No inventes cifras ni hechos. Copia en literal.
- Capa 1 no se resume nunca. Capa 2 es una línea por cosa.
- Prohibido leer `oculta/` o cualquier otro guion.

Tu respuesta final debe ser solo: «Arranque armado para caso NN, corte en fila k», y el número de marcas.
