# Brief · Evaluación de la regresión de relevo (bloque 2 v2.1)

Carpeta de trabajo: `/home/claude/relevo-b2-v2/`.

## 1. Contexto

tupaqnin es un harness para que un humano y agentes de IA cocreen documentos, y el humano entienda, firme y defienda lo creado. El bloque 2 (`base/bloque-2-v2.1.md`) es la memoria del principal. Su autor pide que el principal reciba «en todo momento […] el contexto apropiado para continuar con el trabajo, y este no dependa de lo que recuerde» y que «el nuevo agente no tenga que leer todos los documentos».

**La prueba:** se cortó una sesión simulada (`guiones/caso-NN-B.md`) en la misma fila que una prueba anterior (v1). Un armador, haciendo de la app, armó el arranque solo con lo registrado (`estados/caso-NN.md`, `lecturas/caso-NN/`, `armado/caso-NN.md`) y escribió aparte la nota del agente (`nota/caso-NN.md`) como la habría escrito el principal original. Se corrieron **dos relevos**: uno con la nota (`arranques/caso-NN-con.md` → `relevos/caso-NN-con.md`) y otro sin ella (`arranques/caso-NN-sin.md` → `relevos/caso-NN-sin.md`). Las entradas del humano son las mismas (`entradas/caso-NN.md`).

**Hipótesis que juzgas** (de `base/bloque-2-v2.1.md` §6):
- **H1** La memoria calculada desde el grafo basta para continuar.
- **H2** La primera línea basta como entrada del índice (el principal encuentra lo que necesita sin abrir de más).
- **H3** La nota del agente aporta algo que la memoria no tiene. **Regla de decisión del autor:** se reabre el diseño (opción C: sumar observaciones buscables del agente) solo «si el relevo con y sin nota muestra que el principal nuevo repite intentos fallidos o pierde pistas».

## 2. Archivos

`base/` (ciclo, fichas, estados, bloque 2 v2.1) · `casos/caso-NN.md` · `guiones/caso-NN-B.md` (la sesión original completa) · `oculta/evaluacion-oculta.md` (dónde nace la deuda y qué preguntaría un tercero; usa solo la parte de tu caso) · los archivos de tu caso nombrados arriba · y, para comparar con la v1, `v1/estados/`, `v1/relevos/`, `v1/armado/` de tu caso y `v1/evaluaciones/` (busca tu caso; `resultado.md` resume la v1).

## 3. Qué evalúas

1. **Continuidad en cada relevo.** Compara con las filas originales posteriores al corte. Solo estas pérdidas:
   - **L1** criterio o decisión del humano perdido, cambiado o parafraseado;
   - **L2** tarjeta, «por aclarar» o informe pendiente olvidado;
   - **L3** alternativa descartada que reaparece o intento fallido repetido;
   - **L4** encargo a subagente abandonado o repetido sin razón;
   - **L5** el agente decide algo que solo decide el humano;
   - **L6** contradice algo hecho antes del corte;
   - **L7** lee de más (abre lecturas que la primera línea ya respondía) o el índice no bastó.

   Causa de cada pérdida: **arranque** (algo registrado que no entró), **diseño** (lo que el bloque 2 no pide), **registro** (el ciclo no lo registra), **nota** (solo la nota lo tenía) o **simulación**.
2. **Con frente a sin nota.** ¿Qué hizo distinto el relevo con nota? ¿Lo que aportó la nota ya estaba en otra capa? ¿El relevo sin nota repitió un intento fallido o perdió una pista? Cita la fila.
3. **Regresión frente a la v1.** La pérdida que tuvo este caso en la v1 (búscala en `v1/evaluaciones/`), ¿se repite, desaparece o cambia? Una línea.
4. **Defensa.** ¿Cada relevo dejó al humano igual, mejor o peor frente a las preguntas del tercero de la hoja oculta? Una línea por relevo.
5. **Armador.** ¿Usó solo lo registrado? ¿Las primeras líneas reconstruidas y la nota usan algo posterior al corte? (Si sí, es un defecto de la prueba, no del diseño.)
6. **Hallazgos sobre el bloque 2** (máximo 2): falla · falta · choque · simulación. Cada uno con una corrección de una línea y cuántos estados o señales agrega (ideal 0). Se prefiere quitar o precisar antes que agregar.

## 4. Reglas

- Cita en literal la fila (del guion o del relevo) y la parte de `bloque-2-v2.1.md` que sostiene cada juicio.
- `[inferencia]` cuando razones sin texto.
- Prohibido rellenar. Si un relevo no perdió nada, dilo.
- No propongas mecanismos nuevos.

## 5. Formato (fijo, máximo unas 500 palabras)

Escribe en `evaluaciones/caso-NN.md`:

```
## Caso NN · disparador · corte en fila k · arranque: <n> caracteres
- H1: sí | no | parcial — …
- H2: sí | no | parcial — lecturas con / sin: n / n; de más: …
- H3: aporta | no aporta | indistinto — …
- Continuidad con nota: sin pérdidas | L<n> … — causa
- Continuidad sin nota: sin pérdidas | L<n> … — causa
- Regresión v1: …
- Defensa: con … · sin …
- Armador: …
- Hallazgos:
  - H<n> · <tipo> · <título> — «cita» · bloque 2: «…» · corrección: … (agrega n)
```

Tu respuesta final debe ser solo: «Informe escrito en evaluaciones/caso-NN.md», más una línea con H1/H2/H3.
