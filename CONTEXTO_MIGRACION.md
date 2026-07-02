# Contexto de Migración — Módulo de Liquidaciones (BIA Energy)

> **Propósito de este documento.** Este proyecto (`App_Liquidaciones`) va a migrarse para vivir como **submódulo dentro de otra aplicación**, gestionada desde otra terminal/repo. Este archivo captura **toda** la lógica, decisiones, fórmulas, modelo de datos, integraciones, convenciones y pendientes acumulados, para no perder contexto en la migración.
>
> Fecha de compilación: **2026-07-02**. Aplicación: reconciliación/liquidación SDL + STR de BIA Energy (comercializador de energía en Colombia).

---

## 0. Resumen ejecutivo

La app concilia, mes a mes, la energía y cargos de las **fronteras** (puntos de medida, código SIC tipo `FrtNNNNN`) entre tres fuentes:

- **Facturación BIA** (lo que BIA facturó — insumo maestro).
- **OR / SDL** (lo que reporta el Operador de Red).
- **XM** (el operador del mercado — la "verdad" regulatoria).

De esa conciliación salen **provisiones**, **pérdidas (contingencias)**, **disputas**, indicadores de **congruencia** (NT/propiedad entre Facturación/SDL/TC1), la **proyección de cargos OR**, y el envío de **Cargos STR a NetSuite** (órdenes de compra).

**Módulos (nav):** Inicio (Dashboard) · Cargas · Conciliaciones · Tarifas SDL · Cargos STR · Gestiones · Proyección Cargos OR · Reportes (en construcción) · Administración (Operadores/Usuarios, solo ADMINISTRADOR).

---

## 1. Stack tecnológico

- **Next.js 15** (App Router, route handlers, `runtime="nodejs"`, `maxDuration`, Suspense para `useSearchParams`).
- **React 18** — UI con **estilos inline** (paleta accent `#07c5a8` teal), formato `es-CO`.
- **Prisma 6** + **PostgreSQL (Supabase)**.
- **NextAuth 5 (beta)** — Credentials provider, JWT, bcryptjs, dominio obligatorio `@bia.app`.
- **Zod 4** — validación runtime en los route handlers.
- **xlsx 0.18** — exportaciones a Excel (server-side).
- **Vitest 3** — tests unitarios (módulo NetSuite, motor de conciliación).
- **Vercel** — hosting serverless. `build = prisma generate && next build`.

`package.json` (nombre `app-conciliacion-sdls`). Scripts clave: `dev`, `build`, `test` (vitest run), `db:generate`, `db:push`, `db:seed`.

---

## 2. Convenciones y restricciones CRÍTICAS (leer antes de tocar nada)

### 2.1 Clave de período (la fuente #1 de bugs)
- `PeriodoConciliacion.{anio, mes}` = **mes de CONSUMO**, NO el mes de carga. La **facturación = consumo + 1** (el dashboard muestra "mes en curso (facturación)" = consumo+1).
- Dos representaciones del período conviven:
  - **CUID** = `PeriodoConciliacion.id` (`@default(cuid())`).
  - **String** `"AAAA-MM"` = `${anio}-${String(mes).padStart(2,"0")}` (mes de consumo).
- **Usan `periodo_id` = CUID:** `ResultadoConciliacion`, `ResultadoConciliacionTC1`, `GestionFrontera`, `Provision`, `Contingencia`, `Disputa`, `CargaFuente`, `RegistroSTR`, `RegistroBalance`, `EnvioNetsuiteCargoSTR`, y (vía carga) `RegistroFacturacion`/`RegistroXM`/`RegistroSDL`/`RegistroTC1`/`RegistroCOT`.
- **Usan string `"AAAA-MM"`:** `TarifaSDL.periodo`, `RegistroSTR.mes_consumo` y `EnvioNetsuiteCargoSTR.mes_consumo`/`mes_facturacion`.
- ⚠️ Las queries a `RegistroFacturacion`/`RegistroSDL`/`RegistroTC1`/`TarifaSDL` por período usan el **string** `"AAAA-MM"`, mientras STR/provisiones/contingencias/resultados usan el **CUID**. Confundirlos da resultados en 0 (pasó varias veces).

### 2.2 Colapso de fronteras `_N` (aplica a SDL, TC1 y Congruencia)
- Solo el **código SIC sin guion bajo es válido**. Las `_N` (ej. `Frt11550_1`, `_2`, `_8`) **solo existen en la facturación de BIA**; el OR/SDL y XM manejan la base (`Frt11550`).
- Regla: agrupar por **clave base** (split en el primer `_`), **sumar la energía** (activa + reactiva) de todas las `_N` (y la base si existe) en la frontera base, y **heredar** NT / propiedad / tarifa / factor_m de la base (primer valor no-nulo, priorizando la fila base).
- Si facturación trae solo las `_N` (sin la base) y el OR sí tiene la base → se **crea** la frontera base con la suma. Dedupe por código completo antes de sumar (había doble conteo de activa).

