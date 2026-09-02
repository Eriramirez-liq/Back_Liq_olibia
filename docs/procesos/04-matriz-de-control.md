# 04 — Matriz de Riesgos y Controles

Área de Liquidaciones · Versión 1.0 — 2026-08-31

Este documento identifica los riesgos de los procesos descritos en las
[narrativas](./02-narrativas-procesos.md), los controles que los mitigan y el estado real de cada
control. Los riesgos de fraude tienen tratamiento propio en [05](./05-riesgos-de-fraude.md).

---

## 1. Metodología

**Escala de probabilidad**

| Nivel | Criterio |
|---|---|
| **Alta (3)** | Ocurre o casi ocurre todos los meses; ya ocurrió más de una vez en los últimos 12 meses |
| **Media (2)** | Ocurrencia esperable algunas veces al año |
| **Baja (1)** | Requiere una conjunción poco frecuente de condiciones |

**Escala de impacto** — se mide contra las metas del área y el efecto en estados financieros.

| Nivel | Criterio |
|---|---|
| **Alto (3)** | Afecta la cifra reportada a Contabilidad, produce pago o cobro indebido, o rompe una meta del área de forma sostenida |
| **Medio (2)** | Genera retrabajo significativo, reclamo perdido por vencimiento de plazo o desviación puntual de una meta |
| **Bajo (1)** | Corregible dentro del mismo ciclo sin efecto económico |

**Nivel de riesgo** = probabilidad × impacto → **Crítico (9)** · **Alto (6)** · **Medio (3-4)** ·
**Bajo (1-2)**.

**Clasificación del control**

- **Objetivo:** *Preventivo* (evita que ocurra) · *Detectivo* (lo descubre después) ·
  *Correctivo* (repara).
- **Naturaleza:** *Automático* (lo ejecuta el sistema) · *Manual* · *Híbrido*.
- **Aserción financiera cubierta:** *Integridad* (nada falta) · *Exactitud* (los valores son
  correctos) · *Existencia* (lo registrado ocurrió) · *Corte* (está en el período correcto) ·
  *Valuación* (está bien medido) · *Autorización*.

---

## 2. Registro de riesgos operativos

