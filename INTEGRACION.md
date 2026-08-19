# INTEGRACIÓN — Back_Liq_olibia ⇄ olibia-web

> **Qué es este archivo:** la bitácora viva de la conexión entre este backend y el front.
> Se consulta ante cualquier duda de "¿el front ya consume esto?", "¿por dónde entra la auth?",
> "¿qué se cambió y por qué?". Está pensado para que una sesión de trabajo nueva (persona o
> Claude) se ponga al día leyendo solo este archivo.
>
> **Cómo mantenerlo:** ver [§7 Cómo actualizar este archivo](#7-cómo-actualizar-este-archivo).
> **Espejo en el front:** `olibia-web/docs/INTEGRACION-LIQUIDACIONES.md`.
> **Detalle técnico del mecanismo de auth:** [docs/backend/conexion-olibia-web.md](./docs/backend/conexion-olibia-web.md).

---

## 1. Los dos repos

| Repo | Rol | Puerto local |
|------|-----|--------------|
| `Back_Liq_olibia` (este) | Backend del módulo de Liquidaciones: rutas `/api/*`, Prisma/Supabase, integraciones (NetSuite, bia-bills, XM/Metabase). Tiene además un dashboard propio con login de credenciales. | **4000** (`npm run dev` ya lo fija) |
| `olibia-web` | Front del módulo. Consume este backend desde `src/modules/finance/liquidations/`. | **3000** |

Viven en el mismo workspace y se trabajan en paralelo: **un endpoint nuevo acá no existe para el
front hasta que se declara en su `endpoints.ts`**.

---

## 2. Cómo viaja un request

```
Browser (olibia-web)
  └─ doFetchLiquidations(...)                     scope 'liquidations'
       └─ olibia-web  /api/liquidations-proxy/[...path]
            │  inyecta Authorization: Bearer <Firebase ID token> desde cookie httpOnly
            └─ Back_Liq_olibia  /api/<path>       ← auth() valida y responde
```

- **No hay CORS**: el browser nunca llama directo a este backend.
- **El path es el contrato**: `app/api/X/route.ts` ⇄ `url: '/api/X'` en el `endpoints.ts` del front.
- **Errores**: el front acepta `{ error }` y `{ error, message }`, y prefiere `message` cuando
  está. Para rutas nuevas, seguir una de esas dos formas.
- **Binarios** (xlsx): el front no pasa por `doFetch`; arma la URL del proxy a mano y deja que el
  browser descargue. Hay que mandar `Content-Disposition` — el proxy lo reenvía.

---

## 3. Contrato vigente

Estado desde la perspectiva de este backend. `✅` = declarado y usado por el front ·
`⚠️` = declarado pero sin uso real · `❌` = existe acá y el front no lo conoce.

### Cargas

| Método | Ruta | Estado |
|--------|------|--------|
| GET | `/api/cargas` | ✅ `getCargas` |
| GET | `/api/cargas/estado-periodo` | ✅ `estadoPeriodo` |
| POST | `/api/cargas/preview` | ✅ `previewCarga` (multipart) |
| POST | `/api/cargas/preview-facturacion` | ⚠️ ver [§6](#6-pendientes-y-decisiones-abiertas) |
| POST | `/api/cargas/preview-xm-metabase` | ✅ `previewXmMetabase` |
| POST | `/api/cargas/check-previa` | ✅ `checkPrevia` |
| POST | `/api/cargas/confirmar` | ✅ `confirmarCarga` |
| GET | `/api/cargas/exportar-sdl` | ✅ `exportarSdlUrl` (xlsx) |
| GET | `/api/cargas/exportar` | ❌ export genérico de todas las fuentes, sin declarar en el front |

### Conciliaciones · Gestiones · Congruencia

| Método | Ruta | Estado |
|--------|------|--------|
| POST | `/api/conciliaciones/ejecutar` | ✅ |
| GET | `/api/conciliaciones/detalle` | ✅ |
| GET | `/api/conciliaciones/detalle-tc1` | ✅ |
| GET | `/api/conciliaciones/exportar` | ✅ (xlsx) |
| GET | `/api/conciliaciones/exportar-tc1` | ✅ (xlsx) |
| GET | `/api/gestiones` | ✅ |
| POST | `/api/gestiones/accionable` | ✅ |
| GET | `/api/gestiones/exportar` | ✅ (xlsx) |
| GET | `/api/congruencia` | ✅ |
| GET | `/api/congruencia/exportar` | ✅ (xlsx) |

### Cargos STR / NetSuite

| Método | Ruta | Estado |
|--------|------|--------|
| GET | `/api/cargos-str` | ✅ `cargosStrMatriz` |
| POST | `/api/cargos-str/netsuite/lote` | ✅ `crearLote` |
| GET | `/api/cargos-str/netsuite/lote/activo` | ✅ |
| GET | `/api/cargos-str/netsuite/lote/[loteId]` | ✅ |
| POST | `/api/cargos-str/netsuite/lote/[loteId]/procesar` | ✅ |
| POST | `/api/cargos-str/netsuite/lote/[loteId]/cancelar` | ✅ |
| GET | `/api/cargos-str/netsuite/lotes` | ✅ |
| GET | `/api/cargos-str/netsuite/estados` | ✅ |
| POST | `/api/cargos-str/netsuite/envio/[envioId]/reenviar` | ✅ |
| GET | `/api/cargos-str/netsuite/cron/limpiar-lotes` | — cron de Vercel, auth por `CRON_SECRET` |
| GET | `/api/cargos-str/meses` | ❌ sin declarar en el front |

**Estos endpoints tienen reemplazo en Go, ya trasvasado a bia-bills.** No son de
este repo: viven en el servicio `bia-bills` bajo su prefijo, y el front los
consume con `doFetch` (gateway BIA) y no con `doFetchLiquidations`.

| Método | Ruta en bia-bills | Estado |
|--------|-------------------|--------|
| GET | `/ms-bill/liquidations/health` | ✅ desplegado en dev |
| POST | `/ms-bill/liquidations/cargos-str/preview` | ✅ desplegado, consumido por `usePreviewStrGo` |
| POST | `/ms-bill/liquidations/cargos-str/confirm` | ✅ desplegado, consumido por `useConfirmarStrGo` |
| GET | `/ms-bill/liquidations/cargos-str` | ✅ desplegado, consumido por `useMatrizStrGo` |
| GET | `/ms-bill/liquidations/cargos-str/periods` | ✅ desplegado, consumido por `usePeriodosStrGo` |

**Los `/api/cargos-str` de este repo quedaron sin uso para la matriz.** El front
ya no los llama: STR entero pasa por Go. Siguen acá porque la fase 2 de NetSuite
todavía los necesita.

La fase 2 de NetSuite (los endpoints `/netsuite/...`) **no** se portó a Go: sigue
solo acá y sin desplegar.

### Catálogos, dashboard y administración

| Método | Ruta | Estado |
|--------|------|--------|
| GET | `/api/operadores` · PATCH `/api/operadores/[id]` | ✅ |
| GET, POST | `/api/usuarios` · PATCH `/api/usuarios/[id]` | ✅ |
| GET | `/api/periodos` | ✅ |
| GET | `/api/dashboard` | ✅ |
| GET | `/api/precio-bolsa` | ✅ |
| GET | `/api/proyeccion-cargos-or` | ✅ |
| GET | `/api/tarifas-sdl` | ✅ |
| GET | `/api/sdl-por-or` | ❌ sin declarar en el front |
| GET | `/api/str-por-or` | ❌ sin declarar en el front |
| GET | `/api/me` | — smoke test de la conexión (no lo consume la UI) |
| GET, POST | `/api/auth/[...nextauth]` | — login propio del backend, ajeno al front |

---

## 4. Piezas de este backend que sostienen la conexión

| Archivo | Qué expone | Para qué |
|---------|-----------|----------|
| [lib/auth.ts](./lib/auth.ts) | `auth()` | Sesión del request. Resuelve por cookie de NextAuth (dashboard propio) y, si no hay, por `Authorization: Bearer` (olibia). Devuelve `{ user: { id, nombre, rol }, source }`. **Todas las rutas la usan; no cambia su firma.** |
| [lib/auth-olibia.ts](./lib/auth-olibia.ts) | `bearerToken()` · `verifyOlibiaIdToken()` · `resolveOlibiaUser()` · `authFromAuthorizationHeader()` | Puente con olibia: parseo del header, verificación del Firebase ID token contra las llaves públicas de Google (RS256 + `iss`/`aud` + expiración), y mapeo email → `users` con chequeo de dominio y alta automática opcional. |
| [app/api/me/route.ts](./app/api/me/route.ts) | `GET /api/me` | Smoke test: devuelve la identidad resuelta y por qué camino entró (`source: "olibia"` = el puente funciona). |
| [lib/db.ts](./lib/db.ts) | `db` | Cliente Prisma (Supabase) con parámetros de pooling para serverless. |
| [lib/db-bia.ts](./lib/db-bia.ts) | `consultar()` · `consultarUno()` · `enTransaccion()` · `verificar()` | Acceso a las bases de BIA (`file-compiler`, `calculator-prices`) con `pg`. Prisma no sirve: es un datasource por cliente. |
| [lib/fuentes-migradas.ts](./lib/fuentes-migradas.ts) | `FUENTES_EN_BIA` · `estaMigrada()` | Qué fuentes de carga ya viven en BIA y cuáles siguen en Supabase. Lo consultan los endpoints que mezclan las siete. |

### Variables de entorno de la conexión

| Variable | Default | Efecto si falta o está mal |
|----------|---------|----------------------------|
| `FIREBASE_PROJECT_ID` | — | **401 en todo lo que venga del front.** Debe ser igual al `NEXT_PUBLIC_FIREBASE_PROJECT_ID` de olibia (dev: `bia-eva-dev`) |
| `OLIBIA_ALLOWED_EMAIL_DOMAINS` | `@bia.app` | Correos fuera del dominio → 401 |
| `OLIBIA_AUTO_PROVISION_USERS` | `true` | En `false`, quien no esté dado de alta en Administración recibe 401 |
| `OLIBIA_DEFAULT_ROL` | `ANALISTA` | Rol del alta automática |

---

## 5. Bitácora de cambios

Más reciente primero. Cada entrada: qué cambió, por qué, y qué implica para el otro repo.

### 2026-08-19 — Tarifas SDL: el área sale de los archivos, y ajustes de la tabla

**Backend.** El puerto de Tarifas SDL traía del motor TypeScript un mapa fijo de
operador → área de distribución, y estaba mal: las hojas "Cargos ADD" listan los
operadores de cada área y no coincidían con el mapa. EEP Pereira y EPM son de
Centro, EEP Cartago y EMCALI de Occidente, ENEL de Oriente. Pasó de 14 operadores
con área a **19 de 21**.

El mapa se eliminó. La pertenencia ahora se lee de los archivos, resuelta **por
nombre de mercado** — es lo único que distingue a los que comparten razón social.
El área y los cargos ADD se guardan para todo operador que figure en un ADD, sin
mirar su tipo: un operador tipo USO calcula su tarifa con su propio archivo Y
ADEMÁS pertenece a un área. Detalle completo en
[docs/backend/migracion-a-go.md](./docs/backend/migracion-a-go.md).

Verificado por HTTP contra las bases con los 33 archivos reales: 21 filas, 0
errores, y **210 tarifas comparadas contra el resultado anterior con 0
diferencias** — el cambio agrega datos y no toca ningún cálculo.

AFINIA y AIRE quedan sin área porque entre los 12 archivos no hay ninguno de un
área Caribe. El preview lo avisa; no es un error.

**Qué implica para el front.** Nada del contrato cambia: mismos endpoints, mismos
campos. `distribution_area` y `dt1_add/dt2_add/dt3_add` pasan de venir en `null`
para 7 operadores a venir con valor para 19. El fixture del test de contrato
(`preview.dev.json`) se regeneró contra el backend real.

**Front, en la misma tanda:**

- Tarifas SDL: encabezado en tres bloques —energía arriba, nivel y propiedad en una
  celda de dos líneas—, rótulos en inglés igual que las columnas de la base,
  importes con signo peso y dos decimales, y todo centrado. Tipografía unificada:
  el componente de tabla fija `text-label-xs` en los encabezados pero no fija nada
  en las celdas, así que el cuerpo heredaba 14px y se veía más grande que su propio
  encabezado.
- Cargos STR: `Billing month` rotula una sola vez, sobre la columna del operador, y
  desde ahí se entiende que los meses de las columnas son de **facturación** y no de
  consumo. Rótulos en inglés y matriz centrada.
- Modal de Crear OC: se cortaba por arriba y no se podía llegar al título ni a los
  botones. El overlay del componente de UI es `flex flex-col justify-center` con
  `overflow-y-auto`, y cuando el modal supera el alto de la pantalla el centrado
  recorta la parte de arriba sin que el scroll la alcance. Se acotó a `max-h-[85vh]`
  con scroll solo en el cuerpo. **Mismo riesgo abierto en `AccionableModal`.**

### 2026-08-17 — STR corre en dev sobre Go, y Supabase sale de su circuito

El módulo está **desplegado en desarrollo** (bia-bills, rama
`feat/liquidations-cargos-str`, sin PR a `main` todavía) y el front ya lo consume:
cargar archivos, previsualizar, confirmar y ver la matriz pasan por Go.

Verificado en dev: `{"ok":true,"bases":[file-compiler, calculator-prices]}`, y la
matriz de 2026-07 devolviendo 23 operadores por 1.403.159.917.

**Dos cosas que costaron encontrar y conviene no volver a pisar:**

1. **`services.dev.bia.app` no rutea `/ms-bill`.** Es el DNS que muestra cactus
   para el backend, pero el tráfico real entra por **`olibia.dev.bia.app`**, que es
   el `NEXT_PUBLIC_BACKEND_URL` del front. Probar contra el primero da 404 de
   Envoy con cuerpo vacío, y parece que el módulo no se desplegó. La forma
   confiable de probar es por el proxy del front —
   `<host-del-front>/api/proxy/ms-bill/liquidations/health`— que además inyecta el
   token.
2. **Todo `/ms-bill/*` está detrás de un middleware de auth que corta ANTES de
   rutear.** Una ruta inventada devuelve el mismo 400 que una que existe, así que
   ese 400 no prueba nada sobre si tu endpoint quedó registrado. Hace falta un
   token válido para distinguir.

**Por qué el front no puede depender de este repo para STR:** el backend
TypeScript no está desplegado en desarrollo. Sus llamadas caen al gateway BIA, que
no sirve `/api/...`. Por eso el flujo de STR en el front no usa los catálogos de
períodos ni de operadores: todo sale de la respuesta de cargos.

**Lo que quedó apagado, por decisión:** el historial de cargas y el estado del
período no reflejan las cargas de STR —leen de Supabase y este flujo ya no escribe
ahí—, y el lote de NetSuite espera ids de Supabase que la vista ya no maneja.

**Sin verificar:** el circuito desde la interfaz. Falta desplegar el front (rama
`feat/liquidations-str-ajustes`) y hacer una carga real.

### 2026-08-17 — Cargos STR se trasvasa a bia-bills (fase 3 del port a Go)

El módulo de Cargos STR ya vive en Go dentro de `bia-bills`, rama
`feat/liquidations-cargos-str`, commit `82ba2a1b`. Es el primer módulo que se
despliega desde ahí en vez de este repo.

**Por qué bia-bills y no acá:** cactus solo despliega repos de la organización, y
los backends de BIA son todos Go. En vez de pelear con permisos, el backend se
desarrolla acá y cada endpoint terminado se copia allá.

**Qué implica para el front:** las rutas migradas quedan bajo `/ms-bill/liquidations/...`
y se consumen con `doFetch`, no con `doFetchLiquidations`. Ya están declaradas en
su `endpoints.ts` (`strCargosGo`, `strPeriodosGo`, `strPreviewGo`, `strConfirmGo`,
`strHealthGo`) **pero todavía no se consumen**: la pantalla sigue pegándole a
`/api/cargos-str` de este repo. Se cambia cuando el servicio Go esté verificado en
dev, y es una línea por hook.

**Dos correcciones al plan que salieron de hacerlo:**

1. **No van migraciones en la cadena de bia-bills.** Su `MigrateDB()` corre contra
   la base *del servicio*; una migración `193_*` habría creado las tablas de
   Liquidaciones en la base equivocada. El DDL sigue en `sql/cargos-str/`, aplicado
   por fuera. El documento del port decía lo contrario y se corrigió.
2. **`go.mod` de bia-bills no cambió.** Todas las dependencias estaban ya, en la
   misma versión — se habían pineado así a propósito en la fase 0. El trasvase
   tocó **4 líneas** de su código: la llamada a `RegisterLiquidations(apiPrefix)`.

**Sin verificar todavía:** el servicio corriendo en dev. Falta el PR, y sobre todo
las variables `liq_db_*` en cactus (Environments) — sin ellas el servicio arranca
igual, pero `GET /ms-bill/liquidations/health` responde 503. Son las credenciales
de `file-compiler`/`calculator-prices`, no las del usuario de bia-bills.

Detalle del trasvase y cómo repetirlo para el próximo módulo:
[docs/backend/migracion-a-go.md](./docs/backend/migracion-a-go.md).

### 2026-08-16 — Cargos STR: nombres en inglés y una sola tabla por base

Dos decisiones del usuario, aplicadas juntas:

**Identificadores de base en inglés**, como el resto de las tablas de esas bases (`agents`,
`operators`, `operator_rates`). El vocabulario vuelve al que ya usaba `str_charges` de `bia-bi`:

| Antes | Ahora |
|---|---|
| `liquidaciones_cargos_str` (file-compiler) | `liquidations_str_inputs` |
| `liquidaciones_cargos_str` (calculator-prices) | `liquidations_str_charges` |
| `carga_id` · `periodo` · `or_codigo` · `or_nombre` | `load_id` · `period` · `operator_code` · `operator_name` |
| `valor_factura` · `refactura_N_valor` · `valor_a_pagar` | `invoice_amount` · `reinvoice_N_amount` · `amount_payable` |

Los nombres de tabla además distinguen qué guarda cada una; antes las dos se llamaban igual en
bases distintas. **El código de la aplicación sigue en español** y `lib/cargos-str.ts` traduce.

**Una sola tabla por base.** Se eliminaron las vistas `..._vigente`: cada tabla guarda lo vigente
*y* el histórico. La regla de "el registro más reciente por (`period`, `operator_code`)" pasó de la
base al código, en la constante `VIGENTES` de [lib/cargos-str.ts](./lib/cargos-str.ts).

⚠️ **Con esto, una consulta que vaya directo a la tabla sin aplicar esa regla suma cargas viejas con
nuevas y duplica los montos.** Hoy no puede pasar porque todos los endpoints entran por
`lib/cargos-str.ts`. Si alguna vez se consultan estas tablas desde afuera de la app —un reporte, un
BI— hay que envolver la regla en una vista antes.

**Migración:** las 46 filas de las pruebas se copiaron a las tablas nuevas y se borraron las viejas
junto con sus vistas. En cada base queda una sola relación.

**Verificado contra las bases reales:** dos cargas del mismo período dejan 4 filas en la tabla, la
lectura devuelve 2 (las de la carga más nueva) y el total del período da 1.998 y no 2.198 — que es
exactamente el error que la vista prevenía y ahora previene el código.

### 2026-08-16 — Cargos STR: paso 1 de la migración a bases de BIA (tablas creadas)

**Contexto:** App_Liquidaciones deja de desplegarse en Vercel sobre Supabase y pasa a vivir dentro
del ecosistema de olibia, contra las bases de BIA, **migrando módulo por módulo**. STR es el primero.
`Back_Liq_olibia` sigue siendo un servicio aparte que el front consume — el proxy y el puente de auth
son arquitectura definitiva, no andamio.

**Aplicado** en el RDS de dev (`c4-rds-bia-dev`), con el usuario `liquidaciones_dev`:

**Solo dos tablas, ningún catálogo nuevo:**

| Base | Objeto | Qué guarda |
|---|---|---|
| `file-compiler` | `public.liquidaciones_cargos_str` (+ `_vigente`) | Insumo crudo: `carga_id`, período, operador, factura y hasta 3 ajustes por separado |
| `calculator-prices` | `public.liquidaciones_cargos_str` (+ `_vigente`) | Resultado: `carga_id`, período, operador, nombre legal, `valor_a_pagar` |

**Tres decisiones de diseño:**

- **Append-only.** Nada se reemplaza ni se borra: cada cargue inserta con su `carga_id` y su
  `created_at`, y la lógica consume el más reciente por (período, operador). La unicidad es
  `(carga_id, or_codigo)`. La regla de lectura vive en las vistas `..._vigente` — **los endpoints
  deben consultar la vista, no la tabla**.
- **La homologación se resuelve contra `public.agents`** (catálogo de agentes de XM, ya existente en
  file-compiler), sin crear tabla de configuración. Las 24 abreviaturas de los archivos BalanceSTR
  están las 24 ahí con su nombre legal, incluidos DISPAC, ENERGUAVIARE y PUTUMAYO, que **no** existen
  en `public.operators`. El nombre a mostrar de un OR sale del agente del grupo con
  `activity = 'OPERADOR DE RED'` — verificado: hay exactamente uno por operador en los 23. La
  agrupación (`CSID` + `CSSD` → AIRE) **no es deducible** de `agents` porque no comparten NIT, así
  que sigue en el diccionario `HOMOLOGACION` de `lib/parsers/insumos-str.ts`. Costo asumido: cambiar
  un operador requiere deploy.
- **Se descartó tocar `public.operators`** de calculator-prices: es una tabla de otro producto, su
  grano es operador × región (EEP aparece 3 veces), le faltan 3 de nuestros operadores, y
  `liquidaciones_dev` no tiene propiedad para alterarla.

**Sin destino todavía:** `netsuite_vendor_id`, que hoy vive en `ConfiguracionOR` de Supabase. Se
define en la fase 2 junto con el resto de la integración NetSuite.

**Verificado:** las 24 abreviaturas resuelven contra `public.agents`; los 23 operadores obtienen su
nombre legal; AIRE agrupa `CSID`+`CSSD` tomando el nombre del registro vigente y no del intervenido.
Además, los datos que hoy están en `bia-bi.liquidations_str.str_charges` coinciden **exactamente**
con lo que produce el parser de este backend (ver entrada del 2026-08-14).

**Pasos 2 y 3 (mismo día):**

- **Nuevo** [lib/db-bia.ts](./lib/db-bia.ts): capa de acceso a las dos bases con `pg` (Prisma no
  sirve — es un datasource por cliente). Pools singleton, transacción por base, y `verificar()` para
  diagnóstico. Es infraestructura del programa: la van a usar las fuentes que se migren después.
- **Nuevo** [lib/fuentes-migradas.ts](./lib/fuentes-migradas.ts): un único lugar que declara qué
  fuentes ya viven en BIA. Los endpoints que mezclan las siete (historial, estado-período, dashboard,
  proyección) consultan esto en vez de repetir la decisión. Migrar la próxima fuente es una línea.
- **Nuevo** [app/api/health/route.ts](./app/api/health/route.ts): estado de las tres bases. 200 si
  responden, 503 si alguna falla. Autenticado como el resto.
- **Nuevo** [lib/agentes-str.ts](./lib/agentes-str.ts): resuelve el nombre legal del operador contra
  `public.agents`. Si el catálogo no responde, el preview se muestra igual con una alerta — lo que el
  usuario valida son los montos.
- [lib/parsers/insumos-str.ts](./lib/parsers/insumos-str.ts) reescrito: el período sale del filtro
  (se eliminó la detección por nombre de archivo) y la salida es **una fila por operador** con
  `valor_factura`, `refactura_1/2/3_valor` y `valor_a_pagar` — la misma forma de la tabla destino, así
  lo que se valida en pantalla es lo que se guarda. Los ajustes se ordenan por el mes de su nombre; un
  cuarto archivo **corta la carga con error explícito** en vez de descartarse.
- [confirmar](./app/api/cargas/confirmar/route.ts) adaptado a la nueva forma. **Compatible hacia
  atrás**: `registros_str` pasa a recibir una fila por operador en vez de tres, y como todos sus
  consumidores suman `valor_cop` agrupando por (período, OR), el resultado no cambia. El desglose
  queda en `detalle_json`.

**Verificado:** `tsc` limpio · 152 tests en verde · el test dorado sigue dando CHEC = 70.812.140 /
−4.442 / **70.807.698** y el total del lote 1.460.833.304 · `db-bia` conecta a las dos bases y
`agents` resuelve los **23** nombres legales, eligiendo para AIRE el registro vigente y no el
intervenido.

**Paso 5 — EL CORTE (mismo día). Los datos de STR ya no pasan por Supabase.**

- **Nuevo** [lib/cargos-str.ts](./lib/cargos-str.ts): reemplaza a `registros_str`. Lectura contra las
  vistas `..._vigente` (nunca contra la tabla base, que devuelve el historial completo) y escritura
  en las dos bases.
- [confirmar](./app/api/cargas/confirmar/route.ts): la rama `INSUMOS_STR` ya **no escribe
  `registros_str`**. Escribe en file-compiler y calculator-prices **fuera** de la transacción de
  Prisma —son otras bases— y si esa escritura falla marca la carga como `ERROR` y responde 502: mejor
  que figure fallida a que el historial diga "completada" sin datos detrás.
- [/api/cargos-str](./app/api/cargos-str/route.ts), [meses](./app/api/cargos-str/meses/route.ts) y
  [/api/str-por-or](./app/api/str-por-or/route.ts) leen de calculator-prices. El front sigue mandando
  los CUID de Supabase, así que se traducen a `"AAAA-MM"` y código de OR; esa traducción desaparece
  cuando los filtros de la pantalla dejen de venir de Supabase (paso 6).
- [/api/dashboard](./app/api/dashboard/route.ts) y
  [/api/proyeccion-cargos-or](./app/api/proyeccion-cargos-or/route.ts): su agregado STR también, para
  que ninguna pantalla muestre cero mientras otra muestra el total.
- `FUENTES_EN_BIA` ya incluye `INSUMOS_STR`.

**Orden de escritura y qué pasa si falla.** No hay transacción distribuida: primero file-compiler,
después calculator-prices. Si la segunda falla, se borran las filas de la primera por `carga_id` —no
es "reemplazar historial", es limpiar una carga que nunca llegó a existir— así nunca queda un insumo
sin resultado ni un resultado a medias visible en la matriz.

**Verificado end-to-end contra las bases reales** (con un período de prueba, borrado al terminar):

| Paso | Resultado |
|---|---|
| Parser | 23 operadores |
| Guardado | 23 filas en cada base |
| file-compiler | suma de facturas 1.462.224.971 |
| calculator-prices (vista vigente) | 23 operadores, total **1.460.833.304** |
| CHEC | `CENTRAL HIDROELECTRICA DE CALDAS S.A. E.S.P.` = **70.807.698** |
| Agregado por período (dashboard/proyección) | 1.460.833.304 |
| Append-only | una segunda carga del mismo período no pisa: la vista devuelve la nueva, la tabla conserva las 46 filas |

**Implicancias para olibia-web:** el preview de STR cambia de forma — pasa de 69 filas (23 operadores
× 3 archivos) a **23 filas** con columnas *Operador · Nombre · Factura · Ajuste 1/2/3 · Total*.
Acordado con el usuario. La tabla del wizard las renderiza genéricamente, pero conviene revisarla.
La matriz de Cargos STR ahora muestra el **nombre legal** del operador en vez del código corto.

### 2026-08-14 — Cargas STR: Fase 1 validada contra el endpoint de este backend

**Contexto:** la sesión "Back Insumos STR" (olibia-web, `9dcef04a`) construyó
[services/liquidations-str-ingest/](./services/liquidations-str-ingest/) como servicio aparte —
dev-server Express en :4000 + schema `liquidations_str` en la BD `bia-bi` (RDS)— y dejó su Fase 2
(NetSuite) como pendiente. Al conectar los repos se ve que **este backend ya implementa la cadena
STR completa**: `TipoFuente.INSUMOS_STR` en preview/confirmar, `RegistroSTR` → `registros_str`, y la
Fase 2 ya construida (`EnvioNetsuiteCargoSTR`, `lib/integrations/netsuite/service.ts`, lotes
`/api/cargos-str/netsuite/*` que el front ya consume con 9 hooks). El parser del servicio es una
copia de [lib/parsers/insumos-str.ts](./lib/parsers/insumos-str.ts).

**Decisión:** no se construye el Lambda. El cargue de insumos STR queda como endpoint de este
backend — el que ya existe.

**Validación (este cambio):** [lib/parsers/__tests__/insumos-str.test.ts](./lib/parsers/__tests__/insumos-str.test.ts)
corre el parser de este backend contra los archivos reales de `archivos_ejemplo/STR/` (factu
2026-MAY + refactu 2025-NOV + refactu 2026-MAR) y reproduce el número de referencia que la otra
sesión validó end-to-end con el servicio aparte:

| | Factura | Refacturas | A pagar |
|---|---|---|---|
| CHEC | 70.812.140 | −4.442 | **70.807.698** ✅ |

23 operadores, 69 filas, total del lote **1.460.833.304**, sin errores críticos. Todo el lote queda
etiquetado `2026-05` (mes del archivo factu), que es la regla de negocio confirmada. El test se
saltea solo si los archivos de ejemplo no están (son insumos reales, no versionados).

**Por qué esto primero:** la rama `INSUMOS_STR` de `/api/cargas/preview` **no toca la BD** — es
`parsearInsumosSTR(buffers, anio, mes)` y nada más. Así que el número queda validado con
independencia de dónde vayan a vivir los datos, y sigue siendo válido después del cambio de bases
que viene. El `confirmar` sí escribe, y queda a la espera de esa decisión.

**Implicancias para olibia-web:** el front no cambia (ya manda `INSUMOS_STR` multiFile a
`/api/cargas/preview`), pero **hay que liberar el puerto 4000**: hoy el dev-server del servicio STR
y este backend se lo disputan, y el que gane sirve *todos* los `/api/cargas/*`. Con el dev-server
ahí, periodos/operadores/historial le llegan al front como stubs.

### 2026-08-14 — Puente de autenticación con olibia-web

**Problema:** las rutas de este backend estaban completas y los paths ya coincidían con lo que el
front esperaba, pero `auth()` resolvía la sesión **solo** desde la cookie de NextAuth. El front
manda un Firebase ID token en `Authorization: Bearer` y no tiene esa cookie → 401 en todo.

**Cambios en este repo:**

- **Nuevo** `lib/auth-olibia.ts`: verificación del Firebase ID token y mapeo a `users`.
- `lib/auth.ts`: `auth()` ahora compone cookie de NextAuth → Bearer de olibia. Agrega
  `source: "credentials" | "olibia"` a la sesión. Las 42 rutas quedan intactas.
- **Nuevo** `app/api/me/route.ts`: smoke test de la conexión.
- `package.json`: `dev` pasa a `next dev -p 4000` (antes chocaba con el 3000 del front) y se agrega
  `jose` como dependencia explícita.
- `.env.example` y `.env`: `FIREBASE_PROJECT_ID`, `OLIBIA_ALLOWED_EMAIL_DOMAINS`,
  `OLIBIA_AUTO_PROVISION_USERS`, `OLIBIA_DEFAULT_ROL`.
- **Nuevo** `lib/__tests__/auth-olibia.test.ts`: 14 tests del puente, sin red ni DB.
- **Nuevo** `docs/backend/conexion-olibia-web.md`: mecanismo, envs, runbook y troubleshooting.

**Implicancias para olibia-web:** ninguna funcional — el front ya estaba correcto. Solo se
corrigieron comentarios desactualizados en su `.env.example` y en el proxy.

**Decisión tomada:** el auto-provisioning queda **activado**. Cualquier `@bia.app` autenticado en
olibia entra como `ANALISTA` en su primer request. Es lo que hace usable la conexión sin sembrar
usuarios a mano; si se prefiere alta manual, `OLIBIA_AUTO_PROVISION_USERS=false`.

**Verificado:** `tsc --noEmit` limpio · suite completa 143 tests en verde · backend levantado en
:4000 respondiendo 401 limpio sin token y con token inválido. **No verificado end-to-end con un
token real de Firebase** (requiere login en browser): la comprobación pendiente es
`GET http://localhost:3000/api/liquidations-proxy/api/me` → 200 con `source: "olibia"`.

---

## 6. Pendientes y decisiones abiertas

- **Tarifas SDL — probado en local, falta el trasvase a bia-bills.** El módulo corre contra las
  bases de BIA desde el arnés local (`cmd/liquidations-dev`, puerto 4110) y el front lo consume en
  localhost. Falta pasar los archivos a bia-bills y desplegar en cactus. **El trasvase incluye el
  arreglo del área de distribución de esta tanda y el de notación científica de `toNum` en STR.**
- **AFINIA y AIRE sin área de distribución.** No figuran en ninguna hoja "Cargos ADD": entre los 12
  archivos del lote no hay ninguno de un área Caribe. Verificado archivo por archivo. El preview lo
  avisa. Si XM publica el ADD de Caribe, se suman esos 3 archivos y se llena solo; si en cambio
  pertenecen a una de las cuatro áreas existentes, hay que confirmarlo con el negocio.
- **El slot de develop en cactus es uno solo y gana el último deploy.** Un deploy de otra rama pisa
  el nuestro sin avisar. Ya pasó una vez y costó una sesión de diagnóstico. Coordinar antes de
  desplegar, o conseguir otro ambiente.
- **`AccionableModal` tiene el mismo defecto que tenía el modal de Crear OC**: ancho fijo y sin alto
  máximo. Si le entra una lista larga se va a cortar por arriba y no se va a poder cerrar. No se
  tocó porque no estaba en el alcance.
- **El badge del historial de cargas imprime la clave cruda** (`INSUMOS_TARIFAS_SDL`) al lado de
  fuentes que sí muestran etiqueta. El diccionario ya tiene el rótulo; es una línea.

- **Cargos STR en Go — falta probarlo desde la interfaz.** El backend corre en dev y el front ya lo
  consume, pero nadie hizo todavía una carga real por pantalla. Falta desplegar el front (rama
  `feat/liquidations-str-ajustes`) y hacer el circuito completo. **Ojo**: confirmar desde la
  interfaz escribe filas de verdad en `file-compiler` y `calculator-prices`, y el modelo es
  append-only — no se deshace desde la pantalla. Conviene probar con un período sin datos.
- **El PR a `main` de bia-bills está pendiente a propósito.** La rama
  `feat/liquidations-cargos-str` está desplegada en dev directo, sin merge, hasta validar.
- **El historial de cargas y el estado del período no ven las cargas de STR.** Consecuencia
  aceptada de sacar Supabase del circuito: viven ahí y este flujo ya no escribe. Se resuelve cuando
  se migre el módulo de Cargas completo, porque son de todas las fuentes y no solo de STR.
- **El lote de NetSuite quedó incompatible con la matriz.** Sus endpoints esperan ids de Supabase
  (CUID) y la vista ahora maneja `"2026-07"` y códigos de operador. En dev falla antes de llegar
  porque vive en este backend, que no está desplegado. Hay que resolverlo al migrar NetSuite.
- **Convivencia de dos backends.** Mientras se migra módulo por módulo, el front tiene endpoints en
  las dos formas: `/api/...` por el proxy de App_Liquidaciones y `/ms-bill/liquidations/...` por el
  gateway BIA. No es transitorio de un día — va a durar todo lo que dure el port, así que conviene
  que cada endpoint diga en el `endpoints.ts` de qué lado está.
- **Cargas STR — consolidación en curso.** Fase 1 ya validada contra este endpoint (ver §5). Falta:
  liberar el **:4000** (hoy lo disputa el dev-server del servicio STR); mover la homologación de
  columnas del `HOMOLOGACION` hardcodeado del parser a una columna nueva en `ConfiguracionOR`
  (donde ya vive `netsuite_vendor_id`), portando el seed de 23 operadores de
  `services/liquidations-str-ingest/sql/002_seed_operators.sql`; y archivar el servicio dejando su
  SQL y `docs/PROCESO.md` como referencia.
- **Migración de STR a bases de BIA — pasos 1 a 5 hechos** (ver §5). Los datos de STR ya no pasan por
  Supabase. Lo que falta:
  (4) migrar el histórico de `registros_str`, pendiente de validar con el usuario; el modelo
  append-only lo hace indoloro y se puede hacer en cualquier momento;
  (6) los filtros de la pantalla de Cargos STR siguen mandando CUID de Supabase y los endpoints los
  traducen — sacarlos de ahí elimina esa traducción;
  (7) el historial de cargas y el estado del período todavía se escriben y leen en Supabase
  (`cargas_fuente`, `log_auditoria`): es el andamio que mantiene vivas esas dos pantallas hasta que se
  muevan a file-compiler;
  (8) limpieza — sacar `RegistroSTR` de Prisma, archivar `services/liquidations-str-ingest`, liberar
  el :4000.
- **Fase 2 (NetSuite), sin empezar.** `lib/integrations/netsuite/service.ts` **sigue leyendo
  `registros_str` de Supabase**, que a partir de ahora queda vacío para cargas nuevas. Hasta que se
  migre, el armado de lotes no va a encontrar datos de los períodos cargados con el flujo nuevo.
  También hay que decidir dónde vive `netsuite_vendor_id`.
- **Permisos pendientes (no bloquean).** `liquidaciones_dev` no puede crear schemas en esas bases,
  por eso las tablas viven en `public` con prefijo `liquidaciones_`. Si conceden
  `GRANT CREATE ON DATABASE ... TO liquidaciones_dev`, mover las tablas a un schema `liquidaciones`
  es un `ALTER TABLE ... SET SCHEMA`, instantáneo y sin pérdida de datos.
- **`preview-facturacion` está duplicado.** Este backend tiene `POST /api/cargas/preview-facturacion`
  (con `lib/integrations/bia-bills.ts` y `lib/parsers/facturacion-bia-bills.ts`), y el front tiene
  su propia ruta server-side `/api/liquidations/facturacion-bia` que llama a bia-bills directo. El
  hook `usePreviewFacturacion` usa la del front; la entrada `previewFacturacion` de su `endpoints.ts`
  quedó declarada sin uso. **Hay que elegir un solo dueño de esa integración.**
- **Endpoints sin consumir**: `/api/cargas/exportar`, `/api/cargos-str/meses`, `/api/sdl-por-or`,
  `/api/str-por-or`. Existen acá y el front no los declara.
- **Prefijo en producción**: si este servicio se monta detrás del gateway BIA (tipo
  `/ms-liquidaciones/...`), el cambio del lado del front es solo en su `endpoints.ts`. Conviene
  decidirlo antes de sumar más rutas.
- **Alta de usuarios**: revisar si `ANALISTA` es el rol correcto por defecto para quien entra desde
  olibia, o si conviene `CONSULTA`.

---

## 7. Cómo actualizar este archivo

Actualizalo **en el mismo commit** que el cambio, no después.

- **Agregaste o cambiaste un endpoint** → fila en la tabla de [§3](#3-contrato-vigente) con su estado
  real, y avisá en el espejo del front si ya lo consume.
- **Agregaste una función o módulo que sostiene la conexión** (auth, proxy, integraciones
  compartidas) → fila en [§4](#4-piezas-de-este-backend-que-sostienen-la-conexión).
- **Cambio con impacto en el otro repo, o una decisión que costó tomar** → entrada nueva arriba de
  todo en [§5](#5-bitácora-de-cambios), con fecha, qué cambió, por qué, qué implica del otro lado y
  qué quedó sin verificar.
- **Env nueva de la conexión** → tabla de [§4](#4-piezas-de-este-backend-que-sostienen-la-conexión)
  y `.env.example`.
- **Lo que descubrís que no funciona o queda a medias** → [§6](#6-pendientes-y-decisiones-abiertas).
  Un pendiente anotado vale más que uno recordado.

Dos reglas para que no se pudra:

1. **No dupliques el mecanismo acá.** El cómo funciona la auth vive en
   [docs/backend/conexion-olibia-web.md](./docs/backend/conexion-olibia-web.md); este archivo dice
   *qué hay* y *qué pasó*.
2. **Registrá lo que no verificaste.** Si algo quedó sin probar end-to-end, escribilo — es la
   información que más se pierde entre sesiones.
