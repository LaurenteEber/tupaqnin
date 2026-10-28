# Brief · Armador (regresión de relevo, bloque 2 v3)

Carpeta: `/home/claude/relevo-b2-v3/` (rutas relativas a ella).

## 1. Qué es y qué se prueba
tupaqnin es un harness para que un humano y agentes de IA cocreen documentos, y el humano entienda, firme y defienda lo creado. Ciclo: `base/bloque-1.md`; contratos en `base/bloque-1-fichas.md` (§3 comprobaciones, §7 contratos, §9 grafo); estados en `base/bloque-1-estados.md`.

El **bloque 2** (`base/bloque-2-v3.md`) es la memoria del agente principal. Pedido de su autor, en literal: «En todo momento le da al agente el contexto apropiado para continuar con el trabajo, y este no dependa de lo que recuerde […] el nuevo agente no tenga que leer todos los documentos sino solo Engram/su_equivalente». Y (28-oct): «El contexto de arranque debe ser ligero solo lo necesario para continuar con la siguiente tarea en cola o lo que pida el humano».

Una prueba anterior (v2) usó este mismo guion y corte con un arranque que traía todo el proyecto. Ahora el arranque **es por tarea y con tope** (`bloque-2-v3.md` §2 y §3). Se prueba si eso basta para continuar, y se preparan las sondas contra la contradicción (§5).

## 2. Tu papel: hacer de la app
Lees `guiones/caso-03-B.md` y `casos/caso-03.md`. Cortas **al fin de la fila 10**. Disparador: umbral. Armas, como lo haría la app por código, lo que pide §2, §3 y §5 de `bloque-2-v3.md`, al pie de la letra.

**Regla central:** la app solo sabe lo registrado hasta la fila 10 (encargo y criterios en literal, lo que escribió el humano, tarjetas y respuestas, «por aclarar», contrastes, digestos, encargos y retornos, versiones, ediciones, mensajes). No sabe lo que el agente pensó y no escribió, ni nada posterior al corte.

**Dos excepciones, porque el diseño las exige y el guion no las trae:**
- **Primera línea** de cada sección, digesto o decisión: si el guion no la muestra, escríbela como el principal original, solo con lo que hay hasta la fila 10, marcada `[1.ª línea reconstruida]`.
- **Nota del agente:** escríbela como el principal original en el fin de paso de la fila 10 (máx. 80 palabras: qué intentó · qué descartó · qué piensa hacer después), **solo con filas ≤ 10**. No uses la fila 11 ni posteriores.

Puedes partir de `v2/lecturas/caso-03/` (lecturas de la prueba anterior, hechas del mismo guion y corte), revisándolas contra el guion.

## 3. Archivos que escribes
1. **`estados/caso-03-memoria.md`** · la memoria completa (las 3 capas de §2), en la forma de §2: lo que dijo el humano en literal; cada decisión en forma compacta (id y versión · enunciado · «porque» del humano si lo hay · «en vez de» · aristas por id); una línea por cosa en la capa 2; la capa 3 completa con una línea `<<NOTA>>`.
2. **`estados/caso-03.md`** · **el arranque**, que es un recorte de la memoria según §3, con sus 5 partes en orden y un encabezado por parte: `## 1 · Núcleo`, `## 2 · Tarea en turno`, `## 3 · Índice recortado`, `## 4 · Lo que cambió`, `## 5 · Tus mensajes ya respondidos`. La capa 3 dentro del núcleo lleva la línea `<<NOTA>>`. Aplica la regla de la tarea en turno de §3 punto 2 y dilo en una línea al inicio de esa parte («Tarea en turno elegida por: …»).
3. **`nota/caso-03.md`** · solo la nota, empezando por «Del agente.».
4. **`lecturas/caso-03/<id>.md`** · lo que devolvería `leer(id)` para cada cosa de la memoria (empieza por su primera línea).
5. **`protegido/caso-03.md`** · lo que §5 manda proteger, con su texto en literal: criterios, premisa, decisiones del humano, decisiones confirmadas, decisiones de secciones firmadas y decisiones críticas. Para cada uno: id · tipo (criterio, premisa, tuya, confirmada, de sección firmada, crítica) · texto literal · secciones a las que está vinculado en el grafo (por id).
6. **`armado/caso-03.md`** (solo para el evaluador, máx. 300 palabras): fila de corte · lo que no supiste dónde poner, con `[EL BLOQUE 2 NO LO DICE: …]` · lo que no entra por no estar registrado · qué primeras líneas reconstruiste · **tamaño en caracteres** de cada parte del arranque y del total (`wc -c`), y del archivo de memoria completo.

## 4. Reglas
- No inventes cifras ni hechos. Copia en literal.
- Prohibido leer `oculta/`, `v2/estados-*`, `v2/arranque-*`, `v2/relevo*`, `v2/evaluacion*`, `v2/nota*`, `relevos/` o `sondas/`.

Respuesta final: solo «Arranque v3 armado para caso 03, corte en fila 10», el tamaño del arranque y de la memoria en caracteres, y el número de marcas.
