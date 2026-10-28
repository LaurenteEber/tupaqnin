---
titulo: Bloque 3 · Decisiones de Eber
estado: v2.2 · 28-oct · B3-1 a B3-9 abiertas; B3-10 a B3-13 adoptadas (primer push)
relacion: decisiones de bloque-3.md (v2). Las P1–P7 del 25-oct vienen de bloque-3-plataforma-y-archivos.md §11
---

# Bloque 3 · Decisiones de Eber

*Cada propuesta con su contraste. La última columna es para tu respuesta en literal.*

## Nuevas (27-oct)

| # | Propuesta | En vez de | Elegiría la otra si… | Decisión de Eber (literal) |
|---|---|---|---|---|
| B3-1 | **El agente escribe solo entregando su paso con forma fija** (fichas §7). Tiene dos herramientas de lectura, `buscar` y `leer`; la app valida la salida y la aplica. Una sola puerta de escritura, que ya es la del bloque 1 | Siete herramientas tipadas de escritura (`edit_section`, `propose_record`, `open_act`, `judge_contradiction`…), cada una con sus validaciones | La micro-prueba muestra que Spark no devuelve con fiabilidad una salida estructurada grande (texto, decisiones y vínculos juntos), o un paso necesita tocar varias secciones a la vez | |
| B3-2 | **Mientras firmas una sección, los pasos del agente sobre ella llegan como propuesta,** igual que en una firmada. Lo que explicas es lo que está vigente. Tu propia edición cancela la firma en curso | Congelar una instantánea al abrir la puerta y firmar lo explicado aunque lo vigente cambie, para dejar la sección de inmediato en «cambió» | Las firmas tardan tanto que se acumulan propuestas, o el agente necesita seguir escribiendo esa sección mientras firmas | |
| B3-3 | **Acceso a la demo por invitación:** link con token para el jurado y quien tú invites; video y página del proyecto públicos. ≈ 4–5 h | Demo abierta con captcha, límites por IP y techo de proyectos. ≈ 8–10 h | Las Reglas Oficiales exigen una demo pública sin barrera, o el voto de la comunidad pesa en la evaluación | |
| B3-4 | **Explicación de la firma por texto en el hackathon;** la voz queda como Debería | Voz como Debe: captura en el navegador, transcripción de Meta, aviso de privacidad y borrado del audio. ≈ 2–3 h | Las reglas premian usar más herramientas de Meta, o explicar hablando hace la demo mucho más clara para el jurado | |
| B3-5 | **Espejo git a la hoja de ruta** | Espejo git como Debería (tu P1-b del 25-oct) | Sobran horas el 3-nov, o el jurado técnico valora ver la historia como commits | |
| B3-6 | **Cadena de huellas de la historia a la hoja de ruta.** En el hackathon la historia es solo de agregado por regla de la app | Una cadena de huellas por proyecto desde el primer cambio | Hay un canal de escritura fuera de la app (importar, sincronizar), que es lo que la cadena delata. No protege frente a quien opera el servidor | |
| B3-7 | **Exportar a una carpeta abrible en Obsidian, y «Ver como carpeta», como Debería,** solo si las rebanadas 0–2 están listas el 2-nov. Debe: un volcado del proyecto para ti, como respaldo | La exportación como el primer Debería, antes que las rebanadas 3–4 | El mensaje «tus archivos son tuyos» es parte central del video | |
| B3-8 | **Desplegar el esqueleto en la rebanada 0 (28-oct)** y no volver a desplegar desde la entrega hasta el fin del juzgamiento. El ensayo de restauración, el 2-nov | Desplegar el 26-oct (P7, ya vencido) | Las Reglas Oficiales piden la demo desplegada antes, o fijan otra fecha de juzgamiento | |
| B3-9 | **El marco como panel fijo a la izquierda, plegable** (tu idea del 27-oct). Cada decisión crítica es un ítem con un ícono según su motivo de criticidad (cifra, conclusión o recomendación, varias secciones, marcada por ti) y su enunciado; al desplegarla, una tarjeta con su contraste, tu «porque», tu explicación de firma y enlaces a la historia y al texto. La línea de vuelta va arriba del panel (especificación §9) | Una pestaña más dentro del panel derecho, junto al chat, las tarjetas y el informe | La pantalla del jurado es estrecha y no caben tres columnas: entonces, pestaña | |

