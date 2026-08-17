import { NextRequest, NextResponse } from "next/server"
import { auth } from "@/lib/auth"
import { db } from "@/lib/db"
import { leerCargosVigentes } from "@/lib/cargos-str"

/**
 * GET /api/cargos-str?periodoIds=id1,id2&orIds=id1,id2
 *
 * Devuelve los cargos STR agregados por (operador, periodo_facturacion).
 * Cada período del response incluye su mes de facturación y el de consumo
 * (= facturación - 1 mes).
 *
 * Los datos salen de `calculator-prices` (base de BIA), no de Supabase. Como el
 * front sigue mandando los CUID de períodos y operadores de Supabase, acá se
 * traducen a la clave que usa la tabla nueva: el período como "AAAA-MM" y el
 * operador por código. Esa traducción se elimina cuando los filtros de la
 * pantalla dejen de venir de Supabase.
 *
 * Response shape (sin cambios para el front):
 * {
 *   periodos: [{ id, facturacion: "2026-02", consumo: "2026-01" }, ...]
 *   operadores: [{ codigo, nombre, totales: { [periodoId]: 12345.6 }, total: 9999 }]
 *   totalPorPeriodo: { [periodoId]: 100 }
 *   totalGeneral: 300
 * }
 */
export async function GET(request: NextRequest) {
  const session = await auth()
  if (!session) return NextResponse.json({ error: "No autorizado" }, { status: 401 })

  const { searchParams } = new URL(request.url)
  const periodoIdsParam = searchParams.get("periodoIds")
  const orIdsParam      = searchParams.get("orIds")

  const periodoIds = periodoIdsParam ? periodoIdsParam.split(",").filter(Boolean) : null
  const orIds      = orIdsParam      ? orIdsParam.split(",").filter(Boolean)      : null

  // Traer la data de los períodos (necesitamos anio/mes para los headers)
  const periodosRaw = await db.periodoConciliacion.findMany({
    where: periodoIds && periodoIds.length > 0 ? { id: { in: periodoIds } } : {},
    select: { id: true, anio: true, mes: true },
    orderBy: [{ anio: "asc" }, { mes: "asc" }],
  })

  const periodoStr = (anio: number, mes: number) => `${anio}-${String(mes).padStart(2, "0")}`
  // "AAAA-MM" → CUID, para poder devolver el response con las claves de siempre.
  const idPorPeriodo = new Map(periodosRaw.map((p) => [periodoStr(p.anio, p.mes), p.id]))

  // Los orIds llegan como CUID de configuracion_or; la tabla nueva usa el código.
  let orCodigos: string[] | undefined
  if (orIds && orIds.length > 0) {
    const ors = await db.configuracionOR.findMany({
      where: { id: { in: orIds } },
      select: { codigo: true },
    })
    orCodigos = ors.map((o) => o.codigo)
    // Filtro que no resuelve a ningún código: resultado vacío, no "sin filtro".
    if (orCodigos.length === 0) {
      return NextResponse.json({ periodos: [], operadores: [], totalPorPeriodo: {}, totalGeneral: 0 })
    }
  }

  const registros = await leerCargosVigentes({
    periodos: periodoIds && periodoIds.length > 0 ? [...idPorPeriodo.keys()] : undefined,
    orCodigos,
  })

  // Agregar por (or, periodo)
  const byOR   = new Map<string, { codigo: string; nombre: string; totales: Map<string, number> }>()
  const porPer = new Map<string, number>()
  let totalGen = 0
  for (const r of registros) {
    const pId = idPorPeriodo.get(r.period)
    // Un período con datos que no existe en periodos_conciliacion no tiene
    // columna donde mostrarse; se ignora en vez de romper el response.
    if (!pId) continue

    const valor = Number(r.amount_payable)
    porPer.set(pId, (porPer.get(pId) ?? 0) + valor)
    totalGen += valor
    if (!byOR.has(r.operator_code)) {
      byOR.set(r.operator_code, {
        codigo: r.operator_code,
        nombre: r.operator_name,
        totales: new Map(),
      })
    }
    const acumulado = byOR.get(r.operator_code)!
    acumulado.totales.set(pId, (acumulado.totales.get(pId) ?? 0) + valor)
  }

  // El período de los cargos STR es el de CONSUMO (lo que selecciona el usuario
  // al cargar). La facturación se deriva como consumo + 1 mes.
  function facturacionDe(anio: number, mes: number): { anio: number; mes: number } {
    if (mes === 12) return { anio: anio + 1, mes: 1 }
    return { anio, mes: mes + 1 }
  }
  const incluirSinDatos = !!(periodoIds && periodoIds.length > 0)
  const periodos = periodosRaw
    .filter(p => incluirSinDatos || (porPer.get(p.id) ?? 0) !== 0)
    .map(p => {
      const f = facturacionDe(p.anio, p.mes)
      return {
        id:          p.id,
        consumo:     periodoStr(p.anio, p.mes),
        facturacion: periodoStr(f.anio, f.mes),
      }
    })

  const operadores = Array.from(byOR.values())
    .map(o => {
      const totales: Record<string, number> = {}
      let total = 0
      for (const p of periodos) {
        const v = o.totales.get(p.id) ?? 0
        totales[p.id] = v
        total += v
      }
      return { codigo: o.codigo, nombre: o.nombre, totales, total }
    })
    .sort((a, b) => a.nombre.localeCompare(b.nombre))

  const totalPorPeriodo: Record<string, number> = {}
  for (const p of periodos) totalPorPeriodo[p.id] = porPer.get(p.id) ?? 0

  return NextResponse.json({
    periodos,
    operadores,
    totalPorPeriodo,
    totalGeneral: totalGen,
  })
}
