# Reverificación · defaults técnicos T-1 a T-11

**Respuesta:** los 8 hallazgos quedaron resueltos. Las correcciones abren 6 problemas nuevos: uno de gravedad media-alta (T-5 abre tarjetas que estados §2 no prevé) y dos de gravedad media (cómo se declaran los `new-n` y el formato `D-x` que sigue en el diseño).

## (a) Los 8 hallazgos

| # | Estado | Prueba |
|---|---|---|
| 1 · T-5 | resuelto | espec. §1.5: «Se llama al leer y en cada transacción que cambia aristas, el rol de una sección o tu marca… compara el antes y el después»; §1.1 `human_marked_critical`; §1.3 `marked_critical` |
| 2 · T-3 | resuelto | espec. §3 regla 4: «ids provisionales `new-n` declarados en esta misma salida… nunca vale dentro del texto»; esquema `$defs.ref` y `new_id` |
| 3 · T-6 | resuelto | espec. §3: «no se rechaza por largo… Lo diferido no se recorta»; `note` del esquema sin tope (una nota de 1000 palabras pasa) |
| 4 · T-11 | resuelto | defaults T-11: «cada regla que aplica la app en Go… repite ítems crudos grabados de llamadas reales a Meta» |
| 5 · T-4 | resuelto | espec. §3 regla 2 (base_version, sin solaparse, de atrás hacia adelante, NFC, «dice cuántas veces aparece»); §1.1 `proposal` con `anchor · operation · base_version`; esquema `operation` + `if/then` |
| 6 · T-2 | resuelto | glosario §8 (prefijos); espec. §1 `label_counter`, «UNIQUE por proyecto, inmutable»; fichas §3: «Formulario F-40» no cuenta |
| 7 · T-1 | resuelto | fichas §3: «lista de asentimientos de tu idioma»; T-1 (5) «nunca se traduce»; glosario §9–§10; espec. §9 usa `frame(project)`, `seen_via = frame` |
| 8 · menores | resuelto | T-7 `2006-01-02T15:04:05.000Z`; T-8 «al abrir cada conexión»; T-10 «ni el cuerpo crudo de un error»; micro-prueba 0.8; encabezado v1.2 cuadra con las tablas |

Validé el esquema con `jsonschema` Draft202012: es válido. Acepta una salida completa y válida. Rechaza un `replace` con ancla vacía, la falta de `operation`, un `reviewed_unchanged` sin `reason` o con `reason` vacío, `D-04`, un id interno y `new-0`.

## (b) Problemas nuevos

1. **Media-alta · T-5 abre tarjetas que estados §2 no prevé.** Espec. §1.5 y T-5 dicen: «si una decisión pasa a crítica, abre su tarjeta», para cualquier decisión. Estados §2 la abre solo para «una decisión del agente». Así se abriría una tarjeta para una decisión tuya, para una confirmada o para la que acabas de marcar como crítica. **Corrección:** «si una decisión con `state = agent` pasa a crítica, abre su tarjeta; si es tuya o confirmada, no. En una sección firmada, pasa a `changed_since_signoff` en todos los casos».

2. **Media · Falta cómo se declaran los `new-n`.** Una sección nueva no tiene un campo que la declare: `section_id` admite cualquier `ref`. La unicidad no se exige: probé con `new-3` en una decisión y en un «por aclarar» a la vez, y pasa. Además, `supersedes`, `source_id`, `used_sources`, `change_id` y `used_decisions` aceptan `new-n`, aunque solo pueden apuntar a objetos que ya existen. **Corrección:** en espec. §3 regla 4: «una sección nueva se declara con su entrada en `proposed_roles` (K-2); cada `new-n` se declara una sola vez en toda la salida». En el esquema, sumar `$defs.label` sin `new` y usarlo en esos cinco campos.

3. **Media · Sigue el formato `D-x` / `D-04`.** Aparece en fichas §3, el archivo corregido: «la última entrega contradice D-x». También en fichas l.241 («según D-04»), en bloque-2.md l.41 y l.78, en bloque-2/decisiones.md B2-7 y en bloque-1-decisiones l.70 («(D-04, ids)»). Con T-2, `D-04` ya no es un código del proyecto. **Corrección:** cambiarlo por `dec-x` / `dec-4`.

4. **Baja-media · El glosario no tiene campos del esquema y de la especificación.** En §6, `new_text` aparece sin `operation` y `proposed_open_questions` sin `id`. En §1 faltan la tabla `label_counter` y el campo `label`. Su propia regla dice que lo que no está en el glosario no se usa. **Corrección:** sumar los cuatro.

5. **Baja · T-6 choca con bloque-2.md l.50.** Esa línea dice: «Núcleo: la capa 1 … más la capa 3. **Nunca se recorta.**». T-6 recorta la nota de la capa 3 a 80 palabras. **Corrección:** en bloque-2 §3, aclarar que «la nota entra en la capa 3 con sus primeras 80 palabras (T-6); es su forma, no un recorte por tope».

6. **Baja · La micro-prueba 0.8 se queda corta.** T-3 depende de `pattern` (`ref` frente a `new_id`), y el esquema también usa `anyOf`, `const` y propiedades opcionales. El respaldo en Go solo nombra `if/then` y `minLength`. **Corrección:** en defaults (0.8) y en la `description` del esquema: «toda palabra clave que el modo estricto de Meta no admita (`if/then`, `minLength`, `pattern`, `anyOf`, propiedades opcionales) se omite del esquema enviado y se valida en Go».
