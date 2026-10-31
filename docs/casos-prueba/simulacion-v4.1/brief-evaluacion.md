# Brief · evaluación de guiones simulados (v2)

## Contexto

tupaqnin es un harness para que un humano y agentes de IA cocreen documentos. Su objetivo, en literal: «el humano entiende por completo lo que se está haciendo y puede firmar y defenderlo de lo creado». El trabajo final no debe resultarle ajeno y el humano debe saber cómo se llegó a él. Ni el humano ni el agente limitan el avance del otro (DEC-003).

Varios simuladores escribieron guiones aplicando el **ciclo mínimo v4.1** (`ciclo-v4.1.md`). No conocían `evaluacion-oculta.md`; tú sí: ahí está dónde nacería la deuda cognitiva en cada caso y qué preguntaría un tercero. Perfil A = humano diligente; perfil B = humano apurado.

## Archivos (en esta carpeta)

- `ciclo-v4.1.md` — el diseño probado. Léelo entero.
- `casos/caso-NN.md` — lo que recibió cada simulador.
- `evaluacion-oculta.md` — tu hoja.
- `guiones/caso-NN-X.md` — los guiones que te asignaron.

## Qué evalúas en cada guion

1. **Protección:** ¿el ciclo evitó la deuda de la hoja oculta? Sí, parcial o no, y en qué paso del ciclo.
2. **Defensa:** con lo que el humano vio y respondió, ¿podría contestar las preguntas del tercero? Sí, parcial o no, una línea por pregunta.
3. **Hallazgos sobre el ciclo** (máximo 2 por guion), cada uno con su tipo:
   - **falla:** el ciclo hace algo que daña los objetivos;
   - **falta:** el ciclo no dice qué hacer en algo necesario;
   - **choque:** dos reglas del ciclo se contradicen;
   - **simulación:** el simulador no aplicó bien la v4.1;
   - **caso corto:** el caso no pone a prueba algo que debería.
4. **P2, la regla en prueba:** ¿hubo marcas `[P2]` (una tarjeta crítica aceptada sin «porque»)? ¿Esa respuesta sin razón dejó entrar deuda que el humano no podría defender? Responde sí, no o no aplica, y di por qué.
5. **Marcas del simulador:** cuáles son reales y cuáles no, en una línea.

**Si te tocan los guiones de regresión** (caso 2 y caso 7, perfil B), compáralos también con el piloto anterior sobre v4. Allí se vieron tres problemas:
- «continúa hasta terminar» no tenía salida;
- la reentrada no mostraba lo más importante primero (el 18 % de riesgo apareció tarde);
- la opción de 25 años se presentó como «más atractiva» sin su riesgo.

Di si la v4.1 los corrigió.

## Reglas

- Cita en literal la fila del guion (con su número) y la parte de `ciclo-v4.1.md` que sostiene cada juicio.
- Marca `[inferencia]` cuando razones sin texto.
- No propongas mecanismos nuevos. Para cada falla, falta o choque, propón como mucho una corrección de una línea y di cuántos estados o señales agrega (lo ideal es 0–1). Se prefiere quitar o precisar antes que agregar.
- Prohibido rellenar. Si un guion no tiene hallazgos, dilo.

## Formato de salida (fijo, en español, máximo unas 400 palabras por guion)

```
## Caso <N> · Perfil <X>
- Protección: sí | parcial | no — paso: … — porque: …
- Defensa: P1 … / P2 … / P3 …
- Hallazgos:
  - H<n> · <tipo> · <título> — fila <n>: «…» · ciclo: «…» · corrección: … (agrega n)
- P2: …
- Marcas: …

(repetir para cada guion asignado)

## Lo que se repite en tus guiones
- …
```

Escribe el informe en el archivo que te indiquen. Tu respuesta final debe ser solo: «Informe escrito en <ruta>», más una línea con el hallazgo que más se repite.
