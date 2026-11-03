---
titulo: Backlog de construcción · P1 y P2 de las pruebas con usuarios
estado: v0.1 · 3-nov · hallazgos 10 a 22 de pruebas-usuarios-2nov.md, sin el 18 (ya va en C.7) · propuesta para Eber, con D-5 a D-8 abiertas
relacion: sigue a backlog-cierre.md (C.1 a C.8); las piezas continúan desde C.9. Implementa pruebas-usuarios-2nov.md §2. Mismo formato y mismo «Retorno» que backlog-cierre.md. Tramo P1 va después de la pasada con Meta y antes del despliegue solo si hay tiempo; tramo P2 va después del hackathon
---

# Backlog · P1 y P2

**Respuesta:** 12 piezas en dos tramos.
- **Tramo P1 (C.9 → C.14), 6–8 h `[inferencia]`:** antes de firmar, la app te pone al día. Te muestra qué se decidió y por qué, qué cambió el agente por su cuenta y dónde leer todo. Cada traba que impide firmar trae su botón. No pierdes lo que escribes, no hay cifras rotas y la pantalla tiene menos ruido.
- **Tramo P2 (C.15 → C.20), después del hackathon:** la firma pregunta también por la tesis, objetar admite tu propia opción, se avisa de contradicciones entre secciones y hay pulido de textos y costos.

**Si el 3-nov no alcanza el tiempo:** construir solo **C.9, C.12 y C.10**, en ese orden (unas 3–4 h). Las tres tocan el camino del jurado hasta la firma. El resto del P1 espera.

**El hallazgo 18 no entra aquí:** el pie «Las decisiones críticas se toman en sus tarjetas» ya va en C.7.

**Regla que guía este backlog (criterio de Eber, 2-nov):** «la app debe darle los insumos para ponerse al día, formar el esquema mental y entender, y con ello recién firmar». La sigue el tramo P1. El tramo P2 refuerza lo que la firma comprueba.

**Presupuesto de complejidad (regla 2.4):**
- **Tramo P1:** 0 estados nuevos. 0 eventos nuevos: «Pedir corrección al agente» usa el paso de B-100, y «Leer completo» usa la pestaña Markdown que ya existe.
- **Tramo P2:** según D-6 a D-8, como máximo 1 evento nuevo (el aviso entre secciones, C.17) y 0 estados nuevos.

## Decisiones

