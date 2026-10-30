---
titulo: Handoff · 30-oct · supervisión de Claude Code, iteraciones por pruebas y validación del backlog de la rebanada 2
estado: 30-oct, noche · 1.9 hecha y revisada · para abrir un chat nuevo que siga la supervisión
relacion: sigue a _handoff-29oct.md (que armó ../construccion/backlog-r2.md en otro chat). Reglas de trabajo: _reglas-de-trabajo.md §3 (3, 5, 7, 11 y 12). Reglas del agente de código: AGENTS.md (12 y 13)
---

# Handoff · 30-oct · supervisión e iteraciones

**Respuesta:** el chat nuevo hace tres cosas: (1) **supervisa a Claude Code**: la pieza **1.9** está hecha (`ce65aa1`) y el agente hace un arreglo chico (la historia de una decisión sin punto en el texto); sigue la prueba de Eber con Meta y la 1.8; (2) **itera el producto según las pruebas con Meta** que corre Eber; (3) **valida `backlog-r2.md`** (lo armó otro chat el 29-oct, v0.2, decisiones B-21 a B-35 sin responder) y lo actualiza con lo que cambió el 29 y el 30-oct. Abajo, §4, van **8 choques ya detectados** entre ese backlog y lo adoptado después. Sin recortar el alcance: «por ahora continuemos con el plan aún no hagamos un recorte del alcance».

## 1. Dónde estamos

| Tema | Estado | Archivo |
|---|---|---|
| Rebanada 0 | Hecha (0.1 a 0.10) | `../construccion/backlog.md` |
| Rebanada 1 | Hechas 1.0 a 1.7 con commit (1.5b: interfaz de tres columnas). **1.9a** (micro-prueba de herramientas) pasó 5/5 con Meta, commit `44f0b11`. **1.9 hecha**, commit `ce65aa1` (`make test`, `make vet` y `make web` pasan; sin navegador ni Meta). Después: prueba de Eber con Meta y **1.8** (caso 5 de punta a punta). 46–65 h `[inferencia]` | `../construccion/backlog-r1.md` v0.16 |
| Supervisión | P-1 a P-46. Abiertos: P-44, P-46 y la corrección de P-41 (pasan a la 1.9); P-42 menores | `../construccion/supervision.md` v1.18 (filas 1.9a y 1.9) |
| Rebanada 2 | Backlog v0.2 armado, **sin validar** contra lo del 30-oct; 10 piezas, 37–53 h | `../construccion/backlog-r2.md` |
| Calendario | Corte de la rebanada 1 el **31-oct**; rebanada 2 el 31-oct y 1-nov; 3 el 2-nov; 4 el 3-nov | `_plan-hackathon.md` §2 |
| Créditos de Meta | ≈ 55 USD `[inferencia]`. Hoy: ≈ 0,38 USD en dos corridas reales + ≈ 0,14 USD la micro-prueba | — |

**Commits del agente (repo `tupaqnin/`):** 1.4 `948c463` · 1.5 `f6d40a8` · 1.6 `ece37b4` · 1.5b `368b854` · 1.7 `100d0ab` · arreglos `83d44c4`, `f926d2e`, `f1f05bf`, `a0d6aad`, `9d58ec7` · micro-prueba `44f0b11` · 1.9 `ce65aa1`.

## 2. Iteraciones de producto (29 y 30-oct)

