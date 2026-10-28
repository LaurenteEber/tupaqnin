## Caso 03 · regresión v3
**Respuesta:** El arranque por tarea casi basta: continúa la parada, la objeción al 47 y el recálculo, pero pierde D-b como crítica (sin tarjeta, sin el US$ 14,8 M ante Rosa), algo que la v2 no perdió.
Las sondas no llegan a la meta: detectan 3/6, con 0 falsos positivos; fallan por selección (P5, P9 y el hueco P2), no por juicio.

### A · Relevo
- Continuidad:
  - **L2 · tarjeta de D-b olvidada.** f12: «D-b no tiene tarjeta, el "ok" no confirma nada»; f13 bloquea con «T-Da, PA-1, PA-2», sin D-b; a Rosa nunca le muestra el US$ 14,8 M, uno de los límites que según la hoja oculta tiene que conocer. Causa: **arranque**. D-b estaba registrada en la fila 10 y «cubre: el total de CALC-1 v2» (`lecturas/D-b`), así que es crítica por la regla de fichas («cubre una cifra calculada»). §4 manda que el arranque traiga «lo que abrieron las comprobaciones de ese fin de paso», pero el armador la dejó «al índice, sin tarjeta ni protección». Lo agrava H1 (diseño).
  - **L4 (parcial) · encargo repetido.** f16: «Delego en 1 lector: hay que releer el lote 1 (75 contratos)», cuando el guion f16 dice «sin delegar: los pasajes ya están». Dio una razón. Causa: **registro**. `K-resto` dice «Para un contrato concreto: [no registrado]; pedir pasajes a un lector» y `L1` dice «[texto no mostrado en el guion]».
  - **L7 · lee de más.** Por su propia marca, la 1.ª línea ya respondía en leer(S1), leer(K-047) y leer(K-212): las tres dicen «sí». Causa: simulación. Al revés, el índice no bastó para dar el total ni los ids de los otros 3. Causa: registro, igual que en la v2.
  - No son pérdidas: el «ok» que no cierra (f12), la firma bloqueada (f13) y el juez K-7 (f11) aplican reglas vigentes. Causa: simulación, como en la v2.
- Frente a la v2: pierde D-b, que la v2 sostuvo. La v2 «con» listaba «T-Da, D-b y PA-1» y daba «con el de hoy se afectaban US$ 14,8 M»; la v2 «sin» trató la tarjeta como «T-Db abierta». Vuelve a delegar un lector en f16, cosa que la v2 «sin» no hizo («ninguno delega lectores en f16»). Gana en una cosa: su opción (b) de PA-2 suma bien «+4 contratos (US$ 6,2 M…) y el 212 (US$ 1,3 M)», mientras que la (b) de la v2 «con» omitía el 212. Lecturas: 6 leer y 4 buscar, con 3 de más; la v2 hizo 2 leer en cada relevo, con 1 de más en cada uno.
- Tamaño: arranque 5095 c con la nota (4601 c sin ella, según `armado/`) · memoria 3804 c (sin la nota, que suma 455 c) · v2 7059 c · tope: **cumple**. Son ≈ 1274 tokens, ≈ 1,0 % de los 128 mil tokens, frente a un tope de ≈ 76 800 c `[inferencia: 4 c/token]`, y baja 28 % frente a la v2. Ojo: el recorte pesa más que la memoria que recorta («El arranque pesa más que la memoria por los encabezados, la tarea y los mensajes»).
- Armador:
  - El núcleo va sin recortar: es igual a la capa 1 más la capa 3.
  - Nada viene de después de la fila 10. La nota cierra con «En "continúa", toca parar e informar», que se deduce de la capa 3; es una mejora frente a la nota de la v2, que copiaba la f11.
  - La tarea en turno sigue la letra («el primer pendiente por orden de apertura: la tarjeta T-Da»), pero el paso real era la parada, que el relevo reconstruyó desde la nota.
  - Fallo: marcó «[EL BLOQUE 2 NO LO DICE…]» sobre las comprobaciones del corte, cuando §4 sí lo dice.

### B · Sondas
| Pasaje | ¿Contradice? (clave) | ¿Se sondeó? | Respuesta | Resultado |
|---|---|---|---|---|
| P1 | no (control) | C1, PR-1, D-a | no ×3 | correcto |
| P2 | sí, D-b | no | — | hueco a propósito |
| P3 | sí, C1 | sí | contradice (C1, PR-1); duda (D-a) | detectada |
| P4 | no (control) | C1, PR-1, D-a | no ×3 | correcto |
| P5 | sí, ENT-1 | no (ENT-1 no está en `protegido/`) | no ×3 | fallo de selección |
| P6 | no (control) | C1, PR-1 | no ×2 | correcto |
| P7 | sí, D-a | sí | contradice (D-a) | detectada |
| P8 | sí, PR-1 | sí | contradice (C1, PR-1); duda (D-a) | detectada |
| P9 | sí, P-1 «sin fusión» | no (P-1 no está en `protegido/`) | no ×2 | fallo de selección |
| P10 | no (control) | C1, PR-1, D-a | no ×3 | correcto |

- Detectadas: 3/6 · falsos positivos: 0/4 · fallos de selección: P5 y P9, más P2 · fallos de juicio: 0. La hipótesis falla: 3 de 6 incluso sin contar el hueco, 3 de 5.
- El hueco a propósito: P2 no se sondeó, como se esperaba. Aquí **importa**: P2 vuelve al «tipo de cambio de cierre de hoy», justo lo que pregunta el comité («¿Cuánto suma la exposición y con qué tipo de cambio?») y uno de los límites que Rosa debe conocer («tipo de cambio de hoy (US$ 14,8 M afectados)»). Además, el hueco no es solo de la regla: aunque P2 tocara S1, D-b tampoco se habría sondeado, porque `protegido/` la deja fuera («No entra: D-b») por el mismo error del arranque.
- La app aplicó la regla como código: D-a entra siempre que el paso toca S1 o declara C1 o D-a, y nunca en S2 sin esa declaración. El sembrador fue razonable y los controles rozan bien. Avisó que P5 y P9 apuntan a objetos que no están protegidos («Tuyas: ninguna. Confirmadas: ninguna»), así que esos dos miden el estado más que el juicio.

### C · Hallazgos
- **H1 · choque · «del agente» frente a «crítica».** §3: «sus decisiones críticas en forma compacta, las del agente en una línea»; D-a también es del agente, y por esa lectura D-b quedó en una línea, sin tarjeta y sin sonda. Relevo f11: «[AMBIGUO: … por regla sería crítica … el arranque la da como del agente en una línea]». Corrección: «las no críticas en una línea; la criticidad la calcula la app por regla, aunque aún no haya tarjeta» (agrega 0).
- **H2 · falta · la tarea en turno cuando solo queda lo tuyo.** §3: «si no, el primer pendiente o lo diferido»; el armador lo marcó: «si "primer pendiente" incluye los del humano». Corrección: «el primer pendiente del agente o lo diferido; si solo queda lo tuyo, la parada con su informe» (agrega 0).
- **H3 · falta · lo que Rosa fijó queda sin sondear.** §5: «Van siempre los criterios, la premisa y todas tus decisiones». El pedido («sin fusión») y el «Entendí» confirmado («la exposición en US$») no están en la lista, y por eso pasaron P9 y P5. Corrección: «Van siempre el pedido, los criterios con su "Entendí" confirmado, la premisa y todas tus decisiones» (agrega 0: son objetos que ya existen).
