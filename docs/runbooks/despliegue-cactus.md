# Runbook — Desplegar el backend en cactus

> **Última actualización:** 2026-08-16
> **Aplica a:** ambiente de desarrollo. El backend deja de desplegarse en Vercel.

Cactus es la consola de despliegues de BIA (`console.c4c7ops.com`). Este runbook
cubre el alta del backend de Liquidaciones como servicio.

---

## Antes de empezar

Lo que el repo ya tiene listo:

| Pieza | Estado |
|---|---|
| [`Dockerfile`](../../Dockerfile) | Multi-stage sobre Node 20, con `prisma generate` y los engines de Prisma copiados a la imagen |
| [`.dockerignore`](../../.dockerignore) | Deja fuera secretos, datos de ejemplo y lo específico de Vercel |
| `output: 'standalone'` en [`next.config.ts`](../../next.config.ts) | Requisito del Dockerfile |
| [`GET /api/live`](../../app/api/live/route.ts) | Health check **sin autenticación** y sin tocar la base |

Se usa el build **`dockerfile`**. Los buildpacks de Node del desplegable
(`node-12`, `node-14`, `node-16`) no sirven: Next 15 exige Node ≥ 18.18.

---

## 1. Alta del servicio (Backends → New)

| Campo | Valor | Por qué |
|---|---|---|
| **Name** | `liquidaciones-backend` | |
| **Type** | Service | |
| **Repository** | `git@github.com:Eriramirez-liq/Back_Liq_olibia.git` | No necesita estar en la organización |
| **Access type** | Private | El repo no es público; hay que darle acceso de deploy |
| **Routing type** | Path | Igual que `bia-bills` y `bia-file-compiler` |
| **Prefix path** | ver §2 | |
| **Health check path** | `/api/live` | **No usar `/`**: la raíz redirige al login del dashboard propio |
| **Port** | `8080` | El Dockerfile fija `ENV PORT=8080` |
| **Build** | `dockerfile` | |

---

## 2. El prefijo de ruta — la decisión que hay que tomar

Los servicios de BIA sirven sus rutas **con el prefijo incluido**: `bia-bills`
responde en `/ms-bill/billing-variables` y lo declara en su propio router. Cactus
enruta el prefijo pero **no lo recorta**.

Hay dos caminos:

**A. El servicio tiene su propio host** (prefix path `/`). No hay que tocar nada
más. El front apunta:

```
NEXT_PUBLIC_LIQUIDATIONS_BACKEND_URL=https://<host-del-servicio>
```

**B. Comparte host bajo un prefijo** (ej. `/ms-liquidations`). Entonces Next
tiene que servir bajo ese prefijo, y como `basePath` se resuelve en **build**,
hay que pasarlo como build-arg del Dockerfile:

```
NEXT_PUBLIC_BASE_PATH=/ms-liquidations
```

y el health check pasa a ser `/ms-liquidations/api/live`. El front apunta:

```
NEXT_PUBLIC_LIQUIDATIONS_BACKEND_URL=https://<host>/ms-liquidations
```

En los dos casos **`endpoints.ts` del front no se toca**: su proxy concatena el
host con el path del endpoint.

---

## 3. Variables (Environments → develop → Set up variables)

Se pegan como `KEY=VALUE`, una por línea. Los valores reales **no van en este
archivo ni en el repo**.

```
# ── Supabase (lo que todavía no se migró) ──────────────────────────────
DATABASE_URL=
SUPABASE_CONNECTION_LIMIT=5

# ── Bases de BIA (módulo de Cargos STR) ────────────────────────────────
DB_HOST=
DB_PORT=5432
DB_USER2=
DB_PASSWORD2=
FILE_COMPILER_DB_NAME=file-compiler
CALCULATOR_PRICES_DB_NAME=calculator-prices
BIA_DB_POOL_MAX=5

# ── Autenticación ──────────────────────────────────────────────────────
# Mismo proyecto de Firebase que el NEXT_PUBLIC_FIREBASE_PROJECT_ID del front.
FIREBASE_PROJECT_ID=bia-eva-dev
OLIBIA_ALLOWED_EMAIL_DOMAINS=@bia.app
OLIBIA_AUTO_PROVISION_USERS=true
OLIBIA_DEFAULT_ROL=ANALISTA
# Secreto de NextAuth para el login propio del dashboard. Generar con:
#   node -e "console.log(require('crypto').randomBytes(32).toString('base64'))"
AUTH_SECRET=

# ── Integraciones ──────────────────────────────────────────────────────
# NO cargar NETSUITE_ENABLED: la fase 2 va cerrada. Ver §3.1.
BIA_BILLS_API_URL=
METABASE_API_KEY=
```

### 3.1 · Fase 1 solamente: qué queda fuera

El backend es una sola app Next, así que desplegarla publica sus 44 rutas — no
hay forma de publicar un subconjunto. Para que la **fase 2 (NetSuite) no esté
disponible**, sus nueve rutas están detrás de `NETSUITE_ENABLED`, que está
**cerrado por defecto**: no cargar la variable deja la integración apagada.

