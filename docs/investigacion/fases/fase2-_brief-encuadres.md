# Brief común, fase 2: arquitectura v1 desde distintos encuadres

## Proyecto

Harness ligero, de uso propio primero y luego repo público, para co-construir **documentos** con agentes de IA. El objetivo es que el humano no se vuelva cuello de botella ni acumule deuda cognitiva.

- **Usuario:** economista que hace investigación, metodologías e informes; ejemplo, la metodología de SyE de la gestión estratégica del Estado, con versiones v5 → v7. Usa Claude Code y gentle-ai.
- **Nombre provisional:** `hamutay` (quechua: comprender o reflexionar a fondo).

## Tesis

Para quien explora, construye y debe responder por documentos co-construidos con agentes:
- que la co-construcción sea genuina, con el humano generando en los momentos que definen el trabajo (encuadre, ideación, criterios, decisiones clave) y el agente en el resto;
- que criterios y decisiones se conviertan en evidencia explícita, versionada y con señales contrarias;
- para que el humano pueda decidir, apropiarse y defender sin releerlo todo.

## Lecturas obligatorias

1. `/home/claude/acervo-mecanismos-cuello-de-botella.md`: mecanismos M01–M44, casuísticas C0–C12, huecos H1–H8.
2. `/home/claude/proyecto/fase1/gentle-ai-tecnico.md`: revisión técnica de gentle-ai v4.
3. `/home/claude/proyecto/fase1/engram.md`: revisión de Engram.
4. `/home/claude/informe-investigacion-cuello-de-botella.md`: evidencia académica y de industria (consultar si hace falta).

## Hechos de gentle-ai a respetar

- **Modelo y esfuerzo:** se fijan por rol en el frontmatter (`model:`, `effort:`) con presets. No hay selección dinámica por complejidad. En Claude Code solo los agentes `jd-*` y `review-*` reciben modelo; los subagentes ODD corren con el default. Los subagentes propios deben fijar modelo y esfuerzo en su propio frontmatter.
- **ODD:** se inyecta como obligatorio en cada request y no se puede apagar.
  - Su formato: Specs `S#` literales, Tasks `T#`, Log.
  - Ante un cambio, solo se toca el `S#` afectado.
  - Asume código: commits, tests, `git diff`.
- **Skills propias:** viven en `<proyecto>/.claude/skills/` o `~/.claude/skills/`, sobreviven a `sync` y tienen prioridad las del proyecto.
- **Otras piezas:**
  - La persona fuerza respuestas cortas y una sola pregunta, sin menús; existe `--persona custom`.
  - RDD no revisa prosa.
  - Judgment Day (dos jueces ciegos, máximo dos rondas) es reutilizable como skill.
  - Lossless Blocking Prompts sirve para compuertas humanas.
- **Engram es memoria del agente, no expediente humano:**
  - Tipos libres y versionado por `topic_key`.
  - Conflictos detectados por búsqueda léxica, con juicio del agente.
  - Recomendación: un expediente Markdown en git como fuente de verdad y Engram solo como índice.
- **Lección:** el autor retiró SDD (fases pesadas) por ODD (flujo proporcional). Hay que evitar ceremonia.

## Huecos que gentle-ai no resuelve (foco)

- **H1, H3, H4:** criterio vivo. Criterios que emergen de reacciones, se versionan, alimentan la spec y son auditables.
- **H2:** comprensión humana medible; poder defender lo construido.
- **H5:** reentrada centrada en el humano, en sus decisiones y en lo que olvidó.
- **Co-generación humana** en encuadre e ideación.
- **C1, exploración:** divergencia y comparación sin leerlo todo.
- **Verificación de documentos sin tests:** niveles de riesgo y lentes de documento.
- **C2:** diff semántico y propagación de cambios entre documentos interdependientes.

## Formato de la propuesta

1. Principios del encuadre (3–5).
2. Componentes: qué es instrucción, skill, subagente, hook, plantilla o memoria. Para cada subagente, su modelo y esfuerzo.
3. Flujo: fases proporcionales, en qué momentos co-genera el humano y cuáles son las compuertas.
4. Artefactos: estructura del expediente y de las carpetas.
5. Relación con gentle-ai y Engram: qué se usa, qué se apaga, qué se reemplaza.
6. Qué huecos (H1–H8) y necesidades cubre, y cómo.
7. Riesgos, costos y decisiones abiertas.

Sé concreto: nombres de archivos, campos y reglas. Fundamenta en el acervo, citando M## o fuentes, cuando corresponda. Evita la ceremonia.
