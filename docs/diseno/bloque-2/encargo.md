---
titulo: Bloque 2 · Encargo y plan de trabajo
estado: vigente · 28-oct · bloque 2 en la v3.1 (memoria del principal, rehecho desde cero), alineado con el bloque 1 v4.10
reglas: ../_reglas-de-trabajo.md
---

# Bloque 2 · Encargo y plan

## 1. Encargo de Eber (literal, 26-oct)

> El bloque 2 tiene como objetivo solucionar el problema de capacidad de contexto del agente que se encarga de dirigir el trabajo, el principal (debería ser el equivalente al orquestador en gentle-pi). El problema que resuelve es que en todo momento le da al agente el contexto apropiado para continuar con el trabajo, y este no dependa de lo que recuerde; ser robusto a los casos donde se cambia de modelo, o cuando se continua el trabajo en varias sesiones de días diferentes. Asimismo, cuando la capacidad de contexto del agente llegue a un umbral que deje de asegurar un trabajo de calidad, y se proceda con una compactacion o inicio de sesión (debe ser una capacidad de tupaqnin) el nuevo agente no tenga que leer todos los documentos sino solo Engram/su_equivalente para ponerse al tanto y continuar ágilmente con el trabajo.

> Lo que se muestra en el app es para el humano, que ve los productos del proyecto, las decisiones, etc, toda la documentación de lo que se está construyendo, que potencialmente sean muchos archivos que incluso pueden sobrepasar la capacidad de contexto del agente.

## 2. Pregunta que responde el bloque

**¿Qué recibe el principal en cada arranque para continuar bien, sin releer el proyecto y sin depender de lo que recuerda?**

Cuatro situaciones, un solo mecanismo:

| Situación | Ejemplo en los casos |
|---|---|
| Vuelves otro día | Caso 8 · Valeria, 10 días |
| El contexto cruza el umbral a mitad del trabajo | Caso 7 · Diego, «continúa hasta terminar» |
| Cambia el modelo | Spark → otro modelo a mitad de una sección |
| El proyecto no cabe en la ventana | Caso 3 · Rosa, 300 contratos |

## 3. Fronteras

| Recibe de | Qué |
|---|---|
| Bloque 1 (`../bloque-1/bloque-1.md` v4.10, `bloque-1-decisiones.md`, fichas, estados y flujos) | Vocabulario y momentos del ciclo: pedido, paso, parada con informe fijo, triaje, tarjetas, «por aclarar», premisa, historia, encargo a subagentes, y el grafo del proyecto (fichas §9), del que el estado de trabajo es una consulta |
| Bloque 3 (`../bloque-3/bloque-3.md` v2, propuesta) | Dónde vive (tablas del proyecto, solo escribe la app), lo entregado al agente en cada turno, cambios desde una sesión, aislamiento por proyecto |

**Fuera del bloque 2:** lo que ve el humano (es del bloque 1 y del 3). La memoria del agente no se muestra.

**Insumos:** doc 12 §9 (congelado; solo se toma lo que sirva), doc 13 (gentle-pi: documento de estado, espejo en Engram, umbral medido), código de Engram y de gentle-pi.

## 4. Plan

| Paso | Quién | Salida |
|---|---|---|
| 1 · Decisiones de enfoque | Eber adjudica | `decisiones.md` · hecho (B2-1 a B2-7) |
| 2 · Borrador | Claude | `bloque-2.md` · v1 hecha; v1.1 alineada con el bloque 1 v4.4; v1.2 alineada con la v4.9 y el grafo del proyecto; v2.0 rehecha sobre B2-1 a B2-4; v2.1 con el umbral y B2-5; v2.2 con B2-6; v2.3 tras la validación con el bloque 1; v2.4–v2.5 tras la auditoría; v3.0 con el arranque por tarea y B2-7; v3.1 tras la validación con el bloque 1 |
| 3 · Prueba de relevo | Subagentes | Un agente nuevo recibe solo el paquete y debe continuar. Un evaluador con la hoja oculta mira qué se perdió. Casos de §2 · hecho con los 10 casos (`../../casos-prueba/relevo-b2-v1/`) |
| 4 · Ajuste | Eber adjudica | Rehecho desde cero el 27-oct: B2-1 a B2-4 adoptadas; las anteriores, reemplazadas |
| 4b · Regresión de relevo | Subagentes | Casos 3, 7 y 10, con y sin nota del agente, sobre la v2.1 (base: `../../casos-prueba/relevo-b2-v1/`) · hecho: `../../casos-prueba/relevo-b2-v2/resultado.md` |
| 5 · Auditoría | 1 auditor independiente | Máximo 6 hallazgos |
| 6 · Cierre | Eber | Explica el bloque sin mirar |
