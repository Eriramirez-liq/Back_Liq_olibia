import type { FilaFacturacion, ResultadoParser } from "@/lib/parsers/types"
import type { RegistroBiaBills } from "@/lib/integrations/bia-bills"

/**
 * Normaliza la propiedad de activos que devuelve bia-bills a la convención
 * canónica que usa el motor de conciliación (la misma de la ruta Metabase,
 * `derivarNivelYPropiedad`): "OR" | "Usuario" | "Compartido".
 *
 * bia-bills devuelve "Operador" | "Usuario" | "Compartida".
 */
function normalizarPropiedad(v: string | null | undefined): string | null {
  if (!v) return null
  const s = v.trim().toLowerCase()
  if (s === "operador" || s === "or") return "OR"
  if (s === "compartida" || s === "compartido") return "Compartido"
  if (s === "usuario") return "Usuario"
  return v.trim()
}

/**
 * Mapea las filas del endpoint de Facturación BIA (bia-bills) al shape
 * `FilaFacturacion` que persiste el sistema.
 *
 * El endpoint ya viene filtrado por período (se consulta con `?period=`), así
 * que aquí no se filtra: el `periodo` recibido ("AAAA-MM") se asigna a todas
 * las filas para mantener la convención de clave de período del sistema.
 *
 * Notas de campos no provistos por el endpoint (quedan en null):
 *   - energia_reactiva_ind_tot / cap_tot (solo se expone la penalizada).
 *   - g_bolsa_bia (lo inyecta el orchestrator desde Metabase card 1237).
 *   - c_bia / tarifa_total_bia (no requeridos por las fórmulas de conciliación).
 */
export function mapearFilasBiaBills(
  registros: RegistroBiaBills[],
  periodo: string,
): ResultadoParser<FilaFacturacion> {
  const alertas: string[]         = []
  const erroresCriticos: string[] = []
  const filas: FilaFacturacion[]  = []

  for (const r of registros) {
    const codigo = String(r.sic ?? "").trim()
    if (!codigo) continue

    const nivel = r.nivel_de_tension != null ? String(r.nivel_de_tension) : null

    filas.push({
      codigo_frontera:          codigo,
      periodo,
      nombre_usuario:           r.frontera?.trim() || null,
      operador_red:             r.operador_de_red?.trim() || null,
      energia_kwh:              r.energia_activa ?? 0,
      nt_raw:                   nivel,
      nivel_tension:            nivel,
      propiedad_activos:        normalizarPropiedad(r.propiedad_de_activos),
      energia_reactiva_ind_tot: null,
      energia_reactiva_cap_tot: null,
      energia_reactiva_ind_pen: r.energia_reactiva_inductiva_penalizada ?? null,
      energia_reactiva_cap_pen: r.energia_reactiva ?? null,
      factor_m:                 r.factor_m ?? null,
      g_bia:                    r.tarifa_aplicada_g ?? null,
      g_bolsa_bia:              null,
      t_bia:                    r.tarifa_aplicada_t ?? null,
      d_bia:                    r.tarifa_aplicada_d ?? null,
      pr_bia:                   r.tarifa_aplicada_pr ?? null,
      r_bia:                    r.tarifa_aplicada_r ?? null,
      c_bia:                    null,
      tarifa_total_bia:         null,
      valor_total_cop:          r.valor_total_cop ?? null,
    })
  }

  if (filas.length === 0) {
    alertas.push(
      "El servicio de Facturación BIA no devolvió registros para el período seleccionado.",
    )
  }

  return { filas, alertas, erroresCriticos }
}
