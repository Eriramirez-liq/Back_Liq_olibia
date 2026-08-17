import { NextRequest, NextResponse } from "next/server"
import { auth } from "@/lib/auth"
import { db } from "@/lib/db"
import { SDL_OPERADORES } from "@/lib/constants/operadores"

export async function GET(request: NextRequest) {
  const session = await auth()
  if (!session) return NextResponse.json({ error: "No autorizado" }, { status: 401 })

  const { searchParams } = new URL(request.url)
  const anio = Number(searchParams.get("anio"))
  const mes  = Number(searchParams.get("mes"))

  if (!anio || !mes) {
    return NextResponse.json({ error: "Parámetros anio y mes requeridos" }, { status: 400 })
  }

  const periodo = await db.periodoConciliacion.findUnique({
    where: { uq_periodo_anio_mes: { anio, mes } },
    select: { id: true },
  })

  // SDL y TC1 aplican solo a los 21 ORs de la whitelist.
  const operadores = await db.configuracionOR.findMany({
    where: { activo: true, codigo: { in: SDL_OPERADORES } },
    select: { id: true, codigo: true, nombre: true },
    orderBy: { codigo: "asc" },
  })

  type EstadoFuente = {
    estado: "pendiente" | "cargada" | "error"
    fecha?: string
    usuario?: string
    totalRegistros?: number
    cargaId?: string
  }

  // Una sola consulta para todo el período en vez de una por (fuente, operador).
  // Antes eran 2 + 21×3 = 65 consultas en paralelo: con el pool de Prisma se
  // encolaban y, si la latencia a Supabase no era mínima, el endpoint moría
  // contra el timeout del pool.
  // COT queda fuera a propósito: está en el enum pero no tiene parser ni rama en
  // /api/cargas/preview, así que nunca podría figurar como cargada.
  const FUENTES_SEGUIDAS = [
    "FACTURACION", "XM", "SDL", "BALANCE", "TC1", "INSUMOS_STR", "INSUMOS_TARIFAS_SDL",
  ] as const

  const cargas = periodo
    ? await db.cargaFuente.findMany({
        where: {
          periodo_id: periodo.id,
          tipo_fuente: { in: [...FUENTES_SEGUIDAS] },
        },
        include: { cargado_por: { select: { nombre: true } } },
        orderBy: { createdAt: "desc" },
      })
    : []

  // Como vienen ordenadas por fecha desc, la primera de cada clave es la vigente.
  const clave = (tipoFuente: string, orId?: string | null) => `${tipoFuente}|${orId ?? ""}`
  const ultimaCarga = new Map<string, (typeof cargas)[number]>()
  for (const c of cargas) {
    const k = clave(c.tipo_fuente, c.or_id)
    if (!ultimaCarga.has(k)) ultimaCarga.set(k, c)
  }

  function estadoDe(
    tipoFuente: (typeof FUENTES_SEGUIDAS)[number],
    orId?: string,
  ): EstadoFuente {
    const carga = ultimaCarga.get(clave(tipoFuente, orId))
    if (!carga) return { estado: "pendiente" }
    return {
      estado: carga.estado === "COMPLETADA" ? "cargada" : carga.estado === "ERROR" ? "error" : "pendiente",
      fecha: carga.createdAt.toISOString(),
      usuario: carga.cargado_por.nombre,
      totalRegistros: carga.total_registros ?? undefined,
      cargaId: carga.id,
    }
  }

  const facturacion = estadoDe("FACTURACION")
  const xm = estadoDe("XM")
  // Fuentes sin OR: se cargan una vez por período, como facturación y XM.
  const insumosStr = estadoDe("INSUMOS_STR")
  const insumosTarifasSdl = estadoDe("INSUMOS_TARIFAS_SDL")

  const porOperador = (tipoFuente: "SDL" | "BALANCE" | "TC1") =>
    operadores.map((or: { id: string; codigo: string; nombre: string }) => ({
      orId: or.id,
      codigo: or.codigo,
      nombre: or.nombre,
      ...estadoDe(tipoFuente, or.id),
    }))

  const sdlResultados = porOperador("SDL")
  const balanceResultados = porOperador("BALANCE")
  const tc1Resultados = porOperador("TC1")

  return NextResponse.json({
    facturacion,
    xm,
    insumosStr,
    insumosTarifasSdl,
    sdl: sdlResultados,
    tc1: tc1Resultados,
    balance: balanceResultados,
  })
}
