# Brief · Validación de consistencia y coherencia del bloque 2 (tupaqnin)

Carpeta: `/home/claude/validacion-b2-v3/`. Solo lees; escribes únicamente tu informe en `informes/`.

## Qué es
tupaqnin es un harness para que un humano y agentes de IA cocreen documentos, y el humano entienda, firme y defienda lo creado. El diseño va en 3 bloques: 1 Núcleo (flujo humano-agentes), 2 Memoria del principal, 3 Plataforma. El bloque 1 está cerrado (v4.10; fichas v1.10; estados v1.9). El bloque 2 va en la v3.0: acaba de integrar un arranque por tarea con tope y decisiones en forma compacta (B2-1, B2-2) y sondas contra la reversión silenciosa (B2-7), que también tocaron fichas §3, §5, §7, estados §1 y una hipótesis de bloque-1.md.

Objetivo del bloque 2, en literal de su autor: «en todo momento le da al agente el contexto apropiado para continuar con el trabajo, y este no dependa de lo que recuerde; ser robusto a los casos donde se cambia de modelo, o cuando se continua el trabajo en varias sesiones de días diferentes».

Archivos:
- `bloque-2/bloque-2.md` — la idea (lo que lee el autor). `bloque-2/decisiones.md` — B2-1 a B2-7 con la respuesta del autor (rigen). `bloque-2/encargo.md` — encargo y plan.
- `bloque-1/bloque-1.md` (ciclo) · `bloque-1/bloque-1-fichas.md` (§7 contratos, §8.3 delegación, §9 grafo) · `bloque-1/bloque-1-estados.md` · `bloque-1/bloque-1-flujos.md` · `bloque-1/bloque-1-decisiones.md` (decisiones del bloque 1; consúltalo por búsqueda, es largo).
- `_plan-hackathon.md` (párrafo «Memoria del principal») · `_encuadre-bloques.md` (fronteras; se sabe que está desactualizado: «Expediente» → «Grafo del proyecto», no lo reportes) · `_reglas-de-trabajo.md` (reglas del diseño; su §6 «Estado» está desactualizado, no lo reportes) .

Reglas de diseño que aplican (de `_reglas-de-trabajo.md`): máx. 2 páginas por módulo; idea separada de especificación y de proceso; presupuesto de complejidad; «primero la respuesta»; sin nombres internos innecesarios; toda decisión con contraste.

## Qué buscas (solo esto)
- **Contradicción:** dos textos vigentes que dicen cosas incompatibles.
- **Desfase:** una decisión adoptada (respuesta del autor no vacía) que no está reflejada, o texto que sigue diciendo lo anterior («por confirmar», versiones viejas, nombres retirados).
- **Hueco:** algo que el bloque 2 usa del bloque 1 (término, estado, campo, evento, momento) y que el bloque 1 no define, o al revés: algo que el bloque 1 dice que entrega al bloque 2 y el bloque 2 no recoge.
- **Término:** el mismo concepto con dos nombres, o un nombre con dos sentidos.
- **Referencia rota:** una sección, id o archivo citado que no existe o no dice eso.

No es hallazgo: estilo, preferencias, mejoras, mecanismos nuevos, ni lo que el texto deja explícitamente como «sin probar», hoja de ruta o del bloque 3.

## Reglas
- **Máximo 6 hallazgos.** Prohibido rellenar: si hay menos, entrega menos. Si no hay nada, dilo.
- Cada hallazgo cita **en literal los dos lados** (archivo y sección o línea).
- Corrección mínima de una línea; di qué archivo se toca. Preferir precisar o quitar antes que agregar.
- Gravedad: **alta** (un implementador haría algo distinto según qué archivo lea) · **media** (confunde, pero se resuelve con el contexto) · **baja**.
- `[inferencia]` cuando razones sin texto.

## Formato (máx. ~600 palabras) en `informes/<tu-nombre>.md`
```
# Validación · <alcance>
**Respuesta:** <una línea: consistente | consistente con n ajustes | inconsistente>

| # | Tipo | Gravedad | Dónde (A) «cita» | Dónde (B) «cita» | Corrección (archivo) |
|---|---|---|---|---|---|

## Lo que revisé y está bien
- <máx. 5 líneas>
```
Respuesta final: solo «Informe en informes/<tu-nombre>.md» y el número de hallazgos por gravedad.
