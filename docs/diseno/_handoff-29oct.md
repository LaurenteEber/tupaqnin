---
titulo: Handoff · 29-oct · planear la rebanada 2 (la rebanada 1 sigue en construcción)
estado: 29-oct, noche · para abrir un chat nuevo que planee la rebanada 2
relacion: sigue a _handoff-28oct-d.md. Reglas de trabajo: _reglas-de-trabajo.md (3, 5, 7, 11 y 12). La supervisión de la rebanada 1 sigue en el chat del 29-oct
---

# Handoff · 29-oct · rebanada 2

**Respuesta:** el chat nuevo arma `../construccion/backlog-r2.md`, con el mismo formato y el mismo «Retorno» que `backlog-r1.md`, para la rebanada 2 completa. **Sin recortar el alcance**, en palabras de Eber: «por ahora continuemos con el plan aún no hagamos un recorte del alcance». La rebanada 1 la sigue construyendo Claude Code y la supervisa el chat del 29-oct; este chat no la toca.

## 1. Dónde estamos

| Tema | Estado | Archivo |
|---|---|---|
| Rebanada 0 | Hecha (0.1 a 0.10) | `../construccion/backlog.md` |
| Rebanada 1 | Hechas 1.0 a 1.4 con commit. Siguen 1.5, 1.5b, 1.6, 1.7 y 1.8. 34–48 h en total `[inferencia]` | `../construccion/backlog-r1.md` v0.10 |
| Supervisión | P-1 a P-33; abiertos P-28 (stream crudo), P-32 (el agente no puede deshacer tu rol confirmado), P-33 (copia de `docs/`), P-21 (prueba con Meta) | `../construccion/supervision.md` v1.11 |
| Agente de código | **Claude Code** desde el 29-oct (antes Muse Code). Autonomía dentro de la rebanada: commit local por pieza, sin firmas, sin push; se detiene solo en 5 casos (AGENTS.md, reglas 12 y 13). Sus elecciones van a `../construccion/elecciones-agente.md` | `../../AGENTS.md` |
| Interfaz | Tres columnas: **el marco** a la izquierda, la conversación al centro, el documento a la derecha. El marco mínimo entra en la 1.5b; la línea de vuelta y el «sin ver» exacto quedan para la rebanada 2 | `bloque-3/bloque-3-decisiones.md` (B3-9 precisada) · `bloque-3/pantallas.md` v0.2 · `bloque-3/boceto-ui.html` |
| Decisiones de construcción | B-8 a B-20 adoptadas (B-9 con PDF, en proceso hijo; B-19 cadena de dependencias con pnpm 12.16.1; B-20 valores iniciales) | `../construccion/backlog-r1.md` |
| Versiones | Adoptadas: Vite 6.4.5, TypeScript 5.9.3, plugin-react 4.7.0 (sin mayores durante el hackathon), Node ≥ 24.22.1, pnpm 12.16.1, `ledongthuc/pdf` | `../construccion/versiones-seguras.md` v0.3 |
| Demo | Caso 5 (microplásticos), plantilla real en `internal/demo/case5/`; el gancho de la cita que no existe es de la rebanada 3 | `../demo/caso-5/` |
| Créditos de Meta | ≈ 56 USD libres. El consumo grande de los días 27 a 29 fue del agente de código, no de la app `[inferencia]`. La app solo llama a Meta en las pruebas que corre Eber | — |

## 2. Qué entra en la rebanada 2 (no se recorta)

De `_plan-hackathon.md` §1, en literal: **comprobaciones completas** (las 6 reglas de la app y el comprobador) · **«por aclarar»** · **triaje y vía directa** · **parada con informe** · **preparación de la defensa** · **avisos entre secciones y por nueva versión de una fuente, con su lista de cambios (A-1)** · **comprobador con el par afirmación–pasaje (A-2)** · **reentrada** · **E-4**. Lo que H-3 suma a la 2: cambiar el destinatario o un criterio · marcar una decisión como crítica · el efecto del rol y del destinatario sobre lo firmado (K-13) · descargar el `.md` · crear una sección a mano (K-17). Del bloque 2 (B2-5): **las sondas contra la contradicción (B2-7)**. Del marco (B3-9 precisada): **la línea de vuelta** y **el «sin ver» exacto**. «Listo cuando» del plan: los perfiles apurados de las simulaciones se frenan donde el diseño dice.