| ID | Proceso | Riesgo | Causa raíz | Efecto | Prob. | Imp. | Inherente | Controles | Residual |
|---|---|---|---|---|---|---|---|---|---|
| **R-01** | P01 | Cargar una fuente bajo un **período de consumo equivocado** | El período se elige a mano; los archivos de un OR pueden ir dos meses atrás por diseño | Cifras de un mes guardadas bajo otro; el error solo aparece al conciliar y **el modelo append-only no permite deshacerlo desde la pantalla** | Alta | Alto | **Crítico** | C-01, C-02, C-04 | **Medio** — el control cubre STR y Tarifas SDL, **no** TC1, SDL, XM, Facturación ni Balance |
| **R-02** | Transversal | **Incumplimiento de plazos** por concentración del calendario | Respuesta al SDL en 1 día calendario, solapada con día 7, día 12 y día 15 | Reclamos perdidos, controles omitidos por presión de tiempo | Alta | Alto | **Crítico** | C-15, C-22 | **Alto** — no hay medición formal de cumplimiento |
| **R-03** | P01 | Conciliar con una **fuente incompleta o faltante** | Un OR no envía; se concilia con lo que hay | Fronteras marcadas `INCOMPLETA` que **no generan pérdida ni provisión** y mejoran artificialmente los indicadores | Alta | Alto | **Crítico** | C-05, C-10 | **Medio** |
| **R-04** | P01 | Cargar un **archivo alterado o distinto del enviado por el OR** | Los archivos llegan por correo y se cargan desde el escritorio; el sistema guarda las filas, **no el archivo original ni su huella** | Conciliación sobre datos no auténticos, imposible de reconstruir después | Media | Alto | **Alto** | — | **Alto** — sin control (brecha B-10) |
| **R-05** | P02 | **Nivel de tensión o propiedad de activos errados** en facturación | Diferencia entre TC1 y la configuración de facturación | Tarifa incorrecta al usuario, **con efecto acumulativo mes a mes** hasta su detección | Alta | Alto | **Crítico** | C-11, C-12, C-13, C-14, C-16 | **Bajo** |
| **R-06** | P02 | Vencimiento del **plazo del día 12** con el OR | Volumen de 21 archivos y solapamiento con SDL | Se pierde la ventana de objeción del período | Media | Medio | **Medio** | C-15 | **Medio** |
| **R-07** | P03 | **Aceptar un cobro del OR superior al que corresponde** | La disputa se detecta pero se cierra sin gestión | Sobrecosto de uso de red, no recuperable | Media | Alto | **Alto** | C-18, C-14, C-21, C-22 | **Medio** — depende de un accionable sin revisión independiente |
| **R-08** | P03 / P07 | **Valorización errada de la pérdida** por G de bolsa | Si Metabase no responde, el motor cae a un valor alterno **sin avisar**; el resultado se persiste | Pérdida reportada a Contabilidad con un valor distinto del correcto | Media | Alto | **Alto** | C-20 | **Medio** — exige recordar re-ejecutar antes del día 7 |
| **R-09** | P03 | **Diferencia detectada y no reclamada** dentro del plazo | Ventana de 1 día; sin trazabilidad del reclamo en el sistema | Pérdida de un derecho de cobro contra el OR | Alta | Medio | **Alto** | C-22, C-14 | **Alto** — la evidencia vive solo en el correo |
| **R-10** | P03 / P05 | **Provisiones sin seguimiento** que nunca se liberan ni se depuran | El cruce con balances se hace en Excel | Pasivo sobrestimado; distorsión del resultado del área | Alta | Alto | **Crítico** | C-27, C-28 | **Alto** — C-27 está modelado pero no se opera |
| **R-11** | P04 | El OR **cobra COT por usuarios que no aplican** | Validación 100 % manual, sin universo ni motor | Costo indebido asumido por BIA | Media | Alto | **Alto** | C-23, C-24 | **Alto** |
| **R-12** | P04 | **BIA no traslada al usuario** un COT que sí le cobran | Misma causa | Fuga de margen directa | Media | Alto | **Alto** | C-24 | **Alto** |
| **R-13** | P04 | **Falta de trazabilidad** del análisis COT | Todo en Excel local | Imposible auditar o reconstruir la decisión | Alta | Medio | **Alto** | — | **Alto** (brecha B-02) |
| **R-14** | P05 | **Pagar energía ya pagada** (doble cobro del OR vía balance) | El cruce contra la provisión no está sistematizado | Salida de caja indebida | Media | Alto | **Alto** | C-25, C-27 | **Alto** |
| **R-15** | P05 | Balance liquidado con **tarifa del mes en curso** y no la del período de origen | Verificación manual | Cobro superior al que corresponde | Media | Medio | **Medio** | C-26 | **Medio** |
| **R-16** | P05 | **Provisiones eternas** por falta de análisis de antigüedad | No existe reporte de antigüedad | Estados financieros distorsionados | Alta | Medio | **Alto** | C-28 | **Alto** — el control no existe |
| **R-17** | P06 / P08 | Compensación **reconocida por el OR que nunca llega al usuario** | Traslado manual, sin control sistematizado | Enriquecimiento indebido de BIA, riesgo regulatorio y reputacional | Media | Alto | **Alto** | C-29, C-32 | **Alto** |
| **R-18** | P06 | **Doble compensación**: en factura y por pago bancario | Sin registro único de compensaciones | Doble salida de recursos | Baja | Alto | **Medio** | C-32 | **Alto** — sin registro central no hay forma de detectarlo |
| **R-19** | P06 | **Compensación mal liquidada** (indicador o cálculo errado) | Cálculo manual del comercializador | Reclamo del usuario o pérdida | Media | Medio | **Medio** | C-29 | **Medio** |
| **R-20** | P07 | **Pérdidas o provisiones mal reportadas** a Contabilidad | Armado manual del reporte sobre cifras del sistema | Error en estados financieros | Media | Alto | **Alto** | C-33, C-34, C-35, C-37 | **Alto** — C-35 y C-37 no formalizados |
| **R-21** | P07 | La **cifra reportada no es reproducible** después | El módulo permite re-ejecutar la conciliación y **no versiona** el resultado | Imposible sustentar ante auditoría lo enviado el día 7 | Alta | Alto | **Crítico** | C-36 | **Alto** — el control no existe |
| **R-22** | P08 | **Pasivo con el usuario no registrado** (compensaciones pendientes de trasladar) | Reporte manual y limitado a activos | Subregistro de una obligación | Media | Medio | **Medio** | C-32 | **Medio** |
| **R-23** | P09 | **Deterioro sostenido de indicadores** no detectado a tiempo | Análisis mensual sin registro de causas | Fuga de margen recurrente | Media | Medio | **Medio** | C-38, C-39, C-40 | **Bajo** |
| **R-24** | P09 | **Indicador que mejora por omisión de carga**, no por gestión | Las fronteras `INCOMPLETA` no generan pérdida | Falsa señal de control a Finance | Media | Alto | **Alto** | C-05, C-10 | **Medio** — requiere leer completitud junto al indicador |
| **R-25** | P10 | **Orden de compra duplicada o por monto errado** | Reintentos de envío, recálculos posteriores | Pago duplicado o incorrecto al OR | Baja | Alto | **Medio** | C-41 a C-46 | **Bajo** — es el proceso mejor controlado del área |
| **R-26** | P10 | **Pago dirigido al proveedor equivocado** | El identificador del proveedor en NetSuite es editable desde Administración | Salida de caja hacia un tercero | Baja | Alto | **Medio** | C-47 | **Alto** — el control no existe (ver F-03) |
| **R-27** | Transversal | **Dos copias del mismo dato** durante la migración a Go | TC1 se carga en las bases nuevas pero Conciliaciones, Congruencia y Gestiones leen la copia vieja | Conciliar contra datos desactualizados sin que nadie lo note | Alta | Alto | **Crítico** | C-05 | **Alto** (brecha B-01) |
| **R-28** | Transversal | **Acceso indebido al módulo** | Alta automática como `ANALISTA` para cualquier correo del dominio | Personas ajenas al área con capacidad de cargar y gestionar | Media | Medio | **Medio** | C-07 | **Alto** (ver F-08) |
| **R-29** | Transversal | **Registro de auditoría incompleto** | El log de auditoría vive en el backend TypeScript; **los flujos migrados a Go no escriben en él** | Acciones sin rastro de autor | Alta | Medio | **Alto** | C-06 | **Alto** (brecha B-11) |

