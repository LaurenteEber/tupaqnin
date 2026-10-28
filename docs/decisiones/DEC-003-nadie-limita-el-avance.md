---
id: DEC-003
pregunta: ¿Cómo se asegura que el humano comprenda lo creado sin que humano o agente limiten el avance?
elegida: Nadie bloquea la construcción; quien propone explica; la comprensión se comprueba con actos y la única puerta es "listo para firmar"
estado: confirmado
generada_por: conjunto
critico: true
distingue: DEC-002
cita_humana: "en ninguna manera el humano o IA deben presentarse como un limitante al progreso de la construcción; si el agente IA conoce más propone y aporta siempre explicando la propuesta y asegurándose de que el humano entienda. El harness es de cocreación."
fecha: 2026-10-16
---

# DEC-003 · Nadie limita el avance

**Elegida:**

1. **Construir nunca se bloquea.** Ni el humano ni el agente son compuerta del avance. Cualquiera propone en cualquier momento.
2. **Quien propone explica.** Cuando el agente sabe más, propone y aporta, siempre explicando: una o dos razones, en contraste con la alternativa ("A en vez de B porque…; elegiría B si…"), y con un esquema cuando reemplaza texto.
3. **La comprensión se comprueba con un acto**, no con lectura ni con un "de acuerdo": aplicar, predecir, objetar o reformular. Aceptar sin justificar no frena nada; solo deja la sección como *no firmable*.
4. **También en sentido inverso.** El agente devuelve lo que entendió del humano (intención, criterios, razones) para que lo corrija. Los criterios se guardan versionados fuera del chat.
5. **Una sola puerta: "listo para firmar".** Ahí se exige explicar sin mirar, cero marcadores `[POR ACLARAR]` y comprensión en ambas direcciones.

## Relación con DEC-002

DEC-002 sigue vigente: codirección y comprensión compartida comparable. DEC-003 la **distingue** en un punto: la "generación humana primero" (G1 del diseño v1.2) deja de bloquear y pasa a ser una invitación opcional, y los lotes de decisiones críticas (G3) se reemplazan por una clasificación de riesgo que se resuelve en la puerta de firma.

## Razones

La evidencia completa está en [09](../investigacion/09-consolidacion-preliminar-cocreacion.md).

- **Explicar no basta.** Las explicaciones de la IA suben la aceptación aunque la IA se equivoque (Bansal et al. 2021). En tutoría, las explicaciones del tutor solo se asocian a aprendizaje superficial (Chi et al. 2001).
- **Comprender es hacer.** La comprensión aparece cuando la persona produce algo propio (ICAP, Chi y Wylie 2014). Preguntar después de que la IA genere preserva la comprensión que delegar quita (Shen y Tamkin 2026).
- **La fricción va en la firma, no en el flujo.** "Decidir antes de ver la IA" reduce la dependencia excesiva, pero es la intervención peor valorada (Buçinca et al. 2021). Un tutor que guía en vez de resolver no daña el aprendizaje (Bastani et al. 2025).
- **Conduce quien tiene el conocimiento del tramo.** En programación en pareja, lo que frena es la brecha de conocimiento específico, y las parejas complementarias son las más eficaces (Zieris y Prechelt 2021).
- **Los criterios no sobreviven en el chat.** El cumplimiento de una preferencia ya dicha cae por debajo del 10 % a los 10 turnos (PrefEval 2025).

## Descartes

| Opción | Razón |
|---|---|
| Humano primero con bloqueo (G1 de v1.2) | Frena al agente justo donde sabe más. Se conserva como opción, no como regla |
| Lotes de decisiones críticas en cada paso (G3) | Convierte al humano en una cola; la firma basta en lo material |
| Confiar en la explicación del agente como garantía | Sube la aceptación sin subir la precisión |
| Comprobar comprensión en cada frase | Comprobar por decisión rinde igual que por subpaso (Kulik y Fletcher 2016) y cuesta mucho menos |

## Contrapeso

- Sin bloqueos puede acumularse mucho contenido no firmable y la puerta final volverse una revisión enorme. Se mitiga con ciclos cortos y mostrando siempre cuánto falta por firmar.
- Si los actos se sienten como examen, el humano los saltará. Deben ser de una línea y solo sobre lo causal.
- El teach-back del agente y el detector de desenganche no tienen estudio que los respalde; son inferencias a probar.

## Pendiente

- En la aplicación manual (Fase 0 del ROADMAP), probar qué acto da mejor señal por tiempo invertido y si la puerta única basta.
