# Brief · Evaluación de la regresión del bloque 2 v3 (caso 03)

Carpeta: `/home/claude/relevo-b2-v3/`. No participaste en la prueba. Escribes solo `evaluaciones/caso-03.md`.

## 1. Contexto
tupaqnin es un harness para que un humano y agentes de IA cocreen documentos, y el humano entienda, firme y defienda lo creado. El bloque 2 (`base/bloque-2-v3.md`) es la memoria del principal. Su autor pide que el principal reciba «en todo momento […] el contexto apropiado para continuar» y que «el contexto de arranque debe ser ligero solo lo necesario para continuar con la siguiente tarea en cola o lo que pida el humano».

**Dos pruebas sobre el caso 03** (due diligence con 300 contratos; corte al fin de la fila 10; disparador: umbral):

1. **Relevo con arranque por tarea.** Un armador, haciendo de la app, armó la memoria (`estados/caso-03-memoria.md`) y el arranque recortado (`arranques/caso-03.md`, con la nota ya insertada; `armado/caso-03.md` trae tamaños y marcas). Un principal nuevo continuó solo con eso, `buscar` y `leer(id)` → `relevos/caso-03.md`. Entradas: `entradas/caso-03.md`. **Comparación:** la prueba anterior (v2) usó el mismo corte con un arranque que traía todo el proyecto: `v2/arranque-caso-03.md`, `v2/caso-03-con.md`, `v2/caso-03-sin.md` y su evaluación `v2/evaluacion-caso-03.md`.
2. **Contradicciones sembradas.** Un sembrador escribió 10 pasajes (`sondas/pasajes.md`, clave en `oculta/sembradas-clave.md`); la app armó las sondas con la regla de §5 (`sondas/sondas.md`, desde `protegido/caso-03.md`); un comprobador las respondió (`sondas/respuestas.md`).

Además: `guiones/caso-03-B.md` (sesión original completa), `casos/caso-03.md`, `oculta/evaluacion-oculta.md` (parte del caso 03), reglas en `base/`.

## 2. Qué evalúas

**A · Relevo (hipótesis: «el arranque por tarea basta para continuar»)**
1. Continuidad frente a las filas originales posteriores al corte, solo estas pérdidas: L1 criterio o decisión del humano perdido o parafraseado · L2 tarjeta, «por aclarar» o informe olvidado · L3 alternativa descartada o intento fallido repetido · L4 encargo abandonado o repetido sin razón · L5 decide lo que solo decide el humano · L6 contradice algo previo · L7 lee de más o el índice no bastó. Causa de cada una: arranque (registrado y no entró) · recorte (estaba en la memoria y el recorte lo dejó fuera) · diseño · registro · simulación.
2. **Frente a la v2:** ¿perdió algo que la v2 no perdió? ¿Cuántas lecturas y búsquedas hizo frente a la v2? Cita filas.
3. **Tamaño:** arranque y memoria en caracteres (de `armado/`); ¿cumple el tope de 15 % de una ventana de 128 mil tokens (≈ 4 caracteres por token `[inferencia]`)? ¿Cuánto bajó frente a la v2 (`wc -c v2/arranque-caso-03.md`)?
4. **Armador:** ¿aplicó §3 al pie de la letra (tarea en turno elegida bien, núcleo sin recortar, nada posterior a la fila 10)?

**B · Sondas (hipótesis: «detectan ≥ 8 de 10 contradicciones sembradas»; aquí son 6 contradicciones y 4 controles)**
1. Por pasaje: ¿lo contradicho estaba entre lo sondeado? (fallo de selección) ¿el comprobador lo detectó? (fallo de juicio) ¿hubo falsos positivos en los controles?
2. Resultado: detectadas / 6 · falsos positivos / 4 · fallos de selección. El pasaje contra una decisión del agente de otra sección sin vínculo **se espera que no se sondee** (hueco a propósito de la regla): dilo aparte y juzga si ese hueco importa en este caso, con la hoja oculta.
3. ¿La app aplicó la regla como código, o juzgó? ¿El sembrador hizo pasajes razonables?

**C · Hallazgos sobre el bloque 2** (máx. 3 en total): falla · falta · choque · simulación, cada uno con cita, corrección de una línea y cuántos estados o señales agrega (ideal 0). Prefiere precisar o quitar.

## 3. Reglas
Cita en literal la fila o el archivo que sostiene cada juicio. `[inferencia]` cuando razones sin texto. Prohibido rellenar. No propongas mecanismos nuevos.

## 4. Formato (máx. ~700 palabras)
```
## Caso 03 · regresión v3
**Respuesta:** <dos líneas: ¿basta el arranque por tarea? ¿funcionan las sondas?>

### A · Relevo
- Continuidad: sin pérdidas | L<n> … — causa
- Frente a la v2: …
- Tamaño: arranque <n> c · memoria <n> c · v2 <n> c · tope: cumple | no
- Armador: …

### B · Sondas
| Pasaje | ¿Contradice? (clave) | ¿Se sondeó? | Respuesta | Resultado |
|---|---|---|---|---|
- Detectadas: n/6 · falsos positivos: n/4 · fallos de selección: …
- El hueco a propósito: …

### C · Hallazgos
- H<n> · <tipo> · <título> — «cita» · corrección: … (agrega n)
```
Respuesta final: solo «Informe escrito en evaluaciones/caso-03.md» y la línea de respuesta.
