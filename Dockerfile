# Backend del módulo de Liquidaciones.
#
# Mismo patrón que el Dockerfile de olibia-web: build multi-stage con
# `output: 'standalone'` en next.config.ts, que deja en .next/standalone un
# server.js con solo las dependencias que el runtime realmente usa.
#
# Se usa el build "dockerfile" de cactus. Los buildpacks de Node que ofrece
# (node-12, node-14, node-16) se quedan cortos: Next 15 exige Node >= 18.18.

FROM node:20-slim AS base
# Prisma necesita openssl para elegir el engine correcto en Debian slim.
RUN apt-get update && apt-get install -y --no-install-recommends openssl \
  && rm -rf /var/lib/apt/lists/*

# ── Stage 1: dependencias ───────────────────────────────────────────────────
FROM base AS deps
WORKDIR /app
# Se copia prisma/ junto al manifiesto porque el postinstall de @prisma/client
# necesita el schema para generar el cliente.
COPY package.json package-lock.json ./
COPY prisma ./prisma
RUN npm ci

# ── Stage 2: build ──────────────────────────────────────────────────────────
FROM base AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
ENV NEXT_TELEMETRY_DISABLED=1
# Placeholder: `prisma generate` y `next build` exigen que DATABASE_URL exista,
# pero no se conectan a nada. La URL real la inyecta cactus en runtime; este
# valor NO queda en la imagen final porque este stage se descarta.
ENV DATABASE_URL="postgresql://build:build@localhost:5432/build"
# Si el servicio queda detrás de un prefijo de ruta (ej. /ms-liquidations),
# basePath se resuelve en build: hay que pasarlo acá, no como variable de
# runtime. Vacío = el servicio responde en la raíz de su host.
ARG NEXT_PUBLIC_BASE_PATH=""
ENV NEXT_PUBLIC_BASE_PATH=$NEXT_PUBLIC_BASE_PATH
RUN npm run build

# ── Stage 3: runtime ────────────────────────────────────────────────────────
FROM base AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

RUN addgroup --system --gid 1001 nodejs \
  && adduser --system --uid 1001 nextjs

COPY --from=builder /app/public ./public
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static
# El cliente de Prisma y sus engines binarios NO los arrastra el tracing de
# Next: hay que copiarlos a mano o el servicio arranca y falla en la primera
# consulta con "Cannot find module '.prisma/client'".
COPY --from=builder --chown=nextjs:nodejs /app/node_modules/.prisma ./node_modules/.prisma

USER nextjs
# Coincide con el default del formulario de cactus. server.js respeta PORT.
EXPOSE 8080
ENV PORT=8080
ENV HOSTNAME="0.0.0.0"

CMD ["node", "server.js"]
