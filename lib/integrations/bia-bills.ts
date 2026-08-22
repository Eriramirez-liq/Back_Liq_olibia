/**
 * Cliente para el servicio de Facturación BIA (microservicio `bia-bills`).
 *
 * Consulta el endpoint `GET /ms-bill/billing-variables?period=M-YYYY`, que
 * devuelve, por frontera y período, el insumo maestro de facturación BIA
 * (energías, tarifas por componente, nivel de tensión, propiedad de activos,
 * valor_cot, valor_total_cop, etc.).
 *
 * Reemplaza la consulta a la card de Metabase 73360 (validador-sdl).
 *
 * Variables de entorno:
 *   - BIA_BILLS_API_URL (opcional) — base URL del servicio. Default
 *     "http://localhost:8080". En Vercel debe apuntar a una instancia
 *     alcanzable de bia-bills (nunca con prefijo NEXT_PUBLIC_).
 */

const DEFAULT_BASE_URL = "http://localhost:8080"

/** Una fila del endpoint /ms-bill/billing-variables (una por frontera). */
export interface RegistroBiaBills {
  contract_id: number
  sic: string
  frontera: string
  meter_id: string
  market_type: string
  inicio: string
  fin: string
  dias: number
  periodo: string
  tiene_factura: boolean
  energia_activa: number
  energia_reactiva_inductiva_penalizada: number
  energia_reactiva: number
  factor_m: number | null
  nivel_de_tension: number | null
  operador_de_red: string | null
  propiedad_de_activos: string | null
  es_embebida: boolean
  embedded_parent_provider_id: number | null
  embedded_level_tension_child: number | null
  embedded_agreed_factor: number | null
  comunidad_energetica: boolean
  valor_cot: number | null
  tarifa_aplicada_g: number | null
  tarifa_aplicada_t: number | null
  tarifa_aplicada_d: number | null
  tarifa_aplicada_pr: number | null
  tarifa_aplicada_r: number | null
  valor_total_cop: number | null
}

export class BiaBillsError extends Error {
  status?: number
  body?: string
  constructor(message: string, status?: number, body?: string) {
    super(message)
    this.name = "BiaBillsError"
    this.status = status
    this.body = body
  }
}

function getBaseUrl(): string {
  return (process.env.BIA_BILLS_API_URL ?? DEFAULT_BASE_URL).replace(/\/$/, "")
}

export interface ConsultaFacturacionBiaOptions {
  /** Período en formato "M-YYYY" (ej. "10-2025"), el que espera el endpoint. */
  period: string
  /** Timeout en milisegundos (default 60 s). */
  timeoutMs?: number
}

/**
 * Ejecuta la consulta al endpoint de Facturación BIA y devuelve las filas.
 */
export async function consultarFacturacionBia(
  { period, timeoutMs = 60_000 }: ConsultaFacturacionBiaOptions,
): Promise<RegistroBiaBills[]> {
  const base = getBaseUrl()
  const url  = `${base}/ms-bill/billing-variables?period=${encodeURIComponent(period)}`

  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), timeoutMs)

  let res: Response
  try {
    res = await fetch(url, {
      method: "GET",
      headers: {
        "x-request-id": crypto.randomUUID(),
        "x-user-id":    "olibia-liquidaciones",
      },
      signal: controller.signal,
    })
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    throw new BiaBillsError(
      `No se pudo conectar al servicio de Facturación BIA (${base}): ${msg}`,
    )
  } finally {
    clearTimeout(timer)
  }

  const text = await res.text()

  if (!res.ok) {
    throw new BiaBillsError(
      `El servicio de Facturación BIA respondió ${res.status} ${res.statusText}.`,
      res.status,
      text ? text.slice(0, 500) : undefined,
    )
  }

  let data: unknown
  try {
    data = text ? JSON.parse(text) : []
  } catch {
    throw new BiaBillsError(
      "Respuesta no-JSON del servicio de Facturación BIA.",
      res.status,
      text.slice(0, 500),
    )
  }

  if (!Array.isArray(data)) {
    throw new BiaBillsError(
      "El servicio de Facturación BIA devolvió un formato inesperado (se esperaba un arreglo).",
    )
  }

  return data as RegistroBiaBills[]
}
