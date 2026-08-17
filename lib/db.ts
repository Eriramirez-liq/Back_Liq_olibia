import { PrismaClient } from "@prisma/client"

/**
 * Construye la URL de conexión reforzando parámetros seguros para entornos
 * serverless (Vercel) contra el pooler de Supabase:
 *
 *  - connection_limit → conexiones por instancia. En serverless (una lambda por
 *    request) 1 evita agotar el pool del pooler. Fuera de serverless, con un
 *    proceso largo, 1 serializa TODO: un endpoint que dispara N consultas en
 *    paralelo las encola y revienta contra el pool timeout de 10s. Se controla
 *    con SUPABASE_CONNECTION_LIMIT; el default sirve para servidor largo, que es
 *    a donde va este backend al salir de Vercel.
 *  - pgbouncer=true → desactiva prepared statements (obligatorio en transaction
 *    pooler; inocuo en session/direct).
 *
 * Solo se agregan si NO vienen ya en la DATABASE_URL, para respetar overrides.
 */
const CONNECTION_LIMIT = process.env.SUPABASE_CONNECTION_LIMIT ?? "5"
function buildDatabaseUrl(): string | undefined {
  const raw = process.env.DATABASE_URL
  if (!raw) return undefined
  try {
    const u = new URL(raw)
    if (!u.searchParams.has("connection_limit")) u.searchParams.set("connection_limit", CONNECTION_LIMIT)
    if (!u.searchParams.has("pgbouncer")) u.searchParams.set("pgbouncer", "true")
    return u.toString()
  } catch {
    // Si la URL no es parseable, usarla tal cual.
    return raw
  }
}

const globalForPrisma = globalThis as unknown as { prisma?: PrismaClient }

const prisma =
  globalForPrisma.prisma ??
  new PrismaClient({
    log: ["error"],
    datasources: { db: { url: buildDatabaseUrl() } },
  })

if (process.env.NODE_ENV !== "production") {
  globalForPrisma.prisma = prisma
}

export const db = prisma
