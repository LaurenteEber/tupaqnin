---
titulo: Handoff · 28-oct (c) · del diseño a la construcción
estado: 28-oct, tarde · para continuar en un chat nuevo
relacion: sigue a _handoff-28oct-b.md. Reglas de trabajo: _reglas-de-trabajo.md (sobre todo 3, 7, 9 y 11)
---

# Handoff · 28-oct (c)

**Respuesta:** el diseño de los bloques 1, 2 y 3 está cerrado y alineado. Los defaults técnicos (T-1 a T-11), el backlog de la rebanada 0 (con B-1 a B-7) y `AGENTS.md` están adoptados por Eber. Un agente de código empieza la rebanada 0 **en otra carpeta**, que no está conectada al chat. Lo siguiente es acompañar esa construcción: revisar el retorno de cada pieza contra el diseño.

## 1. Qué se hizo hoy (28-oct)

| Tema | Resultado | Archivo |
|---|---|---|
| Bloque 2 (memoria del principal) | Cerrado: B2-1 a B2-7 adoptadas, regresión de relevo y sondas pasadas | `bloque-2/bloque-2.md` v3.6 · `bloque-2/decisiones.md` |
| Bloque 3 | Decisiones adoptadas: B3-3, B3-9, B3-10 a B3-17 · K-16 adoptada en el bloque 1 («ok, lo adoptamos») | `bloque-3/bloque-3-decisiones.md` · `bloque-1/bloque-1-decisiones.md` |
| Repo público | Especificación del primer commit (esqueleto, sin lógica); Go 1.27 | `bloque-3/repo-estructura.md` v1.1 |
| Modelo de datos y salida fija | En inglés con glosario; etiquetas cortas, ids provisionales `new-n`, `operation`, criticidad al escribir, `model_call` con entradas y salidas | `bloque-3/bloque-3-especificacion.md` v2.8 · `bloque-3/salida-principal.schema.json` · `glosario.md` v1.2 |
| Defaults técnicos | T-1 a T-11 adoptados, validados (técnico y coherencia) y reverificados | `bloque-3/defaults-tecnicos.md` v1.3 · informes en `docs/casos-prueba/validacion-defaults-28oct/` |
| Formato de etiquetas | «D-04» → `dec-4` en fichas, bloque 2 y C4 | `bloque-1/bloque-1-fichas.md` v1.15 |
| Backlog rebanada 0 | 10 piezas (0.1 a 0.10) con Qué · Implementa · Leer · Interfaz · Listo cuando · Depende de · No hacer · Si algo no está · Retorno. B-1 a B-7 adoptadas. Revisado por un agente frío y reverificado | `docs/construccion/backlog.md` v1.2 · informes en `docs/casos-prueba/backlog-r0-frio-28oct/` |
| `AGENTS.md` (raíz) | Nuevo, en inglés (T-1: instrucciones a modelos). El anterior está en `_por_eliminar/AGENTS-14oct.md` | `AGENTS.md` |
| `.gitignore` del repo de código | Entregado a Eber por el chat (no está en esta carpeta): repo-estructura §4 + Go, web, editores | — |

## 2. Qué falta

**De Eber:**
1. **H-2** del plan (`_plan-hackathon.md` §4): fecha del primer despliegue. Bloquea la prueba desplegada de 0.6. También sigue abierta **H-1** (caso de demo).
2. **Clave de Meta** en el `.env` de la carpeta del agente (la usa 0.8). Nunca en el chat ni en el repo.
3. **Carpeta del agente:** debe tener `AGENTS.md`, `CLAUDE.md` y `docs/` completo; si no, el agente no tiene qué leer. Lo más simple: que sea el clon de `LaurenteEber/tupaqnin` con `docs/` adentro (B3-10).
4. **Commit en esta carpeta** de todo lo de hoy (defaults, esquema, glosario, especificación, fichas, bloques 1 y 2, backlog, informes, `AGENTS.md`, repo-estructura).
5. Antes del primer push: revisar `docs/investigacion/11-hackathon-meta.md` (elegibilidad, Ceplan) (B3-11).

**Del chat siguiente:**
1. **Revisar cada pieza que devuelva el agente** contra su fila del backlog: «Listo cuando», «No hacer» y los `[EL DISEÑO NO LO DICE]` que reporte. Lo que sea decisión va a Eber como fila con contraste. Para leer esa carpeta hay que pedir acceso (`device_request_folder_access`).
2. **Encabezado del backlog:** dice «propuesta para Eber»; ahora está adoptado. Corregirlo cuando Eber no lo tenga abierto.
3. **Instrucciones al principal** (`internal/cycle/prompts/principal.en.md`, pieza 0.10): las escribe el agente como borrador; Eber debe revisarlas.
4. **Resultado de la micro-prueba 0.8:** si Meta no admite partes del esquema en modo estricto, la app las valida en Go. Cambiar el esquema de `docs/` lo decide Eber.
5. **Reconciliación de horas:** está pendiente desde el plan (rebanadas 0–2 vs. ≈ 60–65 h; especificación §8 suma 35–48 h solo del bloque 3).
6. **Después de la rebanada 0:** backlog de la rebanada 1 (ciclo mínimo), con el mismo formato y el mismo control del agente frío.

## 3. Qué leer para retomar (en este orden)

1. `_reglas-de-trabajo.md`: cómo trabajar con Eber, sobre todo 3 (Eber adjudica), 7 (una corrección y una reverificación) y 11 (un solo escritor).
2. `docs/construccion/backlog.md`: lo que se está construyendo.
3. `AGENTS.md`: lo que el agente de código tiene como reglas.
4. `bloque-3/defaults-tecnicos.md` y `glosario.md`: para juzgar los retornos.
5. Si hace falta el porqué: `bloque-3/bloque-3-especificacion.md`, `bloque-1/bloque-1-fichas.md` §3 y §7, `bloque-2/bloque-2.md` §2–§4.

## 4. Cómo trabajar con los archivos (aprendido hoy)

- **Rule 11, en la práctica:** antes de escribir, traer una copia fresca (`device_stage_files`, no la vista del shell, que puede ir atrasada); editar una copia con **nombre único** en `/mnt/user-data/outputs/`; escribir con `expectedMtimeMs`; esperar unos 30 s y comparar byte a byte con una nueva copia fresca.
- **Obsidian rellena las tablas** al guardar: editar sobre la copia que trae el último guardado de Eber, nunca sobre una propia más vieja.
- **No tocar un archivo que Eber está respondiendo;** avisarle cuáles quedan cerrados mientras se escriben.
- **Nada se borra:** se mueve a `_por_eliminar/`.
- **No implementar** cuando se pide especificación. No mencionar el repo anterior ni su historia.
- **Eber hace los commits y los push.**
