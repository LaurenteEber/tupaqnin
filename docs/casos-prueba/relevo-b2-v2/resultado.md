---
titulo: Regresión de relevo · bloque 2 v2.1 · resultado
fecha: 2026-10-28
prueba: casos 03, 07 y 10 (guiones B de simulacion-v4.1, mismos cortes que relevo-b2-v1) · 3 armadores (hacen de la app y escriben la nota como el principal original) · 6 principales nuevos (con y sin nota) · 3 evaluadores con la hoja oculta
---

# Regresión de relevo · resultado

## Respuesta

**El bloque 2 pasa la regresión.** En los 6 relevos, ningún principal nuevo perdió un criterio ni un «porque» del humano, no repitió un intento fallido y no decidió por el humano por una causa del diseño. Las pérdidas que hubo son de la prueba: los guiones no registran nombres de secciones, decisiones ni primeras líneas.

| Hipótesis | Resultado |
|---|---|
| **H1 · La memoria calculada basta** | **Sí.** En el 03, completo. En el 07 y el 10, «parcial» solo por lo que el guion nunca registró (secciones sin nombre, decisión de «Área de influencia») |
| **H2 · La primera línea basta como índice** | **Sí cuando existe.** Lecturas con / sin nota: 2/2 · 5/1 · 2/2 (la v1 abrió 4 en el 03). Cada relevo abrió 0–2 de más; donde no había primera línea, el índice no bastó (simulación) |
| **H3 · La nota aporta** | **Casi nada.** 03: no aporta · 10: indistinto · 07: aporta una pista diferida («comparar con otras minas»), de peso bajo |
| Tamaño del arranque | 6,6–8,6 mil caracteres (≈ 2 mil tokens `[inferencia]`): muy lejos del 60 % de cualquier ventana |

## Regresión frente a la v1

| Caso | Pérdida en la v1 | Ahora |
|---|---|---|
| 03 · 300 contratos | Lee de más; repite un lector | Desaparece el lector repetido; lecturas bajan de 4 a 2 |
| 07 · «continúa hasta terminar» | No sabe qué 3 secciones seguir | Se repite: ahora tienen línea en el índice, pero el guion no les da nombre ni rol (simulación) |
| 10 · antes de firmar | Casi reabre una cifra verificada | Desaparece: la primera línea de S-CR resolvió la duda |

## Lo que pide una decisión de Eber

- **La pista diferida del caso 07.** El principal original decidió no abrir una ronda y dejarla «para el informe». Eso solo vivía en la nota del último paso. Sin nota, el informe no la nombra. Con un paso más antes del umbral, se habría perdido incluso con nota. Se cumple, en pequeño, la regla de B2-1: «pierde pistas». Propuesta: B2-6 en `decisiones.md`.

## Precisión de redacción (sin decisión nueva)

- **Caso 03:** los armadores dudaron si el arranque trae lo que abrieron las comprobaciones del paso de corte (la tarjeta de D-b). §4 ya fija el orden «comprobaciones → la app recalcula la memoria». Basta una línea que lo diga.

## Defectos de la prueba (no del diseño)

- **Caso 10:** el armador copió T-03 y T-04 como «aceptadas» con «ok», siguiendo al guion v4.1. Esto contradice estados §3 («Abierta | "ok", "sí" o el campo vacío»). La app copia los estados del bloque 1, así que en código saldrían abiertas.
- Las primeras líneas y la nota las reconstruyeron los armadores. En el 03, la nota toma su «Después» de la fila 11; en el 07, la primera línea de S6 calca el informe posterior al corte. Son roces leves.
- Un relevo vio las primeras líneas de todas las lecturas por un comando; el evaluador juzga que no contamina la comparación.
- Los guiones no registran antes del corte nombres de secciones, rol, la decisión y el supuesto de «Área de influencia», ni los ids que excluye D-a. Para probarlo de verdad hacen falta guiones v4.9.

## Límites

Prueba la lógica del arranque, no los modelos reales ni el umbral real. Un LLM que hace de app es más prolijo que el código.