## Primer push al repo público (28-oct)

*Pedido de Eber, en literal: «Hoy debemos realizar el primer push al repo público de tupaqnin. Analiza bloque 3 y define las especificaciones para una estructura básica del repo y ese sería el primer commit y push». Reglas del hackathon que aplican: «Repositories must be public» y «you must clearly identify what was newly developed during the Hackathon» (periodo: 27-oct a 4-nov). La especificación de estas cuatro decisiones está en `repo-estructura.md`.*

| # | Propuesta | En vez de | Elegiría la otra si… | Decisión de Eber (literal) |
|---|---|---|---|---|
| B3-10 | **Un solo repo público, `LaurenteEber/tupaqnin` (nuevo, creado el 28-oct), con el diseño y el código juntos.** El diseño sigue en `docs/`; el código va en la raíz (Go) y en `web/`. Lo nuevo del hackathon se identifica en `HACKATHON.md`: antes del 27-oct, investigación y diseño; desde el 28-oct, todo el código. El repo nuevo empieza con un solo commit, sin historia anterior | Un repo nuevo solo para el código, con el diseño aparte | Las reglas o el jurado piden que el repo tenga solo trabajo del hackathon, o algún documento de `docs/` no debe ser público | «ok, agrega en bloque-3 tu propuesta lo pusheo» (28-oct) |
| B3-11 | **Limpieza antes del push.** Dejar de versionar, sin borrarlos del disco, `.obsidian/`, `.pi/`, `.tmp-relevo/` y `Event Details.md`, y sumarlos al `.gitignore` junto con `data/`, `*.db`, `node_modules/`, `web/dist/` y `.env`. Revisar `docs/investigacion/11-hackathon-meta.md`, que habla de tu elegibilidad y de Ceplan. | Publicarlos tal cual | Alguno de ellos hace falta para construir o probar la app | «ok» (28-oct) |
| B3-12 | **Estructura del código, con nombres en inglés y documentación en español.** `cmd/tupaqnin/` (un binario: API y página) · `internal/store/` (SQLite, escritura condicionada, historia: especificación §1–2) · `internal/graph/` (consultas del grafo: marco, memoria, criticidad, sondas: fichas §9 y bloque 2) · `internal/cycle/` (fin de paso, comprobaciones, estados: bloque 1) · `internal/memory/` (arranque por tarea, tope, umbral: bloque 2) · `internal/provider/` (cliente de la Responses API de Meta y uno falso para pruebas: §7) · `internal/fingerprint/` (huella `tq-md/1`: §4) · `internal/server/` (HTTP, streaming, acceso: §6) · `web/` (React + Vite) · `deploy/` (Dockerfile y `fly.toml`). Cada paquete trae un `doc.go` que dice qué hace y qué parte del diseño implementa | Nombres en español, o una estructura plana | El código lo leerán solo hispanohablantes: el jurado es internacional | «ok» (28-oct) |
| B3-13 | **El primer commit es un esqueleto que compila y se prueba, sin lógica del ciclo:** `go.mod` (`github.com/LaurenteEber/tupaqnin`, Go 1.24, sin dependencias), `main.go` con `/healthz` y una página mínima embebida, una prueba de `/healthz`, `Makefile` (run, test, build), `.env.example`, CI de GitHub Actions con `go vet` y `go test`, `README.md` en inglés y español y `HACKATHON.md`. Tú haces el commit y el push | Subir solo carpetas vacías; o esperar a tener la rebanada 0 completa | La rebanada 0 está lista hoy mismo | «ok» (28-oct) |

## Del 25-oct que se mantienen

| # | Decisión | Decisión de Eber (literal, 25-oct) | Cambio |
|---|---|---|---|
| P2 | La base de datos es la autoridad | «Ok» | Ninguno |
| P3 | El expediente se guarda como registro y su `.md` se genera | «Ok» | Ninguno; aplica solo si entra B3-7 |
| P5 | Ves objetos, no carpetas | «Ok» | Ninguno |
| P6 | Hosting en Fly.io | «Ok» | Plan B: Railway |

**Reemplazadas:** P1 (git) por B3-5 · P4 (acceso) por B3-3 · P7 (fecha) por B3-8.