### 2.3 Restricciones operativas del equipo
- **Sin pruebas locales** — todo se valida en **Vercel** (deploy). Nunca asumir `npm run dev`.
- **Commit + push por cada tarea** — conventional commits, historial granular; **sin `git add -A` ni `--force`**. Trailer de co-autoría al final del commit.
- **Nunca importar desde paths gitignored** en código de producción (el build de Vercel falla con "Module not found"; pasó con `_dev/mocks/`).
- **Secretos NetSuite/Metabase solo en env vars de Vercel** — jamás en repo/chat, **nunca con prefijo `NEXT_PUBLIC_`** (se filtrarían al cliente).
- **Migraciones que tocan enums o RLS se aplican MANUALMENTE en Supabase** (SQL editor), no vía `prisma migrate`.

### 2.4 Conexión a BD (serverless)
- `lib/db.ts` es un **singleton** de PrismaClient y **refuerza la URL**: agrega `connection_limit=1` y `pgbouncer=true` a `DATABASE_URL` si no vienen. Esto evita el error *"max clients reached in session mode — pool_size: 15"* al correr conciliaciones en Vercel.
- Fix definitivo recomendado: `DATABASE_URL` apuntando al **Transaction pooler de Supabase (puerto 6543)** con `?pgbouncer=true&connection_limit=1`.

---

## 3. Modelo de datos (Prisma)

`datasource db { provider = "postgresql"; url = env("DATABASE_URL") }`

### 3.1 Enums
| Enum | Valores |
|------|---------|
| **Rol** | `ANALISTA`, `ADMINISTRADOR`, `CONSULTA` |
| **TipoFuente** | `FACTURACION`, `XM`, `SDL`, `BALANCE`, `TC1`, `COT`, `INSUMOS_STR`, `INSUMOS_TARIFAS_SDL` |
| **EstadoCarga** | `PENDIENTE`, `PROCESANDO`, `COMPLETADA`, `ERROR` |
| **EstadoPeriodo** | `ABIERTO`, `EN_PROCESO`, `CERRADO`, `ANULADO` |
| **CasoConciliacion** | `A1`, `B1`, `B2`, `C1`, `C2`, `D1`, `D2`, `D3`, `D4`, `INCOMPLETA`, `ERROR` |
| **ResultadoLinea** | `SIN_DIFERENCIA`, `CONTINGENCIA_L1`, `PROVISION_L1`, `PROVISION_L2`, `DISPUTA_L2`, `PROVISION_COMBINADA`, `ALERTA_MANUAL`, `INCOMPLETA` |
| **TipoProvision** | `L1`, `D3`, `COMBINADA` |
| **EstadoProvision** | `PENDIENTE`, `CRUZADO_PARCIAL`, `CRUZADO_TOTAL` |
| **EstadoContingencia** | `PENDIENTE`, `COBRADO`, `CERRADO` |
| **ResultadoContingencia** | `PENDIENTE`, `PERDIDA_REPORTE`, `GANANCIA_REAL`, `PERDIDA_REAL` |
| **TipoResultadoCruce** | `INGRESO`, `COSTO`, `EXACTO` |
| **EstadoDisputa** | `ABIERTA`, `EN_GESTION`, `RESUELTA`, `CERRADA_SIN_AJUSTE` |
| **EstadoLoteNetsuite** | `EN_PROGRESO`, `COMPLETADO`, `CANCELADO` |
| **EstadoEnvioNetsuite** | `PENDIENTE`, `PROCESANDO`, `PROCESADO`, `ERROR` |
| **ConceptoGestion** | `SDL`, `TC1`, `COT` |
| **AccionGestion** | `CAMBIO_SOLICITADO_OR`, `AJUSTE_NO_PROCEDE`, `ERROR_BIA`, `AJUSTE_APLICADO` |
| **AccionAuditoria** | LOGIN, LOGOUT, CARGAR_FUENTE, REEMPLAZAR_FUENTE, EJECUTAR_CONCILIACION, CREAR/ACTUALIZAR_PROVISION, CREAR/ACTUALIZAR_CONTINGENCIA, REGISTRAR_CRUCE, CREAR/ACTUALIZAR_DISPUTA, EXPORTAR_REPORTE, CAMBIAR_CONFIGURACION, CREAR/ACTUALIZAR_USUARIO, ENVIAR_LOTE_NETSUITE, PROCESAR_ENVIO_NETSUITE, REENVIAR_ENVIO_NETSUITE, CANCELAR_LOTE_NETSUITE (no tiene valor para accionables de gestiones — pendiente) |

