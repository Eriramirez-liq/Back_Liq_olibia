# 01 — Manual del Área de Liquidaciones

BIA Energy · Vicepresidencia Financiera (Finance) · Área de Liquidaciones
Versión 1.0 — 2026-08-31

---

## 1. Objetivo del área

Conciliar los cargos que BIA **factura al usuario final** contra los cargos que los distintos
**agentes del mercado eléctrico** le cobran a BIA, de modo que:

1. **Lo que se cobra sea igual a lo que se paga** por cada frontera comercial y período.
2. Cuando no lo sea, se **identifique la diferencia, se cuantifique en pesos, se le asigne
   causa y responsable, y se gestione** hasta su cierre (reclamo al Operador de Red, ajuste en
   facturación, provisión o reconocimiento de pérdida).
3. La posición económica resultante —**pérdidas, provisiones y compensaciones**— se **reporte a
   Contabilidad** en forma completa, exacta y oportuna.

En términos de control interno, el área es la **segunda línea de defensa sobre el margen de
comercialización**: no factura ni paga, pero determina si lo facturado y lo pagado cuadran, y
cuánto vale la diferencia.

## 2. Alcance

**Dentro del alcance**

| Concepto | Contraparte | Periodicidad |
|---|---|---|
| Formato TC1 (nivel de tensión y propiedad de activos) | Operador de Red (OR) | Mensual |
| Preliquidación SDL (activa, reactiva penalizada inductiva y capacitiva, NT, propiedad, factor M) | OR / XM | Mensual |
| COT (cobro del OR por otros conceptos de transporte) | OR | Mensual |
| Balances de energía | OR | Mensual (afecta meses ya facturados) |
| Compensaciones por calidad del servicio | OR → usuario final | Mensual |
| Cargos STR y su orden de compra | OR / NetSuite | Mensual |
| Reporte de cierre de mes (pérdidas y provisiones) | Contabilidad | Mensual, día 7 |
| Reporte TC2 (compensaciones) | Contabilidad | Mensual, antes del 15 |
| Cierre post-facturación (indicadores de desviación) | Interno / Finance | Mensual |

**Fuera del alcance** (se recibe o se entrega, no se ejecuta): la emisión de la factura al
usuario (Facturación / bills), el registro contable y el pago (Contabilidad y Tesorería), el
reporte de fronteras a XM (Operaciones / Mercados) y la atención comercial del cliente.

## 3. Definiciones clave

| Término | Definición operativa en el área |
|---|---|
| **Frontera comercial** | Punto de medición identificado por su código SIC (`Frt#####`). Es la **unidad de conciliación**: todo se compara frontera por frontera. |
| **Período de consumo** | Mes en que se consumió la energía. **Es la clave de todo el módulo.** |
| **Período de facturación** | Mes en que se factura ese consumo = **consumo + 1**. Las matrices de STR se rotulan por mes de facturación; las conciliaciones, por mes de consumo. Confundirlos es la primera causa de error del área. |
| **E_fac** | Energía activa facturada por BIA al usuario (fuente: Facturación / bills). |
| **E_xm** | Energía reportada a XM para esa frontera (fuente: reporte XM). |
| **E_sdl** | Energía con que el OR liquida el uso de red (fuente: preliquidación SDL del OR). |
| **Pérdida** | Diferencia económica **irrecuperable** originada cuando se reportó a XM **más** energía de la facturada (`E_fac < E_xm`). Se valoriza con **G de bolsa**. |
| **Provisión** | Diferencia económica **por causar**, originada cuando se facturó **más** de lo reportado a XM (`E_fac > E_xm`). Se valoriza con **G de facturación**. Se libera cuando llega el balance de energía del OR. |
| **Disputa (L2)** | Diferencia entre lo que el OR liquida (`E_sdl`) y lo reportado a XM (`E_xm`). Es **reclamable al OR**. |
| **Contingencia (L1)** | Diferencia entre facturación y XM pendiente de que el OR la cobre. Al materializarse define pérdida o ganancia real. |
| **Frontera `_N`** | Sufijos (`Frt11550_1`, `_2`, …) que **solo existen en la facturación de BIA**. El OR y XM manejan la frontera base. Regla: colapsar sumando energías y heredando atributos de la base. |
| **Umbral de materialidad** | **±100 kWh** por frontera. Por debajo, las tres fuentes se consideran iguales. |
| **Accionable** | Decisión que el analista registra sobre una frontera con diferencia: `CAMBIO_SOLICITADO_OR`, `AJUSTE_NO_PROCEDE`, `ERROR_BIA`, `AJUSTE_APLICADO`. |

