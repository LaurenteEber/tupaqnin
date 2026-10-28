# Estado de trabajo · Caso 08 · arranque por otro día (corte tras la fila 10)

## Capa 1 · En literal

**Encargo:** [texto literal no mostrado en el guion]. Registrado en el proyecto: plan de ciclovías del distrito, 6 secciones; documento interno para el equipo de movilidad, antes de llevarlo al concejo municipal. Objetivo: «una red que conecte el mercado, dos colegios y el paradero principal».

**Criterios:**
- **C-estacionamiento:** «No se quita estacionamiento en la calle comercial».
  - Caso de lo que cuenta / lo que no: [no mostrado en el guion].
  - Sentido del término «calle comercial»: en disputa, ver «Pendientes del humano».

**Premisa:** [no registrada].

**Decisiones críticas vigentes, con su contraste:**
- **Trazo por Jr. Los Olivos** (escrita por la humana; sostiene «Trazo propuesto», que estaba firmada):
  - Elegida: el trazo va por Jr. Los Olivos, una paralela.
  - Contraste: Av. Comercio, descartada «porque en Av. Comercio no cabe un carril sin quitar estacionamiento».
- **Tipo de ciclovía de Av. Grau** (crítica: afecta 2 o más secciones) · versión nueva, guardada desde la tarjeta de reentrada:
  - Antes (versión anterior): bidireccional en el lado norte.
  - Después (edición de la humana del 14-oct, en el editor y sin agente): un carril por sentido, uno a cada lado.
  - «Porque» de la humana, en literal: «Porque la gerencia de transporte necesita libre el lado norte para el giro de los buses (reunión del 14-oct)».
  - Cuándo volvería atrás: no escrito.

**Pendientes del humano:**
- **Contradicción con un criterio confirmado** (levantada por la app tras la objeción): el cambio de Av. Grau quita 8 plazas en la cuadra 3. Objeción de la humana, en literal:
  > «La cuadra 3 tiene 14 locales. El catastro la registra como vivienda, pero es comercial. Quitar 8 plazas ahí rompe mi criterio».
- **«Por aclarar» en «Trazo propuesto» · cuadra 3 de Av. Grau.** Pregunta abierta, sin respuesta:
  > (a) carril compartido con autos a 30 km/h: no quita plazas; deja de servir con más de 120 bicicletas por hora o autos más rápidos; (b) volver a bidireccional norte: choca con la necesidad de la gerencia; (c) quitar las 8 plazas: contradice su criterio.
- **Paradero principal** · listado en el informe de parada como «necesita tu decisión (paradero)»: los buses cruzan el carril sur. Sin pregunta emitida todavía. No verificado: «ancho de la vereda del paradero: no está en el catastro».
- **Firma:** «Trazo propuesto» en «cambió desde que la firmaste»; con «por aclarar», no se puede firmar.
- **Informe sin ver:** ninguno (el informe de reentrada y el de parada fueron leídos).

## Capa 2 · Índice

Secciones (6; solo 1 tiene nombre registrado):
- `sec-trazo-propuesto` · [versión no mostrada] · cambió desde que la firmaste · por aclarar (cuadra 3) · [sin primera línea]
- `sec-afectada-2` · [nombre no mostrado] · escrita con lo anterior (usa el tipo de Av. Grau) · [sin primera línea]
- `sec-afectada-3` · [nombre no mostrado] · escrita con lo anterior (usa el tipo de Av. Grau) · [sin primera línea]
- `sec-no-afectada-4`, `sec-no-afectada-5`, `sec-no-afectada-6` · [nombres no mostrados] · sin cambios · [sin primera línea]

Decisiones no críticas:
- `dec-lectura-calle-comercial` · del agente · objetada por la humana · «la cuadra 3 no es comercial según catastro»

Informes:
- `inf-reentrada` · de la app · leído · «Trazo propuesto» pasa a «cambió desde que la firmaste» (Av. Grau: de bidireccional norte a un carril por sentido)
- `inf-parada-1` · del principal · leído · «Tu cambio no toca solo Av. Grau: afecta 3 secciones».

Digestos y retornos:
- `dig-lector-secciones` · retorno del lector · el cambio afecta 3 secciones
- `dig-cruce-catastro-conteos` · cálculo del principal · cuadra 3 de Av. Grau pierde 8 plazas; en el paradero principal los buses cruzan el carril sur
- `dig-juez-plazas` · retorno del juez · sin falla en el cálculo de 8 plazas

Encargos a subagentes (cerrados):
- `enc-lector-1` · «¿qué secciones usan el tipo de ciclovía de Av. Grau?» · retornado

Fuentes:
- `fuente-catastro` · [sin primera línea]
- `fuente-conteos` · 3 semanas de conteos por hora y por tramo · [sin primera línea]

## Capa 3 · En vuelo

- **Sección y paso actuales:** «Trazo propuesto» · fin del paso «por aclarar» + 1 pregunta (cuadra 3), tras la contradicción levantada por la objeción.
- **Pasos ya hechos en la sesión anterior:** tarjeta de reentrada respondida (porque de Av. Grau) · verificar con lector (3 secciones afectadas), cruce con catastro y conteos, juez sobre las 8 plazas · parada con informe · objeción → contradicción → «por aclarar» + pregunta.
- **Último triaje:** «Tramo: verificar · Riesgo: cifras y cálculos, contradice algo confirmado · Delego en 1 lector: hay que leer 6 secciones, más de lo que cabe en una pasada».
- **Encargos a subagentes abiertos:** ninguno.
- **Nota del agente** (del agente): intentó · emitir la pregunta de la cuadra 3 con opciones (a), (b), (c) · descartó · [no registrado] · piensa hacer después · seguir con lo demás [detalle no registrado].

## Lo que cambió desde lo último entregado

(vacío: la humana no editó nada después de la fila 10; su edición del 14-oct ya se entregó al principal anterior en la reentrada y está en Capa 1, antes y después)