### 3.2 Modelos principales (campos clave)
- **User** (`users`): id, email (unique), nombre, password (bcrypt), rol (`Rol`), activo, timestamps.
- **PeriodoConciliacion** (`periodos_conciliacion`): id, anio, mes (**CONSUMO**), estado, `@@unique(anio, mes)`.
- **ConfiguracionOR** (`configuracion_or`): id, codigo (unique), nombre, nit, `mapeo_sdl_json`, `mapeo_balance_json`, **`netsuite_vendor_id`**, activo.
- **CargaFuente** (`cargas_fuente`): id, periodo_id, tipo_fuente, or_id, nombre_archivo, estado, total/procesados/error, `reemplaza_id`, `justificacion_reemplazo`, cargado_por_id.
- **RegistroFacturacion** (`registros_facturacion`): codigo_frontera, periodo ("AAAA-MM"), energia_kwh (activa), nt_raw, nivel_tension, propiedad_activos, reactiva ind/cap (tot y **pen**), factor_m, tarifas `g_bia`, **`g_bolsa_bia`**, `t_bia`, `d_bia`, `pr_bia`, `r_bia`, `c_bia`, `tarifa_total_bia`, **`valor_total_cop`** (columna "Total" de Metabase).
- **RegistroXM** (`registros_xm`): codigo_frontera, nombre_frontera, energia_xm_kwh.
- **RegistroSDL** (`registros_sdl`): codigo_frontera, energia_sdl_kwh, valor_sdl_cop, tarifa_sdl, nivel_tension, propiedad_activos, reactiva ind/cap pen, valor_reactiva_cop, tarifa_reactiva, factor_m, `es_duplicado`, or_id.
- **RegistroTC1** (`registros_tc1`): codigo_frontera, niu, nivel_tension, nivel_tension_primario, pct_propiedad_activo, propiedad_activos (derivada), tipo_conexion, id_comercializador, detalle_json.
- **RegistroSTR** (`registros_str`): periodo_id (**CUID** = mes facturación), **`mes_consumo` ("AAAA-MM")**, or_id, valor_cop, detalle_json.
- **RegistroBalance** (`registros_balance`), **RegistroCOT** (`registros_cot`): estructuras análogas por OR/frontera.
- **ResultadoConciliacion** (`resultados_conciliacion`): e_fac, e_xm, e_sdl, delta_l1 (fac−xm), delta_l2 (xm−sdl), caso, resultado_l1/l2, impacto_financiero_l1/l2, requiere_alerta_manual, observaciones, **indicadores extendidos** (`ind_pen_fac/sdl/delta`+`diff_inductiva`, `cap_pen_*`+`diff_capacitiva`, `factor_m_fac/sdl`+`diff_factor_m`, `nivel_tension_fac/sdl`+`diff_nivel_tension`, `propiedad_activos_fac/sdl`+`diff_propiedad`). `@@unique(periodo_id, codigo_frontera)`.
- **ResultadoConciliacionTC1** (`resultados_conciliacion_tc1`): nivel_tension_fac/tc1 + diff, propiedad_fac/tc1 + diff, caso (`SIN_DIFERENCIA`|`DIFERENCIA`|`INCOMPLETA`). Campos planos (sin relaciones). `@@unique(periodo_id, codigo_frontera)`.
- **GestionFrontera** (`gestiones_frontera`): periodo_id (CUID), concepto (`ConceptoGestion`), codigo_frontera, or_id, accion (`AccionGestion`), **`datos_ajustados TEXT[]`**, observacion, gestionado_por_id, timestamps. Campos planos. `@@unique(periodo_id, concepto, codigo_frontera)`.
- **TarifaSDL** (`tarifas_sdl`): periodo ("AAAA-MM"), or_codigo, nivel_tension ("1"|"2"|"3"), propiedad_activos ("OR"|"COMPARTIDO"|"USUARIO"), tarifa_activa, tarifa_reactiva. `@@unique(periodo, or_codigo, nivel_tension, propiedad_activos)`.
- **Provision** (`provisiones`), **Contingencia** (`contingencias`), **Disputa** (`disputas`): derivados de la conciliación, con `resultado_id`, energia_kwh, valores COP, estado. Contingencia guarda `costo_estimado_cop` (= pérdida valorizada).
- **CruceBalance** (`cruces_balance`): cruza balance del OR contra provisión/contingencia.
- **LogAuditoria** (`log_auditoria`).
- **LoteNetsuite** (`lotes_netsuite`) + **EnvioNetsuiteCargoSTR** (`envios_netsuite_cargo_str`): lote de envíos + envío individual (monto_snapshot_cop, mes_consumo, mes_facturacion, estado, intentos, numero_oc, netsuite_internal_id, `idempotency_key` unique, error_*). `@@unique(lote_id, periodo_id, or_id)`.

---

## 4. Migraciones (`prisma/migrations/`)

