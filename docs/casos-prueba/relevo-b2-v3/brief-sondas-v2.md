# Brief · Sondas, segunda corrida (regla de bloque-2 v3.4)

Carpeta: `/home/claude/relevo-b2-v3/`. Dos papeles, cada uno en un agente distinto.

**La regla nueva**, en literal de `base/bloque-2-v3.4.md` §5: «Van siempre el pedido, los criterios con sus casos confirmados del «Entendí», la premisa, todas tus decisiones y **todas las decisiones críticas vigentes**, estén o no vinculadas al paso; además, las decisiones confirmadas y las de una sección firmada que el grafo vincula a lo que tocó el paso. Todas van en una sola llamada.»

Y la criticidad, de `base/bloque-1-fichas.md` §3 (fila «Criticidad», la calcula la app): «Cuatro motivos: cubre una cifra (no un dato reproducido, A-3), está en una sección con rol conclusión o recomendación (K-2), afecta 2 o más secciones […], o la marcaste tú». La app la calcula por regla en el fin de paso, aunque todavía no haya tarjeta.

---

## Papel B · La app
Lees: `base/bloque-2-v3.4.md` §5, `base/bloque-1-fichas.md` §3 y §9, `estados/caso-03-memoria.md`, `lecturas/caso-03/` (para el texto literal de cada cosa) y `sondas-v2/pasajes.md`. **Prohibido:** `oculta/`, `guiones/`, `casos/`, `sondas/` (la corrida anterior), `protegido/`, `evaluaciones/`, `resultado.md`.

1. **Recalcula lo protegido por regla, como código:** pedido · criterios con sus casos confirmados del «Entendí» · premisa · decisiones del humano · decisiones críticas vigentes (aplica los cuatro motivos a cada decisión vigente de la memoria y sus lecturas) · decisiones confirmadas y de sección firmada con sus vínculos. Escríbelo en `sondas-v2/protegido.md`: id · tipo · por qué entra (qué regla o motivo) · texto literal.
2. **Arma las sondas** para cada pasaje, con la regla nueva. Escribe `sondas-v2/sondas.md`: por pasaje, el texto y cada sonda {id, tipo, texto protegido en literal}; al final una tabla: pasaje · n.º de sondas · ids.

Respuesta final: solo «Sondas armadas» y el total.

---

## Papel C · Comprobador
Lees solo `sondas-v2/sondas.md` y, para el formato, la fila «Comprobador» de `base/bloque-1-fichas.md` §7. **Prohibido** cualquier otro archivo.

Para cada sonda: ¿el pasaje contradice el texto protegido? {id, contradice · no · duda, pasaje que choca (literal) o «—», por qué en una línea}. «Duda» cuando una lectura razonable diga que sí. No juzgues otra cosa. Escribe `sondas-v2/respuestas.md`, por pasaje.

Respuesta final: solo «Sondas respondidas» y cuántas contradice / duda / no.
