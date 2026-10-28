---
titulo: Supervisión de la construcción
estado: v1.2 · 28-oct · piezas 0.4 a 0.6; P-1, P-2 y P-4 cerrados; B-8 para Eber · v1.1 · 28-oct · P-1 y P-5 se comprueban en el despliegue de prueba (H-5) · v1 · 28-oct · piezas 0.1 a 0.3
relacion: aplica backlog.md (rebanada 0) y la regla 12 de _reglas-de-trabajo.md. La carpeta del agente (`SideProjects/tupaqnin/`) solo se lee; este registro vive aquí
---

# Supervisión de la construcción

**Respuesta:** 0.1 a 0.6 están hechas y sus pruebas pasan. Hay una falla de diseño que es nuestra, no del agente: el backlog define mal cómo se reemplaza una arista (B-8, para que decidas). Quedan 4 arreglos chicos para el agente.

**Cómo se supervisa cada pieza:** se lee el código en solo lectura y se contrasta con su fila del backlog («Listo cuando», «No hacer») y con el retorno del agente. Eber corre `make test`, `make vet` y la prueba de humo. `git status` en `tupaqnin/` debe quedar igual antes y después.

## Piezas

| Pieza | Fecha | Resultado | Retorno vs. código | Pruebas | Pendiente |
|---|---|---|---|---|---|
| 0.1 · Esqueleto y CI | 28-oct | Cerrada · commit y push | Coincide | `make test` y `make vet` pasan · Eber probó `make run` (el agente no pudo abrir puertos) | — |
| 0.2 · Configuración y registros | 28-oct | Cerrada · commit y push | Coincide. Las pruebas fallaron primero y pasaron después | Las 3 del «Listo cuando» más los valores inválidos | — |
| 0.3 · Base y migraciones | 28-oct | Cerrada · commit de Eber | Coincide. Marcó `[EL DISEÑO NO LO DICE]`: `tupaqnin.db` y `busy_timeout` de 5 s | `make test` pasa (`store` incluido) · prueba de humo con `TQ_PROVIDER=fake make run` | P-1, P-2, P-3 |
| 0.4 · Almacén e historia | 28-oct | Hecha · sin commit | Coincide. Su duda sobre la identidad de una arista reemplazada es real: el hueco viene del backlog (B-8) | `make test` y `make vet` pasan (retorno del agente) | P-6, P-7, B-8 |
| 0.5 · Proveedor de Meta y falso | 28-oct | Hecha · sin commit | Coincide. `store: false`, ítems crudos reenviados byte a byte, errores solo con código y tipo, una fila en `model_call` por llamada. La forma de `text.format` la confirma la 0.8 | Pasan (retorno del agente) | P-8 |
| 0.6 · Streaming | 28-oct | Hecha · sin commit | Coincide. Reenvío desde el inicio, latido cada 15 s y la ruta de 90 s solo con `-tags=devroutes`. Su retorno dice que la prueba desplegada espera H-2: su copia de `docs/` es vieja, porque ahora va en el despliegue de prueba del 30-oct (H-5) | Pasan (retorno del agente) | P-9, P-10 |

## Pendientes

| # | Qué | Quién | Desde | Estado |
|---|---|---|---|---|
| P-1 | `deploy/Dockerfile` copia `go.mod` pero no `go.sum`. Con SQLite, la imagen no compila. Bloquea el despliegue de prueba del 30-oct (H-5) | Agente | 0.3 | Cerrado (commit `fcd5d39`) |
| P-2 | La conexión de lectura puede escribir. La especificación §7 pide «otro de lectura». Corrección: `_pragma=query_only(1)`, con su prueba | Agente | 0.3 | Cerrado (commit `fcd5d39`, con su prueba) |
| P-3 | En el retorno no se listan como `[EL DISEÑO NO LO DICE]` las claves o índices que no da el diagrama (`delivery` y `model_call` no tienen clave). Mirar en 0.4 y 0.9 | Agente | 0.3 | Cumplido en 0.4 a 0.6: cada retorno lista lo que eligió |
| P-4 | `gitignore` (sin punto) está publicado, y al `.gitignore` real le faltan `.env.*`, `*.test`, `.atl/` y `.claude/settings.local.json` | Eber | 0.1 | Cerrado (commit `5424517`) |
| P-5 | En Fly, el volumen `/data` suele ser de root y la imagen corre como `nonroot`: la base podría no poder escribir `[inferencia]` | Despliegue de prueba (H-5, 30-oct) | 0.3 | Abierto |
| P-6 | **El reemplazo de una arista deja una copia idéntica vigente.** `SupersedeEdge` marca la arista como no vigente y crea otra igual, con un id nuevo: la arista «reemplazada» sigue rigiendo. El agente hizo lo que dice el backlog; el hueco es del diseño. Se resuelve con B-8 | Eber (B-8) y después el agente | 0.4 | Abierto |
| P-7 | El evento de una arista no guarda qué arista es (`AddEdge` no llena `data`), así que la historia no alcanza para reconstruir el grafo (estados §8). Va en B-8 | Agente, con B-8 | 0.4 | Abierto |
| P-8 | El cliente HTTP de Meta no tiene tiempo máximo: una llamada colgada deja el turno esperando para siempre. Propuesta: 5 minutos por llamada `[inicial]`, listado como `[EL DISEÑO NO LO DICE]` | Agente | 0.5 | Abierto |
| P-9 | Abrir el stream de un turno que no existe crea una entrada vacía y espera para siempre, solo con latidos. Pasa también con los turnos de antes de un reinicio, porque el hub vive en memoria. Corrección: responder 404 si el turno no existe | Agente | 0.6 | Abierto |
| P-10 | La copia de `docs/` en la carpeta del agente es anterior a H-1, H-5, B3-18 a B3-22 y K-17 | Eber (copiar `docs/`) | 0.6 | Abierto |

## Notas

- **Criterio 3 de `repo-estructura.md` §6 («un solo commit»):** ya no aplica. El repo público tiene 5 commits, y en su historial no hay nada de lo que no se publica.
- **Errores al arrancar:** salen como texto a la salida de error, no en JSON (T-10). Es menor y no bloquea.
- **0.4 cumple lo que se pidió mirar:** el objeto y su evento van en la misma transacción, y toda lectura filtra por proyecto. Menor: `ListObjects` ordena por etiqueta como texto (`dec-10` antes que `dec-2`).
- **Siguiente revisión:** la 0.7 o la 0.8, la que el agente tome primero.
