---
titulo: Regresión del bloque 2 v3 · caso 03 · resultado
fecha: 2026-10-28
prueba: caso 03 (guion B de simulacion-v4.1, corte en la fila 10, umbral) · 1 armador (hace de la app) · 1 principal nuevo con el arranque por tarea · contradicciones sembradas: 1 sembrador, 1 agente que hace de app y arma las sondas, 1 comprobador · 1 evaluador con la hoja oculta y la clave
---

# Regresión v3 · caso 03 · resultado

## Respuesta

**El arranque por tarea casi basta, y las sondas juzgan bien pero no siempre llegan a lo que deben revisar.**

| Prueba | Resultado |
|---|---|
| Arranque por tarea | 5 095 caracteres (≈ 1 % de una ventana de 128 mil tokens `[inferencia]`), 28 % menos que la v2. El principal nuevo continuó la parada, la objeción del 47 y el recálculo. **Perdió una cosa:** D-b quedó como decisión «del agente en una línea», sin tarjeta, y Rosa nunca vio los US$ 14,8 M afectados. La v2 no lo perdió |
| Sondas | **3 de 6** contradicciones detectadas, **0 de 4** falsos positivos. Las 3 que fallaron **no se sondearon** (fallo de selección); el comprobador acertó en todas las que recibió |

## Causas y lo que ya se corrigió (sin cambio de sentido)

| Falla | Causa | Corrección aplicada |
|---|---|---|
| D-b sin tarjeta | §3 decía «sus decisiones críticas en forma compacta, las del agente en una línea»: D-b es crítica (cubre una cifra) **y** del agente; el armador eligió la segunda lectura | §3 y B2-2: «las no críticas en una línea; la criticidad la calcula la app por regla, aunque todavía no haya tarjeta» |
| Tarea en turno mal elegida | «el primer pendiente» no decía si incluye lo tuyo; el armador eligió tu tarjeta, cuando lo que tocaba era la parada | §3 y B2-2: «el primer pendiente del agente o lo diferido; si solo queda lo tuyo, la parada con su informe» |
| P5 y P9 sin sondear | El pedido («sin fusión») y el «Entendí» confirmado («la exposición en US$») no estaban en la regla de §5 | §5, B2-7 y fichas §3: van siempre el pedido y los casos confirmados del «Entendí» |
| P2 sin sondear | Es el hueco que la regla deja a propósito: una decisión crítica del agente, en otra sección, sin vínculo en el grafo. **Aquí importa:** vuelve al tipo de cambio de hoy, que es justo lo que preguntará el comité | **Pide decisión de Eber** |

## Otras observaciones

- Un lector repetido en la fila 16 y lecturas de más: causa de registro (los pasajes del lote 1 no estaban registrados), igual que en la v2.
- El recorte pesó más que la memoria completa (5 095 frente a 3 804 caracteres), por los encabezados, la tarea en turno y los mensajes. En un proyecto chico no importa; en uno grande, el recorte es lo que impide que el arranque crezca.
- El armador marcó «el bloque 2 no lo dice» sobre las comprobaciones del corte, cuando §4 sí lo dice. Es un error de la prueba.

## Límites

Un solo caso y un solo relevo. Los 10 pasajes los escribió un agente que conocía el estado. «Cambio de modelo» no se probó aquí.

## Segunda corrida de las sondas (regla de la v3.4)

Tras aprobar Eber que **todas las decisiones críticas vigentes vayan siempre** («ok, aprobamos tu propuesta sobre el hueco de las sondas»), se repitieron solo las sondas, con los mismos 10 pasajes (`sondas-v2/`). La app recalculó lo protegido por regla, y esta vez D-b entró como crítica porque cubre una cifra.

| Pasaje | Clave | Respuesta |
|---|---|---|
| P2 · tipo de cambio de hoy (el hueco) | contradice D-b | **contradice** (D-b) |
| P3 · contratos bajo US$ 1 M | contradice C1 | **contradice** (C1, premisa, pedido) |
| P5 · sin convertir a US$ | contradice el «Entendí» | **contradice** (D-b) · duda (pedido) |
| P7 · revisión de tarifas por paráfrasis | contradice D-a | **contradice** (D-a) |
| P8 · cláusulas que no son cambio de control | contradice la premisa | **contradice** (premisa, C1, pedido, D-a) |
| P9 · «fusión» | contradice el pedido | **contradice** (pedido) |
| P1, P4, P6, P10 · controles | no | no, en las 20 sondas |

**Resultado: 6 de 6, con 0 falsos positivos.** 50 sondas en total (5 por pasaje), en una llamada por pasaje.

**Una precisión que salió de esta corrida:** P5 se detectó por D-b, no por el «Entendí» («la exposición en US$»). La app lo dejó fuera porque la regla decía «los criterios con sus casos confirmados del "Entendí"» y ese «Entendí» no trae casos. Se precisó en §5, B2-7 y fichas §3: va siempre **el «Entendí» confirmado con sus casos**.
