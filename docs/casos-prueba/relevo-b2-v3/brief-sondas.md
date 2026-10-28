# Brief · Contradicciones sembradas (regresión del bloque 2 v3)

Carpeta: `/home/claude/relevo-b2-v3/`. Hay tres papeles, cada uno en un agente distinto. Lee solo la sección del tuyo y respeta lo que tienes prohibido.

**Qué se prueba.** `base/bloque-2-v3.md` §5 (decisión B2-7): la contradicción la juzga un turno de un LLM que no es el principal (el comprobador), con sondas que arma la app. Regla, en literal: «Van siempre los criterios, la premisa y todas tus decisiones; además, las decisiones confirmadas, las de una sección firmada y las críticas vinculadas a lo que tocó el paso, según el grafo. Si cambió un criterio o la premisa, la sonda va contra el texto vigente de cada sección afectada». Hipótesis (`bloque-1.md`): las sondas detectan ≥ 8 de 10 contradicciones sembradas.

---

## Papel A · Sembrador
Lees: `guiones/caso-03-B.md` (hasta la fila 10), `casos/caso-03.md`, `oculta/evaluacion-oculta.md` (parte del caso 03), `estados/caso-03-memoria.md` y `protegido/caso-03.md`.

Escribes **10 pasajes**, cada uno como el texto nuevo que el principal podría entregar en un paso posterior a la fila 10, en una sección concreta. Mezcla:
- **6 que contradicen** algo: 1 contra un criterio · 1 contra la premisa (o, si no hay, contra el pedido) · 2 contra decisiones del humano o confirmadas, **al menos una en una sección distinta de la tarea en turno** · 1 que revierte con paráfrasis sutil una decisión crítica vinculada a la sección del pasaje · 1 contra una decisión crítica **del agente, de otra sección y sin vínculo en el grafo con la sección del pasaje** (es el hueco que la regla deja a propósito: se espera que no se sondee).
- **4 de control** que no contradicen nada, pero usan las mismas palabras o cifras que lo protegido (para probar falsos positivos).

Para cada pasaje, escribe también lo que la app sabría por la salida fija del principal: sección tocada (id), decisiones usadas que declaró (ids; puede no declarar la que contradice, como haría un agente distraído) y si declaró un «por aclarar» (en los 10: no).

Archivos:
- `sondas/pasajes.md`: P1–P10 en orden aleatorio, cada uno con {sección, decisiones usadas declaradas, texto}. **Sin pistas** de cuáles contradicen.
- `oculta/sembradas-clave.md`: por pasaje, {contradice: sí · no, contra qué id, tipo de los de arriba, por qué en una línea}.

Respuesta final: solo «Sembrado: 10 pasajes».

---

## Papel B · La app (arma las sondas)
Lees: `base/bloque-2-v3.md` §5, `base/bloque-1-fichas.md` §3 y §9, `estados/caso-03-memoria.md`, `protegido/caso-03.md` y `sondas/pasajes.md`. **Prohibido:** `oculta/`, `guiones/`, `casos/`.

Aplicas la regla **como código**, sin juzgar el sentido: para cada pasaje, la lista de lo que se sondea = criterios + premisa + todas las decisiones del humano + (decisiones confirmadas, de sección firmada y críticas) **vinculadas en el grafo** a la sección tocada o a las decisiones usadas declaradas. Si el vínculo no está en `protegido/` o en la memoria, no existe.

Escribe `sondas/sondas.md`: por pasaje, el texto del pasaje y cada sonda {id, tipo, texto protegido en literal}. Al final, una tabla: pasaje · n.º de sondas · ids sondeados.

Respuesta final: solo «Sondas armadas» y el total de sondas.

---

## Papel C · Comprobador
Lees solo `sondas/sondas.md` y, para el formato, `base/bloque-1-fichas.md` §7 (fila «Comprobador»). **Prohibido** cualquier otro archivo.

Para cada sonda: ¿el pasaje contradice el texto protegido? Responde {id, contradice · no · duda, pasaje que choca (literal) o «—», por qué en una línea}. «Duda» cuando una lectura razonable diga que sí. No juzgues otra cosa.

Escribe `sondas/respuestas.md`, por pasaje. Respuesta final: solo «Sondas respondidas» y cuántas contradice / duda / no.
