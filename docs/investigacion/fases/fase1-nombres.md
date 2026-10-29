# Nombres para el proyecto (fase 1)

Fecha: 2026-10-07

**Proyecto:** un harness ligero de instrucciones, skills, memoria y convenciones Markdown que corre sobre agentes existentes (Claude Code, OpenCode, etc.). Sirve para co-construir documentos con IA sin que la persona se vuelva cuello de botella ni acumule deuda cognitiva.

**Restricciones:**
- No debe ser un nombre comercial: nada de productos o marcas existentes ni de aire de marketing.
- No puede parecerse a "gentle-ai" ni a "Engram".
- Debe ser corto, pronunciable en español e inglés y válido como repo y como CLI.

---

## 1. Candidatos (28, en 4 familias)

### A. Quechua: registro, comprensión y oficio andino
| Nombre | Significado |
|---|---|
| hamutay | "Reflexionar, comprender a fondo" (de la raíz de *amauta*, el sabio). Es la idea de que la persona entienda lo que se construye. |
| unanchay | "Señal o marca; entender". Registrar y comprender. |
| yuyay | "Recordar, pensar". Memoria. |
| yachay | "Saber, aprender". |
| khipu | Cuerdas anudadas con que se llevaba registro en el Tawantinsuyu. |
| kamayoq | Especialista encargado de un oficio; los *khipukamayoq* custodiaban los khipus. |
| qillqa | "Escritura, signo escrito". |
| ayni | Reciprocidad: trabajo mutuo. |
| minka | Trabajo colectivo. |
| tinkuy | "Encuentro" de dos partes. |
| awana | "Telar" en quechua. |

### B. Metáforas de oficio en español: tejer, coser y medir
| Nombre | Significado |
|---|---|
| urdimbre | Hilos fijos del telar sobre los que se teje la trama. Los criterios serían la urdimbre del documento. |
| hilvan | Puntada provisional que sostiene la tela antes de la costura definitiva, como borradores que la persona fija. |
| telar | Máquina de tejer. |
| trama | Hilos que se cruzan con la urdimbre. |
| plomada | Instrumento que verifica la vertical; es un criterio de rectitud. |
| bitacora | Cuaderno de navegación. Es el registro de decisiones. |
| cantero | Quien labra la piedra a mano. |

### C. Latín y griego: tradición académica
| Nombre | Significado |
|---|---|
| escolio | Nota explicativa al margen de un texto clásico. |
| glosa | Comentario que explica un texto. |
| marginalia | Anotaciones al margen. |
| phronesis | Sabiduría práctica en Aristóteles, es decir, el juicio. |
| anamnesis | Rememoración. |
| criterium | "Medio para juzgar". |
| ratio | Razón, cálculo, fundamento. |

### D. Español llano
| Nombre | Significado |
|---|---|
| criterio | El criterio vivo. |
| borrador | Documento en construcción. |
| acta | Registro formal de decisiones. |

---

## 2. Verificación de los preseleccionados

**Método:**
- Se consultó directamente el registro de npm (`registry.npmjs.org/<n>`).
- Se consultó directamente PyPI (`pypi.org/pypi/<n>/json`).
- Se consultó directamente crates.io.
- Se revisó si existe un usuario u organización en `github.com/<n>` con WebFetch.
- Se hicieron búsquedas web generales y con `site:github.com`.

**Lo que no se pudo verificar:**
- **Búsqueda de repositorios en GitHub.** No se pudo hacer de forma directa: `github.com/search` está bloqueado por robots.txt y la API de búsqueda está restringida en esta sesión. Para repos homónimos solo hay indicios indirectos por búsqueda web, así que pueden existir repos pequeños con esos nombres que no aparecieron.
- **Registros de marcas.** No se consultaron INDECOPI, USPTO ni EUIPO.

