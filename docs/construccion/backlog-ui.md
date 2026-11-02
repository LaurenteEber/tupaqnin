---
titulo: Backlog de construcción · interfaz en tres paneles
estado: v0.1 · 2-nov · borrador listo para construir
relacion: implementa pantallas.md «Disposición general (2-nov)» y la maqueta `docs/diseno/bloque-3/maqueta-tres-paneles.html`. Solo `web/` y el paquete servido en `internal/server/static/`; nada del servidor. Mismo formato y mismo «Retorno» que backlog-r1.md
---

# Backlog · interfaz en tres paneles

**Respuesta:** 3 piezas, U.1 → U.2 → U.3, todas en `web/`. Al terminar, la página tiene **proyectos a la izquierda, conversación o documento al centro y el marco a la derecha**, con paneles laterales plegables y un divisor entre el centro y la derecha, sobre un fondo de cuadros; y una pestaña «Markdown» que muestra lo que se descarga. **No cambia ninguna regla del producto ni ninguna ruta**: es mover lo que ya existe y sumar tres controles. **Horas: 5–8 `[inferencia]`.** Cada pieza en su commit; la que no quede en verde no se integra (backlog-r4, «Respuesta»).

**Fuentes que mandan, en este orden:** (1) `pantallas.md` «Disposición general (2-nov)» y su tabla de correspondencia; (2) la maqueta `maqueta-tres-paneles.html` para la disposición, los controles, sus atajos y el fondo de cuadros; (3) las reglas que se mantienen: **U-3** (el esqueleto del marco es la única navegación entre secciones: el documento no lleva el selector de secciones de la maqueta), **U-4** (una sola familia tipográfica: el documento no pasa a serifa como en la maqueta), **P7** (la firma va a pantalla completa y oculta el marco y el texto, como hoy) y la regla 7 (ningún id interno ni clave de catálogo a la vista).

## Qué va en cada lugar (de `App.tsx` de hoy a los tres paneles)

| Hoy (`App.tsx`, `.columns`) | Después |
|---|---|
| `aside.col` con el menú «Tus proyectos ▾» (`pmenu`: proyectos, caso de demo, en blanco) | **Izquierda** (U.2): «Nuevo proyecto», «Probar con el caso de demo» y la lista «Tus proyectos», con lo que ya trae `ProjectSummary` (nombre, última actividad, secciones, firmadas, `no_account`) |
| `aside.col` con `FramePanel` y `NewSectionForm` | **Derecha · El marco** (U.1), igual que hoy |
| `section.col` de la conversación (`RequestPanel`, `UnderstandingPanel`, `ConversationPanel` con `StopReportPanel`, `CardPanel`, `composer`) | **Centro · Conversación** (U.1), igual que hoy |
| `section.col.doc` (cabecera con estado, Auditar, Deshacer, Firmar; avisos; `SectionEditor`; `ProposalPanel` ×2; traza) | **Centro · Documento** (U.1), igual que hoy, sin selector de secciones (U-3) |
| `HistoryPanel` (bajo el documento) | **Derecha · Historia** (U.1): pestaña que aparece cuando hay una historia abierta y se cierra con ella |
| Cabecera: `ProjectTitle`, «Inicio», «Descargar .md», idioma | `ProjectTitle` en la cabecera del centro; el idioma y la cuenta a la izquierda abajo (U.2); «Descargar .md» en **Derecha · Markdown** (U.3); «Inicio» sale (la izquierda ya lleva a los proyectos y al inicio) |
| `ModelLine` y `ModeBar` en la cabecera de la conversación | En la cabecera del centro, para las dos vistas (U.1) |
| `MessageInput` | Abajo del centro, **en las dos vistas** (U.1) |
| `HomePage` (sin proyecto) | Centro, igual; la izquierda se ve; la derecha, oculta (U.2) |
| `SignoffPanel` (reemplaza todo) | Igual que hoy: pantalla completa (P7) |

## Elecciones de este backlog (para Eber)

