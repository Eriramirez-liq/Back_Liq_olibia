# Procedimientos — Área de Liquidaciones

> **Documento maestro del área.** Describe formalmente los procesos, los flujos de
> transacciones, los controles y los riesgos (operativos y de fraude) del área de
> Liquidaciones de BIA Energy.
>
> | | |
> |---|---|
> | **Versión** | 1.0 |
> | **Fecha de emisión** | 2026-08-31 |
> | **Elaborado por** | Liquidaciones — Erika Ramírez |
> | **Estado** | Borrador para validación con Finance, Contabilidad y Auditoría |
> | **Próxima revisión** | Semestral, o ante cambio de proceso / sistema |

## Contenido

### Procedimientos — un documento controlado por proceso

Cada proceso del área tiene su **procedimiento propio, independiente y auditable**. Índice y
estructura común en [procedimientos/](./procedimientos/).

| Código | Procedimiento | Plazo crítico | Criticidad |
|---|---|---|---|
| [PR-LIQ-01](./procedimientos/PR-LIQ-01-conciliacion-tc1.md) | Conciliación del Formato TC1 | Día 12 | Alta |
| [PR-LIQ-02](./procedimientos/PR-LIQ-02-preliquidacion-sdl.md) | Conciliación de la Preliquidación SDL | 1 día calendario | **Alta** |
| [PR-LIQ-03](./procedimientos/PR-LIQ-03-conciliacion-cot.md) | Conciliación del COT | Primera semana de M+2 | Alta |
| [PR-LIQ-04](./procedimientos/PR-LIQ-04-balances-de-energia.md) | Validación y Cruce de Balances de Energía | Última semana | Alta |
| [PR-LIQ-05](./procedimientos/PR-LIQ-05-compensaciones.md) | Liquidación y Traslado de Compensaciones | Ciclo de facturación | **Crítica** |
| [PR-LIQ-06](./procedimientos/PR-LIQ-06-cierre-de-mes.md) | Reporte de Cierre de Mes — Pérdidas y Provisiones | **Día 7** | **Crítica** |
| [PR-LIQ-07](./procedimientos/PR-LIQ-07-reporte-tc2.md) | Reporte TC2 de Compensaciones | **Día 15** | Alta |
| [PR-LIQ-08](./procedimientos/PR-LIQ-08-cierre-post-facturacion.md) | Cierre Post-Facturación e Indicadores | Post facturación | Media-Alta |

### Marco transversal

Sustenta los ocho procedimientos y no se repite en cada uno.

| # | Documento | Para qué |
|---|-----------|----------|
| 01 | [Manual del área](./01-manual-liquidaciones.md) | Objetivo, alcance, roles, calendario, sistemas, glosario y políticas |
| 02 | [Narrativas de procesos](./02-narrativas-procesos.md) | Los 10 procesos paso a paso: quién, en qué sistema, con qué evidencia |
| 03 | [Diagramas de flujo](./03-diagramas-flujo.md) | Flujos de transacción, flujo de datos y calendario, en diagramas |
| 04 | [Matriz de control](./04-matriz-de-control.md) | Riesgos operativos, controles, aserciones, responsables y estado |
| 05 | [Riesgos de fraude](./05-riesgos-de-fraude.md) | Esquemas de fraude, banderas rojas, segregación de funciones |
| 06 | [Brechas y plan de remediación](./06-brechas-y-plan.md) | Lo que el proceso exige y el sistema todavía no hace |

## Cómo se mantiene

- **El proceso manda; el sistema lo soporta.** Cuando cambie el proceso, se actualiza la
  narrativa (02) y, si nace o muere un control, la matriz (04) en el mismo acto.
- **Todo control tiene dueño, evidencia y periodicidad.** Un control sin evidencia
  verificable no es un control: es una intención. En la matriz se marca como brecha.
- **Los cambios del sistema que afectan un control** se registran además en
  [`INTEGRACION.md`](../../INTEGRACION.md) §5, que es la bitácora técnica.
- Referencias técnicas: [CONTEXTO_MIGRACION.md](../../CONTEXTO_MIGRACION.md) (modelo de datos y
  motor de cálculo), [docs/backend/](../backend/) (integraciones), [docs/runbooks/](../runbooks/)
  (operación).