| Carpeta | Qué hace | Aplicación |
|---|---|---|
| `20260522000000_baseline` | Esquema base (enums, tablas y FKs de conciliación). | Ya aplicada (baseline) |
| `20260525000000_netsuite_cargos_str` | Enums + tablas `LoteNetsuite`/`EnvioNetsuiteCargoSTR`. | Prisma |
| `20260617000000_netsuite_vendor_id` | Columna `netsuite_vendor_id` en `ConfiguracionOR`. | Prisma |
| `20260617120000_netsuite_audit_enum` | Nuevas acciones de auditoría NetSuite (`ADD VALUE IF NOT EXISTS`). | **Manual en Supabase** (enum) |
| `20260617130000_netsuite_rls` | RLS en tablas NetSuite + revoca anon/authenticated. | **Manual en Supabase** (RLS) |
| `20260630000000_facturacion_valor_total` | Columna `valor_total_cop` en `RegistroFacturacion`. | Prisma |
| `20260630140000_add_rol_consulta` | Rol `CONSULTA` al enum `Rol` (`ADD VALUE IF NOT EXISTS`). | **Manual en Supabase** (enum) |
| `20260701150000_gestiones_frontera` | Enums `ConceptoGestion`/`AccionGestion` + tabla `gestiones_frontera`. | **Manual en Supabase** (enums + tabla) |

> Las marcadas "Manual" deben ejecutarse en el SQL editor de Supabase antes de desplegar el código que las usa.

---

## 5. Variables de entorno

| Variable | Default | Obligatoria | Uso |
|---|---|---|---|
| `DATABASE_URL` | — | ✅ | Postgres/Supabase. `lib/db.ts` le agrega `connection_limit=1&pgbouncer=true`. |
| `NODE_ENV` | — | ❌ | Logging de Prisma. |
| `METABASE_API_KEY` | — | ✅ (queries Metabase) | API key de cuenta de servicio. |
| `METABASE_BASE_URL` | `https://bia.metabaseapp.com` | ❌ | Base URL Metabase. |
| `CRON_SECRET` | — | ❌ (recomendado) | Bearer del cron de limpieza de lotes NetSuite. |
| `NETSUITE_MODE` | `mock` | ❌ | `mock` o `real`. |
| `NETSUITE_BASE_URL` | `https://8312907.suitetalk.api.netsuite.com` | ❌ | Host REST. |
| `NETSUITE_ACCOUNT_ID` | `8312907` | ❌ | Realm/Account. |
| `NETSUITE_CONSUMER_KEY/SECRET`, `NETSUITE_TOKEN_ID/SECRET` | — | ✅ (si `real`) | OAuth 1.0a TBA. **Nunca `NEXT_PUBLIC_`.** |
| `NETSUITE_RECORD_PATH` | `/services/rest/record/v1/purchaseOrder` | ❌ | Endpoint de la OC. |
| `NETSUITE_SUBSIDIARY_ID` | `2` | ❌ | Subsidiary. |
| `NETSUITE_LOCATION_ID` | `null` | ❌ | Location (opcional; solo si se setea). |
| `NETSUITE_ITEM_ID` | `488` | ❌ | Item de línea. |
| `NETSUITE_DEFAULT_QUANTITY` | `1` | ❌ | Cantidad de línea (cargo a tanto alzado). |
| `NETSUITE_DEPARTMENT_ID` | **`131`** | ❌ | Department (cambiado de 129 → 131). |
| `NETSUITE_CATEGORIA_PROVEEDOR_ID` | `27` | ❌ | Categoría de proveedor (custom body). |

---

## 6. Integraciones externas

### 6.1 Metabase (`lib/integrations/metabase.ts`)
Cliente que ejecuta *cards* guardadas vía `POST /api/card/{id}/query/json` con header `X-API-KEY`. `obtenerParametrosCard(id)` lee la metadata (`GET /api/card/{id}`) para reusar los parámetros reales (id/type/target) en queries parametrizadas.

**Cards usadas:**
| Card | Nombre | Uso | Parámetros |
|---|---|---|---|
| **1237** | Precio_Bolsa_Nacional_Dia -jpq | **G de bolsa** nacional para valorizar pérdidas. Columna `promedio_precio_bolsa_nacional`. | `date_type="month"` (array), `version="TxF"` (array), `date="AAAA-MM-01~AAAA-MM-ultimoDia"` (string). Se pasan por **`id`** del parámetro (no por `name`, que rompe). |
| **77419** | proyeccion demanda | **Demanda proyectada** para Proyección Cargos OR. Columnas `mes` (consumo) y `total_kwh`. | Sin parámetros. |
| **76099** | aenc-xm-final | Energía **XM** importada. Columna `total aenc_div_perdidas`, filtro `agente_comercial_que_importa = BIAC`, agrupa por SIC. | — |
| **73360** | validador-sdl | **Facturación BIA** (preview desde Metabase, alternativa a subir archivo). | Filtra por período en cliente. |

> Detalle clave del pasaje de parámetros: los filtros `string/=` esperan el valor como **array** (`["month"]`, `["TxF"]`); los `date/range` como string con `~`. Enviar el campo `name` del parámetro hace que Metabase busque un template-tag inexistente → 500.

