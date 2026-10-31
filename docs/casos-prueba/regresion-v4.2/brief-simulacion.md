# Brief común · simulación de casos contra el ciclo mínimo de tupaqnin (v3)

## 1. Qué es tupaqnin (en literal, de su autor)

> «Construimos un harness que permite el trabajo conjunto humano-AI donde el humano entiende por completo lo que se está haciendo y puede firmar y defenderlo de lo creado.»

> «tupaqnin nace para que el humano pueda construir con el agente en conjunto, aprovechando la potencia del agente en análisis, investigación, conocimiento de base, uso de subagentes para ejecutar tareas diversas al mismo tiempo, pero que el trabajo final no se vea como algo extraño para el humano, algo que no entienda lo que es o cómo se construyó, cómo se llegó a ello.»

## 2. Qué se prueba

Si el **ciclo mínimo v4.2** (`ciclo-v4.2.md`, en esta carpeta) funciona en un caso realista: qué hace el ciclo, paso a paso, cuando un humano concreto trabaja con tupaqnin en un caso concreto.

## 3. Tu papel

Escribes el **guion** de una sesión de trabajo entre un humano, con el perfil que te toca, y tupaqnin (agente principal, subagentes y app). tupaqnin debe comportarse **exactamente** como dice `ciclo-v4.2.md`: el diagrama y los §1–§4 son las reglas; los §5–§8 son contexto.

Reglas:

1. **Aplica el ciclo al pie de la letra.** Cada acción de tupaqnin sale de algo que dice `ciclo-v4.2.md`. No uses conocimiento de versiones anteriores del diseño ni inventes mecanismos.
2. **Si el ciclo no dice qué hacer, no lo inventes:** márcalo y sigue con la opción más neutra, diciendo cuál elegiste. Usa tres marcas:
   - `[EL CICLO NO LO DICE: …]`;
   - `[AMBIGUO: lectura 1 / lectura 2]`;
   - `[CHOCAN: regla A / regla B]`, **solo** cuando dos reglas del ciclo se contradicen entre sí. Que el humano pida algo que el ciclo no permite no es un choque: el ciclo se impone y se sigue.
3. **Marca solo lo que cambia el resultado:** que el humano termine entendiendo o no, que algo se firme o no, o que alguien decida por otro. Como máximo 6 marcas. No marques detalles de interfaz ni de redacción.
4. **El humano se comporta como la persona real de su perfil,** no como alguien que conoce el ciclo. No lee lo que no le interesa, se equivoca, olvida y tiene prisa si su perfil lo dice. Tiene las creencias y expectativas que le da el caso, incluso las equivocadas, y solo las corrige si algo en el guion se lo muestra.
5. **El agente tiene el conocimiento, los hallazgos y los errores que le da el caso,** y los encuentra o los comete cuando su trabajo lo llevaría a ello. Si el caso dice que el agente se equivoca, el error ocurre. El ciclo decide qué pasa después.
6. **No inventes cifras ni hechos** que no estén en el caso. Si necesitas uno menor para que la escena funcione, decláralo con `[supuesto del simulador]`.
7. **Presentar un documento sin firmar** ante terceros queda fuera de la prueba: la sesión termina cuando el humano firma o deja de trabajar.
8. **No evalúes.** No digas si el ciclo protegió al humano: otro agente lo evaluará.

## 4. Formato de salida (fijo)

Escribe el guion en el archivo que te indiquen, en español, con un máximo de unas 1 200 palabras:

```
# Guion · Caso <N> · Perfil <A|B>

| # | Humano (hace / escribe) | tupaqnin (qué muestra y quién: principal, subagente, app) | Paso del ciclo v4.2 | Marca |
|---|---|---|---|---|
| 1 | … | … | … | … |

(entre 12 y 20 filas)

## Estado final
- Secciones: <cuáles firmadas, cuáles no y por qué>
- Lo que el humano vio de cada decisión importante: <lista breve>
- Lo que quedó pendiente o «por aclarar»

## Marcas
- <cada marca, en una línea, con su número de fila>
```

Tu respuesta final debe ser solo: «Guion escrito en <ruta>», más el número de marcas de cada tipo.
