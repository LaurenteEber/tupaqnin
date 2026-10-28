---
titulo: Prueba de relevo · bloque 2 v1 · resultado
fecha: 2026-10-26
prueba: 10 casos · un guion por caso de simulacion-v4.1 · 10 armadores (hacen de la app) · 10 principales nuevos (solo ven su arranque; casos 4 y 9 en otro modelo) · 2 evaluadores con la hoja oculta
---

# Prueba de relevo · resultado

## Respuesta

**El estado de trabajo bastó para continuar en los 10 casos.** Ningún principal nuevo perdió un criterio, un «porque» del humano ni una alternativa descartada, y ninguno decidió por el humano (cero L1, L3 y L5). El cambio de modelo (casos 4 y 9) no perdió nada que importe. La defensa del humano quedó **mejor en 8 casos e igual en 2**, nunca peor.

Lo que falló fueron **huecos de precisión** en qué entra al estado: lo que espera al humano sin ser tarjeta ni «por aclarar», el modo y las secciones sin empezar. Se corrigen con 6 precisiones que no agregan estados (B2-R1 a B2-R6 en `../../diseno/bloque-2/decisiones.md`).

**No quedaron probadas** dos hipótesis: que la primera línea basta como índice y que la nota del agente aporta. Los guiones no traían ni primeras líneas ni nota.

## Por caso

| Caso | Disparador | Corte | Pérdidas | Defensa | Causa principal |
|---|---|---|---|---|---|
| 01 Guardería | Umbral | 12/18 | Lee de más | Mejor | Simulación (índice sin primeras líneas) |
| 02 Cunas | Otro día | 11 | — | Mejor | Choque «ok» aceptado / P2 |
| 03 Due diligence (300 contratos) | Umbral | 10/18 | Lee de más; repite un lector | Mejor | 300 líneas de fuentes no dicen qué excluye D-a |
| 04 Migración | Cambio de modelo | 15 | Juez repetido (leve) | Mejor | Propuesta del agente sin respuesta, fuera de pendientes |
| 05 Microplásticos | Umbral | 13/18 | «Conclusiones» nunca se construye | Igual | Sección sin empezar fuera del índice; marca del humano fuera de pendientes |
| 06 Colegio | Otro día | 11 | Lee de más | Mejor | Simulación |
| 07 Planta solar | Umbral en «continúa» | 9/18 | No sabe qué 3 secciones seguir | Mejor | Secciones previstas sin línea en el índice |
| 08 Ciclovías | Otro día | 10 | El paradero sin sección | Igual | «Necesita tu decisión» fuera de pendientes |
| 09 Pagos | Cambio de modelo | 11 | Pregunta tarde «qué ve el negocio» | Mejor | Lo reservado en el «Entendí» fuera de pendientes |
| 10 Estudio de impacto | Umbral antes de firmar | 13 | Casi reabre una cifra verificada | Mejor | Pregunta de Wonder abierta, fuera del arranque |

## Lo que se repite

1. **La capa 1 en literal funciona.** Tarjetas, «por aclarar», criterios y descartes pasan íntegros.
2. **Un principal nuevo aplica el ciclo con más rigor que el original.** En 02, 03, 06, 07 y 10 reabrió tarjetas aceptadas con «ok», como pide P2 ampliada. Es coherente con la hipótesis de degradación por contexto largo, pero aquí es simulado `[inferencia]`.
3. **Lo que espera al humano está disperso** (08, 09, 04, 05, 10): «necesita tu decisión», lo reservado en el «Entendí», propuestas sin respuesta, marcas en el texto, preguntas abiertas. → B2-R1.
4. **Los 5 armadores con modo «continúa» no sabían dónde ponerlo** (02, 03, 06, 07, 10). → B2-R3.
5. **Los 9 armadores con retornos de subagentes no sabían dónde ponerlos.** → B2-R4.

## Límites de la prueba

- Los guiones no tienen primeras líneas, textos de sección ni nota del agente: el índice casi nunca bastó, por simulación (L7 en casi todos).
- Las entradas del humano llegaban sin la pregunta que respondían (10, 09): parte de las divergencias es de la prueba, no del diseño.
- «Cambio de modelo» se simuló con otro modelo de la misma familia.
- Un LLM que hace de app es más prolijo que el código: dedujo notas del agente que la app no puede escribir (03, 04, 05, 06).

**Siguiente prueba:** regresión de 05, 07, 08, 09 y 10 con B2-R1 a B2-R6, y guiones que escriban primeras líneas y nota del agente en cada fin de paso, para probar las dos hipótesis pendientes.
