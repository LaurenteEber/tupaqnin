# Encuadre 5: artefactos, datos y trazabilidad

*Fase 2 · hamutay v1 · 7 de octubre de 2026. Los ejemplos de GEE y de nichos son ilustrativos: no son decisiones reales.*

## 1. Principios

1. **El expediente es la fuente de verdad, y es Markdown en git.** Engram solo guarda punteros (engram.md §7.2). Si se borra Engram, no se pierde nada del humano.
2. **Un archivo por registro que tiene ciclo de vida; agregados para lo voluminoso.** Criterios, decisiones, descartes y supuestos cambian de estado y se discuten: van uno por archivo. Afirmaciones, fuentes, opciones y eventos de comprensión son muchos y cortos: van en un YAML por tipo.
3. **Los enlaces van en una sola dirección, hacia lo que te sostiene.** Lo inverso se calcula. Así la propagación (C2) es un recorrido del grafo con un script, y no hay enlaces que mantener a mano (M26).
4. **El diff semántico sale del frontmatter, no de la prosa.** Un cambio de sentido sube `version` y deja una línea `cambio:`. `git diff` de la prosa sigue disponible, pero no es lo primero que lee el humano (M19, M37).
5. **El humano confirma y el sistema lo comprueba.** Solo el humano pasa un registro a `confirmado` o `tomada`, y queda su cita textual. Las vistas se generan y nunca se editan.

## 2. Componentes

| Pieza | Tipo | Qué hace |
|---|---|---|
| Bloque `hamutay` en `CLAUDE.md` del proyecto, fuera de los marcadores `gentle-ai:` | Instrucción | Esquema mínimo, reglas de estado, "registro primero, Engram después" |
| `.hamutay/plantillas/{cri,dec,des,sup,sec}.md` | Plantilla | Frontmatter obligatorio por tipo |
| `hm` (un Python con solo stdlib, en `.hamutay/hm.py`) | Script | `nuevo`, `validar`, `impacto`, `cambios`, `deuda`, `vistas`, `reentrada` |
| `hm-registrar`, `hm-reentrada`, `hm-teachback` | Skills de proyecto | Envuelven el script y fijan cuándo preguntar |
| pre-commit → `hm validar` | Hook git | IDs únicos, enlaces que existen, estados válidos, `confirmado` con `cita` |
| SessionStart → `hm vistas && hm reentrada --breve` | Hook Claude | El humano entra por sus decisiones, no por el chat |
| SessionEnd → `hm nuevo ses` | Hook Claude | Archivo de sesión con IDs tocados |
| `hm-extractor` | Subagente, `model: sonnet`, `effort: medium` | Lee el Log y la sesión, y propone registros en estado `propuesto` (M04, M05) |
| `hm-verificador` | Subagente, `model: sonnet`, `effort: high`, solo lectura | Verifica afirmaciones de riesgo alto contra `fuentes.yaml` (M27) |
| `hm-impacto` | Subagente, `model: sonnet`, `effort: medium` | Revisa solo las secciones que marca `hm impacto` |

El teach-back y las compuertas los corre el agente principal: son diálogo, no delegación.

## 3. Flujo (visto desde los registros)

| Fase | Quién genera | Qué registro nace | Compuerta |
|---|---|---|---|
| Encuadre | Humano | `S#` en el encargo; `ideas-propias.md` antes de ver nada del agente (M12) | — |
| Exploración | Agente diverge | `OPC` en `opciones.yaml` | — |
| Criterios | Reacciones del humano → extractor | `CRI` propuesto, con la cita de la reacción | **G1:** confirmar o retirar el `CRI` |
| Construcción | Agente; humano en lo clave | `DEC`, `DES`, `SUP`, `AFI`, `SEC` | **G2:** tomar la `DEC` que afecta a ≥2 secciones o a un `S#` |
| Revisión | Verificador e impacto | `AFI` verificada o refutada; `revisar:` en `SEC` | **G3:** `SEC` de riesgo alto pasa a `cerrada` |
| Cierre | Humano explica | `COM` en `comprension.yaml` | **G4:** sin deuda de comprensión en `DEC` críticas |

