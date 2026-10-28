---
titulo: Bloque 2 · Hipótesis y su estado
relacion: bloque-2.md (la idea) · decisiones.md · pruebas en ../../casos-prueba/ (relevo-b2-v1, relevo-b2-v2, relevo-b2-v3)
---

# Bloque 2 · Hipótesis

*Lo que el bloque 2 da por cierto sin haberlo probado del todo (reglas §2.3). Cada una dice cómo se prueba y qué se sabe hasta ahora.*

| Hipótesis | Cómo se prueba | Hasta ahora |
|---|---|---|
| Un principal nuevo con solo el arranque continúa igual que el que no se reinició | Prueba de relevo | Sí (relevo y regresión, con el arranque completo de la v2) |
| La memoria calculada desde el grafo basta para continuar | Regresión de relevo en los casos 3, 7 y 10 | Sí (regresión) |
| La primera línea basta como entrada del índice | En el caso 3, el principal encuentra lo que necesita sin abrir de más | Sí, donde existe (regresión) |
| La nota del agente aporta algo que la memoria no tiene | Relevo con y sin nota | Casi nada; lo que aportó ahora va en «diferido» (B2-6) |
| El arranque por tarea basta para continuar | Regresión del caso 3 con el arranque por tarea | Casi: continuó la parada, la objeción y el recálculo con un arranque 28 % más chico que la v2 (≈ 1 % de una ventana de 128 mil tokens). Perdió una decisión crítica por una ambigüedad del texto, ya corregida en §3 (relevo-b2-v3) |
| Las sondas detectan las contradicciones que el principal no declara | Contradicciones sembradas en un relevo | 3 de 6, con 0 falsos positivos. Las 3 que fallaron no se sondearon (selección); el juicio no falló ninguna. Dos fallas ya se corrigieron en §5; la tercera es el hueco de las críticas sin vínculo (relevo-b2-v3) |
| Con el umbral en ~40 % libre la calidad se sostiene | La misma tarea con distintas ocupaciones del contexto, por modelo | Sin probar; requiere los modelos reales |