## 4. Roles y responsabilidades

| Rol | Quién | Responsabilidad en el proceso |
|---|---|---|
| **Líder de Liquidaciones** | Jefatura del área | Aprueba reportes a Contabilidad, autoriza reemplazos de carga y accionables de alto impacto, define umbrales y metas, responde ante Finance por los indicadores. |
| **Analista de Liquidaciones** | Equipo del área | Carga fuentes, ejecuta conciliaciones, analiza diferencias, registra accionables, responde al OR, arma los reportes. |
| **Contabilidad** | Fuera del área | Recibe y registra pérdidas, provisiones y compensaciones. Retroalimenta diferencias contra el mayor. |
| **Facturación (bills)** | Fuera del área | Origen de `E_fac` y de las tarifas aplicadas al usuario. Ejecuta los ajustes que el área solicita como `ERROR_BIA`. |
| **Operaciones / Mercados** | Fuera del área | Reporta la energía a XM. Contraparte interna cuando la diferencia es por reporte. |
| **Operador de Red (OR)** | Externo (21 en SDL/TC1, 23 en STR) | Emite TC1, preliquidación SDL, COT, balances y factura de uso de red. Reconoce o rechaza los reclamos. |
| **XM** | Externo | Fuente independiente de energía por frontera. Es el **árbitro** del proceso: define si el reclamo al OR procede. |
| **TI / Producto** | Fuera del área | Sostiene el módulo (backend, front, integraciones). No opera el proceso ni modifica datos de negocio. |

### Perfiles del sistema

| Perfil | Puede | Uso previsto |
|---|---|---|
| `ADMINISTRADOR` | Todo, incluida administración de usuarios y de operadores (incluido `netsuite_vendor_id`) | Líder de Liquidaciones y TI |
| `ANALISTA` | Cargar fuentes, ejecutar conciliaciones, registrar accionables, exportar | Analistas del área |
| `CONSULTA` | Solo lectura y exportación | Contabilidad, auditoría, Finance |

> **Advertencia de control** (ver [05](./05-riesgos-de-fraude.md) F-08): hoy el backend crea
> automáticamente como `ANALISTA` a cualquier persona con correo `@bia.app` que entre por olibia
> (`OLIBIA_AUTO_PROVISION_USERS=true`, `OLIBIA_DEFAULT_ROL=ANALISTA`). El perfil de entrada
> debería ser `CONSULTA`, y el ascenso a `ANALISTA`, un acto explícito del Líder.

## 5. Calendario del ciclo mensual

El área trabaja sobre el **mes de consumo M**, durante el mes calendario **M+1**. Los hitos son
plazos externos: el del día 12 lo fija el OR, el de los días 8-15 lo fija la preliquidación y el
del día 7 lo fija el cierre contable.

