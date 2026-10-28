# Revisión en frío · rebanada 0

## Hallazgos

**1 · 0.10 (y 0.9) · Contradicción · Alta**
- Backlog 0.10: «`POST /api/projects/{projectID}/messages` … guarda `human_message`, arma el arranque (0.9), llama al principal».
- `bloque-2.md` §3: «**Un solo arranque** para otro día (…), cambio de modelo o umbral». §4: «El principal conserva la conversación».
- Con el backlog, cada mensaje crea un arranque y una instancia nueva. Nada dice qué es una «sesión nueva» ni dónde vive la conversación entre turnos (¿ítems crudos de `model_call` reenviados?).
- Corrección: en 0.10, decir que hay arranque `new_session` solo si no hay una conversación viva, y definir cuándo la hay. Los demás turnos reenvían la conversación guardada.

**2 · 0.7 ↔ 0.5 · Dependencia rota · Alta**
- 0.5 «No hacer»: «Reintentos automáticos de la forma: los decide 0.7».
- 0.7: `EndStep(ctx, projectID, turnID string, output json.RawMessage)`, y «si no cumple: un reintento».
- `EndStep` recibe una salida, no un proveedor. No puede volver a llamar al modelo. Tampoco se dice qué se le envía al modelo en el reintento.
- Corrección: decir quién hace el reintento (por ejemplo, 0.10 lo hace cuando `StepResult` pide reintentar). Fijar el mensaje del reintento: los errores de validación, sin ids internos.

**3 · 0.10 (y 0.2) · Adivinanza · Alta**
- 0.10 «Listo cuando»: «Con el proveedor falso, en el navegador … · lo mismo con `-tags=meta` y Spark».
- `AGENTS.md` regla 10: `-tags=meta` es solo para pruebas. Regla 2: no se agregan opciones de configuración. 0.2: sin `META_API_KEY`, `Load` falla.
- Nada dice cómo elige el binario entre el proveedor falso y Meta, ni si con el falso hace falta una clave.
- Corrección: fijar el mecanismo en 0.2 o en 0.10 (una variable `TQ_PROVIDER=fake|meta`, o una etiqueta de compilación) y decir si la clave es obligatoria con el falso.

**4 · 0.10 ↔ 0.6 · Adivinanza · Media**
- 0.10: `POST …/messages` → `{turn_id}` «llama al principal …, cierra con `EndStep` … y emite por el streaming».
- 0.6: `GET …/turns/{turnID}/stream`.
- El cliente recibe el `turn_id` cuando el POST responde. Si el POST espera a que termine el turno, el stream se abre tarde y los `delta` se pierden. No se dice si el turno corre en segundo plano ni si el stream reenvía lo ya emitido.
- Corrección: una frase. «El POST responde al instante y el turno corre en segundo plano. El stream reenvía desde el inicio del turno», u otra regla que Eber elija.

**5 · 0.10 · Adivinanza · Media**
- 0.10: «creas un proyecto, escribes una sección». La interfaz solo trae `POST /api/projects` y `PUT …/sections/{label}` con `base_version`.
- Ninguna ruta crea una sección, y el `PUT` necesita una etiqueta y una versión que ya existan.
- Corrección: decir si `POST /api/projects` crea `sec-1` vacía (versión 0, `draft`) o agregar `POST …/sections`.

**6 · 0.6 ↔ 0.5/0.8 · Dependencia rota · Media**
- 0.6 «Listo cuando»: «90 s de streaming con el proveedor falso». Depende solo de 0.5.
- 0.5: `NewFake(fixturesDir)` lee `testdata/*.json`. 0.8 graba esas respuestas, «una con streaming», pero va después.
- No hay grabación con streaming cuando se hace 0.6. Tampoco se fija el formato de la grabación, cómo elige el falso cuál repetir ni a qué ritmo emite durante 90 s.
- Corrección: en 0.5, fijar el formato de la grabación y una opción de ritmo del falso (por ejemplo, demora por delta), con una grabación hecha a mano para 0.6. O hacer que 0.6 dependa de 0.8.

## Por pieza

- 0.1 · podría construirla sin adivinar: con 1 duda. Los criterios 1, 3 y 4 de §6 (CI verde, un solo commit, qué no se publica) dependen del commit de Eber, y la pieza prohíbe hacerlo. No los puedo comprobar.
- 0.2 · con 2 dudas: los nombres de las variables de los topes de gasto no están. La prueba del error del proveedor necesita `ProviderError`, que llega en 0.5.
- 0.3 · con 2 dudas: el diagrama no trae columnas de fecha ni `project_id` en las tablas hijas. `schema_version` (T-8) no está en el diagrama, que la pieza manda respetar.
- 0.4 · con 1 duda: la firma de `SupersedeEdge(...)` falta, y `edge` no tiene id propio.
- 0.5 · con 1 duda: `Request` no trae `ProjectID`, pero cada llamada debe escribir `model_call`, que pertenece a un proyecto.
- 0.6 · con 2 dudas (hallazgos 4 y 6).
- 0.7 · con 1 duda (hallazgo 2).
- 0.8 · sí.
- 0.9 · con 2 dudas: no se dice cuál es «el caso de prueba» del tope. `Startup` no recibe con qué contar tokens (B-5).
- 0.10 · con 4 dudas (hallazgos 1, 3, 4 y 5).
