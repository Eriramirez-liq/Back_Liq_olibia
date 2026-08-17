# CLAUDE.md — Back_Liq_olibia

Backend del módulo de Liquidaciones: Next 15 (App Router, solo rutas `/api/*` + un dashboard
propio) · Prisma sobre Postgres/Supabase · NextAuth v5 (login de credenciales del dashboard) ·
Vitest. Integraciones: NetSuite (Cargos STR), bia-bills (Facturación BIA), XM/Metabase.

## Antes de tocar cualquier cosa que el front consuma

Este backend **no es autónomo**: su front es `olibia-web`, un repo aparte en el mismo workspace.

**Leé [`INTEGRACION.md`](./INTEGRACION.md)** — es la bitácora viva de la conexión: qué endpoints
existen y cuáles consume el front, cómo entra la auth, qué se cambió y por qué, y qué quedó
pendiente. **Se actualiza en el mismo commit que el cambio**, siguiendo su §7.

Reglas que se rompen seguido:

- El path es el contrato: `app/api/X/route.ts` ⇄ `url: '/api/X'` en el `endpoints.ts` del front.
  Un endpoint nuevo acá no existe para el front hasta que se declara allá.
- Toda ruta autentica con `const session = await auth()` de [`lib/auth.ts`](./lib/auth.ts), que
  resuelve tanto la cookie de NextAuth como el Firebase ID token que reenvía el proxy de olibia.
  No la reimplementes por ruta.
- Errores como `{ error }` o `{ error, message }` — el front normaliza esas dos formas y ninguna otra.
- `npm run dev` usa el puerto **4000** a propósito: el front ocupa el 3000 y apunta acá.

## Documentación

| Archivo | Para qué |
|---------|----------|
| [INTEGRACION.md](./INTEGRACION.md) | Conexión con olibia-web: contrato, bitácora, pendientes |
| [docs/backend/conexion-olibia-web.md](./docs/backend/conexion-olibia-web.md) | Mecanismo de auth entre repos, envs, troubleshooting |
| [docs/backend/api/netsuite-cargos-str.md](./docs/backend/api/netsuite-cargos-str.md) | Integración NetSuite |
| [docs/runbooks/](./docs/runbooks/) | Prisma migrate, lote NetSuite colgado |
| [CONTEXTO_MIGRACION.md](./CONTEXTO_MIGRACION.md) | Historia de la migración Flask → Next |

## Comandos

```bash
npm run dev          # dev server en :4000
npm run test         # vitest (sin red ni DB: todo mockeado)
npx tsc --noEmit     # typecheck — el build NO ignora errores de tipos
npm run db:generate  # prisma generate
```