| # | Nombre | Significado | npm | PyPI | GitHub (usuario u org) | Web / otros | Veredicto |
|---|---|---|---|---|---|---|---|
| 1 | **hamutay** | comprender, reflexionar (quechua) | libre | libre | existe el usuario `hamutay`, sin repos públicos | No se halló ningún producto, empresa ni repo conocido con ese nombre; los resultados solo traían nombres parecidos (Humantay, un lago). crates libre. | **Disponible** |
| 2 | **urdimbre** | hilos base del telar | libre | libre | existe el usuario `urdimbre`, sin repos públicos | No se halló software ni startup relevante. Es una palabra común, y es posible que existan editoriales o colectivos culturales homónimos (no confirmado). crates libre. | **Disponible** |
| 3 | **hilvan** | puntada provisional | libre | libre | existe el usuario `hilvan`, sin repos públicos | Ningún producto relevante; los resultados eran ruido (Hilversum, Hilvy). crates libre. | **Disponible** |
| 4 | **unanchay** | señal; entender (quechua) | libre | libre | `github.com/unanchay` da 404 (libre) | Sin resultados relevantes. | **Disponible**, aunque es largo y menos conocido |
| 5 | **kamayoq** | especialista andino | libre | libre | `github.com/kamayoq` da 404 (libre) | Aparece en proyectos de desarrollo rural, como la Escuela Kamayoq de Practical Action. No es una marca de software. La grafía varía (kamayoq, kamayuq), lo que complica escribirlo. | Disponible, con fricción de grafía |
| 6 | **escolio** | nota al margen | libre | libre | existe la org `Escolio`, sin repos públicos | No se halló un producto conocido. | Disponible, con riesgo bajo por la org |
| 7 | **tinkuy** | encuentro (quechua) | libre | libre | existe el usuario `Tinkuy`, sin repos | En crates.io el nombre está reservado ("tawasuyu's physics DSL"). En la web aparecen "Tinkuy Perú" (tesis PUCP sobre una plataforma de turismo) y "Tinkuy" (congreso CONEGP PUCP). | **Descartar**: está ocupado en el ecosistema y su significado se aleja del proyecto |
| 8 | **qillqa** | escritura (quechua) | libre | libre | existe el usuario `qillqa` con 12 repos (personal) | Es un topónimo con varias entradas en Wikipedia. | Descartar o usar con cautela: el usuario de GitHub está activo |
| 9 | **plomada** | instrumento de verticalidad | libre | libre | existe el usuario `plomada` (persona, 1 fork) | Sin productos relevantes. | Disponible, pero su metáfora es débil para "documentos" |
| 10 | **yuyay** | recordar (quechua) | libre | **ocupado**: "YUYAY Intelligence Framework, UN Office of the Future" | no verificado | Es un framework de IA de un organismo de la ONU. | **Descartar** |
| 11 | **telar** | máquina de tejer | ocupado | **ocupado**: "Un telar para hilos de trabajo: multiplexor de terminal, perfiles de repositorio y agentes" | no verificado | Es un choque directo: una herramienta de agentes en español. | **Descartar** |
| 12 | **bitacora** | cuaderno de navegación | ocupado | **ocupado**: "Live dashboard for coding agents (Codex, Claude Code)" | no verificado | Es un choque directo con el nicho del proyecto. | **Descartar** |
| 13 | **yachay** | saber (quechua) | libre | **ocupado**: "Python SDK for Yachay — Condor Models inference API" | no verificado | Es un producto comercial de IA. Además existe Yachay Tech, una universidad de Ecuador. | **Descartar** |
| 14 | **khipu** | registro anudado | ocupado (job queue) | ocupado (cliente de la API Khipu) | no verificado | Khipu es una pasarela de pagos chilena y una gema de Ruby; también existe khipu-io, una blockchain empresarial. | **Descartar**: es una marca comercial |
| 15 | **marginalia** | anotaciones al margen | ocupado | **ocupado**: "Markdown vault quality scanner for Obsidian…" | no verificado | Choca con el nicho Markdown. Además Marginalia es un buscador conocido (marginalia.nu, según conocimiento previo, no verificado aquí). | **Descartar** |
| 16 | **phronesis** | sabiduría práctica | **ocupado**: "Phronesis CLI + daemon" | libre | no verificado | Es una CLI existente. | **Descartar** |
| 17 | **glosa** | comentario explicativo | libre | ocupado (paquete 0.1.0, casi vacío) | no verificado | En el mundo de la traducción existe "GlossaPress", y "Glosa" suena a glosario. | Riesgo medio |

