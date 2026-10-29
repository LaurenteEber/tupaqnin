---
titulo: Handoff · 28-oct (d) · construcción en marcha, rebanada 1 lista para decidir
estado: 28-oct, noche · para continuar en un chat nuevo
relacion: sigue a _handoff-28oct-c.md. Reglas de trabajo: _reglas-de-trabajo.md (sobre todo 3, 7, 11 y 12)
---

# Handoff · 28-oct (d)

**Respuesta:** el diseño de lo que ve y hace el humano quedó cerrado: inventario terminado (v1.0), pantallas en boceto, cuentas y seguridad mínima adoptadas, caso de demo = caso 5. El agente de código terminó las piezas 0.1 a 0.8 de la rebanada 0. El backlog de la rebanada 1 está escrito y revisado, pero **espera 11 respuestas de Eber (B-8 a B-18)** antes de que el agente lo empiece.

## 1. Qué se hizo hoy (desde el handoff c)

| Tema | Resultado | Archivo |
|---|---|---|
| Inventario de funcionalidades | v1.0, terminado: 66 acciones, todas con rebanada o fuera; validado por un auditor independiente | `bloque-3/funcionalidades.md` |
| Huecos del inventario | B3-18 a B3-22 adoptadas: inicio, cuentas con usuario y clave, crear proyecto con el caso de demo, topes por link en dólares, nota del agente, seguridad mínima (10 puntos) | `bloque-3/bloque-3-decisiones.md` v2.9 |
| Propagado al diseño | Especificación §1.7 (links, cuentas, sesiones), §6 (acceso, topes, seguridad, vencimiento) y §8 (horas) · bloque-3 §4 · glosario v1.4 | `bloque-3/bloque-3-especificacion.md` v2.10 · `bloque-3/bloque-3.md` · `glosario.md` |
| Bloque 1 | K-17 adoptada (crear sección a mano, rol sin valor por defecto) · K-16 adoptada · fichas, estados y bloque-1 §6 alineados con H-3 | `bloque-1/` (decisiones, fichas v1.17, estados v1.11, bloque-1 v4.11) |
| Plan | H-1: **caso 5** como demo, meta: completo · H-2: despliegue público el 3-nov · H-3: rebanada de cada acción (pedir sensibilidad sí funciona) · H-4: rebanada «Entrega» · H-5: despliegue de prueba no público el 30-oct | `_plan-hackathon.md` v1.6 |
| Pantallas | 4 pantallas y 3 piezas (P1 a P7), con lo que lee cada una | `bloque-3/pantallas.md` v0.1 |
| Rebanada 1 | Arquitectura (módulos y rutas) + 8 piezas, revisión fría, una corrección y una reverificación | `../construccion/backlog-r1.md` v0.3 |
| Esquemas nuevos | «Entendí» y Wonder; los tres esquemas sin lookahead (Go no lo compila) | `bloque-3/salida-*.schema.json` |
| Supervisión | 0.1 a 0.8 revisadas; pendientes P-1 a P-14 | `../construccion/supervision.md` v1.4 |
| Reglas | 12: dos carpetas (en `tupaqnin/` solo se lee) · 11: el chat edita en el disco, en su lugar | `_reglas-de-trabajo.md` y su copia en el proyecto |

## 2. Estado de la construcción (carpeta `SideProjects/tupaqnin/`)

- **Hechas:** 0.1 a 0.8 (0.7 sin commit al cierre). La micro-prueba 0.8 pasó: con el esquema de transporte y `strict`, 10 de 10 salidas cumplen el esquema completo en Go; ≈ 0,008 USD por paso de juguete.
- **Siguen:** 0.9 (memoria y arranque) y 0.10 (editor y turno de punta a punta). Después, la rebanada 1.
- **Pendientes abiertos de la supervisión:** P-5 (permisos del volumen en Fly, en el despliegue de prueba) · P-6 y P-7 (aristas, se resuelven con B-8) · P-8 (tiempo máximo de la llamada a Meta) · P-9 (stream de un turno que no existe: 404) · P-10 (copiar `docs/` a la carpeta del agente) · P-13 (quitar `regexp2`, dependencia sin autorizar) · P-14 (registrar ítems diferidos de más de 30 palabras, en la 0.9).

## 3. Qué falta

