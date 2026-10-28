---
titulo: Glosario · del diseño (español) al código (inglés)
estado: v1.4 · 28-oct · suma los campos de las salidas del «Entendí» y de Wonder · v1.3 · 28-oct · suma links, cuentas y sesiones (B3-18, B3-22) · v1.2 · 28-oct · suma `label`, `label_counter`, `operation` y el `id` de «por aclarar» propuesto · v1.1 · 28-oct · suma las etiquetas cortas (§8, T-2), los pendientes, los retornos de los modelos y la marca crítica (validación de los defaults) · v1 · 28-oct · sale de T-1 (defaults-tecnicos.md)
relacion: lo usan bloque-3-especificacion.md §1, salida-principal.schema.json, el backlog y AGENTS.md
---

# Glosario

*El diseño se escribe en español; el código, los datos, la API y las instrucciones a los modelos, en inglés (T-1). Esta tabla liga cada término con su nombre en el código. **Regla:** si un término no está aquí, el agente de código no lo inventa: lo marca `[EL DISEÑO NO LO DICE]` y se agrega aquí. En la base y en JSON, `snake_case`; en Go, `CamelCase`.*

## 1. Tablas

| Diseño | Código |
|---|---|
| PROYECTO · OBJETO · VERSION · VINCULO · EVENTO · ENTREGA · LLAMADA | `project` · `object` · `version` · `edge` · `event` · `delivery` · `model_call` |
| etiqueta corta del objeto · contador de etiquetas por prefijo (T-2) | `label` (campo de `object`) · `label_counter` |
| link de invitación · cuenta · sesión (B3-18, B3-22; espec. §1.7) | `invitation` · `account` · `session` |
| saldo del link · lo gastado · máximo de cuentas · máximo de proyectos sin cuenta · link revocado | `budget_usd` · `spent_usd` · `max_accounts` · `max_anon_projects` · `revoked_at` |
| dueño del proyecto: la cuenta, o la sesión si es sin cuenta | `account_id` · `anon_session_hash` (campos de `project`) |

## 2. Nodos del grafo y sus estados

| Diseño | Código | Estados (diseño → código) |
|---|---|---|
| pedido (con destinatario) | `request` (`audience`) | — |
| criterio | `criterion` | vigente → `current` |
| caso del «Entendí» | `understanding_case` | sin confirmar · confirmado · corregido → `unconfirmed` · `confirmed` · `corrected` |
| premisa | `premise` | vigente · contradicha → `current` · `contradicted` (proyecto exploratorio: `exploratory`) |
| sección | `section` | borrador · lista · firmada · cambió desde que la firmaste · escrita con lo anterior → `draft` · `ready` (se calcula) · `signed` · `changed_since_signoff` · `stale` |
| rol de la sección | `role` | conclusión · recomendación · otro → `conclusion` · `recommendation` · `other`; propuesto · confirmado → `proposed` · `confirmed` |
| decisión | `decision` | del agente · tuya · confirmada · en «por aclarar» · reemplazada · revocada → `agent` · `human` · `confirmed` · `in_open_question` · `superseded` · `revoked` |
| decisión de supuesto | `decision` con `assumption: true` | — |
| la marcaste crítica (cuarto motivo de criticidad) | `human_marked_critical` (en la decisión) | — |
| cifra · dato · cita | `figure` · `datum` · `citation` | — |
| fuente (archivo · web · cálculo) | `source` (`file` · `web` · `calculation`) | — |
| «por aclarar» | `open_question` | abierto · resuelto → `open` · `resolved` |
| tarjeta | `card` | abierta · respondida · obsoleta → `open` · `answered` · `obsolete` |
| propuesta | `proposal` | abierta · aceptada · descartada → `open` · `accepted` · `discarded` |
| firma · firma breve | `signoff` · `brief_signoff` | — |
| pregunta del destinatario | `audience_question` | abierta · abordada · no abordada → `open` · `addressed` · `declined` |
| ronda (investigación · juez) | `round` (`research` · `judge`) | abierta · cerrada → `open` · `closed`; ¿redujo la duda? sí · en parte · no → `yes` · `partly` · `no` |

## 3. Aristas

| Diseño | Código |
|---|---|
| la cubre (cifra → decisión) | `covers` |
| se usa en (decisión → sección) | `used_in` |
| viene de (dato o cita → fuente) | `comes_from` |
| usa (sección → fuente) | `uses` |
| aplica (decisión → criterio) | `applies` |
| depende del supuesto (decisión → supuesto o premisa) | `depends_on_assumption` |
| bloquea («por aclarar» → sección o decisión) | `blocks` |
| reemplaza (decisión → decisión anterior) | `supersedes` |
| explica (firma → decisión) | `explains` |
| sobre (pregunta del destinatario → sección) | `about` |