| Fecha | Iteración | Origen | Dónde quedó |
|---|---|---|---|
| 29-oct | **Tres columnas:** el marco a la izquierda (lo que la app le deja al humano para formar su marco mental: tarjetas, firma, pendientes), la conversación al centro, el documento a la derecha | Eber, sobre el boceto | `bloque-3/pantallas.md` · `bloque-3/boceto-ui.html` · B3-9 precisada · pieza 1.5b |
| 29-oct | P-35: una sección sin texto no aparece «lista» → **B-21** | Prueba en el navegador | `backlog-r1.md` |
| 30-oct | La cadena se detenía tras el primer paso (no era lentitud) → arreglo `9d58ec7`; **B-23** criterios vacíos; **B-24** `stop_reason` en `step_ended` | Prueba con Meta (P-43) | `backlog-r1.md` · espec. §1.1 |
| 30-oct | **Propuesta 30-oct (adoptada entera):** R-1 a R-9 y U-1 a U-6. Nació de dos corridas reales: el caso 5 escribió genérico sin leer fuentes (P-45) y «pobreza…» escribió 10 secciones de memoria, respondió preguntas como secciones, 8 decisiones críticas falsas y todos los roles «conclusión» | Pruebas con Meta | `_propuesta-30oct-fuentes-y-flujo.md` v0.4 |
| 30-oct | En la **1.9**: R-3 herramientas `read_source` / `search_sources` (el agente decide qué leer; B-22 descartada tras la micro-prueba), R-4 sin material solo ofrece «subir material» o «esperar la búsqueda», R-5 `reply` (las respuestas van a la conversación), R-7 decisiones reales ≤ 2 por paso, R-8 rol `conclusion`/`other`, U-1 título con `project_title`, U-2 datos del modelo, U-3 una sola navegación, U-4 una familia tipográfica, U-5 historia de la conversación, U-6 un solo «Entendí» con «Confirmar» que no condiciona el arranque | Propuesta | `backlog-r1.md` §1.9 · `bloque-3/funcionalidades.md` v1.1 (2j a 2m) · `pantallas.md` v0.3 («Precisiones del 30-oct», mandan sobre el boceto) |
| 30-oct | Para la **rebanada 2**: R-1 procedencia (`claim_origin`: `material` · `external` · `model_knowledge`) y el pendiente `unsourced_claim`; R-2 «¿en qué te basaste?» calculado por la app; R-6 plan acordado antes de escribir | Propuesta | `_plan-hackathon.md` · glosario §11 · estados |
| 30-oct | Para la **rebanada 3**: R-9 evidencia real (OpenAlex, arXiv u otra), qué afirmación viene de la fuente del humano, cuál de la buscada, y los conflictos entre fuentes | Eber | `_propuesta-30oct…` v0.4 §4.4 · `_plan-hackathon.md` |
| 30-oct | Validación fría de la propagación: 10 hallazgos verificados y corregidos (rol con 3 valores, `origin` → `claim_origin`, «pasaje leído» = fuente con `delivery` `via = read`, etc.) | Auditor | `supervision.md` · docs citados |

**Tres precisiones del chat, adoptadas por Eber (30-oct):** (1) en la 1.9 no se ofrece «escribir de memoria» (llega con R-1); (2) «leído» = el agente leyó la fuente, sin guardar el tramo; (3) el botón del «Entendí» es «Confirmar», sin «empezar».

## 3. Supervisión: qué sigue

1. **1.9 revisada** (fila en `supervision.md`): coincide, con las tres precisiones ya en el código. Respuestas a sus dudas: (a) «leído» vale para cualquier sesión del proyecto; (b) una decisión sin punto en el texto abre su propia historia (propuesta, contraste, «porque»), no el primer párrafo; el agente aplica (b) con su prueba y se detiene. **Revisar ese arreglo** cuando llegue. Riesgo: los prompts (R-4, R-5, R-7, lectura) no tienen prueba automática; la corrida de Eber es la única comprobación.
2. **Prueba de Eber con Meta en el navegador** (`localhost:8080`, Chrome habilitado): caso 5 llega a la regla 5 (1,5 vs 60) con su tarjeta; «pobreza» sin material ofrece las dos opciones; «¿en qué te basaste?» responde en la conversación; título del proyecto; sin decisiones falsas.
3. **1.8** y el corte del 31-oct.
4. **Pendiente de Eber:** `make microtest-signoff` (Wonder). Los docs de los dos repos quedaron con commit el 30-oct (dos pruebas del código comparan sus copias de los esquemas con `docs/diseno`).

## 4. Backlog de la rebanada 2: choques ya detectados (validar y corregir)

| # | Choque | Propuesta |
|---|---|---|
| 1 | **Numeración:** `backlog-r2` usa B-21 a B-24 con otro sentido; en `backlog-r1` B-21 a B-24 ya están adoptadas y el código las cita (`f1f05bf fix: B-21`) | Renumerar las de la rebanada 2 desde **B-36** (o B2-…); revisar referencias cruzadas |
| 2 | r2 B-22 (tope de 12 pasos, la parada reemplaza a B-17) vs r1 B-24 (`stop_reason`: `no_work` · `agent_pending` · `step_limit`) | Sumar el motivo de la parada (`stop` con informe) a `stop_reason` |
| 3 | **R-8** dejó el rol en `conclusion`/`other`; r2 B-23 y B-27 (regla 2) hablan de «conclusión o recomendación» | Reescribir con `conclusion`; revisar fichas §3 regla 2 y R6 |
| 4 | **R-1, R-2 y R-6 no están en el backlog** (el plan ya los pone en la rebanada 2) | Pieza nueva o dentro de 2.1/2.3: `claim_origin`, `unsourced_claim` como regla de la app, «¿en qué te basaste?», plan acordado (relación con el «Entendí» de U-6) |
| 5 | B-28 suma `stretch` a `salida-principal.schema.json`; el esquema ya cambió (`reply`, rol reducido) | Agregar `stretch` sobre la versión actual del esquema |
| 6 | Piezas 2.4, 2.7 y 2.9 leen `boceto-ui.html`, que está viejo frente a U-1 a U-6 | Leer primero `pantallas.md` v0.3 «Precisiones del 30-oct»; los bocetos que faltan (P4, P6, avisos, línea de vuelta, defensa) se dibujan sobre esa versión |
| 7 | A-2 (par afirmación–pasaje) y el comprobador: con R-3 ahora existe `delivery` `via = read` | Usarlo como evidencia de lectura; no inventar otro registro |
| 8 | **Horas:** r2 37–53 h + R-1/R-2/R-6; la rebanada 1 sumó la 1.9 (12–17 h); el plan da 2 días a la r2 | Informar el número a Eber; **no proponer recorte** salvo que lo pida |

