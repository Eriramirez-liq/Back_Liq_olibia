import { NextRequest, NextResponse } from "next/server"
import { auth } from "@/lib/auth"
import { db } from "@/lib/db"
import * as XLSX from "xlsx"

/**
 * GET /api/cargas/exportar?cargaId=X
 *
 * Genera un .xlsx con los registros GUARDADOS de una carga, cualquiera sea su
 * tipo de fuente (FACTURACION, XM, SDL, BALANCE, TC1, COT). Una fila por
 * registro con todos los campos persistidos, para auditar lo que quedó cargado.
 */
export const runtime = "nodejs"

const num = (v: unknown): number | null => {
  if (v == null) return null
  const n = Number(v)
  return isNaN(n) ? null : n
}

type Fila = Record<string, string | number | null>

/**
 * Devuelve las filas del Excel + el nombre de la hoja según el tipo de fuente.
 * Cada Registro* se mapea a sus columnas conocidas (Decimals → number con num()).
 */
async function construirFilas(
  cargaId: string,
  tipoFuente: string,
): Promise<{ filas: Fila[]; hoja: string }> {
  switch (tipoFuente) {
    case "FACTURACION": {
      const regs = await db.registroFacturacion.findMany({
        where: { carga_id: cargaId },
        orderBy: { codigo_frontera: "asc" },
      })
      return {
        hoja: "Facturación",
        filas: regs.map(r => ({
          "Código Frontera":                  r.codigo_frontera,
          "Nombre Usuario":                   r.nombre_usuario ?? "",
          "Operador Red":                     r.operador_red ?? "",
          "Energía Activa (kWh)":             num(r.energia_kwh),
          "NT (crudo)":                       r.nt_raw ?? "",
          "Nivel de Tensión":                 r.nivel_tension ?? "",
          "Propiedad de Activos":             r.propiedad_activos ?? "",
          "Reactiva Inductiva Total (kVArh)": num(r.energia_reactiva_ind_tot),
          "Reactiva Capacitiva Total (kVArh)":num(r.energia_reactiva_cap_tot),
          "Reactiva Inductiva Pen. (kVArh)":  num(r.energia_reactiva_ind_pen),
          "Reactiva Capacitiva Pen. (kVArh)": num(r.energia_reactiva_cap_pen),
          "Factor M":                         num(r.factor_m),
          "G BIA":                            num(r.g_bia),
          "G Bolsa BIA":                      num(r.g_bolsa_bia),
          "T BIA":                            num(r.t_bia),
          "D BIA":                            num(r.d_bia),
          "PR BIA":                           num(r.pr_bia),
          "R BIA":                            num(r.r_bia),
          "C BIA":                            num(r.c_bia),
          "Tarifa Total BIA":                 num(r.tarifa_total_bia),
          "Valor Total (COP)":                num(r.valor_total_cop),
        })),
      }
    }
    case "XM": {
      const regs = await db.registroXM.findMany({
        where: { carga_id: cargaId },
        orderBy: { codigo_frontera: "asc" },
      })
      return {
        hoja: "XM",
        filas: regs.map(r => ({
          "Código Frontera":      r.codigo_frontera,
          "Nombre Frontera":      r.nombre_frontera ?? "",
          "Energía Activa (kWh)": num(r.energia_xm_kwh),
        })),
      }
    }
    case "SDL": {
      const regs = await db.registroSDL.findMany({
        where: { carga_id: cargaId },
        orderBy: { codigo_frontera: "asc" },
      })
      return {
        hoja: "SDL",
        filas: regs.map(r => ({
          "Código Frontera":                  r.codigo_frontera,
          "Nombre Frontera":                  r.nombre_frontera ?? "",
          "Periodo":                          r.periodo_sdl,
          "Energía Activa (kWh)":             num(r.energia_sdl_kwh),
          "Valor SDL (COP)":                  num(r.valor_sdl_cop),
          "Tarifa SDL ($/kWh)":               num(r.tarifa_sdl),
          "Nivel de Tensión":                 r.nivel_tension ?? "",
          "Propiedad de Activos":             r.propiedad_activos ?? "",
          "Reactiva Inductiva Pen. (kVArh)":  num(r.energia_reactiva_ind_pen),
          "Reactiva Capacitiva Pen. (kVArh)": num(r.energia_reactiva_cap_pen),
          "Valor Reactiva (COP)":             num(r.valor_reactiva_cop),
          "Tarifa Reactiva ($/kVArh)":        num(r.tarifa_reactiva),
          "Factor M":                         num(r.factor_m),
        })),
      }
    }
    case "BALANCE": {
      const regs = await db.registroBalance.findMany({
        where: { carga_id: cargaId },
        orderBy: { codigo_frontera: "asc" },
      })
      return {
        hoja: "Balance",
        filas: regs.map(r => ({
          "Código Frontera":         r.codigo_frontera,
          "Periodo Ajuste":          r.periodo_ajuste,
          "Energía Balance (kWh)":   num(r.energia_balance_kwh),
          "Valor Balance (COP)":     num(r.valor_balance_cop),
          "Tarifa Balance":          num(r.tarifa_balance),
          "Periodo Tarifa":          r.periodo_tarifa,
        })),
      }
    }
    case "TC1": {
      const regs = await db.registroTC1.findMany({
        where: { carga_id: cargaId },
        orderBy: { codigo_frontera: "asc" },
      })
      return {
        hoja: "TC1",
        filas: regs.map(r => ({
          "Código Frontera":          r.codigo_frontera,
          "NIU":                      r.niu ?? "",
          "Nivel de Tensión":         r.nivel_tension ?? "",
          "Nivel Tensión Primario":   r.nivel_tension_primario ?? "",
          "% Propiedad Activo":       r.pct_propiedad_activo ?? "",
          "Propiedad de Activos":     r.propiedad_activos ?? "",
          "Tipo Conexión":            r.tipo_conexion ?? "",
          "Conexión Red":             r.conexion_red ?? "",
          "ID Comercializador":       r.id_comercializador ?? "",
        })),
      }
    }
    case "COT": {
      const regs = await db.registroCOT.findMany({
        where: { carga_id: cargaId },
        orderBy: { codigo_frontera: "asc" },
      })
      return {
        hoja: "COT",
        filas: regs.map(r => ({
          "Código Frontera":  r.codigo_frontera,
          "Nombre Frontera":  r.nombre_frontera ?? "",
          "Periodo COT":      r.periodo_cot ?? "",
          "Valor COT (COP)":  num(r.valor_cot_cop),
          "Tarifa COT":       num(r.tarifa_cot),
        })),
      }
    }
    default:
      return { filas: [], hoja: "Datos" }
  }
}

