import { NextRequest, NextResponse } from "next/server"
import { auth } from "@/lib/auth"
import { db } from "@/lib/db"
import { leerCargosVigentes } from "@/lib/cargos-str"

/**
 * GET /api/str-por-or?periodoId=<CUID>
 *
 * Devuelve el valor STR a pagar agrupado por operador de red para el período.
 * El front sigue mandando el CUID del período de Supabase; acá se traduce a
 * "AAAA-MM", que es la clave de la tabla en calculator-prices.
 * Solo lectura (idempotente). Montos a number solo para display.
 */
export const runtime = "nodejs"

interface OperadorSTR {
  orCodigo: string
  orNombre: string
  valorCop: number
}

interface RespuestaSTRPorOR {
  operadores: OperadorSTR[]
  total: number
}

const VACIO: RespuestaSTRPorOR = { operadores: [], total: 0 }

export async function GET(request: NextRequest) {
  const session = await auth()
  if (!session) return NextResponse.json({ error: "No autorizado" }, { status: 401 })

  const { searchParams } = new URL(request.url)
  const periodoId = searchParams.get("periodoId")
  if (!periodoId) return NextResponse.json({ error: "periodoId requerido" }, { status: 400 })

  const periodo = await db.periodoConciliacion.findUnique({
    where: { id: periodoId },
    select: { anio: true, mes: true },
  })
  if (!periodo) return NextResponse.json(VACIO)

  // Los cargos STR viven en calculator-prices (base de BIA) y su clave es el
  // período como "AAAA-MM", no el CUID de Supabase.
  const periodoStr = `${periodo.anio}-${String(periodo.mes).padStart(2, "0")}`
  const registros = await leerCargosVigentes({ periodos: [periodoStr] })

  const operadores: OperadorSTR[] = registros
    .map((r) => ({
      orCodigo: r.operator_code,
      orNombre: r.operator_name,
      valorCop: Number(r.amount_payable),
    }))
    .sort((a, b) => a.orNombre.localeCompare(b.orNombre))

  const total = operadores.reduce((acc, o) => acc + o.valorCop, 0)

  return NextResponse.json({ operadores, total })
}
