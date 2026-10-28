---
titulo: Handoff · sesión de diseño del 26–27 oct
para: el próximo chat que trabaje en el diseño de tupaqnin
fecha: 27-oct-2026, noche
---

> **Reemplazado por `_handoff-28oct.md` (27-oct, noche).** Este archivo queda como historia.

# Handoff · cómo seguir

**Respuesta corta:** el bloque 1 está completo y validado. El bloque 2 avanza en otro chat. El bloque 3 tiene una propuesta v2 que Eber todavía no adjudicó. El plan del hackathon espera dos decisiones, y la construcción empieza el **28-oct** con la rebanada 0.

## 1. Lee en este orden

1. `docs/diseno/_reglas-de-trabajo.md`: las reglas de trabajo con Eber y de los documentos. **Ojo:** su §6 «Estado» está desactualizado (dice v4.3 y menciona `_por-borrar/`). Lo vigente es este handoff.
2. `docs/diseno/bloque-1/bloque-1.md`: el ciclo v4.7 (con el marco y la firma proporcional; el detalle está en fichas §8).
3. `docs/diseno/bloque-1/bloque-1-fichas.md`, `bloque-1-estados.md` y `bloque-1-flujos.md` (v1.5; fichas v1.6): componentes, contratos, estados y las tres ramas sin probar.
4. `docs/diseno/_plan-hackathon.md`: rebanadas y calendario.
5. `docs/diseno/bloque-3/bloque-3.md`: la propuesta v2 (lo que se toma y lo que cambia).
6. Solo si hace falta: `bloque-1-decisiones.md` (todas adoptadas, con el contraste y la respuesta de Eber en literal), `bloque-1-revision.md` (proceso) y `docs/diseno/bloque-2/bloque-2.md`.

Repo: `~/Documents/SideProjects/tupaqnin_design/` en la computadora de Eber. **La copia del dispositivo es la que rige.**

## 2. Estado por bloque

| Bloque | Estado | Lo que falta |
|---|---|---|
| **1 · Núcleo** | **v4.7: integra K-1 a K-11 (barrido de coherencia) y M-1 a M-4 (el marco mental del humano).** Antes: v4.5 cerrado; v4.6 = reescritura de legibilidad. Integra D1–D22, P1–P6, C1–C6, R1–R6 (con R3′), E-1 a E-4, F-1, F-2, G-1 y **A-1 a A-5** (auditoría de perfiles, 27-oct: dependencias con versión, par afirmación–pasaje, dato reproducido, firma sin decisiones críticas, inventario de cobertura). Fichas, estados y flujos v1.4, verificados por un auditor independiente con una corrección y una reverificación (`bloque-1-revision.md`). Se probó con piloto, simulación de los 10 casos y regresión | La **prueba de cierre de Eber:** explicar el ciclo sin mirar (la está repasando). Una pregunta que llega desde el bloque 3: **quién fija el rol de una sección** (conclusión o recomendación). La propuesta es que el principal lo proponga en el «Entendí» y Eber lo confirme |
| **2 · Memoria del principal** | v1.1 en **otro chat** (`docs/diseno/bloque-2/`). Estado de trabajo en 3 capas, se recalcula desde la historia, entra completo en el hackathon (M7) | V1–V3 abiertas para Eber, en ese chat. **No editar sus archivos desde aquí** |
| **3 · Plataforma y archivos** | **v2 de hoy, propuesta.** Hay 4 archivos en `docs/diseno/bloque-3/`: idea, especificación, decisiones y revisión | Eber responde **B3-1 a B3-8** en `bloque-3-decisiones.md`. Después, una auditoría independiente con 6 hallazgos como máximo. Las Reglas Oficiales del hackathon deciden B3-3, B3-4 y B3-8 |
| **Plan del hackathon** | `_plan-hackathon.md` v1: rebanadas 0 a 4, corte el 31-oct y entrega el 4-nov a mediodía | Eber decide **H-1** (caso de demo: el caso 2 adaptado para un jurado internacional) y **H-2** (calendario) |

## 3. Pendientes de Eber (él decide o hace)

1. Hacer el commit de `_plan-hackathon.md`, `bloque-1/` (si no lo hizo ya) y `bloque-3/`.
2. Responder B3-1 a B3-8, H-1 y H-2.
3. Leer las Reglas Oficiales del hackathon: demo pública o con invitación, fecha de despliegue, uso de herramientas de Meta, repo público y video.
4. Hacer la prueba de cierre del ciclo.
5. ~~Borrar los dos archivos viejos del bloque 3~~ Hecho en `2da4b32` (recuperables de `4fc57e3`).
6. **Agregar a git** `bloque-1/`, `bloque-3/`, `_plan-hackathon.md` y este handoff: el commit `2da4b32` no los incluyó.
7. **Repasar A-1 a A-5** en la prueba de cierre: se aprobaron en bloque, sin rechazos; sobre todo A-3, que cambia una regla adoptada.

## 4. Siguiente trabajo para el chat