export async function GET(request: NextRequest) {
  const session = await auth()
  if (!session) return NextResponse.json({ error: "No autorizado" }, { status: 401 })

  const { searchParams } = new URL(request.url)
  const cargaId = searchParams.get("cargaId")
  if (!cargaId) return NextResponse.json({ error: "cargaId requerido" }, { status: 400 })

  const carga = await db.cargaFuente.findUnique({
    where: { id: cargaId },
    select: {
      id: true, tipo_fuente: true, nombre_archivo: true,
      periodo: { select: { anio: true, mes: true } },
      operador_red: { select: { codigo: true, nombre: true } },
    },
  })
  if (!carga) return NextResponse.json({ error: "Carga no encontrada" }, { status: 404 })

  const { filas, hoja } = await construirFilas(cargaId, carga.tipo_fuente)

  const wb = XLSX.utils.book_new()
  const ws = filas.length > 0
    ? XLSX.utils.json_to_sheet(filas)
    : XLSX.utils.aoa_to_sheet([["La carga no tiene registros."]])
  XLSX.utils.book_append_sheet(wb, ws, hoja)

  const buf = XLSX.write(wb, { type: "buffer", bookType: "xlsx" }) as Buffer
  const orCod = carga.operador_red?.codigo ?? "OR"
  const periodoStr = carga.periodo
    ? `${carga.periodo.anio}-${String(carga.periodo.mes).padStart(2, "0")}`
    : "periodo"
  const fname = `${carga.tipo_fuente.toLowerCase()}_${orCod}_${periodoStr}.xlsx`
  return new NextResponse(new Uint8Array(buf), {
    status: 200,
    headers: {
      "Content-Type": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
      "Content-Disposition": `attachment; filename="${fname}"`,
    },
  })
}