Las compuertas usan Lossless Blocking Prompts: una pregunta, con el registro citado por ID.

## 4. Artefactos

### 4.1 Carpetas

```
gee-metodologia/                 # repo git; `engram init gee-metodologia`
  CLAUDE.md                      # bloque hamutay
  hamutay.yaml                   # humano: eber; prefijos; umbrales de riesgo
  odd/tasks/metodologia-v7.md    # encargo S#/T#/Log (ODD lo encuentra donde lo busca)
  ideas-propias.md               # lo que el humano pensó antes que el agente
  doc/                           # el documento: un archivo por sección
    SEC-01-proposito.md … SEC-09-anexos.md
  registro/
    criterios/CRI-004-indicadores-con-fuente-oficial.md
    decisiones/DEC-007-tres-niveles-de-madurez.md
    descartes/DES-002-indice-compuesto-unico.md
    supuestos/SUP-003-reporte-semestral.md
    afirmaciones.yaml  fuentes.yaml  opciones.yaml  comprension.yaml
  sesiones/SES-2026-10-07a.md
  insumos/                       # v6 original, normas (solo lectura)
  vistas/                        # generadas; en .gitignore
  .hamutay/  hm.py  plantillas/  visitas.yaml   # visitas.yaml local, no se versiona
```

Para GEE v6→v7: el primer commit importa v6 partida en `doc/SEC-*` y se etiqueta `v6-base`. Desde ahí, `hm cambios v6-base` da el diff semántico y `git diff v6-base` el textual.

### 4.2 Tipos, IDs y estados

Los IDs tienen prefijo de tres letras para no chocar con `S#/T#/L#` de ODD. Son correlativos, nunca se reutilizan y el slug puede cambiar. Los scripts resuelven por `glob ID-*`.

