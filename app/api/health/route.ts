import { NextResponse } from "next/server"
import { auth } from "@/lib/auth"
import { db } from "@/lib/db"
import { verificarTodas } from "@/lib/db-bia"
import { FUENTES_EN_BIA } from "@/lib/fuentes-migradas"

/**
 * GET /api/health
 *
 * Estado de las tres bases que hoy usa el módulo: Supabase (Prisma) y las dos
 * de BIA a las que se está migrando. Sirve para verificar el cableado sin tener
 * que cargar un archivo, y para diagnosticar un 500 sabiendo cuál de las tres
 * está caída.
 *
 * Requiere sesión, como el resto de los endpoints.
 * Responde 200 si las tres responden, 503 si alguna falla.
 */
export const runtime = "nodejs"

export async function GET() {
  const session = await auth()
  if (!session) return NextResponse.json({ error: "No autorizado" }, { status: 401 })

  const inicioSupabase = Date.now()
  const supabase = await db
    .$queryRaw`SELECT 1`
    .then(() => ({ base: "supabase", ok: true, latenciaMs: Date.now() - inicioSupabase }))
    .catch((e: unknown) => ({
      base: "supabase",
      ok: false,
      error: e instanceof Error ? e.message : String(e),
    }))

  const bases = [supabase, ...(await verificarTodas())]
  const ok = bases.every((b) => b.ok)

  return NextResponse.json(
    {
      ok,
      bases,
      fuentesMigradas: [...FUENTES_EN_BIA],
    },
    { status: ok ? 200 : 503 },
  )
}
