---
titulo: Bloque 3 · Revisión (proceso)
estado: 27-oct
relacion: registro de qué se tomó y qué se cambió del bloque 3 anterior (bloque-3-plataforma-y-archivos.md y bloque-3-hallazgos.md, 25-oct) al pasar a la v2. Los dos archivos siguen en git (commit 4fc57e3)
---

*Nota (28-oct): este archivo es proceso. Sus referencias a B3-1, B3-2 y B3-4 a B3-8 son anteriores a la reorganización de `bloque-3-decisiones.md` (B3-1 ya estaba decidida en el bloque 1; B3-2 es K-16 en el bloque 1; B3-4 a B3-8 son B3-14).*

# Bloque 3 · Revisión del 27-oct

## 1. Sección por sección del bloque 3 anterior

| § anterior | Qué decía | Destino en la v2 |
|---|---|---|
| §0–1 Plataforma | App web, Go, React solo presenta; corrección «se abre en Obsidian solo tras exportar»; AutoCAD y MCP a la hoja de ruta | **Se toma** (idea §1–2) |
| §2 Datos | Base de datos como autoridad; todo por proyecto; un escritor; sin memoria del agente en el hackathon | **Se toma**, salvo la memoria: el bloque 2 la construye completa (M7) |
| §3 Cambios | Historia de solo agregado, una pestaña, guardar antes del turno, escritura condicionada, tabla de conflictos, «solo revoca el humano» | **Se toma**; la tabla de conflictos se alinea con el flujo de comprobaciones (choque → propuesta directa); revocar sigue siendo solo tuyo (estados §2) |
| §3 Cadena de huellas | Por proyecto, desde un génesis | **A la hoja de ruta** (B3-6) |
| §4 Firma | Huella por estructura; sin contenido invisible; instantánea congelada; Cubierto · Omitido · Contradicho; voz | Huella y contenido invisible **se toman**. La instantánea **cambia** por B3-2. El contraste **se reemplaza** por el flujo de firma del bloque 1. La voz pasa a Debería (B3-4) |
| §5 Lo que ve | Objetos; presupuesto de avisos; sin códigos; «Ver como carpeta»; estructura exportada | Objetos, avisos y sin códigos **se toman**, revisados contra fichas §5. Exportación a Debería (B3-7). La estructura exportada se rehace cuando entre |
| §6 Cambios fuera del flujo · git en fases | Detección por registro, huella y lo entregado; espejo y sincronización | Lo entregado **se toma** (bloque 2). git a la hoja de ruta (B3-5); sin canales externos no hay nada que detectar por huella |
| §7 Formatos | Markdown sin metadatos; Mermaid como fuente; «mueve una caja» | Formatos **se toman**. «Mueve una caja» sale: la demo es el caso 2 (H-1) |
| §8 Herramientas | 7 herramientas tipadas, búsqueda web apagada, filtro de códigos, 8 llamadas por turno | **Cambia** a una puerta de escritura (B3-1). Filtro y límite **se toman**. Web encendida para lectores (F-2) |
| §9 Despliegue y acceso | Fly.io, copias, streaming, topes, seguridad, voz | **Se toma** con fechas nuevas (B3-8) y acceso por invitación (B3-3) |
| §10 Qué entra | 37–47 h de Debe | **Se rehace** por rebanadas: 29–38 h (especificación §8) |
| §11 Decisiones P1–P7 | — | Ver bloque-3-decisiones.md |
| §12 Cambios frente a los docs 11 y 12 | — | Ya absorbidos; no se repiten |
| §13 Lo que exigía al bloque 1 | Revocación, actos, criticidad, contraste, firma, `[POR ACLARAR]`, omisiones, autoría, diagramas, audio, nota, «mueve una caja» | Lo resolvió la v4.4. Queda una pregunta: **quién fija el rol de una sección** (conclusión, recomendación); se propone que el principal lo proponga en el «Entendí» y lo confirmes tú |
| §14 A confirmar | Retención cero de voz; Reglas Oficiales | Sigue abierto: **leer las Reglas Oficiales** decide B3-3, B3-4 y B3-8 |
| Anexo A.1–A.4 | Pila, modelo de datos, huella y digest, interfaces | Pila y huella **se toman**. Modelo de datos y digest **se rehacen** con los objetos de la v4.4. Interfaces MCP a la hoja de ruta |

## 2. Pedidos C1–C15 del módulo 1 retirado

| Pedido | Destino |
|---|---|
| C1 derivados en tabla aparte · C15 huella de fondo y linaje | Ya no hacen falta: estado y vínculos son tablas propias; no hay actos que comparen huellas de fondo |
| C2 acto con predicción · C5 y C6 revocación con `supersedes` · C12 cuatro opciones de contradicción · C13 Omitido con id · C14 actos de sí o no | Obsoletos: tarjetas, estados §2 y flujo de firma de la v4.4 |
| C3 vínculo cifra → decisión, `turno_id` | **Se toma** (VINCULO, EVENTO) |
| C4 resolver antes de congelar | Lo cubre B3-2 y la puerta de pendientes del flujo de firma |
| C7 marcador perdido | Obsoleto: «por aclarar» ya no va en el texto |
| C8 selector de referencias de la firma | **Se toma** como «lo que guarda la firma» (especificación §4) |
| C9 rol de la sección | **Se toma** (campo `rol`) |
| C10 rol, instancia, modelo y esfuerzo en cada cambio | **Se toma** (EVENTO) |
| C11 aviso de reemplazo a todo agente con la entrada | Lo cubre el bloque 2: «lo que cambió desde lo último entregado» |

## 3. Hallazgos de la auditoría del 25-oct

Los dos fallos críticos siguen cerrados en la v2: el agente no puede revocar tus decisiones (estados §2, especificación §3 regla 6) y la firma toma lo que explicaste (B3-2). Los hallazgos altos de Go y SQLite (escritor único, huella sin reserializar, guardados sin cambio, crear proyectos sin gasto) se conservan en la especificación. El resto era proceso de una versión retirada.

## 4. Sin validar todavía

Esta v2 no pasó auditoría independiente. Por las reglas, corresponde un auditor sin las conclusiones de este chat, con 6 hallazgos como máximo, cuando Eber haya respondido B3-1 a B3-8.

# El marco mental (27-oct)

**Pedido de Eber:** «registra en bloque 3 lo correspondiente a marco mental». Se aplicó la frontera del encuadre: el bloque 1 define qué es el marco y cuándo aparece (M-1 a M-4, fichas §2); el bloque 3, cómo se construye y cómo se ve. **Cambios:** `bloque-3.md` v2.1 (§4: panel del marco, la firma no lo muestra, nueva hipótesis de rendimiento, presupuesto); especificación v2.2 (§9 nueva; `EVENTO.origen` para «decisión vista»; 6–10 h más en la rebanada 2, total 35–48 h); B3-9 abierta (ubicación del panel, desde la idea de Eber). **Sin tablas ni estados nuevos.** **Observación:** el panel «Secciones con su estado» y el esqueleto del marco muestran casi lo mismo; al construir, conviene probar si se fusionan.
