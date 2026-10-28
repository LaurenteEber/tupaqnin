# Reverificación · backlog v1.1

## (a) Estado de lo encontrado

**Hallazgos**
1. Arranque en cada mensaje · **resuelto**. B-7: «Si no hay una viva, el turno empieza con un arranque `new_session`». 0.10: «un segundo mensaje dentro de las 2 h no crea otro `startup`». Ver el problema nuevo A.
2. Reintento en `EndStep` · **resuelto**. 0.7: «`EndStep` no llama al modelo … quien corre el turno (0.10) vuelve a llamar una vez con el mensaje `RetryMessage(errors)`».
3. Falso o Meta · **resuelto**. B-6: «`TQ_PROVIDER` = `meta` (por defecto) o `fake`. Con `fake`, `META_API_KEY` no es obligatoria». `AGENTS.md` lo repite.
4. POST y stream · **resuelto**. B-1: «responde al instante … el stream reenvía todo desde el inicio del turno». 0.6: «un cliente que se conecta a los 20 s recibe también lo emitido antes».
5. Crear una sección · **resuelto**. 0.10: «`POST /api/projects/{projectID}/sections` → `{label, version}`».
6. Grabación para 0.6 · **resuelto**. 0.5: «una grabación hecha a mano, `stream-90s.json` (≈ 90 deltas a 1 s), para 0.6» y `DeltaDelay`.

**Dudas por pieza**
- 0.1 · **resuelto**: «Los criterios 1, 3, 4 y 5 los comprueba Eber».
- 0.2 · **resuelto**, las dos: «`TQ_CAP_PROJECT_USD` (3)…» y «la de los errores del proveedor va en 0.5».
- 0.3 · **resuelto**, las dos: «`created_at` de `project`, `version`, `event`…», «todas las tablas hijas llevan `project_id`» y «solo la tabla `schema_version`».
- 0.4 · **resuelto**: «`SupersedeEdge(edgeID string, ev EventInput) (Edge, error)`».
- 0.5 · **resuelto**: «`Request{ProjectID, TurnID, InstanceID string; …}`».
- 0.9 · caso de prueba **resuelto**: «`internal/memory/testdata/seed.go`: 1 pedido…». Contador **parcial**: «`TokenCounter` = la función `CountTokens` de 0.5». `CountTokens(ctx, model, input)` pide el modelo, y `Startup` no lo recibe. Corrección: sumar `model` a `Startup` o definir `TokenCounter func(ctx, input) (int, error)`, ya ligado al modelo.

## (b) Problemas nuevos

**A · 0.10 / B-7 · Adivinanza · Alta**
- B-7: «La conversación es la lista de ítems crudos de sus llamadas (`model_call`), que se reenvía en cada turno».
- Espec. §1, `model_call.raw_items`: «cada ítem de salida, tal cual (§7)».
- `model_call` solo guarda la salida. Para rehacer la conversación faltan las entradas: el arranque, los mensajes del humano, el mensaje del reintento y los cambios avisados.
- Corrección: decir de dónde salen las entradas. Por ejemplo, `model_call` guarda también los ítems de entrada de cada llamada, o se rehacen desde `human_message` y `delivery`.

**B · 0.10 · Adivinanza · Media**
- 0.10: «si cambió el modelo del principal, arranque `model_change`».
- Ninguna pieza dice dónde se fija el modelo del principal: `models.go` solo tiene constantes, no hay una variable en 0.2 y no hay un evento. Tampoco se dice contra qué se compara.
- Corrección: decir de dónde sale el modelo vigente (una constante o una variable `TQ_PRINCIPAL_MODEL`). Decir que se compara con el `model_and_effort` del último `startup` o `model_call` de la instancia.

**C · 0.5 / 0.10 · Adivinanza · Media**
- 0.5: «`NewFake(fixtures ...Fixture)` devuelve las grabaciones en el orden dado, una por llamada».
- Con `TQ_PROVIDER=fake`, la app que corre en el navegador (0.10) no dice qué grabaciones carga ni qué pasa cuando se acaban.
- Corrección: decir que con `fake` la app carga una lista fija de `testdata/` (por ejemplo, `principal-valid.json`), y que al acabarse repite la última o devuelve un `ProviderError`.
