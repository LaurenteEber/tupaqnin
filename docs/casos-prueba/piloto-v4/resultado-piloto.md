---
titulo: Piloto de simulación · casos 2 y 7 contra el ciclo v4
fecha: 2026-10-26
relacion: proceso, no producto; origen de las propuestas P1–P6 en diseno/bloque-1-decisiones.md
---

# Piloto · casos 2 y 7 contra el ciclo v4

**Método.** Cada caso se simuló con dos perfiles de humano: A (diligente) y B (apurado). Fueron cuatro simuladores en paralelo, que no conocían dónde nace la deuda en el caso. Luego un evaluador independiente, que sí lo sabía, revisó los cuatro guiones, y Claude verificó contra los guiones las filas que cita. Las versiones reducidas de los casos, los briefs y los guiones están en esta carpeta.

## Resultado por guion

| Guion | ¿Protegió? | Defensa ante un tercero | Qué protegió / qué falló |
|---|---|---|---|
| 2 · A | Sí | 2 de 3 (no sabe qué pasa si el uso sube al 80 %) | El digesto mostró la ENAHO trimestral y Eber decidió; el cálculo contradijo sus «horas extra» |
| 2 · B | Parcial | 1 de 3 | Sin firma: Wonder la frenó. Pero confirmó tarjetas con «porque» vacío y siguió creyendo en las horas extra |
| 7 · A | Sí | 3 de 3 | «Tú decides» no se ejecutó; el horizonte quedó «por aclarar» y lo decidió Diego |
| 7 · B | Parcial | 0 de 3 | Sin firma. Pero eligió 25 años sin «porque», pulsó «ok» en 8 tarjetas sin leer y siguió creyendo que el riesgo es de menos de 1 % |

**Lectura:** con un humano diligente el ciclo protege. Con uno apurado, la firma es la última defensa y funciona, pero llega tarde: la deuda entra antes, por tarjetas confirmadas sin razón.

## Lo que se repite (base de P1–P6)

1. «Continúa hasta terminar» no tiene salida en el diagrama (3 de 3 guiones que lo usan).
2. Una tarjeta crítica se confirma con «ok» o con el «porque» vacío (los 2 guiones B).
3. La reentrada no abre con lo más importante: el 18 % de riesgo no apareció hasta que se preguntó (7B).
4. La idea o creencia del humano no tiene estado; solo se compara contra «lo confirmado» (3 guiones).
5. No está escrito qué cubre «por aclarar» ni cómo se presentan sus opciones: en 7B, la opción de 25 años se presentó como «más atractiva» sin su riesgo.
6. La pregunta de origen en la firma cubre la cifra, pero no el supuesto del que depende (2A).

**Menor:** quién cierra «¿se redujo la duda?»: el principal lo dice en el digesto y el humano lo corrige.

## Ajustes a la prueba antes de correr los 10 casos

- **[CHOCAN]** solo vale entre dos reglas del ciclo. «Tú decides» no choca: el ciclo lo resolvió bien.
- **Marcar solo lo que cambia el resultado.** Hubo entre 11 y 14 marcas por guion, muchas triviales.
- **No inventar cifras** que no están en el caso.
- **Que el agente se equivoque en algunos casos** (por ejemplo, un lector que trae la fuente que el humano descartó), para probar «contradice lo confirmado».
- **Cubrir mecanismos que ningún guion probó:** «cambió desde que la firmaste», la edición simultánea y el agente nuevo por contexto largo.
- **En el caso 7, el perfil diligente** debe distinguirse de B antes del final.
- **Pendiente de decisión de Eber:** si «¿y si el uso sube al 80 %?» (sensibilidad) es un objetivo del ciclo o se saca de la hoja del evaluador.
