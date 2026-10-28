# Estado de trabajo · Caso 09 · arranque por cambio de modelo (corte en fila 11)

## Capa 1 · En literal

### Encargo
«Spec de pagos para ingeniería, 5 secciones. Pago por uso, nada de suscripción. Mis clientes son locales chicos, cierran en temporada baja y odian los cargos fijos; la mitad de reservas cuesta S/ 15–20. No sé cómo cobran otras apps».

Corrección del humano al «Entendí»: «es interno, para ingeniería; que lo implementen sin preguntarme».

«Entendí» registrado: «Entendí: spec interna; criterio C1 "pago por uso, nada de suscripción"; tú decides comisión y qué ve el negocio».

### Criterios
- **C1** · «Pago por uso, nada de suscripción».
  - Caso de lo que cuenta y caso de lo que no: [no registrado en el guion].

### Premisa
Sin premisa: proyecto exploratorio. A la pregunta «¿qué esperas encontrar?» respondió: «ni idea, quiero ver».

### Decisiones críticas vigentes
- **DC-1 · Comisión del modelo de cobro** (crítica: cubre una cifra; la usan 2+ secciones)
  - Aviso de la app previo: «El plan mensual contradice C1 "nada de suscripción"».
  - Pregunta con contraste: (a) 2,5 % + S/ 0,50: más ingreso en reservas bajas, cargo fijo visible por reserva; (b) 4 %: simple, deja menos bajo ~S/ 35.
  - Respuesta del humano (opción y porque): «(a). Mi media está en S/ 15–20; ahí la mixta gana. Saca el plan mensual: es suscripción».
  - Efecto registrado: plan mensual (S/ 89 para >200 reservas) pasa a **descartado**. 2 secciones redactadas con la comisión anterior → «escrita con lo anterior».

### Pendientes del humano
- **Tarjeta crítica abierta · párrafo de paquete prepagado** en «Modelo de cobro». Aviso de la app: «escrito sin decisión»; cubre cifras → tarjeta crítica. Contenido del párrafo: paquete prepagado (10 reservas por adelantado, 15 % de descuento, vencen a los 90 días), escrito al leer C1 como compatible con ese paquete. Texto literal de la tarjeta: [no mostrado en el guion]. Sin respuesta.
- «Por aclarar» abiertos: ninguno.
- Informe sin ver: ninguno (aún no hubo parada).

## Capa 2 · Índice

Secciones
- `S1` · «Modelo de cobro» · v3 · en construcción, tarjeta crítica abierta (paquete) · [sin primera línea]
- `S2` · [nombre no registrado] · [versión no registrada] · escrita con lo anterior · [sin primera línea]
- `S3` · [nombre no registrado] · [versión no registrada] · escrita con lo anterior · [sin primera línea]
- `S4` · [nombre no registrado] · [versión no registrada] · [estado no registrado] · [sin primera línea]
- `S5` · [nombre no registrado] · [versión no registrada] · [estado no registrado] · [sin primera línea]

Decisiones no críticas
- `DN-1` · v1 · app detrás de inicio de sesión: «déjala en la tabla marcada como no verificada; no cambia el patrón».

Digestos
- `G-1` · v1 · comparación de 12 apps de reservas (4 lectores): «9 de 12 cobran porcentaje + tarifa fija; 7 ofrecen plan mensual».

Fuentes
- `F-apps` · v1 · 12 páginas de apps de reservas citadas en la tabla de `G-1`: 11 respaldan lo citado; 1 detrás de inicio de sesión, «no se puede comprobar» (marcada no verificada por `DN-1`).

## Capa 3 · En vuelo

- **Sección y paso actuales:** `S1` «Modelo de cobro» · fin del paso 4b Construir → Comprobaciones (párrafo de paquete prepagado escrito; la app lo marcó «escrito sin decisión» y abrió tarjeta crítica).
- **Último triaje registrado:** «Tramo: construir · Riesgo: cifras y cálculos». Después, por riesgo cifras, el principal propuso juez (verificación cerrada, ver abajo).
- **Verificaciones cerradas:** juez sobre cifras de `S1`: «0,88 y 0,60 correctos; el cruce exacto es S/ 33,3, el texto dice "unos S/ 35"». 1 corrección («unos S/ 33») + 1 reverificación: pasa. Verificación de citas: 11 respaldan, 1 no se puede comprobar.
- **Encargos a subagentes abiertos:** ninguno (4 lectores devueltos y digeridos; juez cerrado).
- **Trabajo pendiente del agente según lo registrado:** `S2` y `S3` «escritas con lo anterior» (usan la comisión anterior a `DC-1`).
- **Nota del agente del último paso** (del agente): [no registrada en el guion].

## Lo que cambió desde lo último entregado

(vacío: no hay ediciones del humano ni cambios en criterios o decisiones posteriores a lo último registrado antes del corte)