| # | Propuesta | En vez de | Elegiría la otra si… | Decisión de Eber |
|---|---|---|---|---|
| B-103 | **Ponerte al día antes de firmar** (hallazgo 10). **(a)** El informe de parada suma dos apartados, armados por la app desde la historia y **sin modelo**: «Lo que se decidió y por qué», con cada decisión del tramo en su enunciado y su porque en literal; y «Lo que el agente cambió por su cuenta», con los pasos que cambiaron texto sin responder a una decisión ni a un mensaje tuyo, cada uno con su sección y «Ver cambios» (el «Sale / Entra» de B-99). **(b)** Un botón «Leer completo» abre la pestaña Markdown del documento. **(c)** En «Todavía no se puede firmar», cada pendiente trae su botón: ir a la tarjeta, responder el «por aclarar» o **«Pedir corrección al agente»**. Ese botón lanza **un** paso del agente sobre esa sección y ese pendiente (como B-100, con los topes de B-89; si hay un turno en curso, queda en cola, como en C.4) | Un resumen escrito por un modelo (cuesta, puede inventar y no se puede rastrear) · solo el botón «Leer completo» (no dice qué cambió ni por qué) | Las listas salen tan largas que nadie las lee; entonces resumir con un modelo, citando cada decisión | **D-5** · |
| B-104 | **Las potencias se escriben y se comparan bien** (hallazgo 14). El principal escribe las potencias con superíndice Unicode («× 10⁴»), por instrucción en `principal.en.md`. Al comparar cifras (la regla que detecta borrar una cifra citada), la app normaliza `10^4`, `10⁴`, `×10^4` y `1e4` a una misma forma antes de comparar. Corregir «104» a «10⁴» deja de contar como borrar una cifra | Texto enriquecido o LaTeX (cambia el editor y el markdown) | El documento necesita fórmulas, no solo potencias | **Se infiere** |
| B-105 | **Tu explicación de la firma se conserva entre intentos** (hallazgo 16). Se guarda en `sessionStorage` (`tq.sign.{proyecto}.{sección}`, con `try/catch`) mientras escribes. Se recupera al volver a la firma de esa sección y se borra al firmar | Guardarla en el servidor (una ruta y un dato nuevos, para un borrador) | Hace falta retomar la firma desde otro equipo | **Se infiere** |
| B-106 | **La firma pregunta también por la tesis de la sección** (hallazgo 11). Además de las decisiones críticas, Wonder pregunta por la tesis: la frase marcada «aquí se concluye» o, si no hay, la primera decisión que la sostiene, aunque no sea crítica. Las firmas de secciones distintas no repiten la misma decisión si la sección tiene otra | Mantener solo las críticas (hoy: Diego firmó tres veces sobre las 8 pm y nunca sobre el piloto) | Firmar se vuelve tan largo que la gente deja de hacerlo | **D-6** · P2 · cambia el diseño de la firma y la defensa |
| B-107 | **Objetar admite tu propia opción** (hallazgo 12, extiende B-81). El «por aclarar» de una objeción suma una tercera opción, «Escribe la tuya»: tu enunciado y tu porque crean una decisión tuya, que reemplaza a la objetada (con su versión, como cualquier decisión) | Solo «mantener» y «la otra» (hoy) | Las decisiones propias salen mal formuladas y obligan a más «por aclarar» | **D-7** · P2 |
| B-108 | **Aviso de contradicción entre secciones** (hallazgo 13). El comprobador recibe, además de su sección, las decisiones vigentes de las otras secciones (enunciado y porque, no su texto). Si el texto choca con una, deja un «por aclarar» que nombra las dos secciones | Revisar el documento completo con un modelo al firmar (más caro y tardío) · no avisar (hoy) | El costo por paso del comprobador sube más de un 30 % `[inicial]` | **D-8** · P2 |
| B-109 | **Decir que el agente ya empezó** (hallazgo 21). Se mantiene el diseño (`bloque-1.md`: confirmar el «Entendí» no bloquea). Mientras esté sin confirmar, la tarjeta del «Entendí» dice: «El agente ya empezó. Si corriges el Entendí, rehace lo afectado» | Bloquear al agente hasta que confirmes (va contra DEC-003: nadie bloquea) | — | **Se infiere** · P2 |

## Tramo P1 · si hay tiempo el 3-nov

### C.9 · Ponerte al día antes de firmar

| Campo | Contenido |
|---|---|
| **Qué** | El informe de parada dice qué se decidió y por qué y qué cambió el agente por su cuenta; puedes leer el documento completo con un clic; cada traba para firmar trae su botón |
| **Implementa** | Hallazgo 10 · B-103 · el criterio de Eber del 2-nov |
| **Leer** | `internal/cycle/stopreport.go` (el informe) · `internal/cycle/pending.go` y `signoff.go` (por qué no se puede firmar) · la historia del párrafo (`HistoryPanel.tsx` y su fuente en Go) · C.5 (el paso que aplica tu decisión, para reutilizarlo) · C.6 (el «Sale / Entra») · la pestaña Markdown (`markdown.ts`) |
| **Interfaz** | El informe suma `decided` (enunciado, porque, sección, quién) y `agent_changes` (sección, paso, resumen de una línea sacado del propio paso, enlace a su «Sale / Entra»), limitados al tramo desde el informe anterior · «Leer completo» en el informe y en la cabecera de la firma, que abre la pestaña Markdown · en «Todavía no se puede firmar», un botón por pendiente; «Pedir corrección al agente» llama a la misma función que C.5 con la tarea «corrige ‹pendiente› en ‹sección›» |
| **Listo cuando** | Go: el informe de un tramo con 2 decisiones y 1 paso por cuenta propia lista las 2 con su porque en literal y el paso; un paso que aplicó tu decisión no aparece como «por su cuenta» · Go: «Pedir corrección» lanza un paso, respeta `overCap` y con un turno en curso queda en cola · web: los dos apartados y los botones (en y es) · humo con el falso del caso 5: parar, leer el informe, pedir una corrección y ver que el pendiente se cierra |
| **No hacer** | Un modelo que resuma. Una pantalla nueva: va en el informe y en el aviso que ya existen |

### C.10 · Tu explicación de la firma no se pierde