## 4. Eventos de la historia

| Diseño | Código |
|---|---|
| cambio de estado · fin de paso · modo activado · tu mensaje · informe de parada | `state_changed` · `step_ended` · `mode_set` · `human_message` · `stop_report` |
| decisión vista (informe · tarjeta · historia · marco) | `decision_seen` (`report` · `card` · `history` · `frame`) |
| edición · edición deshecha · alerta de edición sospechosa | `edit` · `edit_undone` · `suspicious_edit_alert` |
| caso de aplicación de la firma · pausa de la firma · firma breve | `signoff_case` · `signoff_paused` · `brief_signoff` |
| pregunta del destinatario respondida «no» | `audience_question_declined` |
| cambio en lo que usa (entra · actualizado · revisado sin cambio) | `usage_change` (`entered` · `updated` · `reviewed_unchanged`) |
| nueva versión de una fuente · cambio de rol · cambio de destinatario | `source_new_version` · `role_changed` · `audience_changed` |
| arista (nace · cambia · se reemplaza) | `edge` (`created` · `changed` · `superseded`) |
| respuesta sin «porque» válido · tiempo de un acto obligatorio | `answer_without_reason` · `act_timed` |
| marca crítica (la pones o la quitas) | `marked_critical` |
| sonda · arranque (otro día · cambio de modelo · umbral) · reintento de la salida | `probe` · `startup` (`new_session` · `model_change` · `threshold`) · `output_retry` |

## 5. Quién y roles

| Diseño | Código |
|---|---|
| quién: tú · el agente · la app | `human` · `agent` · `app` |
| principal · lector · juez · comprobador · Wonder (contraste de la firma) | `principal` · `reader` · `judge` · `checker` · `contraster` |

## 6. Salida fija del principal (fichas §7)

| Diseño | Código |
|---|---|
| línea de triaje · riesgo al cerrar | `triage_line` · `closing_risk` |
| riesgos: ninguno · cifras y cálculos · fuentes y citas · conclusiones y recomendaciones · contradice algo confirmado · efecto ante terceros · no se puede comprobar | `none` · `figures_and_calculations` · `sources_and_citations` · `conclusions_and_recommendations` · `contradicts_confirmed` · `third_party_effect` · `unverifiable` |
| texto nuevo (sección, versión base, ancla, operación: reemplaza · inserta después, texto) | `new_text` (`section_id`, `base_version`, `anchor`, `operation`: `replace` · `insert_after`, `text`) |
| decisiones nuevas (id, enunciado, en vez de, elegiría si, supuesto, secciones donde se usa, reemplaza, aplica, depende del supuesto) | `new_decisions` (`id`, `statement`, `instead_of`, `would_choose_if`, `assumption`, `used_in_sections`, `supersedes`, `applies`, `depends_on_assumption`) |
| decisiones usadas · rol propuesto | `used_decisions` · `proposed_roles` |
| «por aclarar» propuesto (id provisional, pregunta, opciones con contraste, consecuencia y riesgo, de qué depende, variantes) | `proposed_open_questions` (`id`, `question`, `options` {`option`, `contrast`, `consequence`, `risk`}, `depends_on`, `variants`) |
| vínculos (cifra → decisión) · fuentes usadas | `figure_links` (`figure_anchor`, `decision_id`) · `used_sources` |
| citas (ancla de la afirmación, fuente, pasaje) | `citations` (`claim_anchor`, `source_id`, `passage`) |
| cambios atendidos (actualizado · revisado sin cambio + «porque») | `handled_changes` (`change_id`, `result`: `updated` · `reviewed_unchanged`, `reason`) |
| diferido (qué, por qué) · nota | `deferred` (`what`, `why`) · `note` |

## 7. Conceptos del ciclo y de la memoria

