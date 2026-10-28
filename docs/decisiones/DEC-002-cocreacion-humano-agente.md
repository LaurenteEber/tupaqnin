---
id: DEC-002
pregunta: ¿Cómo se reparten la conducción y el aporte entre humano y agente?
elegida: Codirección con comprensión compartida comparable
estado: confirmado
generada_por: conjunto
critico: true
distinguida_por: DEC-003
fecha: 2026-10-11
---

# DEC-002 · Cocreación humano–agente

> Distinguida por [DEC-003](DEC-003-nadie-limita-el-avance.md): la generación humana primero (G1) ya no bloquea; nadie limita el avance.

**Elegida:** humano y agente **codirigen** el razonamiento y la construcción. Cualquiera de los dos conduce donde comprende más: el agente puede aportar más que el humano donde su comprensión es mayor, y el humano no lo limita ni queda reducido a aprobador. La meta es una **comprensión compartida comparable y bilateral**: el humano puede explicar, defender y corregir lo construido, y el agente también explicita la intención, los criterios y las decisiones del humano (no solo las razones de su propia producción). No se exigen aportes idénticos ni igualdad instantánea; aportes distintos no implican la misma responsabilidad final sobre el documento. Humano y agente pueden cuestionar y reformular conjuntamente los objetivos, los criterios y el encuadre; el humano no ejerce un liderazgo epistémico exclusivo, pero conserva la capacidad de decidir los compromisos que asume sobre el documento. La generación humana primero (G1 del diseño v1.2) es una **salvaguarda contextual** de los momentos que definen el trabajo, no un límite permanente al aporte del agente.

## Razones

Las razones siguientes derivan del objetivo del proyecto y de riesgos de diseño; el mecanismo y su efectividad quedan como **no verificados** hasta el análisis del corpus y el piloto.

- **Objetivo del proyecto.** El problema que tupaqnin ataca es la deuda cognitiva: documentos que el humano firma pero no comprende del todo. La codirección apunta a eso: que la conducción pueda moverse según quién entiende más, sin que el aporte del agente quede acotado por regla general.
- **Riesgo de diseño.** Un reparto fijo "humano lidera, agente ejecuta" puede dejar al humano conduciendo incluso donde no comprende, y así terminar firmando lo que no puede defender. Es una hipótesis de riesgo, no un resultado empírico establecido.
- **La igualdad de aportes no es la meta.** Comparar cantidades de contribución no mide comprensión ni defensa; exige métricas sin valor para el objetivo.
- **La salvaguarda contextual sigue siendo valiosa.** En el arranque y en las decisiones críticas, la generación humana primero ancla encuadre y criterios; lo que se descarta es convertirla en límite permanente.

## Descartes

| Opción | Razón |
|---|---|
| Humano lidera, agente ejecuta | Va contra el objetivo declarado: reduce al agente a instrumento y deja al humano conduciendo incluso donde no comprende, con riesgo de terminar firmando lo que no defiende. Que este modelo cause más deuda cognitiva es una hipótesis de diseño, no un hecho establecido |
| Igualdad de aportes (humano y agente generan lo mismo) | Confunde la meta (comprensión compartida) con un método; medir la participación en partes iguales no dice nada sobre la defensa del documento |
| Igualdad instantánea por diseño | La comprensión comparable se construye con contacto y evidencia (explicar sin mirar, defensa en ambos sentidos), no se decreta repartiendo el trabajo |
| Límite universal al aporte del agente (generación humana primero, siempre) | Frena al agente justo donde comprende más y convierte una salvaguarda del arranque en un límite permanente, con el humano como cuello de botella |

## Contrapeso

- La codirección sin un reparto fijo es más difícil de gobernar: decidir quién conduce en cada caso puede dar lugar a negociación continua. Los contactos humanos del diseño v1.2 (G1, G3, explicar sin mirar) quedan como anclas mecanizables, reinterpretadas como salvaguardas contextuales y no como límites.
- La comprensión bilateral agrega texto en ambos sentidos: el agente explicita sus razones y también la intención y los criterios del humano, y el humano revisa. Suma carga de lectura y de revisión.
- Riesgo de deriva hacia que el agente conduzca por defecto y el humano se convierta en aprobador sin darse cuenta; lo mitigan los contactos humanos y la defensa calificada por el humano ya previstas en la arquitectura.
- La responsabilidad final no se reparte por simetría: que los aportes sean desiguales no implica la misma responsabilidad legal sobre el documento.

## Pendiente

- Definir, en la fase de análisis del corpus de investigación, cómo se traduce la codirección en mecanismos concretos del MVP, qué salvaguardas contextuales se conservan y con qué evidencia se verifica el mecanismo.
