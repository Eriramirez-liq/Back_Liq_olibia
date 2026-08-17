import { consultar, enTransaccion } from "@/lib/db-bia"
import { resolverNombresOperadores } from "@/lib/agentes-str"
import type { FilaSTR } from "@/lib/parsers/insumos-str"

/**
 * Datos de Cargos STR sobre las bases de BIA.
 *
 * Reemplaza a `registros_str` de Supabase. El desglose crudo vive en
 * `file-compiler.liquidations_str_inputs` y el valor a pagar en
 * `calculator-prices.liquidations_str_charges`, vinculados por `load_id`.
 *
 * ── Una sola tabla por base ────────────────────────────────────────────────
 * Cada tabla guarda TODO: lo vigente y el histórico. Son append-only, así que
 * recargar un período no pisa lo anterior — quedan las dos cargas.
 *
 * De ahí que toda lectura tenga que quedarse con el registro más reciente de
 * cada (período, operador). Esa regla se escribe UNA vez, en `VIGENTES` acá
 * abajo, y todos los endpoints pasan por este módulo. Si alguna consulta futura
 * fuera directo a la tabla sin aplicarla, sumaría cargas viejas con nuevas y
 * duplicaría los montos.
 */

// ─── La regla de "lo vigente", en un solo lugar ─────────────────────────────
// DISTINCT ON se queda con la primera fila de cada grupo según el ORDER BY: por
// (período, operador), la de `created_at` más alto. El desempate por `id` hace
// el resultado estable si dos cargas cayeran en el mismo instante.
const VIGENTES = (tabla: string, columnas: string) => `
  SELECT DISTINCT ON (period, operator_code) ${columnas}
    FROM public.${tabla}
   ORDER BY period, operator_code, created_at DESC, id DESC`

// ─── Lectura ────────────────────────────────────────────────────────────────

export interface CargoStrVigente {
  load_id: string
  period: string
  operator_code: string
  operator_name: string
  amount_payable: string // NUMERIC llega como string: no perder precisión
  created_at: Date
}

export interface FiltroCargosStr {
  periodos?: string[]    // "AAAA-MM"
  orCodigos?: string[]
}

/** Valor a pagar vigente por (período, operador). */
export async function leerCargosVigentes(
  filtro: FiltroCargosStr = {},
): Promise<CargoStrVigente[]> {
  const condiciones: string[] = []
  const params: unknown[] = []

  if (filtro.periodos?.length) {
    params.push(filtro.periodos)
    condiciones.push(`period = ANY($${params.length})`)
  }
  if (filtro.orCodigos?.length) {
    params.push(filtro.orCodigos)
    condiciones.push(`operator_code = ANY($${params.length})`)
  }

  const where = condiciones.length ? `WHERE ${condiciones.join(" AND ")}` : ""

  return consultar<CargoStrVigente>(
    "calculator-prices",
    `SELECT * FROM (${VIGENTES(
      "liquidations_str_charges",
      "load_id, period, operator_code, operator_name, amount_payable, created_at, id",
    )}) AS vigentes
     ${where}
     ORDER BY period, operator_name`,
    params,
  )
}

/**
 * Total a pagar por período. Devuelve un mapa "AAAA-MM" → total; los períodos
 * sin datos no aparecen, para poder distinguir "cero" de "no cargado".
 */
export async function totalesPorPeriodo(periodos: string[]): Promise<Map<string, number>> {
  if (periodos.length === 0) return new Map()

  const filas = await consultar<{ period: string; total: string }>(
    "calculator-prices",
    `SELECT period, COALESCE(SUM(amount_payable), 0) AS total
       FROM (${VIGENTES(
         "liquidations_str_charges",
         "period, operator_code, amount_payable, created_at, id",
       )}) AS vigentes
      WHERE period = ANY($1)
      GROUP BY period`,
    [periodos],
  )

  return new Map(filas.map((f) => [f.period, Number(f.total)]))
}

/** Períodos que ya tienen datos cargados, más reciente primero. */
export async function periodosConCargos(): Promise<string[]> {
  const filas = await consultar<{ period: string }>(
    "calculator-prices",
    `SELECT DISTINCT period FROM public.liquidations_str_charges ORDER BY period DESC`,
  )
  return filas.map((f) => f.period)
}