| Día de M+1 | Hito | Proceso |
|---|---|---|
| 1 – 5 | Llegan los formatos **TC1** de los 21 OR | [P02](./02-narrativas-procesos.md#p02--conciliación-tc1) |
| **7** | **Reporte de cierre de mes a Contabilidad** (pérdidas y provisiones) | [P07](./02-narrativas-procesos.md#p07--cierre-de-mes-pérdidas-y-provisiones-a-contabilidad) |
| **8** | Se descarga **Facturación** y el **reporte a XM** del período | [P01](./02-narrativas-procesos.md#p01--gobierno-del-período-y-cargue-de-fuentes-transversal) |
| 8 – 15 | Los OR envían la **preliquidación SDL**. **Respuesta máximo al día siguiente**, incluidos sábados, domingos y festivos | [P03](./02-narrativas-procesos.md#p03--conciliación-de-preliquidación-sdl) |
| **≤ 12** | **TC1 revisado y respondido** a los OR | [P02](./02-narrativas-procesos.md#p02--conciliación-tc1) |
| **≤ 15** | **Reporte TC2** (compensaciones) a Contabilidad | [P08](./02-narrativas-procesos.md#p08--reporte-tc2-a-contabilidad) |
| Última semana de M+1 y primera de M+2 | Llega y se valida el **COT** | [P04](./02-narrativas-procesos.md#p04--conciliación-cot) |
| Última semana de M+1 | Llegan los **balances de energía** de meses anteriores | [P05](./02-narrativas-procesos.md#p05--balances-de-energía) |
| Posterior a la facturación | **Cierre post-facturación**: indicadores de desviación | [P09](./02-narrativas-procesos.md#p09--cierre-post-facturación-indicadores) |
| Según matriz STR | **Cargos STR** y orden de compra al OR | [P10](./02-narrativas-procesos.md#p10--cargos-str-y-orden-de-compra-netsuite) |

> **Riesgo de calendario (R-02).** La ventana de respuesta al SDL es de **un día calendario**,
> corre en fines de semana y festivos, y se solapa con el cierre contable del día 7 y con el
> vencimiento del TC1 del día 12. La presión de tiempo es la racionalización más común del fraude
> ocupacional y la causa más probable de que una revisión se omita. **Un control que no quepa en
> esa ventana no se va a ejecutar**: por diseño, los controles de P03 deben ser automáticos o
> por excepción.

## 6. Sistemas y fuentes de información

### 6.1 Sistemas

| Sistema | Rol en el proceso | Dueño |
|---|---|---|
| **olibia-web** › Finance › Liquidations | Interfaz del área: Cargas, Conciliaciones, Congruencia, Gestiones, Cargos STR, Tarifas SDL, Proyección, Dashboard, Administración | TI |
| **Back_Liq_olibia** (Next 15 + Prisma + Supabase) | Motor de conciliación SDL y TC1, congruencia, gestiones, integración NetSuite | TI |
| **bia-bills** (Go, `/ms-bill/liquidations/*`) | Módulos migrados: Cargos STR, Tarifas SDL, TC1, Proyección. Escribe en las bases `file-compiler` y `calculator-prices` | TI |
| **Metabase** | Facturación, XM, G de bolsa (card 1237) y demanda proyectada (card 77419) | Datos |
| **Oracle NetSuite** | Emisión de la **orden de compra** al OR por cargos STR (departamento 131) | Finance / TI |
| **Correo corporativo** | Recepción de TC1, SDL, COT, balances y compensaciones; respuesta al OR | Área |
| **Excel** | Procesos **aún no sistematizados**: COT, balances, compensaciones, TC2 y reporte de cierre | Área |

> **Convivencia de dos backends.** Mientras dura la migración a Go, el módulo expone endpoints en
> dos formas (`/api/...` y `/ms-bill/liquidations/...`) y mantiene **dos copias de algunos
> datos**: TC1, por ejemplo, se carga en `file-compiler` pero Conciliaciones, Congruencia y
> Gestiones lo siguen leyendo de `registroTC1` en Supabase, que ya no se alimenta. Es transitorio,
> pero con impacto de control real: ver [06 — Brechas](./06-brechas-y-plan.md) B-01.

### 6.2 Fuentes de información (entradas del proceso)

| Fuente | Origen | Formato | Frecuencia | Tipo en el sistema |
|---|---|---|---|---|
| Facturación BIA | bills / Metabase card 73360 | XLSX o consulta | Mensual, día 8 | `FACTURACION` |
| Reporte XM | XM / Metabase card 76099 | XLSX o consulta | Mensual, día 8 | `XM` |
| Preliquidación SDL | OR (correo) | XLSX por OR | Mensual, días 8-15 | `SDL` |
| TC1 | OR (correo) | Plano CREG de 33 columnas | Mensual, días 1-5 | `TC1` |
| Balance de energía | OR (correo) | XLSX por OR | Mensual, última semana | `BALANCE` |
| COT | OR (correo) | XLSX por OR | Mensual, fin/inicio de mes | `COT` |
| Insumos STR | XM / OR | XLSX `BalanceSTRTipoFactu<AAAA>-<MES>` | Mensual | `INSUMOS_STR` |
| Insumos de tarifas SDL | XM / OR | XLSX `Cargo_Cobro_Uso_Red-Definitivo<COD>-<AAAAMM>` + ADD | Mensual | `INSUMOS_TARIFAS_SDL` |

## 7. Políticas del área

1. **P-01 · La frontera es la unidad de control.** Toda diferencia se identifica, valoriza y
   gestiona **por frontera y período**. No se aceptan ajustes globales sin desagregación.
2. **P-02 · Ninguna fuente se descarta por ausencia.** El universo de conciliación es la **unión**
   de las fronteras de todas las fuentes. Una frontera presente en una sola fuente se reporta
   como `INCOMPLETA`; nunca se omite.
3. **P-03 · El período de consumo se declara y se verifica.** El período elegido al cargar se
   contrasta contra el que traen los archivos. Si no coincide, no se carga.
4. **P-04 · Las cargas son append-only.** Nada se borra. Un dato equivocado se corrige con una
   carga nueva **justificada**, que deja la anterior en el historial.
5. **P-05 · Toda diferencia se cierra con un accionable.** Una frontera con diferencia sin
   accionable registrado es un pendiente del período, no un "no aplica".
6. **P-06 · Lo reportado a Contabilidad se congela.** Emitido el reporte del día 7, el período no
   se re-ejecuta sin dejar constancia y sin reemitir el reporte.
7. **P-07 · Quien liquida no aprueba, y quien aprueba no paga.** Ver la matriz de segregación de
   funciones en [05](./05-riesgos-de-fraude.md) §4.
8. **P-08 · Toda comunicación con el OR es evidencia.** Reclamos, aceptaciones y rechazos se
   conservan y se asocian al período y a la frontera.
9. **P-09 · El árbitro es XM.** Si el dato de BIA coincide con XM, se reclama al OR. Si el del OR
   coincide con XM, no se refuta. La regla no se negocia caso a caso.
10. **P-10 · Las metas de indicadores no se ajustan para cumplirlas.** Congruencia > 95 %,
    pérdida < 0,1 %, diferencia absoluta en kWh < 0,35 %, reporte de más a XM < 0,15 %, reporte
    de menos < 0,2 %. Cambiar una meta es decisión de Finance, documentada.

## 8. Reglas de valorización (base del cálculo económico)

Toda diferencia de energía se convierte a pesos con la **misma estructura tarifaria**; lo único
que cambia es el componente de generación:

```
Valor = Δ energía (kWh) × ( G + T + D + PR + R )
```

| Componente | Origen | Alcance |
|---|---|---|
| **G** — generación | **Pérdida → G de bolsa nacional** (Metabase card 1237, mes de consumo) · **Provisión → G de facturación** | Igual para todas las fronteras del mes |
| **T** — transmisión | Facturación | Igual para todas las fronteras del mes |
| **D** — distribución | Facturación (tarifa del usuario) | **Por frontera** |
| **PR** — pérdidas | Facturación (tarifa del usuario) | **Por frontera** |
| **R** — restricciones | Facturación | Igual para todas las fronteras del mes |
| **C** — comercialización | No participa en pérdidas ni provisiones | — |

**Excepción (casos D3 y B1 extendido):** cuando el OR liquida igual que BIA pero XM difiere, la
distribución se toma **neta del cargo del OR**: `D − tarifa_sdl`. Es la única variante de la
fórmula y la aplica el motor automáticamente.

> **Control C-20.** Si Metabase no responde, el motor cae al `g_bolsa` por frontera de
> facturación. Ese fallback **cambia el valor de la pérdida sin avisar al analista**. Toda
> conciliación ejecutada antes de tener la G de bolsa del mes debe **re-ejecutarse** ("Recalcular
> pérdidas" en el dashboard) **antes** de emitir el reporte del día 7.

## 9. Documentos y registros del área

| Registro | Dónde vive | Retención | Quién accede |
|---|---|---|---|
| Archivos recibidos del OR (TC1, SDL, COT, balances) | Correo + carpeta del área | 5 años | Área, auditoría |
| Cargas del sistema (`cargas_fuente` / `*_loads`) | Base de datos, append-only | Permanente | Área, TI |
| Resultados de conciliación (`resultados_conciliacion`, `resultados_conciliacion_tc1`) | Base de datos | Permanente | Área, auditoría |
| Accionables (`gestiones_frontera`) | Base de datos | Permanente | Área, auditoría |
| Provisiones, contingencias, disputas y cruces de balance | Base de datos | Permanente | Área, Contabilidad |
| Log de auditoría (`log_auditoria`) | Base de datos | Permanente | Administrador, auditoría |
| Lotes y envíos a NetSuite | Base de datos + NetSuite | Permanente | Área, Finance |
| Reportes a Contabilidad (cierre y TC2) | Excel + correo | 5 años | Área, Contabilidad |
| Soporte de compensaciones pagadas por banco | **Sin repositorio formal** (brecha B-06) | 5 años | Área, Tesorería |
