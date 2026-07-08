import { NextRequest, NextResponse } from "next/server"
import { auth } from "@/lib/auth"
import { db } from "@/lib/db"
import { ejecutarCardMetabase, obtenerParametrosCard, MetabaseError } from "@/lib/integrations/metabase"
import { mapearFilasXMMetabase } from "@/lib/parsers/xm-metabase"
import { esPeriodoPermitido } from "@/lib/utils/periodos"

/**
 * POST /api/cargas/preview-xm-metabase
 *
 * Reemplaza la carga de archivo XM por una consulta a Metabase
 * (card 76099 — aenc-xm-final). Filtra por mes de consumo (fecha_inicio /
 * fecha_fin), version=TxF y todos los codigos SIC. El dato es la columna
 * "total aenc_div_perdidas".
 *
 * Body: { anio, mes }
 */
export const runtime    = "nodejs"
// La card de XM puede tardar varios minutos en computar el total del mes.
// 300s es el maximo de funciones serverless en Vercel Pro.
export const maxDuration = 300

// https://bia.metabaseapp.com/question/76099-aenc-xm-final
const METABASE_CARD_ID = 76099

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
  if (!esPeriodoPermitido(anio, mes)) {
    return NextResponse.json(
      { error: "Solo se puede cargar hasta el mes anterior (mes de consumo)." },
      { status: 400 },
    )
  }

  const periodoStr   = `${anio}-${String(mes).padStart(2, "0")}`
  const mm           = String(mes).padStart(2, "0")
  const ultimoDia    = new Date(anio, mes, 0).getDate()
  const fechaInicio  = `${anio}-${mm}-01`
  const fechaFin     = `${anio}-${mm}-${String(ultimoDia).padStart(2, "0")}`

  // Valores a inyectar en los template-tags de la card (por slug).
  // codigo_sic se omite a proposito para traer todas las fronteras.
  const valoresPorSlug: Record<string, unknown> = {
    fecha_inicio: fechaInicio,
    fecha_fin:    fechaFin,
    version:      "TxF",
  }

  // Se lee la definicion REAL de los parametros de la card y se reusa su
  // id/type/target exactos, inyectando solo el `value`. Metabase matchea los
  // parametros por `id`: un parametro armado a mano con el `type` equivocado se
  // ignora y la card cae en su valor por defecto (p. ej. traia version Tx2 en
  // vez de TxF). Reusar el objeto real garantiza que el filtro se aplique.
  let parameters: Array<Record<string, unknown>>
  try {
    const cardParams  = await obtenerParametrosCard(METABASE_CARD_ID)
    const encontrados = new Set<string>()
    parameters = cardParams
      .filter(p => typeof p.slug === "string" && p.slug in valoresPorSlug)
      .map(p => {
        encontrados.add(p.slug as string)
        return { ...p, value: valoresPorSlug[p.slug as string] } as Record<string, unknown>
      })
    // Fallback por si algun slug no vino en la metadata de la card.
    for (const [slug, value] of Object.entries(valoresPorSlug)) {
      if (!encontrados.has(slug)) {
        parameters.push({
          type:   slug === "version" ? "category" : "date/single",
          target: ["variable", ["template-tag", slug]],
          value,
        })
      }
    }
  } catch {
    // Si falla la lectura de metadata, se cae al armado manual.
    parameters = [
      { type: "date/single", target: ["variable", ["template-tag", "fecha_inicio"]], value: fechaInicio },
      { type: "date/single", target: ["variable", ["template-tag", "fecha_fin"]],    value: fechaFin },
      { type: "category",    target: ["variable", ["template-tag", "version"]],      value: "TxF" },
    ]
  }

  let resultado
  try {
    // La card de XM puede tardar varios minutos en computar; damos margen
    // amplio (290s, dentro del maxDuration de 300s de la funcion).
    resultado = await ejecutarCardMetabase({ cardId: METABASE_CARD_ID, parameters, timeoutMs: 290_000 })
  } catch (e) {
    if (e instanceof MetabaseError) {
      return NextResponse.json(
        { error: e.message, detalle: e.body ?? undefined, status: e.status },
        { status: 502 },
      )
    }
    const msg = e instanceof Error ? e.message : String(e)
    return NextResponse.json({ error: `Error al consultar Metabase: ${msg}` }, { status: 500 })
  }

  const alertas: string[] = []
  const mapeo = mapearFilasXMMetabase(resultado.rows, resultado.columnas, periodoStr)
  alertas.push(...mapeo.alertas)
  if (mapeo.erroresCriticos.length > 0) {
    return NextResponse.json({
      preview: [], filasCompletas: [], total: 0,
      columnas: resultado.columnas,
      alertas, erroresCriticos: mapeo.erroresCriticos,
      existeCargaPrevia: false, cargaPreviaId: undefined,
    })
  }

  alertas.push(
    `Metabase: ${resultado.rows.length} filas (${fechaInicio} a ${fechaFin}, version TxF) → ${mapeo.filas.length} fronteras.`,
  )

  // Verificar carga previa de XM para el periodo.
  let existeCargaPrevia = false
  let cargaPreviaId: string | undefined
  const periodoExistente = await db.periodoConciliacion.findUnique({
    where: { uq_periodo_anio_mes: { anio, mes } },
    select: { id: true },
  })
  if (periodoExistente) {
    const cargaPrevia = await db.cargaFuente.findFirst({
      where: { periodo_id: periodoExistente.id, tipo_fuente: "XM", estado: "COMPLETADA" },
      orderBy: { createdAt: "desc" },
    })
    if (cargaPrevia) {
      existeCargaPrevia = true
      cargaPreviaId = cargaPrevia.id
    }
  }

  return NextResponse.json({
    preview:        mapeo.filas.slice(0, 20),
    filasCompletas: mapeo.filas,
    total:          mapeo.filas.length,
    columnas:       resultado.columnas,
    alertas,
    erroresCriticos: [],
    existeCargaPrevia,
    cargaPreviaId,
  })
}
