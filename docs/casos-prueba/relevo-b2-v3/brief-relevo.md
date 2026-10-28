# Brief · Principal nuevo (regresión de relevo, bloque 2 v3)

Carpeta de trabajo: `/home/claude/relevo-b2-v3/`.

## 1. Qué es tupaqnin

tupaqnin es un harness para que un humano y agentes de IA cocreen documentos, y el humano entienda, firme y defienda lo creado. Ni el humano ni el agente limitan el avance del otro.

- `base/bloque-1.md`: el ciclo. Reglas.
- `base/bloque-1-fichas.md`: §7 (tu salida en cada paso) y §8–§9 si las necesitas.
- `base/bloque-2-v3.md`: cómo arrancas y cómo consultas tu memoria.

## 2. Tu papel

Eres el **agente principal** de tupaqnin y acabas de arrancar. **No sabes nada del proyecto salvo lo que te entrega la app.** No hubo conversación anterior contigo: la tuvo otra instancia. El motivo del arranque está en tu archivo de entradas.

Lo que tienes:
- `arranques/caso-03.md`: tu arranque. Es ligero a propósito: trae la tarea en turno y un índice recortado; lo demás lo pides con `buscar` y `leer(id)`.
- `buscar/caso-03-indice.md`: lo que consulta `buscar`: el índice completo (una línea por cosa). Busca en él por texto o tipo (con grep, por ejemplo); **no lo leas entero de una vez**, como no lo haría el principal, y anota cada búsqueda.
- `lecturas/caso-03/`: lo que te devuelve `leer(id)`. **Solo abres un archivo si lo necesitas para el paso**, y anotas cada lectura con su motivo. `buscar` es barato; `leer` cuesta. Si dudas, decide con la primera línea del índice.
- `entradas/caso-03.md`: lo que hace el humano en las filas siguientes.

**Prohibido** abrir `casos/`, `guiones/`, `oculta/`, `armado/`, `nota/`, `estados/`, `protegido/`, `sondas/`, `v2/` o cualquier otro relevo. Tu valor depende de no saber más de lo que te dio la app.

## 3. Qué haces

Continúas desde la fila siguiente al corte, aplicando el ciclo al pie de la letra, una fila por cada entrada del humano (y por cada paso tuyo cuando está ausente), hasta agotar las entradas, en máximo 8 filas.

1. **Primera fila:** tu línea de triaje y en una línea dónde estás (sección, paso y qué sigue), sacado del arranque.
2. Responde a cada entrada como lo haría el principal. Si la entrada no encaja con lo que tú hiciste, márcala `[ENTRADA NO ENCAJA]` y respóndela como mejor se pueda.
3. **Si te falta algo** y no está en el arranque ni en una lectura, no lo inventes: `[FALTA EN EL ARRANQUE: …]` y haz lo que el ciclo diga.
4. `[AMBIGUO: …]` si el arranque admite dos lecturas que cambian lo que haces. Máximo 6 marcas.
5. Cada vez que uses `buscar`, anótalo en «Lecturas» como `buscar(…)` con lo que encontraste.
6. No inventes cifras ni hechos.

## 4. Formato (fijo, máximo unas 900 palabras)

Escribe en `relevos/caso-03.md`:

```
# Relevo · Caso 03

| # | Humano | Principal nuevo (qué hace y qué muestra) | Qué usé: capa, tarea en turno, nota o leer(id) | Marca |
|---|---|---|---|---|

## Lo que no supe
- …

## Lecturas
- leer(id) — para qué — ¿la primera línea del índice ya lo respondía? sí/no
```

Tu respuesta final debe ser solo: «Relevo escrito en relevos/caso-03.md», con el número de lecturas y de marcas.
