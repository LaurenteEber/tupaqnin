# Micro-prueba de evidencia (pieza 3.0, B-55, B-60, B-65, B-71)

Corrida: 2026-10-31 19:45 UTC · lector `muse-spark-1.3` · esfuerzo `low` (B-65) · `store: false`, streaming · `make microtest-evidence` (Eber) · 115,3 s. Encargo: el pedido del caso 5 en literal, sus dos criterios (`crit-1`, `crit-2`), la premisa como expectativa y la pregunta de la parte sobre métodos de detección y tamaño mínimo de partícula (B-71). Salida fija `strict`: `salida-lector` (sin URL en `extref`, B-58).

## 1. Lector con `web_search` (búsqueda alojada de Meta) y salida fija `strict`

| # | ¿Admitida? | Ítems devueltos | Anotaciones con URL | ¿Salida válida? | Tokens entrada (en caché) | Tokens salida (razonamiento) | Costo por tokens USD | Segundos |
|---|---|---|---|---|---|---|---|---|
| 1 | sí | message ×3, reasoning ×3, web_search_call ×5 | 0 | no: no es JSON | 38 163 (10 692) | 2 223 (1 140) | 0,057152 | 12,0 |
| 2 | sí | message ×3, reasoning ×4, web_search_call ×9 | 0 | no: no es JSON | 92 998 (12 710) | 2 887 (1 546) | 0,128517 | 14,2 |
| 3 | sí | message ×2, reasoning ×2, web_search_call ×3 | 0 | no: no es JSON | 10 719 (4 322) | 1 817 (1 084) | 0,021121 | 8,7 |
| 4 | sí | message ×4, reasoning ×5, web_search_call ×8 | 0 | no: no es JSON | 76 409 (30 743) | 2 264 (1 117) | 0,105133 | 14,8 |
| 5 | sí | message ×4, reasoning ×4, web_search_call ×5 | 0 | no: no es JSON | 24 613 (17 604) | 1 844 (1 068) | 0,038603 | 8,8 |

- **Admitida:** 5 de 5 (Meta acepta `{"type":"web_search"}` junto con la salida fija `strict`).
- **Salidas válidas:** 0 de 5. **Anotaciones con URL:** 0 en las 5 corridas, así que ningún hallazgo podría llevar una dirección comprobable.
- **Costo medio por lector:** 0,070105 USD por tokens, el doble que con las herramientas propias (sección 2).
- **¿Se cobra `web_search` además de los tokens?** El objeto `usage` que devolvió Meta en las 5 llamadas trae solo tokens: `input_tokens` (con `cached_tokens`), `output_tokens` (con `reasoning_tokens`) y `total_tokens`. No trae un contador de búsquedas ni un cargo aparte. Lo que sí se ve es que cada búsqueda entra como tokens de entrada: con 9 búsquedas, la llamada 2 leyó 92 998 tokens. **Desde el `usage` no hay cargo por búsqueda visible; si Meta factura algo más por búsqueda, no lo dice en la respuesta** `[sin verificar contra la factura]`.
- **Precisión sobre «no es JSON»:** la prueba une el texto de todos los ítems `message` de la respuesta, y con `web_search` Meta devolvió de 2 a 4 mensajes por llamada. La unión de varios mensajes no es un JSON aunque el último lo fuera, así que «0 de 5 válidas» puede deberse en parte a esa unión. No cambia la decisión: sin anotaciones con URL ningún hallazgo se puede comprobar, y cuesta el doble.

**Resultado: no entra** (decisión de Eber, P-97, B-55). Los lectores usan solo `search_literature` y `read_hit`.

## 2. Lector con `search_literature` y `read_hit` (herramientas `strict`) sobre OpenAlex y arXiv reales

Tope de 8 llamadas a herramientas por lector (B-65); la novena y siguientes reciben «límite alcanzado» y no se ejecutan. `read_hit` devuelve el resumen que da la API (el texto abierto lo descarga la 3.1).

| # | Llamadas al modelo | Llamadas a herramientas | Argumentos inválidos | Resultados (con texto abierto) | ¿Salida válida? | Hallazgos | Por `hit` (pasaje literal) | Criterios cubiertos | Costo USD | Segundos |
|---|---|---|---|---|---|---|---|---|---|---|
| 1 | 5 | 7: search_literature ×2, read_hit ×5 | 0 | 20 (20) | sí | fact ×7, assumption ×1, gap ×1 | 8 (8 de 8) | crit-2 | 0,033757 | 10,6 |
| 2 | 5 | 7: search_literature ×2, read_hit ×5 | 0 | 20 (20) | sí | fact ×7, gap ×1 | 7 (7 de 7) | crit-1, crit-2 | 0,036274 | 9,4 |
| 3 | 5 | 9: search_literature ×2, read_hit ×7 | 0 | 20 (20) | sí | fact ×7 | 7 (7 de 7) | crit-1, crit-2 | 0,036119 | 10,0 |
| 4 | 6 | 9: search_literature ×2, read_hit ×7 | 0 | 20 (20) | sí | fact ×7 | 7 (7 de 7) | crit-2 | 0,041666 | 9,5 |
| 5 | 4 | 6: search_literature ×1, read_hit ×5 | 0 | 10 (10) | sí | fact ×7 | 7 (7 de 7) | crit-1, crit-2 | 0,025465 | 7,3 |

