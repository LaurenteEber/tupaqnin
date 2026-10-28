# Micro-prueba 0.8 · informe

Fecha: 2026-10-28 · modelo del principal: `muse-spark-1.3`, esfuerzo `medium`.

## Palabras clave del esquema en modo estricto

| Palabra clave | ¿La admite? | Detalle |
|---|---|---|
| `if/then` | no | error: código `http_400`, tipo `invalid_request_error`, param `schema` |
| `minLength` | sí | la llamada terminó bien |
| `pattern` | sí | la llamada terminó bien |
| `anyOf` | sí | la llamada terminó bien |
| `const` | sí | la llamada terminó bien |
| `propiedades opcionales` | no | error: código `http_400`, tipo `invalid_request_error`, param `schema` |

Las que no se admitan se omiten del esquema enviado y la app las valida en Go (B-2).

## El esquema completo, por variantes (sin strict)

| Variante | ¿La acepta? | Detalle |
|---|---|---|
| tal cual | sí | la llamada terminó bien |
| sin $schema ni $id | sí | la llamada terminó bien |
| sin if/then | sí | la llamada terminó bien |
| sin pattern | sí | la llamada terminó bien |
| sin $schema/$id, sin if/then y sin pattern | sí | la llamada terminó bien |

## El esquema completo tal cual, en modo estricto

Rechazado en modo estricto: código `http_400`, tipo `invalid_request_error`, param `schema`. El esquema de transporte (`provider.TransportSchema`) pone cada propiedad en `required`, vuelve anulables las opcionales y quita las palabras clave no admitidas; la app valida en Go contra el esquema completo (B-2).

## Diez llamadas de Spark con el esquema de transporte (strict)

| # | Resultado | ¿Cumple el esquema completo en Go? | Tokens entrada | Tokens salida | Costo USD |
|---|---|---|---|---|---|
| 1 | bien | sí | 2837 | 1793 | 0.011167 |
| 2 | bien | sí | 2837 | 1028 | 0.007915 |
| 3 | bien | sí | 2837 | 892 | 0.007337 |
| 4 | bien | sí | 2837 | 772 | 0.006827 |
| 5 | bien | sí | 2837 | 918 | 0.007448 |
| 6 | bien | sí | 2837 | 1256 | 0.008884 |
| 7 | bien | sí | 2837 | 1061 | 0.008056 |
| 8 | bien | sí | 2837 | 886 | 0.007312 |
| 9 | bien | sí | 2837 | 996 | 0.007779 |
| 10 | bien | sí | 2837 | 1566 | 0.010202 |

Cumplen: 10 de 10 · costo total de las que terminaron: 0.082927 USD.

## Grabaciones guardadas (internal/provider/testdata/)

- `microtest-valid.json`: la primera respuesta que cumplió
- `microtest-streaming.json`: la llamada con más deltas, con su ritmo medio entre deltas.
- `microtest-invalid.json`: respuesta en texto plano; cumple el esquema: no.

## Ventana de contexto por modelo

| Modelo | Ventana (tokens) | Fuente |
|---|---|---|
| `muse-spark-1.3` | 1048576 | https://dev.meta.ai/docs/models |
| `muse-wonder-1.0` | 1048576 | https://dev.meta.ai/docs/models |

La ventana alimenta el tope del arranque (≤ 15 %) y el umbral (B2-2, B2-3).