**Suma del 30-oct** (`_propuesta-30oct-fuentes-y-flujo.md`, adoptada): **R-1** procedencia de cada afirmación (tu material · fuente buscada · conocimiento del modelo) y el pendiente «afirmación sin fuente» · **R-2** «¿en qué te basaste?» calculado por la app · **R-6** plan acordado antes de escribir. Para la rebanada 3, **R-9** (evidencia real, OpenAlex o arXiv, conflictos entre fuentes) en palabras de Eber. La pieza 1.9 de la rebanada 1 ya trae `reply`, lectura a demanda y el «Entendí» en una tarjeta.

## 3. Qué hace el chat nuevo, en orden

1. **Leer** (abajo) y comprobar contra el código lo que ya existe de la rebanada 1 (solo lectura en `tupaqnin/`; regla 12).
2. **Bocetos que faltan** en `pantallas.md`, sobre la disposición de tres columnas: P6 informe de parada, «por aclarar» (P4), avisos en el texto, la línea de vuelta del marco y la preparación de la defensa. Antes de dibujar, Eber puede escribir su idea en 5–10 líneas (regla 3.1).
3. **`backlog-r2.md`**: piezas con Qué · Implementa · Leer · Interfaz · Listo cuando · Depende de · No hacer · Si algo no está; rutas nuevas; elecciones técnicas con contraste y la columna «Decisión de Eber» vacía; horas por pieza `[inferencia]`.
4. **Una revisión fría** con un solo auditor y una reverificación (regla 3.7), y una validación con un subagente que lee como el implementador (como se hizo con la rebanada 1).
5. **No recortar.** Si las horas no cierran, se informa el número y se deja la decisión a Eber; no se propone recorte salvo que él lo pida.

## 4. Qué leer (en este orden)

1. `_reglas-de-trabajo.md` y `_propuesta-30oct-fuentes-y-flujo.md`.
2. `_plan-hackathon.md` §1 (rebanadas y lo que suma H-3) y §2 (calendario).
3. `../construccion/backlog-r1.md` (formato, arquitectura, rutas y decisiones B-9 a B-20) y `../construccion/supervision.md` (qué existe y qué falló).
4. `bloque-1/bloque-1-fichas.md` §3 (comprobaciones), §7 (contratos), §8 (triaje, parada, reentrada) y `bloque-1/bloque-1-estados.md` (por aclarar, «escrita con lo anterior»).
5. `bloque-3/bloque-3-especificacion.md` §3, §5 y §9; `bloque-3/pantallas.md` v0.2 y `bloque-3/boceto-ui.html`; `bloque-2/bloque-2.md` (sondas, B2-7).
6. Si hace falta el porqué: `bloque-1/bloque-1-decisiones.md` (A-1 a A-5, E-1 a E-4, K-1 a K-17), `bloque-3/bloque-3-decisiones.md`, `glosario.md`.

## 5. Cómo trabajar (lo aprendido hasta el 29-oct)

- **Editar en el disco con un script corto** y comprobar a los 20–30 s. No suponer que Eber tiene el archivo abierto en Obsidian.
- **En `tupaqnin/` solo se lee** (`git --no-optional-locks`); al agente le habla Eber, con mensajes en inglés que el chat redacta.
- **No crear una decisión nueva para precisar otra:** se precisa la original (así se hizo con B-9 y B3-9).
- **Pasar diseño al repo de código con `rsync` selectivo**, nunca `cp -R docs/`: pisa `elecciones-agente.md` y trae carpetas que Eber quitó (P-33). Los comandos (sin `--delete`, así nunca borran):

  ```
  cd ~/Documents/SideProjects
  rsync -a tupaqnin_design/AGENTS.md tupaqnin/
  rsync -a tupaqnin_design/docs/diseno/ tupaqnin/docs/diseno/
  rsync -a tupaqnin_design/docs/demo/ tupaqnin/docs/demo/
  rsync -a --exclude elecciones-agente.md tupaqnin_design/docs/construccion/ tupaqnin/docs/construccion/
  ```
- **Primero la respuesta; tablas cortas; contraste en cada propuesta** («A en vez de B, porque…; elegiría B si…»).