| Campo | Contenido |
|---|---|
| **Qué** | Si sales de la firma o fallas un intento, al volver encuentras lo que habías escrito |
| **Implementa** | Hallazgo 16 · B-105 |
| **Leer** | El componente de la firma (P7: la firma ocupa la pantalla) · `signoff.go` (intentos) |
| **Interfaz** | Guardar en `sessionStorage` cada 1 s `[inicial]` mientras escribes; recuperar al abrir la firma de esa sección; borrar al firmar · nada se manda al servidor hasta que envías |
| **Listo cuando** | web: guardar, recuperar y borrar con `sessionStorage` (función pura, y que no falle si `sessionStorage` lanza) · humo: escribir, fallar un intento, volver y ver el texto |

### C.11 · La tarjeta no se mueve mientras escribes

| Campo | Contenido |
|---|---|
| **Qué** | Una tarjeta donde escribiste no se redibuja ni cambia de lugar, y su error aparece junto al botón que lo causó |
| **Implementa** | Hallazgo 15 |
| **Leer** | El componente de tarjeta en `web/` y su lista · el refresco del marco en `App.tsx` (`applySummary`) |
| **Interfaz** | **Primero diagnosticar** qué hace que la tarjeta se redibuje: una `key` que cambia, la lista que se reordena o el estado local que se pierde · después: el borrador de cada tarjeta vive fuera del componente (por id de tarjeta), la lista no se reordena mientras una tarjeta tiene texto escrito y el error se muestra junto a «Aplicar» |
| **Listo cuando** | web: con texto escrito en una tarjeta, un refresco del marco no borra el texto ni cambia su posición; el error aparece junto al botón · humo: escribir en una tarjeta mientras el agente trabaja |
| **No hacer** | Pausar el refresco del marco entero |

### C.12 · No se firma un documento vacío

| Campo | Contenido |
|---|---|
| **Qué** | «Firmar» está apagado, con su razón a la vista, si la sección no tiene texto |
| **Implementa** | Hallazgo 19 |
| **Leer** | `signoff.go` y `pending.go` (las razones para no firmar) · el botón «Firmar» |
| **Interfaz** | Una razón más en la lista de «Todavía no se puede firmar»: «Esta sección aún no tiene texto», con el botón «Pedir al agente que la escriba» (el de C.9, o un mensaje en cola si C.9 no está) · la regla vive en Go, no solo en la página |
| **Listo cuando** | Go: firmar una sección vacía se rechaza con esa razón · web: el botón apagado con la razón · humo: proyecto nuevo del jurado, «Firmar» apagado antes del primer paso |

### C.13 · Menos ruido en pantalla

| Campo | Contenido |
|---|---|
| **Qué** | Letra más grande, una sola nota del agente y un contador en las tarjetas |
| **Implementa** | Hallazgo 17 · U-4 (una sola familia tipográfica) |
| **Leer** | Los estilos base de `web/` · `layout.ts` · la nota del agente (dónde se duplica) · la lista de tarjetas |
| **Interfaz** | Letra base de 15–16 px `[inicial]`, con la escala ajustada desde ahí · la nota del agente aparece una sola vez (la del paso) · «Tarjeta 2 de 5» en cada tarjeta abierta |
| **Listo cuando** | web: la nota aparece una vez por paso; el contador · humo en Chrome a 1280 y a 1920 px, sin texto cortado ni desbordes |
| **No hacer** | Cambiar la familia tipográfica ni el orden de los paneles |

### C.14 · Las potencias no se rompen

| Campo | Contenido |
|---|---|
| **Qué** | Las potencias se escriben «× 10⁴», y corregirlas a mano no cuenta como borrar una cifra citada |
| **Implementa** | Hallazgo 14 · B-104 |
| **Leer** | `principal.en.md` · la regla de cifras en `internal/checks` (r2) |
| **Interfaz** | Una línea en las instrucciones del principal sobre las potencias con superíndice · una función `normalizeFigure` que la regla de cifras aplica antes de comparar |
| **Listo cuando** | Go: «3,14 × 104» corregido a «3,14 × 10⁴» no dispara la regla; `1e4`, `10^4` y `10⁴` son la misma cifra; cambiar «10⁴» por «10⁵» sí la dispara · la pasada con Meta muestra las potencias con superíndice `[inferencia]` |
| **No hacer** | Cambiar el editor a texto enriquecido |

## Tramo P2 · después del hackathon

Se construye después de que Eber decida D-6 a D-8. Lo que se puede probar construyendo queda como hipótesis (regla 2.3).

### C.15 · La firma pregunta por la tesis

