---
titulo: Versiones recomendadas del stack · seguridad de la cadena de dependencias
estado: v0.2 · 29-oct · adoptada con cambios por Eber: se mantienen las versiones mayores actuales de la web; pnpm 12.16.1 · v0.1 · 29-oct · recomendación
relacion: complementa B-19 (backlog-r1.md) y la pila de bloque-3-especificacion.md §7. No reemplaza ninguna decisión: lo que contradice el diseño está marcado.
---

# Versiones recomendadas del stack

**Respuesta (adoptado el 29-oct):** se fijan las versiones de la columna «Adoptado». La web **se queda en sus versiones mayores actuales**: Vite 6, TypeScript 5 y plugin-react 4 no tienen vulnerabilidades conocidas, y subirlas costaría ≈ 2–4 h `[inferencia]` y choca con B-19 (7). pnpm es la 12.16.1 que instaló Eber y Node la 24.22.2. Se suben las mayores después del hackathon, o antes si `pnpm audit` marca algo.

**Revisión del chat de diseño (29-oct):** Vite 6.4.5 queda fuera de los seis avisos del 20 y 21-oct, que en la línea 6 afectan hasta la 6.4.3 (corregidos en 6.4.4), y del de la línea 8 sola (`?vite-wasm-instance`); casi todos son del servidor de desarrollo. TypeScript 5.9.3: sin avisos; solo paquetes falsos con nombre parecido (typosquatting). @vitejs/plugin-react 4.7.0: sin avisos (los que hay son de `@vitejs/plugin-rsc`). esbuild 0.25.12: el aviso de junio afecta de la 0.27.3 a la 0.28.0, solo en Windows. Ya estaban cubiertos: `govulncheck` en el CI (v1.9.0), las acciones fijadas por SHA y las imágenes de Docker por digest.

**Cómo se verificó:** OSV (`api.osv.dev/v1/querybatch`) para cada versión exacta; base de advisories de npm (`/-/npm/v1/security/advisories/bulk`); árbol web completo resuelto con `pnpm install --lockfile-only` y la configuración de B-19 (98 paquetes, `pnpm audit`: sin vulnerabilidades conocidas), con pnpm 11.28.7 y con 12.15.0; fechas de publicación del registro de npm y del proxy de Go.

## 1. Lo que motivó la revisión (octubre de 2026)

| Incidente | Qué afecta | Corregido en |
|---|---|---|
| Actualización de seguridad de Node, 21-oct: 10 CVE; la principal, CVE-2026-111497 (escritura fuera de rango en el descompresor Brotli, posible ejecución remota), y dos saltos de la verificación TLS | Node 22, 24 y 26 | 22.23.4 · 24.22.1 · 26.12.1 |
| `launch-editor` empaquetado en Vite: inyección de comandos (Windows) y archivos servidos fuera de `server.fs.allow` | Vite 8.0.0 a 8.1.5, y versiones de las líneas 6 y 7 | 8.3.3 en la línea 8 |
| 14 advisories altos en pnpm 11: path traversal, exfiltración de secretos, salto de `allowBuilds` | pnpm 11 anterior a 11.11.0 | 11.11.0 |
| Ataques a la cadena de suministro en npm: gusano Shai-Hulud (keyv, cacheable), @subql/common y tensorlake por sus workflows de publicación, `my-empty-package` (28-oct) | Paquetes comprometidos que roban credenciales al instalarse o al importarse | Defensa: espera de 7 días, `allowBuilds`, `--frozen-lockfile`, `pnpm audit` |