// ─── Escritura ──────────────────────────────────────────────────────────────

export interface ResultadoGuardado {
  cargaId: string
  filasInsumo: number
  filasResultado: number
}

/**
 * Persiste un cargue en las dos bases.
 *
 * No hay transacción que abarque ambas, así que el orden importa:
 *   1. file-compiler (el desglose)
 *   2. calculator-prices (el valor a pagar, que es lo que la matriz muestra)
 *
 * Si (2) falla, se borran las filas de (1) por `load_id`. No es "reemplazar
 * historial" —eso el modelo no lo permite— sino limpiar una carga que nunca
 * llegó a existir. Así nunca queda un insumo sin su resultado, ni un resultado
 * a medias visible en la matriz.
 */
export async function guardarCargaStr(
  cargaId: string,
  filas: FilaSTR[],
): Promise<ResultadoGuardado> {
  if (filas.length === 0) {
    return { cargaId, filasInsumo: 0, filasResultado: 0 }
  }

  // El nombre se resuelve acá y no se toma de lo que mandó el navegador:
  // `operator_name` es NOT NULL en la tabla de resultado y es lo que verá Finanzas.
  const nombres = await resolverNombresOperadores()
  const sinNombre = filas.filter((f) => !nombres.get(f.or_codigo)).map((f) => f.or_codigo)
  if (sinNombre.length > 0) {
    throw new Error(
      `No se pudo resolver el nombre de estos operadores en el catálogo de agentes: ${sinNombre.join(", ")}.`,
    )
  }

  // ── 1. Insumo crudo en file-compiler ──────────────────────────────────
  const insumo = valoresMultifila(filas, (f) => [
    cargaId, f.periodo, f.or_codigo, f.valor_factura,
    f.refactura_1_valor, f.refactura_2_valor, f.refactura_3_valor,
  ])

  await enTransaccion("file-compiler", (cliente) =>
    cliente.query(
      `INSERT INTO public.liquidations_str_inputs
         (load_id, period, operator_code, invoice_amount,
          reinvoice_1_amount, reinvoice_2_amount, reinvoice_3_amount)
       VALUES ${insumo.placeholders}`,
      insumo.params,
    ),
  )

  // ── 2. Resultado en calculator-prices ─────────────────────────────────
  try {
    const resultado = valoresMultifila(filas, (f) => [
      cargaId, f.periodo, f.or_codigo, nombres.get(f.or_codigo)!, f.valor_a_pagar,
    ])

    await enTransaccion("calculator-prices", (cliente) =>
      cliente.query(
        `INSERT INTO public.liquidations_str_charges
           (load_id, period, operator_code, operator_name, amount_payable)
         VALUES ${resultado.placeholders}`,
        resultado.params,
      ),
    )
  } catch (e) {
    // Limpieza: la carga no llegó a existir, no debe quedar insumo huérfano.
    await consultar(
      "file-compiler",
      "DELETE FROM public.liquidations_str_inputs WHERE load_id = $1",
      [cargaId],
    ).catch((err) =>
      console.error(
        `[cargos-str] falló el rollback del insumo para carga ${cargaId}. ` +
        `Hay que borrarlo a mano: ${err instanceof Error ? err.message : String(err)}`,
      ),
    )
    throw e
  }

  return { cargaId, filasInsumo: filas.length, filasResultado: filas.length }
}

/**
 * Arma los `VALUES ($1,$2,…),($3,$4,…)` de un INSERT multifila junto con sus
 * parámetros. Parametrizado siempre: nunca interpolar valores en el SQL.
 */
function valoresMultifila<T>(
  filas: T[],
  mapear: (fila: T) => unknown[],
): { placeholders: string; params: unknown[] } {
  const params: unknown[] = []
  const grupos: string[] = []

  for (const fila of filas) {
    const valores = mapear(fila)
    const indices = valores.map((_, i) => `$${params.length + i + 1}`)
    params.push(...valores)
    grupos.push(`(${indices.join(", ")})`)
  }

  return { placeholders: grupos.join(", "), params }
}