| Campo | Contenido |
|---|---|
| **Qué** | La firma comprueba que entiendes la conclusión de la sección, no solo sus decisiones críticas |
| **Implementa** | Hallazgo 11 · B-106 (si Eber elige A en D-6) |
| **Leer** | `signoff.go` · `critical.go` · las instrucciones de Wonder para la firma y la defensa · fichas §2 (Firma) |
| **Listo cuando** | Go con salidas grabadas: una sección con tesis marcada recibe una pregunta sobre ella; dos secciones con decisiones distintas no reciben la misma pregunta · reverificar la regresión de la firma (R3′) |
| **Hipótesis** | Preguntar por la tesis no alarga la firma más de un intento en promedio. Se prueba con los 4 perfiles |

### C.16 · «Escribe la tuya» al objetar

| Campo | Contenido |
|---|---|
| **Qué** | Al objetar puedes escribir tu propia opción, que queda como una decisión tuya con su porque |
| **Implementa** | Hallazgo 12 · B-107 (si D-7 = A) |
| **Leer** | B-81 en `backlog-r4.md` · `openquestion.go` · `reason.go` (el porque que no acepta «ok») |
| **Listo cuando** | Go: la opción propia crea una decisión tuya, versionada, que reemplaza a la objetada; un porque vacío o «ok» se rechaza · web: la tercera opción |

### C.17 · Aviso de contradicción entre secciones

| Campo | Contenido |
|---|---|
| **Qué** | Si una sección contradice una decisión vigente de otra, aparece un «por aclarar» que nombra las dos |
| **Implementa** | Hallazgo 13 · B-108 (si D-8 = A) |
| **Leer** | Las instrucciones del comprobador (`checker.en.md`) · lo que recibe hoy el comprobador · la regla 4 de flujos §1 (avisos por aristas) |
| **Listo cuando** | Go con salidas grabadas: el caso de Rosa (Diagnóstico contra Recomendación) deja el «por aclarar» · se mide el costo por paso del comprobador antes y después |
| **Hipótesis** | Las decisiones (sin el texto) bastan para detectar el choque. Se prueba con el caso de Rosa y el caso 5 |

### C.18 · Bienvenida al entrar con un link

| Campo | Contenido |
|---|---|
| **Qué** | Quien entra con un link de invitación ve una línea que dice qué es tupaqnin y qué hacer primero |
| **Implementa** | Hallazgo 20 |
| **Leer** | `HomePage.tsx` · `home.ts` · el link de invitación en `backlog-entrega.md` |
| **Listo cuando** | web: con un link, la línea aparece; sin link, no · en y es |

### C.19 · El «Entendí» avisa que el agente ya empezó

| Campo | Contenido |
|---|---|
| **Qué** | Mientras no confirmes el «Entendí», la tarjeta dice que el agente ya trabaja y que rehace lo afectado si lo corriges |
| **Implementa** | Hallazgo 21 · B-109 |
| **Listo cuando** | web: el texto aparece solo sin confirmar (en y es) |

### C.20 · El costo de «Auditar» y un solo Deshacer

| Campo | Contenido |
|---|---|
| **Qué** | «Auditar» muestra cuánto cuesta antes de pulsarlo, y una edición se deshace con un clic |
| **Implementa** | Hallazgo 22 · B-79 (qué es una edición) |
| **Leer** | El botón «Auditar sección» · `model_call.go` (costos reales para estimar) · B-79 en `backlog-r4.md` |
| **Interfaz** | Costo estimado en el botón: la mediana de las auditorías del proyecto o un valor `[inicial]` · guardados de la misma sección a menos de 5 s `[inicial]` cuentan como una sola edición |
| **Listo cuando** | Go: dos guardados seguidos se deshacen con un clic · web: el costo en el botón · las pruebas de la regla 6 (edición sospechosa) siguen en verde |

## Horas `[inferencia]`

- **Tramo P1:** C.9: 2,5–3 · C.10: 0,5 · C.11: 1–1,5 · C.12: 0,5 · C.13: 1 · C.14: 1. **Total: 6,5–7,5 h.** El recorte mínimo (C.9, C.12, C.10) toma 3,5–4 h.
- **Tramo P2:** C.15: 2–3 · C.16: 1,5 · C.17: 2–3 · C.18: 0,5 · C.19: 0,25 · C.20: 1,5. **Total: 8–10 h.**

## Retorno de cada pieza

El de `backlog.md`, más una captura de la prueba de humo. C.11 reporta su diagnóstico antes del arreglo.
