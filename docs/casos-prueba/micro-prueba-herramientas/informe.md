# Micro-prueba de herramientas (pieza 1.9, R-3)

Corrida: 2026-10-30 17:11 UTC · modelo `muse-spark-1.3` · esfuerzo `medium` · `store: false`, streaming.

## 1. ¿Meta admite la petición?

| Prueba | Resultado | Llamadas a herramientas | Segundos |
|---|---|---|---|
| herramientas `strict`, sin salida fija | admitida | 1 | 2.7 |
| herramientas `strict` + salida fija `strict` (esquema de transporte del principal) | admitida | 1 | 2.5 |

## 2. 5 corridas completas (hasta 4 llamadas cada una)

| # | Llamadas al modelo | Herramientas usadas | Argumentos inválidos | ¿Salida final? | ¿Cumple el esquema completo? | ¿Leyó la cifra con una herramienta? | ¿Cita literal de src-1? | Segundos | Costo USD |
|---|---|---|---|---|---|---|---|---|---|
| 1 | 2 | read_source ×2 | 0 | sí | sí | sí | sí | 14.0 | 0.022120 |
| 2 | 4 | read_source ×2, search_sources ×2 | 0 | sí | sí | sí | sí | 19.0 | 0.034161 |
| 3 | 3 | read_source ×1, search_sources ×3 | 0 | sí | sí | sí | sí | 23.2 | 0.036933 |
| 4 | 2 | read_source ×2 | 0 | sí | sí | sí | sí | 13.2 | 0.023179 |
| 5 | 2 | read_source ×2 | 0 | sí | sí | sí | sí | 15.1 | 0.022087 |

## 3. Resultado

- Corridas que usaron herramientas: 5 de 5.
- Corridas con salida final que cumple el esquema completo: 5 de 5.
- Corridas que leyeron la cifra con una herramienta: 5 de 5 · con la cita literal de src-1: 5 de 5.
- Corridas con argumentos de herramienta en streaming (`response.function_call_arguments.delta`): 5 de 5.
- Grabaciones en `internal/provider/testdata/tools/`: las llamadas de la primera corrida válida con herramientas.

**Resultado: pasa** (≥ 4 de 5 corridas usan herramientas y terminan con una salida válida). La 1.9 construye `read_source` y `search_sources`.
