# Brief · evaluación de guiones simulados

## Contexto

tupaqnin es un harness para que un humano y agentes de IA cocreen documentos. Su objetivo, en literal: «el humano entiende por completo lo que se está haciendo y puede firmar y defenderlo de lo creado». El trabajo final no debe resultarle ajeno y el humano debe saber cómo se llegó a él. Ni el humano ni el agente limitan el avance del otro (DEC-003).

Varios simuladores escribieron guiones de sesiones de trabajo aplicando el **ciclo mínimo v4** (`ciclo-v4.md`). Los simuladores **no** conocían la hoja `evaluacion-oculta.md`. Tú sí la conoces: ahí está dónde nacería la deuda cognitiva en cada caso y qué preguntas haría un tercero.

## Archivos (en esta carpeta)

- `ciclo-v4.md` — el diseño probado. Léelo entero.
- `caso-02.md`, `caso-07.md` — lo que recibió cada simulador.
- `evaluacion-oculta.md` — tu hoja: momento de deuda y preguntas de un tercero.
- `guiones/caso-02-A.md`, `guiones/caso-02-B.md`, `guiones/caso-07-A.md`, `guiones/caso-07-B.md` — los guiones que evalúas. A = humano diligente; B = humano apurado.

## Qué evalúas en cada guion

1. **Protección.** ¿El ciclo evitó la deuda descrita en la hoja oculta? Responde sí, parcial o no, y di **en qué paso** del ciclo se protegió o falló.
2. **Defensa.** Con lo que el humano vio y respondió en el guion, ¿podría contestar las preguntas del tercero? Responde una línea por pregunta: sí, parcial o no, y por qué.
3. **Hallazgos sobre el ciclo** (máximo 2 por guion). Clasifica cada uno:
   - **falla:** el ciclo hace algo que daña los objetivos;
   - **falta:** el ciclo no dice qué hacer en algo necesario. Incluye las marcas `[EL CICLO NO LO DICE]` que lo merezcan; no todas lo merecen;
   - **choque:** dos reglas del ciclo se contradicen;
   - **simulación:** el simulador no aplicó bien la v4. No es culpa del ciclo, pero indica que una regla es fácil de malinterpretar;
   - **caso corto:** el caso no pone a prueba algo que debería.
4. **Marcas del simulador:** ¿cuáles son reales y cuáles no? Responde en una línea.

## Reglas

- Cita en literal la fila del guion (con su número) y la parte de `ciclo-v4.md` que sostiene cada juicio.
- Marca `[inferencia]` cuando razones sin texto.
- No propongas mecanismos nuevos. Para cada falla, falta o choque, propón como mucho una corrección de una línea y di cuántos estados o señales agrega (lo ideal es 0–1). Se prefiere quitar o precisar antes que agregar.
- Prohibido rellenar. Si un guion no tiene hallazgos, dilo.
- Busca lo que se repite entre los guiones: un hueco que aparece en los dos perfiles o en los dos casos pesa más.

## Formato de salida (fijo, en español, máximo unas 1 500 palabras)

```
## Caso <N> · Perfil <X>
- Protección: sí | parcial | no — paso: … — porque: …
- Defensa: P1 … / P2 … / P3 …
- Hallazgos:
  - H<n> · <tipo> · <título> — fila <n>: «…» · ciclo: «…» · corrección: … (agrega n)
- Marcas: …

(repetir para los 4 guiones)

## Lo que se repite
- …

## Sobre la prueba misma
- ¿Los casos y los perfiles pusieron a prueba el ciclo? ¿Qué cambiarías en el brief o en los casos antes de correr los 10?
```

Tu respuesta final debe ser el informe completo.