Después: auditor frío + reverificación (regla 3.7) y validación con un subagente que lea como implementador; Eber responde las elecciones (columna vacía) antes de pasar la 2.1.

## 5. Cómo trabajar

- **En `tupaqnin/` no se escribe nada, no se deja rastro:** solo `GIT_OPTIONAL_LOCKS=0 git --no-optional-locks …`; `git status` igual antes y después. Los registros viven en `tupaqnin_design/` (`supervision.md`).
- **Al agente le habla Eber**, con mensajes en inglés que el chat redacta. Autonomía dentro de la rebanada: commit local por pieza, sin push, se detiene en los 5 casos de AGENTS.md (reglas 12 y 13). **No firma commits:** sin Co-Authored-By, Signed-off-by, Claude-Session ni «Generated with»; sin `--author` ni cambios de git config. Sus elecciones van a `../construccion/elecciones-agente.md` (archivo suyo).
- **Meta:** la clave está en el llavero de macOS (`tupaqnin-meta-api`), nunca en el chat ni en archivos. El agente nunca llama a Meta; las pruebas reales las corre Eber (`make microtest`, `make microtest-tools`, `make microtest-signoff`, servidor con `TQ_PROVIDER=meta`).
- **Editar en el disco con un script corto** y comprobar; no suponer que Eber tiene el archivo abierto en Obsidian. Las celdas de tablas de Obsidian traen relleno: anclar con regex o por línea.
- **Pasos destructivos:** decir qué NO borrar, preferir lo reversible (papelera, `git restore`), nunca `rm` sobre archivos de Eber.
- **No crear una decisión nueva para precisar otra:** se precisa la original.
- **Primero la respuesta; tablas cortas; contraste en cada propuesta.**

### Pasar el diseño al repo de código: solo `rsync` selectivo

Nunca `cp -R docs/`: pisa `elecciones-agente.md` (que escribe el agente) y trae carpetas que Eber quitó (P-33). Sin `--delete`, así nunca borra lo que el agente tiene (por ejemplo `docs/casos-prueba/`):

```
cd ~/Documents/SideProjects
rsync -a tupaqnin_design/AGENTS.md tupaqnin/
rsync -a tupaqnin_design/docs/diseno/ tupaqnin/docs/diseno/
rsync -a tupaqnin_design/docs/demo/ tupaqnin/docs/demo/
rsync -a --exclude elecciones-agente.md tupaqnin_design/docs/construccion/ tupaqnin/docs/construccion/
```

Pasar solo lo que cambió (si solo cambió diseño, basta la segunda línea). Antes de pedir la pieza al agente, comprobar con `diff -q` que el archivo clave llegó.

## 6. Qué leer (en este orden)

1. Este handoff, `_reglas-de-trabajo.md` y `_propuesta-30oct-fuentes-y-flujo.md` v0.4.
2. `../construccion/supervision.md` (Respuesta, piezas, P-34 a P-46) y `../construccion/backlog-r1.md` §1.9 y decisiones B-8 a B-24.
3. `../construccion/backlog-r2.md` v0.2.
4. `bloque-3/pantallas.md` v0.3, `bloque-3/funcionalidades.md` v1.1, `bloque-3/bloque-3-especificacion.md` v2.15 («Precisiones del 30-oct»), `glosario.md` v1.7 §11.
5. Esquemas en `bloque-3/`: `salida-principal`, `salida-entendi` y los tres nuevos de la rebanada 2.
6. En el repo de código (solo lectura): `internal/cycle/runner.go`, `internal/provider/`, `docs/casos-prueba/micro-prueba-herramientas/informe.md`.
