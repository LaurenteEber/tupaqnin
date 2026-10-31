---
titulo: Simulación de los 10 casos contra el ciclo v4.1
fecha: 2026-10-26
relacion: proceso, no producto; origen de las propuestas C1–C6 y de la recomendación sobre P2 en diseno/bloque-1-decisiones.md
---

# Simulación de los 10 casos · ciclo v4.1

**Método:** fueron 18 guiones. Son 8 casos nuevos con dos perfiles cada uno, A diligente y B apurado, más la regresión de los casos 2 y 7 con el perfil B. Los simuladores no conocían dónde nace la deuda. Después, 3 evaluadores independientes revisaron los guiones por grupo, y Claude verificó contra los guiones las filas que sostienen cada propuesta. Las evaluaciones completas están en `evaluaciones/`.

## Resultado

| | Perfil A (diligente) | Perfil B (apurado) |
|---|---|---|
| Protegió | 7 de 8 (caso 9: parcial, porque los códigos quedaron en el texto firmado) | 3 de 10 sí; 7 de 10 parcial |
| Por dónde entró la deuda | Casi nunca | Tarjetas y preguntas respondidas con «ok» y sin «porque» |
| Qué la frenó | Las comprobaciones y el «por aclarar» | Casi siempre, solo la firma (Wonder) |

**Regresión:** la v4.1 corrigió los tres problemas del piloto en los dos casos. «Continúa hasta terminar» ahora tiene salida, lo importante aparece primero y las opciones se presentan con su riesgo.

**P2 (una tarjeta crítica sin «porque»):** en 8 de los 9 guiones del perfil B, el «ok» dejó entrar deuda que el humano no podía defender. La excepción es la regresión del caso 2. La misma fuga aparece en las respuestas a «por aclarar» («B, como dije», «ok» sin elegir).

## Lo que se repite (base de C1–C6)

1. **El agente fija el sentido de los términos del humano** («campo», «calle comercial», «cerraría si…», «pago por uso», «nacional»). La comprobación de contradicción usa esa misma lectura, así que no salta. Aparece en 11 guiones, en los casos 1, 3, 5, 6, 8, 9 y 10.
2. **La firma no está cerrada.**
   - En el segundo intento, el humano aprueba repitiendo lo que Wonder le acaba de mostrar (1B, 3B, 4B).
   - «No coincide» cuando el texto está bien no lleva a ningún paso (10A, 2B, 7B).
   - «Coincide» se lee de dos formas: 10B laxa, 2B estricta.
3. **Editar lo firmado y la reentrada** tienen lecturas opuestas:
   - quién puede editar una sección firmada (6A, 6B);
   - si la edición del humano pasa por las comprobaciones (4A, 4B);
   - qué informe ve el humano al volver si el último cambio fue suyo (8A, 8B).
4. **El texto no se escribe para su lector:** códigos y relato del proceso llegan al texto firmado (9A, 9B; L-11).
5. **El juez y las rondas:**
   - «Necesita tu decisión» y «por aclarar» son dos estados para lo mismo (5B);
   - el humano no descarta las fallas del juez antes de que se corrijan (6A);
   - nadie decide si repetir una ronda vale lo que cuesta (5A, 5B).

## Sobre la prueba

- Las marcas por guion bajaron de 11–14 a 2–5, y la mayoría fueron reales.
- Hubo errores de simulación menores: preguntas que el humano ya había respondido y un estado final que contradecía una fila. Ninguno cambia las conclusiones.
- **Límite:** un LLM que hace de humano diligente es más ordenado que una persona real. Los resultados del perfil A son optimistas.