| # | Propuesta | En vez de | Elegiría la otra si… | Decisión de Eber |
|---|---|---|---|---|
| B-95 | **El documento en el centro, alternando con la conversación** con un selector «Conversación · Documento» en la cabecera del centro; **el centro cambia solo a «Documento»** cuando eliges una sección en el esqueleto, abres un aviso o una propuesta de una sección, o pulsas «Firmar» desde el informe; **cambia solo a «Conversación»** cuando un aviso lleva a una tarjeta, un «por aclarar» o una ronda | Documento en una pestaña del panel derecho (la maqueta lo descartó) | Hubiera que leer el documento y la conversación a la vez sin alternar | **Eber, 2-nov:** «Documento en el centro» |
| B-96 | **La historia va en una pestaña «Historia» del panel derecho**, que aparece al abrir la de un párrafo o una decisión y abre el panel si estaba plegado | Dejarla bajo el documento (hoy) · dentro de «El marco» (maqueta) | La pestaña se pierde de vista | **Se infiere:** la tabla de la disposición del 2-nov |
| B-97 | **Markdown sin dependencias:** la pestaña pide `GET /api/projects/{id}/export.md?lang=` (el mismo de la descarga, B-47) y lo muestra en «Fuente» tal cual (`<pre>`) y en «Vista» con un render propio y mínimo: encabezados `#`–`###`, párrafos, listas `-` y `1.`, `**negrita**`, `*cursiva*`, `` `código` ``; **todo el texto se escapa** y nunca se inserta HTML del documento. Se refresca al abrir la pestaña y tras cada `onSaved`, paso o firma | Una librería de markdown (AGENTS regla 11: dependencia nueva) · un render en Go (cambia el servidor) | El documento usara tablas o notas al pie | **Se infiere:** pedido de Eber y AGENTS regla 11 |
| B-98 | **Preferencias por navegador** en `localStorage`, con `try/catch` (si falla, valores por defecto): `tq.leftOff`, `tq.rightOff` (`"1"`/`"0"`), `tq.rightW` (px), `tq.rightTab`. Por defecto: los dos paneles abiertos, derecha 420 px, pestaña «El marco». Ancho de la derecha entre 300 px y 65 % de la ventana; el centro nunca baja de 360 px | Guardarlas en la cuenta (cambia el servidor) | — | **Se infiere:** maqueta |

## Piezas

### U.1 · Los tres paneles

| Campo | Contenido |
|---|---|
| **Qué** | La página pasa de `.columns` a tres paneles: izquierda (por ahora con lo que había en el menú «Tus proyectos ▾»), centro con «Conversación · Documento», derecha con «El marco · Historia». Paneles laterales plegables, divisor arrastrable y fondo de cuadros en el centro |
| **Implementa** | pantallas.md «Disposición general (2-nov)» · B-95 · B-96 · B-98 · la maqueta (disposición, controles, atajos, fondo) |
| **Leer** | `web/src/App.tsx` (todo el `return`, `reach`, `reachWarning`, `onTodo`, `openSection`, `signFromReport`, `askToSign`) · `web/src/styles.css` · la maqueta · pantallas.md (disposición del 2-nov y del 29-oct) |
| **Interfaz** | **Componentes nuevos:** `Shell` (las tres zonas y el divisor), `LeftPanel`, `RightPanel` (pestañas), `CenterHeader` (selector, `ProjectTitle`, `ModeBar`, botón ▤; debajo `ModelLine`) · **lógica pura y probada en `web/src/layout.ts`:** `readPrefs()/writePrefs()` con `try/catch` (B-98), `clampRightWidth(px, windowW)`, `nextWidthByKey(px, key)` (flechas ±24 px), `routeTarget(target)` → `{center: "conv"\|"doc"\|null, right: "frame"\|"history"\|null}` que decide a dónde lleva cada destino de hoy (`document`, `proposals`, `proposed-sections`, `sign-section`, `history`, `card-*`, `oq-*`, `round-*`) según B-95 y B-96 · `reach(id)` primero aplica `routeTarget`, espera al siguiente cuadro y después hace el `scrollIntoView` y el destello de hoy · **controles:** ☰ y Ctrl/⌘+B pliegan la izquierda (queda una franja de 52 px con iconos, como la maqueta); ▤, ✕ y Ctrl/⌘+. pliegan la derecha; el divisor `role="separator"` con `aria-valuenow`, arrastre con el ratón, doble clic vuelve a 420 px, flechas ±24 px; los selectores son `role="tablist"`/`"tab"` con `aria-selected` · **fondo:** cuadrícula de 28 px con líneas `rgba(31,36,33,.055)` y el resplandor de la maqueta, solo en el centro; tarjetas, editor y composer siguen sobre blanco · **pantallas de menos de 900 px:** la derecha se abre encima del centro (como la maqueta) y la izquierda empieza plegada · **textos nuevos** en `en.json` y `es.json` (los pasa la prueba de P-116 de claves en los dos catálogos) |
| **Listo cuando** | `pnpm test`: pruebas de `layout.ts` (preferencias con `localStorage` que lanza error; límites del ancho; flechas; `routeTarget` para cada destino de hoy, incluido «Firmar» desde el informe → Documento, tarjeta → Conversación, historia → Historia con el panel abierto) · `make web`, `make test` y `make vet` en verde · **las pruebas de recorrido de Go no cambian** (no se tocan rutas) · prueba de humo con Chrome sin cabeza y el proveedor falso, que el agente corre y reporta: abrir el caso 5, plegar y desplegar los dos paneles, arrastrar el divisor, ir del esqueleto al documento y de una tarjeta del marco a la conversación, sin errores en la consola |
| **Depende de** | `b39f69f` (P-116) |
| **No hacer** | Cambiar lo que muestra o hace cada componente existente (solo se mueven). Tocar `internal/` salvo el paquete servido. Sumar dependencias. Agregar el selector de secciones de la maqueta (U-3) o la serifa (U-4). Mostrar el marco durante la firma (P7) |
| **Si algo no está** | `[EL DISEÑO NO LO DICE]` y una línea en `elecciones-agente.md` |

