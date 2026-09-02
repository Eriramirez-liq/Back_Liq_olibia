# 05 — Riesgos de Fraude

Área de Liquidaciones · Versión 1.0 — 2026-08-31

> **Alcance y tono.** Este documento no supone ni insinúa conducta indebida de nadie. Identifica
> **dónde el diseño actual del proceso permitiría que un fraude ocurriera y no fuera detectado**,
> que es exactamente lo que una evaluación de riesgo de fraude debe hacer. Se escribe sobre el
> proceso, nunca sobre las personas.

---

## 1. Por qué este proceso concentra riesgo de fraude

El área reúne, en un solo lugar, las cuatro condiciones que la literatura de fraude ocupacional
identifica como habilitantes:

| Condición | Cómo se presenta acá |
|---|---|
| **Discrecionalidad sobre diferencias de dinero** | El analista decide si una diferencia se reclama, se ajusta o "no procede". Esa decisión mueve pesos y hoy **nadie la revisa** |
| **Relación permanente con contrapartes externas** | Interlocución mensual, directa y bilateral con 21-23 operadores de red, cada uno con interés económico opuesto al de BIA |
| **Procesos fuera del sistema** | COT, compensaciones, balances y el reporte de cierre viven en Excel y correo: sin control de acceso, sin log, sin versionado |
| **Presión de calendario e indicadores** | Ventanas de un día calendario, plazos superpuestos y metas explícitas de pérdida y congruencia: la racionalización más común ("lo hice para cumplir") está servida |

Los tres tipos de fraude ocupacional aplican al área:

- **Corrupción / colusión con un tercero** — favorecer al OR en la conciliación (F-01, F-07).
- **Apropiación indebida de activos** — desviar un pago de compensación o una orden de compra
  (F-03, F-04, F-06).
- **Manipulación de reportes financieros** — alterar pérdidas y provisiones reportadas a
  Contabilidad (F-05, F-10).

---

## 2. Matriz de riesgos de fraude

