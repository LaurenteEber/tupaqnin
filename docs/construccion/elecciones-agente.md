# Elecciones del agente

Cada `[EL DISEÑO NO LO DICE]` que el agente resolvió con la elección más chica (AGENTS.md, regla 13). Una línea por elección: pieza · elección · dónde · alternativa.

| Pieza | Elección | Dónde | Alternativa |
|---|---|---|---|
| 1.3 | Los confusables de UTS #39 se cubren con una tabla a mano de las letras cirílicas, griegas y latinas parecidas a `[a-z0-9]`, el alfabeto de las etiquetas | `internal/graph/labels.go` · `confusables` | La tabla completa de UTS #39, que pide una dependencia nueva |
| 1.3 | El HTML crudo (regla 3) se detecta como un `<` seguido de una letra, `/`, `!` o `?`; rechaza también los autoenlaces de Markdown | `internal/cycle/apply.go` · `rawHTML` | Analizar el Markdown con goldmark (§7), que aún no es dependencia |
| 1.3 | Dos textos que insertan en el mismo punto no se solapan: van en el orden de la salida | `internal/cycle/apply.go` · `overlap` | Rechazarlos como solapados |
| 1.3 | Textos de una misma sección contra dos `base_version` distintas se rechazan | `internal/cycle/apply.go` · `validate` | Aplicar cada uno contra su versión |
| 1.3 | El cambio de texto del agente se registra como `edit` con actor `agent` | `internal/cycle/apply.go` · `apply` | Un evento propio del paso, que el glosario no tiene |
| 1.3 | `used_sources` no nombra sección: la arista `uses` sale de cada sección que el paso escribe; si no escribe texto, no hay `uses` | `internal/cycle/apply.go` · `apply` | Exigir la sección en la salida |
| 1.3 | Las anclas de cifras y citas se buscan en todas las secciones que escribe el paso y deben aparecer una vez en total | `internal/cycle/apply.go` · `validate` | Que la salida nombre la sección de cada ancla |
| 1.3 | El valor de una cifra es el primer número de su ancla; un ancla de cifra sin número se rechaza | `internal/cycle/apply.go` · `apply` y `validate` | Guardar el ancla entera como valor |
| 1.3 | «Tu material» son todas las fuentes del proyecto (en la rebanada 1 las sube la persona); se buscan primero las usadas en el paso | `internal/cycle/apply.go` · `linkData` | Solo las fuentes usadas en el paso |
| 1.3 | Una sección nueva declarada en `proposed_roles` sin texto nace vacía, como las del «Entendí» | `internal/cycle/apply.go` · `apply` | Rechazar la declaración sin texto |
| 1.3 | El agente puede volver a proponer el rol de una sección existente aunque lo hayas confirmado; queda `proposed`, como en la 1.2 | `internal/cycle/apply.go` · `proposeRole` | Ignorar la propuesta si el rol está confirmado |
| 1.3 | La cadena tras el «Entendí» corre como el turno siguiente; el paso del «Entendí» lleva `next_turn_id` y la página lo sigue | `internal/cycle/runner.go` · `runUnderstandingTurn` · `web/src/App.tsx` | Un solo turno con el «Entendí» y la cadena, que retrasa ver el «Entendí» hasta el final |
| 1.3 | El reintento de un paso rechazado lleva «lo que cambió desde que se entregó», con el texto vigente de la sección (B-11) | `internal/cycle/runner.go` · `runStep` | Poner el texto vigente dentro del mensaje de rechazo |
