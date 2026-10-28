# Estado de trabajo · Caso 07 · arranque por umbral (corte tras la fila 9)

## Capa 1 · En literal

**Encargo** (humano, fila 1):
> «Caso de negocio planta solar 40 MW para la mina, para el comité de inversiones. ¿Planta o seguir con la red? Tasa de descuento corporativa: 10 %, no se discute».

Archivos subidos con el encargo: plan de mina, curva de consumo y dos cotizaciones.

**Entendí** (principal, fila 2; el humano no corrigió):
> «Entendí: comparar construir la planta frente a comprar a la red, con VAN a tasa 10 % (C1), para el comité de la minera, en 6 secciones».

**Criterios:**
- **C1:** «Tasa de descuento corporativa: 10 %, no se discute».
  - Caso de lo que cuenta / lo que no: [no registrado: el «Entendí» no trae casos para C1].

**Premisa:** ninguna. No hay idea ni expectativa declarada: no se guarda premisa.

**Decisiones críticas vigentes, con su contraste:**
- **Irradiación** (tomada por el principal; crítica porque cubre una cifra; tarjeta pendiente para el humano):
  - Elegida: serie satelital de 20 años.
  - Contraste: estación de la mina con 3 años (descartada por serie corta).

**Pendientes del humano:**
- **Tarjeta abierta · Irradiación:** serie satelital de 20 años (elegida) frente a estación de la mina con 3 años (descartada por serie corta). Sin respuesta.
- **«Por aclarar»** en «Valor después del cierre», «Evaluación» y «Recomendación». Origen: el plan de mina indica cierre en 2037 (12 años); la planta dura 25. Es alcance: solo el humano decide. Una sola pregunta:
  > (a) evaluar 12 años y vender equipos al cierre · consecuencia: VAN negativo en 18 % de los escenarios, pesa sobre todo el precio de la red; (b) evaluar 25 años vendiendo energía a la red · VAN más atractivo, pero depende de vender a la red 13 años sin la mina.

  Sin respuesta. El agente avanza ambas variantes rotuladas sin elegir.
- **Informe sin ver:** ninguno emitido todavía.

## Capa 2 · Índice

Secciones (6 según el «Entendí»; solo 3 tienen nombre registrado):
- `sec-evaluacion` · [sin versión registrada] · por aclarar (variantes (a) y (b) rotuladas) · [sin primera línea]
- `sec-valor-despues-cierre` · [sin versión registrada] · por aclarar (variantes (a) y (b) rotuladas) · [sin primera línea]
- `sec-recomendacion` · [sin versión registrada] · por aclarar · [sin primera línea]
- 3 secciones más · [sin nombre registrado] · sin empezar · [sin primera línea]

Decisiones no críticas: ninguna registrada.

Digestos: ninguno.

Fuentes:
- `fuente-plan-de-mina` · subida por el humano · [sin primera línea]
- `fuente-curva-de-consumo` · subida por el humano · [sin primera línea]
- `fuente-cotizacion-1` · subida por el humano · [sin primera línea]
- `fuente-cotizacion-2` · subida por el humano · [sin primera línea]

## Capa 3 · En vuelo

- **Modo:** «continúa hasta terminar», activado por el humano (fila 3). Mensaje del humano en literal: «Continúa hasta terminar, estoy en reuniones hasta las 6. Si surge algo, tú decides». Advertencia de la app antes de activar: «Se pueden acumular varias secciones y decenas de decisiones que no habrás visto. Para firmar tendrás que entenderlas igual». Respuesta del humano: «ok». Humano ausente.
- **Sección y paso actuales:** fin del paso de verificar sobre el modelo y las variantes de «Evaluación», «Valor después del cierre» y «Recomendación».
- **Pasos ya hechos:** construir el modelo en código (VAN, costo de la energía en la vida de la planta, degradación); decisión de irradiación; 1 000 escenarios de sensibilidad (irradiación, inversión, precio de la energía, degradación); «por aclarar» del cierre con variantes; verificación.
- **Último triaje:** «Tramo: verificar · Riesgo: cifras y cálculos; conclusiones y recomendaciones · Delego en juez: hay riesgo de la lista».
- **Encargos a subagentes:** juez sobre el modelo y las variantes · retorno: sin fallas. Ninguno abierto. La app comparó cifras con sus fuentes (cotizaciones, curva) · [resultado no mostrado en el guion].
- **Nota del agente** (del agente): intentó · verificar con juez el modelo y las variantes; descartó · estación de la mina como fuente de irradiación (serie corta); piensa hacer después · [no registrado].

## Lo que cambió desde lo último entregado

(vacío: mismo día y misma sesión; el humano no ha editado nada desde que se fue)