Escalas iguales a las de la [matriz de control](./04-matriz-de-control.md#1-metodología).

### F-01 · Cierre indebido de diferencias en favor del Operador de Red

| | |
|---|---|
| **Esquema** | El analista detecta una diferencia real a favor de BIA —una disputa por sobrecobro del OR, o un `DIFERENCIA` de TC1 que corrige tarifa— y la cierra con el accionable `AJUSTE_NO_PROCEDE`, o cierra la disputa como `CERRADA_SIN_AJUSTE`, a cambio de un beneficio del OR o para evitar la gestión |
| **Cómo se ejecutaría** | En Gestiones, registrando el accionable con una observación genérica. El sistema lo acepta sin más: no exige soporte, no compara contra el valor en riesgo y no escala por monto |
| **Activo expuesto** | El valor de las disputas del período. Cada frontera vale `Δ energía × tarifa SDL`; el esquema es **repetible mes a mes** y no deja faltante de caja que alguien eche de menos |
| **Oportunidad actual** | **Alta.** Un solo perfil `ANALISTA` puede conciliar, decidir y cerrar. No hay aprobación por monto, ni revisión por muestreo, ni reporte de accionables por analista |
| **Banderas rojas** | Concentración de `AJUSTE_NO_PROCEDE` en un mismo OR o un mismo analista · disputas de alto valor cerradas el mismo día en que se abren · observaciones repetidas literalmente · un OR con tasa de reclamo muy inferior al promedio |
| **Controles actuales** | C-14 (accionable obligatorio), C-16 (congruencia identifica cuál fuente difiere), C-06 (log parcial) |
| **Controles a implementar** | **Aprobación del Líder para todo `AJUSTE_NO_PROCEDE` y `CERRADA_SIN_AJUSTE` que supere un umbral en pesos** · reporte mensual de accionables por analista y por OR · exigir soporte documental adjunto · rotación de la asignación de operadores entre analistas |
| **Evaluación** | Probabilidad **Media** · Impacto **Alto** · **Riesgo Alto** |

### F-02 · Sustitución o alteración del archivo fuente antes de cargarlo

| | |
|---|---|
| **Esquema** | Se edita el archivo recibido del OR (preliquidación, TC1, balance) antes de subirlo, para que la conciliación no arroje la diferencia que el archivo original produciría |
| **Cómo se ejecutaría** | Abriendo el XLSX en el escritorio, modificando la energía de unas pocas fronteras y cargándolo. El módulo valida estructura y período, **no autenticidad** |
| **Activo expuesto** | Cualquier concepto conciliado; el efecto se traslada al reporte del día 7 |
| **Oportunidad actual** | **Alta.** El sistema guarda **las filas parseadas, no el archivo original ni su huella digital**. Después del hecho no hay forma de comparar contra lo que el OR envió, salvo volviendo al correo |
| **Banderas rojas** | Diferencias que aparecen y desaparecen entre cargas del mismo período · un OR cuyas cifras cuadran sistemáticamente mejor que las de sus pares · cargas hechas fuera del horario habitual |
| **Controles actuales** | C-04 (append-only con autor y fecha), C-03 (justificación de reemplazo) |
| **Controles a implementar** | **Conservar el archivo original y su hash en el momento de la carga**, y compararlo contra el adjunto del correo del OR · recepción de archivos por un canal controlado (buzón del área, no personal) · verificación por muestreo de 2-3 archivos por período contra el correo original |
| **Evaluación** | Probabilidad **Baja-Media** · Impacto **Alto** · **Riesgo Alto** (por ausencia total de control detectivo) |

### F-03 · Redirección del pago al Operador de Red

| | |
|---|---|
| **Esquema** | Se modifica el **identificador de proveedor de NetSuite** asociado a un operador para que la orden de compra —y con ella el pago— se emita hacia otro tercero |
| **Cómo se ejecutaría** | Desde *Administración › Operadores*, editando el campo del proveedor. Es una operación de un solo paso, disponible para el perfil `ADMINISTRADOR`, **sin doble autorización** |
| **Activo expuesto** | El pago mensual de cargos STR del operador, que en la matriz alcanza órdenes de miles de millones de pesos por período |
| **Oportunidad actual** | **Media-Alta.** El flujo de emisión está bien controlado (lote único, monto congelado, idempotencia, trazabilidad), pero **el dato que decide a quién se le paga no lo está** |
| **Banderas rojas** | Cambio del identificador de proveedor poco antes de crear un lote · cambio sin solicitud formal del OR · orden de compra a un proveedor cuyo nombre no corresponde al operador |
| **Controles actuales** | C-41 a C-46 (control del flujo de emisión), C-06 (log parcial) |
| **Controles a implementar** | **C-47: autorización dual documentada para cambiar el proveedor de un operador**, con notificación automática al Líder y a Finance · **C-48: conciliación mensual de las OC emitidas contra la matriz del período**, verificando operador, monto y proveedor · alerta cuando un lote se crea dentro de los N días siguientes a un cambio de proveedor |
| **Evaluación** | Probabilidad **Baja** · Impacto **Alto** · **Riesgo Alto** (impacto por evento, mitigación débil) |

### F-04 · Desvío del pago de compensaciones a usuarios retirados

| | |
|---|---|
| **Esquema** | La compensación de un usuario **retirado** se paga a una cuenta bancaria distinta de la del titular: la del ejecutor o la de un tercero relacionado |
| **Cómo se ejecutaría** | El proceso es íntegramente manual. Quien liquida la compensación es quien informa la cuenta de destino. El beneficiario **ya no es cliente**: no recibe factura, no tiene canal de atención abierto y **es improbable que reclame** un dinero que probablemente no sabía que le correspondía |
| **Activo expuesto** | Efectivo. Es **el único proceso del área que termina en una transferencia bancaria a una persona natural** |
| **Oportunidad actual** | **Alta.** Sin sistema, sin registro central de compensaciones, sin validación de titularidad, sin segregación entre quien liquida, quien aprueba y quien paga, y —lo más grave— **excluido del reporte TC2**, que solo cubre usuarios activos. El tramo de mayor riesgo del proceso es justamente el que ningún reporte mira |
| **Banderas rojas** | Cuentas bancarias repetidas entre usuarios distintos · cuenta cuyo titular no coincide con el usuario · compensaciones a usuarios retirados hace mucho tiempo · concentración de pagos en un mismo ejecutor · montos justo por debajo de cualquier umbral de aprobación |
| **Controles actuales** | C-29 (no trasladar antes de verla en la factura SDL) — establece que el dinero exista, **no** que llegue a quien corresponde |
| **Controles a implementar** | **C-30: validación de titularidad** con certificación bancaria a nombre del usuario · **C-31: segregación estricta** —liquida el analista, aprueba el Líder, paga Tesorería— · **registro central de compensaciones** que cubra activos y retirados, con estado y soporte del traslado · **extender el TC2 a los retirados** como anexo de control · verificación de que no existan cuentas de destino duplicadas · confirmación posterior con una muestra de beneficiarios |
| **Evaluación** | Probabilidad **Media** · Impacto **Alto** · **RIESGO CRÍTICO — la mayor exposición del área** |

### F-05 · Manipulación de la cifra reportada a Contabilidad

| | |
|---|---|
| **Esquema** | Alterar el valor de pérdidas o provisiones del reporte del día 7: reducir la pérdida para cumplir la meta del indicador, o inflar/desinflar la provisión para trasladar resultado entre períodos |
| **Cómo se ejecutaría** | Por tres vías, todas abiertas hoy: (a) armar el reporte en Excel con cifras distintas de las del sistema; (b) **re-ejecutar la conciliación después de haber reportado**, con lo que el sistema deja de coincidir con lo enviado y nadie puede probar cuál era el número original; (c) omitir la re-ejecución con la G de bolsa correcta, sabiendo que el fallback produce un valor menor |
| **Activo expuesto** | La razonabilidad de los estados financieros. También la medición del desempeño del área, si está ligada a incentivos |
| **Oportunidad actual** | **Alta.** El módulo **no versiona** el resultado de una conciliación, **no bloquea** la re-ejecución de un período ya reportado, y el reporte se arma y se envía **sin aprobación formal** de un segundo |
| **Banderas rojas** | Re-ejecuciones de conciliación posteriores al día 7 · diferencias entre el reporte enviado y la consulta al sistema hecha después · indicadores que quedan justo por debajo de la meta mes tras mes · provisiones que se constituyen y liberan sin balance que las respalde |
| **Controles actuales** | C-33 (valorización automática), C-43 (congelamiento de monto, en STR) |
| **Controles a implementar** | **C-36: congelar el período al reportar y versionar el resultado**, de modo que el reporte sea siempre reproducible · **C-35: aprobación documentada del Líder** antes del envío · **C-37: conciliación de retorno con Contabilidad** · alerta automática ante cualquier re-ejecución de un período cerrado |
| **Evaluación** | Probabilidad **Media** · Impacto **Alto** · **Riesgo Alto** |

### F-06 · Compensación ficticia

| | |
|---|---|
| **Esquema** | Liquidar una compensación por una frontera o un usuario que no la generó, o duplicar una legítima, y cobrarla por la vía bancaria |
| **Cómo se ejecutaría** | El cálculo del indicador y la liquidación los hace BIA, en Excel. Si el OR valida por lote y sin verificación fina, una compensación adicional puede pasar |
| **Activo expuesto** | Efectivo, y la relación con el OR |
| **Oportunidad actual** | **Media-Alta.** Sin registro central no hay forma de detectar duplicados ni de conciliar el total liquidado contra el total reconocido por el OR |
| **Banderas rojas** | Compensaciones a fronteras que no registran fallas de servicio · más de una compensación al mismo usuario y período · total liquidado distinto del total reconocido en la factura SDL |
| **Controles actuales** | C-29 (verificación en factura SDL), C-32 (TC2, parcial) |
| **Controles a implementar** | Registro central único · **conciliación total liquidado ↔ total reconocido por el OR** · verificación de unicidad por usuario y período · trazar la compensación al evento de calidad que la origina |
| **Evaluación** | Probabilidad **Baja-Media** · Impacto **Medio** · **Riesgo Medio** |

### F-07 · Extinción indebida de derechos de cobro y aprobación de balances sin respaldo

| | |
|---|---|
| **Esquema** | Aprobar un balance de energía del OR que no tiene provisión que lo respalde —o que ya fue cruzado— y, en paralelo, cerrar la disputa asociada sin ajuste |
| **Cómo se ejecutaría** | El cruce balance ↔ provisión **no se opera en el sistema**: se hace en Excel. Nada impide aprobar dos veces el mismo concepto ni aprobar uno inexistente |
| **Activo expuesto** | Salida de caja por energía ya pagada; derecho de cobro contra el OR que se extingue |
| **Oportunidad actual** | **Alta.** El modelo de datos contempla el cruce con estados de consumo de la provisión, pero **no se usa** |
| **Banderas rojas** | Balances aprobados sin provisión previa · provisiones que desaparecen sin cruce registrado · balances de períodos muy antiguos · el mismo concepto en dos balances |
| **Controles actuales** | C-25, C-26 (verificaciones manuales) |
| **Controles a implementar** | **Operar C-27 en el sistema** (cruce con estado y saldo) · **C-28: reporte de antigüedad de provisiones** · aprobación del Líder para balances sin provisión previa · revisión independiente de las disputas cerradas sin ajuste |
| **Evaluación** | Probabilidad **Media** · Impacto **Alto** · **Riesgo Alto** |

### F-08 · Uso indebido de accesos al módulo

| | |
|---|---|
| **Esquema** | Personas ajenas al área —o que dejaron de pertenecer a ella— con capacidad de cargar fuentes, ejecutar conciliaciones y registrar accionables |
| **Cómo se ejecutaría** | Sin ninguna acción especial: hoy **cualquier correo del dominio corporativo que entre por olibia queda dado de alta automáticamente con perfil `ANALISTA`**, que es un perfil con capacidad de modificar datos del proceso |
| **Activo expuesto** | La integridad de todo el proceso; además, información sensible de montos por operador |
| **Oportunidad actual** | **Alta**, por configuración |
| **Banderas rojas** | Usuarios activos que nunca operan · accesos fuera de horario · usuarios que no pertenecen a Finance con perfil `ANALISTA` |
| **Controles actuales** | C-07 (autenticación federada y restricción de dominio), C-06 (log parcial) |
| **Controles a implementar** | **Perfil de entrada `CONSULTA`** y ascenso a `ANALISTA` como acto explícito del Líder · **revisión trimestral de usuarios y perfiles** · desactivación inmediata al salir del área · completar el log de auditoría en los módulos migrados |
| **Evaluación** | Probabilidad **Media** · Impacto **Medio** · **Riesgo Medio-Alto** |

### F-09 · Manipulación de insumos tarifarios

| | |
|---|---|
| **Esquema** | Alterar los insumos de los que salen las tarifas SDL —cargos ADD y uso de la red— o el factor M, de modo que cambie el valor de disputas, provisiones y pérdidas |
| **Cómo se ejecutaría** | Cargando insumos manipulados: las tarifas se **recalculan** a partir de esos archivos y alimentan la valorización de todo el período |
| **Activo expuesto** | La valorización de todas las diferencias del período: es un cambio de una sola carga con efecto sobre miles de fronteras |
| **Oportunidad actual** | **Media.** El cálculo es automático y determinista —lo que lo hace verificable—, pero la entrada no se autentica (misma causa que F-02) |
| **Banderas rojas** | Tarifas que se desvían del promedio histórico del operador sin causa regulatoria · recargas de insumos después de haber conciliado · tarifas que cambian sin cambio en el archivo del OR |
| **Controles actuales** | C-01 (validación de período), C-09 (vista previa), cálculo determinista y reproducible |
| **Controles a implementar** | **Comparación automática de las tarifas del período contra las del período anterior**, con alerta por desviación · conservación del archivo original y su hash · re-conciliación obligatoria si se recargan insumos de un período ya conciliado |
| **Evaluación** | Probabilidad **Baja** · Impacto **Alto** · **Riesgo Medio-Alto** |

### F-10 · Ocultamiento de pérdidas por omisión deliberada de carga

| | |
|---|---|
| **Esquema** | No cargar —o cargar tarde— una fuente de un operador cuyas fronteras generarían pérdida, para que queden como `INCOMPLETA` y **no entren en el indicador ni en el reporte** |
| **Cómo se ejecutaría** | Simplemente omitiendo la carga. Las fronteras incompletas no se clasifican, no se valorizan y no suman a la pérdida del mes: el indicador mejora sin que nada se haya gestionado |
| **Activo expuesto** | La veracidad de los indicadores del área y de la cifra reportada a Contabilidad |
| **Oportunidad actual** | **Media.** El panel de completitud `n/21` lo hace visible, pero **el indicador no advierte de la completitud con la que se calculó** |
| **Banderas rojas** | Operadores recurrentemente faltantes · aumento del conteo de `INCOMPLETA` junto con mejora del indicador de pérdida · cargas del período hechas después del reporte del día 7 |
| **Controles actuales** | C-05 (completitud), C-10 (unión de fuentes: al menos las fronteras aparecen) |
| **Controles a implementar** | **Publicar el porcentaje de completitud junto a cada indicador** y bloquear el "en meta" si la completitud es inferior al 100 % · exigir justificación por operador faltante al cerrar el período · seguimiento de fronteras `INCOMPLETA` recurrentes por operador |
| **Evaluación** | Probabilidad **Media** · Impacto **Alto** · **Riesgo Alto** |

### F-11 · Fuga de información sensible

| | |
|---|---|
| **Esquema** | Extracción de información comercialmente sensible —montos por operador, cadencia de pagos, cartera de fronteras y usuarios— para uso propio o de un tercero |
| **Cómo se ejecutaría** | Por exportación a Excel, que el módulo ofrece en casi todas las pantallas y **no registra** de forma consistente; o por acceso directo a las tablas, si alguna credencial de base de datos se filtra |
| **Activo expuesto** | Posición negociadora frente a los OR; datos de usuarios finales |
| **Oportunidad actual** | **Media.** Las tablas de NetSuite —que contienen montos por operador e identificadores externos— **se crearon sin políticas de seguridad a nivel de fila**, con el riesgo aceptado y documentado por TI, y con mitigaciones que dependen de una verificación manual posterior al despliegue |
| **Banderas rojas** | Exportaciones masivas o fuera de horario · exportaciones por usuarios con perfil de consulta · credenciales compartidas |
| **Controles actuales** | C-07 (autenticación), acceso a datos solo a través del backend, secretos fuera del repositorio y nunca expuestos al navegador |
| **Controles a implementar** | **Registrar toda exportación** (usuario, pantalla, filtros, volumen) en el log de auditoría · completar las políticas de seguridad a nivel de fila pendientes · revisión periódica de privilegios de base de datos |
| **Evaluación** | Probabilidad **Baja-Media** · Impacto **Medio** · **Riesgo Medio** |

---

## 3. Resumen y priorización

| ID | Riesgo de fraude | Nivel | Control decisivo que falta |
|---|---|---|---|
| **F-04** | Desvío del pago de compensaciones a usuarios retirados | 🔴 **Crítico** | Segregación liquida/aprueba/paga + validación de titularidad + registro central |
| **F-01** | Cierre indebido de diferencias en favor del OR | 🔴 Alto | Aprobación por umbral y revisión independiente de accionables |
| **F-05** | Manipulación de la cifra reportada a Contabilidad | 🔴 Alto | Congelamiento y versionado del período reportado |
| **F-07** | Balances sin respaldo y disputas extinguidas | 🔴 Alto | Operar el cruce balance ↔ provisión en el sistema |
| **F-10** | Ocultamiento de pérdidas por omisión de carga | 🔴 Alto | Indicador con completitud incorporada |
| **F-02** | Alteración del archivo fuente | 🟠 Alto | Conservación y verificación del archivo original |
| **F-03** | Redirección del pago al OR | 🟠 Alto | Autorización dual del proveedor NetSuite |
| **F-09** | Manipulación de insumos tarifarios | 🟠 Medio-Alto | Comparación automática de tarifas contra el período anterior |
| **F-08** | Uso indebido de accesos | 🟠 Medio-Alto | Perfil de entrada `CONSULTA` y revisión trimestral |
| **F-06** | Compensación ficticia | 🟡 Medio | Registro central y conciliación con el OR |
| **F-11** | Fuga de información | 🟡 Medio | Registro de exportaciones |

**Lectura de conjunto.** Los cinco riesgos superiores comparten una sola causa estructural: **una
decisión con efecto económico que una única persona toma, ejecuta y da por cerrada, sin que un
segundo la revise ni el sistema la congele.** Casi todas las medidas propuestas son variantes de
lo mismo —aprobación por umbral, revisión por muestreo, congelamiento del período y registro
central de lo que hoy vive en Excel—, y **no requieren rediseñar el módulo**.

---

## 4. Matriz de segregación de funciones

**Situación actual** (⚫ ejecuta · ⚠️ **conflicto**: concentra funciones que deberían separarse)

| Función | Analista | Líder | Contabilidad | Tesorería | TI |
|---|---|---|---|---|---|
| Cargar fuentes | ⚫ | | | | |
| Ejecutar conciliaciones | ⚫ | | | | |
| Decidir el accionable sobre una diferencia | ⚠️ ⚫ | | | | |
| Cerrar una disputa sin ajuste | ⚠️ ⚫ | | | | |
| Aprobar un balance del OR | ⚠️ ⚫ | | | | |
| Liquidar una compensación | ⚠️ ⚫ | | | | |
| Informar la cuenta bancaria de destino | ⚠️ ⚫ | | | | |
| Armar el reporte a Contabilidad | ⚠️ ⚫ | | | | |
| Aprobar el reporte a Contabilidad | ⚠️ ⚫ | | | | |
| Modificar el proveedor NetSuite de un operador | | ⚫ | | | ⚫ |
| Crear y procesar el lote de órdenes de compra | ⚫ | | | | |
| Registrar contablemente | | | ⚫ | | |
| Ejecutar el pago | | | | ⚫ | |
| Administrar usuarios y perfiles | | ⚫ | | | ⚫ |

**Estado objetivo**

| Función | Analista | Líder | Contabilidad | Tesorería | TI |
|---|---|---|---|---|---|
| Cargar fuentes y ejecutar conciliaciones | ⚫ | | | | |
| **Proponer** el accionable | ⚫ | | | | |
| **Aprobar** accionables sobre umbral, cierres sin ajuste y balances sin provisión | | ⚫ | | | |
| Liquidar la compensación | ⚫ | | | | |
| **Validar titularidad y aprobar** el pago de compensación | | ⚫ | | ⚫ | |
| Ejecutar el pago | | | | ⚫ | |
| Armar el reporte a Contabilidad | ⚫ | | | | |
| **Aprobar** el reporte y **congelar** el período | | ⚫ | | | |
| **Conciliar de retorno** lo reportado vs. lo registrado | | | ⚫ | | |
| Modificar el proveedor NetSuite | | ⚫ (aprueba) | | | ⚫ (ejecuta) |
| Administrar usuarios y perfiles | | ⚫ (autoriza) | | | ⚫ (ejecuta) |

> **Nueve conflictos de segregación en la situación actual, todos sobre el mismo rol.** La
> corrección no exige contratar: exige que **la aprobación de lo que mueve dinero suba un nivel**
> —accionables sobre umbral, cierres sin ajuste, balances sin provisión, pagos de compensación y
> el reporte del día 7— y que Contabilidad haga la conciliación de retorno.

---

## 5. Analítica de detección (banderas rojas monitoreables)

Consultas periódicas sobre los datos del propio módulo. Ninguna requiere desarrollo nuevo: son
lecturas de tablas que ya existen.

| # | Bandera roja | Fuente | Periodicidad | Umbral de alerta |
|---|---|---|---|---|
| 1 | `AJUSTE_NO_PROCEDE` por analista y por operador | `gestiones_frontera` | Mensual | Desviación superior a 2σ del promedio del equipo |
| 2 | Disputas `CERRADA_SIN_AJUSTE` y su valor acumulado | `disputas` | Mensual | Cualquier cierre sobre el umbral de aprobación |
| 3 | Tiempo entre apertura y cierre de una disputa de alto valor | `disputas` | Mensual | Cierre el mismo día |
| 4 | Cargas reemplazadas por período, usuario y justificación | `cargas_fuente` | Mensual | Más de una por fuente y período |
| 5 | Conciliaciones re-ejecutadas **después** del día 7 | `log_auditoria` | Mensual | Cualquiera |
| 6 | Cambios del identificador de proveedor NetSuite | `log_auditoria`, `configuracion_or` | Continuo | Cualquiera |
| 7 | Lotes de OC creados dentro de los 5 días siguientes a un cambio de proveedor | `lotes_netsuite` | Continuo | Cualquiera |
| 8 | Fronteras `INCOMPLETA` recurrentes por operador | `resultados_conciliacion` | Mensual | Tres períodos consecutivos |
| 9 | Operadores faltantes al momento de conciliar | Estado del período | Mensual | Cualquiera |
| 10 | Tarifas SDL con desviación relevante frente al período anterior | `tarifas_sdl` | Mensual | Variación superior al 10 % sin causa regulatoria |
| 11 | Provisiones sin cruce con más de 3 meses de antigüedad | `provisiones` | Mensual | Más del 10 % del saldo |
| 12 | Compensaciones con cuenta bancaria repetida entre usuarios | Registro central (por crear) | Mensual | Cualquiera |
| 13 | Usuarios activos del módulo que no pertenecen al área | `users` | Trimestral | Cualquiera |
| 14 | Accesos y exportaciones fuera de horario laboral | `log_auditoria` | Mensual | Patrón sostenido |

---

## 6. Respuesta ante un indicio

1. **No investigar por cuenta propia ni confrontar al involucrado.** Preservar la evidencia:
   correos, archivos, registros del sistema.
2. **Reportar** por el canal ético de la compañía o a la Vicepresidencia Financiera, según la
   política corporativa de fraude.
3. **Congelar** el período y las operaciones relacionadas (accionables, lotes, pagos) hasta que se
   defina el alcance.
4. **Preservar los registros técnicos**: log de auditoría, historial de cargas y lotes de
   NetSuite tienen retención permanente y **no deben depurarse** durante una revisión.
5. La responsabilidad de la investigación es de Auditoría Interna y Cumplimiento, **no del área**.