### 6.2 NetSuite (`lib/integrations/netsuite/`)
- `client.ts`: factory `mock` (determinista, para tests/dev) vs `real` según `NETSUITE_MODE`.
- `real-client.ts`: arma el cuerpo de la **Purchase Order** (entity/vendor, subsidiary, department **131**, item, categoría proveedor, currency, memo, tranDate; `location` solo si `NETSUITE_LOCATION_ID`). Envío firmado.
- `oauth-tba.ts`: OAuth 1.0a + Token-Based Auth, firma **HMAC-SHA256**.
- `mapper.ts` / `types.ts`: DTOs (`EnvioDto`, `LoteDto`, `LoteResumenDto`, `EstadoEnvioPorCargoDto`). El `amount` siempre string (`Decimal.toFixed(2)`, nunca `Number`).
- Advisory lock key usada: `8312907210000001` (con `$executeRawUnsafe` por bug de BigInt en `$executeRaw`).
- Flujo: crear **lote** → `procesar` (fire-and-forget 202) → polling del lote cada 2.5s → estados PENDIENTE/PROCESANDO/PROCESADO/ERROR → reenviar envíos fallidos. Cron limpia lotes colgados.

### 6.3 TC1 → Google Sheets (pendiente)
Cada carga TC1 debe hacer *append* a un Google Sheet que alimenta Metabase (reemplazo por `COD_FRONTERA` + Periodo). **Service Account pendiente.**

---

## 7. Lógica de negocio — Motor de conciliación (`lib/engine/`)

### 7.1 Conciliación SDL (`conciliacion-sdl.ts`, `.casos.ts`, `conciliacion-orchestrator.ts`)
Compara `e_fac` (Facturación), `e_xm` (XM), `e_sdl` (OR) por frontera. Umbral de diferencia por defecto **100 kWh**. `delta_l1 = fac − xm`, `delta_l2 = xm − sdl`.

**Casos (activa):**
| Caso | Condición | Resultado | Fórmula de valor |
|---|---|---|---|
| **A1** | fac ≈ xm ≈ sdl | SIN_DIFERENCIA | — |
| **B1** | fac < xm = sdl | CONTINGENCIA_L1 (pérdida) | `(xm−fac) × (g_bolsa + t + d + pr + r)` |
| **B1-ext** | fac ≈ sdl < xm | CONTINGENCIA_L1 (pérdida) | `(xm−fac) × (g_bolsa + t + (d − tarifa_sdl) + pr + r)` |
| **B2** | fac > xm = sdl | PROVISION_L1 | `(fac−xm) × (g_bia + t + d + pr + r)` |
| **C1** | fac = xm > sdl | DISPUTA_L2 | `|delta_l2| × tarifa_sdl` |
| **C2** | fac = xm < sdl | DISPUTA_L2 | `|delta_l2| × tarifa_sdl` |
| **D1** | fac < sdl < xm | CONTINGENCIA_L1 + ALERTA_MANUAL | `(xm−fac) × (g_bolsa + t + d + pr + r)` |
| **D2** | fac > sdl > xm | PROVISION_L1 + ALERTA_MANUAL | `(fac−xm) × (g_bia + t + d + pr + r)` |
| **D3** | xm < fac = sdl | PROVISION_L1 (tarifa especial) | `(fac−xm) × (g_bia + t + (d − tarifa_sdl) + pr + r)` |
| **D4** | tres valores distintos | ALERTA_MANUAL + (contingencia si fac<xm, provisión si fac>xm) | según pérdida/provisión |
| **INCOMPLETA** | falta xm o sdl | INCOMPLETA | — |

> **Regla clave (confirmada por la usuaria):** pérdida y provisión son la **misma fórmula**, solo cambia la "g": **pérdida usa `g_bolsa`** (G de bolsa nacional, Metabase card 1237), **provisión usa `g_bia`** (G de facturación). Los `t/d/pr/r` salen de facturación.

**G de bolsa en el orchestrator:** se obtiene una vez por período (mes de consumo) y se inyecta como `g_bolsa_bia` a **todas** las fronteras. Si Metabase falla, cae al `g_bolsa` por-frontera de facturación. El valor de pérdida se **persiste** en `contingencia.costo_estimado_cop` al correr la conciliación → si se corrió antes de tener g_bolsa, hay que **re-ejecutar** (botón "Recalcular pérdidas" del dashboard).

**Indicadores extendidos (`clasificarIndicadores`, fac vs sdl):** inductiva, capacitiva (reactiva penalizada), factor_m, nivel_tension, propiedad. Manejo de null: si ambos null → no hay diff; si uno tiene dato y el otro null → el null cuenta como 0 (sí hay diff).

**Orchestrator — flujo:** resolver período → cargar Facturación/XM/SDL → G de bolsa → **colapso `_N`** + herencia + dedupe → clasificar cada frontera → transacción idempotente (borra por frontera del período y reinserta ResultadoConciliacion + Provision/Contingencia/Disputa). Huérfanas (en XM/SDL pero no en Facturación) → INCOMPLETA. Devuelve `ResumenConciliacion` (conteos por caso, indicadores, provisiones/contingencias/disputas con valores, `gBolsaNacional`).