Otros descartes:
- **anamnesis:** ocupado en npm y PyPI.
- **ayni:** ocupado en npm.
- **criterio:** libre en npm y PyPI, pero es una palabra demasiado genérica para buscarla y distinguirla.
- **minka:** quedó sin verificar. Por conocimiento previo, hay empresas y plataformas con ese nombre en la región (no se confirmó en esta sesión).

---

## 3. Finalistas

### 1) `hamutay` (recomendación principal)
- **Significado:** "comprender o reflexionar a fondo" en quechua. Nombra la tesis central del proyecto: que la persona entienda y pueda defender lo construido, contra la deuda cognitiva. La raíz es la misma de *amauta*, el sabio y maestro, así que tiene raíz cultural peruana y no suena a marketing.
- **Sonoridad:** tres sílabas llanas, *ha-mu-TAI*, que se pronuncian igual en español y en inglés. Funciona como CLI: `hamutay init`, `hamutay review`.
- **Disponibilidad:**
  - npm, PyPI y crates están libres.
  - En GitHub existe un usuario `hamutay` sin repos públicos. No impide crear `<tu-usuario>/hamutay`, pero no se podrá tener la org con ese nombre exacto (alternativa: `hamutay-dev`).
  - No se hallaron productos homónimos.
- **Riesgos:**
  - Conviene confirmar en un diccionario quechua la glosa exacta. El significado viene de conocimiento previo y no se verificó en esta sesión.
  - Conviene documentar la grafía: *hamutay* frente a *hamut'ay*. Para el repo se recomienda la forma sin apóstrofo.

### 2) `hilvan`
- **Significado:** el hilván es la puntada provisional que sostiene la pieza antes de la costura definitiva. Es una metáfora exacta del flujo: el agente hilvana y la persona decide qué se cose. Además, en español *hilvanar ideas* significa articularlas.
- **Sonoridad:** dos sílabas, *hil-VAN*, fácil en inglés, y muy buen nombre de CLI.
- **Disponibilidad:** npm, PyPI y crates están libres. Existe el usuario de GitHub `hilvan`, sin repos. No se hallaron productos.
- **Riesgo:** para un angloparlante la palabra no comunica nada sin explicación. Hay que perder la tilde (*hilván*).

### 3) `urdimbre`
- **Significado:** los hilos fijos del telar sobre los que se teje la trama. Es la metáfora del "criterio vivo": los criterios forman la estructura sobre la que se teje el documento. Es una palabra culta del español con raíz artesanal.
- **Sonoridad:** *ur-DIM-bre*. Se entiende en inglés, aunque es algo más difícil de pronunciar que las otras dos.
- **Disponibilidad:** npm, PyPI y crates están libres. Existe el usuario de GitHub `urdimbre`, sin repos.
- **Riesgo:** al ser una palabra común, podría coincidir con proyectos culturales o editoriales (no confirmado), y es más larga para teclear como CLI.

---

## 4. Descripción del repo (una línea)

- **ES:** "Harness ligero para co-construir documentos con agentes de IA: criterios que se versionan, decisiones registradas y comprensión que se cuida."
- **EN:** "A lightweight harness for co-writing documents with AI agents: versioned criteria, recorded decisions, and human understanding by design."

## 5. Pendientes antes de fijar el nombre
1. Buscar manualmente en `github.com/search?q=hamutay&type=repositories`, porque no fue posible desde esta sesión.
2. Consultar el buscador de marcas de INDECOPI y el de WIPO (Global Brand Database) para el nombre elegido.
3. Confirmar la glosa en un diccionario quechua, por ejemplo el de la Academia Mayor de la Lengua Quechua.