| Ruta | Con la fase 2 cerrada |
|---|---|
| `POST /netsuite/lote`, `/procesar`, `/cancelar`, `/reenviar` | 503 `NETSUITE_NO_DISPONIBLE` |
| `GET /netsuite/lote/[id]`, `/cron/limpiar-lotes` | 503 |
| `GET /netsuite/lote/activo` | 204 — "no hay lote activo" |
| `GET /netsuite/estados` | `{}` |
| `GET /netsuite/lotes` | `{ lotes: [] }` |

Las tres últimas responden "no hay nada" en vez de error **a propósito**: son las
que la pantalla de Cargos STR consulta sola al abrirse. Si devolvieran 503, la
pestaña que sí queremos usar mostraría errores.

En la UI el botón **Crear OC** va a seguir visible; si alguien lo pulsa recibe el
503 con el mensaje de que la integración no está desplegada. No se crea ninguna
orden de compra, ni real ni simulada.

Cuando llegue la fase 2, se abre con `NETSUITE_ENABLED=true` más las
`NETSUITE_*` que correspondan.

### 3.2 · Alcance del ambiente: solo lo validado

De lo que se probó de punta a punta contra este backend hay **una sola cosa**: el
cargue de insumos STR y la visualización de su resultado.

El resto de los endpoints —conciliaciones, gestiones, congruencia, tarifas SDL,
administración— **no es código nuevo**: es el mismo que ya corría en Vercel. Se
despliega igual, pero el front **no expone sus pestañas**: en olibia,
`LIQUIDATIONS_TAB_KEYS_VISIBLES` deja visibles solo **Cargas** y **Cargos STR**.
Habilitar la siguiente es agregar una línea a esa lista cuando se valide.

Por eso `METABASE_API_KEY` y `BIA_BILLS_API_URL` quedan sin cargar: las fuentes
**Facturación BIA** y **XM/CGM** no se probaron contra este despliegue. La
consecuencia es que, dentro de la pestaña Cargas, el wizard **sigue ofreciendo
las ocho fuentes** y elegir esas dos falla. Si molesta, se recorta `FUENTES` en
el front.

Notas sobre algunas:

- **`AUTH_SECRET`** no existe hoy en el `.env` local porque en desarrollo
  NextAuth lo tolera. **En producción es obligatorio** y el servicio no arranca
  sin él.
- **`METABASE_API_KEY`** falta hoy en local, y por eso `/api/precio-bolsa`
  devuelve 502 y la proyección no trae la demanda. Conviene cargarla.
- **`DB_USER2`/`DB_PASSWORD2`** son de un usuario distinto al de `bia-bi`: ese no
  tiene permisos en `file-compiler` ni en `calculator-prices`.
- **`SUPABASE_CONNECTION_LIMIT`**: 1 era el valor de serverless. Fuera de Vercel,
  con un proceso largo, 1 serializa todo y `estado-periodo` muere contra el
  timeout del pool.

---

## 4. El cron que se queda sin plataforma

> En un despliegue de fase 1 esto **no aplica todavía**: el endpoint de limpieza
> está cerrado junto con el resto de NetSuite. Queda documentado para cuando se
> abra la fase 2.

`vercel.json` declara un cron diario que limpia lotes de NetSuite colgados:

```
/api/cargos-str/netsuite/cron/limpiar-lotes   0 6 * * *
```

Fuera de Vercel **no lo dispara nadie**. Hay que recrearlo en la sección
**Jobs** de cactus, llamando a ese endpoint con el header
`Authorization: Bearer <CRON_SECRET>`, que es lo que el endpoint valida.

Mientras no exista, un lote que quede en `EN_PROGRESO` por un fallo no se libera
solo y hay que cancelarlo a mano — ver
[netsuite-lote-colgado.md](./netsuite-lote-colgado.md).

---

## 5. Después de desplegar

1. **Liveness**: `GET https://<host>/api/live` → `200 {"ok":true}` sin token.
2. **Conexión a las bases**: con sesión iniciada en olibia,
   `https://<front>/api/liquidations-proxy/api/health` → 200 y las tres bases en
   `ok: true`. Si `file-compiler` o `calculator-prices` fallan, el servicio no
   está dentro de la red que alcanza el RDS.
3. **Identidad**: `.../api/liquidations-proxy/api/me` → 200 con `source: "olibia"`
   confirma que el puente de autenticación funciona.
4. **Apuntar el front**: cargar `NEXT_PUBLIC_LIQUIDATIONS_BACKEND_URL` en las
   variables del front y **reconstruir su imagen** — al llevar prefijo
   `NEXT_PUBLIC_`, el valor se hornea en el build y no se puede cambiar en
   runtime.

---

## Cosas que van a fallar si se saltan

| Síntoma | Causa |
|---|---|
| El servicio queda "unhealthy" y no recibe tráfico | Health check apuntando a `/` o a `/api/health`, que exige sesión |
| Arranca y muere en la primera consulta | Falta `AUTH_SECRET`, o los engines de Prisma no llegaron a la imagen |
| Todo responde 401 desde el front | `FIREBASE_PROJECT_ID` distinto al del front |
| Los endpoints de Cargos STR fallan y el resto anda | El servicio no alcanza el RDS privado |
| El front sigue apuntando al backend viejo | Se cargó la variable pero no se reconstruyó su imagen |