### Mapa de calor (riesgo inherente)

| | **Impacto Bajo** | **Impacto Medio** | **Impacto Alto** |
|---|---|---|---|
| **Prob. Alta** | — | R-06*, R-13, R-16, R-29 | **R-01, R-02, R-03, R-05, R-09, R-10, R-21, R-27** |
| **Prob. Media** | — | R-15, R-19, R-22, R-23, R-28 | R-04, R-07, R-08, R-11, R-12, R-14, R-17, R-20, R-24 |
| **Prob. Baja** | — | — | R-18, R-25, R-26 |

\* R-06 tiene probabilidad media; se ubica por impacto.

**Los ocho riesgos críticos** concentran tres causas: *un calendario que no deja espacio para
controlar*, *procesos que viven fuera del sistema* y *la ausencia de un cierre formal del
período*.

---

## 3. Matriz de control

| ID | Proceso | Control | Objetivo | Naturaleza | Frecuencia | Responsable | Evidencia | Aserción | Riesgo | Estado |
|---|---|---|---|---|---|---|---|---|---|---|
| **C-01** | P01 | El período declarado se contrasta contra el que traen los nombres de archivo; discrepancia = error crítico que bloquea el confirmar | Preventivo | Automático | Por carga | Sistema | Mensaje de error en la vista previa | Corte | R-01 | 🟡 **Parcial** — solo STR y Tarifas SDL |
| **C-02** | P01 | Se rechaza cargar el mes en curso y meses futuros | Preventivo | Automático | Por carga | Sistema | Rechazo con mensaje | Corte | R-01 | 🟢 Implementado |
| **C-03** | P01 | Reemplazar una carga exige justificación escrita | Preventivo | Automático | Por reemplazo | Analista | Campo de justificación en la carga | Autorización | R-01, F-03 | 🟡 **Parcial** — texto libre, **sin aprobación de un segundo** |
| **C-04** | P01 | Escritura append-only con `load_id`, usuario y fecha; nada se borra | Detectivo | Automático | Por carga | Sistema | Historial de cargas | Existencia, integridad | R-01, F-02 | 🟢 Implementado |
| **C-05** | P01 | Panel de estado del período con avance `n/21` por fuente y operador, y lista de faltantes | Detectivo | Automático | Continuo | Analista | Pantalla Estado del período | Integridad | R-03, R-24 | 🟢 Implementado |
| **C-06** | Transversal | Log de auditoría por acción, usuario e IP | Detectivo | Automático | Continuo | Administrador | Tabla `log_auditoria` | Existencia, autorización | R-29, F-01 | 🟡 **Parcial** — los flujos migrados a Go no escriben en él |
| **C-07** | Transversal | Autenticación federada con verificación de token, restricción de dominio y perfiles | Preventivo | Automático | Continuo | TI | Configuración y sesión | Autorización | R-28 | 🟡 **Parcial** — alta automática como `ANALISTA` |
| **C-08** | Transversal | Cierre formal del período con fecha y responsable | Preventivo | Automático | Mensual | Líder | Estado del período | Corte | R-21 | 🔴 **Modelado, no operado** |
| **C-09** | P01 | Vista previa obligatoria antes de confirmar una carga | Preventivo | Automático | Por carga | Analista | Pantalla de vista previa | Exactitud | R-01, R-04 | 🟢 Implementado |
| **C-10** | P02/P03 | Universo de conciliación = **unión** de fuentes; las fronteras de un solo lado se reportan `INCOMPLETA` con el motivo | Detectivo | Automático | Por conciliación | Sistema | Detalle de conciliación | Integridad | R-03, R-24 | 🟢 Implementado |
| **C-11** | P02 | Parser TC1 con mapeo posicional y sobrescritura por nombre, calibrado por operador | Preventivo | Automático | Por carga | Sistema | Vista previa y conteo de filas | Exactitud | R-05 | 🟢 Implementado — verificado contra los 21 archivos reales |
| **C-12** | P02 | Validación de período con forma, operador presente y fronteras ni vacías ni duplicadas | Preventivo | Automático | Por carga | Sistema | Errores de validación | Integridad | R-05 | 🟢 Implementado |
| **C-13** | P02 | Conciliación automática de nivel de tensión y propiedad por frontera | Detectivo | Automático | Mensual | Sistema | Resultados TC1 | Exactitud | R-05 | 🟢 Implementado |
| **C-14** | P02/P03 | Accionable obligatorio por frontera con diferencia, con tipo y observación | Detectivo | Manual | Por diferencia | Analista | Tabla de gestiones | Existencia | R-05, R-07, R-09 | 🟡 **Parcial** — **sin revisión independiente** (ver F-01) |
| **C-15** | P02 | Revisión y respuesta del TC1 antes del día 12 | Preventivo | Manual | Mensual | Analista | Correo al OR | Corte | R-06 | 🔴 **Sin medición formal** |
| **C-16** | P02/P09 | Congruencia entre las tres fuentes, que identifica **cuál** difiere; meta > 95 % | Detectivo | Automático | Mensual | Analista | Panel de congruencia | Exactitud | R-05 | 🟢 Implementado |
| **C-17** | P03 | Descarga de Facturación y XM el día 8, **antes** de recibir las preliquidaciones | Preventivo | Manual | Mensual | Analista | Cargas del período | Corte, integridad | R-07 | 🟢 Implementado |
| **C-18** | P03 | Motor de clasificación A1–D4 con umbral de 100 kWh y valorización por caso | Detectivo | Automático | Mensual | Sistema | Resultados de conciliación | Exactitud, valuación | R-07 | 🟢 Implementado — con casos de prueba documentados |
| **C-19** | P02/P03 | Colapso de fronteras `_N`: agrupación por clave base, suma de energías, herencia de atributos y deduplicación | Preventivo | Automático | Por conciliación | Sistema | Detalle por frontera | Exactitud | R-07 | 🟢 Implementado |
| **C-20** | P03/P07 | G de bolsa del mes desde Metabase, aplicada a todas las fronteras; re-ejecución obligatoria si se corrió sin ella | Preventivo | Híbrido | Mensual | Analista | Valor de G en el dashboard | Valuación | R-08 | 🟡 **Parcial** — **el fallback es silencioso** |
| **C-21** | P03 | Alertas manuales obligatorias en los casos D1, D2 y D4 | Detectivo | Manual | Por frontera | Analista | Marca en el resultado | Exactitud | R-07 | 🟡 **Parcial** — marca sin bloqueo de cierre |
| **C-22** | P03 | Respuesta y reclamo al OR dentro de un día calendario | Preventivo | Manual | Por OR | Analista | Correo al OR | Existencia | R-09 | 🔴 **Sin medición ni evidencia en el sistema** |
| **C-23** | P04 | Verificación de energía, tarifa y aplicabilidad del COT | Detectivo | Manual | Mensual | Analista | Excel del área | Exactitud | R-11 | 🔴 **Manual, sin evidencia estructurada** |
| **C-24** | P04 | Contraste de lo cobrado por el OR contra lo cobrado al usuario | Detectivo | Manual | Mensual | Analista | Excel del área | Integridad | R-12 | 🔴 **Manual** |
| **C-25** | P05 | Verificación de la energía del balance contra la diferencia de origen | Detectivo | Manual | Por balance | Analista | Excel del área | Exactitud | R-14 | 🔴 Manual |
| **C-26** | P05 | Verificación de que la tarifa aplicada sea la del período de origen | Detectivo | Manual | Por balance | Analista | Excel del área | Valuación | R-15 | 🔴 Manual |
| **C-27** | P05 | Cruce balance ↔ provisión con estado `PENDIENTE / CRUZADO_PARCIAL / CRUZADO_TOTAL` | Detectivo | Automático | Por balance | Sistema | Tabla de cruces | Integridad, valuación | R-10, R-14 | 🔴 **Modelado, no operado** |
| **C-28** | P05 | Reporte de antigüedad de provisiones sin cruce | Detectivo | Automático | Mensual | Líder | Reporte de antigüedad | Valuación | R-16 | 🔴 **No existe** |
| **C-29** | P06 | No trasladar la compensación al usuario antes de verla reflejada en la factura SDL | Preventivo | Manual | Por compensación | Analista | Factura SDL del OR | Existencia | R-17, F-06 | 🟡 Práctica establecida, **sin evidencia estructurada** |
| **C-30** | P06 | Validación de titularidad de la cuenta bancaria del usuario retirado | Preventivo | Manual | Por pago | Tesorería | Certificación bancaria | Autorización | F-04 | 🔴 **No formalizado** |
| **C-31** | P06 | Doble aprobación del pago a usuario retirado; quien liquida no aprueba ni paga | Preventivo | Manual | Por pago | Líder + Tesorería | Aprobación documentada | Autorización | F-04 | 🔴 **No formalizado** |
| **C-32** | P06/P08 | Conciliación entre compensaciones reconocidas por el OR y compensaciones trasladadas al usuario | Detectivo | Manual | Mensual | Analista | Reporte TC2 | Integridad | R-17, R-18, R-22 | 🟡 **Parcial** — **excluye a los usuarios retirados** |
| **C-33** | P07 | Valorización con la estructura tarifaria correcta: G de bolsa para pérdida, G de facturación para provisión; D y PR por usuario | Preventivo | Automático | Mensual | Sistema | Resultados valorizados | Valuación | R-20 | 🟢 Implementado |
| **C-34** | P07 | Movimiento de provisiones: saldo inicial + constituciones − liberaciones = saldo final | Detectivo | Manual | Mensual | Analista | Reporte de cierre | Integridad | R-20 | 🟡 Manual en Excel |
| **C-35** | P07 | Aprobación del Líder antes del envío a Contabilidad | Preventivo | Manual | Mensual | Líder | Correo de aprobación | Autorización | R-20, F-05 | 🔴 **No formalizado** |
| **C-36** | P07 | Congelamiento del período reportado y versionado del resultado de conciliación | Preventivo | Automático | Mensual | Sistema | Estado del período | Corte, existencia | R-21, F-05 | 🔴 **No existe** |
| **C-37** | P07 | Conciliación de retorno: lo reportado por el área vs. lo registrado por Contabilidad | Detectivo | Manual | Mensual | Contabilidad | Cruce documentado | Integridad | R-20 | 🔴 **No formalizado** |
| **C-38** | P09 | Cálculo automático de los cinco indicadores contra su meta, con marca de cumplimiento | Detectivo | Automático | Mensual | Sistema | Dashboard | Exactitud | R-23 | 🟢 Implementado |
| **C-39** | P09 | Análisis de causa de las fronteras de mayor impacto (top 10) | Detectivo | Manual | Mensual | Analista | Análisis del área | — | R-23 | 🟡 Sin registro estructurado |
| **C-40** | P09 | Análisis de tendencia con histórico de 12 meses | Detectivo | Automático | Mensual | Líder | Dashboard histórico | — | R-23 | 🟢 Implementado |
| **C-41** | P10 | Un único lote de envío en curso, garantizado por bloqueo en base de datos | Preventivo | Automático | Por lote | Sistema | Estado del lote | Existencia | R-25 | 🟢 Implementado |
| **C-42** | P10 | Validación de existencia de datos, monto distinto de cero y cargo no procesado previamente | Preventivo | Automático | Por lote | Sistema | Conflictos devueltos | Exactitud, existencia | R-25 | 🟢 Implementado |
| **C-43** | P10 | Congelamiento del monto al crear el lote; se envía el valor congelado | Preventivo | Automático | Por envío | Sistema | Monto del envío | Valuación | R-25, F-05 | 🟢 Implementado |
| **C-44** | P10 | Clave de idempotencia por envío que impide duplicar la orden de compra | Preventivo | Automático | Por envío | Sistema | Registro del envío | Existencia | R-25 | 🟢 Implementado |
| **C-45** | P10 | Reenvío solo desde estado `ERROR` y con lote en curso; cancelación controlada; limpieza automática de lotes colgados | Preventivo | Automático | Por envío | Sistema | Historial de lotes | Existencia | R-25 | 🟢 Implementado |
| **C-46** | P10 | Trazabilidad completa del envío: número de OC, identificador de NetSuite, intentos, error y usuario | Detectivo | Automático | Por envío | Sistema | Detalle del lote | Existencia | R-25 | 🟢 Implementado |
| **C-47** | P10 | Autorización dual para modificar el identificador de proveedor NetSuite de un operador | Preventivo | Manual | Por cambio | Líder + Finance | Aprobación documentada | Autorización | R-26, F-03 | 🔴 **No existe** |
| **C-48** | P10 | Conciliación de órdenes de compra emitidas contra la matriz de cargos del período | Detectivo | Manual | Mensual | Líder | Cruce documentado | Integridad | R-25, F-03 | 🔴 **No formalizado** |

