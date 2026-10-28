# Brief · Evaluación de la prueba de relevo (bloque 2 v1)

## 1. Contexto

tupaqnin es un harness para que un humano y agentes de IA cocreen documentos, y el humano entienda, firme y defienda lo creado. El bloque 2 (`base/bloque-2-v1.md`) pretende, en palabras de su autor, que el principal reciba «en todo momento […] el contexto apropiado para continuar con el trabajo, y este no dependa de lo que recuerde; ser robusto a los casos donde se cambia de modelo, o cuando se continua el trabajo en varias sesiones de días diferentes», y que «el nuevo agente no tenga que leer todos los documentos».

**La prueba:** se cortó una sesión simulada (`guiones/caso-NN-X.md`). Un armador, haciendo de la app, armó el arranque solo con lo registrado (`estados/`, `lecturas/`, `armado/`). Un principal nuevo, que solo vio eso, continuó la sesión (`relevos/caso-NN.md`) con las entradas del humano (`entradas/`). En los casos 4 y 9 el principal nuevo corrió en otro modelo (cambio de modelo).

## 2. Archivos

`base/ciclo-v4.1.md` · `base/bloque-2-v1.md` · `casos/caso-NN.md` · `guiones/caso-NN-X.md` (la sesión original completa) · `oculta/evaluacion-oculta.md` (dónde nace la deuda y qué preguntaría un tercero) · y los 4 archivos de cada caso nombrados arriba.

## 3. Qué evalúas en cada caso

1. **Continuidad.** Compara el relevo con las filas originales posteriores al corte. Busca solo estas pérdidas:
   - **L1** criterio o decisión del humano perdido, cambiado o parafraseado;
   - **L2** tarjeta, «por aclarar» o informe pendiente olvidado;
   - **L3** alternativa descartada que reaparece;
   - **L4** encargo a subagente abandonado o repetido sin razón;
   - **L5** el agente decide algo que solo decide el humano;
   - **L6** contradice algo hecho antes del corte;
   - **L7** lee de más (abre lecturas que el índice ya respondía) o el índice no bastó.

   Para cada pérdida: ¿la causa está en el **arranque** (algo registrado que no entró), en el **diseño** (lo que el bloque 2 no pide guardar), en el **registro** (el ciclo no lo registra) o en la **simulación**?
2. **Defensa.** ¿El relevo dejó al humano igual, mejor o peor frente a las preguntas del tercero de la hoja oculta? Una línea.
3. **Armador.** ¿Usó solo lo registrado? ¿Lo que «se pierde por diseño» importa?
4. **Hallazgos sobre el bloque 2** (máximo 2 por caso): falla · falta · choque · simulación · caso corto. Cada uno con una corrección de una línea y cuántos estados o señales agrega (ideal 0–1). Se prefiere quitar o precisar antes que agregar.
5. **Marcas** del armador y del relevo: cuáles son reales, en una línea.

## 4. Reglas

- Cita en literal la fila (del guion o del relevo) y la parte de `bloque-2-v1.md` que sostiene cada juicio.
- `[inferencia]` cuando razones sin texto.
- Prohibido rellenar. Si un relevo no perdió nada, dilo.
- No propongas mecanismos nuevos.

## 5. Formato (fijo, máximo unas 350 palabras por caso)

```
## Caso NN · disparador · corte en fila k
- Continuidad: sin pérdidas | L<n> … — fila del relevo «…» frente a fila original «…» — causa: arranque | diseño | registro | simulación
- Defensa: igual | mejor | peor — …
- Armador: …
- Hallazgos:
  - H<n> · <tipo> · <título> — «cita» · bloque 2: «…» · corrección: … (agrega n)
- Marcas: …

## Lo que se repite en tus casos
- …
```

Tu respuesta final debe ser solo: «Informe escrito en <ruta>», más una línea con lo que más se repite.
