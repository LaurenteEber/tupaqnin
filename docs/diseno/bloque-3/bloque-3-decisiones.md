---
titulo: Bloque 3 · Decisiones de Eber
estado: v2.5 · 28-oct · todas adoptadas: B3-3, B3-9 y B3-14 a B3-17 (las propuestas reescritas para que se lean solas; la antigua fila «en el hackathon» se dividió en B3-14 a B3-17)
relacion: decisiones de bloque-3.md (v2). Las P1–P7 del 25-oct vienen de bloque-3-plataforma-y-archivos.md §11
---

# Bloque 3 · Decisiones de Eber

*Cada propuesta empieza por la decisión en una frase. Después dice qué significa en la práctica. La última columna es tu respuesta en literal. **Regla (28-oct):** aquí solo se decide lo que es del bloque 3; lo que ya decidió otro bloque se cita al final y no se vuelve a plantear.*

## Adoptadas (28-oct)

| # | Propuesta | En vez de | Elegiría la otra si… | Decisión de Eber (literal) |
|---|---|---|---|---|
| B3-3 | **La demo se abre solo con un link de invitación.** Tú le das el link al jurado y a quien quieras; cada link permite crear pocos proyectos. El video y la página del proyecto sí son públicos. Cuesta ≈ 4–5 h | Una demo abierta a cualquiera, con captcha y límites por IP para frenar el abuso. Cuesta ≈ 8–10 h | Las reglas exigen una demo pública sin barrera, o el voto de la comunidad pesa en la evaluación. *Dato: existe el premio «Community Vote» (US$ 10 000), que se vota en la galería pública del evento* | «adoptamos propuesta» (28-oct) |
| B3-9 | **El marco va en un panel fijo a la izquierda, que se puede plegar.** Cada decisión crítica aparece como un ítem con un ícono que dice por qué es crítica; al abrirla, ves su contraste, tu «porque», tu explicación de firma y enlaces a la historia y al texto. Qué muestra el marco ya lo decidió el bloque 1 (M-1 a M-4); aquí solo se decide dónde va | Una pestaña más en el panel derecho, junto al chat, las tarjetas y el informe | La pantalla del jurado es angosta y no caben tres columnas | «adoptamos propuesta» (28-oct) |
| B3-15 | **Ver el proyecto como carpeta y exportarlo a una carpeta que abre Obsidian: solo si sobra tiempo.** Entra únicamente si las rebanadas 0 a 2 funcionan el 2-nov | Hacerlo antes que las rebanadas 3 y 4 | El mensaje «tus archivos son tuyos» es central en el video | «adoptamos propuesta» (28-oct, respondida sobre la fila que reunía voz, carpeta, git y despliegue) |
| B3-16 | **Guardar la historia también en git, y la «cadena de huellas», quedan para después del hackathon.** En el hackathon, la historia solo crece porque la app no permite otra cosa. La cadena de huellas sirve para delatar cambios hechos por fuera de la app, y en el hackathon no hay ningún camino por fuera | El espejo en git como algo opcional del hackathon (tu P1-b del 25-oct), y la cadena de huellas desde el primer cambio | Sobran horas el 3-nov, o el jurado técnico valora ver la historia como commits; o aparece un camino para escribir por fuera de la app (importar, sincronizar) | «adoptamos propuesta» (28-oct, respondida sobre la fila que reunía voz, carpeta, git y despliegue) |
| B3-17 | **Una vez entregado, la demo no se vuelve a desplegar hasta que termine el juzgamiento.** El ensayo de restaurar la base desde la copia se hace el 2-nov. La fecha del primer despliegue no se decide aquí: está en H-2 del plan | Seguir desplegando cambios durante el juzgamiento | Las reglas fijan otra fecha de juzgamiento, o piden la demo desplegada antes | «adoptamos propuesta» (28-oct, respondida sobre la fila que reunía voz, carpeta, git y despliegue) |

## Adoptadas · voz (28-oct)

| # | Propuesta | En vez de | Elegiría la otra si… | Decisión de Eber (literal) |
|---|---|---|---|---|
| B3-14 | **La voz entra solo si sobra tiempo, pero el repo queda preparado para ella desde ahora.** Sirve para dos cosas: explicar la firma hablando y conversar con el agente. En el hackathon todo funciona por texto. El repo trae desde el primer commit su lugar para la voz (`internal/voice/` y la entrada de texto o voz en `web/`; ver `repo-estructura.md`), para que sumarla no obligue a rehacer nada. Usaría la transcripción de Meta (Muse Voice Transcribe), con aviso de privacidad y sin guardar el audio | Voz obligatoria en el hackathon (≈ 2–3 h solo la firma), o dejarla fuera sin preparar el repo | Sobra tiempo: entonces se implementa. *Dato: la rúbrica pesa 25 % en el uso de herramientas de Meta, y Muse Voice Transcribe es una de ellas* | «La implementación de voz para firma y como interface para interacción con el agentes lo hacemos solo si nos sobra tiempo, pero debe estar en ruta de implementación (el repo debe estar preparado)» y «adoptamos propuesta» (28-oct) |