### 7.2 Conciliación TC1 (`conciliacion-tc1.ts`)
Facturación **vs TC1 únicamente** (no SDL). Compara **nivel de tensión** y **propiedad de activos** (normalizados). Colapso `_N` + herencia. Universo = unión de fronteras del OR (TC1 filtrado por or_id ∪ Facturación con `operador_red = or_codigo`); se cruza por `codigo_frontera` (no por rótulo de operador). Nivel de tensión del TC1 se toma por **posición** (primera columna de nivel tras `TIPO_DE_CONEXION`), no por nombre (fix CENS). Resultado por frontera: `SIN_DIFERENCIA` | `DIFERENCIA` | `INCOMPLETA`.

### 7.3 Congruencia (`congruencia.ts`, `congruencia-reporte.ts`)
Cruza **las 3 fuentes** (Facturación, SDL, TC1) por NT y propiedad. Estados: "Cambio TC1", "Cambio SDL", "Cambio bills", "No se relaciona en el TC1/SDL/Facturación", "Revisar". **Valor vacío = SIN DATO** (se ignora; solo compara fuentes con dato). Colapso `_N` + herencia. `clasificarCongruencia` devuelve `null` si es congruente; el reporte solo lista las no-congruentes. Alimenta el indicador **% Congruencia** del dashboard.

### 7.4 Gestiones (`gestiones.ts`)
Lista **fronteras con diferencia** por concepto, mergeadas con su accionable guardado. Umbral 100 kWh.
- **TC1:** `caso != "SIN_DIFERENCIA"`.
- **SDL:** algún `diff_*` en true, o caso `INCOMPLETA`/`ERROR`, o **regla de activa**: los 3 montos presentes y `|e_fac − e_sdl| > 100` **y** `|e_sdl − e_xm| > 100`.
- **COT:** `[]` (en construcción).
- `FilaGestion`: concepto, periodoId, codigoFrontera, operadorNombre, orId, caso, eFac/eXm/eSdl, `diffs[]` (`{campo, fac, or}`), `gestion` (accionable). Accionables: `CAMBIO_SOLICITADO_OR`, `AJUSTE_NO_PROCEDE`, `ERROR_BIA`, `AJUSTE_APLICADO` (este exige `datos_ajustados`: subconjunto de `activa/inductiva/capacitiva/factor_m/nivel_tension/propiedad`).

### 7.5 Proyección Cargos OR (`proyeccion-cargos-or.ts`)
Matriz por mes (consumo). Constantes **fijas en código**:
- Activa por NT: **NT1 34.52%, NT2 52.96%, NT3 12.52%**.
- Reactiva total = **2.29%** de SDL Energy; por NT: **NT1 47.36%, NT2 48.57%, NT3 4%**.
- STR Energy = SDL activa × **1.08** (+8%).
- **Demanda meses reales** = Σ energía activa facturada del período (dedup por frontera). **Demanda meses proyectados** = Metabase card 77419 (`total_kwh` por mes de consumo).
- **Precios** SDL/NT = promedio de `tarifas_sdl` por NT; **precio STR** = total a pagar STR ÷ STR energy. Meses proyectados: precios = **promedio de los últimos 6 meses reales**.
- `calcularMes(sdlEnergy, precios)` → energías + salida valorizada (COP). Endpoint: `GET /api/proyeccion-cargos-or?mesesProyeccion=N`.

### 7.6 Tarifas SDL (`tarifas-sdl.ts`)
Reconstruye tarifas activa/reactiva por OR/NT/propiedad desde insumos (Cargos ADD + Uso de la Red), replicando los scripts Python. 21 ORs SDL con tipo ADD/USO y área (CENTRO/OCCIDENTE/ORIENTE/SUR para ADD). Fórmulas (activa): `NT1_OR = NT1 − CDN4/(1−PR1)`, `NT1_Compartido = NT1_OR − CDI×0.5`, `NT1_Usuario = NT1_OR − CDI`, `NT2_Usuario = NT2 − CDN4/(1−PR2)`, `NT3_Usuario = NT3 − CDN4/(1−PR3)`. Reactiva: NT1_OR=NT1, compartido/usuario restan CDI×0.5 / CDI. **Pendiente:** CDI/CDN4/PR para ORs tipo ADD + archivos de ejemplo.

---

## 8. Backend — Endpoints (`app/api/`)

**Auth:** todos exigen sesión (`auth()` de `lib/auth.ts`); los de usuarios exigen rol `ADMINISTRADOR`.