| Tipo | Archivo | Estados | Enlaza a |
|---|---|---|---|
| `CRI` criterio | uno | propuesto → confirmado → reemplazado / retirado | `origen` (SES, cita) |
| `DEC` decisión | uno | abierta → tomada → reemplazada / revertida | `criterios`, `supuestos`, `afirmaciones`, `descartes`, `spec` (S#) |
| `DES` descarte | uno | vigente / reabierto | `criterios` que lo descartan, `opcion` |
| `SUP` supuesto | uno | vigente → en_duda → falso | `afirmaciones`, `fuentes` |
| `AFI` afirmación | agregado | sin_verificar → verificada / refutada / ajustada | `fuentes`, `riesgo` |
| `FUE` fuente | agregado | — | — |
| `OPC` opción | agregado | en_juego → elegida (DEC) / descartada (DES) | `puntajes` por CRI |
| `SEC` sección | uno (en `doc/`) | borrador → revisada → cerrada | `depende_de` (DEC, CRI, SUP) |
| `COM` comprensión | agregado | ok / dudó / falló | `items` (ID@version) |
| `SES` sesión | uno | — | IDs tocados |

**Regla de versión.** Si cambia la redacción sin cambiar el sentido, se edita en el mismo ID. Si cambia el sentido, sube `version` y se escribe `cambio:`. Si cambia el objeto, se crea un ID nuevo y el viejo pasa a `reemplazado_por`. El agente propone cuál de los tres es; el humano confirma.

### 4.3 Ejemplos de archivos

`registro/criterios/CRI-004-indicadores-con-fuente-oficial.md`
```markdown
---
id: CRI-004
tipo: criterio
estado: confirmado
version: 2
cambio: "v2: admite registros administrativos, no solo INEI"
origen: [SES-2026-10-02a]
cita: "no quiero indicadores que obliguen a levantar encuestas nuevas; si el registro administrativo existe, vale"
confirmado_por: eber
confirmado_en: SES-2026-10-07a
reemplaza: null
---
Todo indicador de la v7 debe calcularse con una fuente oficial existente
(estadística o registro administrativo), sin levantar información nueva.
**Señal contraria:** los registros administrativos tienen cobertura desigual entre niveles de gobierno.
```

`registro/decisiones/DEC-007-tres-niveles-de-madurez.md`
```markdown
---
id: DEC-007
tipo: decision
estado: tomada
version: 1
spec: [S3]
criterios: [CRI-004, CRI-006]
supuestos: [SUP-003]
afirmaciones: [AFI-031, AFI-032]
descartes: [DES-002]
cita: "tres niveles; cinco no los distingue nadie en campo"
confirmado_por: eber
---
**Contexto** · **Opciones** (OPC-011, OPC-012) · **Por qué** · **Consecuencias** (M15)
```

`doc/SEC-05-escala-de-madurez.md` (encabezado y marca de afirmación)
```markdown
---
id: SEC-05
estado: revisada
riesgo: alto
depende_de: [DEC-007, CRI-004@2, SUP-003]
autoria: mixta
revisar: []          # lo llena `hm impacto`
---
La escala distingue tres niveles de madurez … las entidades reportan dos veces al año.<!--@AFI-031-->
```

`registro/afirmaciones.yaml`
```yaml
- id: AFI-031
  texto: "Las entidades reportan seguimiento dos veces al año"
  seccion: SEC-05
  fuentes: [FUE-008]
  estado: verificada
  riesgo: alto
  verificado_por: hm-verificador@SES-2026-10-07a
```

`registro/comprension.yaml`
```yaml
- id: COM-012
  sesion: SES-2026-10-07a
  items: {DEC-007@1: ok, CRI-004@2: dudo, SUP-003@1: ok}
  nota: "No supo explicar por qué v2 admite registros administrativos"
```

### 4.4 Relación con el formato S#/T#/Log de ODD

**Lo adopto y lo extiendo; no lo reemplazo.** El encargo vive en `odd/tasks/<entregable>.md` para que ODD lo reanude sin cambios (estrategia 2 de gentle-ai-tecnico §9.2).
- `S#`: lo que pidió el humano, textual. Un `S#` puede citar `CRI` y `DEC`, pero no los contiene: el criterio emergente no es requisito literal.
- `T#`: una tarea por sección o grupo de secciones (`T4 (S3) · SEC-05 · DEC-007`).
- `Log`: cronología en voz del humano. El extractor convierte entradas `L#` en registros propuestos, y el `origen` apunta a la `L#`.

Lo que ODD deja fuera ("no hay diario de decisiones exhaustivo") lo cubren los registros tipados. La regla ODD "solo se toca el `S#` afectado" se generaliza: **solo se tocan las `SEC` que `hm impacto` marca.**

### 4.5 Enlaces, propagación y diff semántico

- `hm impacto CRI-004` recorre al revés `depende_de`, `criterios` y `supuestos`. Resultado: DEC-007 → SEC-05, SEC-07. Escribe `revisar: [CRI-004@2]` en esas secciones y nada más.
- Un `depende_de` con versión fijada (`CRI-004@2`) que queda atrás marca la sección como **desfasada**.
- `hm cambios <ref>` compara el frontmatter entre dos commits: estados, versiones, enlaces nuevos o rotos, `cambio:`. Con `grep -l "SUP-003" -r doc registro` basta para lo manual.

### 4.6 Convención de commits

```
dec(DEC-007): tomar tres niveles de madurez

Hm-Sesion: SES-2026-10-07a
Hm-Confirma: humano
Hm-Impacto: SEC-05, SEC-07
```
Los tipos son `cri|dec|des|sup|afi|sec|com|ses|imp`. Hay un commit por compuerta pasada y otro al cerrar la sesión; no hay commits por tarea. `git log --grep "Hm-Confirma: humano"` lista todo lo que el humano avaló.

### 4.7 Vistas generadas (`vistas/`)

- `tablero-criterios.md`: CRI por estado, con su señal contraria, secciones donde aplica y la última versión confirmada.
- `mapa-decisiones.md`: árbol DEC → SUP → AFI con estados. Lo que está en rojo es un supuesto `en_duda` o una afirmación `refutada` bajo una decisión `tomada`.
- `panorama-opciones.md` (exploración): OPC × CRI con puntajes y descartes (M11, M09).
- `reentrada.md`: desde `visitas.yaml.ultimo_commit`, muestra (a) lo que cambió y no confirmaste, (b) lo que confirmaste y cambió después, (c) **lo que olvidaste**: DEC tomadas con `COM` ausente, `dudo`, o de una versión anterior.
- `deuda.md`: el porcentaje de DEC y CRI críticas sin `COM ok` en su versión vigente, que es la métrica de H2.

### 4.8 Caso de nichos de negocio

El repo `nichos-2026/` no tiene `doc/` al inicio.
1. El humano escribe tres nichos en `ideas-propias.md`. El agente agrega `OPC-001…015` a `opciones.yaml`.
2. El humano reacciona: "este exige inventario, no". El extractor propone `CRI-002 sin inventario físico` con esa cita. En G1 el humano lo confirma.
3. Nueve opciones pasan a `DES-*` con `criterios: [CRI-002]`. Si mañana se retira CRI-002, `hm impacto CRI-002` muestra los nueve descartes como reabribles. Así se ve qué se descartó por qué, sin releer nada.
4. Cuando se elige una opción nace `DEC-001` y, recién entonces, `doc/`.

## 5. Relación con gentle-ai y Engram

- **Engram, solo como índice.** Por cada registro confirmado se llama `mem_save(type=decision` para DEC o `policy` para CRI, `topic_key="hm/DEC-007"`, `content=` ruta, dos líneas y estado`)`. Esto hereda la revisión a 6 o 12 meses (`needs_review`). `mem_session_summary` apunta a `sesiones/SES-*.md`. Ante `judgment_required` con `supersedes` o `conflicts_with` sobre CRI o DEC, **siempre** se pregunta al humano. Opcional: `hm validar --citas` comprueba que cada `cita:` exista en `user_prompts` de Engram vía HTTP, para impedir confirmaciones inventadas.
- **ODD:** convive. Se usa su documento extendido y se escribe en CLAUDE.md que en este repo la "unidad de trabajo" es la sección y no hay test-first.
- **Se apaga:** RDD (`review mode disable --scope clone`).
- **Se reutiliza:** Judgment Day como lente opcional en las SEC de riesgo alto.

## 6. Huecos cubiertos

| Hueco | Cómo |
|---|---|
| H1, H4 | CRI nace de la cita, se confirma en G1, se versiona, y `S#`/`SEC` lo referencian con versión |
| H3 | `depende_de` más `cita` y `Hm-Confirma` en git dicen qué criterio se aplicó, dónde y quién lo avaló |
| H2 | `comprension.yaml` con `ID@version` da la deuda medible (`deuda.md`) |
| H5 | `reentrada.md` se ordena por decisiones y por lo olvidado, no por ediciones |
| C1 | OPC, DES y panorama; los descartes se reabren al cambiar un criterio |
| C2 | `hm impacto` y `hm cambios` |
| Verificación sin tests | `riesgo` por SEC y AFI; el verificador solo actúa en lo alto |

## 7. Riesgos, costos y decisiones abiertas

- **Ceremonia de registro.** Es el riesgo principal. Mitigación: el agente escribe todo en `propuesto` y el humano solo responde en las compuertas. Si en el piloto GEE hay más de 3 registros por sesión que el humano no lee, hay que fusionar tipos (por ejemplo, SUP dentro de DEC).
- **Granularidad de AFI.** Marcar cada frase es inviable. Solo se marcan las afirmaciones de riesgo alto o las que sostienen una DEC.
- **Confirmación falsificable.** El agente puede escribir `confirmado_por`. La mitigación es el cotejo de citas con Engram, que depende de una API que cambia rápido.
- **Choque con ODD "MANDATORY".** Hay que probar en Claude Code real que respeta la ruta y las reglas del proyecto.
- **Las marcas `<!--@AFI-->` en la prosa** pueden molestar al exportar a Word. `hm exportar` las elimina.
- **Abiertas:**
  - ¿`vistas/` va versionada o es efímera?
  - ¿Un repo por entregable o un monorepo con IDs con espacio de nombres (`gee:DEC-007`) para la propagación entre documentos?
  - ¿YAML agregado o un archivo por AFI cuando superen ~200?
