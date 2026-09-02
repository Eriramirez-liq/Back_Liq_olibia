# 06 — Brechas y Plan de Remediación

Área de Liquidaciones · Versión 1.0 — 2026-08-31

Lo que el proceso exige y hoy no está: por qué importa, qué cuesta cerrarlo y en qué orden.
Referencias cruzadas a [riesgos](./04-matriz-de-control.md#2-registro-de-riesgos-operativos) y
[fraude](./05-riesgos-de-fraude.md).

---

## 1. Brechas identificadas

### B-01 · Dos copias del mismo dato durante la migración a Go

**Qué pasa.** Los módulos migrados —Cargos STR, Tarifas SDL, TC1 y Proyección— escriben en las
bases de BIA, mientras que Conciliaciones, Congruencia, Gestiones y el dashboard **siguen leyendo
la copia anterior en Supabase**, que ya no se alimenta. En TC1 esto está explícitamente
identificado: migrarlo no cerró su circuito.

**Por qué importa.** Un analista puede cargar el TC1 del mes, ver el contador en `21/21` y
conciliar contra datos del mes anterior sin ninguna señal de que algo no cuadra. Es un riesgo de
integridad que **no deja rastro visible**.

**Riesgos:** R-27, R-03. **Impacto:** Alto. **Esfuerzo:** Alto (migrar Conciliaciones, el
consumidor principal). **Dueño:** TI.
**Mitigación mientras dure:** al conciliar, verificar que la fecha de la última carga de TC1 en la
pantalla corresponde al período que se está conciliando.

### B-02 · No existe motor de conciliación COT

**Qué pasa.** El sistema permite cargar el archivo COT, pero **no lo concilia**: el módulo de
Gestiones devuelve siempre una lista vacía para ese concepto. Todo el análisis —energía, tarifa,
aplicabilidad, contraste con lo cobrado al usuario— se hace en Excel.

**Por qué importa.** Es un concepto que cobra el OR mes a mes y del que no queda universo de
conciliación, valorización, accionable ni evidencia. No se puede auditar ni reconstruir.

**Riesgos:** R-11, R-12, R-13, F-01. **Impacto:** Alto. **Esfuerzo:** Medio (el patrón de SDL y
TC1 es reutilizable; debe seguir la misma regla de universo = unión de fuentes). **Dueño:** TI +
Área.

### B-03 · El cruce balance ↔ provisión está modelado pero no se opera

**Qué pasa.** El modelo de datos contempla provisiones con estado de consumo y una tabla de
cruces con energía, valor y tipo de resultado. **La operación se hace en Excel.**

**Por qué importa.** Sin el cruce en el sistema, nada impide aprobar un balance sin provisión que
lo respalde, cruzar dos veces la misma provisión, o mantener provisiones vivas indefinidamente.

**Riesgos:** R-10, R-14, R-16, F-07. **Impacto:** Alto. **Esfuerzo:** Medio-Bajo — **el modelo ya
existe; falta la pantalla y el flujo**. **Dueño:** TI + Área.

### B-04 · El período no se cierra ni se versiona

**Qué pasa.** El estado `CERRADO` del período existe en el modelo con su fecha y su responsable,
pero **no se usa**. Nada impide re-ejecutar una conciliación de un período ya reportado, y el
resultado **no se versiona**.

**Por qué importa.** Después del día 7 es imposible demostrar que la cifra enviada a Contabilidad
es la que el sistema tenía ese día. Es simultáneamente una brecha de auditoría (R-21) y la
habilitación técnica de F-05.

**Riesgos:** R-21, F-05. **Impacto:** Alto. **Esfuerzo:** Bajo — usar el estado que ya existe y
guardar una instantánea del resultado al cerrar. **Dueño:** TI.

### B-05 · La evidencia de la interlocución con el OR vive solo en el correo

**Qué pasa.** Reclamos, respuestas y aceptaciones del OR no se asocian al período ni a la
frontera dentro del módulo.

**Por qué importa.** El plazo de respuesta es de un día calendario y es el argumento de defensa
del área. Sin evidencia asociada, **no se puede probar que se reclamó a tiempo** ni medir el
cumplimiento (C-15, C-22 no son medibles hoy).

**Riesgos:** R-09, R-06, F-01. **Impacto:** Medio-Alto. **Esfuerzo:** Bajo-Medio — basta un campo
de fecha de reclamo y adjunto en la gestión. **Dueño:** TI + Área.

### B-06 · El proceso de compensaciones no tiene sistema ni registro central

**Qué pasa.** Cálculo, relación al OR, verificación en factura, traslado al usuario y pago
bancario ocurren íntegramente fuera del módulo. El indicador de compensaciones del dashboard es
un marcador vacío.

**Por qué importa.** Es **el único proceso del área que termina en una transferencia de efectivo
a una persona natural**, y hoy no tiene registro único, control de duplicados, validación de
titularidad ni segregación de funciones.

**Riesgos:** R-17, R-18, R-19, **F-04 (crítico)**, F-06. **Impacto:** **Crítico.** **Esfuerzo:**
Medio. **Dueño:** Área + Finance + TI.
**Lo urgente no es el software:** los controles manuales C-30 y C-31 —titularidad y doble
aprobación— **se pueden implementar esta semana**, sin desarrollo.

### B-07 · El TC2 excluye a los usuarios retirados

**Qué pasa.** El reporte a Contabilidad, por definición, cubre solo usuarios activos.

**Por qué importa.** El tramo del proceso con mayor riesgo —el pago bancario a quien ya no es
cliente— **queda fuera del único reporte de control existente**.

**Riesgos:** R-22, **F-04**. **Impacto:** Alto. **Esfuerzo:** Bajo — un anexo de control con los
retirados, aunque contablemente no forme parte del TC2. **Dueño:** Área + Contabilidad.

### B-08 · El flujo de orden de compra quedó desalineado con la migración de STR

**Qué pasa.** STR se migró a Go, pero el circuito de NetSuite sigue solo en el backend
TypeScript, que **no está desplegado en desarrollo**, y espera identificadores que la pantalla
nueva ya no maneja. Además, el servicio de NetSuite lee la tabla de STR en Supabase, que **quedó
vacía para las cargas nuevas**.

**Por qué importa.** El proceso mejor controlado del área quedó operativamente desconectado del
flujo que lo alimenta.

**Riesgos:** R-25, R-27. **Impacto:** Alto (operativo). **Esfuerzo:** Alto. **Dueño:** TI.

### B-09 · Segregación de funciones inexistente

**Qué pasa.** Un mismo perfil carga, concilia, decide el accionable, cierra la disputa, aprueba el
balance, liquida la compensación y arma el reporte. Nueve conflictos identificados en la
[matriz de segregación](./05-riesgos-de-fraude.md#4-matriz-de-segregación-de-funciones).

**Por qué importa.** Es la causa estructural común de los cinco riesgos de fraude más altos.

**Riesgos:** F-01, F-04, F-05, F-07. **Impacto:** **Crítico.** **Esfuerzo:** **Bajo — es una
decisión de gobierno, no un desarrollo.** **Dueño:** Líder de Liquidaciones + Finance.

### B-10 · El archivo original no se conserva ni se verifica

**Qué pasa.** El sistema guarda las filas parseadas, no el archivo recibido ni su huella digital.

**Por qué importa.** No hay forma de verificar que lo cargado es lo que el OR envió: es la
oportunidad que habilita F-02 y F-09.

**Riesgos:** R-04, F-02, F-09. **Impacto:** Medio-Alto. **Esfuerzo:** Medio (almacenamiento de
archivos; en TC1 el archivo llega a pesar 98 MB, por lo que conviene guardar **hash + archivo
original en un repositorio de archivos**, no en la base de datos). **Dueño:** TI.

### B-11 · El log de auditoría no cubre los módulos migrados

**Qué pasa.** El registro de auditoría vive en el backend TypeScript; **los flujos que ya corren
en Go no escriben en él**.

**Por qué importa.** Las acciones sobre STR, Tarifas SDL y TC1 —justamente las que ya se operan
en producción de desarrollo— quedan sin autor registrado en el log central. Debilita casi todas
las banderas rojas de detección.

**Riesgos:** R-29, F-01, F-03, F-05. **Impacto:** Alto. **Esfuerzo:** Bajo-Medio. **Dueño:** TI.

### B-12 · Controles técnicos pendientes con impacto en el proceso

Registrados por TI y con efecto directo sobre el control del área:

| Pendiente técnico | Efecto en el control |
|---|---|
| Tablas de NetSuite creadas **sin políticas de seguridad a nivel de fila**, con riesgo aceptado y mitigación por verificación manual posterior al despliegue | F-11 — montos por operador e identificadores externos expuestos si una credencial se filtra |
| **Un solo ambiente de desarrollo compartido**, donde el último despliegue pisa al anterior sin avisar | Pruebas de control no reproducibles; ya costó una sesión de diagnóstico |
| **Validación de período solo en STR y Tarifas SDL** | C-01 no cubre TC1, SDL, XM, Facturación ni Balance — R-01 sigue vivo en esas fuentes |
| **Alta automática de usuarios como `ANALISTA`** | F-08 |
| **Confirmación desde la interfaz escribe datos reales** en un modelo append-only que no se deshace desde la pantalla | Un error de cargue es permanente y solo se corrige con otra carga justificada |
| **Dos formas de endpoints y una integración de facturación duplicada** entre el front y el backend | Ambigüedad sobre cuál es la fuente de la cifra de facturación |

---

## 2. Plan de remediación

Ordenado por **impacto sobre el riesgo dividido por esfuerzo**. Las tres primeras acciones no
requieren desarrollo.

### Fase 1 — Inmediato (30 días, sin desarrollo)

| # | Acción | Cierra | Dueño |
|---|---|---|---|
| 1 | **Segregar el pago de compensaciones**: quien liquida no aprueba ni paga; exigir certificación bancaria del titular antes de todo pago a usuario retirado | **F-04**, B-06, C-30, C-31 | Líder + Tesorería |
| 2 | **Definir umbral en pesos** por encima del cual `AJUSTE_NO_PROCEDE`, `CERRADA_SIN_AJUSTE` y la aprobación de un balance sin provisión requieren visto bueno del Líder | **F-01**, F-07, B-09 | Líder |
| 3 | **Formalizar la aprobación del reporte del día 7** y dejar constancia de la versión enviada (archivo firmado o correo de aprobación) | **F-05**, C-35, B-04 | Líder |
| 4 | **Cambiar el perfil de entrada a `CONSULTA`** y hacer una revisión de usuarios y perfiles del módulo | F-08, R-28 | TI + Líder |
| 5 | **Anexo de control del TC2 con usuarios retirados** y sus compensaciones pagadas | B-07, F-04 | Área + Contabilidad |
| 6 | **Registro central de compensaciones** en un único archivo controlado, con estado, beneficiario, cuenta y soporte, mientras no haya módulo | B-06, F-04, F-06 | Área |
| 7 | **Rutina mensual de banderas rojas**: las 14 consultas de [05 §5](./05-riesgos-de-fraude.md#5-analítica-de-detección-banderas-rojas-monitoreables) | F-01, F-03, F-05, F-10 | Líder |

### Fase 2 — Corto plazo (90 días, desarrollo acotado)

| # | Acción | Cierra | Esfuerzo |
|---|---|---|---|
| 8 | **Cierre y versionado del período**: usar el estado `CERRADO` que ya existe, guardar la instantánea del resultado al reportar y alertar ante cualquier re-ejecución posterior | **B-04**, R-21, F-05 | Bajo |
| 9 | **Extender la validación de período a todas las fuentes** (TC1, SDL, XM, Facturación, Balance) | R-01, C-01 | Bajo |
| 10 | **Completar el log de auditoría en los módulos migrados a Go**, incluida la exportación de reportes | B-11, R-29, F-11 | Bajo-Medio |
| 11 | **Campo de reclamo en la gestión**: fecha de envío al OR, respuesta y adjunto | B-05, R-09, C-22 | Bajo-Medio |
| 12 | **Completitud incorporada al indicador**: mostrar el porcentaje de fuentes cargadas junto a cada indicador y no marcar "en meta" con completitud inferior al 100 % | **F-10**, R-24 | Bajo |
| 13 | **Autorización dual y notificación** ante cambios del proveedor NetSuite de un operador | **F-03**, C-47 | Bajo |
| 14 | **Alerta por desviación de tarifas SDL** frente al período anterior | F-09 | Bajo |
| 15 | **Conservar hash y archivo original** de cada carga | B-10, F-02 | Medio |

### Fase 3 — Mediano plazo (6 meses, desarrollo mayor)

| # | Acción | Cierra | Esfuerzo |
|---|---|---|---|
| 16 | **Operar el cruce balance ↔ provisión en el sistema**, con estados y reporte de antigüedad | **B-03**, R-10, R-14, R-16, F-07, C-27, C-28 | Medio |
| 17 | **Motor de conciliación COT**, con universo = unión de fuentes | **B-02**, R-11, R-12, R-13 | Medio |
| 18 | **Módulo de compensaciones** de punta a punta: cálculo, relación al OR, verificación en factura SDL, traslado y pago, con control de duplicados | **B-06**, F-04, F-06, R-17, R-18 | Medio-Alto |
| 19 | **Migrar Conciliaciones a Go** y eliminar la copia de datos duplicada | **B-01**, R-27 | Alto |
| 20 | **Realinear el flujo de orden de compra** con el STR migrado | **B-08**, R-25 | Alto |
| 21 | **Completar las políticas de seguridad a nivel de fila** pendientes | B-12, F-11 | Medio |

---

## 3. Seguimiento

| | |
|---|---|
| **Responsable del plan** | Líder de Liquidaciones |
| **Frecuencia de revisión** | Mensual, en el comité del área; trimestral con Finance |
| **Criterio de cierre de una brecha** | El control opera, tiene evidencia verificable y **se probó al menos una vez** |
| **Actualización de esta documentación** | En el mismo acto en que cambie el proceso o se cierre una brecha. Los cambios del sistema que afecten un control se registran además en [`INTEGRACION.md`](../../INTEGRACION.md) §5 |

**Métrica del plan:** brechas cerradas sobre brechas identificadas, y —más relevante—
**riesgos críticos y altos con al menos un control operando y probado**. Hoy: 5 de los 11 riesgos
de fraude no tienen ningún control efectivo.