### U.2 · El panel izquierdo

| Campo | Contenido |
|---|---|
| **Qué** | La izquierda queda como en la maqueta: «Nuevo proyecto» y «Probar con el caso de demo» arriba; «Tus proyectos» como historial; abajo el aviso sin cuenta, el idioma y la cuenta |
| **Implementa** | pantallas.md (zona «Izquierda · proyectos») · B3-18, B3-19 · E.5 y E.6 (lo ya construido) |
| **Leer** | `HomePage.tsx`, `home.ts` (lo que ya hacen el inicio y las cuentas) · `api/client.ts` (`ProjectSummary`, sesión) · la maqueta |
| **Interfaz** | **Nuevo proyecto** y **Probar con el caso de demo** llaman a lo mismo que hoy el menú (`newProject()` y `newProject("case-5")`), con sus errores (`createErrorKey`) en el centro como hoy · **Tus proyectos:** nombre (o «Proyecto sin nombre», regla 7), fecha relativa de `last_activity`, «n secciones · m firmadas»; el abierto, resaltado; un clic llama a `openProject` · **abajo:** si la sesión no tiene cuenta, el aviso de B3-18 con «descárgalo» (abre la pestaña Markdown del proyecto abierto, o no aparece si no hay proyecto) y «crea tu usuario»; «Crear usuario» y «Entrar» llevan al inicio con ese formulario abierto (`HomePage` recibe qué formulario abrir); con cuenta, el usuario y «Salir» · el selector de idioma pasa aquí desde la cabecera · sin proyecto abierto, la derecha no se muestra y el centro es `HomePage`, como hoy |
| **Listo cuando** | `pnpm test`: pruebas en `home.ts` o un módulo nuevo para el texto de cada proyecto de la lista (sin nombre, fecha, contadores) y para qué muestra la parte de abajo con y sin cuenta · humo en Chrome sin cabeza: crear el caso de demo desde la izquierda, volver a la lista, abrirlo de nuevo, cambiar el idioma |
| **Depende de** | U.1 |
| **No hacer** | Cambiar las reglas de cuentas, de proyectos por navegador o de topes (E.1, E.5, B-89) |
| **Si algo no está** | `[EL DISEÑO NO LO DICE]` en el retorno |

### U.3 · La pestaña Markdown

| Campo | Contenido |
|---|---|
| **Qué** | La derecha suma «Markdown»: el documento como se descarga, en «Vista» o «Fuente», con «Copiar» y «Descargar .md» |
| **Implementa** | Pedido de Eber del 2-nov · B-47 (el `.md` viene de Go) · B-97 |
| **Leer** | `api/client.ts` (`exportURL`) · `internal/server` solo para ver qué devuelve `export.md` (no se cambia) · la maqueta |
| **Interfaz** | `web/src/markdown.ts`: `renderMarkdown(src): string` con lo que dice B-97 y escape total · `RightPanel` con la tercera pestaña: «Vista · Fuente», «Copiar» (portapapeles, con aviso si falla) y «Descargar .md» (el mismo enlace `download` de hoy) · se refresca al abrir la pestaña y tras guardar, tras un paso del agente o tras firmar · una línea al pie: «Es lo que se descarga: tu documento tal como está ahora, sin notas del proceso» (en los dos idiomas) |
| **Listo cuando** | `pnpm test` de `markdown.ts`: encabezados, párrafos, listas, énfasis y código; un `<script>` o `<img onerror>` en el texto sale escapado; un texto vacío no rompe · humo en Chrome sin cabeza: editar una sección y ver el cambio en «Vista» y en «Fuente»; descargar el `.md` |
| **Depende de** | U.1 |
| **No hacer** | Una dependencia de markdown. Cambiar lo que exporta Go |
| **Si algo no está** | `[EL DISEÑO NO LO DICE]` en el retorno |

## Horas `[inferencia]`

U.1: 3–5 · U.2: 1–2 · U.3: 1. **Total: 5–8 h.** Va antes del despliegue público del 3-nov, porque el video y el jurado ven esta interfaz.

## Retorno de cada pieza

El mismo de `backlog.md`: qué hace en 5 líneas · archivos · pruebas · lo que no hizo · `[EL DISEÑO NO LO DICE]` · dudas para Eber (máximo 3, con contraste). Además, una captura de la prueba de humo por pieza.