- **Períodos:** `GET /api/periodos`.
- **Cargas:** `GET /api/cargas`, `POST /api/cargas/preview`, `POST /api/cargas/confirmar`, `POST /api/cargas/check-previa`, `GET /api/cargas/estado-periodo`, `GET /api/cargas/preview-facturacion` (Metabase 73360), `GET /api/cargas/preview-xm-metabase` (76099), `POST /api/cargas/exportar-sdl`.
- **Conciliaciones:** `POST /api/conciliaciones/ejecutar` (body `{anio, mes, orId?, tipo:"SDL"|"TC1"}`, `maxDuration=120`), `GET /detalle`, `GET /detalle-tc1`, `POST /exportar`, `POST /exportar-tc1`.
- **Congruencia:** `GET /api/congruencia?periodoId&orCodigo?&estado?`, `POST /api/congruencia/exportar`.
- **Gestiones:** `GET /api/gestiones?concepto=SDL|TC1|COT&periodoId&orId`, `GET`/`POST /api/gestiones/accionable`, `GET /api/gestiones/exportar`.
- **Proyección:** `GET /api/proyeccion-cargos-or?mesesProyeccion=N`.
- **Precio bolsa:** `GET /api/precio-bolsa?anio&mes` (card 1237).
- **Tarifas SDL:** `GET /api/tarifas-sdl?periodos&orCodigos&nivel&propiedad`.
- **Dashboard:** `GET /api/dashboard?periodoId`.
- **SDL/STR por OR:** `GET /api/sdl-por-or?periodoId`, `GET /api/str-por-or?periodoId`.
- **Cargos STR / NetSuite:** `GET /api/cargos-str`, `GET /api/cargos-str/meses`, `POST /api/cargos-str/netsuite/lote`, `GET /lotes`, `GET /lote/[loteId]`, `POST /lote/[loteId]/procesar`, `POST /lote/[loteId]/cancelar`, `GET /lote/activo`, `GET /estados`, `POST /envio/[envioId]/reenviar`, `POST /cron/limpiar-lotes`.
- **Usuarios (admin):** `GET/POST /api/usuarios`, `GET/PUT /api/usuarios/[id]`.
- **Operadores:** `GET/POST /api/operadores?tipo=str|sdl&includeMapeo`, `GET/PUT /api/operadores/[id]`.
- **Auth:** `/api/auth/[...nextauth]`.

**Parsers (`lib/parsers/`):** `facturacion-metabase.ts` (deriva NT/propiedad del código NT: 1/11→NT1-OR, 12→NT1-Usuario, 13→NT1-Compartido, 20→NT2-Usuario, 30→NT3-Usuario; mapea columna "Total"→`valor_total_cop`), `xm-metabase.ts` (card 76099), `sdl.ts`, `xm.ts`, `tc1.ts` (33 columnas CREG, nivel por posición, filtro ID_COMERCIALIZADOR=62371 BIA, propiedad de PORC_PROPIEDAD: 0/101→USUARIO, 50→COMPARTIDO, 100→OR), `insumos-str.ts`, `insumos-tarifas-sdl.ts`.

**Validación (`lib/validation/`):** `cargas.ts`, `netsuite.ts`, `gestiones.ts` (Zod). **Utils:** `periodos.ts` (`esPeriodoPermitido` = solo hasta el mes anterior). **Auth:** `lib/auth.ts` (`auth()` → `{user:{id,nombre,rol}}`), `auth.ts` raíz (NextAuth, `@bia.app`, bcrypt, JWT).

---

## 9. Frontend (`app/(dashboard)/`, `components/`)

**Convenciones:** `"use client"`, estilos inline, accent `#07c5a8`, formato `es-CO`, `Suspense` alrededor de `useSearchParams`, `next/link`, exportaciones Excel vía fetch → blob → download.

**Páginas:**
- **Dashboard (`/`)**: selector de período, **etiqueta G de bolsa** (card 1237, mes de consumo), botón **Recalcular pérdidas** (re-ejecuta conciliación SDL), tabs Panel Principal / Histórico 12M. **KPIs:** Facturación BIA, Cargo STR (→cargos-str-por-or), Cargo SDL (→preliquidaciones-sdl), Pérdidas (→gestiones?concepto=SDL), Provisiones (→gestiones?concepto=SDL). **Indicadores de gestión** (barra con marca de meta + "EN META/FUERA DE META", 2 decimales): **% Congruencia (>95%)** [primero, →congruencia], % Pérdida (<0.1% = pérdida$/facturado$), % Dif. kWh absoluto (<0.35% = (kWh pérdida+provisión)/kWh facturados), % Reportado de más a XM (<0.15% = kWh pérdida/facturados), % Reportado de menos a XM (<0.2% = kWh provisión/facturados). Top 10 fronteras (pérdida+provisión). *(Compensaciones oculto, pestaña "Por Operador de Red" eliminada.)*
- **Cargas (`/cargas`, `/cargas/nueva`)**: wizard de carga por fuente, preview, historial, estado del período.
- **Conciliaciones (`/conciliaciones`)**: ejecutar SDL/TC1, resumen por indicador, detalle por frontera, export Excel.
- **Tarifas SDL (`/tarifas-sdl`)**: tabla filtrable por mes/OR/NT/propiedad/energía.
- **Cargos STR (`/cargos-str`, `/cargos-str/historial`)**: matriz por período/OR con estados NetSuite, crear/procesar/cancelar lote, polling, reenviar, detalle de envío.
- **Gestiones (`/gestiones`)**: **tabs por concepto (SDL/TC1/COT)**, filtros período/OR, tabla de fronteras con diferencias (chips `campo: BIA→OR`), **modal de accionable** (4 acciones; "Ajuste aplicado" → multi-select de datos ajustados), **contador** "N sin gestionar / M gestionadas", **Exportar a Excel**.
- **Proyección Cargos OR (`/proyeccion-cargos-or`)**: matriz columnas=meses (real gris / proyectado azul), secciones **Demanda (kWh)** / **Precio (COP/kWh)** / **Salida flujo Cargos OR (COP)** (SDL sombreado → Active Energy + NT → Reactive Energy + NT → STR → Total Cargos OR resaltado), selector meses a proyectar.
- **Congruencia (`/congruencia`)**: tabla SIC/OR/Estado/Diferencia/Dato errado/Dato correcto, filtros OR/estado, export Excel.
- **Preliquidaciones SDL (`/preliquidaciones-sdl`)**, **Cargos STR por OR (`/cargos-str-por-or`)**: resúmenes por operador.
- **Administración (`/administracion`)**: solo ADMINISTRADOR. Tabs **Operadores** (tabla + edición inline de `netsuite_vendor_id`) y **Usuarios** (alta con rol, cambio de rol, activar/desactivar). El módulo Operadores dejó de estar en el nav principal (`/operadores` redirige a `/administracion`).
- **Reportes (`/reportes`)**: placeholder. **Login (`/login`)**: NextAuth.

