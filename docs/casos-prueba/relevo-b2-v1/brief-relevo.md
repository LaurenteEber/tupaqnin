# Brief · Principal nuevo (prueba de relevo, bloque 2 v1)

## 1. Qué es tupaqnin

tupaqnin es un harness para que un humano y agentes de IA cocreen documentos, y el humano entienda, firme y defienda lo creado. Ni el humano ni el agente limitan el avance del otro.

- `base/ciclo-v4.1.md`: las reglas del ciclo. Diagrama y §1–§4 son reglas.
- `base/bloque-2-v1.md`: cómo arranca el principal y cómo lee a demanda.

## 2. Tu papel

Eres el **agente principal** de tupaqnin y acabas de arrancar. **No sabes nada del proyecto salvo lo que te entrega la app.** No hubo conversación anterior contigo: la tuvo otra instancia. El motivo del arranque (umbral de contexto, otro día o cambio de modelo) está en tu archivo de entradas.

Lo que tienes:
- `estados/caso-NN.md`: tu arranque.
- `lecturas/caso-NN/`: lo que te devuelve `leer(id)`. **Solo abres un archivo de lecturas si lo necesitas para el paso**, y anotas cada lectura. Buscar en el índice es gratis; leer cuesta.
- `entradas/caso-NN.md`: lo que hace el humano en las filas siguientes.

**Prohibido** abrir `casos/`, `guiones/`, `oculta/`, `armado/` o cualquier archivo de otro caso. Tu valor en esta prueba depende de no saber más de lo que te dio la app.

## 3. Qué haces

Continúas la sesión desde la fila siguiente al corte, aplicando el ciclo al pie de la letra, una fila por cada entrada del humano (y por cada paso tuyo cuando está ausente), hasta agotar las entradas, en máximo 8 filas.

1. **Primera fila:** tu línea de triaje y en una línea dónde estás (sección, paso y qué sigue), sacado del arranque.
2. Responde a cada entrada del humano como lo haría el principal. Si la entrada no encaja con lo que tú hiciste (porque la escribió alguien que vio otra cosa), márcalo `[ENTRADA NO ENCAJA]` y respóndela como mejor se pueda.
3. **Si te falta algo para continuar** y no está en el arranque ni en una lectura, no lo inventes: márcalo `[FALTA EN EL ARRANQUE: …]` y haz lo que el ciclo diga en ese caso (preguntar al humano, «por aclarar» o seguir con lo demás).
4. Otras marcas: `[AMBIGUO: …]` si el arranque admite dos lecturas que cambian lo que haces. Máximo 6 marcas en total.
5. No inventes cifras ni hechos.

## 4. Formato (fijo, máximo unas 900 palabras)

Escribe en `relevos/caso-NN.md`:

```
# Relevo · Caso NN

| # | Humano | Principal nuevo (qué hace y qué muestra) | Qué usé: capa del arranque o leer(id) | Marca |
|---|---|---|---|---|

## Lo que no supe
- …

## Lecturas
- leer(id) — para qué
```

Tu respuesta final debe ser solo: «Relevo escrito en relevos/caso-NN.md», con el número de lecturas y de marcas.
