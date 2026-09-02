# 02 — Narrativas de Procesos

Área de Liquidaciones · Versión 1.0 — 2026-08-31

Cada narrativa describe **quién hace qué, en qué sistema, con qué evidencia y qué controla**.
Los códigos `C-xx` remiten a la [matriz de control](./04-matriz-de-control.md); los `R-xx` y
`F-xx`, al [registro de riesgos](./04-matriz-de-control.md#2-registro-de-riesgos-operativos) y a
[riesgos de fraude](./05-riesgos-de-fraude.md). Los diagramas están en
[03](./03-diagramas-flujo.md).

**Convención de estado de sistematización**

| Marca | Significado |
|---|---|
| 🟩 **Sistematizado** | El paso lo ejecuta o lo valida el módulo, y deja rastro en base de datos |
| 🟨 **Híbrido** | El sistema aporta el dato, pero la decisión y el registro son manuales (Excel / correo) |
| 🟥 **Manual** | El proceso vive completamente fuera del módulo |

---

## Inventario de procesos

| Código | Proceso | Estado | Frecuencia | Plazo crítico |
|---|---|---|---|---|
| [P01](#p01--gobierno-del-período-y-cargue-de-fuentes-transversal) | Gobierno del período y cargue de fuentes | 🟩 | Mensual | Día 8 |
| [P02](#p02--conciliación-tc1) | Conciliación TC1 | 🟩 | Mensual | Día 12 |
| [P03](#p03--conciliación-de-preliquidación-sdl) | Conciliación de preliquidación SDL | 🟩 | Mensual | 1 día calendario por OR |
| [P04](#p04--conciliación-cot) | Conciliación COT | 🟥 | Mensual | Primera semana de M+2 |
| [P05](#p05--balances-de-energía) | Balances de energía | 🟨 | Mensual | Última semana |
| [P06](#p06--compensaciones-por-calidad-del-servicio) | Compensaciones por calidad del servicio | 🟥 | Mensual | Ciclo de facturación |
| [P07](#p07--cierre-de-mes-pérdidas-y-provisiones-a-contabilidad) | Cierre de mes: pérdidas y provisiones | 🟨 | Mensual | **Día 7** |
| [P08](#p08--reporte-tc2-a-contabilidad) | Reporte TC2 a Contabilidad | 🟥 | Mensual | **Día 15** |
| [P09](#p09--cierre-post-facturación-indicadores) | Cierre post-facturación (indicadores) | 🟩 | Mensual | Post facturación |
| [P10](#p10--cargos-str-y-orden-de-compra-netsuite) | Cargos STR y orden de compra NetSuite | 🟩 | Mensual | Ciclo de pago al OR |

---

## P01 — Gobierno del período y cargue de fuentes (transversal)

**Objetivo.** Constituir, para cada mes de consumo, el conjunto completo y verificado de datos
sobre el que se ejecutan todas las conciliaciones del área.

**Disparador.** Inicio del ciclo mensual. La descarga de Facturación y XM ocurre el **día 8**;
las fuentes de los OR llegan según su propio calendario.

**Entradas.** Facturación BIA · Reporte XM · Preliquidación SDL (21 archivos) · TC1 (21
archivos) · Balances · COT · Insumos STR · Insumos de tarifas SDL.

**Salidas.** Período constituido con sus fuentes cargadas y trazadas; panel de "Estado del
período" con el avance por fuente y por operador.

**Actores.** Analista (ejecuta) · Líder (autoriza reemplazos) · Sistema (valida).

### Narrativa

1. 🟩 **El analista selecciona el mes de consumo** en *Cargas › Nueva carga*. El sistema
   **rechaza el mes en curso y los futuros**: solo se carga hasta el mes anterior, porque el
   período de consumo debe estar cerrado (**C-02**).
2. 🟩 **Selecciona el tipo de fuente y, si aplica, el operador.** SDL y TC1 exigen operador —son
   las dos fuentes donde cada OR manda su propio archivo—; las demás son archivo único.
3. 🟩 **Sube el archivo y el sistema genera una vista previa.** No se guarda nada todavía: el
   paso de *preview* separa la lectura del archivo de la decisión de incorporarlo (**C-09**).
   Para TC1 el archivo se parsea **en el navegador**: los planos CREG pesan hasta 98 MB y de
   743.530 filas quedan ~127 tras filtrar por `ID_COMERCIALIZADOR = 62371` (BIA).
4. 🟩 **El sistema contrasta el período declarado contra el que traen los nombres de archivo**
   (**C-01**). En Cargos STR (`BalanceSTRTipoFactu<AAAA>-<MES>`) y en Tarifas SDL
   (`Cargo_Cobro_Uso_Red-Definitivo<COD>-<AAAAMM>`) la discrepancia es **error crítico**: no se
   procesa y el botón de confirmar queda deshabilitado. Si el archivo no trae mes en el nombre,
   solo se advierte.
   > Este control nace de dos incidentes reales ya ocurridos: un cargue de 2026-05 hecho con el
   > archivo de abril y uno de 2026-07 hecho con los de junio.
5. 🟩 **El sistema informa si ya existe una carga previa** para ese período, fuente y operador
   (**C-03**). El analista elige:
   - **Reemplazar** → exige **justificación escrita obligatoria**. Queda registrada la carga
     anterior, la nueva, el motivo y el usuario.
   - **Agregar** → habilitado **solo para SDL de EEP Pereira** (archivos complementarios por
     nivel de tensión) **y EPM** (archivo de activa y archivo de reactiva, que se fusionan por
     código SIC). Cualquier otro caso lo rechaza el sistema.
6. 🟩 **Confirma la carga.** La escritura es **append-only** (**C-04**): no se borra nada; lo
   vigente es la última carga (`load_id`), y queda registrado quién cargó qué y cuándo.
7. 🟩 **Verifica el avance en *Estado del período*** (**C-05**): por fuente, cargada o pendiente;
   para SDL y TC1, el contador `n/21` con la lista de operadores faltantes.
8. 🟨 **Cierre del cargue.** Antes de conciliar, el analista confirma que están las fuentes
   necesarias. **El cierre formal del período (`EstadoPeriodo = CERRADO`) existe en el modelo de
   datos pero no se usa operativamente** — brecha B-04.

### Controles del proceso

| ID | Control | Tipo | Estado |
|---|---|---|---|
| C-01 | Validación del período declarado contra el nombre del archivo | Preventivo · automático | Implementado (STR y Tarifas SDL) |
| C-02 | Bloqueo de mes en curso y futuros | Preventivo · automático | Implementado |
| C-03 | Justificación obligatoria para reemplazar una carga | Preventivo · automático | Implementado (texto libre, sin aprobación) |
| C-04 | Modelo append-only con trazabilidad de usuario y fecha | Detectivo · automático | Implementado |
| C-05 | Panel de completitud `n/21` por fuente y operador | Detectivo · automático | Implementado |
| C-09 | Vista previa obligatoria antes de confirmar | Preventivo · automático | Implementado |

### Riesgos asociados

R-01 (período equivocado), R-03 (fuente incompleta que oculta diferencias), R-04 (archivo
alterado antes de cargarse), F-02, F-03.

---

## P02 — Conciliación TC1

**Objetivo.** Verificar que el **nivel de tensión** y la **propiedad de los activos** que cada OR
reporta en el formato TC1 coincidan con los que BIA usa para facturar. Son los dos atributos que
determinan la tarifa: un error en ellos se traduce directamente en cobro incorrecto al usuario,
mes tras mes, hasta que alguien lo detecta.

**Disparador.** Recepción de los TC1 en los primeros días del mes.
**Plazo.** **Revisado y respondido antes del día 12.**

**Entradas.** 21 archivos TC1 (uno por OR) · Facturación del período.
**Salidas.** Resultado de conciliación por frontera · accionables registrados · comunicación al
OR · insumo del indicador de congruencia.

### Narrativa

1. 🟩 **Recepción.** Cada OR envía su TC1 por correo entre los días 1 y 5. El analista verifica
   contra la lista de 21 operadores esperados y **reclama los faltantes** (**C-05**).
2. 🟩 **Cargue** según P01. El parser normaliza las 33 columnas canónicas **por posición** —el
   layout CREG es fijo— y luego **pisa 7 columnas críticas con lo detectado por nombre**, que es
   lo que absorbe las particularidades de cada OR: el typo de CEDENAR (`FROTERA`), EMCALI que
   llama `SIC` a la frontera, ENEL que intercala una columna y desalinea el mapeo posicional,
   CENS con dos columnas de nivel de tensión, ESSA con filas vacías antes del encabezado, AFINIA
   con bytes nulos alrededor de los valores (**C-11**).
   > **Regla no obvia:** en CENS el nivel de tensión válido es **la primera columna** de nivel,
   > no la que se llama así. Se toma **por posición**, nunca por nombre.
3. 🟩 **Validación estructural al confirmar:** período con forma válida, operador presente,
   fronteras ni vacías ni repetidas (**C-12**).
4. 🟩 **Ejecución de la conciliación TC1** (*Conciliaciones › TC1*). El universo es la **unión**
   de las fronteras de Facturación y de TC1, cruzadas por código de frontera normalizado
   (**C-10**). Antes de comparar se aplica el **colapso de fronteras `_N`** (**C-19**).
   Resultado por frontera:
   - `SIN_DIFERENCIA` — coinciden nivel de tensión y propiedad.
   - `DIFERENCIA` — difiere uno o ambos atributos.
   - `INCOMPLETA` — la frontera está en una sola de las dos fuentes, o falta un campo.
5. 🟩 **Análisis en *Gestiones › TC1*.** El analista revisa cada frontera con diferencia,
   determina de qué lado está el error y **registra el accionable** (**C-14**):

   | Accionable | Cuándo | Consecuencia |
   |---|---|---|
   | `CAMBIO_SOLICITADO_OR` | El dato correcto es el de BIA | Se reclama al OR y se espera corrección en el TC1 siguiente |
   | `ERROR_BIA` | El dato correcto es el del OR | Se solicita ajuste a Facturación |
   | `AJUSTE_APLICADO` | Ya se corrigió | Debe indicar **qué campos** se ajustaron |
   | `AJUSTE_NO_PROCEDE` | La diferencia no genera acción | **Requiere justificación — es el accionable de mayor riesgo** (F-01) |

6. 🟩 **Contraste con la tercera fuente (*Congruencia*).** El módulo cruza **Facturación, SDL y
   TC1** simultáneamente y determina **cuál de las tres es la que difiere** (**C-16**): "Cambio
   TC1", "Cambio SDL", "Cambio bills", "No se relaciona en …" o "Revisar" cuando ninguna coincide.
   Este cruce es el que evita que el analista decida "a ojo" cuál fuente tiene la razón.
7. 🟨 **Respuesta al OR antes del día 12**, por correo, con el detalle de fronteras observadas.
   **La evidencia de esa respuesta vive en el correo, no en el módulo** — brecha B-05.
8. 🟩 **Seguimiento.** Las fronteras reclamadas se verifican en el TC1 del mes siguiente.

### Controles del proceso

| ID | Control | Tipo | Estado |
|---|---|---|---|
| C-11 | Parser con mapeo posicional y override por nombre, por operador | Preventivo · automático | Implementado, verificado contra los 21 archivos reales |
| C-12 | Validación de fronteras vacías o duplicadas | Preventivo · automático | Implementado |
| C-13 | Conciliación automática de NT y propiedad, universo = unión | Detectivo · automático | Implementado |
| C-14 | Accionable obligatorio por frontera con diferencia | Detectivo · manual | Implementado (sin revisión independiente) |
| C-15 | Cumplimiento del plazo del día 12 | Preventivo · manual | **Sin medición formal** |
| C-16 | Congruencia entre las 3 fuentes con meta > 95 % | Detectivo · automático | Implementado |

### Riesgos asociados

R-05 (tarifa incorrecta al usuario por NT/propiedad errada, con efecto acumulativo),
R-06 (vencimiento del plazo del OR), F-01, F-09.

---

## P03 — Conciliación de preliquidación SDL

**Objetivo.** Conciliar, frontera por frontera, lo que el OR liquida por uso de red contra lo que
BIA facturó y contra lo reportado a XM; determinar el efecto económico (**pérdida, provisión o
disputa**) y reclamar al OR lo que proceda.

**Este es el proceso central del área y el de mayor impacto económico.**

**Disparador.** Recepción de la preliquidación de cada OR, entre los días 8 y 15.
**Plazo.** **Respuesta máximo al día siguiente de recibida, sin excepción por fin de semana o
festivo.**

**Entradas.** Preliquidación SDL por OR (energía activa, reactiva penalizada inductiva y
capacitiva, nivel de tensión, propiedad de activos, factor M) · Facturación (día 8) · Reporte XM
(día 8) · Tarifas SDL del período · G de bolsa del mes.

**Salidas.** Resultado por frontera con caso y valor · provisiones · contingencias (pérdidas) ·
disputas contra el OR · accionables · reclamo enviado al OR.

### Narrativa

1. 🟩 **Día 8 — descarga de las dos fuentes de referencia.** Facturación y reporte XM del período
   de consumo, desde Metabase o por archivo (**C-17**). Se hacen **antes** de que lleguen las
   preliquidaciones, a propósito: la referencia se fija antes de conocer lo que el OR va a
   cobrar.
2. 🟩 **Recepción y cargue de la preliquidación** de cada OR según P01.
3. 🟩 **Preparación del universo.** El motor cruza por código de frontera normalizado sobre la
   **unión** de las tres fuentes (**C-10**) y aplica el **colapso `_N`** (**C-19**): agrupa por
   clave base, **suma** activa y reactiva de todas las variantes, **hereda** nivel de tensión,
   propiedad, tarifa y factor M de la frontera base, y deduplica antes de sumar.
   > Sin este paso hay doble conteo de energía activa, y con él una frontera que en BIA está
   > partida en `_1`, `_2`, `_8` se compara correctamente contra el único registro del OR.
4. 🟩 **Obtención de la G de bolsa nacional** del mes de consumo (Metabase card 1237). Se aplica a
   **todas** las fronteras. Si Metabase falla, el motor usa el `g_bolsa` de facturación por
   frontera: **el resultado cambia y no hay alerta** (**C-20**, riesgo R-08).
5. 🟩 **Clasificación automática de cada frontera** con umbral de **±100 kWh**
   (`Δ₁ = E_fac − E_xm`, `Δ₂ = E_xm − E_sdl`):

   | Caso | Condición | Resultado | Valor |
   |---|---|---|---|
   | **A1** | fac ≈ xm ≈ sdl | Sin diferencia | — |
   | **B1** | fac < xm = sdl | **Pérdida** (contingencia L1) | `(xm−fac) × (G_bolsa + T + D + PR + R)` |
   | **B1-ext** | fac ≈ sdl < xm | **Pérdida** | `(xm−fac) × (G_bolsa + T + (D − tarifa_sdl) + PR + R)` |
   | **B2** | fac > xm = sdl | **Provisión** | `(fac−xm) × (G_bia + T + D + PR + R)` |
   | **C1** | fac = xm > sdl | **Disputa** — el OR cobró de menos | `|Δ₂| × tarifa_sdl` |
   | **C2** | fac = xm < sdl | **Disputa** — el OR cobró de más | `|Δ₂| × tarifa_sdl` |
   | **D1** | fac < sdl < xm | Pérdida **+ alerta manual** | como B1 |
   | **D2** | fac > sdl > xm | Provisión **+ alerta manual** | como B2 |
   | **D3** | xm < fac = sdl | Provisión con tarifa neta | `(fac−xm) × (G_bia + T + (D − tarifa_sdl) + PR + R)` |
   | **D4** | los tres distintos | **Alerta manual** + pérdida o provisión según el signo | según corresponda |
   | **INCOMPLETA** | falta XM o SDL | Sin clasificar | — |

   **La regla de negocio detrás de los casos** es la política P-09: si BIA coincide con XM, el que
   difiere es el OR y **se reclama**; si el OR coincide con XM, **no se refuta**; si BIA difiere
   de XM, hay pérdida (XM por encima) o provisión (XM por debajo) (**C-18**).
6. 🟩 **Conciliación de los demás campos de la preliquidación** contra facturación: reactiva
   penalizada **inductiva**, reactiva penalizada **capacitiva**, **factor M**, **nivel de
   tensión** y **propiedad de activos**. Cada uno marca su propia bandera de diferencia. Un campo
   nulo frente a un campo con dato **cuenta como diferencia**, no como coincidencia.
7. 🟩 **Persistencia del resultado.** Por cada frontera se guarda el caso, los tres valores de
   energía, los deltas, el impacto financiero de L1 y L2 y las banderas de diferencia; y se crean
   los registros de **provisión**, **contingencia** y **disputa** correspondientes. La operación
   es idempotente por período y frontera.
8. 🟩 **Revisión obligatoria de alertas manuales.** Los casos **D1, D2 y D4** —donde las tres
   fuentes difieren— no se cierran automáticamente: exigen análisis del analista (**C-21**).
9. 🟩 **Registro de accionables** en *Gestiones › SDL* para toda frontera con diferencia
   (**C-14**). El módulo lista como "con diferencia" toda frontera con alguna bandera activa,
   caso `INCOMPLETA`/`ERROR`, o con las tres energías presentes y `|E_fac − E_sdl| > 100` **y**
   `|E_sdl − E_xm| > 100`.
10. 🟨 **Reclamo al OR dentro de la ventana de un día** (**C-22**). Se exporta el detalle de las
    fronteras en disputa y se envía por correo. **La evidencia del reclamo y de la respuesta del
    OR vive en el correo** — brecha B-05.
11. 🟩 **Cierre.** Las provisiones quedan `PENDIENTE` hasta cruzarse con el balance del OR (P05);
    las pérdidas alimentan el reporte del día 7 (P07); las disputas siguen su ciclo hasta
    `RESUELTA` o `CERRADA_SIN_AJUSTE`.

### Controles del proceso

| ID | Control | Tipo | Estado |
|---|---|---|---|
| C-17 | Descarga de Facturación y XM el día 8, previa a la preliquidación | Preventivo · manual | Implementado |
| C-18 | Motor de clasificación A1–D4 con umbral de 100 kWh | Detectivo · automático | Implementado, con casos de prueba documentados |
| C-19 | Colapso de fronteras `_N` con herencia y deduplicación | Preventivo · automático | Implementado |
| C-20 | G de bolsa del mes desde Metabase, con re-ejecución antes del cierre | Preventivo · automático + manual | **Parcial**: el fallback silencioso es una brecha |
| C-21 | Revisión obligatoria de alertas manuales (D1, D2, D4) | Detectivo · manual | Implementado (marca del sistema, sin bloqueo de cierre) |
| C-22 | Respuesta al OR dentro de un día calendario | Preventivo · manual | **Sin medición formal** |
| C-14 | Accionable obligatorio por frontera con diferencia | Detectivo · manual | Implementado (sin revisión independiente) |

### Riesgos asociados

R-07 (aceptar un cobro del OR superior al que corresponde), R-08 (valorización errada por G de
bolsa), R-09 (diferencia no reclamada dentro del plazo y perdida), R-10 (provisión sin
seguimiento que nunca se libera), F-01, F-04, F-09.

---

## P04 — Conciliación COT

**Objetivo.** Validar que el cobro que el OR hace por COT sea correcto: que la **energía** cuadre,
que la **tarifa** sea la que corresponde, que el **usuario aplique** para el cobro y que **lo
cobrado por el OR sea lo mismo que BIA cobró** al usuario.

**Disparador.** Reporte del OR, entre la última semana del mes y la primera del siguiente.

**Entradas.** Reporte COT del OR · Facturación del período · Base de usuarios sujetos al cobro.
**Salidas.** Validación del cobro · reclamo al OR cuando corresponda · registro de diferencias.

### Narrativa

1. 🟥 **Recepción del reporte COT** de cada OR por correo.
2. 🟥 **Verificación de la energía**: la energía sobre la que el OR liquida el COT debe coincidir
   con la del período conciliado.
3. 🟥 **Verificación de la tarifa** aplicada por el OR.
4. 🟥 **Verificación de aplicabilidad**: que el usuario efectivamente **aplique** para el cobro y
   que **se le haya cobrado** en su factura. Este paso cubre dos errores opuestos —cobrar a quien
   no aplica y no cobrar a quien sí— y es el de mayor exposición de margen del proceso.
5. 🟥 **Contraste de lo cobrado por el OR contra lo cobrado por BIA** al usuario.
6. 🟥 **Gestión de diferencias** con el OR por correo.

### Estado y brecha

> **P04 es el proceso menos protegido del área.** El sistema tiene el tipo de fuente `COT` para
> cargar el archivo, pero **no existe motor de conciliación COT**: el módulo de Gestiones
> devuelve siempre una lista vacía para ese concepto. Todo el análisis se hace en Excel, sin
> universo de conciliación, sin valorización automática, sin accionable trazado y sin evidencia
> estructurada. Ver brecha **B-02**.
>
> Cuando se implemente, debe seguir la misma regla de universo que SDL y TC1: **unión de
> fuentes**, con las fronteras de un solo lado reportadas como `INCOMPLETA`.

### Controles del proceso

| ID | Control | Tipo | Estado |
|---|---|---|---|
| C-23 | Verificación de energía, tarifa y aplicabilidad del COT | Detectivo · manual | **Manual, sin evidencia estructurada** |
| C-24 | Contraste de lo cobrado por el OR contra lo cobrado al usuario | Detectivo · manual | **Manual** |

### Riesgos asociados

R-11 (cobro del OR por usuarios que no aplican), R-12 (BIA no traslada el cobro al usuario y
absorbe el costo), R-13 (falta de trazabilidad del análisis), F-01, F-06.

---

## P05 — Balances de energía

**Objetivo.** Validar los balances con que el OR cobra energía **no cobrada en meses anteriores**
—típicamente por haberse reportado de menos a XM—, cruzarlos contra las **provisiones**
constituidas y liberar o ajustar la posición contable.

**Disparador.** Recepción de balances, última semana del mes. **Corresponden a meses ya
facturados.**

**Entradas.** Balance del OR · Provisiones vigentes del período al que corresponde · Tarifas
aplicadas en la facturación de ese período.
**Salidas.** Aprobación (o rechazo) del balance para relacionarlo en el mes siguiente · cruce
contra provisión · reporte a Contabilidad.

### Narrativa

1. 🟩/🟥 **Recepción y cargue** del balance (tipo de fuente `BALANCE` en el módulo).
2. 🟨 **Verificación de la energía cobrada**: que la energía del balance corresponda a la
   diferencia efectivamente dejada de reportar en el mes de origen, no a un valor mayor
   (**C-25**).
3. 🟨 **Verificación de la tarifa aplicada**, que debe ser la del **período de origen** y no la
   del mes en curso (**C-26**).
4. 🟨 **Cruce contra la provisión constituida** para esa frontera y período (**C-27**). El
   resultado puede ser:
   - **Cruce total** — el balance consume la provisión: se libera.
   - **Cruce parcial** — queda saldo de provisión pendiente.
   - **Sin provisión previa** — el OR cobra energía que BIA no había provisionado: **es una
     pérdida no anticipada y debe investigarse**, no aprobarse por defecto.
5. 🟨 **Aprobación (visto bueno) para que el OR lo relacione** en la factura del mes siguiente.
6. 🟨 **Reporte a Contabilidad** del efecto del cruce, dentro del reporte de cierre (P07).

### Estado y brecha

El modelo de datos **ya contempla** el ciclo completo —provisiones con estado
`PENDIENTE / CRUZADO_PARCIAL / CRUZADO_TOTAL` y una tabla de cruces con energía cruzada, valor y
tipo de resultado (ingreso / costo / exacto)—, **pero la operación se hace en Excel**. Mientras
el cruce no se registre en el sistema, no hay control automático que impida:
aprobar un balance **sin provisión que lo respalde**, **cruzar dos veces** la misma provisión, o
**dejar provisiones vivas indefinidamente** sin que nadie las revise. Ver brecha **B-03**.

### Controles del proceso

| ID | Control | Tipo | Estado |
|---|---|---|---|
| C-25 | Verificación de la energía del balance contra la diferencia de origen | Detectivo · manual | Manual |
| C-26 | Verificación de la tarifa del período de origen | Detectivo · manual | Manual |
| C-27 | Cruce balance ↔ provisión con estado de consumo | Detectivo · automático | **Modelado, no operado** |
| C-28 | Antigüedad de provisiones: revisión de las que superan N meses sin cruce | Detectivo · manual | **No existe** |

### Riesgos asociados

R-14 (pago de energía ya pagada / doble cobro del OR), R-15 (balance con tarifa incorrecta),
R-16 (provisiones eternas que distorsionan el balance), F-05, F-07.

---

## P06 — Compensaciones por calidad del servicio

**Objetivo.** Liquidar la compensación que el OR debe al usuario por fallas en la red,
relacionarla ante el OR, verificar que la reconozca en la factura SDL y **trasladarla al usuario**
—en factura si está activo, o mediante **pago bancario** si se retiró.

**Este es el único proceso del área que termina en una salida de efectivo hacia un tercero.**

**Disparador.** Indicadores mensuales de calidad por frontera contra el indicador anual del OR.

**Entradas.** Indicadores del OR · Fronteras afectadas · Estado del usuario (activo / retirado) ·
Datos bancarios del usuario retirado.
**Salidas.** Liquidación de compensación · relación enviada al OR · compensación reconocida en
factura SDL · reconocimiento al usuario (en factura o por pago) · insumo del reporte TC2 (P08).

### Narrativa

1. 🟥 **Cálculo del indicador por frontera y mes** y comparación contra el indicador anual del OR.
   Si el del mes lo supera, la frontera **genera compensación**.
2. 🟥 **BIA liquida la compensación.** El comercializador es quien la calcula, no el OR.
3. 🟥 **Se relaciona al OR** para su validación.
4. 🟥 **El OR valida y la relaciona en la factura SDL.**
5. 🟥 **Verificación de que la compensación quedó efectivamente reflejada en la factura SDL**
   (**C-29**). Este paso es una **condición previa** al traslado: BIA no compensa al usuario
   antes de haberla recibido.
6. 🟥 **Traslado al usuario**, por una de dos vías:
   - **Usuario activo** → se reconoce **en su factura**.
   - **Usuario retirado** → **pago a la cuenta bancaria indicada** (**C-30**, **C-31**).
7. 🟥 **Registro para el reporte TC2** (P08): compensaciones reconocidas en factura y
   compensaciones reconocidas por el OR **pendientes de trasladar**.

### Estado y brecha

> **P06 no está sistematizado en absoluto.** En el módulo el KPI de compensaciones es un
> marcador vacío con la lógica pendiente de definir. Todo —cálculo, relación al OR, verificación
> en factura, traslado y pago— ocurre fuera del sistema.
>
> Es, además, **el proceso con mayor riesgo de fraude del área** (ver [05](./05-riesgos-de-fraude.md)
> **F-04**): combina un cálculo hecho por BIA, un beneficiario que ya no es cliente, una cuenta
> bancaria informada por un canal no verificado y un pago, todo bajo el control de la misma
> persona. Ver brechas **B-06** y **B-07**.

### Controles del proceso

| ID | Control | Tipo | Estado |
|---|---|---|---|
| C-29 | No trasladar al usuario antes de verla reflejada en la factura SDL | Preventivo · manual | Implementado como práctica, **sin evidencia estructurada** |
| C-30 | Validación de titularidad de la cuenta bancaria del usuario retirado | Preventivo · manual | **No formalizado** |
| C-31 | Doble aprobación del pago a usuario retirado (liquida ≠ aprueba ≠ paga) | Preventivo · manual | **No formalizado** |
| C-32 | Conciliación periódica: compensaciones reconocidas por el OR vs. trasladadas | Detectivo · manual | Parcial — es lo que persigue el TC2 (P08) |

### Riesgos asociados

R-17 (compensación reconocida por el OR que nunca llega al usuario), R-18 (doble compensación:
en factura y por banco), R-19 (compensación mal liquidada), **F-04 (desvío del pago),
F-06 (compensación ficticia)**.

---

## P07 — Cierre de mes: pérdidas y provisiones a Contabilidad

**Objetivo.** Reportar a Contabilidad, **el día 7 de cada mes**, la posición de **pérdidas** y
**provisiones** del mes de cierre.

> **Ejemplo canónico:** el **7 de agosto** se envía el reporte del **cierre de julio**, que
> corresponde a **consumos de junio**. La pérdida sale de la energía reportada de más a XM en
> junio, contrastada con lo que el OR cobra en julio por consumos de junio.

**Disparador.** Calendario contable. **Plazo estricto: día 7.**

**Entradas.** Resultados de conciliación SDL del período · G de bolsa, transmisión y restricciones
del mes · tarifas de distribución y pérdidas **por usuario** · provisión del mes anterior ·
balances recibidos en el mes.
**Salidas.** Reporte de pérdidas y provisiones enviado a Contabilidad.

### Narrativa

**Pérdidas**

1. 🟩 Se toma la **energía de diferencia** por mayor reporte a XM (`E_xm > E_fac`) del período.
2. 🟩 Se multiplica por `G de bolsa + transmisión + distribución + pérdidas + restricciones`.
   **G de bolsa, transmisión y restricciones son iguales para todas las fronteras del mes;
   distribución y pérdidas son tarifas por usuario** (**C-33**).
3. 🟩 El valor resultante se reporta a Contabilidad como pérdida del mes.

**Provisiones**

4. 🟨 Se parte de la **provisión anterior**.
5. 🟨 Se restan los **balances recibidos en el mes** (P05), que liberan provisión.
6. 🟩 Se suma la **nueva provisión** por las diferencias del mes: energía de diferencia ×
   `(generación + transmisión + distribución + pérdidas + restricciones)`, con las **tarifas
   aplicadas en la facturación al usuario**.
7. 🟨 Se arma el movimiento del período —saldo inicial, liberaciones, constituciones, saldo
   final— y se envía a Contabilidad (**C-34**).

**Antes de enviar**

8. 🟩 **Verificar que la conciliación se ejecutó con la G de bolsa del mes.** Si se corrió antes
   de que Metabase la publicara, hay que **re-ejecutar** ("Recalcular pérdidas") o el valor
   reportado será el del fallback (**C-20**).
9. 🟨 **Aprobación del Líder de Liquidaciones antes del envío** (**C-35**).
10. 🟨 **Congelar el período.** Una vez enviado, cualquier re-ejecución que cambie las cifras
    obliga a reemitir el reporte (política P-06, **C-36**).

### Estado y brecha

El cálculo lo produce el motor, pero **el armado del reporte y el movimiento de provisiones se
hacen en Excel**. Como el módulo **no bloquea la re-ejecución** de una conciliación ya reportada
y **no versiona** el resultado, hoy no es posible demostrar, después del hecho, que el número
reportado el día 7 es el que el sistema tenía ese día. Ver brecha **B-04**.

### Controles del proceso

| ID | Control | Tipo | Estado |
|---|---|---|---|
| C-33 | Valorización con la estructura tarifaria correcta (G según pérdida/provisión) | Preventivo · automático | Implementado |
| C-34 | Movimiento de provisiones: saldo inicial + constituciones − liberaciones = saldo final | Detectivo · manual | Manual en Excel |
| C-35 | Aprobación del Líder antes del envío a Contabilidad | Preventivo · manual | **No formalizado** |
| C-36 | Congelamiento del período reportado y versionado del resultado | Preventivo · automático | **No existe** |
| C-37 | Conciliación de retorno: lo reportado vs. lo registrado por Contabilidad | Detectivo · manual | **No formalizado** |

### Riesgos asociados

R-20 (subestimación o sobreestimación de pérdidas y provisiones en estados financieros),
R-21 (cifra reportada no reproducible), R-08, **F-05 (manipulación de la cifra reportada)**.

---

## P08 — Reporte TC2 a Contabilidad

**Objetivo.** Reportar, **antes del día 15 de cada mes**, las **compensaciones reconocidas en
factura** y las que **el OR ya reconoció y están pendientes de trasladar al usuario**.

**Alcance explícito:** **solo usuarios activos**. Los retirados no se reportan en el TC2 (su
compensación se paga por banco, P06).

**Entradas.** Compensaciones reconocidas por el OR (P06) · Compensaciones aplicadas en factura ·
Estado activo/retirado del usuario.
**Salidas.** Reporte TC2 enviado a Contabilidad.

### Narrativa

1. 🟥 Se toman las compensaciones **reconocidas por el OR** en el período.
2. 🟥 Se separan **activos** de **retirados**; los retirados quedan **fuera** del reporte.
3. 🟥 Se clasifican los activos en **reconocidas en factura** y **pendientes de trasladar**
   (**C-32**).
4. 🟥 Se envía a Contabilidad antes del día 15.

### Estado y brecha

Proceso íntegramente manual. Su valor de control es alto: **es la única conciliación existente
entre lo que el OR reconoció y lo que efectivamente llegó al usuario**. Al ser manual y no
alcanzar a los usuarios retirados, **el tramo de mayor riesgo del proceso de compensaciones queda
fuera de todo reporte** (ver F-04). Ver brecha **B-07**.

### Riesgos asociados

R-17, R-22 (pasivo con el usuario no registrado), **F-04**, **F-06**.

---

## P09 — Cierre post-facturación (indicadores)

**Objetivo.** Medir las desviaciones entre lo facturado y lo reportado a XM, publicar los
indicadores del área e identificar las fronteras de mayor impacto.

**Disparador.** Cierre de la facturación del período.

### Narrativa

1. 🟩 Ejecutada la conciliación del período, el módulo calcula los indicadores del dashboard
   (**C-38**):

   | Indicador | Fórmula | Meta |
   |---|---|---|
   | **% Congruencia** | fronteras congruentes entre las 3 fuentes / total | **> 95 %** |
   | **% Diferencia por reporte de más a XM** | kWh de pérdida / kWh facturados | **< 0,15 %** |
   | **% Diferencia por reporte de menos a XM** | kWh de provisión / kWh facturados | **< 0,20 %** |
   | **% Diferencia absoluta en kWh** | (kWh pérdida + kWh provisión) / kWh facturados | **< 0,35 %** |
   | **% Pérdida** | valor de pérdida / valor facturado | **< 0,10 %** |

2. 🟩 Cada indicador se muestra contra su meta con marca de **EN META / FUERA DE META**.
3. 🟩 Se identifican las **fronteras de mayor impacto** (top 10 por pérdida + provisión).
4. 🟨 Se analizan las causas de las fronteras críticas y se abren acciones con Facturación,
   Operaciones o el OR según corresponda (**C-39**).
5. 🟨 Se presentan los indicadores a Finance.

### Controles del proceso

| ID | Control | Tipo | Estado |
|---|---|---|---|
| C-38 | Cálculo automático de los cinco indicadores contra meta | Detectivo · automático | Implementado |
| C-39 | Análisis de causa de las fronteras de mayor impacto | Detectivo · manual | Implementado, sin registro estructurado |
| C-40 | Análisis de tendencia (histórico 12 meses) | Detectivo · automático | Implementado |

### Riesgos asociados

R-23 (deterioro sostenido no detectado a tiempo), R-24 (indicador que mejora por omisión de carga
y no por gestión — ver **F-10**).

---

## P10 — Cargos STR y orden de compra NetSuite

**Objetivo.** Consolidar los cargos del Sistema de Transmisión Regional por operador y período y
**emitir la orden de compra** que soporta el pago al OR.

> Este proceso no figuraba en la descripción funcional inicial del área, pero **se ejecuta desde
> el módulo y termina en un compromiso de pago a un tercero**. Por su naturaleza —dinero que sale
> hacia un proveedor— se documenta con el mismo rigor que el resto.

**Entradas.** Insumos STR del período (`BalanceSTRTipoFactu<AAAA>-<MES>`) · catálogo de operadores
con su `netsuite_vendor_id`.
**Salidas.** Matriz de cargos STR por operador y mes de facturación · órdenes de compra en
NetSuite · trazabilidad del envío.

### Narrativa

1. 🟩 **Cargue de insumos STR** según P01, con validación de período contra el nombre del archivo
   (**C-01**). Los archivos `TipoReFactu` —ajustes de meses anteriores— **no** se validan contra
   el período: por definición corresponden a otros meses.
2. 🟩 **Revisión de la matriz de cargos** por operador y **mes de facturación** (consumo + 1).
3. 🟩 **Creación del lote de envío** (hasta **25 cargos** por lote). El sistema permite **un solo
   lote en curso a la vez**, garantizado con bloqueo a nivel de base de datos (**C-41**). Al
   crearlo se valida que haya datos, que el monto no sea cero y que el cargo **no haya sido
   procesado antes** (**C-42**).
4. 🟩 **Congelamiento del monto.** Cada envío guarda el **monto en el momento de crear el lote**;
   lo que se manda a NetSuite es ese valor, no un recálculo posterior (**C-43**).
5. 🟩 **Procesamiento.** El sistema crea la orden de compra en NetSuite por cada envío
   —proveedor según el `netsuite_vendor_id` del operador, departamento 131— con **clave de
   idempotencia** que evita duplicar la OC ante un reintento (**C-44**). Los estados van
   `PENDIENTE → PROCESANDO → PROCESADO | ERROR`.
6. 🟩 **Gestión de fallas.** Solo los envíos en `ERROR` pueden **reenviarse**, y solo mientras el
   lote esté en curso. Un lote puede **cancelarse** si ningún envío está en proceso. Un cron
   limpia lotes colgados (**C-45**).
7. 🟩 **Trazabilidad.** Cada envío guarda el número de OC de NetSuite, su identificador interno,
   los intentos, el error si lo hubo, y quién inició el lote (**C-46**).

### Controles del proceso

| ID | Control | Tipo | Estado |
|---|---|---|---|
| C-41 | Un único lote en curso, con bloqueo a nivel de base de datos | Preventivo · automático | Implementado |
| C-42 | Validación de datos, monto distinto de cero y cargo no procesado previamente | Preventivo · automático | Implementado |
| C-43 | Congelamiento del monto al crear el lote | Preventivo · automático | Implementado |
| C-44 | Clave de idempotencia por envío (evita OC duplicada) | Preventivo · automático | Implementado |
| C-45 | Reenvío solo desde estado `ERROR`; cancelación controlada; cron de lotes colgados | Preventivo · automático | Implementado |
| C-46 | Trazabilidad completa del envío (OC, intentos, usuario, error) | Detectivo · automático | Implementado |
| C-47 | **Autorización dual para modificar el `netsuite_vendor_id` de un operador** | Preventivo · manual | **No existe** — hoy basta el perfil ADMINISTRADOR |
| C-48 | Conciliación de OC emitidas contra la matriz de cargos del período | Detectivo · manual | **No formalizado** |

### Riesgos asociados

R-25 (OC duplicada o por monto errado), R-26 (pago a un proveedor equivocado),
**F-03 (redirección del pago cambiando el proveedor de destino)**.

> **Estado del proceso.** El circuito de NetSuite vive únicamente en el backend TypeScript, que
> **no está desplegado en desarrollo**, y sus endpoints esperan identificadores de Supabase que
> la pantalla nueva de STR ya no maneja. Es decir: **el flujo de STR quedó migrado a Go, pero el
> de la orden de compra no**, y hoy no son compatibles. Ver brecha **B-08**.
