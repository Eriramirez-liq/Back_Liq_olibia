import { NextRequest, NextResponse } from "next/server"
import { auth } from "@/lib/auth"
import { db } from "@/lib/db"
import { consultarFacturacionBia, BiaBillsError } from "@/lib/integrations/bia-bills"
import { mapearFilasBiaBills } from "@/lib/parsers/facturacion-bia-bills"

/**
 * POST /api/cargas/preview-facturacion
 *
 * Insumo maestro Facturacion BIA. Se consulta directamente al microservicio
 * `bia-bills` (GET /ms-bill/billing-variables?period=M-YYYY), reemplazando la
 * card de Metabase 73360 (validador-sdl).
 *
 * El período de conciliación seleccionado en el wizard ({anio, mes} = mes de
 * CONSUMO) se pasa tal cual como filtro del endpoint (formato "M-YYYY").
 *
 * Body: { anio, mes }
 * Response: { preview, filasCompletas, total, alertas, erroresCriticos,
 *             existeCargaPrevia, cargaPreviaId, columnas }
 */

export const runtime     = "nodejs"
export const maxDuration = 60

export async function POST(request: NextRequest) {
  const session = await auth()
  if (!session) return NextResponse.json({ error: "No autorizado" }, { status: 401 })

  let body: { anio?: number; mes?: number }
  try {
    body = await request.json()
  } catch {
    return NextResponse.json({ error: "Body invalido" }, { status: 400 })
  }
  const anio = Number(body.anio)
  const mes  = Number(body.mes)

  if (!anio || !mes) {
    return NextResponse.json({ error: "Parametros anio y mes son obligatorios." }, { status: 400 })
  }

  // No permitir periodos futuros
  const ahora = new Date()
  if (anio > ahora.getFullYear() || (anio === ahora.getFullYear() && mes > ahora.getMonth() + 1)) {
    return NextResponse.json(
      { error: "No se pueden cargar registros para periodos futuros." },
      { status: 400 },
    )
  }

  const periodoStr  = `${anio}-${String(mes).padStart(2, "0")}` // clave interna "AAAA-MM"
  const periodParam = `${mes}-${anio}`                          // formato del endpoint "M-YYYY"

  // 1. Consultar el servicio de Facturación BIA (bia-bills) filtrando por período
  let registros
  try {
    registros = await consultarFacturacionBia({ period: periodParam })
  } catch (e) {
    if (e instanceof BiaBillsError) {
      return NextResponse.json(
        { error: e.message, detalle: e.body ?? undefined, status: e.status },
        { status: 502 },
      )
    }
    const msg = e instanceof Error ? e.message : String(e)
    return NextResponse.json({ error: `Error al consultar Facturación BIA: ${msg}` }, { status: 500 })
  }

  const alertas: string[]         = []
  const erroresCriticos: string[] = []

  // 2. Mapear al shape FilaFacturacion (normaliza propiedad de activos y toma
  //    el período de conciliación como clave). El endpoint ya viene filtrado
  //    por período, así que no se re-filtra.
  const mapeo = mapearFilasBiaBills(registros, periodoStr)
  alertas.push(...mapeo.alertas)
  erroresCriticos.push(...mapeo.erroresCriticos)

  const filtradas = mapeo.filas
  const columnas  = filtradas.length > 0 ? Object.keys(filtradas[0]!) : []

  alertas.push(
    `Facturación BIA (bia-bills): ${filtradas.length} fronteras para period = ${periodoStr}.`,
  )
  if (filtradas.length === 0) {
    alertas.push(
      `No hay registros para el periodo ${periodoStr}. Verifica que el servicio bia-bills tenga datos para ese mes.`,
    )
  }

  // 3. Verificar carga previa
  let existeCargaPrevia = false
  let cargaPreviaId: string | undefined
  const periodoExistente = await db.periodoConciliacion.findUnique({
    where: { uq_periodo_anio_mes: { anio, mes } },
    select: { id: true },
  })
  if (periodoExistente) {
    const cargaPrevia = await db.cargaFuente.findFirst({
      where: {
        periodo_id: periodoExistente.id,
        tipo_fuente: "FACTURACION",
        estado: "COMPLETADA",
      },
      orderBy: { createdAt: "desc" },
    })
    if (cargaPrevia) {
      existeCargaPrevia = true
      cargaPreviaId = cargaPrevia.id
    }
  }

  return NextResponse.json({
    preview:          filtradas.slice(0, 20),
    filasCompletas:   filtradas,
    total:            filtradas.length,
    columnas,
    alertas,
    erroresCriticos,
    existeCargaPrevia,
    cargaPreviaId,
  })
}