| Diseño | Código |
|---|---|
| paso · fin de paso · triaje · vía directa · parada · informe fijo | `step` · `step_end` · `triage` · `direct_path` · `stop` · `stop_report` |
| «Entendí» · «porque» · contraste · encargo · digesto | `understanding` · `reason` · `contrast` · `brief` · `digest` |
| comprobaciones · criticidad · pendientes (del agente) · cambios sin cerrar · inventario de cobertura | `checks` · `criticality` · `pending` (`agent_pending`) · `open_changes` · `coverage_inventory` |
| pregunta de origen · caso nuevo de la firma · preparación de la defensa | `origin_question` · `application_case` · `defense_prep` |
| modo normal · «continúa hasta terminar» | `normal` · `run_to_completion` |
| grafo del proyecto · historia · huella | `project_graph` · `history` · `fingerprint` |
| el marco · línea de vuelta · reentrada | `frame` · `return_line` · `reentry` |
| memoria del principal · capa 1 · capa 2 · capa 3 | `memory` · `essentials` · `index` · `in_flight` |
| arranque · núcleo · tarea en turno · índice recortado · tope · umbral | `startup` · `core` · `current_task` · `trimmed_index` · `cap` · `threshold` |
| lo entregado · lo que cambió · lo diferido · nota del agente | `delivery` · `changes_since_delivery` · `deferred` · `agent_note` |
| sondas · lo protegido · «no lo declaró el agente» | `probes` · `protected` · `undeclared_by_agent` |
| herramientas del principal: buscar · leer(id) | `search` · `read(id)` |
| el marco (función) · decisión vista, con su origen | `frame(project)` · `decision_seen` (`seen_via`: `report` · `card` · `history` · `frame`) |

## 8. Etiquetas cortas (T-2, T-3)

*Lo que ven el principal y tú. En minúscula, en inglés y sin ambigüedad entre tipos. Cada etiqueta es única en su proyecto y no se reutiliza ni se renombra. El principal nunca ve el id interno.*

| Diseño | Prefijo | Ejemplo |
|---|---|---|
| pedido · criterio · caso del «Entendí» · premisa | `req` · `crit` · `case` · `prem` | `crit-2` |
| sección · decisión · cifra · dato · cita · fuente | `sec` · `dec` · `fig` · `dat` · `cit` · `src` | `dec-12` |
| «por aclarar» · tarjeta · propuesta · firma | `oq` · `card` · `prop` · `sign` | `oq-3` |
| pregunta del destinatario · ronda | `aq` · `rnd` | `aq-1` |
| cambio de la lista de una sección (A-1) | `chg` | `chg-7` |
| id provisional de lo que nace en una salida (T-3) | `new` | `new-1` |

## 9. Pendientes y lo que te toca (estados §1)

| Diseño | Código |
|---|---|
| pendientes · lo que te toca (pendientes del humano) · pendientes del agente | `pending` · `human_pending` · `agent_pending` |
| «por aclarar» abierto · tarjeta abierta | `open_question` · `open_card` |
| cifra sin decisión · escrito sin decisión · códigos internos · relato del proceso | `figure_without_decision` · `text_without_decision` · `internal_codes` · `process_narrative` |
| fuente que no sostiene lo afirmado · «falta en el texto» (Wonder) · retorno sin la forma fija | `source_not_supporting` · `missing_in_text` · `malformed_output` |
| lista de asentimientos (por idioma, en el catálogo; T-1) | `assent_list` |

## 10. Retornos de los modelos (fichas §7)

| Diseño | Código |
|---|---|
| lector: hallazgo (pasaje, fuente, tipo: hecho · supuesto · contradicción · vacío) · cubiertos · fuentes leídas | `finding` (`passage`, `source_id`, `kind`: `fact` · `assumption` · `contradiction` · `gap`) · `covered` · `sources_read` |
| verificación de un hallazgo web: entra · no verificado · descartado | `verification`: `entered` · `unverified` · `discarded` |
| juez: falla (pasaje, riesgo, escenario) · resuelta · sigue abierta · lo que el trabajo no cubre | `flaw` (`passage`, `risk`, `scenario`) · `resolved` · `still_open` · `not_covered` |
| comprobador: por sonda contradice · no · duda | `probe_result`: `contradicts` · `no` · `doubt` |
| comprobador: candidatos (contradicción · término dudoso · relato del proceso · escrito sin decisión · fuente que no sostiene · excede lo revisado) | `candidates` (`contradiction` · `doubtful_term` · `process_narrative` · `text_without_decision` · `source_not_supporting` · `exceeds_reviewed`) |
| Wonder: por decisión crítica coincide · falta en tu explicación · falta en el texto | `match` · `missing_in_explanation` · `missing_in_text` |
| Wonder: la pregunta de origen la responde · no · caso nuevo lo aplica bien · no | `answers` · `does_not_answer` · `applies_well` · `does_not_apply` |
| Wonder, campos de su salida: por decisión (marca · qué falta, sin la respuesta · pasaje) · pregunta de origen · caso anterior · caso nuevo | `decisions` (`verdict` · `missing` · `passage`) · `origin_question` (`question` · `answer`) · `previous_case` · `new_case` (`salida-wonder.schema.json`) |
| «Entendí», campos de su salida: lo entendido · por criterio (caso que cuenta · caso que no) · lo leído de tu material · secciones con su rol propuesto | `understood` · `criteria` (`counts` · `does_not_count`) · `material_readings` · `sections` (`title` · `role`) (`salida-entendi.schema.json`) |
