# Conexión con olibia-web (front del módulo de Liquidaciones)

> **Audiencia:** quien trabaje el backend de liquidaciones sabiendo que el front lo consume.
> **Última actualización:** 2026-08-14
>
> Este doc explica **cómo funciona** el mecanismo. El inventario de endpoints, la bitácora de
> cambios y los pendientes viven en [`INTEGRACION.md`](../../INTEGRACION.md) (raíz del repo).

---

## Resumen

`olibia-web` es el front del módulo de Liquidaciones; este repo (`Back_Liq_olibia`) es su backend.
Son dos apps Next separadas, conectadas por un **proxy server-side** que vive en el front:

```
Browser
  └─ doFetchLiquidations (scope 'liquidations')
       └─ olibia-web: /api/liquidations-proxy/[...path]      ← inyecta Authorization
            └─ Back_Liq_olibia: /api/<path>                  ← valida y responde
```

Consecuencias prácticas:

- **No hay CORS**: el browser nunca habla directo con este backend.
- **El path es el contrato**: `app/api/X/route.ts` acá ⇄ `url: '/api/X'` en
  `src/modules/finance/liquidations/data/endpoints.ts` del front. Un endpoint nuevo no existe
  para el front hasta que se declara en ese archivo.
- El front tolera dos convenciones de error (`{ error }` y `{ error, message }`) y prefiere
  `message` cuando está. Para errores nuevos, seguir alguna de las dos.

---

## Autenticación

El usuario se autentica en olibia con **Firebase**. El front guarda el ID token en una cookie
httpOnly y su proxy lo reenvía como `Authorization: Bearer <idToken>`.

`lib/auth.ts` expone un único `auth()` que resuelve la sesión por dos caminos, en orden:

| Orden | Camino | Para qué |
|-------|--------|----------|
| 1 | Cookie de NextAuth (credenciales) | El `/login` y dashboard propios de este backend |
| 2 | `Authorization: Bearer <Firebase ID token>` | Todo el consumo desde olibia-web |

El camino 2 lo implementa `lib/auth-olibia.ts`:

1. Verifica firma RS256 contra las llaves públicas de Google
   (`.../jwk/securetoken@system.gserviceaccount.com`), más `iss`, `aud` y expiración.
2. Toma `email` y `sub` del token.
3. Valida el dominio del correo (`OLIBIA_ALLOWED_EMAIL_DOMAINS`).
4. Busca el `users` local por email. Si no existe y el auto-provisioning está activo, lo crea
   con `OLIBIA_DEFAULT_ROL` y un password aleatorio inservible (ese usuario **no** puede entrar
   por el login de credenciales; su identidad vive en Firebase).

Las rutas no cambian: `const session = await auth()` sigue devolviendo `{ user: { id, nombre, rol } }`.
Ahora además trae `source: "credentials" | "olibia"` por si hace falta distinguir el origen.

Un 401 desde acá no rompe la sesión del front: `doFetchLiquidations` refresca el ID token con
`getIdToken(true)` y reintenta una vez.

---

## Variables de entorno

| Variable | Dónde | Valor |
|----------|-------|-------|
| `FIREBASE_PROJECT_ID` | este repo | Igual a `NEXT_PUBLIC_FIREBASE_PROJECT_ID` del front (dev: `bia-eva-dev`). **Sin esto, todo request del front da 401.** |
| `OLIBIA_ALLOWED_EMAIL_DOMAINS` | este repo | Coma-separados. Default `@bia.app` |
| `OLIBIA_AUTO_PROVISION_USERS` | este repo | `true` (default) crea el usuario en su primer ingreso; `false` exige alta previa en Administración |
| `OLIBIA_DEFAULT_ROL` | este repo | Rol del alta automática. Default `ANALISTA` |
| `NEXT_PUBLIC_LIQUIDATIONS_BACKEND_URL` | olibia-web | URL de este backend. Local: `http://localhost:4000` |

---

## Correr los dos en local

```bash
# terminal 1 — backend (queda en :4000, ya fijado en el script dev)
cd Back_Liq_olibia && npm run dev

# terminal 2 — front (queda en :3000)
cd olibia-web && npm run dev
```

El front ya trae `NEXT_PUBLIC_LIQUIDATIONS_BACKEND_URL=http://localhost:4000` en su `.env.local`.
Si se cambia el puerto de uno, hay que cambiarlo en el otro.

Smoke test de la conexión, con sesión iniciada en olibia:

```
GET http://localhost:3000/api/liquidations-proxy/api/me
→ 200 { id, nombre, rol, source: "olibia" }
```

`source: "olibia"` confirma que el token viajó y se validó. Sin token, `/api/me` responde 401
igual que el resto de `/api`.

---

## Troubleshooting

| Síntoma | Causa probable |
|---------|----------------|
| Todo `/api/*` responde 401 desde el front | `FIREBASE_PROJECT_ID` sin setear o distinto al del front |
| 401 solo para una persona | Su `users` está `activo: false`, o su correo cae fuera de `OLIBIA_ALLOWED_EMAIL_DOMAINS` |
| 401 con auto-provisioning en `false` | El usuario no está dado de alta en Administración |
| 404 en rutas de liquidaciones | El front cayó al backend principal: falta `NEXT_PUBLIC_LIQUIDATIONS_BACKEND_URL` |
| Funciona el dashboard propio pero no el front | Camino 1 (cookie) OK, camino 2 (Bearer) fallando — revisar logs `[auth-olibia]` |

---

## Pendientes conocidos

Se llevan en [`INTEGRACION.md` §6](../../INTEGRACION.md#6-pendientes-y-decisiones-abiertas) para no
duplicarlos: endpoints expuestos que el front no consume, la duplicación de `preview-facturacion` y
la decisión del prefijo de producción tras el gateway BIA.
