import { NextResponse } from "next/server"
import { auth } from "@/lib/auth"

/**
 * Identidad del request. Sirve de smoke test de la conexión con olibia-web:
 * si devuelve 200 con `source: "olibia"`, el proxy del front está reenviando
 * bien el Firebase ID token y este backend lo está validando y mapeando a un
 * usuario local. Un 401 acá significa que el resto de /api también va a fallar.
 */
export async function GET() {
  const session = await auth()
  if (!session) return NextResponse.json({ error: "No autorizado" }, { status: 401 })

  return NextResponse.json({
    id: session.user.id,
    nombre: session.user.nombre,
    rol: session.user.rol,
    source: session.source,
  })
}
