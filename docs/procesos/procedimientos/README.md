# Procedimientos del Área de Liquidaciones

Cada proceso del área tiene **un procedimiento propio, independiente y completo**, emitido como
documento controlado y auditable. Un auditor puede tomar cualquiera de ellos por separado y
recorrer el proceso de punta a punta sin necesitar los demás.

| Código | Procedimiento | Plazo crítico | Soporte de sistema | Criticidad |
|---|---|---|---|---|
| [PR-LIQ-01](./PR-LIQ-01-conciliacion-tc1.md) | Conciliación del Formato TC1 | Día 12 | 🟩 Sistematizado | Alta |
| [PR-LIQ-02](./PR-LIQ-02-preliquidacion-sdl.md) | Conciliación de la Preliquidación SDL | 1 día calendario por operador | 🟩 Sistematizado | **Alta** |
| [PR-LIQ-03](./PR-LIQ-03-conciliacion-cot.md) | Conciliación del COT | Primera semana de M+2 | 🟥 Manual | Alta |
| [PR-LIQ-04](./PR-LIQ-04-balances-de-energia.md) | Validación y Cruce de Balances de Energía | Última semana del mes | 🟨 Híbrido | Alta |
| [PR-LIQ-05](./PR-LIQ-05-compensaciones.md) | Liquidación y Traslado de Compensaciones | Ciclo de facturación | 🟥 Manual | **Crítica** |
| [PR-LIQ-06](./PR-LIQ-06-cierre-de-mes.md) | Reporte de Cierre de Mes — Pérdidas y Provisiones | **Día 7** | 🟨 Híbrido | **Crítica** |
| [PR-LIQ-07](./PR-LIQ-07-reporte-tc2.md) | Reporte TC2 de Compensaciones a Contabilidad | **Día 15** | 🟥 Manual | Alta |
| [PR-LIQ-08](./PR-LIQ-08-cierre-post-facturacion.md) | Cierre Post-Facturación e Indicadores | Posterior a la facturación | 🟩 Sistematizado | Media-Alta |

## Estructura común de cada procedimiento

Los ocho documentos siguen la misma estructura, para que la revisión sea comparable entre
procesos:

| Sección | Contenido |
|---|---|
| **Encabezado de control** | Código, versión, fecha, elaboró, revisó, aprobó, frecuencia de revisión, estado |
| **1. Objetivo** | Para qué existe el proceso y qué protege |
| **2. Alcance** | Qué incluye y qué explícitamente no |
| **3. Definiciones** | Vocabulario necesario para leer el procedimiento sin apoyo externo |
| **4. Documentos de referencia** | Marco normativo y documentos relacionados |
| **5. Responsabilidades** | Matriz RACI y reglas de segregación aplicables |
| **6. Entradas y salidas** | Con origen, destino y oportunidad |
| **7. Condiciones generales** | Reglas de negocio que gobiernan el procedimiento |
| **8. Descripción del procedimiento** | Actividades numeradas con responsable, sistema, evidencia y control |
| **9. Diagrama de flujo** | El proceso y sus puntos de decisión |
| **10. Puntos de control** | Objetivo, naturaleza, frecuencia, responsable, evidencia, aserción, **prueba de auditoría** y estado real |
| **11. Riesgos** | Operativos y de fraude, con su control mitigante |
| **12. Indicadores** | Con fórmula, meta y frecuencia |
| **13. Registros y retención** | Qué se conserva, dónde, cuánto y quién responde |
| **14. Contingencias** | Qué hacer cuando el proceso no transcurre normalmente |
| **15. Control de cambios** | Historial de versiones del documento |

## Convenciones

- **Códigos de control `C-xx`**: numeración única y transversal. Un mismo control que interviene en
  varios procedimientos conserva su código, para que la matriz consolidada no lo duplique.
- **Estado del control**: 🟢 opera · 🟡 parcial (existe pero le falta cobertura, evidencia o
  aprobación) · 🔴 no existe o no se opera.
- **Códigos `R-xx` y `F-xx`**: riesgos operativos y de fraude del registro consolidado.
- **Controles marcados "por implementar"**: son requisitos del procedimiento que hoy no se
  cumplen. Están así a propósito: el procedimiento describe **el proceso correcto**, y el estado
  del control dice **qué falta para cumplirlo**. Su cierre está planificado en
  [06 — Brechas y plan](../06-brechas-y-plan.md).

## Documentos transversales

Los procedimientos se apoyan en el marco común del área, que **no se repite en cada uno**:

| Documento | Contenido |
|---|---|
| [01 — Manual del área](../01-manual-liquidaciones.md) | Objetivo, alcance, roles, calendario, sistemas, políticas y reglas de valorización |
| [04 — Matriz de riesgos y controles](../04-matriz-de-control.md) | Registro consolidado de riesgos y de los controles `C-xx` |
| [05 — Riesgos de fraude](../05-riesgos-de-fraude.md) | Esquemas de fraude, banderas rojas y segregación de funciones |
| [06 — Brechas y plan](../06-brechas-y-plan.md) | Qué falta y en qué orden se cierra |

## Procesos con soporte de sistema no incluidos en esta serie

Dos procesos del área se ejecutan desde el módulo y están documentados en las
[narrativas](../02-narrativas-procesos.md), a la espera de decisión sobre si requieren
procedimiento propio:

- **P01 · Gobierno del período y cargue de fuentes** — proceso transversal cuyas actividades están
  incorporadas dentro de cada procedimiento que las utiliza.
- **P10 · Cargos STR y orden de compra** — no figuraba en el alcance descrito del área, pero se
  ejecuta desde el módulo y termina en un compromiso de pago a un tercero.

## Mantenimiento

- La versión sube cuando cambia una actividad, un control o un responsable. Los cambios se
  registran en la sección 15 del procedimiento afectado.
- Un cambio en el sistema que **cree, elimine o degrade un control** obliga a actualizar el
  procedimiento correspondiente **y** la matriz consolidada, en el mismo acto.
- Revisión programada: **semestral**.
