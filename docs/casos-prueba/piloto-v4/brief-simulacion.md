# Brief común · simulación de casos contra el ciclo mínimo de tupaqnin

## 1. Qué es tupaqnin (en literal, de su autor)

> «Construimos un harness que permite el trabajo conjunto humano-AI donde el humano entiende por completo lo que se está haciendo y puede firmar y defenderlo de lo creado.»

> «tupaqnin nace para que el humano pueda construir con el agente en conjunto, aprovechando la potencia del agente en análisis, investigación, conocimiento de base, uso de subagentes para ejecutar tareas diversas al mismo tiempo, pero que el trabajo final no se vea como algo extraño para el humano, algo que no entienda lo que es o cómo se construyó, cómo se llegó a ello.»

## 2. Qué se prueba

Si el **ciclo mínimo v4** (`ciclo-v4.md`, en esta misma carpeta) funciona en un caso realista. No se prueba si el diseño es bueno en abstracto: se prueba qué hace el ciclo, paso a paso, cuando un humano concreto trabaja con tupaqnin en un caso concreto.

## 3. Tu papel

Escribes el **guion** de una sesión de trabajo entre un humano (con el perfil que te toca) y tupaqnin. tupaqnin incluye al agente principal, a los subagentes y a la app, y debe comportarse **exactamente** como dice `ciclo-v4.md`.

Reglas:

1. **Aplica el ciclo v4 al pie de la letra.** Cada acción de tupaqnin debe salir de algo que dice `ciclo-v4.md` (diagrama, §1–§4). No uses conocimiento de versiones anteriores del diseño ni inventes mecanismos.
2. **Si el ciclo no dice qué hacer, no lo inventes.** Marca `[EL CICLO NO LO DICE: …]` con lo que faltaría decidir y sigue con la opción más neutra, declarando cuál elegiste. Si dos reglas chocan, marca `[CHOCAN: regla A / regla B]`. Si una regla es ambigua, marca `[AMBIGUO: lectura 1 / lectura 2]`. Estas marcas son el resultado más valioso. No las escondas.
3. **El humano se comporta como la persona real de su perfil**, no como alguien que conoce el ciclo. No lee lo que no le interesa, se equivoca, olvida y tiene prisa si su perfil lo dice. Tiene las creencias y expectativas que le da el caso, incluso las equivocadas, y solo las corrige si algo en el guion se lo muestra.
4. **El agente tiene el conocimiento y los hallazgos que le da el caso,** y los descubre en el momento en que su trabajo lo llevaría a encontrarlos.
5. **Sé concreto:** lo que el humano escribe y lo que ve en pantalla va entre comillas, con texto realista y breve.
6. **No evalúes.** No digas si el ciclo protegió o no al humano. Otro agente lo evaluará.

## 4. Formato de salida (fijo)

Escribe el guion en el archivo que te indiquen, en español, con un máximo de unas 1 200 palabras:

```
# Guion · Caso <N> · Perfil <A|B>

| # | Humano (hace / escribe) | tupaqnin (qué muestra y quién lo hace: principal, subagente, app) | Paso del ciclo v4 | Marca |
|---|---|---|---|---|
| 1 | … | … | … | … |

(entre 12 y 20 filas)

## Estado final
- Secciones: <cuáles firmadas, cuáles no y por qué>
- Lo que el humano vio de cada decisión importante: <lista breve>
- Lo que quedó pendiente o «por aclarar»

## Marcas
- <cada marca [EL CICLO NO LO DICE] / [CHOCAN] / [AMBIGUO], en una línea, con el número de fila>
```

Tu respuesta final debe ser solo: «Guion escrito en <ruta>», más el número de marcas de cada tipo.
