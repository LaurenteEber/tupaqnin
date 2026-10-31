---
titulo: Validación y regresión de la v4.2
fecha: 2026-10-26
relacion: proceso, no producto; origen de R1–R6 en diseno/bloque-1-decisiones.md
---

# Validación y regresión · ciclo v4.2

## Validación del documento

Un verificador independiente comparó la v4.2 con todas las decisiones. Encontró 10 problemas y todos se corrigieron:
- C6 estaba deformada: decía «que concluye» en vez de «tiene conclusiones o recomendaciones», y le faltaban partes.
- La parada proponía firmar con «por aclarar» abierto.
- C1 se había reducido a «un caso» en vez de dos: uno de lo que cuenta y otro de lo que no.
- A la vía directa le faltaba «tus criterios».
- Faltaba indicar que volver sobre lo firmado lo pasa a «cambió».
- Quedaba un resto, «por decidir», de cuando existía «Necesita tu decisión».
- Los números de paso del diagrama competían con los de las secciones.
- Algunos detalles adoptados se habían omitido.
- El frontmatter estaba incompleto.

**Términos que usa el bloque 2:** se mantienen estables. Una nota: el bloque 2 dice «lo cambiado desde lo último entregado», que se refiere a lo entregado al agente; el bloque 1 dice «desde tu última sesión», que se refiere al humano. No es contradicción, pero conviene que ese chat lo sepa.

## Regresión (casos 1, 8 y 9 con el perfil apurado)

| Falla en la v4.1 | En la v4.2 |
|---|---|
| Confirmar con «ok» o «sí» (1B, 8B, 9B) | **Corregida.** Las tarjetas quedan abiertas hasta que hay opción y «porque» |
| El agente fija el sentido de «calle comercial» o «pago por uso» (8B, 9B) | **Corregida,** pero depende de una noción de «caso dudoso» sin definir |
| Reentrada tras una edición del humano (8B) | **Corregida** |
| Códigos y relato del proceso en el texto firmado (9B) | **Corregida** |
| Firmar repitiendo lo que se acaba de mostrar (1B) | **Parcial.** Wonder ya no da la respuesta, pero la historia que se muestra se puede repetir al instante |

**¿El «porque» frenó al humano apurado?** No. Hizo falta un «porque» de una línea, y las secciones se firmaron el mismo día. **Límite:** «porque es mi criterio» también pasa, así que el «porque» funciona como reductor de velocidad, no como prueba de comprensión. La prueba de comprensión sigue siendo la firma.

## Precisiones que quedan (R1–R6, para Eber)

Ninguna agrega estados.

1. Definir «caso dudoso».
2. Leer el material del humano (por ejemplo, un boceto) como parte del «Entendí».
3. Después de mostrar la historia de un punto, la pregunta de origen recae sobre otro.
4. Una edición crítica del humano rige de inmediato; su tarjeta solo pide el «porque».
5. Qué pasa con las propuestas abiertas al firmar.
6. Qué cuenta como «ante terceros».