**Resumen del estado**

| Estado | Cantidad | Lectura |
|---|---|---|
| 🟢 Implementado y operando | 20 | El núcleo automático —cargue, motores de conciliación, valorización y NetSuite— está sólido |
| 🟡 Parcial | 12 | Existe el control pero le falta cobertura, evidencia o un segundo par de ojos |
| 🔴 No existe o no se opera | 16 | Concentrados en **COT, compensaciones, balances y cierre del período**: los procesos que viven fuera del sistema |

---

## 4. Controles clave (para pruebas de auditoría)

Si hubiera que probar solo diez, estos son los que sostienen las aserciones principales:

| Control clave | Prueba sugerida | Frecuencia de prueba |
|---|---|---|
| **C-01 / C-02** | Tomar 3 cargas del período y verificar que el nombre del archivo, el período declarado y los datos coinciden | Trimestral |
| **C-04** | Verificar que toda carga vigente tiene usuario, fecha e historial de reemplazos con justificación | Trimestral |
| **C-05** | Confirmar que las 21 fuentes SDL y TC1 estaban completas antes de conciliar el período | Mensual |
| **C-10 / C-18** | Recalcular manualmente 10 fronteras (una por caso A1–D4) y compararlas con el resultado del sistema | Semestral |
| **C-14** | Muestra de 25 fronteras con diferencia: verificar que tienen accionable y que su justificación es consistente | Mensual |
| **C-20** | Verificar que la G de bolsa usada en el período reportado es la publicada por Metabase para ese mes | Mensual |
| **C-27 / C-34** | Recomponer el movimiento de provisiones del trimestre y cuadrarlo contra Contabilidad | Trimestral |
| **C-32** | Trazar 10 compensaciones desde el reconocimiento del OR hasta el usuario final, **incluyendo retirados** | Trimestral |
| **C-43 / C-44** | Cruzar las OC emitidas en NetSuite contra la matriz STR del período: cantidad, montos y proveedor | Mensual |
| **C-47** | Revisar todo cambio de identificador de proveedor NetSuite del período y su autorización | Trimestral |