1. **Actualizar `_reglas-de-trabajo.md` §6** y su copia en el proyecto de claude.ai (`claude/reglas-de-diseno-tupaqnin.md`) con el estado de este handoff.
2. **Integrar las respuestas de Eber** a B3-x y H-x. Lo rechazado se registra con su literal y se ajustan `bloque-3.md` y la especificación.
3. **Lanzar la auditoría del bloque 3** con las reglas §3.7: un auditor sin las conclusiones del chat, contra el bloque 1 v4.4, el bloque 2 v1.1 y el plan.
4. **Reconciliar las horas.** El bloque 3 solo pide 29–38 h de las 60–65 h del plan, antes de la lógica del bloque 1. Hay que recortar o rehacer el Gantt con números de los tres bloques.
5. **Preparar la rebanada 0 (28-oct)** con encargos a agentes de código según la plantilla del bloque 1 §4: micro-prueba de modelos (Spark 1.3 y Wonder 1.0 en la Responses API, incluida la salida fija de B3-1), esqueleto en Go con SQLite desplegado en Fly y la prueba de streaming de 90 s.
6. **Guion del caso de demo** (si se adopta H-1): el caso 2 en inglés y en dólares, del encargo a la firma bloqueada y después firmada.

## 5. Avisos para el chat del bloque 2

- **Bloque 1 v4.5 (A-1, A-2, A-5):** la salida fija del principal suma `citas {ancla, id de fuente, pasaje}` y `cambios atendidos {id del cambio, actualizado · revisado sin cambio + «porque»}`; el retorno del lector suma `fuentes leídas {id}`. Una sección guarda lo que usa con su versión y su lista de cambios sin cerrar; la historia suma eventos de cambio (entra, actualizado, revisado sin cambio) y «nueva versión de una fuente». Ningún estado nuevo. Conviene que el estado de trabajo muestre los cambios sin cerrar por sección.
- **Bloque 1 v4.7 (para el bloque 2):**
  - la etapa inicial se llama **«Pedido»** (K-8); «encargo» queda para la plantilla del subagente (bloque-1.md §4). La capa 1 dice «Encargo y destinatario»: conviene renombrarla;
  - **el marco** (M-1) usa la misma función que calcula las capas 1 y 2: un cambio ahí cambia lo que ve el humano;
  - la capa 3 debe incluir los **cambios sin cerrar** por sección y el **inventario** de las rondas usadas, porque el principal declara sus ids;
  - objetos nuevos: **rol de la sección** (K-2) y **pregunta del destinatario** con estados (K-1); la capa 1 ya lista «preguntas del destinatario sin decidir»;
  - el contrato del principal suma «por aclarar» propuesto y «reemplaza» (fichas §7), y hay contratos para el «Entendí», el pedido de encargo, el digesto y el informe de parada.
- **Bloque 1 v4.6:** el detalle del ciclo (ritmo, cómo respondes, triaje, criticidad, quién hace qué) pasó literal de `bloque-1.md` a `bloque-1-fichas.md` §8. El encargo sigue en `bloque-1.md` §4. Si el bloque 2 toma términos de `bloque-1.md`, ahora están también en fichas §8.
- `bloque-2/encargo.md` enlaza el archivo viejo del bloque 3. La ruta nueva es `../bloque-3/bloque-3.md`.
- El bloque 3 v2 ya guarda lo que pide el bloque 2: lo entregado por turno con su versión, la nota del agente y el modo como eventos, el uso de contexto por llamada y el estado de trabajo como vista con caché. Ver la especificación §5.
- Lo que se le pasó antes y conviene confirmar: el índice usa los 5 estados de la sección más el número de pendientes; el modo «continúa hasta terminar» vive en «en vuelo»; una sección sin texto va como «borrador · sin texto»; la redacción es «desde lo último entregado».

## 6. Reglas que este chat no puede olvidar

- **Desde el chat no se borra nada.** Lo borra Eber. Al pedirle que borre, decir qué NO borrar y que se puede recuperar.
- **Mermaid siempre en TD,** nunca LR. Comprobar que cada diagrama se ve antes de entregar.
- **Toda propuesta lleva contraste** («en vez de», «elegiría la otra si…»), y Eber responde en su columna. Que no rechace ninguna es una alarma.
- **Máximo 2 páginas por documento de idea.** La especificación y el proceso van en archivos aparte.
- **Commit al cerrar cada sesión:** lo hace Eber.
- **Después de escribir en el dispositivo, verificar el archivo.** Ya hubo una reversión inesperada, y las copias en `outputs/` pueden estar viejas.
- **Salidas concisas,** en español y con frases cortas.

## 7. Mapa de archivos

```text
docs/diseno/
├── _reglas-de-trabajo.md      reglas (§6 desactualizado)
├── _encuadre-bloques.md       los tres bloques
├── _plan-hackathon.md         rebanadas y Gantt (sin commit)
├── _handoff-27oct.md          este archivo
├── bloque-1/                  ciclo v4.5 · decisiones · fichas · estados · flujos v1.4 · revisión · auditoría de perfiles
├── bloque-2/                  otro chat
└── bloque-3/                  v2: bloque-3 · especificación v2.1 · decisiones · revisión
docs/casos-prueba/             casos-bloque-1 · piloto-v4 · simulacion-v4.1 · regresion-v4.2 · relevo-b2-v1
docs/decisiones/               DEC-002 · DEC-003
docs/casos/diagnostico-y-aprendizajes.md   lecciones L-01 a L-16
```