**De Eber:**
1. **Responder B-8** (`../construccion/backlog.md`) **y B-9 a B-18** (`../construccion/backlog-r1.md`). Sin eso, el agente no puede empezar la rebanada 1.
2. Pasarle al agente el mensaje sobre la 0.7 (quitar `regexp2`, dejar `errors` en `output_retry`, ítems diferidos en la 0.9).
3. Copiar `docs/` y `AGENTS.md` actualizados a la carpeta del agente (P-10). Los esquemas cambiaron hoy.
4. Commit de todo lo de hoy en `tupaqnin_design` (muchos archivos; `funcionalidades.md`, `pantallas.md`, `supervision.md`, `backlog-r1.md` y los dos esquemas son nuevos).
5. Revisar `pantallas.md` (lo estaba haciendo) y, antes del primer push público, `docs/investigacion/11-hackathon-meta.md` (B3-11).
6. Revisar las instrucciones al principal que escriba el agente en la 0.10 (`internal/cycle/prompts/principal.en.md`).

**Del chat siguiente, en este orden:**
1. **Propagar las respuestas a B-8 a B-18:** al backlog (interfaces de 0.4 y de la rebanada 1), al glosario (`signoff_cancelled` si se adopta B-14; `title` de la sección si B-18) y a la especificación §1.1 (B-18).
2. **La plantilla del caso 5 (B-12),** en `docs/demo/caso-5/`: pedido, destinatario, criterio «solo de campo», artículos ficticios en MD con sus conteos (60 veces crudo, 1,5 veces con el mismo filtro), el cálculo de la comparación como fuente (F-1) y la cita inventada para la rebanada 3. Ajustar el caso al diseño vigente: el caso original usa 4 investigadores (el diseño, máximo 3 lectores), una «marca» de Paula en el texto (ya no existe: es un «por aclarar»), secciones ya escritas (el botón de demo solo llena el pedido) y dura una jornada (la demo, minutos).
3. **La reconciliación de horas:** la rebanada 1 suma 25–35 h `[inferencia]` y el plan le da dos días; «Entrega» suma 14–18 h; espec. §8 da 44–60 h solo del bloque 3. Hay que recortar o paralelizar.
4. **Supervisar 0.9 y 0.10** contra el backlog y su retorno (pedirle a Eber las capturas del retorno; revisar P-8, P-9, P-13 y P-14).
5. Más adelante: el backlog de la rebanada 2, con los bocetos que faltan (P6 informe de parada, el marco y la vuelta tras días).

## 4. Qué leer para retomar (en este orden)

1. `_reglas-de-trabajo.md`: 3 (Eber adjudica), 7 (una corrección y una reverificación), 11 (un escritor; **editar en el disco**), 12 (dos carpetas).
2. `../construccion/supervision.md`: cómo va el agente y qué queda abierto.
3. `../construccion/backlog-r1.md`: la rebanada 1, sus elecciones y su arquitectura.
4. `bloque-3/pantallas.md` y `bloque-3/funcionalidades.md`: qué ve y hace el humano.
5. `_plan-hackathon.md` §1 y §4: rebanadas, calendario y H-1 a H-5.
6. Si hace falta el porqué: `bloque-3/bloque-3-decisiones.md` (B3-18 a B3-22 con su detalle), `bloque-1/bloque-1-fichas.md` §2, §3 y §7, `bloque-3/bloque-3-especificacion.md` §1 a §6.

## 5. Cómo trabajar (aprendido hoy)

- **Editar en el disco, en su lugar,** con un script corto por la shell de la computadora. La copia de ida y vuelta (copiar a la nube y devolver) escribió versiones viejas varias veces, aunque Obsidian estaba cerrado. Después de escribir, comprobar a los 30–40 s que el cambio sigue ahí.
- **En `tupaqnin/` solo se lee** (regla 12): `git --no-optional-locks` con `GIT_OPTIONAL_LOCKS=0`, sin builds ni pruebas; `git status` igual antes y después. Al agente le habla Eber: el chat le prepara el mensaje.
- **Una propuesta que precisa otra no es una fila nueva:** se actualiza la original (pedido de Eber, 28-oct). Quitar las filas que no aportan.
- **Antes de propagar decisiones, contrastarlas con las ya tomadas** y proponer cómo resolver los choques.
- **Validar con un auditor independiente** lo que se declara terminado; verificar cada hallazgo contra su cita antes de aplicarlo.
- La clave de Meta vive en el llavero de macOS (`tupaqnin-meta-api`); `make microtest` la toma de ahí y lo corre Eber. Nunca en el chat ni en archivos.
- Eber hace los commits y los push. Nada se borra: se mueve a `_por_eliminar/`.