Fuentes: [Node.js · October 2026 Security Releases](https://nodejs.org/en/blog/vulnerability/october-2026-security-releases) · [Wiz · keyv y cacheable](https://www.wiz.io/blog/keyv-and-cacheable-npm-supply-chain-attack) · [CSA Singapur · AD-2026-009](https://www.csa.gov.sg/alerts-and-advisories/advisories/ad-2026-009/) · [reporte de Vite y launch-editor](https://github.com/govtech-bb/gov-bb/issues/3334) · advisories de pnpm en la base de npm.

## 2. Web

| Pieza | Propuesta (v0.1) | **Adoptado** | Nota |
|---|---|---|---|
| Node | 24.22.2 | **24.22.2** (`engines.node: ">=24.22.1"`) | Trae el parche del 21-oct |
| pnpm (`packageManager`) | 12.15.0 | **12.16.1** | La que instaló Eber con Homebrew; B-19 actualizada |
| react · react-dom | 19.3.1 | **19.3.1** | |
| @types/react · @types/react-dom | 19.3.0 | **19.3.0** | |
| typescript | 7.0.2 | **5.9.3** | Sin avisos; la 7 queda para después del hackathon |
| vite | 8.3.5 | **6.4.5** | Fuera de los avisos de octubre; la 8 queda para después |
| @vitejs/plugin-react | 6.1.2 | **4.7.0** | La 6 pide Vite 8 |
| @tiptap/core · /pm · /react · /starter-kit | 3.32.0 | **3.32.0** | Todas iguales; el lockfile actual trae 3.32.1, que aún no cumple los 7 días |
| mermaid | 11.17.2 | **11.17.2** | ≥ 11.15, como pide la espec. |
| esbuild | lo resuelve Vite | **0.25.12** (vía Vite) | Solo en `allowBuilds` |

`web/pnpm-workspace.yaml`, como dice B-19:

```yaml
minimumReleaseAge: 10080
strictDepBuilds: true
blockExoticSubdeps: true
allowBuilds:
  esbuild: true
  fsevents: false
```

## 3. Go

| Pieza | Versión | Nota |
|---|---|---|
| Toolchain | go1.27.3 | La que usa Eber; `go.mod` sigue con `go 1.27` |
| modernc.org/sqlite | v1.61.0 | |
| github.com/yuin/goldmark | v1.8.6 | GFM viene en `goldmark/extension` |
| golang.org/x/text | v0.43.0 | |
| github.com/gowebpki/jcs | v1.0.2 | |
| github.com/gzuidhof/tygo | v0.2.21 | Con la directiva `tool` de `go.mod` (B-19) |
| Librería de PDF | pendiente | La aprueba Eber (B-9); se revisa igual antes de fijarla |

## 4. CI y despliegue

**Adoptado: se quedan las acciones y los digests actuales** (ya fijados por SHA y por digest). La tabla queda como referencia para después del hackathon; sus fechas no se revisaron.

| Acción | Tag | SHA |
|---|---|---|
| actions/checkout | v7.0.1 | `3d3c42e5aac5ba805825da76410c181273ba90b1` |
| actions/setup-go | v7.0.0 | `b7ad1dad31e06c5925ef5d2fc7ad053ef454303e` |
| actions/setup-node | v7.1.0 | `949feb2413d6458794dcd2491c4babbbce0c15c1` |
| pnpm/action-setup | v6.1.0 | `ea17c68df8912ef543352723c149a84f56e3d413` |

Imágenes de `deploy/Dockerfile`: ya están fijadas por digest.

## 5. Decisiones (29-oct)

1. **pnpm 12** (12.16.1) en lugar de 11: adoptado; B-19 y la pieza 1.0 actualizadas.
2. **`govulncheck`:** ya corre en el CI.
3. **Conversión entre TipTap y markdown:** sigue `[EL DISEÑO NO LO DICE]`; si hace falta una extensión, se pregunta antes (regla 11).
4. **Versiones mayores de la web:** se mantienen las actuales durante el hackathon.

## 6. Vigencia

Esta revisión vale al 29-oct-2026. Las bases de vulnerabilidades cambian cada día: `pnpm audit` en cada build (B-19) y, si se adopta, `govulncheck`, la mantienen vigente. Durante el hackathon solo se aplican parches de seguridad, sin versiones mayores (B-19 §7).