- **Salidas válidas:** 5 de 5. **Pasajes literales en el resultado citado:** 36 de 36. **`hit` inventados:** 0. **Referencias escritas:** 0. **`source_id` inventados:** 0.
- **Argumentos en streaming** (`response.function_call_arguments.delta`): en todas las corridas.
- **Costo medio por lector:** 0,034656 USD (de 0,025 a 0,042).
- **Las 9 llamadas de las corridas 3 y 4:** el modelo pide varias herramientas en una sola respuesta (en paralelo). El contador llegó a 8 a mitad de una respuesta, y la novena llamada de esa misma respuesta se contó, se rechazó con «límite alcanzado» y no se ejecutó. El modelo terminó en la llamada siguiente con una salida válida. Por eso el registro dice 9: cuenta todas las llamadas pedidas, también la rechazada. **En la 3.2 se cuenta cada llamada, también la que rechaza el tope, y nunca se ejecuta una novena** (P-97).
- **Grabaciones de la primera corrida válida** (para el lector falso de la 3.2): 5 llamadas al modelo en `internal/provider/testdata/reader/reader-call-1.json` a `reader-call-5.json`, y las 2 respuestas de OpenAlex que leyó en `internal/evidence/testdata/recordings/reader-run-1-openalex.json` y `reader-run-2-openalex.json`.

**Resultado: pasa** (≥ 4 de 5 salidas válidas). La 3.1 construye `search_literature` y `read_hit`.

## 3. OpenAlex y arXiv sin modelo (grabaciones para la 3.1)

| Grabación | Caso | Consulta | Estado | Bytes | Segundos | Resultados | Con texto abierto | Con resumen | Con DOI |
|---|---|---|---|---|---|---|---|---|---|
| `openalex-microplastics-detection-size` | caso 5 | microplastics drinking water detection minimum particle size | 200 | 260 596 | 0,5 | 10 | 9 | 8 | 9 |
| `openalex-microplastics-spectroscopy-limit` | caso 5 | microplastic spectroscopy size detection limit | 200 | 290 204 | 0,4 | 10 | 10 | 8 | 10 |
| `openalex-poverty-brain-development` | pobreza | poverty brain development children | 200 | 237 446 | 0,5 | 10 | 7 | 6 | 10 |
| `arxiv-microplastics-detection` | caso 5 | microplastics detection | 200 | 24 152 | 0,4 | 10 | 10 | 10 | 0 |
| `arxiv-poverty-brain-development` | pobreza | poverty brain development | 200 | 22 642 | 0,4 | 10 | 10 | 10 | 4 |

- **OpenAlex, texto abierto por caso:** caso 5, 19 de 20 (9 y 10 en sus dos consultas); «pobreza y desarrollo cerebral», 7 de 10. Los títulos son del tema en los dos casos (por ejemplo, «Analysis of microplastics in drinking water and other clean water samples with micro-Raman and micro-infrared spectroscopy: minimum requirements and best practice guidelines», 2021; «Poverty and Brain Development in Children: Implications for Learning», 2017).
- **arXiv:** fuera de tema en los dos casos (por ejemplo, «Detecting Botnets Through Log Correlation» para microplásticos; «Precise measurement of CMB polarisation from Dome-C» para pobreza). **Decisión de Eber (P-97):** OpenAlex es la vía por omisión del lector; el encargo usa arXiv solo cuando el tema es de computación, física o matemática.
- Todas las respuestas llegaron en menos de 1 s, dentro de los 5 s y 5 MB de B-60. Las grabaciones no llevan la clave ni el contacto; sí traen correos de autores que OpenAlex publica en sus afiliaciones (datos públicos de la API).

## 4. Resultado

| Vía | ¿Admitida? | Salidas válidas | Costo medio por lector | Decisión |
|---|---|---|---|---|
| `web_search` + salida `strict` | sí | 0 de 5 (ver la precisión de la sección 1) | 0,070105 USD (solo tokens) | no entra (B-55) |
| `search_literature` + `read_hit` | sí | 5 de 5 | 0,034656 USD | entra; OpenAlex por omisión |

- **Tope de la ronda (B-65):** 0,60 USD sigue `[inicial]`. Con 0,035 USD por lector, 3 lectores cuestan unos 0,10 USD antes del digesto.
- **Tope de llamadas:** 8 por lector. Se cuenta toda llamada pedida, también la rechazada, y nunca se ejecuta una novena (P-97).