*Primer push al repo público (28-oct). Pedido de Eber, en literal: «Hoy debemos realizar el primer push al repo público de tupaqnin. Analiza bloque 3 y define las especificaciones para una estructura básica del repo y ese sería el primer commit y push». Reglas del hackathon que aplican: «Repositories must be public» y «you must clearly identify what was newly developed during the Hackathon» (periodo: 27-oct a 4-nov). La especificación de B3-10 a B3-13 está en `repo-estructura.md`.*

| # | Propuesta | En vez de | Elegiría la otra si… | Decisión de Eber (literal) |
|---|---|---|---|---|
| B3-10 | **Un solo repo público, `LaurenteEber/tupaqnin`, con el diseño y el código juntos.** El repo es nuevo (28-oct) y empieza con un solo commit, sin historia anterior. El diseño va en `docs/`; el código, en la raíz y en `web/`. `HACKATHON.md` dice qué existía antes del 27-oct (investigación y diseño) y qué se hizo durante el evento (todo el código) | Un repo solo para el código, con el diseño aparte | Las reglas o el jurado piden que el repo tenga solo trabajo del hackathon, o algo de `docs/` no debe ser público | «ok, agrega en bloque-3 tu propuesta lo pusheo» (28-oct) |
| B3-11 | **Antes de publicar, se dejan de versionar los archivos locales, sin borrarlos del disco.** Son `.obsidian/`, `.pi/`, `.tmp-relevo/` y `Event Details.md`; se suman al `.gitignore` junto con los datos y binarios de la app (`data/`, `*.db`, `node_modules/`, `web/dist/`, `.env`). Antes del commit conviene revisar `docs/investigacion/11-hackathon-meta.md`, que habla de tu elegibilidad y de Ceplan | Publicarlos tal cual | Alguno hace falta para construir o probar la app | «ok» (28-oct) |
| B3-12 | **El código va en inglés y la documentación en español, con un paquete por responsabilidad.** Cada paquete dice en su `doc.go` qué hace y qué parte del diseño implementa. El árbol completo está en `repo-estructura.md` | Nombres en español, o todo en una sola carpeta | El código lo leerán solo hispanohablantes (el jurado es internacional) | «ok» (28-oct) |
| B3-13 | **El primer commit es un esqueleto que compila y pasa sus pruebas, sin lógica del ciclo.** Responde `/healthz` y una página mínima, trae CI y el README en inglés y español. El commit y el push los haces tú | Subir solo carpetas vacías, o esperar a que la rebanada 0 esté completa | La rebanada 0 estuviera lista hoy | «ok» (28-oct) |

| # | Del 25-oct | Decisión de Eber (literal, 25-oct) | Cambio |
|---|---|---|---|
| P5 | **Ves objetos, no carpetas:** secciones, decisiones y tarjetas, no archivos | «Ok» | Ninguno. La vista de carpeta es B3-15 |
| P6 | **La app corre en Fly.io** | «Ok» | Plan B: Railway |

## Ya decididas en otros bloques (se citan, no se replantean)

| Antes aquí | Qué dice | Dónde está decidido |
|---|---|---|
| B3-1 | El agente no escribe directo: entrega su salida con forma fija y la app la valida y la aplica. Para leer tiene `buscar` y `leer(id)` | Fichas §1 y §7 · N-1 · B2-2. La duda de si el modelo cumple esa forma sigue como hipótesis en `bloque-3.md` §6 |
| P2 | La base de datos es la autoridad; los archivos son copias | N-1 · B2-1 |
| P3 y parte de la antigua B3-7 | Puedes descargar el documento aunque no esté todo firmado; lo no firmado sale marcado. El `.md` se genera desde la base | K-10 (b). «Expediente» ahora se llama «grafo del proyecto» (N-1) |

**Pasó al bloque 1:** qué pasa con los pasos del agente y con tu edición mientras firmas una sección (antes B3-2). Es una regla del ciclo; está como propuesta **K-16** en `../bloque-1/bloque-1-decisiones.md`.

**Reemplazadas:** P1 (git) por B3-16 · P4 (acceso) por B3-3 · P7 (fecha) por B3-17 y el plan · las antiguas B3-4 a B3-8 por B3-14 a B3-17.