---

## 5. Indicadores de monitoreo del sistema de control

Complementan los indicadores de negocio de P09 y miden **la salud del control**, no la del
resultado. Ninguno existe hoy: son parte del plan de [06](./06-brechas-y-plan.md).

| Indicador | Definición | Meta propuesta |
|---|---|---|
| **Completitud de fuentes al conciliar** | Fuentes cargadas / esperadas al momento de ejecutar | 100 % |
| **Oportunidad de respuesta al OR** | Reclamos enviados dentro del plazo / total | ≥ 95 % |
| **Cobertura de accionables** | Fronteras con diferencia que tienen accionable / total | 100 % |
| **Tasa de `AJUSTE_NO_PROCEDE`** | Accionables de ese tipo / total, por analista y por OR | Vigilar desviaciones, no fijar meta |
| **Antigüedad de provisiones** | Provisiones sin cruce con más de 3 meses / total | ≤ 10 % |
| **Reemplazos de carga** | Cargas reemplazadas / cargas totales, por período y usuario | Vigilar desviaciones |
| **Compensaciones pendientes de traslado** | Reconocidas por el OR y no trasladadas, por antigüedad | Cero con más de 2 meses |
| **Re-ejecuciones posteriores al reporte** | Conciliaciones re-ejecutadas después del día 7 | Cero sin reemisión del reporte |