**Componentes:** `layout/Sidebar.tsx` (nav; subtítulo bajo "BIA Energy" = **"Módulo de Liquidaciones"**; sección admin condicional), `layout/TopBar.tsx`, `ui/MultiSelect.tsx`, `cargas/*` (Wizard, Panel, Tabla), `cargos-str/*` (FilaOperador, CeldaMonto, ModalConfirmarLote, PanelLoteEnCurso, DetalleEnvioModal, BotonCrearOC, Toast, types), `auth/LoginForm.tsx`.

---

## 10. Whitelists de operadores

- **STR** (`lib/constants/operadores.ts` — `STR_OPERADORES`): ~22 ORs (AFINIA, AIRE, CENS, CEO, CHEC, EBSA, EDEQ, EEP_*, EMSA, ENEL, EPM, ESSA, etc.).
- **SDL** (`SDL_OPERADORES`): **21 ORs** (subconjunto/variante; filtrable con `?tipo=sdl`).

---

## 11. Pendientes conocidos (TODO al migrar)

- **Migraciones manuales en Supabase** aún por aplicar según ambiente: `20260617120000` (audit enum), `20260617130000` (RLS), `20260630140000` (rol CONSULTA), `20260701150000` (gestiones_frontera).
- **Rol CONSULTA:** existe el enum y la gestión de usuarios, pero **falta la lógica read-only** (hoy el único gate real es ADMINISTRADOR vs no).
- **Gestiones COT:** `listarCOT` sin implementar (concepto en construcción). **Auditoría de accionables** de gestiones: falta valor en `AccionAuditoria`.
- **TC1 → Google Sheets:** Service Account pendiente (append por carga que alimenta Metabase).
- **Multi-carga SDL EEP Pereira:** el wizard debe ofrecer Reemplazar/Agregar para `EEP_PEREIRA` (NT3 + NT1/2 complementarios).
- **Tarifas SDL:** CDI/CDN4/PR para ORs tipo ADD + archivos de ejemplo.
- **Reportes:** módulo placeholder.
- **Dashboard "Compensaciones":** lógica pendiente (KPI oculto).

---

## 12. Notas para la migración a submódulo

- El código no tiene acoplamientos ocultos fuera de: `DATABASE_URL` (Prisma singleton en `lib/db.ts`), `METABASE_API_KEY`/cards, credenciales NetSuite, y NextAuth (`@bia.app`). Todo lo demás es autocontenido en `app/`, `lib/`, `components/`, `prisma/`.
- Al integrarlo como submódulo, mantener:
  - El **prefijo de rutas** (`/api/...` y páginas) o ajustarlo de forma consistente en front y back a la vez.
  - La **convención de clave de período** (CUID vs `"AAAA-MM"`) — es la trampa más frecuente.
  - El **singleton de Prisma** con el refuerzo de `connection_limit=1&pgbouncer=true` (serverless).
  - Las **env vars** de la sección 5 (especialmente secretos NetSuite/Metabase, nunca `NEXT_PUBLIC_`).
  - Aplicar las **migraciones manuales** de la sección 4 en la BD destino.
- Motores puros reutilizables sin dependencia de framework: `lib/engine/conciliacion-sdl.ts`, `proyeccion-cargos-or.ts`, `congruencia.ts`, `tarifas-sdl.ts` (funciones deterministas, ideales para portar/testear con Vitest).

---

*Documento generado a partir del mapeo completo del repositorio `App_Liquidaciones` (backend, frontend, datos) + el historial de decisiones del proyecto.*
